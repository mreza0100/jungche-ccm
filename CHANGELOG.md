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

### Added

- Mechanics: `.claude/JUNGCHE_MANIFEST.json` — install-time SHA-256 manifest of every Jungche-owned file. Baseline for `/ccm update`'s three-way hash compare (installed vs. current vs. upstream-new) so customization detection is deterministic instead of best-effort diffing.
- Docs: `blueprint/SETUP.md` Step 11 — install now writes the manifest after placeholder substitution.
- Docs: `blueprint/templates/commands/ccm.md` Step 5 — full truth table covering pristine / safe-apply / preserve-user / real-conflict, plus edge cases (new files, deleted files, removed-upstream, pre-manifest installs).
- Docs: `blueprint/templates/commands/ccm.md` Step 9 — manifest regeneration after a successful update so the new on-disk state becomes the next baseline.
- Docs: `blueprint/RELEASE.md` — adopter version-tracking section now documents the manifest alongside `JUNGCHE_VERSION`.

### Migration

Per-bullet upgrade instructions. `/ccm update` reads these and executes them in order. **All steps in this release are non-breaking and run automatically — no user prompts required for the migration itself, though file-by-file changes still go through the normal `/ccm update` apply flow.**

#### → For: `Mechanics: .claude/JUNGCHE_MANIFEST.json` (bootstrap on first run)

An existing v1.0.0 install does NOT have a manifest. Before doing anything else, `/ccm update` MUST bootstrap one from the current on-disk state, treating "what the user has right now" as their starting baseline.

**Steps `/ccm update` runs automatically:**

1. **Detect missing manifest:**
   ```bash
   if [ ! -f .claude/JUNGCHE_MANIFEST.json ]; then
     echo "Pre-1.0.1 install detected — bootstrapping manifest from current state."
   fi
   ```
2. **Identify Jungche-owned files** by intersecting the on-disk filesystem with the v1.0.0 blueprint's file list (fetched from `git show v1.0.0:blueprint/templates/...`):
   - `CLAUDE.md` (root)
   - `.claude/agents/*.md`
   - `.claude/commands/*.md`
   - `.claude/scripts/*.sh`
   - Per-project `CLAUDE.md` and `.claude/agents/*.md` (if monorepo)
3. **Hash each file in its current state** (NOT the original blueprint — the user may have customized since install):
   ```bash
   sha256sum {file} | awk '{print "sha256:" $1}'
   ```
4. **Write `.claude/JUNGCHE_MANIFEST.json`** with `version: "1.0.0"` (their actual install version, NOT the new version yet) and `installed_at: <bootstrap timestamp + note>`:
   ```json
   {
     "version": "1.0.0",
     "installed_at": "2026-04-28T14:32:00Z",
     "bootstrapped": true,
     "bootstrap_note": "Manifest reconstructed from on-disk state on first /ccm update — pre-1.0.1 install. Customization detection treats current state as baseline.",
     "files": { "...": "sha256:..." }
   }
   ```
5. **Warn the user** once, in the update report:
   > "Your install predates the manifest (v1.0.0). I bootstrapped one from your current files. Customization detection going forward is reliable; for THIS update, files you customized between install and now will look pristine. Ask me to show diffs if you want to double-check anything."

**Why this is safe:** the bootstrap conservatively treats current state as baseline. The downside is that customizations made between install and the first `/ccm update` won't be auto-detected as customizations — they'll be silently adopted as the new baseline. Acceptable tradeoff: any subsequent update has accurate detection, and the user gets a one-time warning.

#### → For: `Docs: SETUP.md Step 11` (no action for existing installs)

This change only affects FRESH installs. Existing installs get the same behavior via the bootstrap above. **`/ccm update`: skip — informational only.**

#### → For: `Docs: ccm.md Step 5` (apply on update)

The new `/ccm update` flow uses the truth table. The user gets the new behavior automatically the next time they run `/ccm update` AFTER this release lands — but THIS run is the transition. So:

1. **Use the OLD behavior for this run only** (the pre-truth-table flow that was running when the user typed `/ccm update`). The new ccm.md gets applied as a normal `Docs:` change to `.claude/commands/ccm.md` during this run.
2. **Inform the user** at end of run:
   > "Heads up: starting next `/ccm update`, customization detection uses the new SHA-256 truth table. Your manifest is in place, so there's nothing to do — it just works better."

#### → For: `Docs: ccm.md Step 9` (apply automatically at end of run)

Step 9 (regenerate manifest after update) is the FIRST thing `/ccm update` should adopt — even on this transition run. After the user accepts/rejects each file change, the manifest gets rewritten to reflect the new on-disk reality:

1. After all per-file decisions are committed to disk, regenerate:
   ```bash
   jq -n --arg v "$LATEST_VERSION" --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
     '{version: $v, installed_at: $ts, files: {}}' > .claude/JUNGCHE_MANIFEST.json
   # Append every Jungche-owned file with its new sha256
   ```
2. Drop the `bootstrapped: true` flag if it was present — after a clean update, the manifest is no longer a reconstruction.
3. Update `.claude/JUNGCHE_VERSION` to the new version AFTER the manifest is written successfully (so a partial failure leaves a recoverable state).

#### → For: `Docs: RELEASE.md` (no action)

Maintainer-side documentation only. **`/ccm update`: skip — adopters never read RELEASE.md.**

---

**Migration safety net:** if any step above fails (disk full, permission denied, jq missing), `/ccm update` MUST roll back to the pre-update state — meaning the OLD `JUNGCHE_VERSION` and OLD manifest (or no manifest, if bootstrapping failed). Never leave the user in a half-updated state where the version says "1.0.1" but the manifest is missing or stale.

---

## [1.0.0] — 2026-04-28

**Initial public release** of the Jungche blueprint with the character-mandatory philosophy.

### Added

- **Mechanics: `VERSION` + `CHANGELOG.md`** — release versioning + adopter-facing changelog
- **Tier A: full cast of universal archetypes ship with character preserved** — Jungche, /jc, /professor, /council, /ccm, /ca
- **Tier B: domain archetype skeletons ship as opt-in templates** — /officer, /pm, /mentor, /marketer, /ckm (each with documented placeholder list)
- **Mechanics: pipeline command set** — /build, /dev, /git, /wave, /documenter
- **Mechanics: pipeline audit step** — /build Step 9.5 runs /ca (always) + /officer (when opted in) in parallel between post-merge QA and mono-documenter
- **Docs: `BLUEPRINT.md`** — three-tier framework (Universal / Domain / Mechanics), five load-bearing walls, pipeline architecture
- **Docs: `ARCHETYPES.md`** (NEW) — full cast catalog with voice samples, parameterization, adaptation examples across multiple domains (therapy AI, neuropsych, game studio, FinTech, SCADA, open-source library)
- **Docs: `ADAPTATION.md`** — archetype-by-archetype customization guide, voice-is-non-negotiable rule
- **Docs: `SETUP.md`** — 10-question interactive install interview Claude conducts (Phase 1 interview → Phase 2 customization → Phase 3 smoke test)
- **Templates: `templates/CLAUDE.md`** — Jungche persona section is **non-deletable**; `{SACRED_GROUND}` and `{USER_PERSONA}` placeholders; full cast table with Tier A always-on, Tier B conditional
- **Templates: `templates/agents/per-project/`** — child agents (architect, developer, planner, qa) live under per-project/ to match BLUEPRINT.md file layout

### Changed

- **Philosophy: character is mandatory, content is parameterized** — replaces the previous "technology-agnostic + personality-optional" approach. The blueprint exports a transplantable nervous system, not a sanitized spec.
- **Mechanics: `templates/commands/build.md`** — added Step 9.5 (pipeline audit), updated Pipeline Reference table

### Removed

- **Philosophy: "Character is OPTIONAL" rule** — explicitly forbidden. Tier A archetypes ship with full voice; the empty-template trap is forbidden.
- **Philosophy: technology-agnostic gag rule** — replaced with parameterized-content rule (techs are still placeholders, but the install interview fills them in concretely)

### Notes

- The five load-bearing walls remain non-negotiable: only gitter touches git, QA gates the merge, path variables not hardcoded paths, worktree isolation per pipeline, self-improvement at the source.
- The mock policy (external yes / internal-within-1-hop no) and zero-tolerance test policy are unchanged.
