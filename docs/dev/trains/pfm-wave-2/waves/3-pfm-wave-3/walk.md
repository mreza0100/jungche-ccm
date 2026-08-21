# W3 — pfm-wave-3 walker review

**Branch:** `train/pfm-wave-3` (diff `main...train/pfm-wave-3`) · **Date:** 2026-08-21 · **Mode:** branch, pre-merge

## Specs in the span

| # | Spec | Commit |
|---|------|--------|
| 1 | picker-kill-rename | 97f7b6e |
| 2 | limits-live-fetch | 5449442 |
| 3 | harvestpy-provision-publish | 2aa7552 |
| 4 | claude-launcher-tmux | cca1b8e |
| 5 | doctor-external-deps (+hooks probe) | 672dcc5 |
| 6 | engine-roster-symmetry | cfa14da |
| 7 | chat-status-ask-summary | 5c20958 |

Spec files: `docs/dev/trains/queue/2026-08-21-*.md`. Build evidence: `docs/dev/trains/pfm-wave-2/STATE.md` (builder ledger lines with fence container ids).

## Wave Review

# W3 pfm-wave-3 — Walker Consumer-Tree Review

## Plan

- **Units dispatched:** 7 — `u04-pfm-internal` (order 0), `u01-pfm-cmd-plus2` (order 1), `u06-pfm-internal` (order 2), `u02-pfm-internal-plus2` (order 3), `u03-pfm-internal-plus1` (order 4), `u05-pfm-cmd-plus1` (order 4), `u07-pfm-internal` (order 5, depends on u03/u02/u01/u05).
- **Coverage (disk `plan.json`):** `changedSourceFiles` 222, `ownedFiles` 222, `unownedFiles` **0** (full partition — every changed source file has exactly one owning unit).
- **Excluded from source coverage (20 files, by design):** docs (`docs/README.md`, `docs/dev/pfm-surface.md`, `docs/dev/trains/pfm-wave-2/STATE.md`, two `queue/2026-08-21-*.md` specs, `pfm/AGENTS.md`, `pfm/CLAUDE.md`, `pfm/TESTPLAN.md`) and golden/test-data fixtures under `pfm/testdata/golden/*` (11 files) — non-executable, correctly out of scope.
- **Unresolved count:** 221 (dangling/too-many-ref symbol hops the resolver could not statically close).
- **Tool runtime:** 31.7s.

## Coverage

**Walkers dispatched 7, reported 7, died: none.**

Per-unit `ownedFiles` (disk) vs `filesOpened` (walk report):

| Unit | Owned | Opened-from-owned | Gap |
|---|---:|---:|---:|
| u04-pfm-internal | 26 | 26 | 0 |
| u01-pfm-cmd-plus2 | 23 | 21 | 2 |
| u06-pfm-internal | 32 | 32 | 0 |
| u02-pfm-internal-plus2 | 24 | 21 | 3 |
| u03-pfm-internal-plus1 | 25 | 25 | 0 |
| u05-pfm-cmd-plus1 | 32 | 23 | 9 |
| u07-pfm-internal | 60 | 31 | 29 |
| **Total** | **222** | **179** | **43** |

Every owned file no walker opened, by unit — split by whether the owning walker itself named the gap in its `nodesUnreachable` (acknowledged budget skip) or left it unmentioned (silent gap):

**u01-pfm-cmd-plus2 — 2 gaps, both acknowledged:**
- `pfm/cmd/pfm/explore_deny_command_test.go` (named: "TestExploreDenyFailsOpenAndSteersExploreToTracer — not opened")
- `pfm/cmd/pfm/pipeline_async_test.go` (named: three producer ranges listed as not opened)

**u02-pfm-internal-plus2 — 3 gaps, 2 acknowledged + 1 SILENT:**
- `pfm/internal/archive/archive_test.go` (named: "entire file — test file, lower priority")
- `pfm/internal/kill/kill_test.go` (named: "entire file (~1300 lines) — test file, lower priority")
- `pfm/internal/shared/shared.go` — **SILENT.** Not opened, not mentioned anywhere in this unit's `nodesUnreachable` or notes, despite being one of its 24 owned files.

**u05-pfm-cmd-plus1 — 9 gaps, all acknowledged:**
`pfm/cmd/pfm/booting_row_jail_test.go`, `clear_kill_jail_test.go`, `kill_cli_engine_jail_test.go`, `lineage_e2e_test.go`, `lineage_member_kill_test.go`, `main_test.go`, `picker_cancel_jail_test.go`, `testmain_test.go`, `pfm/internal/kill/tmux_jail_test.go` — every one is individually named in `nodesUnreachable` as a budget skip.

**u07-pfm-internal — 29 gaps, 13 acknowledged + 16 SILENT:**
Acknowledged (named in `nodesUnreachable`): `config/engine_roster_test.go`, `deps/probe_test.go`, `gather/fake_procfs_test.go`, `index/codexstate_test.go`, `store/killed_test.go`, `store/label_candidates_test.go`, `store/queries_test.go`, `store/stress_test.go`, `ui/golden_test.go`, `ui/model_test.go`, `ui/label_kill_test.go`, `ui/fixture_test.go`, `ui/stress_test.go`.
**SILENT** (owned, never opened, never named as a gap): `pfm/internal/action/config_fixture_test.go`, `action/config_policy_test.go`, `action/headless_test.go`, `agentopen/agentopen_test.go`, `ask/ask_test.go`, `compose/compose_test.go`, `compose/golden_test.go`, `compose/label_kill_test.go`, `compose/stress_test.go`, `harvestpy/harvestpy_test.go`, `headless/summary_test.go`, `installer/expected_hooks_test.go`, `kill/viewport_test.go`, `mcpserv/server_test.go`, `spawn/spawn_test.go`, `stats/limits_test.go`.

u04, u06, u03 have **zero** gaps — every owned file was opened.

## Should-have-changed

One entry total, grouped by producer:

**u03-pfm-internal-plus1** — `pfm/internal/reap/busy.go:59-63` (`NewClaudeAgentsConfigured` filter loop). Severity **high**. `os.Stat` failures other than `ErrNotExist` (permission-denied, broken mount) are folded into the same silent "not configured" bucket as a genuinely-absent directory, so `BusySessions()` returns a busy set with an undetected hole instead of the fail-closed error its own doc comment promises — `pfm reap --apply` can then tmux-kill a session that is actually busy. Confirmed **pre-existing** (not a regression of this wave's diff — `git diff 61d5771d..5c20958 -- busy.go` touches only the `exec.LookPath`→`deps.Resolve` swap). Carries `rulingRef: results-w3/rulings-w3-invariants.md:1 HONEST-ABSENCE` — see Standing rulings (CONTRADICTED) below; this entry is excluded from the Defects list per that ruling linkage.

## Defects

9 defects, deduped by location (no duplicate locations found), severity-sorted:

**Critical**

1. **`pfm/internal/harvestmcp/remote.go:435` (consentPage)** — u04. The repo-wide hide→kill text sweep rewrote the literal HTML attribute `type="hidden"` to `type="killed"` on the OAuth `txn` input. `killed` is not a recognized HTML5 input type, so browsers apply the missing-value default (`text`): the internal OAuth transaction id now renders as a visible, editable text box above the passphrase field on the live harvester-MCP consent page, and a user or autofill extension can corrupt the txn correlation before submit.
   Evidence: `<input type="killed" name="txn" value="` + html.EscapeString(txn) + `">`
   Fix: restore `type="hidden"`; add a regression test asserting the rendered page never contains `type="killed"`.

**Med**

2. **`pfm/internal/heal/heal.go:151`** — u04. Same blind sweep flipped a concurrency-safety doc comment's meaning: "an immutable handle **hides** the -wal" (accurate — a stale-read risk) became "an immutable handle **kills** the -wal" (implies destruction), leaving the comment internally inconsistent with the sentence that follows it.
   Fix: restore "hides the -wal".

3. **`pfm/internal/config/config.go:734-784` (`validateCodexHomes`)** — u01. An operator cannot re-home an already auto-discovered Codex account id (`codex.homes:[{id:1, home:<new path>}]`) — the mismatched-path lookup falls through to the id check and fails with a misleading "duplicates id 1" on every `pfm` invocation, with no schema path to actually move an existing id's home.
   Fix: allow a `codex.homes` entry to re-home an existing auto-discovered id, or name the auto-discovery source in the error.

**Low**

4. **`pfm/internal/harvestpy/digest.go:41` and `pfm/internal/dream/morning.go:70`** — u04. Same sweep produced nonsensical comment prose outside the picker/kill domain ("archive compression can **kill** the shipping cost"; "does not **kill** later repositories") — comment-only, no functional change, but both now say something other than intended.
   Fix: restore "hide" in both.

5. **`pfm/internal/naming/naming.go:15`** — u01. The legacy `_HIDE` read-compat prefix is spelled via string concatenation (`"_" + "HI" + "DE"`) specifically so the literal substring never appears in source — defeating any grep-based audit (including this walk's own) that checks the rename left no residual `_HIDE` surface.
   Fix: spell it as a normal string literal with a comment noting it's intentional legacy read-compat.

6. **`pfm/testdata/e2e.sh:146-198`** — u01. Modified (kill/unkill verb assertions) but not invoked by any CI workflow, `dev.sh` target, or Go test — a self-declared "reference harness" that gates nothing automatically; a future kill/unkill CLI regression here is invisible to `go test ./...` and `dev.sh iso e2e`.
   Fix: wire it into a CI step/`dev.sh` subcommand, or relocate it out of `testdata` and mark it manual-only.

7. **`pfm/internal/installer/assets/shim/pfm.zsh:270-283`** — u06. Stale shared comment claims `command` prevents recursion for both `claude()`/`codex()`, but `claude()` no longer uses `command` (hardcodes the launcher path) while `codex()` still does — the comment misdescribes half of what it documents.
   Fix: split/update the comment to describe each function's actual non-recursion mechanism.

8. **`pfm/cmd/pfm/swap_jail_test.go:94`** — u02. The mechanical hide→kill rename renamed `TestChatSwapSchedulesAHiddenWorker` (about a detached/backgrounded reload worker, unrelated to the kill feature) to `TestChatSwapSchedulesAKilledWorker` — a rename-only diff that now misdescribes what the test verifies.
   Fix: revert to a name reflecting "detached worker," independent of kill vocabulary.

9. **`pfm/cmd/pfm/kill_ends_jail_test.go:63-190`** — u05. Three test functions (`TestHidingALiveChatEndsItAndClearsItsHandles`, `TestHidingAChatThatIsNotRunningKillsNothing`, `TestHidingAChatWhoseServerAlreadyDiedStillSucceeds`) kept Hide-vocabulary names/comments/`t.Fatal` messages though their bodies were fully migrated to Kill semantics — unlike every sibling jail-test file in the same rename, which got fully renamed.
   Fix: rename the three functions and their prose to Kill vocabulary.

## Fixes under verification

Disk `plan.json` ownership checked for every fix's file list. **No fix falls into "NOT VERIFIED — CLAIMED BY NO WALKER"** — all seven have at least one owning unit's backed `VERIFIED` claim. One systemic gap applies to five of the seven rows: **`u01-pfm-cmd-plus2` filed zero focus verdicts** (its walk report has no `focusVerdicts` section at all) despite owning `pfm/internal/ui/model.go` (in S1's file list), `pfm/internal/installer/launcher.go` (S4, S5), `pfm/cmd/pfm/doctor.go` (S5, S6), and `pfm/cmd/pfm/statusline_command.go` (S7) — a named reporting hole, noted per row below.

| Fix | Owning units (disk) | Verdicts received | Verdict |
|---|---|---|---|
| **S1-picker-kill-rename** | u05(kill/manager.go), u02(kill/finisher.go, clear_kill_command.go), u07(kill/lineage.go, kill/self.go), u04(hide/doc.go), **u01(ui/model.go)** | u04:VERIFIED, u02:VERIFIED, u05:VERIFIED, u07:VERIFIED; **u01 silent** | **VERIFIED** (4/5 owning units) |
| **S2-limits-live-fetch** | u03(stats/limits.go), u02(statusline/render.go) | u02:VERIFIED, u03:VERIFIED | **VERIFIED** (2/2) |
| **S3-harvestpy-provision-publish** | u05(provision.go, check.go), u04(digest.go), u07(converter.go) | u04:VERIFIED, u05:VERIFIED, u07:VERIFIED | **VERIFIED** (3/3) |
| **S4-claude-launcher-tmux** | u05(launch_command.go, agent_open_command.go, launcher_repair_command.go), **u01(installer/launcher.go)** | u05:VERIFIED; **u01 silent**; u06/u03 correctly NOT_MINE | **VERIFIED** (1/2 owning units) |
| **S5-doctor-external-deps** | u05(deps/registry.go, deps/probe.go), **u01(doctor.go, installer/launcher.go)** | u05:VERIFIED; **u01 silent** | **VERIFIED** (1/2 owning units) |
| **S6-engine-roster-symmetry** | u03(compose.go, compose/types.go), u07(compose/order.go), u06(inject/engine.go), **u01(doctor.go)** | u06:VERIFIED, u03:VERIFIED, u07:VERIFIED **(UNBACKED — excluded)**; **u01 silent** | **VERIFIED** (2/4 owning units, after excluding the unbacked claim) |
| **S7-chat-status-ask-summary** | u02(headless/summary.go), u07(store/summary.go), u05(statusline/runtime.go), u04(statusline/process.go), **u01(statusline_command.go)** | u04:COULD_NOT_CHECK, u02:VERIFIED, u05:VERIFIED, u07:VERIFIED; **u01 silent** | **VERIFIED** (3/5 owning units) |

Representative quoted evidence:

- **S1** read-compat (u04/u05): `KillPrefix = "_KILL"` / `legacyKillPrefix = "_" + "HI" + "DE"` … `func LabelKilled(name string) bool { for _, prefix := range []string{KillPrefix, legacyKillPrefix} {` (`pfm/internal/naming/naming.go`) — zero residual `runHide/runUnhide/runHidden/openHideManager` callers confirmed by repo-wide grep in u02's walk.
- **S2** scope + absence (u02/u03): `if strings.TrimSpace(limit.Kind) != "weekly_scoped" || !strings.EqualFold(strings.TrimSpace(limit.Scope.Model.DisplayName), "Fable") || limit.Percent == nil { continue }` and `if named.Key != "seven_day_fable" || named.Window.Utilization == nil { continue }` (`usagehook/hook.go`, `statusline/render.go`) — an absent/non-Fable window never enters the map, so `seven_day_fable` never renders.
- **S3** final-path build, no rename (u04/u05/u07): `final := filepath.Join(envRoot, desired)` … `staging := final` … `// The environment is never renamed after uv sync; both smokes judge the same final runtime path.` (`harvestpy/provision.go`)
- **S4** session naming + missing-status error (u05): `func readLaunchStatus(path string) (int, error) { content, err := os.ReadFile(path); if err != nil { return 0, fmt.Errorf("launcher status file missing: %w", err) } }` (`launch_command.go`)
- **S5** AST guard fail-closed (u05): `if filesScanned == 0 { t.Fatal("dependency source guard scanned zero production Go files") }` (`deps/guard_test.go`)
- **S6** honest-empty roster (u03/u06): `includeNewClaude: input.Options.View != KilledView && len(input.AccountRoots) != 0` (`compose/compose.go` — **this exact evidence line is the one u07 cited without having opened the file; see Unbacked claims**)
- **S7** opt-in default + write guard (u02/u05): `withSummary := flags.Bool("summary", false, "summarize the last exchange")` (`headless_command.go`) and `if complete { if err := options.Database.PutChatSummary(...)` (`headless/summary.go`)

## Standing rulings

**HONEST-ABSENCE** (`rulings-w3-invariants.md:1`) — 6 RECONFIRMED, 1 **CONTRADICTED**:
- u04 RECONFIRMED — `deps/guard_test.go` fail-closed on zero-scan; `gather/codexproc.go` distinguishes a `PIDs()` error from a genuine empty result.
- u01 RECONFIRMED — `doctor.go`/`config.go`/`expected_hooks.go` distinct ok/missing/broken states.
- u06 RECONFIRMED — `reap/tmux.go` wraps the underlying exec error rather than swallowing it.
- u02 RECONFIRMED — `clear_kill_command.go`'s fail-open path logs the reason, never silently succeeds.
- u05 RECONFIRMED — `deps/probe.go` and `harvestpy/provision.go` both separate a genuine absence (`ErrNotExist`) from a look-failure.
- u07 RECONFIRMED — `gather/procfs_darwin.go` and `agentopen/agentopen.go` both refuse to treat a look-failure as "nothing found."
- **u03 CONTRADICTED** — `pfm/internal/reap/busy.go:45-63`: `if info, err := os.Stat(directory); err == nil && info.IsDir() { available = append(available, directory) }` folds any non-`ErrNotExist` `os.Stat` error into the same silent-drop bucket as a genuinely-absent directory, contradicting the file's own adjacent doc comment ("ANY failure fails the whole probe"). **This is the defect carrying the rulingRef** — see Should-have-changed above (severity high, pre-existing, not a regression of this wave).

**CONFIG-OWNERSHIP** (`rulings-w3-invariants.md:47`) — 7/7 RECONFIRMED (u04, u01, u06, u02, u03, u05, u07) — every unit's grep for hardcoded account-count/emoji/id literals across its owned files returned zero hits; account identity and badge emoji flow exclusively from `internal/config.DefaultEmoji`/roster lookups (representative: `pfm/internal/config/config.go:299-312` `func DefaultEmoji(id int) string { switch id { case 1: return "🥇" ... } }`, consumed by `ui/render.go`'s `accountMedal`/`codexAccountMedal`, never re-hardcoded).

**LEAK-LINE** (`rulings-w3-invariants.md:24`) — 1 RECONFIRMED (u06: new managed-launcher asset and `pfm.zsh` resolve every path through `$HOME` at runtime), 3 NOT_IN_MY_TREE (u01, u02, u03 — their owned files fall outside `blueprint/**`/`docs/**`/`installer/assets/**` territory), and **not addressed at all** by u04, u05, u07 (no LEAK-LINE entry in their `rulings` arrays).

**Additional wave-specific rulings (STATE.md ledger lines)** — all RECONFIRMED, reported by u05 and u07 alongside the three canonical rulings above:
- u05: "STATE.md:69 · picker-kill-rename read-compat" — `naming.go`'s dual-prefix `LabelKilled`.
- u05: "STATE.md:92 · harvestpy-provision-publish" — final-path build, INCOMPLETE marker, no rename (same evidence as S3 above).
- u05: "STATE.md:93 · claude-launcher-tmux" — pane-command status write + `wait-for` (same evidence as S4 above).
- u05: "STATE.md:96 · doctor-external-deps" — single registry, `tmux` MinVersion 1.8, guard-test coverage proof (same evidence as S5 above).
- u07: "STATE.md:69-ish · picker-kill-rename read-compat" — `store/lineage.go`'s physical `hidden` table left un-migrated by design, plus `naming.go`'s dual prefix.

## Unbacked claims

1 item — **EXCLUDED from the defect/should-have-changed counts and from the verdict below**, per the workflow's own marking:

- **u07-pfm-internal**, focusVerdict, "S6-engine-roster-symmetry VERIFIED" — reason: evidence cites `pfm/internal/compose/compose.go`, a file not in u07's `filesOpened`. (S6 is still independently VERIFIED by u06 and u03, both of whom did open the files they cited — see Fixes table above.)

## Verdict

**ROUGH SEAS.**

Rule applied: SHIPWRECK requires a fix with no legitimately-evidenced `VERIFIED` claim from any owning walker, a dead walker, or a critical defect that invalidates one of the seven fixes' own acceptance criteria — none of those hold (all 7 fixes verified by at least one owning unit per disk `plan.json`; 7/7 walkers reported). SMOOTH SAILING requires zero critical/high-severity defects, zero contradicted standing rulings, and no unopened owned files — none of those hold either: one **critical** regression defect exists (OAuth consent-page hidden field turned visible/editable, a real collateral-damage bug this wave's mechanical rename introduced, outside any single fix's own file boundary), one standing ruling is **CONTRADICTED** (HONEST-ABSENCE, pre-existing, backed by a high-severity should-have-changed finding), and 43 of 222 owned files were never opened by any walker.

**NAMED GAPS carried into this verdict, never silence:**
- Dead walkers: **none** (7 dispatched, 7 reported).
- Unopened owned files: **43 total** — 26 self-acknowledged budget skips (u01: 2, u02: 2, u05: 9, u07: 13) and **17 silent/unnamed** (u02: `pfm/internal/shared/shared.go`; u07: 16 test files across `action/`, `agentopen/`, `ask/`, `compose/`, `harvestpy/`, `headless/`, `installer/`, `kill/`, `mcpserv/`, `spawn/`, `stats/` — see Coverage above for the full list).
- **`u01-pfm-cmd-plus2` filed zero focus verdicts** despite owning files in 5 of 7 fixes' target lists (S1, S4, S5, S6, S7) — every one of those fixes still landed a `VERIFIED` from a different owning unit, so no fix is unverified, but u01's own ownership stake in those five fixes went unreported.
