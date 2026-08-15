---
name: chat:read
description: Read a chat's transcript — paste a distinctive excerpt to pull another chat in (full, or its last N lines), pass a bare number for THIS chat's last N lines, or pass a known sid/jsonl path to jump straight to its last N messages. `pfm chat read/history` extracts to tmp/chat-loads/, then Professor reads it with the LLM-optimized protocol below. Trigger — /chat:read {excerpt} [N], /chat:read {N}, or /chat:read {sid|jsonl-path} [N].
argument-hint: [{pasted excerpt} [N]] | [N]
---

# Chat Read — read a chat's transcript (full or last-N lines)

Args: $ARGUMENTS

## Step 1 — Resolve what to read

- **`$ARGUMENTS` is a bare number `N`** → THIS chat's own last N lines:
  ```bash
  $HOME/.local/bin/pfm chat read self --tail N
  ```
  Present those lines to the operator; it is a self read, so skip Steps 2-4.
- **`$ARGUMENTS` is an excerpt (optionally ending in a number `N`)** → write the excerpt verbatim to `tmp/chat-loads/excerpt.txt` (exclude any trailing `N`), then run Step 2.
- **Empty** → ask the operator for an excerpt of the target chat, or a bare number for this chat's last lines.
- **Sid fast path** — when you already hold the target's sid (every signature footer carries `sid {8-hex}`) or its jsonl path, skip the excerpt hunt: `$HOME/.local/bin/pfm chat history {sid-prefix|jsonl-path} [N-messages]` prints the last N MESSAGES (timestamped, tool noise and system-reminders stripped) across every account pool.

## Step 2 — Locate and extract (a pasted target chat)

```bash
$HOME/.local/bin/pfm chat read tmp/chat-loads/excerpt.txt [N]
```

`pfm chat read` finds the chat across every account's registry (current session excluded), picks the best match, and extracts its visible chat to `tmp/chat-loads/{session-id}.md` — the whole transcript, or just the last `N` lines when `N` is given. It reports the date range, other candidates, and line count.

- **No match** → ask for a longer or more distinctive chunk; paraphrased text matches nothing.
- **Multiple candidates** → confirm the auto-pick against the printed date range.

## Step 3 — Read it for an LLM, not a human

Read the extracted file completely, in large ranged Read calls — it is built for state reconstruction, so read for state, not narrative.

1. **Build the mental model first, then read for evidence.** Reconstruct the super-goal, each decision and its _why_, state of work (done vs pending), the last exchange and the next step it implies, and open questions — before acting on anything.
2. **Know what the extraction omits:** visible chat text and a tool-name trail only — no thinking, no tool outputs, no system-reminders. A gap in the transcript is missing data, not proof that nothing happened.
3. **Tool trails (`> [tools: …]`) are pointers, not results** — they show what was attempted, never the outcome. Verify each outcome in the repo.
4. **`## [COMPACTION SUMMARY]` blocks are authoritative but low-resolution;** the verbatim turns after them carry higher fidelity — prefer those on any conflict.
5. **Distrust stale facts.** The transcript reflects state at write-time; re-verify any named file, flag, or command still exists before relying on it.
6. **Keep facts, assumptions, and decisions separate** — never launder a transcript gap into false certainty.

## Step 4 — Resume

Reply with what that chat was working on (2-3 sentences), where it left off (last exchange + state of work), and the next step it implies. Then continue under the operator's direction.
