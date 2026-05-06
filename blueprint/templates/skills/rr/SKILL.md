---
name: rr
description: Reza's Research-and-Report protocol. Research can target the **internet, the local codebase, or both** — RR detects this from the topic and tells the agent which sources to use. Two modes — RR (build an RRP, spawn a research agent to execute the dynamic multi-batch pipeline, deliver the agent's report) and RRP (write a self-contained prompt for the user to run in another chat). Triggered when the user says "RR", "research and report", "RRP", "RR-prompt", "research <topic>", "look into <topic>", or "find out <topic>". Use this skill INSTEAD of jumping straight to web search OR straight to grep — RR is a structured pipeline executed by a delegated agent, not a single query.
---

# RR — Research & Report

> Reza's research protocol. The trigger is `RR` — short for "research and report".

When the user says "RR <topic>", they don't want a single search and a one-paragraph answer. They want a **dynamic research pipeline** that builds knowledge in batches, where each batch is shaped by what the previous batch found, and that finishes with a **report and a plan** — not raw findings.

**Research surface — pick before spawning:**

| Surface | Tools the agent uses | When to pick |
|---------|---------------------|--------------|
| **internet** | WebSearch, WebFetch, context7 | External topics — libraries, regulations, market, competitive landscape, "what does X look like in 2026" |
| **codebase** | Read, Grep, Glob, Bash | Internal topics — "how is auth wired", "where do we use SQS", "find every place that touches PHI", "audit our consent flow" |
| **both** | All of the above | Mixed — "RR best-practice X and how we currently do it", "compare our impl to GraphQL Yoga's recommended pattern" |

If the user doesn't specify, infer from the topic. If genuinely ambiguous, ask one short question ("internet, codebase, or both?") rather than guess. Tell the spawned agent explicitly which surface to use.

There are two modes:

| Mode    | Trigger                                                                             | Output                                                                                                       |
| ------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| **RR**  | "RR <topic>", "research and report on X", "research X", "look into X", "find out X" | Build an RRP, spawn a research agent to execute it, then deliver the agent's report back to the user        |
| **RRP** | "RRP <topic>", "RR-prompt for X", "write me an RR prompt for Y"                     | Write a self-contained prompt the user runs in another chat; do NOT execute and do NOT spawn an agent       |

---

## Mode 1 — RR (delegate to an agent)

**Do not execute the pipeline in the main conversation.** Research generates a lot of tool noise (WebSearch results, fetched pages, context7 dumps) that you don't want filling the main thread. Instead: build an RRP-style self-contained prompt and hand it to a spawned agent. The agent runs the pipeline, returns a clean report, and you deliver that report to the user.

### Step 1 — Refine the goal (in reasoning, not output)

Before doing anything, work out in your reasoning what the user **actually** wants from this research. The topic as stated is rarely the goal:

- "RR vector DBs" → probably means "which vector DB should we use for {PROJECT_NAME} given our stack and scale" — not a generic survey.
- "RR EU AI Act timeline" → probably means "what do we have to do and by when, for our specific risk class" — not a Wikipedia summary.

Restate the refined goal to yourself. If the refinement materially changes the scope and you're not sure, ask the user one short clarifying question before spawning the agent. Otherwise proceed.

### Step 2 — Determine the storage path

Every RR run produces a research file. **The path depends on which command invoked RR.** Pick from this table based on the active command in the conversation:

| Caller context | Storage directory |
| -------------- | ----------------- |
| `/mentor`      | `docs/commands/mentor/research/` |
| `/officer`     | `docs/commands/officer/research/` |
| `/pm`          | `docs/commands/pm/research/` |
| `/marketer`    | `docs/commands/marketer/research/` |
| `/professor`   | `docs/commands/professor/research/` |
| `/council`     | `docs/commands/council/research/{debateName}/` (if inside a debate) |
| `/{DOMAIN_COMMAND}`         | `docs/commands/{DOMAIN_COMMAND}/research/` |
| Other / no command active | `docs/dev/research/` |

**Filename convention:** `{topic-slug}-{YYYY-MM-DD}.md` (e.g., `vector-db-comparison-2026-04-30.md`). Use today's date from the environment context.

**How to detect the caller:** Look at the active slash-command context. If the user invoked `/mentor` and inside that flow asked for an RR, the caller is `/mentor`. If RR was triggered standalone (no command running, just "RR <topic>" at the top level), use `docs/dev/research/`. When in doubt, ask the user one short question: "Which research bucket — mentor / officer / pm / dev?"

If the directory doesn't exist, the spawned agent will create it.

### Step 3 — Build the **scout** RRP and spawn a single agent

Construct a self-contained prompt following the **Mode 2 — RRP** rules below (refined goal at the top, embedded protocol, embedded relevant context, time horizon, deliverable shape). The agent will not see this conversation — inline everything that matters.

This first run is the **scout batch**. Its job is to map the landscape and surface the sub-questions worth exploring in parallel. Tell the scout explicitly: *"Run ONE entry-point batch to map the topic. Then return: (a) what you found in that batch, and (b) a list of 2-6 sub-questions that should be researched in parallel by separate agents to answer the goal fully. Don't try to answer everything yourself — the next step fans out."*

**State the research surface explicitly.** "Research surface: internet only / codebase only / both." For codebase work, give the agent enough orientation to start (key directories, the project's `CLAUDE.md` path, anything specific the user mentioned). For internet work, name the time horizon ("prefer 2025–2026 sources").

**Include the storage path in the RRP.** Tell the scout: *"Write a scout-batch note to `{full path}.scout.md` using the Write tool. Include: (1) **Prompt** verbatim, (2) **Goal**, (3) **Scout batch** — what you searched/grepped and what you learned with sources, (4) **Sub-questions for fan-out** — the 2-6 questions to send to parallel agents, each with a one-line rationale. In your final chat reply, return ONLY the sub-questions list and a one-line headline of what the scout found."*

**(Pre-identification shortcut.)** If during Step 1 the goal is already structured (e.g., "compare A, B, C" or "audit auth, authz, transport, secrets"), you can pre-decide the sub-questions and skip the scout — go straight to fan-out at Step 4. Use the scout when the topic is open-ended; skip it when it isn't.

Use the `Agent` tool, fork form preferred. Pass `model: "sonnet"`. End your turn after spawning. Do not predict findings.

### Step 4 — Fan out: parallel agents, one RRP each

When the scout returns (or immediately, if you pre-identified the sub-questions in Step 1), build **one RRP per sub-question** and spawn them all in **a single message with multiple Agent tool calls** so they run in parallel.

Each fan-out RRP:
- Targets exactly one sub-question (not the full goal)
- Is self-contained — inline the relevant scout findings each agent needs to start
- Includes its own storage path: `{base path}.{sub-question-slug}.md`
- Requires the same file structure (Prompt, Goal, Pipeline run, Findings, Plan, Open questions) and terse chat reply
- Specifies the surface (internet / codebase / both)
- Passes `model: "sonnet"`

Tell the user one sentence — "Fanning out N parallel research agents." — and end your turn. Each agent runs its own dynamic batch pipeline and pressure-test pass before reporting.

### Step 4.5 — Aggregate into ONE file and clean up

When all fan-out agents have returned, synthesize everything into a **single file**:

1. Read each sub-report file to get the full detail (don't rely only on the chat summaries).
2. Write the **aggregate report** to the original storage path `{full path}` (the un-suffixed file from Step 2). This file is the **complete, self-contained record** — no appendix trail, no cross-links to external files. Embed ALL content inline:
   - **(1) Prompt** — the original RR request and the refined goal
   - **(2) Fan-out plan** — the sub-questions and which agent took each
   - **(3) Scout findings** — what the scout batch found (landscape, key sources, what shaped the fan-out)
   - **(4) Per-sub-question Findings** — the **full substantive findings** from each fan-out agent, with citations. Do NOT summarize and cross-link — copy the meat of each sub-report directly into this section. One heading per sub-question.
   - **(5) Verdict** — your synthesis across all sub-reports
   - **(6) Plan** — the action recommendation
   - **(7) Open questions** — anything still unresolved
3. After the aggregate file is written and verified, **delete all intermediate files**: `.scout.md` and every `.{slug}.md`. The aggregate file is the ONE record; the intermediates are scaffolding and must not litter the research directory.

The chat output stays terse — Verdict + key Findings + Plan + path to the single aggregate file. The full trail lives in that one file, not scattered across a directory.

### Step 5 — Deliver the aggregate report

The **one file at the un-suffixed storage path is the complete record.** There are no other files to read. The user does NOT want the per-batch parade or the per-sub-question expansion in the chat — they want the synthesized answer, with a pointer to the single file that has everything.

**Default chat output — terse:**

```
Saved: {full path to file}

## Verdict
{1-3 sentences — the headline answer / decision / recommendation}

## Findings (key points)
{3-7 bullets — the substantive answer condensed. Cite sources inline where it matters.}

## Plan
{concrete, opinionated, actionable — the action the user should take. If a decision: name it.}

## Open questions
{anything unresolved, only if material. Skip the section entirely if there's nothing.}
```

**Do NOT print the Pipeline run / batch list in the chat.** That belongs in the file. If the user wants to see how the agent got there, they read the file or ask "show me the batches."

The agent's **file** must contain the full structure (Goal / Pipeline run / Findings / Plan / Open questions) — that's the persisted research record. Your **chat reply** is the executive summary: Verdict + key Findings + Plan + path to the file.

If the agent's report skipped the Plan or Verdict, do not paper over it — either ask the agent to produce one (SendMessage) or write it yourself based on the findings, clearly marking it as your synthesis.

---

## Mode 2 — RRP (write a prompt for someone else to execute)

When the user says RRP, they will run the prompt **in a different chat** (often a fresh context, possibly a different model). Your job is to write a prompt that produces a usable RR report **without** the executor having access to this conversation, this codebase, or any of the surrounding context.

### Constraints on the prompt you write

- **Self-contained.** No "as we discussed", no reference to prior turns, no assumption that the executor has read any file. If context matters ({PROJECT_NAME} stack, scale, constraints, deadline), inline it in the prompt.
- **Goal first, topic second.** State the refined goal at the top so the executor doesn't waste batches figuring out what's actually wanted.
- **State the surface.** "Research surface: internet / codebase / both." For codebase research the executor needs path orientation; for internet research the executor needs a time horizon.
- **Embed the RR protocol inline.** Do NOT assume the executor has the `rr` skill. Briefly explain the dynamic-batch pipeline, the "satisfies and beyond N+1" rule, and the required report structure. ~10 lines is enough — copy the essentials below.
- **Specify the deliverable shape.** Tell the executor exactly which sections to return (Verdict / Findings / Plan / Open questions in the chat reply; full Pipeline run details in the saved file if any).
- **Name the time horizon (internet research).** "Use sources from 2025–2026 unless older is the canonical reference." Stale data is the #1 RR failure mode for internet topics.
- **Authorize tools.** Internet: "Use web search, fetch official docs, prefer primary sources over blog summaries." Codebase: "Use grep, glob, read; prefer reading actual code over guessing from doc strings."
- Do **not** include secrets, internal URLs, or anything Reza wouldn't paste into a fresh chat window.

### Output format for RRP mode

Wrap the prompt in a fenced block so Reza can copy-paste it cleanly. Briefly above the block, state in one line what context you embedded so he can sanity-check.

```
Context I embedded: {one line}

---PROMPT BELOW — copy into another chat---

```

{the prompt}

```

```

### Reusable RR-protocol snippet to embed inside RRP prompts

When writing an RRP, paste a compact version of the protocol so the executor knows the shape. Suggested wording:

> Run this as a **dynamic research pipeline**, not a single search/grep. The research surface is **{internet / codebase / both}** — use those tools accordingly (web search & fetch for internet; read/grep/glob/bash for codebase). Start with one entry-point query to map the landscape, then plan each next batch based on what the previous batch returned — not a plan written up front. Continue until the goal is answered, then do one extra "pressure-test" batch to look for counter-evidence, newer sources, or contradicting code paths before stopping. Return the result in this shape: **Verdict** (1-3 sentences — the headline answer) / **Findings** (3-7 bullets, citations inline) / **Plan** (concrete, opinionated recommendation) / **Open questions** (anything unresolved). Keep the per-batch breakdown out of the reply — if the user wants the trail, they'll ask.

---

## Common failure modes (avoid these in both modes)

- **Executing RR inline instead of delegating.** RR mode spawns an agent. If you're running WebSearch/WebFetch/Grep in the main conversation for an RR request, you're doing it wrong — that noise is exactly what delegation avoids.
- **Single-shot search/grep.** RR is a pipeline. One WebSearch is not RR. One Grep is not RR. (Applies to whoever executes — main, agent, or the user in another chat.)
- **Picking the wrong surface.** Codebase questions ("how does our auth work") sent as internet research return generic blog posts; internet questions ("which vector DB should we use") sent as codebase research return "we don't have one." Pick deliberately. If unsure, ask.
- **Plan the whole pipeline up front.** The plan is supposed to evolve. If batch 3 was decided before batch 1 ran, the pipeline isn't RR.
- **Stop at first plausible answer.** "Satisfies and beyond N+1" exists because the first answer is often a confident-sounding wrong one. The pressure-test pass catches this.
- **Dump findings, skip the plan.** Reza asked for a report **and a plan**. A wall of facts is half the deliverable.
- **Spam the chat with the batch trail.** The user-facing reply is Verdict + Findings + Plan + file path. The batch-by-batch trail goes in the file. If you find yourself writing "Batch 1 — searched X, found Y" in the chat, stop — that's file content.
- **For both RR and RRP: assume the executor has context.** They don't — neither the spawned agent (RR) nor the user's other chat (RRP). Inline everything that matters.
- **Predicting the agent's findings while it runs.** Once the RR agent is spawned, you know nothing about what it found until it returns. Don't fabricate or summarize in advance.
- **Skipping the file write.** Every RR run produces a research file in the caller's research directory. The file MUST contain the original prompt, the full pipeline run, and the report. If the agent returns findings only in chat, the work isn't persisted — future conversations can't reference it. The file is the canonical record; the chat reply is the courtesy summary.
- **Leaving multiple files behind.** The final state must be ONE aggregate file. Leaving `.scout.md` and `.{slug}.md` files in the research directory after aggregation is a failure — delete the intermediates once the aggregate is written. A research directory full of partial files is confusing and the user should never have to hunt across them to piece together the full answer.

---

## Triggers (so you know when to load this skill)

Load this skill when the user's message includes any of:

- `RR <something>`
- `RRP <something>` / `RR-prompt for <something>` / "write me an RR prompt"
- "research and report on <something>" / "do an RR on <something>"

Do NOT load this skill for ordinary research requests like "look up X" or "what is Y" — those don't need the pipeline.
