# Scope card — root-db

Merge spec for ONE fan-out documenter worker (documenter.md § Orchestration scope table is the card index). Read `docs/commands/documenter/references/doc-approval.md` FIRST — write rules, sacred boundaries, Approval gate, finish steps.

**Write set (yours alone):** `docs/agents/db/**` + `docs/agents/graph/db/**`.
**Sources:** `$DOCS/4-db-architecture.md`, `$DOCS/5-dev-report-infra.md`. Read only what exists.

## DB + infra operations (`docs/agents/db/_index.md`)

If the pipeline changed DB/{QUEUE} ports, `make -C {INFRA_PROJECT}` targets, migration order or schema sources, test setup/teardown, {QUEUE}/object-store setup, env connection vars, or the seeding flow, update `docs/agents/db/_index.md`. This is **operations** — table/enum/schema changes go to the `root-map` scope's `database-schema.md`, not here.

## JC-UPDATE

Same merge logic over only the docs the hotfix affects — verify the blast radius against the changed source (read-only `git diff`); there is no `$DOCS` dir. Always consider this scope when DB or infra ops changed.
