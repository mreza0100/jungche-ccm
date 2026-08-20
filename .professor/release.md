# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Important: wave pipeline (`commands/wave/refine.md` + `agents/scheduler.md`) — precondition anchors: a production behavior a task relies on (an "existing" code path, a CLI verb or flag, an exit code) is code-verified and anchor-cited at refine time, with a missing surface becoming a scheduled dependency task; the scheduler gains an **Anchors** step that grep/read-verifies every prose-relied surface before scheduling — diff-scoped staleness catches a spec broken by later commits, the new step catches one born wrong (RE-REFINE instead of a silent pass).
- Important: `blueprint/CLAUDE.md` — new `## Persona` section (respond as the install's active output style; every reply closes with a one-line **Verdict**) so Codex runtimes, which read only CLAUDE.md/AGENTS.md, carry the Verdict rule; Docs map gains a facts-registry example (`docs/facts/_index.md` — user-ruled invariants, read before touching data lifecycle or {SENSITIVE_DATA}, contradiction = escalate) and the /documenter bullet gains `docs/facts/` (main loop only, on the user's explicit ruling).
- Minor: `blueprint/themes/` — new `tokyo-night.json` palette (47 UI overrides) + a README naming the palette JSON shape; pfm embeds palettes at build time.
- Important: `commands/wave/sentinel.md` → `commands/wave/ccc.md` — the one-shot sentinel auditor becomes the Control & Command Center (CCC): the standing command seat over a running train. Full audit from ground truth on arrival (the sentinel checklist survives as CCC's instrument), then it holds command until the train closes — verifies every DONE/green claim against the tree, rules scope-allocation escalations as ledger lines, routes user-only decisions up with why, dispatches through the orchestrator only; `/wave:orchestrator` now names CCC's ledger-logged rulings authoritative.
#### → For: adopters re-running refresh — `/wave:sentinel` is gone; call `/wave:ccc` (same triggers, plus "take command of the train").
- Important: the founder-name placeholder is retired across the framework — templates address "the user", never a name and never a name placeholder; the `NEEDS-FOUNDER-SPEC` status becomes `NEEDS-USER-SPEC`; officer.md identifies the user by ROLE in legal documents; the placeholder registry drops the token (startup-domain "founder" vocabulary in mentor/marketer is content, not address, and stays).
#### → For: adopters re-running SETUP see no name question; installs carrying the old founder-name token re-resolve it to "the user" on next refresh.
