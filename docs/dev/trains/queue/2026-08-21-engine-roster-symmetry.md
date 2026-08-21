# Engine roster symmetry: 0..N Claude accounts × 0..N Codex accounts, no engine assumed

Status: QUEUED · Refined: 2026-08-21 by CCC (user ruling: "someone might have multiple codex or 1 claude, or no claude and only codex — professor must not depend on only one engine; all combinations handled") · Project: pfm · Fenced wave · Pairs with `2026-08-21-doctor-external-deps.md` (engines become "at least one of", never both Required).

## Why (verified 2026-08-21)

- Claude is a roster: `config.Accounts[]` (0..N, each a config dir). Codex is a singleton: one `paths.CodexRoot` (`~/.codex`) and one `config.Codex` prefs block (`config.go:63-69`), with per-account Codex *overrides* (`config.go:441-445, 704-711`) that still point at the single root.
- The picker always offers "New Claude chat" (`compose.go:59 includeNewClaude` unconditional) but offers Codex only when `CodexAvailable` (`compose.go:60`, computed at `pipeline.go:474` from `CodexRoot` + binary). A Codex-only machine still shows a Claude row that cannot launch.
- `reload_command.go:439,484` and `config.go:971` treat an empty Claude roster as an edge, not a first-class state; the Limits tab, statusline, `/reload`, `chat new` default engine, and the usage hook all assume ≥1 Claude account exists.
- The `ask.engine` setting (`config.go:508`) is validated to `codex|claude` but nothing checks the chosen engine is actually present.

## Contract

An **engine roster** in `internal/config`: `Claude []Account` (today's roster) and `Codex []CodexAccount{ID, Home (CODEX_HOME dir), Emoji, Prefs}` — both 0..N, both discovered the same predicate-based way (Claude: `.cc/N` with valid credentials; Codex: `~/.codex` plus any `codex.homes[]` the user lists, each valid when `auth.json` has `tokens.access_token` + `account_id`), both config-owned (CONFIG-OWNERSHIP). Every surface iterates the roster it is given and renders honestly when a roster is empty ("no Claude accounts configured" is a named state, never a crash, never a phantom row).

## Tasks (inside-out)

### #1 — config: Codex roster + `Engines()` summary

- Add `CodexAccount` and `Config.CodexAccounts []CodexAccount`; default roster = `[{ID:1, Home: paths.CodexRoot}]` when valid, empty otherwise; `codex.homes` config key for more. `EffectiveCodex(id)` resolves per Codex account. `config.Engines()` returns `{Claude int, Codex int}` counts; `config.DefaultEngine()` = `ask.engine` when that engine has ≥1 account, else the one engine present, else an error naming both rosters empty.
- Validation at load: `ask.engine` naming an engine with zero accounts is a config ERROR with the fix in the message. RED-first tests for the four combinations (0/0, N/0, 0/N, N/N).

### #2 — launch surfaces iterate the roster

- Picker: "New Claude chat" row only when `Engines().Claude > 0`; "New Codex chat" per `Engines().Codex > 0`; merged new-chat row's engine toggle offers only present engines; with exactly one engine present the toggle is hidden. `⌃S` account cycling cycles the selected engine's roster; Codex rows carry their account emoji like Claude rows.
- `pfm chat new --engine {claude|codex} [--account N]`: engine defaults to `DefaultEngine()`; `--account` indexes that engine's roster; Codex launches set `CODEX_HOME` to the chosen account's home (anchor: `dream/seat/runner.go:815` already passes `CODEX_HOME=` — ONE launcher seam, reused).
- `/reload` (`reload_command.go:439,484`): empty Claude roster → named error, never a guessed account; a Codex seat reloads across Codex accounts the same way.
- Goldens: add `ui_codex_only_80.ansi` and `ui_claude_only_80.ansi` fixtures.

### #3 — observers iterate the roster

- Limits tab (after `2026-08-21-limits-live-fetch.md`): one card per Claude account AND one per Codex account (`auth.json` per home); an empty roster renders one dim line `no {engine} accounts configured`.
- Statusline + usage hook: engine-aware by the seat's own engine; never read a Claude credential file when the seat is Codex.
- `pfm doctor`: `doctor: engines claude=2 codex=1 default=codex` row; zero engines total is a visible error (the tool cannot launch anything), one engine is fine. Pairs with the deps spec: `claude`/`codex` binaries are Required only when their roster is non-empty.

## Acceptance

- `dev.sh iso test pfm` + `iso verify pfm` green, fence proof quoted; the 0/0, N/0, 0/N, N/N matrix is table-tested at config, compose, and doctor levels.
- Host proof (user-run): with `ask.engine=codex` and the Claude roster emptied in a jail config, `pfm ls` shows only Codex rows and a single new-chat row, `pfm doctor` says `engines claude=0 codex=1`.
- Walker with CONFIG-OWNERSHIP + HONEST-ABSENCE armed.

## Out of scope

- Adding a third engine; the roster type is engine-keyed so one can be added later without a new invariant.
- Codex limits fetching itself (limits-live-fetch owns the fetch; this spec makes it per-account).
