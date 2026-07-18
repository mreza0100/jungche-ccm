---
name: p:tokens
description: "Attributes Claude Code runtime token spend per sub-agent and per workflow run, parsed from local JSONL transcripts, and ranks the heaviest burners with estimated USD cost. Answers 'which agent burned the most tokens', 'what did this workflow/wave/pipeline cost', 'token breakdown', 'per-operation tokens', and any retrospective spend analysis. Flags: (none)/--all (session scope), --by-workflow (per-run cost), --filter <substr> (isolate a label), --session <id>, --detail <id> (per-call), --project <slug>/--root <dir> (transcript roots), --json. Triggered by 'token ledger', 'token attribution', 'heaviest token burner', 'which agent burned the most tokens', 'what did the run/workflow/wave cost', 'token breakdown', 'per-agent tokens', 'per-operation tokens'. Complements /pcm:context-meter: that audits STATIC context size, this covers RUNTIME spend attribution — route static-budget questions to context-meter, after-the-fact spend questions here."
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

### The canonical answers

- **Heaviest burner** → default or `--all` (the per-agent table is sorted by est cost desc; top row is the answer).
- **Per-workflow-run cost** (a `/wave:orchestrator`, a standalone `/wave:builder`, an `/rr`) → `--all --by-workflow`, the `wf_*` row matching the run's `runId`.
- **A wave's end-to-end cost incl. review + remediation** → `--by-workflow` from the chat that ran it (default scope): the `TOTAL` row is the whole chat — the wave's `wf_*` row is the pipelines, `wave-walker` gets its own `wf_*` row (it runs as a Workflow too), and `(non-workflow agents)` is `/jc` + main-loop.
- **One pipeline within a wave** → `--all --filter <pipeline-label>` (nested wave-build children fold under the parent wave's row; isolate one by its label).

## `--by-workflow` scope note

Full mechanics: `p/tokens/README.md` § `--by-workflow` honesty caveat. `/wave:orchestrator`'s walker pass (`wave-walker`) also runs as its own `wf_*` Workflow row, same as the wave itself. Distinct here — a dual-chat wave spans TWO chats: run default scope in the orchestrator chat and `--session {builder-session}` for the builder, and sum. Slice one label with `--filter <label>`.

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

### Example 1

user: what's my heaviest token burner this project?
→ node .claude/commands/p/tokens/token-ledger.mjs --all (top row of the cost-sorted table)

### Example 2

user: what did the RR run cost?
→ node .claude/commands/p/tokens/token-ledger.mjs --all --by-workflow (find the wf_* row)

### Example 3

user: total the my-feature wave
→ node .claude/commands/p/tokens/token-ledger.mjs --all --filter my-feature
