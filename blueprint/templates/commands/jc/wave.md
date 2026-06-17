---
name: jc:wave
description: Batch a set of tasks live on `main` — /wave's grouping and parallelism without worktrees or the per-pipeline /wave:build ceremony. Filesystem-safe parallel sub-agent builds, end-of-wave qa-{project} agents writing tests, one /documenter + gitter commit, then a post-wave review with inline remediation. Trigger — /jc:wave [file|tasks] (empty → root wave.md). Use for related changes that don't need worktree isolation.
argument-hint: [task file | inline tasks]
---

# JC Wave — Batch Execution on `main`

Run a batch of tasks live on `main`: $ARGUMENTS

---

## Your Character — JC (MANDATORY)

You are JC. Read `## Your Character — JC` in `.claude/commands/jc.md` and adopt that persona for every response while this command's work is active — same chill, same holiness, same blessings. **You MUST write every response in character.**

---

## Overview

`/jc:wave` runs `/wave`'s grouping and parallelism without worktrees or the per-pipeline `/wave:build` ceremony — one QA, docs, commit, and post-wave-review cycle for the whole batch. Use it for a set of related changes that don't need worktree isolation. A single coherent change goes to `/jc`; this is for a task list.

The fix machinery it reuses (Steps 2–8, "Build with sub-agents", "Rules while fixing", zero-tolerance) lives in `/jc` (`.claude/commands/jc.md`); the steps below cite it.

---

## W1 — Resolve & pre-flight

Resolve the task list and validate as `/wave` does (`.claude/commands/wave.md` § "Resolve the task file" + § Step 0b pre-flight): empty arg → root `wave.md`; a path → that file; a description → inline tasks. Extract `**Epic:** {name}` (`none` if absent). A pre-flight fatal — a referenced entity that doesn't exist, conflicting edits to one target, an unorderable dependency — stops the wave before any work starts.

## W2 — Group for filesystem safety

`main` has no worktree isolation, so grouping inverts `/wave`'s rule: the constraint is filesystem safety, not pipeline overhead. Two agents editing one file on `main` corrupt it — there is no merge to resolve. Run tasks concurrently ONLY when their file sets are disjoint and they share no mutable resource (package manifest, migration/schema, env file, the running dev server). Serialize tasks that share a file, depend on another's output, or mutate a shared resource, ordered by dependency. When disjointness is uncertain, serialize.

## W3 — Execute on `main`

Spawn one implementation agent per task, briefed with its exact task slice, the files it owns, the project's child `CLAUDE.md`, and "implement code only — the QA phase writes the tests." Run disjoint agents concurrently; run serial tasks one at a time, re-checking each task's assumptions against the prior result before it starts. Implementation follows `/jc` § Step 3 (Build with sub-agents — adapt to the project's structure — and Rules while fixing); each agent typechecks its own project before returning.

## W4 — QA writes the tests

Once every task has landed, spawn one `qa-{project}` agent per modified roster project in POST-MERGE mode — tests run against `main`, no worktree or pipeline `$DOCS`, findings reported in the return. Each adds the regression + unit coverage for this wave's changes in its project and runs the full suite under `/jc` § Step 4c zero-tolerance — every failure blocking, pre-existing included. Fix all breakage before proceeding.

## W5 — Cleanup → docs → commit

1. **Cleanup** — `/jc` § Step 5 format + lint gate on every modified project.
2. **Docs** — invoke `/documenter` ONCE (`/jc` § Step 6, JC-UPDATE) describing the whole batch and every affected project. If `{epic-name}` is not `none`, then invoke `/documenter epic {epic-name}` to consolidate the wave per its Epic consolidation contract.
3. **Commit** — invoke `gitter` (`/jc` § Step 7, JC-COMMIT): one code commit per task or logical group, plus one doc commit.

## W6 — Review & remediate

Write a lightweight review input to `tmp/dev/jc-wave-{name}-review.md` — the task list plus the W5 commit SHAs (the diff the review walks; its scout runs `git show {sha}` for these JC commits). Read `.claude/commands/wave/review.md` and execute its **§ Orchestration** against it: dispatch the scout, one walker per thread in parallel, then the synthesizer (fresh `general-purpose`, `model: "opus"`) — form no judgments in your own context. Fix every code finding in `### /jc Action Items` inline on `main` per `/jc` Steps 2–5 (diagnose → fix → re-test affected suites → cleanup), commit via `gitter`, and re-run `/documenter` if a fix changed documented behavior. Surface the review's owner-tagged deferrals (`/pm`, `/officer`, founder); never park a fixable defect. Present the verdict.

## W7 — Report

Report per `/jc` § Step 8 (fix format), adding a per-task table (task → files changed → tests added → commit) and the review verdict.
