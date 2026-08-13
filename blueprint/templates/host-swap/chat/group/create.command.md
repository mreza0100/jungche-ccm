---
name: chat:group:create
description: Create a chat group on the group bus (append-only ledger + per-member cursors) and auto-join this chat — via $HOME/.claude/commands/chat/group.sh create. Trigger — /chat:group:create {group}.
argument-hint: "{group}"
---

# Chat Group Create

Run `$HOME/.claude/commands/chat/group.sh create {group}` and report the result — the group's bus path and this chat's membership. Group names: letters, digits, `_`, `-`.
