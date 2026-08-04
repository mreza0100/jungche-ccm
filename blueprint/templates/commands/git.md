---
name: git
description: Gateway to gitter, the only agent allowed to run git WRITES — routes push/pull to gitter phases and forwards anything else git-related as freeform. Route ALL git WRITES here; read-only git (status/diff/log/show/rev-parse) runs directly.
argument-hint: [push|pull|freeform request]
---

# Git — Gitter Gateway

Talk to gitter: $ARGUMENTS

Spawn the `gitter` agent (subagent_type: `gitter`) with a brief read off `$ARGUMENTS`; the user's words travel verbatim — gitter interprets them, this command never does.

- starts with `push` — `Phase: PUSH`, and `$MESSAGE` set to the text after `push`, or empty
- starts with `pull` — `Phase: PULL`
- anything else, empty included — name no Phase; the brief opens `The user ran /git with the following request:` and quotes `$ARGUMENTS`. Gitter handles a Phase-less brief as freeform.

`/git push`, or a user request that plainly says to push/publish to remote, is the only thing that may name `Phase: PUSH` — `gitter.md` § Remote Publication Boundary governs.
