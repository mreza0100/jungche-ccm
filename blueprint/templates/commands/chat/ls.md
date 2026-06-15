---
name: chat:ls
description: List the live chats (claude tmux panes) whose working dir is inside THIS repo — each row is a chat's tmux session name (the /chat:inject handle), idle/busy state, and a snippet of its screen to tell them apart. Trigger — /chat:ls.
argument-hint: (no arguments)
---

# Chat LS — live chats in this repo

Run `.claude/commands/chat/chat.sh ls` and report what it lists. Each row is a live chat's tmux session name — the handle for `/chat:inject … :: {session}` — with its idle/busy state and a snippet of its current screen, this chat marked. Only chats whose working dir is inside this repo are shown.
