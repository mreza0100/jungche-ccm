---
name: reload
description: Reboot THIS Claude or Codex chat IN PLACE — same pane, socket, conversation, and current engine account by default. Pass another configured account to switch seats; Claude also accepts --1h on|off. Optional --then "<prompt>" resumes work unattended. Invoke it yourself when registry/config changes require a fresh session or the current account is near its limit. Usage /reload [account] [--1h on|off] [--then "<prompt>"].
---

# `/reload [account] [--1h on|off] [--then "<prompt>"]` — reboot this chat in place

Run this ONCE via the Bash tool — and make it your LAST action, the chat is about to exit:

```
~/.local/bin/pfm chat reload $ARGUMENTS
```

With no arguments, the current engine account is preserved. The command resolves Claude panes
from tmux and Codex app-server turns from their fleet-bound thread identity, then the chat
auto-exits and reboots in the same window and pane. Split siblings are untouched. With `--then`,
the script waits for the reborn chat's input box, then types and submits the prompt.

After running it, reply with ONE short line and END YOUR TURN immediately — a turn still running
when the `/exit` lands is force-killed after 20s, and in-flight sub-agents die with it.

## Cache-only reboot — `/reload --1h on|off`

For Claude, `--1h` flips the chat's prompt-cache TTL across the reboot: `on` = ⚡1h (`ENABLE_PROMPT_CACHING_1H=1`),
`off` = 5m (`FORCE_PROMPT_CACHING_5M=1` — since CC 2.1.215 the harness defaults to 1h, so 5m must
be forced, never assumed). With no account given the chat KEEPS its current account — `/reload --1h off`
is the pure "restart this chat on the 5m cache" move. Without `--1h`, an account reload preserves
the chat's existing cache mode (a flagless elder counts as 1h, the default it actually runs).

## Swapping yourself (limit rescue)

When the current account's usage limit is nearly exhausted and work remains, invoke this yourself
instead of stalling:

1. **Land in-flight work first** — sub-agents, workflows, and background tasks do NOT survive the
   reboot. Finish or checkpoint them; never reload mid-flight.
2. **Pick a different configured account for the current engine** from `pfm config show`.
3. **Swap with a handoff**:
   `~/.local/bin/pfm chat reload <other-n> --then "Continue: <what you were doing + the next concrete step>"`
4. One short line to the user (which account you moved to and why), end turn. The reborn you
   reads the `--then` prompt and continues on the fresh account's budget.
