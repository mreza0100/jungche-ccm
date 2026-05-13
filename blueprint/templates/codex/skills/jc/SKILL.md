---
name: jc
description: Live debug, diagnose & fix on main. The ONE command allowed to edit code directly on main. Invoked as `$jc <bug description or service>`. Targeted hotfixes only.
---

Read `.claude/commands/jc.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** bug description, service name, or diagnostic request.

## Codex-only differences

- When jc.md says "use gitter JC-COMMIT": execute gitter.md Phase 4 (JC-COMMIT) inline via bash. Read `.claude/agents/gitter.md` Phase 4 for the exact protocol.
- Edits on main directly — no worktree. Be surgical.

## ABSOLUTE PROHIBITION — DO NOT PUSH

JC-COMMIT is LOCAL ONLY. Phase 4 = commit + commit-docs. **Nothing else.** Never run `git push` during /jc. If the founder wants the commit pushed, they invoke `$git push` separately. Stop after local commit; report hash; wait.
