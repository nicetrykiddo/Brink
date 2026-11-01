# Case study: ZCode agent on HTB Reactor (sanitized)

This document summarizes a single, authorized HTB machine attempt. It is **not** an exploit walkthrough or proof that a supervisor improves outcomes.

## Evidence

Source: user-supplied ZCode full-session ZIP, analyzed locally and excluded from the repository.

- Runtime: roughly 32.4 minutes, ending without a verified user or root flag.
- Telemetry: 96 model request records (including 2 cancelled), 108 tool usage records.
- Sum of model-request durations: 29.0 minutes, about 90% of the elapsed session. These are elapsed request durations, not CPU usage or guaranteed reasoning-only time.
- Recon identified SSH and a Next.js dashboard. From around minute 5 onward, the agent investigated a React Server Components vulnerability hypothesis by examining local package versions, diffs, and custom harnesses.
- The next substantive live target probes occurred around minute 26. Early probes against a static route yielded non-discriminating responses; later ones did not establish exploitability either. A local harness can help validate a mechanism, but its results do not prove a live exploit works.
- A manual intervention was issued around minute 21. It cancelled or interrupted an in-flight request. The next model response took roughly 273 seconds before producing a live test.
- ContextForge commands appeared during setup and near completion, but they did not operate as mandatory gates for research budgets or pivots.
- Some ContextForge CLI errors were hidden by shell pipelines returning an exit code of 0.

## Brink offline replay v0.1

- Alert `local_research_drift` appeared at approximately **minute 8.4**, after 12 classified local-research tool calls and 188.6 seconds since the previous live probe.
- A model-call latency alert appeared by minute **5.1** after a completed 130.6-second request.
- Other alerts identified 238.3-second and 273.2-second requests, plus masked tool errors.
- The output is a **retrospective heuristic audit**. It has not demonstrated that Brink would have prevented the failure, made a better decision, or even issued these warnings in real time.

## Correct next experiment

Use a new, properly isolated run with the same model, harness, target state, and budget on both sides of an A/B comparison. First verify live observation and alert delivery. Then test whether a specifically authorized intervention mechanism actually changes agent behavior. Measure flags, wall time, human intervention count, alert precision, and false positives. Do not commit full logs, flags or credentials to the public repository.
