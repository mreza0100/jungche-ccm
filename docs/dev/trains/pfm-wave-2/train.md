# Train: pfm-wave-2

**Scheduled:** 2026-08-20 · `main @ 7b01caa` · **Revised:** 2026-08-20 (second scheduler pass — #20–#22 folded into wave 1)
**Builders:** 2 — builder-1 (Codex code seat; scope = wave 1's write paths) · builder-2 (main loop, `/pcm`-guarded only)
**Source specs:** 2 — `docs/dev/trains/queue/2026-08-20-config-ui-mcp-install.md`,
`docs/dev/trains/queue/2026-08-20-pfm-install-surface-dependencies.md`
**Merge log:** one merge, executed at the train level — the queue spec's #20–#22 are #11's producer
dependencies (its e2e harness asserts exactly these surfaces; shared files:
`pfm/e2e/install_e2e_test.go`, `scripts/e2e-linux.sh`, `.github/workflows/install-verify.yml`,
`install_command.go` call forms). One feature → ONE wave. They join wave 1 with internal order
#20 → #21 → #22 → #11-close; no cross-wave dependency exists. A `/wave:refine` merge-mode rewrite was
deliberately NOT run: wave 1 is partially executed and its `spec.md` body is the byte-identical
execution record — rewriting it would falsify the ledger. The residual edge lives as task ordering in
`waves/1-pfm-e2e-verification/ordering.md`; both task bodies stay byte-identical to their sources.
**Pipeline:** this repo has **no worktree pipeline**. Every wave lands on `main`; the orchestrator's
SETUP/MERGE worktree steps do not apply here and `gitter` commits in place.

## Wave table

| # | wave | Touches | tasks | merged-from | flags |
| --- | --- | --- | --- | --- | --- |
| 1 | `pfm-e2e-verification` | pfm, docs (README/INSTALL), CI | #20 → #21 → #22 → #11 (close) | `2026-08-20-config-ui-mcp-install.md` (#11) + `2026-08-20-pfm-install-surface-dependencies.md` (#20–#22) | — |
| 2 | `blueprint-framework` | blueprint, .claude, docs | #13, #14 (R2-gated), **#16 (held)** | — | **F2** (#14 anchors), **F3** (#16 not conformant) |

Wave 1's spec files: `waves/1-pfm-e2e-verification/spec.md` (#11, unchanged) and
`waves/1-pfm-e2e-verification/spec-install-surface.md` (#20–#22); ordering in `ordering.md` beside them.

Independence holds after the merge: wave 1 writes `pfm/**`, `scripts/e2e-linux.sh`,
`.github/workflows/install-verify.yml`, `README.md`, `INSTALL.md`; wave 2 writes only `blueprint/**`,
`.claude/commands/wave/**`, `docs/PLACEHOLDERS.md`, `docs/SETUP.md`, `scripts/placeholder-map.tsv`.
Grep-verified 2026-08-20: `README.md` and `INSTALL.md` carry zero `{FOUNDER_NAME}` occurrences, so
#14's sweep never touches wave 1's doc files. No shared file, no shared symbol, no
producer→consumer edge between the waves.

## Numbering

Source task numbers are **preserved, not remapped**. Thirteen sibling tasks (#1–#10, #12, #15, #17)
already executed under those numbers and are recorded by number in
`docs/dev/trains/queue/2026-08-20-builder-progress.md` and in the source spec's cross-references
("Task #1's schema", "lands with Task #10"). Renumbering would strand that record. The train's number
space **is** the source's, extended — #18, #19 are taken by the F1/F4 findings; the install-surface
spec extends the same space with **#20–#22**. Grep-verified: zero stale `#N` references in any wave
spec — #20–#22's in-body references (#11, #20, #21, #22) all resolve inside this train.

## Source Reconciliation

| Queue file / task | Disposition |
| --- | --- |
| `2026-08-20-config-ui-mcp-install.md` #1 config v2 + `pfm config` | DONE — verified in tree (`pfm/internal/config/config.go`, `pfm/cmd/pfm/config_command.go`) |
| #2 theme engine | DONE — `pfm/internal/theme/theme.go` present |
| #3 account attribution | DONE — compose/statusline canonicalization in tree |
| #4 LS picker carousel | DONE — `ui` model/render/golden fixtures modified |
| #5 Darwin | DONE — `procfs_{linux,darwin,other}.go` added, `verify.yml` modified |
| #6 limits sub-tab + Fable statusline | DONE — `pfm/internal/stats/limits.go` + `usagehook/testdata/` |
| #7 `/swap`→`/reload` | DONE — `internal/swap/` deleted, `internal/reload/` added, `swap.command.md` deleted |
| #8 MCP HTTP daemon | DONE — bearer auth verified at `pfm/cmd/pfm/mcp_serve_command.go:68-70,98-106` |
| #9 harvester cache | DONE — `cache_roundtrip_test.go` added; no production fix needed |
| #10 installer completeness | DONE — verified `settings.go:25-28` (4 hook commands), `:52-53` (`cleanupPeriodDays` 36500), `installer.go:197-202` (`pfm codex agents`) |
| #11 e2e harness | **WAVE 1** — BUILT, not green: `pfm/e2e/{doc.go,README,install_e2e_test.go}`, `scripts/e2e-linux.sh`, `.github/workflows/install-verify.yml` exist in the tree; every local e2e run so far was compile-only (`-run '^$'`); closes at S5 (real local docker run) + S10 (CI, post-push). Call forms reconciled against #20's ruling in `waves/1-pfm-e2e-verification/ordering.md` |
| #12 `pfm update` + `pfm init` | DONE — `update_command.go`, `init_command.go`, `installer/update_metadata.go` |
| #13 blueprint persona / intuita diff / tokyo-night | **WAVE 2** — not started (`blueprint/themes/tokyo-night.json` absent) |
| #14 retire `{FOUNDER_NAME}` | **WAVE 2** — not started (84 occurrences across 18 files); flag **F2**, dispatches only after ruling R2 |
| #15 epic-inject hook | DONE — `epic_inject_command.go` + `store/migration_v5.sql` |
| #16 register the invariant | **WAVE 2, HELD** — not started; flag **F3** |
| #17 `internal/ask` contract | DONE — including the in-flight repair (see § In-flight, closed) |
| `2026-08-20-builder-brief.md` | RETAINED — the executed brief for the DONE span; superseded for new work by this train |
| `2026-08-20-builder-progress.md` | RETAINED — the evidence ledger the DONE dispositions above cite |
| Session finding 1 — read verbs write the store | **#18, RE-REFINE (F1)** — not scheduled |
| Session finding 2 — migration ran ahead of its binary | **#18, RE-REFINE (F1)** — same seam, same ruling |
| Session finding 3 — missing wave-protocol files | **#19, RE-REFINE (F4)** — not scheduled |
| Spec § RND (harvester prompt→GPT) | DEFERRED by the source spec — POC next wave, untouched here |
| Spec § Deferred (5 items) | DEFERRED by the source spec — untouched here |
| `2026-08-20-pfm-install-surface-dependencies.md` #20 verb redesign | **WAVE 1** (merged in) — scheduled, S2a |
| #21 doctor clean on fresh HOME | **WAVE 1** (merged in) — scheduled, S3 |
| #22 apply-time harvestpy skip | **WAVE 1** (merged in) — scheduled, S4 |

## In-flight, closed

The caller reported two repairs in flight. Both are **already landed in the tree** and need no seat:

1. **#17's spec test** — `pfm/internal/ask/ask_test.go` now defines `fakeTranscriptAdapter`
   (l. 19, span kind `turns`) and `fakeHarvesterAdapter` (l. 36, span kind `lines`), driven by
   `TestEvidenceStaysContentAgnosticForTranscriptAndHarvesterAdapters` (l. 91). No bare `Evidence`
   pair remains.
2. **Progress file final summary** — `## Final` exists with all four required items (files changed,
   behavioral changes, full `go test ./...` log, remaining mismatches).

Both fold into gate **S1** as a re-verify, not as work.

## Staleness

Re-run 2026-08-20 (second pass): `git diff --name-only 7b01caa..main` is **empty** — `HEAD` *is*
`7b01caa`. Neither source spec is stale against `main`; no task earns a staleness RE-REFINE.

The working-tree divergence is now **143 dirty files** — the DONE span (#1–#10, #12, #15, #17) plus
#11's five built artifacts — with the only commit today being `7b01caa` itself (02:44). Nothing from
this train has been committed. This remains the train's largest standing risk and is why S1 comes
first.

## Anchors — #20–#22, verified 2026-08-20 (second pass)

Every production surface the three task bodies rely on was grep/read-verified against the working
tree. **All hold; no RE-REFINE.**

- **#20:** `install_command.go:43` (`mode := installer.ModeDryRun`), `:28` (usage string),
  `:31-33` (`--apply`/`--uninstall`/`--dry-run` flags) — exact. No top-level `uninstall` verb
  (dispatch `main.go:82` registers `install` only; sole `"uninstall"` hit is the flag at
  `install_command.go:32`). `pfm help` = `printUsage` (`main.go:491`), dispatch + help both exist for
  (c). Call-site list (e) confirmed: `mcp_serve_command.go:130`, `update_command.go:231`,
  `install_e2e_test.go:110,129,132,144-146`, `README.md:164`, `INSTALL.md:45-149`; the mandated
  re-grep additionally catches `install_command_test.go:21` (`--dry-run`). No `usage: pfm install`
  golden exists — the conditional anchor resolves to "no golden". **Line drift, premise intact:**
  the spec cites `installer.go:52,83,88` for ModeUninstall gating; the tree has those statements at
  `:54,:85,:90` (+2). Not a flag.
- **#21:** `doctor.go:57` (`warnings := 0`) and hard-fail returns `:79,98,103,132` — exact. Exit
  contract confirmed: `warnings != 0` → print `doctor: warnings=%d` → `return 1` (`doctor.go:223-226`).
  `printHarvestPythonDoctor` at `:221` (call) / `:286` (def). Additional hard-fail returns exist at
  `:159,:164,:225,:261` — covered by behavior (b)'s enumeration duty, not an anchor break.
  `internal/paths` override pattern exists (file plan's jail-aware seam).
- **#22:** `install_command.go:55` (`ProvisionHarvest: true`) — exact. `installHarvestProvisioner` /
  `installer.NewHarvestProvisioner` at `install_command.go:16-23`; `Options.ProvisionHarvest` at
  `types.go:81`; provisioning gate sites `installer.go:335,:399` (where the skip-report line lands).
  All five #11 artifacts exist. Nuance recorded in `ordering.md`: the script/workflow never invoke
  `pfm install` directly — the Go harness does; and the harness today fails on ANY "harvestpy"
  output mention (`install_e2e_test.go:311-320`), so #22 (a)'s report line requires the harness edit
  already in #22's file plan.

## Builder plan

| step | seat | wave / task span | blocked-by |
| --- | --- | --- | --- |
| S1 | gate → `gitter` | verify + commit the completed span: `go -C pfm vet ./...`, `go -C pfm test ./...`, `go -C pfm build -o ~/.local/bin/pfm ./cmd/pfm`, hash-verify PATH resolution, then commit #1–#10, #12, #15, #17 — **excluding** the five #11 artifacts (`pfm/e2e/doc.go`, `pfm/e2e/README`, `pfm/e2e/install_e2e_test.go`, `scripts/e2e-linux.sh`, `.github/workflows/install-verify.yml`): committing the workflow now would arm a red CI job on the next push; they land at S9 with the surfaces they assert | — |
| S2a | builder-1 | wave 1 / #20 install/uninstall verb redesign — includes the `README.md`/`INSTALL.md` call-site sweep (seat scope = wave write paths; precedent: builder-1 already wrote `scripts/` and `.github/` in the DONE span) | S1 |
| S2b | builder-2 | wave 2 / #13; then #14 **only after ruling R2** (F2 stands — the `placeholder-map.tsv` question decides #14's file plan) | S1 |
| S3 | builder-1 | wave 1 / #21 doctor exits 0 on fresh install — acceptance is the e2e phase (a) assertion going red→green, watched | S2a |
| S4 | builder-1 | wave 1 / #22 `--skip-harvest` — needs #20's `--yes` to compose with | S3 |
| S5 | builder-1 | wave 1 / #11 close, local half — reconcile e2e call forms per `waves/1-pfm-e2e-verification/ordering.md`, then a REAL `scripts/e2e-linux.sh` docker run watched green (every prior local run was compile-only `-run '^$'` — that never counts) | S4 |
| S6 | builder-2 | wave 2 / #16 | S2b · **held on ruling R3** |
| S7 | gate → builder-1 | full suite + `scripts/leak-check.sh` over the combined diff (leak gate matters because wave 2 rewrites the placeholder table) | S5, S2b |
| S8 | gate → builder-2 | `/wave:walker` over the train diff — after #16 if R3 has landed (§ Registration Duty); if #16 is still held, the walker runs with that gap NAMED in its report, not silently absorbed | S6 (only if R3 landed), S7 |
| S9 | gate → `gitter` | commit wave 1 (#20–#22 + the five #11 artifacts) + wave 2 | S8 |
| S10 | **external — founder** | push on the founder's explicit ask only (publication law); `install-verify.yml` linux + darwin then run — **#11 fully closes only on that CI green**; the darwin job is unverifiable locally. Not schedulable by this train; stated so the gap is never rounded up | S9 |

**Sizing note.** The first pass's S4 idle row (builder-1 with no second pfm task) has dissolved:
#20–#22 fill builder-1's lane with three production tasks plus the #11 close, while builder-2 carries
#13 (+#14 behind R2, #16 behind R3). Builder-1 is now the long pole; builder-2's rulings backlog
(R2, R3) is the counterweight. If R2/R3 never land, builder-2 idles after #13 — stated, not absorbed.

**Parallelism.** S2a/S2b pack both seats because the two waves' write paths are provably disjoint
(re-verified after the merge — see § Wave table) and there is no worktree to serialize on. This
departs from the orchestrator's one-wave-at-a-time O2 loop, which assumes worktree isolation this
repo does not have — see question Q1.

## RE-REFINE flags

Anchors re-run 2026-08-20 over #20–#22: **all hold — no new flags.** The four standing flags are
unchanged and their rulings remain open:

| id | target | why |
| --- | --- | --- |
| F1 | **#18** — store open/migrate seam (session findings 1 + 2) | not scheduled; needs refine |
| F2 | **#14** file plan | three anchors do not hold; one required file missing from the plan |
| F3 | **#16** | one of five mandated registry fields specified |
| F4 | **#19** — missing wave-protocol files (session finding 3) | not scheduled; needs refine |

### F1 — #18, the store open/migrate seam

Findings 1 and 2 are **one seam and one decision**, verified at
`pfm/internal/store/store.go`:

- `Open` → `OpenContext` → `migrate(ctx)` unconditionally (l. 122). There is no read-only open option.
- `migrate` runs inside `WithImmediateTx` (l. 172) — a write transaction on `fleet.db` on *every*
  command that touches the store, read verbs included. That is finding 1: `chat status`, `chat capture`,
  `chat read` need write access, so a least-privilege sandbox refuses them.
- `migrate` refuses a store newer than the binary (ll. 177–183, "database schema version %d is newer
  than supported version %d"). `SchemaVersion = 5` (l. 23) with `migration_v5.sql` shipped by #15.
  That is finding 2: any binary carrying v5 that touched the live DB migrated it, and every
  not-yet-updated binary on the box then hit ll. 177–183 — fleet-wide outage.

The undecided design is the same for both: **who is allowed to migrate the live store, and what does
everyone else do instead.** At least three shapes exist — gate migration to `pfm install` / `pfm update`
only; require the migrating binary to be the ledger-owned canonical copy; make migration explicit
(`pfm store migrate`) and have every other path open read-only. Each answers finding 2 differently and
changes what finding 1's read-only path means under version lag, and each has a different story for the
shared half (`shared.Store`, l. 147). Also unresolved: which verbs are "read verbs", and what a
read-only open reports when the on-disk version is older than the binary.

Not schedulable at zero-gap. Filing it as a task would mean the scheduler picking the architecture.

**Ruling needed (R1):** send #18 to `/wave:refine`. It is the highest-severity finding in this set —
finding 2 already caused a fleet-wide outage and the seam is unchanged.

### F2 — #14 file plan

Full evidence in `waves/2-blueprint-framework/spec.md` § F2. Summary: the plan's "22-file set" is
18 files / 84 occurrences; `blueprint/scripts/build-codex.mjs` and `docs/SETUP.md` carry zero
occurrences; and `scripts/placeholder-map.tsv` — the leak gate's substitution table, tracked and
explicitly excluded by `scripts/leak-check.sh:48,100,108` — is absent from the plan while holding six
`{FOUNDER_NAME}` target rows. **Ruling R2** decides that file's fate.

### F3 — #16 registry conformance

Full evidence in `waves/2-blueprint-framework/spec.md` § F3. **Ruling R3.** #16 is held.

### F4 — #19, missing wave-protocol files

Confirmed: `.claude/commands/wave/` holds `live.md`, `refine.md`, `walker-invariants.md`, `walker.md`.
`blueprint/commands/wave/` additionally templates `builder.md`, `orchestrator.md`, `sentinel.md` —
none installed. (`.claude/agents/scheduler.md` was installed by the caller and is excluded.)

It is not a copy job. Every installed sibling **differs substantively** from its blueprint source —
`refine.md` by 106 diff lines, `walker-invariants.md` by 174, `live.md` by 77 — and the deltas are
design, not placeholder resolution: the worktree pipeline is written out (`live.md`: "This install has
no worktree pipeline: every task lands on `main`"), the real agent roster is written in, the blueprint
persona block is dropped, and paths become repo-relative.

Per file:

- **`builder.md`** (34 lines, placeholders: `{N}` ×2) — thin adaptation; near-schedulable.
- **`orchestrator.md`** (45 lines, `{FOUNDER_NAME}` ×11, `{N}` ×4) — its entire § O2 is worktree-shaped
  ("gitter cuts the worktree", "MERGE — gitter merges and removes the worktree") and its hard law reads
  "ONE WAVE = ONE WORKTREE = ONE MERGE". Restating that law for a repo with no worktrees is a design
  decision no one has made.
- **`sentinel.md`** (36 lines, `{FOUNDER_NAME}` ×5, `{SENSITIVE_DATA}` ×1) — this repo's
  sensitive-data class is undefined.

**Ruling needed (R4):** refine #19 — or split it, scheduling `builder.md` now and refining the other
two. Note the ordering consequence either way: whatever lands in `.claude/commands/wave/` carrying
`{FOUNDER_NAME}` must land **before** #14, whose gate is a repo-wide zero-match grep.

**Impact while unscheduled:** `/wave:orchestrator` is not installed, so this train has no installed
runner — the main loop drives it by hand, as it did for the DONE span via
`2026-08-20-builder-brief.md`. `/wave:builder` is likewise absent, so builder-1's goal cannot cite it;
a written brief substitutes. `/wave:sentinel` is unavailable as an audit route. None of this blocks
the train; all of it is manual.

## Questions only the user can settle

- **Q1 — parallel or serial?** S2a/S2b run both waves at once on `main`. Their write paths are disjoint
  and there is no worktree to isolate them, but `/wave:orchestrator`'s law is one wave at a time.
  Parallel, or serialize wave 1 → wave 2 and accept builder-2 idling through S2?
- **Q2 — `scripts/placeholder-map.tsv` (ruling R2, and wider).** The file is **tracked** and holds the
  real name and email in its search column (rows 9–14), carved out of `scripts/leak-check.sh` at three
  places. The repo's own LEAK-LINE law reads "Nothing identifying ships… no founder PII… in any tracked
  file." Two answers wanted: is the carve-out intentional, and what does #14 do to those rows —
  delete, retarget to `the user`, or leave?
- **Q3 — rulings R1/R3/R4.** Send #18, #16, and #19 to `/wave:refine`, or overrule a flag and schedule
  it as written?
