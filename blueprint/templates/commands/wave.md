# Wave Task Runner

$ARGUMENTS

---

**Autonomous execution contract:** Once `/wave` starts, it runs to completion without stopping for questions. Pre-flight (Step 0b) is the only gate — fail fast or go all the way. Ambiguity mid-run → decide from codebase context, log the decision.

---

## Path variables

| Variable | Value |
|----------|-------|
| `$WAVES` | `docs/dev/waves` |

---

## Resolve the task file

- **Empty/blank:** Task file is `wave.md` at repo root. Read and proceed — no usage instructions.
- **File path:** Read that file.
- **Description (not a path):** Parse directly as inline tasks.

**Wave naming:** Choose a short descriptive kebab-case name (2-4 words) capturing the theme. Defines `$WAVES/{wave-name}/`.

**Name uniqueness (MANDATORY):** Verify name AND all pipeline names don't exist in `$WAVES/archive/` (strip counter prefixes), `$WAVES/`, `docs/dev/builds/archive/` (strip counter prefixes), `docs/dev/builds/`, `tmp/archive/builds/`, `tmp/archive/waves/`. If collision → append `-v2` or choose more specific name. Then: `mkdir -p docs/dev/waves/{wave-name}`.

---

## Runtime

Wave runs on whatever runtime invokes it. Each runtime invokes `/build` in its own way:
- **Claude:** `Skill("build", "{concise-description} [Pipeline: {pipeline-name}] [Wave: {wave-name}]")`
<!-- OPTIONAL: If using a secondary runtime (e.g., Codex), add its invocation pattern here:
- **{SECONDARY_RUNTIME}:** `Agent(build, "{concise-description} [Pipeline: {pipeline-name}] [Wave: {wave-name}]")`
-->

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
3. **Routing feasibility** — every task must be classifiable as FE/BE/{AI_PROJECT_SHORT}/Infra/CROSS/Web. Too vague → fatal.
4. **Dependency ordering** — tasks depending on another's output must have that dependency earlier or already in codebase.

| Result | Action |
|--------|--------|
| All pass | Proceed to 0c |
| Minor ambiguity | Correct inline, log, proceed |
| **Fatal** | **STOP** — no wave dir, no pipelines. Print diagnostic and exit. |

### Step 0c — Command routing triage

#### JC triage

A task is JC if tagged `[CMD: /jc]` OR ALL of: touches ≤3 files, no code logic (config/constants/prompts/CLAUDE.md/env), single project, no new files/schema/tests, no dependency on other tasks. Skip JC detection if `--no-jc` in `$ARGUMENTS`.

Auto-handle: log under `## JC Pre-flight`, run each via `Skill("jc", "{description}")`, remove completed from list. If none remain → "All tasks handled via /jc", archive report, stop.

<!-- OPTIONAL: Domain-specific command triage
#### KM triage

Tasks tagged `[CMD: /km]` are knowledge curation (`{AI_PROJECT}/knowledge/`) that `/build` cannot execute. Run BEFORE `/build` via `Skill("km", "{description}")` because build pipelines may depend on the knowledge files KM authors. Remove completed from list.
-->

### Step 0d — Group, plan, and set up

**Grouping algorithm (the key step):**
- Same-project, same-scope tasks → ONE pipeline
- Same-routing tasks (same set of projects) → ONE pipeline
- Cross-project with no conflicts/overlaps → prefer ONE CROSS pipeline (shares all overhead: planner, architect, QA, merge)
- Only separate when: real dependencies (B needs A's output), conflicting files, or tasks large enough to warrant own pipeline
- Merge single-pipeline sequential waves touching same project into ONE pipeline (one developer modifying a file once < two developers in sequence)
- **When in doubt, group more aggressively.** One pipeline with 5 tasks is far cheaper than 5 pipelines.

**Combined `/build` descriptions:** Keep `$ARGUMENTS` concise — detailed specs live in pre-placed manifest:
```
"FE polish: fix alignment, update errors, add spinner [Pipeline: fe-polish] [Wave: {wave-name}]"
```

**Setup steps:**
1. Organize groups into waves (wave boundaries enforce dependency ordering only)
2. Pre-place manifests: `mkdir -p docs/dev/builds/{pipeline-name}` then Write `docs/dev/builds/{pipeline-name}/0-task.md` with the pipeline-specific task subset
3. Copy the original Professor-refined task file to `$WAVES/{wave-name}/manifest.md` — this preserves the full task spec (all detailed descriptions, compliance flags, architectural intent) as a permanent record of what was requested
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

| # | Wave | Pipeline | Tasks | Routing | Description |
|---|------|----------|-------|---------|-------------|
| 1 | 1    | {name}   | N     | FE-ONLY | {one-liner} |
| 2 | 1    | {name}   | N     | CROSS   | {one-liner} |
| 3 | 2    | {name}   | N     | BE-ONLY | {one-liner} |

**Sequence:** Wave 1 pipelines run sequentially, then Wave 2 begins.
**Estimated pipelines:** {total} | **JC pre-handled:** {j} | **KM pre-handled:** {k}
```

After displaying, proceed immediately to execution — this is informational, not a gate.

---

## Step 1 — Execute waves

For each wave, sequentially launch pipelines via `Skill("build", "...")`. NEVER delegate Skill calls to sub-agents.

**Proactive status emission (MANDATORY):** Before launching each pipeline, emit a one-liner to the user:
```
"Pipeline {current}/{total}: `{pipeline-name}` — starting ({routing}, {task-count} tasks)"
```
After each pipeline completes, emit:
```
"Pipeline {current}/{total}: `{pipeline-name}` — {DONE | FAILED | BLOCKED-DEFERRED}"
```
The user should never have to ask "how much is left?" — the wave runner tells them proactively.

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

```
Skill("professor-wave-review", "$WAVES/{wave-name}/report.md")
```

Append review to report under `## Professor's Wave Review`. Present to user.

**This step is mandatory.** The Professor reads the report, runs a 360 blind-spot sweep, and writes its operational review directly into the report file. No wave is complete without the Professor's verdict. The review becomes part of the archived artifact.

---

## Step 4 — Archive (NON-OPTIONAL — execute AFTER Professor review)

### 4a. Verify and archive straggler builds

Each `/build` pipeline archives itself via the documenter (Step 11 of build.md). The wave's job is to **verify** all its pipelines made it to archive, and catch any stragglers.

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

Remove original task file only if NOT `wave.md` at repo root. Verify:
```bash
test ! -d $WAVES/{wave-name} && test -d $WAVES/archive/${PADDED}-{wave-name} && echo "WAVE_ARCHIVE_OK" || echo "WAVE_ARCHIVE_FAILED"
```

Announce: `"Wave complete ({wave-name}). {X}/{N} succeeded. Builds + wave archived together."`

---

## Rules

- **Group aggressively** — grouping IS the optimization. Never spawn 5 pipelines for 5 small tasks that could be one. Prefer ONE cross-project pipeline over separate single-project pipelines when no conflicts.
- **NEVER delegate Skill calls to sub-agents** — sub-agents lack access to the Skill tool. Always invoke directly.
- **Each pipeline is a full `/build` run** — no shortcuts, no skipping QA.
- **Document every step** — report updated after each pipeline completes.
- **Fully autonomous** — no pauses, no permission requests after pre-flight passes.
- **If a pipeline fails**, log it, continue with remaining pipelines.
- **BLOCKED-DEFERRED** → wave continues; downstream assessed per dependency rules. Worktrees preserved intentionally.
- **Re-group freely** — ignore user's section headings, group by routing/similarity.
- **Professor review then archive** — Professor writes its review into the report FIRST, then the complete artifact (with review) gets archived. Unarchived wave in `$WAVES/` with Final Summary = bug.
- **Clean up** — verify no orphaned worktrees/branches at monorepo root after completion. Exception: BLOCKED-DEFERRED worktrees with matching `BLOCKED.md`.
