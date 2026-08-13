---
name: agent-lane
description: Study one agent and author the dreamer lane profile that governs what its memory will hold. Trigger — /agent-lane {agent-type} to build or rebuild a profile, /agent-lane {agent-type} audit to report the gap without writing. Use when an agent has no lane profile, when its profile predates a change in what it does, or when its maps keep missing the same class of lesson.
argument-hint: "{agent-type} [audit]"
---

# Agent lane — author the profile that decides an agent's memory

The lane profile at `{organ}/lanes/{lane}.md` is the highest-leverage file in the dreamer engine: it tells the distill seat what this agent's corpus is, who the maps serve, and what earns a map at all. Every night that lane ever runs inherits it. A profile written from imagination teaches the engine to harvest work the agent does not do.

So the profile is derived from what the agent ACTUALLY did, never from what its definition says it does. `$1` names the agent type; `audit` reports the gap and stops.

## 1. Understand the agent

Read, in this order: its root wrapper (`.claude/agents/{type}.md`), the child protocol the wrapper points at, the project `CLAUDE.md` it works under, and — the one most people skip — **the spawn brief in the command that launches it**. An agent's real behaviour is set as much by its caller as by its own definition: bans, model tier, and scope usually live in the spawn, not the protocol.

Write down, in a sentence each: what this agent is accountable for, what it is forbidden, and what a failed run costs. Those three decide what earns a map later.

## 2. Assemble the corpus — expect it to be starved

Typed discovery first: metas under the session registry whose `agentType` equals `$1`, paired with a `.jsonl` body.

**Most agents return almost nothing here, and that is the normal case, not an error.** An agent only appears under its own type once it is spawned as a REGISTERED type; anything spawned as `general-purpose` with a protocol brief lands in an undifferentiated pool. Report the census honestly — metas found, how many carry a body, how many are probes rather than work.

When typed discovery is thin, fall back to SUBSTANCE: find transcripts where this agent's work actually happened, whoever ran it — the commands only it runs, the files only it edits, the artifacts only it writes. Write the selection to a corpus file whose leading `#` comments state the rule used and why the typed corpus was insufficient; the engine passes those comments to the seat as provenance.

Never write a profile with no corpus behind it. An agent with neither typed nor substance transcripts gets a reported gap and no file — say so and stop.

## 3. Mine for pitfalls — two passes, not one

Condense before reading; raw transcripts run megabytes. Assistant tool calls become `T {tool}|{input first 200}`, assistant text becomes `A {first 300}`; polling and transport noise carry no evidence.

**Pass A — distil (per transcript).** Read for the knowledge the agent EARNED, which its maps rarely capture:

- **Techniques** — the method that established a fact instead of assuming it.
- **Corrected priors** — the belief the evidence refuted; the wrong turn is as valuable as the truth, because the next agent meets the same evidence.
- **Baselines** — what NORMAL looks like, so a future run does not investigate a standing condition.

**Pass B — count (across transcripts).** These exist in no single transcript and are invisible to a per-transcript reader:

- **Rule violations** — sanctioned instrument bypassed, forbidden tool reached for. Count them; one is a slip, nine is a norm.
- **Role drift** — what it actually spends its turns on versus its charter. Count edits by directory, tool mix, wall-clock.
- **Repeated friction** — the same dead end, retry, or wasted lookup across runs.

Delegate the condensing and counting; do the reading and the judging yourself.

## 4. Diff against what its memory already holds

Read the lane's existing maps and its current profile. Only what is MISSING gets written. A profile that restates an existing map, the agent's own protocol, or a standing repository rule spends the seat's attention on what it already knows — and the profile is re-read on every night that lane runs.

## 5. Author the profile

Repo- and lane-specific worked examples belong HERE. The global seat prompts carry only laws; this file carries the instances that make them land. Shape:

- Identity — which agent, where its protocol lives, which rules it works under.
- Corpus — what a transcript must contain to belong to this lane.
- Audience and earning bar — who the next reader is, and the concrete list of what saves them a failed run.
- Lane laws — the discipline this agent specifically lacks, stated as a rule.
- **The failures this lane has already paid for** — each one real, each with what was written, why it was wrong, and what refuted it. This section does the most work; a lane with no paid failures yet says so rather than inventing one.
- Forgotten triggers — the files whose change invalidates this lane's maps and that its authors keep omitting.
- What never earns a map here.

Then propose it. The profile is a prompt, and a bad one silently poisons every night that lane runs — so it is ratified before it is installed, never written straight to disk.

## 6. Install and verify

Write to `{organ}/lanes/{lane}.md`. Confirm the engine resolves it — the lane must appear to the runner and the organ-local profile must take precedence over any global one of the same name — then confirm the lane can run at all. Report the census from step 2 alongside it, so a starved lane is visible as starved rather than as a quiet profile that will never harvest anything.
