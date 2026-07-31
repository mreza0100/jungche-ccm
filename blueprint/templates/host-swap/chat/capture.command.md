---
name: chat:capture
description: Snapshot another chat's LIVE tmux window — the full scrollback buffer (footer, spinner/idle state, task list, statusline: model, context %, cost) right now. The rendered screen, not the conversation; for the transcript use /chat:read. Target by tmux session name (find it with /chat:ls). Trigger — /chat:capture {tmux-session}.
argument-hint: [tmux-session]
---

# Chat Capture — snapshot another chat's live tmux window

Args: $ARGUMENTS

Capture works only on a LIVE pane — it returns the rendered screen: input box, queued drafts, spinners, agent cards, status bar (a fullscreen-TUI pane retains only the visible screen — the renderer repaints in place). Conversation depth is `/chat:read`'s job, never capture's.

## Steps

1. **Resolve the target.** `$ARGUMENTS` is `{tmux-session}`. If empty, ask the founder for the target, or run `/chat:ls` to find the session.
2. **Capture:** `$HOME/.claude/commands/chat/chat.sh capture {tmux-session}`.
3. **Report** the captured window. If it reports no live pane, the chat is dormant — offer `/chat:read` for its transcript.
