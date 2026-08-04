# Privacy Compliance & DPA Review (Art. 28)

> Adapted from the `compliance-anthropic` skill in github.com/lawve-ai/awesome-legal-skills (Apache-2.0, © Anthropic). Use when reviewing a Data Processing Agreement / Addendum, handling a data-subject request, or assessing a cross-border transfer. Assists legal workflow — not legal advice; regulatory requirements change, so verify currency per the pre-delivery self-check.

## DPA — required elements (GDPR Art. 28(3))

A compliant DPA must define all of:

- **Subject matter and duration** of the processing.
- **Nature and purpose** — what processing occurs and why.
- **Type of personal data** and **categories of data subjects**.
- **Controller's obligations and rights** — its instructions and oversight rights.

## DPA — processor obligations to verify

- **Documented-instructions only** — processor processes solely on the controller's documented instructions (carve-out for legal requirements).
- **Confidentiality** — authorised personnel are bound to confidentiality.
- **Security** — appropriate technical and organisational measures (Art. 32 referenced).
- **Sub-processors** — written authorisation (general or specific); if general, notice of changes with a right to object; sub-processors bound by equivalent terms; processor stays liable for them.
- **Data-subject-rights assistance** — processor helps the controller answer requests.
- **Security & breach assistance** — processor assists with Art. 32–36 (security, breach notice, DPIA, prior consultation).
- **Deletion or return** — on termination, delete or return all data at the controller's choice and delete copies, unless legal retention applies.
- **Audit rights** — controller may audit/inspect, or accept third-party audit reports.
- **Breach notice to controller** — without undue delay (target 24–48h) so the controller can meet its 72h regulatory deadline.

## DPA — international transfers

- **Mechanism identified** — adequacy decision, current **EU SCCs (2021)**, or BCRs.
- **Correct SCC module** — C2P, C2C, P2P, or P2C as the relationship requires.
- **Transfer impact assessment** — completed for transfers to non-adequate countries, with supplementary measures for any gap.
- **UK addendum** — included where UK personal data is in scope.

## Common DPA red flags

| Red flag                                                | Risk                             | Standard position                                                       |
| ---------------------------------------------------------- | ---------------------------------- | ---------------------------------------------------------------------------- |
| Blanket sub-processor authorisation, no notice          | Loss of control over the chain   | Require notice + right to object                                        |
| Breach-notice window > 72h                              | Blocks timely regulatory notice  | Require 24–48h                                                          |
| No audit rights (or only third-party reports)           | Cannot verify compliance         | Accept SOC 2 Type II **plus** audit-on-cause                            |
| Deletion timeline unspecified                           | Data retained indefinitely       | Require deletion within 30–90 days                                      |
| Processing locations unspecified                        | Data processed anywhere          | Require disclosure of locations                                         |
| Outdated SCCs                                            | Invalid transfer mechanism       | Require current (2021) EU SCCs                                          |
| **Liability / insurance / commercial terms in the DPA** | Scope creep, conflicting clauses | Move to the master services agreement — Art. 28 is data-protection only |

## Data-subject request handling

1. **Identify request type** — access, rectification, erasure, restriction, portability, objection (GDPR); opt-out of sale/share, limit sensitive-PI use (CCPA/CPRA).
2. **Identify applicable regime** — by where the subject is and what laws bind us.
3. **Verify identity** — proportionate to data sensitivity; do not demand excessive proof.
4. **Log** — date received, type, requester, regime, deadline, handler.
5. **Apply exemptions** — legal-claim defence, statutory retention, third-party rights, freedom of expression (erasure), litigation hold. Cite the basis for any denial.
6. **Respond and record** — fulfil or explain; tell the requester of their right to complain to the supervisory authority.

**Response timelines:** GDPR/UK-GDPR — 30 days, extendable +60 for complex requests. CCPA/CPRA — acknowledge in 10 business days, substantive response in 45 calendar days, extendable +45. LGPD — 15 days.

## {PROJECT_NAME} application

{PROJECT_NAME} is a processor for its {USER_NOUN}-clients on {SESSION_NOUN} data and a controller for account data — confirm which hat applies before drafting. The **controller named in a processor-side DPA is the client {USER_NOUN}**, not the founder. Sub-processors to cover: the transcription service, the AI analysis service's cloud provider, and the cloud infrastructure provider — each needs its own DPA and a transfer mechanism. Cross-reference the officer's `sub-processor-compliance.md` for current status.
