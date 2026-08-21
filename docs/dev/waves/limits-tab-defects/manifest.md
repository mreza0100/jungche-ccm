# Limits tab defects — phantom accounts, missing Fable, absent Codex

**Status:** QUEUED → train pfm-wave-2 (user-ordered scope addition, 2026-08-21)
**Origin:** user screenshot of the Stats → Limits subtab; CCC-verified anchors below (worktree @85be5bb).
**Territory:** `pfm/internal/config/**`, `pfm/internal/stats/**`, `pfm/internal/statusline/**` (read-only reference), `pfm/internal/ui/**`.

## What the user saw

```
🥉 account 3   limits unavailable          ← no third account exists
🥈 account 2   … 7d-nimbus-quill 0% (?)    ← cryptic key; no Fable row
warning: account 3 limits unavailable: usage endpoint returned 403 Forbidden
(no Codex rows anywhere)
```

## D1 — Phantom account discovery (hardcoded 1..3)

**Anchor:** `pfm/internal/config/config.go:194-204` — when no accounts are configured, discovery fabricates accounts `1..3` unconditionally: no dir-existence check, no credential validation. Host reality: `~/.cc/{1,2,3,4}` all exist; the user has TWO real accounts; `.cc/3` holds stale credentials (403 on the usage endpoint); `.cc/4` is silently invisible because the loop bound is 3.

**Law:** `pfm/CLAUDE.md` CONFIG-OWNERSHIP — a hardcoded account count is a defect. This one lives inside the config package but fabricates state instead of discovering it; the invariant's substance is "discovered, not assumed".

**Fix shape (builder investigates the right validity predicate):**
- An account exists iff discovery finds its marker (e.g. a readable `.cc/N/.credentials.json` — builder confirms the real login marker against what `pfm reload`/launch actually requires).
- Discovery walks the `.cc/` dirs it finds — no fixed upper bound; `.cc/4` is judged by the same predicate, not by a loop constant.
- A dir failing the predicate is a NAMED one-line skip at the visible surface (`skipped .cc/3: no valid credentials`), never a rendered account row, never a per-refresh 403 warning. Error ≠ absence: a real account whose fetch fails still renders — as its error; a non-account never renders as an account.

**Acceptance:** with `.cc/3` stale and `.cc/4` invalid, the Limits tab shows exactly accounts 1 and 2, one named skip line each for 3 and 4 (or a single summarized skip), zero 403 warnings; a regression test watched RED against the unfixed fabrication loop.

## D2 — Fable window missing; unknown keys render as cryptic codenames

**Anchor (known-good parser):** `pfm/internal/statusline/render.go:70-72,333-334` — parses `seven_day`, `seven_day_opus`, `seven_day_fable`, labels `7d-opus` / `7d-fable`. Test fixture `pfm/internal/statusline/statusline_test.go:30` pins `seven_day_fable`.
**Anchor (drifted consumer):** the Limits path (`pfm/internal/stats/limits.go` + its `pfm/internal/ui` renderer) shows `7d-nimbus-quill 0% (?)` and NO Fable row — a second window-key mapping that has diverged from the statusline's.

**Fix shape:**
- ONE canonical window-key mapping shared by statusline and stats (NO-duplication law: extract and import, never a near-copy). Builder reads the actual usage-API response to enumerate today's window keys.
- Fable's 7d window renders wherever the account's API response carries it.
- A window key the mapping does not know renders as a labeled unknown with its raw key AND is a named gap — never a bare codename row; a missing reset timestamp states why (`(?)` unexplained is absence-shaped).

**Acceptance:** account rows show `7d` + `7d-fable` (and `7d-opus` where returned); an unknown-key fixture renders as a labeled unknown; regression test watched RED against the drifted mapping.

## D3 — Codex limits render as absence

**Anchor:** `pfm/internal/stats/limits.go:246-250` — Codex cache read exists with error strings ready (`read Codex cache`, `decode Codex cache`), yet the pane shows zero Codex rows. The data exists on this box: Codex seat statuslines display `weekly 96% left`.

**Fix shape:** builder traces why the Codex section never reaches the render — unpopulated cache path, unwired account set, or a renderer that drops the section — and makes Codex accounts render limit rows; a failed cache read renders AS its error at the visible surface (the strings exist; find where they are swallowed).

**Acceptance:** the Limits subtab shows Codex usage rows for live Codex accounts; with the cache removed, the pane shows the read error, not an empty section; regression test watched RED.

## Cross-cutting

- Every gate through the fence: `dev.sh iso {verify|test} pfm`, logs opening with the fence proof line.
- If the fix changes the CONFIG-OWNERSHIP exemplar set, update the walker-invariants CONFIG-OWNERSHIP entry in the same wave.
- Host cleanup of stale `~/.cc/3` (and judgment on `.cc/4`) is user-owned — the code must be correct with the dirs present; deleting them is not the fix.
