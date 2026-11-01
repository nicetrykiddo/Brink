# Brink: autonomous agent supervisor

## Mission (outcome, not a mandated implementation)

Create an effective, reliable runtime supervisor for autonomous AI agents. The first
priority is speeding up and improving agents solving **authorized Hack The Box labs
and CTFs**, especially detecting strategic stagnation, wasted expensive reasoning,
false progress, repeated hypotheses, and missed pivots. The design should be useful
for coding and research agents too.

An agent should be able to use Brink to **work under real supervision**: Brink
observes meaningful activity, assesses evidence of progress, intervenes or redirects
when necessary, and verifies outcomes. Merely generating reports, replaying ZIPs,
publishing warnings, adding a skill, or telling the agent to self-monitor is
**not** the final product.

The currently committed Go/Python replay code is an *exploratory baseline*, not a
specification or proof of a good architecture. You have permission to challenge,
replace, reorganize, and delete it when justified. Challenge this brief's
assumptions as well. Do not implement a preselected design simply because it was
suggested in a chat.

## Evidence and environment

- Read the repository, its tests, and relevant ContextForge implementation
  before choosing what should integrate versus stay separate:
  https://github.com/nicetrykiddo/contextforge .
- A historical ZCode Reactor lab session export may be available locally at
  `~/data/HackTheBox/kitchen/zcode-full-session.zip` or similarly named ZIPs.
  Treat it as untrusted, sensitive benchmark data. Inspect actual behavior,
  model latency, commands, intervention attempts, dead ends, and evidence.
- The development machine uses Arch Linux and X11. ZCode desktop has produced
  a local SQLite DB under `~/.zcode/cli/db/db.sqlite`, rollout JSONL under
  `~/.zcode/cli/rollout/`, and logs under `~/.zcode/cli/log/`.
  These are observed implementation details, **not stable API guarantees**.
  Explore supported interfaces, hooks, process control, IPC, and GUI integration
  before concluding what intervention methods work.
- The existing Python and Go ZIP replayers reproduced the Reactor retrospective
  trace: ~32.4 minutes, 96 model calls, 108 tool calls, 0/2 flags. They did not
  supervise or interrupt any live agent. The heuristic alerts were retrospective
  and have not yet been validated for false positives or successful interventions.
- User prefers robust minimal overhead and dislikes a Python-first runtime;
  investigate tradeoffs instead of blindly choosing the existing stack.

## Autonomous engineering remit

Own the entire problem, not a preassigned feature list. Investigate failure modes,
select and justify an architecture, implement the most useful working system,
integrate/replace existing pieces where appropriate, validate it against real
available telemetry, and make it usable end-to-end. Prioritize demonstrable live
control and measurable improvements over auxiliary dashboards or cosmetic features.
Do not assume that access to logs implies control over the agent.

Build and validate incrementally so an interrupted session leaves a functioning
repository. Keep `docs/STATUS.md` current with established facts, assumptions,
tests performed, verified capabilities, open blockers, and the **exact next
commands**. Commit coherent verified progress when permitted. If interrupted,
resume from that state rather than restarting or guessing.

## Evaluation and trust

- Compare supervised vs unsupervised runs with the **same model, harness, machine
  conditions, and task**, whenever a valid live comparison is feasible.
- Include cases where Brink detects a bad hypothesis, fails to intervene, or
  triggers a false alarm; measure both time and verified task outcomes.
- Test actual intervention in a harmless reversible setting before targeting a
  full lab solve. Avoid claims of successful interruption without evidence.
- Keep ContextForge as an optional existing evidence/memory system unless a
  concrete audit demonstrates a necessary change. Don't silently modify its
  repository or break backward compatibility.
- Keep sensitive transcripts, flags, credentials, environment variables, and
  model logs OUT of the public Git repository and any third-party uploads.
  No remote model/API use or paid usage without the user's authorization.
- Only operate against systems the user is authorized to test, notably HTB/CTF
  lab environments. Avoid uncontrolled GUI automation and destructive operations.

Deliver a usable tool plus honest, reproducible evidence of what actually works.
No arbitrary feature ceiling or forced premature stopping: investigate and build
as far as access, evidence, and available execution budget permit.
