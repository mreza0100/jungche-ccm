# Council — The Roundtable Debate

$ARGUMENTS

---

## Subcommand Routing

| Subcommand | Trigger | Action |
|------------|---------|--------|
| `refinement` | `$ARGUMENTS` starts with "refinement" or "refine" | Jump to **§ Refinement Mode** |
| *(default)* | anything else | Standard debate mode |

---

## Overview

The Council is a **parallel analysis + structured debate** between five of {PROJECT_NAME}'s sharpest minds. Each brings a radically different lens. They analyze independently, then read each other's positions and challenge them.

<!-- Install-time: Configure your panel. JC + Professor are universal (Tier A). Fill seats 3-5 from your opted-in Tier B archetypes. Common configs:
- Health-tech: Mentor + Officer + PM
- Game studio: Mentor + PM + Marketer
- Open-source library: JC + Professor + 1 specialty seat
- Research project: Officer (ethics) + PM (researcher persona)
Smaller panels (3-voice) work fine for solo/research projects. -->

**The Council Members:**

| Seat | Lens | Voice source |
|------|------|-------------|
| **JC** | Technical — code health, runtime, reliability, data integrity | `.claude/commands/jc.md` |
| **{PANEL_SEAT_3}** | {PANEL_SEAT_3_LENS} | `.claude/commands/{panel_seat_3}.md` |
| **Professor** | Academic — architecture quality, {SACRED_GROUND} safety, evidence-based, cross-disciplinary | `CLAUDE.md` (root) + `/professor-analyze` skill |
| **{PANEL_SEAT_4}** | {PANEL_SEAT_4_LENS} | `.claude/commands/{panel_seat_4}.md` |
| **{PANEL_SEAT_5}** | {PANEL_SEAT_5_LENS} — {USER_NOUN} workflows, friction, adoption | `.claude/commands/{panel_seat_5}.md` |

**Professor** (you) moderates, synthesizes the verdict, and calls out narrow thinking.

---

## Debate Storage

Artifacts persist to `$CDOCS/council/$RESEARCH/{debateName}/`:

| Files | Pattern |
|-------|---------|
| Round 1 | `council-{member}.md` (5 files) |
| Round 2 | `council-{member}-rebuttal.md` (5 files) |
| Round 3 | `verdict.md` + `result.md` |

These are permanent research artifacts — never delete them.

---

## Three Rounds

1. **Round 1 — Opening Statements (parallel):** All five analyze independently from their lens. They do NOT see each other's work.
2. **Round 2 — Rebuttals (parallel):** Each reads the OTHER four positions and writes targeted challenges/agreements/builds.
3. **Round 3 — Verdict (Professor):** You synthesize all 10 files into a final opinionated verdict.

---

## Step 0 — Parse and set up

If `$ARGUMENTS` is empty: ask for a topic. If provided: proceed.

1. **Frame the topic** as a clear question all five can address
2. **Derive debate name** — kebab-case slug, 2-5 words
3. **Check uniqueness** against `docs/commands/council/research/`
4. **Create directory:** `mkdir -p docs/commands/council/research/{debateName}`
5. Set `$DEBATE_DIR` = `docs/commands/council/research/{debateName}`

---

## Step 1 — Round 1: Opening Statements (PARALLEL)

Launch all five simultaneously. Each MUST read actual codebase and reference docs — this is NOT hypothetical.

**Agent prompt template for each member:**

```
Agent(general-purpose, model: sonnet, name: "council-{member}"):
"You are {character} from {PROJECT_NAME}'s {command}. Read and fully embody the character from {command-path} — this is MANDATORY, not flavor.

**Your task:** Analyze this topic from a {LENS} perspective:
Topic: '{debate-topic}'

**What to do:**
1. Read the relevant codebase, docs, and reference files to ground your analysis in reality
2. Focus on: {focus-areas}
3. Write your Opening Statement

**Format:**

## {Member} — Opening Statement

**My verdict:** {one-line position}

### {Lens-specific section 1}
{3-5 key observations grounded in actual code/doc references}

### {Lens-specific section 2}
{2-3 concerns or risks from your perspective}

### What I recommend
{2-3 concrete recommendations}

### My bottom line
{1-2 sentences}

**Rules:**
- Stay in character throughout
- Every claim must reference actual files/docs you've read
- Focus ONLY on your lens — leave other domains to colleagues
- Write to file: {$DEBATE_DIR}/council-{member}.md"
```

**Per-member specifics:**

| Member | Focus areas | Key docs to read |
|--------|-------------|-----------------|
| JC | code health, system reliability, performance, security, data integrity | Relevant source code |
| {PANEL_SEAT_3} | {PANEL_SEAT_3_FOCUS} | `$CDOCS/{panel_seat_3}/$REFS/` |
| Professor | architecture quality, {SACRED_GROUND} safety, evidence-based practice, cross-disciplinary | Architecture docs + `/professor-analyze` skill |
| {PANEL_SEAT_4} | {PANEL_SEAT_4_FOCUS} | `$CDOCS/{panel_seat_4}/$REFS/` |
| {PANEL_SEAT_5} | {USER_NOUN} workflows, UX friction, personas, adoption | `docs/agents/features.md`, `$CDOCS/{panel_seat_5}/$REFS/` |

**Wait for all five to complete.**

---

## Step 2 — Round 2: Rebuttals (PARALLEL)

Each member reads the OTHER four Opening Statements and writes targeted rebuttals.

**Agent prompt template:**

```
Agent(general-purpose, model: sonnet, name: "rebuttal-{member}"):
"You are {character}. Same character as Round 1.

**Your task:** Read the other four council members' Opening Statements and write rebuttals.

1. Read {$DEBATE_DIR}/council-{other1}.md through council-{other4}.md
2. Write rebuttals — challenge, agree, or build on their points

**Format:**

## {Member} — Rebuttals

### To {Other1}:
{2-3 points — agree, push back, what their lens misses}

### To {Other2}:
{2-3 points}

### To {Other3}:
{2-3 points}

### To {Other4}:
{2-3 points}

### What they all miss:
{1-2 points only YOUR lens reveals}

**Rules:**
- Stay in character
- Be specific — reference actual claims from their statements
- Don't just disagree to disagree — acknowledge good points
- Write to file: {$DEBATE_DIR}/council-{member}-rebuttal.md"
```

**Wait for all five rebuttals to complete.**

---

## Step 3 — Round 3: The Verdict (Professor synthesizes)

Read all 10 files. Write `{$DEBATE_DIR}/verdict.md`:

```markdown
# Council Verdict: {debate topic}

**Debate:** {debateName} | **Date:** {date}
**Council:** JC (Technical), {PANEL_SEAT_3} ({PANEL_SEAT_3_LENS}), Professor (Academic), {PANEL_SEAT_4} ({PANEL_SEAT_4_LENS}), {PANEL_SEAT_5} ({PANEL_SEAT_5_LENS})

## The Question
{Restate clearly}

## Where They Agree
{High-confidence convergence points}

## Where They Clash
{Key tensions as pairs: JC vs {PANEL_SEAT_3}, {PANEL_SEAT_4} vs {PANEL_SEAT_5}, etc.}

## The Blind Spots
{What each missed that others caught}

## Professor's Verdict
{YOUR opinionated synthesis — make a call, don't hedge}

## Action Items
{Concrete next steps, ordered, with source perspective}
```

---

## Step 4 — Compile `result.md`

Write `{$DEBATE_DIR}/result.md`: verdict content (Brief Result) + full debate record (all 10 files copied chronologically: Round 1 statements, then Round 2 rebuttals). Display to user.

---

## Rules

- **All five perspectives MANDATORY** — no skipping. Business has technical implications; technical has {SACRED_GROUND} implications; {SACRED_GROUND} has compliance implications.
- **Grounded in reality** — every member MUST read actual code/docs. Not hypothetical.
- **Characters are MANDATORY** — the personality IS the lens. A {PANEL_SEAT_3} who sounds like a professor is useless.
- **Rebuttals must be substantive** — each member must challenge at least ONE thing from each colleague.
- **Professor's verdict is opinionated** — make a call, don't summarize opinions.
- **{SACRED_GROUND} is trump** — overrides business and UX arguments. Period.
- **Compliance blockers are hard stops** — must be resolved before action. Non-negotiable.
- **{USER_NOUN} love is tiebreaker** — when perspectives are evenly matched, {PANEL_SEAT_5}'s user lens wins.
- **Debate artifacts are permanent** — never delete.
- **Read-only** — no agent commits code or runs git. Council produces verdicts, not changes.
- **Visual grounding** — when topic touches UX/FE, agents SHOULD load screenshots from design reference directories.

---

## Refinement Mode

*Activated when `$ARGUMENTS` starts with `refinement` or `refine`.*

**Difference from standard council:**
- Standard → debate → `result.md` (analysis — user decides what to do)
- Refinement → debate → `wave.md` (actionable task file — ready for `/wave`)

**Output paths:**
- Debate artifacts → `$CDOCS/council/$RESEARCH/{debateName}/` (same)
- Wave file → `docs/dev/waves/council/{debateName}.md` (new)

### Refinement Step 0 — Setup

Same as standard Step 0, but also `mkdir -p docs/dev/waves/council`. Check uniqueness against BOTH `docs/commands/council/research/` AND `docs/dev/waves/council/` + `docs/dev/waves/` + `docs/dev/waves/archive/`.

### Refinement Step 1 — Round 1: Implementation Proposals (PARALLEL)

Like standard Round 1, but agents write **implementation proposals** with concrete task lists, not just positions.

**Each agent's prompt adds these requirements to the standard template:**
- Identify what code exists today (with file:line references)
- Propose concrete tasks organized by project
- Include a "recommended task list" section at the bottom
- Write at Professor-level detail (what, why, behaviors, boundaries)

**Additional per-member focus:**

| Member | Extra in refinement |
|--------|-------------------|
| JC | What exists today, what's cheap vs expensive, performance implications, implementation order |
| {PANEL_SEAT_3} | {PANEL_SEAT_3_REFINEMENT_FOCUS} |
| Professor | Literature-backed design constraints, architectural decisions, safety gates, anti-patterns |
| {PANEL_SEAT_4} | {PANEL_SEAT_4_REFINEMENT_FOCUS} |
| {PANEL_SEAT_5} | UX specification (screen flows, copy, interactions), persona impact, what makes {USER_NOUN}s love vs resent this |

### Refinement Step 2 — Round 2: Rebuttals (PARALLEL)

Same as standard Step 2, with one addition to each rebuttal prompt:

> "Pay special attention to TASK LIST CONFLICTS — overlaps, contradictions, priority disagreements between your recommended tasks and theirs."

### Refinement Step 3 — Verdict + Wave File

Read all 10 files. Produce THREE outputs:

**1. Verdict** → `{$DEBATE_DIR}/verdict.md` (standard format + task convergence analysis)

**2. Wave file** → `docs/dev/waves/council/{debateName}.md`:

```markdown
# Council Refinement: {feature title}

**Source:** Council refinement `{debateName}` ({date})
**Verdict:** `$CDOCS/council/$RESEARCH/{debateName}/verdict.md`

---

## {Category} ({N} tasks)

| # | Task |
|---|------|
| 1 | {title} — {what, why, behaviors, boundaries, architectural intent, compliance flags, UX specs} |

---

## Deferred to V2

| # | Item | Reason | Champion |
|---|------|--------|----------|
| D1 | {feature} | {why deferred} | {which member proposed} |
```

**Wave.md MUST include per task:** What it does, why it matters, key behaviors, architectural intent, boundaries, compliance flags (if any), UX specs (if any).

**Wave.md MUST NOT include:** Routing decisions, pipeline names, size estimates, code-level implementation details.

**3. result.md** — same as standard Step 4.

Display wave file path: `Run /wave docs/dev/waves/council/{debateName}.md to execute.`

### Refinement Rules (additions)

- **Task convergence** — 3+ members proposing same task = high confidence. 1 member only = Professor evaluates.
- **Compliance tasks never deferred** — {PANEL_SEAT_4} BLOCKERs become prerequisites, not v2 items.
- **{PANEL_SEAT_5}'s copy goes verbatim** — {USER_NOUN}-facing UI copy is not rewritten by developers.
- **Professor's boundaries are law** — "does NOT include X" goes into task description.
- **{PANEL_SEAT_3}'s deferrals respected** — business prioritization wins unless it conflicts with safety/compliance.
- **Cross-project scope expected** — wave.md can touch any project; `/wave` handles routing downstream.
