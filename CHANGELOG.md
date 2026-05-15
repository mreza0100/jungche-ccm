# Changelog

All notable changes to the Professor blueprint will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**For adopters:** run `/pcm update` in your installed project to apply changes between your local version and the latest release. The update command parses this file to walk you through changes interactively.

---

## How `/pcm update` reads this file

Each release section uses categorized headings the update flow understands:

| Heading         | Apply how                                                                       |
| --------------- | ------------------------------------------------------------------------------- |
| `### Added`     | Auto-apply mechanics changes; ask before adding Tier B archetypes               |
| `### Changed`   | Auto-apply mechanics; show diff + ask for character changes                     |
| `### Fixed`     | Auto-apply (bug fixes don't touch customization)                                |
| `### Removed`   | Walk through interactively — never auto-delete                                  |
| `### Breaking`  | **Interactive walkthrough required.** Each change has explicit migration steps. |
| `### Migration` | Step-by-step transformation instructions for adopters                           |

Bullets MUST follow this shape:

```
- {Tier A|Tier B|Mechanics|Docs|Scripts}: {file path or scope} — {what changed semantically}
```

Optional trailing tags: `(opt-in)` for Tier B additions, `(breaking)` if it requires migration even outside a Breaking section, `(safe-auto)` to mark unconditional auto-apply.

---

## [Unreleased]

---

## [0.6.2] — 2026-05-15

### Added

- Scripts: `notify.sh` — macOS native notification with Glass sound for turns >30s (safe-auto)

### Changed

- Tier A: `agents/gitter.md` — added Remote Publication Boundary section and Phase 5 PUSH hard gate; explicit user request required for any remote push (safe-auto)
- Tier A: `commands/git.md` — added push guard rule to Rules section (safe-auto)
- Mechanics: `codex/agents/gitter.toml` — added remote publication boundary, updated sandbox state to reference default.rules enforcement (safe-auto)
- Mechanics: `codex/agents/build.toml` — removed auto-push after DOCS-COMMIT; remote publication requires explicit user request (safe-auto)
- Mechanics: `codex/agents/wave.toml` — removed auto-push from build child return values and wave-end archive commit (safe-auto)
- Mechanics: `codex/rules/default.rules` — `git push` rule justification updated to protocol-controlled (explicit user request only) (safe-auto)
- Docs: `SETUP.md` — added notification hook documentation with PreToolUse/Stop hook config (safe-auto)

---

## [0.6.1] — 2026-05-14

### Added

- Scripts: `check-codex-research-contract.sh` — validates that Codex installs include shared `360`, `rr`, `rnd`, and `ghostwriter` skill wrappers and that explicit RR remains scout/fan-out/aggregate instead of inline search. (safe-auto)
- Mechanics: `templates/codex/skills/{360,rr,rnd,ghostwriter}/SKILL.md` — shared-skill wrappers for Codex installs so Claude and Codex mirror the same protocols. (safe-auto)

### Changed

- Mechanics: Codex agent wrappers — describe Claude and Codex as peers reading the same Professor contract; wrappers translate runtime mechanics only, not identity. (safe-auto)
- Tier A: `commands/build.md` install contract — `/build` must be materialized from the installed project roster and fail install if optional web/infra-style placeholders or missing agent paths remain. (safe-auto)
- Tier A: `commands/pcm.md` — Codex skill parity now accepts wrappers or symlinks and audits RR contract parity. (safe-auto)

### Fixed

- Mechanics: Codex RR handling — removed stale "rr skill is Claude-side" guidance from Codex command/skill wrappers. Broad research now routes through the shared RR-compatible pipeline; WebSearch/WebFetch is limited to narrow fact checks. (safe-auto)

---

## [0.6.0] — 2026-05-13

### Added

- Mechanics: `commands/pcm.md` § "Update Protocol" — full `/pcm update` implementation with manifest-driven replay, git tag version pinning, three-way hash comparison, three-bucket diff (auto-apply / review / manual). (safe-auto)
- Mechanics: `.professor/` directory — replaces `.claude/PROFESSOR_*` files. Contains `VERSION`, `manifest.json` (interview replay seed + file hashes), and `decisions.md` (human-readable customization log). (breaking)
- Scripts: `format-md.sh` — PostToolUse hook that auto-formats Professor-owned `.md` files after Edit/Write. Wired via `.claude/settings.json`. (safe-auto)
- Tier A: `commands/pcm.md` § "Pipeline Consistency Audit" — deep-walk fan-out architecture. Spawns one Explore agent per scope in parallel, each reading every file and following every reference. Replaces surface-level existence checks with semantic consistency verification. New `cross-refs` scope catches inter-domain inconsistencies. Severity classification (CRITICAL/WARNING/INFO). (safe-auto)

### Changed

- Docs: `SETUP.md` — manifest format expanded with `interview` field, `installed_from_tag`, `schema` version. Install pins to git tag. "Staying current" section rewritten. `.professor/` directory replaces `.claude/PROFESSOR_*`.
- Docs: `INSTALL.md` — Step 9 rewritten for `.professor/` directory with decisions.md. Added Step 8.3 for format-md.sh hook.
- Docs: `RELEASE.md` — git tag convention, expanded adopter version tracking with manifest + three-way flow.
- Docs: `BLUEPRINT.md` — added "Staying current" section, `.professor/` in file layout, format-md.sh in scripts.
- Docs: `README.md` (blueprint + root) — "Staying current" rewritten for manifest-driven updates. Generic clone paths.
- Docs: All blueprint docs — `~/work/professor` → `/path/to/professor` (no hardcoded local paths).
- Tier A: `commands/pcm.md` § audit scopes — `paths` and `tech` scopes removed (folded into deep checks of `agents`, `commands`, `pipeline`, `structure`). 8 scopes retained, all deepened.

### Migration

#### For `.professor/` directory (replaces `.claude/PROFESSOR_*`)

Adopters on v0.5.0 have `.claude/PROFESSOR_VERSION` and `.claude/PROFESSOR_MANIFEST.json`. First `/pcm update` migrates these into `.professor/VERSION` and `.professor/manifest.json`, creates `.professor/decisions.md`, and removes the old files. The missing `interview` field triggers a one-time re-interview to populate the manifest.

---

## [0.5.0] — 2026-05-13

### Breaking

- Tier A: `commands/professor.md` **deleted** — the Professor is no longer a separate command. Professor IS `CLAUDE.md`. Cross-disciplinary analysis, routing, and verdict format are embedded in the root persona. Migration: move any customizations from `.claude/commands/professor.md` into your root `CLAUDE.md`'s Character section and Cross-Disciplinary System Analysis section.
- Tier A: `commands/ca.md` renamed to `commands/audit.md` — `/ca` is now `/audit`. Cortex audit mode removed (handled by Professor directly). Migration: rename `.claude/commands/ca.md` → `audit.md`, update CLAUDE.md command table.
- Tier A: `commands/jm.md` renamed to `commands/pcm.md` — `/jm` is now `/pcm` (Professor Change Manager). Dr. House character enriched. Migration: rename `.claude/commands/jm.md` → `pcm.md`, update CLAUDE.md command table.

### Added

- Tier A: `CLAUDE.md` template — **Professor identity architecture**. 10+ configurable PhDs with `*Think:*` prompts, Cross-Disciplinary System Analysis section (3 simultaneous lenses), mandatory Verdict format on every response, Context Isolation rule (spawn sub-agents when context accumulates). Professor now routes ALL requests — analytical ones handled directly, others dispatched to commands. (breaking)
- Tier A: `CLAUDE.md` template — **Epics system**. Initiative-level persistent context via `docs/epics/{name}/manifest.md`. Lifecycle tracking (PLANNING → IN_PROGRESS → SHIPPED). Professor creates/loads/updates epics; `/documenter` auto-appends pipeline progress. Cross-conversation context that doesn't fit in memory.
- Tier A: `skills/ghostwriter/SKILL.md` — new writing style capture skill. Analyzes 20+ voice samples to extract mechanical fingerprints (sentence rhythm, punctuation habits, vocabulary tier, paragraph architecture), then generates text in that voice. (safe-auto)
- Tier A: `commands/pcm.md` — enriched Dr. House character. Diagnostic obsession, "everybody lies" verification ethos, sarcastic surgical voice. The Professor's backbone underneath the snark.
- Tier A: `commands/audit.md` — lean two-mode auditor (code hygiene, security) with mandatory reference file loading.

### Changed

- Tier A: `CLAUDE.md` template — complete rewrite (267→370 lines). Professor is now the root identity with warm grandfatherly character, 10 configurable PhDs, cross-disciplinary analysis built in, verdict format, context isolation rule. Routing table clearly separates "Professor handles directly" from "route to commands". (breaking)
- Tier A: `commands/build.md` — `docs/dev/tasks` → `docs/dev/builds`, wave-ownership guard in Step 0a, numbered rolling archive (max 10, 3-digit counter). (safe-auto)
- Tier A: `commands/jc.md` — global renames, added `gh` CLI access, CI/CD fix mode. (safe-auto)
- Tier A: `commands/wave.md` — `docs/dev/tasks` → `docs/dev/builds`, manifest copy step, execution plan display, proactive status emission, Professor review before archive, numbered rolling archive. (safe-auto)
- Tier A: `commands/dev.md` — canonical report template, auto-heal escalation with loop prevention, new service detection, detailed mode specs for all 9 modes. (safe-auto)
- Tier A: `commands/documenter.md` — `/jm` → `/pcm`, `docs/dev/tasks` → `docs/dev/builds`, wave-ownership guard, numbered rolling archive. (safe-auto)
- Tier A: `commands/council.md` — Professor voice source changed from `commands/professor.md` to `CLAUDE.md (root)`. (safe-auto)
- Tier B: `commands/pm.md` — added wave-post-review mode, 360 sweep in pre-flight. (safe-auto)
- Tier B: `commands/km.md` — added Step 4.5 Compliance Review Loop. (safe-auto)
- Tier B: `commands/mentor.md` — added Ghostwriter section for external-facing deliverables. (safe-auto)
- Tier B: `commands/marketer.md` — added Ghostwriter section for human voice, profile selection guide. (safe-auto)
- Mechanics: `agents/gitter.md` — compressed 559→375 lines. Same behavior, fewer tokens. (safe-auto)
- Mechanics: `agents/mono-architect.md` — minor wording updates. (safe-auto)
- Mechanics: `agents/mono-documenter.md` — minor wording updates. (safe-auto)
- Mechanics: `skills/360/SKILL.md` — updated for sub-agent delegation pattern. (safe-auto)
- Mechanics: `skills/rr/SKILL.md` — minor updates. (safe-auto)
- Mechanics: `skills/rnd/SKILL.md` — minor updates. (safe-auto)
- Mechanics: Codex templates — global renames, updated cross-references. (safe-auto)
- Docs: `README.md` — complete rewrite reflecting Professor identity architecture. (safe-auto)
- Docs: `BLUEPRINT.md`, `SETUP.md`, `ARCHETYPES.md`, `RELEASE.md`, `INSTALL.md` — global renames (/jm→/pcm, /ca→/audit, Jungche→Professor), Epics system added. (safe-auto)

### Removed

- `commands/professor.md` — Professor merged into CLAUDE.md. No separate command.
- `commands/ca.md` — replaced by `audit.md`.
- `commands/jm.md` — replaced by `pcm.md`.
- Cortex audit mode from `/audit` — now handled by Professor directly via `$CDOCS/professor/$REFS/cortex-audit.md`.

### Migration

**This is a major release.** Adopters should run `/pcm update` which will walk through each breaking change interactively. Key manual steps:

1. **Professor identity:** Your `CLAUDE.md` needs the new Cross-Disciplinary System Analysis section and Verdict format. `/pcm update` will show the diff and let you merge.
2. **Command renames:** Rename `ca.md` → `audit.md`, `jm.md` → `pcm.md`. Delete `professor.md`. Update your CLAUDE.md command table.
3. **Cortex audit:** If you used `/ca cortex`, that mode is now accessed by asking the Professor directly (who loads `$CDOCS/professor/$REFS/cortex-audit.md`).
4. **Epics:** New feature — add the Epics section to your CLAUDE.md if you want cross-conversation context persistence.
5. **Ghostwriter:** New skill — copy `skills/ghostwriter/SKILL.md` to `.claude/skills/ghostwriter/SKILL.md` if desired.

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

No adopter-side migration needed. All changes are structural density improvements — same behavior, fewer tokens. `/pcm update` applies them without prompts.

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

No adopter-side migration needed. All changes are `safe-auto` — `/pcm update` applies them without prompts.
