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

## [0.3.0] — 2026-04-28

### Fixed

- Docs: `INSTALL.md` Batch 7 — persona is now MANDATORY (was "skip" option). Adopter must pick keep-Jungche / rename / custom-voice AND name a sacred-ground topic. Without this fix, fresh installs were silently emitting `CLAUDE.md` files with no persona section, producing tonal whiplash (vanilla orchestrator while `/jc`, `/professor`, `/council` kept their voices). The blueprint philosophy already said Tier A character ships by default; the install protocol disagreed. Now they agree. (breaking)
- Docs: `INSTALL.md` Step 2 — adds explicit "the file MUST contain a `## Your character — {NAME} (MANDATORY` heading" verification before saving CLAUDE.md. Strips install-only `> Rename if you want.` admonition from the emitted file.
- Docs: `INSTALL.md` Step 7 — Tier B command emission now strips the leading `>`-quoted "Required placeholders (fill at install)" meta-block from each emitted `.md` file. That block is installer scaffolding; in v0.1.0–v0.2.0 it leaked into runtime command files (officer.md, mentor.md, marketer.md, ckm.md, pm.md). Adds a verify grep (`fill at install` / `Skip if:` / `Required placeholders` / `Tier B — Domain archetype`) to catch leakage before moving on. (breaking)
- Docs: `INSTALL.md` Hard Rule 4 — reframed from "never inject Freudche's character without explicit consent" to "never inject Freudche-specific *content*". Old wording contradicted the three-tier philosophy (Tier A persona is universal, ships by default; only Freudche-specific *content* — therapy/clinical/AVG/AssemblyAI/etc. — is forbidden).
- Docs: `blueprint/SETUP.md` § 2 — interview-side mirror of Batch 7 fix: persona is non-skippable, sacred-ground prompt added.
- Docs: `blueprint/SETUP.md` Phase 2 step 4 — interview-side mirror of Step 7 fix: notes the meta-block is stripped at emission, so adopters reading SETUP.md see the new behavior documented.

### Added

- Docs: `INSTALL.md` Step 8.5 (NEW) — copies `blueprint/ARCHETYPES.md` to `.claude/ARCHETYPES.md` verbatim during install. Every fresh install now ships with the canonical Cast bible (was previously discoverable only by piecing together individual command files).
- Docs: `blueprint/SETUP.md` Phase 2 step 7a — interview-side documentation of the ARCHETYPES.md copy.

### Migration

#### → For: `Docs: INSTALL.md Batch 7` (existing installs may be missing the persona section)

If your installed `CLAUDE.md` does NOT contain a `## Your character — {NAME} (MANDATORY` heading, you hit the v0.1.0–v0.2.0 bug. Fix:

1. Open your `CLAUDE.md`.
2. Insert a `## Your character` section between the project intro and `## The GOAL`. Use `~/work/jungche-ccm/blueprint/templates/CLAUDE.md`'s persona section (lines 18–41) as a starting point.
3. Adapt the "What NOT to do" first bullet to your project's sacred ground (the topic where Claude must drop the humor: PHI, user funds, physical safety, regulatory output, etc.).
4. Add a `| **{NAME}** (you) | A | Orchestrator, in-character voice |` row at the top of your Cast table.

If your `CLAUDE.md` already has a persona section, **`/ccm update`: skip — informational only.**

#### → For: `Docs: INSTALL.md Step 7` (existing Tier B commands may have leaked meta-blocks)

For each Tier B command you opted into (`/officer`, `/mentor`, `/marketer`, `/ckm`, `/pm`), check the top of `.claude/commands/{cmd}.md`. If the file starts with a `>`-quoted block containing `**Tier B — Domain archetype.**` and `**Required placeholders (fill at install):**`, you hit the leak. Fix per file:

1. Delete the entire leading `>`-quoted block — from the line starting with `> **Tier B — Domain archetype.**` through the line starting with `> **Skip if:**` inclusive (plus any blank `>` lines between).
2. The file should now start directly with the H1 heading (e.g., `# Officer — Compliance & Privacy`) followed by the `Handle this request: $ARGUMENTS` (or equivalent) line.
3. Verify: `grep -lE "fill at install|Required placeholders|Tier B — Domain archetype" .claude/commands/*.md` should return nothing.

#### → For: `Docs: INSTALL.md Step 8.5` (existing installs missing ARCHETYPES.md)

If `.claude/ARCHETYPES.md` doesn't exist in your install, copy it:

```bash
curl -fsSL https://raw.githubusercontent.com/mreza0100/jungche-ccm/main/blueprint/ARCHETYPES.md > .claude/ARCHETYPES.md
```

#### → For: `Docs: INSTALL.md Step 2` (installer-only verification step)

**`/ccm update`: skip — informational only.** No adopter-side files are emitted from this rule directly; the persona-section migration above already covers existing-install impact.

#### → For: `Docs: INSTALL.md Hard Rule 4` (installer-only framing)

**`/ccm update`: skip — informational only.**

#### → For: `blueprint/SETUP.md` § 2 + Phase 2 step 4 + step 7a

**`/ccm update`: skip — informational only.** `SETUP.md` is the human-facing interview reference; no adopter-side files are derived from it. Behavior changes are enforced via `INSTALL.md` (above) which IS the executable install protocol.

---

## [0.2.0] — 2026-04-28

### Added

- Docs: `INSTALL.md` Pre-flight Step 7 — detects existing project markdown (THESIS, MENTOR_BRIEFING, COMPETITOR_LANDSCAPE, REGULATORY_LANDSCAPE, etc.) and surfaces them in findings before classification.
- Docs: `INSTALL.md` Pre-flight Step 1 — non-git-repo handling now explicit. Asks user to `git init` first (preserves `git mv` history) or proceed without git. Never `git init` silently.
- Docs: `INSTALL.md` Step 1.5 (NEW) — Re-home existing project docs with a deterministic classification rubric. Maps content type → destination: `docs/business/` for thesis/vision/strategy, `$CDOCS/mentor/` for GTM/buyer/primer/risk, `$CDOCS/marketer/` for competitor/positioning/channel, `$CDOCS/officer/` for regulatory/compliance, `$CDOCS/pm/` for persona/pain/workflow, `$CDOCS/ckm/` for knowledge primers, `docs/dev/research/` for research-log/open-questions/experiments. Includes `$REFS` vs `$RESEARCH` decision rule, fallback when archetype not opted in (ask to opt-in or place in `docs/dev/research/` and flag), and `git mv` vs plain `mv` per repo state.
- Docs: `INSTALL.md` Step 1 — also creates `$CDOCS/<archetype>/{references,research,resources}` subtrees per opted-in Tier B.
- Docs: `INSTALL.md` Batch 8 — confirmation step now shows proposed re-home moves and unclassified files alongside the file write list.

### Changed

- Docs: `INSTALL.md` — fixed `/tpm` → `/pm` (rename happened upstream); removed `/blueprint` from "core" list (it's the maintainer command in Freudche, not an adopter command); added Tier B framing.
- Docs: `INSTALL.md` — execution Step 9 (record `JUNGCHE_VERSION` + `JUNGCHE_MANIFEST.json`) and Step 10 (smoke test, was Step 9). The manifest write-step was missing in v0.1.0's INSTALL.md.

### Migration

#### → For: `Docs: INSTALL.md Step 1.5` (no action for existing installs)

This change only affects FRESH installs. Existing installs already chose where their docs live; re-running the rubric on a settled project would just churn paths. **`/ccm update`: skip — informational only.** If an existing user wants to re-home docs after the fact, they can ask `/ccm` to apply the rubric retroactively (one-shot operation, not part of `/ccm update`).

#### → For: `Docs: INSTALL.md` Pre-flight + Batch 8 + Step 1 polish

Same — fresh-install only. **`/ccm update`: skip — informational only.**

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

**Reference docs**

- Docs: `BLUEPRINT.md` — three-tier framework, five load-bearing walls, pipeline architecture
- Docs: `ARCHETYPES.md` — full cast catalog with voice samples, parameterization, adaptation examples across domains (therapy AI, neuropsych, game studio, FinTech, SCADA, open-source library)
- Docs: `ADAPTATION.md` — archetype-by-archetype customization guide; voice-is-non-negotiable rule
- Docs: `RELEASE.md` — versioning + maintainer release process; documents `JUNGCHE_VERSION` and `JUNGCHE_MANIFEST.json`
- Docs: `INSTALL.md` (repo root) — interactive installer protocol. Written FOR Claude in the adopter's project: pre-flight checks, 8 question batches (project identity, structure, test/build commands, ports, domain & disciplines, optional commands, character, confirmation), execution order, and hard rules ("never assume", "never overwrite without asking", "never inject Freudche's character"). Includes a Professor template that fills from the discipline-picker batch.
- Docs: `README.md` — quick start hands Claude the `INSTALL.md` URL one-liner; "Staying current" section documents `/ccm update` apply modes; repo layout

### Migration

This is the first public release. Adopters install via the M2M handshake (see `LLM_INSTALL.md`) or the manual `SETUP.md` flow. **No prior version exists to migrate from.**

The `/ccm update` infrastructure ships IN this release so future releases (`v0.2.0+`) can use it. The first time a `v0.1.0`-installed adopter runs `/ccm update`, the manifest will be present (it was written at install) and the three-way compare works on the first invocation.

### Notes

- The yanked `v1.0.0` tag (briefly live ~1 hour on 2026-04-28) is superseded by `v0.1.0`. The pre-stable `0.x` versioning lets the project iterate the public API (cast roster, install protocol, update mechanism, changelog format) without semver violations until things settle.
- The mock policy (external yes / internal-within-1-hop no) and zero-tolerance test policy are inherited unchanged from the parent project (Freudche).
