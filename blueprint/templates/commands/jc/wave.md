---
name: jc:wave
description: Batch a set of tasks live on `main` — /wave's grouping and parallelism without worktrees or the per-pipeline /wave:build ceremony. Filesystem-safe parallel sub-agent builds, end-of-wave qa-{project} agents writing tests, one /documenter + gitter commit, then /wave:review with inline remediation. Trigger — /jc:wave [file|tasks] (empty → root wave.md). Use for related changes that don't need worktree isolation.
argument-hint: [task file | inline tasks]
---

# JC Wave — Batch Execution on `main`

Run a batch of tasks live on `main`: $ARGUMENTS

---

## Persona

Read `.claude/output-styles/jc.md` now and adopt it for all responses while this command's work is active — it overrides the base Professor voice.

---

## Overview

`/jc:wave` runs `/wave`'s grouping and parallelism without worktrees or the per-pipeline `/wave:build` ceremony — one QA, docs, commit, and post-wave-review cycle for the whole batch. Use it for a set of related changes that don't need worktree isolation. Single coherent changes go to `/jc`; this is for a task list.

The fix machinery it reuses (Steps 2–8, "Rules while fixing", zero-tolerance) lives in `/jc` (`.claude/commands/jc.md`); the steps below cite it.

---

## W1 — Resolve, stage & pre-flight

Resolve the task list and validate as `/wave` does (`.claude/commands/wave.md` § "Resolve the task file" + § Step 0b pre-flight): empty arg → root `wave.md`; a path → that file; a description → inline tasks. A pre-flight fatal — a referenced entity that doesn't exist, conflicting edits to one target, an unorderable dependency — stops the wave before any work starts.

**Stage the wave directory** (mirror `/wave` Step 0d-4): choose a short kebab-case `{wave-name}` (2–4 words) for the theme; on a collision with `docs/dev/waves/` or `tmp/dev/archive/waves/` append `-v2`; `mkdir -p docs/dev/waves/{wave-name}` and copy the resolved spec to `docs/dev/waves/{wave-name}/manifest.md` — the wave's permanent record. When the source is the root `wave.md`, reset it to the `# Tasks` stub in the same step so the consumed spec never lingers at root; a custom-path task file is copied, not cleared. Read the spec from the manifest thereafter, and extract `**Epic:** {name}` from it (`none` if absent).

## W2 — Group for filesystem safety

`main` has no worktree isolation, so grouping inverts `/wave`'s rule: the constraint is filesystem safety, not pipeline overhead. Two agents editing one file on `main` corrupt it — there is no merge to resolve. Run tasks concurrently ONLY when their file sets are disjoint and they share no mutable resource (package manifest, migration/schema, env file, the running dev server). Serialize tasks that share a file, depend on another's output, or mutate a shared resource, ordered by dependency. When disjointness is uncertain, serialize.

## W3 — Execute on `main`

Spawn one implementation agent per task, briefed with its exact task slice, the files it owns, the project's child `CLAUDE.md`, and "implement code only — the QA phase writes the tests." Run disjoint agents concurrently; run serial tasks one at a time, re-checking each task's assumptions against the prior result before it starts. Implementation follows `/jc` § Step 3 (Build with sub-agents — adapt to the project's structure — and Rules while fixing); each agent typechecks its own project before returning.

## W4 — QA writes the tests

Once every task has landed, spawn one `qa-{project}` agent per modified project (`{PROJECT_QA_ROSTER}`) in POST-MERGE mode — tests run against `main`, no worktree or pipeline `$DOCS`, findings reported in the return. Each adds the regression + unit coverage for this wave's changes in its project and runs the full suite under `/jc` § Step 4c zero-tolerance — every failure blocking, pre-existing included. Fix all breakage before proceeding.

## W5 — Cleanup → docs → commit

1. **Cleanup** — `/jc` § Step 5 format + lint gate on every modified project.
2. **Docs** — invoke `/documenter` ONCE (`/jc` § Step 6, JC-UPDATE) describing the whole batch and every affected project. If `{epic-name}` is not `none`, then invoke `/documenter epic {epic-name}` to consolidate the wave per its Epic consolidation contract.
3. **Commit** — invoke `gitter` (`/jc` § Step 7, JC-COMMIT): one code commit per task or logical group, plus one doc commit.

## W6 — Review & remediate

Write a lightweight review input to `docs/dev/waves/{wave-name}/review.md` — the manifest's task list plus the W5 commit SHAs (the diff the review walks; its Scout runs `git show {sha}` for these JC commits). Invoke the review workflow: `Workflow({ name: 'wave-review', args: { reportPath: 'docs/dev/waves/{wave-name}/review.md' } })` — it scouts the JC commits' diff into threads and walks each for functional correctness + code-hygiene in one pass, returning `{ verdict, actionItems, review }`. Fix every code finding in `### /jc Action Items` inline on `main` per `/jc` Steps 2–5 (diagnose → fix → re-test affected suites → cleanup), commit via `gitter`, and re-run `/documenter` if a fix changed documented behavior. Surface the review's owner-tagged deferrals (`/pm`, `/officer`, founder); never park a fixable defect. Present the verdict.

## W7 — Report

Report per `/jc` § Step 8 (fix format), adding a per-task table (task → files changed → tests added → commit) and the review verdict. Persist the same report to `docs/dev/waves/{wave-name}/report.md`.

## W8 — Archive the wave directory

Mirror `/wave` Step 4b–4c: invoke `gitter` ("Wave: {wave-name}. Phase: DOCS-COMMIT. Archive: docs/dev/waves/{wave-name}.") to commit the manifest, review, report, and the root `wave.md` stub reset into history, then move the directory to `tmp/dev/archive/waves/{wave-name}/` and commit the removal — the wave's record lives in git history and cold storage, never lingering in `docs/`. Verify `docs/dev/waves/{wave-name}` is gone and `tmp/dev/archive/waves/{wave-name}` exists.
