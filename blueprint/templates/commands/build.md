# Cross-Project Build Pipeline

Run the full {PROJECT_NAME} pipeline for: $ARGUMENTS

**All feature requests MUST go through this pipeline.** No cowboy coding.
**All development happens from this root — there are no child build commands.**
**Autonomous execution contract: once started, `/build` never stops mid-run to ask questions or wait for approval. Pre-flight validation runs before any pipeline work. If validation passes, the pipeline runs to completion. The only defined stop points are: pre-flight failure (before any work starts) and Fix Loop Escalation → BLOCKED-DEFERRED (preserves work for `/jc` resolution).**

---

## Pre-flight — validate before starting any work

Check `$ARGUMENTS` for fatal unrunnability before creating any directory or allocating any port:

- **Coherence:** Is the description specific enough to route? A bare "fix things" or "improve stuff" with zero context cannot be planned. → `PRE-FLIGHT FAILED: Description too vague — what specifically needs to change? No pipeline started.`
- **Self-consistency:** Does the description contradict itself? → Stop with the contradiction noted.

If pre-flight fails: STOP. Return the diagnostic. Do NOT proceed.

---

<!-- OPTIONAL: Dual-runtime support
Both Claude and {SECONDARY_RUNTIME} read this same file and execute the pipeline natively — there is no cross-runtime routing. {SECONDARY_RUNTIME}-specific adaptations (git ownership, Skill→Agent translation) live in the corresponding adapter config.
-->

---

## Step 0 — Name the pipeline, clean up stale dirs, and resolve paths

### 0a. Stale pipeline cleanup (MANDATORY pre-flight)

Before doing anything, check for abandoned pipeline directories in `docs/dev/builds/` (excluding `archive/`).
A pipeline directory is **stale** if it has NO corresponding active worktree in `.worktrees/`:

```bash
for dir in docs/dev/builds/*/; do
  name=$(basename "$dir")
  [ "$name" = "archive" ] && continue
  if [ ! -d ".worktrees/$name" ]; then
    echo "STALE: $dir (no active worktree)"
  fi
done
```

**For each stale directory found:**
- If it contains a `BLOCKED.md` → it is intentionally preserved (deferred for `/jc` resolution — see § Fix Loop Escalation). **SKIP cleanup.** Do NOT archive, do NOT delete. Print `PRESERVED: $dir (BLOCKED-DEFERRED, awaiting resume)` and move on.
- **If it belongs to an active wave** → **SKIP.** Wave-owned builds are NEVER archived individually — they archive together when the wave archives. Detection: `grep -rl "$name" docs/dev/waves/*/report.md 2>/dev/null`. If any match, print `WAVE-OWNED: $dir (belongs to active wave, skipping)` and move on.
- If it contains a `7-post-merge-qa.md` → it completed but wasn't archived. Archive it using the numbered rolling system (see below). **Only for standalone builds (no wave owner).**
- If it has NO completion markers (no `7-*` file, no `BLOCKED.md`) → it was abandoned mid-pipeline. Add an `ABANDONED.md` marker, then archive. **Only for standalone builds (no wave owner).**

```bash
echo "Pipeline abandoned — archived during /build pre-flight cleanup on $(date -I)" > docs/dev/builds/$name/ABANDONED.md
```

**Numbered archive (for standalone builds only — NEVER for wave-owned builds):**
```bash
mkdir -p docs/dev/builds/archive tmp/archive/builds
COUNTER=$(cat docs/dev/builds/archive/.counter 2>/dev/null || echo "0")
NEXT=$((COUNTER + 1))
PADDED=$(printf "%03d" $NEXT)
mv docs/dev/builds/$name docs/dev/builds/archive/${PADDED}-${name}
echo "$NEXT" > docs/dev/builds/archive/.counter
# Evict oldest if more than 10
ARCHIVE_COUNT=$(find docs/dev/builds/archive -maxdepth 1 -type d | wc -l)
ARCHIVE_COUNT=$((ARCHIVE_COUNT - 1))
if [ "$ARCHIVE_COUNT" -gt 10 ]; then
  OLDEST=$(ls -d docs/dev/builds/archive/[0-9]*/ | head -1)
  mv "$OLDEST" tmp/archive/builds/
fi
```

**Empty directories** (zero files) → just remove them: `rmdir docs/dev/builds/$name`

This prevents stale pipeline dirs from accumulating across sessions.

### 0b. Name the pipeline

**If `$ARGUMENTS` contains `[Pipeline: {name}]`:** Extract and use that name — the wave runner pre-assigned it, pre-placed the task manifest at `docs/dev/builds/{name}/0-task.md`, and already ran uniqueness checks. Skip name generation and uniqueness check below; proceed directly to path variable resolution.

**Otherwise (standalone invocation):** Choose a short, descriptive kebab-case name based on the feature (e.g., `session-notes`, `audio-streaming`).

**Name uniqueness check (standalone only — skip when `[Pipeline: ...]` is present):** Before proceeding, verify the chosen name does NOT already exist in:
- `docs/dev/builds/archive/` — archived pipelines
- `docs/dev/builds/` — active pipelines
- `.worktrees/` — active worktrees

```bash
ls docs/dev/builds/archive/ | sed 's/^[0-9]*-//' | grep -x "{name}"
ls docs/dev/builds/ .worktrees/ 2>/dev/null | grep -x "{name}"
```

The first `ls` strips counter prefixes (e.g., `003-radar-surfaces` → `radar-surfaces`) before matching. If the name exists anywhere, append a version suffix (e.g., `session-notes-v2`, `audio-streaming-v3`) or choose a more specific name. Also check `tmp/archive/builds/` for evicted archives. **NEVER reuse an archived pipeline name** — it causes doc conflicts and breaks traceability.

Resolve path variables:
- **`$PIPELINE`** = `{name}` — the pipeline name (kebab-case, unique across active + archived). Extracted from `[Pipeline: {name}]` in `$ARGUMENTS` when present (wave-invoked), otherwise chosen by build (standalone).
- **`$WAVE`** = wave name extracted from `[Wave: {wave-name}]` in `$ARGUMENTS`, otherwise `none`. This value is forwarded to gitter so merge + docs commits carry a `Wave:` trailer for git-history traceability back to `docs/dev/waves/archive/{wave}/`.
- **`$DOCS`** = `docs/dev/builds/{name}` — pipeline docs from repo root
- **`$DOCS_REL`** = `../../../docs/dev/builds/{name}` — pipeline docs from worktree
- **`$DOCS_POST`** = `../docs/dev/builds/{name}` — pipeline docs from project subdir (POST-MERGE)
- **`$WORKTREE`** = `.worktrees/{name}` — pipeline worktree directory (full monorepo checkout)
- **`$ARCHIVE`** = `docs/dev/builds/archive` — archive parent directory
- **`$PROJECTS`** = list of affected projects (e.g., `be`, `fe`, `{AI_PROJECT_KEY}`, `infra`) — set by mono-planner routing
- **Pipeline branch:** `pipeline/{name}` (single branch for all projects)
- **Backend in worktree:** `$WORKTREE/{project-be}`
- **Frontend in worktree:** `$WORKTREE/{project-fe}`
- **{AI_PROJECT_LABEL} in worktree:** `$WORKTREE/{project-cortex}`
- **Web in worktree:** `$WORKTREE/{project-web}`
- **Infrastructure in worktree:** `$WORKTREE/{project-infra}`

```bash
mkdir -p docs/dev/builds/{name}
```

**Write the task manifest** — idempotent, wave runner pre-places this when invoked from `/wave`:
```bash
[ -f docs/dev/builds/{name}/0-task.md ] && echo "manifest exists — wave pre-placed it" || echo "manifest missing — standalone build"
```
- **Exists** → read it as-is, do NOT overwrite. Wave wrote the pipeline-specific task spec here.
- **Missing** (standalone build only) → write it now:
  ```markdown
  # Task: {name}

  {verbatim $ARGUMENTS — stripped of [Wave: ...] and [Pipeline: ...] tokens}

  Wave: {$WAVE or none}
  ```

**Pass `$PIPELINE`, `$DOCS`, `$DOCS_REL`, and `$WORKTREE` to every agent invocation.** Agents should never hardcode doc or worktree paths — they use what you give them.

---

## Step 1a — Parallel Codebase Analysis (child planners)

Spawn ALL FIVE child planners **in parallel** (single message, five Agent calls).
**Model: sonnet** — child planners do structured analysis, not strategic decisions.

```
Agent(general-purpose, model: "sonnet"): "You are the backend planner. Read and follow {project-be}/.claude/agents/planner.md.
  Mode: ANALYSIS. Pipeline: {name}. Feature: {feature request}.
  Analyze the {project-be}/ codebase and write $DOCS/1-analysis-be.md."

Agent(general-purpose, model: "sonnet"): "You are the frontend planner. Read and follow {project-fe}/.claude/agents/planner.md.
  Mode: ANALYSIS. Pipeline: {name}. Feature: {feature request}.
  Analyze the {project-fe}/ codebase and write $DOCS/1-analysis-fe.md."

Agent(general-purpose, model: "sonnet"): "You are the {AI_PROJECT_LABEL} planner. Read and follow {project-cortex}/.claude/agents/planner.md.
  Mode: ANALYSIS. Pipeline: {name}. Feature: {feature request}.
  Analyze the {project-cortex}/ codebase and write $DOCS/1-analysis-{AI_PROJECT_KEY}.md."

Agent(general-purpose, model: "sonnet"): "You are the web planner. Read and follow {project-web}/.claude/agents/planner.md.
  Mode: ANALYSIS. Pipeline: {name}. Feature: {feature request}.
  Analyze the {project-web}/ codebase and write $DOCS/1-analysis-web.md."

Agent(general-purpose, model: "sonnet"): "You are the infrastructure planner. Read and follow {project-infra}/.claude/agents/planner.md.
  Mode: ANALYSIS. Pipeline: {name}. Feature: {feature request}.
  Analyze the {project-infra}/ codebase and write $DOCS/1-analysis-infra.md."
```

**All five MUST be launched in a single message for true parallelism.** Wait for all to complete.

**Idempotency guard:** Before spawning, check which analysis reports already exist: `ls $DOCS/1-analysis-*.md 2>/dev/null`. Only spawn planners for projects whose `1-analysis-{project}.md` does NOT yet exist. If all 5 reports already exist, skip Step 1a entirely and proceed to Step 1b. This prevents duplicate planner launches if Step 1a is re-entered after a partial execution.

---

## Step 1b — Consolidation (mono-planner agent)

Use the `mono-planner` agent. **Model: claude-opus-4-6** — strategic routing decisions.
- Tell it: "Pipeline: {name}. Feature: {feature request}. Read the five analysis reports at $DOCS/1-analysis-{be,fe,{AI_PROJECT_KEY},web,infra}.md and consolidate into $DOCS/1-plan.md."
- It reads the parallel analysis reports, decides routing (BE-ONLY, FE-ONLY, {AI_PROJECT_KEY_UPPER}-ONLY, WEB-ONLY, INFRA-ONLY, CROSS), and writes `$DOCS/1-plan.md`.
- Wait for completion. Read the plan to get the routing decision before proceeding.

---

## Step 2 — Git Setup (worktrees + ports)

Use the `gitter` agent in **SETUP** phase. **Model: sonnet** — structured git ops.
- Tell it: "Pipeline: {name}. Phase: SETUP."
- Creates a single monorepo worktree with all projects (one branch: `pipeline/{name}`)
- Developers work in their respective subdirs within the same worktree
- Wait for confirmation before proceeding.

---

## Step 3 — Cross-Project Architecture (mono-architect)

Use the `mono-architect` agent. **Model: claude-opus-4-6** — critical cross-project architecture decisions + inline research.
- Tell it: "Pipeline: {name}. Read $DOCS/1-plan.md. Write $DOCS/3-architecture.md."
- It designs API contracts, shared types, and integration patterns — but makes NO code-level decisions or TODO stubs.
- Skip if routing is single-project-only (BE-ONLY, FE-ONLY, {AI_PROJECT_KEY_UPPER}-ONLY) with no integration changes.
- Wait for completion.

---

## Step 4 — Child Architecture (parallel if CROSS)

Spawn child architect agents. They read mono-architect's integration contracts and write
project-specific architecture docs to `$DOCS/`. Architects produce docs only — no code stubs in worktrees.
Developers derive their work queue from the architecture docs directly.
**Model: sonnet** — child architects follow mono-architect's spec.

```
Agent(general-purpose, model: "sonnet"): "You are the backend architect. Read and follow the instructions in {project-be}/.claude/agents/architect.md.
  Pipeline: {name}.
  All pipeline docs: $DOCS/.
  Write your architecture doc to $DOCS/3-architecture-be.md.
  You produce the architecture doc ONLY — no code stubs. The developer derives their work queue from your doc.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the frontend architect. Read and follow the instructions in {project-fe}/.claude/agents/architect.md.
  Pipeline: {name}.
  All pipeline docs: $DOCS/.
  Write your architecture doc to $DOCS/3-architecture-fe.md.
  You produce the architecture doc ONLY — no code stubs. The developer derives their work queue from your doc.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the {AI_PROJECT_LABEL} architect. Read and follow the instructions in {project-cortex}/.claude/agents/architect.md.
  Pipeline: {name}.
  All pipeline docs: $DOCS/.
  Write your architecture doc to $DOCS/3-architecture-{AI_PROJECT_KEY}.md.
  You produce the architecture doc ONLY — no code stubs. The {AI_DEVELOPER_ROLE} derives their work queue from your doc.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the web architect. Read and follow the instructions in {project-web}/.claude/agents/architect.md.
  Pipeline: {name}.
  All pipeline docs: $DOCS/.
  Write your architecture doc to $DOCS/3-architecture-web.md.
  You produce the architecture doc ONLY — no code stubs. The developer derives their work queue from your doc.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the infrastructure architect. Read and follow the instructions in {project-infra}/.claude/agents/architect.md.
  Pipeline: {name}.
  All pipeline docs: $DOCS/.
  Write your architecture doc to $DOCS/3-architecture-infra.md.
  You design the big picture blueprint — the DevOps engineer implements from your doc.
  NEVER run git commands — gitter handles all commits."
```

Spawn only the relevant architects based on routing. Wait for completion.

---

## Step 5 — UI/UX Design + Database Architecture (conditional, parallel)

Check `$DOCS/1-plan.md` for frontend visual tasks AND schema changes.
Spawn both agents in a single message if both are needed (they're independent):

### Step 5a — UI/UX Design (conditional)

**If FE visual work is needed:**

```
Agent(general-purpose, model: "sonnet"): "You are the UI/UX designer. Read and follow {project-fe}/.claude/agents/ui-ux.md.
  Pipeline: {name}. All pipeline docs: $DOCS/.
  Read $DOCS/3-architecture.md and $DOCS/3-architecture-fe.md.
  Write your spec to $DOCS/4-ui-ux-spec.md."
```

**If no FE visual work**, skip.

### Step 5b — Database Architecture (conditional — but CHECK EXPLICITLY)

**Detection rule (MANDATORY — do NOT skip this check):** Grep the plan (`$DOCS/1-plan.md`) and
architecture docs for ANY of these signals: `table`, `schema`, `column`, `index`, `enum`, `migration`,
`{SCHEMA_DEFINITION}`, `{ORM}`, `database`. If ANY signal is found, db-admin MUST be invoked.

**This check exists because pipelines have shipped new tables without migration files.
The orchestrator's "judgment call" is not reliable enough — use keyword detection.**

If schema signals are found, spawn the db-admin agent:

```
Agent(general-purpose, model: "sonnet"): "You are the database admin. Read and follow {project-infra}/.claude/agents/db-admin.md.
  Pipeline: {name}.
  All pipeline docs: $DOCS/.
  BE worktree: $WORKTREE/{project-be}. {AI_PROJECT_LABEL} worktree: $WORKTREE/{project-cortex}. Infra worktree: $WORKTREE/{project-infra}.
  Read architecture docs at $DOCS/ and implement schema changes.
  CRITICAL: Every column in the schema MUST have a corresponding SQL migration — either in a CREATE TABLE
  statement or an ALTER TABLE ADD COLUMN statement. This applies to NEW TABLES ({SCHEMA_DEFINITION}) AND new columns
  on existing tables. Count existing files in $WORKTREE/{project-be}/{MIGRATIONS_DIR}/ to determine the next
  migration number. Run your column-level completeness check before finishing — it is BLOCKING.
  Write your database architecture doc to $DOCS/4-db-architecture.md.
  NEVER run git commands — gitter handles all commits."
```

**If no schema signals found in plan or architecture**, skip — but log: "Step 5b: no schema signals detected, skipping db-admin."

---

## Step 6 — Parallel Development (on named worktrees)

Read ports from `$DOCS/ports.md`, then launch agents for the relevant projects.
**Model: sonnet** — developers implement specs, they don't design them.

**Trivial infra tasks (env var additions, config tweaks):** If the infra scope is only adding env vars to non-infra project files (e.g., adding vars to `{project-cortex}/.env.local`), the orchestrator MAY handle it directly instead of spawning a full DevOps agent. Sub-agents sometimes get permission-blocked on `.worktrees/` paths — for 3-line edits, doing it yourself is faster and more reliable.

```
Agent(general-purpose, model: "sonnet"): "You are the backend developer. Read and follow {project-be}/.claude/agents/developer.md.

  Pipeline: {name}.
  Worktree: $WORKTREE/{project-be}. Branch: pipeline/{name}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  IMPORTANT — $DOCS_REL resolves to the ROOT docs directory, NOT to docs/ inside your worktree.
  Example: from $WORKTREE/{project-be}/, $DOCS_REL = ../../../docs/dev/builds/{name}/.
  Write to $DOCS_REL/5-dev-report-be.md. NEVER write to .worktrees/{name}/docs/ — that's inside the worktree and will be lost.
  Backend port: {be_port}.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the frontend developer. Read and follow {project-fe}/.claude/agents/developer.md.

  Pipeline: {name}.
  Worktree: $WORKTREE/{project-fe}. Branch: pipeline/{name}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  IMPORTANT — $DOCS_REL resolves to the ROOT docs directory, NOT to docs/ inside your worktree.
  Example: from $WORKTREE/{project-fe}/, $DOCS_REL = ../../../docs/dev/builds/{name}/.
  Write to $DOCS_REL/5-dev-report-fe.md. NEVER write to .worktrees/{name}/docs/ — that's inside the worktree and will be lost.
  Frontend port: {fe_port}, backend port: {be_port}.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the {AI_PROJECT_LABEL} {AI_DEVELOPER_ROLE}. Read and follow {project-cortex}/.claude/agents/{AI_DEVELOPER_AGENT}.md.

  Pipeline: {name}.
  Worktree: $WORKTREE/{project-cortex}. Branch: pipeline/{name}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  IMPORTANT — $DOCS_REL resolves to the ROOT docs directory, NOT to docs/ inside your worktree.
  Example: from $WORKTREE/{project-cortex}/, $DOCS_REL = ../../../docs/dev/builds/{name}/.
  Write to $DOCS_REL/5-dev-report-{AI_PROJECT_KEY}.md. NEVER write to .worktrees/{name}/docs/ — that's inside the worktree and will be lost.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the web developer. Read and follow {project-web}/.claude/agents/developer.md.

  Pipeline: {name}.
  Worktree: $WORKTREE/{project-web}. Branch: pipeline/{name}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  IMPORTANT — $DOCS_REL resolves to the ROOT docs directory, NOT to docs/ inside your worktree.
  Example: from $WORKTREE/{project-web}/, $DOCS_REL = ../../../docs/dev/builds/{name}/.
  Write to $DOCS_REL/5-dev-report-web.md. NEVER write to .worktrees/{name}/docs/ — that's inside the worktree and will be lost.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the infrastructure DevOps engineer. Read and follow {project-infra}/.claude/agents/devops.md.

  Pipeline: {name}.
  Worktree: $WORKTREE/{project-infra}. Branch: pipeline/{name}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  IMPORTANT — $DOCS_REL resolves to the ROOT docs directory, NOT to docs/ inside your worktree.
  Example: from $WORKTREE/{project-infra}/, $DOCS_REL = ../../../docs/dev/builds/{name}/.
  Write to $DOCS_REL/5-dev-report-infra.md. NEVER write to .worktrees/{name}/docs/ — that's inside the worktree and will be lost.
  If you get permission-blocked on worktree file edits, use Bash with append/write commands as fallback.
  NEVER run git commands — gitter handles all commits."
```

Launch only the relevant developers/engineers/DevOps based on routing. Wait for completion.

---

## Step 7 — QA (BEFORE merge)

**CRITICAL: QA runs against the worktree branches, NOT main.**

Spawn QA agents for the relevant projects.
**Model: sonnet** — QA runs structured validation checklists.

```
Agent(general-purpose, model: "sonnet"): "You are the backend QA engineer. Read and follow {project-be}/.claude/agents/qa.md.

  Mode: PRE-MERGE. Pipeline: {name}.
  Worktree: $WORKTREE/{project-be}. Port: {be_port}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  Write bug report to $DOCS_REL/ — NEVER to docs/ inside the worktree."

Agent(general-purpose, model: "sonnet"): "You are the frontend QA engineer. Read and follow {project-fe}/.claude/agents/qa.md.

  Mode: PRE-MERGE. Pipeline: {name}.
  Worktree: $WORKTREE/{project-fe}. Ports: FE {fe_port}, BE {be_port}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  Write bug report to $DOCS_REL/ — NEVER to docs/ inside the worktree."

Agent(general-purpose, model: "sonnet"): "You are the {AI_PROJECT_LABEL} QA engineer. Read and follow {project-cortex}/.claude/agents/qa.md.

  Mode: PRE-MERGE. Pipeline: {name}.
  Worktree: $WORKTREE/{project-cortex}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  Write bug report to $DOCS_REL/ — NEVER to docs/ inside the worktree."

Agent(general-purpose, model: "sonnet"): "You are the web QA engineer. Read and follow {project-web}/.claude/agents/qa.md.

  Mode: PRE-MERGE. Pipeline: {name}.
  Worktree: $WORKTREE/{project-web}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  Write bug report to $DOCS_REL/ — NEVER to docs/ inside the worktree."

Agent(general-purpose, model: "sonnet"): "You are the infrastructure QA engineer. Read and follow {project-infra}/.claude/agents/qa.md.

  Mode: PRE-MERGE. Pipeline: {name}.
  Worktree: $WORKTREE/{project-infra}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  Write bug report to $DOCS_REL/ — NEVER to docs/ inside the worktree."
```

Spawn QA agents only for relevant projects. Each agent writes:
- `$DOCS/6-bugs-{be,fe,{AI_PROJECT_KEY},web,infra}.md` — bug list per project

After all QA agents complete, **consolidate** the per-project bug files into a single `$DOCS/6-bugs.md`:
- If ALL per-project bug files have `Status: NONE` → write `$DOCS/6-bugs.md` with `Status: NONE`
- If ANY per-project bug file has `Status: OPEN` → write `$DOCS/6-bugs.md` with `Status: OPEN` and list all open bugs from all projects

---

## Fix Loop (BEFORE merge — capped at 3 iterations, never infinite)

QA may have already patched trivial bugs inline (listed under `Inline fixes:` in its bug report header — informational only, no action needed). Everything in `Status: OPEN` is what developer still needs to handle.

**Iteration cap: 3 maximum.** Never run more than 3 fix-loop iterations per pipeline. After iteration 3 fails, escalate to BLOCKED-DEFERRED (see § Fix Loop Escalation below) — do NOT keep looping. Past incidents show runaway fix loops eat orders of magnitude more wall-time than the work they purport to fix.

**Hard timeouts on test commands.** Every test invocation inside the fix loop MUST be wrapped in `timeout 600s <test command>` (10 minutes per invocation). Agent definitions enforce this — see `{project-*}/.claude/agents/{qa,developer,{AI_DEVELOPER_AGENT},devops}.md`. The orchestrator does NOT spawn raw test runs; it spawns agents that own the timeout discipline.

**Hung-process detection.** If any test process sits at 0% CPU for >2 minutes (deadlocked, not slow) the running agent must `kill` it and report `BUG-HUNG-TEST` with the file:line of the hanging test. A hung test is NOT a fix-loop bug — it is a code bug that requires `/jc` on main.

If `$DOCS/6-bugs.md` has `Status: OPEN`:

1. **Developer fixes** — spawn developer agents on their existing worktree branches (same as Step 6, model: sonnet). Developers read `6-bugs.md` directly for bugs with `Status: OPEN`. QA's adversarial tests provide the reproduction — the failing test IS the root cause. Developers debug and fix the code themselves — no separate debugger needed.

2. **Re-run QA** (same as Step 7, model: sonnet)

3. Repeat until `$DOCS/6-bugs.md` has `Status: NONE` OR iteration count reaches 3 OR an escalation trigger fires.

**DO NOT merge until the fix loop completes with zero bugs.**

### Fix Loop Escalation — BLOCKED-DEFERRED

When the fix loop hits ANY of these conditions, abort the loop and mark the pipeline as `BLOCKED-DEFERRED`:

- **Iteration cap reached** — 3 fix-loop iterations passed, bugs still `OPEN`.
- **Hung test detected** — any QA report contains `BUG-HUNG-TEST` (a deterministic deadlock; no amount of fix-loop iterations will fix code that hangs at 0% CPU).
- **Same bug returns** — the same bug ID (or same failing test) appears in two consecutive QA reports despite a developer fix in between (the fix is wrong; no point re-trying).
- **Sub-agent orphan** — a developer or QA sub-agent returns no output / errors out / silently dies without writing its expected report file.

When any condition triggers:

1. **Write `$DOCS/BLOCKED.md`** with:

   ```markdown
   # Pipeline Blocked: {pipeline-name}

   **Status:** BLOCKED-DEFERRED
   **Trigger:** {iteration-cap | hung-test | repeat-bug | sub-agent-orphan}
   **Date:** {YYYY-MM-DD}

   ## Root cause
   {Specific reason — file:line of hung test, bug ID that wouldn't fix, or sub-agent that died.}

   ## State preserved
   - Worktree: `.worktrees/{pipeline-name}/` (NOT torn down)
   - Pipeline docs: `$DOCS/` (all artifacts intact)
   - Ports: still allocated in `.worktrees/.ports`
   - Branch on main: NOT MERGED

   ## Resume protocol
   1. `/jc` the underlying bug on main first (if hung test or stubborn bug). Note the fix commit SHA here: `_______________`
   2. `cd .worktrees/{pipeline-name} && git rebase main` to pick up the fix.
   3. Re-spawn QA agents — skip planners/architects/devs (their work is in the worktree).
   4. If QA passes → gitter MERGE → post-merge QA → audit → documenter (normal pipeline tail).
   5. If QA still fails → ONE more fix-loop iteration max, then re-defer.
   ```

2. **Do NOT** delete the worktree, do NOT release ports, do NOT run gitter MERGE.

3. **Return to the orchestrator with status BLOCKED-DEFERRED.** The wave runner (or human invoker) decides whether to continue, abandon, or resume after `/jc`.

This is intentionally conservative — a deferred pipeline preserves all work done. A failed pipeline with a torn-down worktree loses hours of agent output. The `0a` stale-cleanup rule recognizes `BLOCKED.md` and will NOT auto-archive deferred pipelines.

---

## Merge Phase (only after QA passes with Status: NONE)

### Step 8 — Git Merge + Cleanup

Use the `gitter` agent in **MERGE** phase. **Model: sonnet** — structured git ops.
- Tell it: "Pipeline: {name}. Wave: {$WAVE or 'none'}. Phase: MERGE. Projects: {comma-separated project keys based on routing}."
  - BE-ONLY → `Projects: be`
  - FE-ONLY → `Projects: fe`
  - {AI_PROJECT_KEY_UPPER}-ONLY → `Projects: {AI_PROJECT_KEY}`
  - WEB-ONLY → `Projects: web`
  - CROSS (BE+FE) → `Projects: be,fe`
  - CROSS (BE+FE+{AI_PROJECT_KEY_UPPER}) → `Projects: be,fe,{AI_PROJECT_KEY}`
  - Include `web` if web worktree was used
  - Include `infra` if infrastructure worktree was used

---

## Post-Merge Verification (MANDATORY after every merge)

### Step 9 — Post-Merge QA (on main)

Spawn QA agents for post-merge validation. Since these run from project dirs on `main` (not worktrees), pass `$DOCS_POST` for relative doc access.
**Model: sonnet** — structured validation checklists.

```
Agent(general-purpose, model: "sonnet"): "You are the backend QA engineer. Read and follow {project-be}/.claude/agents/qa.md.

  Mode: POST-MERGE. Pipeline: {name}. Run against {project-be}/ on main.
  Pipeline docs from project dir: $DOCS_POST/. Pipeline docs from root: $DOCS/.
  Follow the runbook at {project-be}/docs/runbook.md.
  Return results inline — do NOT write a per-project report file."

Agent(general-purpose, model: "sonnet"): "You are the frontend QA engineer. Read and follow {project-fe}/.claude/agents/qa.md.

  Mode: POST-MERGE. Pipeline: {name}. Run against {project-fe}/ on main.
  Pipeline docs from project dir: $DOCS_POST/. Pipeline docs from root: $DOCS/.
  Follow the runbook at {project-fe}/docs/runbook.md.
  Return results inline — do NOT write a per-project report file."

Agent(general-purpose, model: "sonnet"): "You are the {AI_PROJECT_LABEL} QA engineer. Read and follow {project-cortex}/.claude/agents/qa.md.

  Mode: POST-MERGE. Pipeline: {name}. Run against {project-cortex}/ on main.
  Pipeline docs from project dir: $DOCS_POST/. Pipeline docs from root: $DOCS/.
  Follow the runbook at {project-cortex}/docs/runbook.md.
  Return results inline — do NOT write a per-project report file."

Agent(general-purpose, model: "sonnet"): "You are the web QA engineer. Read and follow {project-web}/.claude/agents/qa.md.

  Mode: POST-MERGE. Pipeline: {name}. Run against {project-web}/ on main.
  Pipeline docs from project dir: $DOCS_POST/. Pipeline docs from root: $DOCS/.
  Return results inline — do NOT write a per-project report file."

Agent(general-purpose, model: "sonnet"): "You are the infrastructure QA engineer. Read and follow {project-infra}/.claude/agents/qa.md.

  Mode: POST-MERGE. Pipeline: {name}. Run against {project-infra}/ on main.
  Pipeline docs from project dir: $DOCS_POST/. Pipeline docs from root: $DOCS/.
  Return results inline — do NOT write a per-project report file."
```

Spawn only QA agents for projects that were part of this pipeline.

After all post-merge QA agents complete, **write a single** `$DOCS/7-post-merge-qa.md` consolidating the inline results from all agents.

### If Post-Merge QA fails

Spawn a new fix pipeline `{name}-postmerge-fix`:
1. Start a new pipeline named `{name}-postmerge-fix` (creates `$DOCS` for the new name) and write a plan scoped to the bugs found
2. Run the full pipeline cycle: gitter SETUP → architects → developers → QA → fix loop → gitter MERGE
3. Run Post-Merge QA again
4. Repeat until clean

---

### Step 10 — Pipeline Audit (parallel, after post-merge QA passes)

Run a focused code hygiene + compliance audit on the merged code. This catches deeper issues that lint/format don't — security vulnerabilities, compliance violations, dead code, architectural smells, {DOMAIN_DATA_LABEL} exposure, etc.

Spawn TWO agents **in parallel** (single message):

```
Agent(general-purpose, model: "sonnet"): "You are the code auditor. Read and follow .claude/commands/audit.md.
  This is a PIPELINE audit — scope to the projects affected by pipeline {name}: {comma-separated project scopes based on routing, e.g. 'be', 'fe', '{AI_PROJECT_KEY}'}.
  Focus on security (category 8) and code quality (categories 1-7) for the changed code.
  Skip lint/format checks — QA already validated those.
  Read $DOCS/7-post-merge-qa.md for context on what was built.
  Return findings inline — do NOT write to permanent docs."

Agent(general-purpose, model: "sonnet"): "You are the compliance officer. Read and follow .claude/commands/officer.md.
  Mode: audit codebase.
  This is a PIPELINE audit — focus on the code changes from pipeline {name}.
  Scope: the projects affected by this pipeline ({comma-separated project scopes}).
  Check for: PII in logs, missing auth on new endpoints, encryption gaps, {DOMAIN_CONSENT_LABEL} implementation gaps, data handling violations.
  CRITICAL: Read docs/commands/officer/references/todo-ignore.md FIRST — items there are founder-acknowledged and must be
  downgraded to WARNING/INFO per the Todo-Ignore Matching rules in your instructions. Only NEW findings (not in
  todo-ignore.md) can be BLOCKING. Known-deferred items are NON-BLOCKING in pipeline mode.
  Read $DOCS/7-post-merge-qa.md for context on what was built.
  Return findings inline — do NOT update officer reference docs (this is a pipeline check, not a full audit)."
```

After both complete, **write a single** `$DOCS/8-audit.md` consolidating both reports:

```markdown
# Pipeline Audit — $PIPELINE

## Code Audit Findings
{inline results from code auditor}

## Compliance Audit Findings
{inline results from officer}

## Verdict: CLEAN | WARNINGS | BLOCKING

### Blocking Issues (if any)
{security vulnerabilities, compliance violations — MUST be fixed before continuing}

### Warnings (if any)
{code quality findings — documented for future cleanup, do NOT block the pipeline}
```

**If `Verdict: BLOCKING`** — spawn a new fix pipeline `{name}-audit-fix`:
1. Start a new pipeline scoped to the blocking findings
2. Run the full pipeline cycle: gitter SETUP → developers → QA → fix loop → gitter MERGE → post-merge QA → pipeline audit again
3. Repeat until audit is CLEAN or WARNINGS only

**If `Verdict: CLEAN` or `WARNINGS`** — proceed to Step 11.

---

### Step 11 — Documentation & Aggregation (MANDATORY after pipeline audit passes)

Use the `mono-documenter` agent. **Model: sonnet** — structured doc merging.
- Tell it: "Pipeline: {name}. Phase: ARCHIVE. Docs: $DOCS. Archive: $ARCHIVE. Pipeline dir to archive: $DOCS → $ARCHIVE/{name}."

The documenter:
1. **Merges** pipeline decisions into permanent docs (`docs/agents/architecture.md`, `docs/agents/API.md`, child `architecture.md`, `ui-ux.md`, etc.)
2. **Updates** child project docs (`{project-be}/docs/`, `{project-fe}/docs/`, `{project-cortex}/docs/`) with new details
3. **Archives** `$DOCS/` to `$ARCHIVE/{name}/`

All pipeline docs are already in `$DOCS/` — no aggregation needed. Permanent root docs accumulate all decisions. Child project docs get updated with project-specific details.

---

### Step 12 — Commit Docs (MANDATORY after documenter finishes)

Use the `gitter` agent in **DOCS-COMMIT** phase. **Model: sonnet** — structured git ops.
- Tell it: "Pipeline: {name}. Wave: {$WAVE or 'none'}. Phase: DOCS-COMMIT. Projects: {same project keys as MERGE step}."

Gitter commits all doc changes the documenter made on main (including `$DOCS/8-audit.md`).

---

## Pipeline Reference

| # | Step | Who | Produces | Location |
|---|------|-----|----------|----------|
| 1a | Parallel analysis | BE + FE + {AI_PROJECT_KEY_UPPER} + Web + Infra planners | `$DOCS/1-analysis-{be,fe,{AI_PROJECT_KEY},web,infra}.md` | root |
| 1b | Consolidate plan | mono-planner | `$DOCS/1-plan.md` | root |
| 2 | Git setup | gitter (SETUP) | Worktrees, ports, `$DOCS/ports.md` | root |
| 3 | Cross-project arch + research | mono-architect | `$DOCS/3-architecture.md` (integration contracts + research notes) | root |
| 4 | Child arch + research | BE + FE + {AI_PROJECT_KEY_UPPER} + Web + Infra architects | `$DOCS/3-architecture-{be,fe,{AI_PROJECT_KEY},web,infra}.md` (docs only, no code stubs, inline research) | root |
| 5a | UI/UX *(conditional)* | ui-ux | `$DOCS/4-ui-ux-spec.md` | root |
| 5b | DB Architecture *(conditional)* | db-admin | `$DOCS/4-db-architecture.md` + schema/migration changes in worktrees | root (docs) + worktrees (schema) |
| 6 | Develop | BE + FE + Web developers + {AI_PROJECT_KEY_UPPER} {AI_DEVELOPER_ROLE} + Infra devops | Working code in worktrees + `$DOCS/5-dev-report-{be,fe,{AI_PROJECT_KEY},web,infra}.md` | worktrees (code) + root (docs) |
| 7 | QA | BE + FE + {AI_PROJECT_KEY_UPPER} + Web + Infra QA | Adversarial tests in worktrees + `$DOCS/6-bugs-{be,fe,{AI_PROJECT_KEY},web,infra}.md` → consolidated `$DOCS/6-bugs.md` | root |
| - | Fix loop | developers → QA | Repeat until `$DOCS/6-bugs.md` = NONE | |
| 8 | Merge | gitter (MERGE) | Commits + merges to main | |
| 9 | Post-merge QA | BE + FE + {AI_PROJECT_KEY_UPPER} + Web + Infra QA (POST-MERGE) | `$DOCS/7-post-merge-qa.md` (single consolidated file from inline results) | root |
| 10 | Pipeline audit | code-auditor + officer (parallel) | `$DOCS/8-audit.md` (code hygiene + compliance findings) | root |
| 11 | Document | mono-documenter | Merges into permanent docs, archives pipeline to `$ARCHIVE/{name}/` | root |
| 12 | Commit docs | gitter (DOCS-COMMIT) | Commits doc changes on main | root |

---

## Done

When pipeline audit passes, documentation is archived, and doc changes are committed, say: "Build complete ({name}). All tests pass on main. Audit clean. Docs archived and committed."
