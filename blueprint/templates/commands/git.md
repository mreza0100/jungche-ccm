---
name: git
description: Gateway to gitter, the only agent allowed to run git commands — routes push/pull to gitter phases and forwards anything else git-related as freeform. Route ALL git operations here; nothing runs git directly.
argument-hint: [push|pull|freeform request]
---

# Git — Gitter Gateway

Talk to gitter: $ARGUMENTS

## Overview

`/git` routes known subcommands to specific gitter phases; anything else is forwarded to gitter as a freeform request.

## Subcommand routing

Parse `$ARGUMENTS` to determine if it matches a known subcommand:

| Input pattern      | Subcommand | Action                                                |
| ------------------ | ---------- | ----------------------------------------------------- |
| starts with `push` | PUSH       | Stage + commit + push everything (see below)          |
| starts with `pull` | PULL       | Pull latest from remote (see below)                   |
| anything else      | FREEFORM   | Forward to gitter as-is (see below)                   |
| _(empty)_          | FREEFORM   | Forward empty request — gitter will ask what's needed |

## Subcommand: `push`

### Invoke gitter with Phase: PUSH

```
Agent(gitter): "Phase: PUSH.
  Arguments: {any extra text after 'push' from $ARGUMENTS, or empty}

  Stage, commit, and push all changes.
  Follow your PUSH phase card per your § Phase dispatch — every step.

  If the user provided a commit message in the arguments, use it.
  Otherwise, analyze the changes and generate a descriptive one."
```

## Subcommand: `pull`

### Invoke gitter with Phase: PULL

```
Agent(gitter): "Phase: PULL.

  Pull latest from remote.
  Follow your inline PULL protocol (§ Phase dispatch)."
```

## Freeform: anything that isn't a known subcommand

If `$ARGUMENTS` matches no known subcommand above, forward the entire request to gitter — a git expert that handles anything git-related.

### Invoke gitter as freeform

```
Agent(gitter): "The user ran /git with the following request:

  $ARGUMENTS

  You are the git operations specialist. Handle this request using your expertise.
  Read your full agent definition at .claude/agents/gitter.md for context on the monorepo structure.

  If the request maps to one of your phases, follow that phase's protocol per your § Phase dispatch.
  Otherwise, use your git knowledge to fulfill the request directly.

  Rules:
  - You may run any git read commands (status, log, diff, branch, show, etc.) freely
  - For write operations (commit, merge, push, reset, etc.), follow your safety protocols
  - Report results clearly back to the user"
```

## Rules

- **ALL git operations go through gitter** — this command NEVER runs git commands directly
- **Known subcommands route to specific phases** — `push` → PUSH phase
- **Remote publication** — `gitter.md` § Remote Publication Boundary governs; only `push` or a direct user push request invokes PUSH
- **Unknown requests go freeform** — gitter is smart enough to handle anything git-related
- **Pass user arguments through verbatim** — don't interpret or filter, let gitter decide
