# Codex thread rotation orphans the seat's identity binding

## The bug, observed live (2026-08-28)

A Codex seat (COSMOS:ORCHESTRATOR, socket `cx-1787874833-149693-11663`) rotated its
thread id — `01a044f3-9bc4-7070-84cb-95936284c953` → `01a045a4-fa5c-7822-bd63-db056e2a4484` —
after a session reset. The fleet's thread→socket binding did not follow:

- `pfm ls --tsv` showed a `live-codex` row still bound to the OLD thread id
  `01a044f3…` on that socket — even carrying the old project (`intuita`) while the
  pane's actual chat sits in `~/.professor`.
- The NEW thread `01a045a4…` surfaced as a socketless `resume-codex` row.

Consequences, both witnessed:

1. `mcpserv.callerForRequest` (`pfm/internal/mcpserv/backend.go:129`) matches
   `row.ID == threadId && Kind == live-codex && Socket != ""` — the rotated seat
   matches nothing → every identity-bearing MCP tool refuses:
   `MCP _meta.threadId "…" has no live Codex tmux seat` (code 4).
2. The refused agent worked around it with CLI `--allow-unsigned`, filling the
   comms ledger with sender-less rows (ledger ids 307–326: five spawns + fifteen
   injects, all with empty sender_label/sender_session) — which the cosmos then
   cannot draw lines or lineage for. The renderer now warns
   ("N events carry no sender identity — their lines cannot be drawn"), but the
   root cause is the stale binding upstream.

## The law this violates

pfm/CLAUDE.md: "A codex seat driven by `codex app-server` … identity comes from
`CODEX_THREAD_ID` through the fleet's thread→socket binding." A binding that
freezes at spawn time makes the last rung of the identity ladder rot on the
first thread rotation. And: "prefer refusing to guessing" — the refusal is
CORRECT; the binding staleness is the defect. No lookup-side heuristic may
paper over it.

## Fix shape (to refine before building)

Rebind at the source, continuously: the binding must derive from what the live
process is DOING, not what it was born as.

- Anchors: `gather/` codex process scan (which rollout file the live codex
  process currently holds — /proc fd table or process args), the thread→socket
  binding store (find its writer; check `store/` and spawn-time recording in
  `spawn/`), `compose` row classification that pairs live sockets with thread
  ids, `index/` delta parse of `~/.codex/sessions` rollouts.
- A rotated thread's new rollout in the same seat must fold the binding forward:
  same socket, new thread id, and the old thread id's row must not linger as a
  phantom live row (its project/cwd was observably stale too).
- Beware `store/lineage.go` subagent folding: a compact/reset continuation is
  NOT a subagent child; verify how its `session_meta` (`thread_source`,
  `parent_thread_id`) presents before keying on it.
- Regression tests: jail fixture with a live codex seat whose process rolls to a
  new rollout file — `pfm ls` must show ONE live-codex row carrying the NEW
  thread id and the correct cwd; `callerForRequest` with the new thread id must
  resolve. Watched failing first, per law.
- Follow-through: with the binding fixed, the `--allow-unsigned` escape loses
  its everyday excuse — consider whether spawn recording should also carry
  sender identity derivation for `pfm chat new` invoked BY a chat (the luna
  spawns recorded sender-less through the CLI path).

## Status

Complete on branch `engine-fixes`. The live process's open rollout now owns
Codex identity precedence, the pane binding advances to a rotated user thread,
and composition emits one live row with the new id and current cwd. The jailed
rotation regression was watched failing before the fix; the full fenced pfm
suite passes.
