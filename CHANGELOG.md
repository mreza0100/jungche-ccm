# Changelog

All notable changes to the Jungche blueprint will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**For adopters:** run `/jm update` in your installed project to apply changes between your local version and the latest release. The update command parses this file to walk you through changes interactively.

---

## How `/jm update` reads this file

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

---

## [Unreleased]

*Pending changes for the next release will accumulate here.*

## [0.4.0] — 2026-05-08

### Added
- Tier A: `skills/360/SKILL.md` — new exhaustive multi-angle analysis skill with two domains: `test` (10 failure dimensions for QA) and `inquiry` (9 question dimensions for Professor). The blind-spot killer — forces systematic coverage before creative work. (safe-auto)

    #### For adopters:
    Copy `blueprint/templates/skills/360/SKILL.md` to `.claude/skills/360/SKILL.md`. Replace `{USER_PERSONA}` and `{SECONDARY_PERSONA}` in the inquiry domain's Stakeholder conflicts dimension with your persona terms.

### Changed
- Mechanics: `per-project/qa.md` — added Step 3.5 "360° sweep (test domain)" before adversarial test writing. QA agents now walk 10 failure dimensions before writing tests. (safe-auto)
- Tier A: `commands/professor.md` — added Step 1.5 "360° sweep (inquiry domain)" before deep dive. Professor now walks 9 question dimensions before code investigation. (safe-auto)
- Tier A: `skills/rr/SKILL.md` — agents no longer write intermediate files. Scout and fan-out agents return findings in chat only. Orchestrator writes ONE aggregate file at the end. No more `.scout.md` / `.{slug}.md` intermediates to clean up. (safe-auto)

    #### For adopters:
    Replace `.claude/skills/rr/SKILL.md` with `blueprint/templates/skills/rr/SKILL.md`. The change is behavioral — agents produce the same final file, but no intermediate files are created during the pipeline.

- Mechanics: `commands/jm.md` — added Codex skill symlink rule to impact check, "New skill creation" special operation, and skill parity to verification step. JM now guards against duplicating skill content across runtimes. (safe-auto)
- Docs: `CLAUDE.md` template — added `360` to the Skills table (safe-auto)
- Docs: `ARCHETYPES.md` — added "Skills — Thinking Protocols" section with 360° entry between Tier A and Tier B (safe-auto)
- Docs: `BLUEPRINT.md` — added 360° to the Tier A cast list (safe-auto)
- Docs: `SETUP.md` — added Step 7b for skills installation (safe-auto)
- Docs: `INSTALL.md` — added Step 8.6 for skills installation with parameterization instructions (safe-auto)
- Docs: `README.md` — complete rewrite. Problem-first pitch, character samples with actual quotes, pipeline visualization, honest positioning. (safe-auto)

## [0.1.2] — 2026-05-07

### Changed
- Mechanics: marketer.md — 688→381 lines, condensed verbose tables/sections to match token-trimmed density (safe-auto)
- Mechanics: pm.md — 364→158 lines, collapsed analysis framework and output templates (safe-auto)
- Mechanics: documenter.md — 473→252 lines, compressed audit mode and rules (safe-auto)
- Mechanics: wave.md — 404→182 lines, removed verbose explanations and bash blocks (safe-auto)
- Mechanics: council.md — 420→318 lines, simplified setup and round descriptions (safe-auto)
- Mechanics: professor.md — 779→565 lines, consolidated audit sub-mode tables (safe-auto)
- Mechanics: jm.md — 595→564 lines, genericized project references (safe-auto)
- Mechanics: mono-architect.md — 246→159 lines, compressed ownership and step descriptions (safe-auto)
- Mechanics: per-project/developer.md — 241→101 lines, collapsed verbose steps to compact bullets (safe-auto)
- Mechanics: per-project/qa.md — 316→77 lines, removed verbose taxonomy, kept essential checks (safe-auto)

### Migration

No adopter-side migration needed. All changes are structural density improvements — same behavior, fewer tokens. `/jm update` applies them without prompts.

## [0.1.1] — 2026-05-07

### Changed
- Mechanics: gitter.md — replaced merge-lock protocol with lightweight conflict-awareness check (safe-auto)
- Mechanics: gitter.md — reduced from 8 phases to 6 (removed LOCK, UNLOCK) (safe-auto)
- Mechanics: build.md — removed project-lock paragraph and lock release from Step 12 (safe-auto)
- Mechanics: jc.md — removed merge-lock acquisition/release steps, removed ISO environment detection (safe-auto)
- Mechanics: git.md — removed LOCK/UNLOCK from known phases (safe-auto)
- Mechanics: wave.md — removed merge-lock from cleanup checklist (safe-auto)
- Tier A: CLAUDE.md — added parallelization rule to Process section (safe-auto)
- Tier A: CLAUDE.md — added Skills section (rr, rnd) to cast reference (safe-auto)
- Tier B: renamed `/ckm` → `/km` across all templates (safe-auto)

### Added
- Mechanics: skills/rr/SKILL.md — Research & Report dynamic pipeline skill (safe-auto)
- Mechanics: skills/rnd/SKILL.md — Research & Develop iterative goal-seeker skill (safe-auto)
- Mechanics: per-project/qa.md — inline-fix escape hatch for trivial bugs (safe-auto)

### Removed
- Mechanics: Entire merge-lock protocol removed from gitter, build, jc, git, wave (safe-auto)
- Mechanics: ISO environment detection removed from jc (safe-auto)
- Mechanics: `gh` CLI references removed from jc (project-specific, not universal) (safe-auto)

### Migration

No adopter-side migration needed. All changes are `safe-auto` — `/jm update` applies them without prompts.
