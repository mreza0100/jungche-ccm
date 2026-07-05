# The Refresh Pass — Re-derive the Blueprint from Live Source

Executed inside `/pcm:release` (step 3). Re-derives the blueprint from the CURRENT `.claude/` and `CLAUDE.md` state. Edit files directly inside `{BLUEPRINT_CLONE_PATH}blueprint/`.

**Scope (incremental):** `blueprint/refresh-map.json` maps every template to its live source(s) + the SHA-256 of each as of the last sync. `scripts/refresh-scope.sh scan` proves unchanged sources untouched — their templates are skipped; re-derive only CHANGED templates (plus files named in `.professor/release.md`); UNMAPPED-LIVE files get a mapping ruling. `curated` templates have no live source and are never auto-derived. `refresh-scope.sh regen` re-baselines the hashes at release end.

**Update mechanism context:** Adopters install at a specific git tag (`v0.5.0`). Their install creates a `.professor/` directory with `VERSION`, `manifest.json` (interview answers + file hashes as replay seed), `drift.md` (local customizations the merge keeps), and `release.md` (framework changes pending upstream sync). When upstream releases new tags, `/pcm:update` replays interview answers against new templates, runs a three-way hash comparison, and presents changes in three buckets. The blueprint update/release protocol ships in `templates/commands/pcm/` as the `update` and `release` subcommands of the `pcm` command directory; this refresh-pass reference lives at `templates/commands/pcm/references/refresh.md`.

Cross-conversation context persists via **Epics** — initiative-level manifest files (`docs/epics/{name}/manifest.md`) with lifecycle tracking (PLANNING → IN_PROGRESS → SHIPPED).

---

## Three Tiers

| Tier                         | Description                                            | Ships                                                              | Gets parameterized                                                                |
| ---------------------------- | ------------------------------------------------------ | ------------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| **A — Universal archetypes** | Personalities that work in any domain. Voice IS value. | Full character, structure, identity                                | Domain REFERENCES inside the character (Professor's PhDs, JC's stack traces)      |
| **B — Domain archetypes**    | Roles every project needs, content domain-shaped       | Archetype skeleton: identity, voice, charter, modes, doc structure | Regulation, knowledge domain, user persona, market segment — filled via interview |
| **C — Pure mechanics**       | Infrastructure agents and plumbing                     | Mechanics only — no character                                      | Tech-specific commands (test runner, package manager, build tool)                 |

### Tier assignments

**Tier A** — `Professor` (persona), `/jc`, `/pcm` (with its `update` and `release` subcommands), `/wave:builder`, `/dev`, `/git`, `/wave:orchestrator`, `/documenter`, `/animate`, `/save`, `/p:slow-burn`
**Tier B** — `/officer` `{REGULATION}`, `/km` `{KNOWLEDGE_DOMAIN}`, `/pm` `{USER_PERSONA}`, `/mentor` `{MARKET_SEGMENT}`, `/marketer` `{CHANNEL_LANDSCAPE}`
**Tier C** — root agents (mono-planner, mono-architect, mono-documenter, gitter), scripts (worktree.sh, alloc-ports.sh, dev.sh), per-project agents (planner, architect, developer, qa, ui-ux, db-admin, devops, ai-engineer)

### Preservation (untouchable across tiers)

- Voice/tone of every Tier A character
- Archetype identity of Tier B commands
- Pipeline mechanics (planner → architect → developer → QA → gitter; worktree isolation; only-gitter-touches-git; QA gates pre+post merge; path variables)
- Discipline frame (zero-tolerance tests, mock policy, never-destructive-git, never-edit-main)

### Placeholders (project-specific → generic at refresh)

> Human copy — the executable copy is `scripts/placeholder-map.tsv`, applied by `scripts/genericize.sh` as the deterministic first pass on every re-derived template; edit both together. The LLM hand-judges structure only (roster collapsing, domain nouns, persona metaphors).

- `{PROJECT_NAME}` (and any former brand the repo was renamed from — a rename orphans the old name in the blueprint source, so scrub both), per-project directories → `{PROJECT_NAME}`, `{project-a}` etc.
- `Professor` → keep with "rename if you want" comment
- domain/user nouns (the project's subject matter, its users, its work units) → `{DOMAIN_NOUN}`, `{USER_NOUN}`
- the project's regulatory frame, jurisdiction, and legal-entity type → `{REGULATION}`, `{JURISDICTION}`, `{LEGAL_ENTITY_TYPE}`
- All tech specifics (transcription/AI providers, frameworks, ORMs, mobile/web stacks, API layers, databases, infra/cloud/hosting) → `{TECH_STACK_PLACEHOLDER}` per role
- Ports → `{PORT_A}`, `{PORT_B}`; package managers/test runners → `{PACKAGE_MANAGER}`, `{TEST_RUNNER}`
- Blueprint self-references (`{BLUEPRINT_REPO}`, `{GH_USER}`, `{BLUEPRINT_CLONE_PATH}`) → resolved at install: a user with push access to the canonical repo targets it directly; everyone else targets their own fork

Character names (Professor, JC, etc.) ship as **default names with "rename if you want" instruction**. Concrete beats abstract.

---

## 1. Source files to mine

From the project repo:

- `CLAUDE.md` (root), `.claude/agents/*.md`, `.claude/commands/*.md` (Tier A+B, including command directories like `.claude/commands/pcm/`, `.claude/commands/wave/`, `.claude/commands/audit/`, `.claude/commands/quality/`), `.claude/skills/*/SKILL.md` (source-fetched skills only — see next bullet), `.claude/scripts/*.sh`
- **Source-fetched skills** (`rr`, `360`, `ghostwriter`, `vision-factory`) — never vendor a `SKILL.md` copy for these; they live in their own canonical repos and a stale copy is the exact drift this avoids. Refresh maintains only `templates/skills/sources.json` (name → repo); SETUP clones each at install.
- `docs/epics/` structure — Epics section of CLAUDE.md, manifest format, lifecycle, ownership rules
- `docs/agents/` scaffold — the hub `_index.md` format, the `standards.md` skeleton, and the cluster convention (structure only, NEVER doc content — every adopter's documentation body is their own)
- The source's per-project structure → mine it INTO the generic **roster PATTERN**: express each per-project file/section ONCE with `{project}` tokens (one representative project as the shape). NEVER bake the source's project count or role names into a template — the source's concrete roster (its N projects, those roles) is an install instance SETUP expands per entry, not template structure. A template must read correctly at roster size 1. See `PLACEHOLDERS.md` § "Project roster".

## 2. Tier-aware transformations

**Tier A:** KEEP voice/tone/structure/character/pipeline mechanics. REPLACE project identifiers + tech specifics + Professor's disciplines → `{PHD_DISCIPLINE_1}...{PHD_DISCIPLINE_N}`. KEEP Epics section structure (manifest format, lifecycle, ownership rules) — it is domain-agnostic. **Persona depth is load-bearing — PARAMETERIZE a persona's generic sections (swap domain refs for placeholders); NEVER delete or trim them.** Worked voice examples and sections like "What NOT to do", "The relationship with the work", and the Verdict examples ARE the value; mirror the live persona section-for-section, genericized — a thinned persona ships a weaker character. When a live persona carries a section the blueprint lacks, add it (genericized), never drop it.

**Tier B:** KEEP archetype skeleton. REPLACE domain content with named placeholders:

- **Officer:** `{REGULATION}`, `{REGULATION_FRAMEWORK_DOCS}`, `{ENFORCEMENT_AUTHORITY}`, `{DATA_SUBJECT_RIGHTS}`, `{INCIDENT_NOTIFICATION_TIMELINE}`
- **PM:** `{USER_PERSONA}`, `{PRODUCT_DOMAIN}`, `{USER_DAILY_WORKFLOW}`, `{USER_PAIN_POINTS}`
- **Mentor:** `{MARKET_SEGMENT}`, `{JURISDICTION}`, `{LEGAL_ENTITY_TYPE}`, `{FUNDING_LANDSCAPE}`, `{REGULATORY_BODIES}`
- **Marketer:** `{CHANNEL_LANDSCAPE}`, `{TARGET_LANGUAGE}`, `{COMPETITIVE_LANDSCAPE}`, `{INDUSTRY_CONFERENCES}`
- **KM:** `{KNOWLEDGE_DOMAIN}`, `{KNOWLEDGE_TAXONOMY}`, `{KNOWLEDGE_CONSUMERS}`, `{SOURCE_AUTHORITIES}`

**Tier C:** Strip tech specifics, keep structure, no character.

## 3. Output structure

```
{BLUEPRINT_CLONE_PATH}
├── README.md, INSTALL.md, CHANGELOG.md, VERSION, LICENSE
└── blueprint/
    ├── README.md, BLUEPRINT.md, SETUP.md, RELEASE.md
    └── templates/
        ├── CLAUDE.md
        ├── agents/ (mono-planner, mono-architect, mono-documenter, gitter, per-project/{planner,architect,developer,qa}.md)
        ├── commands/ (animate, jc, dev, git, documenter, goal-definer, officer, km, pm, mentor, marketer; plus command directories: wave/ (orchestrator, builder, refine, walker, live, schedule), pcm/ (update, release, references/refresh.md), audit/ (code-hygiene, security), quality/ (prompt, doc), p/ (rnd, 360, slow-burn, tokens/), chat/{save,load,interrogate,…})
        ├── skills/
        │   └── sources.json — source-fetched skills (rr, 360, ghostwriter, vision-factory): cloned from their canonical repos at install, never vendored here. (All former bundled p:* skills — rnd, wave:refine, wave:walker, quality:prompt, quality:doc, audit:code-hygiene, audit:security — are now nested commands under commands/; the domain-hydrated shells live at commands/audit/{code-hygiene,security}.md, filled by RR at setup.)
        ├── workflows/ (wave-build, wave-walker, documenter-fanout, audit-ai-output-sessions)
        ├── scripts/ (worktree.sh, alloc-ports.sh, dev.sh, notify.sh, format-md.sh)
        ├── epics/ (manifest template, lifecycle reference)
        ├── docs-agents/ (_index.md hub skeleton + standards.md skeleton — SETUP Phase 2.7 seeds docs/agents/ from these, then /documenter bootstrap builds the clusters)
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

## 4. SETUP.md — interactive install interview

Exports an interview Claude conducts before touching files. Structure:

**Phase 1 — Interview** (8 questions in order):

1. Project identity (one sentence)
2. Character name & voice (keep Professor or rename?)
3. Project structure (single/monorepo, subproject count + purpose)
4. Tech stack per (sub)project (lang, framework, pkg mgr, test runner, build tool, DB, infra)
5. Professor's disciplines (10+ PhDs — what fits your domain?)
6. Tier B opt-ins: Officer (regulations?), KM (domain?), PM (persona?), Mentor (market+jurisdiction?), Marketer (channels+language?)
7. Sacred ground ("do no harm" in your domain)

**Phase 2 — Customization:** Rewrite every template replacing placeholders with interview answers. `/wave:builder` MUST be materialized from the actual project roster; delete planner/architect/developer/QA/db/devops blocks for missing projects and fail if any referenced agent path does not exist.

**Phase 2.5 — Skill Knowledge Hydration (domain-hydrated skills):**

Skills ship as **empty shells** when their content is project-specific — the structure (frontmatter, headings, report format) is universal, but the audit categories, detection patterns, file paths, and domain concerns must be researched per project.

| Skill                  | What's universal (ships)                                                                                  | What's project-specific (hydrated by RR)                                                                                                                                                  |
| ---------------------- | --------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Analysis Protocol (in the Professor persona output style) | Three-lens protocol (CS + domain + compliance), step sequence, report format, AI/ML audit mode structure | Domain lens content (replaces Psychology lens), compliance framework, cross-disciplinary intersections, AI/ML audit categories + anti-patterns (if project has an AI pipeline subproject) — the refresh interview hydrates this lens directly in the persona output style |
| `audit:code-hygiene`   | Category structure (ghost fields, dead code, stale deps, arch smells, type safety, naming, quality)       | Per-category detection patterns, file paths, known hotspots, linter coverage gaps, project-specific report examples                                                                       |
| `audit:security`       | OWASP category structure (8A-8I), severity guide, report format                                           | Domain-specific PHI/data sensitivity rules, external API checks, framework-specific vulnerabilities, compliance-driven sub-categories                                                     |

**Hydration process:**

1. For each domain-hydrated skill, check if the project has enough context from the interview (Phase 1) to run RR.
2. **If project is defined enough** → run `RR codebase` targeting the project's source to fill each command's knowledge base. The RR agent reads the actual code, identifies patterns, file paths, anti-patterns, and writes the domain-specific sections into the command's body (`.claude/commands/audit/{code-hygiene,security}.md`).
3. **If project is NOT defined enough** (new project, no code yet, or user skipped stack details) → write the skill with the universal structure but mark domain sections as empty:

```markdown
## Category N — {category name}

> **KNOWLEDGE BASE EMPTY** — This section needs project-specific detection patterns.
> Run the Professor's Analysis Protocol or `/audit:code-hygiene` after the codebase has enough code to analyze.
> The Professor will surface this gap: "Knowledge base is empty, waiting for user specification to fill it in."
```

4. **Professor behavior with empty commands:** When a domain-hydrated audit command is invoked and its knowledge base sections are empty, the Professor MUST NOT improvise. Instead: state which sections are empty, ask the user to either (a) provide the specification now, (b) point to code/docs to RR against, or (c) defer. The Professor stays in this loop until the command is filled — never proceeds with best-effort guessing on an empty knowledge base.

5. **Re-hydration:** User can re-run hydration at any time: "fill `/audit:security`" or "hydrate the audit commands" → triggers RR against current codebase to fill/update empty sections.

**Phase 2.6 — Host tooling probe (git-host bridge):** Check the install machine for `gh` and `glab` (`command -v`). For each present, write a one-file host command at `.claude/commands/h/{gh|glab}.md` (the `h:` host namespace) whose `description` records that the CLI is available on this host for {GitHub|GitLab} operations. It carries no procedure — it is the bridge that tells the Professor which CLI to drive: an adopter on GitLab forks + releases professor through `/h:glab`, a GitHub adopter through `/h:gh`, and `/pcm:release` and `/git` read this marker to target the right host. These host-local bridges are KEEP-LOCAL — excluded from the portable blueprint. Absent tools get no command. Then resolve the blueprint repo target: if the user has push access to the canonical repo, set `{BLUEPRINT_REPO}`/`{GH_USER}`/`{BLUEPRINT_CLONE_PATH}` to it; otherwise have them fork it and use the fork.

**Phase 3 — Smoke test:** Run `/wave:builder` with a tiny test feature to verify end-to-end.

## 5. Public README

If `{BLUEPRINT_CLONE_PATH}README.md` is missing → write it from the template below. If it exists → diff against the template; overwrite only if the template changed.

> Expand this structural outline into the full README. Keep it terse, opinionated, pitch-forward.

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
- Manifest-driven updates (`/pcm:update` — git tag pinning, interview replay, three-bucket diff)
- Epics — cross-conversation context persistence via manifest files (PLANNING → IN_PROGRESS → SHIPPED)
- Path conventions ($DOCS, $WORKTREE, $CDOCS)
- Documentation discipline (one agent writes permanent docs)
- VSCode terminals that auto-open into tmux + Claude (`/exit` → back to your shell)

## Quick start
git clone, cd your-project, claude → read blueprint → follow SETUP.md → interview → customize → smoke test

## The cast — Tier A
Professor, /jc, /pcm, /wave:builder, /dev, /git, /wave:orchestrator, /documenter

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
Auto-regenerated from the live upstream repo. Maintained by @{GH_USER}. Issues/PRs welcome (open issue first for large changes).

## License
MIT
```

## 6. Process rules

- `{BLUEPRINT_CLONE_PATH}` is the ONE source of truth. Before editing: `git fetch origin && git pull --ff-only origin main`.
- Use `Edit` for surgical updates, `Write` for new files/full rewrites.
- Preserve manually-curated commentary unless it contradicts current state.
- Do NOT delete `INSTALL.md`, `LICENSE`, or hand-curated root files.

## 7. Report after the refresh pass

```
Refresh pass complete in {BLUEPRINT_CLONE_PATH}blueprint/. {N} files updated, {M} unchanged.
Tier A: {count} | Tier B: {count} | Tier C: {count}
Sources mined: {list}
Generalizations: identifiers→placeholders {count}, tech→placeholders {count}, domain→slots {count}
Character preservation: Professor ✓, JC ✓
Continuing release.
```
