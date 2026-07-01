<!-- ROSTER PATTERN — the per-project child CLAUDE.md, expressed ONCE with {project} tokens.
     SETUP expands this block once per roster entry (a roster of one has NO child CLAUDE.md — its
     conventions live in the root CLAUDE.md). Substitute the entry's name, role, stack, package
     manager, test runner, and ports. Delete sections a given project does not need (a project with
     no database drops § Data Conventions; a pure-infra project drops the two-tier test block).
     Keep ONLY the project-specific delta — NEVER re-declare a workspace rule already in root
     CLAUDE.md (anti-pattern #11). Delete this comment at install. -->

# {PROJECT_NAME} {PROJECT_ROLE}

{PROJECT_STACK}. {TYPING_RULE}, {PACKAGE_MANAGER}.

## Quick Start

```bash
{INSTALL_CMD}
# create .env.local with the vars in § Environment
{DEV_CMD}
```

## Stack

- Runtime: {RUNTIME}
- Framework: {FRAMEWORK}
- Data: {DATA_LAYER} <!-- drop if the project has no datastore -->
- Testing: {TEST_RUNNER}
- Package manager: {PACKAGE_MANAGER}

## Scripts

| Script      | Command         | Description |
| ----------- | --------------- | ----------- |
| `dev`       | {DEV_CMD}       | Dev server  |
| `build`     | {BUILD_CMD}     | Build       |
| `test`      | {TEST_CMD}      | Run tests   |
| `lint`      | {LINT_CMD}      | Lint        |
| `typecheck` | {TYPECHECK_CMD} | Type check  |

## Code Standards

### Logging Convention

- Structured logger only (`{LOGGER_PATH}`) — never raw {RAW_LOG_CALLS}
- Scoped/bound loggers per module with context
- DEBUG at every significant path; `{LOG_LEVEL_ENV}` controls verbosity
- Derived content is still content — anything derived from {DOMAIN_NOUN} data is {SENSITIVE_DATA}. Log `X_length` or `has_X`, never the string.

### File Structure

<!-- A fenced tree or a directory table — whichever reads cleaner for this stack. Show only the
     dirs an agent must know to place code correctly; skip the obvious. -->

```
{PROJECT_TREE}
```

### {FRAMEWORK} Conventions

<!-- The handful of framework-specific rules that differ from defaults or encode a real gotcha.
     One canonical term per concept. No platitudes. -->

- {CONVENTION_1}
- {CONVENTION_2}

### Data Conventions <!-- drop entirely if the project owns no datastore -->

- {DATA_ACCESS_RULE}
- **Schema ownership** — schema design/changes go through the schema-owning agent in the pipeline.
- Never read a migration by name — introspect the live DB.

### Testing Rules

<!-- Two tiers, strict separation. Root CLAUDE.md owns the cross-project mock policy, zero-tolerance
     gates, and parallel-N invariant — restate here ONLY the project-specific mechanics. -->

#### Unit ({UNIT_TEST_DIR})

- {UNIT_RUNNER}; mock ALL external deps — fast, isolated
- Target ≥ 70% coverage; descriptive names

#### Integration ({INTEGRATION_TEST_DIR})

- {INTEGRATION_RUNNER}; mock only external deps, everything within 1 hop runs real
- Setup: `make -C {INFRA_PROJECT} up-test && make -C {INFRA_PROJECT} db-setup-test`; each test seeds its own rows inline
- Runs `{PARALLEL_FLAG}` always — a test that fails at parallel-N is made parallel-safe, never pinned serial
- QA reports `BUG-WRONG-ENV` if any integration test loads `.env.local` as the primary env

### Environment Files

| File         | Purpose           | Infrastructure                                        |
| ------------ | ----------------- | ----------------------------------------------------- |
| `.env.local` | Local development | {INFRA_PROJECT} local — {DB_PORT}                     |
| `.env.test`  | Integration tests | {INFRA_PROJECT} test — {DB_PORT_TEST}, fully isolated |

## Ethics

<!-- The project's slice of sacred ground — the {DOMAIN_ADJ}-safety red lines that apply to THIS
     surface. Concrete, not "be careful". -->

- {ETHICS_RULE_1}
- {ETHICS_RULE_2}
