# Brink status — 2026-11-01

## Established facts

- Working tree began at `3aae9f4`; the untracked `brink-cli` binary predates this work and is untouched.
- Both existing test suites pass. They exercise offline replay, not supervision.
- Read every JSONL member of the complete private Reactor ZIP, including all 96 model request/response records, 417 parts, 108 tool records, three user inputs, session state, todos and turn accounting. No screenshots used or private source copied into Git.
- Reactor: 96 requests to GLM-5.3-Flash, 108 tools, 1,738.201 seconds summed request wall time; two cancelled requests. Zero verified flags in the export. Model-IO and database durations differ slightly; use database accounting consistently.
- Reviewed ContextForge local implementation at `07dc4b3eb586b033a64967dec9270041f9a0258d`, including experiment gates, audit, evidence validation and lifecycle hooks. It remains unchanged and optional.
- Installed OpenCode v2.1.4 answers ACP v1 initialization in an isolated temporary environment without a model request. This establishes protocol availability only.
- `/usr/bin/zcode --help` launches the desktop app, rather than a headless CLI. Do not use it for command discovery again. No GUI input or model prompt was sent.

## Decision and current work

Build a dependency-free Go ACP client/supervisor, separate from replay and optional memory. Use independently executed task checks for progress/success; streaming prose and tool count cannot reset a progress deadline. Confirm cancellation before sending revised instructions. Test cancellation failure explicitly. Keep runtime-specific adapters replaceable.

This is an architectural hypothesis, not an established performance improvement. Time limits can cause false positives; milestone checks and bounded active-tool grace must be tested. A success check supplied by the operator is trusted code and must not simply accept the agent's report.

## Verified checks

`go test ./...` and `python -m unittest discover -s tests -v` passed before edits.

## Blockers and limits

No remote inference or paid model use is authorized. No new HTB solve or matched real-model comparison has run. ACP cancellation/resumption has not yet been tested. ZCode's documented hooks do not establish cancellation of an in-flight model request in the installed desktop build.

## Exact next commands

```sh
go test ./...
go run ./cmd/brink replay "$HOME/data/HackTheBox/kitchen/zcode-full-session.zip"
opencode acp --help
```

Implement ACP supervision and a harmless local cancellation/resumption fixture next; update this file with actual results and runnable commands as each milestone lands.
