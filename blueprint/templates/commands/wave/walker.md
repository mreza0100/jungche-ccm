---
name: wave:walker
description: Wave walk that verifies the code works — one scout enumerates feature-flow/seam/invariant threads AND schedules sensors over the {API_PROTOCOL} fields+gates the wave touched; Sonnet walkers confirm each thread reaches its terminal state while a zero-token rule engine diffs the extracted field/gate cards for disconnects, encoding/casing mismatches, type drift, and auth-fence gaps; judges adjudicate only the flagged anomalies, and one final Opus judgment rules the whole walk before the review is written. Auto-invoked post-merge on main by /wave:orchestrator (§ O6, merge-SHA mode, concurrent with GATE-2) and post-commit by /wave:live; branch mode serves manual pre-merge walks. Also runs standalone code investigation (args.goal — any open code question) and orchestrator/schedule claim-verification panels (args.claims, args.manifestPath). Triggers — "wave walker", "/wave:walker".
---

# Wave Walker — Thread Walk + Mechanical Ledger, One Fold

The Professor verifies the wave's code two ways in one pass, then folds them. Runs BEFORE archive — post-merge on `main` for `/wave:orchestrator` (concurrent with GATE-2), post-commit for `/wave:live`.

- **Thread walk (the floor)** — each feature flow / seam / field / schema change / invariant is walked **end-to-end** by its own fresh agent. This is the proven engine: the seams where real bugs hid — a happy path that never reached its terminal state, a field plumbed through three layers and fed by none, a partial index masquerading as a lock — are exactly what a focused per-thread walk catches and a single-pass read does not.
- **Ledger spine (the mechanical add)** — the same scout schedules Haiku sensors over the {API_PROTOCOL} type-fields + entry-point gates the diff touched; they extract comparable **cards**; a zero-token JavaScript rule engine diffs the cards for the defect classes a prose walk misses by construction — a field produced but consumed nowhere, a value stringified by the producer and indexed as an object by the consumer, a consumer comparing against `'ai_selected'` when the producer only writes `'AI_SUGGESTED'`, a {ROLE_USER}-reachable resolver missing its ownership fence. Only the **flagged** anomalies reach a judge — clean code costs almost nothing.

A diff with **no {API_PROTOCOL} surface** (an {AI_PROJECT}-chain wave, a migration-only wave) runs pure thread-walk — the floor never regresses.

**Read-only.** Static trace only — `git log`/`show`/`diff`, `Read`, `Grep`. No code runs, no DB writes, no edits (the fold writes only the review section). Confirming live behavior is `/qa:live`'s job; this confirms the code is wired to behave correctly.

## Entry points

All invoke the **`wave-walker` workflow** via `Workflow({ scriptPath: '{REPO_ROOT}/.claude/workflows/wave-walker.js', args })` — scriptPath, never `{name}`: name-lookup snapshots at session start and serves a stale copy in a long-running chat. Walk args: `{ reportPath, branch?, ledgerPath?, invariants?, debug?, debugPath?, charter?, extraThreads?, fullGateSweep?, agents? }`:

- **Auto (`/wave:orchestrator` § O6, post-merge):** the BUILDER chat launches the scriptPath form with `{ reportPath }` as its first boundary duty (recording the run-id in STATE.md) — merge-SHA mode walks the wave's merge commit on `main` (the scout greps the report's `**Merge SHA:**` line) — persists the returned `ledger` to `ledgerPath` and the returned `debugRecord` to `debugPath` the same no-agent-ferries-bytes way, and the ORCHESTRATOR rules each finding into the boundary `/jc` lane (orchestrator § O6.2).
- **Invariant registry (`invariants`):** every walk-mode caller reads `.claude/commands/wave/walker-invariants.md` and transcribes its per-entry `**Law:**`/`**Territory:**`/`**Triggers:**`/`**Exemplars:**`/`**Hunt Brief:**` lines into the `{id, law, territory[], triggers[], exemplars[], huntBrief}` array its § Consumption Contract specifies, passed as `args.invariants` — mechanical transcription, no reinterpretation. Absent or `[]` = the floor: no hunters, walker behavior identical to a registry-less walk.
- **Walk telemetry (`debug`, `debugPath`):** `debug` defaults TRUE — the result carries a `debugRecord` (per-seat call/retry tallies, armed-invariant count, judgment counts) and the fold renders it as `### Walk Telemetry` in the review; `debug: false` restores the byte-identical quiet walk. `debugPath` names where the caller persists the record.
- **Auto (`/wave:live` W6, post-commit):** merge-SHA mode — the review file carries the JC commit SHAs.
- **Manual (`wave-walker {report-path}`):** the Professor calls the same workflow with that report path; adding `branch: '{branch}'` selects branch mode — a pre-merge walk of a live worktree (diffs `main...{branch}`).
- **Panel modes (no walk, no writes):** `args.claims` — the orchestrator's pre-ruling verifier panel (one Sonnet-xhigh refute-first verifier per claim × `votes`; per-claim `opus:true` = frontier-hands logic). `args.manifestPath` — MANIFEST-VERIFY (orchestrator § O2): a claim extractor mines the manifest's load-bearing claims breadth-first — ~4-6 per task, EVERY task covered before any goes deep (≤`maxClaims`, default 96); the panel probes each against code (panel ≤`soloThreshold` (8): one verifier per claim; larger: file-cluster batches of ≤4 claims, same Sonnet-xhigh refute-first bar), and a consistency judge flags cross-task conflicts, refuted premises, and freeloader tasks; returns `{ verdicts, consensus, conflicts, claimsMined, claimsVerified, droppedClaimIds, taskIds }` — the caller rules, folds corrections into `manifest-corrections.md`, and re-runs with a higher `maxClaims` when `droppedClaimIds` is non-empty.
- **Investigate (`args.goal`) — RR-for-code, any open code question:** lens probes (default DIRECT / SKEPTIC / BLAST-RADIUS; `lenses` overrides) seed a quote-pinned claim ledger; an Opus brainer steers ≤`maxWaves` waves of ≤`maxLanes` pursue/attack lanes; a Haiku auditor greps every quote-pin; claim status and answer confidence are **computed from ledger topology** (settled = audit-pass + ≥2 independent files + a survived challenge; contested = live counter-evidence), never asserted — the synthesiser's stated confidence may only be lower. Stop: brainer-done / 2 dry waves / wave-cap / budget. Knobs: `scope`, `probeModel/probeEffort`, `brainerModel/brainerEffort`, `auditModel`, `synthModel`, `reportOut` (cited report file). Degrades loudly (dead brainer/synth → best surviving deliverable, `degraded:true`), never silently. Walk-mode custom hooks — a caller shapes a unique walk from args alone: `charter` (free-text duty note; the scout shapes the thread manifest around it, walkers/digests/final judge answer it explicitly — always IN ADDITION to the standard duty, and the security seats never read it) · `extraThreads` (caller-forced threads appended verbatim to the scout's manifest, each `{id, type, name, verify, scope?, files?}` walked as its own thread) · `agents: {seat: {model?, effort?}}` (any tier on any of the 17 seats; unknown seat/tier throws, sub-opus on a frontier seat warns loudly).

## § Orchestration (the `wave-walker` workflow)

`.claude/workflows/wave-walker.js` runs this flow; **this section is its declared copy — update both together.** Every agent is read-only except the fold's review write. Input: the wave's `report.md` (grouping, SUCCEEDED merge SHAs, JC pre-flight).

1. **Scout (1 Sonnet)** — § Role: Scout. Merge-SHA mode: from the report's merge SHAs (a `**Merge SHA:**` line or the Final Summary table), `git diff {merge}^1 {merge}` per pipeline (+ `git show {sha}` per JC commit) → the changed-file set; an EMPTY changed-file set fails the walk fast — never a verdict over nothing. Branch mode (`args.branch`, manual): `git diff --name-only main...{branch}` — a pre-merge worktree diff, `mergeShas` empty. Emits BOTH: (a) the **thread manifest**, and (b) the **ledger schedule** — the touched {API_PROTOCOL} operations, their deduped type-fields (SDL slice filled by the scout itself), file-locality-clustered sensor **jobs**, and the repo-wide **gate files**. No {API_PROTOCOL} surface → empty schedule, the threads carry the wave. The scout also extracts the **live** `{BACKEND_PROJECT}/CLAUDE.md` § Auth Pattern role-fences rule by heading grep (never line numbers) — R6 and the security second-opinion quote it; a checked fallback copy in the script (`AUTH_RULE_FALLBACK`) covers a failed extract and re-syncs on any § Auth Pattern edit.
2. **Walk + Sense + Hunt (parallel, one barrier)** — § Role: Walker. One **Sonnet walker** per thread returns the functional verdict + integration-delta hygiene. In the same barrier, one **invariantHunter** (Sonnet) per ARMED registry entry (armed = the scout's semantic trigger judgment ∪ the engine's zero-token territory-glob fail-safe over the changed-file set) hunts its territory refute-first — failure scenario REQUIRED per finding, pre-existing bugs in scope — and a **coverageCritic** names what no seat covered; both exist only when `args.invariants` is non-empty. **Haiku sensors** extract producer/consumer/writer **slices** per scheduled job (tier-escalating to Sonnet on structured-output death), per-file **gate sweeps** extract the guard chain of every resolver entry point — dispatched only when the diff touches gate-relevant surface (resolver/auth/service files, or any scheduled {API_PROTOCOL} field/job; one hit → the full repo-wide sweep; zero → skipped, Coverage reports `gates: SKIPPED (diff-scoped)`; `fullGateSweep: true` forces it), and one **security auditor** (Sonnet, xhigh) applies `audit/security.md` (8A–8K) diff-scoped in EVERY walk, sweep skipped or not — {SENSITIVE_DATA}/auth/{API_PROTOCOL}/LLM deepest; only defects the diff introduced or worsened. The script zips slices into cards mechanically (zero tokens).
3. **Ledger diff (the script, zero tokens)** — diffs the cards against the rule set: **R1** orphan producer, **R2** phantom consumer (incl. undeclared/fallback-chain reads), **R3** encoding mismatch (incl. the `JSON.parse(JSON.stringify(x))` double-encode regex), **R4** value-set / casing mismatch, **R5** base-type drift, **R6** gate-outlier + mandated-fence violation (quotes the scout's live § Auth Pattern extract), **R7** unfenced ID flow, **R8** dangling refs. Emits anomalies + honest coverage that names every unsensed field.
4. **Judge + Digest (parallel)** — **Sonnet judges** open both ends of each flagged anomaly and rule CONFIRMED / FALSE / UNPROVEN; invariantHunter findings enter this same judge path as the **R9-INV** rule class (survivors escalate like security kills); a killed **security (R6/R7) or near-certain (R3/R4)** verdict is auto-escalated to an **Opus** second opinion that can override. **Territory digests** (Sonnet) catch the un-mechanizable smells the rules and the walk can't see.
5. **Final judgment (1 Opus)** — the whole walk on one desk: thread walks, confirmed + unproven + KILLED verdicts (a wrong kill hides there — it may reinstate after opening the files), digests, security findings, coverage holes. Rules the **authoritative verdict** on the § Report Format scale and names the missed cross-cutting risks only the whole picture shows.
6. **Fold (1 Sonnet)** — § Report Format. Merges thread verdicts + confirmed anomalies + digest findings + security findings + coverageCritic holes + the final judgment (adopts its verdict verbatim; each missedRisk becomes an action item or needs-eyes line), dedups (a thread defect and a ledger anomaly at the same anchor are ONE item), writes `## Professor's Wave Review` into the report — including the `### Walk Telemetry` section when `debug` is on (the default), and returns `{ verdict, actionItems, review }`. The full `ledger` (incl. `security`) and the `debugRecord` travel in the workflow result; the caller persists them.

**Panel modes (no walk):** `args.claims` or `args.manifestPath` skip steps 1–6 entirely — see Entry points; a dead security auditor never sinks a walk, it becomes an explicit Coverage hole.

**Frontier seats** — the final judge (step 5), the second-opinion judge (step 4), and the investigate brainer default to the durable `opus` alias; a limited-time frontier model rides only the invocation args (`finalJudgeModel`, `securityEscalateModel`, `brainerModel`) per root `CLAUDE.md` § Model Selection — never a literal in this file or the script. Security/auth judgment seats never downgrade below `opus`.

## § Role: Scout

Enumerate BOTH the threads to walk AND the ledger schedule, from the wave's actual diff.

**Threads** — aim for **at least 4**; one per feature flow, plus a thread for each seam, field, schema change, or invariant the diff puts at risk. Merge trivial threads; never split for count. Every thread is one of:

| Type                     | Walk path                                                                                           |
| ------------------------ | ----------------------------------------------------------------------------------------------------- |
| **Feature flow**         | a user-facing capability — entry (UI/handler) → each hop → terminal state                           |
| **Seam**                 | a cross-project contract ({API_PROTOCOL} field, {REALTIME_PROTOCOL} channel, {QUEUE} message) — both sides agree |
| **Field**                | a new/changed persisted field — producer → transport → persist → read → surface                     |
| **Schema/DB**            | migrations + constraints — migration ↔ schema ↔ app-layer enforcement                               |
| **Invariant**            | a sacred {DOMAIN_ADJ}/safety rule — every enforcement point holds                                   |
| **Test-data discipline** | changed test + migration files honor the data/schema separation (root CLAUDE.md § Testing)          |
| **Dead-code ripple**     | trace each removed/renamed caller, deleted reference, or dropped field outward into unchanged files |

Always emit a **Test-data discipline** thread when the diff touches any `tests/` or migration file, a **Dead-code ripple** thread when the diff removes/renames a caller or drops a persisted field/column/route/file, and a **Field** thread with an explicit READ-BACK check for every NEW persisted field — the writer AND the reader mapping; a field that writes fine but reads back undefined is the archetypal silent kill (it passes every green gate).

**Ledger schedule** — only when the diff touches the {API_PROTOCOL} contract surface. Enumerate every field of each touched result type (deduped by `OwnerType.field`, SDL slice filled in yourself), cluster them by file locality into producer/consumer/writer sensor jobs each naming its exact files, and list every resolver file repo-wide for the gate sweep. Enumerate mechanically — completeness is the point; the rule engine is only as complete as this schedule.

## § Role: Walker

Walk your one assigned thread end-to-end and confirm it is wired to behave as the spec intends. Read-only.

1. Read the thread spec, then the `files` it names.
2. **Trace it step by step** across every layer it crosses — feature flow: entry → each hop → terminal state; field: producer → transport → persist → **read-back** → surface (confirm the READER's field mapping carries the new field, not just the writer's); seam: emit side ↔ consume side; schema/db: migration ↔ schema ↔ app enforcement; invariant: each enforcement point; dead-code ripple: from each symbol the diff removed/renamed, grep callers/importers across the repo and file each newly-unreachable symbol; test-data discipline: scan changed `tests/` + migration files for schema DDL in test code, `.sql` fixtures under `tests/`, `readFileSync` of a numbered migration, or a test asserting on migration-seed rows instead of inline-inserted ones.
3. At **every** step ask: does this step produce what the next needs, and is the `verify` terminal state reached? Flag any break — a step the chain never calls, a field nothing feeds, a contract the two sides disagree on, an enforcement gap. Also name the concrete input/state under which this step corrupts, aborts, or lies — a failure scenario, not a vibe. Any two set-enumerations the flow assumes equal (a wipe set vs its snapshot set, a terminal-status set vs a poll loop's terminal set, a required-env list vs a validator's list) are diffed member-by-member. Apply the broken-mechanism test: what does this step report when it FAILS — the same as "nothing to do"? Flag it.
4. **In the same pass**, run the integration-delta hygiene lens (`audit/code-hygiene.md` scope-`diff`): above all a repo-wide reuse-grep for a helper/type/hook the wave duplicated against pre-existing repo code (or against a sibling pipeline in merge-SHA mode), plus dead code the integration orphaned. Return these as `hygiene`, separate from functional `defects`.

Output per the `WALK` schema: `flow` (INTACT | AT-RISK | BROKEN | N/A), `trace` (marking where it breaks), `defects` (each `{what, location, jc}`), `hygiene` (each `{kind, where, detail, jc}`), `notes`.

## Report Format

The fold writes this into the report under `## Professor's Wave Review`:

```markdown
## Professor's Wave Review

**Wave:** {name} · **Date:** {date}
**Verdict:** {SMOOTH SAILING | MOSTLY GOOD | ROUGH SEAS | SHIPWRECK}

### Executive Summary

{2-3 sentences — the verdict and the findings that matter}

### Thread Walk

| Thread | Type | Flow | Defects | Notes |
| ------ | ---- | ---- | ------- | ----- |

### Ledger Anomalies (confirmed)

{grouped by rule; each with Expected/Got, anchors, severity. "None" if the ledger found nothing or the diff had no {API_PROTOCOL} surface.}

### Unproven

{ledger anomalies a judge could not verify either way — needs human eyes. "None" if clean.}

### Territory Digests

{one per touched territory — the un-mechanizable smells}

### Security Audit

{diff-scoped `audit/security.md` 8A–8K findings, per-category Expected/Got + severity. "None" if clean; a dead auditor = an explicit Coverage hole.}

### /jc Action Items

{Numbered — every functional defect + confirmed ledger anomaly + digest fix, deduped, each a verbatim `/jc {fix}`. Owner-tagged deferrals (/pm, /officer, founder) for non-code work. "None" if clean.}

### Coverage

{threads walked · fields sensed · UNSENSED fields named explicitly · gates swept — or `SKIPPED (diff-scoped)` when the diff touched no gate-relevant surface · hunters armed/dispatched with per-invariant finding counts · coverageCritic holes · anomalies raised → confirmed/false/unproven · security findings over N categories swept}

### Walk Telemetry

{debug default on — per-seat call/retry tallies, invariant registry armed count, judgment counts; omitted only when `debug: false`}
```

**Verdict scale:** SMOOTH SAILING (nothing) · MOSTLY GOOD (minor only) · ROUGH SEAS (a confirmed high or a BROKEN thread) · SHIPWRECK (a confirmed critical / security anomaly, or multiple broken flows). A smooth-running wave that merged a broken flow OR a confirmed critical anomaly is not SMOOTH SAILING.

## Rules

- **Read-only** — git inspection only; suggest `/jc` candidates, never run them.
- **No orphaned defects** — every fixable code finding (thread defect OR confirmed ledger anomaly OR digest fix) lands in `### /jc Action Items`. "Deferred" is owner-tagged non-code work only.
- **Honest coverage** — the Coverage note names every UNSENSED field as an explicit hole; never claim completeness beyond the data.
- **The floor never regresses** — if the ledger half finds nothing or the diff has no {API_PROTOCOL} surface, the thread walk still runs and carries the wave.
- After finishing: "Wave walk complete. {verdict}."
