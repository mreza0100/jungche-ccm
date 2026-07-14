---
name: chat:load
description: Force-load a directory or file set into context — chat.sh load enumerates every text file, then you read ALL of them in full (no skim, no sampling). Writes nothing. Trigger — /chat:load {dir-or-files}.
argument-hint: [directory or file paths]
---

# Chat Load — force every file in a set into context

Args: $ARGUMENTS

## Steps

1. **Enumerate the authoritative set:**
   ```bash
   $HOME/.claude/commands/chat/chat.sh load $ARGUMENTS
   ```
   It lists EVERY text file (line counts + total). That is the full set — no sampling.
2. **Read every one in full.** Read each listed file with the Read tool, in ranged calls for the large ones. Actually read them all — do not stop after a few. Write nothing.
3. **Report** the mental model you now hold: what the set is, the key facts per area, and how the pieces connect.
