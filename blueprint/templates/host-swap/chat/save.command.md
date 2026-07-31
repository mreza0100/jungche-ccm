---
name: chat:save
description: Mechanically copy this chat from the registry to tmp/prompt-saves/{name}.md — $HOME/.claude/commands/chat/chat.sh save dumps the verbatim transcript + git env snapshot straight from disk, no model rewrite. For a model-written briefing on top, use /chat:dump. Trigger — /chat:save {name?}.
argument-hint: [name?]
---

# Chat Save — mechanical transcript copy from the registry

1. **Name the file.** Kebab-case from `$ARGUMENTS`, or the session's dominant topic if empty. On collision in `tmp/prompt-saves/`, append `-v2` (then `-v3`, …). Target: `tmp/prompt-saves/{name}.md` (gitignored, local only).
2. **Dump the transcript:**
   ```bash
   $HOME/.claude/commands/chat/chat.sh save tmp/prompt-saves/{name}.md
   ```
   It resolves this session's transcript from the registry (`$CLAUDE_CONFIG_DIR` + cwd + `$CLAUDE_CODE_SESSION_ID`) and appends the verbatim visible chat + a git env snapshot. If it errors, report the error verbatim — do not hand-write a substitute.
3. **Report:** `Saved: tmp/prompt-saves/{name}.md ({N} lines, mechanical copy). For a briefing on top, run /chat:dump.`
