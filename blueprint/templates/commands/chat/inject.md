---
name: chat:inject
description: Force a turn into another chat or into this one — 'self' or a live tmux session gets it typed in and submitted now (send-keys); a session-id or excerpt gets it appended to that chat's transcript, answered on resume. Trigger — /chat:inject {target} {message} (or {message} :: {target}).
argument-hint: [[--no-sig] [--force-now] {self | tmux-session | session-id} {message}]
---

# Chat Inject — force a turn into another chat (or this one)

Args: $ARGUMENTS

`/chat:inject` lands a real user turn and auto-picks how:

- **`self`, a live tmux session, or a 🔖 label** → typed into that pane and submitted now (LIVE) — the whole message inline, no length cap and no file, however long. A **label** is the destination's `/rename` name (the 🔖 in its status line); the script resolves it to a tmux session by scanning live panes the way `/chat:ls` does, so you can address a chat as `WAVE` or `VISION` instead of its tmux number. Matching is case-insensitive; an ambiguous label (two chats share it) errors and asks for the session id. The script owns the Enter and protects the target's draft: before typing it presses `Ctrl+S`, which stashes any unsent draft (restored automatically after the next submit) and is a no-op on an empty box — so it never has to read the input; it waits for the text to render, then double-taps Enter (0.15s apart) to defeat a swallowed first press — single-tap when a draft was stashed, so the restored draft is never re-submitted — and confirms the input cleared, so **you never press Enter yourself**. If it cannot confirm submission (target busy mid-turn or in a selector) it warns rather than leaving a half-sent turn. Keep the message a single line — a bare newline submits early in the target's input. `self` targets this chat's own pane, queuing a turn for after the current one completes.
- **session-id or excerpt** → appended to that chat's transcript (RESUME); it answers on its next reopen (backed up first).

## Steps

1. **Split target from message.** Two accepted forms:
   - **Target-first** `{target} {message}` — the first whitespace-delimited token is the target (`self`, a tmux session name, or a session-id); everything after it is the message. This is the form the reply-footer teaches.
   - **Legacy** `{message} :: {target}` — message first, then `::`, then the target; use this when the target is a distinctive **excerpt** (multi-word) rather than a single handle.
     If neither a leading single-token target nor `::` is present, ask the founder for the message and the target.
2. **Resolve the target:**
   - `self`, a tmux session name, a 🔖 label, or a session-id → pass straight through; the script resolves a label to its session itself (`ls`-style scan), so just forward what the founder typed.
   - excerpt → write it to `tmp/chat-loads/inject-target.txt`, then `.claude/commands/chat/chat.sh find tmp/chat-loads/inject-target.txt` for the session-id (confirm an ambiguous match against the printed date range first).
3. **Inject (signed):** `.claude/commands/chat/chat.sh inject {target} "{message}"`. It delivers LIVE for `self` or a live tmux pane, else appends to the transcript. The script self-derives the sender identity — its own tmux session (via `whoami`) plus the short session id — so do not pass your own name or id; identity is the script's job. For an unsigned, operational injection (`/compact`, `/goal`, `/loop`, …), add `--no-sig` before the target — see below.
4. **Report** which path it took from the output — LIVE (answered now) or RESUME (answered on reopen).

## Every injected message is signed

The script appends a footer to the end of every message — `— sid {sender session-id} · to reply: /chat:inject {sender tmux} <message> · 🔖 {sender label}` — so the recipient knows the source and gets a runnable reply command. All three fields are script-derived: the `sid` from the session, the `to reply:` handle from the sender's own tmux session (its live reply handle), and the `🔖 {sender label}` (the sender's own `/rename` name, read from its statusline) shown next to the handle so the recipient sees who sent it and can reply by label too. The 🔖 segment is omitted when the sender has no label. The typed (LIVE) footer is single-line; the RESUME transcript gets a block footer.

**Suppressing the footer** — two ways:

- **Auto** — a message that IS a slash command (starts with `/`) injects verbatim into a live pane with no footer (it would land as command arguments). The command lands clean so the target executes it.
- **Explicit `--no-sig`** — `chat.sh inject --no-sig {target} "{message}"` drops the footer for any message, in both arms (LIVE and RESUME). Use it for operational injections the target must consume clean — `/compact`, `/goal`, `/loop`, and the like — especially when driving another chat programmatically.

## Interrupt a busy target with `--force-now`

By default a message injected into a busy target queues behind its current turn. `chat.sh inject --force-now {target} "{message}"` instead presses `Esc` to interrupt the running tool/flow so the target reads the message now, then delivers it. It only acts on a live pane (ignored with a warning for a transcript/RESUME target — a dormant chat has no running flow), and only interrupts when the target is actually busy; an idle target is delivered to normally. When it does interrupt, it appends a marker — `⚠ FORCE-DELIVERED via Esc (your running flow was interrupted; re-check any in-progress action)` — so the recipient knows its work was cut off and should re-check any half-done action. Combine with `--no-sig` for a clean operational interrupt. Use sparingly: interrupting an agent mid-tool-call can leave its work half-done.

## Concurrent senders are serialized

When several chats inject into the **same** live pane at once, their keystrokes would otherwise interleave into one mangled turn. Each LIVE delivery takes a per-target lock (an atomic `mkdir` lock under `${TMPDIR}/chat-inject-locks/`, released when the inject finishes) so deliveries to one pane run one-at-a-time; a second inject waits up to `CHAT_INJECT_LOCK_TIMEOUT` (30s) then warns rather than colliding. A lock whose owner died or has been held too long is reclaimed automatically. Before typing, the script also exits the target's copy/scroll mode and dismisses a stuck Rewind menu — both silently swallow input.

## To steer a RUNNING chat, target its tmux session — not its UUID

The LIVE send-keys arm fires only for `self` or a **live tmux session name**. A bare session-id has no pane map, so it **falls back to the transcript (RESUME) arm** — which a _running_ target will not see until it reopens, so it cannot stop a live chat in time. To steer a running chat, pass its **tmux session name**.

- **Find a chat's tmux session:** run `/chat:ls`, or `/chat:whoami` inside the target, or read its tmux status bar.
- **Confirm from the output:** `injected LIVE into tmux session …` landed live; `RESUME …` did not reach a running target — re-target by tmux session.
- **Idle only:** `send-keys` lands clean only when the target pane is idle at its prompt; mid-tool-call it can interleave — a heads-down agent cannot be cleanly interrupted.
