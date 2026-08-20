# Wave 1 extension — pfm-install-surface (#20–#22)

**Train:** pfm-wave-2
**Status:** SCHEDULED
**Source:** `docs/dev/trains/queue/2026-08-20-pfm-install-surface-dependencies.md` — Refined 2026-08-20 · main @ `7b01caa` (anchors verified against the working tree)
**Touches:** pfm, docs (README/INSTALL), CI
**Seat:** builder-1 (Codex code seat; scope = this wave's write paths — precedent: builder-1 already wrote `scripts/e2e-linux.sh` and `.github/workflows/install-verify.yml` in the DONE span)
**Write paths (exclusive, added to wave 1):** `pfm/cmd/pfm/**`, `pfm/internal/installer/**`, `pfm/TESTPLAN.md`, `README.md`, `INSTALL.md` — plus wave 1's original `pfm/e2e/**`, `scripts/e2e-linux.sh`, `.github/workflows/install-verify.yml`
**Tasks:** #20 → #21 → #22, then #11 closes (see `ordering.md` beside this file)
**Numbering:** source numbers preserved — see `../../train.md` § Numbering. #20–#22 extend the same number space; #18, #19 remain taken by F1/F4.

These three tasks are the production surfaces Task #11's e2e harness asserts. They were phantom
anchors in the original refinement; the founder ruled them into existence 2026-08-20. They are NOT a
separate wave: the producer→consumer edge to #11 makes them one feature, so they fold into wave 1
with internal ordering #20 → #21 → #22 → #11-close. Task bodies below are byte-identical to the
source spec.

## All-task rules

The `## All-task rules` block of `docs/dev/trains/queue/2026-08-20-config-ui-mcp-install.md` binds every task here verbatim (public repo, live-box law, ship = installed, build agents, no new founder touchpoints).

## Task Reconciliation

| Original | Disposition | New # | Notes |
| --- | --- | --- | --- |
| #11 blocked delta: bare `pfm uninstall` absent | REFINED | #20 | founder ruled: redesign, not spec-adapt |
| #11 blocked delta: `doctor` not clean in fresh HOME | REFINED | #21 | founder ruled: fix doctor |
| #11 blocked delta: no apply-time harvestpy skip | REFINED | #22 | missing dependency, now scheduled |

---

### Task #20 — install/uninstall CLI verb redesign

**Why:** founder ruling 2026-08-20 (revised same day): bare `pfm install` previews the work and names its confirm step; `pfm install --yes` applies; there is no `--dry-run` flag because the bare command IS the dry run; `pfm uninstall` is a real verb — "there is no `--uninstall` for install." Today bare `pfm install` is silently `ModeDryRun` (`install_command.go:43`), apply requires `--apply`, and uninstall is the flag `--uninstall` (`install_command.go:28-48`); no top-level `uninstall` verb is registered in `cmd/pfm` (grep `"uninstall"` in the command dispatch: zero verb hits).

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. Bare `pfm install` runs the PREVIEW (today's `ModeDryRun` mechanics — the full plan of what it wants to do) and closes with exactly one line: `if you agree, run again: pfm install --yes`. Preview exit code semantics unchanged.
b. NEW flag `--yes` APPLIES (today's `--apply` semantics, including provisioning per #22). Bare preview and `--yes` classify identically; only the actions differ — the dry run IS the apply's preview (pfm CLAUDE.md law). `--force` and `--config-dir` unchanged and compose with both forms.
c. NEW top-level verb `pfm uninstall` dispatching `installer.ModeUninstall` (mode exists: `pfm/internal/installer/installer.go:83`); accepts `--config-dir`. Registered in the command dispatch and `pfm help`.
d. `--apply`, `--uninstall`, and `--dry-run` flags on `install` are all REMOVED — unknown-flag error, no hidden alias; usage string (`install_command.go:28`) rewritten to `pfm install [--yes] [--force] [--config-dir DIR]`.
e. Call-site sweep — every invocation and doc mention updated to `pfm install --yes` / `pfm uninstall`: `pfm/cmd/pfm/{mcp_serve_command.go,update_command.go}`, `pfm/e2e/install_e2e_test.go`, `README.md`, `INSTALL.md` (grep-verified list; builder re-greps `install --apply|install --uninstall|--uninstall|--dry-run` repo-wide including `.github/workflows/**` and error strings before closing).
f. Failure path: `pfm install --apply` (or `--dry-run`) after removal prints the unknown-flag usage error naming the new form — the migration hint lives in the usage line, not a compat shim.

**Data model / Contracts:** none / CLI surface only.

**File plan:** EDIT `pfm/cmd/pfm/install_command.go` (mode default → ModeApply, flags cut, usage); NEW `pfm/cmd/pfm/uninstall_command.go` (`runUninstall`); EDIT the dispatch table registering `uninstall`; EDIT call-site files per (d); EDIT `pfm/TESTPLAN.md` rows naming the new surface.

**Boundaries & anchors:** NOT included: any installer.Run behavior change — modes and their effects are untouched, only the CLI mapping moves. Anchors: `install_command.go:26-48` (current flag set), `installer.go:52,83,88` (ModeUninstall gating), `install --help` output shape (usage string is golden-tested if a golden exists — builder greps `usage: pfm install`).

---

### Task #21 — doctor exits 0 on a fresh, correctly-installed HOME

**Why:** founder ruling 2026-08-20: fix doctor — a doctor that can never say "healthy" after a perfect install cannot distinguish broken from fine. Today `doctor` accumulates `warnings` (`doctor.go:57`) and any nonzero count exits 1; a fresh HOME trips PATH/hash and harvestpy-provisioning warnings by construction (observed live: `rc=1` with `pfm_path_resolves` / `pfm_hash_mismatch` warnings).

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. In a HOME where `pfm install` just completed (including with `--skip-harvest`, #22), `pfm doctor` exits 0.
b. Builder enumerates every probe feeding `warnings` in `doctor.go` and classifies: EXPECTED-after-clean-install states (e.g. harvestpy deliberately skipped → reported as `skipped`, not a warning; PATH/hash checks resolve against the target HOME's own install, not the invoking host's shims) vs genuine defects (stay warnings → exit 1). The classification table lands as a comment-free doc line per probe in `pfm/TESTPLAN.md`.
c. Broken-state report (the standing question): a probe that CANNOT run (unreadable dir, exec failure) stays a hard failure — `doctor.go:79,98,103,132` return-1 paths are untouched; "skipped/expected" never absorbs "failed to look".
d. The e2e harness phase (a) assertion (`pfm doctor` exits 0 in the temp HOME) is the acceptance gate — it must go from red to green on this task, watched.

**Data model / Contracts:** none / exit-code contract: 0 = healthy or expected-clean-install state; 1 = defect or failed probe.

**File plan:** EDIT `pfm/cmd/pfm/doctor.go` (probe classification); EDIT `pfm/internal/*` probe helpers only where a probe needs a jail-aware target (follow existing `internal/paths` override pattern — never hardcode HOME); EDIT `pfm/TESTPLAN.md`.

**Boundaries & anchors:** NOT included: new probes, output redesign. Anchors: `doctor.go:57` (`warnings := 0`), `:79,98,103,132` (hard-fail returns), `printHarvestPythonDoctor` (harvestpy warning source).

---

### Task #22 — apply-time harvestpy offline/skip path

**Why:** #11's CI container job must assert harvestpy provisioning "blocked, not attempted"; production `pfm install` apply has no skip surface — `install_command.go:55` hardcodes `ProvisionHarvest: true`, and the harvestpy dry-run's own status line states "apply requires valid cached pins or network". The seam exists (`installer.Options.ProvisionHarvest`); only the CLI wiring and the visible report are missing.

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. `pfm install --yes --skip-harvest` applies with `ProvisionHarvest: false`; the install report prints one line: `harvestpy: skipped (blocked, not attempted)` — the exact string #11 phase (f) asserts. On the bare preview, `--skip-harvest` shows the would-skip plan line.
b. Without the flag, provisioning behavior is unchanged; a provisioning FAILURE stays a loud error, never silently downgraded to skipped (skip is a request, not a fallback).
c. `pfm doctor` (#21) reports the skipped state as `skipped`, exit 0.
d. `.github/workflows/install-verify.yml` linux container job and `scripts/e2e-linux.sh` pass `--yes --skip-harvest`.

**Data model / Contracts:** none / flag → `installer.Options.ProvisionHarvest` (exists, `pfm/internal/installer/types.go`).

**File plan:** EDIT `pfm/cmd/pfm/install_command.go` (flag + wiring); EDIT `pfm/internal/installer/installer.go` (skip report line at the provisioning call site); EDIT `pfm/e2e/install_e2e_test.go`, `scripts/e2e-linux.sh`, `.github/workflows/install-verify.yml`; EDIT `pfm/TESTPLAN.md`.

**Boundaries & anchors:** NOT included: harvestpy internals, pin/cache logic. Anchors: `install_command.go:55` (`ProvisionHarvest: true`), `installer.NewHarvestProvisioner` (`install_command.go:16-23`).
