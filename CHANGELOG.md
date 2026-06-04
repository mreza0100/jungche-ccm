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

## [0.13.0] — 2026-06-04

Full re-mine from the live source after 250 upstream commits. Every command, skill, agent, and script template was regenerated at **high fidelity** — verbatim source with only customized values swapped for placeholders (governed by the new `PLACEHOLDERS.md`), replacing the old abstracted skeletons. The Codex layer is **kept** (blueprint stays dual-runtime) and synced to the new roster; universal skills moved to **source-fetch**.

### Added

- Mechanics: `templates/skills/sources.json` + `SETUP.md` §7a — **skills-from-source**. `rr`, `360`, `ghostwriter`, and `vision-factory` are no longer vendored in the blueprint; SETUP clones each from its canonical public repo at install, so they are always latest and can never drift. (safe-auto)
- Tier A: `skills/quality:doc/` — reference-doc authoring standard (≤500-line topic target, grep-true naming, 8-check Approval gate); successor to the removed `doc-format` skill. (safe-auto)
- Mechanics: `blueprint/PLACEHOLDERS.md` — canonical placeholder map governing every template's regeneration. (safe-auto)
- Mechanics: `templates/vscode/` · `INSTALL.md` · `SETUP.md` · `BLUEPRINT.md` · `README.md` — VSCode tmux launcher. New `vscode/` template dir ships `terminal-profile.json`, `zshrc-cc.snippet.sh`, `tmux.conf`, and a `README.md`; new VSCode terminals open straight into `tmux + cc` (Claude Code inside a tmux session), and on `/exit` the tmux session ends and the terminal falls back to a normal shell — it never closes on you. The `cc` shell function is `typeset -f`-guarded so it never clobbers an existing `cc`. Includes mouse scroll + click-to-copy tmux comfort defaults (macOS `pbcopy`; swap for `xclip`/`wl-copy`/`clip.exe` on Linux/Windows). (opt-in)

### Changed

- Tier A (all command/skill/agent/script templates): re-mined at high fidelity, absorbing 250 commits of drift — doc-cluster maturation (`_index.md` clusters, `quality:doc`, `backlog.md`, all-project flow diagrams), model re-tiering (post-merge QA + AI architect/engineer → opus, gitter → sonnet), LLM-provider abstraction (`{LLM_PROVIDER}`), `p:wave-review` 2.0 fan-out thread-walk, `rr` 1.2 Workflow pipeline, `p:refine` Tier-1 scope-boundary gate, atomic `wave.md` clear. (safe-auto; character changes show diff)
- Tier A (root `CLAUDE.md`): Verdict tightened (≤25 words, only-sanctioned-trailing-line, anti-recap), new "Show gaps as Expected vs Got" rule, "Cleverly funny" trait, docs-map cluster model; blueprint-only scaffolding sections dropped to mirror source. (show diff)
- Mechanics (Codex layer): kept (blueprint stays dual-runtime) and synced to the new roster — `format-md.sh` keeps `AGENTS.md`; the six codex-touched files retain their Codex content while adopting all other upstream improvements. (safe-auto)

### Removed

- Tier A: `/council` (roundtable debate) and `/reddit` commands + their Codex wrappers — retired upstream. (interactive)
- Mechanics: `scripts/check-codex-research-contract.sh` — retired; research-contract parity is now a grep check inside `/pcm`. (interactive)
- Mechanics: vendored copies of `rr`/`360`/`ghostwriter`/`vision-factory` — replaced by source-fetch (see Added). (safe-auto)

### Migration

#### For: all adopters

- **Skill rename** `prompt-quality` → `quality:prompt`: rename your `.claude/skills/prompt-quality/` directory and update references. `/pcm update` walks this.
- **Skills-from-source**: `rr`/`360`/`ghostwriter`/`vision-factory` now install from their public repos via `sources.json`. Re-run skill install (or `/pcm update`) to switch from a vendored copy to the source-fetched one.

#### For: adopters who customized `/council` or `/reddit`

Removed upstream. Keep your local copy if you still use it, or retire it — `/pcm update` will not delete it automatically.

#### For: the VSCode tmux launcher

Opt-in only — nothing changes unless you enable it. Follow `SETUP.md` Phase 5 (merge `terminal-profile.json` into VSCode user `settings.json`, append `zshrc-cc.snippet.sh` to your shell rc, copy `tmux.conf` to `~/.tmux.conf`). Touches your **global** VSCode settings and shell rc. **`/pcm update`: ask before installing — opt-in capability.**

## [0.12.0] — 2026-05-27

### Added

- Scripts: `scripts/memory-sync.sh` · `references/memory-backup.md` · `SETUP.md` (Phase 4) — Claude memory auto-backup. A `SessionEnd` hook syncs Claude Code's persistent project memory (`~/.claude/projects/<key>/memory`) to a private git repo via a symlink + plain-git push script (self-healing on a cut-off push, headless `gh` auth, zero tokens). New optional install phase explains it and asks to opt in; full architecture, 12 tips/pitfalls, and new-machine restore in the reference doc. (opt-in)

### Migration

#### For: all adopters

Opt-in only — nothing changes unless you enable it. To turn it on, follow `SETUP.md` Phase 4 (create a private memory vault, symlink the live memory dir, install the `SessionEnd` hook). Adopters who don't opt in are unaffected. **`/pcm update`: ask before installing — opt-in capability.**

## [0.11.0] — 2026-05-27

### Changed

- Tier A (`commands/pcm.md`): `/pcm` now logs every infrastructure change to `.professor/decisions.md` under "Post-install customizations", not only on `/pcm update` — keeps the human-readable institutional memory current on every pipeline edit. (safe-auto)
- Docs (`INSTALL.md`): decisions.md header and placeholder now state that every `/pcm` change appends, not only `/pcm update`. (safe-auto)

## [0.10.0] — 2026-05-24

### Added

- Tier A (`skills/p:refine/SKILL.md`): new R1→R4 ZERO-GAP protocol — the refined task file becomes a COMPLETE technical spec (routing, data model, contracts, file plan, signatures, mermaid technical-flow diagram per task) that delegates no decision to `/wave` or `/build`; the founder approves a visual + summary at a new R4 gate. (safe-auto)
- Tier A (`commands/build.md`): pre-merge Code Review gate — a code-hygiene audit on the pipeline diff → architect → developer fix loop (cap 2) runs before merge; added fixed-format stdout Status Emission (header, per-phase lines, footer). Pipeline step table renumbered (post-merge audit dropped; documenter → Step 10, commit-docs → Step 11). (safe-auto)
- Tier A (`commands/wave.md`): auto-remediation of post-wave review findings via the hotfix command (new Step 3.4); structured status + per-build index token (`[Build: {n}/{total}]`) + running tally; routing now read from the refined spec, while grouping/ordering/parallelism stay wave-owned. (safe-auto)

### Changed

- Tier C (`agents/mono-architect.md`): honors the ZERO-GAP spec — transcribe/validate the decided design into `3-architecture.md`, never re-design, re-route, or re-scope. (safe-auto)
- Tier B (officer): the compliance archetype is now advisory-only — invoked as a separate clean agent at refinement (`p:refine` R2.6), removed from the build pipeline and the wave. **`/pcm update`: skip — informational only.**

### Migration

#### For: adopters who customized `/p:refine`

Re-read `p:refine`. The task-file per-task format gained `**Routing:**`, `**Data model:**`, `**Contracts:**`, `**File plan:**`, and `**Technical flow:**` (mermaid) sections, plus the R4 founder-approval gate (wave-level mermaid + decision summary). The skill now runs R1→R4 (was R1-R3.5).

#### For: adopters who customized `/build`

Adopt the new pre-merge Code Review phase (`audit:code-hygiene` on the diff → architect → developer, cap 2, residual logged not blocked) and the § Status Emission section (header at end of Step 0, per-phase lines, footer after the final step). The pipeline step table is renumbered: the post-merge audit step is gone; documenter is Step 10 and commit-docs is Step 11.

#### For: adopters who customized `/wave`

Adopt Step 3.4 (auto-remediate the reviewer's `## /jc Action Items` via the hotfix command, append a `## Review Remediation` table); pass the `[Build: {n}/{total}]` token to every `/build` and emit the running tally after each build; read each task's `**Routing:**` from the refined spec in Step 0b instead of re-classifying.

## [0.9.3] — 2026-05-23

### Changed

- Tier A (`/wave`): launch — a wave may start with an uncommitted main (default leave, no prompt); WIP stays on main, excluded from pipelines. (safe-auto)
- Tier A (gitter): MERGE — stashes main's uncommitted WIP around the branch merge and restores it; pauses only on a WIP stash-pop conflict that must be committed first. (safe-auto)
- Tier A (`/documenter`): registry inlined into the command; ARCHIVE now deletes/rewrites superseded entries, not only appends. (safe-auto)
- Tier C (per-project planner/architect): read the project's architecture docs before planning/designing. (safe-auto)
- Tier A (CLAUDE.md): docs-map signpost directs every agent to the documentation hub. (safe-auto)

## [0.9.2] — 2026-05-21

### Fixed

- Scripts: `scripts/worktree.sh` — Harden `prune` worktree detection. Registered worktrees are matched by **basename** rather than full path, so a symlinked or canonicalized repo path can no longer misclassify a live worktree as orphaned and `rm -rf` it; added a fail-safe that prunes nothing if git can't list worktrees. Hardens the v0.9.1 prune against data loss at non-canonical install paths. (safe-auto)

## [0.9.1] — 2026-05-21

### Fixed

- Scripts: `scripts/worktree.sh` · `commands/build.md` — Orphaned-worktree prune. New `worktree.sh prune` subcommand reclaims `.worktrees/{name}` directories left by failed or abandoned pipelines (not a registered git worktree, no active pipeline docs); `/build` pre-flight (Step 0a) now runs it — making the worktree→docs sweep symmetric with the existing docs→worktree one. Registered-but-inactive worktrees are reported for inspection, never auto-removed (they may hold uncommitted work). (safe-auto)

## [0.9.0] — 2026-05-21

### Added

- Tier A: `commands/build.md` · `commands/wave.md` · `commands/documenter.md` · `skills/p:refine/SKILL.md` · `CLAUDE.md` — Epic-update automation. `/p:refine` stamps the target epic in `wave.md`; when work ships for an active epic, the pipeline auto-writes `docs/epics/{name}/update.md` and appends Progress Log + Key Decisions + `pipelines`/`waves` to the manifest. One writer per scope — `/documenter` for a standalone `/build`, `/wave` for a wave; the Professor still owns manifest creation, Vision/Scope, Open Questions, and `status`. (safe-auto)
- Tier A: `agents/gitter.md` · `commands/build.md` · `commands/wave.md` · `codex/agents/build.toml` — Carry-WIP. At pre-flight, if `main` has uncommitted work, the pipeline offers to commit-and-carry it into the worktree (commit-then-branch → the WIP becomes a shared ancestor → the merge cannot conflict over it, and nothing is lost). Default leaves the WIP on `main`. (safe-auto)

### Migration

#### For: adopters who customized `/build`, `/wave`, `/documenter`, or `/p:refine`

Re-merge the epic-update hooks: the `[Epic:]` arg threaded `/wave` → `/build` → `/documenter`; the documenter's ARCHIVE epic step now writes `docs/epics/{name}/update.md` + appends the manifest; the new `/wave` epic-update step (after the post-wave review); and the `**Epic:**` stamp in the `/p:refine` wave.md format.

#### For: adopters who customized `gitter` SETUP

Adopt the `CarryWIP` precondition in Phase 1 (commit-then-branch on `commit`, stash-and-restore on `leave`) and the new `[CarryWIP:]` argument surfaced at `/build` and `/wave` pre-flight.

## [0.8.0] — 2026-05-21

### Added

- Tier A: `.claude/skills/prompt-quality/` — Anthropic prompt-discipline rubric (cut test, line/size thresholds, anti-patterns, per-file-type structural conventions); loaded before any prompt-file edit. (safe-auto)
- Tier B: `.claude/skills/vision-factory/` — Paul Graham–grounded startup-vision forge (CREATE / RESEARCH / STRESS-TEST modes). (opt-in)
- Mechanics: `templates/epics/TEMPLATE.md` — epic manifest template; the blueprint previously shipped no `epics/` template dir. (safe-auto)

### Changed

- Tier A: `commands/pcm.md` — now mandates loading `prompt-quality` before editing CLAUDE.md, agents, commands, or skills.
- Tier B: `commands/km.md` — now loads `prompt-quality` before editing knowledge files.
- Tier A: `templates/CLAUDE.md` — trimmed for token efficiency: dropped the duplicate Commands table (Request Routing is canonical), replaced the Skills table with a one-line pointer, moved the epic manifest format to `epics/TEMPLATE.md`, removed the redundant Communication Standard section and Voice examples bank.

### Breaking

- Tier A: skill `p:analyze` → `p:analysis` (Claude + Codex), matching upstream. (breaking)

#### For adopters: on ≤ v0.7.0

Rename `.claude/skills/p:analyze/` → `.claude/skills/p:analysis/` and the Codex wrapper `.codex/skills/p:analyze/` → `.codex/skills/p:analysis/`. Update the `name:` frontmatter in the renamed `SKILL.md` and any references in `CLAUDE.md` and `commands/council.md`. `/pcm update` applies this automatically via the three-way diff.

## [0.7.0] — 2026-05-15

### Breaking

- Tier A: Reference files (`$CDOCS/professor/$REFS/`, `$CDOCS/audit/$REFS/`) replaced by standalone skills. Protocols now live in `.claude/skills/{name}/SKILL.md` instead of `docs/commands/{cmd}/references/`. Migration: move protocol content from reference files into skills; delete reference files; update all `$CDOCS/$REFS` pointers to skill invocations. (breaking)

#### For adopters:

1. Create `.claude/skills/` directories for: `p:analyze`, `p:refine`, `p:wave-review`, `audit:code-hygiene`, `audit:security` (and `audit:cortex` if your project has an AI/ML pipeline)
2. Move protocol content from reference files into the new SKILL.md files
3. Delete the original reference files and empty `references/` directories
4. Update CLAUDE.md protocol table, routing table, and skills table
5. Create Codex symlinks: `ln -s ../../../.claude/skills/{name}/SKILL.md .codex/skills/{name}/SKILL.md`

### Added

- Tier A: `skills/p:analyze/SKILL.md` — cross-disciplinary analysis protocol as standalone skill. Domain-hydrated: CS lens ships universal, domain + compliance lenses filled by RR at install time. (breaking)
- Tier A: `skills/p:refine/SKILL.md` — wave task refinement protocol (R1-R3.5) as standalone universal skill. Ships as-is. (safe-auto)
- Tier A: `skills/p:wave-review/SKILL.md` — post-wave operational review protocol (W1-W3) as standalone universal skill. Ships as-is. (safe-auto)
- Tier A: `skills/audit:code-hygiene/SKILL.md` — 7-category code hygiene audit as standalone skill. Domain-hydrated: category structure ships universal, detection patterns filled by RR at install time. (breaking)
- Tier A: `skills/audit:security/SKILL.md` — 9-subcategory (8A-8I) security deep scan as standalone skill. Domain-hydrated: OWASP structure ships universal, detection patterns filled by RR at install time. (breaking)
- Mechanics: `codex/skills/{p:analyze,p:refine,p:wave-review,audit:cortex,audit:code-hygiene,audit:security}/SKILL.md` — Codex wrappers for all 6 new skills. (safe-auto)
- Docs: `SETUP.md` Phase 2.5 — Skill Knowledge Hydration protocol. Domain-hydrated skills (p:analyze, audit:code-hygiene, audit:security) get their knowledge bases filled via RR at install time. Empty skills loop with "knowledge base is empty" message until user provides specification. (breaking)

### Changed

- Tier A: `CLAUDE.md` template — protocol table now references skills (`/p:analyze`, `/p:refine`, etc.) instead of reference file paths. Routing table adds 4 new skill rows. Skills table adds 6 new entries with `(domain-hydrated)` markers. (breaking)
- Tier A: `commands/audit.md` — reference file loading replaced with mandatory skill invocation (`/audit:code-hygiene`, `/audit:security`). Cortex audit routes to `/audit:cortex` skill. (breaking)
- Tier A: `commands/wave.md` — Professor review invokes `Skill("p:wave-review", ...)` instead of `Skill("professor", "wave-review ...")`. (safe-auto)
- Tier A: `commands/council.md` — Professor seat references `/p:analyze` skill instead of `$CDOCS/professor/$REFS/analysis.md`. (safe-auto)
- Tier A: `commands/blueprint.md` (Freudche-only) — skills split into `universal/` and `domain-hydrated/` in output structure. Phase 2.5 Skill Knowledge Hydration added. Codex symlink instructions explicit. (safe-auto)

### Removed

- Tier A: `docs/commands/professor/references/` — analysis.md, cortex-audit.md, refinement.md, wave-review.md moved to standalone skills.
- Tier A: `docs/commands/audit/references/` — code-hygiene.md, security.md moved to standalone skills.

### Migration

#### For reference-to-skill migration

The core change: protocols that lived in `docs/commands/{cmd}/references/` are now standalone skills in `.claude/skills/`. This is a structural move — the protocol content is identical, but the invocation pattern changes from "read this reference file" to "invoke this skill."

**Two skill tiers:**

| Tier                | Skills                                        | Ships                                                | Install action                 |
| ------------------- | --------------------------------------------- | ---------------------------------------------------- | ------------------------------ |
| **Universal**       | p:refine, p:wave-review                       | Full protocol, ready to use                          | Copy as-is                     |
| **Domain-hydrated** | p:analyze, audit:code-hygiene, audit:security | Universal structure + `KNOWLEDGE BASE EMPTY` markers | Run RR to fill knowledge bases |

`audit:cortex` has no template — it's entirely project-specific and only created if the project has an AI/ML pipeline subproject.

**`/pcm update` handles:** creating skill directories, copying universal skills, placing domain-hydrated shells, creating Codex symlinks, updating CLAUDE.md references. Domain-hydrated skills require user-driven RR hydration after install.

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
