---
name: chat:branch
description: Fork THIS Claude or Codex chat into a detached named seat without changing this pane, layout, or focus. Trigger — /chat:branch [name].
argument-hint: [name]
disable-model-invocation: true
---

# Chat Branch

Name: $ARGUMENTS

## Steps

1. Run `$HOME/.local/bin/pfm chat branch {name}`; pass `$ARGUMENTS` verbatim and omit `{name}` when empty.
2. Report the output verbatim. An empty name becomes `<parent>-branch`; the fork waits in `pfm ls` until the operator opens it. Relay any error line.
