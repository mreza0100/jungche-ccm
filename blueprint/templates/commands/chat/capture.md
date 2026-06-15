---
name: chat:capture
description: Snapshot another chat's LIVE tmux window — its on-screen footer, spinner/idle state, task list, and statusline (model, context %, cost) right now. The live screen, not the conversation; for history use /chat:read. Trigger — /chat:capture {tmux-session | session-id | pasted excerpt} [scrollback-lines].
argument-hint: [tmux-session | session-id | excerpt] [lines]
---

# Chat Capture — snapshot another chat's live tmux window

Args: $ARGUMENTS

Capture works only on a LIVE pane — a dormant chat has no window to snapshot; for a dormant chat's history, use `/chat:read` instead.

## Steps

1. **Split target from the optional line count.** `$ARGUMENTS` is `{target} [lines]` — `{target}` is a tmux session name, a session-id, or a distinctive excerpt; `[lines]` is optional scrollback depth above the visible screen (default: visible pane only). If `$ARGUMENTS` is empty, ask the founder for the target.
2. **Resolve the target** to a tmux session or session-id:
   - tmux session name or session-id → pass straight through.
   - excerpt → write it verbatim to `tmp/chat-loads/capture-target.txt`, then `.claude/scripts/chat-find.sh tmp/chat-loads/capture-target.txt` for the session-id (confirm an ambiguous auto-pick against the printed date range first).
3. **Capture:** `.claude/scripts/chat-capture.sh {target} [lines]` — resolves the target to its live tmux pane and prints the window.
4. **Report** the captured window to the founder. If the script reports no live pane, the chat is dormant — offer `/chat:read` for its transcript.
