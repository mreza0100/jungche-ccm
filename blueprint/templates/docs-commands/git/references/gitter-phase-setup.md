# Gitter Phase Card — SETUP

Gitter phase card — every core `gitter.md` rule (Remote Publication Boundary, Scoped-commit discipline, BANNED commands, commit convention) binds here.

First pipeline stage — creates the worktree before planning and architecture run.

## 1. Validate preconditions

- Confirm `$DOCS/0-task.md` exists (the pre-placed task spec).
- Confirm no leftover worktree: `./.claude/scripts/worktree.sh list $PIPELINE`. If it exists, warn and stop — never overwrite.
- **Uncommitted changes on main** — handle per the orchestrator's `CarryWIP` directive (`commit` | `leave`, default `leave`). Run only when `git status --porcelain` is non-empty:
  - `commit` — commit main's WIP (untracked included) so the branch inherits it as a shared ancestor:
    ```bash
    git add -A && git commit -m "chore(wip): carry into pipeline/$PIPELINE"
    ```
  - `leave` — stash so worktree creation runs on a clean tree; restored to main in Step 2:
    ```bash
    git stash push --include-untracked -m "pre-pipeline stash: $PIPELINE"
    ```

## 2. Create worktree

```bash
./.claude/scripts/worktree.sh create $PIPELINE
```

Creates branch `pipeline/$PIPELINE` from `main`, checks out the full repo at `.worktrees/$PIPELINE/`, installs deps for every roster project, allocates ports, writes `.env.ports`. Then pop the stash if present and init the audit trail (`$WORKTREE/.checkpoint.json` logs which agent did what; gitignored, archived at MERGE):

```bash
if git stash list | grep -q "pre-pipeline stash: $PIPELINE"; then
  git stash pop || echo "WARNING: stash pop had conflicts — run 'git stash show' to inspect."
fi
bash .claude/scripts/checkpoint.sh init "$WORKTREE" "$PIPELINE"
```

## 3. Record port assignments

Read `$WORKTREE/.env.ports` and write `$DOCS/ports.md` from the template in `gitter-history.md` § ports.md Template (`> Author: gitter` byline, per-roster-project port table, proxy + port-less notes).

Confirm per template.
