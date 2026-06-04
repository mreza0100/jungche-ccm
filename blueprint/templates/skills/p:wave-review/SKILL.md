---
name: p:wave-review
version: "2.0.0"
description: "Post-wave review — fans out one agent per thread (feature flow, integration seam, data field, schema/DB, invariant) to walk the wave's merged code end-to-end and confirm it functionally works, then a synthesizer folds in the operational review (grouping, QA, parallelism) and writes the verdict. Auto-invoked by /wave after all pipelines complete."
---

# Wave Review — Fan-Out Thread Walk + Operations Review

The Professor walks the wave's merged code by fanning out one agent per thread, then reviews how the wave ran. Runs BEFORE archive.

Each thread is walked **end-to-end, step by step**, by its own fresh agent — the seams where a wave's real bugs hide (a happy path that never reached its COMPLETED state, a field plumbed through three layers and fed by none, a partial index masquerading as a lock) are exactly the seams a focused per-thread walk catches and a single-pass read does not.

**Read-only.** Static trace only — `git log`/`show`/`diff`, `Read`, `Grep`. No code runs, no DB writes, no edits. Confirming live behavior is `/qa`'s job; this confirms the code is wired to behave correctly.

## Entry points

- **Auto (`/wave` Step 3):** the wave runner executes § Orchestration directly.
- **Manual (`wave-review {report-path}`):** the Professor executes § Orchestration himself.

Both the wave runner and the Professor are pure dispatchers here — they spawn fresh agents and pass structured data between them, forming no judgments in their own (bloated) context. Every act of cognition — enumerate, walk, synthesize — happens in a clean spawned context. Spawned agents are leaf nodes: they do not spawn further.

## § Orchestration (run by the dispatcher)

Input: the wave's `report.md` (grouping, results, merge SHAs, JC pre-flight). Spawn every agent `general-purpose`, `model: "opus"`.

**Phase A — Scout (1 agent).** Spawn one agent to enumerate the threads:

> Read `CLAUDE.md` (Professor persona + standards) and `.claude/skills/p:wave-review/SKILL.md` § Role: Scout. Enumerate the wave's threads from `{report-path}`. Return the thread manifest only.

**Phase B — Walk (N agents, in parallel).** For each thread in the manifest, spawn one walker — all in a single message so they run concurrently:

> Read `CLAUDE.md` and `.claude/skills/p:wave-review/SKILL.md` § Role: Walker. Walk this thread end-to-end and return your findings: `{thread spec, verbatim from the manifest}`.

**Phase C — Synthesize (1 agent).** Spawn one agent with the report path and all walker findings:

> Read `CLAUDE.md` and `.claude/skills/p:wave-review/SKILL.md` § Role: Synthesizer. Given the report at `{report-path}` and these thread-walk findings `{all walker outputs}`, run the operational review, aggregate the walks, and write the review into the report under `## Professor's Wave Review` per the Report Format. Return the review.

Present the returned review to the user.

## § Role: Scout

Enumerate the threads to walk from the wave's actual diff. Aim for **at least 4**; right-size to the wave — one thread per feature flow, plus a thread for each seam, field, schema change, or invariant that the diff puts at risk. Merge trivial threads; never split for the sake of count.

1. From the report's Final Summary / Grouping Summary / `## JC Pre-flight`: list SUCCEEDED pipeline merge SHAs and any JC commits. Note FAILED/DEFERRED pipelines as not-merged.
2. `git diff {merge}^1 {merge}` per pipeline (`git show {sha}` for a JC fix) → build the changed-and-generated file set.
3. Read `.claude/skills/360/SKILL.md` and run it inline, domain `inquiry`, subject "wave {name} — {N} tasks across {M} pipelines", to surface blind-spot dimensions (sequences, contradictions, missing info, state, auth). Each dimension that touches the diff must be covered by a thread.

**Thread taxonomy** — every thread is one of:

| Type             | Walk path                                                                                                        |
| ---------------- | ---------------------------------------------------------------------------------------------------------------- |
| **Feature flow** | a user-facing capability — entry (UI/handler) → each hop → terminal state                                        |
| **Seam**         | a cross-project contract ({API_PROTOCOL} field, {REALTIME_PROTOCOL} channel, {QUEUE} message) — both sides agree |
| **Field**        | a new/changed persisted field — producer → transport → persist → read → surface                                  |
| **Schema/DB**    | migrations + constraints — migration ↔ schema ↔ app-layer enforcement                                            |
| **Invariant**    | a sacred {DOMAIN_ADJ}/safety rule — every enforcement point holds                                                |

Output **only** the manifest — one entry per thread:

```
- id: T1
  type: feature flow | seam | field | schema/db | invariant
  name: {short name}
  scope: {one line — what this thread covers}
  files: {key paths the walker should start from}
  verify: {what "works" means for this thread — the terminal state or contract to confirm}
```

## § Role: Walker

Walk your one assigned thread end-to-end and confirm it is wired to behave as the spec intends. Read-only: `git log`/`show`/`diff`, `Read`, `Grep`.

1. Read the thread spec, then the `files` it names.
2. **Trace it step by step** across every layer it crosses — feature flow: entry → each hop → terminal state; field: producer → transport → persist → read → surface; seam: emit side ↔ consume side; schema/db: migration ↔ schema ↔ app enforcement; invariant: each enforcement point.
3. At **every** step ask: does this step produce what the next step needs, and is the `verify` terminal state actually reached? Flag any break — a step the chain never calls (a `finish` handler with zero callers), a field nothing feeds, a contract the two sides disagree on, an enforcement gap.
4. Run the hygiene lens on the thread's files: read `.claude/skills/audit:code-hygiene/SKILL.md` and apply its scope-`diff` protocol (duplication & missed reuse first, then hallucinated imports/APIs, over-engineering, dead code, weak types, shallow error handling, naming).

Output:

```
## Thread {id} — {name} ({type})
**Flow:** INTACT | AT-RISK | BROKEN
**Trace:** {step → step → … , marking where it breaks}
**Defects:** {each as: {what} — {file:line} — `/jc {one-line fix}`; or "none"}
**Duplication:** {file:line ↔ file:line; or "none"}
**Notes:** {anything the synthesizer should weigh}
```

## § Role: Synthesizer

You receive the report and every walker's findings. Produce the consolidated review.

1. **Operational review** across these dimensions: grouping quality (efficient? unnecessary splits? cross-project consolidation?), pipeline success rate, QA health (first-try pass rate, fix-loop count, real vs false), parallelism (independent pipelines parallel? conflicts serializing?), scope accuracy (built matches tasked?), token efficiency (3–8 small tasks = ONE pipeline), cross-project coordination.
2. **Aggregate the walks** — collect every walker defect into `### /jc Action Items`, dedup across threads, merge duplication findings.
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
- **Scope to the wave's own diff** — a full-codebase sweep is `/audit`.
- **Honest and constructive** — disaster? say so kindly; clean? celebrate it; every criticism carries a suggestion.
- After finishing: "Wave review complete. {verdict}."

```

```
