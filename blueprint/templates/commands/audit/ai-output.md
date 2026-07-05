---
name: audit:ai-output
version: "1.0.0"
description: "AI pipeline output validation — three-angle faithfulness audit of the AI/LLM-generated data read from its store against the source input that produced it and the pipeline code + prompts/knowledge. Triggered by 'audit ai output', 'validate output fidelity', 'audit output against source', 'check what the pipeline produced'."
---

# Audit: AI Output — Pipeline Output Validation

> Three-angle faithfulness audit. Cross-validate the AI-generated data **in its store** against the **source input** that produced it and the **pipeline code + prompts/knowledge** that govern it.

> **Domain-hydrated shell.** This ships the universal faithfulness method — the three angles, the data-first/code-last walk, the per-unit fan-out, the synthesizer contract. The per-project detail (what the output "channels" are, which store tables/collections they write to, which chain and prompt file governs each, how to query the store) is filled in at install by mapping this shell onto your AI pipeline. The worked source instance validated a {DOMAIN_ADJ} analysis engine whose channels wrote to DB tables; keep the structure, swap the channel/table/prompt specifics for yours.

**Trigger:** `audit ai output`, `validate output fidelity`, `audit output against source`, `check what the pipeline produced`, or any request to verify LLM output quality against what was actually in the source input.

---

## The Protocol

Spawn a clean-context 360 sweep — a `general-purpose` agent reading `.claude/commands/p/360.md`, domain `test`, subject = the audited channel — in parallel with the audit; fold its angle list into the per-unit checks (the same blind-spot backstop `audit/code-hygiene.md` and `audit/security.md` carry).

Three angles, each with one authoritative source:

1. **Source input** (what went IN) — the raw material the pipeline processed (a transcript, document, event stream, …), read from the seed export or the store's source table.
2. **AI-generated data** (what came OUT) — read from the **store** (the pipeline's output tables/collections). Never from a seed JSON: the seed is a frozen export that does not reflect the current chains, so auditing it tests history, not the code.
3. **Pipeline code + prompts/knowledge** (the contract) — the chain code (deterministic guards) plus its prompt under the knowledge/prompt registry.

The goal: is the stored output faithful to the source input, given the code and prompts?

**Execution:** discover the pipeline's output channels → ask the user which to audit → run the `audit-ai-output-sessions` workflow: it spawns one agent per unit of every subject (in parallel), each walking ITS unit data-first, code-last on the chosen channel(s), then a synthesizer judges the whole. One unit per agent bounds each agent's context to a single source + its output, so a long multi-subject sweep stays faithful instead of degrading as one agent walks every unit. The orchestrator never inline-audits — accumulated context biases the verdict and an inline pass has missed real failures before.

---

## Step 0 — Read the codebase

Read:

- The AI project's `CLAUDE.md` — pipeline conventions, chain structure
- The chain config (tiers, timeouts, token limits)
- The specific chain and prompt files identified in Step 1

---

## Step 1 — Identify the pipeline's output channels

Enumerate the channels — never trust a frozen list, chains evolve. Discover them from the store-write layer (the modules that hold the `INSERT`/write statements naming the exact output table/collection to query), cross-checked against the chain registry. Group them into the categories the user recognises.

Build a channel→file map (user-facing name → chain file → prompt file) as a starting reference, and verify each against the code before auditing — the mapping is discovered per install, never hardcoded here.

## Step 2 — Ask which channels to audit

Present the discovered categories and ask the user which to audit (`AskUserQuestion`, multi-select). Auditing every channel at once is expensive — never assume scope.

## Step 3 — Fan out one agent per unit, then synthesize

Run the saved `audit-ai-output-sessions` workflow (`.claude/workflows/audit-ai-output-sessions.js`), passing the chosen channel(s) and — for a re-audit — any already-audited unit ids to `exclude`. Its flow:

1. **Discover** — one agent enumerates from the store every unit of every subject that carries output for the chosen channel(s) (join the source table to the channel's output table) — the unit set is discovered, never hardcoded.
2. **Audit** — one `general-purpose` frontier-tier agent PER UNIT in parallel (`args.frontierModel`, durable default `opus` — faithfulness verdicts on real domain content never run below the frontier tier), each walking ITS unit data-first, code-last per the brief below.
3. **Synthesize** — a final frontier-tier agent (same `args.frontierModel`, default `opus`) quantifies the failure rates, WRITES the full report to `.professor/AUDIT/ai-output/{date}-{channel}.md`, and returns only a pointer + the headline numbers. The chat that invoked the audit then reads that file. This is the standing output contract for every `/audit:*` command — detailed results go to `.professor/AUDIT/{audit-type}/{date}-{component}.md`, kept out of the conversation's context, never dumped inline.

The workflow file is the declared copy of this flow — change both together. The orchestrator never inline-audits.

---

## Per-unit agent brief — walk the unit

Each agent owns ONE unit and walks it **data-first, code-last** — the order is the method. Pairing the source input with the output BEFORE opening the code is what stops you from "confirming" a violation the source never supported: a grounded, source-introduced fact is not a fabricated one, and you only learn that by reading what was there before you reach for a rule. The three sources:

### A. Source input — what went IN

The source is the only thing read from seed files. Get it from the seed export or the store's source table — both hold the same input. Preserve whatever stable locator the pipeline uses (segment index, message id, row id) and resolve any coded fields (e.g. a speaker/actor int) via the unit's mapping.

### B. AI-generated data — what came OUT (STORE ONLY)

Read the chain's stored output from the **store** — never from seed JSON.

- Query the store via the project's sanctioned DB/query command (never a raw client).
- Discover the table/collection and columns from the chain's store-write code — never assume a name.
- Confirm currency: check the created-at timestamp and row counts so you know which run produced the rows. If the output is empty or stale, regenerate it by running the analysis pipeline so the current chains write to the store — then audit the store.

### C. Pipeline code + prompts/knowledge — the contract

Read this LAST — only to localize a discrepancy the source-vs-output walk already exposed, never to pre-judge the output before you have seen what was in the input. Both halves:

- **The chain code** (from Step 1) — the deterministic guards and post-processing (drop-filters, brakes, actor enforcement) that shape output before it reaches the store, and how it prepares input (projection, unit selection, actor mapping). Faithfulness is often owned by code, not the prompt.
- **The prompt** under the knowledge/prompt registry — role and system instructions, output schema, domain rules and severity scales, compliance/forbidden-output blocks, few-shot examples.

---

### Cross-validation checks (per channel)

During the paired walk, flag any of these from the source-vs-output comparison; the scope and instruction-compliance rows are confirmed against the prompt in the root-cause pass:

| Check                      | What to look for                                                                                                                        | Severity if violated |
| -------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- | -------------------- |
| **Completeness**           | Did the model label/process ALL required units? (per-unit chains: count match; selective chains: no obvious skips)                       | HIGH                 |
| **Faithfulness**           | Do excerpts/quotes/references map to real source content? No hallucinated text?                                                          | CRITICAL             |
| **Actor accuracy**         | Are speakers/actors correctly attributed? No cross-attribution? Any outer-actor rule respected?                                          | CRITICAL             |
| **Instruction compliance** | Does the output follow the prompt's format, field constraints, severity scales, enums?                                                   | HIGH                 |
| **Grounding**              | Are labels/scores justified by what was actually in the source? Or is the model inventing significance?                                  | CRITICAL             |
| **Scope respect**          | Does the output stay within the prompt's boundaries? No content the prompt forbids ({FORBIDDEN_DOMAIN_OUTPUTS})?                         | CRITICAL             |
| **Locator accuracy**       | Do timestamps/indices/offsets correspond to the correct units? (for chains that emit them)                                              | MEDIUM               |
| **Consistency**            | For multi-item outputs: are severity/label distributions plausible? No all-neutral on a clearly non-neutral unit?                        | HIGH                 |
| **Token efficiency**       | Is the output bloated with echo-back data that should be derived post-hoc? Unnecessary repetition?                                       | MEDIUM               |

### How to walk the unit

**Phase 1 — paired walk (data only).** Audit your one assigned unit:

1. **Fetch the source AND this unit's channel output together** (angle A + angle B) — never one without the other.
2. **Read them side by side** — for each output row, find the source unit(s) it claims to come from and read what was actually there.
3. **Note every discrepancy** against the checks above (faithful/unfaithful + evidence: unit, index, output field). Do not open the code yet.
4. Cover every output row; sample only when the unit's output is high-volume. A unit with zero output rows is a completeness check — read the source and report whether anything codable was missed.

**Phase 2 — aggregate (within your unit).** Cluster the recurring discrepancies you found; cross-unit clustering is the synthesizer's job, not yours.

**Phase 3 — root-cause (now read the code).** For each cluster, open the chain code + prompt (angle C) to find WHERE it originates — a missing guard, a prompt rule, a brake that alarms but never drops. A rule the output "violates" is a finding only when the source-vs-output evidence already proved the output wrong; a grounded output that a rule dislikes is a rule question, not an output failure.

**Phase 4 — report** your unit's findings (structured for the synthesizer).

### Red flags to watch for

- Wrong-actor attribution when quoted content is present
- Excerpts that don't match any unit in the source (hallucinated)
- All-neutral labeling on a unit with obvious signal
- Severity inflation (everything is "high" without justification)
- Missing labels at the end of long units (model fatigue/truncation)
- Fabricated reasoning that sounds authoritative but doesn't match the source

---

### The synthesizer's report — written to the AUDIT file, led by the numbers

The synthesizer writes this to `.professor/AUDIT/ai-output/{date}-{channel}.md`. It LEADS with the failure rate — an overall %, then a per-category breakdown where each failure type carries its own denominator (so a mislabel category reads as failures / total-in-that-category, and a 1-of-1 reads as 100% with the n flagged) — then the detail:

```markdown
# AI Output Audit — {channel} — {date}

**Output source:** store (rows created_at {range}) · **Units audited:** {count}

## Failure rate

- **Overall: {N} of {total} outputs wrong = {%}.**

| Category      | Failures | Population | % within category | % of all failures |
| ------------- | -------- | ---------- | ----------------- | ----------------- |
| {label A → B} | 1        | 15         | 6.7%              | …                 |
| {label C → D} | 1        | 1 (n=1)    | 100%              | …                 |

## Verdict

{**FAITHFUL** / **MOSTLY FAITHFUL (N issues)** / **UNFAITHFUL — FIX REQUIRED**}

## Findings — CRITICAL / HIGH / MEDIUM·LOW

{per finding: subject · unit · index · field · Got vs Expected · source evidence}

## Completeness

{missed codable content, per subject}

## Recurring pattern & recommendations

{the one root confusion that explains the most failures — where a single example would help most; prompt fixes → /km, code/guard fixes → /jc}
```

---

## Constraints

- **Read-only** — this skill does NOT modify code. It produces findings and recommendations.
- **Evidence-based** — every finding references a specific unit index, a specific output field, and the prompt instruction it violates.
- **Domain lens first** — a technically valid output that's misleading in the domain is still a failure.
- **Sacred ground** — if the model is producing forbidden output ({FORBIDDEN_DOMAIN_OUTPUTS}), that's CRITICAL regardless of whether the prompt asked for it.
- **Delegate fixes** — if findings warrant code changes, recommend `/jc` (single chain fix) or `/wave:orchestrator` (systematic migration). This skill diagnoses, it doesn't treat.
