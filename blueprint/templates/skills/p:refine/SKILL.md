---
name: p:refine
version: "1.0.0"
description: "Wave task refinement — critically evaluates a task list through the R1-R4 protocol into a ZERO-GAP wave.md (complete technical spec: routing, data model, contracts, file plan, mermaid flow) that delegates no decision to /wave or /build. Interactive discovery, PM + officer consultation, confidence scoring, founder approval gate. Subcommand `poc <goal>` refines a proof-of-concept idea into an airtight spec and hands it to /build or /wave to build a working prototype under RND/POC/."
---

# Refine — Wave Task Refinement

> Critically refine the task list — question, reshape, and strengthen every work item until each is specified completely enough that pipeline agents implement it without making a single decision.

**Trigger:** `refine <tasks>`, `refine this`, `write wave.md`, or when preparing tasks for `/wave`.

**Subcommand:** `refine poc <goal>` runs the **Refine-to-Prototype** flow (§ Subcommand: `poc`) — skip R1–R4. Bare `refine <tasks>` runs R1–R4 below.

## ZERO GAP — the contract (read first)

wave.md is the single source of truth. It leaves **zero decisions** — technical or product — for `/wave` or `/build`: every field, column type, API signature, message shape, file path, route, behavior, and copy string is decided here and written down. Downstream agents execute the spec; they never re-decide, re-scope, or override.

Division of labor: the **founder** answers the main questions (R1.5) and approves the final visual + summary (R4). The **Professor** supplies all technical detail from the codebase walk — the founder is never asked to hand-specify fields or signatures.

Run **R1 → R4 in order**. Every gate is blocking — never skip a step or move past a gate until its exit condition is met.

## Step R1 — Read the codebase first

Read:

- `CLAUDE.md` (root) — system overview, current state
- `docs/agents/architecture/` cluster — how the roster's projects connect, or the single project's internal structure (start at `_index.md`)
- `docs/agents/api/` cluster — **GREP the cluster for specific endpoints/operations the tasks touch** — never read in full
- `docs/agents/graph/db/postgres.mmd` — whole DB schema (tables, columns, FKs) for data-model decisions; names match the database exactly
- `$CDOCS/officer/$REFS/officer.md` — compliance posture, feature inventory, regulatory lines _(if the Officer archetype is installed)_
- Child `CLAUDE.md` files for projects the tasks likely touch

You CANNOT refine tasks without understanding what exists.

### R1 walk — one entry per ORIGINAL task

Walk the actual code **once per original task**. Build a per-task reconciliation note:

| Field                    | What to capture                                                                                           |
| ------------------------ | --------------------------------------------------------------------------------------------------------- |
| **Original #**           | Task number as user wrote it. Preserve through R2/R3.                                                     |
| **Original title**       | Exactly as user wrote it.                                                                                 |
| **Code referenced**      | File paths, components, chains, endpoints this task names or implies. If something doesn't exist, say so. |
| **What exists today**    | One line on current state.                                                                                |
| **What's missing**       | Gap between what's asked and what's in code.                                                              |
| **Concrete-spec status** | `READY` / `NEEDS-CLARIFICATION` / `NEEDS-FOUNDER-SPEC`                                                    |

`NEEDS-FOUNDER-SPEC` tasks **must** surface as Tier-1 questions in R1.5. Never silently merge, drop, or renumber them.

**Why the walk matters:** a wave once shipped with a named task silently replaced by an unrelated step after refinement renumbered tasks. The reconciliation walk prevents this — every original task number must trace through R3 with identity intact.

## Step R1.5 — Interactive Discovery

Talk to the human before evaluating. Ask the RIGHT questions in a single batch.

**Question categories:** Missing specs + scope boundary (TIER 1 — always first), Missing context/WHY, Priority & urgency, Dependencies, Compliance flags, Technical preferences, Behavioral spec gaps, Overlaps & conflicts.

**Tier 1 is mandatory — two gates, surfaced first:** (a) `NEEDS-FOUNDER-SPEC` tasks — founder answers spec / defer / drop. (b) **Scope boundary** — restate the founder's full objective from the whole conversation (not just the trigger args) and confirm what this wave includes versus defers; when the trigger carries only a subset of a broader objective, name the deferred remainder and confirm it is intentionally out. Wave scope traces to the founder's full objective, never narrowing silently to the last thing discussed.

**Format:** `# Professor's Questions — {wave theme}` header. 5-15 questions max. All in ONE message. Don't ask what you can derive from code.

### Confidence scoring (gates exit from R1.5)

Score every task 0-100 after each Q&A round:

| Score  | Meaning      |
| ------ | ------------ |
| 95-100 | READY        |
| 80-94  | MOSTLY-CLEAR |
| 60-79  | PARTIAL      |
| <60    | UNCLEAR      |

**Overall confidence = MINIMUM task score** (not average).

**Gates:** All >= 95 -> proceed to R-POC. All >= 85, min >= 90 -> one final focused round. Any <85 -> mandatory next round.

**Hard cap: 3 rounds.** After Round 3, any task still <95: (a) provide spec now, (b) defer from wave, (c) drop entirely. No "proceed at low confidence."

## Step R-POC — Prototype Validation (founder-gated)

Assess which tasks benefit from a throwaway POC. Pattern-copies and config changes don't need this. Novel features, untested integrations, and complex chains do.

**Recommend POC when:**

- Novel pattern — no existing code to copy from
- Cross-project integration — untested data flow across multiple projects
- Uncertain feasibility — approach "should work" but untested
- Complex chain/prompt — {AI_SERVICE_NAME} behavior needs iteration (if the project has an AI pipeline)
- Ambiguous architecture — multiple valid approaches

Present recommendation, wait for founder approval. Unapproved tasks skip to R2.

For each approved POC, spawn an RND agent in `RND/{poc-name}/`. Run independent POCs in parallel.

### R-POC output — embed findings in wave.md

RND findings are NOT just internal notes — they are **mandatory implementation constraints** for build agents. After RND completes, write a dedicated section in wave.md BEFORE the task list:

```markdown
## RND-Validated Mandatory Rules (ALL tasks MUST follow)

{Preamble: what was tested, what failed without these rules, what succeeded with them.}

**Rule N — {concise rule title}.**
{What to do + why it matters. Include failure mode: "Without this, the model returns X instead of Y."}
```

**What to capture in this section:**

- Every technique that made the difference between success and failure
- Exact prompt language that worked (copy verbatim — build agents won't have RND context)
- Failure modes with numbers (e.g., "positional array → 163/190, indexed → 190/190")
- Non-obvious constraints the model exhibited (skipping, hallucinating, losing count)
- Token/time/cost measurements that inform config choices (max_output_tokens, retries)

**What NOT to capture:** internal RND iteration details, discarded approaches, tooling quirks. Only validated production-relevant findings.

**RND artifacts:** Do NOT delete `RND/`. Build agents and QA may reference the raw outputs for verification. The founder decides cleanup timing.

## Step R2 — Critically evaluate each task

For every task, ask:

| Question                               | Action if "no"                                 |
| -------------------------------------- | ---------------------------------------------- |
| **Well-scoped?**                       | Split into distinct tasks                      |
| **Specific enough?**                   | Rewrite with concrete requirements             |
| **Necessary?** Serves the {USER_NOUN}? | Flag low-priority or remove                    |
| **Feasible at current state?**         | Add prerequisites or flag dependency           |
| **Obvious gaps?**                      | Add missing task with `[PROFESSOR ADDED]` tag  |
| **Overlapping?**                       | Merge into one clear task                      |
| **Scope creep?**                       | Tighten boundaries — state what's NOT included |
| **Compliance line?**                   | Add compliance flags or scope down             |
| **Executable by `/build`?**            | Tag `[CMD: /km]`, `[CMD: /jc]`, etc.           |

## Step R2.5 — PM consultation (founder-gated)

Invoke `/pm wave-consult` with post-R2 task list _(if the PM archetype is installed)_:

- **Bucket A (autonomous):** user-facing names, labels, microcopy. Apply directly, tag `[PM-COPY]`.
- **Bucket B (questions only):** scope changes, kills, defers, behavior changes. Relay verbatim to founder. Tag approved items `[PM-INPUT-APPROVED]`.

PM consultation is mandatory. Bucket split is non-negotiable. If founder doesn't respond, do NOT proceed to R3.

## Step R2.6 — Compliance review (officer, clean agent)

Spawn `/officer` in Advisory mode as a fan-out agent with fresh context (has NOT seen R1-R2.5) — the regulatory lens, separated the same way R2.5 separates the product lens. _(Only if the Officer archetype is installed.)_

```
Agent(general-purpose): "Use the Skill tool to invoke /officer with arguments:
  Advisory — review these refined feature specs for compliance ({REGULATION} / {SENSITIVE_DATA} / consent / data-minimization).
  Tasks: {post-R2 task list, verbatim}.
  Return ADVISORY per-task flags only, each citing its regulatory line. Do NOT design consent mechanisms, schemas, or mandates."
```

Officer is **advisory** — it returns compliance flags, never requirements:

1. Fold returned observations into R3 as `[WATCH: ...]` tags on the affected tasks.
2. Anything that would mandate a new consent scope, schema, or hard requirement → surface to the founder for a decision; never auto-encode it.

## Step R3 — Rewrite with complete technical design (ZERO GAP)

**Identity preservation rules (mandatory):**

1. Every original task traces to: **REFINED** / **MERGED INTO #N** / **DEFERRED** / **DROPPED (founder-approved)**
2. Renumbering allowed, but include Task Reconciliation table mapping every original -> new number / disposition
3. Never silently reuse an original task name for a different concept

### The Professor decides everything (ZERO GAP)

Decide and write down, per task — leaving nothing for `/build` or `/wave` to judge:

- **Routing** — exact set of projects the task touches.
- **Data model** — every new or changed table, column (with exact type), index, enum, constraint.
- **Contracts** — exact API schema (types, inputs, mutations/queries with arg + return types), resolver/handler signatures, message-queue schemas, socket/event payloads.
- **File plan** — every file to create or edit, each with the functions/exports/components it gains and their signatures.
- **Product** — behavior (success/failure/edge), UX, copy, scope.

The Professor derives all of this from the R1 codebase walk — the founder is asked only the main questions (R1.5), never to author fields or signatures. `/wave` keeps all execution discipline — grouping same-routing tasks for token efficiency, pipeline naming, wave ordering/sequencing, parallelism; `/build` only implements. Neither re-decides the spec's content.

### wave.md format

Use **heading-per-task** format. NEVER use markdown tables for task descriptions — task content contains pipe characters (enum definitions, union types) that break table parsing, and 2000+ character cells produce unreadable output after formatting.

```markdown
# Tasks

**Epic:** {kebab-name | `none`}
**Scope:** {the objective this wave delivers}
**Deferred:** {parts of the founder's broader objective this wave intentionally omits — or `none`}

## Task Reconciliation

| Original | Disposition | New # | Notes |
| -------- | ----------- | ----- | ----- |
| ...      | ...         | ...   | ...   |

---

## {Category 1} ({N} tasks)

### Task #{N} — {enhanced title}

`[PM-COPY: ...]` `[WATCH: ...]` (tags on own line)

{One-line summary of what this task does.}

**Why:** {problem or value}

**Routing:** {exact project set}

**Key behaviors:**

(a) {behavior — success path, failure path, edge cases as lettered paragraphs}

(b) ...

**Data model:** {new/changed tables, columns with exact types, indexes, enums, constraints — or "none"}

**Contracts:** {API schema (types/inputs/mutations/queries with arg + return types), resolver/handler signatures, message-queue schemas, socket/event payloads — or "none"}

**File plan:** {every file to create or edit, each with the functions/exports/components it gains and their signatures}

**Technical flow:**

\`\`\`mermaid
{flowchart of the data/control path through the stack — e.g. UI component → API op → backend resolver → service → DB / queue → worker chain → DB → socket → UI}
\`\`\`

**Boundaries:** {what's NOT included}

**Named anchors:** {existing files/components/identifiers to reuse — name them; carry every parity claim with its exact anchor}
```

**Rules:**

- Group by domain/category, number sequentially across all categories
- Separate tasks with `---` horizontal rules
- Flag compliance: `[WATCH: ...]`, `[BLOCKED: ...]`, `[FIXES GAP: ...]`
- Flag command routing: `[CMD: /km]`, `[CMD: /jc]` — mandatory for non-`/build` tasks
- Tables are ONLY for the reconciliation section (short, fixed-width cells)

**Detail quality bar (ZERO GAP) — EVERY task MUST include all of:**

1. What it does — one-line summary
2. Why it matters — `**Why:**`
3. Routing — `**Routing:**` (exact project set)
4. Key behaviors — `**Key behaviors:**` lettered (a), (b), (c)...
5. Data model — `**Data model:**` (exact tables/columns/types, or "none")
6. Contracts — `**Contracts:**` (exact schema / signatures / queue / events, or "none")
7. File plan — `**File plan:**` (every file + functions/exports + signatures)
8. Technical flow — `**Technical flow:**` mermaid diagram
9. Boundaries — `**Boundaries:**`
10. Named anchors — `**Named anchors:**`

A task missing any section is not ZERO GAP — it is not done.

### Constraints

- You MAY write `wave.md` at repo root — the ONLY file you create
- Stamp the target epic at the top of `wave.md` (`**Epic:** {name}`) so `/wave` routes its progress to `docs/epics/{name}/`. Determine it during R1.5 when unclear; write `none` if the work isn't epic-tied
- You do NOT write source files — you specify the complete implementation (data model, contracts, file plan, signatures) in wave.md; `/build` writes the code from your spec
- You MAY add tasks with `[PROFESSOR ADDED]` tag or remove/merge redundant ones
- PM consultation is mandatory (twice) — R2.5 and R3.5 _(when the PM archetype is installed)_
- Task identity is sacred — reconciliation table required
- Confidence-gated R1.5 — every task >= 95% or explicitly dispositioned
- wave.md is not final until the founder approves the R4 gate — proceed R3 → R3.5 → R4

## Step R3.5 — PM second opinion (post-refinement)

Spawn PM as a fan-out agent with fresh context (has NOT seen R1-R3) _(if the PM archetype is installed)_:

```
Agent(general-purpose): "Use the Skill tool to invoke /pm with arguments:
  wave-post-review — Independent post-refinement review of wave.md."
```

1. Present PM's response to founder verbatim
2. Ask founder about incorporating suggestions
3. Apply approved items with `[PM-POST]` tag

R3.5 is mandatory. PM gets only wave.md — fresh eyes. Professor does NOT pre-judge PM's review.

## Step R4 — Founder approval gate (visual + summary)

The founder authored none of the technical detail — R4 is where they see it and rule on it. After R3.5's PM input is folded in, present in ONE message:

1. **Wave-level technical flow** — a single mermaid diagram: every task as a node tagged with its routing, plus the data/control edges and dependencies between tasks. This is the "visual on technical ground."
2. **Decision summary** — lead with the wave's **Scope / Deferred** boundary so the founder approves what is excluded, not only what is built; then one line per task: routing + the key technical decisions the Professor made (data model, contract, approach) + the key product decisions (behavior, scope). Surface every choice the founder did NOT explicitly make.

Then the **founder approves or adjusts.** Apply every adjustment to wave.md (and the affected per-task technical flows); re-present if the change is structural; loop until approved. wave.md is not final and `/wave` must NOT run until the founder approves this gate.

After R4 approval: "Wave file written to `wave.md` with {N} refined tasks (ZERO GAP). Reconciliation: {counts}. R1.5 confidence: {%} in {N} round(s). Compliance (R2.6): {N} WATCH flags. PM input: {counts}. Founder approved the flow + summary at R4. Run `/wave` to execute."

---

## Subcommand: `poc <goal>` — Refine-to-Prototype

`refine poc <goal>` interrogates a proof-of-concept idea into an airtight spec, then hands it to `/build` (or `/wave`) to build a working prototype under `RND/POC/{name}/`.

**POC vs RND:** RND _develops_ — it iterates on a metric until it converges. This _refines_ — it asks the right questions until the spec leaves nothing vague, then delegates the build. The moat is the refinement: a POC proves the right thing only when the spec did.

**POC vs the main flow:** R1–R4 writes root `wave.md` for the real projects (worktrees, merge-to-main). A POC is a self-contained, disposable prototype that lives under `RND/POC/` and exists only to answer "does this approach work?"

**Not the in-flow R-POC step:** § R-POC spawns RND agents to _validate_ a wave task mid-refinement. This subcommand is the whole job — refine a standalone POC, then build it.

Run P1 → P4 in order; every gate blocks.

### P1 — Scope the walk

Read only the code the POC exercises or stubs — the patterns, contracts, or chains it borrows. A POC reuses real anchors where cheap and fakes the rest; decide which is which.

### P2 — Interactive discovery

Ask the founder one focused batch (`# POC Questions — {goal}`):

- **What must it prove?** The single question the POC answers — feasibility, UX, LLM behavior, or integration.
- **Success criteria** — the observable signal that says "it works."
- **Real vs faked** — what must be genuine to prove the point; what may be stubbed.
- **Scope boundary** — what the POC deliberately does NOT do.
- **Stack** — which real projects/libraries it borrows from, or whether it stands alone.

Score the spec on the same scale as § Confidence scoring. Loop until it reaches >= 95 or the founder accepts a lower bar. Hard cap 3 rounds.

### P3 — Write the POC spec

Write `RND/POC/{name}/spec.md` — the ONLY file you create. At the same ZERO-GAP bar as wave.md but scoped to the prototype, it carries: **Goal**, **Proves**, **Success criteria**, **Real vs faked**, **Build plan** (every file to create under `RND/POC/{name}/`, each with what it does and its signatures), **How to run it**, **Boundaries**.

### P4 — Hand off

Recommend the builder — one self-contained probe → `/build`; several parallel probes → `/wave` — with the build target pinned to `RND/POC/{name}/`. Give the founder the exact command; the founder runs it.
