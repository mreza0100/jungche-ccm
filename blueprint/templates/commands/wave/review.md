---
name: wave:review
description: Post-wave review — fans out one agent per thread (feature flow, integration seam, data field, schema/DB, invariant) to walk the wave's merged code end-to-end and confirm it functionally works, then a synthesizer folds in the operational review (grouping, QA, parallelism) and writes the verdict. Auto-invoked by /wave after all pipelines complete. Triggers: "wave review", "/wave:review".
---

# Wave Review — Fan-Out Thread Walk + Operations Review

The Professor walks the wave's merged code by fanning out one agent per thread, then reviews how the wave ran. Runs BEFORE archive.

Each thread is walked **end-to-end, step by step**, by its own fresh agent — the seams where a wave's real bugs hide (a happy path that never reached its terminal state, a field plumbed through three layers and fed by none, a partial index masquerading as a lock) are exactly the seams a focused per-thread walk catches and a single-pass read does not.

**Read-only.** Static trace only — `git log`/`show`/`diff`, `Read`, `Grep`. No code runs, no DB writes, no edits. Confirming live behavior is `/qa`'s job; this confirms the code is wired to behave correctly.

## Entry points

Both entry points invoke the **`wave-review` workflow** (`.claude/workflows/wave-review.js`) with `args: { reportPath }`:

- **Auto (`/wave` Step 3):** the wave runner calls `Workflow({ name: 'wave-review', args: { reportPath } })`.
- **Manual (`wave-review {report-path}`):** the Professor calls the same workflow with that report path.

The workflow is the dispatcher; every act of cognition (enumerate, walk, synthesize) runs in a clean leaf agent that does not spawn further. The caller only invokes it and presents the returned review.

## § Orchestration (the `wave-review` workflow)

`.claude/workflows/wave-review.js` runs this flow; **this section is its declared copy — update both together.** Every agent is `opus`, a leaf node, and read-only except the synthesizer's review write. Input: the wave's `report.md` (grouping, results, merge SHAs, JC pre-flight).

1. **Scout (1 agent)** — § Role: Scout. Enumerate the threads from `report.md`; return the manifest plus the integrated changed-file set and merge SHAs.
2. **Walk (one agent per thread, parallel)** — § Role: Walker. Each walker reads its thread's files **once** and returns BOTH the functional verdict AND the code-hygiene findings — the two lenses share one read. This is how `/audit:code-hygiene` is wired in: not a separate sweep (Step 7 already audited each pipeline's diff pre-merge) but the integration delta, folded into the walk.
3. **Synthesize (1 agent)** — § Role: Synthesizer. Run the operational review, fold every walker's defects and hygiene findings into `### /jc Action Items`, consolidate cross-pipeline duplication, write `## Professor's Wave Review` into the report, and return `{ verdict, actionItems, review }`.

The caller presents the returned review to the user and drains `### /jc Action Items` (`/wave` Step 3.4).

## § Role: Scout

Enumerate the threads to walk from the wave's actual diff. Aim for **at least 4**; right-size to the wave — one thread per feature flow, plus a thread for each seam, field, schema change, or invariant that the diff puts at risk. Merge trivial threads; never split for the sake of count.

1. From the report's Final Summary / Grouping Summary / `## JC Pre-flight`: list SUCCEEDED pipeline merge SHAs and any JC commits. Note FAILED/DEFERRED pipelines as not-merged.
2. `git diff {merge}^1 {merge}` per pipeline (`git show {sha}` for a JC fix) → build the changed-and-generated file set.
3. Read `.claude/commands/p/360.md` and run it inline, domain `inquiry`, subject "wave {name} — {N} tasks across {M} pipelines", to surface blind-spot dimensions (sequences, contradictions, missing info, state, auth). Each dimension that touches the diff must be covered by a thread.

**Thread taxonomy** — every thread is one of:

| Type                     | Walk path                                                                                                                                                                      |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Feature flow**         | a user-facing capability — entry (UI/handler) → each hop → terminal state                                                                                                      |
| **Seam**                 | a cross-project contract ({API_PROTOCOL} field, {REALTIME_PROTOCOL} channel, {QUEUE} message) — both sides agree                                                              |
| **Field**                | a new/changed persisted field — producer → transport → persist → read → surface                                                                                                |
| **Schema/DB**            | migrations + constraints — migration ↔ schema ↔ app-layer enforcement                                                                                                          |
| **Invariant**            | a sacred {DOMAIN_ADJ}/safety rule — every enforcement point holds                                                                                                              |
| **Test-data discipline** | the wave's changed test + migration files honor the data/schema separation (root CLAUDE.md § Testing & Environment)                                                            |
| **Dead-code ripple**     | code the wave orphaned — trace each removed/renamed caller, deleted reference, or dropped field/route outward into unchanged files to surface what the change left unreachable |

Always emit a **Test-data discipline** thread when the diff touches any `tests/` or migration file — it scans for the four anti-patterns the Walker checks below.

Always emit a **Dead-code ripple** thread when the diff removes or renames a caller, deletes a reference, or drops a persisted field, column, route, file, or the last import of a dependency — any change that can orphan code or a package elsewhere. It is the one thread licensed to leave the diff: it follows only the call/reference graph the change perturbed, never a full-repo sweep (that is `/audit:code-hygiene`).

Output **only** the manifest — one entry per thread:

```
- id: T1
  type: feature flow | seam | field | schema/db | invariant | test-data discipline
  name: {short name}
  scope: {one line — what this thread covers}
  files: {key paths the walker should start from}
  verify: {what "works" means for this thread — the terminal state or contract to confirm}
```

## § Role: Walker

Walk your one assigned thread end-to-end and confirm it is wired to behave as the spec intends. Read-only: `git log`/`show`/`diff`, `Read`, `Grep`.

1. Read the thread spec, then the `files` it names.
2. **Trace it step by step** across every layer it crosses — feature flow: entry → each hop → terminal state; field: producer → transport → persist → read → surface; seam: emit side ↔ consume side; schema/db: migration ↔ schema ↔ app enforcement; invariant: each enforcement point; dead-code ripple: from each symbol the diff removed, renamed, or dropped, grep its callers, importers, and references across the whole repo — when the wave deleted the last caller of an unchanged function, export, field, route, or file, that target is now orphaned; trace one hop further (do the orphan's own dependencies fall dead too?) and file each newly-unreachable symbol as a `/jc` defect — remove if dead (only once it clears `/audit:code-hygiene`'s end-to-end deadness bar), or restore the caller the wave forgot to wire; test-data discipline: scan the changed `tests/` + migration files for the four anti-patterns the data/schema-separation rule (root CLAUDE.md § Testing & Environment) forbids — (1) schema DDL in test code (`CREATE TABLE`/`CREATE TYPE`/`ALTER TABLE`/raw DDL), (2) any `.sql` fixture under a `tests/` path, (3) `readFileSync` (or equivalent) of a numbered migration file in a test, (4) a test asserting on rows from a global/migration seed instead of inserted inline at the scenario start — file each hit as a `/jc` defect.
3. At **every** step ask: does this step produce what the next step needs, and is the `verify` terminal state actually reached? Flag any break — a step the chain never calls (a handler with zero callers), a field nothing feeds, a contract the two sides disagree on, an enforcement gap.
4. **In the same pass** — the thread's files are already open from the trace, so read them once — run the code-hygiene lens: `.claude/commands/audit/code-hygiene.md` scope-`diff` protocol (duplication & missed reuse first, then hallucinated imports/APIs, over-engineering, dead code, weak types, shallow error handling, naming). Per-pipeline hygiene already ran pre-merge (`wave/build.md` Step 7), so spend your effort on the **integration delta**: above all a repo-wide Cat 8 reuse-grep for a helper/type/hook a _sibling pipeline_ duplicated (no per-pipeline review could see it), plus dead code the integration orphaned. Return these as the structured `hygiene` findings, separate from functional `defects`.

Output:

```
## Thread {id} — {name} ({type})
**Flow:** INTACT | AT-RISK | BROKEN | N/A (hygiene-only thread)
**Trace:** {step → step → … , marking where it breaks}
**Defects:** {functional breaks — each: {what} — {file:line} — `/jc {one-line fix}`; or "none"}
**Hygiene:** {code-quality findings — each: {kind} — {file:line} — {detail} — `/jc {fix}`; or "none"}
**Duplication:** {file:line ↔ file:line, cross-pipeline copies first; or "none"}
**Notes:** {anything the synthesizer should weigh}
```

## § Role: Synthesizer

You receive the report and every walker's findings. Produce the consolidated review.

1. **Operational review** across these dimensions: grouping quality (efficient? unnecessary splits? cross-project consolidation?), pipeline success rate, QA health (first-try pass rate, fix-loop count, real vs false), parallelism (independent pipelines parallel? conflicts serializing?), scope accuracy (built matches tasked?), token efficiency (3–8 small tasks = ONE pipeline), cross-project coordination.
2. **Aggregate the walks** — collect every walker defect AND every `Hygiene` finding into `### /jc Action Items`, dedup across threads, and merge duplication findings — especially **cross-pipeline duplicates** (the same helper written independently in two pipelines), which only you, seeing all walks at once, can catch.
3. The verdict weighs code quality (the walks) alongside operations — a smooth-running wave that merged a broken flow is not SMOOTH SAILING.
4. Write the review into the report under `## Professor's Wave Review` using the Report Format.

## Report Format

```markdown
## Professor's Wave Review

**Wave:** {name} · **Date:** {date}
**Verdict:** {SMOOTH SAILING | MOSTLY GOOD | ROUGH SEAS | SHIPWRECK}

### Executive Summary

{2-3 sentences}

### What Went Well / What Could Improve

{Bullets each}

### Thread Walk

| Thread | Type | Flow                    | Defects | Notes       |
| ------ | ---- | ----------------------- | ------- | ----------- |
| {name} | {t}  | {INTACT/AT-RISK/BROKEN} | {n}     | {one-liner} |

**Duplication:** {`file:line ↔ file:line`, or "none"}

### Pipeline-by-Pipeline

| Pipeline | Tasks | Routing | QA                   | Verdict | Notes       |
| -------- | ----- | ------- | -------------------- | ------- | ----------- |
| `{name}` | {n}   | {route} | {PASS/FIX-LOOP/FAIL} | {v}     | {one-liner} |

### /jc Action Items

{Numbered — each a `/jc {fix}` candidate with file location. "None" if clean.}

### Operational Metrics

| Metric             | Value      | Assessment                                  |
| ------------------ | ---------- | ------------------------------------------- |
| Tasks to Pipelines | {N} to {M} | {EFFICIENT / COULD-GROUP-MORE / OVER-SPLIT} |
| Success rate       | {X}/{M}    | {percentage}                                |
| First-pass QA rate | {Y}/{M}    | {percentage}                                |
| Fix loops needed   | {count}    | {NONE / MINIMAL / EXCESSIVE}                |

### Recommendations for Next Wave

{Numbered, actionable}

### Final Thought

{One warm, professorial sentence}
```

Every fixable code defect lands in `### /jc Action Items` — `/wave` Step 3.4 drains only that section, so a defect parked in a prose aside is orphaned forever. "Deferred" is reserved for non-code owners — copy/product → `/pm`, erasure/consent → `/officer`, tooling/deps → founder — each tagged with its owner. A finding that contradicts a settled spec decision still lists here with a one-line "reconcile against spec first" note; the dispatcher decides, you never silently drop it.

## Rules

- **Read-only** — git inspection only (`log`/`show`/`diff`); suggest `/jc` candidates, never run them.
- **No orphaned defects** — every fixable code defect lands in `### /jc Action Items`. "Deferred" is owner-tagged non-code work only.
- **Scope to the wave's own diff** — with one bounded exception: the **Dead-code ripple** thread follows the call/reference graph the diff perturbed into unchanged files, to catch code the wave orphaned. A full-codebase sweep is still the `audit:code-hygiene` / `audit:security` commands.
- **Honest and constructive** — disaster? say so kindly; clean? celebrate it; every criticism carries a suggestion.
- After finishing: "Wave review complete. {verdict}."
