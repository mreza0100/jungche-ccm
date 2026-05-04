# Wave Task Runner

$ARGUMENTS

---

**Autonomous execution contract:** Once `/wave` is started, it runs to completion without stopping to ask questions or wait for approval. Pre-flight validation (Step 0b) is the only gate — fail fast before any pipeline work begins, or go all the way. If something is ambiguous mid-run, use codebase context to decide and log the decision.

---

## Path variables

| Variable | Value | Semantic |
|----------|-------|----------|
| `$WAVES` | `docs/dev/waves` | Root of all wave runner docs (reports, archives) |

---

## Resolve the task file

Treat `$ARGUMENTS` as follows:

**If empty or blank:** The task file is `wave.md` at the repo root. Read it and proceed — do NOT ask the user for arguments, do NOT show usage instructions. Just go.

**If a file path:** Read that file as the task file.

**If a description (not a file path):** Treat it as an inline task list — skip reading a file and parse the description directly as the tasks to run.

**Wave naming:** After reading the task file, choose a short descriptive **wave name** (kebab-case, 2-4 words) that captures the theme of the tasks (e.g., `ux-polish`, `auth-fixes`, `dashboard-mvp`). This name defines the wave directory: `$WAVES/{wave-name}/`.

**Name uniqueness check (MANDATORY):** Before proceeding, verify the chosen wave name AND all pipeline names within it do NOT already exist in:
- `$WAVES/archive/` — archived waves
- `$WAVES/` — active waves
- `docs/dev/tasks/archive/` — archived pipelines
- `docs/dev/tasks/` — active pipelines

```bash
ls docs/dev/waves/archive/ docs/dev/waves/ docs/dev/tasks/archive/ docs/dev/tasks/ 2>/dev/null | grep -x "{name}"
```

If any name collides, append a version suffix (e.g., `ux-polish-v2`) or choose a more specific name. **NEVER reuse an archived name** — it causes doc conflicts and breaks traceability.

After the wave name is finalized and uniqueness check passes, create the wave directory:
```bash
mkdir -p docs/dev/waves/{wave-name}
```

---

You orchestrate all tasks described in that file by invoking `/build` for each one **sequentially**
(Skill tool cannot be delegated to sub-agents, so pipelines run one at a time).
You document progress in `$WAVES/{wave-name}/report.md` and archive everything when done.

### Runtime — runs natively wherever invoked

Wave runs on whatever runtime invokes it — Claude in Claude Code, or any other compatible runtime from a terminal. Each runtime invokes `/build` in its own way:

- **Claude:** `Skill("build", "{concise-combined-description} [Pipeline: {pipeline-name}] [Wave: {wave-name}]")`
<!-- OPTIONAL: If using a secondary runtime (e.g., Codex), add its invocation pattern here:
- **{SECONDARY_RUNTIME}:** `Agent(build, "{concise-description} [Pipeline: {pipeline-name}] [Wave: {wave-name}]")`
-->

Each pipeline runs to completion before the next one starts. Your job at the wave level is grouping, sequencing, and reporting — not routing.

**Grouping strategy:** TOKEN EFFICIENCY AND WALL-CLOCK TIME BOTH IMPROVE WITH GROUPING. Pipelines run sequentially (Skill tool can't be delegated to sub-agents), and each pipeline pays full overhead — mono-planner, mono-architect, per-project planners/architects/developers/QA, merge, archive. One pipeline handling 5 tasks is dramatically cheaper AND faster end-to-end than 5 pipelines handling 1 task each. Group aggressively — see Step 0d for the algorithm.

---

## Step 0 — Read, validate, and plan grouping

### Step 0a — Read the task file

1. Read the task file (resolved in "Resolve the task file" above).
2. The task file may come in two forms:
   - **Professor-refined** — already has critically evaluated tasks with refined scope, functional requirements, compliance flags, architectural intent, and behavioral specs. The Professor has already questioned, split, merged, and added prerequisite tasks. The Professor describes WHAT needs to happen but does NOT decide routing, grouping, sizing, or parallelism.
   - **Raw user input** — unstructured bullet points, paragraphs, or a mix. Needs the Professor's interactive refinement.

3. **If the task file is Professor-refined:** Use it as-is — proceed to Step 0b (pre-flight validation).
4. **If the task file is raw/unstructured:** Invoke the Professor to interactively refine the tasks. The Professor will read the codebase, ask the user a batch of targeted questions (scope, priorities, dependencies, compliance, behavioral details), wait for answers, then critically evaluate and rewrite the tasks with full specification depth. Use the Skill tool:

   ```
   Skill("professor", "Write the following tasks to wave.md with interactive refinement:\n{paste the raw task content here}")
   ```

   After the Professor finishes writing `wave.md`, re-read it and proceed to Step 0b (pre-flight validation).

### Step 0b — Pre-flight validation

Before creating any wave directory or starting any pipeline work, validate the (now-refined) task spec is actually runnable. **FAIL FAST** — stop completely before any pipeline overhead if the spec is broken.

**1. Existence checks.** For each task, grep the codebase for any specific named entities it references — components, services, tables, API endpoints, chain names, agent names, file paths. If a task says "update the FooBar component" and no such component exists in the codebase, that task has a fatal spec error.

```bash
# Quick existence check example — adapt project dirs to your monorepo layout
grep -r "{named_entity}" {project-be}/ {project-fe}/ {project-cortex}/ 2>/dev/null | head -5
```

**2. Conflict detection.** Scan all tasks for incompatible changes to the same target. Two tasks that rename the same function in different ways, or add and remove the same column, are a fatal conflict.

**3. Routing feasibility.** Every task must be classifiable as FE / BE / {AI_PROJECT_SHORT} / Infra / CROSS / Web. A task so vague it can't be routed at all (e.g., "make everything better") fails this check.

**4. Dependency ordering.** Tasks that explicitly depend on another task's output must have that dependency either earlier in the wave OR already present in the codebase.

**Pre-flight outcome:**

| Result | Action |
|--------|--------|
| All checks pass | Proceed to Step 0c immediately — no announcement, no gate |
| Minor ambiguity (wrong path, easily inferable) | Correct inline, log it, proceed |
| **Fatal issue** | **STOP — do not create wave dir, do not start pipelines** |

**On fatal failure:**
```
PRE-FLIGHT FAILED — wave cannot proceed.

Issues found:
- Task {N}: References "{name}" which does not exist in the codebase.
- Task {N}: Too vague to route — cannot determine which project(s) are affected.
...

Clarify these issues and re-run /wave.
```

### Step 0c — Command routing triage (auto-handle)

**Tasks can be routed to commands other than `/build`.** The Professor tags tasks with `[CMD: /jc]`, etc. in the wave spec. This step detects and routes them before `/build` grouping.

#### JC triage

5. **Identify JC candidates.** Scan every task for trivial ones that are `/jc` hotfixes instead of full `/build` pipelines. A task is a **JC candidate** if it has `[CMD: /jc]` OR ALL of these are true:
   - Touches <= 3 files
   - No code logic (config edits, constant changes, prompt text edits, CLAUDE.md updates, env var tweaks)
   - Single project scope (not cross-project)
   - No new files, no schema changes, no new tests needed
   - No dependency on other tasks in the wave

   Skip JC detection entirely if `--no-jc` appears in `$ARGUMENTS` — all tasks go through /build.

6. **Auto-handle JC candidates — no confirmation:**
   - If JC candidates found:
     1. Log them under `## JC Pre-flight` in the report
     2. Run each sequentially:
        ```
        Skill("jc", "{task description}")
        ```
     3. Remove completed tasks from the task list
     4. If no tasks remain: announce "All tasks handled via /jc — no wave needed!", archive the report (Step 3 only), stop
     5. Otherwise, continue to further triage

<!-- OPTIONAL: Domain-specific command triage
#### {DOMAIN_COMMAND} triage

If your project has a domain-specific command (e.g., /ckm for knowledge curation) that produces
non-code artifacts `/build` cannot handle, add a triage block here.

7. **Identify {DOMAIN_COMMAND} tasks.** Scan remaining tasks for `[CMD: /{DOMAIN_COMMAND}]` tags. These are {DOMAIN}-specific tasks that `/build` cannot execute — `/build` agents write code, not {DOMAIN} knowledge files. A {DOMAIN_COMMAND} task authors curated content under `{project-cortex}/knowledge/`.

8. **Auto-handle {DOMAIN_COMMAND} tasks BEFORE `/build` pipelines — no confirmation:**
   - If {DOMAIN_COMMAND} tasks found:
     1. Log them under `## {DOMAIN_COMMAND} Pre-flight` in the report
     2. Run each sequentially:
        ```
        Skill("{DOMAIN_COMMAND}", "{task description}")
        ```
     3. Remove completed tasks from the task list
     4. If no tasks remain: announce "All tasks handled via /{DOMAIN_COMMAND} — no /build needed!", archive the report (Step 3 only), stop
     5. Otherwise, proceed to Step 0d with the remaining tasks
   - If NO {DOMAIN_COMMAND} tasks found: proceed directly to Step 0d

   **Why {DOMAIN_COMMAND} runs before `/build`:** `/build` pipelines may depend on the knowledge/content
   files that `/{DOMAIN_COMMAND}` authors. Running the domain command first ensures the files exist when
   `/build` agents need them.
-->

### Step 0d — Group, plan, and set up

9. Analyze dependencies between tasks — which ones MUST run sequentially vs which can run in parallel.
10. **Grouping — THIS IS THE KEY STEP.** Group tasks aggressively by similarity before creating waves:

   - **Same-project, same-scope tasks go together.** If you have 5 small FE-only UI tweaks, they become ONE `/build` run with a combined description, not 5 separate pipelines. Same for BE-only, {AI_PROJECT_SHORT}-only, or Infra-only tasks.
   - **Same-routing tasks go together.** Tasks that touch the same set of projects (e.g., all BE+FE cross-project tasks) should be grouped into a single `/build` run.
   - **Cross-project consolidation — PREFER one pipeline over many.** When tasks touch different projects but have NO dependency conflicts or file overlaps, merge them into ONE cross-project `/build` run. `/build` natively handles CROSS routing — it creates a single worktree with all needed subprojects and runs their agents in parallel. A single CROSS pipeline is ALWAYS cheaper than separate single-project pipelines because they share the entire overhead: mono-planner, mono-architect, documentation, QA orchestration, and gitter merge. Example: 6 FE tasks + 1 BE task with no conflicts = ONE pipeline routed as CROSS, not two pipelines (FE-ONLY + BE-ONLY).
   - **Only separate what MUST be separate.** Split into different `/build` runs only when:
     - Tasks have real dependencies (Task B depends on Task A's output)
     - Tasks touch conflicting files (same file with incompatible changes)
     - Tasks are large/complex enough to warrant their own pipeline (major features, not tweaks)
   - **Small related tasks = one pipeline.** 3-8 small related tasks should become a single `/build` with a combined feature description like: "FE polish: fix button alignment, update error messages, add loading spinner to dashboard, adjust sidebar padding"
   - **Merge single-pipeline sequential waves.** If Wave N is a single pipeline and Wave N+1 is also a single pipeline touching the same project, merge them into ONE pipeline in ONE wave. A single developer agent modifying the same file once with all changes is less risky than two developer agents modifying it in sequence — no merge needed. The file overlap concern that motivates sequencing is actually better handled inside a single pipeline. Only keep them separate if they are truly incompatible (e.g., Task B literally cannot be designed until Task A's output exists at runtime, not just in the same file).
   - **When in doubt, group more aggressively.** One pipeline handling 5 related tasks is far cheaper AND faster end-to-end than 5 pipelines each handling 1 task. Each pipeline has overhead (planner, architect, QA — all multiplied).

   **How to write combined `/build` descriptions:**
   Give `/build` a concise combined description with pipeline name and wave tokens. Detailed task specs live in the pre-placed manifest — keep `$ARGUMENTS` short:
   ```
   "FE polish: fix button alignment, update error messages, add loading spinner, adjust sidebar padding [Pipeline: fe-polish] [Wave: {wave-name}]"
   ```

11. Organize groups into **waves** — each wave is a set of `/build` pipelines that (in principle) could run in parallel. Remember: Skill tool runs pipelines sequentially; wave boundaries only enforce dependency ordering.
12. **Pre-place pipeline manifests.** For each pipeline, create its doc directory and write the task manifest containing ONLY the tasks assigned to that pipeline:

   ```bash
   mkdir -p docs/dev/tasks/{pipeline-name}
   ```

   Use the Write tool to save the pipeline-specific task manifest to `docs/dev/tasks/{pipeline-name}/0-task.md`:

   ```markdown
   # Task: {pipeline-name}

   Wave: {wave-name}

   {The grouped tasks assigned to this pipeline — extracted from the wave spec.
    For single-pipeline waves this is the full wave spec content.
    For multi-pipeline waves it is the relevant subset only.}
   ```

   `/build` detects pre-placed manifests and reads them as-is — it will NOT overwrite.

13. Write the **wave source-of-truth** — the full refined task spec:
   Use the Write tool to save the refined task file to `$WAVES/{wave-name}/wave.md`.
14. Create `$WAVES/{wave-name}/report.md` with the initial plan:

```markdown
# Wave Report: {wave-name}

**Task file:** {original-task-file-name}
**Wave name:** {wave-name}
**Started:** {timestamp}
**Total tasks:** N (original) -> J via /jc + M pipelines
**Waves:** W

## JC Pre-flight (if any)
{List tasks handled via /jc before the wave, or omit this section if none}

## Grouping Summary

| Pipeline | Tasks included | Routing |
|----------|---------------|---------|
| `{pipeline-a}` | Task 1, Task 2, Task 3 | FE-ONLY |
| `{pipeline-b}` | Task 4 | BE+FE |
| `{pipeline-c}` | Task 5, Task 6 | BE-ONLY |

## Execution Plan

### Wave 1 (parallel)
- [ ] `{pipeline-a}` — FE polish group (3 tasks)
- [ ] `{pipeline-c}` — BE fixes group (2 tasks)

### Wave 2 (parallel, depends on Wave 1)
- [ ] `{pipeline-b}` — cross-project feature (1 task)

---

## Progress Log
```

---

## Step 1 — Execute waves sequentially, grouped pipelines within each wave in parallel

For each wave:

1. **Log the wave start** in `$WAVES/{wave-name}/report.md`:
   ```
   ### Wave N — Started {timestamp}
   ```

2. **Launch pipelines using the Skill tool directly.** Invoke `/build` via `Skill("build", ...)` from the main conversation — NEVER delegate Skill calls to sub-agents (sub-agents do not have access to the Skill tool).

   **CRITICAL LIMITATION:** The Skill tool cannot be called from inside an Agent. This means:
   - **Pipelines within a wave run SEQUENTIALLY**, not in parallel
   - The wave runner invokes `/build` directly, one pipeline at a time
   - This is the correct and only working pattern — do NOT attempt to parallelize via Agent wrappers

   ```
   Skill("build", "{concise-description-for-group-A} [Pipeline: {pipeline-a-name}] [Wave: {wave-name}]")
   # Wait for completion, log result
   Skill("build", "{concise-description-for-group-B} [Pipeline: {pipeline-b-name}] [Wave: {wave-name}]")
   # Wait for completion, log result
   ```

   **Bracket tokens are parseable — do not reword them.** `/build` extracts `[Pipeline: ...]` to use the wave's chosen pipeline name (skipping its own naming step) and `[Wave: ...]` to forward to gitter for commit trailers. The `$ARGUMENTS` is a **concise description**, not the full wave content — detailed task specs are pre-placed in `docs/dev/tasks/{pipeline-name}/0-task.md`.

   Each "pipeline" may contain multiple original tasks grouped together — grouping is the primary token-saving mechanism since parallelism across pipelines is not available.

3. **Wait for the pipeline to complete** before starting the next one in the wave.

4. **Log each pipeline result** in `$WAVES/{wave-name}/report.md`:

   On success:
   ```
   - [x] `{pipeline-a}` (3 tasks) — **DONE**
   - [x] `{pipeline-c}` (2 tasks) — **DONE**
   ```

   On failure (pipeline ran to completion but final QA / merge fundamentally failed):
   ```
   - [x] `{pipeline-b}` (1 task) — **FAILED** — {reason}
   ```

   On deferred (`/build` aborted via Fix Loop Escalation in `build.md` — wrote `$DOCS/BLOCKED.md`, preserved worktree):
   ```
   - [ ] `{pipeline-d}` (3 tasks) — **BLOCKED-DEFERRED**
     - Trigger: {iteration-cap | hung-test | repeat-bug | sub-agent-orphan}
     - Worktree preserved: `.worktrees/{pipeline-d}/`
     - Resume note: `docs/dev/tasks/{pipeline-d}/BLOCKED.md`
     - Next action: `/jc` the underlying bug, then resume per BLOCKED.md protocol
   ```

5. **Proceed to the next wave** only after all pipelines in the current wave are complete.

   **BLOCKED-DEFERRED handling — the wave continues, it does NOT stop.** A deferred pipeline does NOT block downstream pipelines unless a downstream task explicitly requires its output. Re-read each subsequent pipeline's task definition in `wave.md` and grep for explicit cross-references to the deferred pipeline's task numbers or output artifacts:
   - **No explicit dependency found** -> run the pipeline normally.
   - **Explicit dependency found** -> also defer the dependent pipeline (write its own `BLOCKED.md` noting it is gated on the upstream defer), do not attempt to run it.
   - **Ambiguous** -> default to running the downstream pipeline (the safer call — deferred pipelines rarely produce missing artifacts, and the downstream architect will catch real dependency gaps during analysis). Log the decision in the report.

---

## Step 2 — Final report

After all waves complete, update `$WAVES/{wave-name}/report.md` with the final summary:

```markdown
## Final Summary

**Completed:** {timestamp}
**Total tasks:** N (grouped into M pipelines)
**Pipelines succeeded:** X
**Pipelines failed:** Y
**Pipelines deferred:** Z (worktrees preserved — see `BLOCKED.md` in each pipeline doc dir; resume after `/jc`)

### Results
| Pipeline | Tasks included | Status | Notes |
|----------|---------------|--------|-------|
| `{pipeline-a}` | Task 1, 2, 3 | DONE | ... |
| `{pipeline-b}` | Task 4 | DONE | ... |
| `{pipeline-d}` | Task 6, 7, 8 | BLOCKED-DEFERRED | Trigger: hung-test. Worktree preserved. |
```

---

## Step 3 — Archive (NON-OPTIONAL — execute BEFORE Professor review)

**This step has been skipped in past waves due to context exhaustion. It is a blocking requirement. If you are running low on context, archive FIRST, then skip the Professor review — never the other way around.**

1. Archive the wave directory (handle pre-existing partial archives):
   ```bash
   mkdir -p $WAVES/archive
   # Remove any partial archive from a previous failed attempt
   rm -rf $WAVES/archive/{wave-name}
   mv $WAVES/{wave-name} $WAVES/archive/{wave-name}
   ```

2. Remove the original task file ONLY if it is NOT `wave.md` at repo root (that file is the user's reusable task list — never delete it):
   ```bash
   # Only delete if the original path is NOT ./wave.md or wave.md at repo root
   if [[ "{original-task-file-path}" != "wave.md" && "{original-task-file-path}" != "./wave.md" ]]; then
     rm -f {original-task-file-path}
   fi
   ```

3. **Verify the archive succeeded:**
   ```bash
   # Active directory must be GONE, archive must EXIST
   test ! -d $WAVES/{wave-name} && test -d $WAVES/archive/{wave-name} && echo "ARCHIVE_OK" || echo "ARCHIVE_FAILED"
   ```
   If `ARCHIVE_FAILED`, investigate and fix before continuing.

4. Announce completion:
   ```
   "Wave complete ({wave-name}). {X}/{N} succeeded. Archived to $WAVES/archive/{wave-name}/."
   ```

---

## Step 4 — Professor Review (best-effort — skip if context is critically low)

After archiving, invoke the Professor to review the wave's execution — how it operated, what went well, what could improve. Hand him the archived wave report as input.

Use the Skill tool:

```
Skill("professor", "wave-review $WAVES/archive/{wave-name}/report.md")
```

Wait for the Professor's review. When it comes back:
1. Append the Professor's review to `$WAVES/archive/{wave-name}/report.md` under a new `## Professor's Wave Review` section
2. Present the Professor's review to the user

**If context is critically low:** Skip this step — archive is the priority. The Professor review is valuable but not blocking.

---

## Rules

- **Always group aggressively.** Grouping IS the optimization — it saves both tokens AND wall-clock time since pipelines run sequentially and each one pays full overhead (mono-planner, mono-architect, child planners/architects/developers/QA, merge, archive). Never spawn 5 pipelines for 5 small tasks that could be one pipeline. Prefer ONE cross-project pipeline over separate single-project pipelines when tasks don't conflict (`/build` handles CROSS routing natively).
- **NEVER delegate Skill calls to sub-agents.** Sub-agents do not have access to the Skill tool. Always invoke `/build` via `Skill("build", ...)` directly from the wave runner conversation. Attempting to wrap Skill calls in Agent() will silently fail or cause the agent to improvise its own broken orchestration.
- **Each grouped pipeline is a full `/build` run** — no shortcuts, no skipping QA. But one pipeline can handle multiple related tasks.
- **Document every step** — the report file must be kept up to date after each pipeline completes.
- **Fully autonomous once launched** — once `/wave` is invoked, it does not pause, ask for permission, or request confirmation at any point. Pre-flight validation (Step 0b) is the only gate: if validation passes, the wave runs to completion. If validation fails, the wave stops immediately with a diagnostic before creating any directory or starting any pipeline. Mid-run ambiguity is never escalated to the user — make the call from codebase context and log the decision.
- **If a pipeline fails**, log it and continue with remaining pipelines. Don't block the whole wave.
- **If a pipeline returns BLOCKED-DEFERRED** (`/build` wrote `$DOCS/BLOCKED.md` per its Fix Loop Escalation), do NOT mark it failed and do NOT touch its worktree. Log it as `BLOCKED-DEFERRED` per Step 1 step 4, then assess downstream pipelines for explicit dependency on its output (per Step 1 step 5). The wave continues. Past incident: runaway fix loops silently torching wall-time before this escalation existed drove this rule.
- **Re-group freely** — ignore the user's section headings. Group by project routing and similarity, not by how the user organized the task file.
- **Archive is NON-NEGOTIABLE.** Every completed wave MUST be archived before the conversation ends. Archive (Step 3) runs BEFORE Professor Review (Step 4) — if context is low, skip the review but NEVER skip the archive. A wave sitting in `$WAVES/` with a Final Summary but no archive is a bug. Past incidents: waves left unarchived due to context exhaustion — this rule exists because of that.
- **Clean up after yourself** — after the wave completes, verify no orphaned worktrees, stale branches, or unreleased merge locks remain. Run `git worktree list` and check `.worktrees/.merge-lock/` **at the monorepo root** (never use a relative path — see gitter.md for why). Also sweep for stray lock dirs inside child projects: `ls {project-be}/{project-fe}/{project-cortex}/{project-infra}/{project-web}/.worktrees 2>/dev/null` should return nothing. **Exception:** worktrees for `BLOCKED-DEFERRED` pipelines (those whose `docs/dev/tasks/{name}/BLOCKED.md` exists) are intentional preservations — leave them in place. Sanity-check by listing each worktree against the deferred set and only flagging worktrees that have NO matching `BLOCKED.md`.
