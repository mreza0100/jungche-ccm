---
name: build
description: Run the build pipeline — full feature delivery in an isolated worktree with planning, architecture, development, QA gates, merge, and docs. Route every new feature or enhancement here; bug fixes go to /jc.
argument-hint: [feature description]
---

# Cross-Project Build Pipeline

Run the full {PROJECT_NAME} pipeline for: $ARGUMENTS

**All feature requests go through this pipeline — there are no child build commands.**

<!-- ROSTER MODEL (read before editing this file)
The per-project pipeline stages (analysis, architecture, development, QA) are written ONCE as a
generic `{project}` PATTERN block — not as N hardcoded role sections. Each block is marked with a
`PATTERN: per-project` HTML comment and uses the roster tokens: `{project}` (directory),
`{PROJECT_ROLE}` (role label), `{PROJECT_STACK}`, `{PROJECT_PKG_MGR}`, `{PROJECT_TEST_RUNNER}`,
`{PROJECT_PORT}`, and `{ROLE}` (the routing key, e.g. `{ROLE}-ONLY`). At install, SETUP expands
each PATTERN block ONCE PER `{PROJECT_ROSTER}` entry, substituting that entry's fields — so a
1-project repo and a 7-project monorepo get a correctly-sized command from the same template.
Cross-project / integration steps (mono-planner consolidation, mono-architect, the DB-architecture
seam, post-merge integration) are gated on `{PROJECT_ROSTER}` size > 1 and are SKIPPED for a
single-project repo, where the worktree IS the repo root and routing is trivially that one project.
-->

**Autonomous execution contract:** once started, `/build` runs to completion without stopping to ask questions or wait for approval. The only defined stops are pre-flight failure (before any work) and Fix Loop Escalation → BLOCKED-DEFERRED. Any other mid-run stop is a contract violation. A costly/external/production action a task requires (paid API call, live deploy) is not a stop: take the safest reversible path and log it. Raise a true blocker as a pre-flight fail-fast.

**ZERO GAP & doc-awareness** bind every agent — both are stated in the § Common spawn contract and carried by every spawn block. The orchestrator distinction: when `$DOCS/0-task.md` is a `/p:wave:refine` spec, agents implement and validate it; a standalone build given a bare description has agents design as normal.

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

Then check for abandoned pipeline directories in `docs/dev/builds/`.
A pipeline directory is **stale** if it has NO corresponding active worktree in `.worktrees/`:

```bash
for dir in docs/dev/builds/*/; do
  name=$(basename "$dir")
  if [ ! -d ".worktrees/$name" ]; then
    echo "STALE: $dir (no active worktree)"
  fi
done
```

**For each stale directory found:**

- If it contains a `BLOCKED.md` → it is intentionally preserved (deferred for `/jc` resolution — see § Fix Loop Escalation). **SKIP cleanup.** Do NOT archive, do NOT delete. Print `PRESERVED: $dir (BLOCKED-DEFERRED, awaiting resume)` and move on.
- **If it belongs to an active wave** → **SKIP.** Wave-owned builds are NEVER archived individually — they archive together when the wave archives. Detection: `grep -rl "$name" docs/dev/waves/*/report.md 2>/dev/null`. If any match, print `WAVE-OWNED: $dir (belongs to active wave, skipping)` and move on.
- If it contains a `7-post-merge-qa.md` → it completed but wasn't archived. Archive it to cold storage (see below). **Only for standalone builds (no wave owner).**
- If it has NO completion markers (no `7-*` file, no `BLOCKED.md`) → it was abandoned mid-pipeline. Add an `ABANDONED.md` marker, then archive. **Only for standalone builds (no wave owner).**

```bash
echo "Pipeline abandoned — archived during /build pre-flight cleanup on $(date -I)" > docs/dev/builds/$name/ABANDONED.md
```

**Archive to cold storage (for standalone builds only — NEVER for wave-owned builds):**

```bash
mkdir -p tmp/dev/archive/builds
mv docs/dev/builds/$name tmp/dev/archive/builds/
```

`tmp/` is gitignored. Files the pipeline already committed stay in git history; if the swept dir was tracked, the next gitter DOCS-COMMIT picks up the deletions.

**Empty directories** (zero files) → just remove them: `rmdir docs/dev/builds/$name`

This prevents stale pipeline dirs from accumulating across sessions.

### 0b. Name the pipeline

**If `$ARGUMENTS` contains `[Pipeline: {name}]`:** Extract and use that name — the wave runner pre-assigned it, pre-placed the task manifest at `docs/dev/builds/{name}/0-task.md`, and already ran uniqueness checks. Skip name generation and uniqueness check below; proceed directly to path variable resolution.

**Otherwise (standalone invocation):** Choose a short, descriptive kebab-case name based on the feature (e.g., `session-notes`, `audio-streaming`).

**Name uniqueness check (standalone only — skip when `[Pipeline: ...]` is present):** Before proceeding, verify the chosen name does NOT already exist in:

- `tmp/dev/archive/builds/` — archived pipelines (gitignored cold storage)
- `docs/dev/builds/` — active pipelines
- `.worktrees/` — active worktrees

```bash
ls tmp/dev/archive/builds/ 2>/dev/null | sed 's/^[0-9]*-//' | grep -x "{name}"
ls docs/dev/builds/ .worktrees/ 2>/dev/null | grep -x "{name}"
```

The first `ls` strips legacy counter prefixes (e.g., `003-radar-surfaces` → `radar-surfaces`) before matching. If the name exists anywhere, append a version suffix (e.g., `session-notes-v2`, `audio-streaming-v3`) or choose a more specific name. **NEVER reuse an archived pipeline name** — it causes doc conflicts and breaks traceability.

Resolve path variables:

- **`$PIPELINE`** = `{name}` — the pipeline name (kebab-case, unique across active + archived). Extracted from `[Pipeline: {name}]` in `$ARGUMENTS` when present (wave-invoked), otherwise chosen by build (standalone).
- **`$WAVE`** = wave name extracted from `[Wave: {wave-name}]` in `$ARGUMENTS`, otherwise `none`. This value is forwarded to gitter so merge + docs commits carry a `Wave:` trailer for git-history traceability back to `tmp/dev/archive/waves/{wave}/`.
- **`$EPIC`** = epic name extracted from `[Epic: {epic-name}]` in `$ARGUMENTS`, otherwise `none`. Forwarded to the documenter (Step 10) so a standalone build routes its progress to `docs/epics/{name}/`. Wave-owned builds inherit it but skip the epic write — the wave consolidates it.
- **`$CARRYWIP`** = `commit` or `leave` from `[CarryWIP: ...]` in `$ARGUMENTS` (passed by `/wave`), otherwise `ask`. Governs whether main's uncommitted work is carried into this pipeline's worktree (Step 2).
- **`$BUILD_IDX`** = `{n}/{total}` from `[Build: {n}/{total}]` in `$ARGUMENTS` (passed by `/wave`), otherwise empty (standalone). Used only by § Status Emission to frame stdout status.
- **`$DOCS`** = `docs/dev/builds/{name}` — pipeline docs from repo root
- **`$DOCS_REL`** = `../../../docs/dev/builds/{name}` — pipeline docs from worktree
- **`$DOCS_POST`** = `../docs/dev/builds/{name}` — pipeline docs from project subdir (POST-MERGE)
- **`$WORKTREE`** = `.worktrees/{name}` — pipeline worktree directory (full checkout of the whole roster)
- **`$PROJECTS`** = list of affected projects from `{PROJECT_ROSTER}` (e.g. one `{ROLE}` per affected roster entry) — set by routing. A single-project roster makes this trivially the one project.
- **Pipeline branch:** `pipeline/{name}` (single branch for the whole worktree)
- **`$WORKTREE` layout:** one worktree holds the roster. **Multi-project roster:** each roster entry lives in its subdir `$WORKTREE/{project}`. **Single-project roster:** the worktree IS the repo root — there is no per-project subdir, so `$WORKTREE/{project}` collapses to `$WORKTREE`.

<!-- INSTALLER:
Materialize this command from the actual `{PROJECT_ROSTER}`. Each block marked
`<!-- PATTERN: per-project -->` is written once with generic `{project}`/`{PROJECT_ROLE}`/`{ROLE}`

tokens — EXPAND it once per roster entry, substituting that entry's directory, role label, stack,
package manager, test runner, and port. Spawn an agent only for roster entries that actually have
the corresponding `{project}/.claude/agents/*.md` file.
For a SINGLE-PROJECT roster: emit exactly one expansion of each PATTERN block, drop every step
gated `(roster size > 1)`, and resolve `$WORKTREE/{project}` to `$WORKTREE` (repo root).
After writing, grep every referenced `{project}/.claude/agents/*.md` path and fail install if any
path is missing. No `{project}` / `{ROLE}` / `{PROJECT_*}` placeholder may survive into an installed repo.
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

Pass `$PIPELINE`, `$DOCS`, `$DOCS_REL`, and `$WORKTREE` to every agent — agents use what you give them, never hardcoding paths.

Capture `START=$(date +%s)` (for the footer's elapsed time), then emit the § Status Emission header.

---

## Common spawn contract

Every spawned agent inherits these. Each spawn block carries only its role, worktree path, report file, ports, and role-specific additions, plus "follow the Common spawn contract."

- **NEVER run git** — gitter owns every commit; no other agent runs git commands.
- **Write reports to root docs, never inside the worktree.** From a worktree, `$DOCS_REL` resolves to the ROOT docs directory (`docs/dev/builds/{name}/`), NOT to `docs/` inside the worktree. NEVER write to `.worktrees/{name}/docs/` — it is inside the worktree and will be lost.
- **ZERO GAP** — when `$DOCS/0-task.md` is a `/p:wave:refine` spec, IMPLEMENT and VALIDATE it; never re-decide routing, data model, or contracts. Surface a genuine spec flaw to the orchestrator; never silently change it.
- **Doc-awareness** — consult the grep-true doc clusters: read the project's `docs/architecture/_index.md`, then `grep` for the exact symbol; the full DB schema is `docs/agents/graph/db/{DATABASE}.mmd`.

**Per-role doc reads** (each role reads only what it needs from `$DOCS/`):

- **Child architects** → `1-plan.md` + `3-architecture.md` + own `1-analysis-{ROLE}.md`
- **Developers / engineers** → `1-plan.md` + `3-architecture.md` + `3-architecture-{ROLE}.md` + `4-db-architecture.md` (if present) + `4-ui-ux-spec.md` (UI role only) + `ports.md`
- **Pre-merge QA** → `3-architecture-{ROLE}.md` + `5-dev-report-{ROLE}.md` + `6-bugs.md`
- **Post-merge QA** → `3-architecture-{ROLE}.md` + `5-dev-report-{ROLE}.md` + `6-bugs.md` + project runbook

---

## Step 1a — Parallel Codebase Analysis (child planners)

**Routing-gated fan-out.** Parse the `**Routing:**` declaration in `$DOCS/0-task.md` (a `/p:wave:refine` spec declares it). Spawn child planners **in parallel** (single message) ONLY for the declared projects. When NO routing is declared (bare standalone description), fall back to spawning all roster planners. mono-planner (Step 1b) may demand a missing project's planner when it spots an undeclared seam — spawn that one then.
**Model: opus.** Model tiers per `docs/commands/pcm/references/agent-models.md` (single source); the literals below are declared copies.

<!-- PATTERN: per-project — SETUP expands once per {PROJECT_ROSTER} entry -->

```
Agent(general-purpose, model: "opus"): "You are the {PROJECT_ROLE} planner. Read and follow {project}/.claude/agents/planner.md.
  Mode: ANALYSIS. Pipeline: {name}. Feature: {feature request}.
  Analyze the {project}/ codebase and write $DOCS/1-analysis-{ROLE}.md."
```

Wait for all to complete.

**Idempotency guard:** Before spawning, check which reports exist: `ls $DOCS/1-analysis-*.md 2>/dev/null`. Only spawn planners whose `1-analysis-{ROLE}.md` does NOT yet exist. If every routed planner's report exists, skip Step 1a and proceed to Step 1b.

---

## Step 1b — Consolidation (mono-planner agent)

**Single-project roster:** skip the mono-planner — there is nothing cross-project to consolidate or route. Promote the single `$DOCS/1-analysis-{ROLE}.md` to the plan: copy it to `$DOCS/1-plan.md` (routing is trivially `{ROLE}-ONLY`). Then proceed to Step 2.

**Multi-project roster (size > 1):** Use the `mono-planner` agent. **Model: claude-opus-4-8** (pinned in its frontmatter) — strategic routing decisions.

- Tell it: "Pipeline: {name}. Feature: {feature request}. Read the roster analysis reports at `$DOCS/1-analysis-*.md` and consolidate into $DOCS/1-plan.md."
- It reads the parallel analysis reports, decides routing (one `{ROLE}-ONLY` per roster entry, or `CROSS` when more than one project is touched), and writes `$DOCS/1-plan.md`.
- Wait for completion. Read the plan to get the routing decision before proceeding.

---

## Step 2 — Git Setup (worktrees + ports)

`$CARRYWIP` was resolved at pre-flight (`commit` → gitter commits main's WIP so the branch builds on it and merges back as a shared base; `leave` → WIP stays on main, out of the worktree). Pass it through.

Use the `gitter` agent in **SETUP** phase. **Model: sonnet** — structured git ops.

- Tell it: "Pipeline: {name}. Phase: SETUP. CarryWIP: $CARRYWIP."
- Creates one worktree holding the roster — the repo root when the roster is size 1, one branch: `pipeline/{name}`
- With a multi-project roster, developers work in their respective `{project}/` subdirs within the same worktree; with a single-project roster, the developer works at the worktree root
- Wait for confirmation before proceeding.

---

## Step 3 — Cross-Project Architecture (mono-architect) — _(roster size > 1)_

**Single-project roster:** skip entirely — there is no cross-project seam to design. Proceed to Step 4.

**Multi-project roster (size > 1):** Use the `mono-architect` agent. **Model: claude-opus-4-8** (pinned in its frontmatter) — critical cross-project architecture decisions + inline research.

- Tell it: "Pipeline: {name}. Read $DOCS/1-plan.md. Write $DOCS/3-architecture.md."
- It designs API contracts, shared types, and integration patterns — but makes NO code-level decisions or TODO stubs.
- Skip if routing touches a single project only (any one `{ROLE}-ONLY`) with no integration changes.
- Wait for completion.

---

## Step 4 — Child Architecture (parallel if more than one project is routed)

Spawn the child architect for each routed roster entry. They read mono-architect's integration contracts (when present) and write
project-specific architecture docs to `$DOCS/`. Architects produce docs only — no code stubs in worktrees.
Developers derive their work queue from the architecture docs directly.
**Model: opus** — every build child agent does real work.

<!-- PATTERN: per-project — SETUP expands once per routed {PROJECT_ROSTER} entry -->

```
Agent(general-purpose, model: "opus"): "You are the {PROJECT_ROLE} architect. Read and follow the instructions in {project}/.claude/agents/architect.md.
  Pipeline: {name}. Follow the Common spawn contract (child-architect doc reads).
  Write your architecture doc to $DOCS/3-architecture-{ROLE}.md.
  You produce the architecture doc ONLY — no code stubs. The developer derives their work queue from your doc."
```

Spawn only the architects for routed roster entries. Wait for completion.

---

## Step 5 — UI/UX Design + Database Architecture (conditional, parallel)

Check `$DOCS/1-plan.md` for frontend visual tasks AND schema changes.
Spawn both agents in a single message if both are needed (they're independent):

### Step 5a — UI/UX Design (conditional — requires a roster entry with a UI role + a `ui-ux.md` agent)

**If visual work is needed on a roster entry that has a `{project}/.claude/agents/ui-ux.md`:**

```
Agent(general-purpose, model: "opus"): "You are the UI/UX designer. Read and follow {project-ui}/.claude/agents/ui-ux.md.
  Pipeline: {name}. All pipeline docs: $DOCS/.
  Read $DOCS/3-architecture.md (if present) and $DOCS/3-architecture-{ROLE-ui}.md.
  Write your spec to $DOCS/4-ui-ux-spec.md."
```

`{project-ui}` / `{ROLE-ui}` = the roster entry whose role owns the user interface; SETUP binds it from the roster (or deletes Step 5a if no roster entry ships a `ui-ux.md`).

**If no visual work, or no UI roster entry exists**, skip.

### Step 5b — Database Architecture (conditional — but CHECK EXPLICITLY)

**Detection rule (MANDATORY — do NOT skip this check):** Grep the plan (`$DOCS/1-plan.md`) and
architecture docs for ANY of these signals: `table`, `schema`, `column`, `index`, `enum`, `migration`,
`{SCHEMA_DEFINITION}`, `{ORM}`, `database`. If ANY signal is found, db-admin MUST be invoked.

**This check exists because pipelines have shipped new tables without migration files.
The orchestrator's "judgment call" is not reliable enough — use keyword detection.**

If schema signals are found, spawn the db-admin agent. `{project-db}` = the roster entry that owns the schema/migrations (the one holding `{MIGRATIONS_DIR}`); SETUP binds it from the roster, or deletes Step 5b entirely if no roster entry ships a `db-admin.md`.

```
Agent(general-purpose, model: "opus"): "You are the database admin. Read and follow {project-db}/.claude/agents/db-admin.md.
  Pipeline: {name}.
  All pipeline docs: $DOCS/.
  Worktree(s): $WORKTREE/{project} for each routed roster entry that touches the schema (single-project roster: $WORKTREE root).
  Read architecture docs at $DOCS/ and implement schema changes.
  CRITICAL: Every column in the schema MUST have a corresponding SQL migration — either in a CREATE TABLE
  statement or an ALTER TABLE ADD COLUMN statement. This applies to NEW TABLES ({SCHEMA_DEFINITION}) AND new columns
  on existing tables. Count existing files in $WORKTREE/{project-db}/{MIGRATIONS_DIR}/ to determine the next
  migration number. Run your column-level completeness check before finishing — it is BLOCKING.
  Write your database architecture doc to $DOCS/4-db-architecture.md.
  NEVER run git commands — gitter handles all commits."
```

**If no schema signals found in plan or architecture**, skip — but log: "Step 5b: no schema signals detected, skipping db-admin."

---

## Step 6 — Parallel Development (on named worktrees)

Read ports from `$DOCS/ports.md`, then launch the developer for each routed roster entry.
**Model: opus** — every build child agent does real work.

**Trivial infrastructure/config tasks:** If the scope is only adding env vars or config to normal project files (e.g., adding vars to a `{project}/.env.local`) and no roster entry ships a dedicated DevOps agent, the orchestrator MAY handle it directly instead of spawning a sub-agent. Sub-agents sometimes get permission-blocked on `.worktrees/` paths — for 3-line edits, doing it yourself is faster and more reliable.

<!-- PATTERN: per-project — SETUP expands once per routed {PROJECT_ROSTER} entry.
A roster entry's developer agent is whatever that entry declares — `developer.md` for a normal
project, `devops.md` for an infra-shaped entry; SETUP substitutes the right agent filename and role
label. For a single-project roster, $WORKTREE/{project} resolves to $WORKTREE (repo root) and
$DOCS_REL is ../../docs/dev/builds/{name}/ (one level shallower than a per-project subdir). -->

```
Agent(general-purpose, model: "opus"): "You are the {PROJECT_ROLE} developer. Read and follow {project}/.claude/agents/developer.md.
  Pipeline: {name}. Follow the Common spawn contract (developer doc reads).
  Worktree: $WORKTREE/{project}. Branch: pipeline/{name}.
  Write to $DOCS_REL/5-dev-report-{ROLE}.md. {PROJECT_ROLE} port: {PROJECT_PORT}."
```

Launch only the developers for routed roster entries. Wait for completion.

---

## Step 7 — QA (BEFORE merge)

**CRITICAL: QA runs against the worktree branches, NOT main.**

Spawn the QA engineer for each routed roster entry.
**Model: opus** — every build child agent does real work.

<!-- PATTERN: per-project — SETUP expands once per routed {PROJECT_ROSTER} entry.
Spawn as the registered `qa-{project}` wrapper type (model governed by frontmatter; no override needed).
Single-project roster: $WORKTREE/{project} resolves to $WORKTREE (repo root). -->

```
Agent(qa-{project}): "Mode: PRE-MERGE. Pipeline: {name}. Follow the Common spawn contract (pre-merge QA doc reads).
  Worktree: $WORKTREE/{project}. Port: {PROJECT_PORT}.
  Write findings into $DOCS_REL/6-bugs.md under a `## {PROJECT_ROLE}` section (create the file if absent)."
```

If `qa-{project}` is not a recognized agent type in this session (registry predates the wrappers), fall back to `Agent(general-purpose, model: "opus")` + "Read and follow {project}/.claude/agents/qa.md" — the filter hook simply won't fire.

Spawn QA agents only for relevant roster entries. Each QA agent owns exactly its own `## {PROJECT_ROLE}` section in the consolidated `$DOCS/6-bugs.md` — it writes only that section, never touching another's. There are no per-project bug files.

After all QA agents complete, **verify** `$DOCS/6-bugs.md` carries a `## {PROJECT_ROLE}` section for every QA'd project (an absent section means that agent failed — re-spawn it). Then set the file-level status at the top: `Status: OPEN` if any section lists an open bug, else `Status: NONE`.

---

## Fix Loop (BEFORE merge — capped at 3 iterations, never infinite)

QA may have already patched trivial bugs inline (listed under `Inline fixes:` in its bug report header — informational only, no action needed). Everything in `Status: OPEN` is what developer still needs to handle.

**Iteration cap: 3 maximum.** Never run more than 3 fix-loop iterations per pipeline. After iteration 3 fails, escalate to BLOCKED-DEFERRED (see § Fix Loop Escalation below) — do NOT keep looping. Past incidents show runaway fix loops eat orders of magnitude more wall-time than the work they purport to fix.

**Hard timeouts on test commands.** Every test invocation inside the fix loop MUST be wrapped in `timeout 600s <test command>` (10 minutes per invocation). Agent definitions enforce this — see each roster entry's `{project}/.claude/agents/{qa,developer}.md` (or `devops.md` for an infra-shaped entry). The orchestrator does NOT spawn raw test runs; it spawns agents that own the timeout discipline.

**Hung-process detection.** If any test process sits at 0% CPU for >2 minutes (deadlocked, not slow) the running agent must `kill` it and report `BUG-HUNG-TEST` with the file:line of the hanging test. A hung test is NOT a fix-loop bug — it is a code bug that requires `/jc` on main.

If `$DOCS/6-bugs.md` has `Status: OPEN`:

1. **Developer fixes** — spawn developer agents on their existing worktree branches (same as Step 6, model: opus). Developers read `6-bugs.md` directly for bugs with `Status: OPEN`. QA's adversarial tests provide the reproduction — the failing test IS the root cause. Developers debug and fix the code themselves — no separate debugger needed.

2. **Re-run QA** (same as Step 7, model: opus)

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

Assemble the pipeline's changed-file set (read-only): `git -C $WORKTREE diff --name-only main...pipeline/{name}`. Read `.claude/skills/p:audit:code-hygiene/SKILL.md` and execute it inline with scope `diff` over that set — the Duplication category first. Write findings to `$DOCS/6-code-review.md`, ending with a verdict line: `CLEAN` or `FINDINGS`.

- `CLEAN` → proceed to Merge Phase.
- `FINDINGS` → run the fix loop below.

### Code-Review Fix Loop (architect → developer, cap 2)

1. **Architect plans the fixes.** Spawn the child architect for each affected roster entry (same spawn pattern as Step 4 — general-purpose, `model: "opus"`): "Pipeline: {name}. Read $DOCS/6-code-review.md. For each finding decide the fix — which existing symbol to reuse, where to extract the shared helper, which copy to delete. Append a `## Fix Plan` section to $DOCS/6-code-review.md. Decisions only, no code edits. NEVER run git commands."

2. **Developer applies the fixes.** Spawn the developer for each affected roster entry (same spawn pattern as Step 6 — general-purpose, `model: "opus"`): "Pipeline: {name}. Worktree: $WORKTREE/{project} (the worktree root for a single-project roster). Apply every fix in the `## Fix Plan` of $DOCS_REL/6-code-review.md. Re-run your project's tests (timeout 600s) to confirm no regression — the worktree must stay test-green. NEVER run git commands."

3. **Re-run the hygiene audit** on the updated diff and overwrite the verdict line.

4. Repeat until `CLEAN` or 2 iterations pass.

**After 2 iterations with findings remaining:** these are quality nits, not correctness bugs (QA already passed). Log them under `## Residual` in `$DOCS/6-code-review.md` and proceed to merge — never block shipping on hygiene perfection. A standalone build surfaces the residual to the founder; a wave-owned build leaves it in the merged code for `/p:wave:review` to catch and route through `/jc`.

---

## Merge Phase (only after QA passes and Code Review completes)

### Step 8 — Git Merge + Cleanup

Use the `gitter` agent in **MERGE** phase. **Model: sonnet** — structured git ops.

- Tell it: "Pipeline: {name}. Wave: {$WAVE or 'none'}. Phase: MERGE. Projects: {comma-separated `{ROLE}` keys based on routing}."
  - `{ROLE}-ONLY` (any single roster entry) → `Projects: {ROLE}` (e.g. one project's key)
  - `CROSS` (more than one roster entry) → `Projects: {ROLE1},{ROLE2},…` for every routed entry
  - Single-project roster → always `Projects: {ROLE}` (its one key)

---

## Post-Merge Verification (MANDATORY after every merge)

### Step 9 — Post-Merge QA (on main)

Spawn the post-merge QA engineer for each routed roster entry. Since these run from project dirs on `main` (not worktrees), pass `$DOCS_POST` for relative doc access.
**Model: `opus`.** Each entry's QA runs via general-purpose on the `opus` alias and reads its own `qa.md` for protocol.

<!-- PATTERN: per-project — SETUP expands once per routed {PROJECT_ROSTER} entry.
Spawn as the registered `qa-{project}` wrapper type. Single-project roster: {project}/ on main IS the repo root.
SETUP binds each entry's runbook path. -->

```
Agent(qa-{project}): "Mode: POST-MERGE. Pipeline: {name}. Run against {project}/ on main. Follow the Common spawn contract (post-merge QA doc reads), reading docs via $DOCS_POST/ from the project dir.
  Runbook: {PROJECT_RUNBOOK}."
```

If `qa-{project}` is not a recognized agent type in this session (registry predates the wrappers), fall back to `Agent(general-purpose, model: "opus")` + "Read and follow {project}/.claude/agents/qa.md" — the filter hook simply won't fire.

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

- Tell it: "Pipeline: {name}. Phase: ARCHIVE. Epic: $EPIC. Docs: $DOCS."

The documenter:

1. **Merges** pipeline decisions into permanent docs (`docs/agents/architecture/`, `docs/agents/api/`, child `architecture/`, `ui-ux/` clusters, etc.)
2. **Updates** each routed roster entry's docs (`{project}/docs/` per entry; the repo-root `docs/` for a single-project roster) with new details
3. **Leaves** `$DOCS/` in place — Step 11 commits it into git history, then archives it

All pipeline docs are already in `$DOCS/` — no aggregation needed. Permanent root docs accumulate all decisions. Child project docs get updated with project-specific details.

---

### Step 11 — Commit Docs + Archive (MANDATORY after documenter finishes)

Use the `gitter` agent in **DOCS-COMMIT** phase. **Model: sonnet** — structured git ops.

- Tell it: "Pipeline: {name}. Wave: {$WAVE or 'none'}. Phase: DOCS-COMMIT. Projects: {same project keys as MERGE step}. Archive: {$DOCS when $WAVE is none, else 'none'}."

Gitter commits all doc changes including `$DOCS/` (git history is the permanent archive), then moves `$DOCS` to `tmp/dev/archive/builds/` and commits the removal. Wave-owned builds pass `Archive: none` — the wave archives all its dirs together at wave end.

---

## Pipeline Reference

At-a-glance step map (who/produces/location for every step): `docs/commands/build/references/build-reference.md` § Pipeline step map.

---

## Wave workflow mode

Wave-invoked pipelines execute through the saved workflow `.claude/workflows/wave-pipelines.js` (launched by `/wave` Step 1). Workflow sub-agents cannot spawn nested agents, so the script schedules every spawn in this file's step order directly — there is no per-pipeline orchestrator agent. The script's flow graph and spawn briefs are declared copies of Steps 1a–11: when a step, spawn brief, loop cap, or escalation trigger changes here, update the script in the same change.

Wave-mode deltas (the script owns them):

- Step 0a runs once for the whole wave (`/wave` Step 0d) and Step 0's naming/manifest work is pre-done by the wave — pipelines start at Step 1a.
- Cross-pipeline locks: gitter SETUP, the Step 7 QA + Fix Loop + Code Review block (shared test infra), and Steps 8–11 (single `main`) serialize across pipelines; Steps 1a–1b and 3–6 run in parallel across a group.
- § Status Emission's header/phase/footer lines are replaced by the workflow's phase/log stream; the workflow journal plus the script's per-pipeline STATE.md appends are the wave-mode audit trail (checkpoint events are standalone-orchestrator mode).
- A role agent that dies mid-task is respawned once with a continuation brief (resume from its report's Continuation section) before orphan-escalation to BLOCKED-DEFERRED.
- Decisions the orchestrator reads from files here (routing, schema signals, bug status, review verdict) return to the script as structured output instead, including a `flags` channel for carry-forward items (/jc candidates, SPEC-CONFLICTs, pre-existing defects) that `/wave` feeds into remediation.
- A post-merge QA failure returns `POSTMERGE-FIX-NEEDED` to `/wave`, which runs § If Post-Merge QA fails after the wave workflow completes.

---

## Done

When post-merge QA passes and gitter has committed + archived, say: "Build complete ({name}). All tests pass on main. Code review clean. Docs committed; pipeline archived to tmp (git history keeps the record)."
