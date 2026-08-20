# infra — the isolated dev fence

Development must never destabilize the live box: code changes happen in a git worktree under `.worktrees/`, and every build/test runs inside the `pfm-dev` container with that worktree mounted at `/worktree` — own HOME, own tmux, no published ports. Files are edited on the host; the container only builds and tests. Design and law: `docs/dev/isolated-dev-foundation.md`.

Entry point — from the worktree checkout:

```bash
.claude/scripts/dev.sh iso test pfm     # suite, in-container
.claude/scripts/dev.sh iso e2e          # the tagged e2e harness
.claude/scripts/dev.sh iso shell        # interactive fresh machine
```

The first output line of every run is the fence proof (`fence: container=… HOME=/root`); a run that cannot print it did not run inside the fence. Docker absent reports `TOOLCHAIN-MISSING` — never a silent host fallback.
