---
name: wave:walker
description: Wave walk that verifies the code works — one scout enumerates feature-flow/seam/invariant threads AND schedules sensors over the {API_PROTOCOL} fields+gates the wave touched; Sonnet walkers confirm each thread reaches its terminal state while a zero-token rule engine diffs the extracted field/gate cards for disconnects, encoding/casing mismatches, type drift, and auth-fence gaps; judges adjudicate only the flagged anomalies, and one final Opus judgment rules the whole walk before the review is written. Auto-invoked post-merge on main by /wave:orchestrator (§ O6, merge-SHA mode, concurrent with GATE-2) and post-commit by /wave:live; branch mode serves manual pre-merge walks. Also runs standalone code investigation (args.goal — any open code question) and orchestrator/schedule claim-verification panels (args.claims, args.manifestPath). Fast mode ("walker fast <mission>", "fast walk") — inline consumer-tree trace of any target (writers → every consumer → every hop → terminals) via one Sonnet lead + parallel Haiku tracers, no Workflow; § Fast mode. Triggers — "wave walker", "/wave:walker", "walker fast", "fast walk".
---

# Wave Walker — Thread Walk + Mechanical Ledger, One Fold

The Professor verifies the wave's code two ways in one pass, then folds them. Runs BEFORE archive — post-merge on `main` for `/wave:orchestrator` (concurrent with GATE-2), post-commit for `/wave:live`.

- **Thread walk (the floor)** — each feature flow / seam / field / schema change / invariant is walked **end-to-end** by its own fresh agent. This is the proven engine: the seams where real bugs hid — a happy path that never reached its terminal state, a field plumbed through three layers and fed by none, a partial index masquerading as a lock — are exactly what a focused per-thread walk catches and a single-pass read does not.
- **Ledger spine (the mechanical add)** — the same scout schedules Haiku sensors over the {API_PROTOCOL} type-fields + entry-point gates the diff touched; they extract comparable **cards**; a zero-token JavaScript rule engine diffs the cards for the defect classes a prose walk misses by construction — a field produced but consumed nowhere, a value stringified by the producer and indexed as an object by the consumer, a consumer comparing against `'ai_selected'` when the producer only writes `'AI_SUGGESTED'`, a {ROLE_USER}-reachable resolver missing its ownership fence. Only the **flagged** anomalies reach a judge — clean code costs almost nothing.

A diff with **no {API_PROTOCOL} surface** (an {AI_PROJECT}-chain wave, a migration-only wave) runs pure thread-walk — the floor never regresses.

**Read-only.** Static trace only — `git log`/`show`/`diff`, `Read`, `Grep`. No code runs, no DB writes, no edits (the fold writes only the review section). Confirming live behavior is `/qa:live`'s job; this confirms the code is wired to behave correctly.

## Entry points

All invoke the **`wave-walker` workflow** via `Workflow({ scriptPath: '{REPO_ROOT}/.claude/workflows/wave-walker.js', args })` — scriptPath, never `{name}`: name-lookup snapshots at session start and serves a stale copy in a long-running chat. Walk args: `{ reportPath, branch?, ledgerPath?, invariants?, debug?, debugPath?, charter?, extraThreads?, fullGateSweep?, securityFilesPerAuditor?, agents? }`:

- **Auto (`/wave:orchestrator` § O6, post-merge):** the BUILDER chat launches the scriptPath form with `{ reportPath }` as its first boundary duty (recording the run-id in STATE.md) — merge-SHA mode walks the wave's merge commit on `main` (the scout greps the report's `**Merge SHA:**` line) — persists the returned `ledger` to `ledgerPath` and the returned `debugRecord` to `debugPath` the same no-agent-ferries-bytes way, and the ORCHESTRATOR rules each finding into the boundary `/jc` lane (orchestrator § O6.2).
- **Invariant registry (`invariants`):** every walk-mode caller reads `.claude/commands/wave/walker-invariants.md` and transcribes its per-entry `**Law:**`/`**Territory:**`/`**Triggers:**`/`**Exemplars:**`/`**Hunt Brief:**` lines into the `{id, law, territory[], triggers[], exemplars[], huntBrief}` array its § Consumption Contract specifies, passed as `args.invariants` — mechanical transcription, no reinterpretation. Absent or `[]` = the floor: no hunters, walker behavior identical to a registry-less walk.
- **Walk telemetry (`debug`, `debugPath`):** `debug` defaults TRUE — the result carries a `debugRecord` (per-seat call/retry tallies, armed-invariant count, judgment counts) and the fold renders it as `### Walk Telemetry` in the review; `debug: false` restores the byte-identical quiet walk. `debugPath` names where the caller persists the record.
- **Auto (`/wave:live` W6, post-commit):** merge-SHA mode — the review file carries the JC commit SHAs.
- **Manual (`wave-walker {report-path}`):** the Professor calls the same workflow with that report path; adding `branch: '{branch}'` selects branch mode — a pre-merge walk of a live worktree (diffs `main...{branch}`).
- **Panel modes (no walk, no writes):** `args.claims` — the orchestrator's pre-ruling verifier panel (one Sonnet-xhigh refute-first verifier per claim × `votes`; per-claim `opus:true` = frontier-hands logic). `args.manifestPath` — MANIFEST-VERIFY (orchestrator § O2): a claim extractor mines the manifest's load-bearing claims breadth-first — ~4-6 per task, EVERY task covered before any goes deep (≤`maxClaims`, default 96); the panel probes each against code (panel ≤`soloThreshold` (8): one verifier per claim; larger: file-cluster batches of ≤4 claims, same Sonnet-xhigh refute-first bar), and a consistency judge flags cross-task conflicts, refuted premises, and freeloader tasks; returns `{ verdicts, consensus, conflicts, claimsMined, claimsVerified, droppedClaimIds, taskIds }` — the caller rules, folds corrections into `manifest-corrections.md`, and re-runs with a higher `maxClaims` when `droppedClaimIds` is non-empty.
- **Investigate (`args.goal`) — RR-for-code, any open code question:** lens probes (default DIRECT / SKEPTIC / BLAST-RADIUS; `lenses` overrides) seed a quote-pinned claim ledger; an Opus brainer steers ≤`maxWaves` waves of ≤`maxLanes` pursue/attack lanes; a Haiku auditor greps every quote-pin; claim status and answer confidence are **computed from ledger topology** (settled = audit-pass + ≥2 independent files + a survived challenge; contested = live counter-evidence), never asserted — the synthesiser's stated confidence may only be lower. Stop: brainer-done / 2 dry waves / wave-cap / budget. Knobs: `scope`, `probeModel/probeEffort`, `brainerModel/brainerEffort`, `auditModel`, `synthModel`, `reportOut` (cited report file). Degrades loudly (dead brainer/synth → best surviving deliverable, `degraded:true`), never silently. Walk-mode custom hooks — a caller shapes a unique walk from args alone: `charter` (free-text duty note; the scout shapes the thread manifest around it, walkers/digests/final judge answer it explicitly — always IN ADDITION to the standard duty, and the security seats never read it) · `extraThreads` (caller-forced threads appended verbatim to the scout's manifest, each `{id, type, name, verify, scope?, files?}` walked as its own thread) · `agents: {seat: {model?, effort?}}` (any tier on any of the 17 seats; unknown seat/tier throws, sub-opus on a frontier seat warns loudly).

## Fast mode — `walker fast <mission>`

An inline trace, minutes-scale, no Workflow: given any target (a table, a {API_PROTOCOL} field, a prompt
slot, a queue message, an API entry), map every writer and every consumer hop-by-hop to its
terminal. Deliverable: ONE consumer tree — file:line per node, fields per edge, quote-pinned
edges, terminals typed, closed-world coverage accounting. Raw map only — no recommendations,
no verdicts. Read-only, like every walk.

**Gear selection:** "where does X go / who feeds X / map it NOW" → fast mode. An open question
needing adjudicated evidence → investigate (`args.goal`). Post-merge wave verification → the full
walk. Fast mode's map may FEED a judgment; it never makes one.

Spawn ONE Sonnet lead (`subagent_type: general-purpose`) with the Lead prompt; it spawns Haiku
tracers per the protocol. Relay its map; persist to `tmp/walks/{slug}.md` when it outgrows chat.

### Lead prompt

Walk this code trace inline: "«mission»". Repo «root» («project list»). READ-ONLY —
Read/Grep/Glob/git inspection only; you and every agent you spawn never edit or write files.

THE SYNC-TREE LAW (absolute): dispatch ALL tracer Agent calls of a wave in ONE message and wait
for their results IN THIS TURN. Never background, never a "monitor", never end your turn while a
dispatched result is missing. A failed/empty dispatch = a COVERAGE HOLE named loudly (thread +
full bucket) — never silently absent, and never fabricate progress you have not received.

1. SCOUT + INVENTORY. Stamp git state (HEAD short-SHA + dirty-line count) into the report header
   — the repo may mutate mid-walk. Resolve the target's canonical anchors and emit its SPELLING
   SET — every spelling the edge wears (snake_case table, camelCase ORM symbol, type names, SQL
   and queue string literals, jsonb keys, {API_PROTOCOL} fields). Build the INVENTORY: grep -rln the
   mission's term set across every project (excluding tests/, generated/, node_modules/,
   **pycache**/, .claude/, docs/, tmp/) — every hit file is inventory; count test files per
   project (one line, never opened). Enumerate ≤8 THREADS (merge trivial; never split for count),
   classify hop types, assign EVERY inventory file to exactly one bucket — the inventory ends
   fully assigned. MANDATORY thread: any status/step/jsonb SIDE-CHANNEL the writers touch gets
   its own thread traced to ITS surfaces — a side-channel is a data path, not a footnote.
2. DISPATCH one HAIKU tracer per thread — all in ONE message, awaited in-turn — each carrying:
   mission, thread entry (file:line + hop type + recipe lines), depth=1, SPELLING SET, its
   bucket, and the trunk anchors so no tracer re-walks the trunk. Prefix every tracer's
   description with a short run slug (e.g. "ccrt-dive:T3 …") — token attribution slices runs by
   label, and colliding labels across runs merge their costs.
3. MOP-UP (one round, same sync law): reassign NOT-MINE files, dead tracers' buckets, and
   FRONTIER to fresh Haiku tracers; one Sonnet resolver takes the AMBIGUOUS set. A tracer that
   corrected an earlier claim: re-verify that tracer's SIBLING claims yourself — a correction
   replaces only the corrected claim. Then stop; the unwalked stays a named FRONTIER leaf.
4. MERGE. Dedupe converged nodes by anchor (file:symbol) — mark convergence. One consumer tree:
   writer → target → each reader → every hop → terminal (RENDERED-SURFACE | LLM-CONTEXT | EGRESS
   | DEAD-END | FRONTIER + resume grep). Duties: (a) every "consumed" claim NAMES its reader at
   file:line — a writer output with no named reader is a DEAD-END, said explicitly; (b)
   same-named types across projects: field-by-field diff or write "shapes NOT diffed" — never
   "consistent" from a glance; (c) every FE query document and every component in the inventory:
   mounted/imported-at file:line or DEAD-END-unmounted; (d) auth/guard chains are QUOTED per
   resolver — never "mirrors X". COVERAGE: git stamp · inventory accounting (assigned /
   dispositioned / leftover, per thread, tracer-verified vs lead-verified vs presence-grep-only)
   · terms + dirs · every hole and frontier named. TELEMETRY: tracers dispatched vs reports
   received (must reconcile), mop-up rounds, corrections.

### Tracer prompt

Trace ONE thread of «mission», depth «d» of 3. READ-ONLY. Budget ≤20 tool calls — beyond it,
return what you have, rest as FRONTIER. Entry: «file:line — hop type». Recipes: «lines».
SPELLING SET (grep every spelling before any absence claim): «spellings». Trunk (do NOT
re-walk): «anchors». BUCKET (every file returns with a disposition): «files».

1. EVERY edge quotes ≤2 verbatim lines of the code that makes it, with file:line — no edge
   without its quote. A docstring/comment mention is a LEAD, never an edge.
2. Disposition each bucket file: EDGE (quoted) | RED-HERRING (term present, no live edge — one
   line of proof) | NOT-MINE (return, don't walk) | FRONTIER (real edge, unfinished) |
   FAILED-TO-LOOK (a tool call failed — NEVER reported as "no edge").
3. Big files: grep with line numbers, then Read the matching RANGES — never "clean" from a
   truncated read.
4. An opened function or node: ALL branches (success/skip/fail/reuse) + EVERY side-effect write.
5. Cross-cutting property (audit/auth/logging/AI-marking/i18n) absent? Only after finding the
   repo's MECHANISM and checking its registry — else AMBIGUOUS, not absent.
6. Edges ride STRING LITERALS too (queue type strings, step keys, jsonb keys) — grep the
   literal, not just the symbol. Unpinnable (dynamic dispatch) → AMBIGUOUS with evidence.
7. Fan-out outside your bucket: 1-2 new entries → walk them yourself; 3+ and depth<3 → one HAIKU
   child each, ALL in ONE message awaited in-turn; at depth 3 → FRONTIER leaves.
8. RETURN raw material, never conclusions: subtree (quotes, fields per edge, children merged),
   candidate terminals with evidence, full bucket dispositions, Frontier, ONE coverage line.
   The lead rules dead ends; you report what you saw.

### Recipes (stamped per hop type — adopter anchors)

> Genericized anchors below — swap `{project}` and the per-project paths for YOUR OWN repo's
> layout at setup; the hop-type taxonomy and recipe SHAPE are what transfers, the paths are not.

| Hop type              | Recipe                                                                                                                                                                                                                             |
| ---------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| DB-TABLE               | table name + {ORM} symbol (`{project}/.../{ORM}/schema.ts`) + {AI_PROJECT} repository layer (`{AI_PROJECT}/src/**/db/`) — all spellings repo-wide; migrations `{project}/schema/{ORM}/*.sql`                                     |
| GRAPH-NODE             | factory + `add_node`/`add_edge` in your pipeline's graph-definition module (edges IN and OUT) + all branches + every step_status/jsonb write                                                                                      |
| SIDE-CHANNEL           | the column/key written → every reader of the key AND of the PAYLOAD inside it → their surfaces                                                                                                                                    |
| {API_PROTOCOL}-FIELD   | SDL → resolver (guard chain QUOTED) → query → FE query documents (`{project}/src/graphql/queries.ts` + `generated/`) → consumers; FE screens live in BOTH `{project}/app/` (file-based routes) AND `{project}/src/components/`   |
| PROMPT-SLOT            | content `{AI_PROJECT}/knowledge/prompts/**/*.md`; loader shims `{AI_PROJECT}/src/**/prompts/**` — BOTH before any claim; terminal LLM-CONTEXT, and a tool-using agent's reply may continue to a rendered chat surface — follow it |
| CROSS-CUTTING          | BE audit = the audit-log plugin's op-name map wired via its hook in the app entrypoint; BE auth = guard helpers + middleware; {AI_PROJECT} LLM = the LLM factory + its provider config; FE i18n = the locales directory           |
| {QUEUE}                | message TYPE string literal → publishers → consumer → downstream write                                                                                                                                                            |
| WS/SSE                 | event name → emit → subscriber → rendering component                                                                                                                                                                              |
| IMPORT-CALL            | symbol imports + call sites repo-wide, all spellings                                                                                                                                                                               |
| SEED-EXPORT            | `{project}/seeding/**` + export paths (round-trip)                                                                                                                                                                                 |

**Knobs:** threads ≤8 (default 6 — fewer, wider tracers cut wall-clock), tracer budget ≤20 tool
calls, depth ≤3, one mop-up round. Cost shape (measured): ~$5 full tree — Haiku tracers
~$0.10-0.15 each, the Sonnet LEAD is the cost center (cache-read growth over its tool calls), so
the token lever is a short lead context: the lead delegates instead of self-grepping and tracers
return compact raw material. ~15-20 min; the heavy engine remains the tool for adjudicated
verdicts.

## § Orchestration (the `wave-walker` workflow)

`.claude/workflows/wave-walker.js` runs this flow; **this section is its declared copy — update both together.** Every agent is read-only except the fold's review write. Input: the wave's `report.md` (grouping, SUCCEEDED merge SHAs, JC pre-flight).

1. **Scout (1 Sonnet)** — § Role: Scout. Merge-SHA mode: from the report's merge SHAs (a `**Merge SHA:**` line or the Final Summary table), `git diff {merge}^1 {merge}` per pipeline (+ `git show {sha}` per JC commit) → the changed-file set; an EMPTY changed-file set fails the walk fast — never a verdict over nothing. Branch mode (`args.branch`, manual): `git diff --name-only main...{branch}` — a pre-merge worktree diff, `mergeShas` empty. **File-set reconciliation:** the scout also returns `changedFileCount` — a separately-executed `wc -l` of the same name-only diff, never the length of its enumerated list; the engine reconciles the two (one corrective scout retry naming both numbers, then the walk FAILS) — every lens is scoped by the enumerated list, and a walk over an untrusted denominator never renders a verdict. Emits BOTH: (a) the **thread manifest**, and (b) the **ledger schedule** — the touched {API_PROTOCOL} operations, their deduped type-fields (SDL slice filled by the scout itself), file-locality-clustered sensor **jobs**, and the repo-wide **gate files**. No {API_PROTOCOL} surface → empty schedule, the threads carry the wave. The scout also extracts the **live** `{BACKEND_PROJECT}/CLAUDE.md` § Auth Pattern role-fences rule by heading grep (never line numbers) — R6 and the security second-opinion quote it; a checked fallback copy in the script (`AUTH_RULE_FALLBACK`) covers a failed extract and re-syncs on any § Auth Pattern edit.
2. **Walk + Sense + Hunt (parallel, one barrier)** — § Role: Walker. One **Sonnet walker** per thread returns the functional verdict + integration-delta hygiene. In the same barrier, one **invariantHunter** (Sonnet) per ARMED registry entry (armed = the scout's semantic trigger judgment ∪ the engine's zero-token territory-glob fail-safe over the changed-file set) hunts its territory refute-first — failure scenario REQUIRED per finding, pre-existing bugs in scope — and a **coverageCritic** names what no seat covered; both exist only when `args.invariants` is non-empty. **Haiku sensors** extract producer/consumer/writer **slices** per scheduled job (tier-escalating to Sonnet on structured-output death), per-file **gate sweeps** extract the guard chain of every resolver entry point — dispatched only when the diff touches gate-relevant surface (resolver/auth/service files, or any scheduled {API_PROTOCOL} field/job; one hit → the full repo-wide sweep; zero → skipped, Coverage reports `gates: SKIPPED (diff-scoped)`; `fullGateSweep: true` forces it), and **security auditors** (Sonnet, xhigh) run in EVERY walk, sweep skipped or not: the changed files cluster sorted into slices of ≤`securityFilesPerAuditor` (default 12), one auditor per slice applying `audit/security.md` (8A–8K) with the full changed set as cross-file context — {SENSITIVE_DATA}/auth/{API_PROTOCOL}/LLM deepest; only defects the diff introduced or worsened; each returns `filesOpened`/`filesSkipped`, and the engine merges the slices (findings concatenated, `categoriesSwept` intersected) so the headline always carries its denominator — files opened / in scope, every unopened changed file NAMED unswept, `null` only when every slice auditor died (AUDIT DIED, an explicit coverage hole). The script zips slices into cards mechanically (zero tokens).
3. **Ledger diff (the script, zero tokens)** — diffs the cards against the rule set: **R1** orphan producer, **R2** phantom consumer (incl. undeclared/fallback-chain reads), **R3** encoding mismatch (incl. the `JSON.parse(JSON.stringify(x))` double-encode regex), **R4** value-set / casing mismatch, **R5** base-type drift, **R6** gate-outlier + mandated-fence violation (quotes the scout's live § Auth Pattern extract), **R7** unfenced ID flow, **R8** dangling refs. Emits anomalies + honest coverage that names every unsensed field.
4. **Judge + Digest (parallel)** — **Sonnet judges** open both ends of each flagged anomaly — and the PRODUCER behind any claimed shape-fix (the middleware/service/emitter that emits the shape a consumer claims to handle, even outside the cited anchors; a test's fabricated envelope is never evidence) — and rule CONFIRMED / FALSE / UNPROVEN; invariantHunter findings enter this same judge path as the **R9-INV** rule class (survivors escalate like security kills); a killed **security (R6/R7) or near-certain (R3/R4)** verdict is auto-escalated to an **Opus** second opinion that can override. **Territory digests** (Sonnet) catch the un-mechanizable smells the rules and the walk can't see.
5. **Final judgment (1 Opus)** — the whole walk on one desk: thread walks, confirmed + unproven + KILLED verdicts (a wrong kill hides there — it may reinstate after opening the files), digests, security findings, coverage holes. Rules the **authoritative verdict** on the § Report Format scale and names the missed cross-cutting risks only the whole picture shows.
6. **Fold (1 Sonnet)** — § Report Format. Merges thread verdicts + confirmed anomalies + digest findings + security findings + coverageCritic holes + the final judgment (adopts its verdict verbatim; each missedRisk becomes an action item or needs-eyes line), dedups (a thread defect and a ledger anomaly at the same anchor are ONE item), writes `## Professor's Wave Review` into the report — including the `### Walk Telemetry` section when `debug` is on (the default), and returns `{ verdict, actionItems, review }`. The full `ledger` (incl. `security`) and the `debugRecord` travel in the workflow result; the caller persists them.

**Verdict contradictions (the script, zero tokens, between steps 2 and 5).** Each walker's verdict is paired to the files its thread spec names (`computeVerdictContradictions`); where two or more seats walked the SAME file and disagree — one INTACT, another AT-RISK or BROKEN — the pair is ESCALATED to the final judge (step 5) as a NAMED contradiction and carried into the fold's Coverage. Never averaged, merged, or settled by the more optimistic verdict: a clean verdict built on evidence the file does not contain reads exactly like an earned one, so the judge opens the file and names which seat is wrong — a verdict resting on invented evidence (a line count, a parity claim, text reported removed that is still there) is VOID, not a dissenting opinion. The scan states its own coverage on every walk, zero included: files compared, and every walked thread whose spec named no files — uncomparable, never counted as agreement.

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

**Reconciliation count** — return `changedFileCount`: the printed integer of a separately-executed `wc -l` over the same name-only diff(s) (merge-SHA mode: all diffs through one `sort -u | wc -l` pipe), never the length of the enumerated list. The engine fails the walk when list and count disagree — enumerate every file, no salience filtering, no truncation.

## § Role: Walker

Walk your one assigned thread end-to-end and confirm it is wired to behave as the spec intends. Read-only.

1. Read the thread spec, then the `files` it names.
2. **Trace it step by step** across every layer it crosses — feature flow: entry → each hop → terminal state; field: producer → transport → persist → **read-back** → surface (confirm the READER's field mapping carries the new field, not just the writer's); seam: emit side ↔ consume side; schema/db: migration ↔ schema ↔ app enforcement; invariant: each enforcement point; dead-code ripple: from each symbol the diff removed/renamed, grep callers/importers across the repo and file each newly-unreachable symbol; test-data discipline: scan changed `tests/` + migration files for schema DDL in test code, `.sql` fixtures under `tests/`, `readFileSync` of a numbered migration, or a test asserting on migration-seed rows instead of inline-inserted ones.
3. At **every** step ask: does this step produce what the next needs, and is the `verify` terminal state reached? Flag any break — a step the chain never calls, a field nothing feeds, a contract the two sides disagree on, an enforcement gap. Also name the concrete input/state under which this step corrupts, aborts, or lies — a failure scenario, not a vibe. Any two set-enumerations the flow assumes equal (a wipe set vs its snapshot set, a terminal-status set vs a poll loop's terminal set, a required-env list vs a validator's list) are diffed member-by-member. Apply the broken-mechanism test: what does this step report when it FAILS — the same as "nothing to do"? Flag it. A step claiming to HANDLE a shape it receives (a response envelope, an error body, a message payload) is verified against the code that EMITS that shape — open the producer and quote it; a test's fabricated envelope is never evidence the two sides agree.
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

{threads walked · fields sensed · UNSENSED fields named explicitly · gates swept — or `SKIPPED (diff-scoped)` when the diff touched no gate-relevant surface · hunters armed/dispatched with per-invariant finding counts · coverageCritic holes · named verdict contradictions with the final judge's ruling on each, over N files walked by 2+ seats · anomalies raised → confirmed/false/unproven · security findings over N categories swept everywhere, auditors returned/dispatched, files opened/in-scope, every UNSWEPT file named}

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
