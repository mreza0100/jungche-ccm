# Wave Task Runner

$ARGUMENTS

---

**Autonomous execution contract:** Once `/wave` starts, it runs to completion without stopping for questions. Pre-flight (Step 0b) is the only gate — fail fast or go all the way. Ambiguity mid-run → decide from codebase context, log the decision. Inventing any other stop — a mid-wave "founder decision needed" pause — or re-scoping the wave to avoid one is a contract violation. A task being costly, external, or production-affecting is not a stop; raise only a true blocker, as a pre-flight fail-fast.

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

**Epic:** extract `**Epic:** {name}` from the task file → `{epic-name}` (`none` if absent). Forward it to every `/build` and write the wave's consolidated epic update in Step 3.5.

**Wave naming:** Choose a short descriptive kebab-case name (2-4 words) capturing the theme. Defines `$WAVES/{wave-name}/`.

**Name uniqueness (MANDATORY):** Verify name AND all pipeline names don't exist in `$WAVES/archive/` (strip counter prefixes), `$WAVES/`, `docs/dev/builds/archive/` (strip counter prefixes), `docs/dev/builds/`, `tmp/archive/builds/`, `tmp/archive/waves/`. If collision → append `-v2` or choose more specific name. Then: `mkdir -p docs/dev/waves/{wave-name}`.

---

## Runtime

Wave runs on whatever runtime invokes it. Each runtime invokes `/build` in its own way:

- **Claude:** `Skill("build", "{concise-description} [Pipeline: {pipeline-name}] [Build: {n}/{total}] [Wave: {wave-name}] [Epic: {epic-name}] [CarryWIP: {carry-wip}]")`
<!-- OPTIONAL: If using a secondary runtime (e.g., Codex), add its invocation pattern here:
- **{SECONDARY_RUNTIME}:** `Agent(build, "{concise-description} [Pipeline: {pipeline-name}] [Build: {n}/{total}] [Wave: {wave-name}] [Epic: {epic-name}] [CarryWIP: {carry-wip}]")`
  -->

`{n}/{total}` = this pipeline's index across the whole wave (e.g. `2/6`); `build.md`'s § Status Emission renders the per-build header, phase lines, and footer from it.

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
5. **Uncommitted work on main** — a wave may launch with a dirty `main`; that's fine, no prompt. Default to `leave` (WIP stays on `main`, excluded from the pipelines) and forward `[CarryWIP: leave]` to every `/build`. Gitter watches for overlap when each pipeline merges back (gitter Phase 2 § Merge to main) and pauses the wave only if the WIP cannot be cleanly restored after a merge — a critical overlap that must be committed first.

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

Tasks tagged `[CMD: /km]` are knowledge curation (`{AI_PROJECT}/knowledge/`) that `/build` cannot execute. Run BEFORE `/build` via `Skill("km", "{description}")` because build pipelines may depend on the knowledge files KM authors. Remove completed from list.
-->

### Step 0d — Group, plan, and set up

**Grouping algorithm (the key step) — owned by `/wave`, not refinement.** Objective: the **fewest pipelines** that still respect dependencies. Each pipeline carries heavy fixed overhead (planners, mono-planner/architect, QA, code review, merge, post-merge QA, documenter), so pipeline count is the dominant token lever. Group by each task's declared `**Routing:**` (ZERO-GAP wave.md states it — never re-classify):

- Same-routing tasks (same roster-entry set) → ONE pipeline
- More than one roster entry, no conflicts/overlaps → ONE CROSS pipeline (shares all overhead). On a single-project roster every task shares one routing, so they collapse into the fewest pipelines by dependency alone — CROSS never applies.
- Only separate when: real dependencies (B needs A's output), conflicting files, or a task large enough to warrant its own pipeline
- Merge sequential same-project tasks into ONE pipeline (one developer touching a file once < two in sequence)
- **When in doubt, group more aggressively.** One pipeline with 5 tasks is far cheaper than 5 pipelines.

**Combined `/build` descriptions:** Keep `$ARGUMENTS` concise — detailed specs live in pre-placed manifest:

```
"FE polish: fix alignment, update errors, add spinner [Pipeline: fe-polish] [Wave: {wave-name}]"
```

**Setup steps:**

1. Organize groups into waves (wave boundaries enforce dependency ordering only)
2. Pre-place manifests: `mkdir -p docs/dev/builds/{pipeline-name}` then Write `docs/dev/builds/{pipeline-name}/0-task.md` with the pipeline-specific task subset
3. Copy the task file to `$WAVES/{wave-name}/manifest.md` (the permanent record of the full spec — descriptions, compliance flags, architectural intent). Then, **if the task file is the root `wave.md`, immediately overwrite it with the `# Tasks` stub** — copy-to-manifest and clear are one atomic step `/wave` owns, so the consumed spec never lingers in `wave.md`. After a wave, any content in root `wave.md` is a fresh next-wave draft (often uncommitted) — never clear it outside this step.
4. Write grouping/execution plan to `$WAVES/{wave-name}/wave.md`
5. Create `$WAVES/{wave-name}/report.md` with initial plan:

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

**Before launching any pipeline, display the full execution plan as a table.** This gives the user a clear picture of what is about to happen.

Output format:

```
## Execution Plan

| # | Wave | Pipeline | Tasks | Routing      | Description |
|---|------|----------|-------|--------------|-------------|
| 1 | 1    | {name}   | N     | {ROLE}-ONLY  | {one-liner} |
| 2 | 1    | {name}   | N     | CROSS        | {one-liner} |
| 3 | 2    | {name}   | N     | {ROLE}-ONLY  | {one-liner} |

**Sequence:** Wave 1 pipelines run sequentially, then Wave 2 begins.
**Estimated pipelines:** {total} | **JC pre-handled:** {j} | **KM pre-handled:** {k}
```

After displaying, proceed immediately to execution — this is informational, not a gate.

---

## Step 1 — Execute waves

For each wave, sequentially launch pipelines via `Skill("build", "...")`. NEVER delegate Skill calls to sub-agents.

**Status emission (MANDATORY):** Each `/build` prints its own header, phase lines, and footer (`build.md` § Status Emission) from the `[Build: {n}/{total}]` token you pass. After each build returns, add the running tally — only the wave runner knows it:

```
Wave {wave-name}: {done}/{total} done · {failed} failed · {deferred} deferred
```

The user should never have to ask "how much is left?" — the stream tells them.

Log each result in report:

- Success: `- [x] \`{pipeline}\` — **DONE** ✓`
- Failure: `- [x] \`{pipeline}\` — **FAILED** ✗ — {reason}`
- Deferred: `- [ ] \`{pipeline}\` — **BLOCKED-DEFERRED** ⚠️` (trigger, worktree path, resume note, next action)

**BLOCKED-DEFERRED handling:** The wave continues. Check downstream pipelines for explicit dependency on deferred pipeline's output. No dependency → run normally. Explicit dependency → also defer. Ambiguous → default to running (safer). Log decision.

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

Read `.claude/skills/p:wave-review/SKILL.md` and execute its **§ Orchestration** against `$WAVES/{wave-name}/report.md`. You are the dispatcher: spawn the scout, then one walker per thread in parallel, then the synthesizer — fresh `general-purpose` agents, `model: "opus"` — and form no judgments in your own bloated context. The synthesizer writes the review into the report under `## Professor's Wave Review` and returns it.

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

Then copy the review's owner-tagged deferrals (the non-code items it routed to other command owners or the founder) into the same `## Review Remediation` table with their owner, so they surface to the founder instead of dying in the archived review. The reviewer is forbidden from parking a fixable code defect as "deferred" (see `p:wave-review`), so anything here genuinely needs a non-code owner's call.

---

## Step 3.5 — Epic update (epic-tied waves only)

Skip if `{epic-name}` is `none`. Otherwise write ONE consolidated entry for the whole wave — per-build documenters skip epic writes for wave-owned builds, so the wave is the sole writer here:

1. Append to `docs/epics/{epic-name}/update.md` (create if absent):
   ```
   ### {YYYY-MM-DD} — Wave: {wave-name}
   - {1-3 bullet summary across the wave's pipelines}
   ```
2. In `docs/epics/{epic-name}/manifest.md`: append the same under `## Progress Log`; append new decisions surfaced across the wave under `## Key Decisions` (deduped); add `{wave-name}` to `waves:`; bump `updated:`.

Append only — leave Vision & Scope, Open Questions, Discoveries, and `status` to the Professor.

---

## Step 4 — Archive (NON-OPTIONAL — execute AFTER Professor review)

### 4a. Verify and archive straggler builds

Each `/build` pipeline archives itself via the documenter (Step 11 of build.md). The wave's job is to **verify** all its pipelines made it to archive, and catch any stragglers that the documenter missed (compaction, timeout, agent error).

For each pipeline that belonged to this wave (listed in the report's Grouping Summary table):

```bash
mkdir -p docs/dev/builds/archive

for pipeline in {list of pipeline names from report}; do
  # Skip if already archived (documenter handled it)
  if ls docs/dev/builds/archive/*-${pipeline} 1>/dev/null 2>&1 || [ -d "docs/dev/builds/archive/${pipeline}" ]; then
    echo "ALREADY_ARCHIVED: $pipeline"
    continue
  fi
  # Catch straggler — documenter missed it
  if [ -d "docs/dev/builds/$pipeline" ]; then
    echo "STRAGGLER: $pipeline — archiving now"
    mv "docs/dev/builds/$pipeline" "docs/dev/builds/archive/$pipeline"
  fi
done
```

### 4b. Archive the wave itself

**Numbered rolling archive (max 10).** Each archived wave gets a 3-digit counter prefix. When the archive exceeds 10 items, the oldest is evicted to `tmp/archive/waves/` (gitignored cold storage).

```bash
mkdir -p $WAVES/archive tmp/archive/waves

# Read and increment counter
COUNTER=$(cat $WAVES/archive/.counter 2>/dev/null || echo "0")
NEXT=$((COUNTER + 1))
PADDED=$(printf "%03d" $NEXT)

# Archive with numbered prefix
rm -rf $WAVES/archive/${PADDED}-{wave-name}  # remove partial from previous failed attempt
mv $WAVES/{wave-name} $WAVES/archive/${PADDED}-{wave-name}
echo "$NEXT" > $WAVES/archive/.counter

# Evict oldest if more than 10
ARCHIVE_COUNT=$(find $WAVES/archive -maxdepth 1 -type d | wc -l)
ARCHIVE_COUNT=$((ARCHIVE_COUNT - 1))
if [ "$ARCHIVE_COUNT" -gt 10 ]; then
  OLDEST=$(ls -d $WAVES/archive/[0-9]*/ | head -1)
  mv "$OLDEST" tmp/archive/waves/
fi
```

### 4c. Cleanup and verify

Remove a custom (non-root) task file now; the root `wave.md` was already reset to its stub at Step 0d-3. Verify:

```bash
test ! -d $WAVES/{wave-name} && test -d $WAVES/archive/${PADDED}-{wave-name} && echo "WAVE_ARCHIVE_OK" || echo "WAVE_ARCHIVE_FAILED"
```

Announce: `"Wave complete ({wave-name}). {X}/{N} succeeded. Builds + wave archived together."`

---

## Rules

- **Group aggressively** — grouping IS the optimization. Never spawn 5 pipelines for 5 small tasks that could be one. Prefer ONE multi-roster (CROSS) pipeline over separate single-entry pipelines when no conflicts.
- **NEVER delegate Skill calls to sub-agents** — sub-agents lack access to the Skill tool. Always invoke directly.
- **Each pipeline is a full `/build` run** — no shortcuts, no skipping QA.
- **Document every step** — report updated after each pipeline completes.
- **Fully autonomous** — no pauses, no permission requests after pre-flight passes.
- **If a pipeline fails**, log it, continue with remaining pipelines.
- **BLOCKED-DEFERRED** → wave continues; downstream assessed per dependency rules. Worktrees preserved intentionally.
- **Re-group freely** — ignore user's section headings, group by routing/similarity.
- **Professor review then archive** — Professor writes its review into the report FIRST, then the complete artifact (with review) gets archived. Unarchived wave in `$WAVES/` with Final Summary = bug.
- **Clean up** — verify no orphaned worktrees/branches at the repo root after completion. Exception: BLOCKED-DEFERRED worktrees with matching `BLOCKED.md`.
