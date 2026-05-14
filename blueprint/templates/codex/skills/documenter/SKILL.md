---
name: documenter
description: "Documentation source of truth. Invoked as $documenter [subcommand]. Subcommands — audit. Handles ARCHIVE and JC-UPDATE phases. Owns docs/agents/ and child project docs."
---

Read `.claude/commands/documenter.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** subcommand (`audit`) or pipeline context for ARCHIVE/JC-UPDATE.

## Codex-only differences

- DOCS-COMMIT: execute gitter.md Phase 3 inline via bash instead of spawning a separate gitter agent.
