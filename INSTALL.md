# INSTALL — Interactive setup for Professor

> This document is **instructions for Claude Code**, not for you to execute manually. To install Professor on your project, open Claude Code in that project and paste:
>
> ```
> Read https://raw.githubusercontent.com/mreza0100/professor/main/INSTALL.md (or your local clone) and walk me through the interactive install. Ask me each section's questions one at a time and wait for my answers before proceeding. Do not assume — confirm everything.
> ```

If you (Claude) are reading this: **you are the installer**. The user's input is the source of truth — never invent stack details, names, or domain assumptions. Ask the questions in this file in batches, wait for replies, then generate.

A blueprint pasted blindly is a museum exhibit; a blueprint shaped to the user's stack is a working pipeline. This installer's job: **ask, don't assume**.

---

## Prerequisites (verify before the interview)

- **Claude Code CLI**, logged in — you're running in it now.
- **A git repository.** If the project isn't one, ask whether to `git init` first (recommended — it gives `git mv` history preservation for the doc re-homing step). Never `git init` silently.
- **`jq`** — required by the host installer and several hooks. If missing, warn and point at `brew install jq` / `apt install jq`.
- Optional, per opt-in: `prettier` via `npx` (markdown auto-format), `tmux` (VSCode launcher, host fleet), `node` (Codex mirror compiler), `gh` or `glab` (git-host skill).
- **10–15 minutes of the user's attention** for the interview.

## What this install will never do

State this to the user up front:

- **Never commits or pushes.** The installer only writes files; `git status` shows the plan, committing is the user's call.
- **Never overwrites without asking.** An existing `CLAUDE.md` or `.claude/` stops the install until the user chooses overwrite / merge / abort.
- **Never installs opt-in pieces silently.** Tier B roles, Codex, statusline, hooks, VSCode launcher, host fleet, themes, memory backup — each lands only on an explicit yes.
- **Never touches paths outside the plan:** root `CLAUDE.md`, `.claude/`, `.professor/`, `docs/`, per-project `CLAUDE.md` + `.claude/`, plus — only for approved host-level opt-ins — specific files under `~/.claude/`.

---

## Where the full spec lives

This file is the front door: prerequisites, the interview, the confirmation gate, and verification. The **generation spec** — every template, placeholder, and write step — is [`blueprint/SETUP.md`](./blueprint/SETUP.md) (Phase 2), with [`blueprint/PLACEHOLDERS.md`](./blueprint/PLACEHOLDERS.md) as the substitution law and [`blueprint/BLUEPRINT.md`](./blueprint/BLUEPRINT.md) as the philosophy. Read all of them before writing any file.

### Corrections to SETUP.md — this file wins until a release folds them in

SETUP.md is the richest spec but carries a few stale lines. Where the two disagree, **this list is authoritative**:

1. **Smoke test:** `/wave:builder add-readme-section` no longer works — `/wave:builder` is orchestrated-only and refuses anything but a brief path. Verify the install with `/dev status` and a tiny `/jc` task instead (see § Verify below).
2. **`AGENTS.md` is compiled, not symlinked.** If Codex is opted in, `scripts/build-codex.mjs` generates it from `CLAUDE.md`; never `ln -sf CLAUDE.md AGENTS.md`.
3. **`codex-mirror.sh` does not exist.** The Codex layer is `scripts/build-codex.mjs` (compiler) + `scripts/codex-sync.sh` (hooks); any SETUP.md reference to `codex-mirror.sh` reads as those two.
4. **Skip the Council interview question.** No `/council` command ships; the answer configures nothing. Omit `council_panel` from the manifest.
5. **Clone at the latest tag**, not the hardcoded example tag in SETUP.md's clone command (`git ls-remote --tags` tells you the newest `v*`).

---

## Pre-flight (silent — before asking anything)

1. `git rev-parse --is-inside-work-tree` — confirm a git repo (else see Prerequisites).
2. Confirm `tmp/` and `.worktrees/` are absent or gitignored; flag if present and tracked.
3. Detect existing `CLAUDE.md` / `.claude/` — if present, **STOP** and ask: overwrite, merge, or abort.
4. `git status` — warn on uncommitted work; don't proceed without acknowledgment.
5. Detect package managers (`pnpm-lock.yaml`, `package-lock.json`, `yarn.lock`, `uv.lock`, `Cargo.toml`, `go.mod`, `pyproject.toml`, `requirements.txt`) and monorepo hints (multiple top-level dirs with their own manifests).
6. Surface existing documentation:
   ```bash
   find . -maxdepth 2 -type f -name "*.md" \
     ! -name "README.md" ! -name "LICENSE*" ! -name "CHANGELOG*" ! -name "CONTRIBUTING*" \
     ! -path "./node_modules/*" ! -path "./.git/*" ! -path "./.claude/*"
   ```
   These need re-homing into the Professor taxonomy — but **do not classify yet**; classification depends on the Tier B opt-ins from Batch 5.

Report findings in one short paragraph before the first question, e.g.:

> "I see a Node + Python monorepo with `api/` (pnpm) and `worker/` (uv). No existing `.claude/`. Working tree is clean. I also see 17 root-level markdown files (THESIS, COMPETITOR_LANDSCAPE, …) — those get re-homed once we settle your optional roles. Ready?"

---

## The interview

Ask in batches, numbered as below. Wait for the user's reply between batches — nobody answers 20 questions in one message. If the user reverses an earlier answer, update silently and re-ask only what that invalidates.

### Batch 1 — Project identity

1. Project name? (used in `CLAUDE.md`, commands, banner text)
2. One sentence — what does this project DO? (becomes the pitch in `CLAUDE.md`)
3. Mission / north star — what does success look like, in one line?

### Batch 2 — The roster

Structure is a **roster of 1..N projects**. One project is valid and first-class — not a degraded mode.

4. How many projects does this repo hold? For **each**: directory, role label (backend / frontend / worker / …), tech stack, package manager, test runner, dev port(s), and whether it owns shared infra (Docker, DB).
5. Cross-project communication boundaries, if any? (REST / GraphQL / gRPC / queue / shared types / shared DB — this shapes the cross-project architect's job.)
6. Any **specialists** between architect and QA? (`ui-ux`, `db-admin`, `devops`, `ai-engineer` — optional, per project.)

### Batch 3 — Commands (one row per roster entry)

7. For each project: test, lint, typecheck, build, dev, and install commands. `"skip"` is a valid answer for any of them.

### Batch 4 — Ports

8. Which ports does `main` use for dev today? (Worktree ranges allocate from BASE+1 — backend 3000 means worktrees get 3001–3099.)
9. Any ports to avoid?

### Batch 5 — Domain, disciplines, optional roles

10. The project's domain, one phrase. ("B2B SaaS for legal firms", "consumer mobile game", "clinical AI assistant", …)
11. The Professor pairs computer science with **your** domain. Which disciplines should the doctorates cover? Pick 1–3 or name your own: Psychology · Medicine · Finance · Law · Game design · Education · Linguistics · Cryptography · Distributed systems · Operations research · Bioinformatics · Music/audio · Other.
12. Which two disciplines, combined, are the **intersection lens** — the Professor's unique superpower for this project?
13. What FAILURE modes should the Professor specifically watch for? ("data leakage between tenants", "race conditions during checkout", …) These become scored dimensions.
14. Tier B roles — opt in per role, each with a short parameter follow-up:
    - `/officer` — compliance (which regulation? GDPR, HIPAA, SOC2, …)
    - `/km` — knowledge curation (which domain? which taxonomy?)
    - `/pm` — product (which user persona?)
    - `/mentor` — business (which market + jurisdiction?)
    - `/marketer` — visibility (which channels + language?)
    - or your own: purpose, scope, owned doc paths under `$CDOCS`.
15. **Codex dual-runtime?** (optional — everything works without it). Yes creates `.codex/` plus a compiled `AGENTS.md` per correction 2; no skips the layer entirely.

### Batch 6 — Character (MANDATORY — cannot be skipped)

16. The orchestrator persona is the Professor by default: grandfatherly, warm, precise, cross-disciplinary. Choose:
    - **"keep Professor"** — name and voice as-is; only the domain references adapt.
    - **"rename"** — keep the voice, change the name (give the new name).
    - **"custom voice"** — keep the structure, reshape the personality (3–6 tone keywords + a one-line vibe).
17. **Persona depth: full or compact?** Same behavioral contract (the Verdict rule, sacred ground, the Analysis Protocol) — full carries the showcase voice, compact spends fewer tokens every turn.
18. **Sacred ground** — the topics where the character drops the humor and reports flat. Be concrete: "privacy" is too vague; "patient session content and identifying details" is usable.

> **Why mandatory:** strip the persona and Claude falls back to vanilla assistant tone in every interactive turn while `/jc` and `/pcm` keep their voices — tonal whiplash. Adopters rename freely; what never ships is the _source project's_ domain content.

### Batch 7 — Host-level extras (each opt-in, each touches `~/` — ask, don't assume)

19. Which of these, if any?
    - **Statusline** — native `pfm statusline`: three-line status bar with model, fleet counts, context, cache window, cost, spend, and rate limits. Requires the `pfm` binary.
    - **Global settings merge** — `{"cleanupPeriodDays": 36500}` merged key-by-key into `~/.claude/settings.json`; stock Claude Code silently deletes transcripts _and orphaned worktrees_ after 30 days, and `0` is a validation error, not "off".
    - **Notify hooks** — "Professor is done — your turn" on 30s+ turns (macOS built-in; Linux swaps in `notify-send`).
    - **Markdown formatter hook** — prettier on framework-owned `.md` files after every edit.
    - **VSCode tmux launcher** — new terminals open into tmux + Claude (needs `tmux`).
    - **Multi-account fleet + `/chat:*`** — only offer if the user runs multiple accounts or several parallel chats; requires Go 1.24+, `tmux`, `zsh`, `fzf`, and a permanent clone. Build `pfm` first; its self-installer embeds the host command cards and helpers. Review `pfm install --dry-run`, then run `pfm install --apply`. Disclose plainly: fleet-launched chats run with permission prompts disabled by design.
    - **Theme** — Tokyo Night, source-fetched to `~/.claude/themes/`.
    - **Memory backup** — SessionStart/SessionEnd hooks sync Claude's project memory to a private git vault the user creates first. Protocol: `blueprint/references/memory-backup.md`.

### Batch 8 — Confirmation gate (mandatory, even if the user seems eager)

Before touching any file, show:

- The directory layout and full file list (count + paths).
- The customized persona frontmatter.
- **Proposed re-home moves** for existing docs (one row per file: source → destination + reason), and the files you couldn't classify (default proposal: `docs/dev/research/`).
- Anything you're still uncertain about.

The user types **"go"** to proceed — or corrects anything first.

---

## Re-homing existing project docs

For every file surfaced in pre-flight step 6, classify by filename hint **and** a quick content scan, then move with `git mv` (never copy-then-delete). First match wins:

| Signature                                                   | Destination                                               |
| ----------------------------------------------------------- | --------------------------------------------------------- |
| `THESIS`, `VISION`, `STRATEGY`, `MISSION`, `BUSINESS_MODEL` | `docs/business/<slug>.md`                                 |
| `GLOSSARY`, `TERMS`                                         | `docs/business/glossary.md` (one per project)             |
| Market / GTM / funding / risk material                      | `$CDOCS/mentor/…` if `/mentor` opted in                   |
| `COMPETITOR`, `SEO`, `POSITION`, `BRAND_VOICE`, channels    | `$CDOCS/marketer/…` if `/marketer` opted in               |
| `REGULATORY`, `COMPLIANCE`, `GDPR`, `HIPAA`, `PRIVACY`      | `$CDOCS/officer/…` if `/officer` opted in                 |
| `PERSONA`, `USER_*`, pains, jobs-to-be-done, workflows      | `$CDOCS/pm/…` if `/pm` opted in                           |
| Domain primers, protocols, methodologies                    | `$CDOCS/km/…` if `/km` opted in                           |
| Research logs, open questions, experiments, spikes          | `docs/dev/research/<slug>.md` (always available)          |
| `README`, `LICENSE*`, `CHANGELOG*`, `CONTRIBUTING*`         | **keep at root — never move**                             |
| Anything ambiguous                                          | **ask the user** (default proposal: `docs/dev/research/`) |

Within an archetype's `$CDOCS/<cmd>/` tree: `$REFS` = living must-know loaded nearly every invocation; `$RESEARCH` = looked-up analysis loaded on demand. "The rules / the canon" → `$REFS`; "I looked into X, here's what I found" → `$RESEARCH`.

If a file matches an archetype the user did **not** pick, offer the opt-in once (the file's existence suggests they need it); if still no, place it at `docs/dev/research/<slug>.md` and flag the imperfect homing in the final report. Leave all moves staged, uncommitted.

---

## Execution

Run **`blueprint/SETUP.md` Phase 2, steps in order**, applying the § Corrections above. Highlights the installer must not miss:

- **Materialization law:** templates carry PATTERN blocks written once with `{project}` tokens; expand each block **once per roster entry** — a roster of one expands once, never carry a block for a project the roster doesn't list.
- **Persona:** the emitted `CLAUDE.md` MUST contain the `## Your character — {NAME} (MANDATORY` heading; the chosen depth's output styles copy to `.claude/output-styles/{professor,jc,dr-house}.md` at bare filenames (the `.full`/`.compact` suffix exists only in the blueprint).
- **Tier B meta-blocks:** each emitted Tier B command starts at its H1 — the leading `>`-quoted "Tier B — Domain archetype" block is installer briefing, **delete it**. Verify by grepping the emitted file for `fill at install` / `Skip if:` / `Tier B — Domain archetype`; any hit means the block leaked.
- **`/wave:builder` roster pruning:** generate it from the actual roster; delete every block for a project that doesn't exist, then fail the step if any `{OPTIONAL_*}` placeholder remains or any referenced `*/.claude/agents/*.md` path is missing.
- **Scripts:** fill `PROJECTS=(…)`, port arrays, and dev start/stop blocks from the interview — then `chmod +x .claude/scripts/*.sh`. A script left with `{PLACEHOLDER}` tokens no-ops or fails; none of them is runnable unfilled.
- **State:** write `.professor/VERSION`, `.professor/manifest.json` (interview answers as the replay seed + SHA-256 hashes of every file you wrote, computed post-substitution), and seed `.professor/drift.md` (an `## Update history` table with the install row + an empty `## Post-install customizations`) and an empty `.professor/release.md`. These four files are what `/pcm:update` and `/pcm:release` operate on — updates replay the manifest's answers so the user never re-answers the interview.

---

## Verify the install

1. Port allocator round-trip (skip if the project needed no ports):
   ```bash
   .claude/scripts/alloc-ports.sh alloc smoke-test
   .claude/scripts/alloc-ports.sh list
   .claude/scripts/alloc-ports.sh free smoke-test
   ```
2. `/dev status` — proves `dev.sh` was filled correctly for this stack.
3. A deliberately tiny `/jc` task (e.g. "add an installation badge to the README") — exercises investigate → fix → test → gitter commit end to end, on something safe.

The first real run reveals anything the installer missed. When it does: `/pcm {issue}` — the fix lands in the agent definitions at the source, not as a patch note.

---

## Optional companions

Not part of Professor, never vendored — mention once, install only on request:

- **[claude-seo](https://github.com/AgriciDaniel/claude-seo)** (MIT) — SEO analysis plugin for Claude Code: 25 sub-skills, 18 specialist agents, technical SEO through AI-search (GEO) optimization. Useful when the project has a public web surface:
  ```
  /plugin marketplace add AgriciDaniel/claude-seo
  /plugin install claude-seo@agricidaniel-claude-seo
  /seo setup
  ```

---

## Hard rules for the installer (you, Claude)

1. **Never assume.** Every name, path, command, and port comes from the user's answers.
2. **Never overwrite without asking.** Existing `CLAUDE.md` / `.claude/` stops the install.
3. **Never install what wasn't picked.** No Tier B role, hook, or host extra lands silently.
4. **Never inject the source project's domain content.** The _voice_ is universal and ships by default; source-project domain references, vendor names, and jurisdiction specifics do not. Persona = "your project's flavor of Professor."
5. **Never run `git add` / `git commit` / `git push`.** Files only; the user reviews and commits.
6. **Never run destructive commands.** No `rm -rf`, no force-overwrite; back up to `tmp/` first if needed.
7. **Confirm before write.** Batch 8's "go" is mandatory.
8. **Keep it terse.** Batches are a few questions each; the user's time matters.

---

## Final report

```
Professor installed — your project's nervous system is live. 🧠

Files written: {N}
Customized for: {project name} ({roster size} project(s))
Professor disciplines: {list} — intersection lens: {lens}
Persona: {name}, {depth} depth
Optional roles: {list or "none"}   Codex: {yes|no}   Host extras: {list or "none"}
Docs re-homed: {N moved, M flagged}

Next:
- Read CLAUDE.md and confirm the structure looks right
- Run the three verification steps above if you haven't
- When something feels off: /pcm {the issue} — surgery at the source
- To update later: /pcm:update

Public repo: https://github.com/mreza0100/professor — file an issue if the installer missed your stack.
```
