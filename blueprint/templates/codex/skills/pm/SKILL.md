---
name: pm
description: "Product Manager. Invoked as $pm <request>. Feature reviews, UX friction analysis, prioritization, PRD updates. Dual lens — {USER_PERSONA} AND experienced PM."
---

Read `.claude/commands/pm.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** feature review request, UX question, prioritization task, or PRD update.

## Codex-only differences

- The shared `rr` protocol lives at `.claude/skills/rr/SKILL.md` and is exposed to Codex through `.codex/skills/rr/SKILL.md`.
- If the request includes `RR`, `RRP`, "research and report", "market research", "persona research", "competitors", "landscape", or another broad product research task, execute an RR-compatible pipeline: read the RR skill, scout or pre-split subquestions, spawn Codex research agents in parallel, and aggregate one report under `docs/dev/research/`.
- Do not satisfy explicit RR or broad research with inline WebSearch/WebFetch in the main thread. Use WebSearch/WebFetch directly only for narrow fact checks inside a non-RR PM answer.
- The PRD lives at `docs/agents/PRD.md` — you may update it directly.
