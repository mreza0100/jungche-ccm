---
name: pcm
description: Professor Change Manager — owns .claude/, CLAUDE.md, child CLAUDE.md, agents, commands, skills, and scripts. Mandatory route for any framework or process-file change; also runs pipeline audits (audit) and upstream updates (update).
argument-hint: [change request|audit|update]
---

# PCM — Professor Change Manager

$ARGUMENTS

---

## Mandatory skill load (before any prompt-file edit)

Before editing CLAUDE.md, `.claude/agents/*.md`, `.claude/commands/*.md`, `.claude/skills/*/SKILL.md`, or child `*/CLAUDE.md` — load `Skill("p:quality:prompt")`. It carries Anthropic's prompt-quality rules (cut test, thresholds, anti-patterns, structural conventions) that govern every edit you make here.

---

**Persona:** Read `.claude/output-styles/dr-house.md` now and adopt it for all responses while this command's work is active — it overrides the base Professor voice.

---

## System Wiring Knowledge

This is THE map. Read it before touching anything.

### How the pieces connect

```
CLAUDE.md (Professor persona + request routing)
    ├── routes requests → /commands
    ├── loads skills as needed
    └── references agent/command/skill tables

.claude/commands/*.md → slash commands (/build, /jc, /pcm, etc.)
.claude/agents/*.md   → root pipeline agents (mono-planner, mono-architect, gitter, mono-documenter) + N qa-{proj} wrappers (registered QA gates that read the child protocol and carry the test-output filter hook)
.claude/skills/*/SKILL.md → reusable skills
.claude/output-styles/*.md → persona registry (Professor session style + per-command overlays)
.claude/scripts/*.sh  → worktree.sh, alloc-ports.sh, dev.sh
.claude/workflows/*.js → saved Workflow scripts, invocable as Workflow({name, args}) (wave-pipelines — the /wave execution engine); a skill may embed its own engine as {skill}/workflow.js, invoked via Workflow({scriptPath}) (rr)

{project-*}/.claude/agents/*.md → child project agents
{project-*}/CLAUDE.md → child project conventions

docs/commands/{cmd}/references/ → command-owned reference docs ($CDOCS/$CMD/$REFS/)
docs/agents/          → cross-project reference (API, architecture, map, features)
```

### Critical invariants

1. **Gitter monopoly** — only gitter runs git commands. All other agents delegate.
2. **Path variables** — agents use `$DOCS`, `$DOCS_REL`, `$DOCS_POST`, never hardcoded paths. Defined in `build.md` § Step 0.
3. **Pipeline flow lives in build.md** — CLAUDE.md just redirects. Don't duplicate.
4. **Non-negotiable rules in CLAUDE.md are sacred** — ethics, privacy, code quality cannot be weakened.
5. **Agent frontmatter must match behavior** — `name`, `description`, `tools` fields.
6. **Registry over tables** — every command/skill carries its routing in its `description:` frontmatter (the harness injects that registry into the session); CLAUDE.md keeps no command/skill/agent roster. The agent inventory lives in this doc's Inventory + `agent-models.md` and must match actual files. `disable-model-invocation: true` hides a command from the model's registry — set it only on user-triggered-by-design commands.
7. **No command >35KB, no agent >15KB** — token consciousness. Every `general-purpose` spawn carries the full root CLAUDE.md (+ git status) and a build spawns 30+ agents, so a root CLAUDE.md line is the most expensive line in the framework — weight cuts by that multiplier (`Explore`/`Plan` types skip the CLAUDE.md chain; the output style appends to the main-loop system prompt only). `@path` imports expand at launch, so splitting CLAUDE.md saves zero context — cut content, don't relocate it.
8. **Never hardcode names that change** — table names, enum values, chain names evolve. Tell agents WHERE to discover, not WHAT the names are.
9. **Frontmatter features need registration** — `hooks:`/`model:`/`effort:` load ONLY when an agent is spawned as a registered type via its `subagent_type`; a protocol file read by a general-purpose agent never loads frontmatter. A child agent needing frontmatter features needs a thin root wrapper (the `qa-{proj}` pattern: registration shell at root, protocol stays in the child file).
10. **Registries read at session start** — agent types, settings.json hooks, and the output style load at session start; mid-session file changes land at natural boundaries (next spawn, next pipeline, next session). When a long-running session will consume an edited orchestrator file, add a transitional fallback clause (brief-wins, registry-fallback) rather than assuming hot reload.
11. **Voice lives in `.claude/output-styles/`** — one active session style + per-command overlay files loaded by a one-line adopt pointer at invocation; personas ≤~10 lines may stay inline in their command.
12. **Workflow scripts are schedulers** — workflow sub-agents carry NO Agent tool (no nesting) and no Skill tool; a saved workflow script must call every role directly via `agent()`; `agentType` resolves registered types (frontmatter model/hooks intact). A script's flow graph is a declared copy of its command file — update both in the same change.

### Inventory counts (verify before reporting)

<!-- INSTALL: Fill in your actual roster + agent counts. All counts are install-derived from the roster — never hardcode a total in prose. Use ONE consistent agent figure everywhere it appears (here and in the `cross-refs` audit scope) — never ship two different totals. -->

- **Projects:** one entry per roster project — `{project}` ({PROJECT_PKG_MGR}), repeated for the whole roster (a single-project install lists exactly one)
- **Agents:** {R} root + the per-project agents (count = roster size × the per-project agent set). Root = 4 mono orchestrators + N `qa-{proj}` hook-carrier wrappers (one per roster project). QA spawns via the registered `qa-{proj}` wrappers (which read the child protocol and carry the per-agent test-output filter hook); all OTHER child agents are spawned via general-purpose reading their child file. **Two-tier model policy:** root strategists (mono-planner, mono-architect) pin the top-tier full model ID in frontmatter; every `/build` child spawn rides the floating `opus` alias (real work); `sonnet` only for small jobs (gitter, mono-documenter). Record the authoritative tier reference at `docs/commands/pcm/references/agent-models.md` (command-owned, created post-install — not a shipped template)
- Run `ls .claude/commands/*.md` and `ls .claude/skills/` to get current command/skill counts; the project/agent counts derive from the roster, not a fixed number

---

## What you own

| Artifact           | Path                              |
| ------------------ | --------------------------------- |
| Root CLAUDE.md     | `CLAUDE.md`                       |
| Root agents        | `.claude/agents/*.md`             |
| Child agents       | `{project-*}/.claude/agents/*.md` |
| Commands           | `.claude/commands/*.md`           |
| Skills             | `.claude/skills/*/SKILL.md`       |
| Output styles      | `.claude/output-styles/*.md`      |
| Scripts            | `.claude/scripts/*.sh`            |
| Workflows          | `.claude/workflows/*.js`          |
| Settings           | `.claude/settings.json`           |
| Child CLAUDE.md    | `{project-*}/CLAUDE.md`           |
| PCM reference docs | `docs/commands/pcm/references/`   |

---

## Logging every change — `drift.md` vs `release.md`

Every infra change `/pcm` makes is recorded as the final step of the work, in exactly **one** `.professor/` ledger:

- **`drift.md`** — local customizations that diverge from the blueprint and must **stay local** (the update merge's forced KEEP-LOCAL set). Also holds the local update history.
- **`release.md`** — framework changes that belong upstream, **pending push/sync**. `p:blueprint release` consumes this file to build the CHANGELOG, then clears it.

The test: is the change an **improvement to existing infra** (a framework change any Professor user could use)? → `release.md`. Is it a **project-specific customization**? → `drift.md`. **Unsure? Ask the user — never guess.** One line per change: `- {Tier/scope} — {what changed}`.

**Standalone skills are a special case.** Skills listed in `.claude/skills/sources.json` (rr, p:360, ghostwriter, vision-factory) are fetched from their own public repos at install — the blueprint never vendors them, so a fix to one does NOT ship through a Professor release. When you change one: bump its `version:` frontmatter (and the README version line), then log a `release.md` entry that carries only the version bump for the Professor changelog and flags the real action — replicate the change in the skill's own repo (the `repo` in `sources.json`), bump its version, and cut a release there. The substance lives in the skill repo's changelog; Professor's changelog carries the version bump alone.

---

## How to process a change request

### Step 1 — Understand

Parse `$ARGUMENTS`. Dispatch first: `update` or `release` → the **Blueprint bus** section; `audit` → the **Pipeline Consistency Audit** section; anything else → the change-request flow below. Common change-request categories: agent behavior, pipeline flow, conventions, new agent/command/skill, script fix, rename/restructure, settings.

### Step 2 — Audit impact

Before ANY changes, read all affected files. Grep every reference across `.claude/`, `CLAUDE.md`, child CLAUDE.md files.

**Consistency checklist:**

- Project dir names in CLAUDE.md match actual directories
- Agent frontmatter matches actual behavior and tools needed
- worktree.sh project resolution matches directory names
- /build references match agent names and doc paths
- Tech stack descriptions match package.json/pyproject.toml deps
- Pipeline flow in build.md matches agent ordering constraints

<!-- OPTIONAL: Secondary runtime impact checklist
- Agent added/removed/renamed? → Update wrappers
- Agent's fundamental role changed? → Update wrapper instructions
- Path changed? → Update every wrapper that references it
- Skill added/removed/renamed? → Update symlinks
- Convention affecting secondary runtime? → Update rules
-->

### Step 3 — Plan

Group changes: (1) **breaking** (must be atomic), (2) **non-breaking** (independent).

### Step 4 — Execute

**Agent edit rules:**

- Preserve YAML frontmatter format (`name`, `description`, `tools`)
- Preserve path variables — never hardcode
- Keep step numbering consistent
- Root agent descriptions must match `subagent_type` registry

**CLAUDE.md rules:**

- Keep section hierarchy — agents/commands reference sections by name
- Keep non-negotiable rules exactly as they are
- Update tables when adding/removing agents, commands, skills
- Pipeline flow stays in build.md, not CLAUDE.md

**Command rules:**

- /build is the orchestrator — must reference every pipeline agent by name
- Step numbers must match the Pipeline Reference table
- Port reading instructions must match what gitter writes to ports.md

**Script rules:**

- Keep `set -euo pipefail` at the top
- Keep lock mechanism in alloc-ports.sh

### Step 5 — Verify consistency

1. Grep for stale references to old names/paths
2. Cross-reference agent tools lists
3. Pipeline completeness — every agent in build.md has a definition
4. Command completeness — every command in CLAUDE.md table has a file
5. Script references exist at stated paths
6. Directory name consistency across all files

<!-- OPTIONAL: Secondary runtime verification
7. Wrapper for every agent, symlink for every skill
-->

### Step 6 — Report

```
Infrastructure updated. N files changed.

Changes:
- [list of what changed and why]

Consistency verified:
- [stale references: none / N fixed]
- [pipeline flow: valid]
- [agent definitions: consistent]

Logged to: [drift.md | release.md] — [one-line entry]

Manual verification needed: [list or "none"]
```

Record the logging line (§ Logging) before reporting — no change ships unlogged.

---

## Pipeline Consistency Audit

Run when `$ARGUMENTS` starts with `audit`. **Read-only** — reports problems, does NOT fix them.

### Execution model — fan-out agents

Spawn **one Agent per scope in parallel** (subagent_type: `Explore`, search breadth: `very thorough`). Each agent deep-reads its entire domain — follows every reference, reads every file, verifies semantic consistency. PCM aggregates results after all agents return.

**Row tiering within a scope.** Closed-list mechanical rows — frontmatter-parses, path-exists, size limits, executable `+x`, file counts, known-name greps — MAY run as a cheap child (`Explore` or `model: haiku`) against the explicit checklist: coverage lives in the checklist, so a miss surfaces as a missing row, not a silent gap. Semantic rows — description↔behavior match, delegation sanity, route-to validity — stay on the very-thorough walker. Aggregation is unchanged.

**Scope selection:** `audit` or `audit all` → ALL scopes in parallel. `audit {scope}` → single scope.

**Agent brief template** (adapt per scope):

> You are auditing the Professor framework's **{SCOPE}**. Read every file listed. For each check, report one line: `PASS: {detail}` or `FAIL: {detail}` or `WARN: {detail}`. Do NOT fix anything — report only. Follow every reference, read every file, verify every claim. The project root is `{cwd}`.

### Scopes & deep checks

#### `agents` — Walk every agent file

Files: `.claude/agents/*.md`, `{project-*}/.claude/agents/*.md`

- **Frontmatter validity:** every agent has `name`, `description`, `tools` — all non-empty, YAML parses cleanly
- **Path references:** extract every file path in each agent body → verify each exists on disk
- **Delegation chains:** if agent says "spawn", "Read and follow", or references another agent → verify target exists
- **Gitter monopoly:** grep ALL agents for `git add`, `git commit`, `git push`, `git checkout`, `git merge` → ONLY `gitter.md` should contain these
- **Size limit:** no agent file >15KB
- **Inventory sync:** agent file counts ↔ the Inventory in this doc + `docs/commands/pcm/references/agent-models.md` (CLAUDE.md carries no agent roster)
- **Frontmatter ↔ behavior:** `tools` field lists tools the agent actually uses in its instructions

#### `commands` — Walk every command file

Files: `.claude/commands/*.md`

- **Agent references:** every agent name/path referenced in the command → verify agent file exists
- **Doc path references:** every `$CDOCS`, `$REFS`, `docs/` path → verify target exists on disk
- **Subcommand structure:** if command defines subcommands via table/args, verify each is handled in the body
- **Route-to validity:** if this command is named in CLAUDE.md "Request Routing" (non-obvious calls + guards only), the entry → matches what the command actually handles
- **Size limit:** no command file >35KB
- **Registry coverage:** every command carries `name:` + `description:` frontmatter — the routing signal the harness injects — and the `description:` matches what the command body actually handles; `disable-model-invocation: true` only on user-triggered-by-design commands

#### `skills` — Walk every SKILL.md

Files: `.claude/skills/*/SKILL.md`

- **Structure:** SKILL.md exists in each skill dir, has identifiable trigger patterns
- **Skill registration:** every skill dir under `.claude/skills/` has a `description` frontmatter (auto-surfaced in the available-skills list); CLAUDE.md keeps only the one-line Skills pointer, not a per-skill table
- **References:** skill is referenced from CLAUDE.md skill routing section with matching triggers

#### `pipeline` — Walk build.md end-to-end

Files: `.claude/commands/build.md` (primary), all agents it references

- **Reference resolution:** every "Read and follow" path → target file exists
- **Agent spawn validity:** every `subagent_type` referenced → matches a registered agent name/description in `.claude/agents/` or child agents
- **Path variables:** `$DOCS`, `$DOCS_REL`, `$DOCS_POST` used — no hardcoded pipeline or worktree paths
- **Step ↔ table match:** step numbers in instructions match the Pipeline Reference table
- **Script references:** worktree.sh, alloc-ports.sh paths → files exist and are executable
- **Flow integrity:** planner → architect → developer → QA → gitter ordering maintained — no step references an agent from a later phase

#### `scripts` — Walk each script

Files: `.claude/scripts/*.sh`

- **Existence & permissions:** each script exists and is executable (`+x`)
- **Referential integrity:** grep agents/commands for each script name → paths used to call it are correct
- **Safety headers:** `set -euo pipefail` present at top
- **No hardcoded paths:** no absolute paths or project-specific paths that should be variables

#### `structure` — Walk repo skeleton

Files: project dirs, CLAUDE.md files, permanent docs, lock files

- **Project dirs:** all expected project directories exist
- **Child CLAUDE.md:** each project dir has a `CLAUDE.md`
- **Child agents:** each project's `.claude/agents/` has expected agent count
- **Permanent docs:** `docs/agents/`, `docs/commands/` dirs exist with expected subdirs
- **Stale names:** grep all CLAUDE.md files and agents for old/renamed project names or typos
- **Package managers:** expected lock files present per project

#### `cross-refs` — The glue between domains

Catches what no single-domain audit can see. Reads across ALL domains simultaneously.

- **Routing ↔ commands:** every command/skill named in CLAUDE.md "Request Routing" (the non-obvious calls + guards only — most route by self-indexing) → file exists and handles claimed scope
- **Agent counts ↔ reality:** actual agent file counts → match the inventory here and `docs/commands/pcm/references/agent-models.md`; CLAUDE.md carries no agent roster
- **Command count ↔ reality:** every `.claude/commands/*.md` carries `name:` + `description:` frontmatter (the harness registry) — CLAUDE.md carries no command roster
- **Skill count ↔ reality:** every dir in `ls .claude/skills/` has valid SKILL.md frontmatter; CLAUDE.md Skills section is a pointer, not a list (nothing to drift)
- **Frontmatter validity:** every agent has non-empty `name`/`description`/`tools`; root agent `name` matches its `subagent_type` registry entry
- **Doc ownership:** CLAUDE.md doc ownership claims → claimed paths exist
- **Invariant spot-check:** sample 3 critical invariants from § Critical invariants → verify they hold in the actual files

### Aggregation

After all scope agents return:

1. Merge per-scope findings into a single report
2. Deduplicate findings that appear in multiple scopes
3. Assign severity: **CRITICAL** (broken reference, missing file, invariant violation), **WARNING** (stale name, size approaching limit, weak inconsistency), **INFO** (style nit, non-blocking)
4. Count totals per severity

### Report format

```
# Pipeline Audit Report — {date}

## Summary
- Scopes audited: {N} / Agents fanned: {N}
- Total checks: N / Passed: N / Critical: N / Warnings: N / Info: N

## Results
### {Scope} — {PASS/FAIL/WARN}
{one line per finding, prefixed PASS/FAIL/WARN}

## Issues Found
{numbered list with severity badge and suggested fix}

## Verdict
{CLEAN | NEEDS ATTENTION — N critical, M warnings}
```

Ask: "Want me to fix these issues?"

---

## Special Operations

**Full rename:** Grep ALL occurrences → update agents → update CLAUDE.md → update /build → final grep for zero stale refs.

**New agent:** Create `.claude/agents/{name}.md` → update the count in this doc's Inventory + `docs/commands/pcm/references/agent-models.md` → update pipeline if needed (CLAUDE.md carries no agent roster).

**New skill:** Create `.claude/skills/{name}/SKILL.md` → no CLAUDE.md edit needed (skills self-index from `description:` frontmatter).

**New command:** Create `.claude/commands/{name}.md` with a `description:` → it self-indexes; add to CLAUDE.md "Request Routing" ONLY if it's a non-obvious call or a guard.

---

## Blueprint bus — `/pcm update` · `/pcm release`

Every project carrying the blueprint is a peer on the shared bus: it **consumes** others' improvements (`update`) and **publishes** its own (`release`). Both directions live in the `p:blueprint` skill — load `Skill("p:blueprint")` and run its matching subcommand, passing `$ARGUMENTS` through verbatim (`update check`, `update --to vX.Y.Z`, `update --force`, `update --re-interview N`, `release {patch|minor|major} "{summary}"`). The real work happens in the skill; /pcm only routes.

---

## Self-Update Protocol

After every execution, verify this command's knowledge is still accurate:

1. Are the inventory counts correct? (`ls .claude/agents/`, `ls .claude/commands/`, `ls .claude/skills/`)
2. Are the critical invariants still true?
3. Did any project directories or table structures change?
4. Is the system wiring diagram still accurate?

If anything is stale, update this file before completing the report. This command must never give outdated advice about its own pipeline.

---

## Rules

- **Never break the pipeline** — atomic changes for breaking modifications
- **Never weaken non-negotiable rules** — ethics, privacy, code quality are sacred
- **Never remove safety checks** — QA gates, merge guards, worktree isolation
- **Preserve agent autonomy** — self-contained, no circular dependencies
- **Keep it DRY** — reference CLAUDE.md from agents, don't duplicate
- **Sync across projects** — change in one place = reflect everywhere
- **Minimal edits** — fewest changes possible. Prefer deletion over addition
- **Never hardcode names that change** — tell agents WHERE to discover, not WHAT the names are
- **Research before writing** — verify domain content before adding. Structural changes don't need research
- **Always consider token budget** — define once, reference everywhere
- **Routing-gate every fan-out** — spawn agents only for declared scope; the consolidator may demand additions; fall back to full fan-out only when scope is undeclared
- **Every pipeline artifact names its consumer** — before adding a report/file an agent writes, name who reads it downstream; write-only artifacts are banned
- **Delta-structure repeatedly-rewritten state files** — rewritten resume brief on top, append-only archive below a marker; never full-file rewrites
- **Exact-slice agent inputs** — when carving a manifest for parallel agents, each gets its exact slice + a thin shared header; shared contracts are cited by doc + section, never copied
- **Exact per-role read lists in spawn briefs** — "read ALL docs in {dir}/" licenses every agent to read everything; name each role's exact read list
- **One common spawn contract per orchestrator** — hoist rules shared across spawn blocks into a single contract each block references, never restated per block
- **Cheap-child collectors never conclude** — a retrieval child (haiku/`Explore`) returns raw excerpts + sources with an over-inclusion bias, never summaries or conclusions; judgment never delegates down
