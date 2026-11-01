# Brink status — 2026-11-01

## Verified milestones

1. `4b3585e`: direct audit of the complete private Reactor export and ContextForge implementation; [case study](reactor-case-study.md) and [architecture](architecture.md). No screenshots or public solutions.
2. `a2cb1f7`: dependency-free Go live supervisor implemented: ACP initialization, new session, streamed tool lifecycle, independent external checks, finite milestone progress, bounded tool grace, cancel/settle/redirect, overall deadline and owned process-group cleanup. See [README](../README.md).
3. Real installed OpenCode v2.1.4 plus a loopback scripted provider: cancelled turn acknowledged, original HTTP stream closed, resumed tool wrote the expected file, independent checker accepted its content. One verification-only baseline took 2.996 s; supervised case 1.443 s. A later race-instrumented pair passed at 3.053 s and 1.458 s. **Synthetic control validation, not real-model/HTB performance evidence.**

## Tests and measurements

- `go test ./... -count=1`, `go test -race ./... -count=1`, and `go vet ./...` pass.
- Local control fixtures cover stalls, lies, milestone reuse, active-tool grace, ignored cancellation, permission denial, checker failure/timeout/schema errors, already-verified success, missed stalls and false interventions. A longer-budget control proves the false-intervention fixture would succeed uninterrupted.
- Opt-in real OpenCode integration passes. Test setup uses an empty credential environment, fresh XDG directories, local scripted provider and one allowed file edit.
- External-check microbenchmark: 2.506 ms/check, 30 iterations; this is spawn/check overhead, not a solve-speed benchmark.
- Historical Python tests and the private Go replay were rerun after implementation and passed; Python replay remains only a historical comparison fixture. No Python dependency in live runtime.
- Hardening adds SIGTERM handling, Linux parent-death signals for owned children, strict prompt-result validation, and bounded configuration input. Cancellation errors never count as a successful redirect.
- Full evidence and limitations: [validation](validation.md).

## Established environment and integration facts

- Original untracked `brink-cli` binary is untouched. No Git history rewritten or remote push performed.
- ContextForge local implementation reviewed at `07dc4b3eb586b033a64967dec9270041f9a0258d`; memory, gates and hooks remain optional and unchanged.
- The Reactor ZIP has 96 model exchanges, 417 parts, 108 tool records, three inputs and zero verified flags (95 main-turn requests plus one title request). Summed model request time is 1,738.201 s. Keep it private outside this repository.
- `/usr/bin/zcode --help` launches the desktop app; do not use it for CLI discovery. Bundled CLI exists at `/usr/lib/zcode/glm/zcode.cjs` (version 0.17.0).
- Read bundled CLI implementation: it has a stdio `app-server`, `session/create`, `session/send`, `session/stop` and session snapshots/events. `session/send` immediately admits background work; its response does not mean the turn completed. `session/stop` aborts a controller; the response alone does not prove cancellation settled.
- A network-disabled Bubblewrap process with a masked home directory successfully answered `runtime/capabilities`. No model prompt or live desktop mutation was sent. The protocol differs from ACP and cannot be passed to Brink's ACP adapter directly. [Reproducible isolated probe](zcode-interface.md).
- Online ZCode hook documentation conflicts with some bundled guide text on workspace hook execution. Do not rely on either as installed-build conformance proof. No global hooks installed.
- The documented Bubblewrap capability probe was executed verbatim and passed.

## Current limits and next work

Brink now has a working live control path, but its first decision policy is an evidence deadline with operator-supplied redirect instructions. It does not yet independently select strategic attack pivots or semantically judge arbitrary target results. A sparse/incorrect milestone checker can interrupt productive work. The check program must be independently trustworthy; same-user execution is not a sandbox.

No remote inference/paid use is authorized, so a new real-model lab solve and matched live comparison have not run. No local text-model runtime was found in PATH; the inspected Hugging Face cache contains a speech model. This is a scoped availability check, not an exhaustive disk inventory. Continue locally: extract a second host adapter only when tested protocol semantics justify it; start with an isolated ZCode app-server and local provider, proving idle-after-stop before another send. Keep control and evidence independent of its private storage layout.

## Exact next commands

```sh
go build -o /tmp/brink ./cmd/brink
/tmp/brink probe -- opencode acp
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
BRINK_ACP_HOST=/usr/bin/opencode go test ./internal/supervise -run TestOpenCodeLocal -v -count=1
node /usr/lib/zcode/glm/zcode.cjs --help
```

Before any real-model comparison, explicitly obtain inference authorization, provision independent evaluator checks and paired isolated task conditions. Do not infer permission from existing credentials or replay data. Do not rerun the historical offensive commands just to validate the supervisor.
