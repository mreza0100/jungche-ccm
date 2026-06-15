---
name: chat:load
description: Force-load a directory or file set into context — chat.sh load enumerates every file, then you must read ALL of them and rewrite each into a mental-model doc, gated by chat.sh load-check (every file referenced + substantive) so nothing is skipped. Trigger — /chat:load {dir-or-files}.
argument-hint: [directory or file paths]
---

# Chat Load — force-load a file set into a verified mental model

Args: $ARGUMENTS

Load the target into context for real — not a skim. The check at the end proves you read everything; you are done only when it prints `COMPLETE`.

## Steps

1. **Enumerate the authoritative set:**
   ```bash
   .claude/commands/chat/chat.sh load $ARGUMENTS
   ```
   It lists EVERY text file (line counts + total). This is the full set you must cover — no sampling.
2. **Read all of them.** Read every file in the manifest, in ranged Read calls. Actually read them — do not announce victory after a few.
3. **Rewrite into a mental-model doc.** Write `tmp/chat-loads/loaded-{slug}.md`: a `## {full-file-path}` section per file restating its purpose, key facts, and how it connects to the rest — substantive, in your own words (the rewrite is the proof you read it). End with a `## Synthesis` section holding the overall mental model.
4. **Verify — do not skip:**
   ```bash
   .claude/commands/chat/chat.sh load-check tmp/chat-loads/loaded-{slug}.md $ARGUMENTS
   ```
   - `INCOMPLETE` → it lists the files you missed, or flags a too-thin doc. Read/expand those, then re-run. Loop until `COMPLETE`.
   - `COMPLETE` → every file is genuinely loaded.
5. **Report** the synthesized mental model and confirm `COMPLETE`.
