# doctor: probe every external command pfm calls, and the engines' own doctors

Status: QUEUED · Refined: 2026-08-21 by CCC (user ruling: "the doctor has to be deep — python libraries, codex doctor, claude doctor, any process pfm calls that might not exist on another machine") · Project: pfm · Fenced wave

## Why (verified 2026-08-21)

- `pfm/cmd/pfm/doctor.go` probes config, PATH canonicality, the SQLite store, crumbs, roots, process table, and the harvestpy environment (which IS deep: `harvestpy/check.go` hashes `uv pip list --format freeze` against the provisioned lock at `:239-246` and runs a live import+conversion smoke).
- It does NOT probe any other external command. `doctor.go:400-402` prints `claude.binary` / `codex.binary` as config strings and never resolves or runs them. Exec sites found by grep (non-test): `tmux` ×86, `git` ×19, `sh` ×6, `ps` ×2 (`gather/procfs_darwin.go:124`, `action/process_darwin.go:50`), `script` ×2, `sleep` ×2, `setsid`/`nohup` ×1 each, `python3` fallback (`harvestpy/converter.go:60`), plus the configured `claude` and `codex` binaries. On a machine without tmux, `pfm doctor` prints `warnings=0` — a check whose broken state reads healthy.

## Tasks (inside-out)

### #1 — `internal/deps`: ONE registry of external commands

- New package `pfm/internal/deps` with a table-driven registry: `{Name, Purpose, Required bool, Platforms []string, VersionArgs []string, MinVersion string, Parse func(string) (string, error)}` for: `tmux` (≥ the minimum the fleet's `-L`/`wait-for`/`respawn-pane -k` usage needs — the builder derives it from the flags actually used, states it in a comment with the man-page reference), `git`, `sh`, `ps` (darwin only), `script`, `setsid`/`nohup` (linux only), `sleep`, `uv` (provisioned binary path, not PATH), the configured `claude` and `codex` binaries (names come from `internal/config` — CONFIG-OWNERSHIP), and the harvestpy interpreter (provisioned path).
- `deps.Resolve(name)` = the single place an exec site obtains a binary path (`exec.LookPath` + platform filter). Every production `exec.Command("tmux"…)` etc. migrates to `deps.Resolve` (or the existing `CommandTmux` constructor takes its path from it — ONE seam, no 86 edits of call sites if the tmux wrapper already centralises; verify and state which).
- Guard test (JAIL, RED-first): a test greps `pfm/cmd` + `pfm/internal` (non-test) for `exec.Command(` / `exec.CommandContext(` / `exec.LookPath(` whose first arg is a string literal and asserts the literal is in the registry — watched failing before the migration on at least `"ps"`.

### #2 — `pfm doctor deps`: resolve, version, minimum, self-doctor

- New doctor section, one row per registry entry, same `doctor: …` line grammar: `doctor: dep tmux path=/usr/bin/tmux version=3.4 min=3.2 ok` · `doctor: dep ps platform=darwin skipped (not this platform)` · `doctor: dep codex path=(none) MISSING required — install: <one-line hint>` · `doctor: dep claude path=… version=2.1.238 self_doctor=ok`.
- Three states, never conflated: **ok**, **missing** (LookPath failed — an absence, counts as a warning when `Required`), **broken** (found but the version call failed, returned garbage, or is below `MinVersion` — an error with the raw first line quoted). A probe that could not run (timeout, permission) is **broken**, never "missing".
- Engine self-doctors, delegated not reimplemented: `claude` → run `claude --version` and `claude doctor` in non-interactive mode if the installed CLI supports it (the builder checks `claude doctor --help`; if it is interactive-only, record `self_doctor=unavailable (interactive-only)` — honest, not a fake ok); `codex` → `codex --version` and `codex doctor`/`codex debug` equivalent the same way. Output is summarised to ONE line each; the raw output lands under `tmp/` only when a flag `--verbose` is given.
- harvestpy keeps its existing five rows (already deep); `python3` PATH fallback in `converter.go:60` is removed if the provisioned interpreter is mandatory (it is — "no Go fallback converter"), or registered if a path still legitimately uses it. No dead fallbacks.
- Timeouts: every version/self-doctor call bounded (5s), on `context` — a hung `codex doctor` must not hang `pfm doctor`.
- Exit code: doctor exits 0 only with zero `Required` deps missing/broken (the spec at `queue/2026-08-20-pfm-install-surface-dependencies.md` § doctor-exits-0-after-clean-install still holds: a clean install on a machine WITH the deps exits 0).
- Tests (JAIL): fake PATH dirs with stub binaries printing version strings → ok/min-violation/garbage/missing/timeout rows each asserted by exact line; platform filtering asserted on both GOOS values via the registry's `Platforms`.

### #3 — `pfm install` preflight uses the same registry

- `pfm install` runs `deps` preflight first and refuses (exit 1, the rows printed) when a `Required` dep is missing — instead of failing later inside tmux or git with a raw exec error. `--force` does not bypass a missing Required dep; `--skip-harvest` only skips the harvestpy rows. TESTPLAN rows added.

### #4 — `doctor hooks`: every hook pfm installed is present and points at THIS binary (user-ordered 2026-08-21)

- Anchors: `internal/installer/settings.go:79-159` (Claude: `UserPromptSubmit` group+usage commands, `SessionEnd` clear-kill, statusline), `installer.go:912-918` (candidate `settings.json` per account config dir), `codex_hooks.go:11-68` (`~/.codex/hooks.json`, `appendCodexClearHook`), `settings_ownership.go:34-157` (the ownership ledger: which commands pfm owns, per file), `doctor.go` has no hook probe today (`grep -i hook cmd/pfm/doctor.go` → 0).
- Source of truth = the installer's own expected-hook list (the same function that installs them, exported as `installer.ExpectedHooks(config)` → `[{File, Event, Command}]`) — doctor never carries a second list (NO duplication). For each expected hook: file exists and parses → event array present → a command entry matching `installerOwnedHookCommand` → its binary path equals the canonical `~/.local/bin/pfm` (a hook pointing at a stale/foreign pfm is **broken**, not ok).
- Scope: every Claude account in the roster (global config-dir `settings.json`; plus a project-level `.claude/settings.json` ONLY if the installer writes one — verify in `installer.go`; if it never does, say so in the row grammar and probe nothing there), and every Codex home (`hooks.json`). Rows: `doctor: hook claude[1] SessionEnd clear-kill ok` · `doctor: hook codex ~/.codex/hooks.json UserPromptSubmit usage MISSING — run pfm install` · `doctor: hook claude[2] settings.json broken error=<parse error>` · a hooks file that exists but is unreadable is **broken**, never "missing".
- Ownership ledger cross-check: a command present in the file but absent from the ledger (or vice versa) is a named `drift` row — the state `pfm uninstall` would mishandle.
- Tests (JAIL, RED-first): plant a settings.json missing one owned hook → MISSING row + warning; point one hook at `/usr/local/bin/pfm` → broken row; corrupt hooks.json → broken; all present → ok rows and zero warnings. Watched failing against HEAD where doctor prints nothing about hooks.

## Acceptance

- `dev.sh iso test pfm` + `iso verify pfm` green with fence proof. The registry guard test is RED-then-GREEN in the ledger line.
- Host proof (user-run after mirror build): `pfm doctor` shows one `hook` row per installed hook (all ok on this host) and one `dep` row per registry entry with real versions for tmux/git/claude/codex; renaming `tmux` out of PATH in a jail HOME makes the row say `MISSING required` and doctor exit non-zero.
- `docs/dev/pfm-surface.md` doctor row updated; no machine-absolute path in any tracked file.
- Walker with HONEST-ABSENCE armed — the three-state contract is the invariant under test.

## Out of scope

- Auto-installing missing dependencies (doctor reports, install refuses; nothing fetches tmux/git for the user).
- Docker / the `dev.sh iso` fence — a developer-only dependency, not a runtime one.
