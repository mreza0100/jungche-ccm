---
name: pcm:context-meter
description: "Audits Claude Code context consumption across CLAUDE.md, agents, commands, skills, and MCP servers, then ranks the heaviest offenders against Professor's size limits and reports prioritized token savings (`--verbose` for per-file detail). Triggered by 'context budget', 'token budget', 'context-budget', 'context meter', 'context-meter', 'audit context', 'what's eating my context', or after adding/growing an agent, command, or skill."
---

# Context Budget

Measure what every loaded pipeline component costs in context, find the bloat, and rank fixes by tokens reclaimed.

## Measure

`/context` is the ground-truth meter — its live breakdown outranks any wc-derived estimate, and the always-loaded floor is read off it every run, never carried as a number in this file or recalled from a past audit. Estimate off-meter surfaces at `words × 1.3` for prose, `chars / 4` for code/tables; report bytes (exact) beside the token estimate (the budget that matters). The byte sweeps may run on a cheap child (`Explore`/haiku); the judgment over the numbers stays with the auditor.

Enumerate every surface from disk — rosters and counts are derived per run:

```bash
find .claude/agents {project}/.claude/agents .claude/commands .claude/skills .claude/output-styles -name '*.md' -not -path '*/worktrees/*' -exec wc -c {} + | sort -rn | head -25
wc -l CLAUDE.md {project}/CLAUDE.md $(find .claude -name SKILL.md -not -path '*/worktrees/*') | sort -rn
```

Flag a file when:

- `CLAUDE.md`, root or child: over 200 lines; a child that restates root rules rather than holding only its delta
- `.claude/agents/*.md` and `{project}/.claude/agents/*.md`: over 15 KB, or `description` over 30 words — every agent description loads into every spawn
- `.claude/commands/**/*.md`, nested command dirs included (`pcm/`, `wave/`, `quality/`, `audit/`, `p/`, `h/`): over 35 KB
- `SKILL.md`, under `.claude/skills/*/` and embedded in command dirs: over 500 lines, or `description` plus when-to-use over 1,536 chars combined
- `.claude/output-styles/*.md`: the active style bills against the always-loaded floor, overlays only on their command's invocation
- `.mcp.json`: a server wrapping a CLI already on PATH (`gh`) — schemas are deferred, so tool count costs little until fetched

The size limits are `/pcm`'s (§ Hard thresholds, § Critical invariants); this command only measures against them.

## Classify

Sort every component into one bucket:

- Always loaded: root CLAUDE.md, agent `description` frontmatter (present in every spawn even when that agent is never invoked), the active output style (main-loop system prompt only — subagents never receive it), and skill content kept after invocation. The recurring tax; weigh it hardest.
- On demand: command bodies, skill bodies, agent bodies — paid only when invoked. Cheaper bloat, still real.
- MCP schemas: deferred. Tool schemas load through `ToolSearch` and stay unbilled until fetched, so a many-tool server costs almost nothing while idle; what is always present is the deferred-tool name list (a few tokens per tool) plus whatever schemas this session pulled. Rank a server by how often its schemas actually get pulled, not by raw tool count.

## Report

In order: the always-loaded total read from `/context`, with its composition named (CLAUDE.md chain, agent descriptions, output style, MCP name list); one row per surface — root CLAUDE.md, child CLAUDE.md, agents, commands, skills, MCP — carrying count, bytes, and estimated tokens; every over-limit file with its size, its limit, and the suggested trim; then the savings ranked by tokens reclaimed, top three first.

`--verbose` adds per-file token counts, the heaviest files line by line, and the MCP tool list with per-tool schema estimates.

## Rules

- Report only — never edit. Trimming a prompt file routes through `/pcm`, which loads `/quality:prompt` and verifies consistency. Surface the savings; the founder approves the cut.
- Rank by tokens reclaimed, not file count — and since MCP schemas are deferred, target the always-loaded floor before chasing tool counts.
- Derive every count from the filesystem and reconcile it against `/context`; an inventory claim written in any prompt file, this one included, is not evidence.
