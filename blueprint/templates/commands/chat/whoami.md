---
name: chat:whoami
description: Print THIS chat's own tmux session name — its identity, the address another chat injects to — via $HOME/.claude/commands/chat/chat.sh whoami. Trigger — /chat:whoami.
argument-hint: (no arguments)
---

# Chat Whoami — this chat's own tmux handle

Run `$HOME/.claude/commands/chat/chat.sh whoami` and report this chat's tmux session name. If it errors (this chat is not inside tmux), say so plainly.
