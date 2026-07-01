---
name: bb
description: Bye-bye — hide this chat from cc-ls and fully close it (runs cc-hide.sh --exit; the chat auto-/exits, then its own tmux server is killed). Also reaps any detached /chat:new --detach teammates this chat spawned.
disable-model-invocation: true
---

# `/bb` — bye-bye: hide this chat from `cc-ls`, then `/exit`

Run this once via the Bash tool and report its output in one short line:

```
bash ~/.claude/bin/cc-hide.sh --exit
```

It adds the **current** chat's transcript to `cc-ls`'s hide list (`~/.claude/.cc-ls-hidden`), then
gracefully closes the chat by typing `/exit` into its tmux pane ~1.5s later, and finally `kill-server`s
the chat's own tmux so no idle pane is left behind. Each chat has its own tmux socket, so this only kills
this chat — never a sibling. Any detached teammates this chat spawned with `/chat:new --detach` are reaped
too (their `cc-new-*` servers killed), so none are left running headless. Nothing is deleted — the
conversation is kept (just hidden, part of history).
Keep your reply to one line (the chat is about to exit). To bring it back later: `cc-ls --hidden` then
`⌃X` (or `cc-ls -a`).
