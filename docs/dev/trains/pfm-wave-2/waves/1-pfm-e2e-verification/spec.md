# Wave 1 — pfm-e2e-verification

**Train:** pfm-wave-2
**Status:** SCHEDULED
**Source:** `docs/dev/trains/queue/2026-08-20-config-ui-mcp-install.md` — Refined 2026-08-20 · main @ `7b01caa`
**Touches:** pfm, docs (README/INSTALL), CI
**Seat:** builder-1 (Codex code seat; scope = this wave's write paths)
**Write paths (exclusive):** `pfm/e2e/**`, `scripts/e2e-linux.sh`, `.github/workflows/install-verify.yml` — extended 2026-08-20 by the merged-in #20–#22 with `pfm/cmd/pfm/**`, `pfm/internal/installer/**`, `pfm/TESTPLAN.md`, `README.md`, `INSTALL.md` (see `spec-install-surface.md`)
**Tasks:** #20 → #21 → #22 (bodies in `spec-install-surface.md`) → #11 (below, closes last) — order and #11 call-form reconciliation in `ordering.md`
**Numbering:** source numbers preserved — see `../../train.md` § Numbering.

Task body below is byte-identical to the source spec. Its call forms `pfm install --apply` (phases a,
c) and bare `pfm uninstall` (phase d) predate the founder's 2026-08-20 ruling — `ordering.md` carries
the reconciliation; this body is not edited.

## All-task rules

- **Public repo.** No founder name, email, or machine-absolute `/home/…` path in any code, fixture, comment, or doc. Test fixtures invent neutral values.
- **Only gitter commits; publication (push/tag/release) is founder-owned** and never instructed by this spec.
- **Live-box law:** never reboot devbox; never touch live `cc-*`/`cx-*`/`vsct*` sockets or real `~/.claude`/`~/.cc`/`~/.codex` in tests — every fixture runs in a temp HOME/jail. Never press Enter/⌃O in a jailed picker (launch path escapes the jail); drive picker tests through the pure `ui` layer or `--plain`.
- **Ship = installed:** every pfm task ends with `go build -o ~/.local/bin/pfm ./cmd/pfm` and hash-verifies which binary PATH resolves.
- **Build agents:** `dev` per task, `qa` per modified project, except tasks tagged `[CMD: /pcm]` (blueprint/CLAUDE.md/.claude are hook-guarded).
- **Pre-authorized founder touchpoints:** writing `~/.config/pfm/config.json` on devbox (Task #1 rollout); one Haiku API call per stale-token limits refresh (Task #6); installing the MCP daemon systemd user unit on devbox (Task #8); registering MCP servers in client configs when enabled (Task #10). No other outward-facing action is authorized.

---

### Task #11 — E2E install → init → update → uninstall harness (Linux container + macOS runner) [MILESTONE]

**Why:** every installer test today calls `Run()` in-process with a fakeRunner; nothing executes the real binary end-to-end, and Darwin never runs at all. Founder ordered: fresh isolated Linux + Darwin machines, everything mechanical checked, uninstall too, update too, no markdown-content greps.

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors (serial phases, one Go test file, build tag `e2e`):**
a. **Install:** exec the freshly-built REAL `pfm` binary, `HOME=$(t.TempDir())`, `pfm install --apply`. Assert mechanically: every target path from the installer-surface table exists (symlinks resolve, zsh shim sourced line present in `.zshrc`, hook entries present via JSON parse — never text grep), `zsh -n` passes on the shim, `pfm doctor` exits 0 in that HOME.
b. **Init:** `pfm init $TMP/project` (Task #12b) then assert the scaffold file set exists with placeholders intact (file EXISTENCE and JSON validity only — no markdown content assertions, founder's rule).
c. **Update:** build the PREVIOUS release tag's binary from a local clone, install it into a second fresh HOME, then run the HEAD binary's `pfm install --apply` over it; assert the resulting file set + hook entries are byte/set-identical to phase (a)'s fresh install (convergence = the update path).
d. **Uninstall:** `pfm uninstall` (ModeUninstall); assert every installer-owned path is gone AND a planted manual hook + foreign file survived.
e. Runs serially (`-p 1`, ordered subtests); each phase's failure prints the differing path list, not a diff blob.
f. CI: NEW workflow `install-verify.yml` — job `linux` runs the e2e test inside `ubuntu:24.04` container (installs zsh/tmux/git only — harvestpy provisioning explicitly skipped via the installer's existing offline/skip path, asserted as "blocked, not attempted"); job `darwin` on `macos-14` (the "brand-new isolated Darwin machine" — every GH runner is fresh) runs the same test natively. Both jobs required.
g. Local runbook: `pfm/e2e/README` one-liner `go test -tags e2e -p 1 ./e2e/...` + the docker wrapper `scripts/e2e-linux.sh` for devbox runs (never against the real HOME — the harness refuses to run when `HOME` equals the invoking user's real home).

**Data model / Contracts:** none.

**File plan:** NEW `pfm/e2e/install_e2e_test.go` (phases as ordered subtests, `//go:build e2e`), `pfm/e2e/doc.go`; NEW `scripts/e2e-linux.sh`; NEW `.github/workflows/install-verify.yml`.

**Boundaries & anchors:** devbox itself is NEVER the test host for install/uninstall (live fleet); reboots forbidden everywhere; tmux config validation runs `tmux -f <conf> -L e2e-probe-$$ start-server \; kill-server` inside the container/temp socket only. No markdown-content grep tests (founder's explicit rule — existence + parseability only).
