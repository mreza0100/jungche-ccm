---
name: 360
description: Exhaustive multi-angle analysis. Invoked by `360`, `three-sixty`, `do a 360`, "what could go wrong", or explicit blind-spot sweep requests.
---

Read `.claude/skills/360/SKILL.md` in full — it is the source-of-truth protocol. Follow it verbatim.

## Codex-only execution mapping

- Embedded 360 runs must use a separate clean-context Codex agent.
- Spawn the 360 agent with only the subject, domain (`test` or `inquiry`), and the instruction to read `.claude/skills/360/SKILL.md`.
- Do not include the caller's prior conclusions in the 360 prompt. The whole point is to catch the assumptions already contaminating the room.
- The 360 agent returns angle lists in chat. The caller decides which angles require investigation or changes.
