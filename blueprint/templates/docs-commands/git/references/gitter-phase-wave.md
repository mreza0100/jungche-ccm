# Gitter Phase Card — WORKTREE-CHECKPOINT + SYNC

Gitter phase card — every core `gitter.md` rule (Remote Publication Boundary, Scoped-commit discipline, BANNED commands, commit convention) binds here.

## WORKTREE-CHECKPOINT

Invoked by `/wave:orchestrator` at each task boundary (relayed from the builder). Commits ON the worktree branch only — no main contact, no merge lock.

1. The orchestrator provides the task id and the EXPLICIT file list (the task's changed files + its `tasks/task-{n}-report.md`). Stage only that list — a checkpoint that sweeps half-written next-task files corrupts the per-task diff the orchestrator reviews.
2. In `$WORKTREE`: `git add <explicit paths>` → verify the staged set (`git status --porcelain`) → commit, type `feat($PIPELINE)`, desc `T{n}: {task title}`, standard trailers.
3. Return the checkpoint sha — the orchestrator's per-task diff anchor.

## SYNC

Invoked by `/wave:orchestrator` at milestones and before end-of-wave gates. Merges CURRENT `main` INTO the worktree branch so divergence surfaces where the wave's tests can exercise it — never in a blind end-merge.

1. In `$WORKTREE`: `git merge main --no-edit`.
2. Conflicts: MAIN wins on files the wave never intentionally changed (a concurrent `/jc` hotfix must survive); the branch wins on the wave's own files; ambiguous overlap on a wave-owned file → report both versions to the orchestrator and stop, never guess.
3. Report the merged + conflict-resolved file list. The orchestrator re-runs affected test profiles after any conflicted SYNC.
