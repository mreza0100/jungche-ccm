# Wave 1 — internal task ordering and #11 reconciliation

**Reader:** builder-1 (steps S2a–S5) and the walker (S8). This file is ordering notes, not spec —
neither `spec.md` (#11) nor `spec-install-surface.md` (#20–#22) is edited by it. Task identity and
bodies stay byte-identical to their sources.

## Order inside the wave

`#20 → #21 → #22 → #11-close`. The edges, verified in the tree 2026-08-20:

- **#22 needs #20:** `--skip-harvest` composes with `--yes` (#22 behavior a); `--yes` exists only
  after #20.
- **#21 sits between:** its acceptance gate is the e2e phase (a) `doctor` exit-0 assertion, exercised
  through #20's new call form; #22 behavior (c) then asserts `doctor` reports `skipped`.
- **#11 closes last:** its harness asserts all three surfaces.

## #11 reconciliation — stale call forms (spec text vs #20's ruling)

Task #11's spec body (`spec.md`) predates the founder's 2026-08-20 revision and still says
`pfm install --apply` (phases a, c) and bare `pfm uninstall` (phase d). The built harness follows the
pre-ruling production surface instead. Neither text is edited; the ruling column below governs, and
#20's call-site sweep (behavior e) plus #22's file plan execute it:

| Site | Today | Under the ruling |
| --- | --- | --- |
| `pfm/e2e/install_e2e_test.go:110,129,132` | `harness.pfm(home, "install", "--apply")` | `install --yes` (with `--skip-harvest` where #22 phase f applies) |
| `pfm/e2e/install_e2e_test.go:144-146` | comment + `install --uninstall` | top-level `uninstall` — the spec's phase (d) form becomes REAL under #20 (c); delete the "shipped surface names ModeUninstall as install --uninstall" comment |
| `pfm/e2e/install_e2e_test.go:311-320` | ANY "harvestpy" mention in output = failure | flips: assert the exact report line `harvestpy: skipped (blocked, not attempted)` (#22 behavior a); env plumbing `PFM_E2E_HARVESTPY_GATE` in `install-verify.yml:39,66` follows |
| `pfm/cmd/pfm/mcp_serve_command.go:130` | "run pfm install --apply" | "run pfm install --yes" |
| `pfm/cmd/pfm/update_command.go:231` | "install --apply after staging" | new form (#20 e) |
| `pfm/cmd/pfm/install_command_test.go:21` | `runInstall([]string{"--dry-run"}, …)` | bare preview — caught by #20 (e)'s repo-wide re-grep |
| `README.md:164`, `INSTALL.md:45-149` | `--dry-run` / `--apply` / `install --uninstall` | new forms; INSTALL.md:60's exit-97 gate paragraph stays true, only spellings change |

Note on #22 (d): neither `scripts/e2e-linux.sh` nor `install-verify.yml` invokes `pfm install`
directly — the Go harness does. "Pass `--yes --skip-harvest`" lands in the harness's install
invocations; the script/workflow edits are the env and docs plumbing around it. All three files are
in #22's file plan.

## #11 close conditions (S5 local + S10 CI)

1. All prior local e2e invocations were compile-only (`-run '^$'`). S5 requires a REAL
   `scripts/e2e-linux.sh` docker run, watched green.
2. The darwin job cannot run locally; #11 fully closes only when `install-verify.yml` (linux +
   darwin) runs green in CI — which happens after S9's commit AND a founder-ordered push (S10).
   Until then #11's status is "locally green, CI pending", stated, never rounded up.
