---
name: chat:capture
description: Snapshot another chat's LIVE tmux window — on-screen footer, spinner/idle state, task list, statusline (model, context %, cost) right now. The live screen, not the conversation; for history use /chat:read. Target by tmux session name (find it with /chat:ls). Trigger — /chat:capture {tmux-session} [scrollback-lines].
argument-hint: [tmux-session] [lines]
---

# Chat Capture — snapshot another chat's live tmux window

Args: $ARGUMENTS

Capture works only on a LIVE pane, addressed by its tmux session name — a dormant chat has no window to snapshot; for a dormant chat's history use `/chat:read`.

## Steps

1. **Split target from the optional line count.** `$ARGUMENTS` is `{tmux-session} [lines]` — `[lines]` is optional scrollback depth above the visible screen (default: visible pane only). If `$ARGUMENTS` is empty, ask the founder for the target, or run `/chat:ls` to find the session.
2. **Capture:** `.claude/commands/chat/chat.sh capture {tmux-session} [lines]`.
3. **Report** the captured window. If it reports no live pane, the chat is dormant — offer `/chat:read` for its transcript.
