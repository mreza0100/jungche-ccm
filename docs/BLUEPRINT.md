# BLUEPRINT — The Philosophy

The discipline + character of the pipeline. Read this before installing it.

> **Personality is load-bearing.** Strip the Professor's voice and you have a Confluence wiki. Strip JC's panic energy and the hotfix command becomes a checklist. Strip Professor's cross-disciplinary depth and the analysis becomes generic. The blueprint is a transplantable nervous system — characters, multi-PhD professor and all — refitted to your domain at install time. It drops into **any Claude Code project at any repo size**: structure is captured at install as a **roster** of 1..N projects, so a single-project repo (roster of one — first-class) and a multi-project monorepo get correctly-sized files from the same templates.

---

## The three-tier framework

Every command, agent, and rule sorts into one of three tiers:

| Tier                         | Description                                                             | What ships                                                             | What gets parameterized                                                                           |
| ---------------------------- | ----------------------------------------------------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| **A — Universal archetypes** | Personalities that work in any domain. The voice IS the value.          | Full character, voice, structure, signature traits, archetype identity | Domain-specific REFERENCES inside the character (Professor's PhDs, JC's example stack traces)     |
| **B — Domain archetypes**    | Roles every serious project needs, but content is heavily domain-shaped | Archetype skeleton: identity, voice, charter, mode list, doc structure | Regulation name, knowledge domain, user persona, market segment — filled at install via interview |
| **C — Pure mechanics**       | Infrastructure agents and pipeline plumbing                             | Mechanics only — no character needed                                   | Tech-specific commands (test runner, package manager, build tool)                                 |

### The cast (Tier A — universal)

- **The Professor** — Grandfatherly polymath with 10+ PhDs. Warm, precise, gently devastating. The orchestrator voice and root persona. Lives in CLAUDE.md — NOT a separate command. Disciplines parameterize per project.
- **/jc** — "JESUS CHRIST production is on fire" panic-debug mode. Chill on the surface, holy at the core. The one command allowed to edit `main` directly.
- **/pcm** — meta-engineer that edits the pipeline at the source. Surgery, not journaling. `pcm audit [scope]` (`agents`, `commands`, `skills`, `pipeline`, `scripts`, `structure`, `cross-refs`, or `all`) walks the pipeline's own files against a checklist per scope; `/pcm:context-meter` audits the framework's own context budget.
- **/wave:{orchestrator,builder,refine,walker,live,ccc}, /jc, /dev, /git, /documenter, /goal-manager, /p:slow-burn, /sleep** — pipeline mechanics with light Professor voice in their reports. `/chat:*` is the same tier but installs host-level (`~/.claude/commands/chat/`, opt-in) from the self-contained `pfm` binary alongside `/reload`.

> Each Tier A persona (Professor, JC, Dr. House) ships in two selectable depths — **full** (rich, showcase voice) and **compact** (lean voice plus the same Verdict / sacred-ground / Analysis-Protocol contract, fewer tokens every turn) — chosen at install.

**Bundled commands (ship with the blueprint):**

- **the framework bus** — `/pcm:update` consumes upstream releases, `/pcm:release` regenerates + publishes the blueprint.
- **/wave:refine** — wave task refinement into a zero-gap spec.
- **/wave:walker** — merge-gating end-to-end functional + hygiene walk, run against the merge candidate before the merge it can condemn.
- **/wave:ccc** — the Control & Command Center: the standing command seat over a running train. Full audit from ground truth on arrival, then holds command until the train closes — verifies claims against the tree, rules scope-allocation escalations, dispatches through the orchestrator.
- **/p:360** — exhaustive multi-angle analysis. Two domains: `test` (10 failure dimensions for QA) and `inquiry` (9 question dimensions for Professor). Ships as a portable command (`blueprint/commands/p/360.md`) — not source-fetched.
- **/p:rnd** — goal-driven iterative research-and-develop loop.
- **/p:tokens** — per-agent/per-workflow token spend attribution parsed from local transcripts, ranked by estimated cost.
- **/quality:doc** / **/quality:prompt** — doc-shaping and prompt-quality gates.
- **/audit:code-hygiene** / **/audit:security** / **/audit:ai-output** — code-hygiene, security, and AI-output audit scopes, each carrying their own 360-sweep pre-step. Code-hygiene additionally has a Sweep Mode (`code-hygiene sweep`) that promotes a report-only run to actively removing confirmed-dead code and unused dependencies, end-to-end behind QA.
- **/qa:live** — live end-to-end QA of the running app on the dev stack: no mocks, no seeded data, judgment-based rather than regression assertions.

**Source-fetched skills (installed at setup from canonical public repos via `blueprint/skills/sources.json`, never vendored):**

- **rr** — Research-and-Report protocol.
- **ghostwriter** — captures a writer's mechanical fingerprint and generates in that voice.
- **vision-factory** — forge, validate, and stress-test a startup vision.

**Bundled skills (ship with the blueprint):**

- **legal** — an attributed reference shelf for the Professor and `/officer`: DPA and DPIA drafting, breach response, privacy notices, vendor due diligence, NDA/risk triage, and a pre-delivery self-check.

### The optional cast (Tier B — opt-in at install)

- **/officer** — compliance enforcer. Pick your regulation(s). (GDPR, HIPAA, FDA, SOC2, ISO 27001, MiFID, none.)
- **/km** — knowledge curator. Pick your knowledge domain.
- **/pm** — user+product hybrid. Pick your user persona.
- **/mentor** — business advisor. Pick your market + jurisdiction.
- **/marketer** — visibility strategist. Pick your channels + language.

### The plumbing (Tier C — invisible)

- `mono-planner`, `mono-architect`, `mono-documenter`, `gitter`, `tracer`, `scheduler`, `architect`, and one `{role}-{project}` wrapper per roster entry per role (the `qa-{project}` gates among them) — root agents. Role-defined, not character-defined.
- `worktree.sh`, `alloc-ports.sh`, `dev.sh`, `notify.sh` — scripts.
- `pfm statusline` — native status bar with model, fleet counts, context, git, cost, spend, and rate limits. Wired in the host settings by `pfm install`.
- `settings-global.json` — a handful of keys merged (never overwritten) into the adopter's own `~/.claude/settings.json`. Currently one key, `cleanupPeriodDays: 36500`, which turns off Claude Code's default 30-day auto-delete of session transcripts and orphaned git worktrees.
- `vscode/` — VSCode tmux launcher: new terminals open into tmux + Claude, `/exit` → shell. Ships a companion `tmux.conf` (mouse + clipboard). Opt-in; edits user `settings.json` + shell rc + `~/.tmux.conf`.
- Per-project agents (`planner`, `architect`, `developer`, `qa`) — role-defined.

---

## The five load-bearing walls

Touch anything else, but leave these five alone. They are non-negotiable:

### 1. Only `gitter` touches git

The `gitter` agent is the **single git operator**. No other agent runs `git add`, `git commit`, `git merge`, `git push`. This isn't bureaucracy — it's safety:

- Centralizes destructive operations behind one well-tested code path.
- Prevents agents from racing each other for the merge.
- Makes "what got committed" auditable.

If an agent needs to commit, it asks gitter. Gitter has phases: SETUP, MERGE, DOCS-COMMIT, JC-COMMIT, PUSH, PULL.

### 2. QA gates the merge

The pipeline runs QA on the worktree branch BEFORE merging to main. Test failures block the merge. Then it runs **post-merge QA on main** to verify the merge didn't break anything. Zero tolerance for "pre-existing failures" — if a test was broken before your pipeline, your pipeline fixes it. Every pipeline leaves main cleaner than it found it.

### 3. Path variables, not hardcoded paths

Agents receive paths as variables:

| Variable     | Purpose                            | Example                                  |
| ------------ | ---------------------------------- | ---------------------------------------- |
| `$PIPELINE`  | Pipeline name (kebab-case, unique) | `{some-feature}`                         |
| `$DOCS`      | Pipeline docs from repo root       | `docs/dev/tasks/{some-feature}`          |
| `$DOCS_REL`  | Pipeline docs from worktree        | `../../../docs/dev/tasks/{some-feature}` |
| `$WORKTREE`  | Worktree directory                 | `.worktrees/{some-feature}`              |
| `$ARCHIVE`   | Archive parent                     | `docs/dev/tasks/archive`                 |
| `$CDOCS`     | Command-owned docs root            | `docs/commands`                          |
| `$REFS`      | Reference docs subdir              | `references`                             |
| `$RESEARCH`  | Research docs subdir               | `research`                               |
| `$RESOURCES` | Static resources subdir            | `resources`                              |

Agents NEVER hardcode `docs/dev/tasks/...` — they use what `/wave:builder` passes them. Path conventions can change without rewriting every agent.

### 4. Worktree isolation per pipeline

Every `/wave:builder` invocation creates:

- A git branch: `pipeline/{name}`
- A worktree checkout: `.worktrees/{name}/` (full repo)
- A unique port allocation (whatever ports your stack needs)
- Pipeline docs: `docs/dev/tasks/{name}/`

This means you can run **multiple pipelines in parallel on the same machine** without port collisions or git state corruption. When the pipeline completes, gitter merges to main, the worktree is removed, and the docs are archived.

### 5. Self-improvement at the source

When something goes wrong in the pipeline, you don't write a "lesson" file. You invoke `/pcm` (the meta-agent that owns the pipeline itself). It edits the actual agent definition or command instructions to prevent the bug class going forward. **Surgery at the source.** Pipeline files are meant to evolve.

---

## The non-negotiable rules baked into every install

These rules appear in `CLAUDE.md` and are referenced by every agent. They are the contract:

1. **No code on main except gitter merges and `/jc` commits.**
2. **Only gitter runs git commands.**
3. **Never commit broken code** — QA must pass first.
4. **Never merge before QA passes** — both pre-merge and post-merge.
5. **Never reuse pipeline names** — check `docs/dev/tasks/`, `docs/dev/tasks/archive/`, `.worktrees/` first.
6. **Never run destructive git commands** — no `--force`, no `reset --hard`, no `clean -fdx` without explicit user approval.
7. **Never swallow exceptions silently** — every catch logs the full traceback. Silent failures hide bugs.
8. **No mocking internal dependencies within 1 hop** — mock only external services (paid APIs, third-party SaaS, anything flaky and outside your trust boundary). Real DB, real queue, real internal services.
9. **All failing tests are blocking** — no "pre-existing failure" excuse.
10. **All infrastructure ops go through a single project-owned script** — never reach around it directly from agent code.

---

## Pipeline architecture

```
                              ┌─────────────────┐
                              │  /wave:builder  │
                              └────────┬────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  child planners     │ (parallel — one per affected project)
                          │  analyze codebase   │
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  mono-planner       │ → docs/dev/tasks/{name}/1-plan.md
                          │  consolidates plan  │
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  gitter SETUP       │ → worktree, branch, ports
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  mono-architect     │ → 3-architecture.md
                          │  cross-project      │   (contracts, shared types, inline research)
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  child architects   │ (parallel — per project)
                          │  + library research │
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  child developers   │ (parallel — implements code)
                          │  + happy-path tests │
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  child QAs          │ (parallel — adversarial tests)
                          │  + bug reports      │
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  fix loop           │ (developer fixes QA bugs;
                          │                     │   capped iterations, hard timeouts)
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  gitter MERGE       │ → squash to main
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  POST-MERGE QA      │ (run on main, catches merge bugs)
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  /audit:* + /officer   │ (parallel — code audit + compliance audit)
                          │  (officer optional) │   if /officer is opted in
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  mono-documenter    │ → updates permanent docs
                          │                     │   archives pipeline dir
                          └──────────┬──────────┘
                                     ▼
                          ┌─────────────────────┐
                          │  gitter DOCS-COMMIT │
                          └─────────────────────┘
```

Hotfix path: `/jc {bug}` → locate → diagnose → fix → test → gitter JC-COMMIT. Same safety, less ceremony.
Meta path: `/pcm {request}` → edits the agent definitions at the source.

---

## File layout (what you end up with after install)

```
your-project/
├── CLAUDE.md                          ← root rules + Professor persona (the nervous system's brain)
├── AGENTS.md                          ← (OPTIONAL) COMPILED from CLAUDE.md by `scripts/build-codex.mjs` (model aliases swapped to Codex names, a Codex-adapter section appended; Codex reads this by convention)
├── .professor/
│   ├── VERSION                        ← installed blueprint version (e.g., vX.Y.Z)
│   ├── manifest.json                  ← interview answers + file hashes (machine-readable replay seed)
│   ├── drift.md                       ← local customizations the merge keeps (human-readable)
│   └── release.md                     ← framework changes pending upstream sync
├── .claude/
│   ├── agents/                        ← root agents (mono-planner, mono-architect, mono-documenter, gitter, tracer, scheduler, architect, {role}-{project} wrappers)
│   ├── commands/                      ← /wave:{orchestrator,builder,refine,walker,live,ccc}, /jc, /pcm:{update,release,context-meter}, /dev, /git, /documenter, /qa:live, /audit:{code-hygiene,security,ai-output}, /quality:{prompt,doc}, /p:{rnd,360,slow-burn,tokens}, /goal-manager, /sleep, /animate + opt-in Tier B (`/chat:*` is NOT here — `pfm install` installs it host-level)
│   ├── output-styles/                 ← persona registry (Professor session style + per-command overlays)
│   ├── scripts/                       ← worktree.sh, alloc-ports.sh, dev.sh, notify.sh, format-md.sh, filter-test-output.sh, checkpoint.sh, git-lock.sh, guard-stamp.sh, drain-wait.sh, limits-hook.sh
│   ├── workflows/                     ← project-local Workflow scripts such as documenter-fanout and audit-ai-output-sessions; Wave Walker runs from the permanent Professor clone
│   ├── skills/                        ← bundled legal shelf + source-fetched sources.json skills (rr, ghostwriter, vision-factory); reasoning protocols ship as nested commands under commands/
│   └── settings.json                  ← permissions, env vars, hooks (notify, formatter, statusline)
├── .codex/                            ← (OPTIONAL) pointer layer over .claude/ — never a restatement of it
│   ├── config.toml                    ← sandbox reach + the {CODEX_MODEL}/{CODEX_REASONING_EFFORT} pins
│   ├── rules/                         ← repo-law.rules — execpolicy door lock (gitter never registered)
│   ├── agents/                        ← mono-documenter.toml + per-project/{developer,qa}.toml — INERT registry, ships forward-compatible
│   └── skills/                        ← GENERATED by `scripts/build-codex.mjs` (symlinks + SKILL.md pointers) + hand-written wave-builder/chat cards
├── {project-a}/                       ← first subproject (you name it)
│   ├── CLAUDE.md                      ← project-specific rules
│   └── .claude/agents/                ← project agents (planner, architect, developer, qa)
├── {project-b}/                       ← second subproject
│   ├── CLAUDE.md
│   └── .claude/agents/
├── docs/
│   ├── agents/                        ← cross-project permanent docs (architecture, API, map, features)
│   ├── commands/{cmd}/                ← command-owned docs ($CDOCS root)
│   │   ├── references/                ← must-know
│   │   ├── research/                  ← looked-up material
│   │   └── resources/                 ← static assets
│   └── dev/
│       ├── tasks/{pipeline}/          ← temp pipeline docs
│       ├── tasks/archive/             ← completed pipelines
│       └── waves/                     ← wave runner artifacts
└── .worktrees/                        ← git worktree checkouts (gitignored)
    ├── {pipeline}/                    ← per-pipeline checkout
    └── .ports                         ← port allocation registry
```

For a single-project repo, drop the `{project-a}/`, `{project-b}/` layer — agents live in `.claude/agents/` only, no child CLAUDE.md files.

---

## What you get out of the box

A `.claude/` infrastructure — a **transplantable nervous system** — that turns Claude Code from "an AI that writes code when you ask" into **a self-disciplined engineering team with character**. Built by the Professor (the grandfatherly polymath behind the glass).

- **Worktree isolation** — every feature gets its own git worktree branch + a unique port allocation. Multiple parallel pipelines on the same repo without collisions.
- **A pipeline that refuses cowboy coding** — `planner → architect → developer → QA → merge`. QA gates block bad code from reaching `main`.
- **One agent owns git** — only `gitter` runs `git add` / `commit` / `merge`. Centralized, auditable, safe.
- **Hotfix mode** — `/jc` lets you bypass the full pipeline for surgical bug fixes, but still routes through tests + gitter.
- **Cross-disciplinary analysis** — the Professor brings 10+ PhDs of your choice to bear on architecture, design, and safety/correctness questions. The Analysis Protocol lives in the active persona (`.claude/output-styles/professor.md`).
- **Self-improvement** — `/pcm` is the meta-agent that edits its own pipeline rules at the source.
- **Optional dual-runtime** — Codex (OpenAI) can mirror the Claude pipeline as a cheaper implementation layer. Same manuals, different runtime. Everything works without it.
- **Path conventions that scale** — `$DOCS`, `$WORKTREE`, `$CDOCS` so agents never hardcode paths.
- **Documentation discipline** — pipeline docs are temporary and archived; only one agent (`mono-documenter`) writes to permanent project docs.
- **Memory backup (opt-in)** — a `SessionEnd` hook auto-syncs Claude's persistent project memory to a private repo, so a machine wipe doesn't lose what Claude learned. Plain git, zero tokens. See `references/memory-backup.md`.

---

## What you adapt vs. what you keep

**Keep verbatim:**

- The `gitter` agent (with project list adjusted at install)
- The `worktree.sh` and `alloc-ports.sh` scripts (with port ranges adjusted)
- The pipeline flow in `/wave:builder`
- The path variable conventions
- The five load-bearing walls
- The non-negotiable rules
- **The character voices** — the Professor's grandfatherly precision, JC's panic energy, Professor's cross-disciplinary structure, the Tier B archetype identities

**Adapt at install (via the SETUP interview):**

- Project name + project list (your subprojects)
- Tech stack descriptions in each project's `CLAUDE.md`
- Test / lint / typecheck / build commands the agents run
- Port ranges (whatever's free on your machine)
- Professor's 10+ PhD disciplines (matched to your domain)
- The project roster (your 1..N projects — directories, stacks, package managers, test runners, ports)
- Tier B opt-ins (which optional archetypes you enable — regulation, knowledge domain, user persona, market segment)
- The character name (default: "Professor") if you want a different persona

See `SETUP.md` for the install interview and adaptation guidance.

---

## Optional: Codex dual-runtime

Professor's nervous system can optionally span **two AI runtimes**: Claude Code (Anthropic) and Codex (OpenAI). Everything works with Claude alone — Codex adds a cheaper implementation layer.

**How it works:**

- `.claude/` is always the source of truth — command manuals, agent definitions, scripts, shared skills
- `.codex/` is a **pointer layer** over `.claude/`, never a restatement of it — `scripts/build-codex.mjs` compiles it: `.codex/skills/` gets a true directory symlink for each `.claude/skills/*` skill and a generated `SKILL.md` pointer for each single-file `.claude/commands/**/*.md` command (nested names flatten — `/wave:orchestrator` → `wave-orchestrator`); `AGENTS.md` is compiled from `CLAUDE.md` (model aliases swapped to Codex equivalents, output-style-adoption pointers stripped, a Codex-adapter section appended — not a symlink); `.codex/agents/*.toml` is generated from `.claude/agents/*.md`; the `.mcp.json` server list mirrors into `.codex/config.toml` as a managed fence. Every generated file carries a marker line — only marker-carrying output is ever overwritten or deleted; anything else at a managed path is a CONFLICT, reported and left untouched. `generate` writes, `check` reports MISSING/STALE/ORPHAN/CONFLICT and writes nothing. Declared NOT covered: per-project `.claude/skills`, output-styles, workflows, hooks (Claude-harness-only), `$HOME`-level MCP registries.
- `scripts/codex-sync.sh` keeps the mirror deterministic: a `PostToolUse` hook flags the repo dirty on any Edit/Write to a Claude source (`.claude/**`, any `CLAUDE.md`, `$HOME/.claude/commands/**`); a `Stop` hook then re-runs `build-codex.mjs generate` + `check` before the turn ends — a mirror that fails to compile blocks the turn rather than shipping broken. (A raw Bash write bypasses the hook; `pcm audit structure` is the backstop.)
- `.codex/agents/*.toml` (`mono-documenter.toml` + the `per-project/{developer,qa}.toml` pattern) is a role registry that is INERT at the probed Codex version — `spawn_agent` has no agent-selection parameter — and ships forward-compatible anyway; `.codex/rules/repo-law.rules` is the execpolicy door lock
- Claude and Codex mirror the same Professor contract. The pointer layer translates mechanics (slash commands, agent spawning), never identity or protocol. Git stays gitter's alone: a Codex role gets read-only git (`status`/`log`/`diff`/`show`) and nothing more — there is no `gitter.toml`, and there must not be one.

**Division of labor:**

| Task                             | Runtime                | Why                                                                                             |
| -------------------------------- | ---------------------- | ----------------------------------------------------------------------------------------------- |
| Planning, architecture, research | Claude                 | Judgment-heavy, low token volume                                                                |
| Heavy implementation             | Codex                  | Cheaper per token                                                                               |
| QA / adversarial tests           | Claude                 | Codex shouldn't grade itself                                                                    |
| Git operations                   | Claude (`gitter`) only | Codex gets read-only git (status/log/diff/show); commits, merges, and pushes are gitter's alone |

**Opting in:** the installer asks at Batch 6 Q15b. If yes, it creates `.codex/` (`config.toml`, `rules/repo-law.rules`, the `agents/` registry), runs `scripts/build-codex.mjs generate` to compile `.codex/skills/`, `.codex/agents/*.toml`, and `AGENTS.md`, and wires `scripts/codex-sync.sh` into the hooks so every later framework edit re-compiles before its turn ends. If no, the entire layer is skipped. No pipeline operation requires Codex.

See `blueprint/codex/README.md` for the full integration guide.

---

## Staying current — the update mechanism

The blueprint evolves. Releases ship as **git tags** (`vX.Y.Z`) on `mreza0100/professor`. Adopters don't track `main` — they hop between tagged releases via `/pcm update`.

### How it works

At install time, SETUP.md creates a `.professor/` directory at the repo root containing:

1. **`VERSION`** — the release tag you installed from
2. **`manifest.json`** — all interview answers (replay seed) + SHA-256 hashes of every Professor-owned file
3. **`drift.md`** — local customizations the merge keeps (what makes your install different from vanilla Professor)
4. **`release.md`** — framework changes pending upstream sync; `/pcm:release` consumes and clears it

When you run `/pcm update`, the update protocol:

1. Fetches available git tags from the public repo
2. Clones the target tag into temp
3. **Replays your interview answers** against the new templates (same substitution as install)
4. **Three-way hash comparison** per file: installed baseline vs. current on-disk vs. re-parameterized upstream
5. Classifies changes into three buckets: **auto-apply** (upstream changed, you didn't touch), **review** (conflict or character change), **manual** (breaking migration, new interview questions)
6. Applies accepted changes, regenerates the manifest
7. Appends to `drift.md` — records version jump, which files you kept over upstream, new opt-ins

The interview manifest is the key innovation — it means updates don't require re-answering the install interview. Your answers are the replay seed. Only genuinely new questions (new template placeholders) trigger a mini re-interview.

### Version semantics

| Bump      | Adopter impact                                         |
| --------- | ------------------------------------------------------ |
| **Patch** | Bug fixes — mostly auto-apply                          |
| **Minor** | New features/commands — mix of auto + interactive      |
| **Major** | Breaking changes — full walkthrough, no silent applies |

### What it never overwrites silently

- Your persona section in CLAUDE.md (you may have evolved the voice intentionally)
- Files you've customized post-install (detected via hash mismatch)
- Command-owned docs under `docs/commands/` (your content, not templates)
- `.claude/settings.json` (hand-curated per project)

See `SETUP.md` § "Staying current" for user-facing docs. See `blueprint/commands/pcm/update.md` for the full implementation.

---

## The smell test

**Could a neuropsychology lab, a tabletop RPG studio, and a SCADA controls team all read this blueprint and see _their version of the Professor, JC, and the audit cast_ — same archetypes, different content?**

If yes, the blueprint is right.
If anyone has to delete personality before using it, the blueprint failed.

The mechanics survive every stack. The characters' voices survive every domain. Personality is not decoration — it's load-bearing. If you find yourself stripping voice to "make it generic," stop and parameterize the content instead.
