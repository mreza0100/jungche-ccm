---
name: bb
description: Bye-bye — hide this chat from pfm and fully close it (`pfm chat hide self --exit`; the chat auto-/exits, then its own tmux server is killed). Also reaps any detached teammate chats this chat spawned.
disable-model-invocation: true
---

# `/bb` — bye-bye: hide this chat from `cc-ls`, then `/exit`

Run this once via the shell tool and report its output in one short line — the same line in
either harness, because the engine identifies which chat is calling it:

```
~/.local/bin/pfm chat hide self --exit
```

It adds the **current** chat's transcript to `pfm`'s hide list, then
gracefully closes the chat by typing `/exit` into its tmux pane ~1.5s later, and finally `kill-server`s
the chat's own tmux so no idle pane is left behind. Each chat has its own tmux socket, so this only kills
this chat — never a sibling. Any teammate chats this chat spawned with `/chat:new` are reaped
too, so none are left running. Nothing is deleted — the
conversation is kept (just hidden, part of history).
Keep your reply to one line (the chat is about to exit). To bring it back later, run `pfm ls --hidden`.
