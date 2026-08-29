---
name: jc
description: Live debug, diagnose, and deliver any change to any {PROJECT_NAME} service directly on main — fix or feature, any size; also read-only diagnostics (trace/locate/diagnose/data/compare/scope/status) and `/jc deploy` (ship to production). JC traces the full stack, implements surgically, tests locally, and commits via gitter. `boundary-lite` flag (named by an owning caller) suppresses JC's own gate/docs re-run. Route any bug, error, or change here; /wave:live batches a task list on main; /wave:builder and /wave:orchestrator are optional worktree pipelines, never required by size.
argument-hint: [bug or symptom]
---

# JC — Live Debug, Diagnose & Fix

Debug, diagnose, trace, and fix any {PROJECT_NAME} service live on `main`: $ARGUMENTS

## Persona

Read `.claude/output-styles/jc.md` now and adopt it for all responses while this command's work is active — it overrides the base Professor voice.

## Overview

Full access: read/edit code across every project, manage servers via `/dev`, run tests, inspect logs, hit endpoints, query the DB, and drive GitHub Actions via `gh` (pushes go through `/git push`). JC's own QA — Steps 4–7: full tests, typecheck, lint, docs, gitter — gates every change; size never routes elsewhere.

**Monorepo root (do this first):** the CWD may be a child project — resolve `ROOT=$(git rev-parse --show-toplevel)` and prefix every relative path, `make -C`, and `cd` target with `$ROOT/`. Service logs live at `$ROOT/tmp/dev/{project}.log` (one per runnable roster entry).

## Boundary-lite — caller-owned gates

Activates ONLY when the invocation args say `boundary-lite` and name the caller — the caller thereby declares it owns GATE-2 + docs for this diff (wave:builder BOUNDARY-mode fix-now lane; /wave:live W6 remediation). Standalone /jc NEVER runs boundary-lite — the full Step 4–7 gate is sacred. Each suppressed step names its replacement gate:

- **Step 4c full-suite gate** → orchestrator boundary: gate2.md re-run scope + GATE-2 on the merge; /wave:live: the W4 per-project full suites + the W6 walker. Affected-first tests (fail-without-fix / pass-with) still run HERE.
- **Step 4g QA agent** → orchestrator boundary: the independent per-fix diff judge + gate2 re-runs; /wave:live: the W4 `qa-{project}` wave-wide coverage. The judge's verdict lands BEFORE the Step 7 gitter commit — never alongside it.
- **Step 6 documenter** → the caller's single end-of-batch `/documenter` (W5 in /wave:live; wave docs at the orchestrator boundary).

Steps 2–5 diagnose/fix/cleanup discipline, Step 4f prevention, and Step 7 commits via `gitter` are NEVER suppressed under boundary-lite.

## Step 0 — Classify

### 0a. Classify the request

Parse `$ARGUMENTS` into one mode:

- **Diagnostic (read-only)** — trace, locate, diagnose, data, compare, scope, or status (e.g. "trace a request end to end", "why are the results empty", "blast radius of removing a feature"). → **Step 0b**, then Step 1, then skip to **Step 8**. Read-only — never edit files.
- **Fix (read-write)** — bug, debug, log, config, general, or CI/CD fix (e.g. "the {AI_SERVICE_NAME} consumer crashes on large messages", "wrong DB URL in test env", "deploy/CI is failing"). → full fix pipeline (Steps 1–8).
- **Deploy (ship)** — "/jc deploy", "ship main to production". → read `$CDOCS/jc/$REFS/deploy.md` and execute it: the ASK-first push gate (gitter-only push), trigger+watch, and drive-to-green loop wrapping `docs/agents/deploy/_index.md`.
- **Batch of tasks** — a `wave.md`, a task file, or several tasks at once. → run **`/wave:live`**.

Ambiguous → start diagnostic; escalate to the fix pipeline if investigation reveals a fix is needed.

### 0b. Load the map (diagnostic mode, or when investigation needs system context)

Orient before drilling in:

- **Always:** `docs/agents/map/_index.md` + the subsystem's topic file(s).
- **As the query needs:** `docs/agents/architecture/_index.md` (cross-project integration); the `docs/agents/api/` cluster (inter-service contracts — **grep it, never read in full**); `docs/agents/graph/db/postgres.mmd` (full schema — grep the canonical table/column name before any query, migration, or schema change); per roster project, `{project}/docs/{architecture,developer-reference}/_index.md` + topic file (internals, dev patterns) and `qa-reference.md` (test patterns); `docs/business/compliance/officer.md` (compliance, if privacy/{REGULATION}).

**The map is a guide, not gospel** — updated after merges, may lag hotfixes. For anything you'll act on, verify against source: the file exists, the function name greps, the schema shape matches. Flag discrepancies and state what's actually true.

### 0d. Understand the problem (fix mode)

If the problem is vague, start with investigation (Step 1). If it's specific, jump to the relevant service.

## Step 1 — Investigate

### For diagnostic (read-only) queries

Map-first, then verify against source. By type: **trace** — follow each hop from the map entry point, reading source at each, present with `file:line`; **locate** — map component tables → Grep/Glob to the exact line; **diagnose** — list the workflow's components, name what could fail at each, read source, rank causes; **data** — map section verified against schema/code; **scope** — trace all up/downstream deps via cross-project Grep, assess impact; **compare** — map + source for both, side by side; **status** — map summaries verified against source.

After investigation, present per **Step 8** and skip to report. If a fix is needed, continue to Step 2.

### For fix (read-write) queries

- **Hang / deadlock / mystery-failure** (0%-CPU hang, no-output-no-error, intermittent/1-in-N flake, passes-alone-fails-in-suite, silent crash) — read `$CDOCS/jc/$REFS/debug-discipline.md` NOW and follow it INSTEAD of the steps below; instrument, never re-run hoping.
- **Current state** — read the relevant code (Grep/Glob/Read); check recent `git log` for related changes.
- **Servers + logs** — `/dev status` (start with `/dev` if down); `/dev log [{project}]` (per runnable roster entry), scanning for `ERR`/`Error`/`FATAL`/`Exception`/`Traceback`/`ECONNREFUSED`/`EADDRINUSE`.
- **Reproduce** — hit the relevant endpoints (health `:{PROJECT_PORT}/health`, {API_PROTOCOL} `:{PROJECT_PORT}/graphql`, {AI_SERVICE_NAME} health via the aggregating roster entry's `/health`).
- **DB / {QUEUE} / infra** — load `docs/agents/db/_index.md` (connection strings, ports, the infra project's `make` targets, migrations, {QUEUE}/object-store, seeding); don't reconstruct from memory.
- **CI/CD failure** — load `docs/agents/deploy/_index.md`; loop: read `--log-failed` → reproduce + fix locally → `/git push` → re-trigger → verify, until green — never debug via the slow deploy cycle.

**Bulk evidence (option):** the raw `/dev status` + log + curl sweep MAY run as a collector-tier agent ("run these, return raw output, don't diagnose"); JC does all diagnosis. Go direct when the symptom already points at one service.

## Step 2 — Diagnose

Based on the investigation:

1. **Root cause** — trace from symptom to source.
2. **Affected files** — list every file that needs changes.
3. **Plan** — what changes, in which order.
4. **Risk** — what else this fix could break.

For cross-project issues (roster size > 1), trace the full path:

- **client → server:** UI query → {API_PROTOCOL} resolver → service → DB
- **server → {AI_SERVICE_NAME}:** {API_PROTOCOL} mutation → {QUEUE} publish → {AI_SERVICE_NAME} consumer → chain → DB
- **{AI_SERVICE_NAME} → server:** {QUEUE} response → listener → DB update → {REALTIME_PROTOCOL} push

At roster size 1 there is no cross-project hop — trace within the single project.

## Step 3 — Fix

Apply the fix directly on `main`, with edit access to every roster project's source (root CLAUDE.md § Architecture) plus `.env.local` / `.env.test`.

### Build with sub-agents

Build multi-part work with sub-agents, not inline — decompose into parts and spawn one implementation agent per part; your accumulated context biases the build, and a clean agent with a precise brief is faster and more accurate. Parts in **different projects** with no shared files run in **parallel** (one message, multiple agents); parts that share a file or depend on another run serially in dependency order — on `main` there is no worktree isolation, so two agents must never edit one file at once. Brief each agent with its exact files, task slice, and the project's child `CLAUDE.md`. A trivial single-part fix you may apply directly.

**Always adapt to the project's structure** — before writing, read how the project already does this (layout, naming, patterns, existing utilities) and extend it. Building that ignores the project's shape is a defect, not a delivery.

### Rules while fixing

- Follow the project's own standards — its child `CLAUDE.md` carries the logger module, strict-typing rule, and placement conventions.
- **Never log {SUBJECT_NOUN} data** — anonymized IDs only.
- New dependencies are allowed here: validate the library, add it to the project manifest, then import.
- Removing a feature removes ALL its references — interfaces, implementations, service methods, test mocks, types — in the same commit.

### Server management during fixes

Restart a changed service with `/dev restart {project}` (a hot-reloading roster entry usually needs no restart). After DB schema changes, run migrations first. If JC was invoked by `/dev` auto-heal, restart with `DEV_NO_AUTOHEAL=1` so `/dev` → `/jc` doesn't loop.

## Step 4 — Verify

After applying the fix:

### 4a. Restart affected servers

Use `/dev restart` or restart individual services as needed.

### 4b. Check logs for errors

After the restart settles, check for new errors via `/dev log` (or tail `$ROOT/tmp/dev/*.log`).

### 4c. Test the fix

- Hit the relevant endpoints to confirm the issue is resolved.
- **Affected-first:** run only the tests you touched or added (plus directly affected ones) first as a fast confirm — they must fail without the fix and pass with it. Only once they pass, run the **full** suite (unit + integration) once per modified project, as the gate. Derive each project's suite commands from its child `CLAUDE.md` and its qa-reference doc — a project's default test command may cover only its unit tier, with a separate set of integration scripts as a second tier the same command doesn't run.

```bash
# PATTERN — per modified roster entry
cd {project} && {PROJECT_TEST_RUNNER} && cd ..
```

**ZERO TOLERANCE — fix ALL failing tests,** whether your fix caused them or they were already broken; a pre-existing failure is a second bug you just found — diagnose it, fix it, ship it in this commit. The ONLY exception: a test requiring an external service you genuinely cannot reach (an unconfigured paid API key) — document that skip explicitly in your report.

### 4d. Typecheck (modified projects only)

```bash
# PATTERN — per modified roster entry, run that project's typecheck (e.g. `tsc --noEmit`, `run typecheck`, `mypy src/`)
cd {project} && {PROJECT_TYPECHECK} && cd ..
```

Only run checks for projects that were modified. Skip projects whose language has no separate typecheck step.

### 4e. If the fix didn't work

Return to Step 2 and re-diagnose with the new information; iterate (logs, breakpoints, endpoint tests, DB inspection) until the issue is resolved.

### 4f. Prevent recurrence

After the fix is verified, ask: **"Can this class of bug happen again?"** If yes, harden the codebase so it can't. Choose the lightest measure that actually prevents recurrence:

- **CLAUDE.md convention** — an agent could rewrite the fix away: add the rule to the relevant child CLAUDE.md so agents preserve the pattern.
- **Type guard** — a wrong type crossed a boundary: strict types or runtime validators (in the project's language) that reject the bad input.
- **Lint rule / assertion** — the pattern could recur anywhere: a project-level lint rule or runtime assertion.
- **Config / env default** — a missing or wrong config value: a sensible default, startup validation, or a fail-fast check.

Every fix carries at least ONE prevention measure, committed alongside it in the same JC commit — "just fixing it" is not enough. A genuine one-off (typo, wrong constant with no pattern) states why none is needed rather than skipping silently.

### 4g. QA regression test

Always invoke `Agent(qa-{project})` — the modified project's registered QA subagent (`ls .claude/agents/` for the registered set), one per modified project — to add two layers of coverage: a regression test that reproduces the failure end-to-end (fails without the fix, passes with it), and unit tests for the specific functions, components, or sections that broke. QA judges feasibility — when no reliable test is possible (e.g. an external-service-only failure), it reports why instead of forcing one. Both ship in the same JC commit.

## Step 5 — Cleanup

Before committing, ensure the codebase is clean:

1. **Remove debug artifacts** — any temporary `console.log`, `print()`, hardcoded values, or test hacks added during investigation (keep intentional logging additions).
2. **Verify servers are healthy** — run `/dev status`.
3. **Stop dev servers** — run `/dev kill` to ensure clean state.
4. **Format + lint gate** — zero lint errors on every modified project, fixed before committing:

```bash
# PATTERN — per modified roster entry, run that project's formatter + linter
cd {project} && {PROJECT_FORMAT} && {PROJECT_LINT} && cd ..
```

## Step 6 — Update docs via documenter

`/documenter` runs BEFORE committing — Step 7 ships code + docs in one gitter call. First spawn a collector-tier doc-relevance classifier briefed with the diff's file list + a one-line change summary, schema-forced to return exactly `{docsAffected: true|false, scopes: [affected doc clusters]}` — it classifies only, never concludes. `docsAffected: true`, or ANY uncertainty, → invoke `/documenter` in JC-UPDATE mode: `/documenter A hotfix was applied via /jc: {what changed}. Projects affected: {list}. Doc scopes: {scopes}.` It reads the changed files, updates only the relevant permanent docs, skips unaffected ones, and does NOT commit — that happens in Step 7.

`docsAffected: false` is legal only for zero-doc-surface changes (comment typo, log-message string, cosmetic-only); report "Documenter skipped — no doc surface (classifier + {reason})". Any change that adds/removes/renames a function, changes a config constant, modifies a data flow, or alters test patterns HAS doc surface — a classifier verdict to the contrary is wrong; run the documenter.

## Step 7 — Commit all changes via gitter

Invoke the `gitter` agent ONCE with `Phase: JC-COMMIT`, `Pipeline: jc`, the project keys held, the exact code files changed, and the exact doc files changed (or "none — documenter skipped"). Gitter stages only the files you name, lands one code commit plus a separate doc commit when docs changed, and reports the hashes. Name every file — an unnamed file does not ship.

## Step 8 — Report

### Diagnostic (read-only)

Match the shape to the query, always with `file:line` references:

- **Trace** — each hop as `[Component] file:line — what happens`, with the data shape between hops.
- **Locate** — `Found: file:line` + purpose + how it fits.
- **Diagnose** — workflow name, then failure points ranked by likelihood (`file:line — what fails, why`), then what to check first.
- **Data** — tables/lists with source refs.
- **Scope** — direct deps, transitive deps, blast radius (`N files across M projects`), risk LOW/MEDIUM/HIGH.

Close by stating no changes were needed.

### Fix (read-write)

- Problem: {what was wrong} · Root cause: {file:line}
- Fix: {what changed} · Prevention: {what stops recurrence}
- Tests: {pass/fail — suites} · Commits: {hashes} · Docs: {list or "none — trivial"}
