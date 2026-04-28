# Changelog

All notable changes to the Jungche blueprint will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**For adopters:** run `/ccm update` in your installed project to apply changes between your local version and the latest release. The update command parses this file to walk you through changes interactively.

---

## How `/ccm update` reads this file

Each release section uses categorized headings the update flow understands:

| Heading | Apply how |
|---------|-----------|
| `### Added` | Auto-apply mechanics changes; ask before adding Tier B archetypes |
| `### Changed` | Auto-apply mechanics; show diff + ask for character changes |
| `### Fixed` | Auto-apply (bug fixes don't touch customization) |
| `### Removed` | Walk through interactively — never auto-delete |
| `### Breaking` | **Interactive walkthrough required.** Each change has explicit migration steps. |
| `### Migration` | Step-by-step transformation instructions for adopters |

Bullets MUST follow this shape:

```
- {Tier A|Tier B|Mechanics|Docs|Scripts}: {file path or scope} — {what changed semantically}
```

Optional trailing tags: `(opt-in)` for Tier B additions, `(breaking)` if it requires migration even outside a Breaking section, `(safe-auto)` to mark unconditional auto-apply.

**Migration sub-headings (inside `### Migration`):** for each bullet that needs adopter-side action beyond the file copy, write a `#### → For: <bullet identifier>` sub-heading followed by numbered steps. `/ccm update` matches each `#### → For:` against the bullets above and runs the listed steps in order. If a bullet is informational only, write `**`/ccm update`: skip — informational only.**` so the update flow knows there's nothing to do.

---

## [Unreleased]

*Pending changes for the next release will accumulate here.*

---

## [0.1.0] — 2026-04-28

**First public release.** The Jungche blueprint with the character-mandatory philosophy, the install-time manifest mechanism, and the LLM-first install protocol.

> **Versioning note:** this release replaces a briefly-published `v1.0.0` tag (live ~1 hour on 2026-04-28) that was yanked. `v0.1.0` reflects the project's actual maturity — pre-stable, evolving fast. `v1.0.0` will return when the public API (cast roster, install protocol, update mechanism, changelog format) is committed-to. No content was lost in the yank — `v1.0.0`'s contents are subsumed here.

### Added

**Philosophy & framework**

- Three-tier model (Tier A universal archetypes / Tier B domain archetypes / Tier C pure mechanics) — character is mandatory, content is parameterized at install
- Five load-bearing walls — only `gitter` touches git, QA gates the merge (pre AND post), path variables not hardcoded paths, worktree isolation per pipeline, self-improvement at the source (`/ccm` edits agent definitions; no journal files)

**Tier A — universal archetypes (full character preserved)**

- Tier A: Jungche persona — Dr. House senior engineer (sarcastic, witty, blunt, emoji-fluent); default name with rename-freely instruction
- Tier A: `/jc` — "Jesus Christ but make it cool" panic-debug mode; the ONLY command allowed to edit `main` directly
- Tier A: `/professor` — grandfatherly polymath with 10+ parameterized PhDs; intersection lens
- Tier A: `/council` — three-round debate (opening / rebuttal / verdict); JC + Professor mandatory + 3 parameterized panel seats
- Tier A: `/ccm` — meta-engineer that edits the pipeline at the source; ships `update` subcommand for adopter-side version pulls
- Tier A: `/ca` — codebase hygiene + 9-category security audit; Jungche-in-janitor-mode voice

**Tier B — domain archetypes (opt-in templates with documented placeholders)**

- Tier B: `/officer` (opt-in) — compliance enforcer; `{REGULATION}`, `{ENFORCEMENT_AUTHORITY}`, `{DATA_SUBJECT_RIGHTS}`, `{INCIDENT_NOTIFICATION_TIMELINE}`, `{SACRED_GROUND_DATA}`
- Tier B: `/ckm` (opt-in) — knowledge curator; `{KNOWLEDGE_DOMAIN}`, `{KNOWLEDGE_TAXONOMY}`, `{KNOWLEDGE_CONSUMERS}`, `{SOURCE_AUTHORITIES}`, `{KNOWLEDGE_ROOT}`
- Tier B: `/pm` (opt-in) — user+product hybrid; `{USER_PERSONA}`, `{USER_PROFESSION}`, `{PRODUCT_DOMAIN}`, `{USER_DAILY_WORKFLOW}`, `{USER_PAIN_POINTS}`, `{PERSONA_VARIANTS}`; Love Meter framework
- Tier B: `/mentor` (opt-in) — business advisor; `{MARKET_SEGMENT}`, `{JURISDICTION}`, `{LEGAL_ENTITY_TYPE}`, `{FUNDING_LANDSCAPE}`, `{REGULATORY_BODIES}`, `{TAX_INCENTIVES}`
- Tier B: `/marketer` (opt-in) — anti-hype visibility strategist; `{CHANNEL_LANDSCAPE}`, `{TARGET_LANGUAGE}`, `{COMPETITIVE_LANDSCAPE}`, `{INDUSTRY_CONFERENCES}`, `{AUDIENCE_VOCABULARY}`

**Tier C — pure mechanics**

- Mechanics: `/build` — full pipeline orchestration (planner → architect → developer → QA → gitter merge → post-merge QA → pipeline audit → archive)
- Mechanics: `/build` Step 9.5 — pipeline audit runs `/ca` (always) + `/officer` (when opted in) in parallel between post-merge QA and mono-documenter
- Mechanics: `/dev`, `/git`, `/wave`, `/documenter` — pipeline plumbing with light Jungche voice in reports

**Templates**

- Templates: `templates/CLAUDE.md` — Jungche persona section non-deletable; `{SACRED_GROUND}` and `{USER_PERSONA}` placeholders; full cast table
- Templates: `templates/agents/` — root agents (`gitter`, `mono-planner`, `mono-architect`, `mono-documenter`)
- Templates: `templates/agents/per-project/` — child agents (`planner`, `architect`, `developer`, `qa`) for monorepo installs
- Templates: `templates/scripts/` — `worktree.sh`, `alloc-ports.sh`, `dev.sh`

**Install protocol**

- Mechanics: `SETUP.md` — 10-question interactive interview (Claude in adopter project conducts it before any file write)
- Mechanics: `.claude/JUNGCHE_VERSION` — single-line semver written at install for adopter version tracking
- Mechanics: `.claude/JUNGCHE_MANIFEST.json` — install-time SHA-256 manifest of every Jungche-owned file (post-placeholder-substitution). Baseline for `/ccm update`'s three-way customization detection (installed vs. current vs. upstream-new). Format: `{ version, installed_at, files: { "<path>": "sha256:<hash>" } }`.

**Update mechanism (`/ccm update`)**

- Mechanics: truth-table-based customization detection — four cases (pristine / safe-apply / preserve-user / real-conflict) plus edge cases (new files, deleted files, removed-upstream, pre-manifest installs)
- Mechanics: per-bullet migration sub-headings (`#### → For: <bullet>`) so changelog entries can specify exact upgrade steps per change
- Mechanics: manifest regeneration after every successful update so the new on-disk state becomes the next baseline
- Mechanics: rollback safety net — failed updates restore the prior `JUNGCHE_VERSION` + manifest rather than leaving a half-updated state

**Machine-to-machine install**

- Docs: `LLM_INSTALL.md` (repo root) — token-dense entry point another LLM fetches when a user types "install Jungche in this project". Contains identity, full cast roster, three-tier model, install protocol, lazy-load URL map, verification gate, and efficiency rules. Bypasses README marketing copy and avoids fetching reference docs that aren't needed during install.

**Reference docs**

- Docs: `BLUEPRINT.md` — three-tier framework, five load-bearing walls, pipeline architecture
- Docs: `ARCHETYPES.md` — full cast catalog with voice samples, parameterization, adaptation examples across domains (therapy AI, neuropsych, game studio, FinTech, SCADA, open-source library)
- Docs: `ADAPTATION.md` — archetype-by-archetype customization guide; voice-is-non-negotiable rule
- Docs: `RELEASE.md` — versioning + maintainer release process; documents `JUNGCHE_VERSION` and `JUNGCHE_MANIFEST.json`
- Docs: `README.md` — quick start leads with the M2M one-liner; "Staying current" section documents `/ccm update` apply modes; repo layout

### Migration

This is the first public release. Adopters install via the M2M handshake (see `LLM_INSTALL.md`) or the manual `SETUP.md` flow. **No prior version exists to migrate from.**

The `/ccm update` infrastructure ships IN this release so future releases (`v0.2.0+`) can use it. The first time a `v0.1.0`-installed adopter runs `/ccm update`, the manifest will be present (it was written at install) and the three-way compare works on the first invocation.

### Notes

- The yanked `v1.0.0` tag (briefly live ~1 hour on 2026-04-28) is superseded by `v0.1.0`. The pre-stable `0.x` versioning lets the project iterate the public API (cast roster, install protocol, update mechanism, changelog format) without semver violations until things settle.
- The mock policy (external yes / internal-within-1-hop no) and zero-tolerance test policy are inherited unchanged from the parent project (Freudche).
