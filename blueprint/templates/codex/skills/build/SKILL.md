---
name: build
description: "Cross-project build pipeline. Invoked as $build <feature> or $build with Pipeline/Task manifest/Wave args. Runs the full pipeline end to end including all git work."
---

Read `.claude/commands/build.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** feature description string, or structured `Pipeline: {name}. Task manifest: {path}. [Wave: {wave}]` when called from wave.

## Codex-only differences

- SETUP: run `bash .claude/scripts/worktree.sh create $PIPELINE` instead of using the gitter agent.
- MERGE / DOCS-COMMIT / push: git work is YOURS — execute gitter.md Phase 2-3 inline via bash. Read `.claude/agents/gitter.md` as your git protocol manual.
- ARCHIVE: spawn `Agent(mono-documenter, "Pipeline: $PIPELINE. Wave: $WAVE. ARCHIVE phase.")`.
- No `Skill(...)` calls — spawn child agents via Codex Teams `Agent(role, "...")` for each pipeline slot.
