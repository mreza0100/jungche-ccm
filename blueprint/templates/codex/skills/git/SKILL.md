---
name: git
description: Gitter gateway — routes git requests to gitter protocol. Invoked as `$git push [message]`, `$git pull`, or `$git <freeform request>`. Executes gitter.md protocol inline via bash.
---

Read `.claude/commands/git.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** subcommand (`push [message]`, `pull`) or freeform git request.

## Codex-only differences

- The separate gitter agent is not available. Execute git operations directly inline via bash, following `.claude/agents/gitter.md` protocol for commit messages, merge format, and lock semantics.
