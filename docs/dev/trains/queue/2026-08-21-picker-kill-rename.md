# Picker actions: drop `reload` + ⌃R, wrap the cursor, own the account roster, rename hide → kill

Status: QUEUED · Refined: 2026-08-21 by CCC (user-ordered, verified anchors) · Project: pfm · Fenced wave (worktree + `dev.sh iso`)

## Why

- `[⟳ reload]` on a chat row only rescans the picker list (`cmd/pfm/commands.go:191` → `forceFull = true`); the user does not want it as a row action.
- `internal/ui/model.go:169` `validAccount` falls back to `ids := []int{1, 2, 3}` — a hardcoded roster, CONFIG-OWNERSHIP defect class (the Limits wave fixed discovery but missed this caller).
- ⌃X / `✖ hide` writes the hidden ledger AND kills the live server (`cmd/pfm/commands.go:384-389` `hideApplier`: "Hiding a running chat ENDS it"). The word undersells the act; the user rules the verb is **kill**, everywhere a human or agent reads it.

## Tasks (inside-out)

### #1 — `validAccount` roster from config only

- Anchor: `pfm/internal/ui/model.go:169-177`.
- Remove the `{1,2,3}` literal. `validAccount(account, roster)` takes the roster as a required argument; an empty roster returns 0 (no valid account) — never a guessed id. Update every caller (grep `validAccount(`).
- RED first: a test asserting that account 3 is NOT valid when the roster is `[1,2]`, watched failing before the change.

### #2 — remove the `reload` row action

- Anchors: `pfm/internal/ui/render.go:576` (`{"⟳", "reload"}` in the action carousel), `pfm/internal/ui/model.go:338-341` (`actionIndex` case 1 → `OutcomeReload`), the `ctrl+t` key case in `model.go`, footer strings `render.go:254,257` (`⌃T reload`), `OutcomeReload` in `types.go:103` and its consumer `cmd/pfm/commands.go:191-193`.
- Remove the action, the key, the outcome, and the footer text end to end; renumber `actionIndex` so `open → reboot → 1h → kill` stays contiguous. Rescan remains available however it is today outside the row carousel (verify: grep what else triggers `forceFull`); if nothing else does, remove `forceFull` too — no dead code.
- Golden files regenerate: `pfm/testdata/golden/ui_80.ansi`, `ui_120.ansi` (lines 7 and 17). `pfm/TESTPLAN.md` rows naming ⌃T update. `docs/dev/pfm-surface.md:18` carousel text updates.
- Tests: `pipeline_async_test.go:519,523` reference `OutcomeReload` — rewrite or delete with the outcome.

### #3 — rename hide → kill, every reader-facing surface

Scope (851 occurrences, 59 non-test Go files — `grep -rn -i 'hide\|hidden' pfm --include='*.go' | grep -v _test`):

- Package `pfm/internal/hide` → `pfm/internal/kill` (types `HideChange` → `KillChange`, `Manager` verbs, `doc.go`).
- Picker: action `{"✖", "hide"}` → `{"✖", "kill"}`; footer `⌃X hide` → `⌃X kill`; `toggleHidden` → `toggleKilled`; `reportHides` receipt text `hid N, ended N` → `killed N (N live ended)`.
- CLI: `pfm chat hide|unhide` → `pfm chat kill|unkill`; `pfm ls --hidden|-H` → `pfm ls --killed|-K` ("the killed ledger"); `pfm internal clear-hide|hide-exit` → `clear-kill|kill-exit` (`cmd/pfm/main.go:332,379`, `clear_hide_command.go` → `clear_kill_command.go`). Installer hook commands follow: `internal/installer/settings.go:21`, `codex_hooks.go:18`. `pfm doctor` row `hidden=… orphaned_hidden=…` → `killed=… orphaned_killed=…`.
- MCP: tools `chat_hide`/`chat_unhide` → `chat_kill`/`chat_unkill` (`internal/mcpserv/actions.go:132-141`, `types.go:249` `HideInput` → `KillInput`); `docs/dev/pfm-surface.md:66,93` follow.
- Installed assets under `pfm/internal/installer/assets/` that mention hide as the chat verb (grep; `chat/inject.command.md:35` uses "hides" in plain English — leave English prose alone, rename only the verb/feature).
- **Compat (data already in the wild — MUST hold):** the SQLite table `hidden` / column `hidden_at` (`internal/store/hidden.go:28,108`, `store.go:208,229`) keeps its physical name; Go identifiers rename, schema does not (no migration, `user_version` stays 5). Labels already written as `_HIDE…` are still recognised by the read side (`compose`'s label exclusion) alongside the new `_KILL…` form; new writes use `_KILL…`. A regression test proves an existing `_HIDE`-labelled row and an existing `hidden` table row are still treated as killed after the rename.
- Docs: `docs/dev/pfm-surface.md`, `docs/BLUEPRINT.md`, `docs/README.md`, `pfm/TESTPLAN.md`, `pfm/README*` — rename the verb; no machine-absolute paths, no PII.

### #4 — remove ⌃R project rotation

- Anchors: `pfm/internal/ui/model.go:291-295` (`ctrl+r` advances `model.rotation`), `model.go:71,129` (state + snapshot load), `model.go:787-795` (group ordering by rotation), `render.go:179-182` (`project rotation %d` status text), `render.go:254,257` footer (`⌃R projects` / `⌃R rotate`), `cmd/pfm/commands.go:187` (`rotation = outcome.Rotation` persisted across picker re-opens — grep `Rotation` in `internal/ui/types.go` and the snapshot store).
- Ruling (CCC): a rotation nobody can turn is dead state — remove the feature end to end (key, field, snapshot persistence, footer, status text), groups render in their natural order (`rotation == 0` today). Golden files + TESTPLAN rows follow.

### #5 — cursor wraps top↔bottom in the Chats list

- Anchor: `pfm/internal/ui/model.go:360-371` (`up`/`ctrl+p` stops at 0, `down`/`ctrl+n` stops at `len(filtered)-1`).
- `up` on the first row moves to the last; `down` on the last row moves to the first; empty list is a no-op; `actionIndex` resets to 0 as today. Stats tab cursors (`model.go:454-475`) are NOT in scope.
- RED first: two tests (wrap up from 0, wrap down from last) watched failing before the change; the existing "stops at the edge" assertions, if any, flip.

## Acceptance

- `.claude/scripts/dev.sh iso test pfm` green with fence proof; `iso verify pfm` green; `go vet -tags e2e ./e2e/...` compiles.
- `grep -rn -i 'hide' pfm --include='*.go' | grep -v _test | grep -iv 'hideVimModeIndicator\|hides who is speaking'` returns ONLY the physical schema strings in `internal/store` — every other hit is a defect.
- `pfm ls` golden files show `▶ open → ⚡ reboot → 🕐 1h → ✖ kill`; footer carries no ⌃T and no ⌃R; `up` at row 1 lands on the last row.
- `scripts/leak-check.sh` clean.
- Walker (`/wave:walker`, branch mode) over the merge candidate — CONFIG-OWNERSHIP and HONEST-ABSENCE invariants in scope.

## Out of scope

- Reworking what kill does (store write + live kill stays as is).
- `hideVimModeIndicator` (a Claude settings key, not ours).
- Host cleanup of `~/.cc/3` (user-owned; config entry already removed 2026-08-21).
