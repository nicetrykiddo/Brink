# Brink

Brink supervises an owned agent process through ACP v1. It runs independent task checks, bounds time without verified progress, cancels an overdue turn, waits for it to settle, and resumes the same session with operator-supplied instructions. The runtime is dependency-free Go on Linux.

**Live cancellation and resumption are verified with installed OpenCode v2.1.4 and a loopback scripted provider. Improved real-model HTB/CTF solving is not yet established.** The [Reactor analysis](docs/reactor-case-study.md) explains the failure that motivates this work. [Current status](docs/STATUS.md) includes reproducible commands and open work.

```sh
go build -o /tmp/brink ./cmd/brink
go test ./...
go test -race ./...
go vet ./...
/tmp/brink probe -- opencode acp
```

`probe` performs initialization only: no session or model prompt. It does not establish cancellation support. `supervise` sends prompts and uses whatever provider the selected host is configured to use; configure and authorize that inference separately.

## Run a supervised task

Keep configuration and the checking program outside the agent's writable workspace. For example, save the following as a private JSON file, replacing absolute paths and instructions with your actual authorized task:

```json
{
  "cwd": "/absolute/isolated/workspace",
  "prompt": "Complete the authorized task, preserving concrete evidence for its acceptance checks.",
  "redirect": "The evidence deadline expired. Identify the unproved prerequisite, run the shortest discriminating test, and reassess the mechanism if its prerequisite remains unsupported. Preserve new positive evidence and obey the original scope.",
  "check": ["/absolute/trusted/check-program", "/absolute/isolated/workspace"],
  "milestones": ["applicability", "initial-access", "user-access"],
  "stall_seconds": 180,
  "max_seconds": 1800,
  "tool_grace_seconds": 30,
  "cancel_seconds": 10,
  "check_every_seconds": 2,
  "check_timeout_seconds": 5,
  "max_redirects": 2
}
```

```sh
/tmp/brink supervise /absolute/private/run.json -- opencode acp > /absolute/private/control.jsonl
```

The check is an argument vector, executed without an implicit shell in `cwd`. It must exit zero and emit one JSON object:

```json
{"success": false, "milestones": ["applicability"]}
```

`success` is required. Milestones must belong to the finite configured list. Each new milestone extends the evidence window once; repeats, prose, token output, notes and tool counts do not. Set an empty list if only final success can be measured, understanding that this makes false interventions more likely during productive intermediate work. Checks must be cheap, read-only and specific enough to reject forged or stale results. For HTB, use an authorized evaluator or independent access/flag validation, rather than trusting a file saying the agent succeeded. Brink supplies the check mechanism, not a universal exploit oracle.

Time fields default to the values above when omitted. The operator must supply `cwd`, `prompt`, `check`, and `redirect` (unless `max_redirects` is zero). The hard run deadline includes startup and check time; cancellation cleanup can add up to `cancel_seconds`. An active tool gets finite grace on the current evidence window, not unlimited extensions. The progress window starts after host setup. An agent that finishes without passing its check gets a bounded retry with the redirect instructions. The audit records `unverified_end` before that final check; this event alone does not mean failure.

Brink exits zero only on independently checked success, including success already present before starting a host. It exits nonzero on unverified completion, exhausted limits, check/protocol errors or failed cancellation. It never sends a second prompt while the previous prompt is outstanding. Failed cancellation stops the owned process group and reports an error.

## Local verification without inference

```sh
go test ./internal/supervise -run 'TestControl|TestKnownPolicyLimits|TestFalseAlarmControl' -v
BRINK_ACP_HOST=/usr/bin/opencode go test ./internal/supervise -run TestOpenCodeLocal -v -count=1
```

The opt-in integration test starts real OpenCode in fresh temporary XDG directories with an empty credential environment, a loopback-only provider, and permission to write one fixture file. It runs a baseline and a supervised case, checks actual file contents, and checks that the interrupted HTTP stream closes. It never contacts a remote inference provider. The provider deliberately simulates a slow route and a faster route after redirection; its timing difference is a control test, **not a real-model performance claim**. See [validation](docs/validation.md).

## Capabilities and boundaries

- ACP v1 is the first host adapter, not a requirement that arbitrary runtimes already expose control. Unsupported hosts need a separate adapter. No GUI automation or live database mutation is used.
- A host's cancelled-turn result proves that its prompt settled. It cannot guarantee provider billing cancellation or undo remote/tool side effects. The local integration additionally checks closure of the scripted provider connection.
- Permission requests received over ACP are cancelled. Brink grants no filesystem/terminal client capabilities. The host can still use its own tools under its existing permissions: **Brink is not a sandbox**. Configure host permissions and OS isolation independently.
- Audit output contains event names, times, counts and cancellation status. Raw prompts, tool output, model text, host stderr, session IDs, milestone names and credentials are excluded. Host-native logs still follow the host's own storage policy.
- External checks are trusted operator code. Same-user agents can modify accessible files or the evaluator; separate permissions/evaluator isolation are needed for adversarial validation.
- ContextForge remains optional memory. It is not required or modified. [Architecture and audit](docs/architecture.md) explain why its experiment bookkeeping does not replace runtime control.

## Historical diagnostic

```sh
go run ./cmd/brink replay "$HOME/data/HackTheBox/kitchen/zcode-full-session.zip"
```

The existing replay classifies command strings and emits retrospective alerts. Its hard-coded hints are specific to the historical experiment and are not the live supervision policy. It does not prove exploitability or predict a saved solve. The original Python replay remains a historical comparison fixture; no Python is required for live Brink.

Keep private ZIPs, flags, credentials and model logs outside public Git. The repository contains source, synthetic tests and sanitized analysis only.
