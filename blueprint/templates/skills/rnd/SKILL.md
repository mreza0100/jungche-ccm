---
name: rnd
description: RND (Research & Develop) — goal-driven iterative skill. Takes a goal, plans multiple approaches, executes them one by one evaluating each result, and adapts the remaining plan as knowledge grows. Stops when the goal is satisfied with the best result found. Triggered when the user says "RND <goal>", "research and develop", "iterate until <goal>", or "find the best approach for".
---

# RND — Research & Develop

> The iterative goal-seeker. Where RR maps the landscape and reports, RND *builds* something — trying approaches in sequence, learning from each attempt, and delivering the best result that satisfies the goal.

The user gives you a **goal** — not a topic to survey, but an **outcome to achieve**. Your job is to reach that outcome through structured iteration, adapting your approach as you learn.

---

## When to load this skill

Load when the user's message includes:

- `RND <goal>` — the canonical trigger
- "research and develop <goal>"
- "iterate until <goal>" / "keep trying until <goal>"
- "find the best approach for <goal>"
- "try different ways to <goal>"

Do NOT load for:
- `RR <topic>` — that's the research-and-report skill (knowledge-seeking, not goal-seeking)
- One-shot implementation requests ("implement X") — those go to `/build` or `/jc`
- Pure research questions with no execution ("how does X work?") — use RR or inline answer

The key distinction: **RND requires a testable goal and iterative execution.** If there's no way to evaluate "did we achieve this?", it's probably not RND.

---

## The RND loop

```
Goal
  |
  v
Phase 1 — PLAN
  |  List N approaches (ordered by confidence / cost)
  |  Each approach has: what to try, how to evaluate
  |
  v
Phase 2 — EXECUTE (loop)
  +-------------------------------------+
  |  Pick next approach                 |
  |  Execute it                         |
  |  Evaluate: does it satisfy the goal?|
  |  Track best result so far           |
  |  Adapt remaining approaches if      |
  |    this attempt revealed new info   |
  +-------------------+-----------------+
                      |
         +------------+------------+
         | satisfied AND confident  |   loop exhausted OR
         | this is the best?        |   early-exit threshold met
         +------------+------------+
                      |
                      v
Phase 3 — DELIVER
  Best result + rationale
```

---

## Phase 1 — PLAN

### Step 1 — Define the goal precisely

Before planning approaches, make the goal concrete and testable:

- What does "satisfied" look like? What can you check, measure, or observe?
- What does "best" mean here? Faster? More accurate? Fewer tokens? Cleaner output? Simplest code?
- Are there hard constraints (stack, time, budget, size)?

If the user's goal is vague, resolve it in your reasoning. If it's ambiguous in a way that changes which approaches make sense, ask one short clarifying question. Otherwise proceed.

### Step 2 — List approaches

Generate 2-5 approaches ordered from most-promising to least. For each:
- **Name** — a short label
- **Hypothesis** — why this approach might work
- **Method** — what you will actually do (concrete, executable)
- **Evaluation** — how you'll know if it worked and how well

Output the plan to the user before executing. This gives them a chance to redirect before you invest effort.

**Approach ordering principles:**
- Lead with the simplest thing that might work (cheap to try, easy to learn from)
- Put high-confidence approaches first, speculative ones last
- If approaches are mutually exclusive (different architectures), order by implementation cost
- If approaches are variations of the same idea, order by how much they change

---

## Phase 2 — EXECUTE (the loop)

### The depth mandate — non-negotiable

RND's value comes from actually stressing solutions against reality, not from confirming they look reasonable in markdown. Every execution MUST follow these rules:

1. **Use real-world-sized inputs.** If the goal involves processing transcripts, use large transcripts (hundreds of segments, multi-speaker, 45+ minutes of session content). If it involves database queries, use realistic row counts. If it involves LLM chains, use inputs that match production length and complexity. Toy fixtures prove toy things — they tell you the plumbing connects, not whether the building survives an earthquake.

2. **Design adversarial inputs.** For every approach, actively try to break it. Malformed data, boundary values, missing fields, contradictory inputs, Unicode edge cases, empty-but-valid, valid-but-pathological. The goal is to find where the solution fails, not to confirm it works on the happy path.

3. **Use actual execution paths.** For LLM/AI chains: call the real LLM (via `get_llm()` or equivalent) with real-sized prompts. For database queries: run against real schemas with realistic data shapes. For API endpoints: hit the actual endpoint. Mocking the thing you're testing is not testing — it's writing a letter to yourself and feeling validated when you agree with it.

4. **Write results to `RND/{goal-name}/`.** All prototype code, test scripts, and result artifacts go in a sandbox folder. Never modify real project files during RND. Clean up `__pycache__` and build artifacts before reporting.

### 360° integration — systematic blind-spot sweep on failure

When an approach **fails** or scores **partial**, spawn a 360° sweep before iterating:

```
Agent(general-purpose): "Read .claude/skills/360/SKILL.md and execute the 360° protocol.
Subject: {one-sentence description of what the failed approach was trying to achieve}
Domain: test
Output the full 360° angle list grouped by dimension."
```

Feed the returned angles into your next iteration — they reveal blind spots your approach missed. This is mandatory for failed approaches, optional for passing ones. The 360° agent runs in a clean context (no prior RND findings) to avoid confirmation bias.

### Per-approach execution

For each approach in order:

1. **Execute at scale** — actually do the thing with real-world-sized inputs. Run code, write the prompt, call the API, read the files, compute the result. Don't describe what you'd do — do it. Don't test with 3 items when production handles 300.

2. **Stress-test** — after the happy path works, try to break it. Feed adversarial inputs, boundary values, concurrent scenarios, malformed data. If it survives, note what you threw at it. If it breaks, that's the most valuable data point in the loop.

3. **Evaluate** — apply the success criterion defined in the plan. Be explicit: "This approach achieves X but not Y. Score: partial / full / fail." Include what adversarial inputs it survived and which ones broke it.

4. **Track best** — compare to the current best result. Update if this is better.

5. **Adapt remaining plan** — this is the most important step. What did this attempt teach you?
   - Did it reveal that a later approach is a dead end? Remove it.
   - Did it suggest a better variation? Swap the next approach.
   - Did it partially work in a way that suggests a combination approach? Add it.
   - Was it a total surprise? Reorder remaining approaches.
   - **Did it fail?** Run 360° (see above) and let the angles inform the next approach.
   
   Show the user the updated approach list if it changed significantly.

6. **Early exit conditions:**
   - **Clear winner:** The result fully satisfies the goal AND survives adversarial testing AND is demonstrably better than any remaining approach could be. Stop.
   - **Diminishing returns:** All remaining approaches are variations of a failing pattern. Stop.
   - **User abort:** User signals "good enough." Stop.

### What "execute" means by context

RND is domain-agnostic. Execution depends on the goal:

| Goal type | Execution method | Scale requirement |
|-----------|-----------------|-------------------|
| Prompt engineering | Write the prompt, call the real LLM, evaluate the output | Use production-length inputs, not 2-sentence toy prompts |
| Algorithm / code | Implement it, run it (Bash), measure the result | Test with realistic data volumes (hundreds of items, not 3) |
| LLM/AI chains | Import and invoke the actual chain with `get_llm()` | Real model, real-sized transcripts/inputs, real structured output parsing |
| Research question | Search/read/grep, synthesize, evaluate completeness | — |
| UI/UX pattern | Describe or prototype the pattern, evaluate usability criteria | — |
| Data query | Write the query, run it, evaluate the output shape | Against realistic data shapes and row counts |
| Architecture decision | Reason through the tradeoffs, evaluate against constraints | — |

For code/command approaches: **run them at scale**. Don't just reason about whether they'd work — actually execute, observe, and then try to break the result with adversarial inputs.

### Loop discipline

- **Show your work per iteration.** After each attempt, output: what you tried, what happened, whether it satisfied the goal, and what you're changing about the remaining plan.
- **Don't skip.** If an approach is on the list, execute it or explicitly remove it with a reason. No invisible skips.
- **Don't run more than 5 approaches without checking in** with the user. Long loops signal the goal needs refinement.

---

## Phase 3 — DELIVER

When the loop ends (goal satisfied, exhausted, or user abort):

```
## RND Result

**Goal:** {the original goal, stated precisely}

**Winner:** {approach name}

**Result:** {the actual output — code, prompt, answer, decision, etc.}

**Why this approach won:** {brief rationale — what made it better than others}

**Stress-tested with:** {what adversarial/large inputs it survived — e.g., "500-segment transcript, malformed character JSON, empty goals array, concurrent session state"}

**Approaches tried:** {table or list — name, outcome, what it taught you}

**Discarded approaches:** {any that were planned but removed, and why}

**360° angles triggered:** {which failed approaches triggered 360° sweeps, and what blind spots those revealed}
```

If the loop exhausted without full satisfaction:

```
## RND Result — Best Effort

**Goal:** {goal}

**Best result:** {the closest you got}

**Gap:** {what the goal required that wasn't achieved}

**Recommendation:** {one concrete next step — different goal framing, new approach category, user decision needed}
```

---

## Adaptive planning — the critical difference from other skills

The reason RND exists separately from `/build` or RR is the **adaptive loop**. Most pipelines plan up front and execute. RND's plan is a living document:

- After approach 1: you know more than before it ran. Update the plan.
- After approach 2: you know even more. Update again.
- By approach 3: you might have replaced the original plan entirely — and that's correct behavior, not drift.

The key invariant: **the goal stays fixed, the approaches evolve.** Never let the approach loop drift the goal itself. If the goal turns out to be the wrong question, stop the loop and surface that to the user — don't silently reframe it.

---

## Common failure modes

- **Testing with toy inputs and claiming victory.** A 3-item fixture passing does not prove the solution works at production scale. If you tested with small inputs, you validated the wiring — not the system. Scale up or don't claim confidence.
- **Executing approaches in your head instead of running them.** "This would work because..." is analysis, not execution. RND requires actually doing the thing and observing what happens.
- **Not stress-testing after the happy path passes.** The happy path passing is the START of evaluation, not the end. Feed adversarial inputs. Try to break it. If you can't break it, you've earned confidence. If you didn't try, you haven't.
- **Skipping 360° after a failure.** When an approach fails, you have a blind spot. 360° exists to find it systematically. Skipping it means your next approach inherits the same blind spot.
- **Mocking the thing you're testing.** If you're validating an LLM chain, calling a fake LLM proves nothing about the chain's real behavior. Use actual execution paths.
- **Not adapting the plan after each attempt.** If you run approach 1, learn something, then run approach 2 exactly as originally planned without considering what approach 1 revealed — you've broken the loop.
- **Setting an unevaluable success criterion.** "Find the best prompt" without defining what "best" means makes the loop aimless. Pin down the criterion in Phase 1.
- **Running too many approaches without checking in.** 5 approaches with no solution is a signal the goal framing is wrong, not a signal to try approaches 6-10 blindly.
- **Delivering the first passing result instead of the best.** "Satisfies the goal" ≠ "best result." Track the best across all approaches.
- **Letting the goal drift.** The goal is fixed. The approaches adapt. Never swap these.
