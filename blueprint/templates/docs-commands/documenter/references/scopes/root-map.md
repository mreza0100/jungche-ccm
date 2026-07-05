# Scope card — root-map

Merge spec for ONE fan-out documenter worker (documenter.md § Orchestration scope table is the card index). Read `docs/commands/documenter/references/doc-approval.md` FIRST — write rules, sacred boundaries, Approval gate, finish steps.

**Write set (yours alone):** `docs/agents/map/**`.
**Sources:** `$DOCS/5-dev-report-*.md`, `$DOCS/0-task.md`. Read only what exists.

## System map cluster

Merge into the matching topic file (`components.md`, `workflows.md`, `database-schema.md`, `integration-boundaries.md`, `access-control.md`): new components, modified workflows, new/changed boundaries, tables, ports, tests, permissions. Must reflect actual current state.

## JC-UPDATE

Same merge logic over only the docs the hotfix affects — verify the blast radius against the changed source (read-only `git diff`); there is no `$DOCS` dir. Always check this cluster on a hotfix (documenter.md § Orchestration JC-UPDATE note).
