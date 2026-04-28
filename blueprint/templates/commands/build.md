# Cross-Project Build Pipeline

Run the full pipeline for: $ARGUMENTS

**All feature requests MUST go through this pipeline.** No cowboy coding.

---

## Step 0 — Name the pipeline, clean up stale dirs, and resolve paths

### 0a. Stale pipeline cleanup (MANDATORY pre-flight)

Before doing anything, check for abandoned pipeline directories in `docs/dev/tasks/` (excluding `archive/`). A pipeline directory is stale if it has NO corresponding active worktree in `.worktrees/`:

```bash
for dir in docs/dev/tasks/*/; do
  name=$(basename "$dir")
  [ "$name" = "archive" ] && continue
  if [ ! -d ".worktrees/$name" ]; then
    echo "STALE: $dir (no active worktree)"
  fi
done
```

For each stale directory:
- Contains a `BLOCKED.md` → intentionally preserved (deferred for `/jc` resolution — see § Fix Loop Escalation). **SKIP cleanup.** Print `PRESERVED: $dir (BLOCKED-DEFERRED)` and move on.
- Has `7-post-merge-qa.md` → completed but not archived. Move to `docs/dev/tasks/archive/`.
- No completion marker (and no `BLOCKED.md`) → abandoned. Add `ABANDONED.md` and move to archive.
- Empty → `rmdir`.

### 0b. Name the pipeline

Choose a short kebab-case name. Verify uniqueness against `docs/dev/tasks/`, `docs/dev/tasks/archive/`, and `.worktrees/`:

```bash
ls docs/dev/tasks/archive/ docs/dev/tasks/ .worktrees/ 2>/dev/null | grep -x "{name}"
```

If exists, append `-v2` / `-v3` or pick a more specific name. Never reuse archived pipeline names.

### 0c. Resolve path variables

- `$PIPELINE` = `{name}`
- `$DOCS` = `docs/dev/tasks/{name}` — pipeline docs from repo root
- `$DOCS_REL` = `../../../docs/dev/tasks/{name}` — pipeline docs from worktree
- `$DOCS_POST` = `../docs/dev/tasks/{name}` — pipeline docs from project subdir (post-merge)
- `$WORKTREE` = `.worktrees/{name}` — pipeline worktree directory
- `$ARCHIVE` = `docs/dev/tasks/archive`
- `$PROJECTS` = list of affected projects (decided by mono-planner)
- Pipeline branch: `pipeline/{name}`

```bash
mkdir -p docs/dev/tasks/{name}
```

**Pass `$PIPELINE`, `$DOCS`, `$DOCS_REL`, `$WORKTREE` to every agent invocation.** Agents never hardcode paths.

---

## Step 1a — Parallel Codebase Analysis (child planners)

Invoke ALL child planners in parallel. Each writes `$DOCS/0-analysis-{project}.md`.

For each subproject in your repo, send one Agent call with `general-purpose` subagent type:

```
Agent(general-purpose, parallel): "You are the {PROJECT} planner. Read and follow {PROJECT_DIR}/.claude/agents/planner.md. Mode: ANALYSIS. Pipeline: $PIPELINE. Docs path: $DOCS. Feature request: $ARGUMENTS."
```

**Parallel:** all planners run simultaneously. Each writes its own analysis file. Wait for all to finish before Step 1b.

---

## Step 1b — mono-planner consolidation

```
Agent(mono-planner): "Pipeline: $PIPELINE. Docs path: $DOCS. Feature request: $ARGUMENTS."
```

Output: `$DOCS/1-plan.md`. Read it. The `Routing` section determines `$PROJECTS`.

---

## Step 2 — gitter SETUP

```
Agent(gitter): "Phase: SETUP. Pipeline: $PIPELINE. Docs: $DOCS."
```

Gitter runs `.claude/scripts/worktree.sh create $PIPELINE`, then writes `$DOCS/2-ports.md`.

---

## Step 3 — mono-architect (cross-project)

```
Agent(mono-architect): "Pipeline: $PIPELINE. Docs: $DOCS. Plan: $DOCS/1-plan.md."
```

Output: `$DOCS/3-architecture.md`.

---

## Step 4 — Per-project planner TASK mode (parallel)

For each project in `$PROJECTS`:

```
Agent(general-purpose, parallel): "You are the {PROJECT} planner. Read and follow {PROJECT_DIR}/.claude/agents/planner.md. Mode: TASK. Pipeline: $PIPELINE. Docs: $DOCS."
```

Each writes `$DOCS/4-tasks-{project}.md`.

---

## Step 5 — Per-project architects (parallel)

For each project in `$PROJECTS`:

```
Agent(general-purpose, parallel): "You are the {PROJECT} architect. Read and follow {PROJECT_DIR}/.claude/agents/architect.md. Pipeline: $PIPELINE. Docs: $DOCS."
```

Each writes `$DOCS/5-architecture-{project}.md`.

---

## Step 6 — Per-project developers (parallel)

For each project in `$PROJECTS`:

```
Agent(general-purpose, parallel): "You are the {PROJECT} developer. Read and follow {PROJECT_DIR}/.claude/agents/developer.md. Pipeline: $PIPELINE. Docs: $DOCS_REL. Worktree: $WORKTREE."
```

Each implements code in `$WORKTREE/{project-dir}` and writes `$DOCS/6-runbook-{project}.md`.

---

## Step 7 — Per-project QA (parallel, PRE-MERGE)

For each project in `$PROJECTS`:

```
Agent(general-purpose, parallel): "You are the {PROJECT} QA. Read and follow {PROJECT_DIR}/.claude/agents/qa.md. Mode: PRE-MERGE. Pipeline: $PIPELINE. Docs: $DOCS_REL. Worktree: $WORKTREE."
```

Each writes `$DOCS/7-qa-{project}.md`. If any QA reports blockers, route back to that project's developer for the fix loop.

### Fix Loop Discipline (load-bearing — do not relax)

- **Iteration cap: 3 maximum.** Never run more than 3 fix-loop iterations per pipeline. After iteration 3 fails, escalate to BLOCKED-DEFERRED (below). Runaway fix loops eat orders of magnitude more wall-time than the work they purport to fix — one historical incident wedged a pipeline for 7+ hours on a single hung test.
- **Hard timeouts on every test invocation.** Every test command inside the fix loop MUST be wrapped in `timeout {N}s <cmd>` (default ~600s / 10 min). Agent definitions own this discipline — the orchestrator does NOT spawn raw test runs.
- **Hung-process detection.** If a test process sits at 0% CPU for >2 minutes (deadlocked, not slow), the agent must `kill` it and report `BUG-HUNG-TEST` with the file:line of the hang. A hung test is NOT a fix-loop bug — it is a code bug that needs `/jc` on main.
- **BLOCKED-DEFERRED escalation.** When the cap trips or a hang is detected, the orchestrator writes `$DOCS/BLOCKED.md` (symptom, last green commit, suspected file, repro command), preserves the worktree, releases merge locks, and reports the blocker to the user. Do NOT keep looping. Do NOT silently abandon the worktree — it must remain on disk so `/jc` can resume from it.

---

## Step 8 — gitter MERGE

```
Agent(gitter): "Phase: MERGE. Pipeline: $PIPELINE. Projects: $PROJECTS. Docs: $DOCS."
```

Gitter acquires merge locks, commits worktree changes, merges to main, removes worktree, releases locks.

---

## Step 9 — Per-project QA (parallel, POST-MERGE)

For each project in `$PROJECTS`:

```
Agent(general-purpose, parallel): "You are the {PROJECT} QA. Read and follow {PROJECT_DIR}/.claude/agents/qa.md. Mode: POST-MERGE. Pipeline: $PIPELINE. Docs: $DOCS_POST."
```

If a post-merge bug is reported: spawn a new `/jc` pipeline to fix it.

---

## Step 9.5 — Pipeline audit (parallel, optional)

After post-merge QA passes, run `/ca` (always) and `/officer` (only if Tier B `/officer` is opted in) in parallel:

```
Agent(general-purpose, parallel): "Run /ca on the changes from this pipeline. Output: $DOCS/8-code-audit.md."
```

```
Agent(general-purpose, parallel, conditional on /officer being opted in):
  "Run /officer audit on the changes from this pipeline. Output: $DOCS/8-officer-audit.md."
```

If either audit reports CRITICAL or BLOCKER findings: spawn a new `/jc` pipeline to address them BEFORE proceeding to documentation. CRITICAL/BLOCKER audit findings block doc-commit the same way QA blocks merge.

---

## Step 10 — mono-documenter

```
Agent(mono-documenter): "Pipeline: $PIPELINE. Docs: $DOCS. Archive: $ARCHIVE."
```

Updates permanent docs, archives the pipeline directory, writes `SUMMARY.md`.

---

## Step 11 — gitter DOCS-COMMIT

```
Agent(gitter): "Phase: DOCS-COMMIT. Pipeline: $PIPELINE. Summary: $ARCHIVE/$PIPELINE/SUMMARY.md."
```

Done. Report to user:

```
Pipeline {PIPELINE} complete.
- Branch merged: pipeline/{PIPELINE} → main ({merge-sha})
- Docs commit: {docs-sha}
- Permanent docs updated: {list}
- Archive: $ARCHIVE/{PIPELINE}/
```

---

## Pipeline Reference Table

| Step | What | Agent | Output |
|------|------|-------|--------|
| 0 | Cleanup + name + paths | (orchestrator) | none |
| 1a | Parallel codebase analysis | child planners (parallel) | `$DOCS/0-analysis-{project}.md` |
| 1b | Consolidate plan | mono-planner | `$DOCS/1-plan.md` |
| 2 | Worktree + ports | gitter SETUP | `$WORKTREE/`, `$DOCS/2-ports.md` |
| 3 | Cross-project architecture | mono-architect | `$DOCS/3-architecture.md` |
| 4 | Per-project tasks | child planners TASK (parallel) | `$DOCS/4-tasks-{project}.md` |
| 5 | Per-project architecture | child architects (parallel) | `$DOCS/5-architecture-{project}.md` |
| 6 | Implementation | child developers (parallel) | code + `$DOCS/6-runbook-{project}.md` |
| 7 | Pre-merge QA | child QAs (parallel) | `$DOCS/7-qa-{project}.md` |
| 8 | Merge | gitter MERGE | merge commit on main |
| 9 | Post-merge QA | child QAs POST-MERGE (parallel) | report or new `/jc` |
| 9.5 | Pipeline audit | /ca (always) + /officer (if opted in) (parallel) | `$DOCS/8-code-audit.md` + `$DOCS/8-officer-audit.md` |
| 10 | Documentation | mono-documenter | permanent doc edits + archive |
| 11 | Docs commit | gitter DOCS-COMMIT | docs commit on main |
