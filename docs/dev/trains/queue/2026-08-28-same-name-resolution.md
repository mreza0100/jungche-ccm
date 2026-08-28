# Same-name chats across time must not break injection

## The incident, observed live (2026-08-28 ~03:00)

COSMOS:ORCHESTRATOR (live codex seat, socket `cx-1787876438-392293-3669`) and
its spawned children could not message each other over chat MCP. Root cause,
verified by hand:

- THREE live codex panes carried the tmux window name `COSMOS:ORCHESTRATOR` —
  the current seat plus two abandoned earlier incarnations
  (`cx-1787874833-149693-11663`, `cx-1787876356-350521-10220`; both unattached,
  0 prompts on their current threads, idle 25–50 min, roster rows unnamed
  "Codex chat" / account 0 — rotation orphans per
  2026-08-28-codex-thread-rebind.md).
- `pfm chat resolve COSMOS:ORCHESTRATOR` (roster-backed) resolved UNIQUELY,
  exit 0, to the live seat — the roster's naming precedence gave exactly one
  row that name.
- Injection refused: `inject/engine.go` Resolve tries
  `resolve.Session → resolve.Label → resolve.CxWindow` (engine.go:359-385).
  For a codex chat, Session misses (sessions are socket names), Label misses
  (🔖 statusline scrape is a Claude mechanism), and `resolveCxWindow`
  (resolve.go:384) matches raw panes by window name — three matches → Code 2
  "ambiguous codex thread name … matches panes". Correct refusal, wrong layer.

The user's law: **opening chats with the same name across time is normal use
and must never break messaging.** Duplicate window names among live panes
happened naturally in one evening.

## The laws this violates

- "The dry run IS the apply's preview" (pfm/CLAUDE.md): `chat_resolve` says
  yes, injection says ambiguous — the preview and the delivery disagree
  because they resolve through DIFFERENT implementations.
- K3 one-implementation: naming judgment lives in `naming/` + roster
  composition, yet the inject resolver chain re-derives identity from a raw
  pane scan that has never heard of the roster.
- "Prefer refusing to guessing" holds — but refusal is only honest when the
  fleet's own knowledge cannot disambiguate. Here it could.

## Fix shape (to refine before building)

- Resolution by NAME consults the roster first: name → composed rows (the
  single naming implementation) → the row's socket + pane. A unique NAMED row
  wins even when stale panes share the window name. The pane-scan resolvers
  remain as fallback for names the roster does not know (fresh spawn before
  index catch-up), and the guarded delivery sequence still verifies the pane
  before typing.
- Refuse only on ROSTER-level ambiguity (two live rows genuinely carrying the
  same name): list the candidates with thread ids/sockets and tell the sender
  to address by thread id. That refusal text names the disambiguator.
- Keep `chat_resolve` and inject on the SAME resolution path end to end —
  one implementation, so the preview can never disagree with the delivery
  again.
- Regression (watched failing first): jail with two codex panes sharing a
  window name where the roster names exactly one → inject resolves to the
  named row's pane; roster-ambiguous twin case → Code 2 with both candidates
  listed. Cover the MCP surface too (`mcpserv/server.go:403` gate).

## Addendum (03:00, same night — the fallback case observed live)

A respawned seat (window `COSMOS:EXISTENTIALIST`, socket
`cx-1787878284-638987-45870`) ran ~3 minutes WITHOUT writing a rollout: no
thread to index → no roster row → a live chat the roster cannot see. The
cosmos rendered its inbound message honestly as `unresolved <socket>` under
the "unknown" star. This is the "fresh spawn before index catch-up" window
the roster-first design must keep the pane-scan fallback for — and evidence
that a pre-rollout seat is reachable only by socket, never by name, until
its first turn lands.

## Status

Complete on branch `engine-fixes`. CLI and MCP injection consult one composed
roster matcher before the raw pane fallbacks, and their preview paths use the
same gate. A unique live named row wins over stale duplicate window names;
genuine live roster ambiguity returns Code 2 with thread ids and sockets; a
roster miss still reaches a unique pre-rollout Codex window. Explicit MCP
`cxwin` resolution remains Codex-scoped. The CLI jail and MCP gate regressions
were watched failing before the fix; the full fenced pfm suite passes.
