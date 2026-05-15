---
name: audit-code-hygiene
version: "1.0.0"
description: "Code hygiene audit — ghost fields, dead code, deps, arch, types, naming, quality. Scopes: all, ghosts, dead, deps, arch, types, naming, quality, or per-project."
---

# Code Hygiene — Audit Sub-Mode

> Systematic code hygiene scan across the codebase.

**Trigger:** `code-hygiene`, `code-hygiene <scope>`, or when `/audit` routes to code hygiene scopes.

**Scopes:** `all`, `ghosts`, `dead`, `deps`, `arch`, `types`, `naming`, `quality`, or per-project scopes matching your subproject names.

Each category is independent — run only applicable ones based on scope.

---

## Category 1 — Ghost Fields & Dual-Writes

Fields, columns, or properties that exist in multiple places for the same concept, are kept in sync manually, or exist as legacy compatibility shims that nobody dares to remove.

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.
> Run `/audit code` or ask the Professor to hydrate after the codebase has enough code to analyze.
> The Professor will surface this gap: "Knowledge base is empty, waiting for user specification to fill it in."

<!-- Detection patterns filled by RR at install time. Examples of what RR discovers:
     - DB schema dual-writes: grep patterns for your ORM (Drizzle, Prisma, SQLAlchemy, etc.)
     - API/DB mismatches: compare API schema fields against DB columns
     - FE fallback chains: ?? or || patterns reading same value from multiple sources
     - Enum duplication: same values defined in multiple layers -->

**Files to check:**

<!-- Filled by RR: schema files, API definitions, FE state management, enum definitions -->

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

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.
> Identify your linter coverage (what's already caught) and focus on what linters CANNOT catch.

<!-- Detection patterns filled by RR. Categories to discover:
     - What your linters already catch (unused imports, vars, etc.) — document to SKIP
     - Unused exports: exported functions/classes with zero imports outside own file
     - Commented-out code blocks (if not caught by linter)
     - Unreachable branches, orphaned files, unused state
     - Per-project chain: which layer calls which — dead ends in the call chain -->

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

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.
> Identify your package managers, dependency files, and config-only dependencies to exclude.

<!-- Detection patterns filled by RR:
     - Package manager(s) and dependency file(s) to scan
     - Config-only dependencies to exclude (babel plugins, webpack loaders, pytest plugins, etc.)
     - DevDependencies leaking into production imports
     - Duplicate functionality packages -->

**Report format:**

```
STALE-DEP: {package_name} in {project}
  Listed in: {dependency file}
  Imports found: {0 | N (list files)}
  Verdict: {remove | keep (used by config) | investigate}
```

---

## Category 4 — Architectural Smells

Patterns that work but are structurally wrong — they'll cause pain as the codebase grows.

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.
> Identify your architecture layers, boundaries, and known anti-patterns.

<!-- Detection patterns filled by RR:
     - Cross-boundary writes (which project owns which tables/resources)
     - God classes/modules (project-specific thresholds and known offenders)
     - Circular dependencies
     - Wrong-layer violations (e.g., SQL in service layer, business logic in resolvers)
     - N+1 query patterns specific to your API layer
     - Copy-pasted logic hotspots -->

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

Places where type strictness is bypassed or types are structurally weak.

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.
> Identify your type system(s), linter rules already covering this, and gaps linters miss.

<!-- Detection patterns filled by RR:
     - Language-specific: `any` (TS), `Any` (Python), etc.
     - Type ignore comments without justification
     - Duplicate type definitions across files
     - Overly broad types (string for known sets, object for structured data)
     - Framework-specific type gaps -->

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

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.
> Identify your naming conventions, domain terminology, and cross-project consistency rules.

<!-- Detection patterns filled by RR:
     - Cross-project naming for key domain terms
     - Method prefix conventions (find/get/list/create/update/delete)
     - Domain terminology drift
     - File naming convention per project/language
     - Snake_case leaking into camelCase contexts (or vice versa) -->

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

**How to detect:**

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.
> Identify what your linters already catch and focus on semantic quality issues they miss.

<!-- Detection patterns filled by RR:
     - Magic strings & numbers (status comparisons, hardcoded values)
     - i18n violations (if applicable)
     - Framework-specific component design violations
     - Overly complex expressions
     - Project-specific quality patterns -->

**Report format:**

```
QUALITY: {issue_type}
  Where: {file:line}
  What: {description}
  Impact: {readability | maintainability | correctness risk}
  Fix: {specific improvement}
```
