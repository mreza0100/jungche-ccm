---
name: chat:read
description: Read an earlier chat into this one — the founder pastes a distinctive excerpt, .claude/scripts/chat-read.sh finds that chat across every account's registry (via the shared chat-find.sh) and extracts its full visible conversation to tmp/chat-loads/{session-id}.md, then Professor reads it with the LLM-optimized protocol below and resumes. Trigger — /chat:read {pasted excerpt}.
argument-hint: [pasted excerpt from the target chat]
---

# Chat Read — resume an earlier chat from a pasted excerpt

Excerpt: $ARGUMENTS

## Step 1 — Capture the excerpt

If `$ARGUMENTS` is empty, ask the founder to paste a chunk of the target chat — a few exact, distinctive lines beat a long generic block. Write it verbatim (zero edits) to `tmp/chat-loads/excerpt.txt`, overwriting any previous one.

## Step 2 — Locate and extract

```bash
.claude/scripts/chat-read.sh tmp/chat-loads/excerpt.txt
```

`chat-read.sh` calls the shared finder `chat-find.sh`, which greps every account's registry for the excerpt (current session excluded), picks the best match, and extracts its full visible chat to `tmp/chat-loads/{session-id}.md` — reporting the date range, other candidates, and line count.

- **No match** → ask for a longer or more distinctive chunk; paraphrased text matches nothing.
- **Multiple candidates** → confirm the auto-pick against the printed date range; if it is the wrong chat, extract the right candidate directly: `jq -rf .claude/scripts/transcript-extract.jq {candidate}.jsonl > tmp/chat-loads/{session-id}.md`.

## Step 3 — Read it for an LLM, not a human

Read the extracted file completely, in large ranged Read calls — it is built for state reconstruction, so read for state, not narrative.

1. **Build the mental model first, then read for evidence.** Reconstruct the super-goal, each decision and its _why_, state of work (done vs pending), the last exchange and the next step it implies, and open questions — before acting on anything.
2. **Know what the dump omits:** visible chat text and a tool-name trail only — no thinking, no tool outputs, no system-reminders. A gap in the transcript is missing data, not proof that nothing happened.
3. **Tool trails (`> [tools: …]`) are pointers, not results** — they show what was attempted, never the outcome. Verify each outcome in the repo.
4. **`## [COMPACTION SUMMARY]` blocks are authoritative but low-resolution;** the verbatim turns after them carry higher fidelity — prefer those on any conflict.
5. **Distrust stale facts.** The transcript reflects state at write-time; re-verify any named file, flag, or command still exists before relying on it.
6. **Keep facts, assumptions, and decisions separate** — never launder a transcript gap into false certainty.

## Step 4 — Resume

Reply with what that chat was working on (2-3 sentences), where it left off (last exchange + state of work), and the next step it implies. Then continue under the founder's direction.
