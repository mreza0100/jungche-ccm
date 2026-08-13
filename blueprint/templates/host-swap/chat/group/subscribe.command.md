---
name: chat:group:subscribe
description: Join a chat group — this chat by default, or an explicit tmux-session/🔖 label; group messages then arrive automatically at turn starts. Trigger — /chat:group:subscribe {group} [label].
argument-hint: "{group} [label]"
---

# Chat Group Subscribe

Run `$HOME/.claude/commands/chat/group.sh subscribe {group} [label]` (label defaults to this chat's tmux session name). Membership starts at the current ledger position — only future messages arrive. Report the join confirmation.
