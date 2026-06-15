---
name: chat:inject
description: Force a turn into another chat or into this one — 'self' or a live tmux session gets it typed in and submitted now (send-keys); a session-id or excerpt gets it appended to that chat's transcript, answered on resume. Trigger — /chat:inject {message} :: {target}.
argument-hint: [{message} :: {self | tmux-session | session-id | excerpt}]
---

# Chat Inject — force a turn into another chat (or this one)

Args: $ARGUMENTS

`/chat:inject` lands a real user turn and auto-picks how:

- **`self` or a live tmux session** → typed into that pane and submitted now (LIVE). `self` targets this chat's own pane, queuing a turn for after the current one completes.
- **session-id or excerpt** → appended to that chat's transcript (RESUME); it answers on its next reopen (backed up first).

## Steps

1. **Split message from target.** `$ARGUMENTS` is `{message} :: {target}`, where `{target}` is `self`, a tmux session name, a session-id, or a distinctive excerpt. If `::` is absent, ask the founder for the message and the target.
2. **Resolve the target:**
   - `self`, a tmux session name, or a session-id → pass straight through.
   - excerpt → write it to `tmp/chat-loads/inject-target.txt`, then `.claude/commands/chat/chat.sh find tmp/chat-loads/inject-target.txt` for the session-id (confirm an ambiguous match against the printed date range first).
3. **Inject:** `.claude/commands/chat/chat.sh inject {target} "{message}"`. It delivers LIVE for `self` or a live tmux pane, else appends to the transcript.
4. **Report** which path it took from the output — LIVE (answered now) or RESUME (answered on reopen).

## To steer a RUNNING chat, target its tmux session — not its UUID

The LIVE send-keys arm fires only for `self` or a **live tmux session name**. A bare session-id has no pane map, so it **falls back to the transcript (RESUME) arm** — which a _running_ target will not see until it reopens, so it cannot stop a live chat in time. To steer a running chat, pass its **tmux session name**.

- **Find a chat's tmux session:** run `/chat:ls`, or `/chat:whoami` inside the target, or read its tmux status bar.
- **Confirm from the output:** `injected LIVE into tmux session …` landed live; `RESUME …` did not reach a running target — re-target by tmux session.
- **Idle only:** `send-keys` lands clean only when the target pane is idle at its prompt; mid-tool-call it can interleave — a heads-down agent cannot be cleanly interrupted.
