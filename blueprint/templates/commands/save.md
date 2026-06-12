---
name: save
description: Dump the session's complete working context into tmp/prompt-saves/{name}.md as a self-contained continuation briefing. Invoked mid-chat before /compact (which may lose detail) or when handing work to a fresh chat. Epic saves are handled by /documenter epic, not here. Trigger — /save {description}.
---

# Save — Full Context Dump for Continuation

Save everything this session knows about: $ARGUMENTS

You are about to be compacted or replaced. The file you write now is the ONLY thing the next session gets — anything you leave out is lost. **Completeness beats brevity everywhere in this command.** When in doubt, include it.

**Dispatch:** if `$ARGUMENTS` starts with `epic`, invoke `Skill("documenter")` with args `epic {remainder of $ARGUMENTS}` and stop — the epic subcommand lives in `/documenter epic`. Otherwise run Steps 1-4 below.

## Step 1 — Name and create the file

Derive `{name}` as kebab-case from `$ARGUMENTS` (e.g., "blueprint release flow" → `blueprint-release-flow`). If `$ARGUMENTS` is empty, derive it from the session's dominant topic.

```bash
mkdir -p tmp/prompt-saves
ls tmp/prompt-saves/ 2>/dev/null | grep -x "{name}.md"
```

On collision, append `-v2` (then `-v3`, …). Target: `tmp/prompt-saves/{name}.md` — gitignored, local only.

## Step 2 — Write the dump

Walk the ENTIRE conversation from the very first message — not just recent turns, and not your summary memory of it. Write these sections, all of them, skipping none (write "none" where truly empty):

```markdown
# Continuation: {name}

**Saved:** {YYYY-MM-DD HH:MM} · **Branch:** {git branch} · **Topic:** {$ARGUMENTS}

## 1. Mission

The founder's full objective across the whole conversation — what they are ultimately trying to achieve, not just the last request. Include the why when it was stated.

## 2. Founder instructions (verbatim, in order)

Every instruction, correction, preference, and approval the founder gave this session — quoted verbatim, numbered, in chronological order. Mid-run additions and scope changes included. These are the contract; reproduce them exactly.

## 3. State of work

- **Done:** each completed item WITH evidence — file paths, commit SHAs, test results, command outputs.
- **In flight:** exactly where work stopped, mid-step, with enough precision to resume without re-deriving (current step number of any protocol, which agent/tool was running, what it returned).
- **Not started:** queued items, in intended order.

## 4. Files touched

Every file created, edited, deleted, or moved — full path, what changed, why. Where the change is subtle or partial, include the exact before/after snippet or diff. Include files OTHER sessions/agents changed if this session depends on them.

## 5. Decisions and rationale

Every decision made — what was chosen, why, and what was rejected (with the reason it lost). Include decisions the founder made via questions/answers.

## 6. Discoveries and gotchas

Everything learned the hard way: bugs found, quirks, failed attempts and why they failed, surprising file/system states, things that contradict the docs. This is the section a fresh session can least re-derive — be exhaustive.

## 7. Environment

Branch, uncommitted files (`git status` snapshot), worktrees, ports, running processes, env files involved, external state (remote repos, clones, their SHAs/versions).

## 8. Open questions

Anything awaiting the founder's answer, with the context needed to decide.

## 9. References

Exact paths, symbols, commands, doc clusters, URLs, session/agent IDs relevant to continuing.

## 10. Last 5 rounds (verbatim)

The EXACT chat text of the last 5 user↔assistant rounds, oldest first — the founder's message in full and your visible reply in full, quoted verbatim with zero paraphrasing or trimming. Between the two, note in brackets which tools/agents ran (one line, e.g. `[ran: Edit save.md, spawned gitter → 50df70f]`) — tool output itself is not chat text. Fewer than 5 rounds exist → include them all.

## 11. Next steps (precise, ordered — the footer, ALWAYS the last section)

The exact remaining work, ordered, each step with enough detail to execute directly: file paths, commands, agent briefs, expected outcomes. Reference protocol step numbers where a command/skill is mid-flow. This section closes the file — the next session reads it last and acts on it first.
```

## Step 3 — Completeness pass (mandatory)

Re-scan the conversation from the top one more time and ask of every exchange: "would the next session need this, and is it in the file?" Anything missing → add it now. The bar: **a fresh Claude with ZERO context, given only this file, continues seamlessly** — no re-reading of the old chat, no re-asking the founder, no re-discovering gotchas.

## Step 4 — Report

```
Saved: tmp/prompt-saves/{name}.md ({N} lines)
Continue in a new chat with:
  Read tmp/prompt-saves/{name}.md and continue the work it describes.
```
