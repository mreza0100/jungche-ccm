---
name: audit
description: "Codebase analysis & quality — code hygiene, security. Invoked as $audit [scope]. Read-only scan with mandatory skill invocation."
---

Read `.claude/commands/audit.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** optional scope (defaults to `all`).

This command is READ-ONLY — no code changes, no commits.

**MANDATORY:** Invoke the corresponding skill(s) for your scope BEFORE scanning: `/p:audit:code-hygiene`, `/p:audit:security`.
