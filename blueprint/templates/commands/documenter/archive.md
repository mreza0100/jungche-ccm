---
name: documenter:archive
description: ARCHIVE mode — merge a completed pipeline's decisions into permanent docs (hub + clusters), clean the backlog, consolidate the active epic, then leave $DOCS for gitter. Invoked by mono-documenter from the build/wave pipeline.
argument-hint: [PIPELINE archive directive]
allowed-tools: Bash(cat:*)
---

# Documenter — ARCHIVE mode

Pipeline: $ARGUMENTS

The shared base and the epic consolidation contract are injected below from their single sources.

!`cat .claude/commands/documenter/_base.txt`

!`cat .claude/commands/documenter/_epic-consolidation-contract.txt`

---

## Step 1 — Read all pipeline documents

All pipeline docs are in `$DOCS/`. Read everything that exists (`{project}` ranges over the roster's per-project suffixes):

- `1-plan.md`, `1-analysis-{project}.md`
- `3-architecture.md`, `3-architecture-{project}.md`
- `4-ui-ux-spec.md`, `4-db-architecture.md`
- `5-dev-report-{project}.md`
- `6-bugs-{project}.md`, `6-bugs.md`, `7-post-merge-qa.md`
- `ports.md` (ephemeral, discard)

<!-- Install-time: the `{project}` suffix expands to one file per roster entry; a single-project install has just one suffix (or none). -->

Only read files that exist.

## Step 2 — Merge decisions into permanent docs

Read pipeline docs and **intelligently integrate** new information. Permanent docs read as "current state" — not changelogs: every step below both adds what the pipeline introduced and deletes or rewrites what it removed, renamed, or superseded. A removed endpoint, component, or feature vanishes from its doc — it does not linger. Route each merge into the topic file whose cluster `_index.md` entry matches; if none fits, create one and register it.

### 2a. `docs/agents/architecture/` cluster (cross-project ONLY)

Source: `3-architecture.md`. **Scope guard:** before adding content, ask "would this fit in `{project}/docs/architecture/`?" If yes, write it there instead. Root = topology + integration contracts only. KEEP: system topology, project boundaries, inter-project data flows, cross-project rules. NEVER: internals of a single project.

Merge into the matching topic file (see the cluster `_index.md`): new integration patterns, cross-boundary data flows, updated roles/access, new contracts. Remove superseded content. Route child-internal details to 2b–2d.

### 2b–2d. Child `architecture/` clusters

For each affected project, read `3-architecture.md` + `3-architecture-{project}.md`. Merge into the matching topic file under `{project}/docs/architecture/` (see the cluster `_index.md`): internal structure, schema/route/chain additions, data flow patterns. Remove superseded content.

### 2e. UI project's `docs/ui-ux/` cluster (skip if no roster entry has a UI)

Source: `4-ui-ux-spec.md`. Merge into the matching topic file under the UI-owning project's `docs/ui-ux/` (see the cluster `_index.md`): design tokens, component designs, screen layouts, interaction patterns, accessibility.

### 2f. `docs/agents/api/` cluster

Sources: `3-architecture.md` (contracts), the `5-dev-report-{project}.md` files whose projects expose an API (API Reference sections).

**Scope:** inter-service communication protocol ONLY — API queries/mutations/subscriptions exposed across boundaries, REST crossing boundaries, messaging events, queue contracts, shared types, error codes, auth headers. NEVER: internal helpers, private endpoints.

Route by surface (see the cluster `_index.md` for the current file set). Consumers grep for an operation, then read its surface file — keep each entry self-contained. Remove superseded entries.

### 2g-1. `docs/agents/features/` cluster

If this pipeline added/modified/removed features: update the matching category topic file (see the cluster `_index.md`). Skip if no feature changes.

### 2g-2. Clean `docs/dev/backlog.md`

Purpose: Remove shipped features from this parking lot. You are the ONLY cleanup mechanism.

Execute:

1. Read full file
2. For each section (§1, §2, …) and each "Refactor / Cleanup Tasks" row, compare against the features cluster entries just added (Step 2g-1) + pipeline dev reports
3. Apply: **SHIPPED in full** → delete section, renumber subsequent. **SHIPPED in part** → rewrite to remaining scope, add `> **Partially shipped {YYYY-MM-DD} ({PIPELINE}):** {summary}`. **NOT SHIPPED** → leave untouched
4. Fix stale references to archived pipeline docs

Match criteria: name overlap, concept overlap, component overlap (same files/chains/types). When in doubt, leave the section.

Skip if Step 2g-1 was skipped. Do NOT add new sections during ARCHIVE.

### 2g-3. `docs/agents/map/` cluster

Merge into the matching topic file (see the cluster `_index.md`): new components, modified workflows, new/changed boundaries, tables, ports, tests, permissions. Delete or rewrite entries for anything removed or renamed. Must reflect actual current state.

### 2h. Child permanent docs from dev reports

For each affected project, read `5-dev-report-{project}.md` and update for what it introduced or removed:

- New endpoints → `docs/api-reference.md`
- New patterns → `docs/developer-reference/` cluster (route into the matching topic file — see the cluster `_index.md`)
- New setup/env vars → `docs/runbook.md`
- New test patterns → `docs/qa-reference.md`
- Removed/renamed endpoint, pattern, or env var → delete or rewrite its entry in the same doc

### 2i. Workflow graph diagrams

If pipeline touched workflow graph definitions (new/changed/removed nodes or edges), regenerate affected `.mmd` files per `/documenter:graphs`. Skip if no graph topology changes.

### 2j. Consolidate into the active epic (standalone builds only)

**Wave-owned builds skip this.** If `grep -rl "$PIPELINE" docs/dev/waves/*/report.md` matches, the wave consolidates its own epic update (wave.md Step 3.5). Print `SKIP-EPIC: $PIPELINE belongs to active wave` and move on.

Resolve the epic: use the `Epic:` value from your invocation (build Step 11 passes `$EPIC`) when it names an epic; otherwise (`none`/absent) match a `docs/epics/*/manifest.md` (`status: IN_PROGRESS`) whose slug the pipeline name contains. Skip only if no epic resolves either way.

Consolidate the pipeline into the matched epic `{name}` per the Epic consolidation contract (injected above): merge what shipped (dev reports + the features added in Step 2g-1) into `update.md` — `## Delivered` per area, `## State of work` refreshed; fold new architectural/scope decisions from `$DOCS/1-plan.md` and `3-architecture.md` into the manifest's `## Key Decisions` (deduped); add one `## Progress Log` line; add `{PIPELINE}` to `pipelines:`; bump `updated:`. `## Discoveries` and `## Open Questions` stay untouched in this mode.

## Step 3 — Leave the pipeline directory in place

You do not move, archive, or delete `$DOCS/`. The orchestrator invokes gitter DOCS-COMMIT next: it commits all docs — including `$DOCS/` — into git history, then moves the directory to `tmp/dev/archive/builds/` (standalone builds) or leaves it for the wave to archive with all its builds together (wave-owned).

## Step 4 — Confirm

```
Documentation updated. Pipeline: $PIPELINE.
  Root: architecture | API | map | features — updated | no changes
  Backlog: N section(s) removed | N section(s) partially updated | no changes
  Epic: {epic-name} progress updated | no active epic match
  {project} docs: updated | no changes
  Hub: updated | no changes
  Next: gitter DOCS-COMMIT commits these changes and archives $DOCS to tmp/dev/archive/builds/.
```

## Step 5 — Format all touched markdown

Run `npx prettier --write --prose-wrap preserve <file>` on every `.md` file you created or edited in this mode. This normalizes formatting for consistent LLM read/write.

**NOTE:** You do NOT commit. The orchestrator invokes gitter DOCS-COMMIT after you finish.
