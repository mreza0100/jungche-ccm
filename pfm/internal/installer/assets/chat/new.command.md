---
name: chat:new
description: Spawn a fresh named teammate chat on its own immutable socket. It starts detached by default; `--attach` opens it in this terminal after launch. Choose Claude or Codex with `--engine`, and drive the chat by name through /chat:inject. Trigger — /chat:new {name} [options] [prompt].
argument-hint: "{name} [--engine cc|cx] [--attach] [prompt]"
---

# Chat New — spawn a teammate chat

Args: $ARGUMENTS

- **Default** — starts detached on its own immutable `cc-*` or `cx-*` socket and returns immediately.
- **`--attach`** — starts the same way, then attaches this terminal to the new chat.
- **Name** — the first positional argument, or `--name NAME`; names resolve through `pfm chat resolve` and never alter the socket identity.

## Steps

1. **Spawn:** `$HOME/.local/bin/pfm chat new $ARGUMENTS`.
2. **Report** the engine's output verbatim: the teammate's name and immutable socket. On error, relay the line; nothing was spawned.
