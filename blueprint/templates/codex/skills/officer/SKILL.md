---
name: officer
description: Compliance. Invoked as `$officer <question or audit request>`. Answers regulatory questions, audits codebase/architecture for privacy violations. Advisory only.
---

Read `.claude/commands/officer.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** compliance question, audit request, or architecture review.

## Codex-only differences

- Read `docs/commands/officer/references/` directly for the full regulatory base. For latest developments not yet in the file, use WebSearch/WebFetch for narrow fact checks.
- If the request explicitly includes `RR`, `RRP`, "research and report", "regulatory landscape", or another broad research task, execute the shared RR-compatible pipeline through `.codex/skills/rr/SKILL.md` instead of doing inline web research. For compliance topics, prefer primary regulatory sources and preserve uncertainty.
- ADVISORY ONLY — never issue mandates, never design schemas, never add consent flags unless the founder has approved.
