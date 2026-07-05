# Gitter — History & Large-File Registry

Reference data for gitter. Behavioral merge gotchas live in `gitter-phase-merge.md` § Gotchas (same dir); this file holds query-on-demand reference only. Gitter owns and self-updates both.

## Confirmation Templates

Every gitter phase ends with the matching confirmation:

- **SETUP** — `Worktrees ready. Pipeline: $PIPELINE.\n  Branch: pipeline/$PIPELINE -> $WORKTREE (ports {per-roster-project})`
- **MERGE** — `Merge complete. Pipeline: $PIPELINE.\n  Merged: pipeline/$PIPELINE -> main\n  Worktrees: cleaned up\n  Commit: <short-hash>`
- **DOCS-COMMIT** — `Docs committed. Pipeline: $PIPELINE.\n  Docs: committed or no changes\n  Archived to tmp: {paths or none}`
- **JC-COMMIT** — `Committed.` (+ both commit hashes if two commits made)
- **WORKTREE-CHECKPOINT** — `Checkpoint T{n}: {sha} on pipeline/$PIPELINE.`
- **SYNC** — `Synced main -> pipeline/$PIPELINE. Merged: {files}. Conflicts resolved: {files or none}.`
- **PUSH** — `Pushed. Here's what went up:\n  Commit: <short-hash>\n  Message: "$MESSAGE"`
- **PULL** — `Pulled. Up to date with origin/main.`

## ports.md Template

SETUP writes `$DOCS/ports.md` from `$WORKTREE/.env.ports` using this exact template — one row per roster project (a project with no HTTP port shows `—`):

```markdown
> Author: gitter

# Port Assignments — $PIPELINE

| Service        | Port         | Worktree Path        |
| -------------- | ------------ | -------------------- |
| {PROJECT_ROLE} | {port or —}  | $WORKTREE/{project}  |

{Note any dev-server proxy routing and any port-less project — e.g. a pure queue consumer with no HTTP port.}
```

## Pre-Migration History (optional — fill in from your own repo)

If your repo has a history discontinuity (e.g. a submodule → monorepo migration), record how to reach pre-discontinuity history here — otherwise delete this section. Example:

```bash
git log --all -- {project}/          # history across the boundary
git blame {project}/{file}           # works through the merge boundary
```

## Large Files in Git History (optional — fill in from your own repo)

Candidates for a future `git filter-repo` / BFG pass if repo size becomes a problem. Seed the table from your repo, or delete this section.

| File                | Size | Location  | Introduced | Purpose |
| ------------------- | ---- | --------- | ---------- | ------- |
| {path/to/large.bin} | {MB} | {project} | {when}     | {why}   |

**To nuke from history later** (if migrating to LFS or removing):

```bash
git filter-repo --path {path/to/large-dir/} --invert-paths
```
