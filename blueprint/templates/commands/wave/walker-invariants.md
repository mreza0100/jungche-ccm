---
name: wave:walker-invariants
description: The wave walker's invariant registry — durable, machine-readable sacred cross-cutting semantics that a per-wave diff-scoped walk misses by construction. Consumed by the wave-walker engine's scout + invariantHunter + coverageCritic seats via `args.invariants` (see § Consumption Contract below). Guarded, pcm-owned like `walker.md`.
---

# Wave Walker — Invariant Registry

> This ships with ONE illustrative entry. Replace it with YOUR project's real cross-cutting invariants —
> the sacred rules, frozen-record classes, lifecycle state machines, and fail-closed guards a diff-scoped
> walk can't see. A registry that only holds the example arms nothing (the example's exemplar list is
> empty, so it stays a floor); it is here to teach the format, not to seed real coverage.

## § Registry Format

One `##` section per invariant. Each entry:

- **Law** — the invariant's rule, quoted VERBATIM from its CLAUDE.md source, with the source pointer.
  Where no dedicated CLAUDE.md bullet exists yet for a dimension (noted per-entry below), the closest
  codified law is quoted and the gap is flagged.
- **Territory** — glob patterns (`*` = one path segment, `**` = any depth; no brace expansion — list
  alternatives as separate globs) naming where violations of this class live, REGARDLESS of the current
  diff. This is what lets the hunter catch pre-existing bugs no wave ever touches. An entry's territory
  must contain its own class's known instances — a territory narrower than its exemplars makes the
  registry's blind spot the walker's.
- **Triggers** — free-text diff predicates the scout judges semantically (a territory-glob match is the
  zero-token engine-side fail-safe floor beneath this — see `computeArmedInvariants`,
  `.professor/ENGINES/wave-walker/engine/src/engine.ts`).
- **Exemplars** — 2-4 already-confirmed bugs of exactly this class, cited `file:line`. Exemplars are
  what make a finder sharp.
- **Hunt Brief** — the enumeration duty handed to the invariantHunter verbatim.

## § Registration Duty

A wave that INTRODUCES a new invariant (a new sacred rule, a new frozen-record class, a new lifecycle
state machine) registers it here in the SAME wave — a process duty for `/wave:refine`'s spec checklist
and the orchestrator's archive duties, not an engine mechanism. A registry that is never updated is
exactly as blind as no registry.

## § Curation

Who curates — `/pcm` mechanically, or a frontier review per new entry — is OPEN, for founder ruling.
Until ruled, treat any addition to this file as `/pcm`-routed (guarded file) with the SAME rigor as a
CLAUDE.md edit — a bad entry doesn't just fail to help, it can either arm nothing (dead territory
globs) or arm everything (a territory of `**`), both silently.

---

## HONEST-ABSENCE

> Illustrative entry — a domain-agnostic invariant that fits any codebase. Fill Territory/Exemplars with
> your project's real paths and confirmed bugs; delete this note when you do.

**Law:** "An error never renders as ABSENCE — absence is a claim about the world ('no data exists'); an
error is a claim about ourselves ('we failed to look'). Every empty/no-data/degraded state distinguishes
the two. The test: ask what this mechanism would report if it were BROKEN — same answer as 'nothing
here'? That is the bug, found before it reaches a user." — `CLAUDE.md` (root, § Code). A universal
cross-cutting law: it applies to every empty-state render, health check, and gate verdict in any project.

**Territory:**

- `{project}/src/**/*health*`
- `{project}/src/**/*status*`
- `.github/workflows/deploy-*.yml`

**Triggers:** diff touches a probe/gate/healthcheck/empty-state branch; diff adds an error-suppressing
idiom (`2>/dev/null`, `|| true`, `set +e`, a `catch`/`except` with no re-raise).

**Exemplars:** *(cite 2-4 real confirmed bugs of this class once you have them — `file:line` + a one-line
description + severity. Exemplars are what make a finder sharp; an empty list still arms the hunter on
territory + triggers alone.)*

**Hunt Brief:** For every probe/gate/empty-state in the territory, apply the broken-mechanism test: what
does this step report when the thing it checks ERRORS (permission, timeout, malformed output) — does it
read the same as "nothing here" / "all clear"? Enumerate every swallowed exit code, `2>/dev/null`,
`|| true`, and empty catch in the territory and judge each by name — an unnamed swallow is an unjudged
swallow.

---

## § Consumption Contract (`args.invariants`)

The engine never reads this file directly — the JS engine layer has no filesystem access (Workflow
sandbox). The registry's data arrives structured via `args.invariants`, an array of:

```json
{
  "id": "HONEST-ABSENCE",
  "law": "... verbatim law text + source pointer ...",
  "territory": ["{project}/src/**/*health*", "..."],
  "triggers": ["diff touches a probe/gate/empty-state branch", "..."],
  "exemplars": ["health.ts:42 — an upstream error renders as an empty list (HIGH)", "..."],
  "huntBrief": "walk every probe/gate/empty-state for the broken-mechanism test"
}
```

Each field maps directly from this doc's per-entry `**Law:**` / `**Territory:**` / `**Triggers:**` /
`**Exemplars:**` / `**Hunt Brief:**` lines — mechanical, list-to-array transcription. `Configs.
parseInvariants` (`.professor/ENGINES/wave-walker/engine/src/config.ts`) validates the shape and throws
loudly on a malformed entry (missing `id`/`law`/`huntBrief`, empty/non-array `territory`). Absent or `[]`
→ THE FLOOR: no `invariantHunter`/`coverageCritic` dispatched, walker behavior byte-identical to the
registry-less walker. The caller-side transcription duty is documented in `walker.md` § Entry points.
