# Scope card — root-api

Merge spec for ONE fan-out documenter worker (documenter.md § Orchestration scope table is the card index). Read `docs/commands/documenter/references/doc-approval.md` FIRST — write rules, sacred boundaries, Approval gate, finish steps.

**Write set (yours alone):** `docs/agents/api/**`.
**Sources:** `$DOCS/0-task.md` Contracts (legacy: `3-architecture.md`), `$DOCS/5-dev-report-{project}.md` files whose project exposes an API (API Reference sections). Read only what exists.

## Inter-service API cluster

**Scope:** inter-service communication protocol ONLY — {API_PROTOCOL} exposed to consuming projects, REST crossing boundaries, {REALTIME_PROTOCOL} events, {QUEUE} contracts, shared types, error codes, auth headers. NEVER: internal helpers, private endpoints.

Route by surface (e.g. `graphql-queries-*.md`, `graphql-mutations-*.md`, `rest.md`, `websocket.md`, `sse-*.md`, `sqs-*.md`, `shared-types.md` — see the cluster `_index.md` for the current file set). Consumers grep for an operation, then read its surface file — keep each entry self-contained.

## JC-UPDATE

Same merge logic over only the docs the hotfix affects — verify the blast radius against the changed source (read-only `git diff`); there is no `$DOCS` dir.
