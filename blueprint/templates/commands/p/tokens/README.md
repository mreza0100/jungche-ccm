# token-ledger

Per-agent / per-operation token attribution for Claude Code sessions, parsed straight
from the sub-agent JSONL transcripts Claude Code writes locally. Zero dependencies
(node: builtins only), READ-ONLY over transcripts, no network. Node 20+.

This is the "WHICH agent / WHICH operation burned the tokens" view that Claude Code's
native OpenTelemetry **metrics cannot give** — `agent.name` is redacted to `"custom"`
for user-defined sub-agents, so the JSONL files are the only local source of per-agent
truth. (See the RR report that produced this tool.)

## Usage

Run from the monorepo root (the project slug is derived from the cwd):

```bash
# Most recent session for the current project (cwd-derived):
node .claude/commands/p/tokens/token-ledger.mjs

# Every session for this project (heaviest token burner = top row, sorted by cost):
node .claude/commands/p/tokens/token-ledger.mjs --all

# What did each workflow run cost? (one row per wf_* run, sorted by cost):
node .claude/commands/p/tokens/token-ledger.mjs --all --by-workflow

# Total one /wave:build pipeline or /wave feature (by label substring):
node .claude/commands/p/tokens/token-ledger.mjs --all --filter my-feature

# A specific conversation (by id or by path to its dir / main .jsonl):
node .claude/commands/p/tokens/token-ledger.mjs --session <session-id>

# Drill into one agent's individual API calls (by agentId OR label substring):
node .claude/commands/p/tokens/token-ledger.mjs --detail "BE developer"
node .claude/commands/p/tokens/token-ledger.mjs --session <id> --detail a26fc4c505ee2af1b

# Machine output:
node .claude/commands/p/tokens/token-ledger.mjs --json

# Extra root / project override:
node .claude/commands/p/tokens/token-ledger.mjs --root /some/other/.claude --project -Users-you-work-project
```

### Flags

| Flag                    | Purpose                                                                                                                                                                               |
| ----------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--all`                 | Every session for this project (default is the most recent with sub-agents).                                                                                                          |
| `--session <id\|path>`  | One conversation, by id or by path to its dir / main `.jsonl`.                                                                                                                        |
| `--project <slug>`      | Project slug override (default: slugified cwd).                                                                                                                                       |
| `--root <dir>`          | Extra transcript root (repeatable).                                                                                                                                                   |
| `--detail <id\|substr>` | List one agent's individual API calls in order.                                                                                                                                       |
| `--by-workflow`         | Group by workflow run (`wf_*`) — one row per run + a `(non-workflow agents)` summary row + TOTAL.                                                                                     |
| `--filter <substr>`     | Restrict the per-agent table + totals to rows whose label or model id contains `<substr>` (case-insensitive); prints the match count. Composes with `--all` / `--session` / `--json`. |
| `--json`                | Machine-readable output.                                                                                                                                                              |

### `--by-workflow` honesty caveat

`--by-workflow` groups every agent file under each distinct `wf_*` run directory.
It captures **Workflow-engine runs** (e.g. `/rr`, or the wave-pipelines engine when it
runs as a Workflow) **exactly**.

It does **NOT** total a plain `/wave`. A `/wave` runs each `/wave:build` in the **main
session**, and `/wave:build` spawns its plan/arch/dev/QA agents as **session-level**
sub-agents — not as `wf_*` workflow runs. Those land in the `(non-workflow agents)`
summary row. To total one `/wave:build` pipeline or one `/wave` feature, use
`--filter <feature-label>` (e.g. `--filter my-feature`), which sums every agent
row carrying that feature name.

Default scope is the most recent session **that has sub-agents** for the current
project. The project is identified by slugifying the cwd (every `/` → `-`), matching
Claude Code's own `projects/{slug}` naming.

## What it reads

Two transcript roots, auto-discovered:

1. `~/.claude/projects/{slug}/` — standard layout.
2. `~/.claude-sessions/s*/projects/{slug}/` — an optional multi-account session layout.

Where both roots are **hardlinks to the same inodes**, the tool de-duplicates sessions
by `(dev, inode)` of the main file — it never double-counts a session seen under both
roots.

Within a session:

- `{conversationId}.jsonl` — the **MAIN** conversation loop → its own row.
- `{conversationId}/subagents/agent-*.jsonl` — each sub-agent → one row.
- `{conversationId}/subagents/workflows/wf_*/agent-*.jsonl` — nested workflow
  sub-agents (a Workflow-engine run, e.g. `/rr`) → one row each. See the
  `--by-workflow` honesty caveat above: a plain `/wave` is NOT a `wf_*` run.

## Schema notes (verified against real files)

- **Usage** lives on every `assistant` line at `message.usage`:
  `input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`.
  Model is `message.model`.
- **Dedup is mandatory.** Streaming writes multiple `assistant` lines per API call,
  all sharing one `message.id` (verified: 43 raw lines → 18 distinct calls in one file).
  The tool keys on `(message.id, requestId)` and keeps the **last** occurrence — the
  final line carries complete cumulative usage. Summing raw lines overcounts ~2-3x.
- **Agent label** is resolved in priority order:
  1. `agent-{id}.meta.json` → `description` (the richest — e.g. `"BE developer"`,
     `"gitter SETUP"`, `"FE QA pre-merge"`; present for `/wave:build`/`/wave` sub-agents).
  2. `agent-{id}.meta.json` → `agentType` (e.g. `"workflow-subagent"`, `"general-purpose"`).
  3. `attributionAgent` on the assistant line.
  4. First-user-message prompt snippet (the agent's task brief).
  5. The raw `agentId`.

## Cost model

Per-MTok rates are **EDITABLE constants** at the top of `token-ledger.mjs` (`PRICING`).
Matched by substring on the lowercased model id (`opus`, `sonnet`, `haiku`, `fable`,
`mythos`). Cache-write = 1.25× input rate, cache-read = 0.1× input rate (standard
Anthropic prompt-caching multipliers). Unknown model → cost 0 + a one-time warning,
never a crash. **Update these rates when prices change** — they are best-effort defaults,
not authoritative billing.

## Token-definition calibration (read this to interpret the harness's numbers)

The Claude Code workflow harness reports a `subagent_tokens` figure. Validated against a
known run (`wf_2c1d0117-cad`: harness reported **31 agents / 1,268,238 subagent_tokens /
579 tool_uses**), this tool's 31-agent totals were:

| definition                                            | value         | vs 1,268,238 |
| ----------------------------------------------------- | ------------- | ------------ |
| output-only                                           | 131,933       | 10%          |
| input + output (no cache)                             | 207,996       | 16%          |
| **input + output + cache-write ("fresh"/billed-new)** | **1,382,232** | **109%**     |
| + cache-read (grand total)                            | 10,231,459    | 807%         |

So the harness's `subagent_tokens` maps to the **fresh / billed-new** definition —
`input + output + cache_creation`, i.e. everything **except** the 8.8M cache-read tokens.
It is NOT output-only, NOT input+output, and NOT the grand total. The ~9% gap (the
harness reads ~1.27M; this tool sums 1.38M) is a flush-timing artifact: the harness
fired its report before the last one or two streaming agents flushed their final usage
lines — one agent's fresh-token total (113,452) almost exactly equals the gap (113,994).

The table footer prints all four definitions on every run so you can read whichever the
context calls for.

## Caveats

- Read-only by design. `--detail` content hints are truncated at ~80 chars — but these
  transcripts can contain sensitive prompt content, so treat `--detail` output as
  sensitive and do not pipe it anywhere it would be retained.
- `attributionAgent`/`attributionSkill` are generic for workflow sub-agents
  (`"workflow-subagent"`/`"rr"`); the real per-worker identity for those lives only in
  the task prompt (first user line), which the label falls back to when meta has no
  `description`.
- Cost is an **estimate**. Verify against your actual Anthropic billing before trusting
  absolute dollar figures; the relative ranking is what's reliable.
- Malformed JSONL lines are skipped silently and counted (reported on stderr).
