---
name: reload
description: Reboot THIS chat IN PLACE — same pane, same tmux socket, same conversation — under another Claude account (1|2) and/or with the ⚡1h-cache flipped (--1h on|off). Env binds at birth, so a reboot is the only way to change either. Account optional when --1h is given (cache-only reboot keeps the account). Optional --then "<prompt>" auto-types a follow-up into the reborn chat so work continues unattended. Invoke it YOURSELF (with --then as a handoff) when the current account's usage limit is nearly exhausted and work remains. Usage /reload <1|2|--1h on|off> [--then "<prompt>"].
---

# `/reload [<1|2>] [--1h on|off] [--then "<prompt>"]` — reboot this chat under another account / cache mode

Run this ONCE via the Bash tool — and make it your LAST action, the chat is about to exit:

```
~/.local/bin/pfm chat reload $ARGUMENTS
```

~1.5s later this chat auto-`/exit`s and **reboots IN PLACE — same window, same pane, same tmux
socket — under the target env** (medal badge + theme switch accordingly). Fully seamless in
every terminal: nothing disconnects, the pane just blinks and returns with the same conversation.
Split siblings untouched. With `--then`, the script waits for the reborn chat to reach its input
box, then types and submits the prompt — the reborn you has the full conversation, so a short
directive is enough.

After running it, reply with ONE short line and END YOUR TURN immediately — a turn still running
when the `/exit` lands is force-killed after 20s, and in-flight sub-agents die with it.

## Cache-only reboot — `/reload --1h on|off`

`--1h` flips the chat's prompt-cache TTL across the reboot: `on` = ⚡1h (`ENABLE_PROMPT_CACHING_1H=1`),
`off` = 5m (`FORCE_PROMPT_CACHING_5M=1` — since CC 2.1.215 the harness defaults to 1h, so 5m must
be forced, never assumed). With no account given the chat KEEPS its current account — `/reload --1h off`
is the pure "restart this chat on the 5m cache" move. Without `--1h`, an account reload preserves
the chat's existing cache mode (a flagless elder counts as 1h, the default it actually runs).

## Swapping yourself (limit rescue)

When the CURRENT account's usage limit is nearly exhausted and work remains, invoke this yourself
instead of stalling:

1. **Land in-flight work first** — sub-agents, workflows, and background tasks do NOT survive the
   reboot. Finish or checkpoint them; never reload mid-flight.
2. **Pick a DIFFERENT account**: current = `${CLAUDE_CONFIG_DIR:-~/.claude}` — `~/.claude` → 1,
   `~/.cc/2` (legacy `~/.claude2`/`~/.claude3`) → 2.
3. **Swap with a handoff**:
   `~/.local/bin/pfm chat reload <other-n> --then "Continue: <what you were doing + the next concrete step>"`
4. One short line to the user (which account you moved to and why), end turn. The reborn you
   reads the `--then` prompt and continues on the fresh account's budget.
