---
name: pfm
description: Professor Framework Management — owns .claude/, CLAUDE.md, child CLAUDE.md, agents, commands, skills, and scripts. Mandatory route for any framework or process-file change; also runs pipeline audits (`audit [scope]`, e.g. `audit all`) and folds the steering-conscience inbox (`retro`). Release-bound framework changes route to the blueprint clone per § Where a change lands.
---

# PFM — Professor Framework Management

$ARGUMENTS

---

## Mandatory skill load (before any prompt-file edit)

Hook-enforced: guards deny prompt-file edits until `.claude/commands/quality/prompt.md` is READ this session (Read auto-stamps the quality marker). Its rules govern prose leanness for ANY prompt; **§ Claude-harness prompt law** below carries the harness-specific file rules (size limits, voice location, hooks, routing); **§ Authoring conventions** below governs the file skeleton (frontmatter + shape).

---

## System Wiring Knowledge

### How the pieces connect

- `CLAUDE.md` — request routing + guards; routes non-obvious requests to commands, names mandatory-load obligations; carries no rosters (§ Authoring conventions, no-rosters law)
- `.claude/commands/*.md` — slash commands (/wave:builder, /jc, /pfm, …)
- `.claude/agents/*.md` — root pipeline agents (gitter) + qa-{proj} wrappers, one per project (registered QA gates that read the child protocol and carry the test-output filter hook)
- `.claude/skills/*/SKILL.md` — reusable skills (`ls .claude/skills/` for the current set)
- `.claude/scripts/*.{sh,mjs}` — worktree.sh, alloc-ports.sh, dev.sh, codex-sync.sh (the `pfm codex` hook bridge)
- `.claude/workflows/*.js` — saved Workflow scripts, invocable as Workflow({name, args}) — each a declared copy of its command file's orchestration section (§ Critical invariants, workflow-scripts-are-schedulers); a skill may embed its own engine as {skill}/workflow.js via Workflow({scriptPath})
- `{project}/.claude/agents/*.md` — child project agents; `{project}/CLAUDE.md` — child project conventions. A `{project}` held as a git submodule lands its commits in the child repo, and the monorepo pins a pointer (gitter-owned)
- `docs/commands/{cmd}/references/` — command-owned reference docs ($CDOCS/$CMD/$REFS/); `docs/agents/` — documenter-owned cross-project reference clusters (`api/`, `architecture/`, `map/`, `features/`) + `standards.md`, `graph/`; `docs/facts/` — user-ruled system facts (main-loop-written, on explicit ruling only)

### Critical invariants

- **Path variables** — agents use `$DOCS`, `$DOCS_REL`, `$DOCS_POST`, never hardcoded paths. Defined in `wave/builder.md` § Step 0.
- **Pipeline flow lives in wave/builder.md** — CLAUDE.md just redirects. Don't duplicate.
- **Agent frontmatter must match behavior** — `name`, `description`, `tools` fields.
- **Registry over tables** — a command/skill's `description:` frontmatter IS its routing (the harness injects that registry into every session); `disable-model-invocation: true` hides a command from the model's registry — set it only on user-triggered-by-design commands. The roster ban and what CLAUDE.md may carry: § Authoring conventions (CLAUDE.md).
- **No command >35KB, no agent >15KB** — token consciousness. Every `general-purpose` spawn carries the full root CLAUDE.md (+ git status) and a build spawns 30+ agents, so a root CLAUDE.md line is the most expensive line in the framework — weight cuts by that multiplier (`Explore`/`Plan` types skip the CLAUDE.md chain; the fleet prompt rides the main-loop system prompt only). `@path` imports expand at launch, so splitting CLAUDE.md saves zero context — cut content, don't relocate it.
- **Never hardcode names, counts, or rosters that change** — table names, enum values, chain names, agent/queue/chain tallies evolve. Tell agents WHERE to discover (`ls`, a registry file, the owning script), not WHAT the values are.
- **Frontmatter features need registration** — `hooks:`/`model:`/`effort:` load ONLY when an agent is spawned as a registered type via its `subagent_type`; a protocol file read by a general-purpose agent never loads frontmatter. A child agent needing frontmatter features needs a thin root wrapper (the `qa-{proj}` pattern: registration shell at root, protocol stays in the child file).
- **Registries read at session start** — agent types, settings.json hooks, and the injected fleet prompt load at session start; mid-session file changes land at natural boundaries (next spawn, next pipeline, next session). When a long-running session will consume an edited orchestrator file, add a transitional fallback clause (brief-wins, registry-fallback) rather than assuming hot reload.
- **Workflow scripts are schedulers** — workflow sub-agents carry NO Agent tool (no nesting) and no Skill tool; a saved workflow script must call every role directly via `agent()` — `agentType` resolves registered types (frontmatter model/hooks intact). A script's flow graph is a declared copy of its command file — update both in the same change. **One-level nesting only** (`workflow()` inside a child throws): when a workflow can't be nested at a call site, that site inlines the same `agent()` fan-out as a second declared copy. Sync set today: `doc-approval.md` ↔ `quality/doc.md` § Approval.

### Inventory (derive, never recall)

<!-- INSTALL: this section is derive-only by design — no fixed counts to fill in. The bash commands below run against the actual roster/filesystem every time, so a single-project install and a ten-project install both get correct answers from the same text. -->

- **Projects:** derive with `ls -d {project}*/`; each child CLAUDE.md § Quick Start names its package manager
- **Agents:** enumerate with `ls .claude/agents/ {project}/.claude/agents/` — every agent is registered at root on the `{role}-{proj}` convention (`qa-{proj}`, `developer-{proj}`, …), plus the project-neutral `gitter` and `architect`. A root wrapper is a thin registration shell — frontmatter (name, description, model, tools, hooks) over a one-line pointer to the child protocol at `{project}/.claude/agents/{role}.md`; a `{project}` whose child repo is not readable from the monorepo inlines its protocols at root instead. Model tiers per CLAUDE.md § Model Selection
- Commands and skills: `ls .claude/commands/ .claude/skills/`

---

## Claude-harness prompt law

Harness-specific rules for files Claude Code loads at runtime — the general prompt law (cut test, compaction, anti-patterns) lives in `/quality:prompt` and applies on top.

### The harness prompt stream

In the Claude Code harness the LLM reads one concatenated context: root `CLAUDE.md`, the auto-loaded skill descriptions, the active command or agent, and every skill loaded this session — all at once. Audit any harness prompt against that whole stream (per `/quality:prompt § The prompt stream`).

### Hard thresholds (Anthropic-published)

- CLAUDE.md (any): ≤ 200 lines
- SKILL.md body: ≤ 500 lines — split via progressive disclosure above this
- Skill `description` + `when_to_use`: ≤ 1,536 chars combined
- Sub-agent body: no formal cap — Anthropic's own examples run 20–35 lines

Above threshold = split into a referenced file (one level deep, with a Table of Contents at the top if >100 lines).

### Voice location

Voice lives in the fleet prompt (`templates/prompts/professor.md`), injected via `pfm` `claude.systemPrompt = "professor"`. CLAUDE.md and every agent/skill/command carry zero voice. Cross-file dedup targets: child CLAUDE.md keeps only its delta vs root CLAUDE.md; a project agent keeps only its delta vs the project CLAUDE.md it reads at start.

### Hooks vs prompts

For things that must happen every time (formatting, validation, secret-scanning), write a hook (`.claude/settings.json` PreToolUse / PostToolUse) — deterministic, cheap. Prompts are advisory; the model can drift. Once a hook owns an invariant, delete the prompt rule that restated it — keeping both is duplication against a deterministic mechanism.

### Where harness content goes (anti-bloat routing)

- Behavioral rules → prompt files (CLAUDE.md, agents, commands, skills)
- Incident narratives ("on 2026-XX-XX...") → commit message / epic manifest (`docs/epics/{name}/`) — never prompt files
- Architectural decisions / why-this-design → epic manifest or `docs/commands/{cmd}/references/` — prompts encode the rule, not the rationale
- Voice / character flavor → the fleet prompt (`templates/prompts/professor.md`) — zero voice in CLAUDE.md, agents, skills, commands
- Project-specific tooling → child CLAUDE.md only — never per-project agents (they inherit via parent)
- Cross-cutting templates (report format, plan shape) → one canonical reference file — never duplicated per-project

## What you own

- Root CLAUDE.md: `CLAUDE.md`; child CLAUDE.md: `{project}/CLAUDE.md`
- Agents: `.claude/agents/*.md` (root) + `{project}/.claude/agents/*.md` (child)
- Commands: `.claude/commands/*.md`; skills: `.claude/skills/*/SKILL.md`
- Scripts: `.claude/scripts/*.{sh,mjs}`; workflows: `.claude/workflows/*.js`; settings: `.claude/settings.json`
- Codex mirror: `.codex/` + `AGENTS.md` files + `$HOME/.codex/` — generated from the local Claude sources by `pfm codex build`; `pfm codex check` gates drift. The `codex-sync.sh` hooks run both after a Claude-source edit. Hand-written keepers: `.codex/config.toml` (except its generated `mcp_servers` fence, compiled from `.mcp.json`), `.codex/rules`
- PFM reference docs: `docs/commands/pfm/references/`

---

## Where a change lands

Classify FIRST — before any edit. The classification decides the source of truth. **Unsure? Ask the user — never guess.**

- **Framework change** → edit the canonical blueprint template at `{BLUEPRINT_CLONE_PATH}` under that repo's own law and gates, then log `.professor/release.md` for its release flow. Never put project-specific behavior into the blueprint.
- **Project customization** → edit this project's local file directly. That local file is the source of truth; it is not regenerated from the template. Use `.professor/drift.md` only when a human-readable customization note is useful, never as merge machinery.
- **Engine mirror** → never edit the generated output by hand. Change its local Claude source, then run `pfm codex build` and `pfm codex check` (or the owning compiler for another engine).
- **Upstream project-template delta** → run `pfm update check`, inspect the printed template diff, hand-apply the parts that belong locally, then advance that file's pin with `pfm update pin <local>`. New or retired mappings use the report's `pin --template` or `drop` action; a template this project will never take: `pfm update ignore <template>`. No baseline yet (the install predates `pfm init`): `pfm update adopt [--at <ref>]` once.

There is no local-stopgap-to-regeneration ceremony. A framework fix and a project customization are separate changes in their respective sources of truth.

Ledger entries append as FINAL changelog bullets — `- {Tier}: {scope} — {semantic change}`, plus `#### → For:` when adopters must act and `(cost)` on env/hook/permission/model deltas — the framework repo's release copies them verbatim.

**Standalone-skill special case:** a change to a `sources.json` skill logs one `release.md` line and bumps the skill's `version:` frontmatter — the framework repo's release flow ships the substance to the skill's own public repo; the Professor changelog carries only the version pointer + re-pull note.

**Retro inbox — `.professor/retro.md`:** the main-loop steering-conscience ledger (sessions append per its header; wave retros archive with their wave) — an inbox `/pfm` consumes, never a change log. The `retro` dispatch sweeps entries lacking `Resolved:`, folds each `Amend:` into the named file through the normal change flow (or rules it `judgment` — no text fix), stamps `Resolved: {date} — {where}` under the entry in place, and logs every fold to drift/release as usual.

---

## How to process a change request

### Step 1 — Understand

Parse `$ARGUMENTS`. Dispatch first: `audit` → the **Pipeline Consistency Audit** section; `retro` → the § Where-a-change-lands retro-inbox fold pass; anything else → the change-request flow below. Common change-request categories: agent behavior, pipeline flow, conventions, new agent/command/skill, script fix, rename/restructure, settings.

### Step 2 — Audit impact

Before ANY changes, read all affected files. Grep every reference across `.claude/`, `CLAUDE.md`, child CLAUDE.md files.

**Facts verify against code, never against prior text.** Every factual claim a framework file makes about the codebase — a path, mechanism, config, protocol, count — is verified against the code at write time (scout with an Explore agent for anything non-trivial); a rewrite derives from what the code says, never from what the file used to say. Files lie; the code doesn't.

**Consistency checklist:**

- Project dir names in CLAUDE.md match actual directories
- Agent frontmatter matches actual behavior and tools needed
- worktree.sh project resolution matches directory names
- /wave:builder references match agent names and doc paths
- Tech stack descriptions match package.json/pyproject.toml deps
- Pipeline flow in wave/builder.md matches agent ordering constraints

### Step 3 — Plan

Group changes: (1) **breaking** (must be atomic), (2) **non-breaking** (independent).

### Step 4 — Execute

**Open the gate (before the edit pass).** A PreToolUse hook (`pfm-guard.sh`) denies Edit/Write to `.claude/**` and any `CLAUDE.md` (root or child) unless BOTH session-keyed markers are fresh: reading `quality/prompt.md` stamps the quality marker automatically, and the pfm marker is stamped with the exact command the deny message provides (it carries your session key). Markers slide on every allowed edit — an active session never expires mid-batch; the 1500s TTL reaps only abandoned sessions, and the Stop hook clears this session's markers at turn end. If a write is denied, follow the deny message and retry.

**Agent edit rules:**

- Preserve YAML frontmatter format (`name`, `description`, `tools`)
- Preserve path variables — never hardcode
- Keep step numbering consistent
- Root agent descriptions must match `subagent_type` registry

**CLAUDE.md rules:**

- Keep section hierarchy — agents/commands reference sections by name
- Keep non-negotiable rules exactly as they are

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

### Step 6 — Report

Report, in order: "Infrastructure updated, N files changed" — the changes (what and why) — consistency verified (stale references none/N-fixed; pipeline flow valid; agent definitions consistent) — for a framework change, "Logged to: release.md — {one-line entry}"; for a project customization, "Local source changed directly" — repos touched beyond this one ({BLUEPRINT_CLONE_PATH}, $HOME) with their uncommitted state, or "none" — manual verification needed (list, or "none").

For a release-bound framework change, record the `.professor/release.md` ledger line (§ Where a change lands) before reporting.

---

## Pipeline Consistency Audit

Run when `$ARGUMENTS` starts with `audit`. **Read-only** — reports problems, does NOT fix them.

### Execution model — fan-out agents

Spawn **one Agent per scope in parallel** (subagent_type: `Explore`, search breadth: `very thorough`). Each agent deep-reads its entire domain — follows every reference, reads every file, verifies semantic consistency. PFM aggregates results after all agents return.

**Row tiering within a scope.** Closed-list mechanical rows — frontmatter-parses, path-exists, size limits, executable `+x`, file counts, known-name greps — MAY run as a cheap child (`Explore` or `model: haiku`) against the explicit checklist: coverage lives in the checklist, so a miss surfaces as a missing row, not a silent gap. Semantic rows — description↔behavior match, delegation sanity, route-to validity — stay on the very-thorough walker. Aggregation is unchanged.

**Scope selection:** `audit` or `audit all` → ALL scopes in parallel. `audit {scope}` → single scope.

**Agent brief template** (adapt per scope):

> You are auditing the Professor framework's **{SCOPE}**. Read every file listed. For each check, report one line: `PASS: {detail}` or `FAIL: {detail}` or `WARN: {detail}`. Do NOT fix anything — report only. Follow every reference, read every file, verify every claim. The project root is `{cwd}`.

### Scopes & deep checks

Seven scopes: `agents`, `commands`, `skills`, `pipeline`, `scripts`, `structure`, `cross-refs`. The per-scope checklists live in `docs/commands/pfm/references/audit-scopes.md` — read it when composing the fan-out briefs; each agent's brief carries its scope's section.

### Aggregation

After all scope agents return:

1. Merge per-scope findings into a single report
2. Deduplicate findings that appear in multiple scopes
3. Assign severity: **CRITICAL** (broken reference, missing file, invariant violation — and ALWAYS any claim wrong in the REASSURING direction: promising a protection, isolation, or sanitization the code does not provide), **WARNING** (stale name, size approaching limit, weak inconsistency), **INFO** (style nit, non-blocking)
4. Count totals per severity

### Report format

Shape, in order: title "Pipeline Audit Report — {date}" — summary (scopes audited, agents fanned; total checks / passed / critical / warnings / info) — per-scope results, one PASS/FAIL/WARN-prefixed line per finding — numbered issues with severity badge + suggested fix — verdict: CLEAN, or NEEDS ATTENTION — N critical, M warnings.

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

### File-type laws

Shape: match the existing files of the same kind — the live registry is the template.

- **Sub-agents** (`.claude/agents/*.md`): frontmatter `name` (kebab-case), `description` (one sentence carrying a "when to delegate" phrase — the auto-delegation routing weight), `tools` (minimal allowlist), `model: inherit|opus|sonnet|haiku`. Body IS the system prompt — role sentence, numbered procedure, short checklist, output format; subagents see only their own prompt + env.
- **Slash commands** (`.claude/commands/*.md`): frontmatter `name`, `description` (action verb first; names every subcommand/mode/flag — § Descriptions), `argument-hint`, `disable-model-invocation: true` on side-effect commands. `$ARGUMENTS`/`$1`/`$N` substitute at invocation; a bang-prefixed backticked command (!`cmd`) injects live shell output before Claude sees the prompt.
- **Skills** (`.claude/skills/*/SKILL.md`): frontmatter `name` (lowercase-hyphenated, ≤64 chars, no reserved words anthropic/claude), `description` (what AND when, every mode/trigger named, highest-signal case first, third person, ≤1,024 chars; with `when_to_use` combined ≤1,536). Body: role line, triggers, behavioral steps, 3–5 diverse `### Example` sections, only non-obvious constraints. Skill content stays in context all session and re-attaches after compaction — every line is a recurring tax.

### CLAUDE.md (root + child)

Keep: bash commands Claude can't guess, code-style rules that differ from defaults, architectural decisions / invariants, non-obvious gotchas, repo etiquette / test runners.

NOT: standard language conventions, file-by-file descriptions, "write clean code" platitudes, info Claude can read from the code. **Placement by scope:** root CLAUDE.md carries only rules binding 2+ projects — a rule scoped to one project lives in that project's CLAUDE.md. Child CLAUDE.md files keep only the project-specific delta — never re-declare workspace rules already in root.

**No skill/command rosters.** Claude Code indexes skills and commands itself — it reads every `SKILL.md` and command `description:` at startup and loads a body only on a match. A list of skills or commands in CLAUDE.md is dead weight that rots on every add, so leave it out. CLAUDE.md carries only what auto-indexing can't: **guards** (what's forbidden or must route through a command), **routing decisions** (which handler wins for an ambiguous intent), and **mandatory-load obligations** (when a skill is required at a step). Existence is the filesystem's job; obligation is CLAUDE.md's.

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

- **User-ordered only** — framework files change ONLY on the user's explicit in-session command, never autonomously, never as automation, never as a side effect of other work; an improvement spotted mid-task is proposed, not applied
- **Never break the pipeline** — atomic changes for breaking modifications
- **Never weaken non-negotiable rules** — ethics, privacy, code quality are sacred
- **Never remove safety checks** — QA gates, merge guards, worktree isolation
- **Preserve agent autonomy** — self-contained, no circular dependencies
- **Keep it DRY** — reference CLAUDE.md from agents, don't duplicate
- **Sync across projects** — change in one place = reflect everywhere
- **Prefer deletion over addition** — root's surgical-changes law governs the rest
- **Research before writing** — verify domain content before adding. Structural changes don't need research
- **Always consider token budget** — define once, reference everywhere
- **Routing-gate every fan-out** — spawn agents only for declared scope; the consolidator may demand additions; fall back to full fan-out only when scope is undeclared
- **Every pipeline artifact names its consumer** — before adding a report/file an agent writes, name who reads it downstream; write-only artifacts are banned
- **Delta-structure repeatedly-rewritten state files** — rewritten resume brief on top, append-only archive below a marker; never full-file rewrites
- **Exact-slice agent inputs** — when carving a manifest for parallel agents, each gets its exact slice + a thin shared header; shared contracts are cited by doc + section, never copied
- **Exact per-role read lists in spawn briefs** — "read ALL docs in {dir}/" licenses every agent to read everything; name each role's exact read list
- **One common spawn contract per orchestrator** — hoist rules shared across spawn blocks into a single contract each block references, never restated per block
- **Every check names what its OWN broken state reports** — authoring or editing any instrument that returns a verdict (probe, health check, gate, audit, walker, lint), ask what it reports when IT is broken rather than when the world is clean. Same answer both ways = not a check but a coincidence detector, and it will bless the failure it exists to catch (`kill -0` cannot distinguish a healthy waiter from a reparented deaf one; `PPID ≠ 1` can — a pane capture on the wrong socket returns silence identical to a quiet chat; a capture that cannot reach its target exits non-zero). Build the distinguishing signal INTO the instrument: a law forbidding the mistake is strictly weaker than a check detecting it
