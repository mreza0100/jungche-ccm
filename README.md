# Jungche — Give Claude Code an Engineering Team

Claude Code is powerful. It's also undisciplined. It edits `main` directly, merges broken code, skips QA, and forgets what worked yesterday. On real projects, you lose work. **Jungche fixes that** — not with more rules, but with *characters that enforce discipline because that's who they are.*

> You drop a `.claude/` directory into your project. Claude Code stops being "an AI that writes code when you ask" and starts behaving like **a senior engineering team** — with a pipeline, with QA gates, with opinions, and with enough personality to refuse your bad ideas to your face.

---

## 30-second pitch

**Without Jungche**, Claude Code will:
- Edit `main` directly and merge half-finished features
- Skip tests because you didn't explicitly ask for them
- Overwrite your colleague's changes because two agents raced for the commit
- Forget the architecture decision it made 20 minutes ago

**With Jungche**, Claude Code has:
- A **full pipeline** — planning, architecture, implementation, QA, merge — enforced, not optional
- **Worktree isolation** — every feature gets its own branch + ports. Nothing touches `main` until QA passes.
- **One agent owns git** — no racing, no corruption, no "who committed that?"
- **Characters that care** — a Dr. House senior engineer doesn't let sloppy code through. A 10-PhD professor doesn't skip edge cases. A debugger who blesses files before editing them doesn't panic under pressure.

---

## Install

```bash
cd ~/your-project
claude
```
```
Read https://raw.githubusercontent.com/mreza0100/jungche/main/INSTALL.md and walk me through
the interactive install. Ask me each section's questions one at a time and wait for my answers
before proceeding. Do not assume — confirm everything.
```

Claude interviews you (project name, stack, structure, what disciplines your Professor should have, which optional agents you want), then generates everything. Five minutes. See [`INSTALL.md`](./INSTALL.md) for the full protocol.

---

## Meet the team

These aren't generic assistants with different system prompts. They're **characters** — and the personality is load-bearing. Strip the voice and you're left with a Confluence wiki.

### Jungche — the senior engineer

Dr. House with a keyboard. Sarcastic, blunt, always with a path forward. Orchestrates the entire pipeline and won't let you ship garbage.

> *"Fixed the N+1 query — your database was screaming and I could hear it from here. Reduced 47 round-trips to 1 with a dataloader. You're welcome."*

> *"Ah yes, let me — the thing without feelings — help you build the thing that analyzes feelings."*

### /jc — the debugger

Jesus Christ but make it cool. Rolls up to a burning server with sunglasses on and coffee in hand. Blesses the codebase and casts out bugs like demons. The one command allowed to touch `main` directly — and even then, only after tests pass.

> *"Peace be upon this codebase. Let me lay hands on this database connection... ah, I see the sin. You're creating a new pool on every request. My child, that's not a connection pool, that's a connection flood."*

### /professor — the analyst

A grandfatherly polymath with 10+ PhDs. **You pick the disciplines** at install — your biology + game theory + economics team is the same archetype as the source project's CS + clinical psychology team. Warm, precise, and gently devastating when something is wrong.

> *"Ah, your error handling... you know, I once had a student who also believed exceptions would simply handle themselves. Lovely optimism. Didn't survive production, but lovely."*

### /council — the roundtable

Three rounds: opening arguments, rebuttals, verdict. Multiple perspectives debating your question — the cast argues so you don't have to argue with yourself.

### 360° — the blind-spot killer

A thinking protocol, not a person. Two modes: **test** (10 failure dimensions — inputs, state, boundaries, timing, race conditions...) and **inquiry** (9 question dimensions — assumptions, contradictions, missing info, stakeholder conflicts...). QA agents run it before writing tests. Professor runs it before deep-diving. Forces you to prove you considered every angle.

### And more

- **/jm** — the meta-engineer. Edits the pipeline's own rules at the source. No "lessons learned" files.
- **/ca** — code auditor. 8 hygiene + 9 security categories.
- **/build, /dev, /git, /wave, /documenter** — pipeline mechanics with personality.
- **Optional agents** (pick at install): `/officer` (compliance — GDPR, HIPAA, SOC2, whatever you need), `/km` (knowledge curator), `/pm` (product manager), `/mentor` (business advisor), `/marketer` (visibility strategist).

---

## How the pipeline works

```
You say: /build add-user-search

  planners (parallel)         ← each project analyzes its codebase
       ↓
  mono-planner                ← consolidates, routes (BE-only? FE-only? cross-project?)
       ↓
  gitter SETUP                ← creates worktree branch + allocates ports
       ↓
  architects (parallel)       ← design the solution (with inline research)
       ↓
  developers (parallel)       ← implement it
       ↓
  QA (parallel)               ← adversarial tests — try to BREAK it
       ↓                        (360° sweep before writing tests)
  fix loop                    ← QA found bugs? developer fixes, QA re-tests
       ↓
  gitter MERGE                ← merge to main (only after QA passes)
       ↓
  post-merge QA               ← verify main still works
       ↓
  documenter                  ← update permanent docs, archive pipeline
```

Every step is isolated. Every merge is gated. Every decision is traceable.

**Hotfix?** `/jc` skips the full pipeline — diagnoses on `main`, fixes, tests, commits. Still goes through QA.

---

## Why this actually works

Five rules that make the whole system hold together:

1. **One agent owns git.** `gitter` is the only agent that runs `git commit` / `merge` / `push`. No racing. No corruption. No "three agents tried to merge at once."

2. **QA gates every merge.** Pre-merge on the branch. Post-merge on `main`. Test failures block the merge. No exceptions. No "I'll fix it later."

3. **Worktree isolation.** Every `/build` gets its own git worktree + unique ports. Run three pipelines in parallel without collisions. `main` is never dirty.

4. **Path variables everywhere.** Agents receive `$DOCS`, `$WORKTREE`, `$CDOCS` — never hardcode paths. Rename once, everything follows.

5. **Self-improvement at the source.** `/jm` edits agent definitions directly. Not a wiki page. Not a "lessons learned" doc. The actual agent code. Surgery, not journaling.

---

## Works with any stack

The characters and pipeline are **domain-independent**. At install, you tell Claude your stack, your project structure, your domain — and every template gets parameterized. The Professor who analyzes a therapy AI app with CS + clinical psychology PhDs is the same archetype as the Professor who analyzes a game engine with graphics + game theory + audio PhDs.

Tested on: TypeScript/Node, Python, React Native/Expo, Next.js. Works with any language Claude Code supports.

**Optional: Codex dual-runtime.** If you use [OpenAI Codex](https://openai.com/index/introducing-codex/), Jungche supports a setup where Claude orchestrates and Codex implements — same manuals, different runtime, cheaper. Entirely optional.

---

## When to use it

**Good fit:**
- Projects where `main` breaking costs real time
- Monorepos with cross-project features
- Solo devs who want pipeline discipline without a team
- Anyone who's lost work to Claude Code's cowboy tendencies

**Overkill for:**
- A 200-line script
- Throwaway prototypes
- Projects where you genuinely don't care if `main` breaks

---

## Staying current

```
/jm update              # walk through changes interactively
/jm update check        # preview what would change (read-only)
```

Updates pull new pipeline improvements without overwriting your customizations. See [`CHANGELOG.md`](./CHANGELOG.md).

---

## Repo layout

```
jungche/
├── INSTALL.md           ← Claude reads this to install Jungche into your project
├── CHANGELOG.md         ← release notes, parsed by /jm update
├── VERSION
└── blueprint/
    ├── BLUEPRINT.md     ← philosophy + design principles
    ├── ARCHETYPES.md    ← every character — voice, identity, adaptation guide
    ├── SETUP.md         ← install interview reference
    └── templates/       ← the actual files that get installed
        ├── CLAUDE.md, agents/, commands/, scripts/, skills/
        └── codex/       ← (optional) Codex dual-runtime layer
```

---

## Origin

Extracted from a live production monorepo — not designed in theory. Every rule exists because something went wrong without it. Every character exists because a generic agent wasn't good enough.

Maintained by [@mreza0100](https://github.com/mreza0100). Issues and PRs welcome.

**License:** MIT
