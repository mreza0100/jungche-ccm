---
name: chat
description: Message the tmux-based agent chats in this repo through `pfm chat` — inject a turn into a teammate's pane, find your own address, and read delivery receipts. Use when a protocol says "ping", "inject", or "reply to {session}".
---

<!--
HAND-WRITTEN CARD — codex-mirror.sh never overwrites this file (its directory
name is listed in the script's HANDWRITTEN list). Note that this card has NO
generated counterpart: `chat` is a HOST-level command family (`~/.claude/commands/chat/`,
not this repo's `.claude/commands/`), so the mirror — which only scans
`.claude/commands/**` — never sees it and generates nothing for it, not even a
per-subcommand pointer. This card fills that gap with the one page a lane
actually needs mid-wave.
-->

# pfm chat essentials (Codex shape)

Canonical tool: `$HOME/.local/bin/pfm chat`.

## Send (inject)

`$HOME/.local/bin/pfm chat inject {target} '{one-line message}'`

- `{target}` = exact tmux session name or a pane label; resolution scans every socket. Builder lanes get the orchestrator's address from the BRIEF / `$WAVES/lanes.md`.
- ONE line per message — send N injects for N lines. Plain text is auto-signed with your reply address; a `/`-prefixed harness command travels unsigned.
- Success prints `injected LIVE … Enter confirmed` plus a delivery-proof screen capture — read it; that is your receipt. Queued-on-busy is normal (a busy pane queues your turn).
- exit 3 = the target chat is dead.
- exit 4 = no matching chat; exit 6 = the message was not delivered.

## Who am I

`$HOME/.local/bin/pfm whoami` → your own address; include it in pings so verdicts route back.

## Builder ping discipline (both channels, every time)

1. `$HOME/.local/bin/pfm chat inject {orchestrator} '{ping}'` — the fast path. Under the default
   workspace-write sandbox this FAILS (unix-socket connects are kernel-blocked);
   that is expected, and it is exactly why step 2 is not optional.
2. Append the one-line event to `tmp/wave-sensor/events.log` — the guaranteed
   wake (the orchestrator's waiter polls ~10s). An inject can fail; the spool
   cannot — never skip it.

## Boundaries

- `--then` exists solely to queue a steer behind a `/compact` you were told to send; every `/compact` inject REQUIRES it.
- Never capture or scrape another pane to infer state — ping and ask; the orchestrator rules from reports.
