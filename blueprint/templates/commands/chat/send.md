---
name: chat:send
description: Send a message to another chat — the founder names a target chat by pasting a distinctive excerpt of it; .claude/scripts/chat-find.sh resolves the excerpt to that chat's session-id across every account, then chat-mail.sh drops the message into that chat's inbox. `inbox` reads this chat's own unread. Trigger — /chat:send {message} :: {target excerpt}, or /chat:send inbox.
argument-hint: [{message} :: {target excerpt}] | [inbox]
---

# Chat Send — message another chat by text

Args: $ARGUMENTS

A chat is addressed by its text: paste a distinctive line of the target chat and the shared finder resolves it to that chat's session-id — the same matcher `/chat:read` uses. The message lands in that chat's inbox, which it reads with `/chat:send inbox` or sees surfaced when it next resumes.

**Dispatch on the first token of `$ARGUMENTS`:** `inbox` → § Read inbox. Anything else → § Send.

## Send

1. **Split message from target.** `$ARGUMENTS` is `{message} :: {target excerpt}`. If the `::` delimiter is absent, ask the founder for the two parts: the message, and a few exact distinctive lines from the target chat.
2. **Resolve the target.** Write the excerpt verbatim to `tmp/chat-loads/send-target.txt`, then:
   ```bash
   .claude/scripts/chat-find.sh tmp/chat-loads/send-target.txt
   ```
   It prints `{session-id}<TAB>{path}` plus a match report (hits, date range, other candidates).
   - **No match** → ask for a longer or more distinctive chunk.
   - **Ambiguous** → confirm the auto-pick against the printed date range before sending.
3. **Deliver** to the resolved session-id:
   ```bash
   .claude/scripts/chat-mail.sh send {session-id} "{message}"
   ```
4. **Confirm:** report the target session, the match confidence (hits + date range), and that the target reads it via `/chat:send inbox` or on its next resume.

## Read inbox

```bash
.claude/scripts/chat-mail.sh inbox
```

Prints this chat's unread messages and archives them to `.read/`. Relay each to the founder and act on it under their direction.
