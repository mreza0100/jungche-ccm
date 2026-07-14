# Gitter Phase Card — MERGE

Gitter phase card — every core `gitter.md` rule (Remote Publication Boundary, Scoped-commit discipline, BANNED commands, commit convention) binds here. Gitter owns § Gotchas and self-updates it.

Invoked **after QA** reports `Status: NONE` in `$DOCS/6-bugs.md`.

## 0. Acquire the merge lock + check for concurrent merges

The advisory lock guards `main` against two pipelines merging at once; busy = another pipeline is mid-merge — report busy, retry shortly. Released in Step 6b. Then check concurrency:

```bash
bash .claude/scripts/git-lock.sh acquire "pipeline/$PIPELINE"
git status --short  # main may carry uncommitted WIP — a wave can launch dirty; expected
ls .worktrees/*/MERGING 2>/dev/null && echo "CONCURRENT MERGE DETECTED" || echo "Clear"
```

If another pipeline is actively merging, wait and retry.

## 1. Validate preconditions

- Read `$DOCS/6-bugs.md` from disk and confirm it contains `Status: NONE` — file absent or status not NONE → refuse and report which. **Wave-v2 mode** (orchestrator passes `Wave-mode: v2`): `$DOCS` derives to `docs/dev/waves/$WAVE` instead, and the precondition is `$DOCS/gate1.md` ON DISK, all-green (per-project PASS, no unresolved `INTEGRATION-UNRUN`). The dispatch brief's own verdict text never substitutes for the file — a merge validated against an in-brief claim is ungated.
- Confirm worktree exists: `./.claude/scripts/worktree.sh list $PIPELINE`

## 2. Commit all worktree changes

```bash
cd $WORKTREE
git add -A
if ! git diff --cached --quiet; then
  git commit  # type: feat($PIPELINE), desc: "$PIPELINE implementation"
fi
cd -
```

## 3. Merge to main

`main` may carry uncommitted WIP (a wave can launch dirty). Stash it, merge on a clean tree, restore — `--no-ff` guarantees an explicit merge commit for traceability:

```bash
git checkout main
WIP_STASH=
git status --porcelain | grep -q . && git stash push --include-untracked -m "merge-wip: $PIPELINE" && WIP_STASH=1
git merge pipeline/$PIPELINE --no-ff -m "..."  # type: merge($PIPELINE)
[ -n "$WIP_STASH" ] && { git stash pop || echo "WIP-POP-CONFLICT"; }
```

**Permission classifier blocks the stash on a large dirty `main`** — if the sandbox denies `git stash push` even after the orchestrator's empty-overlap ruling (see § Gotchas), skip the stash and merge directly on the dirty tree; git only blocks a merge if it would overwrite uncommitted changes, so a confirmed-empty overlap merges cleanly.

**Branch merge conflicts** — `git diff --name-only --diff-filter=U` to list, resolve (implementation over scaffolding, newer over older, worktree branch when in doubt), commit: type `merge($PIPELINE)`, desc "resolve conflicts for $PIPELINE".

**WIP stash-pop conflicts** (`WIP-POP-CONFLICT`) — main's uncommitted WIP critically overlaps the merged changes. The only condition that pauses the wave: STOP, list the conflicting files to the WATCHER handle (`tmp/wave-sensor/watcher.handle`; the founder only when no watcher runs), and ask for a commit-or-resolve ruling on the WIP — never discard it. A clean pop restores the WIP and the wave continues.

Verify with `git log --oneline -5`.

## 4. Propagate new .env fields

For each roster project, compare worktree `.env.local`/`.env.test` with main; append new keys (preceded by `# Added by pipeline $PIPELINE`) to main. Skip silently if none.

## 5. Archive the audit trail, then clean up worktree — UNCONDITIONAL

**This step runs in BOTH standalone and Wave-v2 mode, in the SAME dispatch as the merge itself — a MERGE that returns with `.worktrees/{name}` still on disk is INCOMPLETE.** The DOCS-COMMIT `Archive:` parameter governs docs archival only — `Archive: none` never skips this step. Salvage before removal: any wave/build doc dirty INSIDE the worktree's copy (`$WORKTREE/docs/dev/waves/**`, `$WORKTREE/docs/dev/builds/**` — builders sometimes write through worktree-relative paths) diffs against its root counterpart; unique or differing content is copied root-side first. Removal never touches the BRANCH (`pipeline/{name}` stays as the revert path).

```bash
bash .claude/scripts/checkpoint.sh archive "$WORKTREE" "$DOCS/audit-trail.json"
./.claude/scripts/worktree.sh remove $PIPELINE
ls .worktrees/   # VERIFY: $PIPELINE must be absent — if listed, the merge is not done; retry/report, never proceed silently
```

The completion report's final line states `worktree removed: {name}` — the orchestrator treats a MERGE report without it as unfinished.

## 6. Update § Gotchas (only if needed)

Update only if a new gotcha, git-structure change, or future-merge workaround was discovered. Never log routine merges.

## 6b. Release the merge lock

```bash
bash .claude/scripts/git-lock.sh release
```

Confirm per template.

## Gotchas

Gitter's living memory of merge gotchas — self-updated when a structural change or recurring problem is discovered, never for routine merges. Seed it from your own repo. Common shapes:

- **Worktree artifacts:** `.env.ports`, `.env.local`, `.env.test` get staged. Check `git status` and unstage generated files before committing.
- **Dependency-symlink projects:** a roster project whose worktree symlinks the main checkout's dependency dir (e.g. `node_modules`, `.venv`) can appear in `git status` as a new tracked file — unstage before committing. If it slips to main, `git rm --cached {project}/{dep-dir}` and commit immediately.
- **Concurrent pipeline conflicts:** when multiple pipelines modify the same files, keep the implementation version. The conflict-awareness check prevents simultaneous merges, not simultaneous development.
- **Large-binary / LFS-pointer mismatch blocks merge:** binaries showing `M` on `main` pre-merge despite identical bytes = git expects LFS pointers. `git restore` does NOT fix it; `git stash push -- <files>` (only the flagged binaries) → merge → `git stash pop` does. Long-term: migrate binaries to Git LFS.
- **Permission classifier blocks `git stash push` on main's dirty tree even with a pre-verified empty overlap:** the sandbox's auto-mode classifier can flag `git stash push --include-untracked` on a large dirty `main` as irreversible-destruction risk, regardless of an orchestrator ruling that the branch diff and main's dirty-file list have zero overlap. Workaround: skip the stash and run `git merge pipeline/$PIPELINE --no-ff` directly on the dirty tree — git only refuses a merge if it would overwrite uncommitted local changes, so a confirmed-empty overlap merges cleanly without touching main's WIP. Verify post-merge that main's dirty-file count is unchanged (`git status --porcelain | wc -l` before/after must match) and that `git status --porcelain | grep -E '^(UU|AA|DD)'` is empty.
