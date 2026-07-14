---
name: chat:inject
description: Force a turn into another chat or into this one — 'self' or a live tmux session gets it typed in and submitted now (send-keys); a session-id or excerpt gets it appended to that chat's transcript, answered on resume. Restart all MCP servers on 'restart mcp' by self-injecting /mcp disable then /mcp enable. Trigger — /chat:inject {target} {message} (or {message} :: {target}).
argument-hint: [[--force-now] [--then {steer}] [--file {path}] {self | tmux-session | session-id} {message}]
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
   - excerpt → write it to `tmp/chat-loads/inject-target.txt`, then `$HOME/.claude/commands/chat/chat.sh find tmp/chat-loads/inject-target.txt` for the session-id (confirm an ambiguous match against the printed date range first).
3. **Inject (signed):** `$HOME/.claude/commands/chat/chat.sh inject {target} "{message}"`. It delivers LIVE for `self` or a live tmux pane, else appends to the transcript. The script self-derives the sender identity — its own tmux session (via `whoami`) plus the short session id — so do not pass your own name or id; identity is the script's job. Signing is automatic and mandatory; only `/`-prefixed commands travel unsigned (auto-detected — see below).
4. **Report** which path it took from the output — LIVE (answered now) or RESUME (answered on reopen).

## Every injected message is signed

The script appends a footer to the end of every message — `— sid {sender session-id} · to reply: /chat:inject {sender tmux} <message> · 🔖 {sender label}` — so the recipient knows the source and gets a runnable reply command. All three fields are script-derived: the `sid` from the session, the `to reply:` handle from the sender's own tmux session (its live reply handle), and the `🔖 {sender label}` (the sender's own `/rename` name, read from its statusline) shown next to the handle so the recipient sees who sent it and can reply by label too. The 🔖 segment is omitted when the sender has no label. The typed (LIVE) footer is single-line; the RESUME transcript gets a block footer.

**Unsigned prompts — only `/`-prefixed commands, auto-detected (founder law):**

- A message starting with `/` is a harness command and injects verbatim with no footer — a trailing signature would corrupt its arguments. The script detects the prefix itself; no flag needed.
- Every plain-text message is signed, always. It is impossible to send a normal prompt unsigned — there is no opt-out flag; the message prefix alone decides. (Sender identity is load-bearing: an unsigned nudge hides who is speaking.)

## Interrupt a busy target with `--force-now`

By default a message injected into a busy target queues behind its current turn. `chat.sh inject --force-now {target} "{message}"` instead presses `Esc` to interrupt the running tool/flow so the target reads the message now, then delivers it. It only acts on a live pane (ignored with a warning for a transcript/RESUME target — a dormant chat has no running flow), and only interrupts when the target is actually busy; an idle target is delivered to normally. When it does interrupt, it appends a marker — `⚠ FORCE-DELIVERED via Esc (your running flow was interrupted; re-check any in-progress action)` — so the recipient knows its work was cut off and should re-check any half-done action. Use sparingly: interrupting an agent mid-tool-call can leave its work half-done.

## Send shell syntax safely with `--file`

`chat.sh inject --file {path} {target}` reads the message body from a file instead of argv. The mangler is the CALLER's shell — not tmux, not chat.sh: a message carrying redirects, pipes, backticks, or `$` needs only one imperfect quote for the caller's shell to eat or execute part of it (a relayed command example once ran as `zsh: command not found: cmd` on the recipient's box). A file never crosses a shell, so command forms arrive byte-exact. Use it for any message carrying shell metacharacters, and for anything spanning lines.

## `--file` carries the MESSAGE, not the document

The file holds the words you would have typed — an IMPERATIVE. It is not a way to paste a document into a chat. A brief, a spec, or a report is dispatched **by pointer** — `"/jc per {abs path} — {the one-line ask}"` — never by injecting its content: a chat handed a markdown document reads it as _material_, not as an _order_, and will study it instead of acting on it.

## A /compact focus is a POINTER, not a payload

`chat.sh` REFUSES a `/compact` whose focus exceeds `COMPACT_FOCUS_MAX` (600 chars; exit 6, nothing typed). A body that long is typed as a bracketed PASTE, the TUI collapses it to `[Pasted text #N] · paste again to expand`, the Enter lands on the collapsed block, and **the compaction never fires** — the message just sits in the composer as queue-limbo (seen twice live). Write the hold to a file and make the focus a pointer:

`chat.sh inject --then "{steer}" {target} "/compact hold: read {abs path} — {the 2-3 facts that must survive verbatim}"`

This is also the better practice regardless of the transport: a file on disk survives the summary intact, while a 2,000-character focus competes with the summary for the same budget.

## Carry a follow-up past a /compact with `--then`

`chat.sh inject --then "{steer}" {target} "/compact {focus}"` delivers `{steer}` as a second turn the moment the primary turn finishes and the pane returns to idle. It exists for `/compact`: compaction leaves the chat idle, so a steer typed while it runs is swallowed — `--then` rides out the busy→idle transition, then types the steer onto the settled pane. The waiter is **detached**, because a self-inject's waiter runs inside the very turn it must wait on — that turn cannot end until the inject returns — so it survives the turn as a background process and delivers through a fresh inject that takes its own lock. The steer follows the same signature rule as any message (a `/`-prefixed steer travels bare). It works for any primary message (it simply waits for that turn to end), but `/compact` is the case it is built for; a non-live (RESUME) target has no turn to wait on, so `--then` is ignored there with a warning. **A `/compact` inject REQUIRES `--then` (founder law)** — chat.sh rejects a steerless compact: compaction ends at an idle prompt with no turn fired, stranding the target command-less.

## Restart all MCP servers — "restart mcp"

`/mcp disable` then `/mcp enable` cycles every MCP server. Both are user-typed slash commands, not tools, so fire them on this pane as two self-injects, in order — they queue behind the current turn FIFO, so disable runs first and enable second:

```bash
$HOME/.claude/commands/chat/chat.sh inject self "/mcp disable"
$HOME/.claude/commands/chat/chat.sh inject self "/mcp enable"
```

Use two ordered injects, not `--then`: the `--then` waiter fires on the current turn's busy→idle edge and beats the still-queued `/mcp disable`, so enable lands first and the servers stay down. FIFO ordering is what guarantees disable-then-enable.

When the founder says "restart mcp", run exactly these two.

## Concurrent senders are serialized

When several chats inject into the **same** live pane at once, their keystrokes would otherwise interleave into one mangled turn. Each LIVE delivery takes a per-target lock (an atomic `mkdir` lock under `${TMPDIR}/chat-inject-locks/`, released when the inject finishes) so deliveries to one pane run one-at-a-time; a second inject waits up to `CHAT_INJECT_LOCK_TIMEOUT` (30s) then warns rather than colliding. A lock whose owner died or has been held too long is reclaimed automatically. Before typing, the script also exits the target's copy/scroll mode and dismisses a stuck Rewind menu — both silently swallow input.

## To steer a RUNNING chat, target its tmux session — not its UUID

The LIVE send-keys arm fires only for `self` or a **live tmux session name**. A bare session-id has no pane map, so it **falls back to the transcript (RESUME) arm** — which a _running_ target will not see until it reopens, so it cannot stop a live chat in time. To steer a running chat, pass its **tmux session name**.

- **Find a chat's tmux session:** run `/chat:ls`, or `/chat:whoami` inside the target, or read its tmux status bar.
- **Confirm from the output:** `injected LIVE into tmux session …` landed live; `RESUME …` did not reach a running target — re-target by tmux session.
- **Idle only:** `send-keys` lands clean only when the target pane is idle at its prompt; mid-tool-call it can interleave — a heads-down agent cannot be cleanly interrupted.
