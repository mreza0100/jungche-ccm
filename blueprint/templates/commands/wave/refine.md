---
name: wave:refine
description: Wave task refinement — critically evaluates a task list through the R1-R4 protocol into a ZERO-GAP wave spec at docs/dev/waves/queue/ (complete technical spec: routing, data model, contracts, file plan, mermaid flow) that delegates no decision to /wave:schedule, /wave:orchestrator, or /wave:builder. Subcommand `poc <goal>` refines a proof-of-concept idea into an airtight spec and hands it to /wave:builder or /wave:orchestrator to build a working prototype under .professor/RND/POC/. Triggers: "refine", "refine this", "/wave:refine", "write wave.md", "refine tasks", "refine poc".
---

# Refine — Wave Task Refinement

> Critically refine the task list — question, reshape, and strengthen every work item until each is specified completely enough that pipeline agents implement it without making a single decision.

**Trigger:** `refine <tasks>`, `refine this`, `write wave.md`, or when preparing tasks for `/wave:orchestrator`.

**Subcommand:** `refine poc <goal>` runs the **Refine-to-Prototype** flow (§ Subcommand: `poc`) — skip R1–R4. Bare `refine <tasks>` runs R1–R4 below.

## ZERO GAP — the contract (read first)

The spec — wave.md format, written to `docs/dev/waves/queue/{YYYY-MM-DD}-{slug}.md`; root `wave.md` belongs to `/wave:schedule` alone, so concurrent refines never clobber — is the single source of truth. (Parked wave packages — deferred work carrying its own RND, entered by pointing `/wave:refine` at their manifest when scheduled — live at `docs/dev/backlog/waves/{name}/`; the feature backlog is `docs/dev/backlog/backlog.md`.) It leaves **zero decisions** — technical or product — for `/wave:orchestrator` or `/wave:builder`: every field, column type, API signature, message shape, file path, route, behavior, and copy string is decided here and written down. Downstream agents execute the spec; they never re-decide, re-scope, or override.

Division of labor: the **founder** answers the main questions (R1.5) and approves the final visual + summary (R4). The **Professor** supplies all technical detail from the codebase walk — the founder is never asked to hand-specify fields or signatures.

**Founder interaction:** every founder question in this skill — R1.5 discovery, R-POC approval, R2.5 Bucket B relay, R3.5, the R4 gate, P2 — goes through the `AskUserQuestion` tool. The founder sees ONLY the dialogs — chat prose between them never reaches the founder — so all context (what the walk found, why this needs them) travels inside the question text itself. When their answer is itself a clarification question, the next dialog's question text opens by answering it, simpler and more concrete than the previous attempt (product language, a worked example), never a rephrase. Batch large sets into consecutive calls, ≤4 questions per call, each with concrete options; free-form answers arrive via the built-in "Other".

**Founder-touchpoint forecast (mandatory in R1.5):** every secret to place, live-pipeline/deploy review, destructive-op ratification, and merge nod the wave will EVER need is asked NOW and recorded in the spec as pre-authorized (a `WATCH:`/founder-ratified line per item). God speed's only failure is waiting for the founder — a wave that must stop mid-flight for a founder answer is a failed spec.

Run **R1 → R4 in order**. Every gate is blocking — never skip a step or move past a gate until its exit condition is met.

## Step R1 — Read the codebase first

Read:

- `CLAUDE.md` (root) — system overview, current state
- `docs/agents/architecture/` cluster — how the roster's projects connect, or the single project's internal structure: read `_index.md`, grep the cluster for the subsystems the tasks touch; never read the cluster in full
- `docs/agents/api/` cluster — **GREP the cluster for specific endpoints/operations the tasks touch** — never read in full
- `docs/agents/graph/db/{DATABASE}.mmd` — whole DB schema (tables, columns, FKs) for data-model decisions; names match the database exactly
- `$CDOCS/officer/$REFS/officer.md` — compliance posture, feature inventory, regulatory lines _(if the Officer archetype is installed)_
- Child `CLAUDE.md` files for projects the tasks likely touch

You CANNOT refine tasks without understanding what exists.

### R1 walk — one entry per ORIGINAL task

Walk the actual code **once per original task** — fanned out, never in your own loop: read-only `Explore` (Sonnet) readers, one per subsystem cluster of tasks, each returning its tasks' reconciliation-note fields below as cards with file:line evidence. You judge the cards (spot-check anchors that smell wrong) and author everything downstream — retrieval is the readers', judgment stays yours. Build a per-task reconciliation note:

| Field                    | What to capture                                                                                                                                                                                                                                                                                                                   |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Original #**           | Task number as user wrote it. Preserve through R2/R3.                                                                                                                                                                                                                                                                             |
| **Original title**       | Exactly as user wrote it.                                                                                                                                                                                                                                                                                                         |
| **Code referenced**      | File paths, components, chains, endpoints this task names or implies. If something doesn't exist, say so. A path cited as a DATA SOURCE (fixtures/corpus/records to copy or reuse) is opened to confirm it CONTAINS that data — results-only or empty = NEEDS-CLARIFICATION, resolved in the spec, never left to the builder. |
| **What exists today**    | One line on current state.                                                                                                                                                                                                                                                                                                        |
| **What's missing**       | Gap between what's asked and what's in code.                                                                                                                                                                                                                                                                                      |
| **Reuse targets**        | Existing helpers, components, hooks, types, or query fragments this task must import rather than rebuild — apply `/audit:code-hygiene` Category 8 (Duplication & Missed Reuse) discovery to the task's domain so the spec names what to call, not just what to write. Empty only when the task is genuinely net-new.              |
| **Concrete-spec status** | `READY` / `NEEDS-CLARIFICATION` / `NEEDS-FOUNDER-SPEC`                                                                                                                                                                                                                                                                            |

`NEEDS-FOUNDER-SPEC` tasks **must** surface as Tier-1 questions in R1.5. Never silently merge, drop, or renumber them.

## Step R1.5 — Interactive Discovery

Talk to the human before evaluating. Ask the RIGHT questions in a single batch.

**Question categories:** Missing specs + scope boundary (TIER 1 — always first), Missing context/WHY, Priority & urgency, Dependencies, Compliance flags, Technical preferences, Behavioral spec gaps, Overlaps & conflicts.

**Tier 1 is mandatory — two gates, surfaced first:** (a) `NEEDS-FOUNDER-SPEC` tasks — founder answers spec / defer / drop. (b) **Scope boundary** — restate the founder's full objective from the whole conversation (not just the trigger args) and confirm what this wave includes versus defers; when the trigger carries only a subset of a broader objective, name the deferred remainder and confirm it is intentionally out. Wave scope traces to the founder's full objective, never narrowing silently to the last thing discussed.

**Format:** open with a brief `# Professor's Questions — {wave theme}` explanation of what the walk found, then ask via `AskUserQuestion` batches (Tier 1 first). 5-15 questions max. Don't ask what you can derive from code.

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
- Cross-project integration — untested data flow across 3+ projects
- Uncertain feasibility — approach "should work" but untested
- Complex chain/prompt — LLM behavior needs iteration (if the project has an AI pipeline)
- Ambiguous architecture — multiple valid approaches

Present recommendation, wait for founder approval. Unapproved tasks skip to R2.

For each approved POC, spawn an RND agent in `.professor/RND/{poc-name}/`. Run independent POCs in parallel.

### R-POC output — embed RND findings in wave.md

Applies to ANY wave built from RND work — POCs spawned at R-POC above, or a pre-existing `.professor/RND/` run the tasks originate from. During the walk, study the RND output itself: the result, the iteration trail, and the exact prompts it converged on.

RND findings are NOT just internal notes — they are **mandatory implementation constraints** for build agents. Write a dedicated section in wave.md BEFORE the task list:

```markdown
## RND-Validated Mandatory Rules (ALL tasks MUST follow)

{Preamble: what was tested, what failed without these rules, what succeeded with them.}

**Rule N — {concise rule title}.**
{What to do + why it matters. Include failure mode: "Without this, the model returns X instead of Y."}

**Validated prompt — {name / where it runs}:**

\`\`\`text
{the prompt EXACTLY as RND validated it — full text, byte-identical}
\`\`\`

_Why this wording works: {the behavior it produces; what failed without it}._
```

**What to capture in this section:**

- Every prompt RND validated, EXACTLY as it is — full text in a fenced block, byte-identical, never paraphrased, trimmed, or "improved" — each with the reason that wording works. The wave is the only carrier; build agents reconstruct nothing, and a rewritten prompt is an unvalidated prompt
- COVERAGE is a gate: every key behavior that ADDS an LLM call names its validated prompt artifact here (path + the fenced text); a behavior with no validated prompt is explicitly staged to the wave where its prompt validates, never shipped on an unvalidated one
- Every technique that made the difference between success and failure
- Failure modes with numbers (e.g., "positional array → 163/190, indexed → 190/190")
- Non-obvious constraints the LLM exhibited (skipping, hallucinating, losing count)
- Token/time/cost measurements that inform config choices (max_output_tokens, retries)

**What NOT to capture:** internal RND iteration details, discarded approaches, tooling quirks. Only validated production-relevant findings.

**RND artifacts:** Do NOT delete `.professor/RND/`. Build agents and QA may reference the raw outputs for verification. The founder decides cleanup timing.

## Step R2 — Critically evaluate each task

For every task, ask:

| Question                               | Action if "no"                                 |
| --------------------------------------- | ----------------------------------------------- |
| **Well-scoped?**                       | Split into distinct tasks                      |
| **Specific enough?**                   | Rewrite with concrete requirements             |
| **Necessary?** Serves the {USER_NOUN}? | Flag low-priority or remove                    |
| **Feasible at current state?**         | Add prerequisites or flag dependency           |
| **Obvious gaps?**                      | Add missing task with `[PROFESSOR ADDED]` tag  |
| **Overlapping?**                       | Merge into one clear task                      |
| **Scope creep?**                       | Tighten boundaries — state what's NOT included |
| **Compliance line?**                   | Add compliance flags or scope down to the {REGULATION} boundary |
| **Executable by `/wave:builder`?**     | Tag `[CMD: /km]`, `[CMD: /jc]`, etc.           |

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
3. A condition calling for a legal/compliance-record update NEVER folds into a task deliverable or clause — it routes to the R4 paper-trail list (§ Constraints legal fence).

## Step R3 — Rewrite with complete technical design (ZERO GAP)

**Identity preservation rules (mandatory):**

1. Every original task traces to: **REFINED** / **MERGED INTO #N** / **DEFERRED** / **DROPPED (founder-approved)**
2. Renumbering allowed, but include Task Reconciliation table mapping every original -> new number / disposition
3. Never silently reuse an original task name for a different concept

### The Professor decides everything (ZERO GAP)

Decide and write down, per task — leaving nothing for `/wave:orchestrator` or `/wave:builder` to judge:

- **Routing** — exact set of roster projects the task touches.
- **Build agents** — the conditional specialists the task needs: `db-admin` when it carries a data-model change, `ui-ux` when it carries UI visual work. The build spawns exactly what this line declares — refine wrote the data model, so refine knows.
- **Data model** — every new or changed table, column (with exact type), index, enum, constraint.
- **Contracts** — exact API schema (types, inputs, operations with arg + return types), resolver/handler signatures, message-queue schemas, realtime event payloads.
- **File plan** — every file to create or edit, each with the functions/exports/components it gains and their signatures. Every file marked DELETE is grep-verified single-purpose (top-level defs); a grab-bag or removed-code-interleaved-with-kept file is `EDIT (strip X by def-boundaries)`, never a wholesale/range delete. A dropped column/enum/table names its FULL coupling: every WHERE / ON CONFLICT / caller AND every raw-SQL string reference — SQL column refs are invisible to typecheckers. A removed Settings/config field names its env-var scrub set (every `.env*` variant + infra/deploy carriers). A removal spanning >10 files or >3 layers is declared a FAN-OUT candidate (parallel disjoint sub-slices + a reconcile), never one serial hand.
- **Product** — behavior (success/failure/edge), UX, copy, scope.
- **{SENSITIVE_DATA} channels** — every place the task moves {SUBJECT_NOUN} content. Content reaches the access-controlled DB and nowhere else: a clause routing it to a log, metric label, error string, or telemetry payload is a SACRED-GROUND decision and cannot be written into a mechanism paragraph — it surfaces at the R4 gate as its own line, in plain words ("this logs the {SUBJECT_NOUN}'s verbatim crisis disclosure to the ops stream"), or it does not ship. A risk escalation carries the POINTER (key, segment index, metric), never the text.

`/wave:orchestrator` keeps all execution discipline — task ordering, milestones, per-task verification, gates; `/wave:builder` only implements. Neither re-decides the spec's content.

### Brief-authoring lore (write into the matching task sections, never a loose preamble)

- **km-coupling, both directions** — a task whose code drops/renames a `{slot}` a live prompt references is marked `[CMD: /km]`-coupled up front (persist-hop check both ways); its km-needed spec enumerates DIRECT-template-render tests alongside the consuming chains — each needs a matching inputs-dict update when slots change.
- **Extraction/consolidation tasks** — the done-checklist greps test patch targets for the removed/moved import (`mock.patch("<module>.…")` whitebox patterns); stale patch targets survive targeted re-verification.
- **Module-creating tasks** — the done-checklist greps the tree for forward-ref scaffolds/skip-guards naming the new module (`pytest.importorskip`, `try/except ImportError`, guess-name skips); a scaffold waiting on that name silently re-skips or mis-fires the moment the module lands.
- **Numbering hygiene** — a task editing files that carry old wave-N comments states: stamp the current wave name only, never inherit historical wave/task numbering phrasing.
- **New-principal threat-model delta** — a wave promoting a role to a live authenticating principal carries the delta IN THE SPEC: enumerate every `require{ORG_UNIT}Access`-only (no `requireRole`) resolver and rule the new principal's reachability before merge.
- **Security-remediation briefs** — every read-redaction fix pairs with a write-path check for the same field+role, and every fenced resolver's verification plan includes an adversarial sibling-path judge; self-QA and the named-leak test both miss the sibling read path.

### wave.md format

Use **heading-per-task** format. NEVER use markdown tables for task descriptions — task content contains pipe characters (enum definitions, union types) that break table parsing, and 2000+ character cells produce unreadable output after prettier formatting.

```markdown
# Tasks

**Status:** QUEUED
**Refined:** {YYYY-MM-DD} · main @ {short-sha}
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

**Routing:** {exact roster project set}

**Build agents:** {dev projects [, db-admin] [, ui-ux]}

**Key behaviors:**

(a) {behavior — success path, failure path, edge cases as lettered paragraphs}

(b) ...

**Data model:** {new/changed tables, columns with exact types, indexes, enums, constraints — or "none"}

**Contracts:** {API schema (types/inputs/operations with arg + return types), resolver/handler signatures, message-queue schemas, realtime event payloads — or "none"}

**File plan:** {every file to create or edit, each with the functions/exports/components it gains and their signatures}

**Technical flow:**

\`\`\`mermaid
{flowchart of the data/control path through the stack — e.g. UI component → {API_PROTOCOL} op → backend resolver → service → DB / {QUEUE} → worker chain → DB → {REALTIME_PROTOCOL} → UI}
\`\`\`

**Boundaries:** {what's NOT included}

**Named anchors:** {existing files/components/identifiers to reuse — name them; carry every parity claim with its exact anchor}
```

**Rules:**

- Group by domain/category, number sequentially across all categories
- **Wave train:** tasks spanning more than one code impact area → the top-level grouping becomes wave sections (each `## Wave {k}:` section = one partition = one full downstream wave): dependency-ordered, aggressively few (every extra wave costs a full boundary — two compactions, reconcile, full gates) yet single-area each; tasks number sequentially across waves. **GRAPH WIDTH IS THE THROUGHPUT CEILING — state it, because only this step can.** N linear chains keep at most N builders busy however many run; "aggressively few" buys cheap boundaries and pays in a NARROW graph, and that trade is invisible unless it is written down. So the partition map states its WIDTH (waves runnable in parallel) against the lane count, and where width < lanes it names each SOFT dependency — a shared file surface or shared primitives, never a data/contract edge — as a carve candidate with what the carve would cost. **By dispatch time the graph is FIXED and every surplus hand is permanent**: an idle builder then is a refine-time decision arriving late, and no dispatch can undo it. A rules block binding only some waves' tasks is written inside EACH wave section it governs (self-contained manifests beat DRY); only ALL-task sections sit in the top preamble. Every wave heading MUST carry all four header-block fields — missing any = not a valid partition; `/wave:orchestrator` builds its train map from these blocks alone, never a full-file read:

  ```markdown
  ## Wave {k}: {kebab-area} ({N} tasks)

  **Changes:** {one line — what this wave delivers}
  **Touches:** {project set + key subsystems/dirs}
  **Depends:** {Wave {j} | none}
  **Tasks:** #{a}–#{b} {| list form for non-contiguous sets: #4–#7, #9, #11–#13}
  **Epic:** {name — OPTIONAL fifth line; overrides the source top-level epic for this wave only}
  ```

- Separate tasks with `---` horizontal rules
- Flag compliance: `[WATCH: ...]`, `[BLOCKED: ...]`, `[FIXES GAP: ...]`
- Flag command routing: `[CMD: /km]`, `[CMD: /jc]` — mandatory for non-`/wave:builder` tasks
- Tables are ONLY for the reconciliation section (short, fixed-width cells)

Every task MUST contain all 11 sections shown in the template above (summary, Why, Routing, Build agents, Key behaviors, Data model, Contracts, File plan, Technical flow, Boundaries, Named anchors); missing any = not ZERO GAP.

Mark **milestones**: tag every Kth task-group heading `[MILESTONE]` (in a train: the opening `### Task` heading of the checkpoint span — wave sections replace group headings) — `/wave:orchestrator` runs its checkpoint gate there (cross-project typecheck + affected profiles + main-SYNC + reconcile refresh). Unmarked → `/wave:orchestrator` defaults to every group boundary.

### Constraints

- You write the spec to `docs/dev/waves/queue/{YYYY-MM-DD}-{slug}.md` — the ONLY file you create. `**Status:** QUEUED` + `**Refined:** {date} · main @ {short-sha}` (the HEAD you walked in R1 — `/wave:schedule`'s staleness anchor) head the file
- Stamp the target epic at the top of the spec (`**Epic:** {name}`) so `/wave:orchestrator` routes its progress to `docs/epics/{name}/`. Determine it during R1.5 when unclear; write `none` if the work isn't epic-tied
- You do NOT write source files — you specify the complete implementation (data model, contracts, file plan, signatures) in wave.md; `/wave:builder` writes the code from your spec
- **Legal/compliance documents are sacred ground — FOUNDER-OWNED.** DPIA, DPA, RoPA, privacy policy, consent documents, sub-processor register, data-flow records, feature inventory — anything under `$CDOCS/officer/$REFS/` or of legal character. You NEVER edit one, and wave.md NEVER carries a task, clause, file-plan entry, or `[CMD: /officer]` routing that orders any agent to create or update one. A task whose completion seems to need a records update ships code only; the paper need is listed (exact paths) in the R4 decision summary as a founder-owned paper-trail item _(when the Officer archetype is installed)_
- You MAY add tasks with `[PROFESSOR ADDED]` tag or remove/merge redundant ones
- PM consultation is mandatory (twice) — R2.5 and R3.5 _(when the PM archetype is installed)_
- Task identity is sacred — reconciliation table required
- Confidence-gated R1.5 — every task >= 95% or explicitly dispositioned
- wave.md is not final until the founder approves the R4 gate — proceed R3 → R3.5 → R4

## Step R3.5 — PM second opinion (post-refinement)

Spawn PM as a fan-out agent with fresh context (has NOT seen R1-R3) _(if the PM archetype is installed)_:

```
Agent(general-purpose): "Use the Skill tool to invoke /pm with arguments:
  wave-post-review — Independent post-refinement review of the spec at docs/dev/waves/queue/{file}."
```

1. Present PM's response to founder verbatim
2. Ask founder about incorporating suggestions
3. Apply approved items with `[PM-POST]` tag

R3.5 is mandatory. PM gets only the spec file — fresh eyes. Professor does NOT pre-judge PM's review.

## Step R4 — Founder approval gate (visual + summary)

The founder authored none of the technical detail — R4 is where they see it and rule on it. After R3.5's PM input is folded in, present in ONE message:

1. **Wave-level technical flow** — a single mermaid diagram: every task as a node tagged with its routing, plus the data/control edges and dependencies between tasks. This is the "visual on technical ground."
2. **Decision summary** — lead with the wave's **Scope / Deferred** boundary so the founder approves what is excluded, not only what is built — a train additionally opens with its partition map (the wave header blocks: which impact areas merge in which order); then one line per task: routing + the key technical decisions the Professor made (data model, contract, approach) + the key product decisions (behavior, scope). Surface every choice the founder did NOT explicitly make.

Then the **founder approves or adjusts.** Apply every adjustment to wave.md (and the affected per-task technical flows); re-present if the change is structural; loop until approved. This gate approves the **spec**, not its execution — refine ends when the spec is queued; `/wave:schedule` (which builds the consolidated train for `/wave:orchestrator`) is the founder's separate decision afterward.

After R4 approval, refine is complete — report: "Spec queued at `docs/dev/waves/queue/{file}` with {N} refined tasks (ZERO GAP). Next: `/wave:schedule`. Reconciliation: {counts}. R1.5 confidence: {%} in {N} round(s). Compliance (R2.6): {N} WATCH flags. PM input: {counts}. Founder approved the flow + summary at R4."

---

## Subcommand: `poc <goal>` — Refine-to-Prototype

`refine poc <goal>` interrogates a proof-of-concept idea into an airtight spec, then hands it to `/wave:builder` (or `/wave:orchestrator`) to build a working prototype under `.professor/RND/POC/{name}/`.

This subcommand refines a question-driven spec and delegates the build — distinct from RND (which iterates on a metric until it converges), from R1–R4 (which queues a wave spec for the roster projects), and from the in-flow R-POC step (which validates a wave task mid-refinement); a POC is a self-contained, disposable prototype under `.professor/RND/POC/` that exists only to answer "does this approach work?"

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

Write `.professor/RND/POC/{name}/spec.md` — the ONLY file you create. At the same ZERO-GAP bar as wave.md but scoped to the prototype, it carries: **Goal**, **Proves**, **Success criteria**, **Real vs faked**, **Build plan** (every file to create under `.professor/RND/POC/{name}/`, each with what it does and its signatures), **How to run it**, **Boundaries**.

### P4 — Hand off

Recommend the builder — one self-contained probe → `/wave:builder`; several parallel probes → `/wave:orchestrator` — with the build target pinned to `.professor/RND/POC/{name}/`. Give the founder the exact command; the founder runs it.
