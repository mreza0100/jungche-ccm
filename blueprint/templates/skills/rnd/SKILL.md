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
  │
  ▼
Phase 1 — PLAN
  │  List N approaches (ordered by confidence / cost)
  │  Each approach has: what to try, how to evaluate
  │
  ▼
Phase 2 — EXECUTE (loop)
  ┌─────────────────────────────────────┐
  │  Pick next approach                 │
  │  Execute it                         │
  │  Evaluate: does it satisfy the goal?│
  │  Track best result so far           │
  │  Adapt remaining approaches if      │
  │    this attempt revealed new info   │
  └───────────────────┬─────────────────┘
                      │
         ┌────────────┴────────────┐
         │ satisfied AND confident  │   loop exhausted OR
         │ this is the best?        │   early-exit threshold met
         └────────────┬────────────┘
                      │
                      ▼
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

### Per-approach execution

For each approach in order:

1. **Execute** — actually do the thing. Run code, write the prompt, call the API, read the files, compute the result. Don't describe what you'd do — do it.

2. **Evaluate** — apply the success criterion defined in the plan. Be explicit: "This approach achieves X but not Y. Score: partial / full / fail."

3. **Track best** — compare to the current best result. Update if this is better.

4. **Adapt remaining plan** — this is the most important step. What did this attempt teach you?
   - Did it reveal that a later approach is a dead end? Remove it.
   - Did it suggest a better variation? Swap the next approach.
   - Did it partially work in a way that suggests a combination approach? Add it.
   - Was it a total surprise? Reorder remaining approaches.
   
   Show the user the updated approach list if it changed significantly.

5. **Early exit conditions:**
   - **Clear winner:** The result fully satisfies the goal AND is demonstrably better than any remaining approach could be. Stop.
   - **Diminishing returns:** All remaining approaches are variations of a failing pattern. Stop.
   - **User abort:** User signals "good enough." Stop.

### What "execute" means by context

RND is domain-agnostic. Execution depends on the goal:

| Goal type | Execution method |
|-----------|-----------------|
| Prompt engineering | Write the prompt, call the LLM, evaluate the output |
| Algorithm / code | Implement it, run it (Bash), measure the result |
| Research question | Search/read/grep, synthesize, evaluate completeness |
| UI/UX pattern | Describe or prototype the pattern, evaluate usability criteria |
| Data query | Write the query, run it, evaluate the output shape |
| Architecture decision | Reason through the tradeoffs, evaluate against constraints |

For code/command approaches: **run them**. Don't just reason about whether they'd work — actually execute and observe.

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

**Approaches tried:** {table or list — name, outcome, what it taught you}

**Discarded approaches:** {any that were planned but removed, and why}
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

- **Executing approaches in your head instead of running them.** "This would work because..." is analysis, not execution. RND requires actually doing the thing and observing what happens.
- **Not adapting the plan after each attempt.** If you run approach 1, learn something, then run approach 2 exactly as originally planned without considering what approach 1 revealed — you've broken the loop.
- **Setting an unevaluable success criterion.** "Find the best prompt" without defining what "best" means makes the loop aimless. Pin down the criterion in Phase 1.
- **Running too many approaches without checking in.** 5 approaches with no solution is a signal the goal framing is wrong, not a signal to try approaches 6-10 blindly.
- **Delivering the first passing result instead of the best.** "Satisfies the goal" ≠ "best result." Track the best across all approaches.
- **Letting the goal drift.** The goal is fixed. The approaches adapt. Never swap these.
