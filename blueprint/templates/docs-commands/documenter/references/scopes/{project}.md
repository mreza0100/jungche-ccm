<!-- Install materializes one {project}.md per roster entry (one per project in the install's roster), each derived from this pattern. -->

# Scope card — {project}

Merge spec for ONE fan-out documenter worker (documenter.md § Orchestration scope table is the card index). Read `docs/commands/documenter/references/doc-approval.md` FIRST — write rules, sacred boundaries, Approval gate, finish steps.

**Write set (yours alone):** `{project}/docs/**` + `docs/agents/graph/{project}/**`.
**Sources:** `$DOCS/0-task.md` (Contracts), `$DOCS/5-dev-report-{project}.md`; legacy archives: `3-architecture.md`, `3-architecture-{project}.md`. Read only what exists.

## Architecture merge (`{project}/docs/architecture/`)

Merge into the matching topic file (see the cluster `_index.md`): internal structure, schema/route/chain additions, data flow patterns. Remove superseded content. Cross-project topology belongs to the `root-arch` scope, not here.

## Permanent docs from the dev report

New endpoints → `{project}/docs/api-reference.md`; new patterns → `{project}/docs/developer-reference/` cluster (matching topic file per its `_index.md`); new setup/env vars → `{project}/docs/runbook/` cluster; new test patterns → `{project}/docs/qa-reference.md`; removed/renamed endpoint, pattern, or env var → delete or rewrite its entry in the same doc.

## Flow diagrams (`docs/agents/graph/{project}/`)

Changed diagrammed flow for this project — resolvers, services, {REALTIME_PROTOCOL} handlers, {QUEUE} consumers/publishers, the {SESSION_NOUN} state machine → regenerate the affected `.mmd` per the format contract in `docs/agents/graph/_index.md`; verify each renders (`npx -y -p @mermaid-js/mermaid-cli mmdc -i <f>.mmd -o /tmp/<f>.svg`, exit 0); keep the `_index.md` rows accurate. Skip if no diagrammed flow changed.

## JC-UPDATE

Same merge logic over only the docs the hotfix affects — verify the blast radius against the changed source (read-only `git diff`); there is no `$DOCS` dir.
