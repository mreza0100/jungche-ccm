---
name: chat:whoami
description: Print THIS chat's own tmux session name — its identity, the address another chat injects to — via `pfm whoami`; `--label` prints its 🔖 label instead (session-name fallback) — the group-bus identity. Trigger — /chat:whoami [--label].
argument-hint: "[--label]"
---

# Chat Whoami — this chat's own tmux handle

Run `pfm whoami $ARGUMENTS` and report this chat's tmux session name; with `--label`, its 🔖 label (the group-bus identity, falling back to the session name when unlabeled). If it errors (this chat is not inside tmux), say so plainly.
