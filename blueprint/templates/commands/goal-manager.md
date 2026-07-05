---
name: goal-manager
description: Turn an ambition into a runnable prompt, or keep a live goal resumable. Two modes. Default — compile a fuzzy super-goal into a sharp, self-contained prompt for a fresh Professor chat (interrogate → ground in the repo → write to tmp/prompt-saves/{slug}.md). `epic {name}` — write/refresh an epic's continuation prompt at docs/epics/{name}/prompts/resume-latest.md (+ dated archive): a # Report narrative of what happened and why, then a # Prompt the founder pastes into /goal so a stopped session resumes exactly where it left off. Runs at every checkpoint. Route here to turn an ambition into a prompt, or a paused/in-flight epic into a runnable continuation — not to execute it.
argument-hint: [super goal] | epic [epic-name]
---

# Goal — compile a super-goal into an executable prompt

Super goal: $ARGUMENTS

You define the goal; a future session does the work. The deliverable is a prompt file — never the goal's implementation. If `$ARGUMENTS` starts with `epic`, run § Subcommand: epic instead of Steps 1–4. If `$ARGUMENTS` is empty, ask for the goal in one line before anything else.

## Step 1 — Interrogate

The goal as stated is never the goal. Ask targeted questions (AskUserQuestion, up to 4 per round, up to 2 rounds) until you can answer all five:

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

## Step 3 — Write the prompt

Target: `tmp/prompt-saves/{slug}.md` — `{slug}` is kebab-case from the refined goal; on collision append `-v2`, `-v3`. Create `tmp/prompt-saves/` if missing.

Prompt structure — all sections, `none` where truly empty. The Operating mode and Continuity blocks are fixed text (fill only `{goal-slug}`): embed them verbatim in every prompt.

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
At every milestone and session end: run `/documenter epic {goal-slug}` — it consolidates the session into `update.md` (`## State of work` rewritten with percent-complete + exact next step, `## Delivered` merged) and the manifest, then refreshes the continuation prompt via `/goal-manager epic {goal-slug}`.
Resume after any stop: paste `docs/epics/{goal-slug}/prompts/resume-latest.md` into `/goal` (refreshed at every checkpoint — a # Report of what happened + a # Prompt that continues from the exact next step), or `Load epic {goal-slug}` → read `manifest.md` + `update.md` → continue from `## State of work`.

## Open questions

{what the executor must confirm with the founder before irreversible moves}
```

Prompt rules: self-contained — the executor has the repo, not this chat; point at files rather than pasting file bodies; no secrets, credentials, or internal URLs.

## Step 4 — Deliver

Report the file path and one line stating what context you embedded. The prompt lives in the file only — never echo it into the chat.

## Subcommand: epic — write the continuation prompt for a paused or in-flight epic

`/goal-manager epic {name}` — the goal already exists; the epic carries it. No interrogation: this is extraction into a **continuation prompt** the founder pastes into `/goal` so a fresh chat resumes exactly where the last one stopped. It runs at every checkpoint — a session is always one stop from unrecoverable, and `resume-latest.md` is its always-current rescue.

1. **Load the epic.** Read `docs/epics/{name}/manifest.md` + `update.md` + the constitution (`goal-prompt.md`, or whichever prompt file `## Files` registers as the goal source). No `{name}` given or epic missing → `ls docs/epics/` and ask in one line.
2. **Locate the live state**, in priority order: `update.md` `## State of work` (percent complete, in-flight position, exact next step); manifest `## Key Decisions` entries that supersede or refine the constitution (overrides outrank goal-prompt text — list them explicitly); `## Open Questions`; and any program-counter or register files those sections name (read them; cite their paths, never copy their bodies).
3. **Write two files** to `docs/epics/{name}/prompts/` (create it if missing): the rolling `resume-latest.md` (overwrite every run — the one file the founder pastes) and a dated `resume-{YYYY-MM-DD}.md` (`-2`/`-3` on same-day collision — the archive). Both carry IDENTICAL content. Both stay unregistered in the manifest `## Files` and are never auto-loaded by `Load epic`.

   The file is two sections — `# Report` then `# Prompt`:

   **`# Report`** — founder-facing prose so a reader grasps the state at once: what has been done so far, the key decisions made and WHY (cite the register rows), and the current project state (percent complete, what is green and committed, what is in flight). Narrative, not a template.

   **`# Prompt`** — the self-contained resume prompt; pasting it into `/goal` must let a fresh session continue exactly where it stopped:
   - the **Operating mode** block from Step 3, verbatim;
   - `## Mission` — POINT at `docs/epics/{name}/goal-prompt.md` (never copy or fork it — two constitutions drift); state only the deltas (decisions and route overrides since it was written), each with its register/manifest citation;
   - `## Current state` — percent complete, what is done with evidence paths, the exact in-flight position, the exact next step, lifted from `## State of work`;
   - `## Resume protocol` — ordered reads for the fresh session: `Load epic {name}` → the northstar note (`mental-model/build-northstar.md`) → the program-counter/register files by path → the constitution; re-form the mental model first, then continue from the next step;
   - `## Safety guards` — the route tripwire the epic runs under (e.g. zero real data / never push / green gitter checkpoints, else stop and re-confirm), only-gitter-commits, the epic's sacred-ground constraints, and any in-flight background work to check on (workflow run ids, drains, agents) that `## State of work` names;
   - `## Open questions` — carried from the manifest; `none` if empty.

4. **Deliver.** Report the two paths and one line: what state snapshot they carry and which overrides they list. Never echo the prompt into the chat.

**Checkpoint integration:** `/documenter epic` invokes this subcommand after it consolidates `update.md` + the manifest, so `resume-latest.md` is always current. All continuation-prompt structure lives here — `/documenter epic` and the Professor's checkpoint discipline only call `/goal-manager epic {name}`; they never restate the format. Continuation prompts are point-in-time snapshots: the newest supersedes older dated ones.

State fast-moving facts (commit counts, ahead/behind, push status) by pointing at their live source (`git`, `00-STATE.md`), never as hardcoded numbers — a snapshot count is false by the next checkpoint and reads as a false alarm.
