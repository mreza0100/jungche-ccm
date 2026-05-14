---
name: council
description: Roundtable debate — multiple perspectives debate a topic in 3 rounds. Invoked as `$council <topic>`. Read-only.
---

Read `.claude/commands/council.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** debate topic, or `refinement <topic>` to produce a wave.md task file.

## Codex-only differences

- Spawn each council member as a Codex Teams child agent.
- For regulatory content, read `docs/commands/officer/references/` directly.
- The shared `rr` protocol lives at `.claude/skills/rr/SKILL.md` and is exposed to Codex through `.codex/skills/rr/SKILL.md`.
- If the council topic includes `RR`, `RRP`, "research and report", "competitors", "landscape", or another broad research dependency, run the RR-compatible research pipeline first, then feed the aggregate findings into the council. Do not let council-member debate substitute for RR evidence gathering.
- Use WebSearch/WebFetch directly only for narrow fact checks inside a non-RR council run.
