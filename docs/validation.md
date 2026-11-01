# Validation record — 2026-11-01

## Reproduce

```sh
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./internal/supervise -run '^$' -bench BenchmarkEvidenceCheck -benchtime=30x -count=1
BRINK_ACP_HOST=/usr/bin/opencode go test ./internal/supervise -run TestOpenCodeLocal -v -count=1
```

Ordinary tests use deterministic local subprocesses. Only the last command launches installed OpenCode v2.1.4, and it uses a scripted loopback provider with a fresh environment and temporary workspace. No remote inference credentials are inherited. The test permits editing only the fixture file, with all other actions denied by configuration. The checker compares actual file content to the expected local value. It does not accept a model's success statement.

## Established control behavior

- Initialization negotiates ACP v1; mismatched versions are rejected. Prompt stop reasons are validated, and malformed cancellation results never permit resumption.
- A stalled turn is cancelled. Its outstanding response settles before the next prompt is submitted.
- In the real-host test, the scripted HTTP provider sees its original streaming connection close before redirection. OpenCode emits the cancelled prompt result. Its next turn executes a real file-writing tool and passes the external check.
- Repeated milestone names extend the deadline only once; invented milestones are rejected.
- Active tools receive bounded grace. Unrelated sessions' tool updates do not grant grace.
- Claimed success with no accepted evidence is rejected and retries remain bounded.
- A host that ignores cancellation gets no redirect. The owned process group is stopped and the result is an error.
- Failed, malformed, hanging or oversized checks cannot establish success. Host-supplied permission requests are cancelled.
- A task already passing its external check needs no host startup or model request.
- Raw host/model content is excluded from audit output; synthetic sentinel content is checked for leaks.

## Real host, scripted provider result

One recorded matched pair on this machine:

| Case | Total wall time | Redirects | Interrupted provider streams | Verified output |
| --- | ---: | ---: | ---: | --- |
| Verification-only baseline | 2.996 s | 0 | 0 | Yes |
| Supervised | 1.443 s | 1 | 1 | Yes |

The supervised evidence deadline fired at 1.3595 s, cancellation settled at 1.3755 s, and redirection was submitted at 1.3780 s. Most initial time was host startup (about 0.85 s). Both cases include the same checking mechanism; the baseline has no strategic redirect and a longer evidence deadline. A cancellation after verified completion is cleanup, not a strategic intervention.

The provider deliberately spends two seconds on the initial route and answers the redirect immediately. Therefore this experiment validates wall-clock control, resumed tool execution and independent verification; **the timing difference cannot estimate HTB speedup or the quality of a real model's pivot**. Exact durations vary on repetition. A subsequent run under Go race instrumentation also passed: baseline 3.053 s, supervised 1.458 s, one interrupted provider stream and one acknowledged cancelled turn before redirection.

The evidence-check subprocess benchmark measured 2.506 ms per check over 30 iterations on an i5-13450HX. At the default two-second polling interval, that is approximately 0.13% elapsed-time occupancy for this tiny checker, not a measured CPU percentage. Real checks can be much more expensive. No resident-memory or production-load guarantee is inferred from this microbenchmark.

CLI tests also cover configuration defaults, invalid/trailing/oversized configuration, missing arguments and success verification before host startup. SIGINT and SIGTERM use bounded cancellation cleanup. Linux parent-death signals stop direct owned children if their creating process/thread dies; this does not establish cleanup of detached descendants or crash recovery.

## Deliberately demonstrated limitations

The `false_alarm` fixture would produce valid output after 180 ms without emitting tool events. A 120 ms evidence deadline stops it too early. The uninterrupted control with a longer deadline succeeds. This explicitly demonstrates a false intervention caused by insufficient observability/budget; elapsed time alone cannot know an investigation is unproductive.

The `missed_stall` fixture never makes progress, but its evidence deadline exceeds the whole task budget. Brink detects no strategic stall before the hard deadline. This demonstrates a missed intervention under bad calibration rather than pretending the task deadline is a successful pivot.

The productive-tool fixture is permitted to finish inside bounded grace. This prevents that known false positive, but malicious or inaccurate tool-state reporting can delay review by at most the configured grace. A finite milestone list prevents unlimited fresh labels; it does not protect an evaluator that trusts agent-controlled assertions.

## Still unproven

No matched real-model HTB/CTF comparison, user/root flag improvement, automatic choice of an effective attack pivot, real-provider billing cancellation, recovery across supervisor crashes, Windows support, or intervention in an existing ZCode desktop session has been established. No global hooks, permissions, credentials or ContextForge files were modified.
