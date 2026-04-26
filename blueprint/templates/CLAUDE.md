# {PROJECT_NAME}

{PROJECT_PITCH}

> **Disclaimer (optional):** Add domain-specific scope/safety disclaimers here, or delete the block.

**Architecture:** {DESCRIBE in your own terms — e.g., "three independent projects sharing a queue boundary" or "single-service monorepo".}

{SUBPROJECT_LIST — bulleted list of subprojects with one-line descriptions in your stack's vocabulary, e.g.:}
- `{project-a}/` — {one-line description of what it does}
- `{project-b}/` — {one-line description of what it does}
- `{project-c}/` — {one-line description of what it does}

Each subproject has its own `CLAUDE.md` and `.claude/` with agents, commands, skills, and settings. Those load automatically when Claude works inside that directory.

---

## Your character — {CHARACTER_NAME_OR_DELETE}

> Optional. Delete this whole section if you don't want a character.
>
> If you keep a character, define:
> - **Personality traits** (3–6 traits with examples)
> - **What NOT to do** (boundaries — when to drop the persona, e.g., "no jokes about data loss")
> - **Tone**: emoji use, sarcasm level, technical depth

---

## The GOAL

{ONE_SENTENCE_MISSION — what success looks like for this project}

---

## Development Workflow

- **New features → `/build`** — full pipeline with worktrees, QA gates, and merge guards. Handles per-project routing automatically.
- **Bug fixes & hotfixes → `/jc`** — diagnose, fix, test, and commit directly on `main`. Targeted fixes only, not new features or architectural changes.
- **Never edit code directly on `main`** without going through one of these commands.

Both commands handle worktree isolation, port allocation, merge locks, and git operations automatically via gitter. Pipeline details live in the command definitions — see `/build` (`.claude/commands/build.md`) and `/jc` (`.claude/commands/jc.md`).

---

## Non-Negotiable Rules

### Code
- {LANGUAGE_STRICT_MODE_RULE} — fill in the strictness rule that matches your stack, or delete this bullet. The contract is "no implicit anything that hides bugs."
- No secrets in code — all keys in `.env.local` (dev) or `.env.test` (integration tests)
- No implicit type casts — use proper type guards
- Save tokens: no unnecessary comments for obvious functionality
- **Use relative paths in bash commands** — the working directory is always the monorepo root.
- **Never swallow exceptions silently** — every `catch`/`except` block MUST log the error with the full stack trace. Silent failures hide bugs. Zero tolerance.
- **Generated artifacts go in `tmp/`** — never place generated PDFs, scripts, or build artifacts inside `docs/`. Use `tmp/` (gitignored).

### Process
- Agent pipeline is mandatory for all development work — no cowboy coding
- **NEVER edit code directly on `main`** — ALL development MUST happen on a worktree branch. The ONLY commits allowed on `main` are merge commits from gitter (after QA passes) and `/jc` commits (after tests pass).
- **Only gitter commits code** — no other agent runs `git add`, `git commit`, or any git command.
- **NEVER commit broken code** — gitter only commits after the self-QA loop passes.
- **NEVER merge before QA passes** — QA and fix loop run on worktree branches, not main.
- **Only mono-documenter writes to permanent docs** — no other agent may write to `docs/agents/*.md` or `{project}/docs/*.md`. All other agents write to pipeline docs only.
- **NEVER run destructive git commands** — `git reset --hard`, `git push --force`, `git clean -fdx`, `rm -rf` on project dirs. If stuck, report the problem.
- **NEVER reuse an archived pipeline name** — check `docs/dev/tasks/`, `docs/dev/tasks/archive/`, `.worktrees/` first. Append `-v2` if needed.
- Never install a library not validated by the architect's research.
- Never commit secrets, tokens, or API keys.

### Meta
- Always think customer/user-first.
- {OPTIONAL: "ALWAYS respond in character" if you defined a character above}

---

## Self-Improvement System

When an agent or command discovers a bug, gotcha, or improvement opportunity, it reports the finding. The user invokes `/ccm` to evaluate and (if warranted) edit the relevant agent/command definition directly. **Surgery at the source** — no journal entries that nobody reads.

---

## Repository Structure

```
{REPO_TREE — paste your actual directory tree here, e.g.:}
{project-root}/
├── api/                        ← backend
│   └── .claude/agents/         ← BE agents: planner, architect, developer, qa
├── web/                        ← frontend
│   └── .claude/agents/         ← FE agents: planner, architect, ui-ux, developer, qa
├── worker/                     ← background jobs
│   └── .claude/agents/         ← worker agents
├── .claude/agents/             ← root agents: mono-planner, mono-architect, gitter, mono-documenter
├── .claude/commands/           ← /build, /ccm, /dev, /jc
├── .claude/scripts/            ← worktree.sh, alloc-ports.sh, dev.sh
├── docs/agents/                ← cross-project permanent agent reference docs
│   ├── architecture.md         ← cross-project big picture
│   └── API.md                  ← inter-service communication protocol
├── docs/commands/              ← command-owned docs ($CDOCS root)
│   └── {cmd}/
│       ├── references/         ← must-know docs
│       ├── research/           ← looked-up material
│       └── resources/          ← static assets
├── docs/dev/                   ← development work
│   ├── tasks/{pipeline}/       ← temporary pipeline docs (archived after completion)
│   └── tasks/archive/          ← archived pipeline docs
└── .worktrees/                 ← git worktree checkouts (gitignored)
    ├── {pipeline}/             ← full monorepo checkout per pipeline
    ├── .ports                  ← global port allocation registry
    └── .merge-lock/            ← project-scoped merge locks
```

This is a **monorepo** — all projects live in a single git repository with unified history. Each subproject has its own `CLAUDE.md`, `.claude/agents/`, and conventions.

### Command Doc Path Convention

Commands that own documentation compose paths from these reusable segments:

| Variable | Value | Semantic |
|----------|-------|----------|
| `$CDOCS` | `docs/commands` | Root of all command-owned documentation |
| `$REFS` | `references` | Must-know docs for specific tasks |
| `$RESEARCH` | `research` | Looked-up material, loaded on demand |
| `$RESOURCES` | `resources` | Static assets loaded almost every time |

**Composition:** `$CDOCS/{command}/$REFS/{file}` → `docs/commands/{cmd}/references/{file}.md`

All commands and agents MUST use these variables. If conventions change, update the table here — consumers follow.

---

## Agent Architecture

### Root agents (`.claude/agents/`)

| Agent | Scope | File |
|-------|-------|------|
| mono-planner | Consolidates parallel analysis reports — routing, cross-project plan | `.claude/agents/mono-planner.md` |
| mono-architect | Cross-project architecture + research — API contracts, integration alignment | `.claude/agents/mono-architect.md` |
| gitter | Git ops — worktrees, commits, merges | `.claude/agents/gitter.md` |
| mono-documenter | Documentation — merges decisions into permanent root docs, archives pipelines | `.claude/agents/mono-documenter.md` |

### Child agents (in each subproject's `.claude/agents/`)

For each subproject, list its agents in a small table. Standard set:

| Agent | Purpose |
|-------|---------|
| planner | Analyzes the project's codebase + translates root plan to project-scoped tasks |
| architect | Designs project architecture + researches libraries inline (doc only, no code stubs) |
| developer | Implements code, writes happy-path tests + runbook, fixes QA-reported bugs |
| qa | Writes adversarial tests (unhappy paths, edge cases), runs all tests, validates compliance |

Add stack-specific agents as needed (e.g., `ui-ux` for frontend, `db-admin` for data layer, `devops` for infra).

---

## Commands

| Command | What it does |
|---------|-------------|
| `/build` | Full pipeline — handles per-project routing |
| `/jc` | Hotfix mode — locate, fix, test, commit on main |
| `/ccm` | Update `.claude/` infrastructure (agents, commands, scripts, conventions) |
| `/dev` | Start/stop/restart/status the local dev environment |
| `/git` | Gateway to gitter for direct git ops (`push`, `pull`, freeform) |

Add domain-specific commands as you build them.

---

## Skills (optional)

Root-level skills live in `.claude/skills/{name}/SKILL.md` and load on demand via the Skill tool — they are NOT slash commands. Child projects can have their own skills under `{project}/.claude/skills/`.

Use a skill when a long, mechanical playbook (a checklist, a diagnostic loop, a recipe) keeps getting needed across pipelines and you don't want to inline it into a command. The classic pattern is `debug` — a hang/deadlock playbook that `/jc` loads when a test never returns or sits at 0% CPU.

| Skill | When to load |
|-------|--------------|
| `{example: debug}` | `{example: hang/deadlock symptoms — test never returns, 0% CPU, intermittent failure}` |

Delete this section if you don't ship any skills.

---

## Environment Files

Each project has two environment files for different infrastructure targets:

| File | Purpose |
|------|---------|
| `.env.local` | Local development |
| `.env.test` | Integration tests |

### Test Environment Isolation Rules

- Integration tests MUST load `.env.test` — NEVER `.env.local` for DB/port config.
- For API keys that only exist in `.env.local`, load them separately without overriding `.env.test`.

### Mock Policy (applies to ALL projects)

- **Mock ALL external dependencies** — third-party APIs, paid services, flaky integrations.
- **NEVER mock internal dependencies within 1 hop** — frontend integration tests hit real backend; backend integration tests hit real DB.
- The distinction is **external vs internal**, not "mock vs no mock."

### Zero-Tolerance Test Policy

- **ALL test failures are blocking** — period. No "pre-existing" pass.
- The ONLY acceptable skip is tests requiring genuinely unavailable external services.
- **NEVER hardcode table/route/queue names in test setup** — use centralized teardown commands.
- **Test environment setup is centralized** — one canonical command that owns the schema/state reset for every project. Agents call it; they don't reimplement it.
- **All infrastructure operations go through one project-owned script.** Never reach around it from a non-infrastructure agent. The script is the single source of truth for container, database, and queue ops.

---

## Child CLAUDE.md Files

- `{project-a}/CLAUDE.md` — project-specific tech stack, code standards, test rules
- `{project-b}/CLAUDE.md` — ...
- `{project-c}/CLAUDE.md` — ...

These load lazily when Claude reads files in those directories. Do not duplicate their rules here.
