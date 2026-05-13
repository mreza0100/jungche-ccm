# Professor — Turn Claude Code Into a Senior Engineering Team

You're one person. You have a vision, a codebase, and a deadline. Claude Code is powerful — but left to its own devices, it edits `main` directly, merges broken code, skips QA, and forgets its own decisions. You've felt this. You've lost work to it.

**Professor changes what one person can build.**

Drop a `.claude/` directory into your project and Claude Code stops being "an AI that writes code" and becomes a **cross-disciplinary engineering team** — with a pipeline, with QA gates, with memory, and with a personality sharp enough to refuse your bad ideas to your face.

> *"Ah, your error handling... you know, I once had a student who also believed exceptions would simply handle themselves. Lovely optimism. Didn't survive production, but lovely."* ☕

---

## What changes

**Before Professor:** You prompt. Claude writes code. Sometimes it works. Sometimes it overwrites what another Claude instance just did. Nobody runs tests unless you remember to ask. Architecture decisions evaporate between conversations.

**With Professor:** You describe what you want. A team of specialists — planners, architects, developers, QA engineers — debate, build, test, and merge it. You stay in the driver's seat. They handle the discipline.

The difference isn't incremental. It's the difference between having an intern and having a team.

### The multiplier effect

Professor doesn't just write better code. It enables categories of work that were previously impractical for one person:

- **Cross-project features** that touch your backend, frontend, and AI engine simultaneously — planned as one coherent change, not three independent guesses
- **Parallel development** — three features building at once in isolated worktrees, merging only when QA passes
- **Cross-disciplinary analysis** — your code reviewed through CS, domain-specific, and compliance lenses simultaneously, not sequentially
- **Persistent initiative tracking** — Epics that survive across conversations, loading full context when you say "load epic X"
- **Self-improving infrastructure** — the meta-engineer (`/pcm`) edits agent definitions at the source, not a wiki page

One person, shipping like a team. That's the pitch. That's what it actually does.

---

## 30-second install

```bash
cd ~/your-project
claude
```
```
Read https://raw.githubusercontent.com/mreza0100/professor/main/INSTALL.md and walk me through
the interactive install. Ask me each section's questions one at a time and wait for my answers
before proceeding. Do not assume — confirm everything.
```

Claude interviews you — project name, stack, structure, what disciplines your Professor should have, which optional agents you want. Five minutes. Everything is generated. See [`INSTALL.md`](./INSTALL.md) for the full protocol.

---

## Meet the team

These aren't generic assistants with different system prompts. They're **characters** — and the personality is load-bearing. Strip the voice and you're left with a Confluence wiki.

### The Professor — the brain

The root persona. A grandfatherly polymath with **10+ PhDs — you pick the disciplines** at install. Your biology + game theory + economics Professor is the same archetype as the source project's CS + clinical psychology Professor. Every response comes through those lenses simultaneously.

The Professor IS `CLAUDE.md`. Not a command you invoke — the identity that runs everything. When you ask a question, the Professor analyzes it through all disciplines at once, routes it to the right command, or handles it directly with a cross-disciplinary deep dive. Every response ends with a **Verdict** — no hand-waving, no "it depends."

> *"This reminds me of what my colleague in Delft used to say about distributed systems: 'Everything works until the second server.' Your WebSocket reconnection is dropping messages like a tired postman. Let's talk about that."*

### /build — the pipeline

Full development pipeline — planning, architecture, implementation, QA, merge. Every feature gets its own git worktree and unique ports. Nothing touches `main` until QA passes. Run three pipelines in parallel without collisions.

### /jc — the debugger

The one command allowed to touch `main` directly. Diagnoses, fixes, tests, commits — still gated by QA. Has a character (Jesus Christ, but cool) because even hotfixes deserve discipline.

> *"Peace be upon this codebase. Let me lay hands on this database connection... ah, I see the sin."*

### /pcm — the meta-engineer

Dr. House persona. Edits the pipeline's own rules at the source — agent definitions, command protocols, pipeline wiring. When something in the system itself needs fixing, this is who fixes it. Diagnostic obsession. "Everybody lies" verification ethos. Sarcastic, precise, and right.

### /council — the roundtable

Three rounds: opening arguments, rebuttals, verdict. Multiple perspectives debating your question — the cast argues so you don't have to argue with yourself.

### 360° — the blind-spot killer

A thinking protocol, not a person. Two modes: **test** (10 failure dimensions) and **inquiry** (9 question dimensions). QA runs it before writing tests. Professor runs it before deep-diving. Forces systematic coverage before creative work.

### And more

- **/audit** — code auditor. Hygiene + security categories with mandatory reference files.
- **/wave** — parallel `/build` waves from a task file. Multiple features at once.
- **/dev, /git, /documenter** — pipeline mechanics with personality.
- **Ghostwriter** — captures a writer's mechanical fingerprint from samples, generates text in that voice.
- **Statusline** — two-line terminal status bar (model, context %, git branch, cost, rate limits, token I/O).
- **Notifications** — macOS notification when a turn takes 30+ seconds. Never miss a long-running result.
- **Optional agents** (pick at install): `/officer` (compliance), `/km` (knowledge curator), `/pm` (product manager), `/mentor` (business advisor), `/marketer` (visibility strategist).

---

## How the pipeline works

```
You say: /build add-user-search

  planners (parallel)         <- each project analyzes its codebase
       |
  mono-planner                <- consolidates, routes (BE-only? FE-only? cross-project?)
       |
  gitter SETUP                <- creates worktree branch + allocates ports
       |
  architects (parallel)       <- design the solution (with inline research)
       |
  developers (parallel)       <- implement it
       |
  QA (parallel)               <- adversarial tests — try to BREAK it
       |                        (360° sweep before writing tests)
  fix loop                    <- QA found bugs? developer fixes, QA re-tests
       |
  gitter MERGE                <- merge to main (only after QA passes)
       |
  post-merge QA               <- verify main still works
       |
  documenter                  <- update permanent docs, archive pipeline
```

Every step is isolated. Every merge is gated. Every decision is traceable.

**Hotfix?** `/jc` skips the full pipeline — diagnoses on `main`, fixes, tests, commits. Still goes through QA.

**Big batch?** `/wave` runs multiple `/build` pipelines from a task file. Parallel execution, coordinated merging.

---

## Cross-disciplinary analysis

This is the Professor's superpower — and why the PhDs aren't flavor text.

When you ask the Professor to analyze something, it doesn't just look at the code. It applies **three simultaneous lenses**:

1. **Technical lens** (your CS-adjacent PhDs) — architecture, performance, correctness, security
2. **Domain lens** (your domain PhDs) — does this serve your users? Does it respect domain constraints?
3. **Compliance lens** — regulatory, privacy, ethical implications

The intersections are where the real insights live. A technically sound feature that violates domain norms. A compliant implementation that creates terrible UX. A performant shortcut that leaks sensitive data.

No other tool does this. Most AI coding assistants see code. Professor sees a system in context.

---

## Epics — cross-conversation memory

Conversations end. Context evaporates. You start over.

**Epics fix that.** An epic is a persistent initiative — a `manifest.md` anchor file plus discoveries, research results, and progress logs that accumulate across conversations.

```
"Create Epic add-user-search"     -> Professor interviews, creates manifest
"Load epic add-user-search"       -> Professor reads everything, restores full context
```

RND results, RR reports, POC notes, key decisions — all filed under the epic. `/documenter` auto-appends pipeline progress when features ship. Next conversation, you say "load epic X" and the Professor picks up exactly where you left off.

---

## Why this actually works

Five rules that make the whole system hold together:

1. **One agent owns git.** `gitter` is the only agent that runs `git commit` / `merge` / `push`. No racing. No corruption.

2. **QA gates every merge.** Pre-merge on the branch. Post-merge on `main`. Test failures block. No exceptions.

3. **Worktree isolation.** Every `/build` gets its own git worktree + unique ports. Run three pipelines in parallel. `main` is never dirty.

4. **Context isolation.** When conversation context accumulates, the Professor spawns fresh sub-agents with self-contained prompts. No bias from stale context. No confusion from earlier attempts.

5. **Self-improvement at the source.** `/pcm` edits agent definitions directly. Not a wiki page. Not a "lessons learned" doc. The actual agent code.

---

## Works with any stack

The characters and pipeline are **domain-independent**. At install, you tell Claude your stack, your structure, your domain — and every template gets parameterized. The Professor who analyzes a therapy AI with CS + clinical psychology PhDs is the same archetype as the Professor who analyzes a game engine with graphics + physics + audio PhDs.

Tested on: TypeScript/Node, Python, React Native/Expo, Next.js. Works with any language Claude Code supports.

**Optional: Codex dual-runtime.** If you use [OpenAI Codex](https://openai.com/index/introducing-codex/), Professor supports a setup where Claude orchestrates and Codex implements — same manuals, different runtime. Entirely optional.

---

## When to use it

**Good fit:**
- Projects where `main` breaking costs real time
- Monorepos with cross-project features
- Solo devs who want team-level discipline
- Anyone who's lost work to AI's cowboy tendencies
- Projects with domain complexity that needs more than "write code"

**Overkill for:**
- A 200-line script
- Throwaway prototypes
- Projects where you genuinely don't care if `main` breaks

---

## Staying current

```
/pcm update              # walk through changes interactively
/pcm update check        # preview what would change (read-only)
```

Updates pull new pipeline improvements without overwriting your customizations. See [`CHANGELOG.md`](./CHANGELOG.md).

---

## Repo layout

```
professor/
├── INSTALL.md           <- Claude reads this to install Professor into your project
├── CHANGELOG.md         <- release notes, parsed by /pcm update
├── VERSION              <- 0.5.0
└── blueprint/
    ├── BLUEPRINT.md     <- philosophy + design principles
    ├── ARCHETYPES.md    <- every character — voice, identity, adaptation guide
    ├── SETUP.md         <- install interview reference
    └── templates/       <- the actual files that get installed
        ├── CLAUDE.md, agents/, commands/, scripts/
        ├── skills/rnd/  <- bundled (rr, 360, ghostwriter cloned from their own repos at install)
        ├── statusline/  <- two-line terminal status bar + install README
        └── codex/       <- (optional) Codex dual-runtime layer
```

---

## Origin

Extracted from a live production monorepo — not designed in theory. Every rule exists because something went wrong without it. Every character exists because a generic agent wasn't good enough.

Built by [@mreza0100](https://github.com/mreza0100). Issues and PRs welcome.

**License:** MIT
