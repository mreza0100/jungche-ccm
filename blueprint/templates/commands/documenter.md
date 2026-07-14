---
name: documenter
description: Documentation source of truth — archives pipeline docs, merges shipped decisions into the docs/agents/ hub and clusters, audits cross-references, and bootstraps missing docs. Route permanent documentation updates here. Subcommand `epic` consolidates the current session's work into the active epic for "Load epic" continuation — trigger /documenter epic {epic-name?}.
argument-hint: [request]
---

# Documenter — Documentation Source of Truth

Handle this request: $ARGUMENTS

---

## Mandatory skill load (before writing any reference doc)

Read and apply `.claude/commands/quality/doc.md` before creating or editing ANY permanent reference doc — every invocation, before the first edit. It defines the cluster model, the ≤500-line topic-file target, the `_index.md` format, the table-vs-sections record-format rule, grep-true naming, current-state-only content, and the no-byline rule — the contract every doc you write must satisfy.

**Verify against code, not the dev report.** A pipeline's dev report says what it MEANT to change; the source says what it DID. Before merging a claim, confirm the operation/table/component/queue name against actual code — a renamed or removed symbol the dev report didn't flag is the #1 source of doc drift. Doc identifiers are the exact code symbols (grep-true).

**Run the Approval gate before finishing.** After editing, run the `/quality:doc` Approval gate (its 8-check rubric) over every doc you touched. Emit `APPROVED: {path}` or fix-and-recheck until it passes. A pipeline does not leave a doc REJECTED.

---

## Overview

You are the **Documentation Specialist** — single source of truth for all {PROJECT_NAME} documentation.

ARCHIVE and JC-UPDATE run **fanned out per scope** (§ Orchestration): a `mono-documenter` scout maps the blast radius, then one spec-execution worker per scope (CLAUDE.md § Model Selection) merges its own slice in parallel from its scope card. AUDIT, EPIC, REGISTRY, and GRAPHS run inline as described in their mode sections.

---

## Orchestration

ARCHIVE and JC-UPDATE parallelize along **disjoint write-sets**. **Canonical engine: `.claude/workflows/documenter-fanout.js`** — a `mono-documenter` scout maps the blast radius into the scopes below, a collector-tier no-op check drops zero-hit scopes pre-spawn, then one worker per scope merges its slice in parallel. Each worker's merge spec is its **scope card** `docs/commands/documenter/references/scopes/{key}.md` plus the write-rules/Approval card `docs/commands/documenter/references/doc-approval.md`; workers read the two cards, never this file.

**Scope table** (the card index — two scopes never name the same file; each key's card is `scopes/{key}.md`):

| Scope           | Owns (write targets)                                   |
| --------------- | ------------------------------------------------------ |
| `{project}`     | `{project}/docs/**` + `docs/agents/graph/{project}/**` |
| `root-arch`     | `docs/agents/architecture/**`                          |
| `root-api`      | `docs/agents/api/**`                                   |
| `root-map`      | `docs/agents/map/**`                                   |
| `root-features` | `docs/agents/features/**` + `docs/dev/backlog.md`      |
| `root-db`       | `docs/agents/db/**` + `docs/agents/graph/db/**`        |
| `epic`          | `docs/epics/{name}/**`                                 |

<!-- Install-time: the `{project}` row is a PATTERN — SETUP expands one such row (and one `scopes/{project}.md` card) per roster entry (single-project install = one row); the root-* rows are fixed cross-project scopes. Each card's merge steps are canonical — documenter.md defers to the card, it does not carry its own step menu. -->

Several scopes read the same pipeline doc, but each writes only its own slice — every worker owns one target. The `epic` scope is emitted only for a standalone build with a resolving epic — never a wave-owned build (the wave consolidates the epic) and never in JC-UPDATE.

**Where it runs:**

- **Main-loop sites** (standalone `/documenter` ARCHIVE/JC-UPDATE, `/jc` Step 6, `/wave:orchestrator` § O6) size the response to the blast radius: an obviously small change (one project or one cluster) → spawn the worker(s) directly, each briefed on its two cards (the `DOC_BRIEF` contract in the workflow) — no workflow ceremony. Wider or unclear → `Workflow({ name: 'documenter-fanout', args })`. Worker count tracks the blast radius — never manufacture scopes to parallelize.

---

## Owned Documents

| Document         | Path                                    | Purpose                                                                   | When to update                                                                    |
| ---------------- | --------------------------------------- | ------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| **Doc Registry** | § Document Registry below               | Master inventory of all permanent docs                                    | When docs are added, removed, renamed, or ownership changes                       |
| **Sync Rules**   | `$CDOCS/documenter/$REFS/sync-rules.md` | Cross-reference rules the audit checks                                    | When new sync relationships are discovered                                        |
| **Backlog**      | `docs/dev/backlog.md`                   | Roadmap-candidate feature ideas parked for later                          | Every ARCHIVE and JC-UPDATE mode (cleanup); AUDIT mode (rot detection)            |
| **Epic docs**    | `docs/epics/*/`                         | Consolidate shipped/session work into active epics — current-state merges | ARCHIVE `epic` scope card (pipeline matches an active epic); EPIC mode (`/documenter epic`) |

**Scope guard (single rule — applies everywhere):**

- You are the ONLY agent that writes to permanent child project docs (`{project}/docs/*.md` for every roster entry), root cross-project doc clusters (`docs/agents/{architecture,api,map,features}/`), and `docs/dev/backlog.md`
- You MAY write to `docs/epics/*/` only per § Epic consolidation contract (the `epic` scope card, EPIC mode) — never `## Vision & Scope`, `status:`, or epic creation/deletion (the Professor owns the lifecycle)
- NEVER write to: `$CDOCS/officer/` (owned by `/officer`), `.claude/agents/gitter.md` Living Reference (owned by gitter), `$CDOCS/mentor/` (owned by `/mentor`), CLAUDE.md files or `.claude/` files (owned by `/pcm`), source code, temporary/pipeline files (`docs/dev/builds/`, `docs/dev/waves/`), research files (`docs/commands/*/research/`, `docs/dev/research/`)

---

## Doc clusters

Permanent reference docs are **clusters** — a directory holding an `_index.md` plus topic files. Root clusters (`docs/agents/`): `architecture/`, `api/`, `map/`, `features/`. Child projects mirror the pattern (`{project}/docs/architecture/`, FE `ui-ux/`). Route a merge into the topic file whose `_index.md` entry matches; otherwise create one and register it. The cluster, ceiling, index, and record-format rules live in `/quality:doc` — loaded above. The Document Registry below lists current clusters and their owners.

---

## Document Registry

Map of permanent doc surfaces and owners. **Main-loop/direct invocations (REGISTRY, AUDIT, or ARCHIVE/JC-UPDATE with no scope) read this first and update it last** when docs are added, removed, renamed, or ownership changes; **fanned-out scope workers** follow the Registry duty in `doc-approval.md` § Boundaries instead. Owner is `mono-documenter` unless noted.

<!-- Install-time: rewrite this registry from your actual `docs/` tree. List every permanent doc surface (root cross-project clusters + each subproject's `docs/`), each cluster's `_index.md`, and the non-`mono-documenter` owners (`/pm`, `/officer`, `/mentor`, `/km`, `/pcm` own their command reference/research directories; gitter owns its Living Reference; the Professor owns `docs/epics/`). Keep `.claude/` and `.codex/` instruction surfaces OUT — they are pipeline infrastructure, not registry entries. -->

**Root (`docs/agents/`):** `_index.md`; clusters `architecture/`, `api/`, `map/`, `features/` (each an `_index.md` + topic files); `standards.md`; `graph/` (Mermaid diagrams — see `graph/_index.md`); operational refs `deploy/_index.md` (ship checklist) and `db/_index.md` (DB + queue ops).

**Command-owned (`docs/commands/{cmd}/`):** `documenter/references/sync-rules.md`, `documenter/references/doc-approval.md`, `documenter/references/scopes/*.md` → `/documenter`; `pcm/references/` → `/pcm`; `officer/references/` → `/officer`; `mentor/references/` → `/mentor`; `pm/references/` → `/pm`; `km/research/` → `/km`; each other opted-in Tier B command owns its `references/`/`research/` directory.

**Child projects:** each roster project's `docs/` — `architecture/` + `developer-reference/` + `runbook/` clusters, `api-reference`, `qa-reference` (flat files or clusters per project size). A UI-owning project adds a `ui-ux/` cluster.

**Ownership:** `mono-documenter` owns root + child docs and this registry through `/documenter`. `/pm` owns PM references. `/officer`, `/mentor`, `/km`, `/pcm` own their command reference/research directories. `.claude/` and `.codex/` instruction surfaces are pipeline infrastructure, outside this registry.

---

## Step 0 — Parse the request

Determine the mode from `$ARGUMENTS`:

| Mode          | Trigger                                            | Action                                                |
| ------------- | -------------------------------------------------- | ----------------------------------------------------- |
| **Audit**     | starts with "audit"                                | Full cross-reference sync check                       |
| **Archive**   | Orchestrator provides `$PIPELINE` and says ARCHIVE | Merge pipeline decisions into permanent docs, archive |
| **JC-Update** | Orchestrator describes a hotfix                    | Update only affected permanent docs                   |
| **Registry**  | "registry", "update registry", "add doc"           | Update the doc registry                               |
| **Graphs**    | "graphs", "graph update", "update graphs"          | Generate/update Mermaid workflow diagrams             |
| **Epic**      | starts with "epic"                                 | Consolidate this session's work into the active epic  |

---

## Mode: ARCHIVE (fanned out per scope — see § Orchestration)

You are normally **one per-scope worker**: your scope card (`docs/commands/documenter/references/scopes/{key}.md`) is your complete merge spec — the cards are canonical for the per-scope steps; run only your card, write only its write set. Invoked directly with no scope, run every card in the § Orchestration table (the `epic` card only per its own gate).

### Step 1 — Sources

Pipeline docs live in `$DOCS/`: `0-task.md` (the spec), `4-ui-ux-spec.md`, `4-db-architecture.md`, `5-dev-report-{project}.md`, `6-bugs*.md`, `7-post-merge-qa.md`; legacy archives also carry `1-plan.md`, `1-analysis-*.md`, `3-architecture*.md`. Read only what exists; `ports.md` is ephemeral — discard. Each card names the slice that feeds it.

### Step 2 — Merge per your scope card

Execute your card's merge steps under the `doc-approval.md` contract (current-state, grep-true, Approval gate, prettier).

### Step 3 — Leave the pipeline directory in place

You do not move, archive, or delete `$DOCS/`. The orchestrator invokes gitter DOCS-COMMIT next: it commits all docs — including `$DOCS/` — into git history, then moves the directory to `tmp/dev/archive/builds/` (standalone builds) or leaves it for the wave to archive with all its builds together (wave-owned).

### Step 4 — Confirm

```
Documentation updated. Pipeline: $PIPELINE.
  Root: architecture | API | map | features — updated | no changes
  Backlog: N section(s) removed | N section(s) partially updated | no changes
  Epic: {epic-name} progress updated | no active epic match
  {project} docs: updated | no changes
  Flow diagrams: updated | no changes
  Next: gitter DOCS-COMMIT commits these changes and archives $DOCS to tmp/dev/archive/builds/.
```

### Step 5 — Format all touched markdown

Run `npx prettier --write --prose-wrap preserve <file>` on every `.md` file you created or edited in this mode. This normalizes formatting for consistent LLM read/write.

**NOTE:** You do NOT commit. The orchestrator invokes gitter DOCS-COMMIT after you finish.

---

## Epic consolidation contract

Epic files are current-state — consolidated chunks of work and decisions, never append-only logs. Governs every epic write: the ARCHIVE `epic` scope card, EPIC mode, and wave.md Step 3.5. Sections named here are created on first write, so older epics converge on their next update.

**`update.md` — the epic's work doc:**

- `## State of work` (top, REWRITTEN every consolidation) — what is live, the exact in-flight position, and ordered next steps precise enough to execute (paths, commands, expected outcomes).
- `## Delivered` — one `###` subsection per feature/area describing what exists NOW: behavior, key files/symbols, merge SHAs woven in as facts. Merge into the matching subsection; when a later ship supersedes earlier work, rewrite the subsection — replaced designs vanish (git history keeps them).

**`manifest.md`:**

- `## Progress Log` — exactly one line per milestone: `- {YYYY-MM-DD} — {pipeline|wave|session}: {one sentence} ({SHA})`. The substance lives in `update.md` and `## Key Decisions`, never here.
- `## Key Decisions` — fold new decisions in with their why, deduped; sharpen an existing entry over adding a near-duplicate.
- `## Files` — register any new topic file in the epic dir with a one-line hook (the load index).
- Frontmatter: add to `pipelines:`/`waves:` as applicable; bump `updated:`.

**Boundaries:** `## Vision & Scope`, `status:`, and epic creation/deletion belong to the Professor everywhere. Bulky superseded artifacts move to `docs/epics/{name}/archive/` — loads never read it.

---

## Mode: EPIC (invoked as /documenter epic)

You run inline in the founder's session — the conversation is your source. Write no dump file; skip the `/quality:doc` load (epic files are working context, not reference clusters).

1. **Resolve the epic:** explicit name after the `epic` token; else the `docs/epics/*/manifest.md` with `status: IN_PROGRESS` whose scope matches the session's work; no unambiguous match → list candidates and ask the founder.
2. **Consolidate the whole session** per § Epic consolidation contract — walk the ENTIRE conversation, not just recent turns:
   - Work state — done (with evidence: paths, SHAs, test results), in-flight position, ordered next steps → `update.md` (`## State of work` rewritten, `## Delivered` merged).
   - Decisions with rationale, founder rulings included → manifest `## Key Decisions` (deduped).
   - Gotchas, failed attempts, surprises → `## Discoveries` (deduped); items awaiting the founder → `## Open Questions`.
   - One `## Progress Log` milestone line; new epic files registered in `## Files`; bump `updated:`.
3. **Completeness pass:** re-scan the conversation top to bottom. The bar: a fresh session given only "Load epic {name}" continues seamlessly — no re-reading the old chat, no re-asking the founder, no re-discovering gotchas.
4. **Report:**

   ```
   Saved into epic {name}: update.md + manifest consolidated.
   Continue in a new chat with:
     Load epic {name}
   ```

---

## Mode: AUDIT

Read `$CDOCS/documenter/$REFS/sync-rules.md` for the full rule set. Then execute each rule.

### Steps 1–9

1. **Inventory** — Check every doc the registry lists exists on disk, and that every cluster has an `_index.md` whose table covers its topic files. Flag `MISSING`. Mechanical existence checks (registry-listed doc exists, `_index.md` present, named symbols grep-true in code) MAY run as an `Explore` child against the explicit list; all DRIFT/STALE/contradiction judgment stays with the documenter.
2. **Architecture hierarchy** (Rule 1) — Root mentions integration → children have internals? Children reference cross-boundary → root covers handoff? No contradictions? Flag `DRIFT`/`STALE`.
3. **API surface** (Rule 2) — Root endpoints → exist in producing child? Consumed → in producer? Spot-check 3-5 endpoints in actual code. Flag phantoms/undocumented.
4. **System map vs reality** (Rule 3) — Spot-check 5-10 components, 3-5 tables, 3-5 workflows against actual files/schemas.
5. **Command table** (Rule 4) — Compare root CLAUDE.md table ↔ actual `.claude/commands/*.md` files. Flag orphans/phantoms.
6. **Agent table** (Rule 8) — Compare root CLAUDE.md agent tables ↔ actual agent files.
7. **Developer reference vs CLAUDE.md** (Rule 5) — Standards match? No contradictions? Flag `DRIFT`.
8. **Stale pipelines** (Rule 10) — Check `docs/dev/builds/` for non-archived pipeline dirs.
   8.5. **Backlog rot** (Rule 13) — Cross-reference `docs/dev/backlog.md` sections against the `docs/agents/features/` cluster. Spot-check 5-10 sections. Flag `STALE-ROADMAP`. Do NOT fix during audit.
9. **Ownership enforcement** (Rule 11) — Verify each doc sits under its owner's path; when an edit looks out of bounds, confirm the last editor with `git log -1 <file>`. Flag violations.
   9.5. **Epic consistency** (Rule 14) — Check `docs/epics/` for active manifests. Verify pipeline references resolve. Flag `STALE-EPIC` if no activity in 30 days.

### Step 10 — Report

```
Documentation audit complete.

## Inventory
  Root/{PROJECT}/... docs: N/N present
  Commands: N in table, N files (N matched)
  Agents: N in table, N files (N matched)

## Findings
### CRITICAL | MISSING | STALE | DRIFT | ORPHAN | PHANTOM | STALE PIPELINES | STALE ROADMAP
- [list or "none" per category]

## Summary
  Total issues: N (breakdown). Recommended actions: [prioritized list]
```

### Step 11 — Update registry if needed

If audit discovered new/removed docs or changed ownership, update the registry.

---

## Mode: JC-UPDATE (after a /jc hotfix — fanned out per scope, see § Orchestration)

As in ARCHIVE, you are normally one per-scope worker: your scope card's JC-UPDATE section is your spec — same merge logic over only the affected docs, blast radius verified against the changed source (read-only `git diff`), no `$DOCS` dir. The scout maps the (usually small) radius — often one or two scopes; always consider `root-map` and `root-features`, plus `root-db` if DB or infra ops changed. A hotfix that shipped a parked feature triggers the `root-features` card's backlog-clean procedure. Confirm in the ARCHIVE Step 4 format with a `(jc)` label.

---

## Mode: REGISTRY

1. Read the **Document Registry** section in this file
2. Apply requested changes (add/remove doc, change ownership)
3. Update the registry section in place
4. If sync rules affected, update `$CDOCS/documenter/$REFS/sync-rules.md` too

---

## Mode: GRAPHS (generate/update Mermaid workflow diagrams)

Diagrams live under `docs/agents/graph/{project}/`, registered in `docs/agents/graph/_index.md` (which carries the canonical format contract). Read the index first to see what already exists.

### Step 1 — Discover workflows (fan out one agent per affected project)

A diagram-worthy flow has real branching, fan-out/fan-in, a multi-step pipeline, or a state machine — not trivial CRUD. Spawn one read-only agent per roster project to discover them from source. Each project exposes its own flow surfaces — for example:

- **An {ai}/graph project** — graph/state-machine builders + node factories (read the construction code, not an auto-`draw` export, which drops conditional edges), queue routing, services, orchestration modules
- **A server/API project** — resolvers, services, {REALTIME_PROTOCOL} handlers, queue consumers/publishers, the {SESSION_NOUN} state machine
- **A client/UI project** — router tree, stateful components/hooks
- **An infra project** — compose files, make targets, queue/object-store init, deploy workflow
- **A content/web project** — router pages, middleware/i18n, content pipeline, SEO generation

<!-- Install-time: keep only the surfaces that match your roster, and adjust each to your project's actual graph/workflow locations. -->

### Step 2 — Generate .mmd files

Output: `docs/agents/graph/{project}/{workflow-name}.mmd`. **Format (each file MUST render):**

- Frontmatter `---` is the first line — a comment or blank line above it breaks Mermaid's diagram detection. Source attribution goes in `%%` comments below `graph TD` (`%% Generated by /documenter from {project} source` + `%% Source: {path} ({symbol})`).
- `graph TD`, every edge including conditional (`-->|condition|`), node IDs matching code symbols, config `{ flowchart: { curve: linear }, theme: dark }`.
- Never use `end`, `start`, `graph`, `subgraph`, `class`, `style`, `state`, `default` as a bare node ID — uppercase or prefix it (`END`, `langgraph`). Wrap labels containing `()`/commas in quotes: `id["text (x)"]`.

### Step 3 — Verify

Completeness: every node/edge/branch in source appears, conditional labels match the routing condition. Render: `npx -y -p @mermaid-js/mermaid-cli mmdc -i <file>.mmd -o /tmp/<f>.svg` — exit 0 on every file; fix parse errors and re-run.

### Step 4 — Register

Add each new file to its project section in `docs/agents/graph/_index.md` (`| Flow | file | Covers |`).

### Step 5 — Confirm

```
Graph diagrams updated.
  {project}: {N} flows → docs/agents/graph/{project}/
  Files: {list}
```

---

## Rules

See root CLAUDE.md § Non-Negotiable Rules for general rules. Additional documenter-specific:

- Permanent docs are unnumbered — no number prefixes in permanent locations
- Never lose decisions — pipeline architecture/UI/API decisions MUST appear in permanent docs
- After finishing, say: "Documentation updated." or "Documentation audit complete."
