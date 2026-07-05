---
name: p:tokens
description: "Attributes Claude Code runtime token spend per sub-agent and per workflow run, parsed from local JSONL transcripts, and ranks the heaviest burners with estimated USD cost. Answers 'which agent burned the most tokens', 'what did this workflow/wave/pipeline cost', 'token breakdown', 'per-operation tokens', and any retrospective spend analysis. Triggered by 'token ledger', 'token attribution', 'heaviest token burner', 'which agent burned the most tokens', 'what did the run/workflow/wave cost', 'token breakdown', 'per-agent tokens', 'per-operation tokens'. Complements /pcm:context-meter: that audits STATIC context size, this covers RUNTIME spend attribution — route static-budget questions to context-meter, after-the-fact spend questions here."
---

# Token Ledger

Per-agent and per-workflow-run token attribution from Claude Code's local JSONL transcripts — the runtime spend view native OpenTelemetry can't give (OTel redacts custom agent names to `"custom"`).

## When to load

- "Which agent / operation burned the most tokens?" — retrospective spend ranking.
- "What did this workflow / wave / pipeline / feature cost?"
- Any token breakdown, per-operation token, or after-the-fact cost-attribution question.
- NOT for static context budget (how big are CLAUDE.md/agents/skills) — that is `/pcm:context-meter`.

## How to invoke

Run from the monorepo root (project slug derives from cwd):

```bash
node .claude/commands/p/tokens/token-ledger.mjs [flags]
```

| Flag                    | Purpose                                                                                                                           |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| (none)                  | Most recent session for this project.                                                                                             |
| `--all`                 | Every session for this project.                                                                                                   |
| `--by-workflow`         | Group by `wf_*` workflow run — one row per run + `(non-workflow agents)` summary + TOTAL.                                         |
| `--filter <substr>`     | Restrict the per-agent table + totals to rows whose label or model id contains `<substr>` (case-insensitive); prints match count. |
| `--session <id\|path>`  | One conversation by id or path.                                                                                                   |
| `--detail <id\|substr>` | One agent's individual API calls in order.                                                                                        |
| `--project <slug>`      | Project slug override.                                                                                                            |
| `--root <dir>`          | Extra transcript root (repeatable).                                                                                               |
| `--json`                | Machine-readable output.                                                                                                          |

### The three canonical answers

- **Heaviest burner** → default or `--all` (the per-agent table is sorted by est cost desc; top row is the answer).
- **Per-workflow-run cost** → `--all --by-workflow`.
- **Per-`/wave:builder`-pipeline or per-`/wave:live`-feature cost** → `--all --filter <feature-label>`.

## `--by-workflow` honesty caveat

`--by-workflow` groups agent files under each `wf_*` directory and captures Workflow-engine runs (e.g. `/rr`) **exactly**. A plain `/wave:live` is **not** a `wf_*` run: `/wave:live` runs each `/wave:builder` in the main session, and `/wave:builder` spawns its plan/arch/dev/QA agents as session-level sub-agents — they land in the `(non-workflow agents)` row, not a per-run row. To total a `/wave:builder` pipeline or a `/wave:live` feature, use `--filter <label>`, never `--by-workflow`.

## Token-definition calibration

The footer prints four definitions of "tokens" — read whichever the question needs:

- **output-only** — generated tokens.
- **in+out** — input + output, no cache.
- **fresh (in+out+cache-write)** — what the harness's `subagent_tokens` reports.
- **grand total (+cache-read)** — adds cache-read, which dominates real spend.

The harness's headline `subagent_tokens` is the **fresh** number — it EXCLUDES cache-read, so it is not the grand total. Cache-read is usually the largest component of actual cost.

## Constraints

- READ-ONLY by design — the tool only reads transcripts; it never writes or sends anything.
- `--detail` content hints can contain sensitive prompt text — treat `--detail` output as sensitive; do not pipe or retain it.
- Costs are ESTIMATES from the EDITABLE `PRICING` table at the top of `token-ledger.mjs`. Trust the relative ranking, verify absolute dollars against real Anthropic billing, and update the rates when Anthropic prices change.

<example>
user: what's my heaviest token burner this project?
→ node .claude/commands/p/tokens/token-ledger.mjs --all   (top row of the cost-sorted table)
</example>

<example>
user: what did the RR run cost?
→ node .claude/commands/p/tokens/token-ledger.mjs --all --by-workflow   (find the wf_* row)
</example>

<example>
user: total the my-feature wave
→ node .claude/commands/p/tokens/token-ledger.mjs --all --filter my-feature
</example>
