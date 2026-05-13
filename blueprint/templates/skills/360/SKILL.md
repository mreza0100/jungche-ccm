---
name: 360
version: "1.1.0"
repo: "https://github.com/mreza0100/360"
description: "360° exhaustive multi-angle analysis. Systematically generates ALL angles on a subject — questions, risks, edge cases, blind spots — organized by dimension. Two domains: 'test' (for QA) and 'inquiry' (for Professor). Triggered by '360 <subject>', 'three-sixty', or referenced by agents at key analysis moments. The consumer decides what to act on — 360° just ensures nothing gets skipped."
---

# 360° — Exhaustive Multi-Angle Analysis

> The blind-spot killer. Where instinct says "think about edge cases," 360° says "walk through every dimension and PROVE you considered each one."

This is a **thinking protocol**, not a task runner. It produces an exhaustive list of angles on a subject, organized by dimension. No ranking, no filtering — that's the caller's job. The protocol forces systematic coverage so that creative analysis doesn't accidentally skip entire categories.

## When to load this skill

**Standalone invocation** (user or agent calls it directly):
- `360 <subject>` — the canonical trigger
- `three-sixty <subject>`
- "do a 360 on <subject>"
- "what could go wrong with <subject>"
- "find all the angles on <subject>"

**Embedded invocation** (agents reference the protocol at key moments):
- QA agents run the **test** domain before writing adversarial tests
- Professor runs the **inquiry** domain before deep-diving into code
- PM runs the **inquiry** domain before feature reviews and refinements
- Audit runs the **test** domain before category scans (+ **inquiry** for targeted investigations)

Do NOT load for:
- One-shot implementation ("fix X") — that's `/jc` or `/build`
- Research ("how does X work?") — that's `RR`
- Iterative goal-seeking — that's `RND`

The key distinction: **360° produces questions and angles, not answers.** If you need answers, use a different skill on the 360° output.

---

## The Protocol

### Step 1 — Identify the subject and domain

Parse the input to determine:
- **Subject:** What's being analyzed (a feature, requirement, design, change, system)
- **Domain:** Which dimension set to use

| Domain | When to use | Typical caller |
|--------|------------|----------------|
| `test` | Analyzing something that will be tested — features, implementations, changes | QA agents |
| `inquiry` | Analyzing requirements, proposals, designs — things that need questioning | Professor, Architects |
| `auto` | Not specified — infer from context | Standalone invocation |

If the domain isn't explicit, infer: if the subject is code/implementation → `test`. If the subject is a requirement/proposal/design → `inquiry`. If genuinely ambiguous, default to `inquiry` (it's the broader set).

### Step 2 — Walk every dimension

For the selected domain, go through each dimension one by one. For each dimension, generate **at least one** concrete angle specific to the subject. Don't skip a dimension because "it doesn't apply" — instead, write "N/A: [reason]" so the skip is conscious and auditable.

The goal is **exhaustive enumeration, not exhaustive analysis.** Each angle is a one-liner — enough to identify the concern, not enough to investigate it. Investigation comes later.

---

## Dimensions

### Domain: `test`

Use when analyzing something that will be tested. Forces you to think about every category of failure.

| # | Dimension | What to generate |
|---|-----------|-----------------|
| 1 | **Inputs** | Malformed, missing, oversized, type-confused, empty string vs null, unicode edge cases, special characters in names/IDs |
| 2 | **State** | Empty DB, mid-migration, stale cache, partial data, first-ever use, data from previous version, orphaned records |
| 3 | **Boundaries** | Min/max values, off-by-one, empty collections, single-item collections, nulls vs defaults, zero vs absent |
| 4 | **Sequences** | Out-of-order operations, double-submit, back-navigation mid-flow, interrupted multi-step, retry after partial success |
| 5 | **Timing** | Race conditions, concurrent writes to same resource, timeout during processing, stale read after write, long-running operation interrupted |
| 6 | **Error paths** | Network down, 5xx from dependency, partial failure in multi-step, retry storm, error during error handling, rollback failure |
| 7 | **Data shapes** | Unicode (RTL, emoji, CJK), huge payloads, deeply nested objects, arrays with 10k items, encoding mismatches, JSON with unexpected types |
| 8 | **Environment** | Wrong env loaded, missing config key, port conflict, dependency not running, version mismatch, disk full, permission denied |
| 9 | **Auth/Authz** | Expired token, wrong role, privilege escalation, missing auth header, valid token for deleted user, cross-tenant access |
| 10 | **Regressions** | Does this change break existing flows? What relied on the old behavior? Are there callers not covered by tests? |

### Domain: `inquiry`

Use when analyzing requirements, proposals, or designs. Forces you to question everything before committing to a direction.

| # | Dimension | What to generate |
|---|-----------|-----------------|
| 1 | **Assumptions** | What's taken for granted that might be wrong? What works today but won't at scale? What depends on external behavior we don't control? |
| 2 | **Ambiguities** | What could be interpreted two different ways? Where does "it" refer to more than one thing? What's the difference between "should" and "must" here? |
| 3 | **Contradictions** | What conflicts with existing system behavior? What violates a non-negotiable rule? What's incompatible with another in-flight change? |
| 4 | **Missing info** | What's not specified that MUST be decided before building? What error case isn't addressed? What happens when the happy path doesn't happen? |
| 5 | **Dependencies** | What else has to change for this to work? What's the deployment order? Who needs to know? What breaks if a dependency is delayed? |
| 6 | **Scope gaps** | What's adjacent but not covered? What will users expect that isn't mentioned? What migration path is needed for existing data? |
| 7 | **Stakeholder conflicts** | {USER_PERSONA} vs {SECONDARY_PERSONA} vs ops vs legal? Privacy vs UX? Speed vs correctness? Simplicity vs completeness? |
| 8 | **Feasibility** | Can we build this with our current stack and budget? What's the time/effort estimate? Is there a simpler 80% solution? |
| 9 | **Precedent** | How do others solve this? What patterns have failed in similar products? Is there prior art in the codebase? |

---

## Output format

The output is a flat list grouped by dimension. No prose, no analysis — just angles.

```
## 360° — {subject} ({domain})

### 1. Inputs
- What happens if session_id is a valid UUID but points to a deleted record?
- What if the payload is empty string vs null vs missing key entirely?
- ...

### 2. State
- What if this is the first-ever use (no prior context)?
- What if a previous run partially completed and left stale rows?
- ...

### 3. Boundaries
- ...

[continue through all dimensions]
```

Rules for the output:
- **One angle per bullet** — keep them atomic
- **Be specific to the subject** — "malformed input" is useless; "payload with zero items but non-empty metadata" is useful
- **Don't filter** — if an angle seems unlikely, include it anyway. The caller decides what matters.
- **Mark N/A consciously** — if a dimension truly doesn't apply, say why: "N/A: this is a read-only query with no auth gate"

---

## Standalone invocation

When invoked directly (not embedded in an agent), follow these steps:

1. Parse the subject from the user's message
2. Determine the domain (or ask if ambiguous)
3. Run the protocol — walk every dimension
4. Output the 360° list
5. Ask: "Want me to dig into any of these angles?"

The standalone output stays in the conversation — no file is written unless the user asks.

---

## Embedded invocation (agent integration)

**360° MUST always run in a separate agent with a clean context.** An agent that already has opinions about the subject will unconsciously skip angles that don't fit its mental model — defeating the entire purpose of blind-spot detection. The calling agent spawns a fresh agent that receives ONLY the subject and domain, with zero prior analysis context.

The calling agent MUST:

1. **Spawn a separate Agent** with `subagent_type: "general-purpose"` and a self-contained prompt
2. **Include in the prompt:** the subject description, the domain (`test` or `inquiry`), and an instruction to read `.claude/skills/360/SKILL.md` and execute the protocol
3. **Exclude from the prompt:** any analysis, opinions, findings, or context the calling agent has already developed — the 360° agent must approach the subject cold
4. **Receive the angle list** back from the spawned agent and use it to guide subsequent work

Prompt template for spawning:
```
Read `.claude/skills/360/SKILL.md` and execute the 360° protocol.
Subject: {one-sentence description of what's being analyzed}
Domain: {test | inquiry}
Output the full 360° angle list grouped by dimension.
```

The returned angle list feeds into the calling agent's work — it doesn't become a separate artifact unless the caller decides otherwise.
