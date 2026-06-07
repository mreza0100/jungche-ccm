# Placeholder Convention — canonical map

> **The regeneration principle (MANDATORY):** a template IS the live source file, **verbatim** — same structure, same mechanics, same character, same working logic — with ONLY the project-specific _values_ swapped for the placeholders below. Do NOT abstract, skeletonize, summarize, or "genericize" the prose. Keep the code. Swap the customized variables. If a line works and carries no project-specific value, it ships unchanged.

Every regeneration agent reads this file and applies it uniformly. One canonical token per concept — never invent a synonym.

## Identity

| Source value                                                                                                              | Placeholder                                                                                         |
| ------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| the project name — current AND any former/renamed-from brand (a rename orphans the old name in the source, so scrub both) | `{PROJECT_NAME}`                                                                                    |
| the project tagline                                                                                                       | `{PROJECT_TAGLINE}`                                                                                 |
| the project domain (its `.com` etc.)                                                                                      | `{PROJECT_DOMAIN}`                                                                                  |
| the founder's name                                                                                                        | `{FOUNDER_NAME}`                                                                                    |
| `Professor` (persona)                                                                                                     | **keep `Professor`** — ships as default name with a "rename if you want" note; never placeholder it |
| the canonical blueprint repo `owner/repo`                                                                                 | `{BLUEPRINT_REPO}`                                                                                  |
| the blueprint repo owner (GH/GL handle)                                                                                   | `{GH_USER}`                                                                                         |
| the local blueprint clone path                                                                                            | `{BLUEPRINT_CLONE_PATH}`                                                                            |

> Blueprint self-references resolve at install: a user with push access to the canonical repo targets it directly; everyone else targets their own fork.

## Project roster — structure-agnostic (N projects, single-project first-class)

The blueprint does NOT assume a fixed set of sub-projects. Structure is a **roster**: an ordered list of 1..N projects captured from the SETUP interview. A single-project repo is a roster of **one** — first-class, not a stripped path. The source this was mined from happens to have several projects; that is an _instance_ of the roster, never baked into the templates.

Each roster entry has: directory, role label, tech stack, package manager, test runner, dev port(s). Templates reference the roster with generic per-entry tokens — never the source's concrete role names:

| Concept                       | Placeholder             | Notes                                                         |
| ----------------------------- | ----------------------- | ------------------------------------------------------------- |
| the project roster (the list) | `{PROJECT_ROSTER}`      | 1..N entries; SETUP fills from the interview                  |
| one entry's directory         | `{project}`             | generic — used inside per-project PATTERN blocks              |
| one entry's role label        | `{PROJECT_ROLE}`        | the adopter's own labels, not `backend/frontend/ai/infra/web` |
| one entry's stack             | `{PROJECT_STACK}`       | lang / framework / ORM / UI etc. for that entry               |
| one entry's package manager   | `{PROJECT_PKG_MGR}`     |                                                               |
| one entry's test runner       | `{PROJECT_TEST_RUNNER}` |                                                               |
| one entry's dev port(s)       | `{PROJECT_PORT}`        |                                                               |
| routing key for an entry      | `{ROLE}-ONLY`           | per-roster; keep `CROSS` for multi-project changes            |

### Materialization (how SETUP expands the roster)

Templates carry per-project **PATTERN blocks** written once with the generic `{project}` tokens. At install SETUP **expands each pattern block once per roster entry**, substituting that entry's fields — so a 1-project and a 7-project adopter get correctly-sized files from the same template. Pattern sites: `build.md`/`wave.md` per-project pipeline stages, `agents/per-project/{planner,architect,developer,qa}.md` (instantiated per entry), and `worktree.sh`/`dev.sh` (which hold a `PROJECTS=(…)` array SETUP fills and iterate it).

### Single-project collapse

When the roster has one entry: the worktree is the repo root (no per-project subdir), cross-project/integration steps are skipped, routing is trivially that one project, and "monorepo" framing drops to "the project." A template must read correctly at roster size 1 — never assume more than one.

### Materialization-expansion tokens (SETUP renders these per roster entry)

These are **NOT hand-filled.** SETUP renders them by expanding the per-project PATTERN blocks once per roster entry (roster size 1 = a single expansion). Each token below is an expansion product, not an interview answer.

| Token                                | Concept                                                            |
| ------------------------------------ | ------------------------------------------------------------------ |
| `{PROJECT_AGENT_ROSTER}`             | rendered list of every per-project agent across the roster         |
| `{PROJECT_PLANNER_ROSTER}`           | per-roster list of planner agents                                  |
| `{PROJECT_ARCHITECT_ROSTER}`         | per-roster list of architect agents                                |
| `{PROJECT_DEVELOPER_ROSTER}`         | per-roster list of developer agents                                |
| `{PROJECT_QA_ROSTER}`                | per-roster list of QA agents                                       |
| `{PROJECT_ANALYSIS_REPORT_LIST}`     | per-roster analysis-report paths                                   |
| `{PROJECT_ARCHITECTURE_REPORT_LIST}` | per-roster architecture-report paths                               |
| `{PROJECT_DEV_REPORT_LIST}`          | per-roster dev-report paths                                        |
| `{PROJECT_BUG_REPORT_LIST}`          | per-roster bug-report paths                                        |
| `{ROSTER_DOC_PATHS}`                 | space-joined roster doc directories                                |
| `{PROJECT_TYPING_RULES}`             | per-stack typing block (one per roster entry's language)           |
| `{PROJECT_TYPECHECK}`                | per-project typecheck command                                      |
| `{PROJECT_FORMAT}`                   | per-project format command                                         |
| `{PROJECT_LINT}`                     | per-project lint command                                           |
| `{PROJECT_INSTALL_CMD}`              | per-project install command                                        |
| `{PROJECT_RUN_CMD}`                  | per-project run command                                            |
| `{PROJECT_ENV_FILES}`                | per-project env-file set                                           |
| `{HEALTH_PROBE}`                     | per-roster health-probe script block SETUP expands                 |
| `{ENV_FILE_PROVISION}`               | per-roster env-file provisioning block                             |
| `{ENV_BOOTSTRAP}`                    | per-roster env-bootstrap block                                     |
| `{POST_INSTALL_HOOKS}`               | per-roster post-install hook block                                 |
| `{STATUS_EXTRA_PROBES}`              | per-roster extra status probes                                     |
| `{DEV_PROCESS_PATTERN}`              | per-roster dev-process pattern block                               |
| `{DEV_PREREQS}`                      | per-roster dev prerequisites                                       |
| `{PORT_DEFAULTS}`                    | per-roster port-default assignments                                |
| `{SEED_PROJECT}`                     | optional-role marker — seed project (`"-"` sentinel when absent)   |
| `{INFRA_PROJECT}`                    | optional-role marker — infra project (`"-"` sentinel when absent)  |
| `{AI_PROJECT}`                       | optional-role marker — AI project (`"-"` sentinel when absent)     |
| `{MIGRATIONS_DIR}`                   | optional-role marker — migrations dir (`"-"` sentinel when absent) |

## Tech stack (per role — keep the mechanics, swap the names)

| Source tech                                                     | Placeholder                                                                  |
| --------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| Express, GraphQL Yoga, Drizzle ORM, postgres.js                 | `{BACKEND_STACK}` (in CLAUDE.md headers); inline: `{ORM}`, `{API_FRAMEWORK}` |
| Expo SDK 52, React Native 0.76, Expo Router, Apollo, NativeWind | `{FRONTEND_STACK}`; inline `{UI_FRAMEWORK}`                                  |
| Python 3.12+, LangChain, LangGraph, SQLAlchemy                  | `{AI_STACK}`; inline `{AI_FRAMEWORK}`                                        |
| Next.js 15, Tailwind                                            | `{WEB_STACK}`                                                                |
| Docker Compose, LocalStack, PostgreSQL                          | `{INFRA_STACK}`                                                              |
| package mgrs `pnpm` / `npm` / `uv`                              | `{BE_PKG_MGR}` / `{FE_PKG_MGR}` / `{AI_PKG_MGR}`                             |
| test runners Vitest / Jest / pytest                             | `{BE_TEST_RUNNER}` / `{FE_TEST_RUNNER}` / `{AI_TEST_RUNNER}`                 |
| `Postgres` / `postgres.mmd`                                     | `{DATABASE}` / keep `postgres.mmd` filename (generic)                        |
| cross-cutting `GraphQL`, `WebSocket`, `SQS`                     | `{API_PROTOCOL}`, `{REALTIME_PROTOCOL}`, `{QUEUE}`                           |

## External services (vendors)

| Source value                                            | Placeholder                                             |
| ------------------------------------------------------- | ------------------------------------------------------- |
| Gemini / Google / Vertex AI / `GEMINI_API_KEY` / `AIza` | `{LLM_PROVIDER}` / `{LLM_API_KEY}` / `{LLM_KEY_PREFIX}` |
| AssemblyAI                                              | `{TRANSCRIPTION_SERVICE}`                               |
| Resend                                                  | `{EMAIL_SERVICE}`                                       |
| `europe-west4` / EU residency                           | `{DATA_REGION}`                                         |

## Ports

| Source value                | Placeholder                          |
| --------------------------- | ------------------------------------ |
| 3000 (BE), 4000 (web)       | `{BACKEND_PORT}`, `{WEB_PORT}`       |
| 5432 / 5433 (db local/test) | `{DB_PORT}` / `{DB_PORT_TEST}`       |
| 4566 / 4567 (localstack)    | `{QUEUE_PORT}` / `{QUEUE_PORT_TEST}` |

## Domain nouns (the `{DOMAIN_NOUN}` family)

| Source value                                                              | Placeholder                                                                                                      |
| ------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| therapist                                                                 | `{USER_NOUN}`                                                                                                    |
| patient / client                                                          | `{SUBJECT_NOUN}`                                                                                                 |
| therapy session                                                           | `{SESSION_NOUN}`                                                                                                 |
| clinical / therapeutic (adj)                                              | `{DOMAIN_ADJ}`                                                                                                   |
| therapy / clinical practice (the field)                                   | `{DOMAIN_NOUN}`                                                                                                  |
| the domain's "sacred ground" / do-no-harm frame                           | `{DOMAIN_SAFETY}` — every domain has a hard "never do this" line; keep the frame, swap the specifics             |
| DSM-5, diagnoses, treatment recommendations, diagnostic labels            | `{FORBIDDEN_DOMAIN_OUTPUTS}` — the `{DOMAIN_SAFETY}` examples for this domain; keep the guard, swap the examples |
| CBT/DBT/psychodynamic, Jung/Rogers/Freudian, Gottman, CCRT, Rose of Leary | `{DOMAIN_FRAMEWORKS}` — keep one illustrative slot                                                               |
| clinic / SUPERVISOR / THERAPIST roles                                     | `{ORG_UNIT}` / `{ROLE_SUPER}` / `{ROLE_USER}`                                                                    |

## The 10 PhDs (root CLAUDE.md persona)

Keep the **structure** (5 + 5, each with title + bullets + "Think:" line). Keep the **5 CS disciplines** as strong defaults (they fit any software project) with a "swap if your domain differs" note. Replace the **5 Psychology disciplines** with `{PHD_DOMAIN_DISCIPLINE_1..5}` — slots the interview fills with the adopter's domain expertise.

## Regulation / compliance

| Source value                      | Placeholder                                    |
| --------------------------------- | ---------------------------------------------- |
| GDPR                              | `{REGULATION}`                                 |
| EU AI Act                         | `{AI_REGULATION}`                              |
| GDPR articles (Art. 17 etc.)      | `{REGULATION}` Art. `{N}` — keep the structure |
| PHI / PII                         | `{SENSITIVE_DATA}`                             |
| MDR / SaMD / NEN 7510 / ISO 27001 | `{DOMAIN_STANDARDS}`                           |
| Dutch / NL / BV / KVK             | `{JURISDICTION}` / `{LEGAL_ENTITY_TYPE}`       |
| `europe-west4` / EU               | `{DATA_REGION}`                                |

## Paths (mostly generic pipeline paths — KEEP unchanged)

Keep verbatim: `docs/agents/`, `docs/commands/` (`$CDOCS`), `docs/epics/`, `docs/dev/{builds,waves,backlog.md}`, `.worktrees/`, `tmp/`, `.claude/`, path-vars `$DOCS`/`$CDOCS`/`$REFS`/`$WORKTREE`.
Swap only the project-named leaves: the AI project's `knowledge/` dir → `{AI_PROJECT}/knowledge/`, its Python package `src/<pkg>/` → `{AI_PROJECT}/src/...`, machine-absolute `/Users/<user>/.../<repo>/...` → `{REPO_ROOT}/...`.

## Model pins

`model: sonnet` / `claude-opus-4-6` / `model: "opus"` → keep as literal defaults but add a one-line `{MODEL_TIER}` note where a pin appears, per the existing blueprint convention. Do not churn working pins into placeholders inside prompt bodies.

## Codex layer is KEPT — do NOT propagate Codex-removals

The blueprint is a **dual-runtime** product (Claude + Codex). If the live source has retired its Codex layer, do **NOT** adopt that removal: for any file whose live change was _"remove a Codex section/line/reference,"_ **preserve the Codex content from the CURRENT blueprint template** while adopting every OTHER live improvement.

This makes the codex-touched files a 3-way merge — read all three:

1. **Live source** (the spine: adopt all non-Codex improvements).
2. **Current blueprint template** (re-inject the Codex sections/lines/refs that live deleted).
3. **This map** (apply placeholders).

Codex-touched shipped templates: root `CLAUDE.md` (keep the "Two-runtime team" section + `.codex/` refs), `commands/build.md` (keep the dual-runtime paragraph), `commands/wave.md` (keep the dual Skill/Agent runtime block), `commands/documenter.md` (keep `.codex/` in ownership), `commands/pcm.md` (keep ALL Codex-management: invariants stay at 10, Special-Ops Codex steps, codex audit scope — also fix the 34-vs-31 agent-count inconsistency to ONE consistent generic count), `scripts/format-md.sh` (keep `AGENTS.md` in the allow-list). Keep `AGENTS.md` references generally — it is the Codex-side mirror of `CLAUDE.md`.

## Ignored artifacts (do NOT ship, drop references)

`/anneal`, `audit:cortex`, `cortex-drain-wait.sh`, `km-guard.sh` and the Knowledge-guard / No-inline-LLM-prompts rules that reference them are **out of scope**. When a shipped template (root `CLAUDE.md`, `/km`, `/documenter`, `dev`, `wave`, `pcm`, `settings.json`) references them, drop that row/rule/line — exactly as `council`/`reddit` are dropped — so the blueprint has no dangling pointers.
