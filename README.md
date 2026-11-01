# Brink

**Observe. Detect. Intervene (eventually).**

Brink is an experimental, deterministic supervisor for autonomous agents. It targets HTB/CTF workflows first, but the engine is designed to support other tasks. **v0.1 implements offline replay only.** It does not control ZCode, interrupt the model, verify flags, or use any LLM.

## First use: audit a ZCode run

Requires Python 3.10+ and an exported ZCode session ZIP containing `database/part.jsonl`, `database/tool_usage.jsonl`, and `database/model_usage.jsonl` (optional).

```bash
python3 -m brink replay "$HOME/Downloads/zcode-full-session(1).zip" --json replay-report.json
python3 -m unittest discover -s tests -v
```

Output contains counts, tool mechanism classifications, delayed model calls, and time spent on local research without a live probe. Reports **never include raw prompts, commands, outputs, or model reasoning**. Do not commit/export raw session ZIPs to a public repo: they can contain credentials, flags, private data, and internal reasoning.

## What the findings do and do not prove

- Long model calls consume wall-clock time, but may contain legitimate analysis. The current threshold is an **alert**, not proof that the model is wrong.
- `local_research_drift` detects local package/code investigation that continues after the last live target probe. Some long local investigations are necessary. The rule is **heuristic** and may have false positives/negatives.
- `masked_tool_error` flags a known trap: piping a failing CLI command into `tail` or `head` can hide the failing exit code.
- Every alert is **retrospective**. No real-time detection or GUI intervention has been verified.
- No attack techniques, test target IPs, secrets, or raw transcripts belong in this repository.

## Next milestones

1. Add live read-only ZCode SQLite/JSONL observation and stable incremental event IDs.
2. Benchmark alert precision against manually reviewed checkpoints, not just alert counts.
3. Prove X11 GUI control against a disposable window before testing ZCode. Only then add optional, explicitly authorized intervention.
4. Add OpenCode adapter. Keep mission policy separate from adapter and supervisor.
5. Integrate ContextForge as an optional evidence store; keep backward compatibility and avoid duplicated responsibilities.

## Constraints

Python standard library only; safe on Linux; no subprocess/network calls during replay. Treat `.zcode` and raw exports as sensitive. The GitHub repo must contain **source and synthetic tests only**.
