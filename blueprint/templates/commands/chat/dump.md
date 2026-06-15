---
name: chat:dump
description: Dump the session for continuation into tmp/prompt-saves/{name}.md — a model-written briefing header (mission, state of work, hidden context, open questions, next steps), then .claude/commands/chat/chat.sh save appends the verbatim chat + env snapshot from the registry. For a raw mechanical copy without the briefing, use /chat:save. Subcommand `epic` routes the briefing into the active epic. Trigger — /chat:dump {description} or /chat:dump epic {epic-name?}.
---

# Dump — Briefing Header + Script-Dumped Transcript

Save this session's work on: $ARGUMENTS

The file you produce is the ONLY thing the next session gets. The verbatim chat is captured by script in Step 3 — spend your tokens only on what the transcript cannot show.

**Dispatch:** if `$ARGUMENTS` starts with `epic`, run § Subcommand: `epic` (same dump, different destination). Otherwise run Steps 1-4.

## Step 1 — Name the file

Derive `{name}` as kebab-case from `$ARGUMENTS` (e.g., "blueprint release flow" → `blueprint-release-flow`). If `$ARGUMENTS` is empty, derive it from the session's dominant topic. On collision in `tmp/prompt-saves/`, append `-v2` (then `-v3`, …). Target: `tmp/prompt-saves/{name}.md` — gitignored, local only.

## Step 2 — Write the briefing header

Write these sections to the target file — all of them, `none` where truly empty. Founder messages and your visible replies arrive verbatim via Step 3; the header carries ONLY what the transcript cannot:

```markdown
# Continuation: {name}

**Saved:** {YYYY-MM-DD HH:MM} · **Branch:** {git branch} · **Topic:** {$ARGUMENTS}

## 1. Mission

The founder's full objective across the whole conversation — what they are ultimately trying to achieve, and the why when stated.

## 2. State of work

- **Done:** each completed item with evidence — file paths, commit SHAs, test results.
- **In flight:** exactly where work stopped, mid-step — protocol step number, which agent/tool was running, what it returned.
- **Not started:** queued items, in intended order.

## 3. Hidden context

Everything the next session needs that never appeared in visible chat text: tool-result discoveries, failed attempts and why they failed, decisions made in reasoning, surprising file/system states, contradictions with docs. The transcript cannot carry these — be exhaustive here.

## 4. Open questions

Anything awaiting the founder's answer, with the context needed to decide.

## 5. Next steps (precise, ordered)

The exact remaining work, each step executable directly: file paths, commands, agent briefs, expected outcomes. Reference protocol step numbers where a command/skill is mid-flow. The next session reads the transcript for color but acts on this list.
```

Before moving on, re-scan the conversation once from the top for Hidden-context misses — the bar: a fresh Claude given only this file (header + transcript) continues seamlessly.

## Step 3 — Append the transcript (script)

```bash
.claude/commands/chat/chat.sh save tmp/prompt-saves/{name}.md
```

The script appends the verbatim chat (founder + assistant visible text, tool-name trail) plus a git environment snapshot, resolved from `$CLAUDE_CONFIG_DIR` and `$CLAUDE_CODE_SESSION_ID`. If it errors, write the verbatim founder instructions and last 5 rounds into the header by hand instead — never save a file that silently lacks the chat record.

## Step 4 — Report

```
Saved: tmp/prompt-saves/{name}.md ({N} lines — briefing header + script-dumped transcript)
Continue in a new chat with:
  Read tmp/prompt-saves/{name}.md and continue the work it describes.
```

## Subcommand: `epic` — dump into the epic instead

`/chat:dump epic {epic-name?}` routes the same dump into the epic's persistent context (`docs/epics/{name}/`) so a fresh chat continues via "Load epic {name}" instead of a tmp file.

1. **Resolve the epic:** explicit `{epic-name}` if given; otherwise the `docs/epics/*/manifest.md` with `status: IN_PROGRESS` whose scope matches the session's work. No unambiguous match → list candidates and ask the founder.
2. **Write the dump** — header (Step 2) to `docs/epics/{name}/save-{topic}.md` (`{topic}` kebab-cased from `$ARGUMENTS` after the `epic` token, or from the session's dominant topic), then run the script against that path. Epic dumps are committed to the repo — skim the appended transcript for pasted credentials or secrets and redact before finishing.
3. **Update `manifest.md`:** append a dated session summary under `## Progress Log`; fold new decisions into `## Key Decisions` (deduped) and new gotchas into `## Discoveries`; add unanswered items to `## Open Questions`; bump `updated:`.
4. **Report:**

```
Saved into epic {name}: docs/epics/{name}/save-{topic}.md + manifest updated.
Continue in a new chat with:
  Load epic {name}
```
