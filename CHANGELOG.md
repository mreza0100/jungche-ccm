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

## [Unreleased]

*Pending changes for the next release will accumulate here.*
