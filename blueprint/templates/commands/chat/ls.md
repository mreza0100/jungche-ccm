---
name: chat:ls
description: List live chats (claude tmux panes) — by default those whose working dir is inside THIS repo; `--all` lists every live chat on the box with its dir. Each row is a chat's tmux session name (the /chat:inject handle), idle/busy state, and a snippet of its screen to tell them apart. Trigger — /chat:ls [--all].
argument-hint: [--all]
---

# Chat LS — live chats

Run `$HOME/.claude/commands/chat/chat.sh ls` (pass `--all` through when the user asked for chats across all dirs/projects) and report what it lists, noting which row is this chat. The repo-scoped view ends with a `(+N live in other dirs …)` hint when chats exist elsewhere — mention it so the user knows `--all` reaches them. Any listed session name is directly addressable from here: `/chat:inject <session> <message>` works across projects.
