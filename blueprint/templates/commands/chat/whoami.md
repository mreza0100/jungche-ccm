---
name: chat:whoami
description: Print THIS chat's own tmux session name — its identity, the address another chat injects to — via .claude/commands/chat/chat.sh who-i-am. Trigger — /chat:whoami.
argument-hint: (no arguments)
---

# Chat Whoami — this chat's own tmux handle

Run `.claude/commands/chat/chat.sh who-i-am` and report this chat's tmux session name — the handle another chat targets with `/chat:inject … :: {that session}`. If it errors (this chat is not inside tmux), say so plainly.
