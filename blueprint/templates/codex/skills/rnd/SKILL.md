---
name: rnd
description: RND goal-driven iteration. Invoked by `RND`, `research and develop`, `iterate until`, or "find the best approach for" a testable outcome.
---

Read `.claude/skills/rnd/SKILL.md` in full — it is the source-of-truth protocol. Follow it verbatim.

## Codex-only execution mapping

- Treat RND as goal-seeking execution, not research reporting.
- Keep the goal fixed while approaches evolve.
- Use Codex tools and agents to try approaches one by one, evaluate each result, and adapt the next approach based on evidence.
- When an approach fails or scores partial, run the required 360 sweep in a separate clean-context Codex agent before iterating.
- Persist scratch outputs only where the RND protocol or user request specifies.
