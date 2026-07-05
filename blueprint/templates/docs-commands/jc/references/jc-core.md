# JC Core — Fix Loop (Steps 2–8)

> Declared copy of `.claude/commands/jc.md` §§ 2–8 (Diagnose → Fix → Verify → Cleanup → Docs → Commit → Report) — synced whenever jc.md's core steps change; source of truth is `.claude/commands/jc.md`. Consumed by `/wave:live` (W3–W8) so a wave batch doesn't hold the full `/jc` command (persona, Step 0 classify, Step 1 investigate, the debug-discipline on-demand card, and the boundary-lite section stay JC-invocation-only and are out of scope here). If this card is missing or stale, fall back to `.claude/commands/jc.md` directly.

---

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

---

## Step 3 — Fix

Apply the fix directly on `main`. You have full edit access to every roster project's source and config:

- `{project}/src/**` (or the project's source root) — source code for any affected roster entry
- An infra/config project's compose or deployment files, if the roster has one
- Environment files (`.env.local`, `.env.test`)

### Build with sub-agents

Build multi-part work with sub-agents, not inline — decompose into parts and spawn one implementation agent per part; your accumulated context biases the build, and a clean agent with a precise brief is faster and more accurate. Parts in **different roster projects** with no shared files run in **parallel** (one message, multiple agents); parts that share a file or depend on another's output run serially in dependency order — on `main` there is no worktree isolation, so two agents must never edit one file at once. Brief each agent with its exact files, task slice, and the project's child `CLAUDE.md`. A trivial single-part fix you may apply directly.

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

Restart a changed service with `/dev restart {project}` (a hot-reloading dev server usually needs no restart). After DB schema changes, run migrations first. If JC was invoked by `/dev` auto-heal, restart with `DEV_NO_AUTOHEAL=1` so `/dev` → `/jc` doesn't loop.

---

## Step 4 — Verify

After applying the fix:

### 4a. Restart affected servers

Use `/dev restart` or restart individual services as needed.

### 4b. Check logs for errors

After the restart settles, check for new errors via `/dev log` (or tail `$ROOT/tmp/dev/*.log`).

### 4c. Test the fix

- Hit the relevant endpoints to confirm the issue is resolved
- **Affected-first:** run only the tests you touched or added (plus directly affected ones) first as a fast confirm — they must fail without the fix and pass with it. Only once they pass, run the **full** test suite for every modified project, once, as the gate — one PATTERN block per modified roster entry, using that entry's test runner:

```bash
# PATTERN — per modified roster entry
cd {project} && {PROJECT_TEST_RUNNER} && cd ..
```

**ZERO TOLERANCE — fix ALL failing tests.** If tests fail, you fix them — period. It does not matter
whether the failure was caused by your hotfix or was pre-existing. JC leaves `main` cleaner than he
found it. "Pre-existing" is not an excuse — it's a second bug you just discovered. Diagnose it, fix it,
and include it in your commit. If you walked past a broken test and committed anyway, you blessed
broken code — and that is not what JC does.

The ONLY acceptable exception: a test that requires external services you genuinely cannot reach
(e.g., a paid API key that isn't configured locally). In that case, document the skip explicitly
in your report. Everything else gets fixed.

### 4d. Run typecheck

```bash
# PATTERN — per modified roster entry, run that project's typecheck (e.g. `run build`, `tsc --noEmit`, `run mypy src/`)
cd {project} && {PROJECT_TYPECHECK} && cd ..
```

Only run checks for projects that were modified. Skip projects whose language has no separate typecheck step.

### 4e. If the fix didn't work

Go back to Step 2 — re-diagnose with the new information. Repeat until the issue is resolved.
Do NOT give up after one attempt. Use logs, breakpoints, endpoint testing, and database inspection
to iteratively narrow down the root cause.

### 4f. Prevent recurrence

After the fix is verified, ask: **"Can this class of bug happen again?"** If yes, harden the codebase so it can't:

| Prevention type           | When to use                                      | Example                                                                                      |
| ------------------------- | ------------------------------------------------ | -------------------------------------------------------------------------------------------- |
| **CLAUDE.md convention**  | An agent could rewrite the fix away              | Add rule to the relevant child CLAUDE.md so agents know to preserve the pattern              |
| **Type guard**            | The bug was caused by a wrong type at a boundary | Add strict types or runtime validators (in the project's language) that reject the bad input |
| **Lint rule / assertion** | The bug is a pattern that could recur anywhere   | Add a project-level lint rule or runtime assertion                                           |
| **Config / env default**  | The bug was a missing or wrong config value      | Add sensible defaults, validation on startup, or fail-fast checks                            |

**Rules:**

- At least ONE prevention measure is required for every fix. "Just fixing it" is not enough — if it broke once, it will break again.
- Choose the lightest measure that actually prevents recurrence. A CLAUDE.md rule for agent-caused regressions, a type guard for data shape issues.
- If the fix is truly a one-off (typo, wrong constant value with no pattern), explain why no prevention is needed instead of skipping silently.
- Prevention changes are committed alongside the fix in the same JC commit — not as a separate step.

### 4g. QA regression test

Always invoke `Agent(qa-{project})` — the modified project's registered QA subagent, one per modified project — to add two layers of coverage: a regression test that reproduces the failure end-to-end (fails without the fix, passes with it), and unit tests for the specific functions, components, or sections that broke. QA judges feasibility — when no reliable test is possible (e.g. an external-service-only failure), it reports why instead of forcing one. Both ship in the same JC commit.

---

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

If lint errors exist, fix them before committing. JC does not bless unformatted code. 🙏

---

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

---

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

---

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
