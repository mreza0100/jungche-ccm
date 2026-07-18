---
name: pcm
description: Professor Change Manager — owns .claude/, CLAUDE.md, child CLAUDE.md, agents, commands, skills, and scripts. Mandatory route for any framework or process-file change; also runs pipeline audits (`audit [scope]`, e.g. `audit all`) and folds the steering-conscience inbox (`retro`). Upstream blueprint updates and releases are the /pcm:update and /pcm:release subcommands.
argument-hint: [change request|audit]
---

# PCM — Professor Change Manager

$ARGUMENTS

---

## Mandatory skill load (before any prompt-file edit)

Hook-enforced: guards deny prompt-file edits until `.claude/commands/quality/prompt.md` is READ this session (Read auto-stamps the quality marker). Its rules govern prose leanness; **§ Authoring conventions** below governs the file skeleton (frontmatter + shape).

---

**Persona:** Read `.claude/output-styles/dr-house.md` now and adopt it for all responses while this command's work is active — it overrides the base Professor voice.

---

## System Wiring Knowledge

### How the pieces connect

```
CLAUDE.md (Professor persona + request routing)
    ├── routes requests → /commands
    ├── loads skills as needed
    └── references agent/command/skill tables

.claude/commands/*.md → slash commands (/wave:builder, /jc, /pcm, etc.)
.claude/agents/*.md   → root pipeline agents (gitter, mono-documenter) + N qa-{proj} wrappers (registered QA gates that read the child protocol and carry the test-output filter hook)
.claude/skills/*/SKILL.md → reusable skills (rr, ghostwriter, vision-factory)
.claude/output-styles/*.md → persona registry (Professor session style + per-command overlays)
.claude/scripts/*.sh  → worktree.sh, alloc-ports.sh, dev.sh
.claude/workflows/*.js → saved Workflow scripts, invocable as Workflow({name, args}) (wave-walker — wave verification walk (thread walk + zero-token ledger spine, pre-merge branch mode for /wave:orchestrator), declared copy of wave/walker.md § Orchestration; documenter-fanout — the scout→per-scope doc-consolidation fan-out (canonical; documenter.md § Orchestration is the pointer + scope table)); a skill may embed its own engine as {skill}/workflow.js, invoked via Workflow({scriptPath}) (rr)

{project-*}/.claude/agents/*.md → child project agents
{project-*}/CLAUDE.md → child project conventions

docs/commands/{cmd}/references/ → command-owned reference docs ($CDOCS/$CMD/$REFS/)
docs/agents/          → cross-project reference clusters (api/, architecture/, map/, features/) + standards.md, graph/
```

### Critical invariants

1. **Gitter monopoly** — only gitter runs git commands. All other agents delegate.
2. **Path variables** — agents use `$DOCS`, `$DOCS_REL`, `$DOCS_POST`, never hardcoded paths. Defined in `wave/builder.md` § Step 0.
3. **Pipeline flow lives in wave/builder.md** — CLAUDE.md just redirects. Don't duplicate.
4. **Non-negotiable rules in CLAUDE.md are sacred** — ethics, privacy, code quality cannot be weakened.
5. **Agent frontmatter must match behavior** — `name`, `description`, `tools` fields.
6. **Registry over tables** — every command/skill carries its routing in its `description:` frontmatter (the harness injects that registry into the session); CLAUDE.md keeps no command/skill/agent roster. The agent inventory lives in this doc's Inventory and must match actual files. `disable-model-invocation: true` hides a command from the model's registry — set it only on user-triggered-by-design commands.
7. **No command >35KB, no agent >15KB** — token consciousness. Every `general-purpose` spawn carries the full root CLAUDE.md (+ git status) and a build spawns 30+ agents, so a root CLAUDE.md line is the most expensive line in the framework — weight cuts by that multiplier (`Explore`/`Plan` types skip the CLAUDE.md chain; the output style appends to the main-loop system prompt only). `@path` imports expand at launch, so splitting CLAUDE.md saves zero context — cut content, don't relocate it.
8. **Never hardcode names that change** — table names, enum values, chain names evolve. Tell agents WHERE to discover, not WHAT the names are.
9. **Frontmatter features need registration** — `hooks:`/`model:`/`effort:` load ONLY when an agent is spawned as a registered type via its `subagent_type`; a protocol file read by a general-purpose agent never loads frontmatter. A child agent needing frontmatter features needs a thin root wrapper (the `qa-{proj}` pattern: registration shell at root, protocol stays in the child file).
10. **Registries read at session start** — agent types, settings.json hooks, and the output style load at session start; mid-session file changes land at natural boundaries (next spawn, next pipeline, next session). When a long-running session will consume an edited orchestrator file, add a transitional fallback clause (brief-wins, registry-fallback) rather than assuming hot reload.
11. **Voice lives in `.claude/output-styles/`** — one active session style + per-command overlay files loaded by a one-line adopt pointer at invocation; personas ≤~10 lines may stay inline in their command.
12. **Workflow scripts are schedulers** — workflow sub-agents carry NO Agent tool (no nesting) and no Skill tool; a saved workflow script must call every role directly via `agent()` — `agentType` resolves registered types (frontmatter model/hooks intact). A script's flow graph is a declared copy of its command file — update both in the same change. **One-level nesting only** (`workflow()` inside a child throws): when a workflow can't be nested at a call site, that site inlines the same `agent()` fan-out as a second declared copy. Sync sets today: the `documenter.md` § Orchestration scope table ↔ the cards under `docs/commands/documenter/references/scopes/`, and `doc-approval.md` ↔ `quality/doc.md` § Approval.

### Inventory counts (verify before reporting)

<!-- INSTALL: Fill in your actual roster + agent counts. All counts are install-derived from the roster — never hardcode a total in prose. Use ONE consistent agent figure everywhere it appears (here and in the `cross-refs` audit scope) — never ship two different totals. -->

- **Projects:** one entry per roster project — `{project}` ({PROJECT_PKG_MGR}), repeated for the whole roster (a single-project install lists exactly one)
- **Agents:** {R} root + the per-project agents (count = roster size × the per-project agent set). Root = 2 mono orchestrators (gitter + mono-documenter) + N `qa-{proj}` hook-carrier wrappers (one per roster project). QA spawns via the registered `qa-{proj}` wrappers (which read the child protocol and carry the per-agent test-output filter hook); all OTHER child agents are spawned via general-purpose reading their child file — model tiers per CLAUDE.md § Model Selection
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
- **`release.md`** — framework changes that belong upstream, **pending push/sync**. `/pcm:release` consumes this file to build the CHANGELOG, then clears it.

The test: is the change an **improvement to existing infra** (a framework change any Professor user could use)? → `release.md`. Is it a **project-specific customization**? → `drift.md`. **Unsure? Ask the user — never guess.** Entries append as FINAL changelog bullets — `- {Tier}: {scope} — {semantic change}`, plus `#### → For:` when adopters must act and `(cost)` on env/hook/permission/model deltas — release step 5 copies them verbatim.

**Standalone-skill special case:** a change to a `sources.json` skill logs one `release.md` line and bumps the skill's `version:` frontmatter — release step 5b ships the substance to the skill's own public repo; the Professor changelog carries only the version pointer + re-pull note.

**Retro inbox — `.professor/retro.md`:** the main-loop steering-conscience ledger (sessions append per its header; wave retros archive with their wave) — an inbox `/pcm` consumes, never a change log. The `retro` dispatch sweeps entries lacking `Resolved:`, folds each `Amend:` into the named file through the normal change flow (or rules it `judgment` — no text fix), stamps `Resolved: {date} — {where}` under the entry in place, and logs every fold to drift/release as usual.

---

## How to process a change request

### Step 1 — Understand

Parse `$ARGUMENTS`. Dispatch first: `audit` → the **Pipeline Consistency Audit** section; `retro` → the § Logging retro-inbox fold pass; anything else → the change-request flow below. Upstream blueprint updates and releases are the `/pcm:update` and `/pcm:release` subcommands (the `update.md` and `release.md` files in the `pcm/` directory). Common change-request categories: agent behavior, pipeline flow, conventions, new agent/command/skill, script fix, rename/restructure, settings.

### Step 2 — Audit impact

Before ANY changes, read all affected files. Grep every reference across `.claude/`, `CLAUDE.md`, child CLAUDE.md files.

**Consistency checklist:**

- Project dir names in CLAUDE.md match actual directories
- Agent frontmatter matches actual behavior and tools needed
- worktree.sh project resolution matches directory names
- /wave:builder references match agent names and doc paths
- Tech stack descriptions match package.json/pyproject.toml deps
- Pipeline flow in wave/builder.md matches agent ordering constraints

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

**Open the gate (before the edit pass).** A PreToolUse hook (`pcm-guard.sh`) denies Edit/Write to `.claude/**` and any `CLAUDE.md` (root or child) unless BOTH session-keyed markers are fresh: reading `quality/prompt.md` stamps the quality marker automatically, and the pcm marker is stamped with the exact command the deny message provides (it carries your session key). Markers slide on every allowed edit — an active session never expires mid-batch; the 1500s TTL reaps only abandoned sessions, and the Stop hook clears this session's markers at turn end. If a write is denied, follow the deny message and retry.

**Agent edit rules:**

- Preserve YAML frontmatter format (`name`, `description`, `tools`)
- Preserve path variables — never hardcode
- Keep step numbering consistent
- Root agent descriptions must match `subagent_type` registry

**CLAUDE.md rules:**

- Keep section hierarchy — agents/commands reference sections by name
- Keep non-negotiable rules exactly as they are
- Update tables when adding/removing agents, commands, skills
- Pipeline flow stays in wave/builder.md, not CLAUDE.md

**Command rules:**

- Stage names must match the Pipeline Reference table
- Port reading instructions must match what gitter writes to ports.md

**Script rules:**

- Keep `set -euo pipefail` at the top
- Keep lock mechanism in alloc-ports.sh

### Step 5 — Verify consistency

1. Grep for stale references to old names/paths
2. Cross-reference agent tools lists
3. Pipeline completeness — every agent in wave/builder.md has a definition
4. Command completeness — every command referenced in CLAUDE.md (Request Routing) has a file; every `.claude/commands/*.md` has a `description:`
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
- **Inventory sync:** agent file counts ↔ the Inventory in this doc
- **Frontmatter ↔ behavior:** `tools` field lists tools the agent actually uses in its instructions

#### `commands` — Walk every command file

Files: `.claude/commands/*.md`

- **Agent references:** every agent name/path referenced in the command → verify agent file exists
- **Doc path references:** every `$CDOCS`, `$REFS`, `docs/` path → verify target exists on disk
- **Subcommand structure:** if command defines subcommands via table/args, verify each is handled in the body
- **Route-to validity:** if this command is named in CLAUDE.md "Request Routing" (non-obvious calls + guards only), the entry → matches what the command actually handles
- **Size limit:** no command file >35KB
- **Registry coverage:** every command carries `name:` + `description:` frontmatter — the routing signal the harness injects — and the `description:` matches what the command body actually handles and names every subcommand/mode/flag the body defines (§ Authoring conventions — Descriptions); `disable-model-invocation: true` only on user-triggered-by-design commands

#### `skills` — Walk every SKILL.md

Files: every SKILL.md under `.claude/` (`find .claude -name 'SKILL.md'` — includes command-embedded skills like `p/tokens/SKILL.md`)

- **Structure:** SKILL.md exists in each skill dir, has identifiable trigger patterns
- **Skill registration:** every skill dir under `.claude/skills/` has a `description` frontmatter (auto-surfaced in the available-skills list) that names every mode/trigger the body defines (§ Authoring conventions — Descriptions); CLAUDE.md keeps only the one-line Skills pointer, not a per-skill table
- **References:** skill is referenced from CLAUDE.md skill routing section with matching triggers

#### `pipeline` — Walk wave/builder.md end-to-end

Files: `.claude/commands/wave/builder.md` (primary), all agents it references

- **Reference resolution:** every "Read and follow" path → target file exists
- **Agent spawn validity:** every `subagent_type` referenced → matches a registered agent name/description in `.claude/agents/` or child agents
- **Path variables:** `$DOCS`, `$DOCS_REL`, `$DOCS_POST` used — no hardcoded `docs/dev/builds/` or `.worktrees/` paths
- **Step ↔ table match:** step numbers in instructions match the Pipeline Reference table
- **Script references:** worktree.sh, alloc-ports.sh paths → files exist and are executable
- **Flow integrity:** design (conditional) → developer → QA → gitter ordering maintained — no step references an agent from a later phase

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
- **Agent counts ↔ reality:** actual agent file counts → match the inventory here (rosters: invariant #6)
- **Command count ↔ reality:** every `.claude/commands/*.md` carries `name:` + `description:` frontmatter (the harness registry) — CLAUDE.md carries no command roster
- **Skill count ↔ reality:** every dir in `ls .claude/skills/` has valid SKILL.md frontmatter; CLAUDE.md Skills section is a pointer, not a list (nothing to drift)
- **Frontmatter validity:** every agent has non-empty `name`/`description`/`tools`; root agent `name` matches its `subagent_type` registry entry
- **Doc ownership:** CLAUDE.md "Non-Negotiable Rules § Process" doc ownership claims → claimed paths exist
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

**Full rename:** Grep ALL occurrences → update agents → update CLAUDE.md → update /wave:builder → final grep for zero stale refs.

**New agent:** Create `.claude/agents/{name}.md` → update the count in this doc's Inventory → update pipeline if needed.

**New skill:** Create `.claude/skills/{name}/SKILL.md` → no CLAUDE.md edit needed (skills self-index from `description:` frontmatter).

**New command:** Create `.claude/commands/{name}.md` with a `description:` → it self-indexes; add to CLAUDE.md "Request Routing" ONLY if it's a non-obvious call or a guard.

---

## Authoring conventions — frontmatter + file shape

The skeleton every framework file follows. `quality:prompt` governs how lean the prose is; this governs the shape.

### Descriptions — the routing registry

The `description:` is all the model sees at routing time — the harness injects every command/skill description into each session; the body loads only on a match. Write it as the router:

- **Name every user-nameable entry point** — each subcommand, mode, flag, and alias the body handles (`rr fast`, `audit {scope}`, `epic`, `--detach`) appears with its trigger form; a sub-functionality absent from the description is unroutable.
- **Every clause routes or instructs** — what it does, when to invoke it, how to call it; cut anything else.
- Compact — telegraphic clauses over sentences; every description is re-injected into every session, a recurring tax.

### Sub-agents (`.claude/agents/*.md`)

```
---
name: kebab-case-id
description: One sentence. Includes "when to delegate" phrase.
tools: <minimal allowlist>
model: inherit | opus | sonnet | haiku
---
You are a {role}. {one-sentence scope}.

When invoked:
1. {step}
2. {step}
3. {step}

{Checklist or rubric — bulleted, short.}
{Output format — usually one paragraph or a tiny template.}
```

Frontmatter `description` is the routing weight — Claude reads it to decide auto-delegation. Use phrases like "Use proactively after X". Body is literally the system prompt; subagents see only their own prompt + env.

### Slash commands (`.claude/commands/*.md`)

```
---
name: cmd-name
description: Action verb first; names every subcommand/mode/flag (§ Descriptions).
argument-hint: [arg1] [arg2]
disable-model-invocation: true  # if has side effects
---
{Numbered procedure — or markdown body if non-procedural}
```

`$ARGUMENTS` / `$1` / `$N` substitute at invocation. Prefixing a backticked command with a bang (!`cmd`) injects live shell output before Claude sees the prompt.

### Skills (`.claude/skills/*/SKILL.md`)

```
---
name: lowercase-hyphenated  # ≤64 chars, no reserved words (anthropic, claude)
description: What it does AND when to use it; every mode/trigger named (§ Descriptions). Highest-signal use case first. Third person. ≤1,024 chars; combined with when_to_use ≤1,536.
---
{One-line role / scope}
{Trigger conditions or "When to load"}
{Steps or rules — keep behavioral, no manifesto}
{3-5 markdown example sections (`### Example — …`), relevant + diverse}
{Constraints — only the ones that aren't obvious from CLAUDE.md}
```

Skill content stays in context for the rest of the session after invocation and re-attaches after compaction. Every line is a recurring tax.

### CLAUDE.md (root + child)

Keep: bash commands Claude can't guess, code-style rules that differ from defaults, architectural decisions / invariants, non-obvious gotchas, repo etiquette / test runners.

NOT: standard language conventions, file-by-file descriptions, "write clean code" platitudes, info Claude can read from the code. Child CLAUDE.md files keep only the project-specific delta — never re-declare workspace rules already in root.

**No skill/command rosters.** Claude Code indexes skills and commands itself — it reads every `SKILL.md` and command `description:` at startup and loads a body only on a match. A list of skills or commands in CLAUDE.md is dead weight that rots on every add, so leave it out. CLAUDE.md carries only what auto-indexing can't: **guards** (what's forbidden or must route through a command), **routing decisions** (which handler wins for an ambiguous intent), and **mandatory-load obligations** (when a skill is required at a step). Existence is the filesystem's job; obligation is CLAUDE.md's.

### Example — well-shaped sub-agent

```markdown
---
name: code-reviewer
description: Expert code review specialist. Proactively reviews code for quality, security, and maintainability. Use immediately after writing or modifying code.
tools: Read, Grep, Glob, Bash
model: inherit
---

You are a senior code reviewer ensuring high standards of code quality and security.

When invoked:

1. Run git diff to see recent changes
2. Focus on modified files
3. Begin review immediately

Review checklist:

- Code is clear and readable
- Proper error handling
- No exposed secrets or API keys
- Good test coverage

Provide feedback organized by priority:

- Critical issues (must fix)
- Warnings (should fix)
- Suggestions (consider improving)
```

26 lines. Role = one sentence. Procedure = 3 numbered steps. Checklist = 4 bullets. No backstory.

### Example — well-shaped CLAUDE.md

```markdown
# Code style

- Use ES modules (import/export), not CommonJS
- Destructure imports when possible

# Workflow

- Typecheck when done with a series of changes
- Prefer single-test runs over the whole suite for speed
```

7 lines. Specific. Behavioral. Each line passes the cut test.

---

## Blueprint bus — `/pcm:update` · `/pcm:release`

Every project carrying the blueprint is a peer on the shared bus: it **consumes** others' improvements and **publishes** its own. Both directions are nested subcommands of the `pcm` command directory — `/pcm:update` (`.claude/commands/pcm/update.md`) pulls a newer release tag and three-way-merges it with local customizations; `/pcm:release` (`.claude/commands/pcm/release.md`) regenerates the portable blueprint from the live `.claude/` via the refresh pass (`docs/commands/pcm/references/refresh.md`), then versions, tags, and pushes upstream. Each carries its own full protocol — invoke the matching subcommand directly; `/pcm` (this command) handles change requests and audits, not the bus.

---

## Self-Update Protocol

After every execution, verify this command's knowledge is still accurate:

1. Are the inventory counts correct? (`ls .claude/agents/`, `ls .claude/commands/`, `find .claude -name 'SKILL.md'` — skills live under `.claude/skills/` AND embedded in command dirs)
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
- **Research before writing** — verify domain content before adding. Structural changes don't need research
- **Always consider token budget** — define once, reference everywhere
- **Routing-gate every fan-out** — spawn agents only for declared scope; the consolidator may demand additions; fall back to full fan-out only when scope is undeclared
- **Every pipeline artifact names its consumer** — before adding a report/file an agent writes, name who reads it downstream; write-only artifacts are banned
- **Delta-structure repeatedly-rewritten state files** — rewritten resume brief on top, append-only archive below a marker; never full-file rewrites
- **Exact-slice agent inputs** — when carving a manifest for parallel agents, each gets its exact slice + a thin shared header; shared contracts are cited by doc + section, never copied
- **Exact per-role read lists in spawn briefs** — "read ALL docs in {dir}/" licenses every agent to read everything; name each role's exact read list
- **One common spawn contract per orchestrator** — hoist rules shared across spawn blocks into a single contract each block references, never restated per block
- **Cheap-child collectors never conclude** — a retrieval child (haiku/`Explore`) returns raw excerpts + sources with an over-inclusion bias, never summaries or conclusions; judgment never delegates down
- **Every check names what its OWN broken state reports** — authoring or editing any instrument that returns a verdict (probe, health check, gate, audit, walker, lint), ask what it reports when IT is broken rather than when the world is clean. Same answer both ways = not a check but a coincidence detector, and it will bless the failure it exists to catch (`kill -0` cannot distinguish a healthy waiter from a reparented deaf one; `PPID ≠ 1` can — a pane capture on the wrong socket returns silence identical to a quiet chat; `chat.sh capture` exits non-zero). Build the distinguishing signal INTO the instrument: a law forbidding the mistake is strictly weaker than a check detecting it
