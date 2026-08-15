---
name: bb
description: Bye-bye — hide THIS Codex chat from the cc-ls picker and fully close it. Explicit-only — run when the operator says bb, /bb, $bb, or bye-bye. Never trigger on your own.
---

Run exactly this one shell command, then STOP — no summary, no explanation, no extra
commentary (the chat is about to close):

```
~/.local/bin/pfm chat hide self --exit
```

It adds THIS Codex chat's rollout to the fleet's hide list, then gracefully closes the
chat — types `/quit` into this pane ~1.5s later and kills this chat's own `cx-*` tmux
server, so no idle pane is left behind. Each Codex chat has its own tmux socket, so this
only closes THIS chat, never a sibling. Nothing is deleted — the conversation is kept
(just hidden, part of history). To bring it back later, run `pfm ls --hidden`.
Keep any reply to one line.
