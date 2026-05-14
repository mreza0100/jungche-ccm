# Codex Integration Layer

Optional dual-runtime setup for the Professor pipeline. Everything in `.codex/` is a config layer that points to `.claude/` as the single source of truth. Delete this entire directory and the pipeline runs fine on Claude Code alone.

---

## What this is

OpenAI's Codex CLI mirrors the same Professor pipeline contract that Claude Code runs. The `.codex/` directory contains runtime adapters, not a second personality:

- **`config.toml`** — global Codex settings (personality override, sandbox, Teams enablement)
- **`agents/*.toml`** — wrappers that tell Codex "read this `.claude/` manual and follow it"
- **`skills/`** — interactive `$name` invocations (Codex's equivalent of Claude's `/name` slash commands)
- **`AGENTS.md`** (at repo root) — a symlink to `CLAUDE.md` so Codex reads the same root instructions

Codex never gets its own copy of pipeline logic or identity. Every `.toml` file says "read `.claude/commands/X.md`" or "read `.claude/agents/X.md`" — the markdown manual is always the source of truth. `AGENTS.md` is the same root contract as `CLAUDE.md`.

---

## Why it's optional

Claude Code can run the full pipeline alone: `/build`, `/wave`, `/jc`, all root agents, all child agents, all git operations via gitter. Codex is optional because the pipeline must not require a second runtime, not because Codex is subordinate. When enabled, Codex mirrors the same contract with Codex-specific mechanics.

The value proposition:

| What                                      | Single runtime      | Dual runtime                                        |
| ----------------------------------------- | ------------------- | --------------------------------------------------- |
| Shared contract/persona                   | Professor contract  | Same Professor contract                             |
| Orchestration, planning, architecture     | Current runtime     | Either runtime when invoked                         |
| Implementation (developer/engineer slots) | Current runtime     | Either runtime by assignment                        |
| QA / adversarial tests                    | Current runtime     | Separate assigned QA agent; do not self-grade       |
| Git operations                            | Gitter protocol     | Whoever orchestrates follows the same gitter manual |
| Documentation                             | Documenter protocol | Same documenter protocol                            |

---

## Division of labor

When **one runtime orchestrates**:

- It spawns child agents for every pipeline step
- It follows the same `.claude/commands/*.md` manuals
- It owns git through the same `.claude/agents/gitter.md` protocol

When **Codex orchestrates** (e.g., `$build`, `$wave`):

- Codex reads the same `.claude/commands/*.md` manuals
- Codex spawns child agents via Codex Teams (`Agent(role, "...")`)
- Codex owns git inline (reads `.claude/agents/gitter.md` protocol, executes bash git commands)
- The separate gitter agent is NOT involved — Codex is self-contained

When **one runtime delegates to the other**:

- The delegating runtime writes a scoped, self-contained brief
- The receiving runtime executes within that scope
- QA and git ownership remain explicit in the parent workflow

---

## Setup

### 1. Create AGENTS.md symlink

At your repo root:

```bash
ln -s CLAUDE.md AGENTS.md
```

Codex reads `AGENTS.md` as its project doc. The symlink ensures both runtimes read the same file. In your `CLAUDE.md`, add a dual-runtime section that says both runtimes mirror the same Professor contract and that wrappers translate mechanics only.

### 2. Copy config.toml

```bash
mkdir -p .codex
cp templates/codex/config.toml .codex/config.toml
```

Review and adjust:

- `approval_policy` — `"never"` for full auto, `"on-failure"` for a human gate
- `sandbox_mode` — `"danger-full-access"` for worktrees + infra, `"relaxed"` for simpler setups
- `job_max_runtime_seconds` — increase for complex pipelines
- `[shell_environment_policy.set]` — add project-specific env vars

### 3. Generate .toml wrappers

Create `.codex/agents/` with one `.toml` per command and per role agent. Three types:

#### Type 1: Command wrappers

One per `/command` (build, jc, wave, dev, git, pcm, council, audit, documenter, plus any Tier B commands you opted into).

Pattern:

```toml
name = "{command-name}"
description = "Codex runner for /{command-name} — {one-line purpose}."
nickname_candidates = ["{Name}", "Codex {Name}"]
developer_instructions = """
You are part of the {PROJECT_NAME} dual-runtime team. Read AGENTS.md for shared context...

Read .claude/commands/{command-name}.md in full — it is your complete role manual. Follow it verbatim.

{Codex-specific differences — typically: Skill() calls become Agent() spawns, gitter agent calls become inline bash git commands}
"""
```

See `agents/build.toml` and `agents/jc.toml` for examples.

#### Type 2: Role agent wrappers

One per role per subproject (e.g., `be-developer.toml`, `fe-planner.toml`, `worker-ai-engineer.toml`).

Pattern:

```toml
name = "{prefix}-{role}"
description = "{Role} agent for scoped work under {project-dir}."
nickname_candidates = ["{PREFIX} {Role}"]
developer_instructions = """
You are part of the {PROJECT_NAME} dual-runtime team...

Read `{project-dir}/.claude/agents/{role}.md` as your read-only role manual.
Read `{project-dir}/CLAUDE.md` for project conventions.

Rules:
- Implement only {project}-scoped tasks.
- {project-specific conventions}
- Never run git commands; gitter owns git.
- Complete your scoped task and return.
"""
```

See `agents/developer.toml` for the example.

#### Type 3: Git operator wrapper

Exactly one — `gitter.toml`. Points to `.claude/agents/gitter.md` and defines the spawn protocol (phase-based invocation). See `agents/gitter.toml`.

### 4. Create skills

Skills are Codex's interactive invocation mechanism — the equivalent of Claude's `/command` slash commands. Each skill is a directory under `.codex/skills/{name}/` containing a `SKILL.md` file.

```
.codex/skills/
├── build/SKILL.md      ← $build
├── jc/SKILL.md         ← $jc
├── wave/SKILL.md       ← $wave
├── dev/SKILL.md        ← $dev
├── git/SKILL.md        ← $git
├── professor/SKILL.md  ← $professor
├── council/SKILL.md    ← $council
└── ...                 ← one per command
```

Each `SKILL.md` follows the same pattern:

```markdown
---
name: { command-name }
description: "{one-line description}. Invoked as ${command-name} <args>."
---

Read `.claude/commands/{command-name}.md` in full — it is your complete role manual. Follow it verbatim.

**Argument:** {what the user passes}

## Codex-only differences

- {Skill() → Agent() substitution}
- {gitter agent → inline bash git}
- {any other runtime differences}
```

See `skills/build/SKILL.md` for the example.

### 5. Add research/utility skills (optional)

If you have runtime-agnostic skills in `.claude/skills/` (like `rr`, `rnd`, `360`, `ghostwriter`), symlink them into `.codex/skills/` so Codex can use them too. These skills share the same `SKILL.md` across both runtimes — no Codex-specific wrapper needed:

```bash
ln -s ../../.claude/skills/rr .codex/skills/rr
ln -s ../../.claude/skills/rnd .codex/skills/rnd
ln -s ../../.claude/skills/360 .codex/skills/360
ln -s ../../.claude/skills/ghostwriter .codex/skills/ghostwriter
```

---

## The three .toml types at a glance

| Type               | Example                              | Count                 | Points to                            | Git access                  |
| ------------------ | ------------------------------------ | --------------------- | ------------------------------------ | --------------------------- |
| Command wrapper    | `build.toml`, `jc.toml`, `wave.toml` | ~15 (one per command) | `.claude/commands/{name}.md`         | Yes (when orchestrating)    |
| Role agent wrapper | `be-developer.toml`, `fe-qa.toml`    | ~4 per subproject     | `{project}/.claude/agents/{role}.md` | No (sandbox blocks `.git/`) |
| Git operator       | `gitter.toml`                        | 1                     | `.claude/agents/gitter.md`           | Yes (phase-based)           |

---

## Git ownership rules

The key Codex-specific difference is **who owns git**:

**When Codex orchestrates a full pipeline** (`$build`, `$wave`):

- Codex reads `.claude/agents/gitter.md` as its git protocol manual
- Codex executes gitter phases inline via bash git commands
- Anywhere the command manual says "Use the gitter agent" → "Execute gitter.md Phase X inline"
- The separate gitter agent is NOT involved

**When Codex runs as a scoped implementer** (role agent, delegated task):

- Codex has NO git access — the sandbox blocks `.git/` writes
- Only the orchestrating runtime (Claude or the parent Codex agent) handles git

**When Codex runs `/jc`** (hotfix mode):

- Codex executes gitter.md Phase 4 (JC-COMMIT) inline — LOCAL commits only
- **Codex MUST NOT push** — JC-COMMIT is local. Push is a separate explicit action.

---

## What NOT to do

- **Don't make Codex a requirement** — every pipeline operation must work with Claude Code alone
- **Don't duplicate logic in .toml files** — they point to `.claude/` manuals, not restate them
- **Don't let Codex edit `.claude/` or `CLAUDE.md`** — those are the source of truth, edited only by `/pcm`
- **Don't let Claude edit `.codex/`** — it's Codex's config layer, co-owned by `/pcm`
- **Don't use loose parenthetical lists in developer_instructions** — Codex can misread `"(commit, lock, push)"` as authorization to perform all listed actions. Be explicit about what is and isn't allowed.
- **Don't put pipeline logic in .toml files** — if you find yourself writing more than ~30 lines of developer_instructions, the logic belongs in the `.claude/` manual, not the wrapper.

---

## File structure after setup

```
your-project/
├── CLAUDE.md                    ← source of truth (Claude reads this)
├── AGENTS.md                    ← symlink → CLAUDE.md (Codex reads this)
├── .claude/                     ← pipeline source of truth
│   ├── agents/                  ← agent manuals (read by both runtimes)
│   ├── commands/                ← command manuals (read by both runtimes)
│   ├── scripts/                 ← worktree.sh, alloc-ports.sh, dev.sh
│   └── skills/                  ← Claude skills + shared research skills
├── .codex/                      ← OPTIONAL Codex config layer
│   ├── config.toml              ← global Codex settings
│   ├── rules/                   ← safety rules (git protection, destructive ops)
│   │   └── default.rules        ← prefix_rule() definitions
│   ├── agents/                  ← .toml wrappers pointing to .claude/ manuals
│   │   ├── build.toml           ← command wrapper
│   │   ├── jc.toml              ← command wrapper
│   │   ├── wave.toml            ← command wrapper
│   │   ├── gitter.toml          ← git operator
│   │   ├── be-developer.toml    ← role agent wrapper
│   │   ├── be-planner.toml      ← role agent wrapper
│   │   ├── fe-developer.toml    ← role agent wrapper
│   │   └── ...                  ← one per role per subproject
│   └── skills/                  ← $name interactive invocations
│       ├── build/SKILL.md       ← mirrors /build
│       ├── jc/SKILL.md          ← mirrors /jc
│       ├── rr -> symlink        ← shared research skill
│       └── ...
└── {subprojects}/
    ├── CLAUDE.md
    └── .claude/agents/          ← per-project role manuals (read by .toml wrappers)
```

---

## Generating wrappers for your project

The example `.toml` files in this template use placeholders:

| Placeholder                  | Replace with                                                         |
| ---------------------------- | -------------------------------------------------------------------- |
| `{PROJECT_NAME}`             | Your project's display name (e.g., "Acme Platform")                  |
| `{BACKEND_PROJECT}`          | Your backend subproject directory (e.g., "acme-api")                 |
| `{INFRA_PROJECT}`            | Your infra subproject directory (e.g., "acme-infra")                 |
| `{KNOWLEDGE_ROOT}`           | Where domain knowledge lives (e.g., "acme-ai/knowledge/")            |
| `{LANGUAGE_AND_CONVENTIONS}` | Your stack's code rules (e.g., "TypeScript strict mode, ESM")        |
| `{USER_PERSONA}`             | Your primary user persona (e.g., "therapist", "analyst", "operator") |
| `{MARKET_SEGMENT}`           | Your target market (e.g., "GGZ", "fintech", "edtech")                |
| `{REGULATION}`               | Applicable regulations (e.g., "GDPR, EU AI Act, HIPAA")              |
| `{project-a}`, `{project-b}` | Your actual subproject directory names                               |

For a project with N subprojects and M roles per subproject, you need:

- ~15 command wrapper `.toml` files (one per command)
- N x M role agent `.toml` files (one per role per subproject)
- 1 gitter `.toml` file
- ~15 skill directories (one per command)

Total: roughly `16 + (N x M)` files. For a 4-subproject monorepo with 4 roles each, that's ~32 files.
