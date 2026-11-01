"""Deterministic, read-only replay of exported ZCode sessions.

Security boundary: reports contain *metadata only*, never prompts, command text,
model reasoning, tool output or environment variables. No network/LLM calls.
"""

from __future__ import annotations

import argparse
from collections import Counter, deque
import json
from pathlib import Path
import re
import sys
import zipfile


LOCAL_HINTS = (
    "/pkg/", "npm pack", "npm view", "npm install", "rsc_patch",
    "harness", "react-server-dom", "server.node.", "node -e", "conditions.js",
    "exp1.js", "exp2.js", "diff ", "patch.diff", "shim/node_modules",
    "package.json", "app-render/action-handler", "server-action-request-meta",
)
LIVE_HINTS = (
    "curl ", "nmap ", "gobuster ", "ffuf ", "feroxbuster ",
    "ssh ", "sshpass ", "nc ", "ping ", "dig ", "nikto ",
    "whatweb ", "httpx ", "wfuzz ", "sqlmap ",
)
MEMORY_HINTS = ("contextforge", ".ctx/")
DIAGNOSTIC_RE = re.compile(
    r"(?:command not found|no such file or directory|"
    r"the following arguments are required|traceback \(most recent call last\))",
    re.I,
)


def to_int(value, default=0):
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


def command_category(tool_name: str, command: str) -> str:
    if tool_name != "Bash":
        return "other_tool"
    s = command.lower()
    if any(k in s for k in MEMORY_HINTS):
        return "memory"
    # Package archaeology takes precedence over commands embedded in scripts.
    if any(k in s for k in LOCAL_HINTS):
        return "local_research"
    if any(k in s for k in LIVE_HINTS):
        return "live_probe"
    if any(k in s for k in ("find ", "grep ", "sed ", "cat ", "ls ", "pwd", "wc ")):
        return "local_other"
    return "other"


def iter_jsonl(archive: zipfile.ZipFile, name: str):
    if name not in archive.namelist():
        return
    with archive.open(name) as fp:
        for raw in fp:
            if raw.strip():
                yield json.loads(raw)


def load_events(path: Path) -> tuple[list[dict], dict]:
    with zipfile.ZipFile(path) as archive:
        names = set(archive.namelist())
        if "database/part.jsonl" not in names or "database/tool_usage.jsonl" not in names:
            raise ValueError("Expected ZCode full-session ZIP with part and tool_usage JSONL")

        tools = list(iter_jsonl(archive, "database/tool_usage.jsonl"))
        usage = list(iter_jsonl(archive, "database/model_usage.jsonl"))
        sessions = list(iter_jsonl(archive, "database/session.jsonl"))
        by_call = {t.get("tool_call_id"): t for t in tools}
        events = []
        for item in iter_jsonl(archive, "database/part.jsonl"):
            data = item.get("data")
            if isinstance(data, str):
                try:
                    data = json.loads(data)
                except json.JSONDecodeError:
                    continue
            if not isinstance(data, dict) or data.get("type") != "tool":
                continue
            state = data.get("state") or {}
            inp = state.get("input") or {}
            command = str(inp.get("command", "")) if isinstance(inp, dict) else ""
            tool_name = str(data.get("tool", "unknown"))
            stat = by_call.get(data.get("callID"), {})
            ts = to_int(stat.get("started_at"), to_int(item.get("time_created")))
            duration = to_int(stat.get("duration_ms"))
            exit_code = stat.get("exit_code")
            output = state.get("output")
            diagnostic = isinstance(output, str) and bool(DIAGNOSTIC_RE.search(output))
            events.append({
                "kind": "tool", "ts": ts, "end": to_int(stat.get("completed_at"), ts + duration),
                "tool": tool_name, "category": command_category(tool_name, command),
                "duration_ms": duration, "failed": (exit_code not in (None, 0) or bool(stat.get("error_type"))),
                "masked_error": diagnostic and exit_code in (None, 0),
                "checkpoint_success": tool_name == "Bash" and "contextforge" in command.lower()
                    and isinstance(output, str) and ("Checkpoint saved" in output or " [verified]" in output),
            })
        for m in usage:
            start = to_int(m.get("started_at"))
            duration = to_int(m.get("duration_ms"))
            end = to_int(m.get("completed_at"), start + duration)
            events.append({"kind": "model", "ts": start, "end": end,
                           "duration_ms": duration,
                           "status": str(m.get("status", "unknown")),
                           "input_tokens": to_int(m.get("input_tokens")),
                           "output_tokens": to_int(m.get("output_tokens"))})
        events.sort(key=lambda e: (e["ts"], e["kind"]))
        start_candidates = [e["ts"] for e in events if e["ts"] > 0]
        start = min(start_candidates) if start_candidates else 0
        end = max((e["end"] for e in events), default=start)
        if sessions:
            end = max(end, to_int(sessions[0].get("time_updated")))
        meta = {"session_count": len(sessions), "started_ms": start, "ended_ms": end,
                "tool_records": len(tools), "model_records": len(usage)}
        return events, meta


def detect(events: list[dict], meta: dict, *, model_budget_s=60,
           research_budget_s=180, min_research_calls=8, fail_window=8, fail_limit=3):
    """Return conservative alerts. A signal is NOT a claim that an exploit failed."""
    alerts = []
    last_live_end = None
    local_since_live = 0
    drift_reported = False
    recent_failures = deque(maxlen=fail_window)
    tool_counts = Counter()
    model_count = 0
    model_total = 0
    model_cancelled = 0
    masked_count = 0
    checkpoints = 0

    def alert(time_ms, code, description, evidence):
        alerts.append({"elapsed_s": max(0, round((time_ms - meta["started_ms"]) / 1000, 1)),
                       "code": code, "message": description, "evidence": evidence,
                       "intervention": "advisory_only"})

    for e in events:
        if e["kind"] == "model":
            model_count += 1
            model_total += e["duration_ms"]
            if e.get("status") == "cancelled":
                model_cancelled += 1
            if e["duration_ms"] >= model_budget_s * 1000:
                alert(e["end"], "slow_model_call", "Model call exceeded its latency budget.",
                      {"duration_s": round(e["duration_ms"] / 1000, 1), "budget_s": model_budget_s})
            continue
        category = e["category"]
        tool_counts[category] += 1
        if category == "live_probe":
            last_live_end = e["end"]
            local_since_live = 0
            drift_reported = False
        elif category == "local_research":
            local_since_live += 1
            elapsed = max(0, e["ts"] - (last_live_end or meta["started_ms"]))
            if (not drift_reported and local_since_live >= min_research_calls
                    and elapsed >= research_budget_s * 1000):
                alert(e["ts"], "local_research_drift",
                      "Extended local investigation without a new live target test.",
                      {"research_calls_since_live": local_since_live,
                       "seconds_since_live": round(elapsed / 1000, 1)})
                drift_reported = True
        if e["masked_error"]:
            masked_count += 1
            alert(e["end"], "masked_tool_error", "Tool output contains an error despite a successful or missing exit code.",
                  {"tool": e["tool"], "category": category})
        if e["checkpoint_success"]:
            checkpoints += 1
        recent_failures.append(bool(e["failed"]))
        if len(recent_failures) == fail_window and sum(recent_failures) >= fail_limit:
            if not alerts or alerts[-1]["code"] != "tool_failure_burst":
                alert(e["end"], "tool_failure_burst", "Several recent tool calls failed.",
                      {"failed_in_recent_window": sum(recent_failures), "window": fail_window})
            recent_failures.clear()

    elapsed = max(0, meta["ended_ms"] - meta["started_ms"])
    return {
        "schema_version": 1,
        "privacy": "Metadata only; no prompts, commands, tool outputs, or raw model traces.",
        "scope": "Retrospective replay. No real-time alerts or interventions were performed.",
        "duration_s": round(elapsed / 1000, 1),
        "model_calls": model_count, "cancelled_model_calls": model_cancelled,
        "tool_calls": sum(tool_counts.values()),
        "model_wall_s_sum": round(model_total / 1000, 1),
        "category_counts": dict(tool_counts),
        "masked_tool_errors": masked_count, "verified_checkpoint_responses": checkpoints,
        "alerts": sorted(alerts, key=lambda a: a["elapsed_s"]),
    }


def summarize(report: dict) -> str:
    lines = [f"Brink offline replay | duration {report['duration_s'] / 60:.1f}m | "
             f"{report['model_calls']} model calls | {report['tool_calls']} tools",
             f"Model-call wall time (sum): {report['model_wall_s_sum'] / 60:.1f}m "
             f"({report['cancelled_model_calls']} cancelled requests included)",
             f"Mechanisms: {', '.join(f'{k}={v}' for k, v in sorted(report['category_counts'].items()))}",
             f"Alerts: {len(report['alerts'])} (retrospective, advisory only)"]
    for a in report["alerts"]:
        detail = a["evidence"]
        lines.append(f"  +{a['elapsed_s'] / 60:.1f}m  {a['code']}  {detail}")
    return "\n".join(lines)


def main(argv=None) -> int:
    parser = argparse.ArgumentParser(prog="brink", description="Offline agent supervisor replay")
    sub = parser.add_subparsers(dest="action", required=True)
    cmd = sub.add_parser("replay", help="Analyze a ZCode exported full-session ZIP")
    cmd.add_argument("zip", type=Path)
    cmd.add_argument("--json", metavar="PATH", type=Path, dest="output")
    cmd.add_argument("--model-budget", type=int, default=60, help="Model-call latency threshold in seconds")
    cmd.add_argument("--research-budget", type=int, default=180, help="Local-research drift threshold in seconds")
    args = parser.parse_args(argv)
    try:
        events, meta = load_events(args.zip)
        report = detect(events, meta, model_budget_s=args.model_budget,
                        research_budget_s=args.research_budget)
        print(summarize(report))
        if args.output:
            args.output.parent.mkdir(parents=True, exist_ok=True)
            args.output.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
            print(f"Metadata-only JSON written to {args.output}")
        return 0
    except (OSError, ValueError, zipfile.BadZipFile, json.JSONDecodeError) as exc:
        print(f"brink: {exc}", file=sys.stderr)
        return 2
