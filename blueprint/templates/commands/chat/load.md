---
name: chat:load
description: Resume an earlier chat in this session — the founder pastes any distinctive chunk of it, .claude/scripts/chat-load.sh finds the source transcript in the session registry and extracts its full visible conversation to tmp/chat-loads/{session-id}.md, then Professor reads all of it and picks up where that chat left off. Trigger — /chat:load {pasted excerpt}.
argument-hint: [pasted excerpt from the old chat]
---

# Chat Load — resume an earlier session from a pasted excerpt

Excerpt: $ARGUMENTS

## Step 1 — Capture the excerpt

If `$ARGUMENTS` is empty, ask the founder to paste a chunk of the old chat — a few exact, distinctive lines beat a long generic block. Write the excerpt verbatim (zero edits) to `tmp/chat-loads/excerpt.txt`, overwriting any previous one.

## Step 2 — Locate and extract

```bash
.claude/scripts/chat-load.sh tmp/chat-loads/excerpt.txt
```

The script greps every past session in the registry for the excerpt (the current session is excluded), picks the best match, and extracts its full visible chat to `tmp/chat-loads/{session-id}.md`, reporting the date range, other candidates, and line count.

- **No match** → ask the founder for a longer or more distinctive chunk; paraphrased text matches nothing.
- **Multiple candidates** → confirm the auto-pick against the printed date range; if it is the wrong chat, extract the right candidate directly: `jq -rf .claude/scripts/transcript-extract.jq {candidate}.jsonl > tmp/chat-loads/{session-id}.md`.

## Step 3 — Load it all

Read the extracted file COMPLETELY — multiple Read calls when it exceeds one. The dump carries visible chat text and tool-name trails only; thinking and tool outputs are absent, so re-derive missing details from the repo rather than inventing them.

## Step 4 — Resume

Reply with: what that chat was working on (2-3 sentences), where it left off (the last exchange and the state of work), and the next step it implies. Then continue under the founder's direction.
