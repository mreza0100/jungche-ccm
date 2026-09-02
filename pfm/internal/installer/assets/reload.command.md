---
name: reload
description: '{{RELOAD_USAGE}}'
---

# `/reload [--account N] [--1h on|off] [--fresh [--hide]] [--then "<prompt>"]` — reboot this chat in place

Run this ONCE via the Bash tool — and make it your LAST action, the chat is about to exit:

```
~/.local/bin/pfm chat reload [--account N] [--1h on|off] [--then "<prompt>"]
```

**Every setting has a flag. There are no positional arguments.** Whatever words the request
used, map them to a flag first:

| the request says | pass |
| --- | --- |
| "cache off", "5m cache", "short cache" | `--1h off` |
| "cache on", "1h cache", "long cache" | `--1h on` |
| "account 2", "switch seats", "other account" | `--account 2` |
| "then continue with X" | `--then "X"` |
| "fresh", "new conversation", "start over here" | `--fresh` |
| "fresh and hide the old one", "replace this chat", `/handoff` | `--fresh --hide` |
| nothing in particular | no flags at all |

`pfm chat reload cache off` is not a call — `cache` is not an argument, and the command will
refuse it. If a rejection ever comes back, read the flag it names rather than reshuffling the
words.

**Do not pass `--sock`.** With no `--sock`, the command finds the CALLING chat's own pane by
itself — Claude panes from tmux, Codex from the fleet-bound thread identity. `--sock` exists only
to reboot a DIFFERENT chat from outside it, which is not what this command is for.

**Typed by the human, `/reload …` never reaches the model** — the `pfm internal reload-intercept`
UserPromptSubmit hook executes it and blocks the prompt (Claude seats only; a Codex seat still
routes through this body). This body is for the model's OWN calls: limit rescue, `/handoff`, a
config change that needs a fresh session.

With no flags, the current engine account is preserved. The chat auto-exits and reboots in the
same window and pane; split siblings are untouched. With `--then`, the script waits for the reborn
chat's input box, then types and submits the prompt.

After running it, reply with ONE short line and END YOUR TURN immediately — a turn still running
when the `/exit` lands is force-killed after 20s, and in-flight sub-agents die with it.

## Cache-only reboot — `/reload --1h on|off`

For Claude, `--1h` flips the chat's prompt-cache TTL across the reboot: `on` = ⚡1h (`ENABLE_PROMPT_CACHING_1H=1`),
`off` = 5m (`FORCE_PROMPT_CACHING_5M=1` — since CC 2.1.215 the harness defaults to 1h, so 5m must
be forced, never assumed). With no `--account` the chat KEEPS its current account — `/reload --1h off`
is the pure "restart this chat on the 5m cache" move. Without `--1h`, an account reload preserves
the chat's existing cache mode (a flagless elder counts as 1h, the default it actually runs).

## Fresh conversation — `/reload --fresh [--hide]`

`--fresh` reboots into a NEW session id in the same pane, account, and cwd — the old conversation
is untouched and stays resumable from the picker. Add `--hide` and the conversation left behind is
hidden from the picker instead (a permanent kill recorded once the reboot completes — never before,
so a reload that fails leaves the live chat listed; `pfm chat unkill <id>` brings it back).
`--hide` needs `--fresh`: a reload that resumes the same conversation cannot hide it. Pairs with
`--then` for a handoff: reboot fresh, hide the chat being replaced, hand the reborn chat its
first prompt — which is exactly what `/handoff` does.

## Reloading yourself onto another account (limit rescue)

When the current account's usage limit is nearly exhausted and work remains, invoke this yourself
instead of stalling:

1. **Land in-flight work first** — sub-agents, workflows, and background tasks do NOT survive the
   reboot. Finish or checkpoint them; never reload mid-flight.
2. **Pick a different configured account for the current engine** from `pfm config show`.
3. **Reload with a handoff**:
   `~/.local/bin/pfm chat reload --account <other-n> --then "Continue: <what you were doing + the next concrete step>"`
4. One short line to the user (which account you moved to and why), end turn. The reborn you
   reads the `--then` prompt and continues on the fresh account's budget.
