---
name: professor-refine
description: Wave task refinement — critically evaluates a task list through R1-R3.5 protocol, produces wave.md.
---

Read `.claude/skills/professor-refine/SKILL.md` in full — it is the source-of-truth protocol. Follow it verbatim.

## Codex-only execution mapping

- R1.5 interactive discovery requires user Q&A — Codex surfaces questions and waits.
- R-POC validation spawns separate Codex agents per POC.
- PM consultation (R2.5, R3.5) invokes the PM command skill.
- The ONLY file this skill creates is `wave.md` at repo root.
