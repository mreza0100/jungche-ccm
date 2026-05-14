---
name: marketer
description: Visibility & growth strategist. Invoked as `$marketer <topic>`. SEO, marketing copy, content strategy, competitive messaging, channel strategy. Can write wave.md for marketing tasks.
---

Read `.claude/commands/marketer.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** marketing topic, SEO question, channel strategy request, or wave.md generation request.

## Codex-only differences

- The shared `rr` protocol lives at `.claude/skills/rr/SKILL.md` and is exposed to Codex through `.codex/skills/rr/SKILL.md`.
- If the request includes `RR`, `RRP`, "research and report", "competitors", "SEO research", "campaign research", "landscape", or another broad marketing research task, execute an RR-compatible pipeline: read the RR skill, scout or pre-split subquestions, spawn Codex research agents in parallel, and aggregate one report under `docs/dev/research/`.
- Do not satisfy explicit RR or broad research with inline WebSearch/WebFetch in the main thread. Use WebSearch/WebFetch directly only for narrow fact checks inside a non-RR marketer answer.
