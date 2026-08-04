# Red Team Verifier — Adversarial QC on Legal Drafts

> Distilled from the `red-team-verifier` skill in github.com/lawve-ai/awesome-legal-skills (AGPL-3.0, © Patrick Munro) — summarised methodology, not copied. The pre-delivery self-check (`pre-delivery-self-check.md`) is the condensed action list; this file is the fuller adversarial method for a dedicated verification pass.

Purpose: independently verify AI-generated legal content for factual accuracy, correct citations, and adequate disclaimers **before** it reaches a client or regulator. It answers the first question anyone asks of AI legal output — "how do I know this is accurate?"

## Adversarial mindset

- **Assume error until proven correct** — do not trust the draft; prove each claim.
- **Seek contradictory evidence** — actively look for what refutes a claim.
- **Question every number** — recompute every calculation.
- **Demand a source** — every factual claim needs verifiable attribution.
- **Test logical consistency** — hunt internal contradictions ("3 categories" — are there exactly 3?).
- **Challenge interpretations** — verify any legal reading against an authoritative source.

## Six verification categories

1. **Factual accuracy** — dates, deadlines, transition periods, article/§ references, numeric thresholds, entity names, timelines.
2. **Legal-authority citations** — primary vs. secondary sources, correct citation format (EUR-Lex / official-journal), current vs. superseded provisions, primary law not confused with guidance.
3. **Arithmetic** — independently recompute every timeline and deadline from the verified effective date; verify percentages and figures; check internal consistency.
4. **Source verification** — verify every claim against an official source; flag any unsourced statistic or quote; cross-reference critical claims across multiple sources.
5. **Speculation detection** — separate opinion from fact; label predictive statements as speculation; acknowledge unsettled law; spot interpretive framing inserted by the model.
6. **Disclaimer adequacy** — not-legal-advice where appropriate, jurisdiction stated, version/date of regimes cited, professional-consultation note where needed.

## Source hierarchy

1. Primary legal sources — official legislation (EUR-Lex, official gazettes, legislation.govt.nz).
2. Official guidance — regulator publications.
3. Secondary — court decisions, legal commentary.
4. Tertiary — news, blogs (use with extreme caution).

## Severity taxonomy for findings

- **CRITICAL** (must fix before distribution) — factual errors, arithmetic mistakes, legal misstatements, attribution errors.
- **HIGH** (strongly recommend fixing) — missing critical disclaimer, undisclosed regulatory uncertainty, jurisdiction ambiguity, superseded references.
- **MODERATE** — unsourced statistics, opinion framed as fact, vague language, incomplete citations.
- **LOW** — minor inconsistencies, stylistic nits.

## Known model-hallucination patterns to hunt

- Plausible but non-existent article numbers (e.g. citing 42(5) when the act ends at 42(4)).
- Confident but wrong dates (right month, wrong day).
- Guidance presented as binding legal requirement.
- Outdated/superseded provisions cited as current.
- Arithmetic slips in deadline calculation (e.g. "18 months from October" landing a month off).

## Output

A verification report: overall distribution-readiness (READY / NEEDS REVISION / MAJOR CORRECTIONS), verified facts with sources, errors by severity with the correction and the correct source, unsupported claims to source-or-cut, and missing-disclaimer additions. When in doubt, verify; when you cannot verify, flag it.
