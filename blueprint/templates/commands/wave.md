---
name: wave
description: Wave task runner — executes a batch of /wave:build pipelines from a wave task file with dependency grouping, QA, archive, and post-wave review. Route parallel feature batches here; specs come from /wave:refine.
argument-hint: [task file|inline tasks]
---

# Wave Task Runner

$ARGUMENTS

---

**Autonomous execution contract:** Same contract as `/wave:build`. Wave-specific deltas: pre-flight (Step 0b) is the wave's only gate; once it passes, the wave runs every pipeline to completion. A mid-wave "founder decision needed" pause, or re-scoping the wave to dodge one, is a contract violation — surface a true blocker only at pre-flight as a fail-fast.

---

## Path variables

| Variable | Value            |
| -------- | ---------------- |
| `$WAVES` | `docs/dev/waves` |

---

## Resolve the task file

- **Empty/blank:** Task file is `wave.md` at repo root. Read and proceed — no usage instructions.
- **File path:** Read that file.
- **Description (not a path):** Parse directly as inline tasks.

**Epic:** extract `**Epic:** {name}` from the task file → `{epic-name}` (`none` if absent). Forward it to every `/wave:build` and write the wave's consolidated epic update in Step 3.5.

**Wave naming:** Choose a short descriptive kebab-case name (2-4 words) capturing the theme. Defines `$WAVES/{wave-name}/`.

**Name uniqueness (MANDATORY):** Verify name AND all pipeline names don't exist in `$WAVES/`, `docs/dev/builds/`, `tmp/dev/archive/builds/`, `tmp/dev/archive/waves/` (strip legacy counter prefixes when matching). If collision → append `-v2` or choose more specific name. Then: `mkdir -p docs/dev/waves/{wave-name}`.

---

## Runtime

Wave runs on whatever runtime invokes it. Each runtime invokes `/wave:build` in its own way:

- **Claude:** `Skill("wave:build", "{concise-description} [Pipeline: {pipeline-name}] [Build: {n}/{total}] [Wave: {wave-name}] [Epic: {epic-name}] [CarryWIP: {carry-wip}]")`
<!-- OPTIONAL: If using a secondary runtime (e.g., Codex), add its invocation pattern here:
- **{SECONDARY_RUNTIME}:** `Agent(wave-build, "{concise-description} [Pipeline: {pipeline-name}] [Build: {n}/{total}] [Wave: {wave-name}] [Epic: {epic-name}] [CarryWIP: {carry-wip}]")`
  -->

`{n}/{total}` = this pipeline's index across the whole wave (e.g. `2/6`); `wave/build.md`'s § Status Emission renders the per-build header, phase lines, and footer from it.

Pipelines run sequentially (Skill tool can't be delegated to sub-agents). Your job: grouping, sequencing, reporting.

---

## Step 0 — Read, validate, and plan grouping

### Step 0a — Read the task file

Two forms:

- **Professor-refined** — already has critically evaluated tasks with refined scope, functional requirements, compliance flags, architectural intent. Use as-is → proceed to 0b.
- **Raw/unstructured** — invoke Professor for interactive refinement: `Skill("professor", "Write the following tasks to wave.md with interactive refinement:\n{raw content}")`. Re-read after Professor finishes → proceed to 0b.

### Step 0b — Pre-flight validation

**FAIL FAST** before any pipeline overhead.

1. **Existence checks** — grep for named entities each task references (components, tables, endpoints, files). Fatal if referenced entity doesn't exist.
2. **Conflict detection** — incompatible changes to same target across tasks → fatal.
3. **Routing present** — a ZERO-GAP wave.md declares each task's `**Routing:**` (which roster entries — one `{ROLE}` or `CROSS`). Read it; never re-classify. A single-project roster makes routing trivially that one project. Fatal only if a task lacks a routing declaration and is too vague to classify (an unrefined task file).
4. **Dependency ordering** — tasks depending on another's output must have that dependency earlier or already in codebase.
5. **Uncommitted work on main** — a wave may launch with a dirty `main`; that's fine, no prompt. Default to `leave` (WIP stays on `main`, excluded from the pipelines) and forward `[CarryWIP: leave]` to every `/wave:build`. Gitter watches for overlap when each pipeline merges back (gitter Phase 2 § Merge to main) and pauses the wave only if the WIP cannot be cleanly restored after a merge — a critical overlap that must be committed first.

| Result          | Action                                                           |
| --------------- | ---------------------------------------------------------------- |
| All pass        | Proceed to 0c                                                    |
| Minor ambiguity | Correct inline, log, proceed                                     |
| **Fatal**       | **STOP** — no wave dir, no pipelines. Print diagnostic and exit. |

### Step 0c — Command routing triage

#### JC triage

A task is JC if tagged `[CMD: /jc]` OR ALL of: touches ≤3 files, no code logic (config/constants/prompts/CLAUDE.md/env), single project, no new files/schema/tests, no dependency on other tasks. Skip JC detection if `--no-jc` in `$ARGUMENTS`.

Auto-handle: log under `## JC Pre-flight`, run each via `Skill("jc", "{description}")`, remove completed from list. If none remain → "All tasks handled via /jc", archive report, stop.

<!-- OPTIONAL: Domain-specific command triage
#### KM triage

Tasks tagged `[CMD: /km]` are knowledge curation (`{AI_PROJECT}/knowledge/`) that `/wave:build` cannot execute. Run BEFORE `/wave:build` via `Skill("km", "{description}")` because build pipelines may depend on the knowledge files KM authors. Remove completed from list.
-->

### Step 0d — Group, plan, and set up

**Grouping algorithm (the key step) — owned by `/wave`, not refinement.** Objective: the **fewest pipelines** that still respect dependencies. Each pipeline carries heavy fixed overhead (planners, mono-planner/architect, QA, code review, merge, post-merge QA, documenter), so pipeline count is the dominant token lever. Group by each task's declared `**Routing:**` (ZERO-GAP wave.md states it — never re-classify):

- Same-routing tasks (same roster-entry set) → ONE pipeline
- More than one roster entry, no conflicts/overlaps → ONE CROSS pipeline (shares all overhead). On a single-project roster every task shares one routing, so they collapse into the fewest pipelines by dependency alone — CROSS never applies.
- Only separate when: real dependencies (B needs A's output), conflicting files, or a task large enough to warrant its own pipeline
- Merge sequential same-project tasks into ONE pipeline (one developer touching a file once < two in sequence)
- **When in doubt, group more aggressively.** One pipeline with 5 tasks is far cheaper than 5 pipelines.

**Combined `/wave:build` descriptions:** Keep `$ARGUMENTS` concise — detailed specs live in pre-placed manifest:

```
"FE polish: fix alignment, update errors, add spinner [Pipeline: fe-polish] [Wave: {wave-name}]"
```

**Setup steps:**

1. Organize groups into waves (wave boundaries enforce dependency ordering only). Record each pipeline's `dependsOn` — the pipeline names whose merged output it needs; Step 1 forwards it so the workflow skips dependents of a deferred pipeline.
2. Run the stale sweep once for the whole wave: read and execute `docs/commands/build/references/build-reference.md` § 0a (wave-mode pipelines skip their per-build Step 0a).
3. Pre-place manifests: `mkdir -p docs/dev/builds/{pipeline-name}` then Write `docs/dev/builds/{pipeline-name}/0-task.md` carrying ONLY this pipeline's exact task slice plus a thin shared-rules header. The header carries genuinely shared rules and contracts this pipeline's tasks depend on (cross-cutting compliance flags, shared types/API contracts, conventions). It must NOT carry other pipelines' task bodies, their adjudications, or the full wave manifest / reconciliation table — cite that section by name instead of inlining it. Cross-pipeline content in a `0-task.md` is leakage; keep each slice clean. After pre-placing, verify the slices are real slices — distinct file hashes across the wave's pipelines; identical manifests mean the full wave spec leaked into every pipeline.
4. Copy the task file to `$WAVES/{wave-name}/manifest.md` (the permanent record of the full spec — descriptions, compliance flags, architectural intent). Then, **if the task file is the root `wave.md`, immediately overwrite it with the `# Tasks` stub** — copy-to-manifest and clear are one atomic step `/wave` owns, so the consumed spec never lingers in `wave.md`. After a wave, any content in root `wave.md` is a fresh next-wave draft (often uncommitted) — never clear it outside this step.
5. Write the grouping rationale to `$WAVES/{wave-name}/wave.md`, and the executable payload to `$WAVES/{wave-name}/workflow.json` — Step 1 executes exactly this file, never an ad-hoc reconstruction:

   ```json
   {
     "waveName": "{wave-name}",
     "epicName": "{epic-name or none}",
     "carryWip": "leave",
     "timestamp": "{YYYY-MM-DD}",
     "total": 3,
     "groups": [
       [
         {
           "pipelineName": "task-a",
           "idx": 1,
           "description": "Task A: short description",
           "routing": ["{project}"],
           "dependsOn": []
         },
         {
           "pipelineName": "task-b",
           "idx": 2,
           "description": "Task B: short description",
           "routing": ["{project-x}", "{project-y}"],
           "dependsOn": []
         }
       ],
       [
         {
           "pipelineName": "task-c",
           "idx": 3,
           "description": "Task C: depends on B output",
           "routing": ["{project}"],
           "dependsOn": ["task-b"]
         }
       ]
     ]
   }
   ```

   Every field is required. `groups` = one inner array per execution wave, in dependency order; `idx` numbers pipelines across the whole wave; `routing` = declared project keys; `dependsOn` = pipeline names whose merged output this one needs — the workflow skips dependents of a deferred pipeline and marks them `SKIPPED-DEPENDENCY`.

6. Create `$WAVES/{wave-name}/STATE.md` — delta-structured so transitions append instead of rewriting the whole file:

   - **TOP (rewritten every transition):** a live resume brief for a cold session, then a `## Next` block — what runs next and why.
   - **A marker line** exactly: `<!-- APPEND-ONLY BELOW — never rewrite -->`.
   - **BELOW the marker (append-only, NEVER rewritten):** an adjudication/decision archive — locked decisions and adjudications carried from refinement, then discovered facts as they surface. Only ever append to this section.

   Seed the top brief and `## Next` from the round plan; seed the archive with refinement's locked decisions/adjudications.

7. Create `$WAVES/{wave-name}/report.md` with initial plan:

```markdown
# Wave Report: {wave-name}

**Task file:** {name} | **Started:** {timestamp}
**Total tasks:** N → J via /jc + M pipelines | **Waves:** W

## Grouping Summary

| Pipeline | Tasks included | Routing |

## Execution Plan

### Wave N (parallel)

- [ ] `{pipeline}` — description (N tasks)
```

---

## Step 0e — Present execution plan

**Before launching any pipeline, display the full execution plan as a table.** This gives the user a clear picture of what's about to happen.

Output format:

```
## Execution Plan

| # | Wave | Pipeline | Tasks | Routing | Description |
|---|------|----------|-------|---------|-------------|
| 1 | 1    | {name}   | N     | {ROLE}-ONLY | {one-liner} |
| 2 | 1    | {name}   | N     | CROSS   | {one-liner} |
| 3 | 2    | {name}   | N     | {ROLE}-ONLY | {one-liner} |

**Sequence:** Wave 1 pipelines run concurrently — each owns its isolated per-pipeline test stack, so SETUP/QA/GATE-1 run inline with no cross-pipeline lock; only gitter MERGE serializes against `main`. Then Wave 2 begins.
**Estimated pipelines:** {total} | **JC pre-handled:** {j} | **KM pre-handled:** {k}
```

After displaying, proceed immediately to execution — this is informational, not a gate.

---

## Step 1 — Execute waves

Read `$WAVES/{wave-name}/workflow.json` (written at Step 0d — the single execution source of truth) and launch the saved group scheduler with its contents verbatim. `wave-pipelines.js` sequences groups, runs a group's pipelines in parallel, handles `dependsOn` deferral and the durable STATE.md scribe — and composes ONE `wave-build` workflow per pipeline (the same single-pipeline engine `/wave:build` launches standalone). Per-pipeline build mechanics — SETUP, plan, architecture, develop, targeted QA + fix loop, GATE-1, merge, GATE-2, docs — live in `wave-build.js`, not the scheduler (`wave/build.md` § Wave workflow mode):

`Workflow({name: "wave-pipelines", args: <workflow.json contents>})`

Record the returned `runId` in STATE.md's TOP brief. The workflow runs in the background (`/workflows` shows live per-pipeline progress) — wait for its completion notification, do not poll.

**Resume:** same session — relaunch with `resumeFromRunId: {runId from STATE.md}`; completed agents return cached. New session — the run cache is unavailable: edit `workflow.json` to drop pipelines STATE.md's appended outcome lines already mark DONE, then launch fresh from it; BLOCKED-DEFERRED pipelines resume per their `BLOCKED.md` protocol instead.

**Pause & amend:** a founder pause = `TaskStop` on the workflow task; resume per the rules above. Mid-wave spec amendments land only in NOT-yet-started pipelines' `0-task.md` (agents read it at spawn) — a running pipeline is never re-scoped mid-flight.

Each result's `flags` (carry-forward /jc candidates, SPEC-CONFLICTs, pre-existing defects — also scribed into STATE.md as pipelines land) feed Step 3.4 remediation alongside the review's action items.

When the workflow returns, map each result into the report and emit the tally:

```
Wave {wave-name}: {done}/{total} done · {failed} failed · {deferred} deferred
```

- `DONE` → `- [x] \`{pipeline}\` — **DONE** ✓`
- `FAILED` / `MERGE-FAILED` → `- [x] \`{pipeline}\` — **FAILED** ✗ — {reason}`
- `BLOCKED-DEFERRED` → `- [ ] \`{pipeline}\` — **BLOCKED-DEFERRED** ⚠️` (trigger, worktree path, resume note, next action)
- `SKIPPED-DEPENDENCY` → `- [ ] \`{pipeline}\` — **SKIPPED** (depends on {deferred pipeline})`
- `POSTMERGE-FIX-NEEDED` → run `wave/build.md` § If Post-Merge QA fails for that pipeline now, then update its row

Then update `$WAVES/{wave-name}/STATE.md` once: rewrite the TOP brief and `## Next` block for the post-wave position; append fresh decisions/facts under the append-only marker. Never rewrite the append-only section.

**Fallback (transitional):** when the Workflow tool is unavailable in the session, run the sequential path — for each wave in order, `Skill("wave:build", "{description} [Pipeline: {name}] [Build: {n}/{total}] [Wave: {wave-name}] [Epic: {epic-name}] [CarryWIP: {carry-wip}]")` directly (never via a sub-agent), logging results and STATE.md updates after each build returns.

---

## Step 2 — Final report

Update report with:

```markdown
## Final Summary

**Completed:** {timestamp} | **Pipelines:** X succeeded, Y failed, Z deferred

| Pipeline | Tasks | Status | Notes |
```

---

## Step 3 — Professor Review (NON-OPTIONAL)

Read `.claude/commands/wave/review.md` and execute its **§ Orchestration** against `$WAVES/{wave-name}/report.md`. You are the dispatcher: spawn the scout, then one walker per thread in parallel, then the synthesizer — fresh `general-purpose` agents, `model: "opus"` — and form no judgments in your own bloated context. The synthesizer writes the review into the report under `## Professor's Wave Review` and returns it.

Model tiers per `docs/commands/pcm/references/agent-models.md` (single source); literals here are declared copies.

Present the returned review to the user.

**This step is mandatory.** The fan-out walks every thread of the wave's merged code end-to-end and the synthesizer folds in the operational review. No wave is complete without the Professor's verdict. The review becomes part of the archived artifact.

---

## Step 3.4 — Auto-remediate review findings (/jc)

The Step 3 review is read-only — it names `/jc` candidates under `### /jc Action Items` but never runs them. The wave runner closes the loop: actionable findings get fixed on `main` now, not abandoned in the archive.

Read `### /jc Action Items` from the returned review:

- **"None"** → proceed to Step 3.5.
- **One or more** → group related items by project/area into as few `/jc` calls as sensible, then run each from your own context (never delegate Skill to a sub-agent):

  ```
  Skill("jc", "{finding + file location, verbatim from the action item}")
  ```

  `/jc` diagnoses, fixes, tests, and commits each on `main`. Append the outcomes to the report:

  ```markdown
  ## Review Remediation

  | Finding | Result                   | Commit            |
  | ------- | ------------------------ | ----------------- |
  | {item}  | FIXED / NO-OP / DEFERRED | {short-hash or —} |
  ```

If `/jc` judges an item too large for a hotfix (needs a feature, not a fix), it logs `DEFERRED` with the reason — never block the wave on it.

Then copy the review's owner-tagged deferrals (the non-code items it routed to other command owners or the founder) into the same `## Review Remediation` table with their owner, so they surface to the founder instead of dying in the archived review. The reviewer is forbidden from parking a fixable code defect as "deferred" (see `wave:review`), so anything here genuinely needs a non-code owner's call.

---

## Step 3.5 — Epic update (epic-tied waves only)

Skip if `{epic-name}` is `none`. Otherwise consolidate ONE entry for the whole wave — per-build documenters skip epic writes for wave-owned builds, so the wave is the sole writer here. Apply the Epic consolidation contract (`.claude/commands/documenter.md` § Epic consolidation contract) to `docs/epics/{epic-name}/`: merge the wave's shipped work into `update.md` (`## Delivered` per area, `## State of work` refreshed); fold decisions surfaced across the wave into `## Key Decisions` (deduped); add one `## Progress Log` line; add `{wave-name}` to `waves:`; bump `updated:`. `## Vision & Scope`, `## Discoveries`, `## Open Questions`, and `status` stay the Professor's.

---

## Step 4 — Commit & Archive (NON-OPTIONAL — execute AFTER Professor review)

Everything goes into git history first, then to gitignored cold storage under `tmp/` — no archive stays in `docs/`.

### 4a. Inventory the wave's dirs

Wave-owned builds are never archived individually, so every pipeline dir from the report's Grouping Summary should still be at `docs/dev/builds/{pipeline}`:

```bash
for pipeline in {list of pipeline names from report}; do
  [ -d "docs/dev/builds/$pipeline" ] && echo "PRESENT: $pipeline" || echo "MISSING: $pipeline — already moved, skipping"
done
```

Collect the present dirs plus `$WAVES/{wave-name}` as the archive list. Exception: leave a `BLOCKED.md` (deferred) pipeline dir in place — it archives when resumed.

### 4b. Gitter commit + archive

Invoke the `gitter` agent: "Pipeline: {wave-name}. Wave: {wave-name}. Phase: DOCS-COMMIT. Archive: {archive list from 4a}."

Gitter commits all doc changes (report with review, remediation, epic updates, build dirs) into git history, moves the build dirs to `tmp/dev/archive/builds/` and the wave dir to `tmp/dev/archive/waves/`, then commits the removals.

### 4c. Cleanup and verify

Remove a custom (non-root) task file now; the root `wave.md` was already reset to its stub at Step 0d-3. Verify:

```bash
test ! -d $WAVES/{wave-name} && test -d tmp/dev/archive/waves/{wave-name} && echo "WAVE_ARCHIVE_OK" || echo "WAVE_ARCHIVE_FAILED"
```

Announce: `"Wave complete ({wave-name}). {X}/{N} succeeded. Builds + wave committed to history and archived to tmp."`

---

## Rules

- **Group aggressively** — grouping IS the optimization. Never spawn 5 pipelines for 5 small tasks that could be one. Prefer ONE multi-roster (CROSS) pipeline over separate single-entry pipelines when no conflicts.
- **NEVER delegate Skill calls to sub-agents** — sub-agents lack access to the Skill tool. Always invoke directly.
- **Each pipeline is a full `/wave:build` run** — no shortcuts, no skipping QA.
- **Document every step** — report updated after each pipeline completes.
- **Fully autonomous** — no pauses, no permission requests after pre-flight passes.
- **If a pipeline fails**, log it, continue with remaining pipelines.
- **BLOCKED-DEFERRED** → wave continues; downstream assessed per dependency rules. Worktrees preserved intentionally.
- **Re-group freely** — ignore user's section headings, group by routing/similarity.
- **Professor review then archive** — Professor writes its review into the report FIRST, then the complete artifact (with review) gets archived. Unarchived wave in `$WAVES/` with Final Summary = bug.
- **Clean up** — verify no orphaned worktrees/branches at the repo root after completion. Exception: BLOCKED-DEFERRED worktrees with matching `BLOCKED.md`.
