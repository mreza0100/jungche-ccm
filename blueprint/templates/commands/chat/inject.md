---
name: chat:inject
description: Force a turn into another chat — auto-picks delivery: a live tmux pane gets it typed in and submitted now (send-keys); a dormant chat gets it appended to its transcript, answered on resume. Target by tmux session, session-id, or pasted excerpt. Trigger — /chat:inject {message} :: {target}.
argument-hint: [{message} :: {tmux-session | session-id | excerpt}]
---

# Chat Inject — force a turn into another chat (live if it's running)

Args: $ARGUMENTS

`/chat:inject` lands a real user turn in another chat and auto-picks how:

- **live tmux pane** → typed in and submitted now; the chat answers immediately.
- **dormant / not in tmux** → appended to its transcript; the chat answers on its next resume (backed up first).

## Steps

1. **Split message from target.** `$ARGUMENTS` is `{message} :: {target}`, where `{target}` is a tmux session name, a session-id, or a distinctive excerpt. If `::` is absent, ask the founder for the message and the target.
2. **Resolve the target** to a tmux session name or session-id:
   - tmux session name or session-id → pass straight through.
   - excerpt → write it to `tmp/chat-loads/inject-target.txt`, then `.claude/scripts/chat-find.sh tmp/chat-loads/inject-target.txt` for the session-id (confirm an ambiguous match against the printed date range first).
3. **Inject:** `.claude/scripts/chat-inject.sh {target} "{message}"`. It delivers LIVE if the target is a live tmux pane, else appends to the transcript.
4. **Report** which path it took from the script's output — LIVE (answered now) or RESUME (answered when that chat next opens). For a live nudge the target pane must be idle at its prompt; mid-turn it can interleave.

## To steer a RUNNING chat, target its tmux session — not its UUID

The LIVE send-keys arm fires only when the target resolves to a **live tmux pane**. A bare **session-id reaches it only if that chat self-registered** (the SessionStart hook); an unregistered or older chat has no UUID→pane map, so a session-id **falls back to the transcript (RESUME) arm** — which a _running_ target will not see until it reopens. A transcript append does not stop a live chat in time. **So to steer a running chat, pass its tmux session name.**

- **Find a chat's tmux session:** run `tmux display-message -p '#{session_name}'` inside it, or read its tmux status bar.
- **Confirm from the output:** `injected LIVE into tmux session …` landed live; `RESUME (no live pane found)` did not reach a running target — re-target by tmux session.
- **Idle only:** `send-keys` lands clean only when the target pane is idle at its prompt; mid-tool-call it can interleave — a heads-down agent cannot be cleanly interrupted.
