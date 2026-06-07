# Blueprint — Export Pipeline Infrastructure

Maintain and publish a portable blueprint of `.claude/` so any Claude Code project can adopt the full Professor experience — characters, multi-PhD professor — refitted to their domain at install time.

Argument: `$ARGUMENTS` — subcommand (defaults to `export` if blank).

| Subcommand                                  | Action                                                                      |
| ------------------------------------------- | --------------------------------------------------------------------------- |
| `export` _(default)_                        | Regenerate blueprint files, commit, push to public `professor` repo         |
| `refresh`                                   | Regenerate blueprint files locally — no commit, no push                     |
| `status`                                    | Show what would change if `refresh` ran now (no writes)                     |
| `release {patch\|minor\|major} "{summary}"` | Bump VERSION, finalize CHANGELOG, commit + tag + push (calls refresh first) |

---

## Constants

- **Public repo:** `{BLUEPRINT_REPO}` (public GitHub)
- **Local clone:** `{BLUEPRINT_CLONE_PATH}` — the ONLY working copy
- **Blueprint tree:** `{BLUEPRINT_CLONE_PATH}blueprint/`
- **Public README:** `{BLUEPRINT_CLONE_PATH}README.md` — hand-curated, repo root
- **GH user:** `{GH_USER}`

If `{BLUEPRINT_CLONE_PATH}` missing, export clones it (or creates repo first if missing on GitHub).

---

## Philosophy

The blueprint exports a **transplantable nervous system**. An adopter gets the same characters, ranks, and swagger — refitted via interview. A neuropsych lab, RPG studio, and SCADA team all see _their version_ of the same archetypes. **The character IS the pipeline** — strip it and you ship a Confluence wiki. Cross-conversation context persists via **Epics** — initiative-level manifest files (`docs/epics/{name}/manifest.md`) with lifecycle tracking (PLANNING → IN_PROGRESS → SHIPPED).

**Update mechanism:** Adopters install at a specific git tag (`v0.5.0`). Their install creates a `.professor/` directory with `VERSION`, `manifest.json` (interview answers + file hashes as replay seed), `drift.md` (local customizations the merge keeps), and `release.md` (framework changes pending upstream sync). When upstream releases new tags, `/pcm update` replays interview answers against new templates, runs a three-way hash comparison, presents changes in three buckets (auto-apply / review / manual), and appends to `drift.md`. The update protocol lives in `templates/commands/pcm.md` § "Update Protocol".

---

## Three Tiers

| Tier                         | Description                                            | Ships                                                              | Gets parameterized                                                                |
| ---------------------------- | ------------------------------------------------------ | ------------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| **A — Universal archetypes** | Personalities that work in any domain. Voice IS value. | Full character, structure, identity                                | Domain REFERENCES inside the character (Professor's PhDs, JC's stack traces)      |
| **B — Domain archetypes**    | Roles every project needs, content domain-shaped       | Archetype skeleton: identity, voice, charter, modes, doc structure | Regulation, knowledge domain, user persona, market segment — filled via interview |
| **C — Pure mechanics**       | Infrastructure agents and plumbing                     | Mechanics only — no character                                      | Tech-specific commands (test runner, package manager, build tool)                 |

### Tier assignments

**Tier A** — `Professor` (persona), `/jc`, `/pcm`, `/audit`, `/build`, `/dev`, `/git`, `/wave`, `/documenter`, `/blueprint`
**Tier B** — `/officer` `{REGULATION}`, `/km` `{KNOWLEDGE_DOMAIN}`, `/pm` `{USER_PERSONA}`, `/mentor` `{MARKET_SEGMENT}`, `/marketer` `{CHANNEL_LANDSCAPE}`
**Tier C** — root agents (mono-planner, mono-architect, mono-documenter, gitter), scripts (worktree.sh, alloc-ports.sh, dev.sh), per-project agents (planner, architect, developer, qa, ui-ux, db-admin, devops, ai-engineer)

### Preservation (untouchable across tiers)

- Voice/tone of every Tier A character
- Archetype identity of Tier B commands
- Pipeline mechanics (planner → architect → developer → QA → gitter; worktree isolation; only-gitter-touches-git; QA gates pre+post merge; path variables)
- Discipline frame (zero-tolerance tests, mock policy, never-destructive-git, never-edit-main)

### Placeholders (project-specific → generic at refresh)

- Read `{BLUEPRINT_CLONE_PATH}blueprint/PLACEHOLDERS.md` — the canonical value→token map, cloned during refresh — and apply it uniformly, using one canonical token per concept. Never invent a synonym for a concept the map already names.
- Blueprint self-references (`{BLUEPRINT_REPO}`, `{GH_USER}`, `{BLUEPRINT_CLONE_PATH}`) → `{BLUEPRINT_REPO}`, `{GH_USER}`, `{BLUEPRINT_CLONE_PATH}` — resolved at install: a user with push access to the canonical repo targets it directly; everyone else targets their own fork

Character names (Professor, JC, etc.) ship as **default names with "rename if you want" instruction**. Concrete beats abstract.

---

## Subcommand: `refresh`

Re-derive blueprint from CURRENT `.claude/` and `CLAUDE.md` state. Edit files directly inside `{BLUEPRINT_CLONE_PATH}blueprint/`.

### 1. Source files to mine

From `{PROJECT_NAME}` repo:

- `CLAUDE.md` (root), `.claude/agents/*.md`, `.claude/commands/*.md` (Tier A+B), `.claude/skills/*/SKILL.md` (bundled + domain-hydrated only — see next bullet), `.claude/scripts/*.sh`
- **Source-fetched skills** (`rr`, `p:360`, `ghostwriter`, `vision-factory`) — never vendor a `SKILL.md` copy for these; they live in their own canonical repos and a stale copy is the exact drift this avoids. Refresh maintains only `templates/skills/sources.json` (name → repo); SETUP clones each at install.
- `docs/epics/` structure — Epics section of CLAUDE.md, manifest format, lifecycle, ownership rules
- One representative project's child agents + CLAUDE.md (e.g., `{BACKEND_PROJECT}/`) as per-project template

### 2. Tier-aware transformations

**Tier A:** KEEP voice/tone/structure/character/pipeline mechanics. REPLACE `{PROJECT_NAME}` identifiers + tech specifics + Professor's disciplines → `{PHD_DISCIPLINE_1}...{PHD_DISCIPLINE_N}`. KEEP Epics section structure (manifest format, lifecycle, ownership rules) — it is domain-agnostic.

**Tier B:** KEEP archetype skeleton. REPLACE domain content with named placeholders:

- **Officer:** `{REGULATION}`, `{REGULATION_FRAMEWORK_DOCS}`, `{ENFORCEMENT_AUTHORITY}`, `{DATA_SUBJECT_RIGHTS}`, `{INCIDENT_NOTIFICATION_TIMELINE}`
- **PM:** `{USER_PERSONA}`, `{PRODUCT_DOMAIN}`, `{USER_DAILY_WORKFLOW}`, `{USER_PAIN_POINTS}`
- **Mentor:** `{MARKET_SEGMENT}`, `{JURISDICTION}`, `{LEGAL_ENTITY_TYPE}`, `{FUNDING_LANDSCAPE}`, `{REGULATORY_BODIES}`
- **Marketer:** `{CHANNEL_LANDSCAPE}`, `{TARGET_LANGUAGE}`, `{COMPETITIVE_LANDSCAPE}`, `{INDUSTRY_CONFERENCES}`
- **KM:** `{KNOWLEDGE_DOMAIN}`, `{KNOWLEDGE_TAXONOMY}`, `{KNOWLEDGE_CONSUMERS}`, `{SOURCE_AUTHORITIES}`

**Tier C:** Strip tech specifics, keep structure, no character.

### 3. Output structure

```
{BLUEPRINT_CLONE_PATH}
├── README.md, INSTALL.md, CHANGELOG.md, VERSION, LICENSE
└── blueprint/
    ├── README.md, BLUEPRINT.md, SETUP.md, RELEASE.md
    └── templates/
        ├── CLAUDE.md
        ├── agents/ (mono-planner, mono-architect, mono-documenter, gitter, per-project/{planner,architect,developer,qa}.md)
        ├── commands/ (blueprint, build, jc, dev, git, wave, documenter, pcm, audit, officer, km, pm, mentor, marketer)
        ├── skills/
        │   ├── sources.json — source-fetched skills (rr, p:360, ghostwriter, vision-factory): cloned from their canonical repos at install, never vendored here
        │   ├── {p:rnd, p:refine, p:wave-review, p:quality:prompt, p:quality:doc}/ — bundled, ship as-is
        │   └── {p:analysis, p:audit:code-hygiene, p:audit:security}/ — domain-hydrated shells, filled by RR at setup
        ├── scripts/ (worktree.sh, alloc-ports.sh, dev.sh, notify.sh, format-md.sh)
        ├── epics/ (manifest template, lifecycle reference)
        ├── statusline/statusline-command.sh
        └── vscode/ (terminal-profile.json, zshrc-cc.snippet.sh, tmux.conf)
```

> **Statusline:** Tier C universal mechanic with light Professor personality in emoji choices (🟢→⚡→🔥→🚨 escalation, ◆/◇/○ model symbols). Ships as-is. SETUP.md copies to `~/.claude/statusline-command.sh` + adds config to settings.json.

> **VSCode tmux launcher:** Tier C universal mechanic, **opt-in** at install (it edits the user's _global_ editor + shell config, so SETUP.md asks first). New VSCode terminals open straight into `tmux + cc` — Claude Code inside a tmux session; on `/exit`, claude ends, the tmux session ends, and control falls back to a normal interactive shell — the terminal never closes on you. Ships the three files below (defined inline here — no repo source to mine); SETUP.md merges `terminal-profile.json` into the user's VSCode `settings.json`, appends `zshrc-cc.snippet.sh` to the user's shell rc, and copies `tmux.conf` to `~/.tmux.conf` (mouse scroll + click-to-copy — the comfort tmux-in-a-terminal assumes). The `cc` launcher is `typeset -f`-guarded so it **never clobbers an existing `cc`**.

`templates/vscode/terminal-profile.json` — merge into VSCode user `settings.json` (macOS keys shown; swap `osx`→`linux`/`windows` on other platforms):

```jsonc
"terminal.integrated.profiles.osx": {
  "tmux + claude": {
    "path": "/bin/zsh",
    "args": ["-l"],
    "env": { "VSCODE_AUTO_CC": "1" }
  }
},
"terminal.integrated.defaultProfile.osx": "tmux + claude"
```

`templates/vscode/zshrc-cc.snippet.sh` — append to the user's shell rc (`~/.zshrc`):

```zsh
# ── cc: Claude Code in tmux (reuses an existing cc if one is already defined) ──
if ! typeset -f cc >/dev/null; then
  cc() {
    if [[ -n "$TMUX" ]]; then
      claude "$@"
    else
      tmux new-session "claude $*"   # claude exits → tmux ends → back to shell
    fi
  }
fi

# ── VSCode: new terminals open straight into tmux + cc ──
# The "tmux + claude" profile sets VSCODE_AUTO_CC. Run cc once, then unset it so
# the tmux/claude children never re-trigger. On /exit you land back in a shell.
if [[ -o interactive && -n "$VSCODE_AUTO_CC" ]]; then
  unset VSCODE_AUTO_CC
  cc
fi
```

`templates/vscode/tmux.conf` — copy to `~/.tmux.conf` (mouse + click-to-copy; macOS `pbcopy` — swap for `xclip`/`wl-copy`/`clip.exe` on Linux/Windows):

```tmux
set -g mouse on
bind -T copy-mode    MouseDragEnd1Pane send-keys -X copy-pipe-and-cancel "pbcopy"
bind -T copy-mode-vi MouseDragEnd1Pane send-keys -X copy-pipe-and-cancel "pbcopy"
bind -T copy-mode    DoubleClick1Pane send-keys -X select-word \; send-keys -X copy-pipe-and-cancel "pbcopy"
bind -T copy-mode-vi DoubleClick1Pane send-keys -X select-word \; send-keys -X copy-pipe-and-cancel "pbcopy"
bind -T copy-mode    TripleClick1Pane send-keys -X select-line \; send-keys -X copy-pipe-and-cancel "pbcopy"
bind -T copy-mode-vi TripleClick1Pane send-keys -X select-line \; send-keys -X copy-pipe-and-cancel "pbcopy"
```

### 4. SETUP.md — interactive install interview

Exports an interview Claude conducts before touching files. Structure:

**Phase 1 — Interview** (8 questions in order):

1. Project identity (one sentence)
2. Character name & voice (keep Professor or rename?)
3. Project structure (single/monorepo, subproject count + purpose)
4. Tech stack per (sub)project (lang, framework, pkg mgr, test runner, build tool, DB, infra)
5. Professor's disciplines (10+ PhDs — what fits your domain?)
6. Tier B opt-ins: Officer (regulations?), KM (domain?), PM (persona?), Mentor (market+jurisdiction?), Marketer (channels+language?)
7. Sacred ground ("do no harm" in your domain)

**Phase 2 — Customization:** Rewrite every template replacing placeholders with interview answers. `/build` MUST be materialized from the actual project roster; delete planner/architect/developer/QA/db/devops blocks for missing projects and fail if any referenced agent path does not exist.

**Phase 2.5 — Skill Knowledge Hydration (domain-hydrated skills):**

Four skills ship as **empty shells** because their content is project-specific — the structure (frontmatter, headings, report format) is universal, but the audit categories, detection patterns, file paths, and domain concerns must be researched per project.

| Skill                  | What's universal (ships)                                                                                  | What's project-specific (hydrated by RR)                                                                                                                                                  |
| ---------------------- | --------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `p:analysis`           | Three-lens protocol (CS + domain + compliance), step sequence, report format, Cortex audit mode structure | Domain lens content (replaces Psychology lens), compliance framework, cross-disciplinary intersections, AI/ML audit categories + anti-patterns (if project has an AI pipeline subproject) |
| `p:audit:code-hygiene` | Category structure (ghost fields, dead code, stale deps, arch smells, type safety, naming, quality)       | Per-category detection patterns, file paths, known hotspots, linter coverage gaps, project-specific report examples                                                                       |
| `p:audit:security`     | OWASP category structure (8A-8I), severity guide, report format                                           | Domain-specific PHI/data sensitivity rules, external API checks, framework-specific vulnerabilities, compliance-driven sub-categories                                                     |

**Hydration process:**

1. For each domain-hydrated skill, check if the project has enough context from the interview (Phase 1) to run RR.
2. **If project is defined enough** → run `RR codebase` targeting the project's source to fill each skill's knowledge base. The RR agent reads the actual code, identifies patterns, file paths, anti-patterns, and writes the domain-specific sections into the skill's SKILL.md.
3. **If project is NOT defined enough** (new project, no code yet, or user skipped stack details) → write the skill with the universal structure but mark domain sections as empty:

```markdown
## Category N — {category name}

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.
> Run `/p:analysis` or `/p:audit:code-hygiene` after the codebase has enough code to analyze.
> The Professor will surface this gap: "Knowledge base is empty, waiting for user specification to fill it in."
```

4. **Professor behavior with empty skills:** When a domain-hydrated skill is invoked and its knowledge base sections are empty, the Professor MUST NOT improvise. Instead: state which sections are empty, ask the user to either (a) provide the specification now, (b) point to code/docs to RR against, or (c) defer. The Professor stays in this loop until the skill is filled — never proceeds with best-effort guessing on an empty knowledge base.

5. **Re-hydration:** User can re-run hydration at any time: "fill `/p:audit:security`" or "hydrate skills" → triggers RR against current codebase to fill/update empty sections.

**Phase 2.6 — Host tooling probe (git-host bridge):** Check the install machine for `gh` and `glab` (`command -v`). For each present, write a one-file index skill at `.claude/skills/host-{gh|glab}/SKILL.md` whose `description` records that the CLI is available on this host for {GitHub|GitLab} operations. It carries no procedure — it is the bridge that tells the Professor which CLI to drive: an adopter on GitLab forks + releases professor through `glab`, a GitHub adopter through `gh`, and `/blueprint release` and `/git` read this marker to target the right host. Absent tools get no skill. Then resolve the blueprint repo target: if the user has push access to the canonical repo, set `{BLUEPRINT_REPO}`/`{GH_USER}`/`{BLUEPRINT_CLONE_PATH}` to it; otherwise have them fork it and use the fork.

**Phase 3 — Smoke test:** Run `/build` with a tiny test feature to verify end-to-end.

### 5. Process

- `{BLUEPRINT_CLONE_PATH}` is the ONE source of truth. Before editing: `git fetch origin && git pull --ff-only origin main`.
- Use `Edit` for surgical updates, `Write` for new files/full rewrites.
- Preserve manually-curated commentary unless it contradicts current state.
- Do NOT delete `INSTALL.md`, `LICENSE`, or hand-curated root files.

### 8. Report after refresh

```
Blueprint refreshed in {BLUEPRINT_CLONE_PATH}blueprint/. {N} files updated, {M} unchanged.
Tier A: {count} | Tier B: {count} | Tier C: {count}
Sources mined: {list}
Generalizations: identifiers→placeholders {count}, tech→placeholders {count}, domain→slots {count}
Character preservation: Professor ✓, JC ✓
Ready to /blueprint export.
```

---

## Subcommand: `export` (refresh + push)

```pseudo
1. Ensure clone exists + up-to-date:
   if !exists {BLUEPRINT_CLONE_PATH}.git → create repo on GH if needed → clone
   else → fetch + pull --ff-only (STOP if fails)

2. Run refresh subcommand (bail on failure)

3. Update public README.md:
   if missing → write from § Public README template below
   if exists → diff against template; overwrite only if template changed

4. Commit + push:
   git add -A
   exit if no staged changes
   commit: "update: blueprint refresh from {PROJECT_NAME}\nSource: {{PROJECT_NAME}-sha-short}\nCo-Authored-By: Professor <noreply@anthropic.com>"
   git push origin main

5. Report: URL, commit sha, files changed, source SHA
```

---

## Subcommand: `release {patch|minor|major} "{summary}"`

```pseudo
1. Validate args (bump type + summary required, bail if missing)
   patch = bug fixes/doc tweaks | minor = new archetype/command/step | major = breaking/migration

2. Run refresh (STOP if fails)

3. Read VERSION, compute new version

4. Build CHANGELOG bullets from `.professor/release.md` — the pending-sync queue is the source of what ships (format: "- {Tier}: {scope} — {semantic change}").
   if release.md empty → prompt maintainer for bullets
   Per-bullet migration sub-headings (#### → For:) required for adopter-side action
   Informational-only bullets marked: **`/pcm update`: skip — informational only.**

5. Finalize CHANGELOG:
   [Unreleased] → [{NEW_VERSION}] — {YYYY-MM-DD}
   Insert fresh empty [Unreleased] above

6. echo "{NEW_VERSION}" > {BLUEPRINT_CLONE_PATH}VERSION

7. Commit + tag + push:
   commit: "release: v{NEW_VERSION} — {summary}\nSource: {sha}\nCo-Authored-By: Professor <noreply@anthropic.com>"
   git tag -a "v{NEW_VERSION}" -m "v{NEW_VERSION}"   # annotated — --follow-tags skips lightweight tags
   git push origin main --follow-tags (STOP if fails, NEVER force-push)

8. Clear `.professor/release.md` — its entries shipped in this release; empty the pending list, keep the header.

9. Report: tag URL, commit, source SHA, changelog bullets
```

### Pre-release checklist

- `gh auth status` authenticated as `{GH_USER}`
- Refresh succeeded
- `[Unreleased]` non-empty
- No secrets in staged diff
- Staged templates grep clean (0 hits) for the project brand (current AND former name), founder name, and `/Users/` machine paths — refresh swaps the brand for `{PROJECT_NAME}`, so a single leftover is a refresh bug, not an exception
- New version > local version

---

## Subcommand: `status`

1. In `{BLUEPRINT_CLONE_PATH}`: `git fetch origin && git status && git diff --stat origin/main...HEAD`
2. Spot-check key `{PROJECT_NAME}` sources against current blueprint contents
3. Report stale files + local uncommitted state. No writes, no pushes.

---

## Hard rules

**NEVER:** push secrets, commit project-specific identifiers (the project's own brand name — current AND former — founder PII, internal URLs, machine-absolute `/Users/` paths), force-push, ship Tier A characters with empty placeholders, strip archetype identity to abstraction, auto-bump README version without re-checking template, stage in `tmp/` or anywhere outside `{BLUEPRINT_CLONE_PATH}`. **Repo is PUBLIC — every export is world-visible on push.**

---

## Pre-flight checks (before any subcommand)

1. `gh auth status` — must be `{GH_USER}`
2. `git status` in `{PROJECT_NAME}` — note uncommitted state (don't fail)
3. `{BLUEPRINT_CLONE_PATH}` — if missing AND command is `export`/`refresh`, clone/create per above
4. Inside clone: confirm clean or only in-progress refresh edits (bail on unrelated dirty state)

---

## Public README template

> When writing/refreshing `{BLUEPRINT_CLONE_PATH}README.md`, expand this structural outline into the full README. Keep it terse, opinionated, pitch-forward.

**Structure (expand at runtime):**

```
# Professor — Multi-Agent Claude Code Pipeline

One-paragraph pitch: portable .claude/ that turns Claude Code into a self-disciplined engineering team with character. Personality is load-bearing.

## What you get
- Full cast (Professor, JC, Audit + Tier B opt-ins)
- Pipeline (planner→architect→developer→QA→gitter)
- Worktree isolation + port allocation
- Single git owner (gitter)
- Hotfix mode (/jc)
- Self-improvement at source (/pcm)
- Manifest-driven updates (`/pcm update` — git tag pinning, interview replay, three-bucket diff)
- Epics — cross-conversation context persistence via manifest files (PLANNING → IN_PROGRESS → SHIPPED)
- Path conventions ($DOCS, $WORKTREE, $CDOCS)
- Documentation discipline (one agent writes permanent docs)
- VSCode terminals that auto-open into tmux + Claude (`/exit` → back to your shell)

## Quick start
git clone, cd your-project, claude → read blueprint → follow SETUP.md → interview → customize → smoke test

## The cast — Tier A
Professor, /jc, /pcm, /audit, /build, /dev, /git, /wave, /documenter

## Tier B (opt-in)
/officer, /km, /pm, /mentor, /marketer

## The five load-bearing walls
1. Only gitter touches git
2. QA gates the merge (pre+post)
3. Path variables, not hardcoded
4. Worktree isolation per pipeline
5. Self-improvement at the source

## When to use it
✅ Multi-project monorepos, complex pipelines, teams losing work to half-finished branches, decision-audit matters, agents with voice
⚠️ Overkill for: 200-line scripts, throwaway prototypes, projects where main can break

## Origin & maintenance
Auto-regenerated from live {PROJECT_NAME} repo. Maintained by @{GH_USER}. Issues/PRs welcome (open issue first for large changes).

## License
MIT
```

---

## Reporting

Always end with one of:

- `Blueprint exported. URL: https://github.com/{BLUEPRINT_REPO}/commit/{sha}`
- `Blueprint refreshed in {BLUEPRINT_CLONE_PATH}. Run /blueprint export to publish.`
- `Blueprint status: {N} files would change. Run /blueprint refresh or /blueprint export.`
