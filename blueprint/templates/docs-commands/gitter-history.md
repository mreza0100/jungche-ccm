# Gitter — History & Large-File Registry

Reference data for gitter. Behavioral merge rules stay in `.claude/agents/gitter.md` § Living Reference; this file holds query-on-demand reference only. Gitter owns and may self-update it via the Edit tool.

## Pre-Migration History

Add notes here about any repo migration history (e.g. submodule-to-monorepo). `git log --all -- <path>` searches across pre-migration commits when `--all` is present.

## Large Files in Git History

Candidates for future `git filter-repo` or BFG removal if repo size becomes a problem.

| File | Size | Location | Introduced | Purpose |
| ---- | ---- | -------- | ---------- | ------- |

**To nuke from history later** (if migrating to LFS or removing):

```bash
git filter-repo --path <large-asset-dir>/ --invert-paths
```

## Confirmation Templates

Every gitter phase ends with the matching confirmation:

- **SETUP** — `Worktrees ready. Pipeline: $PIPELINE.\n  Branch: pipeline/$PIPELINE -> $WORKTREE (ports: one per roster server)`
- **MERGE** — `Merge complete. Pipeline: $PIPELINE.\n  Merged: pipeline/$PIPELINE -> main\n  Worktrees: cleaned up\n  Commit: <short-hash>`
- **DOCS-COMMIT** — `Docs committed. Pipeline: $PIPELINE.\n Docs: committed or no changes\n Archived to tmp: {paths or none}`
- **JC-COMMIT** — `Committed.` (+ both commit hashes if two commits made)
- **PUSH** — `Pushed. Here's what went up:\n  Commit: <short-hash>\n  Message: "$MESSAGE"`
- **PULL** — `Pulled. Up to date with origin/main.`

## ports.md Template

SETUP writes `$DOCS/ports.md` from `$WORKTREE/.env.ports` using this template:

```markdown
> Author: gitter

# Port Assignments — $PIPELINE

| Service   | Port        | Worktree Path       |
| --------- | ----------- | ------------------- |
| {project} | {port or —} | $WORKTREE/{project} |

Note any proxy wiring between roster projects. Note which projects are port-less.
```
