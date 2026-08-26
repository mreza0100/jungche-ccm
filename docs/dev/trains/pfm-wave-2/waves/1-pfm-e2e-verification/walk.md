# Wave 1 — pfm e2e verification — walker report (branch mode)

**Branch:** train/pfm-wave-2 (HEAD 15f7270) vs main (da6101d) · **Worktree:** .worktrees/pfm-wave-2
**Scope:** #21 doctor jail tests (`pfm/cmd/pfm/doctor.go`, `doctor_jail_test.go`, `doctor_harvest_test.go`), #11-close e2e install harness (`pfm/e2e/install_e2e_test.go`, `pfm/e2e/doc.go`, `pfm/e2e/README`, `scripts/e2e-linux.sh`, `.github/workflows/install-verify.yml`, `pfm/TESTPLAN.md`).
**Spec:** docs/dev/trains/pfm-wave-2/waves/1-pfm-e2e-verification/spec.md

## Professor's Wave Review (Wave 1 · 2026-08-21 · ROUGH SEAS)

### Executive Summary

Eleven threads walked the branch (`train/pfm-wave-2`, HEAD `15f7270`) against the wave-1 charter (doctor jail honest-absence, e2e real-HOME refusal, previous-tag clone provenance, CI workflow scope). No thread is flatly broken end to end, and the fenced CI/devbox entry points (`scripts/e2e-linux.sh`, `.github/workflows/install-verify.yml`) are correctly and unconditionally isolated. But two of this wave's own headline safety claims do not hold against the code on disk, and neither was caught by the eleven-thread sweep — only a side-channel security audit and one dissenting thread found them:

1. **`pfmPathWarnings` (doctor's PATH-hijack detector) was narrowed to make its own new test pass.** In production (`resolved.Home` = the real `$HOME`), a `pfm` binary shadowing the canonical one anywhere outside `$HOME` on PATH (`/usr/local/bin`, a shared toolchain dir, …) is now skipped before it is ever stat'd or hashed — silently defeating the doctor's own documented contract ("checking command resolution alone is insufficient"). Locked in by a test named for the weakening itself (`TestPFMPathWarningsIgnoreHostShimsOutsideTargetHome`). **HIGH.**
2. **Two shipped docs (`pfm/e2e/README`, `pfm/TESTPLAN.md:529`) certify a safety property the e2e harness does not implement.** `refuseRealHome` is a tautology on Unix (`os.UserHomeDir() == os.Getenv("HOME")` verbatim), so it actually enforces "refuse unless `PFM_DEV_FENCE=1`", never a real-home identity check — and silently no-ops on empty/unset `HOME`. `sourceRepo()`'s "ready" fast path also breaks the "every state dir traces to `t.TempDir()`" guarantee both docs assert. **HIGH.**

Beyond these two, a systemic pattern recurs across four independent packages (config, statusline, hide, doctor) where a probe's own "could-not-run" state collapses into "nothing found" — the repo's own named defect class — and a second pattern (config-ownership: hardcoded account rosters / medal emojis / `.cc/N` literals) that this wave's own commit (`faf36c9`) declared fixed survives live in six other locations, one of which (the medal-emoji copies) has already visibly drifted.

Verdict is **ROUGH SEAS**, not SHIPWRECK: the only `BROKEN` thread (T11) collapsed on its harm claim under direct verification — the "unreviewed duplicate" commits are byte-identical, already-reviewed content from main, not new unreviewed work — and no data-corrupting or credential-leaking defect is live today. Fix the two HIGH items before S9; the rest can ride a follow-up train.

### Thread Walk

| Thread | Name | Flow (as walked) | Defects | Final Judgment Ruling |
|---|---|---:|---:|---|
| T1 | E2E harness refuses the invoking user's real HOME on every entry path | AT-RISK | 2 | **CONFIRMED**, both halves. `refuseRealHome` is a tautology (gates only on `PFM_DEV_FENCE`, silently no-ops on empty `HOME`); `sourceRepo()`'s ready-branch returns the live repo path, breaking the `t.TempDir()`-only invariant. Wins every file-level contradiction against T2/T9 on `install_e2e_test.go`. |
| T2 | Previous-release-tag phase clones from a staged fixture, never HEAD/bundle | INTACT | 0 | Its own narrow claim (charter Q2, tag provenance) is **SATISFIED** and correct — verified `git tag --list "v*"`, hard `t.Fatalf` on miss, zero `bundle` references. Its blanket file-level INTACT for `install_e2e_test.go` does not survive alongside T1's confirmed defects in the same file — same-file coexistence, not a wrong trace. |
| T3 | install-verify.yml CI trigger scope and no leaked machine path/secret | INTACT | 0 | **CONFIRMED clean** — `on:` exactly `{push:main, pull_request, workflow_dispatch}`, `permissions: contents: read`, zero `secrets.*`, `HOME=$(mktemp -d)` both jobs, actions pinned to semver. The apparent T1-vs-T3 contradiction is a file-attribution artifact: T1's defects live in `install_e2e_test.go`, not this file. |
| T4 | Doctor's new harvest/jail probes distinguish "failed to probe" from "nothing found" | AT-RISK | 2 | **CONFIRMED** — mechanism is correct at both surfaces (`printHarvestPythonDoctor`'s Lstat-skip, `crumbHealth`'s Stat-error split) but the exact branches that matter (a genuine probe failure, not mere absence) carry zero test coverage; `TestCrumbHealthUnreadablePathRemainsAnError` exercises the wrong branch despite its name. Charter Q4: mechanism satisfied, coverage not. |
| T5 | Config.AccountSkips — writer through every reader (CLI/UI/statusline) | INTACT | 0 | **VOID** for `pfm/internal/config/config.go`, `pfm/internal/statusline/render.go`, `pfm/internal/ui/render.go` — genuine, independently-confirmed defects (R9-INV-1, R9-INV-6, R9-INV-7, the `rateLimits.UnmarshalJSON` swallow) sit inside those files, outside the one field T5 traced. The `AccountSkips` plumbing itself remains sound on its own narrow claim. |
| T6 | install --skip-harvest prints the exact gate line, never touches the provisioner | AT-RISK | 1 | **VOIDED OUTRIGHT** — the flow itself is confirmed correct (fakes prove zero provisioner calls on the skip path); its one defect (missing TESTPLAN.md rows) was filed against `main`'s TESTPLAN.md, but the three rows already exist verbatim on the branch (`TESTPLAN.md:512-513`) and land with the merge. No action needed. |
| T7 | usagehook.DescribeWindows contract between statusline render and the hook's window-key canon | AT-RISK | 1 | **CONFIRMED** — `rateLimits.UnmarshalJSON` silently drops any key whose value fails to unmarshal (no log seam at all), a genuine regression versus the pre-wave plain-struct decode that failed loudly; a malformed known window becomes an indistinguishable, **persisted** false zero. |
| T8 | Account-roster/config-dir hardcodes removed in favor of internal/config; medal-emoji fallback re-verified | AT-RISK | 3 | **CONFIRMED**, all three: `mcpserv/backend.go` never reconciles `ClaudeRoots` (dormant, unreachable via any production caller today); the medal-emoji switch is tripled across `config.go`/`ui/render.go`/`statusline/render.go` and has already drifted (the UI copy is missing case 4); `statusline/render.go` still gates the Codex segment on the literal `account == 4` the wave's own `commands.go` fix just replaced elsewhere. Wins every contradiction against T5 on `config.go`, `statusline/render.go`, `ui/render.go`, `paths_test.go`. |
| T9 | Changed test files honor data/schema separation (no inline DDL, no migration-seeded fixtures) | INTACT | 0 | Criterion-scoped verdict is correct (zero DDL/migration coupling across all 18 files) but its blanket file-level sweep is not evidence of anything outside that one criterion — a single-criterion pass that emits a whole-file verdict is a coincidence detector. Its own affirmative answer to charter Q1 ("YES, `refuseRealHome` compares `$HOME` to `os.UserHomeDir()`…") is **VOID** — that comparison is the tautology T1 found. |
| T10 | Callers of the removed hardcoded account-roster/CodexCachePath logic | AT-RISK | 1 | **CONFIRMED** — `hide/finisher.go`'s own fallback silently defaults to zero `ClaudeRoots` (an absence-vs-failure conflation) instead of routing through `pfmconfig.Defaults`; dormant today because the sole production caller (`main.go:407-409`) always supplies both `Paths` and `ClaudeRoots` explicitly. |
| T11 | Branch carries unrelated "Limits tab" commits beyond wave 1's declared scope | BROKEN | 2 | **DOWNGRADED to AT-RISK.** Its Scope-disclosure defect stands (`walk.md`'s Scope line genuinely omits the Limits-tab files the branch also carries). Its HARM claim is **VOID** and disproven by direct diff: `git diff main..train/pfm-wave-2` over the named files is empty — the branch commits (`630fab1`/`b908c4d`/`15f7270`) are cherry-pick twins of already-merged, already-reviewed main commits (`faf36c9`/`a66c4cc`/`50a92de`, same author, same timestamp), so nothing lands unreviewed. Residual: no `STATE.md` ledger line records those three commits reaching the train branch. |

### Ledger Anomalies

**HONEST-ABSENCE**

| ID | Verdict | Sev | Expected | Got | Anchor |
|---|---|---|---|---|---|
| R9-INV-1 | CONFIRMED | med | A probe that could not run returns a distinguishable error, never "nothing found" — the ReadDir call 30 lines above already splits `os.ErrNotExist` from a genuine error. | `hasValidAccountCredentials` discards `os.ReadFile`'s error entirely (`if err != nil { return false }`) — absent, unreadable, and malformed-JSON credentials all collapse into the identical `AccountSkip{Reason:"no valid credentials"}`. An operator with a permission bug sees the same text as a never-authenticated account. | `pfm/internal/config/config.go:336-346`, surfaced at `pfm/internal/stats/limits.go:129-130` |
| R9-INV-5 | killed (holds) | — | — | Kill re-verified: credentials resolve from `options.ConfigDir` before `accountNumber` is ever read (`:169` before `:178`); the int only names the cache file/warn-flag/label. A wrong number cannot suppress a cap warning; the proposed fix would regress `~/.claude3` and collide it with account 1. | `pfm/internal/stats` Evaluate() |

**LEAK-LINE**

| ID | Verdict | Sev | Expected | Got | Anchor |
|---|---|---|---|---|---|
| R9-INV-3 | CONFIRMED | med | `scripts/leak-check.sh`'s PATTERN catches every personal-looking path class that ships in a tracked file. | The maintainer's real local project directory name is checked into two tracked docs, and PATTERN has no rule for generic `~/...` home-relative paths at all (only literal `{the literal user home path}` and `/Users/[A-Za-z0-9]`) — reproduced: the grep exits 1 (no match) against a `~/work/...` line. | `docs/dev/trains/queue/2026-08-21-limits-live-fetch.md:5`, `docs/dev/trains/pfm-wave-2/STATE.md:66`; gate at `scripts/leak-check.sh` |
| R9-INV-4 | CONFIRMED (reinstated) | low | CLAUDE.md: "Nothing identifying ships … in ANY TRACKED FILE. `scripts/leak-check.sh` … is the backstop, NOT THE PLAN — write it clean the first time." | 8 tracked lines (`STATE.md:46`, `train.md:61`, `waves/2-blueprint-framework/spec.md:26,32`, `queue/2026-08-20-config-ui-mcp-install.md:35,320,326,444`) still carry the source-project brand name — treating the pre-push hook firing as proof of compliance inverts the written rule. Nothing has leaked (the hook blocks the push), but the branch is currently unpushable, which is exactly when `--no-verify` gets typed. | Listed anchors above |
| R9-INV-2 | killed (holds) | — | — | Kill re-verified: the GitHub handle is the repo's own public masthead (`README.md:202`, `LICENSE:3`), registered as `{GH_USER}` in `scripts/placeholder-map.tsv:23` — PATTERN deliberately omits it. | n/a |

**CONFIG-OWNERSHIP**

| ID | Verdict | Sev | Expected | Got | Anchor |
|---|---|---|---|---|---|
| R9-INV-6 | CONFIRMED | med | `theme.Palette`'s own doc comment: "a palette change cannot leave a stray hardcoded color behind." | `headerStyle`'s background is hardcoded `"#5f3dc4"` inside `configureStyles(palette)` — the one function whose entire job is applying the operator's theme — and `Palette` has no field for it, so `pfm config set theme …` cannot recolor the header. | `pfm/internal/ui/render.go:23` and `:65` |
| R9-INV-7 | CONFIRMED | med | pfm/CLAUDE.md:57: "medal emoji … outside the config package is a defect." | `accountBadgeForID` (statusline) and `accountMedal` (ui) each re-implement `config.defaultEmoji()`'s id→emoji switch as a fallback; the two copies have already drifted — the ui copy is missing `case 4 → 🍀` that the statusline/config copies have, so the picker and the statusline disagree on the same account's default badge with no test catching it. | `pfm/internal/statusline/render.go:445-459`, `pfm/internal/ui/render.go:773-786` |
| R9-INV-8 | CONFIRMED | low | A hardcoded account count outside `internal/config` is a defect (same rule, reload's roster-validation path). | `reload.rosterContains` substitutes a hardcoded `[]int{1,2,3}` when the caller's `AccountIDs` is empty, reproducing config's roster policy outside `internal/config`. Reachable whenever config legitimately resolves to zero accounts (no error). | `pfm/internal/reload/reload.go:258-267` |
| R9-INV-9 | CONFIRMED | low | Same rule, agent-open's account roster. | `agentopen.New` falls back to a hardcoded 3-account roster with literal `filepath.Join(home, ".cc", "2"/"3")` when `dependencies.Accounts` is empty — hits both halves of the rule (hardcoded count AND a `.cc/N` literal) at once. | `pfm/internal/agentopen/agentopen.go:150-167` |

### Territory Digests

None.

### Security Audit (diff-scoped)

**8I — Improper access control / integrity**

- Expected: `pfmPathWarnings` hashes and compares every `pfm` binary reachable via PATH against the canonical one, per its own doc comment ("checking command resolution alone is insufficient").
  Got: a new unconditional `targetHome` filter `continue`s past any candidate resolving outside `resolved.Home` — the real `$HOME` in production — before it is ever stat'd or hashed, so a shadowing `pfm` anywhere outside `$HOME` on PATH is invisible to the check while the shell would still execute it. Locked in by a test named for the weakening. **MED** (`pfm/cmd/pfm/doctor.go:415, 436-439`, caller `:83`) — escalated to **HIGH** by the final judge as the wave's single most severe item.
- Expected: `scripts/e2e-linux.sh`'s docker invocation follows the same least-privilege/pinning discipline the wave already applies to GitHub Actions (`actions/checkout@v4.2.2` etc.).
  Got: floating `golang:1.24-bookworm` tag (no digest pin), full read-write bind mount of the host working tree with no functional need for write access (traced: the harness only ever copies into `t.TempDir()` or reads via `git tag --list`), root-level unpinned `apt-get install`. **LOW** (`scripts/e2e-linux.sh:14-25`).

**8A — Improper error handling / honest-absence**

- Expected: a doctor probe's broken/missing state is distinguishable from a deliberate operator choice.
  Got: `printHarvestPythonDoctor` treats an absent harvest-python root as a clean, non-warning "skipped" state without checking why it's absent — collapsing a deliberate `--skip-harvest` and an unexpectedly wiped/corrupted install into the identical rc-0 output. **LOW** (`pfm/cmd/pfm/doctor.go:293-296`).
- Expected: every dynamically-sourced string reaching the Limits TUI row passes through the diff's own new `cleanField()` escape-sanitizer, matching the guard just added for `Name`/`Status`.
  Got: `account.Label` is interpolated unsanitized — currently inert (every present-day producer returns a fixed string) but an incomplete instance of the guard the diff itself introduced. **LOW** (`pfm/internal/ui/render.go`, `renderStatsPanel`).

No CRITICAL or additional HIGH findings in the diff-scoped audit; full slice (43/38 files) opened by 4/4 auditors, `categoriesSwept` beyond the above: none.

### /jc Action Items

**High priority — before S9:**

1. `/jc` gate the `pfmPathWarnings` out-of-home skip behind the test/iso-jail signal only (`PFM_HOME`/`PFM_DEV_FENCE` set) so every PATH candidate is still stat'd and hashed on a real install; update `TestPFMPathWarningsIgnoreHostShimsOutsideTargetHome` to set that signal, and add its mirror asserting a shadowing out-of-home `pfm` DOES warn when the signal is absent. — `pfm/cmd/pfm/doctor.go:415,436-439`, caller `:83`; test `pfm/cmd/pfm/doctor_jail_test.go:64-88`
2. `/jc` rewrite `refuseRealHome` to gate solely and explicitly on `PFM_DEV_FENCE` with no empty-`HOME` early return (or resolve the real home independently of `$HOME` via `os/user.Current().HomeDir` if a genuine identity check is wanted); drop or opt-in-gate the `sourceRepositoryReady` fast path so `h.repo` is provably `TempDir`-scoped; then correct `pfm/e2e/README` and `TESTPLAN.md:529` to state the mechanism actually enforced. — `pfm/e2e/install_e2e_test.go:187-200, 222-228`; `pfm/e2e/README:7-9`; `pfm/TESTPLAN.md:529`

**Medium — this wave or immediate follow-up:**

3. `/jc` add `TestDoctorHarvestUnreadableRootIsNotSkipped` (non-`ErrNotExist` Lstat failure on the harvest root) and a genuine permission-denied `crumbHealth` case, watching both **fail** against a widened `err != nil` before/while fixing; rename `TestCrumbHealthUnreadablePathRemainsAnError` to match the branch it actually exercises. — `pfm/cmd/pfm/doctor.go:293, 512-514`; `pfm/cmd/pfm/doctor_harvest_test.go`, `doctor_jail_test.go:88-95`
4. `/jc` branch `hasValidAccountCredentials`'s `os.ReadFile`/unmarshal error so an unreadable or malformed credentials file returns a distinct reason (e.g. `"credentials unreadable: %v"`) instead of the fixed `"no valid credentials"` string. — `pfm/internal/config/config.go:336-346`
5. `/jc` add a Log seam to `statusline.Runtime` and log the raw unmarshal error per dropped key in `rateLimits.UnmarshalJSON` (or route per-key decode failures through the same `unknown[key]` label path) so a schema-drifted known window renders as a visible gap, not a false persisted zero. — `pfm/internal/statusline/render.go:91-103`, consumed at `:322-343, :536-558`
6. `/jc` export one config-owned badge helper (e.g. `config.DefaultEmoji(id int)`) and delete the `accountBadgeForID`/`accountMedal` switch duplicates in favor of it (missing lookup returns the honest `·`, never a re-derived medal); replace `statusline/render.go`'s `account == 4` gate with the same `Engine:"codex"` discriminator `commands.go` now uses. — `pfm/internal/config/config.go:256`; `pfm/internal/statusline/render.go:195, 207, 445-459`; `pfm/internal/ui/render.go:773-786`
7. `/jc` reassign `resolved.ClaudeRoots = machine.ProjectRoots()` in `mcpserv/backend.go:newBackend()` to match the `loadCommandRuntime` reconciliation pattern, or delete `newBackend()`/`mcpserv.New()` as dead code if `NewConfigured` is meant to be the only production entrypoint. — `pfm/internal/mcpserv/backend.go:30-40`
8. `/jc` route `hide.NewFinisher`'s fallback through `pfmconfig.Defaults(resolved.Home, resolved.ClaudeRoots).ProjectRoots()` instead of trusting bare `paths.Resolve()`, or fail loudly when both `Dependencies.Paths` and `Dependencies.ClaudeRoots` are empty rather than silently defaulting to zero roots. — `pfm/internal/hide/finisher.go:58-77`
9. `/jc` reload: make `rosterContains` fail closed — an empty accounts slice should return `false` (reject), never substitute a hardcoded 1-3 roster. — `pfm/internal/reload/reload.go:258-267`
10. `/jc` agentopen: stop synthesizing a hardcoded 1/2/3 roster on empty `Accounts` — return an error, or construct any needed default via `pfmconfig.DefaultAccountDir`, never a re-literaled `.cc/N` path. — `pfm/internal/agentopen/agentopen.go:150-167`
11. `/jc` add a Header background field to `theme.Palette` (e.g. `HeaderBg`), populate it per theme, and read it in `configureStyles` instead of the literal `"#5f3dc4"`. — `pfm/internal/ui/render.go:23, 65`
12. `/jc` rewrite `walk.md`'s Scope line from `git diff --stat main...train/pfm-wave-2` (the actual merge-base diff) so every touched file is disclosed; add a `STATE.md` ledger line naming the actor/timestamp for `630fab1`/`b908c4d`/`15f7270` reaching `train/pfm-wave-2`. — `docs/dev/trains/pfm-wave-2/waves/1-pfm-e2e-verification/walk.md:3-4`; `docs/dev/trains/pfm-wave-2/STATE.md:61-67`
13. `/jc` persist an explicit "harvest was skipped" marker (written by `--skip-harvest` at install time) and have the doctor check that marker instead of bare directory absence, so an unexpected disappearance of a previously-provisioned harvest-python root still surfaces as a warning. — `pfm/cmd/pfm/doctor.go:293-296`
14. `/jc` scrub the two `~/work/<project>`-style lines to a genericized path, and widen `leak-check.sh`'s PATTERN with a `~/...` home-relative alternative so this class of leak is caught mechanically. — `docs/dev/trains/queue/2026-08-21-limits-live-fetch.md:5`, `docs/dev/trains/pfm-wave-2/STATE.md:66`, `scripts/leak-check.sh`
15. `/jc` constrain single-criterion sweep threads (T9-style) to emit a criterion-scoped verdict string (e.g. `INTACT[test-data-discipline]`) that the aggregator cannot fold into whole-file clearance, and forbid any thread from answering a charter question using files outside its assigned scope — silence with a named gap only. (Walk-methodology fix, not a code fix.)

**Low / informational:**

16. `/jc` `source = cleanField(account.Label)` so the diff's new sanitizer covers every field the Limits row renders, not two of three. — `pfm/internal/ui/render.go` (`renderStatsPanel`)
17. `/jc` add `:ro` to the `-v` mount in `scripts/e2e-linux.sh` and pin the base image by digest (`golang:1.24-bookworm@sha256:<digest>`). — `scripts/e2e-linux.sh:14-25`
18. `/jc` note-in-comment: `PFM_E2E_HARVESTPY_GATE` in CI/script env is documentation-only (the test hardcodes its own copy, `e2eGateExpected` at `install_e2e_test.go:30`) — either delete the three CI/script copies or make the test read the var with `e2eGateExpected` as its fallback. — `.github/workflows/install-verify.yml:40,68`, `scripts/e2e-linux.sh:14`
19. Optionally scrub the 8 pre-existing source-project-brand lines before the next push attempt so the branch is push-ready (no functional fix required — `.githooks/pre-push` already blocks any leak). — `STATE.md:46`, `train.md:61`, `waves/2-blueprint-framework/spec.md:26,32`, `queue/2026-08-20-config-ui-mcp-install.md:35,320,326,444`

**No action (voided or resolved by merge):**

- T6's TESTPLAN.md-gap defect — the three rows already exist on the branch (`TESTPLAN.md:512-513`); lands with the merge, nothing to do.
- Duplicate-commit-lineage hygiene note (T11) — no functional fix pre-merge; optionally note in the eventual merge commit message that `630fab1`/`b908c4d`/`15f7270` are superseded-by-content duplicates of `faf36c9`/`a66c4cc`/`50a92de` so future `git blame` archaeology isn't misled.

### Coverage

- **Threads walked:** 11/11 reported (sync-dispatch reconciled: 11 dispatched, 11 received).
- **Ledger anomalies:** 9 → confirmed 7, false 2 (one reinstated on appeal: R9-INV-4), unproven 0.
- **Security audit:** 4 findings over 0 swept categories named; 43/38 files opened by 4/4 auditors, 0 unswept.
- **Invariants:** 3 registered, 3 armed — HONEST-ABSENCE, LEAK-LINE, CONFIG-OWNERSHIP.
- **Verdict contradictions:** 15 named, all resolved by the final judge (5 genuine, flagged seat wins every time; 10 not-contradictions — different criteria or misattributed files; 6 voided outright with zero unresolvable).
- **Named coverage-critic gaps (holes, not clean):**
  1. `pfm/internal/paths/paths.go` `ClaudeRoots` hardcode removal and its downstream consumers (`index.go`, `hide/finisher.go`, `reap/busy.go`, `archive.go`, `mcpserv/backend.go`, `doctor.go`, `chat_satellite_command.go`, `config_command.go`) — no hunter verified every caller gets `ClaudeRoots` re-populated before use; T10 later confirmed two of these callers (`mcpserv/backend.go`, `hide/finisher.go`) do NOT, closing part of this hole but not all of it (`reap/busy.go`, `archive.go`, `chat_satellite_command.go`, `config_command.go` remain unverified).
  2. `pfm/TESTPLAN.md` shows zero diff across the range that produced every other listed file's changes — no hunter pinned the exact commit range treated as "the diff"; either real TESTPLAN.md changes went unwalked or the changed-files list carries a stale no-op entry. Still open.
  3. `stats.go`'s new `AccountLimits.Engine/Label/Status` and `Window.ResetNote` fields, and whether the TUI Limits tab and the statusline renderer present skip/error/reset state consistently — T5 named only the narrower `Config.AccountSkips` slice of this territory; whether T7's separate statusline `DescribeWindows` path now diverges from the TUI's presentation was never checked. Still open.
  4. `statusline/refresh_gpt.go` + `runtime.go`'s new shared `GPTCachePath(jailHome, uid)` helper — pure mechanical dedup, not named by any thread; nobody confirmed the two call sites still resolve to the identical directory post-refactor. Still open.
  5. `pfm/internal/action/{config_fixture_test.go,executor_test.go,headless_test.go,stress_test.go,synth_test.go}`'s new hardcoded 3-account `testMachineConfig` test fixture — plausibly fixture-scoped plumbing T10 covers by proxy, but no hunter explicitly confirmed it doesn't reintroduce, inside the test suite, the exact account-count assumption CONFIG-OWNERSHIP hunts out of production code. Still open.
- **UNSENSED fields:** none (0 sensor fields scheduled this walk).
- **Gate sweep:** SKIPPED — no project profile supplied.
- **Named file-level contradictions and their ruling** (21 multi-seat files, 15 contradictions found — see Thread Walk table above for the per-thread ruling on each; listed here for the record): `.github/workflows/install-verify.yml` (T3 wins, file-attribution artifact), `docs/dev/trains/pfm-wave-2/STATE.md` (T11 half-right, downgraded), `pfm/cmd/pfm/commands.go` (T8/T10 defects live in adjacent files, commands.go itself clean — not a real contradiction), `pfm/cmd/pfm/doctor_harvest_test.go` (T4 wins), `pfm/cmd/pfm/doctor_jail_test.go` (T4 wins), `pfm/cmd/pfm/install_command_test.go` (T6 voided, not a contradiction), `pfm/e2e/install_e2e_test.go` (T1 wins over T2+T9), `pfm/internal/config/config.go` (T8/T10/T11 win over T5, T5 void here), `pfm/internal/installer/harvest_integration_test.go` (T6 voided), `pfm/internal/paths/paths_test.go` (T8 wins over T5), `pfm/internal/stats/limits.go` (T11 downgraded, not a real contradiction with T5 here), `pfm/internal/statusline/render.go` (T7+T8 win over T5, T5 void here), `pfm/internal/statusline/statusline_test.go` (T7 wins, different criterion), `pfm/internal/ui/render.go` (T8 wins over T5, T5 void here), `pfm/internal/usagehook/hook_test.go` (T7 wins, different criterion).

### Walk Telemetry

**Seats:**
- 2nd-opinion: 1 call(s)
- coverage-critic: 1 call(s)
- final-judge: 1 call(s)
- invariant-hunt: 3 call(s)
- judge: 2 call(s)
- scout: 1 call(s)
- security: 4 call(s)
- walk: 11 call(s)

**Invariant registry:** 3 registered, 3 armed (HONEST-ABSENCE, LEAK-LINE, CONFIG-OWNERSHIP)

**Coverage:** threads 11/11 · sensors 0/0 · hunters 3/3 (9 finding(s)) · digests 0/0
- gate sweep: SKIPPED — no project profile supplied
- coverage-critic gaps: pfm/internal/paths/paths.go ClaudeRoots hardcode removal and its downstream consumers (index.go, hide/finisher.go, reap/busy.go, archive.go, mcpserv/backend.go, doctor.go, chat_satellite_command.go, config_command.go) (Resolve() now returns nil claudeRoots instead of the old .cc/1,2,3/projects default, delegating discovery to config - the same class of fix CONFIG-OWNERSHIP hunts - but that hunter explicitly scoped only paths.go's fleet.db/.cc-ls-hidden literals as walked and never mentions this ClaudeRoots default removal or verifies every caller gets it re-populated via runtime_config.go before use.); pfm/TESTPLAN.md, listed in this walk's changed-files but showing zero diff across faf36c9~1..120c97f, the range that produced every other listed file's changes (No hunter's coverage note pins the exact commit range treated as 'the diff'; either the walk's true denominator differs from what the file list implies (meaning real TESTPLAN.md changes went unwalked) or the changed-files list carries a stale no-op entry nobody flagged.); stats.go's new AccountLimits.Engine/Label/Status and Window.ResetNote fields, and whether the TUI Limits tab and the statusline renderer present skip/error/reset state consistently (A new test exercises the writer(limits.go)-to-TUI-reader chain, but no thread names this field set as its own territory - T5 names only Config.AccountSkips, a narrower slice of the same Limits-tab renovation - and nobody checked whether T7's separate statusline DescribeWindows path needs the same richer status text or now diverges from the TUI's presentation.); statusline/refresh_gpt.go + runtime.go's new shared GPTCachePath(jailHome, uid) helper replacing two independently-inlined cache-path computations (Pure mechanical dedup, not named by any thread; nobody confirmed the two call sites (RefreshGPT's default CachePath, DefaultRuntime's rateDir) still resolve to the identical directory post-refactor, which sits directly upstream of T7's statusline seam without being claimed as part of it.); pfm/internal/action/{config_fixture_test.go,executor_test.go,headless_test.go,stress_test.go,synth_test.go} newly threading a hardcoded 3-account testMachineConfig fixture through action-package tests (Plausibly fixture-only plumbing that T10's dead-code-ripple territory covers by proxy, but no hunter explicitly confirmed this new test-only hardcoded roster stays fixture-scoped rather than reintroducing, inside the test suite, the exact account-count assumption CONFIG-OWNERSHIP is hunting out of production code.)
- verdict contradictions: 15 over 21 file(s) walked by 2+ seats
  - .github/workflows/install-verify.yml: T3-ci-workflow-scope-and-hygiene INTACT vs T1-e2e-real-home-refusal (AT-RISK) — escalated to the final judge
  - docs/dev/trains/pfm-wave-2/STATE.md: T2-e2e-previous-tag-source-staging INTACT vs T11-out-of-scope-limits-tab-commits (BROKEN) — escalated to the final judge
  - pfm/cmd/pfm/commands.go: T5-account-skips-field-readback INTACT vs T8-config-ownership-hardcode-removal (AT-RISK)/T10-dead-code-ripple (AT-RISK) — escalated to the final judge
  - pfm/cmd/pfm/doctor_harvest_test.go: T9-test-data-discipline INTACT vs T4-doctor-harvest-jail-honest-absence (AT-RISK) — escalated to the final judge
  - pfm/cmd/pfm/doctor_jail_test.go: T9-test-data-discipline INTACT vs T4-doctor-harvest-jail-honest-absence (AT-RISK) — escalated to the final judge
  - pfm/cmd/pfm/install_command_test.go: T9-test-data-discipline INTACT vs T6-skip-harvest-install-gate (AT-RISK) — escalated to the final judge
  - pfm/e2e/install_e2e_test.go: T2-e2e-previous-tag-source-staging/T9-test-data-discipline INTACT vs T1-e2e-real-home-refusal (AT-RISK) — escalated to the final judge
  - pfm/internal/config/config.go: T5-account-skips-field-readback INTACT vs T8-config-ownership-hardcode-removal (AT-RISK)/T10-dead-code-ripple (AT-RISK)/T11-out-of-scope-limits-tab-commits (BROKEN) — escalated to the final judge
  - pfm/internal/installer/harvest_integration_test.go: T9-test-data-discipline INTACT vs T6-skip-harvest-install-gate (AT-RISK) — escalated to the final judge
  - pfm/internal/paths/paths_test.go: T9-test-data-discipline INTACT vs T8-config-ownership-hardcode-removal (AT-RISK) — escalated to the final judge
  - pfm/internal/stats/limits.go: T5-account-skips-field-readback INTACT vs T11-out-of-scope-limits-tab-commits (BROKEN) — escalated to the final judge
  - pfm/internal/statusline/render.go: T5-account-skips-field-readback INTACT vs T7-usagehook-window-descriptor-seam (AT-RISK)/T8-config-ownership-hardcode-removal (AT-RISK) — escalated to the final judge
  - pfm/internal/statusline/statusline_test.go: T9-test-data-discipline INTACT vs T7-usagehook-window-descriptor-seam (AT-RISK) — escalated to the final judge
  - pfm/internal/ui/render.go: T5-account-skips-field-readback INTACT vs T8-config-ownership-hardcode-removal (AT-RISK) — escalated to the final judge
  - pfm/internal/usagehook/hook_test.go: T9-test-data-discipline INTACT vs T7-usagehook-window-descriptor-seam (AT-RISK) — escalated to the final judge

**Judgment:** confirmed 7 · false 2 · unproven 0 · 2nd-opinion: 2 dispatched, 0 overturned, 2 re-examined-killed · final judge: ROUGH SEAS, reinstated 1
