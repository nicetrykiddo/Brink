# Reactor session: what failed and what supervision must change

This is a sanitized analysis of the complete private ZCode export, not a walkthrough or a successful solve. The ZIP stays outside this repository. No screenshots were used. The analysis does not depend on public solutions and does not identify the intended exploit.

## What was examined

All eleven archive members were decoded directly: 96 model exchanges in `model-io.jsonl`, the session record, 108 messages, 417 message parts, 108 tool usage records, 96 model usage records, three turn records, three input history records, three admitted inputs, three session entries, and six todos. Each model exchange includes request messages and a response; the model-IO stream contains repeated conversation history, so its 10,811 request-message entries are not 10,811 independent actions. Raw private material is not included here.

Use database request durations for timing below. Model-IO durations differ by tens of milliseconds and must not be mixed with database accounting. Elapsed minutes are relative to the earliest recorded model request, rather than the agent's later shell timer. The original roughly 30-minute claim excluded setup; export accounting spans roughly 32.4 minutes.

| Measure | Observed |
| --- | --- |
| Model | GLM-5.3-Flash, same provider/model on all 96 records |
| Model requests | 96: 95 main-turn requests and one session-title request; including two cancellations |
| Tool usage | 108 records |
| Summed request durations | 1,738.201 seconds, about 29 minutes |
| Median request duration | 5.259 seconds |
| Five longest requests | 63.104, 71.256, 130.562, 238.345, 273.152 seconds |
| Outcome | No verified user flag, root flag, or foothold in the export |

Request time includes waiting, generation and other provider/runtime overhead. It is not CPU time or a direct measure of hidden reasoning. The long completed requests had early first-token timestamps but continued generating for much longer. A token stream therefore cannot be used as a progress heartbeat.

## Timeline and interpretation

### Minutes 0–3: useful initial evidence, plus avoidable setup friction

The agent checked the authorized lab's status, performed TCP discovery, fetched the homepage and common routes, inspected build manifests and downloaded application chunks. The visible evidence was SSH on port 22 and a Next.js dashboard on port 3000. A client chunk exposed a Next.js version string. No visible action IDs or useful extra routes were found in the inspected responses.

This was useful recon, but the conclusions had to remain scoped. A client version does not prove the server has a reachable vulnerable parser. Missing action IDs in downloaded chunks do not prove no server actions exist. A route list or wordlist is a sample, not a proof that every other surface is absent.

Several ContextForge invocations initially failed because the shell returned to the parent directory between commands. Absolute working directories or a host-maintained session directory would have avoided this. An exit status of zero did not consistently mean the requested operation succeeded: pipelines or later successful commands masked earlier errors.

### Minutes 3–8: an attractive hypothesis displaces applicability testing

A 130.562-second model request preceded a middleware bypass test and the beginning of local React decoder research. The agent downloaded package source and read parser internals. This was a plausible investigation, but it lacked the cheapest decisive target question: does a request reach the parser that the hypothesis requires?

The old replay's local-research alert fires near minute 8.4. That timestamp is a candidate review point, not a validated optimal intervention time. A legitimate exploit investigation can require source analysis. The useful intervention would ask which prerequisite is observed, which remains assumed, and what small target test distinguishes them. Automatically declaring the exploit impossible at that point would have been unjustified.

### Minutes 8–20: local understanding advances; task success does not

The agent compared package versions and patches, traced server action handling, built a decoder harness and repeatedly repaired module-resolution/react-server conditions. A 238.345-second model request occurred during this stretch. The harness eventually resolved references to JavaScript constructors/functions.

That result was real local progress: the custom harness demonstrated behavior in the selected local module and configuration. It did not establish target parser reachability, arbitrary execution on the target, a shell or either flag. The failure was the unbounded transition from plausible mechanism to increasingly expensive local reconstruction before validating that dependency on the actual target.

The harness repair loop also illustrates why command deduplication is inadequate. Many commands differed and some fixes were useful, yet all served an unvalidated prerequisite. Conversely, a repeated command can be valid when waiting for a genuine changing dependency. Supervision needs an experiment's purpose and evidence, not a raw repetition count.

### Minute 21.12: manual interruption works at the turn boundary, slowly

The first human redirect asked for short live discriminating tests and a pivot after negative results. The preceding request was recorded as cancelled after 71.256 seconds. The next request took 273.152 seconds before the first live test near minute 25.67. This is evidence of a cancelled host request followed by revised instructions, but not proof of immediate provider cancellation or efficient redirection.

The wording alone did not cap the next request. A runtime supervisor must measure time from intervention to the next verified action and be able to bound an unproductive resumed turn too. Adding a warning without an enforceable control path would not fix this failure.

### Minutes 25.67–28.04: target tests expose a missing prerequisite

The initial benign and crafted POSTs to the homepage both returned a normal HTTP 200 quickly. The response headers later included a cache hit and prerender metadata. These results did not demonstrate that the intended decoder processed either body. The agent returned to local source inspection, consuming more time before changing the target route.

A second redirect asked the agent to close that mechanism and try other surfaces. The agent argued that a dynamic 404 route justified one further discriminator. That is a potentially legitimate methodological correction: a non-discriminating experiment should not be treated as a clean refutation. But calling the 404 route dynamic did not independently prove decoder reachability either. The test returned a fast 404 without the predicted effect.

A better supervisor distinguishes three outcomes: applicability unestablished, a hypothesis contradicted by an applicable control, and success verified. Two identical HTTP statuses alone cannot choose among all three. Brink should trigger review of an overdue experiment while preserving that uncertainty.

### Minutes 28–32: rushed alternatives and overstated negatives

The agent tried targeted paths, image optimization, source maps, credential pairs, virtual hosts and a final string-body decoder variant. No foothold was established. The credential burst occupied roughly 110 seconds near the end. The agent reopened the supposedly closed decoder route for another body-format variation; a mechanism ledger could make the reason and budget for reopening explicit.

Some final conclusions exceeded the evidence:

- The UDP scan output was filtered with an expression matching TCP protocol labels. Its absence of displayed matches cannot establish the absence of open UDP ports. A successful pipeline also cannot establish that the scan itself succeeded.
- A few image and source-map checks establish only those exact request results, not that all SSRF/source disclosure mechanisms are impossible.
- Fast 404s from two parser variants do not definitively close every RSC attack or prove a decoder was reached.
- The final ContextForge record was eventually accepted with a verified label, but earlier `add` invocations failed due to incorrect CLI arguments. The label and a linked findings file do not independently validate the remote-security claims in that file.

The final report honestly admitted failure to obtain flags. Its coverage and mechanism closure claims still needed calibration. A supervisor must verify success separately and preserve the limits of negative evidence.

## Why the run failed

The observed failure is failure to achieve the task within this run, not proof that the machine was unsolvable. The strongest supported causes are an untested target applicability assumption, an unbounded local dependency/harness investigation, expensive resumed model generation after a redirect, inadequate enforcement of experiment limits, and optimistic interpretation of weak/filtered observations. Model latency magnified the cost of each strategic mistake. ContextForge stored context but was not an external controller.

There is no counterfactual evidence that interrupting at minute 8 would have produced either flag. Earlier review might recover investigation time; it might also interrupt necessary analysis. The supervisor must measure that tradeoff rather than advertise the roughly 20-minute interval as guaranteed savings.

## What would demonstrate improvement

First prove cancellation, acknowledgement, revised instructions and an independently verified outcome in a harmless local task. Include an uncooperative host, a productive long-running task, a lying success report, a missed stall and a false intervention. These are control and policy tests, not HTB performance evidence.

Then run paired lab attempts with the same model, host version, target state, scope, resource budget and machine conditions. Use fresh isolated workspaces and prohibit cross-run memory. Record time to verified foothold/user/root success, failures at a fixed budget, model/provider time, tool time, cancellation latency, redirect-to-action latency and human input. Score alerts against reviewed evidence at the time, without borrowing later knowledge. Vary order across multiple tasks/runs; one rerun on the same already-understood machine is not a fair benchmark.

A useful improvement means more verified solves or faster equally successful solves after charging for checks, false alarms and interruption overhead. No such real-model comparison has been performed or claimed here.
