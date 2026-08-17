---
name: wave:refine
description: Wave refinement — walks the code and writes ONE zero-gap, feature-scoped wave spec (every layer of the feature in one wave, tasks ordered inside-out — schema/contracts first, then each project outward to the surface) to docs/dev/trains/queue/{YYYY-MM-DD}-{slug}.md; asks {FOUNDER_NAME} only what the code cannot answer; partitioning and cross-spec ordering belong to the scheduler agent (via /wave:orchestrator). Also invoked BY the scheduler agent in merge mode (two+ approved specs in, one unified spec out, non-interactive). Subcommand `poc <goal>` refines a proof-of-concept idea into an airtight spec and builds it itself under .professor/RND/POC/{name}/ — no worktree, no gitter, no QA gates. Triggers: "refine", "refine this", "/wave:refine", "refine tasks", "refine poc".
---

# Refine — write the wave spec

One deliverable: `docs/dev/trains/queue/{YYYY-MM-DD}-{slug}.md` — the complete executable spec of ONE wave. ZERO GAP: every field, column type, signature, file path, route, behavior, and copy string is decided here. Downstream (the scheduler agent, `/wave:orchestrator`, builder) executes the spec verbatim and never re-decides; a gap found downstream routes back here as a delta, never gets filled there.

## A wave is a FEATURE

A wave carries every layer of one feature together — contracts, schema, and every roster project the feature crosses. A feature is never split by project layer. Tasks are ordered INSIDE-OUT — schema and contracts first, then each project outward to the user-facing surface — so each layer lands on something that already exists. Work belonging to a different feature = a separate spec; the scheduler agent owns ordering and merging across specs.

## R1 — Walk the code

Read: root `CLAUDE.md`, `docs/facts/_index.md`, child CLAUDE.md of the touched projects, `$CDOCS/officer/$REFS/officer.md` _(when the Officer archetype is installed)_; grep how the roster's projects connect on the touched subsystems; introspect the live DB for canonical names. Fan out read-only `Explore` (Sonnet) readers, one per subsystem cluster, each returning per-task cards with file:line evidence:

- Code referenced: paths/components/chains the task names or implies; nonexistent = say so. A path cited as a data source is opened to confirm it contains that data.
- What exists today / what's missing.
- Reuse targets: existing helpers/components/types to import, never rebuild.
- Incumbent patterns: how this codebase already solves the task's mechanism classes.
- Status: `READY` / `NEEDS-CLARIFICATION` / `NEEDS-FOUNDER-SPEC`.

Readers retrieve; you judge and author.

## R2 — Ask {FOUNDER_NAME} until clear

`AskUserQuestion` rounds, ≤4 questions per call. First always: (a) `NEEDS-FOUNDER-SPEC` tasks — spec / defer / drop; (b) scope boundary — restate {FOUNDER_NAME}'s full objective and confirm what this wave includes vs defers; scope never narrows silently. Ask nothing derivable from code. Continue until every task is clear or {FOUNDER_NAME} disposes it (defer/drop) — proceeding unclear is not an option. Forecast every {FOUNDER_NAME} touchpoint the wave will EVER need (secrets, deploy reviews, destructive-op ratifications, merge nods) and record each as pre-authorized in the spec — a wave that stops mid-flight for a {FOUNDER_NAME} answer is a failed spec.

A task needing prototype evidence before it can be specified runs `refine poc` or `/p:rnd` first; the spec embeds the findings (§ RND findings).

## R3 — Decide and write

Settle every technical branch before writing: transport, data placement, mechanism, failure modes, migration. Every mechanism decision first names the incumbent pattern it follows; deviating carries a one-line justification. A branch with 2+ defensible options and materially different consequences goes to {FOUNDER_NAME}; everything else you settle. Per task, decide and write:

- **Routing** — the exact roster project set the task touches + the conditional build agents (`db-admin-{project}` on data-model change, `ui-ux-{project}` on visual work).
- **Data model** — every table, column with exact type, index, enum, constraint.
- **Contracts** — exact {API_PROTOCOL} schema, resolver/handler signatures, {QUEUE} message schemas, {REALTIME_PROTOCOL} event payloads.
- **File plan** — every file to create/edit with the functions/exports it gains and their signatures. DELETE only for grep-verified single-purpose files; otherwise `EDIT (strip X by def-boundaries)`. A dropped column/enum/table names its full coupling including raw-SQL string references; a removed config field names its env-var scrub set. A removal spanning >10 files or >3 layers is declared a fan-out candidate.
- **Behavior** — success/failure/edge paths, UX, copy, scope.
- **{SENSITIVE_DATA} channels** — every place the task moves {SUBJECT_NOUN} content. Content reaches the access-controlled DB and nowhere else; a clause routing it to a log, metric label, error string, or telemetry payload surfaces at the R4 gate as its own plain-words line or does not ship. Escalations carry the pointer, never the text.

### Spec format

Header: `# Wave: {slug}` · `**Status:** QUEUED` · `**Refined:** {YYYY-MM-DD} · main @ {short-sha}` (the HEAD walked in R1 — the scheduler agent's staleness anchor) · `**Epic:** {name | none}` · `**Touches:** {project set}` · `**Scope:**` · `**Deferred:**`. Then `## Task Reconciliation` (Original | Disposition | New # | Notes — every original task traces to REFINED / MERGED INTO #N / DEFERRED / DROPPED). Then tasks in inside-out order, `### Task #{N} — {title}` separated by `---`, each with exactly these sections (`none` allowed):

1. One-line summary + `[WATCH: ...]` flags, then `**Why:**`
2. `**Routing:**` + `**Build agents:**`
3. `**Key behaviors:**` lettered success/failure/edge paths
4. `**Data model:**`
5. `**Contracts:**`
6. `**File plan:**`
7. `**Boundaries & anchors:**` what's NOT included + existing files/identifiers to reuse, every parity claim with its exact anchor

Rules blocks binding a subset of tasks sit above those tasks; all-task rules sit in the header. `[CMD: /km]` / `[CMD: /jc]` tags route non-builder tasks. Tag `[MILESTONE]` on checkpoint task headings.

### RND findings (when RND/POC fed the wave)

Write `## RND-Validated Mandatory Rules` before the task list: every validated prompt in a fenced block, byte-identical — never paraphrased (a rewritten prompt is an unvalidated prompt) — labelled with where it runs; every technique that separated success from failure, with numbers; every behavior adding an LLM call names its validated prompt artifact or is staged to the wave where its prompt validates.

## R4 — Review + {FOUNDER_NAME} gate

- **Officer (MANDATORY on sacred ground):** a wave touching {SENSITIVE_DATA}, consent, retention, auth, or a role boundary gets a fresh-context `/officer` Advisory pass (opus agent); its flags fold in as `[WATCH:]` tags. Anything mandating a new consent scope or schema goes to {FOUNDER_NAME}, never auto-encoded. _(When the Officer archetype is installed.)_
- **Architect (always):** fresh-context `Agent(subagent_type: "architect")` on the spec path only. Present findings to {FOUNDER_NAME} verbatim; apply what he approves.
- **{FOUNDER_NAME} gate:** present the Scope/Deferred boundary plus one line per task (routing + the key technical and product decisions made for him). Loop until approved; approval queues the spec. Running the train (`/wave:orchestrator`) is his separate decision.

**Legal fence (sacred):** DPIA, DPA, RoPA, privacy policy, consent docs, anything under `$CDOCS/officer/$REFS/` or of legal character — you never edit one and the spec never carries a task or clause ordering any agent to; a paper need is listed in the R4 summary as a {FOUNDER_NAME}-owned item.

**Merge mode (scheduler-invoked):** input = two+ already-approved specs ruled one feature. Produce the unified spec non-interactively (no AskUserQuestion — you run inside an agent): union the tasks, re-order inside-out, reconcile overlaps (one reconciliation table covering both sources), keep every RND rule block. A genuine contradiction between the sources is returned as a question for the scheduler's {FOUNDER_NAME} gate, never settled silently.

## Subcommand: `poc <goal>`

Interrogate a proof-of-concept idea into an airtight spec, then build it directly — disposable sandbox under `.professor/RND/POC/{name}/`, no gitter, no QA gates, no worktree.

1. **P1 — Scope:** read only the code the POC exercises or stubs; decide real vs faked.
2. **P2 — Ask {FOUNDER_NAME}** one focused batch: what it must prove, the observable success signal, real vs faked, scope boundary, stack. Loop until clear.
3. **P3 — Spec:** write `.professor/RND/POC/{name}/spec.md` — Goal, Proves, Success criteria, Real vs faked, Build plan (every file + signatures), How to run, Boundaries.
4. **P4 — Build:** dispatch sonnet agents per the build plan (independent probes in parallel); report the result.
