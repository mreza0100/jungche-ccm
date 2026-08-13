---
name: chat:group:send
description: Send a message to a chat group — appends to the group ledger and nudges caught-up members (message text inline); --to {glob} targets the doorbells at matching member labels (e.g. MEM_*); long or multi-line content via --file. Trigger — /chat:group:send {group} [--to {glob}] {message} (or --file {path} [caption]).
argument-hint: "{group} [--to {glob}] {message...}"
---

# Chat Group Send

Run `$HOME/.claude/commands/chat/group.sh send {group} [--to '{glob}'] "{message}"` — quote the message and keep it one line; for long or multi-line content write a file first and pass `--file {path} [caption]`. `--to` routes attention, not visibility: only members matching the glob get doorbells, but the message stays on the group ledger and reaches everyone at their next read. Report the send confirmation, each nudge line, and the target match count when --to was used.
