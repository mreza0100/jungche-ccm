---
name: goal-definer
description: Compile a fuzzy super-goal into a sharp, self-contained prompt for a fresh Professor chat in this repo — interrogate the founder until the goal is airtight, ground it in the codebase, and write the prompt to tmp/prompt-saves/{slug}.md. Route here when the founder states a big ambition and wants it turned into a prompt, not executed.
argument-hint: [super goal]
---

# Goal — compile a super-goal into an executable prompt

Super goal: $ARGUMENTS

You define the goal; a future session does the work. The deliverable is a prompt file — never the goal's implementation. If `$ARGUMENTS` is empty, ask for the goal in one line before anything else.

## Step 1 — Interrogate

The goal as stated is never the goal. Ask targeted questions (AskUserQuestion, up to 4 per round, up to 2 rounds) until you can answer all five:

- **Outcome** — what does done look like? What is different in the product or system when this succeeds?
- **Scope** — what is explicitly in, what is explicitly out?
- **Constraints** — deadlines, sacred ground (tenant isolation, permissions, audit trails, money paths), tech choices already made.
- **Evidence** — what proof will the founder accept? Tests, metrics, demo, report.
- **Endgame** — is the finish line shipped code, validated knowledge, or a decision?

Ask only questions whose answer would change the prompt; when the remaining unknowns are things the executor can discover from the repo, stop asking and move on.

## Step 2 — Ground in the repo

The executor is a fresh Professor session in this repo with no memory of this conversation. Build its head start:

- Locate the surfaces the goal touches (start at `docs/agents/_index.md` and the relevant child `CLAUDE.md`); collect the 3–8 entry-point files or docs the executor should read first.
- Check `docs/epics/` and recent waves for prior work the goal builds on or collides with.
- Identify the natural first move for the executor: a command (`/build`, `/jc`, `/wave`), a skill (`p:wave:refine`, `p:rnd`, `rr`), or direct analysis.
- Distill what the executor cannot infer from the repo: founder intent, tradeoffs already decided, context from this conversation.

## Step 3 — Write the prompt

Target: `tmp/prompt-saves/{slug}.md` — `{slug}` is kebab-case from the refined goal; on collision append `-v2`, `-v3`. Create `tmp/prompt-saves/` if missing.

Prompt structure — all sections, `none` where truly empty. The Operating mode and Continuity blocks are fixed text (fill only `{goal-slug}`): embed them verbatim in every prompt.

```markdown
# {refined goal, one line}

## Operating mode

This is a super-goal — few who attempt it succeed. Work accordingly:

1. Read this entire prompt first and build a mental model of the end state — see far ahead of what you are building before touching anything.
2. Plan in smart workflows: decompose toward the end state, but decide each next step dynamically from what the last step taught you — the plan serves the mental model, never the reverse.
3. When you hit a real dead end, look back: reconsider earlier decisions, revise the plan, and re-approach. Backtracking a wrong decision is progress; repeating it is not.

## Mission

{outcome + why; what "done" looks like, measurable}

## Context

{distilled interrogation answers and decisions already made — inlined, with no reference to "this conversation"}

## Repo orientation

{entry-point files/docs to read first; related epics or waves}

## Scope

**In:** {...}
**Out:** {...}

## Constraints

{sacred ground, tech constraints, deadlines}

## Success criteria

{the evidence the founder will accept}

## Suggested route

{the first command/skill the executor should invoke, and why}

## Continuity — Epic {goal-slug}

This goal outlives any session. At kickoff: create Epic `{goal-slug}` (`docs/epics/{goal-slug}/`), copy this prompt to `docs/epics/{goal-slug}/goal-prompt.md` (register it in the manifest's `## Files`), and fill `## Vision & Scope` (what the goal is and what "done" means).
At every milestone and session end: run `/documenter epic {goal-slug}` — it consolidates the session into `update.md` (`## State of work` rewritten with percent-complete + exact next step, `## Delivered` merged) and the manifest, so the epic always says how much of the goal is left.
Resume after any stop: `Load epic {goal-slug}` → read `manifest.md` + `update.md` → continue from the next step in `## State of work`.

## Open questions

{what the executor must confirm with the founder before irreversible moves}
```

Prompt rules: self-contained — the executor has the repo, not this chat; point at files rather than pasting file bodies; no secrets, credentials, or internal URLs.

## Step 4 — Deliver

Report the file path and one line stating what context you embedded. The prompt lives in the file only — never echo it into the chat.
