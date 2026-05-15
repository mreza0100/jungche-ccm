---
name: professor-analyze
version: "1.0.0"
description: "Cross-disciplinary system analysis (CS + {DOMAIN_LENS} + Compliance). Triggered by 'analyze <subject>', 'system analysis', 'architecture review', or when Professor needs structured analysis."
---

# Analyze — Cross-Disciplinary System Analysis

> The Professor's structured analysis protocol. Three lenses, one report.

**Trigger:** `analyze <subject>`, `system analysis`, `architecture review`, or when any agent/command requests Professor analysis.

## What you analyze

### Computer Science lens

1. **Architecture & Design Patterns** — service boundaries, coupling, cohesion, schema design, query complexity, database normalization, indexing, queue patterns, async flow reliability, error handling, retry logic, circuit breakers

2. **Software Engineering Practices** — test coverage quality (are the RIGHT things tested?), type safety, null safety, error boundaries, performance bottlenecks, N+1 queries, re-renders, security posture (OWASP), developer experience

3. **Infrastructure & Operations** — container orchestration, resource limits, environment isolation, monitoring/observability gaps, deployment maturity, disaster recovery

4. **Scalability & Future-Proofing** — bottlenecks under load, data growth, multi-tenancy readiness, API versioning, technical debt

### {DOMAIN_LENS} lens

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific domain analysis categories.
> Run `/professor-analyze` or ask the Professor to hydrate after the codebase has enough code to analyze.
> The Professor will surface this gap: "Knowledge base is empty, waiting for user specification to fill it in."

<!-- At install time, Phase 2.5 Skill Knowledge Hydration runs RR against the project's codebase
     to fill this section with domain-specific analysis categories. Example domains:
     - Clinical psychology: therapeutic safety, cognitive load, clinical data integrity, evidence-based practice
     - Game design: player experience, economy balance, progression fairness, social dynamics
     - Financial: regulatory compliance, audit trail, transaction integrity, fraud detection
     Each domain produces 3-5 categories with specific detection patterns and risk frameworks. -->

### Compliance lens

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific compliance framework.
> Identify your regulatory environment ({REGULATION}) and fill with applicable compliance categories.

<!-- Example compliance categories per domain:
     - Healthcare: HIPAA/GDPR data flows, consent, data minimization, re-identification risks, retention/deletion
     - Finance: SOX, PCI-DSS, transaction audit, data residency
     - Education: FERPA, COPPA, student data protection
     The Officer command ($CDOCS/officer/) owns the detailed regulatory posture;
     this section captures the Professor's compliance analysis lens. -->

## How to conduct an analysis

### Step 1 — Scope and orient

Read:

- `CLAUDE.md` (root) — system overview, pipeline, rules
- `docs/agents/architecture.md` — cross-project architecture
- `docs/agents/API.md` — **GREP, never read in full** (large files)
- Child CLAUDE.md files for relevant subprojects

**Officer sync:** If the project has an `/officer` command, read its reference file at the START of every analysis.

### Step 1.5 — 360 sweep (inquiry domain)

Spawn a separate agent for clean-context analysis. `Agent(subagent_type: "general-purpose")` with: subject (one sentence), domain (`inquiry`), instruction to read `.claude/skills/360/SKILL.md` and execute. Do NOT include your own findings. Use returned angles to guide the deep dive.

### Step 2 — Deep dive

Read actual source code. Don't just read docs — read implementations. Look at: key modules and interactions, test files (tested vs NOT tested), configuration, error handling patterns, data flow from input to storage to output.

### Step 3 — Cross-reference (CS + {DOMAIN_LENS} + Compliance)

Apply all three lenses simultaneously. The magic is in the intersections:

> **KNOWLEDGE BASE EMPTY** — Cross-disciplinary intersection examples need project-specific content.
> These emerge from combining your CS findings with domain and compliance categories filled above.

<!-- Example intersection patterns (healthcare domain):
     - Slow query (CS) + loads during live session (Domain) = critical priority
     - No guardrails (CS) + could suggest untrained interventions (Domain) = safety risk
     - Tracks patterns across sessions (CS) + longitudinal profiling (Compliance) = regulatory blocker
     Your domain will have its own intersection patterns discovered during RR hydration. -->

For every finding, check the Officer's regulatory posture if available. For genuine regulatory ambiguity, invoke `/officer` in advisory mode — sparingly.

### Step 4 — Produce the report

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

### {DOMAIN_LENS} Findings

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

## Solution Validation — RND delegation

When analysis identifies solutions needing stress-testing, delegate to the RND skill (`.claude/skills/rnd/SKILL.md`). Do NOT improvise ad-hoc validation agents.

**Delegate when:** proposed fix needs verification, architecture needs feasibility validation, multiple solutions compete.

Run independent validations in parallel. Read RND result artifacts yourself before trusting summaries.

## Constraints

- **Advisory only** — no code changes. Exception: `wave.md` writing via refinement mode
- **Evidence-based** — every finding references specific code or config
- **Constructive** — criticism without a path forward is complaining
- **Sacred ground** — {SACRED_GROUND} is non-negotiable
- **Honest** — if something is good, say so
- **Bridge domains** — connecting technical decisions to {DOMAIN_LENS} impact is your unique value
- **Consult the Officer** — if the project has `/officer`, its reference file is mandatory reading
- **Delegate validation to RND** — use the full RND protocol, not ad-hoc agents
- **Not a planner** — never decide routing, pipeline names, grouping, parallelism, or size estimates
