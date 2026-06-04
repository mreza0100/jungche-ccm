# Placeholder Convention — canonical map

> **The regeneration principle (MANDATORY):** a template IS the live Freudche source file, **verbatim** — same structure, same mechanics, same character, same working logic — with ONLY the Freudche-specific *values* swapped for the placeholders below. Do NOT abstract, skeletonize, summarize, or "genericize" the prose. Keep the code. Swap the customized variables. If a line works and carries no Freudche-specific value, it ships unchanged.

Every regeneration agent reads this file and applies it uniformly. One canonical token per concept — never invent a synonym.

## Identity

| Freudche value | Placeholder |
| --- | --- |
| `Freudche` | `{PROJECT_NAME}` |
| `AI Therapist-Assistant` (tagline) | `{PROJECT_TAGLINE}` |
| `freudche.com` | `{PROJECT_DOMAIN}` |
| `Reza` (founder) | `{FOUNDER_NAME}` |
| `Professor` (persona) | **keep `Professor`** — ships as default name with a "rename if you want" note; never placeholder it |

## Sub-projects (role-named, stable)

| Freudche dir | Placeholder |
| --- | --- |
| `freudche-be` | `{BACKEND_PROJECT}` |
| `freudche-fe` | `{FRONTEND_PROJECT}` |
| `freudche-cortex` | `{AI_PROJECT}` |
| `freudche-infra` | `{INFRA_PROJECT}` |
| `freudche-web` | `{WEB_PROJECT}` |
| `Cortex` (service name in prose) | `{AI_SERVICE_NAME}` |
| project keys `be/fe/cortex/web/infra` | `{be}/{fe}/{ai}/{web}/{infra}` (lowercase role keys) |
| routing keys `BE-ONLY/CORTEX-ONLY/CROSS` | `{BACKEND}-ONLY` / `{AI}-ONLY` / `CROSS` (keep `CROSS`) |

## Tech stack (per role — keep the mechanics, swap the names)

| Freudche tech | Placeholder |
| --- | --- |
| Express, GraphQL Yoga, Drizzle ORM, postgres.js | `{BACKEND_STACK}` (in CLAUDE.md headers); inline: `{ORM}`, `{API_FRAMEWORK}` |
| Expo SDK 52, React Native 0.76, Expo Router, Apollo, NativeWind | `{FRONTEND_STACK}`; inline `{UI_FRAMEWORK}` |
| Python 3.12+, LangChain, LangGraph, SQLAlchemy | `{AI_STACK}`; inline `{AI_FRAMEWORK}` |
| Next.js 15, Tailwind | `{WEB_STACK}` |
| Docker Compose, LocalStack, PostgreSQL | `{INFRA_STACK}` |
| package mgrs `pnpm` / `npm` / `uv` | `{BE_PKG_MGR}` / `{FE_PKG_MGR}` / `{AI_PKG_MGR}` |
| test runners Vitest / Jest / pytest | `{BE_TEST_RUNNER}` / `{FE_TEST_RUNNER}` / `{AI_TEST_RUNNER}` |
| `Postgres` / `postgres.mmd` | `{DATABASE}` / keep `postgres.mmd` filename (generic) |
| cross-cutting `GraphQL`, `WebSocket`, `SQS` | `{API_PROTOCOL}`, `{REALTIME_PROTOCOL}`, `{QUEUE}` |

## External services (vendors)

| Freudche | Placeholder |
| --- | --- |
| Gemini / Google / Vertex AI / `GEMINI_API_KEY` / `AIza` | `{LLM_PROVIDER}` / `{LLM_API_KEY}` / `{LLM_KEY_PREFIX}` |
| AssemblyAI | `{TRANSCRIPTION_SERVICE}` |
| Resend | `{EMAIL_SERVICE}` |
| `europe-west4` / EU residency | `{DATA_REGION}` |

## Ports

| Freudche | Placeholder |
| --- | --- |
| 3000 (BE), 4000 (web) | `{BACKEND_PORT}`, `{WEB_PORT}` |
| 5432 / 5433 (db local/test) | `{DB_PORT}` / `{DB_PORT_TEST}` |
| 4566 / 4567 (localstack) | `{QUEUE_PORT}` / `{QUEUE_PORT_TEST}` |

## Domain nouns (the `{DOMAIN_NOUN}` family)

| Freudche | Placeholder |
| --- | --- |
| therapist | `{USER_NOUN}` |
| patient / client | `{SUBJECT_NOUN}` |
| therapy session | `{SESSION_NOUN}` |
| clinical / therapeutic (adj) | `{DOMAIN_ADJ}` |
| therapy / clinical practice (the field) | `{DOMAIN_NOUN}` |
| DSM-5, diagnoses, treatment recommendations, diagnostic labels | `{FORBIDDEN_DOMAIN_OUTPUTS}` — **Sacred Ground**; keep the guard, swap the examples |
| CBT/DBT/psychodynamic, Jung/Rogers/Freudian, Gottman, CCRT, Rose of Leary | `{DOMAIN_FRAMEWORKS}` — keep one illustrative slot |
| clinic / SUPERVISOR / THERAPIST roles | `{ORG_UNIT}` / `{ROLE_SUPER}` / `{ROLE_USER}` |

## The 10 PhDs (root CLAUDE.md persona)

Keep the **structure** (5 + 5, each with title + bullets + "Think:" line). Keep the **5 CS disciplines** as strong defaults (they fit any software project) with a "swap if your domain differs" note. Replace the **5 Psychology disciplines** with `{PHD_DOMAIN_DISCIPLINE_1..5}` — slots the interview fills with the adopter's domain expertise.

## Regulation / compliance

| Freudche | Placeholder |
| --- | --- |
| GDPR | `{REGULATION}` |
| EU AI Act | `{AI_REGULATION}` |
| GDPR articles (Art. 17 etc.) | `{REGULATION}` Art. `{N}` — keep the structure |
| PHI / PII | `{SENSITIVE_DATA}` |
| MDR / SaMD / NEN 7510 / ISO 27001 | `{DOMAIN_STANDARDS}` |
| Dutch / NL / BV / KVK | `{JURISDICTION}` / `{LEGAL_ENTITY_TYPE}` |
| `europe-west4` / EU | `{DATA_REGION}` |

## Paths (mostly generic pipeline paths — KEEP unchanged)

Keep verbatim: `docs/agents/`, `docs/commands/` (`$CDOCS`), `docs/epics/`, `docs/dev/{builds,waves,backlog.md}`, `.worktrees/`, `tmp/`, `.claude/`, path-vars `$DOCS`/`$CDOCS`/`$REFS`/`$WORKTREE`.
Swap only the Freudche-named leaves: `freudche-cortex/knowledge/` → `{AI_PROJECT}/knowledge/`, `src/freudche_cortex/` → `{AI_PROJECT}/src/...`, machine-absolute `/Users/reza/work/Freudche/...` → `{REPO_ROOT}/...`.

## Model pins

`model: sonnet` / `claude-opus-4-6` / `model: "opus"` → keep as literal defaults but add a one-line `{MODEL_TIER}` note where a pin appears, per the existing blueprint convention. Do not churn working pins into placeholders inside prompt bodies.

## Codex layer is KEPT — do NOT propagate Codex-removals

The blueprint is a **dual-runtime** product (Claude + Codex). Freudche retired Codex and stripped all Codex content from its live files — but the blueprint **keeps** the Codex layer. Therefore, for any file where the live change was *"remove a Codex section/line/reference,"* do **NOT** adopt that removal: **preserve the Codex content from the CURRENT blueprint template** while adopting every OTHER live improvement.

This makes the codex-touched files a 3-way merge — read all three:
1. **Live source** (the spine: adopt all non-Codex improvements).
2. **Current blueprint template** (re-inject the Codex sections/lines/refs that live deleted).
3. **This map** (apply placeholders).

Codex-touched shipped templates: root `CLAUDE.md` (keep the "Two-runtime team" section + `.codex/` refs), `commands/build.md` (keep the dual-runtime paragraph), `commands/wave.md` (keep the dual Skill/Agent runtime block), `commands/documenter.md` (keep `.codex/` in ownership), `commands/pcm.md` (keep ALL Codex-management: invariants stay at 10, Special-Ops Codex steps, codex audit scope — also fix the 34-vs-31 agent-count inconsistency to ONE consistent generic count), `scripts/format-md.sh` (keep `AGENTS.md` in the allow-list). Keep `AGENTS.md` references generally — it is the Codex-side mirror of `CLAUDE.md`.

## Ignored artifacts (do NOT ship, drop references)

`/anneal`, `audit:cortex`, `cortex-drain-wait.sh`, `km-guard.sh` and the Knowledge-guard / No-inline-LLM-prompts rules that reference them are **out of scope**. When a shipped template (root `CLAUDE.md`, `/km`, `/documenter`, `dev`, `wave`, `pcm`, `settings.json`) references them, drop that row/rule/line — exactly as `council`/`reddit` are dropped — so the blueprint has no dangling pointers.
