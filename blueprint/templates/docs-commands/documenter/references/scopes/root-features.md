# Scope card — root-features

Merge spec for ONE fan-out documenter worker (documenter.md § Orchestration scope table is the card index). Read `docs/commands/documenter/references/doc-approval.md` FIRST — write rules, sacred boundaries, Approval gate, finish steps.

**Write set (yours alone):** `docs/agents/features/**` + `docs/dev/backlog.md`.
**Sources:** `$DOCS/5-dev-report-*.md`, `$DOCS/0-task.md`.

## Features cluster

If this pipeline added/modified/removed features: update the matching category topic file (see the cluster `_index.md`). Skip if no feature changes.

## Clean `docs/dev/backlog.md`

Purpose: Remove shipped features from this parking lot. You are the ONLY cleanup mechanism.

Execute:

1. Read full file
2. For each section (§1, §2, …) and each "Refactor / Cleanup Tasks" row, compare against the features cluster entries just added (above) + pipeline dev reports
3. Apply: **SHIPPED in full** → delete section, renumber subsequent. **SHIPPED in part** → rewrite to remaining scope, add `> **Partially shipped {YYYY-MM-DD} ({PIPELINE}):** {summary}`. **NOT SHIPPED** → leave untouched
4. Fix stale references to archived pipeline docs

Match criteria: name overlap, concept overlap, component overlap (same files/chains/{API_PROTOCOL} types). When in doubt, leave the section.

Skip if the features merge was skipped. Do NOT add new sections during ARCHIVE.

## JC-UPDATE

A hotfix that shipped a parked feature triggers the backlog-clean procedure above; purely a bug fix → features check only.
