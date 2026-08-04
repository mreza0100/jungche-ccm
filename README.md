# Professor — Turn Claude Code Into a Senior Engineering Team

Claude Code is powerful and undisciplined. Left alone it edits `main` directly, merges before tests pass, forgets yesterday's architecture decision, and cheerfully overwrites what another instance just wrote. If you've shipped real work with it, you already know this.

**Professor is the discipline layer.** Drop it into any repo — one project or fifteen — and you get a pipeline with QA gates, a single owner for git, isolated worktrees, memory that survives conversations, and a cast of specialists with enough personality to tell you your idea is bad.

> _"Ah, your error handling... I once had a student who also believed exceptions would simply handle themselves. Lovely optimism. Didn't survive production, but lovely."_ ☕

---

## Install

```bash
cd ~/your-project
claude
```

Then paste:

```
Read https://raw.githubusercontent.com/mreza0100/professor/main/INSTALL.md and walk me through
the interactive install. Ask me each section's questions one at a time and wait for my answers
before proceeding. Do not assume — confirm everything.
```

Claude interviews you — structure, stack, which disciplines your Professor should hold, which optional agents you want — then generates everything. About five minutes. Full protocol in [`INSTALL.md`](./INSTALL.md).

---

## The rules that make it work

Everything else is detail. These five are load-bearing:

1. **One agent owns git writes.** Only `gitter` runs `commit` / `merge` / `push` / `checkout`. Read-only git is open to everyone. No races, no half-merges.
2. **QA gates every merge.** Once on the branch, once again on `main` after. A failing test blocks. There is no flag to skip it.
3. **Worktree isolation.** Each pipeline gets its own worktree and its own ports. Three features build at once without colliding. `main` stays clean.
4. **Context isolation.** Long conversations rot. The Professor spawns fresh sub-agents with self-contained briefs rather than dragging stale context forward.
5. **Self-improvement at the source.** `/pcm` edits the agent definitions themselves — not a wiki, not a "lessons learned" file. The rules and the code that follows them are the same artifact.

---

## The cast

These are characters, not system prompts with different adjectives. The voice is load-bearing — strip it and you have a Confluence page nobody reads.

**The Professor** — the root persona, and the identity behind `CLAUDE.md` itself. A grandfatherly polymath with ten-plus doctorates that **you choose at install**. Every answer arrives through those lenses at once and ends with a **Verdict** — a decision, not a shrug.

**`/jc`** — the one route allowed to deliver on `main`. Diagnoses, fixes, tests, commits, still QA-gated. Has a character because hotfixes are exactly where discipline usually dies.

**`/pcm`** — the meta-engineer. Owns `.claude/`, every `CLAUDE.md`, the agents, the commands. `pcm audit [scope]` walks the framework's own files against a checklist; `/pcm:update` and `/pcm:release` ride the blueprint bus between installs.

**`/wave:builder`** — the full pipeline: plan → architect → implement → QA → merge, in an isolated worktree.

**`/p:360`** — a thinking protocol rather than a person. Ten failure dimensions in `test` mode, nine question dimensions in `inquiry`. QA runs it before writing tests; the Professor runs it before a deep dive.

Each persona ships in **full** and **compact** depths — same contracts, fewer tokens per turn. Pick at install.

---

## How a feature ships

```
/wave:builder add-user-search

  planners (parallel)      each project reads its own codebase
       |
  mono-planner             consolidates, routes single- vs cross-project
       |
  gitter SETUP             worktree branch + port allocation
       |
  architects (parallel)    design, with inline research
       |
  developers (parallel)    implement
       |
  QA (parallel)            adversarial tests — try to break it
       |                   (360° sweep runs before any test is written)
  fix loop                 QA found bugs → developer fixes → QA re-runs
       |
  gitter MERGE             only reachable once QA is green
       |
  post-merge QA            prove main still works
       |
  documenter               permanent docs updated, pipeline archived
```

Small change? `/jc` delivers on `main` under its own QA. Batch of related work? `/wave:live` runs them on `main` without worktrees; `/wave:orchestrator` drives parallel worktree pipelines from a task file, and `/wave:walker` walks the merged result to prove it actually works.

---

## What ships

| | |
|---|---|
| **Pipeline** | `/wave:builder` · `/wave:orchestrator` · `/wave:live` · `/wave:refine` · `/wave:schedule` · `/wave:walker` · `/wave:watcher` |
| **Delivery** | `/jc` · `/dev` · `/git` · `/documenter` · `/qa:live` |
| **Framework** | `/pcm` · `/pcm:update` · `/pcm:release` · `/pcm:context-meter` |
| **Thinking** | `/p:360` · `/p:rnd` · `/p:tokens` · `/p:slow-burn` · `/sleep` · `/animate` |
| **Quality** | `/quality:doc` · `/quality:prompt` · `/audit:code-hygiene` · `/audit:security` · `/audit:ai-output` |
| **Optional roles** | `/officer` (compliance) · `/km` (knowledge) · `/pm` (product) · `/mentor` (business) · `/marketer` (visibility) |
| **Agents** | `gitter`, `mono-{planner,architect,documenter}`, and per-project `planner` / `architect` / `developer` / `qa` |
| **Source-fetched** | `rr` (research & report) · `ghostwriter` (voice fingerprinting) · `vision-factory` — cloned from their own repos at install, never vendored, so they can't silently drift |

**Also included:** a two-line statusline (model, context %, branch, cost, rate limits) · VSCode terminals that open straight into tmux + Claude · an opt-in memory backup hook that syncs Claude's project memory to a private repo · multi-account fleet tooling (`cc-ls`, `/bb`, `/swap`) for running several subscriptions at once · the `/chat:*` family for reading, forking, injecting into, and coordinating across live sessions · the Tokyo Night theme, source-fetched.

**Codex dual-runtime (optional).** `build-codex.mjs` compiles the entire Codex surface from the Claude sources — an `AGENTS.md` beside every `CLAUDE.md`, agent TOMLs, skill cards, MCP config — with model aliases and command prefixes rewritten for that runtime. Pure string transforms, so nothing is paraphrased or invented. A `Stop` hook recompiles and blocks the turn on a failed build, which means the mirror cannot quietly drift out of sync.

---

## Any repo, any shape

Structure is captured as a **roster** — an ordered list of 1..N projects, each with its own directory, stack, package manager, test runner, and ports. Install expands every template once per entry.

- **One project** is a roster of one, and it is first-class rather than a stripped-down path. The worktree is the repo root, routing collapses, cross-project steps disappear, and the prose reads as "the project."
- **A monorepo** lights up per-project agents and cross-project routing automatically.

The characters are domain-independent. A Professor holding graphics, physics, and audio doctorates for a game engine is the same archetype as one holding CS and clinical psychology for a medical tool — you pick the disciplines, the archetype does the rest.

Exercised on TypeScript/Node, Python, React Native/Expo, and Next.js; nothing in it is stack-specific.

**Good fit:** anything where a broken `main` costs real time, where features cross project boundaries, or where you're one person who wants a team's discipline.
**Overkill for:** a 200-line script, a throwaway prototype, or a repo where `main` breaking genuinely doesn't matter.

---

## Cross-conversation memory

Conversations end and context evaporates. **Epics** are the fix — a persistent `manifest.md` plus the discoveries, research, and progress that accumulate around an initiative.

```
"Create Epic add-user-search"   → Professor interviews you, writes the manifest
"Load epic add-user-search"     → Professor reads it all back, full context restored
```

Research results, proof-of-concept notes, and decisions file themselves under the epic; `/documenter` folds shipped work into it. Next session picks up where the last one stopped.

---

## Staying current

Releases are annotated git tags. Your install records a manifest — interview answers plus file hashes — and updates replay those answers against the new templates:

```
/pcm:update              # interactive update to the latest tag
/pcm:update check        # read-only preview
/pcm:update --to vX.Y.Z  # pin a specific release
```

Changes land in three buckets: **auto-apply** where upstream moved and you didn't, **review** where both moved, **manual** for migrations and new interview questions. Local customizations you want kept are recorded in `drift.md` and survive every update. See [`CHANGELOG.md`](./CHANGELOG.md).

---

## Repo layout

```
professor/
├── INSTALL.md           Claude reads this to install into your project
├── CHANGELOG.md         release index; full notes live in releases/
├── VERSION
└── blueprint/
    ├── BLUEPRINT.md     philosophy and design principles
    ├── SETUP.md         the install interview
    ├── refresh-map.json every template ↔ its live source
    └── templates/
        ├── CLAUDE.md, agents/, commands/, workflows/, scripts/
        ├── output-styles/  personas, full and compact
        ├── skills/         sources.json only — nothing vendored
        ├── statusline/, vscode/, themes/, epics/
        ├── host-swap/      multi-account fleet tooling
        └── codex/          optional dual-runtime layer
```

---

## Maintainer setup

```bash
git config core.hooksPath .githooks
```

Arms the committed pre-push gate: `scripts/leak-check.sh` scans every outgoing diff for brand names, maintainer PII, and machine-absolute paths, and a leaking push fails mechanically rather than politely. It also warns on lightweight `v*` tags, since releases are annotated. Release tooling lives beside it — `genericize.sh` with `placeholder-map.tsv` for the deterministic placeholder pass, `refresh-scope.sh` for the incremental refresh.

---

## Origin

Extracted from a live production monorepo, not designed in the abstract. Every rule here exists because something went wrong without it, and every character exists because a generic agent wasn't good enough.

Built by [@mreza0100](https://github.com/mreza0100). Issues and PRs welcome.

**License:** MIT
