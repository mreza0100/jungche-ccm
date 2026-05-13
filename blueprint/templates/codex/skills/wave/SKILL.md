---
name: wave
description: Wave task runner. Reads a wave.md file and runs each pipeline sequentially. Invoked as `$wave <path/to/wave.md>`. Argument is the path to the wave manifest file.
---

Read `.claude/commands/wave.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** the wave manifest file path (e.g. `$wave docs/dev/waves/my-wave/wave.md` or `$wave ./wave.md`).

## Codex-only differences

- `Skill("build", ...)` → spawn a Codex Teams child agent: `Agent(build, "Pipeline: {name}. Task manifest: docs/dev/builds/{name}/0-task.md. [Wave: {wave-name}]")`
- `Skill("professor", ...)` → spawn `Agent(professor, "...")` for refinement, or refine inline if the wave file is already structured
- Git work (SETUP, MERGE, DOCS-COMMIT, push) is YOURS — execute gitter.md protocol inline via bash. Read `.claude/agents/gitter.md` as your git protocol manual.
- Wave-end archive: `mkdir -p docs/dev/waves/archive && mv docs/dev/waves/{name} docs/dev/waves/archive/{name}` then commit + push.
