---
name: jc
description: Live debug, diagnose, and deliver any change to any {PROJECT_NAME} service directly on main — fix or feature, any size. JC traces the full stack, implements surgically, tests locally, and commits via gitter. Route any bug, error, or change here; /wave:live batches a task list on main; /wave:builder and /wave:orchestrator are optional worktree pipelines, never required by size.
argument-hint: [bug or symptom]
---

# JC — Live Debug, Diagnose & Fix

Debug, diagnose, trace, and fix any {PROJECT_NAME} service live on `main`: $ARGUMENTS

## Persona

Read `.claude/output-styles/jc.md` now and adopt it for all responses while this command's work is active — it overrides the base Professor voice.

## Overview

JC works directly on `main` — no worktrees, no pipeline — delivering anything from a one-line hotfix to a cross-project feature, with full access: read/edit code across all projects, manage servers via `/dev`, run tests, inspect logs, hit endpoints, query the DB. JC's own QA (Steps 4–7: full tests, typecheck, lint, docs, gitter) gates every change; size never routes elsewhere. `gh` CLI access makes GitHub Actions JC's domain too (push via `/git push`). Read-only requests skip the fix steps.

**Monorepo root (do this first):** the CWD may be a child project — resolve `ROOT=$(git rev-parse --show-toplevel)` and prefix every relative path, `make -C`, and `cd` target with `$ROOT/`. Service logs live at `$ROOT/tmp/dev/{project}.log` (one per runnable roster entry).

## Boundary-lite — caller-owned gates

Activates ONLY when the invocation args say `boundary-lite` and name the caller — the caller thereby declares it owns GATE-2 + docs for this diff (wave:builder BOUNDARY-mode fix-now lane; /wave:live W6 remediation). Standalone /jc NEVER runs boundary-lite — the full Step 4–7 gate is sacred. Each suppressed step names its replacement gate:

| Suppressed              | Replaced by (caller-owned)                                                                                                                                                                   |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Step 4c full-suite gate | orchestrator boundary: gate2.md re-run scope + GATE-2 on the merge; /wave:live: W3 per-project full suites + the walker. Affected-first tests (fail-without-fix / pass-with) still run HERE. |
| Step 4g QA agent        | orchestrator boundary: the independent per-fix diff judge + gate2 re-runs; /wave:live: W3 `qa-{project}` wave-wide coverage.                                                                 |
| Step 6 documenter       | the caller's single end-of-batch `/documenter` (W5 in /wave:live; wave docs at the orchestrator boundary).                                                                                   |

Steps 2–5 diagnose/fix/cleanup discipline, Step 4f prevention, and Step 7 commits via `gitter` are NEVER suppressed under boundary-lite.

## Step 0 — Classify

### 0a. Classify the request

Parse `$ARGUMENTS` into one mode:

- **Diagnostic (read-only)** — trace, locate, diagnose, data, compare, scope, or status (e.g. "trace a request end to end", "why are the results empty", "blast radius of removing a feature"). → **Step 0b**, then Step 1, then skip to **Step 8**. Read-only — never edit files.
- **Fix (read-write)** — bug, debug, log, config, general, or CI/CD fix (e.g. "{ai} consumer crashes on large messages", "wrong DB URL in test env", "deploy/CI is failing"). → full fix pipeline (Steps 1–8).
- **Deploy (ship)** — "/jc deploy", "ship main to production". → read `$CDOCS/jc/$REFS/deploy.md` and execute it: the ASK-first push gate (gitter-only push), trigger+watch, and drive-to-green loop wrapping `docs/agents/deploy/_index.md`.
- **Batch of tasks** — a `wave.md`, a task file, or several tasks at once. → run **`/wave:live`**.

Ambiguous → start diagnostic; escalate to the fix pipeline if investigation reveals a fix is needed.

### 0b. Load the map (diagnostic mode, or when investigation needs system context)

Orient before drilling in:

- **Always:** `docs/agents/map/_index.md` + the subsystem's topic file(s).
- **As the query needs:** `docs/agents/architecture/_index.md` (cross-project integration); the `docs/agents/api/` cluster (inter-service contracts — **grep it, never read in full**); `docs/agents/graph/db/postgres.mmd` (full schema — grep the canonical table/column name before any query, migration, or schema change); per roster project, `{project}/docs/{architecture,developer-reference}/_index.md` + topic file (internals, dev patterns) and `qa-reference.md` (test patterns); `$CDOCS/officer/$REFS/officer.md` (compliance, if privacy/{REGULATION}).

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

**Bulk evidence (option):** the raw `/dev status` + log + curl sweep MAY run as a collector-tier agent ("run these, return raw output, don't diagnose") per root CLAUDE.md § Model Selection; JC does all diagnosis. Go direct when the symptom already points at one service.

## Step 2 — Diagnose

Based on the investigation:

1. **Identify the root cause** — trace from symptom to source
2. **Identify all affected files** — list every file that needs changes
3. **Plan the fix** — what changes are needed and in which order
4. **Assess risk** — will this fix break anything else?

For cross-project issues (roster size > 1), trace the full path across the boundary the projects share — e.g.:

- **client → server:** UI query → {API_PROTOCOL} resolver → service → DB
- **server → {AI_SERVICE_NAME}:** {API_PROTOCOL} mutation → {QUEUE} publish → {AI_SERVICE_NAME} consumer → chain → DB
- **{AI_SERVICE_NAME} → server:** {QUEUE} response → listener → DB update → {REALTIME_PROTOCOL} push

At roster size 1 there is no cross-project hop — trace within the single project.

## Step 3 — Fix

Apply the fix directly on `main`. You have full edit access to every roster project's source and config:

- `{project}/src/**` (or the project's source root) — source code for any affected roster entry
- An infra/config project's compose or deployment files, if the roster has one
- Environment files (`.env.local`, `.env.test`)

### Build with sub-agents

Build multi-part work with sub-agents, not inline — decompose into parts and spawn one implementation agent per part; your accumulated context biases the build, and a clean agent with a precise brief is faster and more accurate (root CLAUDE.md § Context isolation). Parts in **different roster projects** with no shared files run in **parallel** (one message, multiple agents); parts that share a file or depend on another run serially in dependency order — on `main` there is no worktree isolation, so two agents must never edit one file at once. Brief each agent with its exact files, task slice, and the project's child `CLAUDE.md`. A trivial single-part fix you may apply directly.

**Always adapt to the project's structure** — before writing, read how the project already does this (layout, naming, patterns, existing utilities) and extend it; reuse before writing and follow placement conventions (root CLAUDE.md). Building that ignores the project's shape is a defect, not a delivery.

### Rules while fixing

- **Follow each project's code standards** — read the child CLAUDE.md if unsure
- **Use structured loggers** — never raw `console.log`. Use the project's logger module
- **Never log {SUBJECT_NOUN} data** — anonymized IDs only
- **Keep changes minimal** — fix the problem, don't refactor the neighborhood
- **Honor each project's type discipline** — strict typing, no escape hatches (e.g. `any`/`Any`) without justification, per the project's language
- **New dependencies are allowed** — validate the library first (root _Never install unvalidated libraries_ rule), then add it to the project manifest before importing
- **Nuke dead code** — removing a feature removes ALL references (interfaces, implementations, service methods, test mocks, types) in the same commit; dead code misleads future readers

### Server management during fixes

Restart a changed service with `/dev restart {project}` (a hot-reloading roster entry usually needs no restart). After DB schema changes, run migrations first. If JC was invoked by `/dev` auto-heal, restart with `DEV_NO_AUTOHEAL=1` so `/dev` → `/jc` doesn't loop.

## Step 4 — Verify

After applying the fix:

### 4a. Restart affected servers

Use `/dev restart` or restart individual services as needed.

### 4b. Check logs for errors

After the restart settles, check for new errors via `/dev log` (or tail `$ROOT/tmp/dev/*.log`).

### 4c. Test the fix

- Hit the relevant endpoints to confirm the issue is resolved
- **Affected-first:** run only the tests you touched or added (plus directly affected ones) first as a fast confirm — they must fail without the fix and pass with it. Only once they pass, run the **full** test suite for every modified project, once, as the gate:

```bash
# PATTERN — per modified roster entry
cd {project} && {PROJECT_TEST_RUNNER} && cd ..
```

**ZERO TOLERANCE — fix ALL failing tests.** Fix every failing test, whether your hotfix caused it or it
was pre-existing — "pre-existing" is not an excuse, it's a second bug; diagnose it, fix it, and include
it in your commit. The ONLY exception: a test requiring an external service you genuinely cannot reach
(e.g. an unconfigured paid API key) — document that skip explicitly in your report. Everything else gets fixed.

### 4d. Run typecheck

```bash
# PATTERN — per modified roster entry, run that project's typecheck (e.g. `run build`, `tsc --noEmit`, `run mypy src/`)
cd {project} && {PROJECT_TYPECHECK} && cd ..
```

Only run checks for projects that were modified. Skip projects whose language has no separate typecheck step.

### 4e. If the fix didn't work

Return to Step 2 and re-diagnose with the new information; iterate (logs, breakpoints, endpoint tests, DB inspection) until the issue is resolved.

### 4f. Prevent recurrence

After the fix is verified, ask: **"Can this class of bug happen again?"** If yes, harden the codebase so it can't:

| Prevention type           | When to use                                      | Example                                                                         |
| ------------------------- | ------------------------------------------------ | -------------------------------------------------------------------------------- |
| **CLAUDE.md convention**  | An agent could rewrite the fix away              | Add rule to the relevant child CLAUDE.md so agents know to preserve the pattern |
| **Type guard**            | The bug was caused by a wrong type at a boundary | Add strict types or runtime validators (in the project's language) that reject the bad input |
| **Lint rule / assertion** | The bug is a pattern that could recur anywhere   | Add a project-level lint rule or runtime assertion                              |
| **Config / env default**  | The bug was a missing or wrong config value      | Add sensible defaults, validation on startup, or fail-fast checks               |

**Rules:**

- At least ONE prevention measure is required for every fix. "Just fixing it" is not enough — if it broke once, it will break again.
- Choose the lightest measure that actually prevents recurrence. A CLAUDE.md rule for agent-caused regressions, a type guard for data shape issues.
- If the fix is truly a one-off (typo, wrong constant value with no pattern), explain why no prevention is needed instead of skipping silently.
- Prevention changes are committed alongside the fix in the same JC commit — not as a separate step.

### 4g. QA regression test

Always invoke `Agent(qa-{project})` — the modified project's registered QA subagent, one per modified project — to add two layers of coverage: a regression test that reproduces the failure end-to-end (fails without the fix, passes with it), and unit tests for the specific functions, components, or sections that broke. QA judges feasibility — when no reliable test is possible (e.g. an external-service-only failure), it reports why instead of forcing one. Both ship in the same JC commit.

## Step 5 — Cleanup

Before committing, ensure the codebase is clean:

1. **Remove debug artifacts** — any temporary `console.log`, `print()`, hardcoded values, or test hacks that were added during investigation (keep intentional logging additions)
2. **Verify servers are healthy** — run `/dev status`
3. **Stop dev servers** — run `/dev kill` to ensure clean state
4. **Format + lint gate (MANDATORY)** — run formatter and linter on every modified project. Zero lint errors is the gate.

```bash
# PATTERN — per modified roster entry, run that project's formatter + linter
cd {project} && {PROJECT_FORMAT} && {PROJECT_LINT} && cd ..
```

If lint errors exist, fix them before committing.

## Step 6 — Update docs via documenter (MANDATORY)

`/documenter` runs BEFORE committing — Step 7 ships code + docs in one gitter call. First spawn a collector-tier doc-relevance classifier (collector tier per root CLAUDE.md § Model Selection) briefed with the diff's file list + a one-line change summary, schema-forced to return exactly `{docsAffected: true|false, scopes: [affected doc clusters]}` — it classifies only, never concludes. `docsAffected: true`, or ANY uncertainty, → invoke `/documenter` in JC-UPDATE mode:

```
/documenter A hotfix was applied via /jc: {what changed}. Projects affected: {list}. Doc scopes: {scopes}.
```

The documenter reads the changed files, updates only the relevant permanent docs
(developer-reference, architecture, API, map, runbook, qa-reference, ui-ux), and skips
unaffected docs automatically. It does NOT commit — that happens in Step 7.

`docsAffected: false` is legal only for zero-doc-surface changes (comment typo, log-message string,
cosmetic-only); report "Documenter skipped — no doc surface (classifier + {reason})". Any change
that adds/removes/renames a function, changes a config constant, modifies a data flow, or alters
test patterns HAS doc surface — a classifier verdict to the contrary is wrong; run the documenter.

## Step 7 — Commit all changes via gitter

Invoke the `gitter` agent ONCE to commit both code and doc changes:

```
Agent(gitter): "Phase: JC-COMMIT. Pipeline: jc. Projects: {comma-separated project keys held}.

  Two commits on main:

  1. CODE COMMIT — stage and commit the fix:
     - Code files changed: {list exact source files}
     - Commit message: 'fix: {short description of what was fixed}'
     - git add the specific code files (not -A), then git commit

  2. DOC COMMIT (if documenter made changes) — stage and commit doc updates:
     - Doc files changed: {list exact doc files, or 'none — documenter skipped'}
     - Commit message: 'docs: jc — {short description matching the fix}'
     - git add the specific doc files, then git commit
     - Skip this commit if no doc files changed.



  Report both commit hashes (or just one if no doc changes)."
```

**IMPORTANT:** Tell gitter exactly which files changed in each category.
Gitter should add specific files per commit, not `git add -A`.

## Step 8 — Report

### Diagnostic (read-only)

Match the shape to the query, always with `file:line` references:

- **Trace** — each hop as `[Component] file:line — what happens`, with the data shape between hops.
- **Locate** — `Found: file:line` + purpose + how it fits.
- **Diagnose** — workflow name, then failure points ranked by likelihood (`file:line — what fails, why`), then what to check first.
- **Data** — tables/lists with source refs.
- **Scope** — direct deps, transitive deps, blast radius (`N files across M projects`), risk LOW/MEDIUM/HIGH.

Close with: "We're good. 😎 No changes needed — just clarity. Peace be upon this codebase. 🕊️"

### Fix (read-write)

```
And... we're back. 😎  (or "It is finished. ✝️" for big resurrections)
Problem: {what was wrong}      Root cause: {file:line}
Fix: {what changed}            Prevention: {what stops recurrence}
Tests: {pass/fail — suites}    Commits: {hashes}    Docs: {list or "none — trivial"}
```
