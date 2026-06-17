---
name: chat:inject
description: Force a turn into another chat or into this one — 'self' or a live tmux session gets it typed in and submitted now (send-keys); a session-id or excerpt gets it appended to that chat's transcript, answered on resume. Trigger — /chat:inject {target} {message} (or {message} :: {target}).
argument-hint: [[--no-sig] {self | tmux-session | session-id} {message}]
---

# Chat Inject — force a turn into another chat (or this one)

Args: $ARGUMENTS

`/chat:inject` lands a real user turn and auto-picks how:

- **`self` or a live tmux session** → typed into that pane and submitted now (LIVE). `self` targets this chat's own pane, queuing a turn for after the current one completes.
- **session-id or excerpt** → appended to that chat's transcript (RESUME); it answers on its next reopen (backed up first).

## Steps

1. **Split target from message.** Two accepted forms:
   - **Target-first** `{target} {message}` — the first whitespace-delimited token is the target (`self`, a tmux session name, or a session-id); everything after it is the message. This is the form the reply-footer teaches.
   - **Legacy** `{message} :: {target}` — message first, then `::`, then the target; use this when the target is a distinctive **excerpt** (multi-word) rather than a single handle.
     If neither a leading single-token target nor `::` is present, ask the founder for the message and the target.
2. **Resolve the target:**
   - `self`, a tmux session name, or a session-id → pass straight through.
   - excerpt → write it to `tmp/chat-loads/inject-target.txt`, then `.claude/commands/chat/chat.sh find tmp/chat-loads/inject-target.txt` for the session-id (confirm an ambiguous match against the printed date range first).
3. **Inject (signed):** `CHAT_INJECT_FROM_NAME="{this chat's name}" .claude/commands/chat/chat.sh inject {target} "{message}"`. It delivers LIVE for `self` or a live tmux pane, else appends to the transcript. Pass `CHAT_INJECT_FROM_NAME` with this chat's own display name (the 🔖 name in your status line) so the recipient sees who sent it. For an unsigned, operational injection (`/compact`, `/goal`, `/loop`, …), add `--no-sig` before the target — see below.
4. **Report** which path it took from the output — LIVE (answered now) or RESUME (answered on reopen).

## Every injected message is signed

The script appends a footer to the end of every message — `— from {name} · sid {sender session-id} · to reply: /chat:inject {sender tmux} <message>` — so the recipient knows the source and gets a runnable reply command. The `sid` is always derived; the `to reply:` line appears whenever the sender is in tmux (its session is the live reply handle); the human `{name}` appears only when you pass `CHAT_INJECT_FROM_NAME`. The typed (LIVE) footer is single-line; the transcript and any long-message spill file get a block footer, and that spill's live pointer also names the sender.

**Suppressing the footer** — two ways:

- **Auto** — a message that IS a slash command (starts with `/`) injects verbatim into a live pane: no footer (it would land as command arguments) and no long-message file-cap (a file pointer gets read, never run). The command lands clean so the target executes it.
- **Explicit `--no-sig`** — `chat.sh inject --no-sig {target} "{message}"` drops the footer for any message, in every arm (LIVE, the spill file, and RESUME). Use it for operational injections the target must consume clean — `/compact`, `/goal`, `/loop`, and the like — especially when driving another chat programmatically. The long-message cap still applies; only the signature is gone.

## To steer a RUNNING chat, target its tmux session — not its UUID

The LIVE send-keys arm fires only for `self` or a **live tmux session name**. A bare session-id has no pane map, so it **falls back to the transcript (RESUME) arm** — which a _running_ target will not see until it reopens, so it cannot stop a live chat in time. To steer a running chat, pass its **tmux session name**.

- **Find a chat's tmux session:** run `/chat:ls`, or `/chat:whoami` inside the target, or read its tmux status bar.
- **Confirm from the output:** `injected LIVE into tmux session …` landed live; `RESUME …` did not reach a running target — re-target by tmux session.
- **Idle only:** `send-keys` lands clean only when the target pane is idle at its prompt; mid-tool-call it can interleave — a heads-down agent cannot be cleanly interrupted.
