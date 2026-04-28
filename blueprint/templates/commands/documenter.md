# Documenter — Documentation Source of Truth

> **Tier C with light Jungche voice.** Mechanics — owns all permanent doc logic. Universal across stacks.

Update permanent docs for: $ARGUMENTS

---

## Overview

`/documenter` is the single source of truth for permanent documentation logic. Permanent docs are long-lived references that survive across pipelines:
- `docs/agents/architecture.md` — cross-project architecture
- `docs/agents/API.md` — inter-service contracts
- `docs/agents/map.md` — system map
- `docs/agents/features.md` — feature registry
- `{project}/docs/architecture.md` — per-project architecture
- `{project}/docs/developer-reference.md` — per-project dev patterns
- `{project}/docs/qa-reference.md` — per-project test patterns
- `{project}/docs/runbook.md` — per-project ops runbook
- `{project}/docs/ui-ux.md` (if applicable) — per-project UI/UX reference

**Only `/documenter` (or the `mono-documenter` agent it delegates to) writes to these files.** Pipeline agents write to ephemeral `docs/dev/tasks/{pipeline}/` only.

---

## Subcommand routing

| Mode | Trigger | Action |
|------|---------|--------|
| **JC-UPDATE** | `$ARGUMENTS` describes a hotfix change | Update permanent docs to reflect the hotfix |
| **ARCHIVE** | invoked by `/build` after pipeline merge | Merge pipeline decisions into permanent docs; archive pipeline directory |
| **AUDIT** | `$ARGUMENTS` starts with `audit` | Cross-reference sync check across all permanent docs |
| *(default)* | freeform | Update permanent docs to reflect a described change |

---

## Pre-flight (every invocation)

Read for context:
- `CLAUDE.md` (root) — project structure, command/agent tables
- `docs/agents/map.md` — current system map (orient)
- `$CDOCS/documenter/$REFS/doc-registry.md` — registry of all permanent docs (which exist, what they cover)
- `$CDOCS/documenter/$REFS/sync-rules.md` — rules for keeping docs in sync

---

## JC-UPDATE Mode

Triggered after a hotfix via `/jc`. The user (or `/jc` Step 6) invokes:

```
/documenter A hotfix was applied via /jc: {description of what changed}. Projects affected: {list}.
```

Action:

1. **Identify affected docs.** Based on the description and project list, identify which permanent docs need updating:
   - Code change → `developer-reference.md` for that project
   - Schema change → `architecture.md` (cross-project), per-project architecture, possibly `API.md`
   - New endpoint → `API.md` + per-project architecture
   - Test pattern change → `qa-reference.md`
   - Workflow change → `map.md`
   - UI change → `ui-ux.md` (if applicable)
2. **Read the changed source files** — verify what actually changed
3. **Update only the relevant docs** — surgical edits, never full rewrites
4. **Skip unaffected docs** — don't touch docs the change doesn't affect
5. **Report changes** — list which doc files were updated and what changed

**Do NOT commit.** `/jc` Step 7 invokes gitter to commit both code and doc changes in two commits.

---

## ARCHIVE Mode

Triggered by `/build` Step 14 after the pipeline's POST-MERGE QA passes. The orchestrator invokes:

```
Agent(mono-documenter): "Phase: ARCHIVE. Pipeline: {name}. ...
  Merge pipeline decisions into permanent docs, archive the pipeline directory."
```

Action:

1. **Read the full pipeline docs** at `docs/dev/tasks/{name}/`:
   - `1-plan.md` (mono-planner)
   - `2-{project}-plan.md` (per-project planners)
   - `3-architecture.md` (mono-architect)
   - `4-{project}-architecture.md` (per-project architects)
   - `5-{project}-runbook.md` (per-project developers, if any)
   - `6-{project}-qa-report.md` (per-project QA)
   - `7-pipeline-audit.md` (code-auditor + officer audit, if applicable)
2. **Identify what's permanent** — which decisions, patterns, contracts, runbook entries belong in long-lived docs
3. **Update permanent docs** — surgical edits only:
   - `docs/agents/architecture.md` — new cross-project patterns or contracts
   - `docs/agents/API.md` — new endpoints, mutations, SQS messages
   - `docs/agents/map.md` — workflow changes
   - `docs/agents/features.md` — new features or scope changes
   - Per-project `architecture.md`, `developer-reference.md`, `qa-reference.md`, `runbook.md`, `ui-ux.md` — per-project changes
4. **Archive the pipeline directory** — `mv docs/dev/tasks/{name}/ docs/dev/tasks/archive/{name}/`
5. **Report changes** — list updated permanent docs, archive location

After ARCHIVE, the orchestrator invokes `gitter Phase: DOCS-COMMIT` to commit the doc changes.

---

## AUDIT Mode

Triggered by `/documenter audit`. Cross-reference sync check across all permanent docs.

Checks:

1. **Existence** — every doc referenced in `$CDOCS/documenter/$REFS/doc-registry.md` exists
2. **Cross-references** — links between docs resolve to existing files
3. **Currency** — does any doc reference a function, file, or pattern that no longer exists in the source?
4. **Coverage gaps** — are there source patterns not documented anywhere?
5. **Sync drift** — are cross-project contracts (`API.md`) consistent with per-project architectures?
6. **Map accuracy** — does `map.md` reflect current code?

Output a structured audit report. Read-only — does NOT fix issues. Ask user if they want fixes via a follow-up `/documenter` invocation.

---

## Freeform Mode

For ad-hoc updates the user requests directly:

```
/documenter Update map.md to reflect the new auth flow we discussed.
```

Read context, identify affected docs, make surgical edits, report changes. Do NOT commit (commits are gitter's domain).

---

## Rules

- **Only mono-documenter writes to permanent docs** — no other agent has permission. If an agent needs a doc updated, it asks `/documenter`.
- **Surgical edits, not rewrites** — preserve unrelated content; touch only what changed
- **Read source before writing docs** — verify what's actually true; the source is truth
- **Never commit** — gitter handles commits; documenter only edits
- **Skip unaffected docs** — don't touch what the change doesn't affect
- **Light Jungche voice in reports** — clean, dry, slightly impressed when docs survive a refactor cleanly
- **Owned reference files** — `$CDOCS/documenter/$REFS/doc-registry.md` and `sync-rules.md` are documenter-owned and updated as the system evolves
