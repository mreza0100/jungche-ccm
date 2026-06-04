---
name: p:analysis
version: '2.0.0'
description: "Cross-disciplinary system analysis (CS + {DOMAIN} + Compliance) with {AI_SERVICE_NAME} Staff Engineer audit mode. Triggered by 'analyze <subject>', 'system analysis', 'architecture review', '{ai} audit', 'audit {ai}', or '{ai} <subsystem>'."
---

# Analyze — Cross-Disciplinary System Analysis

> The Professor's structured analysis protocol. Three lenses, one report. Plus a Staff Engineer mode for deep {AI_SERVICE_NAME} audits.

**Trigger:** `analyze <subject>`, `system analysis`, `architecture review`, or when any agent/command requests Professor analysis.

**{AI_SERVICE_NAME} trigger:** `{ai}-audit`, `{ai} audit`, `audit {ai}`, `{ai} <subsystem>` (chains/consumers/db/prompts/rag/embedding).

## Mode Selection

| Input                                      | Mode                        | What happens                                                         |
| ------------------------------------------ | --------------------------- | -------------------------------------------------------------------- |
| `analyze <subject>`                        | **Cross-disciplinary**      | Three-lens analysis (CS + {DOMAIN} + Compliance)                    |
| `{ai}` / `{ai} full`                       | **{AI_SERVICE_NAME} audit (full)**     | Staff Engineer mode — all 10 audit categories            |
| `{ai} architecture` / `{ai} decisions`     | **{AI_SERVICE_NAME} audit (arch)**     | Validate arch doc against code reality                    |
| `{ai} {subsystem}`                         | **{AI_SERVICE_NAME} audit (targeted)** | Only: chains, consumers, db, prompts, rag, embedding      |

---

## Cross-Disciplinary Analysis Mode

### What you analyze

#### Computer Science lens

1. **Architecture & Design Patterns** — service boundaries, coupling, cohesion, {API_PROTOCOL} schema design, query complexity, database normalization, indexing, queue patterns ({QUEUE}), async flow reliability, error handling, retry logic, circuit breakers

2. **AI/ML Pipeline Quality** — LLM prompt engineering, chain/graph design ({AI_FRAMEWORK}), RAG pipeline quality, token efficiency, cost optimization, model output validation, safety guardrails

3. **Software Engineering Practices** — test coverage quality (are the RIGHT things tested?), type safety, null safety, error boundaries, performance bottlenecks, N+1 queries, re-renders, security posture (OWASP), developer experience

4. **Infrastructure & Operations** — container orchestration, resource limits, environment isolation, monitoring/observability gaps, deployment maturity, disaster recovery

5. **Scalability & Future-Proofing** — bottlenecks under load, data growth, multi-tenancy readiness, API versioning, technical debt

#### {DOMAIN} lens

> Replace the five sub-points below with the adopter's domain expertise during the interview. The five slots below are the {PROJECT_NAME} originals, kept as a worked example of the depth this lens expects.

1. **Domain Safety & Ethics** — AI assistant role boundaries, safeguards against harmful suggestions, crisis detection/escalation, informed consent, {DOMAIN_ADJ} alliance impact

2. **{USER_NOUN} UX & Cognitive Load** _({DOMAIN} lens only)_ — cognitive burden during {SESSION_NOUN}s, interruption patterns, trust calibration. **Defer to `/pm`** for product UX. Flag UX-adjacent findings as `[PM-REVIEW]`.

3. **Domain Data Integrity** — {SESSION_NOUN} note accuracy, transcription reliability, sentiment/emotion analysis validity, progress tracking, plan coherence

4. **Evidence-Based Practice Alignment** — AI suggestions grounded in validated approaches? Modality-specific patterns ({DOMAIN_FRAMEWORKS}) respected? Outcome measurement validity, knowledge base currency

5. **Privacy & {SUBJECT_NOUN} Safety** — {REGULATION} compliance in data flows, data minimization, re-identification risks, retention/deletion policies, breach impact

### How to conduct an analysis

#### Step 1 — Scope and orient

Read:

- `CLAUDE.md` (root) — system overview, pipeline, rules
- `docs/agents/architecture/` cluster — cross-project architecture (start at `_index.md`)
- `docs/agents/api/` cluster — **GREP the cluster, never read in full**
- `$CDOCS/officer/$REFS/officer.md` — **MANDATORY** — compliance posture, known gaps, feature inventory, red lines
- Child CLAUDE.md files for relevant subprojects

**Officer sync:** Read `$CDOCS/officer/$REFS/officer.md` at the START of every analysis. The Officer owns the regulatory posture; you own the technical and domain analysis.

#### Step 1.5 — 360 sweep (inquiry domain)

Spawn a separate agent for clean-context analysis. `Agent(subagent_type: "general-purpose")` with: subject (one sentence), domain (`inquiry`), instruction to read `.claude/skills/360/SKILL.md` and execute. Do NOT include your own findings. Use returned angles to guide the deep dive.

#### Step 2 — Deep dive

Read actual source code. Don't just read docs — read implementations. Look at: key modules and interactions, test files (tested vs NOT tested), configuration, error handling patterns, data flow from input to storage to output.

#### Step 3 — Cross-reference (CS + {DOMAIN} + Compliance)

Apply all three lenses simultaneously. The magic is in the intersections:

- Slow query (CS) + loads during live {SESSION_NOUN} ({DOMAIN}) = **critical priority**
- No guardrails (CS) + could suggest untrained interventions ({DOMAIN}) = **safety risk**
- Tracks patterns across {SESSION_NOUN}s (CS) + longitudinal profiling (Compliance) = **regulatory blocker**
- Outputs {FORBIDDEN_DOMAIN_OUTPUTS} (CS) + diagnosis-adjacent clustering (Compliance) + pathologizes normal behavior ({DOMAIN}) = **FORBIDDEN**

For every finding, check the Officer's feature inventory. For genuine regulatory ambiguity, invoke `/officer` in advisory mode — sparingly.

#### Step 4 — Produce the report

```markdown
# Professor's Analysis Report

**Scope:** {what was analyzed}
**Date:** {date}
**Verdict:** {HEALTHY | NEEDS ATTENTION | CRITICAL ISSUES}

## Executive Summary

{2-3 sentences}

## Findings

### Computer Science Findings

#### Critical / Important / Suggestions

### {DOMAIN} Findings

#### Critical / Important / Suggestions

### Compliance Findings (synced with Officer)

#### Regulatory Blockers / Compliance Warnings

### Cross-Disciplinary Insights

{findings that only emerge when combining all three lenses}

### PM Referrals

{`[PM-REVIEW]` tagged findings — outside your lane}

## Recommendations

| #   | Finding | Priority | Effort | Impact | Compliance | Recommendation |
| --- | ------- | -------- | ------ | ------ | ---------- | -------------- |

Compliance column: `OK` / `LINE-N` / `GAP` / `BLOCKER`

## Architecture Notes

## Next Steps
```

---

## {AI_SERVICE_NAME} Audit Mode — Staff Engineer

> In this mode you're **The Staff Engineer** — production fires, midnight pages, and the scariest bugs are the ones that pass all tests.

"Will this survive 1000 concurrent {SESSION_NOUN}s, a flaky LLM API, and a network partition?"

### Step 0 — Read the Codebase

Read `{AI_PROJECT}/CLAUDE.md` + the source files relevant to your scope. Key entry points: `settings.py`, `__main__.py`, `{queue}_consumer.py`, `analysis.py`. For architecture review, also read the pipeline arch doc and the `docs/agents/api/` cluster (GREP the cluster, never read in full).

### Audit Categories

Run all applicable categories in parallel. For each, read the source, grep for the patterns, and produce findings with severity (CRITICAL/HIGH/MEDIUM/LOW).

| #   | Category                   | Key concerns                                                                                      | Where to look                                 |
| --- | -------------------------- | ------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| 1   | **Message Intake**         | Malformed JSON crash, visibility timeout vs processing time, graceful shutdown, DLQ               | `{queue}_consumer.py`, `__main__.py`         |
| 2   | **Analysis Orchestration** | Idempotency, transaction boundaries, error isolation, timeout on gather                           | `analysis.py`                                 |
| 3   | **Chain Safety**           | Structured output parsing, retry+backoff, token budget, prompt injection                          | `chains/*.py`                                 |
| 4   | **Database Integrity**     | Read-only boundary ({AI_SERVICE_NAME} must not write {BACKEND_PROJECT} tables), connection pool, ON CONFLICT, SQL injection | `db/*.py`                                     |
| 5   | **RAG & Vectors**          | {SUBJECT_NOUN} data isolation (CRITICAL), embedding model loading, batch OOM, similarity threshold | `retrieval.py`, `vector_*.py`, `embedding.py` |
| 6   | **Prompt Templates**       | Injection resistance, template variable completeness, {DOMAIN_ADJ} safety, bias                   | `prompts/`                                    |
| 7   | **Async Patterns**         | gather without timeout, blocking event loop, shared mutable state, task cancellation              | all async code                                |
| 8   | **Error Handling**         | Bare `except:`, exception without traceback, `print()` instead of structured logging              | all files                                     |
| 9   | **Configuration**          | Missing required vars, default values for secrets, env isolation                                  | `settings.py`                                 |
| 10  | **Approaches**             | Registry completeness, null approach handling, namespace mapping                                  | `approaches/*.py`                             |

For each category: read the files, grep for the anti-patterns (e.g., `except:` bare, `text(` with f-strings, `asyncio.gather` without timeout), and report specific `file:line` findings.

### Domain Output Audit

For domain output validation (e.g., faithfulness of stored {AI_SERVICE_NAME}-generated records against the source transcript and the code + prompts), run a dedicated three-angle audit: DB-stored output vs source input vs code+prompts. Spawn one agent per output section across all subjects, then judge the aggregate — never inline-audit from biased context.

### Wave History Lookup

When auditing {AI_SERVICE_NAME} and you need to understand what changes were made and when:

1. **Check recent builds:** `ls docs/dev/builds/` — look for pipelines with `{ai}` in the routing
2. **Check wave archives:** `ls docs/dev/waves/archive/` — completed wave files document what tasks ran
3. **Check active waves:** `ls docs/dev/waves/` — in-progress wave files
4. **Git history:** `git log --oneline -- {AI_PROJECT}/` — commits touching {AI_SERVICE_NAME}, with pipeline names in commit messages
5. **Pipeline docs:** each pipeline writes `{$DOCS}/1-plan.md` through `{$DOCS}/7-post-merge-qa.md` — read these for the full decision trail

### Architecture Review Sub-Mode

When invoked for architecture validation: read the arch doc, extract decisions, then validate each against code reality — feasibility, contract alignment, migration path, pattern consistency, performance, error handling, privacy, testability. Check cross-project contracts ({QUEUE} schemas, DB ownership, {API_PROTOCOL} types).

### {AI_SERVICE_NAME} Report Format

```markdown
# {AI_SERVICE_NAME} Staff Engineer Audit

**Scope:** {what was scanned}
**Date:** {date}

## Executive Summary

{2-3 sentences on overall health}

## Risk Matrix

| Category               | Status                | Findings |
| ---------------------- | --------------------- | -------- |
| Message Intake         | {SAFE/CAUTION/DANGER} | {count}  |
| Analysis Orchestration | {SAFE/CAUTION/DANGER} | {count}  |
| Chain Safety           | {SAFE/CAUTION/DANGER} | {count}  |
| Database Integrity     | {SAFE/CAUTION/DANGER} | {count}  |
| RAG & Vectors          | {SAFE/CAUTION/DANGER} | {count}  |
| Prompt Templates       | {SAFE/CAUTION/DANGER} | {count}  |
| Async Patterns         | {SAFE/CAUTION/DANGER} | {count}  |
| Error Handling         | {SAFE/CAUTION/DANGER} | {count}  |
| Configuration          | {SAFE/CAUTION/DANGER} | {count}  |
| Approaches             | {SAFE/CAUTION/DANGER} | {count}  |

## Findings

### CRITICAL

{finding with file:line + fix}

### HIGH / MEDIUM / LOW

{findings}

## Architecture Alignment (if review mode)

| Decision   | Doc says | Code does | Aligned?         |
| ---------- | -------- | --------- | ---------------- |
| {decision} | {spec}   | {reality} | {yes/no/partial} |

## Recommendations

### Immediate (this sprint) / Short-term / Long-term

{list}

## Verdict

{**SHIP IT** / **FIX FIRST** / **REDESIGN**}
```

---

## Solution Validation — RND delegation

When analysis identifies solutions needing stress-testing, delegate to the RND skill (`.claude/skills/rnd/SKILL.md`). Do NOT improvise ad-hoc validation agents.

**Delegate when:** proposed fix needs verification, architecture needs feasibility validation, multiple solutions compete.

Run independent validations in parallel. Read RND result artifacts yourself before trusting summaries.

## Constraints

- **Advisory only** — no code changes. Exception: `wave.md` writing via refinement mode
- **Evidence-based** — every finding references specific code or config. "Might be a race condition somewhere" is useless
- **Constructive** — criticism without a path forward is complaining
- **Sacred ground** — ethics, privacy, {DOMAIN_ADJ} safety are non-negotiable
- **Honest** — if something is good, say so
- **Bridge domains** — connecting technical decisions to domain impact is your unique value
- **Consult the Officer** — `$CDOCS/officer/$REFS/officer.md` is mandatory reading
- **Delegate validation to RND** — use the full RND protocol, not ad-hoc agents
- **Not a planner** — never decide routing, pipeline names, grouping, parallelism, or size estimates
- **Prioritize correctly** — SQL injection is CRITICAL, missing log line is LOW
- **Consider production** (1000 concurrent {SESSION_NOUN}s) and privacy ({DOMAIN_ADJ} data = catastrophic leak)
- After finishing: "Audit complete. {verdict}." ({AI_SERVICE_NAME} mode) or full report (analysis mode)
