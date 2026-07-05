# Scope card — epic

Merge spec for ONE fan-out documenter worker (documenter.md § Orchestration scope table is the card index). Read `docs/commands/documenter/references/doc-approval.md` FIRST — write rules, sacred boundaries, Approval gate, finish steps.

**Write set (yours alone):** `docs/epics/{name}/**`.
**Sources:** `$DOCS/5-dev-report-*.md`, `$DOCS/0-task.md` (legacy: `1-plan.md`/`3-architecture.md`). Read only what exists.

**Governing write contract:** `documenter.md` § Epic consolidation contract — read that section before writing (it stays canonical there; `wave.md` Step 3.5 and `wave/orchestrator.md` also cite it).

## Epic consolidation (standalone builds only)

**Wave-owned builds skip this scope entirely** — it is never emitted for a wave-owned build (the wave consolidates its own epic update via `wave.md` Step 3.5). If `grep -rl "$PIPELINE" docs/dev/waves/*/report.md` matches, print `SKIP-EPIC: $PIPELINE belongs to active wave` and stop.

Resolve the epic: use the `Epic:` value from your invocation (build Step 10 passes `$EPIC`) when it names an epic; otherwise (`none`/absent) match a `docs/epics/*/manifest.md` (`status: IN_PROGRESS`) whose slug the pipeline name contains. Skip only if no epic resolves either way.

Consolidate the pipeline into the matched epic per the Epic consolidation contract: merge what shipped (dev reports + the features the `root-features` scope added) into `update.md` — `## Delivered` per area, `## State of work` refreshed; fold new architectural/scope decisions from `$DOCS/0-task.md` (legacy trails: `1-plan.md`/`3-architecture.md`) into `## Key Decisions` (deduped); add one `## Progress Log` line; add `{PIPELINE}` to `pipelines:`; bump `updated:`. `## Discoveries` and `## Open Questions` stay untouched in this mode.

## JC-UPDATE

Never emitted in JC-UPDATE mode — this scope is standalone-ARCHIVE only.
