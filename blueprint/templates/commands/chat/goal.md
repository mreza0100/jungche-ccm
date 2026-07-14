---
name: chat:goal
description: Compile an ambition into a runnable /goal and FIRE it at a live chat — a teammate chat by tmux name, or THIS very chat itself via self-inject (the one way to run /goal on yourself). A goal is ALWAYS inline text under 4,000 chars — never a path to a file. Two modes. Default — read the goal from this chat (or an explicit one-liner), interrogate → ground in the repo → compile the ≤4k goal string (record copy at tmp/prompt-saves/{slug}.md) → fire at the target. `epic {name}` — write/refresh an epic's continuation prompt at docs/epics/{name}/prompts/resume-latest.md (+ dated archive): a # Report narrative, then a ≤4k # Prompt fired into /goal so a stopped session resumes exactly where it left off. /wave:orchestrator uses this to set the builder's train-wide goal. Route here to turn an ambition into a fired goal, or a paused epic into a runnable continuation.
argument-hint: [target-chat?] [super goal] | epic [epic-name]
---

# Chat Goal — compile a goal and fire it at a live chat (or at this one)

Super goal: $ARGUMENTS

You define the goal; the target session does the work. The deliverable is a **fired `/goal`** — a goal is ALWAYS inline text, **≤4,000 characters, single-line at fire time, never a path to a file** (deep context rides as repo-file pointers INSIDE the text). The target is any live chat by its tmux name (`/chat:ls`), **including the chat calling this command** — self-inject is the only way to fire `/goal` on yourself, since it is a user-typed command (§ Launch). If `$ARGUMENTS` starts with `epic`, run § Subcommand: epic instead of Steps 1–4. When `$ARGUMENTS` is empty, take what the founder has been driving at in this chat as the super-goal and refine it in Step 1; ask for a one-line goal only when the chat gives you nothing. **Non-interactive lane:** a caller that already holds the finished goal (e.g. `/wave:orchestrator` firing the builder's train-wide goal) skips Steps 1–3 and runs § Launch directly with its own ≤4k string — no interrogation, no record-copy detour.

## Step 1 — Interrogate

The goal as stated — or as read from the chat — is never the whole goal. Start from what the conversation already settles, then ask targeted questions (AskUserQuestion, up to 4 per round, up to 2 rounds) to close the gaps until you can answer all five:

- **Outcome** — what does done look like? What is different in the product or system when this succeeds?
- **Scope** — what is explicitly in, what is explicitly out?
- **Constraints** — deadlines, sacred ground (tenant isolation, permissions, audit trails, money paths), tech choices already made.
- **Evidence** — what proof will the founder accept? Tests, metrics, demo, report.
- **Endgame** — is the finish line shipped code, validated knowledge, or a decision? When a founder-gated step (paid spend, sacred ground, an irreversible action) sits on the critical path, the endgame is phased — autonomous up to the gate, then an explicit founder stop — not a "fully autonomous to done" mandate that contradicts its own gate.

Ask only questions whose answer would change the prompt; when the remaining unknowns are things the executor can discover from the repo, stop asking and move on.

## Step 2 — Ground in the repo

The executor is a fresh Professor session in this repo with no memory of this conversation. Build its head start:

- Locate the surfaces the goal touches (start at `docs/agents/_index.md` and the relevant child `CLAUDE.md`); collect the 3–8 entry-point files or docs the executor should read first.
- Check `docs/epics/` and recent waves for prior work the goal builds on or collides with.
- Identify the natural first move for the executor: a command (`/wave:builder`, `/jc`, `/wave:orchestrator`, `/wave:refine`, `/p:rnd`), a skill (`rr`), or direct analysis.
- Distill what the executor cannot infer from the repo: founder intent, tradeoffs already decided, context from this conversation.

## Step 3 — Compile the goal string (≤4,000 chars)

The fired artifact is the goal STRING: flowing prose compiled from the sections below, **≤4,000 characters, no newlines at fire time** — mission + measurable done-state, sacred-ground constraints, success evidence, the first move, and read-first repo pointers (paths the target reads for depth; the pointers ride inside the text, the string itself is never a path). Persist a readable record copy — the same content in the section layout below — to `tmp/prompt-saves/{slug}.md` (`{slug}` kebab-case from the refined goal; `-v2` on collision; create the dir if missing). The record is for humans and resumes; what fires is the string.

Section layout for the record copy — all sections, `none` where truly empty. The Operating mode and Continuity blocks are fixed text (fill only `{goal-slug}`): embed them verbatim, and compress their substance into the fired string (mental-model note first, backtrack on dead ends, banners don't end the mission, re-form the model after compacts).

```markdown
# {refined goal, one line}

## Operating mode

This is a super-goal — few who attempt it succeed. Work accordingly:

1. Read this entire prompt first, build a mental model of the end state, and write it to a short northstar note (`docs/epics/{goal-slug}/mental-model/build-northstar.md`) — see far ahead before touching anything; no build work exists until that note does.
2. Plan in smart workflows: decompose toward the end state, but decide each next step dynamically from what the last step taught you — the plan serves the mental model, never the reverse.
3. When you hit a real dead end, look back: reconsider earlier decisions, revise the plan, and re-approach. Backtracking a wrong decision is progress; repeating it is not.
4. Chain the work to done: after each green milestone continue straight to the next; surface to the founder at genuine decision gates and blockers, not to report progress. Done is the Success criteria below — not a green slice, and not a `/goal` "achieved"/"could-not-be-achieved" banner (that ends a harness instance, not the mission). If a banner fires with the criteria still unmet, continue, or resume on the next re-injection.
5. On resume and after any `/compact`, re-form the mental model from your northstar note before reading "the next step" — jumping straight to the next step is the drift that loses the end-state.

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

{the evidence the founder will accept — a working end-to-end increment plus independent or held-out proof, not self-graded per-component green}

## Suggested route

{the first command/skill the executor should invoke, and why}

## Continuity — Epic {goal-slug}

This goal outlives any session. At kickoff: create Epic `{goal-slug}` (`docs/epics/{goal-slug}/`), copy this prompt to `docs/epics/{goal-slug}/goal-prompt.md` (register it in the manifest's `## Files`), and fill `## Vision & Scope` (what the goal is and what "done" means).
At every milestone and session end: run `/documenter epic {goal-slug}` — it consolidates the session into `update.md` (`## State of work` rewritten with percent-complete + exact next step, `## Delivered` merged) and the manifest, then refreshes the continuation prompt via `/chat:goal epic {goal-slug}`.
Resume after any stop: fire the `# Prompt` of `docs/epics/{goal-slug}/prompts/resume-latest.md` into `/goal` (inline — the goal is the prompt text, never the path) (refreshed at every checkpoint — a # Report of what happened + a # Prompt that continues from the exact next step), or `Load epic {goal-slug}` → read `manifest.md` + `update.md` → continue from `## State of work`.

## Open questions

{what the executor must confirm with the founder before irreversible moves}
```

Prompt rules: self-contained — the executor has the repo, not this chat; point at files rather than pasting file bodies; no secrets, credentials, or internal URLs.

## Step 4 — Deliver

Report the compiled goal string's char count, the record-copy path, and one line stating what context you embedded. Then fire it (§ Launch) — at the named target immediately when the invocation named one; otherwise offer the founder the target choice first (a chat name, or `self`).

## Launch — fire /goal at the target

`/goal` is a user-typed command, so a chat can never run it on itself directly — `/chat:inject` typing into a pane is the one way to fire it, and it works on ANY live chat: a teammate by tmux name, or `self` for the chat running this command.

```bash
$HOME/.claude/commands/chat/chat.sh inject {target|self} "/goal {the compiled goal string}"
```

The inject carries the goal STRING inline — never a file path (a path is not a goal; the machinery does not read files). Single line, ≤4,000 chars — over the cap → compress the string (tighten prose, move depth into read-first pointers), never split into two goals. Capture-verify the fire: `Goal set:` echoed and the `◎ /goal active` statusline mark visible. For `self` the command queues into this pane and runs after the current turn. When the founder is present and no target was named, always offer before firing — they may want to read the goal before an autonomous loop starts. Live-verified semantics: an inline `/goal` registers cross-pane, survives `/compact`, and a successor `/goal` REPLACES harness state but not conversation memory — close a goal by delivering its terminal condition, never by overwrite.

## Subcommand: epic — write the continuation prompt for a paused or in-flight epic

`/chat:goal epic {name}` — the goal already exists; the epic carries it. No interrogation: this is extraction into a **continuation prompt** whose `# Prompt` fires into `/goal` so a fresh chat resumes exactly where the last one stopped. It runs at every checkpoint — a session is always one stop from unrecoverable, and `resume-latest.md` is its always-current rescue.

1. **Load the epic.** Read `docs/epics/{name}/manifest.md` + `update.md` + the constitution (`goal-prompt.md`, or whichever prompt file `## Files` registers as the goal source). No `{name}` given or epic missing → `ls docs/epics/` and ask in one line.
2. **Locate the live state**, in priority order: `update.md` `## State of work` (percent complete, in-flight position, exact next step); manifest `## Key Decisions` entries that supersede or refine the constitution (overrides outrank goal-prompt text — list them explicitly); `## Open Questions`; and any program-counter or register files those sections name (read them; cite their paths, never copy their bodies).
3. **Write two files** to `docs/epics/{name}/prompts/` (create it if missing): the rolling `resume-latest.md` (overwrite every run — the one file the founder pastes) and a dated `resume-{YYYY-MM-DD}.md` (`-2`/`-3` on same-day collision — the archive). Both carry IDENTICAL content. Both stay unregistered in the manifest `## Files` and are never auto-loaded by `Load epic`.

   The file is two sections — `# Report` then `# Prompt`:

   **`# Report`** — founder-facing prose so a reader grasps the state at once: what has been done so far, the key decisions made and WHY (cite the register rows), and the current project state (percent complete, what is green and committed, what is in flight). Narrative, not a template.

   **`# Prompt`** — the self-contained resume prompt, **≤4,000 chars** (it fires as an inline goal — depth lives in the ordered reads, never in the prompt body); firing it into `/goal` must let a fresh session continue exactly where it stopped:
   - the **Operating mode** block from Step 3, verbatim;
   - `## Mission` — POINT at `docs/epics/{name}/goal-prompt.md` (never copy or fork it — two constitutions drift); state only the deltas (decisions and route overrides since it was written), each with its register/manifest citation;
   - `## Current state` — percent complete, what is done with evidence paths, the exact in-flight position, the exact next step, lifted from `## State of work`;
   - `## Resume protocol` — ordered reads for the fresh session: `Load epic {name}` → the northstar note (`mental-model/build-northstar.md`) → the program-counter/register files by path → the constitution; re-form the mental model first, then continue from the next step;
   - `## Safety guards` — the route tripwire the epic runs under (e.g. zero real data / never push / green gitter checkpoints, else stop and re-confirm), only-gitter-commits, the epic's sacred-ground constraints, and any in-flight background work to check on (workflow run ids, drains, agents) that `## State of work` names;
   - `## Open questions` — carried from the manifest; `none` if empty.

4. **Deliver.** Report the two paths and one line: what state snapshot they carry and which overrides they list. Never echo the prompt into the chat. Then offer to launch it here (§ Launch).

**Checkpoint integration:** `/documenter epic` invokes this subcommand after it consolidates `update.md` + the manifest, so `resume-latest.md` is always current. All continuation-prompt structure lives here — `/documenter epic` and the Professor's checkpoint discipline only call `/chat:goal epic {name}`; they never restate the format. Continuation prompts are point-in-time snapshots: the newest supersedes older dated ones.

State fast-moving facts (commit counts, ahead/behind, push status) by pointing at their live source (`git`, `00-STATE.md`), never as hardcoded numbers — a snapshot count is false by the next checkpoint and reads as a false alarm.
