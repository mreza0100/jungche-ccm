# STATE — pfm-wave-2

## Resume brief

**Train:** `docs/dev/trains/pfm-wave-2/` · scheduled 2026-08-20 · revised 2026-08-20 (2nd scheduler pass) · base `main @ 7b01caa`
**Pipeline:** on `main`, no worktrees. `gitter` alone writes git.
**Seats:** builder-1 = Codex code seat (scope = wave 1's write paths) · builder-2 = main loop (`/pcm`-guarded only)
**Waves:** 1 `pfm-e2e-verification` (#20 → #21 → #22 → #11-close; specs `spec.md` + `spec-install-surface.md`, order in `ordering.md`) · 2 `blueprint-framework` (#13, #14-behind-R2, #16-held)
**Position:** next event is still **S1** (verify + commit the DONE span, excluding #11's five artifacts). Builder plan runs S1–S10 in `train.md`.
**#11:** BUILT, not green — all local e2e runs were compile-only; closes at S5 (real `scripts/e2e-linux.sh` docker run, watched) + S10 (CI green after a founder-ordered push; darwin is unverifiable locally).
**#20–#22:** anchors verified against the tree 2026-08-20 — all hold, no new RE-REFINE.
**Blocked on the user:** R1 (#18 refine) · R2 (`placeholder-map.tsv` → gates #14) · R3 (#16 refine) · R4 (#19 refine) · Q1 (parallel vs serial)
**Standing risk:** 143 files dirty, only commit today is `7b01caa` itself (02:44). The DONE span #1–#10, #12, #15, #17 plus #11's artifacts exist only in the working tree. S1 closes the span half.
**Held tasks are not dispatched** until the ruling lands: #14 (R2), #16 (R3). #18 and #19 are not in a wave at all.

Read `train.md` for the wave table, builder plan, anchor verdicts, and flag evidence. The ledger below
is the position — its last line is where the train stands. One line per event, append-only, no prose
reports.

<!-- LEDGER — append below this marker, never edit above it -->

- 2026-08-20 · scheduler · train written · waves 2 · tasks scheduled 3 (#11, #13, #14) · held 1 (#16) · re-refine 4 (F1 #18, F2 #14-plan, F3 #16, F4 #19)
- 2026-08-20 · scheduler · staleness clean — `7b01caa..main` empty, HEAD is the refined sha
- 2026-08-20 · scheduler · merge log empty — single queued spec
- 2026-08-20 · scheduler · verified DONE in tree: #1 #2 #3 #4 #5 #6 #7 #8 #9 #10 #12 #15 #17
- 2026-08-20 · scheduler · verified NOT started: #11 #13 #14 #16
- 2026-08-20 · scheduler · in-flight repairs verified CLOSED: ask_test fake adapters present, progress `## Final` present
- 2026-08-20 · scheduler · `go -C pfm build ./...` exit 0 · `go -C pfm vet ./...` exit 0
- 2026-08-20 · builder-1 · #11 RED compile gate · e2e skeleton failed as expected with undefined `runInstallE2E`
- 2026-08-20 · builder-1 · #11 implemented · added `pfm/e2e/{doc.go,README,install_e2e_test.go}`, `scripts/e2e-linux.sh`, `.github/workflows/install-verify.yml`
- 2026-08-20 · builder-1 · #11 gates · tagged e2e compile, `bash -n`, `go -C pfm vet ./...`, `go -C pfm test ./...`, and installed binary build passed; first full test run had one unrelated inject-jail flake, immediate rerun passed
- 2026-08-20 · builder-1 · #11 blocked deltas · production `runInstall` has no apply-time harvestpy offline/skip path and `doctor` cannot be clean in a fresh HOME; production exposes `pfm install --uninstall`, not the spec’s bare `pfm uninstall`
- 2026-08-20 · builder-1 · #11 final gate rerun · gofmt, tagged e2e compile, vet, full test, and installed binary build all passed after final surface assertions
- sentinel audit @2026-08-20 · 5 findings · top: orchestrator seat ran the pre-train brief (#11 out-of-scope by its own law) with a fail-closed inject channel — the train never had a live runner
- 2026-08-20 · founder · rulings: #11's three deltas become dependency tasks #20 (verb redesign) #21 (doctor clean) #22 (harvestpy skip) — queued as `queue/2026-08-20-pfm-install-surface-dependencies.md`; refine+scheduler law patched (precondition anchors / Anchors step) via /pcm; scheduler re-run ordered
- 2026-08-20 · scheduler · re-run (founder-ordered) · intake 1 QUEUED spec `2026-08-20-pfm-install-surface-dependencies.md` (#20 #21 #22)
- 2026-08-20 · scheduler · anchors #20–#22 grep/read-verified against tree · ALL HOLD · only drift: installer.go ModeUninstall cites 52/83/88, tree has 54/85/90 (premise intact) · no new RE-REFINE
- 2026-08-20 · scheduler · merge: #20–#22 folded INTO wave 1 (producer→consumer edge to #11 = one feature) · internal order #20→#21→#22→#11-close · refine-merge deliberately skipped: wave 1 partially executed, its spec body is the execution record
- 2026-08-20 · scheduler · staleness re-run clean — `7b01caa..main` still empty · tree divergence 143 dirty files incl. #11's five artifacts · only commit today is 7b01caa itself (02:44)
- 2026-08-20 · scheduler · verified: #11 BUILT not green (all local e2e runs compile-only `-run '^$'`) · e2e still calls `install --apply`/`install --uninstall` · reconciliation written to `waves/1-pfm-e2e-verification/ordering.md`, source specs untouched
- 2026-08-20 · scheduler · train revised in place: wave table, § Anchors, § Builder plan S1–S10 (S4 idle row dissolved, S1 now excludes #11 artifacts, S10 names the founder-push CI gap) · specs added `waves/1-pfm-e2e-verification/{spec-install-surface.md,ordering.md}` · queue spec stamped SCHEDULED · F1–F4 standing verbatim · rulings R1–R4 + Q1 still open
- 2026-08-20 · gitter · S1 DONE · span #1–#10 #12 #15 #17 + synth.go K1 hotfix committed @750375d (121 files, gates green, hash-verified) · framework/train @2d3692c (20 files, leak-check clean) · #11's five artifacts held for S9 · no push
- 2026-08-20 · main-loop · K1 hotfix context: picker ✦-new emitted bare claude/codex once config.json landed → naked `_professor-N` chats; fixed in synth.go, stale mise-shadowed pfm binary removed; shim restaging deferred — `pfm install --apply` aborts on corrupt harvestpy env (`~/.local/state/pfm/harvest-python/.../.venv/bin/python` missing), lands with #22's --skip-harvest
- 2026-08-20 · builder-1 · #20 DONE · files `pfm/cmd/pfm/{install_command.go,uninstall_command.go,main.go,install_command_test.go,mcp_serve_command.go,update_command.go}`, `pfm/e2e/install_e2e_test.go`, `README.md`, `INSTALL.md`, `pfm/TESTPLAN.md` · RED compile failure observed first; full `dev.sh test pfm`, tagged e2e compile (`GOFLAGS='-tags=e2e -run=^$'`), `dev.sh verify/build pfm`, installed binary build, and scoped retired-form greps passed · deviation: repo-wide legacy matches remain only in immutable specs/history and docs outside wave-1 write paths; zero retired install invocations/flags in wave-1 code, e2e, docs, TESTPLAN, workflow, script, or error strings
- 2026-08-20 · orchestrator-seat · PROFESSOR_ORCHESTRATOR live on `cx-1787254760-1211015-7721` (Full Access) · launch self-test PASSED (builder ACK) · #20 dispatched as sole in-flight goal
- 2026-08-20 · builder-2 · #13 DONE (uncommitted, awaits S9) · blueprint/CLAUDE.md ## Persona + facts-registry delta from the intuita section diff · blueprint/themes/{tokyo-night.json,README.md} · `dev.sh test blueprint` all steps passed · #14 waits on ruling R2
- 2026-08-20 · orchestrator · ESCALATION · #20 DONE cannot be accepted: behavior (e) requires every invocation/doc mention plus a repo-wide retired-form sweep, but current `docs/{README.md,SETUP.md,ARCHITECTURE.md}` and `docs/dev/pfm-surface.md` retain retired install forms outside wave-1 write paths; founder ruling required on scope versus acceptance
- 2026-08-20 · main-loop · #20 escalation RULED: accepted — out-of-fence docs fixed by main loop (four files, retired forms now zero); #20 committed @456fefd · #13+#16 @536c1bf · framework state @5947121 · main stabilized (user-ordered)
- 2026-08-20 · main-loop · FENCE stands (user-ordered, design docs/dev/isolated-dev-foundation.md APPROVED): infra/ pfm-dev container + dev.sh iso @7f71137 · worktree .worktrees/pfm-wave-2 on train/pfm-wave-2 · smoke green (iso build+verify, ro /worktree mount, fence proof line) · #11's five artifacts moved onto the train branch · code waves fenced, markdown on main, mirror only at gated close · hold lifted, #21 dispatch ordered
- 2026-08-20 · orchestrator · #20 ACCEPTED · user-authorized main-loop ruling: wave-1 fence was clean; owning seat updated `docs/{README.md,SETUP.md,ARCHITECTURE.md}` and `docs/dev/pfm-surface.md`; retired install-form grep across those files is zero · S3/#21 unblocked
- 2026-08-20 · orchestrator · HOLD (user-ordered) · no builder task in flight; last completed #20 is ACCEPTED and fully gated (`dev.sh test pfm`, tagged e2e compile, verify/build, installed-binary build, scoped retired-form greps, and owning-seat doc grep green); #21 was not dispatched · resume only when the main loop lifts the hold
