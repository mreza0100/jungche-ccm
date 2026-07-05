# Scope card — root-arch

Merge spec for ONE fan-out documenter worker (documenter.md § Orchestration scope table is the card index). Read `docs/commands/documenter/references/doc-approval.md` FIRST — write rules, sacred boundaries, Approval gate, finish steps.

**Write set (yours alone):** `docs/agents/architecture/**`.
**Sources:** `$DOCS/0-task.md` Contracts + dev reports (current builds); legacy: `3-architecture.md`. Read only what exists.

## Cross-project architecture merge

Root = topology + integration contracts only. KEEP: system topology, project boundaries, inter-project data flows, cross-project rules. Child-internal detail is outside your write set — the child scope owns it; root = topology + integration contracts only.

Merge into the matching topic file (`overview.md`, `integration-contracts.md`, or a per-subsystem file — see the cluster `_index.md`): new integration patterns, cross-boundary data flows, updated roles/access, new {REALTIME_PROTOCOL}/{API_PROTOCOL}/REST/{QUEUE} contracts. Remove superseded content.

## JC-UPDATE

Same merge logic over only the docs the hotfix affects — verify the blast radius against the changed source (read-only `git diff`); there is no `$DOCS` dir.
