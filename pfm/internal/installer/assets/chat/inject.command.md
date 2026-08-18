---
name: chat:inject
description: Force a turn into another chat or into this one — 'self', a live tmux session, or its 🔖 label gets it typed in and submitted now (send-keys); a session-id or excerpt gets it appended to that chat's transcript, answered on resume. Long bodies automatically become durable short pointers. `--force-now` interrupts a busy target (Esc); `--file {path}` reads shell-sensitive input safely; `--then {steer}` queues a follow-up turn for after the primary lands (e.g. post-/compact); repeat it to chain several steers, delivered in order one settled turn apart. Restart all MCP servers on 'restart mcp' by self-injecting /mcp disable then /mcp enable. Trigger — /chat:inject {target} {message} (or {message} :: {target}).
argument-hint: [[--force-now] [--then {steer}]... [--file {path}] {self | tmux-session | session-id} {message}]
---

# Chat Inject — force a turn into another chat (or this one)

Args: $ARGUMENTS

`/chat:inject` lands a real user turn and auto-picks how:

- **`self`, a live tmux session, or a 🔖 label** → typed into that pane and submitted now (LIVE). Short prose travels inline; long prose becomes a durable signed pointer. Slash commands always travel in full through locked, paced literal chunks. A **label** is the destination's `/rename` name (the 🔖 in its status line); pfm resolves it to a tmux session by scanning live panes the way `/chat:ls` does. Matching is case-insensitive; an ambiguous label errors and asks for the session id. The engine owns the Enter, protects an unsent draft, and confirms that the composer cleared, so **you never press Enter yourself**. A busy Claude or Codex pane accepts the message in its composer queue without interruption. `self` targets this chat's own pane and queues the turn after the current one.
- **session-id or excerpt** → appended to that chat's transcript (RESUME); it answers on its next reopen (backed up first).

## Steps

1. **Split target from message.** Two accepted forms:
   - **Target-first** `{target} {message}` — the first whitespace-delimited token is the target (`self`, a tmux session name, or a session-id); everything after it is the message. This is the form the reply-footer teaches.
   - **Legacy** `{message} :: {target}` — message first, then `::`, then the target; use this when the target is a distinctive **excerpt** (multi-word) rather than a single handle.
     If neither a leading single-token target nor `::` is present, ask the operator for the message and the target.
2. **Resolve the target:**
   - `self`, a tmux session name, a 🔖 label, or a session-id → pass straight through; pfm resolves a label to its session itself (`ls`-style scan), so just forward what the operator typed.
   - excerpt → write it to `tmp/chat-loads/inject-target.txt`, then `$HOME/.local/bin/pfm chat find tmp/chat-loads/inject-target.txt` for the session-id (confirm an ambiguous match against the printed date range first).
3. **Inject (signed):** `$HOME/.local/bin/pfm chat inject {target} "{message}"`. It delivers LIVE for `self` or a live tmux pane, else appends to the transcript. The engine self-derives the sender identity — its own tmux session (via `whoami`) plus the short session id — so do not pass your own name or id; identity is the engine's job. Signing is automatic and mandatory; only `/`-prefixed commands travel unsigned (auto-detected — see below).
4. **Report** which path it took from the output — LIVE (answered now) or RESUME (answered on reopen).

## Every injected message is signed

The script appends a footer to the end of every message — `— sid {sender session-id} · to reply: /chat:inject {sender tmux} <message> · 🔖 {sender label}` — so the recipient knows the source and gets a runnable reply command. All three fields are script-derived: the `sid` from the session, the `to reply:` handle from the sender's own tmux session (its live reply handle), and the `🔖 {sender label}` (the sender's own `/rename` name, read from its statusline) shown next to the handle so the recipient sees who sent it and can reply by label too. The 🔖 segment is omitted when the sender has no label. The typed (LIVE) footer is single-line; the RESUME transcript gets a block footer.

**Unsigned prompts — only `/`-prefixed commands, auto-detected (operator rule):**

- A message starting with `/` is a harness command and injects verbatim with no footer — a trailing signature would corrupt its arguments. The script detects the prefix itself; no flag needed.
- Every plain-text message is signed, always. It is impossible to send a normal prompt unsigned — there is no opt-out flag; the message prefix alone decides. (Sender identity is load-bearing: an unsigned nudge hides who is speaking.)

## Interrupt a busy target with `--force-now`

By default a message injected into a busy target queues behind its current turn. `$HOME/.local/bin/pfm chat inject --force-now {target} "{message}"` instead presses `Esc` to interrupt the running tool/flow so the target reads the message now, then delivers it. It only acts on a live pane (ignored with a warning for a transcript/RESUME target — a dormant chat has no running flow), and only interrupts when the target is actually busy; an idle target is delivered to normally. When it does interrupt, it appends a marker — `⚠ FORCE-DELIVERED via Esc (your running flow was interrupted; re-check any in-progress action)` — so the recipient knows its work was cut off and should re-check any half-done action. Use sparingly: interrupting an agent mid-tool-call can leave its work half-done.

## Send shell syntax safely with `--file`

`$HOME/.local/bin/pfm chat inject --file {path} {target}` reads the message body from a file instead of argv. The compatibility spelling `{target} --file {path}` is also accepted, but keep flags first in authored commands. The mangler is the CALLER's shell — not tmux or pfm: a message carrying redirects, pipes, backticks, or `$` needs only one imperfect quote for the caller's shell to eat or execute part of it. A file never crosses a shell, so command forms arrive byte-exact. A short prose file uses bracketed paste for multi-line safety; a long prose file is copied into the canonical auto-file store and only its pointer enters the chat. A slash command read from a file still travels in full as paced literal chunks.

## Long bodies automatically become durable pointers

Plain prose has no user-visible size limit. Its complete signed wire message stays at or below the empirically measured safety boundary:
720 runes for Claude, 900 for Codex. Above it, pfm writes the original body byte-exact to
`~/.local/state/pfm/inject-bodies/<utc-stamp>-<target>.md` (mode 0600; bodies older than seven
days are pruned) and sends a short message: the bounded first-line caption plus
`read <path> fully`, followed by the normal signature. The success receipt says `AUTO-FILE` and
names the same path. An 8 KiB body therefore reaches the recipient as a harmless pointer, never
as a composer-filling paste. Do not retry an `AUTO-FILE` receipt: the body is already durable and
the pointer is the delivered turn. If the pointer itself needs more than one literal send, pfm
chunks it under the same target lock; it never rejects the body for size.

## `--file` carries the MESSAGE, not the document

The file holds the words you would have typed — an IMPERATIVE. It is not a way to paste a document into a chat. A brief, a spec, or a report is dispatched **by pointer** — `"/jc per {abs path} — {the one-line ask}"` — never by injecting its content: a chat handed a markdown document reads it as _material_, not as an _order_, and will study it instead of acting on it.

## A long /compact focus fires in full

Slash commands never become auto-file pointers. pfm splits them into literal chunks of at most
512 runes, holds one per-target lock for the complete stream, and presses Enter only after the
last chunk. A long `/compact` focus therefore reaches the harness byte-exact and fires; there is
no command-size refusal. It still requires `--then`:

`$HOME/.local/bin/pfm chat inject --then "{steer}" {target} "/compact {complete focus}"`

A durable hold file remains useful when facts must survive the summary verbatim, but it is a
memory-design choice rather than a transport workaround.

## Carry a follow-up past a /compact with `--then`

`$HOME/.local/bin/pfm chat inject --then "{steer}" {target} "/compact {focus}"` delivers `{steer}` as a second turn the moment the primary turn finishes and the pane returns to idle. It exists for `/compact`: compaction leaves the chat idle, so a steer typed while it runs is swallowed — `--then` rides out the busy→idle transition, then types the steer onto the settled pane. The waiter is **detached**, because a self-inject's waiter runs inside the very turn it must wait on — that turn cannot end until the inject returns — so it survives the turn as a background process and delivers through a fresh inject that takes its own lock. The steer follows the same signature rule as any message (a `/`-prefixed steer travels bare). It works for any primary message (it simply waits for that turn to end), but `/compact` is the case it is built for; a non-live (RESUME) target has no turn to wait on, so `--then` is ignored there with a warning. **A `/compact` inject REQUIRES `--then` (operator rule)** — pfm rejects a steerless compact: compaction ends at an idle prompt with no turn fired, stranding the target command-less.

**`--then` is repeatable** — `--then "{s1}" --then "{s2}" --then "{s3}"` chains the steers in the given order, each delivered as its own turn after the previous one settles to idle (the waiter delivers the first, then the recursive inject re-queues the remainder, one confirmed hop at a time). Use it to script a short sequence onto one target — e.g. `/compact` → confirm state → start the next phase. No steer may itself start with `/compact` (checked upfront, whatever the primary): compact-steering-into-compact recurses and loses the thread.

## Restart all MCP servers — "restart mcp"

`/mcp disable` then `/mcp enable` cycles every MCP server. Both are user-typed slash commands, not tools, so fire them on this pane as two self-injects, in order — they queue behind the current turn FIFO, so disable runs first and enable second:

```bash
$HOME/.local/bin/pfm chat inject self "/mcp disable"
$HOME/.local/bin/pfm chat inject self "/mcp enable"
```

Use two ordered injects, not `--then`: the `--then` waiter fires on the current turn's busy→idle edge and beats the still-queued `/mcp disable`, so enable lands first and the servers stay down. FIFO ordering is what guarantees disable-then-enable.

When the operator says "restart mcp", run exactly these two.

## Concurrent senders are serialized

When several chats inject into the **same** live pane at once, their keystrokes would otherwise interleave into one mangled turn. Each LIVE delivery takes a per-target lock (an atomic `mkdir` lock under `${TMPDIR}/chat-inject-locks/`, released when the inject finishes) so deliveries to one pane run one-at-a-time; the lock covers every chunk through confirmed submission. A second inject waits up to `CHAT_INJECT_LOCK_TIMEOUT` (30s) then warns rather than colliding. A lock whose owner died or has been held too long is reclaimed automatically. Before typing, pfm also exits the target's copy/scroll mode and dismisses a stuck Rewind menu — both silently swallow input.

## To steer a RUNNING chat, target its tmux session — not its UUID

The LIVE send-keys arm fires only for `self` or a **live tmux session name**. A bare session-id has no pane map, so it **falls back to the transcript (RESUME) arm** — which a _running_ target will not see until it reopens, so it cannot stop a live chat in time. To steer a running chat, pass its **tmux session name**.

- **Find a chat's tmux session:** run `/chat:ls`, or `/chat:whoami` inside the target, or read its tmux status bar.
- **Confirm from the output:** `injected LIVE into tmux session …` landed live; `RESUME …` did not reach a running target — re-target by tmux session.
- **Busy panes queue safely:** normal delivery types into the Claude or Codex composer queue without interrupting the running turn. Use `--force-now` only when the current turn must be stopped.
