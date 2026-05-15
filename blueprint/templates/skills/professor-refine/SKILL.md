---
name: professor-refine
version: "1.0.0"
description: "Wave task refinement — critically evaluates a task list through R1-R3.5 protocol, produces wave.md. Interactive discovery, PM consultation, confidence scoring."
---

# Refine — Wave Task Refinement

> Critically refine the task list — question, reshape, and strengthen every work item. Every task must be worth building and described well enough for pipeline agents to build it right.

**Trigger:** `refine <tasks>`, `refine this`, `write wave.md`, or when preparing tasks for `/wave`.

## Step R1 — Read the codebase first

Read:

- `CLAUDE.md` (root) — system overview, current state
- `docs/agents/architecture.md` — how subprojects connect
- `docs/agents/API.md` — **GREP for specific endpoints/mutations the tasks touch** — never read in full
- Child CLAUDE.md files for projects the tasks likely touch

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

## Step R1.5 — Interactive Discovery

Talk to the human before evaluating. Ask the RIGHT questions in a single batch.

**Question categories:** Missing specs (TIER 1 — always first), Scope clarification, Missing context/WHY, Priority & urgency, Dependencies, Compliance flags, Technical preferences, Behavioral spec gaps, Overlaps & conflicts.

**Tier 1 is mandatory:** `NEEDS-FOUNDER-SPEC` tasks surfaced first. Founder answers: (a) spec, (b) defer, (c) drop.

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

Present recommendation, wait for founder approval. Unapproved tasks skip to R2.

For each approved POC, spawn an RND agent. Run independent POCs in parallel. After wave.md is written: clean up POC artifacts.

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

Invoke `/pm wave-consult` with post-R2 task list:

- **Bucket A (autonomous):** user-facing names, labels, microcopy. Apply directly, tag `[PM-COPY]`.
- **Bucket B (questions only):** scope changes, kills, defers, behavior changes. Relay verbatim to founder. Tag approved items `[PM-INPUT-APPROVED]`.

PM consultation is mandatory. Bucket split is non-negotiable.

## Step R3 — Rewrite with depth

**Identity preservation rules (mandatory):**

1. Every original task traces to: **REFINED** / **MERGED INTO #N** / **DEFERRED** / **DROPPED (founder-approved)**
2. Renumbering allowed, but include Task Reconciliation table mapping every original -> new number / disposition
3. Never silently reuse an original task name for a different concept

### wave.md format

Use **heading-per-task** format. NEVER use markdown tables for task descriptions.

```markdown
# Tasks

## Task Reconciliation

| Original | Disposition | New # | Notes |
| -------- | ----------- | ----- | ----- |

---

## {Category 1} ({N} tasks)

### Task #{N} — {enhanced title}

`[PM-COPY: ...]` `[WATCH: ...]` (tags on own line)

{One-line summary of what this task does.}

**Why:** {problem or value}

**Key behaviors:**

(a) {behavior — success path, failure path, edge cases as lettered paragraphs}

**Architectural intent:** {non-obvious choices stated explicitly}

**Boundaries:** {what's NOT included}

**Named anchors:** {file paths, components, identifiers}
```

### Constraints

- You MAY write `wave.md` at repo root — the ONLY file you create
- You do NOT write code
- PM consultation is mandatory (twice) — R2.5 and R3.5
- Task identity is sacred — reconciliation table required
- After writing wave.md, proceed to R3.5

## Step R3.5 — PM second opinion (post-refinement)

Spawn PM as a fan-out agent with fresh context (has NOT seen R1-R3). Present PM's response to founder verbatim. Apply approved items with `[PM-POST]` tag.

After R3.5: "Wave file written to `wave.md` with {N} refined tasks. Reconciliation: {counts}. R1.5 confidence: {%} in {N} round(s). PM input: {counts}. Run `/wave` to execute."
