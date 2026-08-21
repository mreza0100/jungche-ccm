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

## Acceptance

- `dev.sh iso test pfm` + `iso verify pfm` green with fence proof. The registry guard test is RED-then-GREEN in the ledger line.
- Host proof (user-run after mirror build): `pfm doctor` shows one `dep` row per registry entry with real versions for tmux/git/claude/codex; renaming `tmux` out of PATH in a jail HOME makes the row say `MISSING required` and doctor exit non-zero.
- `docs/dev/pfm-surface.md` doctor row updated; no machine-absolute path in any tracked file.
- Walker with HONEST-ABSENCE armed — the three-state contract is the invariant under test.

## Out of scope

- Auto-installing missing dependencies (doctor reports, install refuses; nothing fetches tmux/git for the user).
- Docker / the `dev.sh iso` fence — a developer-only dependency, not a runtime one.
