---
name: pcm:context-meter
description: "Audits Claude Code context consumption across CLAUDE.md, agents, commands, skills, and MCP servers, then ranks the heaviest offenders against {PROJECT_NAME}'s size limits and reports prioritized token savings (`--verbose` for per-file detail). Triggered by 'context budget', 'token budget', 'context-budget', 'context meter', 'context-meter', 'audit context', 'what's eating my context', or after adding/growing an agent, command, or skill."
---

# Context Budget

Measure what every loaded pipeline component costs in context, find the bloat, and rank fixes by tokens reclaimed.

## When to load

- A session feels sluggish or output quality is degrading
- You just added or grew an agent, command, or skill and want to catch creep
- You're deciding whether there's room to add more before trimming
- The founder runs `/pcm:context-meter` (or `--verbose` for per-file detail)

## Measure

Token estimate: `words × 1.3` for prose, `chars / 4` for code/tables. Report both bytes and the estimate; bytes are exact, tokens are the budget that matters. Treat `/context` as the ground-truth meter — its live breakdown is authoritative over any wc-derived estimate; reconcile estimates against it. The wc-byte sweeps below may run on a cheap child (`Explore`/haiku); the judgment over the numbers stays with the auditor.

Scan each surface and tally per file:

| Surface                | Path                                                | Limit     | Flag when                                                                                                                     |
| ----------------------- | ---------------------------------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------ |
| Root persona + routing | `CLAUDE.md`                                         | 200 lines | > 200 lines                                                                                                                   |
| Child conventions      | `{project}/CLAUDE.md`                               | —         | restates root rules (should hold only the delta)                                                                              |
| Agents                 | `.claude/agents/*.md` + `{project}/.claude/agents/*.md` | 15 KB | > 15 KB, or `description` > 30 words (loads into every spawn)                                                                 |
| Commands               | `.claude/commands/*.md`                             | 35 KB     | > 35 KB                                                                                                                       |
| Skills                 | `.claude/skills/*/SKILL.md`                         | 500 lines | > 500 lines, or combined `description` + when-to-use > 1,536 chars                                                            |
| MCP                    | `.mcp.json`                                         | —         | a server wrapping a CLI already on PATH (`gh`, `git`) — schemas are deferred, so tool count costs little until fetched       |

```bash
# bytes + lines per agent/command (sorted heaviest first)
wc -lc .claude/agents/*.md .claude/commands/*.md | sort -rn | head -20
```

## Classify

Sort every component into one bucket:

- **Always loaded** — root CLAUDE.md, agent `description` frontmatter (present in every Task spawn even when the agent is never invoked), the active output style appended to the main-loop system prompt, and any skill content kept after invocation. This is the recurring tax; weigh it hardest. The always-loaded floor sits at ≈9–10k tokens.
- **On demand** — command bodies, skill bodies, agent bodies: paid only when invoked. Bloat here is cheaper but still real.
- **MCP schemas** — deferred: tool schemas load on demand via `ToolSearch` and stay unbilled until fetched, so a many-tool server costs almost nothing while idle. What's always present is the deferred-tool name list (a few tokens per tool) plus any fetched schema for the session. Rank a server by how often its schemas actually get pulled, not by raw tool count.
- **Output style** — the active style in `.claude/output-styles/` (set via settings.json `outputStyle`) appends to the MAIN-LOOP system prompt only; subagents never receive it. The registry's overlay personas (`dr-house`, `jc`) load only on their command's invocation, not by default.

## Report

```
Context Budget — {PROJECT_NAME}
═══════════════════════════════════════
Always-loaded overhead: ~X,XXX tokens   (CLAUDE.md chain + agent descriptions + output style + MCP name list; floor ≈9–10k)

Surface        Count   Bytes     ~Tokens
Root CLAUDE.md   1     XX,XXX     X,XXX
Child CLAUDE.md  N     XX,XXX     X,XXX
Agents          NN     XXX,XXX    XX,XXX
Commands        NN     XXX,XXX    XX,XXX
Skills          NN     XXX,XXX    XX,XXX
MCP (tools)     N       —         XX,XXX

Over limit (N):
  - <file>  <size>  (limit <limit>)   → <suggested trim>

Top savings:
  1. <action> → ~X,XXX tokens
  2. <action> → ~X,XXX tokens
  3. <action> → ~X,XXX tokens
```

`--verbose`: add per-file token counts, the heaviest files line-by-line, and the MCP tool list with per-tool schema estimates.

## Rules

- Report only — never edit. Trimming a prompt file routes through `/pcm` (it loads `/quality:prompt` and verifies consistency). Surface the savings; let the founder approve the cut.
- Rank by tokens reclaimed, not file count — and since MCP schemas are deferred, target the always-loaded floor (CLAUDE.md chain, agent descriptions, output style, kept skill content) before chasing tool counts.
- Verify counts against the filesystem (`ls`, `wc`) and reconcile against `/context`; never trust a CLAUDE.md inventory claim.

### Example 1

Founder: /pcm:context-meter
Skill: Scans → NN agents (~14k tok on-demand), NN commands (~38k on-demand), NN skills (~9k), 2 MCP servers (schemas deferred — name list only until fetched), CLAUDE.md 198 lines (~6k). Always-loaded floor ≈9–10k, confirmed against /context.
Over limit: wave/builder.md 41 KB (limit 35 KB). Bloated description: gitter (38 words).
Top saving: split wave/builder.md's Pipeline Reference into a referenced file → ~3k tokens, back under limit.

### Example 2

Founder: do I have room to add another MCP server's tools?
Skill: Always-loaded floor ≈9–10k. Its schemas are deferred — they cost only a few tokens of name list until `ToolSearch` fetches them, so adding the server barely moves the floor. Fine — but if it wraps a CLI already on PATH and you find the schemas getting pulled every turn, that server is the first cut.
