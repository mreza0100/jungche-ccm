# Cross-Project Build Pipeline

Run the full {PROJECT_NAME} pipeline for: $ARGUMENTS

**All feature requests MUST go through this pipeline.** No cowboy coding.
**All development happens from this root — there are no child build commands.**
**Autonomous execution contract: once started, `/build` never stops mid-run to ask questions or wait for approval. Pre-flight validation runs before any pipeline work. If validation passes, the pipeline runs to completion. The only defined stop points are: pre-flight failure (before any work starts) and Fix Loop Escalation → BLOCKED-DEFERRED (preserves work for `/jc` resolution). Inventing any other stop — a "needs-a-decision" pause, a "this looks risky" halt — is a contract violation, not caution. A costly, external, or production-affecting action a task requires (paid API call, live deploy) is not a stop: decide from context, take the safest reversible path, and log it. Raise a true blocker as a pre-flight fail-fast, never mid-run.**

**ZERO GAP contract: when the task manifest (`$DOCS/0-task.md`) is a `/p:refine` ZERO-GAP spec — routing, data model, contracts, file plan, and signatures all present — every pipeline agent (planner, architects, developers) IMPLEMENTS and VALIDATES it; none re-decides routing, re-designs the data model or contracts, or re-scopes. Thread this into every agent spawn. Surface a genuine spec flaw (flag to the orchestrator / BLOCKED-DEFERRED), never silently change it. A standalone build given a bare description: agents design as normal.**

**Doc-awareness — thread into every agent spawn:** to understand existing code, consult the grep-true doc clusters — read the project's `docs/architecture/_index.md`, then `grep` the cluster for the exact symbol; the whole DB schema (real {DATABASE} names) is `docs/agents/graph/db/postgres.mmd`. Doc identifiers match code/DB names verbatim.

---

## Pre-flight — validate before starting any work

Check `$ARGUMENTS` for fatal unrunnability before creating any directory or allocating any port:

- **Coherence:** Is the description specific enough to route? A bare "fix things" or "improve stuff" with zero context cannot be planned. → `PRE-FLIGHT FAILED: Description too vague — what specifically needs to change? No pipeline started.`
- **Self-consistency:** Does the description contradict itself? → Stop with the contradiction noted.
- **Uncommitted work on main:** read-only `git status --porcelain`. If non-empty AND no `[CarryWIP: ...]` was passed (standalone), warn the founder — list the files — and ask: **commit & carry** main's WIP into this pipeline (gitter commits it to main; the branch builds on it and merges back cleanly as a shared base, losing nothing) or **leave on main** (excluded from the build). Set `$CARRYWIP`. Allowed here because pre-flight precedes all work; never asked once the pipeline is running.

If pre-flight fails: STOP. Return the diagnostic. Do NOT proceed.

---

<!-- OPTIONAL: Dual-runtime support
Both Claude and {SECONDARY_RUNTIME} read this same file and execute the pipeline natively — there is no cross-runtime routing. {SECONDARY_RUNTIME}-specific adaptations (git ownership, Skill→Agent translation) live in the corresponding adapter config (`.codex/agents/build.toml`).
-->

---

## Status Emission

Print fixed-format status to stdout so a watching operator never has to ask "where are we" — every runtime emits identically. `$BUILD_IDX` = `[Build: {n}/{total}]` from `$ARGUMENTS` when wave-invoked, else empty (standalone).

**Header** (once, at the end of Step 0 — `Wave $WAVE · Build $BUILD_IDX` only when wave-invoked, else just the name):

```
═══ Wave $WAVE · Build $BUILD_IDX $PIPELINE ═══
Objective: {one line from 0-task.md}
Tasks:     {count}
════════════════════════════════════════
```

**Phase lines** — as each major phase completes, emit `▸ {phase} … {result}`:
Analysis→done · Plan→{routing} · Architecture→done · UI/UX→done · Database→done · Develop→done · QA→`PASS · {n} fix loop(s)` · Code Review→`{CLEAN / FINDINGS→fixed / N residual}` · Merge→`{commit sha}` · Post-merge QA→`{PASS / FIX}` · Docs→archived. Omit the UI/UX and Database lines when those conditional steps did not run.

**Footer** (once, after Step 11 — or on BLOCKED-DEFERRED, appending the trigger and the `$DOCS/BLOCKED.md` resume hint):

```
────────────────────────────────────────
{✓ DONE | ✗ FAILED | ⚠ BLOCKED-DEFERRED} Build $BUILD_IDX $PIPELINE · {elapsed}m
```

---

## Step 0 — Name the pipeline, clean up stale dirs, and resolve paths

### 0a. Stale pipeline cleanup (MANDATORY pre-flight)

**First, prune orphaned worktrees** — `.worktrees/{name}` directories left by failed or abandoned pipelines that no agent otherwise reclaims (the inverse of the doc-dir sweep below):

```bash
bash .claude/scripts/worktree.sh prune
```

It removes `.worktrees/` dirs that are not registered git worktrees and have no active pipeline docs; registered-but-inactive worktrees are reported for inspection, never auto-removed (they may hold uncommitted work).

Then check for abandoned pipeline directories in `docs/dev/builds/` (excluding `archive/`).
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
- **`$EPIC`** = epic name extracted from `[Epic: {epic-name}]` in `$ARGUMENTS`, otherwise `none`. Forwarded to the documenter (Step 10) so a standalone build routes its progress to `docs/epics/{name}/`. Wave-owned builds inherit it but skip the epic write — the wave consolidates it.
- **`$CARRYWIP`** = `commit` or `leave` from `[CarryWIP: ...]` in `$ARGUMENTS` (passed by `/wave`), otherwise `ask`. Governs whether main's uncommitted work is carried into this pipeline's worktree (Step 2).
- **`$BUILD_IDX`** = `{n}/{total}` from `[Build: {n}/{total}]` in `$ARGUMENTS` (passed by `/wave`), otherwise empty (standalone). Used only by § Status Emission to frame stdout status.
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
- **Installed project worktrees:** `{INSTALLED_PROJECT_WORKTREE_LIST}`

<!-- INSTALLER:
Materialize this command from the actual project roster. Delete every planner,
architect, developer, QA, db-admin, and devops block for projects that do not
exist in the target repo. Do not map missing web/infra placeholders onto another
project. After writing, grep every referenced `*/.claude/agents/*.md` path and
fail install if any path is missing.
If a project archetype is absent, delete its entire optional block and every
report-list/table reference to that project. `{OPTIONAL_*}` placeholders are
install-time scaffolding and MUST NOT survive into an installed repo.
-->

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

Capture `START=$(date +%s)` (for the footer's elapsed time), then emit the § Status Emission header.

---

## Step 1a — Parallel Codebase Analysis (child planners)

Spawn ALL INSTALLED child planners **in parallel** (single message, one Agent call per installed project).
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

Agent(general-purpose, model: "sonnet"): "You are the web planner. Read and follow {OPTIONAL_WEB_PROJECT_DIR}/.claude/agents/planner.md.
  Mode: ANALYSIS. Pipeline: {name}. Feature: {feature request}.
  Analyze the {OPTIONAL_WEB_PROJECT_DIR}/ codebase and write $DOCS/1-analysis-web.md."

Agent(general-purpose, model: "sonnet"): "You are the infrastructure planner. Read and follow {OPTIONAL_INFRA_PROJECT_DIR}/.claude/agents/planner.md.
  Mode: ANALYSIS. Pipeline: {name}. Feature: {feature request}.
  Analyze the {OPTIONAL_INFRA_PROJECT_DIR}/ codebase and write $DOCS/1-analysis-infra.md."
```

**All installed planners MUST be launched in a single message for true parallelism.** Wait for all to complete.

**Idempotency guard:** Before spawning, check which analysis reports already exist: `ls $DOCS/1-analysis-*.md 2>/dev/null`. Only spawn planners for projects whose `1-analysis-{project}.md` does NOT yet exist. If all installed project reports already exist, skip Step 1a entirely and proceed to Step 1b. This prevents duplicate planner launches if Step 1a is re-entered after a partial execution.

---

## Step 1b — Consolidation (mono-planner agent)

Use the `mono-planner` agent. **Model: claude-opus-4-6** — strategic routing decisions.

- Tell it: "Pipeline: {name}. Feature: {feature request}. Read the installed project analysis reports at {PROJECT_ANALYSIS_REPORT_LIST} and consolidate into $DOCS/1-plan.md."
- It reads the parallel analysis reports, decides routing ({PROJECT_ROUTING_OPTIONS}, CROSS), and writes `$DOCS/1-plan.md`.
- Wait for completion. Read the plan to get the routing decision before proceeding.

---

## Step 2 — Git Setup (worktrees + ports)

`$CARRYWIP` was resolved at pre-flight (`commit` → gitter commits main's WIP so the branch builds on it and merges back as a shared base; `leave` → WIP stays on main, out of the worktree). Pass it through.

Use the `gitter` agent in **SETUP** phase. **Model: sonnet** — structured git ops.

- Tell it: "Pipeline: {name}. Phase: SETUP. CarryWIP: $CARRYWIP."
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

Agent(general-purpose, model: "opus"): "You are the {AI_PROJECT_LABEL} architect. Read and follow the instructions in {project-cortex}/.claude/agents/architect.md.
  Pipeline: {name}.
  All pipeline docs: $DOCS/.
  Write your architecture doc to $DOCS/3-architecture-{AI_PROJECT_KEY}.md.
  You produce the architecture doc ONLY — no code stubs. The {AI_DEVELOPER_ROLE} derives their work queue from your doc.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the web architect. Read and follow the instructions in {OPTIONAL_WEB_PROJECT_DIR}/.claude/agents/architect.md.
  Pipeline: {name}.
  All pipeline docs: $DOCS/.
  Write your architecture doc to $DOCS/3-architecture-web.md.
  You produce the architecture doc ONLY — no code stubs. The developer derives their work queue from your doc.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the infrastructure architect. Read and follow the instructions in {OPTIONAL_INFRA_PROJECT_DIR}/.claude/agents/architect.md.
  Pipeline: {name}.
  All pipeline docs: $DOCS/.
  Write your architecture doc to $DOCS/3-architecture-infra.md.
  You design the big picture blueprint — the DevOps engineer implements from your doc.
  NEVER run git commands — gitter handles all commits."
```

Spawn only the relevant architects based on routing. Wait for completion.

The {AI_PROJECT_LABEL} architect runs on `model: "opus"` — {DOMAIN_ADJ} chain design is the highest-stakes architecture in the pipeline, and the post-merge QA tier (Step 9) matches it.

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
Agent(general-purpose, model: "sonnet"): "You are the database admin. Read and follow {OPTIONAL_INFRA_PROJECT_DIR}/.claude/agents/db-admin.md.
  Pipeline: {name}.
  All pipeline docs: $DOCS/.
  BE worktree: $WORKTREE/{project-be}. {AI_PROJECT_LABEL} worktree: $WORKTREE/{project-cortex}. Infra worktree: $WORKTREE/{OPTIONAL_INFRA_PROJECT_DIR}.
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

**Trivial infrastructure/config tasks:** If the target repo has no dedicated infra project/agent and the scope is only adding env vars or config to normal project files (e.g., adding vars to `{project-cortex}/.env.local`), the orchestrator MAY handle it directly instead of spawning a DevOps agent. Sub-agents sometimes get permission-blocked on `.worktrees/` paths — for 3-line edits, doing it yourself is faster and more reliable.

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

Agent(general-purpose, model: "opus"): "You are the {AI_PROJECT_LABEL} {AI_DEVELOPER_ROLE}. Read and follow {project-cortex}/.claude/agents/{AI_DEVELOPER_AGENT}.md.

  Pipeline: {name}.
  Worktree: $WORKTREE/{project-cortex}. Branch: pipeline/{name}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  IMPORTANT — $DOCS_REL resolves to the ROOT docs directory, NOT to docs/ inside your worktree.
  Example: from $WORKTREE/{project-cortex}/, $DOCS_REL = ../../../docs/dev/builds/{name}/.
  Write to $DOCS_REL/5-dev-report-{AI_PROJECT_KEY}.md. NEVER write to .worktrees/{name}/docs/ — that's inside the worktree and will be lost.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the web developer. Read and follow {OPTIONAL_WEB_PROJECT_DIR}/.claude/agents/developer.md.

  Pipeline: {name}.
  Worktree: $WORKTREE/{OPTIONAL_WEB_PROJECT_DIR}. Branch: pipeline/{name}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  IMPORTANT — $DOCS_REL resolves to the ROOT docs directory, NOT to docs/ inside your worktree.
  Example: from $WORKTREE/{OPTIONAL_WEB_PROJECT_DIR}/, $DOCS_REL = ../../../docs/dev/builds/{name}/.
  Write to $DOCS_REL/5-dev-report-web.md. NEVER write to .worktrees/{name}/docs/ — that's inside the worktree and will be lost.
  NEVER run git commands — gitter handles all commits."

Agent(general-purpose, model: "sonnet"): "You are the infrastructure DevOps engineer. Read and follow {OPTIONAL_INFRA_PROJECT_DIR}/.claude/agents/devops.md.

  Pipeline: {name}.
  Worktree: $WORKTREE/{OPTIONAL_INFRA_PROJECT_DIR}. Branch: pipeline/{name}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  IMPORTANT — $DOCS_REL resolves to the ROOT docs directory, NOT to docs/ inside your worktree.
  Example: from $WORKTREE/{OPTIONAL_INFRA_PROJECT_DIR}/, $DOCS_REL = ../../../docs/dev/builds/{name}/.
  Write to $DOCS_REL/5-dev-report-infra.md. NEVER write to .worktrees/{name}/docs/ — that's inside the worktree and will be lost.
  If you get permission-blocked on worktree file edits, use Bash with append/write commands as fallback.
  NEVER run git commands — gitter handles all commits."
```

Launch only the relevant developers/engineers/DevOps based on routing. Wait for completion.

The {AI_PROJECT_LABEL} {AI_DEVELOPER_ROLE} runs on `model: "opus"` — LLM-chain implementation carries the highest correctness risk in the pipeline.

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

Agent(general-purpose, model: "sonnet"): "You are the web QA engineer. Read and follow {OPTIONAL_WEB_PROJECT_DIR}/.claude/agents/qa.md.

  Mode: PRE-MERGE. Pipeline: {name}.
  Worktree: $WORKTREE/{OPTIONAL_WEB_PROJECT_DIR}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  Write bug report to $DOCS_REL/ — NEVER to docs/ inside the worktree."

Agent(general-purpose, model: "sonnet"): "You are the infrastructure QA engineer. Read and follow {OPTIONAL_INFRA_PROJECT_DIR}/.claude/agents/qa.md.

  Mode: PRE-MERGE. Pipeline: {name}.
  Worktree: $WORKTREE/{OPTIONAL_INFRA_PROJECT_DIR}.
  ALL pipeline docs: $DOCS/ (at root). From your worktree: $DOCS_REL/.
  Write bug report to $DOCS_REL/ — NEVER to docs/ inside the worktree."
```

Spawn QA agents only for relevant projects. Each agent writes:

- `{PROJECT_BUG_REPORT_LIST}` — bug list per installed project

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
   4. If QA passes → code review → gitter MERGE → post-merge QA → documenter (normal pipeline tail).
   5. If QA still fails → ONE more fix-loop iteration max, then re-defer.
   ```

2. **Do NOT** delete the worktree, do NOT release ports, do NOT run gitter MERGE.

3. **Return to the orchestrator with status BLOCKED-DEFERRED.** The wave runner (or human invoker) decides whether to continue, abandon, or resume after `/jc`.

This is intentionally conservative — a deferred pipeline preserves all work done. A failed pipeline with a torn-down worktree loses hours of agent output. The `0a` stale-cleanup rule recognizes `BLOCKED.md` and will NOT auto-archive deferred pipelines.

---

## Code Review (BEFORE merge — pre-merge hygiene gate, capped at 2 iterations)

The last gate before the worktree reaches `main`: a hygiene pass on the pipeline's own diff, fixed in place. QA proved the code works; this proves it's clean. Runs only after the Fix Loop converges (`$DOCS/6-bugs.md` = `Status: NONE`).

### Hygiene audit on the diff

Assemble the pipeline's changed-file set (read-only): `git -C $WORKTREE diff --name-only main...pipeline/{name}`. Read `.claude/skills/audit:code-hygiene/SKILL.md` and execute it inline with scope `diff` over that set — the Duplication category first. Write findings to `$DOCS/6-code-review.md`, ending with a verdict line: `CLEAN` or `FINDINGS`.

- `CLEAN` → proceed to Merge Phase.
- `FINDINGS` → run the fix loop below.

### Code-Review Fix Loop (architect → developer, cap 2)

1. **Architect plans the fixes.** Spawn the child architect(s) for the affected projects (same spawn pattern as Step 4 — sonnet via general-purpose for BE/FE/web/infra, `model: "opus"` via general-purpose for {AI_PROJECT_KEY}): "Pipeline: {name}. Read $DOCS/6-code-review.md. For each finding decide the fix — which existing symbol to reuse, where to extract the shared helper, which copy to delete. Append a `## Fix Plan` section to $DOCS/6-code-review.md. Decisions only, no code edits. NEVER run git commands."

2. **Developer applies the fixes.** Spawn the developer(s)/{AI_DEVELOPER_ROLE} for the affected projects (same spawn pattern as Step 6 — sonnet via general-purpose for BE/FE/web/infra, `model: "opus"` via general-purpose for {AI_PROJECT_KEY}): "Pipeline: {name}. Worktree: $WORKTREE/{PROJECT_DIR}. Apply every fix in the `## Fix Plan` of $DOCS_REL/6-code-review.md. Re-run your project's tests (timeout 600s) to confirm no regression — the worktree must stay test-green. NEVER run git commands."

3. **Re-run the hygiene audit** on the updated diff and overwrite the verdict line.

4. Repeat until `CLEAN` or 2 iterations pass.

**After 2 iterations with findings remaining:** these are quality nits, not correctness bugs (QA already passed). Log them under `## Residual` in `$DOCS/6-code-review.md` and proceed to merge — never block shipping on hygiene perfection. A standalone build surfaces the residual to the founder; a wave-owned build leaves it in the merged code for `/p:wave-review` to catch and route through `/jc`.

---

## Merge Phase (only after QA passes and Code Review completes)

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
**Model: `opus`** — post-merge bug-catch quality matters more than token cost; pre-merge QA stays on Sonnet. Each project's QA runs via general-purpose on the `opus` alias and reads its own `qa.md` for protocol.

```
Agent(general-purpose, model: "opus"): "You are the backend QA engineer. Read and follow {project-be}/.claude/agents/qa.md.
  Mode: POST-MERGE. Pipeline: {name}. Run against {project-be}/ on main.
  Pipeline docs from project dir: $DOCS_POST/. Pipeline docs from root: $DOCS/.
  Follow the runbook at {project-be}/docs/runbook/_index.md."

Agent(general-purpose, model: "opus"): "You are the frontend QA engineer. Read and follow {project-fe}/.claude/agents/qa.md.
  Mode: POST-MERGE. Pipeline: {name}. Run against {project-fe}/ on main.
  Pipeline docs from project dir: $DOCS_POST/. Pipeline docs from root: $DOCS/.
  Follow the runbook at {project-fe}/docs/runbook.md."

Agent(general-purpose, model: "opus"): "You are the {AI_PROJECT_LABEL} QA engineer. Read and follow {project-cortex}/.claude/agents/qa.md.
  Mode: POST-MERGE. Pipeline: {name}. Run against {project-cortex}/ on main.
  Pipeline docs from project dir: $DOCS_POST/. Pipeline docs from root: $DOCS/.
  Follow the runbook at {project-cortex}/docs/runbook/_index.md."

Agent(general-purpose, model: "opus"): "You are the web QA engineer. Read and follow {OPTIONAL_WEB_PROJECT_DIR}/.claude/agents/qa.md.
  Mode: POST-MERGE. Pipeline: {name}. Run against {OPTIONAL_WEB_PROJECT_DIR}/ on main.
  Pipeline docs from project dir: $DOCS_POST/. Pipeline docs from root: $DOCS/."

Agent(general-purpose, model: "opus"): "You are the infrastructure QA engineer. Read and follow {OPTIONAL_INFRA_PROJECT_DIR}/.claude/agents/qa.md.
  Mode: POST-MERGE. Pipeline: {name}. Run against {OPTIONAL_INFRA_PROJECT_DIR}/ on main.
  Pipeline docs from project dir: $DOCS_POST/. Pipeline docs from root: $DOCS/."
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

### Step 10 — Documentation & Aggregation (after post-merge QA passes)

Use the `mono-documenter` agent. **Model: sonnet** — structured doc merging.

- Tell it: "Pipeline: {name}. Phase: ARCHIVE. Epic: $EPIC. Docs: $DOCS. Archive: $ARCHIVE. Pipeline dir to archive: $DOCS → $ARCHIVE/{name}."

The documenter:

1. **Merges** pipeline decisions into permanent docs (`docs/agents/architecture/`, `docs/agents/api/`, child `architecture/`, `ui-ux/` clusters, etc.)
2. **Updates** child project docs (`{project-be}/docs/`, `{project-fe}/docs/`, `{project-cortex}/docs/`) with new details
3. **Archives** `$DOCS/` to `$ARCHIVE/{name}/`

All pipeline docs are already in `$DOCS/` — no aggregation needed. Permanent root docs accumulate all decisions. Child project docs get updated with project-specific details.

---

### Step 11 — Commit Docs (MANDATORY after documenter finishes)

Use the `gitter` agent in **DOCS-COMMIT** phase. **Model: sonnet** — structured git ops.

- Tell it: "Pipeline: {name}. Wave: {$WAVE or 'none'}. Phase: DOCS-COMMIT. Projects: {same project keys as MERGE step}."

Gitter commits all doc changes the documenter made on main.

---

## Pipeline Reference

| #   | Step                            | Who                                                           | Produces                                                                                      | Location                         |
| --- | ------------------------------- | ------------------------------------------------------------- | --------------------------------------------------------------------------------------------- | -------------------------------- |
| 1a  | Parallel analysis               | Installed project planners: {PROJECT_PLANNER_ROSTER}          | `{PROJECT_ANALYSIS_REPORT_LIST}`                                                              | root                             |
| 1b  | Consolidate plan                | mono-planner                                                  | `$DOCS/1-plan.md`                                                                             | root                             |
| 2   | Git setup                       | gitter (SETUP)                                                | Worktrees, ports, `$DOCS/ports.md`                                                            | root                             |
| 3   | Cross-project arch + research   | mono-architect                                                | `$DOCS/3-architecture.md` (integration contracts + research notes)                            | root                             |
| 4   | Child arch + research           | Installed project architects: {PROJECT_ARCHITECT_ROSTER}      | `{PROJECT_ARCHITECTURE_REPORT_LIST}` (docs only, no code stubs, inline research)              | root                             |
| 5a  | UI/UX _(conditional)_           | ui-ux                                                         | `$DOCS/4-ui-ux-spec.md`                                                                       | root                             |
| 5b  | DB Architecture _(conditional)_ | db-admin                                                      | `$DOCS/4-db-architecture.md` + schema/migration changes in worktrees                          | root (docs) + worktrees (schema) |
| 6   | Develop                         | Installed project developers: {PROJECT_DEVELOPER_ROSTER}      | Working code in worktrees + `{PROJECT_DEV_REPORT_LIST}`                                       | worktrees (code) + root (docs)   |
| 7   | QA                              | Installed project QA agents: {PROJECT_QA_ROSTER}              | Adversarial tests in worktrees + `{PROJECT_BUG_REPORT_LIST}` → consolidated `$DOCS/6-bugs.md` | root                             |
| -   | Fix loop                        | developers → QA                                               | Repeat until `$DOCS/6-bugs.md` = NONE                                                         |                                  |
| -   | Code review _(pre-merge gate)_  | audit:code-hygiene (diff) → architects → developers           | `$DOCS/6-code-review.md` (loops until CLEAN, cap 2)                                           | worktrees (code) + root (docs)   |
| 8   | Merge                           | gitter (MERGE)                                                | Commits + merges to main                                                                      |                                  |
| 9   | Post-merge QA                   | Installed project QA agents (POST-MERGE): {PROJECT_QA_ROSTER} | `$DOCS/7-post-merge-qa.md` (single consolidated file from inline results)                     | root                             |
| 10  | Document                        | mono-documenter                                               | Merges into permanent docs, archives pipeline to `$ARCHIVE/{name}/`                           | root                             |
| 11  | Commit docs                     | gitter (DOCS-COMMIT)                                          | Commits doc changes on main                                                                   | root                             |

---

## Done

When post-merge QA passes, documentation is archived, and doc changes are committed, say: "Build complete ({name}). All tests pass on main. Code review clean. Docs archived and committed."
