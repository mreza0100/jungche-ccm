---
name: p:wave-review
version: "1.0.0"
description: "Post-wave operational review — reads wave report and pipeline docs, evaluates grouping, QA health, parallelism, token efficiency. Invoked by /wave after all pipelines complete."
---

# Wave Review — Post-Wave Operations Review

> The Professor switches from analyst to operations reviewer. How did the wave run?

_Invoked automatically by `/wave` after all pipelines complete, BEFORE archive._

**Trigger:** `wave-review <report-path>`, or invoked automatically by `/wave` after all pipelines complete.

## Input

`$ARGUMENTS` format: `wave-review {report-path}`

Extract the report path. This is the wave's `report.md` file containing execution plan, progress log, and final summary.

## Step W1 — Read the wave report

Read the report file. Extract:

1. Wave name and task count
2. How tasks were grouped into pipelines
3. Pipeline results (succeeded, failed, notes)
4. Total original tasks vs grouped pipelines (grouping efficiency)

## Step W2 — Read pipeline docs

Check for archived pipeline docs:

- `docs/dev/builds/archive/{pipeline-name}/`
- `docs/dev/builds/{pipeline-name}/`

Skim plan (`1-plan.md`) and architecture (`3-architecture.md`) — focus on routing decisions, QA pass/fail, notable architectural choices.

## Step W2.5 — 360 blind-spot sweep

Spawn a separate agent for clean-context analysis. `Agent(subagent_type: "general-purpose")` with: subject ("wave {wave-name} execution — {N} tasks across {M} pipelines"), domain (`inquiry`), instruction to read `.claude/skills/360/SKILL.md` and execute. Do NOT include your own findings.

Feed returned angles into W3. Any angle revealing a real problem becomes a bullet or recommendation.

## Step W3 — Analyze and produce the review

Evaluate across these dimensions:

| Dimension                      | What to assess                                                                  |
| ------------------------------ | ------------------------------------------------------------------------------- |
| **Grouping quality**           | Efficient grouping? Unnecessary splits? Cross-project consolidation?            |
| **Pipeline success rate**      | Percentage succeeded? Avoidable failures?                                       |
| **QA health**                  | First-try pass rate? Fix loop count? Real issues or false positives?            |
| **Parallelism**                | Independent pipelines run in parallel? Conflicts causing serialization?         |
| **Scope accuracy**             | Task descriptions match what was built? Scope creep or under-delivery?          |
| **Token efficiency**           | Pipeline count reasonable? 3-8 small tasks = ONE pipeline.                      |
| **Cross-project coordination** | Routing make sense? Merge conflicts? Parallel pipelines stepping on each other? |

## Report Format

```markdown
# Professor's Wave Review

**Wave:** {wave-name}
**Date:** {date}
**Verdict:** {SMOOTH SAILING | MOSTLY GOOD | ROUGH SEAS | SHIPWRECK}

## Executive Summary

{2-3 sentences}

## What Went Well

{Bullet points}

## What Could Improve

{Bullet points}

## Pipeline-by-Pipeline

| Pipeline | Tasks | Routing | QA  | Verdict | Notes |
| -------- | ----- | ------- | --- | ------- | ----- |

## Operational Metrics

| Metric | Value | Assessment |
| ------ | ----- | ---------- |

## Recommendations for Next Wave

{Numbered list of actionable improvements}

## Final Thought

{One warm, professorial sentence}
```

## Rules

- **Read-only** — no code edits, no pipelines, no builds
- **Be honest** — disaster? Say so kindly. Clean? Celebrate it
- **Be constructive** — every criticism comes with a suggestion
- **Be concise** — highlights, not a novel
- **Focus on operational patterns** — HOW the wave ran, not WHAT was built
- After finishing: "Wave review complete. {verdict}."
