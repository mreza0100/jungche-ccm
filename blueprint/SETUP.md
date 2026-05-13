# SETUP — Installing Professor

Run inside your target project. Claude reads this file, conducts an interview, then customizes every template before copying into your repo. Result: a `.claude/` that reads like it was written for your project, because it was.

---

## Prerequisites

- Git repository (at least one commit on `main` or `master`)
- Claude Code CLI installed and configured
- 10 minutes for the interview

---

## How to install

**The fastest path:** let Claude conduct the interview.

```bash
# Clone the blueprint at a specific release tag (put it anywhere you like)
git clone --branch v0.5.0 https://github.com/mreza0100/professor.git /path/to/professor

# Inside YOUR project
cd /path/to/your-project
claude
> Read every file in /path/to/professor/blueprint/.
> Follow SETUP.md to install Professor in THIS project.
> Conduct the interview before touching any files.
```

> **Note:** `/path/to/professor` is wherever you cloned the repo — `~/tools/professor`, `~/repos/professor`, `/tmp/professor`, anywhere. The blueprint reads from there during install; afterwards you can keep it around for future `/pcm update` or delete it (updates can re-fetch via git tags).

Claude runs Phase 1 (interview), then Phase 2 (customization), then Phase 3 (smoke test). You answer about 10 questions. Claude does the rest.

**The manual path:** read `BLUEPRINT.md`, `ARCHETYPES.md`, `ADAPTATION.md`, copy templates manually, replace placeholders by hand. Slower but doable.

---

## Phase 1 — The interview

Claude (in your target project) asks these questions, in this order. Answer them however you want — short, long, with examples. Claude will turn them into template parameters.

### 1. Project identity

> What does your project do, in one sentence?

This becomes `{PROJECT_NAME}` and `{PROJECT_PITCH}`. Example: "Freudche is an AI clinical administrative tool that listens to therapy sessions and assists the therapist."

### 2. Character name & voice (MANDATORY — cannot be skipped)

> Default character is **Professor** — grandfatherly polymath with 10+ PhDs. Warm, precise, gently devastating. Cross-disciplinary lens. Takes life easy but not too easy. Pick: keep Professor, rename (voice stays), or supply a custom voice (3–6 tone keywords + a one-line vibe). You MUST land on one — the persona section is load-bearing infrastructure, not optional flavor.

Most adopters keep Professor as-is. The voice transplants well across domains. If you want a different name (e.g., "Beatrix" for a finance project, "Gandalf" for an open-source library), name it. The voice can stay.

Then tell Claude your **sacred ground** — the topics where the character drops the humor and reports flat (e.g., "patient data", "user funds", "physical safety in autonomous control"). This goes into the persona's "What NOT to do" block. Without sacred ground defined, the character will make jokes in places it shouldn't.

### 3. Project structure

> Single project, or monorepo? If monorepo, how many subprojects, and what does each do?

For each subproject, Claude needs:

- Directory name (you choose)
- One-line description
- Tech: language, framework, package manager, test runner, build tool, dev server port

Example:

- `api` — Express + GraphQL backend, pnpm, vitest, port 3000
- `web` — React frontend, npm, jest, port 5173
- `worker` — Python processing service, uv, pytest, no port (queue consumer)
- `infra` — Docker Compose for PostgreSQL + Redis
- `marketing` — Next.js marketing site, npm, Vercel, port 3001

### 4. Tech stack details

For each subproject, Claude pins these into the agents and scripts:

- Test command (`pnpm test`, `pytest`, `cargo test`, etc.)
- Lint command
- Typecheck command (if applicable)
- Build command
- Dev server start command
- Dependency install command (`pnpm install`, `uv sync`, `cargo build`, etc.)

These go into `worktree.sh`, `dev.sh`, and the developer + qa agent files.

### 5. Professor's disciplines

> The Professor archetype holds 10+ PhDs. The voice is grandfatherly polymath; the **disciplines** parameterize per project. What 10+ disciplines should YOUR Professor hold, to span your domain?

Examples:

| Project type        | 10 disciplines                                                                                                                                                                                   |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Therapy AI          | CS, Clinical Psych, AI/ML, HCI, Statistics, Linguistics, Privacy/Security, UX, Software Architecture, Therapy Methodology                                                                        |
| Neuropsych research | Neuroscience, Cognitive Science, Computational Modeling, Statistics, Clinical Methodology, Software Engineering, Information Theory, Linguistics, Philosophy of Mind, Research Methods           |
| Game studio         | Game Design, Narrative Theory, Probability, Behavioral Economics, UX, Mathematics, Art Direction, Audio Design, Software Engineering, Player Psychology                                          |
| FinTech trading     | Financial Engineering, Statistics, ML, Distributed Systems, Securities Law, Game Theory, Microeconomics, Software Engineering, Cybersecurity, Behavioral Finance                                 |
| Open-source library | Software Engineering, Programming Language Theory, Distributed Systems, Cryptography, Type Theory, Compiler Design, Operating Systems, Performance Engineering, API Design, Documentation Theory |

Pick the 10 that span what your project needs to reason about. Claude embeds them into the Professor command file.

Also: identify the **intersection lens** — which two disciplines, when combined, produce your Professor's unique superpower? (Freudche: CS × Clinical Psychology. Neuropsych: Neuroscience × Computational Modeling. Game studio: Game Design × Player Psychology.)

### 6. Council panel

> The Council debates topics with 5 voices in three rounds (opening / rebuttal / verdict). Universal members **JC + Professor** are always in. Who fills the other 3 seats?

Standard panel: pick 3 from the Tier B opt-ins below. Most projects pick Officer + PM + Mentor, or Officer + PM + Marketer.

Smaller council (3 voices: JC + Professor + 1) works fine for solo or research projects. The three-round structure scales.

### 7. Tier B opt-ins

For each Tier B archetype, opt in or skip. For each opt-in, fill in the placeholders.

#### `/officer` — compliance enforcer

> Do you have regulatory exposure? GDPR, HIPAA, FDA, SOC2, ISO 27001, MiFID, export controls, supply-chain rules, financial reporting?

If yes, fill in:

- `{REGULATION}` — the framework name(s)
- `{ENFORCEMENT_AUTHORITY}` — the body that enforces
- `{DATA_SUBJECT_RIGHTS}` — the rights framework
- `{INCIDENT_NOTIFICATION_TIMELINE}` — your breach-notification deadline

If no, skip — most projects don't need this.

#### `/km` — knowledge curator

> Do you maintain a curated research corpus? (Therapy approaches, game design patterns, legal precedents, scientific protocols, etc.)

If yes, fill in:

- `{KNOWLEDGE_DOMAIN}` — what's in the corpus
- `{KNOWLEDGE_TAXONOMY}` — how it's organized
- `{KNOWLEDGE_CONSUMERS}` — what reads from it
- `{SOURCE_AUTHORITIES}` — what counts as primary

If no, skip.

#### `/pm` — user+product hybrid

> Do you have an end-user persona that should shape product decisions?

If yes, fill in:

- `{USER_PERSONA}` — primary user (therapist, gamer, surgeon, lawyer, developer, etc.)
- `{PRODUCT_DOMAIN}` — what the product does
- `{USER_DAILY_WORKFLOW}` — what a typical day looks like
- `{USER_PAIN_POINTS}` — what hurts in their current workflow
- `{PERSONA_VARIANTS}` — secondary personas

If no (e.g., pure infrastructure library), skip.

#### `/mentor` — business advisor

> Is this a commercial venture? Do you need NL/US/UK/etc. company formation, funding, GTM, regulatory cost/benefit advice?

If yes, fill in:

- `{MARKET_SEGMENT}` — your market
- `{JURISDICTION}` — country + regions
- `{LEGAL_ENTITY_TYPE}` — local entity type (BV, LLC, GmbH, Ltd, etc.)
- `{FUNDING_LANDSCAPE}` — VCs, angels, grants relevant to your space
- `{REGULATORY_BODIES}` — agencies/laws affecting business operations

If no (open-source, research, hobby), skip.

#### `/marketer` — visibility strategist

> Do you market this product, write content, attend conferences, or run sales/SEO?

If yes, fill in:

- `{CHANNEL_LANDSCAPE}` — channels your audience uses
- `{TARGET_LANGUAGE}` — primary marketing language (en, nl, de, ja, etc.)
- `{COMPETITIVE_LANDSCAPE}` — named competitors
- `{INDUSTRY_CONFERENCES}` — events that matter

If no, skip.

### 7b. Codex dual-runtime (OPTIONAL)

> Do you also use OpenAI Codex? (Everything works without it — this adds a second runtime for cheaper implementation.)

If yes: the installer creates `.codex/` with `.toml` wrappers that mirror `.claude/`, plus an `AGENTS.md` symlink → `CLAUDE.md`. Claude orchestrates and does QA; Codex implements. Codex can also run full pipelines end-to-end in Teams mode.

If no: skip — the entire Codex layer is omitted. No pipeline operation requires it.

### 8. Sacred ground

> What does "do no harm" mean in your domain? Privacy, safety, correctness, financial integrity, narrative coherence, scientific reproducibility, security?

This becomes `{SACRED_GROUND}` and is referenced by:

- The Professor (the "sacred ground" rule where humor disappears)
- JC (the trigger that escalates from chill to temple-flipping)
- Officer (if opted in — the protected category)
- Council (the trump card in verdicts)

Be specific. "Privacy" is too vague. "Patient session content and identifying details" is concrete. "Financial transaction integrity at the millisecond level" is concrete. "Scientific data reproducibility for FDA submissions" is concrete.

### 9. Port allocation

> What port ranges are free on your dev machine?

Claude pins them into `alloc-ports.sh`. Default is something like 3000-3099 for backend, 8080-8179 for frontend, 5432-5531 for postgres, etc. — adjust to whatever's free.

### 10. Confirmation

Claude shows you a summary of all answers + a list of files that will be written. You confirm or edit. Then Phase 2 begins.

---

## Phase 2 — Customization

Claude takes your answers and:

1. **Writes root `CLAUDE.md`** — fills in `{PROJECT_NAME}`, `{PROJECT_PITCH}`, the Professor persona section, the project structure tree, the non-negotiable rules. Strict-mode rules adapted to your stack.
2. **Writes per-project `CLAUDE.md` files** (if monorepo) — tech stack details, conventions.
3. **Writes Tier A command files** — `/build`, `/jc`, `/pcm`, `/dev`, `/git`, `/wave`, `/documenter`, `/council`, `/audit`. Voice intact, domain content filled.
4. **Writes Tier B command files** for each opt-in — `/officer`, `/km`, `/pm`, `/mentor`, `/marketer`. Archetype skeletons with your placeholders filled. The leading `>`-quoted "Required placeholders (fill at install)" meta-block from each template is stripped before save — that block is install-time scaffolding, not runtime content. A correctly-installed Tier B command starts with the H1 heading and goes straight to the `$ARGUMENTS` line.
5. **Writes root agents** — `gitter`, `mono-planner`, `mono-architect`, `mono-documenter` with your project list pinned.
6. **Writes per-project agents** (if monorepo) — `planner`, `architect`, `developer`, `qa` per project, with your test/lint/build commands pinned.
7. **Writes scripts** — `worktree.sh`, `alloc-ports.sh`, `dev.sh`, `notify.sh` with your tech stack's setup logic and port ranges.
   7a. **Copies the Cast bible** — `blueprint/ARCHETYPES.md` lands at `.claude/ARCHETYPES.md` verbatim, so future `/pcm`, `/council`, and `/wave` work has one canonical reference for who's who and what voice each archetype carries.
   7b. **Installs skills** — clones skills from their public repos into `.claude/skills/{name}/`. These are universal thinking protocols (Tier A) maintained as standalone repos. The installer clones each, parameterizes where needed (360°'s stakeholder names from Batch 5), and removes the `.git/` directory so they're plain files in your project.

| Skill         | Repo                                         | Parameterization                                                     |
| ------------- | -------------------------------------------- | -------------------------------------------------------------------- |
| `rr`          | https://github.com/mreza0100/rr              | None                                                                 |
| `360`         | https://github.com/mreza0100/360             | Replace `{USER_PERSONA}` and `{SECONDARY_PERSONA}` in inquiry domain |
| `ghostwriter` | https://github.com/mreza0100/ghost-writer    | None                                                                 |
| `rnd`         | Bundled in `blueprint/templates/skills/rnd/` | None                                                                 |

7c. **Installs statusline** — copies `statusline-command.sh` to `~/.claude/statusline-command.sh` and adds the statusline config block to `~/.claude/settings.json`. Two-line status bar with model, context, git, cost, rate limits. Requires `jq`.
7d. **Configures notifications** — `notify.sh` hooks into Claude Code's `PreToolUse` and `Stop` events via `~/.claude/settings.json` hooks. Sends a macOS notification when a turn takes 30+ seconds. Character name is parameterized from Batch 7.
7e. **Configures markdown auto-formatter** — `format-md.sh` hooks into Claude Code's `PostToolUse` event for `Edit` and `Write` tools. When Claude edits a Professor-owned `.md` file (CLAUDE.md, `.claude/`, `docs/commands/`, `docs/agents/`, `docs/epics/`, `docs/dev/`, `docs/business/`, or child project CLAUDE.md files), prettier auto-formats it. Non-Professor files are ignored. Add to `.claude/settings.json`:

    ```json
    {
      "hooks": {
        "PostToolUse": [
          {
            "matcher": "Edit|Write",
            "hooks": [
              {
                "type": "command",
                "command": "bash .claude/scripts/format-md.sh"
              }
            ]
          }
        ]
      }
    }
    ```

    Requires `jq` and `prettier` (`npx prettier` — works if prettier is a project devDependency or globally installed). Fails silently if either is missing. 8. **Creates directory structure** — `docs/agents/`, `docs/commands/`, `docs/dev/tasks/`, `docs/dev/tasks/archive/`, `docs/dev/waves/`, `.worktrees/` (gitignored).

8b. **(If Codex opted in)** Creates `.codex/` layer — `config.toml`, `.toml` agent wrappers pointing to `.claude/commands/*.md` and `.claude/agents/*.md`, skill wrappers mirroring commands. Creates `AGENTS.md` symlink → `CLAUDE.md`. If Codex was NOT opted in, this step is skipped entirely. 9. **Updates `.gitignore`** — adds `.worktrees/`, `tmp/`. 10. **Creates `.professor/` directory** — Professor's own state at the repo root. Contains `VERSION` (installed version), `manifest.json` (machine-readable replay seed + file hashes), and `decisions.md` (human-readable record of what's different from vanilla Professor). 11. **Writes `.professor/VERSION`** — the blueprint version tag installed from. 12. **Writes `.professor/manifest.json`** — generates `.professor/manifest.json` containing (a) the blueprint version installed from, (b) ALL interview answers as a replay seed, and (c) SHA-256 hashes of every installed file post-substitution. This manifest is what `/pcm update` uses for three-way comparison (installed baseline vs current on-disk vs re-parameterized upstream) and for replaying interview answers against new template versions. Format:
`json
    {
      "schema": 1,
      "version": "0.5.0",
      "installed_from_tag": "v0.5.0",
      "installed_at": "2026-04-28T14:32:00Z",
      "updated_at": null,
      "interview": {
        "project_name": "neurolab",
        "project_pitch": "AI-assisted neuropsychological assessment platform",
        "character_name": "Professor",
        "character_voice": "keep",
        "sacred_ground": "patient cognitive assessment data and diagnostic accuracy",
        "structure": "monorepo",
        "subprojects": [
          { "dir": "api", "desc": "Express GraphQL backend", "pkg": "pnpm" },
          { "dir": "web", "desc": "React frontend", "pkg": "npm" }
        ],
        "tech_commands": {
          "api": { "test": "pnpm test", "lint": "pnpm lint", "typecheck": "pnpm tsc --noEmit", "build": "pnpm build", "dev": "pnpm dev" },
          "web": { "test": "npm test", "lint": "npm run lint", "typecheck": "skip", "build": "npm run build", "dev": "npm run dev" }
        },
        "disciplines": ["Neuroscience", "Cognitive Science", "Computational Modeling", "Statistics", "Clinical Methodology", "Software Engineering", "Information Theory", "Linguistics", "Philosophy of Mind", "Research Methods"],
        "intersection_lens": "Neuroscience × Computational Modeling",
        "council_panel": ["Officer", "PM", "Mentor"],
        "tier_b": {
          "officer": { "enabled": true, "regulation": "HIPAA", "authority": "HHS OCR", "rights": "HIPAA Privacy Rule", "notification": "60 days" },
          "km": { "enabled": false },
          "pm": { "enabled": true, "persona": "clinical neuropsychologist", "domain": "cognitive assessment", "workflow": "patient intake → battery selection → administration → scoring → report", "pain_points": "manual scoring, report writing time" },
          "mentor": { "enabled": true, "market": "clinical neuropsych SaaS", "jurisdiction": "US", "entity": "LLC", "funding": "NIH SBIR, health-tech VCs", "bodies": "FDA (if SaMD), state licensing boards" },
          "marketer": { "enabled": false }
        },
        "codex": false,
        "ports": { "api": 3000, "web": 5173, "db": 5432 }
      },
      "files": {
        "CLAUDE.md": "sha256:fa7b1ba7e0f3...",
        ".claude/commands/jc.md": "sha256:e3b0c44298fc...",
        ".claude/commands/pcm.md": "sha256:2c26b46b68ff..."
      }
    }
    `
The `interview` field is the replay seed — `/pcm update` re-applies these answers to new upstream templates, then compares hashes to detect conflicts vs safe auto-applies. The `files` field is SHA-256 of every installed file AFTER placeholder substitution (a mismatch means the user edited post-install). The `installed_from_tag` records which git tag was used, enabling `/pcm update` to `git clone --branch` the exact version for diffing.

---

## Phase 3 — Smoke test

After install, Claude runs a tiny `/build` to verify the pipeline works end-to-end:

```
/build add-readme-section
```

Walk through the prompts. The first run reveals anything missed in adaptation. If something asks the wrong question or runs the wrong command, invoke `/pcm` to fix it at the source.

---

## What if I want to add a Tier B archetype later?

You can opt in any Tier B archetype after install:

```
claude
> Add /officer to my Professor install. We're now subject to {REGULATION}.
```

Claude reads the blueprint's Tier B template for that archetype, runs the relevant subset of the interview, and copies + customizes the file. No reinstall needed.

Same for adding a new Tier A archetype if you build one — `/pcm` copies the template, you parameterize the content, done.

---

## Common gotchas

1. **Worktree script can't find your tools.** Make sure your shell environment is loaded inside the script — `source ~/.zshrc`, use absolute paths, or pin tool versions in a script-local `PATH`.
2. **Port allocation false positives.** `lsof -i :PORT` checks aren't always reliable across IPv4/IPv6 — adjust the script if you see false positives on your OS.
3. **Gitter tries to merge with conflicts unresolved.** That's a gap in your gitter setup; the template handles it, but if you simplified, restore the conflict-detection block.
4. **Agents writing to permanent docs.** Only `mono-documenter` should write to `docs/agents/` or `{project}/docs/`. If another agent tries, that's a `/pcm` fix at the source agent.
5. **`.worktrees/.ports` corrupted.** Manually edit; the format is one whitespace-separated line per pipeline.
6. **Character feels generic after install.** You probably stripped voice instead of parameterizing content. Re-read `ADAPTATION.md` § "What NOT to change" — voice is non-negotiable. Invoke `/pcm` and tell it which command lost its voice.

---

## After install

- Read `ARCHETYPES.md` so you know the cast you just installed.
- Read `BLUEPRINT.md` § "The five load-bearing walls" — these don't change, ever.
- Verify the statusline shows in your terminal (you should see model, context %, git branch). If not, check `~/.claude/settings.json` has the statusLine config and `jq` is installed.
- Verify notifications work — start a task that takes 30+ seconds and check you get the macOS notification when the turn completes.
- Run `/build` for new features. Run `/jc` for hotfixes. Run `/pcm` to evolve the pipeline. Run `/council` for hard decisions. Run the Professor analysis for cross-disciplinary analysis.
- The pipeline is supposed to evolve. Static configurations rot — evolving ones get sharper with use.

Welcome to the cast.

---

## Staying current — `/pcm update`

When new versions of Professor are released (as git tags on `mreza0100/professor`), your install can pull updates without losing customizations:

```
/pcm update              # Full interactive update to latest release tag
/pcm update check        # Read-only — preview what would change
/pcm update --to v1.2.0  # Pin to a specific version tag
/pcm update --force      # Re-apply manifest (repair mode)
/pcm update --re-interview 5  # Re-answer interview question 5
```

### How it works

1. Reads `.professor/VERSION` + `.professor/manifest.json` (your installed version, interview answers, file hashes)
2. Fetches available git tags from `mreza0100/professor` via `git ls-remote`
3. Clones the target tag into temp, reads `CHANGELOG.md` entries between your version and target
4. **Replays your interview answers** against new templates → computes re-parameterized upstream hashes
5. **Three-way hash comparison** per file (installed baseline vs current on-disk vs upstream new):
   - Upstream changed + you didn't touch → **auto-apply**
   - You customized + upstream didn't change → **keep yours**
   - Both changed → **conflict** — shows diff, you decide
   - New file from upstream → **auto-add** (mechanics) or **ask** (Tier A/B)
6. Presents changes in three buckets: auto-apply, review, manual
7. Applies accepted changes, regenerates manifest with new hashes + updated version
8. Appends to `.professor/decisions.md` — records which files you kept over upstream, new opt-ins, re-interview changes

### Version semantics

Releases follow semver via git tags (`v0.5.0`, `v0.6.0`, `v1.0.0`):

| Bump      | What it means for you                                  |
| --------- | ------------------------------------------------------ |
| **Patch** | Bug fixes, doc tweaks — mostly auto-apply              |
| **Minor** | New features/commands — mix of auto + interactive      |
| **Major** | Breaking changes — full walkthrough, no silent applies |

### What it never touches without asking

- Your `CLAUDE.md` persona section (character voice may have drifted intentionally)
- Files under `docs/commands/{cmd}/` (command-owned content, not templates)
- `.claude/settings.json` (hand-curated per project)
- Any file you've customized post-install (detected via hash mismatch)

See `RELEASE.md` for how releases are produced. See `pcm.md` § "Update Protocol" for the full implementation.
