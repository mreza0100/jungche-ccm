---
name: audit:code-hygiene
description: "Code hygiene audit — duplication & missed reuse, ghost fields, dead code, deps, arch, types, naming, quality, magic numbers. Tuned for AI-authored code. Scopes: all, dup, ghosts, dead, deps, arch, types, naming, quality, magic, diff, sweep, plus one per project in the roster ({project}); the sweep scope additionally removes confirmed-dead code and unused dependencies end-to-end behind QA."
argument-hint: [scope]
---

# Code Hygiene — Audit Sub-Mode

> Systematic code hygiene scan across every project in the roster (one project, or many).

**Trigger:** `code-hygiene`, `code-hygiene <scope>`.

**Scopes:** `all`, `dup`, `ghosts`, `dead`, `deps`, `arch`, `types`, `naming`, `quality`, `magic`, `diff`, `sweep`, plus a per-project scope for each `{project}` in the roster (a single-project repo has just one).

Each category is independent — run only applicable ones based on scope. Scope `diff` restricts every category to a provided changed-file set (e.g., a wave's merged diff) plus the call-sites and imports that touch those files — used by `/wave:review`.

**This codebase is largely AI-authored — weight the checks accordingly.** LLM-written code fails in characteristic ways: it regenerates logic instead of importing what already exists (duplication is the top signal), over-builds simple tasks, leaves stubs and dead branches when it pivots, imports packages that may not exist, and reaches for `any`/broad types and swallowed errors. Before accepting any new function, component, or util, grep for the existing one it should have called.

---

## Sweep Mode — report-only by default, remediate on `sweep`

Every run is **report-only by default**: detect, tier, and STOP — it never deletes. The `sweep` scope (`code-hygiene sweep [{project}|all]`) is the one mode that _removes_ confirmed-dead code and unused dependencies. It governs **Category 2 (Dead Code)** and **Category 3 (Stale Dependencies)** only; every other category stays advisory.

**The end-to-end deadness bar.** A symbol is dead only when it has zero consumers across _the whole roster_ and across every surface a static import-grep misses — because this is a {DOMAIN_ADJ} product, and a false "dead" becomes a regression in a live {SESSION_NOUN}:

- {API_PROTOCOL} contract — the {API_PROTOCOL} schema (SDL/types the UI/client queries by name, never imports)
- {QUEUE} message contracts between roster projects (payload fields the consumer reads, never imports)
- a prompt/asset registry — assets loaded by string name at runtime (e.g. a prompt loader), never statically imported
- {ORM} migrations, {UI_FRAMEWORK} file-routes, and JSON/(de)serialization mappers — registered by name or convention, not import
- Test-only, config-only (babel/webpack/jest/pytest/ruff), and reflection consumers

A candidate that can't be proven past this bar is not dead — flag it and keep it.

**Sweep procedure:**

1. **Detect** Category 2 + 3 candidates per their detection steps below.
2. **Prove deadness** adversarially — for each candidate, try to prove it still _alive_ against the bar above before declaring it dead.
3. **Tier** the survivors: **TIER-1** confirmed-dead (cleared the bar) · **TIER-2** uncertain (a consumer surface unresolved — verify first) · **kept** (a real consumer found).
4. **Approval gate** — present the full tiered kill-list and STOP. Cut only the set the founder approves; never remediate piecemeal mid-scan.
5. **Remove** the approved set in a worktree (never on `main`), run the full QA gate — green tests are the only empirical proof the cut was truly dead — and have gitter merge. Git is the undo.

---

## Category 1 — Ghost Fields & Dual-Writes

Fields, columns, or properties that exist in multiple places for the same concept, are kept in sync manually, or exist as legacy compatibility shims that nobody dares to remove.

**How to detect:**

1. **DB schema dual-writes:** Grep for cases where the same logical value is written to multiple columns or tables. Look for patterns like:
   - Two UPDATE statements in the same function writing similar data
   - Fields with similar names on different tables (e.g., two columns for the same concept)
   - One roster project writing to another project's owned columns (cross-boundary writes)

2. **{API_PROTOCOL}/DB mismatches:** Compare {API_PROTOCOL} schema fields against DB schema columns. Look for:
   - {API_PROTOCOL} fields that don't map to any DB column (computed? stale?)
   - DB columns with no corresponding {API_PROTOCOL} field (dead storage?)
   - Fields that exist on both the {API_PROTOCOL} type AND as a nested resolver

3. **Client-side fallback chains:** Grep UI/client projects for `??` or `||` fallback patterns that read the same value from multiple sources (e.g., `user?.fieldA ?? user?.fieldB`). These indicate a ghost field that should have been consolidated.

4. **Enum duplication:** Check if the same enum values are defined in multiple places (DB enum, {API_PROTOCOL} enum, TypeScript enum, Python enum) and whether they're in sync.

**Files to check (per roster project that applies):**

- the {ORM}/schema definition — all DB columns
- the {API_PROTOCOL} schema — all {API_PROTOCOL} types
- any project's DB-write layer — cross-boundary writes
- UI/client source — fallback chains, dual reads

**Report format per finding:**

```
GHOST: {field_name}
  Where: {file:line} + {file:line}
  What: {description of the duplication}
  Risk: {what breaks if you remove one side}
  Fix: {which side to keep, which to remove}
```

---

## Category 2 — Dead Code

Code that is never called, never imported, or commented out and left to rot.

> **Automated by linters:** unused imports/vars and commented-out code are caught by each project's linter (e.g. `@typescript-eslint/no-unused-vars` in TS projects, Ruff `F401`/`ERA001` in Python; `noUnusedLocals`/`noUnusedParameters` in tsconfig catch dead locals at compile time). This category focuses on what linters CANNOT catch: unused exports, orphaned files, unreachable branches, dead call chains, and unused UI state.

**How to detect:**

1. **Unused exports:** For each project, identify exported functions/classes/constants and grep for their usage. An export with zero imports outside its own file is likely dead. Focus on:
   - Service methods that no resolver calls
   - Utility functions that nothing imports
   - Types/interfaces defined but never referenced
   - Constants defined but never used

2. **Commented-out code blocks (TS projects only):** Grep for large commented-out sections (3+ consecutive lines starting with `//`). Python projects are handled by Ruff `ERA001`.

3. **Unreachable branches:** Look for:
   - `if (false)` or `if (true)` guards
   - Switch cases that can never match
   - Functions that always return early before reaching later code
   - Error handlers for errors that can't occur

4. **Orphaned files:** Files that nothing imports or references. Check:
   - `.ts` files in `src/` with no import statement pointing to them
   - `.py` files with no import and not in `__init__.py`
   - Test files for modules that no longer exist
   - Stale migration files or seed data files

5. **Unused state (UI-project-specific):** Grep for component state (e.g. `useState`) where the setter is never called elsewhere in the component, or the state value is never read.

6. **TODO/FIXME archaeology:** Grep for `TODO`, `FIXME`, `HACK`, `XXX` comments. Check if the referenced work was ever completed elsewhere.

7. **Placeholder stubs (LLM artifact):** Functions left unfinished by AI generation — `throw new Error("not implemented")`, `raise NotImplementedError`, a lone `...` as a TS function body, bare `pass` in a non-empty class method, `// rest of implementation here`. Grep for these and confirm whether the real implementation landed elsewhere or the stub shipped.

**Scope-specific checks (apply to whichever roster project fits the role):**

- **API/service projects:** Check resolvers -> services -> repositories chain. If a repo method exists but no service calls it, and no resolver calls that service method — it's dead.
- **UI/client projects:** Check components. If a component file exists but is never imported in any route, screen, or parent component — it's dead.
- **AI/pipeline projects (if the roster has one):** Check chains. If a chain function exists but the orchestrator never calls it — it's dead. Check prompt templates not referenced by any chain.

**Deadness bar:** a `Safe to remove: yes` verdict — and any sweep cut — holds only when the symbol clears the end-to-end deadness bar (§ Sweep Mode); absence from its own project is suspicion, not proof.

**Report format:**

```
DEAD: {symbol_name} in {file:line}
  Type: {unused export | commented code | orphaned file | unreachable branch | unused state | stale TODO}
  Last meaningful use: {git blame date if helpful, or "never"}
  Safe to remove: {yes | yes but check X first | no because Y}
```

---

## Category 3 — Stale Dependencies

Packages installed but never imported, or imported but outdated/deprecated.

**How to detect:**

1. **Installed but unused:** For each dependency in `package.json` / `pyproject.toml`:
   - Grep the project's `src/` directory for any import of that package
   - If zero imports found, it's a stale dependency
   - Exception: babel plugins, webpack loaders, jest transformers, pytest plugins — these are used by config, not imports. Check config files before flagging.

2. **DevDependencies in production:** Check if any `devDependencies` are imported in `src/` code.

3. **Duplicate functionality:** Multiple packages that do the same thing (e.g., both `axios` and `node-fetch` for HTTP).

4. **Phantom / hallucinated imports (LLM artifact):** An import naming a package NOT in `package.json` / `pyproject.toml`. AI confidently imports packages that don't exist — a supply-chain (slopsquatting) risk. Cross-check every imported package name against the manifest; flag any that resolve to nothing.

**Files to check (per roster project):**

- each project's manifest (`package.json` / `pyproject.toml` / etc.) — declared deps vs actual imports in that project's `src/`

**Report format:**

```
STALE-DEP: {package_name} in {project}
  Listed in: {dependencies | devDependencies | pyproject.toml}
  Imports found: {0 | N (list files)}
  Verdict: {remove | keep (used by config) | investigate}
```

---

## Category 4 — Architectural Smells

Patterns that work but are structurally wrong — they'll cause pain as the codebase grows.

> **Partially automated:** Bare `except Exception:` is caught by Ruff `BLE001`. Unused function args are caught by Ruff `ARG`. God files, god functions, deep nesting, and complexity are NOT in ESLint — they live here because they need semantic context.

**How to detect:**

1. **Cross-boundary writes:** each roster project should only write to the tables it owns. Grep one project's DB code for INSERT/UPDATE to tables owned by another project. Any documented exception (a project writing one or two columns it does not own) should still be flagged as it should be migrated.

2. **God classes/modules:** Classes or modules with too many methods (>15) or mixed responsibilities. Known patterns:
   - Repository classes that combine step-status updates, record saves, and multiple per-feature saves in one class
   - Settings/config models with 30+ fields that should be grouped into nested sub-models

3. **Circular dependencies:** Module A imports from B, B imports from A.

4. **Shallow or unsafe error handling (LLM-prone):** AI optimizes for the happy path. Flag: silent swallowing (`except.*:\s*pass`, empty `catch {}`); over-broad catches (`except Exception`, bare `except:`, `catch (e)`) that neither re-raise nor log the stack trace; resource acquisition (DB connection, cursor, file) with no `finally`/`with` to release on error paths; retry loops with no backoff. Same problem handled differently across files is also a smell.

5. **Missing abstractions / wrong layer:**
   - SQL strings in service layer (should be in repository)
   - Business logic in resolvers (should be in services)
   - **API/service projects:** Resolvers with inline parallel-fan-out (e.g. `Promise.all()`) doing parallel DB queries
   - **UI/client projects:** Components doing {API_PROTOCOL} queries directly instead of through custom hooks

6. **N+1 query patterns:** {API_PROTOCOL} resolvers that trigger a DB query per item in a list.

7. **Over-engineering / speculative generality (LLM-prone):** AI defaults to over-built "enterprise" shapes for simple tasks. Flag an interface/`Protocol`/abstract base class with exactly one implementor or subclass, a factory/builder that always returns one concrete type, a wrapper class that only delegates to one member, a config object passed through layers but with most fields never read, or generics parameterized at a single call site. Only flag when no second consumer is foreseeable. (Duplication itself → Category 8.)

**Report format:**

```
SMELL: {pattern_name}
  Where: {file:line}
  What: {description}
  Impact: {what goes wrong as codebase grows}
  Fix: {recommended refactor}
```

---

## Category 5 — Type Safety Gaps

Places where TypeScript strict mode or Python type hints are bypassed, or where types are structurally weak.

> **Automated by linters:** `@typescript-eslint/no-explicit-any` (warn), `@typescript-eslint/consistent-type-assertions` (warn), `@typescript-eslint/no-non-null-assertion` (warn), `@typescript-eslint/ban-ts-comment` (error, 10 char min). Ruff `PGH` catches `# type: ignore` without error code. This category focuses on what linters CANNOT catch.

**How to detect:**

1. **`Any` usage (Python):** Grep for `: Any`, `-> Any` in Python project source files. Must have justification comment per CLAUDE.md rules.

2. **`# type: ignore` without justification (Python):** Each should have a comment explaining WHY.

3. **Duplicate type definitions:** Same interface/type defined independently in multiple files with different shapes. Grep for identical interface/type names across files.

4. **Overly broad types:** `string` for known sets (should be union/enum), `object` or `{}` for typed data, `dict[str, Any]` for structured data that should be TypedDict/Pydantic.

5. **Double `as any` (TS):** Grep for `as any) as any` — indicates the developer gave up on typing entirely.

6. **Hallucinated API calls (LLM artifact):** AI calls methods/attributes that don't exist or passes wrong argument shapes — confident, plausible, and sometimes type-checking against loose types. Run `tsc --noEmit` and grep its output for "Property … does not exist"; for Python projects run `mypy`/`pyright` and look for missing-attribute errors. Pay special attention to recently changed or low-frequency third-party APIs.

**Report format:**

```
TYPE-GAP: {type} in {file:line}
  Code: {the offending line}
  Risk: {what could go wrong at runtime}
  Fix: {proper type to use, or "add justification comment"}
```

---

## Category 6 — Naming Inconsistencies

Same concept with different names across projects, or naming that doesn't follow conventions.

**How to detect:**

1. **Cross-project naming:** Same domain concept should have the same name everywhere. Check key domain terms across every project in the roster.

2. **Service method prefix inconsistency (API/service projects):** `find*` for read-by-criteria (repo), `get*` for read-by-id (service), `list*` for read-all/paginated, `create*`/`add*` for inserts, `update*` for modifications, `delete*`/`remove*` for deletions.

3. **Domain terminology drift (UI/client projects):** "{SESSION_NOUN}" vs "appointment", "{Entity}" vs "Person" vs "People", etc.

4. **File naming convention violations:** follow each project's own convention (e.g. TS API projects `kebab-case.ts`; TS UI projects `PascalCase.tsx` for components, `camelCase.ts` for utilities; Python projects `snake_case.py`).

5. **Snake_case leaking into TypeScript:** When TS types mirror DB columns, snake_case field names leak in.

6. **Inconsistent error code naming (API/service projects):** Error constants that don't match the actual entity.

7. **Boolean parameter naming:** Bare `consent: boolean` or `force: boolean` — ambiguous at call sites.

8. **Scope-dishonest destructive names:** a delete/erase/clear/reset op named for a broader scope than it performs — `delete{Subject}` that removes only {subject}-level rows (not the {subject} record or {session}-level data), `clearCache` that clears one key. Name it for what it actually touches (`delete{Subject}AnalysisData`). Highest-risk on erasure / {REGULATION} Art. {N} paths: a partial delete behind a total-sounding name is a compliance trap a future caller wires straight to.

**Report format:**

```
NAMING: {the inconsistency}
  Places: {file:line}, {file:line}, ...
  Convention: {what it should be}
  Fix: {rename A to B, or rename B to A}
```

---

## Category 7 — Code Quality & Clean Design

Readability, maintainability, and design patterns. These issues don't cause bugs today — they cause bugs tomorrow.

> **Automated by linters:** Nested ternaries (`no-nested-ternary` error), `console.*` (`no-console` error), `print()` (Ruff `T20`), line length (`max-len: 120` warn / Ruff `E501`). This category focuses on what linters CANNOT catch.

**How to detect:**

1. **Magic strings & numbers:** Literal values used directly in logic instead of named constants.
   - **Domain value-set comparisons:** role/status/type literals compared as raw strings (`=== '{ROLE_USER}'`, `=== "{STATUS_LITERAL}"`) — a fixed value-set must be a typed enum referenced everywhere, never a string retyped per site. A value-set with no enum at all (grep the same literals across many files) is the root finding, not a per-site nit.
   - **Hardcoded hex colors (UI projects):** `#[0-9a-fA-F]{3,8}` in component files — should use theme classes
   - **Magic numbers:** Timeout values, retry counts, buffer sizes without named constants

2. **Hardcoded i18n strings (UI projects):** user-visible string literals in markup that bypass the translation function (e.g. `t()`) — text-element children, ternary copy (`cond ? 'one' : 'other'`), accessibility labels. Grep the changed UI set for these; plurals must use the i18n library's plural keys (e.g. `_one`/`_other`), never an inline ternary. This is recall-prone in prose review (a hardcoded singular/plural pair once shipped past both this audit and UI QA) — the durable backstop is a lint guard (e.g. `react/jsx-no-literals` scoped to text components); flag its absence rather than relying on the human eye.

3. **UI component design violations:**
   - **Inline sub-components:** Function/const component declarations inside another component — re-create on every render
   - **Data fetching in presentation components:** queries/mutations inside modals/leaf components
   - **Missing memoization on expensive callbacks:** Callbacks passed as props without `useCallback` (or the framework equivalent)

4. **Python `__init__.py` hygiene (Python projects):** Empty when they should export, or stuffed with logic when they should be thin.

5. **Overly complex expressions:** Chained optional access >3 levels, long boolean conditions that should be extracted.

**Report format:**

```
QUALITY: {issue_type}
  Where: {file:line}
  What: {description}
  Impact: {readability | maintainability | correctness risk}
  Fix: {specific improvement}
```

---

## Category 8 — Duplication & Missed Reuse (DRY)

The top failure mode of AI-authored code: it regenerates logic instead of importing what already exists, so the same function, component, hook, type, or query fragment gets written many times instead of once-and-called. Clones carry their source's bugs to every copy and drift apart over time. Highest-yield category — run it first.

**How to detect:**

1. **Reinvented helpers (the core check):** For each new or changed function/util, grep the codebase for an existing export that already does the job. AI writes a fresh `formatDate`/`validateEmail`/`apiClient` because it never searched for the one in `utils/`. Signal: inline logic (date formatting, HTTP calls, validation, DB session creation) appearing outside the project's designated `utils/`/`services/`/`hooks/`/repository location.

2. **Near-duplicate components (UI projects):** Two component files with near-identical import lists and markup structure (e.g., `UserCard` vs `{Role}Card`) — should be one parametric component with a `variant`/`role` prop.

3. **Duplicated hooks (UI projects):** The same `[data, loading, error]` async-state body repeated across components instead of one shared `useAsync`/`useResource` hook. Grep for repeated async-state + fetch patterns outside `hooks/`.

4. **Repeated query fragments:** The same {ORM} `.where(eq(...))` or {AI_FRAMEWORK} `.filter(...)` clause at ≥3 call sites — extract a shared query helper.

5. **Duplicate definitions:** Same Pydantic model, {API_PROTOCOL} type, {AI_FRAMEWORK} chain `description=`, tool signature, constant, or `beforeEach` test-setup body defined in multiple files. Grep symbol/description names for cross-file duplicates.

6. **Near-duplicate-with-variation:** Copies that look identical now but will drift — flag them before one gets fixed and the others don't.

7. **Repeated membership/permission predicates:** the same multi-value test (`role === '{ROLE_USER}' || role === '{ROLE_SUPER}'`, status-set checks) written at ≥2 sites — extract one named predicate (`canRecord{Session}(role)`) so the policy lives in one place and call sites read intent, not values. Divergent copies of the "same" check are an authorization-drift bug, not just duplication.

**Detection aids (use what's installed; fall back to grep):** `jscpd --min-lines 5 --pattern "**/*.{ts,tsx}"` for TS/React; `pylint --enable=duplicate-code` or PMD CPD for Python. A git add/delete ratio above ~2.5 over recent history signals new code piling up instead of replacing old.

**Report format:**

```
DUP: {what is duplicated}
  Copies: {file:line} ↔ {file:line} [↔ ...]
  Existing original: {file:line if one already existed, else "none — N parallel copies"}
  Drift risk: {what breaks when one copy changes and the others don't}
  Fix: {extract to {location}, call it from each site}
```
