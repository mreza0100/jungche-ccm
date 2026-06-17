---
name: quality:prompt
description: Use BEFORE editing any prompt file — CLAUDE.md, agents, commands, skills, or /km knowledge files. Enforces Anthropic's prompt-quality rules (cut test, line thresholds, positive framing, one canonical term, frontmatter discipline). Mandatory load for /pcm and /km.
---

Read `.claude/commands/quality/prompt.md` in full — it is the source-of-truth protocol. Follow it verbatim.

## Codex-only execution mapping

- Load this BEFORE editing any prompt file (CLAUDE.md, AGENTS.md, agents, commands, skills, knowledge files).
- This skill is advisory at write-time — it changes how you write, not what you commit. No side effects.
