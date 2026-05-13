# BLUEPRINT — The Philosophy

The discipline + character of the pipeline. Read this before installing it.

> **Personality is load-bearing.** Strip the Professor's voice and you have a Confluence wiki. Strip JC's panic energy and the hotfix command becomes a checklist. Strip Professor's cross-disciplinary depth and the analysis becomes generic. The blueprint is a transplantable nervous system — characters, councils, multi-PhD professor and all — refitted to your domain at install time.

---

## The three-tier framework

Every command, agent, and rule sorts into one of three tiers:

| Tier | Description | What ships | What gets parameterized |
|------|-------------|------------|-------------------------|
| **A — Universal archetypes** | Personalities that work in any domain. The voice IS the value. | Full character, voice, structure, signature traits, archetype identity | Domain-specific REFERENCES inside the character (Professor's PhDs, Council panel composition, JC's example stack traces) |
| **B — Domain archetypes** | Roles every serious project needs, but content is heavily domain-shaped | Archetype skeleton: identity, voice, charter, mode list, doc structure | Regulation name, knowledge domain, user persona, market segment — filled at install via interview |
| **C — Pure mechanics** | Infrastructure agents and pipeline plumbing | Mechanics only — no character needed | Tech-specific commands (test runner, package manager, build tool) |

### The cast (Tier A — universal)

- **The Professor** — Grandfatherly polymath with 10+ PhDs. Warm, precise, gently devastating. The orchestrator voice and root persona. Lives in CLAUDE.md — NOT a separate command. Disciplines parameterize per project.
- **/jc** — "JESUS CHRIST production is on fire" panic-debug mode. Chill on the surface, holy at the core. The one command allowed to edit `main` directly.
- **/council** — roundtable debate, three rounds: opening, rebuttal, verdict. Panel composition adapts to the archetypes you opt into.
- **/pcm** — meta-engineer that edits the pipeline at the source. Surgery, not journaling.
- **/audit** — code auditor. 8 categories of hygiene + 9 of security.
- **/build, /jc, /dev, /git, /wave, /documenter** — pipeline mechanics with light Professor voice in their reports.
- **360°** — exhaustive multi-angle analysis skill. Two domains: `test` (10 failure dimensions for QA) and `inquiry` (9 question dimensions for Professor). The blind-spot killer — forces systematic coverage before creative analysis.

### The optional cast (Tier B — opt-in at install)

- **/officer** — compliance enforcer. Pick your regulation(s). (GDPR, HIPAA, FDA, SOC2, ISO 27001, MiFID, none.)
- **/km** — knowledge curator. Pick your knowledge domain.
- **/pm** — user+product hybrid. Pick your user persona.
- **/mentor** — business advisor. Pick your market + jurisdiction.
- **/marketer** — visibility strategist. Pick your channels + language.

### The plumbing (Tier C — invisible)

- `mono-planner`, `mono-architect`, `mono-documenter`, `gitter` — root agents. Role-defined, not character-defined.
- `worktree.sh`, `alloc-ports.sh`, `dev.sh`, `notify.sh` — scripts.
- `statusline-command.sh` — two-line status bar (model, context, git, cost, rate limits). Installed to `~/.claude/`.
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

| Variable | Purpose | Example |
|----------|---------|---------|
| `$PIPELINE` | Pipeline name (kebab-case, unique) | `{some-feature}` |
| `$DOCS` | Pipeline docs from repo root | `docs/dev/tasks/{some-feature}` |
| `$DOCS_REL` | Pipeline docs from worktree | `../../../docs/dev/tasks/{some-feature}` |
| `$WORKTREE` | Worktree directory | `.worktrees/{some-feature}` |
| `$ARCHIVE` | Archive parent | `docs/dev/tasks/archive` |
| `$CDOCS` | Command-owned docs root | `docs/commands` |
| `$REFS` | Reference docs subdir | `references` |
| `$RESEARCH` | Research docs subdir | `research` |
| `$RESOURCES` | Static resources subdir | `resources` |

Agents NEVER hardcode `docs/dev/tasks/...` — they use what `/build` passes them. Path conventions can change without rewriting every agent.

### 4. Worktree isolation per pipeline

Every `/build` invocation creates:
- A git branch: `pipeline/{name}`
- A worktree checkout: `.worktrees/{name}/` (full repo)
- A unique port allocation (whatever ports your stack needs)
- Pipeline docs: `docs/dev/tasks/{name}/`

This means you can run **multiple pipelines in parallel on the same machine** without port collisions or git state corruption. When the pipeline completes, gitter merges to main, the worktree is removed, and the docs are archived.

### 5. Self-improvement at the source

When something goes wrong in the pipeline, you don't write a "lesson" file. You invoke `/pcm` (the meta-agent that owns the pipeline itself). JM edits the actual agent definition or command instructions to prevent the bug class going forward. **Surgery at the source.** Pipeline files are meant to evolve.

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
                              ┌──────────────┐
                              │  /build req  │
                              └──────┬───────┘
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
                          │  /audit + /officer     │ (parallel — code audit + compliance audit)
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
├── AGENTS.md                          ← (OPTIONAL) symlink → CLAUDE.md (Codex reads this)
├── .claude/
│   ├── agents/                        ← root agents (mono-planner, mono-architect, gitter, mono-documenter)
│   ├── commands/                      ← /build, /jc, /pcm, /dev, /git, /wave, /documenter, /council, /audit + opt-in Tier B
│   ├── scripts/                       ← worktree.sh, alloc-ports.sh, dev.sh, notify.sh
│   ├── skills/                        ← cloned from public repos (rr, 360, ghostwriter) + bundled (rnd)
│   └── settings.json                  ← permissions, env vars, hooks (notify.sh + statusline wired here)
├── .codex/                            ← (OPTIONAL) Codex runtime — .toml wrappers pointing to .claude/ manuals
│   ├── config.toml
│   ├── agents/                        ← command wrappers + role agent wrappers
│   └── skills/                        ← skill wrappers mirroring commands
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
- **A debating council** — `/council` runs five archetypes (Tier A + opted-in Tier B) in parallel on a topic, three rounds, then synthesizes a verdict.
- **Cross-disciplinary analysis — the Professor brings 10+ PhDs of your choice to bear on architecture, design, and safety/correctness questions.
- **Self-improvement** — `/pcm` is the meta-agent that edits its own pipeline rules at the source.
- **Optional dual-runtime** — Codex (OpenAI) can mirror the Claude pipeline as a cheaper implementation layer. Same manuals, different runtime. Everything works without it.
- **Path conventions that scale** — `$DOCS`, `$WORKTREE`, `$CDOCS` so agents never hardcode paths.
- **Documentation discipline** — pipeline docs are temporary and archived; only one agent (`mono-documenter`) writes to permanent project docs.

---

## What you adapt vs. what you keep

**Keep verbatim:**
- The `gitter` agent (with project list adjusted at install)
- The `worktree.sh` and `alloc-ports.sh` scripts (with port ranges adjusted)
- The pipeline flow in `/build`
- The path variable conventions
- The five load-bearing walls
- The non-negotiable rules
- **The character voices** — the Professor's grandfatherly precision, JC's panic energy, Professor's cross-disciplinary structure, Council's debate format, the Tier B archetype identities

**Adapt at install (via the SETUP interview):**
- Project name + project list (your subprojects)
- Tech stack descriptions in each project's `CLAUDE.md`
- Test / lint / typecheck / build commands the agents run
- Port ranges (whatever's free on your machine)
- Professor's 10+ PhD disciplines (matched to your domain)
- Council panel composition (which Tier B archetypes you opt in)
- Tier B opt-ins (regulation, knowledge domain, user persona, market segment)
- The character name (default: "Professor") if you want a different persona

See `ADAPTATION.md` for archetype-by-archetype customization. See `ARCHETYPES.md` for the full cast and adaptation examples. See `SETUP.md` for the install interview.

---

## Optional: Codex dual-runtime

Professor's nervous system can optionally span **two AI runtimes**: Claude Code (Anthropic) and Codex (OpenAI). Everything works with Claude alone — Codex adds a cheaper implementation layer.

**How it works:**
- `.claude/` is always the source of truth — command manuals, agent definitions, scripts
- `.codex/` is a config layer: `.toml` wrappers that point to the same `.claude/commands/*.md` and `.claude/agents/*.md` manuals
- `AGENTS.md` is a symlink → `CLAUDE.md` (Codex reads `AGENTS.md` by convention)
- Claude orchestrates, plans, and does QA. Codex implements. When Codex runs a full pipeline (`$build`), it handles git work inline per `gitter.md`.

**Division of labor:**

| Task | Runtime | Why |
|------|---------|-----|
| Planning, architecture, research | Claude | Judgment-heavy, low token volume |
| Heavy implementation | Codex | Cheaper per token |
| QA / adversarial tests | Claude | Codex shouldn't grade itself |
| Git operations | Either | Whoever orchestrates owns git for that run |

**Opting in:** the installer asks at Batch 6 Q15b. If yes, it creates `.codex/`, `AGENTS.md`, and all `.toml` wrappers. If no, the entire layer is skipped. No pipeline operation requires Codex.

See `templates/codex/README.md` for the full integration guide.

---

## The smell test

**Could a neuropsychology lab, a tabletop RPG studio, and a SCADA controls team all read this blueprint and see *their version of the Professor, JC, and Council* — same archetypes, different content?**

If yes, the blueprint is right.
If anyone has to delete personality before using it, the blueprint failed.

The mechanics survive every stack. The characters' voices survive every domain. Personality is not decoration — it's load-bearing. If you find yourself stripping voice to "make it generic," stop and parameterize the content instead.
