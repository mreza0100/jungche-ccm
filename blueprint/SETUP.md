# SETUP — Installing Jungche

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
# Clone the blueprint somewhere
git clone https://github.com/mreza0100/jungche-ccm.git ~/work/jungche-ccm

# Inside YOUR project
cd ~/path/to/your-project
claude
> Read every file in ~/work/jungche-ccm/blueprint/.
> Follow SETUP.md to install Jungche in THIS project.
> Conduct the interview before touching any files.
```

Claude runs Phase 1 (interview), then Phase 2 (customization), then Phase 3 (smoke test). You answer about 10 questions. Claude does the rest.

**The manual path:** read `BLUEPRINT.md`, `ARCHETYPES.md`, `ADAPTATION.md`, copy templates manually, replace placeholders by hand. Slower but doable.

---

## Phase 1 — The interview

Claude (in your target project) asks these questions, in this order. Answer them however you want — short, long, with examples. Claude will turn them into template parameters.

### 1. Project identity

> What does your project do, in one sentence?

This becomes `{PROJECT_NAME}` and `{PROJECT_PITCH}`. Example: "Freudche is an AI clinical administrative tool that listens to therapy sessions and assists the therapist."

### 2. Character name & voice

> Default character is **Jungche** — Dr. House senior engineer. Sarcastic, witty, blunt-but-helpful, emoji-fluent. Ships first, jokes second. Keep that voice, or give your character a new name + signature trait?

Most adopters keep Jungche as-is. The voice transplants well across domains. If you want a different name (e.g., "Beatrix" for a finance project, "Gandalf" for an open-source library), name it. The voice can stay.

### 3. Project structure

> Single project, or monorepo? If monorepo, how many subprojects, and what does each do?

For each subproject, Claude needs:
- Directory name (you choose)
- One-line description
- Tech: language, framework, package manager, test runner, build tool, dev server port

Example (Freudche):
- `freudche-be` — Express + GraphQL backend, pnpm, vitest, port 3000
- `freudche-fe` — Expo React Native frontend, npm, jest, port 8081
- `freudche-cortex` — Python AI engine, uv, pytest, no port (SQS consumer)
- `freudche-infra` — Docker Compose for PostgreSQL + LocalStack
- `freudche-web` — Next.js marketing site, npm, Vercel, port 3001

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

| Project type | 10 disciplines |
|--------------|----------------|
| Therapy AI | CS, Clinical Psych, AI/ML, HCI, Statistics, Linguistics, Privacy/Security, UX, Software Architecture, Therapy Methodology |
| Neuropsych research | Neuroscience, Cognitive Science, Computational Modeling, Statistics, Clinical Methodology, Software Engineering, Information Theory, Linguistics, Philosophy of Mind, Research Methods |
| Game studio | Game Design, Narrative Theory, Probability, Behavioral Economics, UX, Mathematics, Art Direction, Audio Design, Software Engineering, Player Psychology |
| FinTech trading | Financial Engineering, Statistics, ML, Distributed Systems, Securities Law, Game Theory, Microeconomics, Software Engineering, Cybersecurity, Behavioral Finance |
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

#### `/ckm` — knowledge curator

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

### 8. Sacred ground

> What does "do no harm" mean in your domain? Privacy, safety, correctness, financial integrity, narrative coherence, scientific reproducibility, security?

This becomes `{SACRED_GROUND}` and is referenced by:
- Jungche (the "don't joke about X" rule)
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

1. **Writes root `CLAUDE.md`** — fills in `{PROJECT_NAME}`, `{PROJECT_PITCH}`, the Jungche persona section, the project structure tree, the non-negotiable rules. Strict-mode rules adapted to your stack.
2. **Writes per-project `CLAUDE.md` files** (if monorepo) — tech stack details, conventions.
3. **Writes Tier A command files** — `/build`, `/jc`, `/ccm`, `/dev`, `/git`, `/wave`, `/documenter`, `/professor`, `/council`, `/ca`. Voice intact, domain content filled.
4. **Writes Tier B command files** for each opt-in — `/officer`, `/ckm`, `/pm`, `/mentor`, `/marketer`. Archetype skeletons with your placeholders filled.
5. **Writes root agents** — `gitter`, `mono-planner`, `mono-architect`, `mono-documenter` with your project list pinned.
6. **Writes per-project agents** (if monorepo) — `planner`, `architect`, `developer`, `qa` per project, with your test/lint/build commands pinned.
7. **Writes scripts** — `worktree.sh`, `alloc-ports.sh`, `dev.sh` with your tech stack's setup logic and port ranges.
8. **Creates directory structure** — `docs/agents/`, `docs/commands/`, `docs/dev/tasks/`, `docs/dev/tasks/archive/`, `docs/dev/waves/`, `.worktrees/` (gitignored).
9. **Updates `.gitignore`** — adds `.worktrees/`, `tmp/`.
10. **Records install version** — writes the blueprint's current `VERSION` to `.claude/JUNGCHE_VERSION`. This is what `/ccm update` reads later to determine which CHANGELOG entries apply when pulling future updates.
11. **Writes install manifest** — generates `.claude/JUNGCHE_MANIFEST.json` mapping every installed blueprint-derived file (CLAUDE.md, agents, commands, scripts) to its SHA-256 hash AS COPIED — i.e., after placeholders were filled but before the user has touched anything. This is the baseline `/ccm update` uses to detect which files the user has since customized vs. which are still pristine. Format:
    ```json
    {
      "version": "1.0.0",
      "installed_at": "2026-04-28T14:32:00Z",
      "files": {
        ".claude/commands/jc.md": "sha256:e3b0c44298fc...",
        ".claude/commands/professor.md": "sha256:2c26b46b68ff...",
        "CLAUDE.md": "sha256:fa7b1ba7e0f3..."
      }
    }
    ```
    Hashes are computed AFTER placeholder substitution so they reflect the actual on-disk state — a hash mismatch later means the user (or another agent) edited the file post-install.

---

## Phase 3 — Smoke test

After install, Claude runs a tiny `/build` to verify the pipeline works end-to-end:

```
/build add-readme-section
```

Walk through the prompts. The first run reveals anything missed in adaptation. If something asks the wrong question or runs the wrong command, invoke `/ccm` to fix it at the source.

---

## What if I want to add a Tier B archetype later?

You can opt in any Tier B archetype after install:

```
claude
> Add /officer to my Jungche install. We're now subject to {REGULATION}.
```

Claude reads the blueprint's Tier B template for that archetype, runs the relevant subset of the interview, and copies + customizes the file. No reinstall needed.

Same for adding a new Tier A archetype if you build one — `/ccm` copies the template, you parameterize the content, done.

---

## Common gotchas

1. **Worktree script can't find your tools.** Make sure your shell environment is loaded inside the script — `source ~/.zshrc`, use absolute paths, or pin tool versions in a script-local `PATH`.
2. **Port allocation false positives.** `lsof -i :PORT` checks aren't always reliable across IPv4/IPv6 — adjust the script if you see false positives on your OS.
3. **Gitter tries to merge with conflicts unresolved.** That's a gap in your gitter setup; the template handles it, but if you simplified, restore the conflict-detection block.
4. **Agents writing to permanent docs.** Only `mono-documenter` should write to `docs/agents/` or `{project}/docs/`. If another agent tries, that's a `/ccm` fix at the source agent.
5. **`.worktrees/.ports` corrupted.** Manually edit; the format is one whitespace-separated line per pipeline.
6. **Character feels generic after install.** You probably stripped voice instead of parameterizing content. Re-read `ADAPTATION.md` § "What NOT to change" — voice is non-negotiable. Invoke `/ccm` and tell it which command lost its voice.

---

## After install

- Read `ARCHETYPES.md` so you know the cast you just installed.
- Read `BLUEPRINT.md` § "The five load-bearing walls" — these don't change, ever.
- Run `/build` for new features. Run `/jc` for hotfixes. Run `/ccm` to evolve the pipeline. Run `/council` for hard decisions. Run `/professor` for cross-disciplinary analysis.
- The pipeline is supposed to evolve. Static configurations rot — evolving ones get sharper with use.

Welcome to the cast.

---

## Staying current — `/ccm update`

When new versions of Jungche are released, your install can pull updates without losing customizations:

```
/ccm update            # Walk through changes interactively
/ccm update check      # Read-only — preview what would change
/ccm update --to v1.2.0  # Pin to a specific version
```

How it works:
1. Reads `.claude/JUNGCHE_VERSION` (your current install)
2. Fetches the latest blueprint from `mreza0100/jungche-ccm`
3. Reads `CHANGELOG.md` entries between your version and the latest
4. Walks you through each change — auto-applying mechanics, asking before character changes, opt-in for new Tier B archetypes, interactive walkthrough for breaking migrations
5. Updates `.claude/JUNGCHE_VERSION` to the new version

See `RELEASE.md` in the blueprint repo for how releases are produced and what each change category means.
