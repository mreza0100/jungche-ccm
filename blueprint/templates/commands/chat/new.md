---
name: chat:new
description: Spawn a fresh teammate chat. By default it opens in a side-by-side tmux pane (like /chat:branch, but a NEW empty chat instead of a fork); with --detach it runs as a headless detached session on its own socket — off-screen, not a pane. Either way it's auto-named from this chat's prefix (RR → RR_1, RR_2 …), inherits this chat's model, and is driven with /chat:inject {name}, which returns its screen as proof. Spawn teammates to do the work while you stay back and orchestrate; detached teammates are reaped by /bb. Trigger — /chat:new [--detach] [name].
argument-hint: [--detach] [name]
---

# Chat New — spawn a teammate chat

Args: $ARGUMENTS

- **Default** — shares this chat's tmux server, so it closes when this chat does.
- **`--detach`** — found by name across sockets, so you drive it without attaching; watch it live with `tmux -L <socket> attach`.

## Steps

1. **Spawn:** `$HOME/.claude/commands/chat/chat.sh new {args}` — forward `$ARGUMENTS` verbatim (`--detach` first if present, then the optional prefix override).
2. **Report** the script's output verbatim: the teammate's name, model, and — for `--detach` — its socket plus the inject/attach commands. On error (not in tmux, `claude` not on PATH) relay the line; nothing was spawned.
