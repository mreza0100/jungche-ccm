---
name: chat:find
description: Identify a chat by its text — the founder pastes a distinctive excerpt and .claude/scripts/chat-find.sh resolves it to a session across every account, reporting the session-id, transcript path, date range, and other candidates. The bare lookup primitive behind chat:read / chat:send / chat:inject. Trigger — /chat:find {pasted excerpt}.
argument-hint: [pasted excerpt from the target chat]
---

# Chat Find — identify a chat from a pasted excerpt

Excerpt: $ARGUMENTS

The shared finder behind the whole `chat:` family, surfaced on its own: it tells you _which_ chat an excerpt belongs to, without reading, messaging, or injecting it.

## Steps

1. **Capture the excerpt.** If `$ARGUMENTS` is empty, ask the founder to paste a few exact, distinctive lines from the target chat. Write them verbatim to `tmp/chat-loads/find.txt`.
2. **Resolve:**
   ```bash
   .claude/scripts/chat-find.sh tmp/chat-loads/find.txt
   ```
   It greps every account's registry (current session excluded), prints `{session-id}<TAB>{transcript-path}` on stdout, and a report — match confidence, date range, other candidates — on stderr.
   - **No match** → ask for a longer or more distinctive chunk.
3. **Report** the matched session-id, its date range, and the transcript path. Offer the next move: `/chat:read` to pull it in, `/chat:send` to message it, `/chat:inject` to write a turn into it.
