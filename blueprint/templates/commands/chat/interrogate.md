---
name: chat:interrogate
description: "Interrogate a finished Claude Code session — recover its REASONING, not just its conclusion — by discovering the predecessor session and fork-resuming it to ask one direct question, read-only. Triggered by 'ask the predecessor', 'why did the previous/last agent...', 'what did that session decide', or when picking up parked work whose rationale is lost. Frame as /chat:interrogate."
argument-hint: [{session or topic} {question}]
---

# Chat Interrogate — interrogate a finished session

Talk to a finished session. Epics and memory store what was decided; the transcript still holds _why_.

## When to load

- Picking up parked or deferred work and the rationale isn't written down
- "Why did the last agent reject approach X / pick this contract / skip that test?"
- Auditing a past decision before reversing it

## Discover the session

Sessions live at `$CLAUDE_CONFIG_DIR/projects/<sanitized-cwd>/*.jsonl` (`CLAUDE_CONFIG_DIR` defaults to `~/.claude`). The sanitized cwd is `pwd | tr '/.' '--'` — the same derivation `chat.sh` itself uses.

```bash
DIR="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/projects/$(pwd | tr '/.' '--')"
ls -t "$DIR"/*.jsonl | head -10   # most-recent first; the uuid is the filename
```

Identify the right one by recency and topic. For a pipeline-specific ask, read the archived `audit-trail.json` (from `checkpoint.sh`) — its `session` field is the uuid that built that pipeline. The candidate-session listing/`ls` sweep may run on a cheap child; reading and interpreting transcripts stays at full tier.

## Hold the ask

```bash
claude --fork-session --resume <uuid> -p "<your question>"
```

`--fork-session` makes a read-only fork (the predecessor's history is not modified); `--resume` loads its full context; `-p` asks non-interactively and returns the answer. Quote the question precisely — you're asking the agent as it was, with everything it knew.

## Rules

- Read-only — chat interrogate never edits the predecessor session or any file. It retrieves reasoning; acting on it is a separate, normal task.
- One question per call; ask a follow-up with another `-p` call against the same uuid.
- If no session matches, say so — do not fabricate the predecessor's reasoning from the transcript yourself.

<example>
Founder: ask — why did the research agent drop the candidate library X?
Command: ls -t finds the relevant research session (uuid 9f3c…). →
       claude --fork-session --resume 9f3c… -p "Why did you recommend against adopting library X for the work ledger?"
       Returns: "Heavy new runtime for the existing stack; the lighter event-log approach gets 80% of the value without it."
</example>

<example>
Founder: ask the predecessor what it decided about the {feature} data shape
Command: Reads $DOCS archived audit-trail.json → session uuid a17b… →
       claude --fork-session --resume a17b… -p "What did you decide about the {feature} data shape, and why?"
</example>
