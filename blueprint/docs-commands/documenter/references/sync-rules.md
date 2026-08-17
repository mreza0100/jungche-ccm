# Documentation Sync Rules

Rules that `/documenter audit` uses to verify cross-reference consistency.
Each rule defines a source, target, and what "in sync" means.

<!-- Install-time: the per-project references below name roster-example roles ({BACKEND_PROJECT} produces the API, {FRONTEND_PROJECT} consumes it, {AI_PROJECT} owns the async chains/{QUEUE} producers, {INFRA_PROJECT} owns services, {WEB_PROJECT} the content site). Expand or prune these to the actual roster; a single-project install collapses the producer/consumer rules to that one project plus root. -->

---

## Rule 1: Architecture Hierarchy

**Source:** `docs/agents/architecture/_index.md` (cross-project big picture; cluster)
**Targets:** each roster project's `{project}/docs/architecture/_index.md` (all clusters)

**Check:**

- Root mentions integration patterns → children have the corresponding internal details
- Children reference services/components → root covers the cross-project handoff
- No contradictions (e.g., root says "REST" but child says "{API_PROTOCOL}" for the same endpoint)
- New components in children that cross project boundaries → must appear in root

---

## Rule 2: API Surface Consistency

**Source:** `docs/agents/api/` (consolidated API surface; cluster)
**Targets:** each API-exposing project's `{project}/docs/api-reference` (flat file or cluster)

**Check:**

- Every endpoint/query/mutation in root → exists in the producing child's api-reference
- Every endpoint consumed by {FRONTEND_PROJECT} → exists in {BACKEND_PROJECT}'s api-reference
- {QUEUE} message types in {AI_PROJECT} → match {BACKEND_PROJECT}'s {QUEUE} publish schemas
- No phantom endpoints (in docs but not in code)
- No undocumented endpoints (in code but not in docs)

---

## Rule 3: System Map vs Reality

**Source:** `docs/agents/map/` (system map; cluster)
**Targets:** Actual codebase files, routes, tables, services

**Check:**

- Components listed in map → corresponding files/directories exist
- Database tables in map → match actual schema files
- Workflows in map → entry points exist in code
- Port mappings in map → match `.env.local` and infra configs
- No phantom entries (in map but deleted from code)
- No major undocumented components (significant files not in map)

---

## Rule 4: Command Table Accuracy

**Source:** Root `CLAUDE.md` command table
**Targets:** `.claude/commands/*.md` (actual command files)

**Check:**

- Every command in CLAUDE.md table → has a corresponding `.claude/commands/{name}.md` file
- Every `.claude/commands/*.md` file → has a corresponding entry in CLAUDE.md table
- Command descriptions in table → reasonably match the command file's purpose
- No orphan commands (file exists, not in table)
- No phantom commands (in table, file missing)

---

## Rule 5: Developer Reference vs CLAUDE.md Standards

**Source:** Child `CLAUDE.md` files (coding standards, conventions)
**Targets:** Each roster project's `{project}/docs/developer-reference/_index.md` cluster indices (all clusters)

**Check:**

- Standards declared in CLAUDE.md → reflected in developer-reference guidance
- No contradictions (CLAUDE.md says "strict mode" but dev-ref shows permissive patterns)
- Dev-ref doesn't invent standards not in CLAUDE.md

---

## Rule 6: Runbook vs Package Config

**Source:** Child `docs/runbook.md` files
**Targets:** Dependency manifest (`package.json` / `pyproject.toml`), `.env.local`, `.env.test`

**Check:**

- Setup steps reference correct dependency manager commands
- Env vars mentioned in runbook → exist in `.env.local` / `.env.test` templates
- Port numbers in runbook → match actual configs
- No stale setup steps for removed dependencies

---

## Rule 7: QA Reference vs Test Infrastructure

**Source:** Child `docs/qa-reference.md` files
**Targets:** Actual test files, test configs, Makefile targets

**Check:**

- Test commands in qa-reference → actually work (scripts/targets exist)
- Coverage targets mentioned → match CI/test configs
- Test patterns described → match actual test file conventions

---

## Rule 8: Agent Table Accuracy

**Source:** Root `CLAUDE.md` agent tables (root + child)
**Targets:** `.claude/agents/*.md`, each roster project's `{project}/.claude/agents/*.md`

**Check:**

- Every agent in CLAUDE.md table → has a corresponding agent file
- Every agent file → has a corresponding entry in CLAUDE.md table
- Agent descriptions → reasonably match agent file purpose
- No orphan agents, no phantom agents

---

## Rule 9: Doc Completeness per Project

**Expected docs per project** (rows are roster-example roles — expand/prune to the actual roster):

| Project             | Required Docs                                                                                                                                                        |
| ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **{BACKEND_PROJECT}**  | `architecture/_index.md` (cluster), `api-reference.md`, `developer-reference/_index.md` (cluster), `qa-reference.md`, `runbook/_index.md` (cluster)                  |
| **{FRONTEND_PROJECT}** | `architecture/_index.md` (cluster), `api-reference.md`, `developer-reference/_index.md` (cluster), `qa-reference.md`, `runbook.md`, `ui-ux.md`                       |
| **{AI_PROJECT}**       | `architecture/_index.md` (cluster), `api-reference/_index.md` (cluster), `developer-reference/_index.md` (cluster), `qa-reference.md`, `runbook/_index.md` (cluster) |
| **{INFRA_PROJECT}**    | `runbook-local.md`, `runbook-test.md`                                                                                                                                |
| **Root**            | `docs/agents/architecture/_index.md` (cluster), `docs/agents/api/_index.md` (cluster), `docs/agents/map/_index.md` (cluster), `docs/agents/features/_index.md` (cluster) |

**Check:** All required docs exist. Flag missing docs as `MISSING`.

---

## Rule 12: Feature Registry Completeness

**Source:** `docs/agents/features/` (feature registry; cluster)
**Targets:** Actual codebase — {API_PROTOCOL} resolvers, REST routes, {FRONTEND_PROJECT} screens, {AI_PROJECT} chains/consumers, {INFRA_PROJECT} services

**Check:**

- Every feature category in registry → has corresponding code implementations
- New resolvers/routes/screens/chains not in registry → flag as `UNDOCUMENTED-FEATURE`
- Features in registry that no longer exist in code → flag as `PHANTOM-FEATURE`
- Categories are logically organized and not overlapping
- Each feature entry includes: name, project(s), brief description, status (active/planned)

---

## Rule 13: Backlog Cleanliness

**Source:** `docs/dev/backlog/backlog.md` (parked roadmap candidates)
**Targets:** `docs/agents/features/` (shipped feature registry; cluster)

**Check:**

- No section in `backlog.md` describes a feature that already exists in the features registry
- Match on concept overlap (name, description, component paths, chain/resolver names)
- Partial matches → section should have a `> **Partially shipped {date} ({pipeline}):** …` header marking what landed
- Refactor/Cleanup rows at the bottom → same rule (if refactor landed, row is removed)
- Archived pipeline references (`tmp/dev/archive/builds/{pipeline}/`) → must still resolve; broken references flagged

**Severity:** `STALE-ROADMAP` — not critical (nothing breaks), but file rot causes future pipelines to re-research solved problems. Clean up during ARCHIVE/JC-UPDATE every time; audit flags accumulated rot.

**Who enforces:** the `root-features` scope card's Clean-backlog section, during ARCHIVE and JC-UPDATE. Audit detects rot; cleanup happens in pipeline/hotfix modes.

---

## Rule 14: Epic Manifest Consistency

**Source:** `docs/epics/*/manifest.md` (active epic manifests)
**Targets:** `docs/agents/features/`, `tmp/dev/archive/builds/`

**Check:**

- Every epic with `status: IN_PROGRESS` → has had a pipeline ship in the last 30 days (otherwise flag `STALE-EPIC`)
- Every pipeline listed in an epic's `pipelines:` frontmatter → exists in `tmp/dev/archive/builds/` or an active wave
- Every epic with `status: SHIPPED` → all features referenced in its Progress Log exist in the features registry
- No two epics claim the same pipeline in their `pipelines:` list

**Severity:** `STALE-EPIC` — informational. Epics may pause intentionally; flag is for awareness, not urgency.

**Who enforces:** the `epic` scope card (progress auto-update) per `documenter.md` § Epic consolidation contract. Audit detects staleness.

---

## Rule 10: No Stale Pipelines

**Source:** `docs/dev/builds/` (non-archive)
**Check:**

- Any pipeline directory that has a `7-post-merge-qa.md` (or equivalent completion marker) but hasn't been archived → flag as `STALE-UNARCHIVED`
- Any pipeline directory with no recent activity → flag as `STALE-ABANDONED`
- `tmp/dev/archive/builds/` should only contain completed pipelines

---

## Rule 11: Ownership Enforcement

**Check all permanent docs for correct ownership:**

- `$CDOCS/officer/` → owned by `/officer`, must NOT be edited by mono-documenter
- `.claude/agents/gitter.md` Living Reference section → owned by gitter, must NOT be edited by anyone else
- `$CDOCS/mentor/` → owned by `/mentor`, must NOT be edited by mono-documenter
- `$CDOCS/documenter/$REFS/` → owned by `/documenter`
- `$CDOCS/pcm/` → owned by `/pcm`
- `$CDOCS/professor/` → owned by Professor (CLAUDE.md)
- `$CDOCS/km/` → owned by `/km`
- All child project `docs/*.md` → owned by mono-documenter (via `/documenter`)
- `.claude/` files → owned by `/pcm`, not documenter

---

## Severity Levels

| Level             | Meaning                                                       | Action                                                                         |
| ----------------- | ------------------------------------------------------------- | ------------------------------------------------------------------------------ |
| **CRITICAL**      | Doc contradicts code or another doc                           | Must fix immediately                                                           |
| **MISSING**       | Required doc doesn't exist                                    | Create stub or full doc                                                        |
| **STALE**         | Doc exists but is outdated vs codebase                        | Update to match reality                                                        |
| **DRIFT**         | Two docs that should sync have diverged                       | Reconcile                                                                      |
| **ORPHAN**        | Doc/command/agent exists without registry entry               | Add to registry or delete                                                      |
| **PHANTOM**       | Registry/table entry for something that doesn't exist         | Remove entry                                                                   |
| **STALE-ROADMAP** | `backlog.md` section describes a feature that already shipped | Remove section (full match) or partial-ship-note the remainder (partial match) |
| **OK**            | In sync, no issues                                            | No action needed                                                               |
