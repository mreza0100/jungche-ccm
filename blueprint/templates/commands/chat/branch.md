---
name: chat:branch
description: Fork THIS chat into a side-by-side tmux pane (the `Ctrl+b %` split) — a fresh-session-id fork that inherits this chat's model and takes the name you give it (the fork can't rename itself). The original chat stays live in the left pane; unlike the built-in in-place branch, which displaces your current chat. Trigger — /chat:branch [name].
argument-hint: [name]
disable-model-invocation: true
---

# Chat Branch — fork this chat into a side-by-side pane

Name: $ARGUMENTS

Mechanics: `split-window -h` runs the fork in the new pane; pass `--fork-session` (otherwise the fork drops to the default model) and `--name` for the given name. Reads `$CLAUDE_CODE_SESSION_ID` and `$TMUX` directly — nothing to resolve.

## Steps

1. **Branch:** `$HOME/.claude/commands/chat/chat.sh branch {name}` — pass `$ARGUMENTS` verbatim as the fork's name (omit if empty).
2. **Report** the script's output verbatim. On error (no session id, not in tmux, `claude` not on PATH) relay the line; nothing was spawned.
