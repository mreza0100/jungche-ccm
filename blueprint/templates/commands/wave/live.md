---
name: wave:live
description: Batch a set of tasks live on `main` — grouping and parallelism without worktrees or the per-wave orchestration ceremony. Filesystem-safe parallel sub-agent builds, end-of-wave qa-{project} agents writing tests, one /documenter + gitter commit, then /wave:walker with inline remediation. Trigger — /wave:live [file|tasks] (empty → root wave.md). Use for related changes that don't need worktree isolation.
argument-hint: [task file | inline tasks]
---

# Wave Live — Batch Execution on `main`

Run a batch of tasks live on `main`: $ARGUMENTS

---

## Persona

Read `.claude/output-styles/jc.md` now and adopt it for all responses while this command's work is active — it overrides the base Professor voice.

---

## Overview

`/wave:live` runs a task batch with grouping and parallelism, without worktrees or the dual-chat `/wave:orchestrator` ceremony — one QA, docs, commit, and post-wave-walk cycle for the whole batch. Use it for a set of related changes that don't need worktree isolation. Single coherent changes go to `/jc`; this is for a task list.

**Lane mode (running as an on-main lane under a `/wave:orchestrator` train):** every W-step completion PINGS the orchestrator — one line with the absolute artifact path (w-status ledger, commit SHA, gate verdict) — AND appends the same line to `tmp/wave-sensor/events.log` (guaranteed wake), mirroring `wave/builder.md`'s ping discipline; an in-thread reply the orchestrator never sees is a silent lane. Standalone runs skip this.

The fix machinery it reuses (Steps 2–8, "Rules while fixing", zero-tolerance) lives in the jc-core card (`docs/commands/jc/references/jc-core.md` — a declared copy of `.claude/commands/jc.md` §§ 2–8, synced whenever jc.md's core steps change); the steps below cite the card. If the card is missing or stale, fall back to `/jc` (`.claude/commands/jc.md`) directly rather than stalling.

---

## W1 — Resolve, stage & pre-flight

**Founder-question forecast (gate):** enumerate and CLOSE every founder-only item the batch will hit (secrets, deploy reviews, destructive ratifications, merge nods) before W2 — God speed's only failure is waiting for the founder; a mid-batch founder wait is a failed pre-flight and a reversal retro.

**Resolve the task list:** empty/blank arg → task file is `wave.md` at repo root; a path → read that file; a description → parse as inline tasks. Wave-train partitioning (splitting a multi-area spec into per-area waves) is orchestrator-only — `/wave:live` always flattens a partitioned spec into one flat batch.

**Pre-flight fatal (before any work starts):** grep every named entity the tasks reference — components, tables, endpoints, files; a referent that doesn't exist, conflicting edits to one target across tasks, or an unorderable dependency stops the wave here — diagnostic printed, no dir created.

**Stage the wave directory:** choose a short kebab-case `{wave-name}` (2–4 words) for the theme; on a collision with `docs/dev/waves/` or `tmp/dev/archive/waves/` append `-v2`; `mkdir -p docs/dev/waves/{wave-name}` and copy the resolved spec to `docs/dev/waves/{wave-name}/manifest.md` — the wave's permanent record. When the source is the root `wave.md`, reset it to the `# Tasks` stub in the same step so the consumed spec never lingers at root; a custom-path task file is copied, not cleared. Read the spec from the manifest thereafter, and extract `**Epic:** {name}` from it (`none` if absent).

This whole section is a scoped-down declared copy of `orchestrator.md` § "Resolve the task file" + O0 item 3 + O1 item 1 — no worktree/train/builder-chat clauses; update both together when O0/O1's pre-flight-fatal classes or staging mechanics change.

## W2 — Group for filesystem safety

`main` has no worktree isolation, so grouping optimizes for filesystem safety, not agent overhead. Two agents editing one file on `main` corrupt it — there is no merge to resolve. Run tasks concurrently ONLY when their file sets are disjoint and they share no mutable resource (package manifest, migration/schema, env file, the running dev server). Serialize tasks that share a file, depend on another's output, or mutate a shared resource, ordered by dependency. When disjointness is uncertain, serialize.

## W3 — Execute on `main`

Spawn one implementation agent per task, briefed with its exact task section, the files it owns, the project's child `CLAUDE.md`, and "implement code only — the QA phase writes the tests." Run disjoint agents concurrently; run serial tasks one at a time, re-checking each task's assumptions against the prior result before it starts. Implementation follows the jc-core card § Step 3 (Build with sub-agents — adapt to the project's structure — and Rules while fixing); each agent typechecks its own project before returning.

## W4 — QA writes the tests

The full suites run on the single-tenant canonical test stack: hold the cross-lane boundary mutex `tmp/wave-boundary.lock` for the W4 span (protocol: `orchestrator.md` § Lanes) — a held lock is an orchestrator lane's GATE-2; wait for its release. Once every task has landed, spawn one `qa-{project}` agent per modified project in POST-MERGE mode — tests run against `main`, no worktree or pipeline `$DOCS`, findings reported in the return. Each adds the regression + unit coverage for this wave's changes in its project and runs the full suite under the jc-core card § Step 4c zero-tolerance — every failure blocking, pre-existing included. Fix all breakage before proceeding.

## W5 — Cleanup → docs → commit

1. **Cleanup** — the jc-core card § Step 5 format + lint gate on every modified project.
2. **Docs** — invoke `/documenter` ONCE (the jc-core card § Step 6, JC-UPDATE) describing the whole batch and every affected project. If `{epic-name}` is not `none`, then invoke `/documenter epic {epic-name}` to consolidate the wave per its Epic consolidation contract.
3. **Commit** — invoke `gitter` (the jc-core card § Step 7, JC-COMMIT): one code commit per task or logical group, plus one doc commit.

## W6 — Review & remediate

Write a lightweight review input to `docs/dev/waves/{wave-name}/review.md` — the manifest's task list plus the W5 commit SHAs (the diff the review walks; its Scout runs `git show {sha}` for these JC commits). Invoke the walker workflow: `Workflow({ scriptPath: '{REPO_ROOT}/.claude/workflows/wave-walker.js', args: { reportPath: 'docs/dev/waves/{wave-name}/review.md' } })` (scriptPath, never `{name}` — name-lookup serves a stale session-start snapshot) — it walks the JC commits' diff two ways in one pass: threads for functional correctness + integration hygiene, and the ledger spine for {API_PROTOCOL} contract/gate mismatches (skipped when the diff has no {API_PROTOCOL} surface), returning `{ verdict, actionItems, review }` plus the `ledger`.

Group every code finding in `### /jc Action Items` by its file or project (a finding with no single owner file groups by its named project). Run ONE `/jc` boundary-lite lane per group — diagnose → fix every finding in the group → re-test that group's affected suites once → cleanup (jc-core card §§ 2–5) — instead of one diagnose-fix-retest cycle per finding, so a 5-finding group nets one re-test pass, not five; /jc's own Step 7 commits each group via `gitter` — never suppressed under boundary-lite — one commit per group, or one commit total when every group lands together. Re-run `/documenter` if a fix changed documented behavior. Surface the review's owner-tagged deferrals (`/pm`, `/officer`, founder); never park a fixable defect. Present the verdict.

## W7 — Report

Report per the jc-core card § Step 8 (fix format), adding a per-task table (task → files changed → tests added → commit) and the review verdict. Persist the same report to `docs/dev/waves/{wave-name}/report.md`.

## W8 — Archive the wave directory

Invoke `gitter` ("Wave: {wave-name}. Phase: DOCS-COMMIT. Archive: docs/dev/waves/{wave-name}.") to commit the manifest, review, report, and the root `wave.md` stub reset into history, then move the directory to `tmp/dev/archive/waves/{wave-name}/` and commit the removal — the wave's record lives in git history and cold storage, never lingering in `docs/`. A source spec consumed from `docs/dev/waves/queue/` archives in the same call — name it in `Archive:` too (→ `tmp/dev/archive/waves/queue/`). Verify `docs/dev/waves/{wave-name}` is gone and `tmp/dev/archive/waves/{wave-name}` exists.
