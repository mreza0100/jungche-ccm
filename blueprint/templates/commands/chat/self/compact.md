---
name: chat:self:compact
description: Compact THIS chat's own context now with a focus you author — not a raw passthrough. First read the session for what's next and what's noise, write a strong /compact focus around it, then self-inject /compact (you can't trigger it on yourself directly). Takes two prompts — what to hold, and a steer that runs the moment compaction lands. Trigger — /chat:self:compact <hold> || <steer>.
argument-hint: <what to hold> || <post-compact steer>
---

# Chat Self-Compact — author a focused /compact, then steer what comes after

You cannot run the harness `/compact` on yourself — it is a user-typed command, not a tool you hold. `/chat:inject` typing into your own pane is the one way to fire it on this session, and to queue the turn that runs after it. Injecting to `self` targets this chat's own pane; each inject queues a turn that runs after the current one, in the order you inject them.

`$ARGUMENTS` carries two prompts split on `||`:

- **left — what to hold:** the seed for what the compaction must preserve.
- **right — post-compact steer:** the instruction that runs the moment compaction completes.

## Step 1 — Read the session before you compact

You are the best-placed summarizer of your own context — do the salience work `/compact` cannot:

- **Next** — the in-flight task and the exact next step: what must survive the compaction.
- **Noise** — resolved tangents, verbose tool output, dead ends, superseded plans: what the compaction should drop.

## Step 2 — Author the /compact focus

Fold the left arg together with the next step and key state you found into one single-line focus, and name the noise to ignore. This line is the difference between a compaction that keeps the thread and one that loses it — write it well.

## Step 3 — Fire the compaction, carry the steer

One inject does both. `--then` holds the steer and delivers it the moment compaction finishes and the pane settles to idle — a follow-up typed while compaction runs is swallowed, so `chat.sh` waits out the busy→idle transition for you, in a detached waiter that survives this turn ending:

```bash
.claude/commands/chat/chat.sh inject --no-sig --then "{right arg — post-compact steer}" self "/compact {authored focus}"
```

`--no-sig` keeps both clean as operational injections; each lands as a single line. With no `||` steer, drop `--then` and inject only the compaction. Report that compaction is queued with its steer to follow.
