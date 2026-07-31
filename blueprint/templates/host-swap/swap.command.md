---
name: swap
description: Reboot THIS chat IN PLACE — same pane, same tmux socket, same conversation — under another account (whichever account numbers you configured in zshrc-swap.snippet.sh, e.g. 1|2|3) and/or with the ⚡1h-cache flipped (--1h on|off). Env binds at birth, so a reboot is the only way to change either. Account optional when --1h is given (cache-only reboot keeps the account). Optional --then "<prompt>" auto-types a follow-up into the reborn chat so work continues unattended. Invoke it YOURSELF (with --then as a handoff) when the current account's usage limit is nearly exhausted and work remains.
allowed-tools: Bash(~/.claude/bin/cc-swap-chat.sh:*)
---

# `/swap [<n>] [--1h on|off] [--then "<prompt>"]` — reboot this chat under another account / cache mode

**This command needs an engine you write.** The blueprint ships the `/swap` *contract* below —
the mechanism a reboot-in-place script must implement — but not the script itself: it depends on
how you launch chats (tmux socket layout, per-account config dirs), which is host-specific and
not something this template can assume for you. `allowed-tools` above names
`~/.claude/bin/cc-swap-chat.sh` as the conventional drop-in location; point it at wherever you
actually place your script, and author that file yourself. Until it exists, `/swap` has nothing
to run.

## What the engine must do

Given `$ARGUMENTS` (`[<n>] [--1h on|off] [--then "<prompt>"]`), the script this command shells
out to must:

1. **Resolve the target env** — the account config dir for `<n>` (skip if no account arg — see
   § Cache-only reboot) and/or the cache-TTL env var for `--1h`.
2. **Respawn in place** — end this chat's process, then launch a fresh `claude` in the
   **SAME tmux pane and the SAME tmux socket**, under the resolved env. Split sibling panes and
   any other chat's socket are untouched; nothing disconnects the terminal, the pane just blinks
   and returns with the same conversation (`claude --resume`/`--continue` over this chat's own
   transcript, or equivalent).
3. **Optional `--then "<prompt>"` handoff** — after the respawned chat reaches its input box,
   type and submit the prompt automatically, so work continues unattended.

Once wired up, invoking `/swap` should feel like: run the engine once via the Bash tool as your
LAST action (the chat is about to exit), then reply with ONE short line and END YOUR TURN
immediately — a turn still running when the exit lands is force-killed after 20s, and in-flight
sub-agents die with it.

## Cache-only reboot — `/swap --1h on|off`

`--1h` flips the chat's prompt-cache TTL across the reboot: `on` = ⚡1h (`ENABLE_PROMPT_CACHING_1H=1`),
`off` = 5m (`FORCE_PROMPT_CACHING_5M=1` — since CC 2.1.215 the harness defaults to 1h, so 5m must
be forced, never assumed). With no account given the chat KEEPS its current account — `/swap --1h off`
is the pure "restart this chat on the 5m cache" move. Without `--1h`, an account swap preserves
the chat's existing cache mode (a flagless elder counts as 1h, the default it actually runs).

## Swapping yourself (limit rescue)

When the CURRENT account's usage limit is nearly exhausted and work remains, invoke this yourself
instead of stalling:

1. **Land in-flight work first** — sub-agents, workflows, and background tasks do NOT survive the
   reboot. Finish or checkpoint them; never swap mid-flight.
2. **Pick a DIFFERENT account**: current = `${CLAUDE_CONFIG_DIR:-~/.claude}` — `~/.claude` → 1,
   `~/.claude2` → 2, `~/.claude3` → 3 (whichever numbers you configured).
3. **Swap with a handoff**:
   `/swap <other-n> --then "Continue: <what you were doing + the next concrete step>"`
   (requires your engine at the `allowed-tools` path to be wired up — see above).
4. One short line to the user (which account you moved to and why), end turn. The reborn you
   reads the `--then` prompt and continues on the fresh account's budget.
