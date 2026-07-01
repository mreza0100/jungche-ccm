---
name: p:2opinion
description: "Get a blind, independent second opinion on the problem currently on the table — spawn a fresh agent that never saw this chat, point it at the primary sources, let it reach its own verdict, then compare to surface what we're missing or doing wrong. Triggered by 'second opinion', 'fresh eyes', 'what are we missing', 'are we doing this wrong', 'sanity-check this approach'. Frame as /p:2opinion."
argument-hint: [what to get a second opinion on]
---

# Chat 2nd Opinion — a blind, independent read of the current problem

This chat is thick with our reasoning, our dead-ends, and the path we've half-talked ourselves into. A second opinion is only worth something if it's blind to the first. This spawns a fresh agent that never saw any of it, points it at the primary sources, lets it reach its own verdict, then compares — the gap between its read and ours is the blind spot the noise was hiding.

## When to load

- Circling, stuck, or about to commit to an approach and want it checked.
- "What are we missing?", "are we doing this wrong?", "fresh eyes on this".
- A decision, design, or diagnosis that matters enough to be worth a blind read.

## Step 1 — Name the subject

From `$ARGUMENTS`, or if empty, from the live thread: the one decision, design, diagnosis, or piece of code on the table. Frame it as a question the agent answers from scratch — not "confirm what we concluded".

## Step 2 — Build the briefing — strip the noise

This step is the whole command. The agent's only value is that it is blind to our reasoning; protect that.

Include:

- The question, stated neutrally.
- Exact pointers to the primary artifacts — file paths, the actual proposal, the real data — so it reads source, not our summary of it.
- The genuine constraints and goal that bound the problem.

Leave out our conclusion, our leaning, the approach we're about to take, the alternatives we already rejected, and the conversation's accumulated detours. If the briefing hints at the answer we want, it's poisoned — rewrite until it's a question, not an answer.

## Step 3 — Spawn the fresh agent

One `general-purpose` agent, clean context — the briefing is its entire world. Instruct it: "You have no prior context on this. Read the listed artifacts yourself. Reason from first principles: what is the right call, what is being overlooked, and where would this go wrong? Be direct — disagree if disagreement is warranted, don't reassure. Advisory only: read, verify, and report; change no files."

## Step 4 — Compare and surface the delta

The agent reached its verdict blind to ours. Read its opinion against our actual direction and report:

- Where it independently agrees — earned confidence, not an echo.
- Where it diverges — that divergence is the finding. Lead with it.
- What it raised that we never considered.

Relay it straight. Do not argue the agent back toward our original position — the moment you defend the first opinion, the second is worthless.

## Rules

- Blind by design — never seed the agent with our conclusion or preferred path.
- Primary sources over summary — point it at the real files so its read is independent, not a re-read of ours.
- One agent, one independent read — a second opinion is singular, not a panel.
- Relay honestly — surface divergence even when it indicts the current plan, especially then.

## Example — a settled design

Founder: `/p:2opinion the retry-queue design we just settled on`
Command: Subject = should failed-work retry live on the message queue or a DB poll loop? Briefs a fresh agent with the design-doc path + the queue and worker code, NOT our pick. The agent independently argues for the DB poll on idempotency grounds. Report: we chose the queue; the blind read flags idempotency as the gap we glossed over.

## Example — an in-flight refactor

Founder: `am I overcomplicating this auth refactor? get a second opinion`
Command: Subject = the auth refactor approach. Fresh agent gets the current auth code + the refactor goal, blind to the plan. Returns: the refactor solves a problem two layers up; a simpler fix exists at the request-handler boundary. Report surfaces the simpler path as the thing we were missing.
