---
name: council
description: Roundtable debate — multiple perspectives debate a topic in 3 rounds. Invoked as `$council <topic>`. Read-only.
---

Read `.claude/commands/council.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** debate topic, or `refinement <topic>` to produce a wave.md task file.

## Codex-only differences

- Spawn each council member as a Codex Teams child agent.
- For regulatory content, read `docs/commands/officer/references/` directly. The `rr` skill is Claude-side — use WebSearch/WebFetch for research.
