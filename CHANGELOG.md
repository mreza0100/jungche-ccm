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

- [v0.31.1](releases/v0.31.1.md) — chat:inject — a message that is itself a slash command lands verbatim (no footer, no file-cap) so the target runs it.
- [v0.31.0](releases/v0.31.0.md) — chat: refinements: injected messages auto-signed with a runnable reply command + target-first /chat:inject syntax; /chat:load now write-free (load-check removed); subcommand who-i-am → whoami.
- [v0.30.0](releases/v0.30.0.md) — chat: family consolidated to one `chat.sh` engine, injection-only delivery (/chat:send + inbox removed; added /chat:dump, /chat:ls, /chat:whoami, /chat:load; /chat:save now a mechanical copy); MIGRATION: /chat:send → /chat:inject.
- [v0.29.0](releases/v0.29.0.md) — /chat command family (find/read/send/inject/capture + mail bus & live-tmux inject), documenter ARCHIVE split with bang-cat shared fragments, goal-definer → goal-manager with epic continuation prompts, gitter scoped-commit discipline, statusline effort/limit/ultracode upgrades; MIGRATION: /chat:load → /chat:read, /goal-definer → /goal-manager.
- [v0.28.0](releases/v0.28.0.md) — /save → /chat:save (script-dumped transcripts, 5-section header) + new /chat:load (resume a past chat from a pasted excerpt) and /goal-definer (compile a super-goal into a fresh-session prompt); MIGRATION: /save renamed, three new scripts.
- [v0.27.1](releases/v0.27.1.md) — /save loses its epic redirect; epic saves live solely in /documenter epic.
- [v0.27.0](releases/v0.27.0.md) — epic system redesign: current-state epics (update.md work doc, one-line milestones, Files index, archive/) + /documenter epic session-save mode + Epic consolidation contract; MIGRATION: downstream installs refactor their existing epics to the new design.
- [v0.26.0](releases/v0.26.0.md) — rr v1.3.0: committed judge-steered research engine (research → judge → research, pressure-test saturation, tiered models) released in its canonical repo; skill-embedded workflow-engine pattern recorded in /pcm wiring.
- [v0.25.0](releases/v0.25.0.md) — wave-as-workflow execution engine (saved workflow + workflow.json + forensics hardening), new /animate command (educational flow animations, dual accuracy gates), p:wave skill namespace renames, docs-commands install fix, opt-in host multi-account swap.
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
