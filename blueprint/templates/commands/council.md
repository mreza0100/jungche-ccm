# Council — The Roundtable Debate

> **Tier A — Universal archetype.** Three-round debate structure (opening / rebuttal / verdict) is universal. Panel composition parameterizes per install — universal voices (JC + Professor) are mandatory; the other 3 seats are filled from your opted-in Tier B archetypes.

$ARGUMENTS

---

## Subcommand Routing

| Subcommand | Trigger | Action |
|------------|---------|--------|
| `refinement` | `$ARGUMENTS` starts with "refinement" or "refine" | Jump to **§ Refinement Mode** below |
| *(default)* | anything else | Continue to **§ Overview** — standard debate mode |

---

## Overview

The Council is a **parallel analysis + structured debate** between {N} of your sharpest archetypes. Each brings a radically different lens to the same topic. They analyze independently, then read each other's positions and challenge them — producing a richer, more battle-tested conclusion than any single perspective could.

**The default panel:**

| Seat | Tier | Lens |
|------|------|------|
| **JC** | A (universal) | Technical diagnostics — code health, runtime behavior, system reliability, data integrity |
| **Professor** | A (universal) | Cross-disciplinary rigor — architecture, sacred-ground safety, evidence base |
| **`{PANEL_SEAT_3}`** | B (opt-in) | Typically a domain seat — pick from your Tier B opt-ins |
| **`{PANEL_SEAT_4}`** | B (opt-in) | Typically a stakeholder seat — pick from your Tier B opt-ins |
| **`{PANEL_SEAT_5}`** | B (opt-in) | Typically a user/product seat — pick from your Tier B opt-ins |

> **Install-time parameterization:** Replace `{PANEL_SEAT_N}` with the names of your opted-in Tier B archetypes. Common configurations:
>
> - **Health-tech / therapy AI:** Mentor + Officer + PM
> - **Game studio:** Mentor + PM + Marketer
> - **Open-source library:** (no Tier B — JC + Professor + 1 specialty seat works fine)
> - **Research project:** Officer (IRB/ethics) + PM (researcher persona)
>
> Smaller panels work — a 3-voice council (JC + Professor + 1) handles solo or research projects fine.

**Jungche** (the orchestrator) moderates — sets the topic, runs the rounds, synthesizes the verdict, calls out when someone's being too narrow.

---

## Debate Storage

All debate artifacts persist to `$CDOCS/council/$RESEARCH/{debateName}/` where `{debateName}` is a kebab-case slug derived from the topic.

**Files per debate:**

| File | Round | Author |
|------|-------|--------|
| `council-jc.md` | 1 | JC — Opening Statement |
| `council-professor.md` | 1 | Professor — Opening Statement |
| `council-{PANEL_SEAT_3}.md` | 1 | Opening Statement |
| `council-{PANEL_SEAT_4}.md` | 1 | Opening Statement |
| `council-{PANEL_SEAT_5}.md` | 1 | Opening Statement |
| `council-{member}-rebuttal.md` (one per member) | 2 | Rebuttals |
| `verdict.md` | 3 | Jungche — Synthesized Verdict |
| `result.md` | Final | Compiled deliverable — brief result + full debate record |

These are **permanent research artifacts** — do NOT delete them.

---

## How It Works — Three Rounds

### Round 1 — Opening Statements (parallel)

All panel members analyze the topic simultaneously from their unique perspective. Each writes a structured position paper. They do NOT see each other's work yet.

### Round 2 — Rebuttals (parallel)

Each member reads the OTHER members' positions and writes rebuttals — challenging, agreeing, or building on what the others said. This is where the magic happens: business shark poking holes in technical idealism, professor questioning growth-at-all-costs, JC pointing out elegant architecture is actually broken in production, Officer red-flagging what everyone else handwaved past, PM asking whether any of them thought about what users actually want.

### Round 3 — Verdict (Jungche synthesizes)

Jungche reads all opening statements + rebuttals, delivers the final synthesized verdict. Where do they agree? Where do they clash? What's the actual path forward that balances all lenses?

---

## Step 0 — Parse the topic and set up debate directory

If `$ARGUMENTS` is empty, ask the user what they want the council to debate. Don't proceed without a topic.

**Topic framing:** Reframe as a clear question or decision all panel members can address.

**Debate name:** Derive a kebab-case slug (2-5 words). Check uniqueness against `$CDOCS/council/$RESEARCH/`. Append `-v2`, `-v3` if needed.

**Create the debate directory:**

```bash
mkdir -p docs/commands/council/research/{debateName}
```

Set `$DEBATE_DIR` for all subsequent file paths.

---

## Step 1 — Round 1: Opening Statements (PARALLEL)

Launch all panel members simultaneously. Each runs as an Agent with their full character loaded.

**CRITICAL:** Each agent MUST read the actual codebase and reference docs relevant to the topic — this is NOT a hypothetical debate. Every claim must be grounded in what actually exists.

**Pattern (repeat per panel member):**

```
Agent(general-purpose, model: sonnet, name: "council-{member}"):
"You are {Member} from {Project}'s /{member} command. Read and fully embody the character from .claude/commands/{member}.md. This is MANDATORY, not flavor.

**Your task:** Analyze this topic from a {LENS} perspective:
Topic: '{debate-topic}'

**What to do:**
1. Read the relevant codebase, docs, and reference materials to ground your analysis
2. Focus on: {lens-specific concerns}
3. Write your Opening Statement as a structured position paper

**Format your Opening Statement as:**

## {Member} — Opening Statement

**My verdict:** {one-line position}

### {Lens-specific section 1}
{3-5 key observations grounded in actual references}

### {Lens-specific section 2}
{Concerns / risks / what worries you}

### {Lens-specific section 3}
{Recommendations}

### My bottom line
{1-2 sentences — your core position}

**Rules:**
- Stay in character throughout
- Every claim references actual files, functions, or reference docs
- Focus ONLY on the {LENS} lens — leave other lenses to other members
- Be honest — don't sugarcoat, don't catastrophize
- Write to file: {$DEBATE_DIR}/council-{member}.md"
```

**Wait for all members to complete before proceeding.**

---

## Step 2 — Round 2: Rebuttals (PARALLEL)

Each member reads the OTHER members' Opening Statements and writes targeted rebuttals.

**Pattern (repeat per panel member):**

```
Agent(general-purpose, model: sonnet, name: "rebuttal-{member}"):
"You are {Member} — same character as before.

**Your task:** Read the other panel members' Opening Statements and write your rebuttals.

1. Read each {$DEBATE_DIR}/council-{other-member}.md
2. Write rebuttals — challenge, agree, or build

**Format:**

## {Member} — Rebuttals

### To {Other Member 1}:
{2-3 points — where you agree, where you push back, what their lens misses}

### To {Other Member 2}:
{2-3 points}

### To {Other Member 3}:
{2-3 points}

### What they all miss:
{1-2 points only YOUR lens reveals}

**Rules:**
- Stay in character
- Be specific — reference actual claims
- Don't disagree just to disagree — acknowledge good points
- Write to file: {$DEBATE_DIR}/council-{member}-rebuttal.md"
```

**Wait for all rebuttals to complete before proceeding.**

---

## Step 3 — Round 3: The Verdict (Jungche synthesizes)

Read all opening statements + all rebuttals. Synthesize into a final verdict at `{$DEBATE_DIR}/verdict.md`:

```markdown
# Council Verdict: {debate topic}

**Debate:** {debateName}
**Date:** {date}
**Council:** {list of seats}

## The Question
{Restate the debate topic clearly}

## Where They Agree
{Points all members converged on — high-confidence conclusions}

## Where They Clash
{Key disagreements as tension pairs:}
- **{Member A} vs {Member B}:** {the lens-vs-lens tension}
- ...

## The Blind Spots
{What each member missed that others caught — this is the value of the debate}

## Jungche's Verdict
{YOUR synthesis — opinionated. Don't "split the difference" — make a call.
State what to do, in what order, why this path beats the alternatives.}

## Action Items
{Concrete next steps, ordered by priority, with which lens they come from:}
1. {action} — *(from {Member}'s {lens} analysis)*
2. ...
```

---

## Step 4 — Compile `result.md`

After the verdict, compile the entire debate into `{$DEBATE_DIR}/result.md`:

```markdown
# Council Result: {debate topic}

## Brief Result
{Verdict content from verdict.md — TL;DR for the user}

## Full Debate Record

### Round 1 — Opening Statements
{Content of each council-{member}.md, separated by `---`}

### Round 2 — Rebuttals
{Content of each council-{member}-rebuttal.md, separated by `---`}
```

**Display `result.md` to the user** — that's the deliverable.

---

## Rules

- **All panel perspectives are MANDATORY** — no skipping a member, even if the topic seems to only concern one domain. The whole point is that every decision affects every lens.
- **Grounded in reality** — every member MUST read actual code, docs, and data.
- **Characters are MANDATORY** — JC sounds like JC, Professor sounds like Professor, etc. The personality IS the lens. A Mentor who sounds like a professor is useless.
- **Rebuttals must be substantive** — "I agree with everything" is not a rebuttal. Each member finds at least ONE thing to challenge from each colleague.
- **Jungche's verdict is opinionated** — don't hedge. Make a call.
- **`{SACRED_GROUND}` is the trump card** — if any panel member raises a `{SACRED_GROUND}` concern, it overrides business and convenience arguments.
- **Compliance blockers are hard stops** — if `/officer` (when on the panel) identifies a regulatory BLOCKER, it must be resolved before the recommended action proceeds.
- **Debate artifacts are permanent** — do NOT delete files in `$CDOCS/council/$RESEARCH/{debateName}/`.
- **No member runs git, edits code, or makes changes** — this is a DEBATE command, not an action command. Verdict only.
- **Visual grounding** — when the debate topic touches UX or user-facing features, council agents SHOULD load available screenshots and design references to ground their analysis in what actually exists.

---

## Refinement Mode

*Activated when `$ARGUMENTS` starts with `refinement` or `refine`.*

In this mode, the Council doesn't just debate — it **designs the best possible implementation** of a feature, then produces a `/wave`-consumable task file.

**The difference from standard council:**
- Standard → debate → verdict → `result.md` (analysis — user decides what to do)
- Refinement → debate → verdict → `wave.md` (actionable — ready for `/wave`)

**Output paths:**
- Debate artifacts → `$CDOCS/council/$RESEARCH/{debateName}/` (same as standard)
- Wave file → `docs/dev/waves/council/{debateName}.md` (NEW — `/wave`-consumable)

---

### Refinement Step 0 — Parse, frame, and set up

Same as standard Step 0, but frame as a design challenge, not an analysis question.

**Create both directories:**
```bash
mkdir -p docs/commands/council/research/{debateName}
mkdir -p docs/dev/waves/council
```

---

### Refinement Step 1 — Round 1: Implementation Proposals (PARALLEL)

All panel members analyze the feature simultaneously and write **implementation proposals** — not just positions. Each proposal must answer: what to build, how it should work, what constraints apply from their lens, and what tasks are needed. Each must end with "My recommended task list".

**CRITICAL:** Each agent MUST read the actual codebase and relevant docs. Every proposal must be grounded in what exists today.

Launch all panel members in parallel using the same Agent pattern from Step 1, but with implementation-focused prompts per lens:

- **JC (Technical):** What code changes are needed, what patterns to reuse, architecture decisions, cheap vs expensive, performance implications
- **Professor (Academic):** Architecture quality, safety considerations, evidence-based design, what could go wrong
- **Tier B seats:** Each applies their specific lens (business prioritization, compliance prerequisites, UX/product, etc.)

**Wait for all members to complete before proceeding.**

---

### Refinement Step 2 — Round 2: Rebuttals (PARALLEL)

Same as standard council Round 2, but add to each rebuttal prompt:

```
In your rebuttals, pay special attention to TASK LIST CONFLICTS — where your recommended tasks
overlap, contradict, or have different priorities than the other members'. Call out:
- Tasks that appear in multiple lists (good — convergence signal)
- Tasks that one member recommends and another opposes (needs resolution)
- Tasks missing from others' lists that you consider essential
- Priority disagreements (one member says "v1 must-have", another says "defer to v2")
```

**Wait for all rebuttals to complete before proceeding.**

---

### Refinement Step 3 — Verdict + Wave File (Jungche synthesizes)

Read all documents and produce TWO outputs:

#### Output 1: Verdict (same format as standard council)

Include task convergence analysis (which tasks appeared in 3+ lists = high confidence).

#### Output 2: Wave file

Synthesize all implementation proposals + rebuttals into a **single wave.md** at `docs/dev/waves/council/{debateName}.md`.

**This wave file must:**
1. Follow the Professor's wave.md format — numbered tasks, categorized by domain, with full specification depth
2. Include tasks from ALL perspectives — technical work, compliance prerequisites, UX specs, architectural constraints, business prioritization
3. Resolve all conflicts from the rebuttals — where members disagreed, Jungche picks the winner
4. Include compliance flags from Officer (when on panel): `[WATCH: ...]`, `[BLOCKED: ...]`, `[FIXES GAP: ...]`
5. Mark deferred items clearly: `[DEFERRED TO V2: reason]`
6. Reference the council debate for traceability

**Wave file format:**

```markdown
# Council Refinement: {feature title}

**Source:** Council refinement `{debateName}` ({date})
**Council:** {list of seats}
**Verdict:** `$CDOCS/council/$RESEARCH/{debateName}/verdict.md`

---

## {Category 1} ({N} tasks)

| # | Task |
|---|------|
| 1 | {title} — {what, why, behaviors, boundaries, architectural intent, compliance flags, UX specs} |
| 2 | ... |

## {Category 2} ({N} tasks)

| # | Task |
|---|------|
| 3 | ... |

---

## Deferred to V2

| # | Item | Reason | Champion |
|---|------|--------|----------|
| D1 | {feature} | {why deferred} | {which member proposed it} |
```

**The wave.md MUST NOT include:**
- Routing decisions (mono-planner decides)
- Pipeline names (`/wave` decides)
- Size estimates (planners estimate)
- Code-level details (architects/developers decide)

**The wave.md MUST include (for every task):**
1. What it does (concrete functionality)
2. Why it matters (problem being solved)
3. Key behaviors (success, failure, edge cases)
4. Architectural intent (new service? extend existing? sync/async?)
5. Boundaries (what this task does NOT include)
6. Compliance flags (if applicable)
7. UX specs (if applicable — copy, framing, interaction patterns)

#### Output 3: Compiled result.md

Same as standard council Step 4.

**Then display the wave file path to the user:**
```
Council refinement complete. {N} tasks refined across {M} categories.
Wave file: docs/dev/waves/council/{debateName}.md
Debate record: docs/commands/council/research/{debateName}/result.md

Run `/wave docs/dev/waves/council/{debateName}.md` to execute.
```

---

### Refinement Rules (in addition to standard council rules)

- **Implementation proposals replace position papers** — Round 1 agents write concrete task lists, not abstract positions. Every agent must produce a "recommended task list" section.
- **Task convergence is signal** — when 3+ members independently propose the same task, high confidence. When only 1 proposes it and others don't mention it, Jungche evaluates whether it's a unique insight or an overreach.
- **The wave.md is the deliverable** — every contested decision must be resolved IN the wave file, not left as "see verdict."
- **Compliance tasks are never deferred** — if the Officer (when on panel) flags a BLOCKER, it becomes a prerequisite task, not a deferred item. BLOCKERs ship in v1 or the feature doesn't ship.
- **PM's copy goes verbatim** — when PM (when on panel) writes specific UI copy, it goes into task descriptions verbatim. Developers should not rewrite user-facing copy.
- **Professor's boundaries are law** — when the Professor says "this task does NOT include X," that boundary goes into the wave.md task description.
- **Mentor's deferrals are respected** — when the Mentor (when on panel) says "defer to v2" with sound reasoning, the item moves to the Deferred table.
- **Cross-project scope is expected** — refinement can produce tasks touching any project. The wave.md doesn't care about routing — `/wave` and mono-planner handle that downstream.
