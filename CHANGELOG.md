# Changelog

All notable changes to the Professor blueprint will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**For adopters:** run `/pcm update` in your installed project to apply changes between your local version and the latest release. The update command parses this file to walk you through changes interactively. Each release's full notes live in [`releases/`](releases/) — one file per version, so you read one release at a time.

---

## How `/pcm update` reads this file

Each release file (`releases/vX.Y.Z.md`) uses categorized headings the update flow understands:

| Heading        | Apply how                                                                       |
| -------------- | ------------------------------------------------------------------------------- |
| `## Added`     | Auto-apply mechanics changes; ask before adding Tier B archetypes               |
| `## Changed`   | Auto-apply mechanics; show diff + ask for character changes                     |
| `## Fixed`     | Auto-apply (bug fixes don't touch customization)                                |
| `## Removed`   | Walk through interactively — never auto-delete                                  |
| `## Breaking`  | **Interactive walkthrough required.** Each change has explicit migration steps. |
| `## Migration` | Step-by-step transformation instructions for adopters                           |

Bullets MUST follow this shape:

```
- {Tier A|Tier B|Mechanics|Docs|Scripts}: {file path or scope} — {what changed semantically}
```

Optional trailing tags: `(opt-in)` for Tier B additions, `(breaking)` if it requires migration even outside a Breaking section, `(safe-auto)` to mark unconditional auto-apply.

---

## [Unreleased]

---

## Releases

Reverse-chronological. Click a version to read its full notes.

- [v0.24.0](releases/v0.24.0.md) — token-optimization campaign: persona → output styles, Analysis Protocol folded into persona (`p:analysis` removed), `/audit` removed, routing-gated `/build` + common spawn contract + consolidated 6-bugs, delta STATE.md waves, gitter/build/refine trims, QA promoted to registered hook-carrier wrappers with test-output filter, rr collection/judgment split, prompt-law rails.
- [v0.23.0](releases/v0.23.0.md) — `/slow-burn`: session-limit pacing command — checkpointed rounds, cache-aware naps/hibernations, intensity dial 0–10, cross-session resume.
- [v0.22.0](releases/v0.22.0.md) — Registry-over-tables completes: all 14 command templates gain routing frontmatter and `/pcm` audits the registry, not tables; `templates/docs-agents/` scaffold + SETUP Phase 2.7 end dangling hub references; standalone `sources.json` skills release from their own repos (release 5b / update 8b); `p:refine` R4 approves the spec, not `/wave` execution.
- [v0.21.0](releases/v0.21.0.md) — `notify.sh` turn-done banner gains project/session/last-prompt/duration context, an osascript-first engine, and a per-session-id stamp race fix; `CLAUDE.md` adds a decision-integrity rule and drops the routing table for frontmatter routing.
- [v0.20.0](releases/v0.20.0.md) — `p:refine` carries RND-validated prompts into waves verbatim (byte-identical, with the reason each works) for any RND-sourced wave.
- [v0.19.0](releases/v0.19.0.md) — Archive flow rework (git history + tmp, no `docs/` archives), AskUserQuestion refinement gates, `/blueprint` becomes the `p:blueprint` skill (breaking), two-tier model policy, new `/save` context-dump command, `notify.sh` origin context.
- [v0.18.0](releases/v0.18.0.md) — Templates generalized to a **roster model** (1..N projects, single-project first-class) honoring the "any repo/any stack" promise; release notes split into per-version files; README/BLUEPRINT drift fixed.
- [v0.17.0](releases/v0.17.0.md) — Professor-native skills consolidated into the `p:*` namespace; `/pcm` gains a two-ledger system (`drift.md` + `release.md`) and a rebase-first, ask-to-sync Update Protocol.
- [v0.16.1](releases/v0.16.1.md) — SETUP §7h links the theme repo's "match your terminal background" guide.
- [v0.16.0](releases/v0.16.0.md) — Claude Code themes are now source-fetched at install via `themes/sources.json`; first theme `tokyo-night`.
- [v0.15.0](releases/v0.15.0.md) — `/blueprint` is now self-hosting (ships as a Tier A template); whole published output scrubbed of project identity.
- [v0.14.0](releases/v0.14.0.md) — `/p:refine poc` Refine-to-Prototype subcommand; SETUP §7g host-tooling probe (git-host bridge).
- [v0.13.0](releases/v0.13.0.md) — Full high-fidelity re-mine after 250 upstream commits; universal skills moved to source-fetch; Codex layer kept.
- [v0.12.0](releases/v0.12.0.md) — Claude memory auto-backup via a `SessionEnd` hook syncing to a private git repo (opt-in).
- [v0.11.0](releases/v0.11.0.md) — `/pcm` logs every infrastructure change to `.professor/decisions.md`, not only on `/pcm update`.
- [v0.10.0](releases/v0.10.0.md) — `/p:refine` R1→R4 ZERO-GAP protocol; `/build` pre-merge Code Review gate; `/wave` post-review auto-remediation.
- [v0.9.3](releases/v0.9.3.md) — Uncommitted-main handling: `/wave` may start dirty, gitter stashes/restores WIP around merge; documenter ARCHIVE rewrites entries.
- [v0.9.2](releases/v0.9.2.md) — `worktree.sh prune` hardened to match registered worktrees by basename, preventing data loss at non-canonical install paths.
- [v0.9.1](releases/v0.9.1.md) — New `worktree.sh prune` subcommand reclaims orphaned `.worktrees/{name}` dirs; `/build` pre-flight runs it.
- [v0.9.0](releases/v0.9.0.md) — Epic-update automation across the pipeline; Carry-WIP commit-and-carry of uncommitted `main` into the worktree.
- [v0.8.0](releases/v0.8.0.md) — `prompt-quality` rubric and `vision-factory` skills added; epic manifest template; skill rename `p:analyze` → `p:analysis`.
- [v0.7.0](releases/v0.7.0.md) — Reference files replaced by standalone skills (`p:analyze`, `p:refine`, `p:wave-review`, `audit:*`) with Codex wrappers and SETUP hydration.
- [v0.6.2](releases/v0.6.2.md) — `notify.sh` macOS notifications for long turns; gitter/git remote-publication boundary and push hard gate.
- [v0.6.1](releases/v0.6.1.md) — Shared `360`/`rr`/`rnd`/`ghostwriter` Codex skill wrappers; Codex agents framed as Claude peers; research-contract validator.
- [v0.6.0](releases/v0.6.0.md) — Full `/pcm update` implementation (manifest-driven replay, git-tag pinning, three-way diff); `.professor/` directory; `format-md.sh` hook.
- [v0.5.0](releases/v0.5.0.md) — Professor identity merged into `CLAUDE.md`; Epics system; `/ca`→`/audit`, `/jm`→`/pcm`, `professor.md` deleted; ghostwriter skill.
- [v0.4.0](releases/v0.4.0.md) — `360` exhaustive multi-angle analysis skill (test + inquiry domains); QA/Professor 360° sweeps; `rr` drops intermediate files.
- [v0.1.2](releases/v0.1.2.md) — Token-trimming pass: ten command/agent templates condensed to denser form, same behavior.
- [v0.1.1](releases/v0.1.1.md) — Merge-lock protocol removed across gitter/build/jc/git/wave; `rr`/`rnd` skills added; `/ckm`→`/km` rename.
