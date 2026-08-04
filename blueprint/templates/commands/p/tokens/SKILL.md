---
name: p:tokens
description: "Attributes Claude Code runtime token spend per sub-agent and per workflow run, parsed from local JSONL transcripts, and ranks the heaviest burners with estimated USD cost. Answers 'which agent burned the most tokens', 'what did this workflow/wave/pipeline cost', 'token breakdown', 'per-operation tokens', and any retrospective spend analysis. Flags: (none)/--all (session scope), --by-workflow (per-run cost), --filter <substr> (isolate a label), --session <id>, --detail <id> (per-call), --project <slug>/--root <dir> (transcript roots), --json. Triggered by 'token ledger', 'token attribution', 'heaviest token burner', 'which agent burned the most tokens', 'what did the run/workflow/wave cost', 'token breakdown', 'per-agent tokens', 'per-operation tokens'. Complements /pcm:context-meter: that audits STATIC context size, this covers RUNTIME spend attribution — route static-budget questions to context-meter, after-the-fact spend questions here."
---

# Token Ledger

Run from the monorepo root (the project slug derives from cwd); read-only over transcripts, no network:

```bash
node .claude/commands/p/tokens/token-ledger.mjs [flags]
```

`--help` prints the full flag list; `p/tokens/README.md` carries the mechanics and schema notes.

## Which invocation answers which question

- Heaviest burner: `--all` — the per-agent table sorts by est cost descending, so the top row is the answer.
- One Workflow run's cost: `--all --by-workflow`, the run's `wf_*` row.
- One wave, pipeline, or feature: `--all --filter <label>` — sums every agent row whose label carries it.
- A whole chat's spend: `--by-workflow` in that chat (default scope) — `TOTAL` is the chat, `wf_*` rows are its Workflow runs, `(non-workflow agents)` is everything else.
- One agent's individual calls: `--detail <id|label-substr>`.

## What gets a `wf_*` row

Only a Workflow-engine run — a script under `.claude/workflows/` or a skill-embedded engine (`/rr`). An orchestrated wave is not one: `/wave:orchestrator` and `/wave:builder` run in their chats' main sessions and spawn session-level sub-agents, which land in `(non-workflow agents)`; total a wave with `--filter <wave-label>` instead. The wave's walker pass runs the `wave-walker` script, so its cost sits in a separate `wf_*` row, outside that filter. A dual-chat wave spans two chats — sum default scope in the orchestrator chat with `--session {builder-session}` for the builder.

## Reading the output

- Footer totals: output-only, in+out, fresh (in+out+cache-write), grand total (+cache-read). The harness's `subagent_tokens` is the fresh number — it excludes cache-read, usually the largest component of real spend.
- Costs are estimates from the editable `PRICING` table atop `token-ledger.mjs`: trust the ranking, verify absolute dollars against Anthropic billing, update the rates when prices change.
- `--detail` content hints can carry sensitive prompt text — read it, never pipe or retain it.
