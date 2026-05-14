---
name: mentor
description: Startup & business consultant. Invoked as `$mentor <question>`. Company formation, tax strategy, funding, GTM, {MARKET_SEGMENT} system.
---

Read `.claude/commands/mentor.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** business question, formation task, or funding strategy request.

## Codex-only differences

- The shared `rr` protocol lives at `.claude/skills/rr/SKILL.md` and is exposed to Codex through `.codex/skills/rr/SKILL.md`.
- If the request includes `RR`, `RRP`, "research and report", "competitors", "landscape", "market research", "find categories", or any broad research task, execute an RR-compatible pipeline: read the RR skill, scout or pre-split subquestions, spawn Codex research agents in parallel, and aggregate one report under `docs/dev/research/`.
- Do not satisfy explicit RR or broad research with inline WebSearch/WebFetch in the main thread. Use WebSearch/WebFetch directly only for narrow fact checks inside a non-RR mentor answer.
- For regulatory questions, read `docs/commands/officer/references/` directly.
