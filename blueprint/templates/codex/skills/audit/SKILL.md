---
name: audit
description: "Codebase analysis & quality — code hygiene, security. Invoked as $audit [scope]. Read-only scan with mandatory reference file loading."
---

Read `.claude/commands/audit.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** optional scope (defaults to `all`).

This command is READ-ONLY — no code changes, no commits.

**MANDATORY:** Load the reference file(s) for your scope from `docs/commands/audit/references/` BEFORE scanning.
