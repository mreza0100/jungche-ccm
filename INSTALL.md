# INSTALL — Interactive setup for Jungche

> This document is **instructions for Claude Code**, not for you to execute manually. To install Jungche on your project, open Claude Code in that project and paste:
>
> ```
> Read https://raw.githubusercontent.com/mreza0100/jungche/main/INSTALL.md (or your local clone) and walk me through the interactive install. Ask me each section's questions one at a time and wait for my answers before proceeding. Do not assume — confirm everything.
> ```

If you (Claude) are reading this: **you are the installer**. The user's input is the source of truth — never invent stack details, names, or domain assumptions. Ask the questions in this file, batch them in groups, wait for replies, then customize.

---

## Why interactive

A blueprint pasted blindly is a museum exhibit. A blueprint shaped to the user's stack is a working pipeline. The same five load-bearing walls hold (only-gitter-touches-git, QA-gates, path variables, worktree isolation, self-improvement at the source) — but every other surface needs to fit the user's project.

This installer's job: **ask, don't assume**.

---

## Pre-flight (Claude does this silently before asking the user anything)

1. Confirm we are in a git repository (`git rev-parse --is-inside-work-tree`). If NOT a git repo, ask the user whether to `git init` first (recommended — gives `git mv` history-preservation for any re-homing later) or proceed without git. Don't `git init` silently.
2. Confirm `tmp/` and `.worktrees/` are absent or already gitignored. If present and ungitignored, flag it.
3. Detect existing `CLAUDE.md` and `.claude/` — if present, **STOP** and ask the user whether to overwrite, merge, or abort.
4. Run `git status` and warn if uncommitted work exists. Don't proceed without acknowledgment.
5. Detect package manager hints (`pnpm-lock.yaml`, `package-lock.json`, `yarn.lock`, `uv.lock`, `Cargo.toml`, `go.mod`, `requirements.txt`, `pyproject.toml`).
6. Detect monorepo hints (multiple top-level dirs each with their own `package.json` / `pyproject.toml` / `Cargo.toml`).
7. **Detect existing project documentation** — surface non-blueprint markdown that looks like research/strategy/onboarding artifacts. Run:
   ```bash
   find . -maxdepth 2 -type f -name "*.md" \
     ! -name "README.md" ! -name "LICENSE*" ! -name "CHANGELOG*" ! -name "CONTRIBUTING*" \
     ! -path "./node_modules/*" ! -path "./.git/*" ! -path "./.claude/*"
   ```
   These are existing material that needs re-homing into the Jungche taxonomy. **Do NOT classify yet** — classification depends on which Tier B archetypes the user opts into (Batch 6). Just list them in the findings paragraph.

Report findings in one short paragraph BEFORE asking questions, e.g.:
> "I see this is a Node + Python monorepo with `api/` (pnpm) and `worker/` (uv). No existing `.claude/` setup. Working tree is clean. I also see 17 root-level markdown files (THESIS, MENTOR_BRIEFING, COMPETITOR_LANDSCAPE, REGULATORY_LANDSCAPE, etc.) — those will need re-homing once we settle which Tier B archetypes you want. Ready to ask questions."

---

## Question batches

Ask in numbered batches. Wait for the user's reply between batches. Never ask all at once — the user will not answer 20 questions in a single message.

### Batch 1 — Project identity (3 questions)

```
1. What is the project name? (used in CLAUDE.md, public commands, banner text)

2. One sentence — what does this project DO? (this becomes the project pitch in CLAUDE.md)

3. Mission / north star — what does success look like? (one line, replaces "The GOAL" section)
```

### Batch 2 — Structure (4 questions)

```
4. Is this a single project or a monorepo?
   - "single" → one .claude/agents/ at root, no child CLAUDE.md
   - "monorepo" → continue to question 5

5. List your subprojects (directory + one-line description each). Format:
   - api/ — Node.js GraphQL backend
   - web/ — Next.js frontend
   - worker/ — Python ETL service

6. For each subproject, what is the package manager?
   (pnpm / npm / yarn / uv / poetry / cargo / go / mix / etc.)

7. Are there obvious cross-project communication boundaries already?
   (REST? GraphQL? gRPC? message queue? shared types? shared DB?)
   This shapes mono-architect's job.
```

### Batch 3 — Test & build commands (one row per subproject)

```
For EACH subproject from question 5, give me:

- test command       (e.g., `pnpm test`)
- lint command       (e.g., `pnpm lint`, `ruff check`)
- typecheck command  (e.g., `pnpm tsc --noEmit`, `mypy .`)
- build command      (e.g., `pnpm build`, `cargo build`)
- dev command        (e.g., `pnpm dev`)

If a command doesn't exist for a project (e.g., no separate typecheck), say "skip".
```

### Batch 4 — Ports

```
8. Which ports does main currently use for dev?
   (e.g., backend 3000, frontend 5173, db 5432)
   I'll allocate worktree ranges starting BASE+1 (so backend worktrees get 3001–3099, etc.).

9. Any ports I should AVOID? (other tools, system services, conflicts you've hit)
```

### Batch 5 — Domain & disciplines (this drives /professor customization)

```
10. What's the project's domain in one phrase?
    (e.g., "B2B SaaS for legal firms", "consumer mobile game", "internal data platform",
     "developer tooling", "fintech payments", "clinical AI assistant")

11. /professor is your cross-disciplinary system analyst. The default version pairs Computer
    Science with one or more domain disciplines. For YOUR project, which disciplines should
    /professor draw on? Pick 1–3 from this menu, or name your own:

    - Psychology (clinical, behavioral, UX)
    - Medicine (clinical safety, diagnosis, regulatory)
    - Finance (markets, risk, regulatory)
    - Law (compliance, privacy, contracts)
    - Game design (mechanics, balance, retention)
    - Education (pedagogy, assessment, accessibility)
    - Linguistics (NLP, i18n, content)
    - Cryptography (protocols, primitives, threat models)
    - Distributed systems (consensus, replication, partitioning)
    - Operations research (optimization, scheduling, queueing)
    - Bioinformatics
    - Music / audio
    - Other: ___

12. What are the FAILURE modes /professor should specifically watch for?
    (e.g., "data leakage between tenants", "race conditions during checkout",
     "hallucinated medical advice", "regression in player engagement")
    These become the dimensions /professor scores against.

13. Read-only or also-suggests-changes? Default is read-only — /professor produces analysis,
    not code. Override if you want.
```

### Batch 6 — Optional commands

```
14. Beyond the core Tier A + C (/build, /jc, /jm, /dev, /git, /wave, /documenter,
    /professor, /council, /ca), which Tier B archetypes do you want? Pick from the menu,
    request your own, or skip:

    - /officer — privacy/compliance auditor (GDPR, HIPAA, SOC2, FDA, ISO 27001, MiFID, etc.)
    - /mentor — startup/business advisor
    - /marketer — visibility & growth strategist
    - /pm — user+product-manager hybrid (or your own user-persona variant)
    - /km — domain knowledge curator (for projects with a knowledge base)
    - your own: ___ — describe purpose, scope, owns-which-docs

    For each you pick, I will need: scope, owned doc paths under $CDOCS, and one-line purpose.

15b. Do you also use OpenAI Codex? (OPTIONAL — everything works without it)
     If yes: I'll create a `.codex/` layer that mirrors `.claude/` — same command manuals,
     wrapped in .toml files for Codex's runtime. Also creates an `AGENTS.md` symlink to
     `CLAUDE.md`. Division of labor: Claude orchestrates + QA, Codex implements (cheaper tokens).
     If no: the Codex layer is skipped entirely. No impact on any pipeline operation.
```

### Batch 7 — Character (MANDATORY — choose name + sacred-ground)

```
15. Your orchestrator persona — Jungche by default (sarcastic, witty, blunt-but-helpful Dr.
    House senior engineer; emoji-fluent; ships first, jokes second). The voice is universal
    and load-bearing — it ships with every Jungche install. You CANNOT skip it. You can:

    - "keep Jungche" → keep the name + voice as-is, I'll just adapt the
      "what NOT to joke about" bullet to your domain
    - "rename" → keep the voice, change the name (e.g., "Beatrix" for fintech,
      "Gandalf" for an OSS library) — give me the new name
    - "custom voice" → keep the structure but reshape the personality (give me 3–6
      tone keywords + a one-line "vibe"); the persona section MUST still be written

16. What is your project's SACRED GROUND — the topics where the character
    drops the humor and reports flat? (e.g., "patient data + clinical safety",
    "user funds + financial integrity", "PII + privacy", "physical safety
    in autonomous control"). This goes into the "What NOT to do" block.
```

> **Why this is mandatory:** the blueprint philosophy treats character as load-bearing infrastructure. Strip the persona section and Claude defaults to vanilla assistant tone in every interactive turn while `/jc`, `/professor`, and `/council` keep their voices — producing tonal whiplash. Tier A characters ship with full voice. Adopters can rename freely (Hard Rule 4 is not "ask permission to give it character" — it's "don't import Freudche-specific *content* like therapy/clinical references"). Domain content gets parameterized; the orchestrator persona always lands.

### Batch 8 — Confirmation before write

```
Before I touch any file, I'll show you:
- The directory layout I'll create
- The list of files I'll write (count + paths)
- The customized /professor frontmatter
- The customized /ca scope table
- **Proposed re-home moves for existing project docs** (one row per file: source → destination, with the classification reason)
- **Files I cannot classify** (will ask you per-file or default to docs/dev/research/)
- Any questions where I'm still uncertain

Type "go" to proceed, or correct anything that's wrong (e.g., "move 03_THE_PAIN.md to docs/business/ instead").
```

---

## After confirmation — execution order

Do these IN THIS ORDER. Each step depends on the previous.

### Step 1 — Scaffolding

```bash
mkdir -p .claude/{agents,commands,scripts}
mkdir -p docs/{agents,commands,dev/tasks/archive,dev/waves,dev/research}
echo -e ".worktrees/\ntmp/" >> .gitignore
```

For each Tier B archetype the user opted into in Batch 6, also create its `$CDOCS` subtree:
```bash
# Example for /mentor opted in:
mkdir -p docs/commands/mentor/{references,research,resources}
# Repeat for /officer, /pm, /marketer, /km, etc. — only the ones picked.
```

### Step 1.5 — Re-home existing project docs

For every file surfaced in Pre-flight Step 7, classify and move into the Jungche taxonomy. **Do NOT skip this — leaving research docs at the root means they get ignored by every command and the project loses context.**

#### Classification rubric

Apply rules in order. First match wins. Match BOTH on filename hints AND on a quick content scan (first 500 chars + headings).

| Content signature | Destination | Notes |
|---|---|---|
| Names matching `THESIS`, `VISION`, `MISSION`, `STRATEGY`, `PARALLEL_PROJECTS`, `PRODUCT_VISION` | `docs/business/<slugified-name>.md` | Lowercase + hyphenate the name |
| Names matching `GLOSSARY`, `TERMS`, `DICTIONARY` | `docs/business/glossary.md` | One canonical glossary per project |
| Names matching `BUYER`, `MARKET`, `GTM`, `GO_TO_MARKET`, `BUSINESS_MODEL`, `PRIMER` (domain primer), `*MENTOR*`, `RISK*`, `INTERNATIONAL_*`, `FUNDING` | `$CDOCS/mentor/$REFS/<slug>.md` (if living must-know) or `$CDOCS/mentor/$RESEARCH/<slug>.md` (if looked-up analysis) | Only if `/mentor` opted in. If not, see fallback below. |
| Names matching `COMPETITOR`, `INCUMBENT`, `POSITION`, `SEO`, `CHANNEL`, `CONTENT_GAP`, `BRAND_VOICE` | `$CDOCS/marketer/$REFS/` or `$CDOCS/marketer/$RESEARCH/` | Only if `/marketer` opted in |
| Names matching `REGULATORY`, `COMPLIANCE`, `GDPR`, `HIPAA`, `FDA`, `PRIVACY`, `LEGAL_LANDSCAPE`, `CERTIFICATION` | `$CDOCS/officer/$REFS/` or `$CDOCS/officer/$RESEARCH/` | Only if `/officer` opted in |
| Names matching `PERSONA`, `USER_*`, `*_PAIN`, `*_PAIN_MAP`, `JOBS_TO_BE_DONE`, `USER_STORY`, `WORKFLOW`, `DAILY_LIFE` | `$CDOCS/pm/$REFS/` or `$CDOCS/pm/$RESEARCH/` | Only if `/pm` opted in |
| Names matching `KNOWLEDGE`, `DOMAIN_PRIMER`, `PROTOCOL`, `METHODOLOGY`, `FRAMEWORK_<domain>` | `$CDOCS/km/$RESEARCH/` | Only if `/km` opted in |
| Names matching `RESEARCH_LOG`, `OPEN_QUESTIONS`, `VALIDATION_LOG`, `EXPERIMENTS`, `SPIKE_*`, `INVESTIGATION` | `docs/dev/research/<slug>.md` | Always available — no archetype required |
| Names matching `MENTOR_BRIEFING`, `INVESTOR_*`, `PITCH`, `ONE_PAGER`, `FOUNDER_STORY` | `$CDOCS/mentor/$REFS/<slug>.md` (if `/mentor`) OR `docs/business/<slug>.md` (fallback) | These are the "show to outsider" docs |
| `README.md`, `LICENSE*`, `CHANGELOG*`, `CONTRIBUTING*`, `CODE_OF_CONDUCT*` | **KEEP AT ROOT — DO NOT MOVE** | Standard repo conventions |
| Anything else (no filename match, ambiguous content) | **ASK THE USER** before moving | Default proposal: `docs/dev/research/<slug>.md` |

#### `$REFS` vs `$RESEARCH` decision

Within an archetype's `$CDOCS/<cmd>/` directory:
- **`$REFS`** = living must-know, loaded almost every invocation (regulatory framework, persona, GTM plan, briefing, primer)
- **`$RESEARCH`** = looked-up analysis loaded on demand (competitor scan, risk deep-dive, market study)
- **`$RESOURCES`** (some archetypes) = static assets loaded almost every time (playbook, templates)

If the doc reads like "the rules / the canon / what every advisor needs to know," it's `$REFS`. If it reads like "I went and looked into X and here's what I found," it's `$RESEARCH`.

#### Fallback when archetype not opted in

If a file matches an archetype the user did NOT pick (e.g., `REGULATORY_LANDSCAPE.md` but no `/officer`), do this:

1. **Ask the user** if they want to opt into that archetype now — the file's existence suggests they need it. Run a quick mini-interview if yes.
2. **If still no**, place at `docs/dev/research/<slug>.md` and **flag in the final report** that the file wasn't ideally homed: "REGULATORY_LANDSCAPE.md is in `docs/dev/research/` because you skipped `/officer` — re-run install with `/officer` opted in to move it to `$CDOCS/officer/$REFS/`."

#### Execution

For each classified file:

```bash
# If git repo (preferred — preserves history):
git mv <source> <destination>

# If NOT a git repo:
mv <source> <destination>
```

**Never** `cp` + delete — always `mv` so the file isn't accidentally duplicated.

After moves are staged, do NOT commit — that's the user's call (per Hard Rule 5). Just leave the renames staged so `git status` shows the plan and the user can review before committing.

#### What about subdirectories under `docs/` that already exist?

If the user already has `docs/research/` or `docs/strategy/` (different from Jungche's `docs/dev/research/` and `docs/business/`), classify each file inside and propose re-homing into the Jungche structure. Don't preserve the user's old taxonomy if it conflicts with Jungche's — Jungche has one canonical layout per command's $CDOCS. But ASK before moving if uncertain.

### Step 2 — CLAUDE.md (root)

Copy `blueprint/templates/CLAUDE.md` and substitute every `{PLACEHOLDER}` from Batches 1–7.

If the user picked monorepo: include the per-project tables. If single project: drop them.

**The "## Your character" persona section is MANDATORY — you must write it.** From Batch 7:
- If the user said "keep Jungche": keep the section verbatim, only adapting `{SACRED_GROUND}` (Batch 7 Q16), `{WHAT_THE_PROJECT_BUILDS}`, and `{YOUR_LANGUAGE}` placeholders to their domain. The "What NOT to do" first bullet must reference their sacred ground.
- If the user said "rename": same as above, plus replace every "Jungche" with the new name. Keep the voice description verbatim.
- If the user said "custom voice": keep the section's *shape* (heading, "MANDATORY" framing, "Core personality traits" bulleted list, "What NOT to do" block) but reshape the bullets using their tone keywords + vibe line. NEVER ship a CLAUDE.md without the persona section.

After writing, verify: the file MUST contain a `## Your character — {NAME} (MANDATORY` heading. If it doesn't, you skipped a step — go back and write it.

Also remove these install-only meta-comments from the body of CLAUDE.md before saving:
- The `> **Rename if you want.**` admonition that prefaces the persona section in the template (it's an install instruction, not a runtime instruction).
- Any `{INSTRUCTIONAL_COMMENT}` blocks in `< >` braces inside the template body.

### Step 3 — Per-project CLAUDE.md (if monorepo)

For each subproject, write `{project}/CLAUDE.md` with:
- Tech stack (from Batch 2/3)
- Test/lint/typecheck/build commands (from Batch 3)
- Project-specific rules (start with the universal ones, ask the user if they want to add more)

### Step 4 — Agents

Copy `blueprint/templates/agents/` into `.claude/agents/` (root level).

If monorepo: also copy `planner.md`, `architect.md`, `developer.md`, `qa.md` into each `{project}/.claude/agents/` and substitute tech-stack placeholders per project.

If single project: only the root `.claude/agents/` set is needed. Skip `mono-planner.md` and `mono-architect.md` — they're only useful when there are cross-project contracts to consolidate. The orchestrator goes straight `planner → architect → developer → qa`.

### Step 5 — Customize /professor

Write `.claude/commands/professor.md` based on Batch 5 answers. Use the **Professor template** below — replace placeholders with the user's answers.

### Step 6 — Copy core commands

Copy from `blueprint/templates/commands/`:
- `build.md` — substitute project list
- `jc.md` — substitute project list
- `jm.md` — substitute project list
- `dev.md` — substitute per-project start/stop blocks

Add the `git` command (gitter gateway — see template).

### Step 7 — Tier B commands (the user's opt-ins from Batch 6)

For each Tier B command the user picked, copy `blueprint/templates/commands/{cmd}.md` to `.claude/commands/{cmd}.md`, then **substitute placeholders AND strip the install-only meta-block**.

The meta-block is the leading `>`-quoted block that begins with `**Tier B — Domain archetype.**` and ends with `**Skip if:** ...`. It exists in the template ONLY to brief you (the installer) on which placeholders need filling. It is install-time scaffolding, not runtime content. **DELETE IT IN THE EMITTED FILE.** A correctly-installed Tier B command starts with the H1 heading (e.g., `# Officer — Compliance & Privacy`) and goes straight to the `Handle this request: $ARGUMENTS` line — no leading `>` block, no "fill at install" bullets.

For each command:

1. Read the template at `blueprint/templates/commands/{cmd}.md`.
2. Identify every backtick-quoted Freudche-domain placeholder in the body (e.g., `\`patient medical data\``, `\`Dutch GGZ market\``) and replace with the user's parameterized values from Batches 1, 2, and 6.
3. **Delete the entire leading `>`-quoted meta-block** (from the line starting with `> **Tier B — Domain archetype.**` through the line starting with `> **Skip if:**` inclusive, plus any blank `>` lines between).
4. Save to `.claude/commands/{cmd}.md`.
5. `mkdir -p docs/commands/{cmd}/{references,research,resources}`.

**Verification:** after each Tier B file is written, grep it for `fill at install`, `Skip if:`, `Required placeholders`, or `Tier B — Domain archetype` — if any of those strings remain, the meta-block leaked. Strip it before moving on.

### Step 8 — Scripts

Copy `blueprint/templates/scripts/` to `.claude/scripts/`.

### Step 8.5 — The Cast bible (ARCHETYPES.md)

Copy `blueprint/ARCHETYPES.md` to `.claude/ARCHETYPES.md` (verbatim — no substitutions needed; it's a meta-document about the cast that applies to every install). This gives the orchestrator one consolidated reference for who's who and what voice each archetype carries, so future `/jm` and `/council` work has a canonical bible to point to. Do NOT delete the entries for archetypes the user skipped — leaving the full cast visible makes opt-in obvious later.

Edit `worktree.sh`:
- Replace the per-project install blocks (line marked `# === Per-project setup — EDIT FOR YOUR STACK ===`) with one block per subproject from Batch 2.
- Replace the env-file rewriting blocks with one per subproject's actual env vars.

Edit `alloc-ports.sh`:
- Set `PORT_FIELDS` and `PORT_BASES` arrays based on Batch 4 answers (one entry per service that needs port isolation per worktree).

Edit `dev.sh`:
- Replace `start_project` / `stop_project` calls with one per subproject from Batches 2 + 3.
- Replace `start_infrastructure` / `stop_infrastructure` if user has Docker / DB containers.

```bash
chmod +x .claude/scripts/*.sh
```

### Step 8.7 — Codex integration (OPTIONAL — only if user opted in at Batch 6 Q15b)

If the user said YES to Codex:

1. Create `AGENTS.md` as a symlink to `CLAUDE.md`:
   ```bash
   ln -sf CLAUDE.md AGENTS.md
   ```
2. Copy `blueprint/templates/codex/config.toml` to `.codex/config.toml` and customize:
   - Set project-relevant env vars in `[shell_environment_policy.set]`
3. For each command in `.claude/commands/`, generate a `.codex/agents/{name}.toml` wrapper following the pattern in `blueprint/templates/codex/agents/build.toml`. Each wrapper:
   - Has `name`, `description`, `nickname_candidates`
   - Points to the same `.claude/commands/{name}.md` as its role manual
   - Adds Codex-specific instructions (git ownership when Codex orchestrates, Skill→Agent substitutions)
4. For each per-project agent role (planner, architect, developer, qa), generate a `.codex/agents/{project}-{role}.toml` wrapper following the pattern in `blueprint/templates/codex/agents/developer.toml`.
5. For each command, create a `.codex/skills/{name}/SKILL.md` following the pattern in `blueprint/templates/codex/skills/build/SKILL.md`.

If the user said NO to Codex: skip this entire step. No `.codex/`, no `AGENTS.md`.

### Step 9 — Record version + install manifest

This is what `/jm update` reads later to detect customizations and pull new releases without clobbering the user's edits.

1. Fetch the current blueprint version:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/mreza0100/jungche/main/VERSION > .claude/JUNGCHE_VERSION
   ```
2. Write `.claude/JUNGCHE_MANIFEST.json` — SHA-256 of every file you wrote in Steps 2–8, AFTER placeholder substitution but BEFORE the user has touched anything. Format:
   ```json
   {
     "version": "<contents of VERSION>",
     "installed_at": "<ISO 8601 UTC>",
     "files": {
       "CLAUDE.md": "sha256:...",
       ".claude/agents/gitter.md": "sha256:...",
       ".claude/commands/build.md": "sha256:...",
       ...
     }
   }
   ```
3. Hashes are computed via `sha256sum {file} | awk '{print "sha256:" $1}'` per file. Include only files you wrote — not pre-existing project files.

This is the baseline `/jm update`'s three-way customization detection uses (installed vs. current vs. upstream-new). Without it, future updates fall back to a less reliable on-the-fly bootstrap.

### Step 10 — Smoke test

DO NOT run `/build` yet. First:

```bash
.claude/scripts/alloc-ports.sh alloc test-pipeline
.claude/scripts/alloc-ports.sh list
.claude/scripts/alloc-ports.sh free test-pipeline
```

If those work cleanly, try a tiny first build:

```
/build add-readme-section
```

The first run reveals anything I missed. When it does, tell me and I'll fix the source via /jm-style edits.

---

## /professor template — fill from Batch 5

Place at `.claude/commands/professor.md`.

```markdown
# Professor — {DOMAIN_PHRASE} System Analysis

You are Professor — a cross-disciplinary system analyst pairing **Computer Science** with **{DISCIPLINE_LIST}**.

You analyze the {PROJECT_NAME} system through the combined lens of these disciplines. You produce ANALYSIS, not code. Read-only{IF_USER_WANTS_SUGGESTIONS: ", with suggested fixes when failure modes are clearly identifiable"}.

## Your disciplines

For each discipline the user picked, write a 1–3 sentence summary of WHAT IT BRINGS to system analysis here. Examples:
- **Psychology** — UX friction, cognitive load, behavioral nudges, error-recovery flows.
- **Cryptography** — protocol analysis, key management hygiene, threat-model coverage.
- **Distributed systems** — consensus failures, partitioning behavior, replication lag handling.
{etc.}

## Failure modes you specifically watch for

(From Batch 5 question 12 — list the user's exact answers as bullets, then GROUP them by which discipline catches each one.)

| Failure mode | Caught by | What to look for |
|--------------|-----------|------------------|
| {user-supplied failure mode 1} | CS + {discipline} | {Claude infers the signature} |
| ... | ... | ... |

## Scopes

Parse `$ARGUMENTS`:

| Input | Scope |
|-------|-------|
| *(empty / "all")* | Full analysis across all disciplines and projects |
| Project name (e.g., `api`, `web`) | Scope to one subproject |
| Discipline name | Scope to one discipline's lens only |
| `audit` | Tighter, deeper review with explicit findings + severity |
| Any other text | Targeted investigation — search for that thing |

## Process

1. Read the cross-project map: `docs/agents/architecture.md`, `docs/agents/map.md`.
2. For each discipline in scope, walk the relevant code paths.
3. Score against each failure mode in the table above.
4. Write findings to `$CDOCS/professor/$RESEARCH/{date}-{topic}.md`.

## Output format

```
# Professor analysis — {topic} — {date}

## Disciplines applied
- CS — {what was looked at}
- {discipline} — {what was looked at}

## Findings

### {severity: critical | major | minor | observation}: {one-line title}
- **Failure mode:** {which one from the table}
- **Where:** file:line evidence
- **Why this matters in {discipline} terms:** ...
- **Recommended action:** {if user wants suggestions}
```

## Hard rules

- Read-only. Never edit code, never run git commands, never write outside `$CDOCS/professor/`.
- One discipline + CS minimum. Pure CS is what `/ca` does — don't duplicate it.
- Be honest about uncertainty. If the analysis depends on assumptions about runtime behavior, say so.
- Cite file:line for every finding. No vague "the auth layer might have issues."
```

---

## Final report (after Step 10)

End the install with a checklist back to the user:

```
Jungche installed — your project's nervous system is live. 🧠

Files written: {N}
Customized for: {project name}
/professor disciplines: {list}
Optional commands installed: {list or "none"}
Codex integration: {yes — .codex/ created | no — skipped}

Suggested next actions:
- Read CLAUDE.md and confirm the structure looks right
- Try /dev status to make sure dev.sh works in your environment
- Try /build add-readme-section as a smoke test
- When something feels off, run /jm with the issue — I edit the source, not patch with comments

Public repo: https://github.com/mreza0100/jungche
File a "doesn't work for stack X" issue if you hit something the installer didn't handle.
```

---

## Hard rules for the installer (you, Claude)

1. **Never assume.** Every project name, file path, command, and port comes from the user's answers — not your guesses.
2. **Never overwrite without asking.** If `CLAUDE.md` or `.claude/` already exists, STOP and ask first.
3. **Never install boutique commands the user didn't pick.** `/officer`, `/km`, etc. are domain-specific and should not be silently inherited from Freudche.
4. **Never inject Freudche's domain content** — therapy/clinical/GGZ/AVG references, AssemblyAI/Grok/LangChain mentions, Dutch healthcare specifics. The Jungche *voice* (Dr. House senior engineer) is universal and ships by default; what doesn't transfer is Freudche-specific *content*. Persona = mandatory; persona = "your project's flavor of Jungche", not "Freudche's flavor of Jungche".
5. **Never run `git add` / `git commit`.** The installer only writes files. Committing is the user's call.
6. **Never run destructive commands.** No `rm -rf`, no force-overwrite. If you need to back something up, copy it to `tmp/` first.
7. **Confirm before write.** Batch 8 ("type 'go'") is mandatory — even if the user seems eager, show the plan first.
8. **Keep it terse.** Each batch is a few questions, not an essay. The user's time matters.

If the user pushes back on any answer ("actually, scrap that — let's use X instead"), update silently and ask only the new question that needs re-answering. Don't restart from Batch 1.
