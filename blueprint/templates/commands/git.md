# Git — Gitter Gateway

> **Tier C with light Jungche voice.** Mechanics — direct gateway to the `gitter` agent. Universal across stacks.

Talk to gitter directly: $ARGUMENTS

---

## Overview

`/git` is a thin gateway to the `gitter` agent. Use this when you need git ops outside a `/build` or `/jc` pipeline — pushing pending changes, pulling latest, or any other git request you'd otherwise hand to gitter manually.

**Important:** the rule **only `gitter` touches git** still applies. `/git` doesn't bypass gitter — it just gives you a shorter path to invoking gitter without writing the full Agent invocation block.

---

## Subcommand routing

| Subcommand | Trigger | Action |
|------------|---------|--------|
| `push [message]` | `$ARGUMENTS` starts with `push` | Stage all changes, commit (with optional message), push to remote |
| `pull` | `$ARGUMENTS` starts with `pull` | Pull latest from remote with `--ff-only` |
| `status` | `$ARGUMENTS` is `status` | Show git status across the repo |
| *(anything else)* | Freeform | Forward the full request to gitter as a freeform git operation |

---

## Subcommand: `push [message]`

```
Agent(gitter): "Phase: PUSH. Owner: /git invocation by user.
  Stage all changes (excluding secrets, env files, .worktrees/, tmp/), commit with the message:
  '{message or 'update: pending changes'}'
  Push origin main (or current branch if a PR branch is checked out — fail loudly if push to a non-main, non-PR-branch)."
```

The `gitter` agent is responsible for:
- Scanning for accidentally-staged secrets (`.env`, credentials, keys)
- Refusing destructive ops without explicit user confirmation
- Reporting commit hashes and remote update status

---

## Subcommand: `pull`

```
Agent(gitter): "Phase: PULL. Owner: /git invocation by user.
  Run git fetch, then git pull --ff-only on the current branch.
  If the pull cannot fast-forward (diverged history, merge needed), STOP and report — do not auto-merge."
```

---

## Subcommand: `status`

A lightweight read-only check — `gitter` can run `git status` and report. No staging, no commit, no push.

---

## Freeform mode

For anything else (`/git rebase`, `/git tag`, `/git stash`, etc.), forward the full request to gitter:

```
Agent(gitter): "Phase: FREEFORM. Owner: /git invocation by user.
  User request: '$ARGUMENTS'
  Apply the requested git operation. Refuse destructive operations (force push, hard reset, branch delete on main) without explicit user confirmation. Report the result."
```

---

## Rules

- **Only `gitter` touches git** — `/git` is a gateway, not a bypass. Never run git commands directly from this command.
- **Refuse destructive ops without confirmation** — the gitter agent enforces this; this command echoes the rule
- **Light Jungche voice in reports** — celebrate clean pushes with ✅, warn about diverged history with 🚩
- After finishing: forward gitter's report verbatim, then add a one-line Jungche summary
