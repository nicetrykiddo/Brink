# Supervision is a control and evidence problem

Brink optimizes verified success within wall-clock and resource limits. A shorter transcript, more tool calls, or earlier warnings are not substitutes. The current replay implementation cannot deliver that objective: its command substring classifier is specific to one session, mistakes embedded commands for executed actions, and has no independent outcome check or control path. Keep it temporarily as a reproducible historical diagnostic, not as the live policy.

## What must be separate

1. **Host connection:** receive lifecycle/tool events, send instructions, request cancellation, and establish whether cancellation completed. Capabilities must be demonstrated per runtime/version. Missing capabilities stay missing.
2. **Evidence:** distinguish an agent's claim from an independently observed milestone. For CTFs this can include a verified access check or flag acceptance through an authorized evaluator. HTTP status, local reproduction, or a saved note alone proves neither a foothold nor decoder reachability.
3. **Decision:** identify an overdue experiment, preserve useful progress, account for active work and intervention cost, and choose a bounded intervention. A generic supervisor cannot infer arbitrary scientific or offensive progress perfectly from shell command strings.
4. **Evaluation:** measure completion, elapsed time, intervention latency, missed stalls, false interventions and downstream damage. Compare the same model, host, task, target state and conditions. Synthetic control tests validate mechanics only.

The first live implementation uses ACP v1 as an existing host interface rather than inventing an agent-specific transport. An external, operator-controlled check supplies milestone and success evidence. This deliberately starts with measurable experiments and evidence deadlines, not a claim that deterministic rules understand arbitrary investigations. Deadline expiration asks for a discriminator/reassessment; it does not assert the hypothesis is false. Only new independently checked milestones extend the progress window. Active tool execution gets finite grace, and the whole run retains a hard deadline.

The supervisor must wait for the outstanding prompt to finish after `session/cancel`. An acknowledgement establishes host-level turn cancellation, not necessarily provider billing termination or rollback of a tool's side effects. If the host does not settle, Brink stops its owned subprocess and reports failure rather than issuing concurrent replacement prompts. Never signal an unrelated desktop process or mutate a live telemetry DB.

## ContextForge audit

Reviewed `ctx.py` and `contextforge_reasoning.py` in the existing local ContextForge checkout at `07dc4b3eb586b033a64967dec9270041f9a0258d` (origin: <https://github.com/nicetrykiddo/ContextForge>). Its `lane_start`, `lane_gate`, `lane_result`, and `audit` already cover falsifiers, deadlines, normalized test signatures, repeated attempts, evidence IDs, and reopening stopped routes. Proof validation checks a recorded source; it does not independently establish a remote exploit or task success. Gates only run when the caller invokes them. Compaction hooks restore recorded state; they do not cancel stalled model requests.

Reimplementing a memory ledger or adding another self-monitoring skill would duplicate ContextForge without fixing the control gap. Keep it optional, callable by an agent, and out of the supervisor's critical path. No changes to that repository are necessary. Its published benchmarks measure storage/retrieval, not improved real-model completion. Its README also leaves licensing unresolved, so do not copy its implementation into Brink.

## Runtime evidence

| Interface | Established | Unestablished |
| --- | --- | --- |
| Reactor ZIP and ZCode local files | Retrospective observation | Any live control from file access |
| ZCode documented hooks | Tool admission/context/stop hooks described by vendor | In-flight model cancellation; installed-build conformance |
| OpenCode v2.1.4 ACP | Local initialization negotiated v1 | Cancellation/resumption until a live test proves it |
| Owned subprocess | OS process ownership enables bounded cleanup | Undoing remote effects or killing detached grandchildren |

References: [ACP initialization](https://agentclientprotocol.com/protocol/v1/initialization), [prompt turns](https://agentclientprotocol.com/protocol/v1/prompt-turn), [cancellation](https://agentclientprotocol.com/protocol/v1/cancellation), [ZCode hooks](https://zcode.z.ai/en/docs/hooks). Vendor documentation changes; the local experiments take precedence for capability claims. No hooks were installed into global configuration.
