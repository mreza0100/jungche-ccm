---
name: rnd
description: RND (Research & Develop) — goal-driven iterative skill. Takes a goal, plans multiple approaches, executes them one by one evaluating each result, and adapts the remaining plan as knowledge grows. Stops when the goal is satisfied with the best result found. Triggered when the user says "RND <goal>", "research and develop <goal>", "iterate until <goal>", "keep trying until <goal>", "find the best approach for <goal>", or "try different ways to <goal>".
---

# RND — Research & Develop

Take a goal — an outcome to achieve, not a topic to survey — and reach it by structured iteration: plan several approaches, execute them one at a time against reality, evaluate each, adapt the rest, deliver the best validated result.

## NEVER touch real code (the one inviolable boundary)

RND works ONLY inside its sandbox (§ Sandbox layout). It NEVER edits a real project file — not a `.py`, not a prompt under `knowledge/`, not via `/km`, not via any tool. It validates the fix by **in-process monkey-patch** (import the production module, patch the target at runtime from the sandbox) and ships the deliverable as a **`PROPOSED_DIFF.md`**. The Professor (or `/jc` / `/wave:builder`) judges that proposal and applies the real change only after the user ratifies it, having seen the completed RND result: whatever instruction authorized the RND itself (e.g. "fix it", "RND this and fix", "fix it with my steering") authorizes the research and the proposal, never the landing. Present the deliverable and wait for that post-RND ratification before any real file changes. An RND agent that edits a real file has broken the skill — stop and revert. This holds even when the goal is "fix X": RND's job is to find and PROVE the fix, never to land it.

## Sandbox layout

`.professor/RND/{family}/{N}-{goal}/` — `{family}` = the short name of the chain/call family the RND targets (illustrative: `extraction`, `classification`), or the system under study for a non-chain goal; `{goal}` drops the family prefix (`extraction/1-two-pass`, never `extraction/1-extraction-two-pass`); `{N}` = next integer inside that family dir (`ls` it to derive). Every artifact of one RND lives in its own numbered dir.

## Self-contained harness — no pipeline dependency

RND simulates what the production pipeline does; the harness itself carries ZERO mechanical or prompt dependency on that pipeline's code — no imported chain module, no copied or linked pipeline code, no replicated transform (slicer, filter, interleaver, template). Scope is purely the issue under study: every transformation an input undergoes is written in the sandbox and visible there. An inherited production mechanic smuggles its behavior into the experiment as an invisible confound — the observation stops being attributable to the thing under test. Production enters exactly twice, both times as a black box: the Step 0 baseline (invoked whole, verbatim) and the final in-process monkey-patch validation of `PROPOSED_DIFF.md`.

## When to load

RND needs a testable goal and iterative execution; with no way to evaluate "did we achieve this?", it is not RND. Route elsewhere:

- `RR <topic>` — knowledge-seeking, not goal-seeking
- one-shot implementation ("implement X") — `/jc` or `/wave:builder`
- pure research with no execution ("how does X work?") — RR, or answer inline

## Phase 1 — PLAN

### Step 1 — Define the goal precisely

Make the goal concrete and testable before planning approaches:

- What does "satisfied" look like — what can you check, measure, or observe?
- What does "best" mean here: faster, more accurate, fewer tokens, cleaner output, simplest code?
- What hard constraints bind (stack, time, budget, size)?

Resolve a vague goal in your reasoning; ask one short question only when the ambiguity changes which approaches make sense. An unevaluable criterion ("find the best prompt") makes the whole loop aimless — pin it down here.

### Step 2 — List approaches

Generate 2-5 approaches, each carrying: name, hypothesis (why it might work), method (concrete, executable), evaluation (how you will know it worked and how well). Order most-promising first — the simplest thing that might work leads, speculative last; mutually-exclusive architectures order by implementation cost, variations of one idea by how much they change.

Output the plan to the user before executing, so they can redirect before you invest effort.

## Phase 2 — EXECUTE

### Step 0 — Reproduce the baseline first (gate)

Before trying any improvement, run the exact thing you intend to improve — the production chain, query, or system, invoked whole as a black box, never rebuilt or partially copied into your harness — and record what it produces today. Every experimental delta is measured against that recorded baseline: a baseline reconstructed inside your harness measures your harness's drift, not a real gain.

### The depth mandate — non-negotiable

RND's value comes from stressing solutions against reality, not from confirming they look reasonable in markdown. Every execution follows these:

- **Real-world-sized inputs.** Long documents: hundreds of segments, multi-source, production-length. Queries: realistic row counts. LLM chains: production length and complexity. Toy fixtures prove the plumbing connects, not that the building survives an earthquake — 3 items where production handles 300 validates wiring, and confidence stops there.
- **Adversarial inputs by design.** For every approach, actively try to break it: malformed data, boundary values, missing fields, contradictory inputs, Unicode edge cases, empty-but-valid, valid-but-pathological. Finding where it fails is worth more than confirming the happy path.
- **Real model, real calls.** The experiment calls the actual LLM at production model + config — own client or provider SDK, per § Self-contained harness — and a fake LLM proves nothing: mocking the thing under test is writing a letter to yourself and feeling validated when you agree. Queries: real schema, realistic data shapes. Endpoints: hit the actual endpoint. A step genuinely too expensive to run is documented as omitted with confidence lowered, never silently substituted.
- **Test the artifact you intend to ship.** Validate a proposed code change in-process: import the production module, monkey-patch the target function or attribute at runtime from the sandbox, run it. A sandbox copy that "behaves like" the patch validates your understanding of the fix, not the fix — a `# simulates the fixed version` comment documents that shortcut, it does not absolve it.
- **Sandbox only.** Prototype code, test scripts, and result artifacts live in the sandbox (§ Sandbox layout); no real project file changes, no git branch or worktree (RND never uses `isolation: "worktree"` agents). Clean up `__pycache__` and build artifacts before reporting.
- **Live data between approaches.** When approach N consumes approach M's output, run M live in the same execution and pipe the result. A hardcoded literal of a prior LLM emission (`APPROACH_1_OUTPUT = [...]`) is unreproducible — the model is non-deterministic — and rots the moment the upstream prompt or seed changes.

### Per-approach execution

1. **Execute at scale** — run the code, write the prompt, call the API, read the files, compute the result. "This would work because…" is analysis, not execution.
2. **Stress-test** — the happy path passing starts evaluation. Feed the adversarial inputs, boundary values, concurrent scenarios, malformed data. Surviving earns confidence; skipping it earns none, and a break is the most valuable data point in the loop.
3. **Evaluate** — apply the plan's success criterion explicitly: "achieves X but not Y. Score: partial / full / fail", naming which adversarial inputs it survived and which broke it.
4. **Track best** — compare against the current best and update. Satisfying the goal ≠ best result; the delivery is the best across all approaches, never the first that passes.
5. **Adapt the remaining plan** — the most important step. A later approach revealed as a dead end → remove it. A better variation → swap the next one. Partial success suggesting a combination → add it. Total surprise → reorder. Failure → run the blind-spot sweep below and let the angles inform the next approach. Show the user the updated list when it changed significantly.
6. **Early exit** when the result fully satisfies the goal, survives adversarial testing, and beats anything a remaining approach could reach; when every remaining approach is a variation of a failing pattern; or when the user signals "good enough".

### Blind-spot sweep on failure

An approach that fails or scores partial means a blind spot; sweep before iterating — mandatory after a failure, optional after a pass. Spawn it in a clean context (no prior RND findings) to avoid confirmation bias:

`Agent(general-purpose)`: "Enumerate this attempt's blind spots across the failure dimensions — inputs and boundaries, state and concurrency, failure and recovery, scale and performance, security and trust, integration and contracts, observability, and the assumptions nobody stated. Subject: {one sentence on what the failed approach was trying to achieve}. Return the angle list grouped by dimension and nothing else — no analysis of your own."

Feed the returned angles into the next iteration.

### What "execute" means by goal type

- Prompt engineering: draft the candidate in the sandbox, call the real LLM, evaluate.
- Algorithm / code: implement it in the sandbox, run it (Bash), measure the result.
- LLM/AI chains: rebuild only the call under study in the sandbox, hit the real model at production model + config, parse the structured output yourself.
- Data query: write it, run it against realistic shapes and row counts, evaluate the output shape.
- Research question: search, read, grep, synthesize, evaluate completeness.
- UI/UX pattern: prototype the pattern, evaluate against usability criteria.
- Architecture decision: reason through the tradeoffs against the constraints.

### Prompt RNDs — build the fix from ranked failures

A prompt-engineering RND usually finds the failing instruction already IN the prompt, violated anyway (the contrastive ✗→✓ + counterweight technique lives in `/quality:prompt § Teaching by example`). Build that example from the failures, not intuition:

- Collect every failure mode across the runs and rank by frequency — the frequent ones mark where the model is most confused, and earn an example first.
- Cluster by shared confusion: the frequent failures usually trace to one root — a single label over-applied across distinct situations, or one surface cue overriding the real signal. Fix the cluster, not each failure.
- Add the fewest contrastive examples that resolve the cluster, never one per failure. One example placed on the boundary the model keeps crossing carries the long tail behind it.
- Confirm generalization on a held-out case. A fix that only moves the cases you tuned on is overfit; the lever that generalizes is the discriminating wording, not the example text.

### Loop discipline

- Show your work per iteration: what you tried, what happened, whether it satisfied the goal, what changes in the remaining plan.
- Execute every approach on the list, or remove it with a stated reason — no invisible skips.
- Check in with the user before a 6th approach; five with no solution signals the goal framing is wrong rather than that approaches 6-10 will land.
- The goal stays fixed while the approaches evolve — by approach 3 the plan may be entirely replaced, which is correct behavior, not drift. If the goal itself turns out to be the wrong question, stop the loop and surface that rather than silently reframing it.

## Phase 3 — DELIVER

When the loop ends, report under the heading `## RND Result`: the goal stated precisely; the winning approach; the validated fix written to the sandbox's `PROPOSED_DIFF.md` — the exact code/prompt change to apply, never applied to a real file here, landed by the Professor (or `/jc` / `/wave:builder`) only after user ratification; why that approach beat the others; the adversarial and large inputs it survived (e.g. a 500-segment document, malformed nested JSON, an empty array input, concurrent write state); every approach tried with its outcome and what it taught; approaches planned then discarded, with the reason; which failures triggered blind-spot sweeps and what blind spots those revealed.

If the loop exhausted without full satisfaction, report under `## RND Result — Best Effort`: the goal, the closest result reached, the gap against what the goal required, and one concrete next step — a different goal framing, a new approach category, or the user decision needed.

**A verdict is PROVEN only when every frozen gate passes.** A gate that fails as frozen makes the result `Best Effort`, whatever the headline numbers say — the user rules on the residual before anything ships. Report each residual by its own signature, not as a count: the input it fired on, the exact fault, and its rate. A fault seen twice on the same input is a reproducible defect, never noise, and naming it that way is what lets the next reader recognise it in production.
