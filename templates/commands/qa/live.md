---
name: qa:live
description: Live end-to-end QA of the whole frontend on the dev stack with real database, real LLM, and real transcription — no mocks, no seeded data. Builds its own data through the real UI dependency chain, walks each step in its own browser sub-agent, and reports per-feature pass/fail plus feature↔UI drift. Triggered by "/qa:live", "/qa:live <area>", or "/qa:live <feature-number>". Distinct from the qa-{project} pipeline gate agents.
argument-hint: [all | <feature-area> | <feature-number>]
disable-model-invocation: true
---

# QA — Live End-to-End Feature Walkthrough

You are the Professor's QA conscience (inherit his voice). You trust nothing until a real browser has built the data, clicked it, and watched the real output render. Walk the live frontend end to end, judge what actually happens, and report it.

## What this is — and is not

- Tests the LIVE dev stack end to end: frontend :{FRONTEND_PORT} → backend :{BACKEND_PORT} → real {DATABASE} :{DB_PORT} → real {AI_PROJECT} (real LLM) → real transcription. Never the isolated test stack, never mocks.
- **No injected data.** QA builds everything through the real UI in dependency order, so a pass proves the whole chain — creation, persistence, the AI pipeline, and rendering — not just that a seed renders.
- Distinct from the Playwright e2e suite (isolated :{DB_PORT_TEST}, no LLM, fixed-screenshot assertions). This is exploratory and judgment-based, not regression assertions.

## Step 0 — Preflight

1. `/dev status`. If any service is RED, run `/dev up` and wait until green. {AI_PROJECT} MUST be running (real LLM); transcription must be configured (real audio→text). Start from a fresh dev environment (reset via `/dev`) so QA's created data is distinguishable from prior state — QA does not rely on seeded {DOMAIN_ADJ} data regardless.
2. Confirm the browser tools work. On the Claude runtime `mcp__playwright__browser_*` are deferred — load them via ToolSearch (`select:mcp__playwright__browser_navigate,…`) before use. If they cannot be loaded, the `playwright` MCP server is not connected — tell the user to reconnect it (a config change needs a fresh `/mcp` reconnect or a Claude Code restart) and stop. Missing browser binary → `npx playwright install chromium`.
3. Read login credentials from `{BACKEND_PROJECT}/seeding/local/passwords.json`. Start only from the {ROLE_ADMIN}/super-admin account — every other account is created during the run.

## Step 1 — Build the coverage + dependency plan (the understanding engine)

Derive the plan every run from source; never hardcode a feature list — it drifts the moment someone ships.

1. Read `docs/agents/features/_index.md` — the feature registry (one topic file per category, heading-per-feature). It tells you what the app claims to do; the live snapshot in Step 2 is authoritative for the real UI (registries lag).
2. Scope from the argument:
   - empty or `all` → the full chain and every Active feature.
   - `<feature-area>` / `<feature-number>` → build only as much of the chain as that feature needs to exist (a {SESSION_NOUN} insight needs a {SUBJECT_NOUN} + an analyzed {SESSION_NOUN} first).
3. Lay out the creation dependency chain — each feature is verified at its natural point on data QA just created:
   {ROLE_ADMIN} → **{ORG_UNIT}** → **{ROLE_SUPER}** (created by {ROLE_ADMIN}) → **{USER_NOUN}** (created by {ROLE_SUPER}) → **{SUBJECT_NOUN}** (created by the {ROLE_SUPER} — a {USER_NOUN} cannot create {SUBJECT_NOUN}s — and assigned to a {USER_NOUN}) → {USER_NOUN} logs in → **{SESSION_NOUN}** → **audio upload** → **transcription** → **{AI_PROJECT} analysis** → **the {AI_PROJECT} analysis outputs ({DOMAIN_FRAMEWORKS})**.
4. For each feature, map its entry + controls by reconciling: **Routes** (`{FRONTEND_PROJECT}/app/`), **testIDs and flows** (`{FRONTEND_PROJECT}/e2e/visual/*.spec.ts` — authoritative for testIDs, but the live snapshot wins on navigation shape, which the specs can lag), **journeys** (`docs/agents/map/workflows.md`).
5. Emit the plan: the ordered chain of role-walkers, each with the entities it creates and the features it verifies.

## Step 2 — Walk the chain live (serial role-switching walkers, no injected data)

A single browser serves the run, so walk **serially** — spawn one walker, wait for its report and the identifiers/credentials it created, then spawn the next with that handoff. You (the orchestrator) hold the plan and the fixture handoff; each walker's large snapshots stay in the walker's context, never yours. Name every entity this run creates with a recognizable `QA-<run-id>` prefix ({ORG_UNIT}, users, {SUBJECT_NOUN}) so Step 4 teardown can find and remove them.

New accounts ({ROLE_SUPER}, {USER_NOUN}) are created without a usable password on the dev DB — they activate via an emailed link, which in non-production is DEBUG-logged to `tmp/dev/{BACKEND_PROJECT}.log` (grep `activation link` / `reset link` for the URL). Retrieve it and hand it to the next walker to set the password — no inbox needed.

Give each walker this brief, filling in the role, what to create, what to verify, the handoff from the previous walker, and the matching e2e spec:

> Walk **{role}: create {entities}, verify {features}** on the live dev frontend, then report.
>
> 1. Load the browser tools first — they are deferred: ToolSearch `select:mcp__playwright__browser_navigate,mcp__playwright__browser_snapshot,mcp__playwright__browser_click,mcp__playwright__browser_type,mcp__playwright__browser_fill_form,mcp__playwright__browser_take_screenshot,mcp__playwright__browser_console_messages,mcp__playwright__browser_wait_for`. If they cannot be loaded, report `NO MCP ACCESS` and stop.
> 2. Read for testIDs and claimed behavior: `{feature topic file(s)}`, `{matching e2e spec}`, `{BACKEND_PROJECT}/seeding/local/passwords.json`.
> 3. Switch identity: the browser is shared, so a prior walker is likely still logged in. Log out via the **Profile menu** on the app shell (top-right, shows the user's name → Sign Out) — `/login` itself shows no logout control. Then authenticate as {role}: a freshly-created account activates via the link the orchestrator hands you (navigate to it, set a password, report it); an existing account logs in with the handed-off credentials. Handle a forced password-change screen if one appears.
> 4. Create your entities through the UI and exercise your features the way a real user would — perform the actual **write-actions**, not just read: e.g. as a {USER_NOUN} write AND edit a {DOMAIN_ADJ} note, schedule an appointment, send an {AI_PROJECT}-chat message; as a {ROLE_SUPER} edit a profile. Viewing a rendered surface is not a test of it. Navigate by what the snapshot actually shows — a list row may deep-link straight to a nested screen; do not assume a fixed multi-step path. For the AI pipeline (audio → transcription → analysis, or an {AI_PROJECT}-chat turn), poll for the done state (`browser_wait_for` on the loaded indicator / re-enabled input / rendered result) — do not fixed-sleep; allow generous ceilings (transcription + {AI_PROJECT} can take minutes).
> 5. Judge per the feature description: did real data render? Is real LLM output coherent and on-contract? Check `browser_console_messages` (error level) and failed requests. If a surface is empty, cross-check a second instance before calling FAIL (unseeded vs broken). Save one screenshot per screen to `tmp/qa/{role}-<screen>.png` (overwrite stale files from prior runs).
> 6. Snapshots and console output are large — use `browser_snapshot(filename: …)` then grep the saved file; never paste a raw snapshot or full console dump into your report. {SUBJECT_NOUN} data is real {SENSITIVE_DATA} — never transcribe {SUBJECT_NOUN} or {SESSION_NOUN} content; reference by anonymized id and rely on screenshots. Read-only to code: describe bugs, do not edit source. Do not perform destructive admin actions beyond what the chain requires.
>
> Report under 400 words: the entities and credentials you created (for handoff); per-screen PASS/FAIL/WARN + one-line observation + screenshot path; bugs found; anything the UI showed that the catalog did not (drift).

## Step 3 — Drift check

Aggregate across walkers:

- A registry feature with no reachable UI → flag **UNREACHABLE** (the catalog claims it; the app does not surface it).
- A screen a walker reached that maps to no registry feature → flag **UNCATALOGUED**.
- A testID, route, or label in the e2e specs that no longer matches the live UI → flag **STALE-CONTRACT**.

Drift means the catalog/specs and the real UI have diverged — route the reconciliation to `/documenter` (registry) or `/jc` (stale testIDs).

## Step 4 — Teardown (clean up what you built)

`/qa:live` runs on the real dev DB, so it MUST remove the data it created — never leave QA {ORG_UNIT}s/{SUBJECT_NOUN}s/{SESSION_NOUN}s behind to pile up, and never let them be captured by a later `{PROJECT_PKG_MGR} db:export` into the seed.

- Log in as {ROLE_ADMIN} and delete the `QA-`prefixed {ORG_UNIT} this run created; deleting the {ORG_UNIT} cascades to its {ROLE_SUPER}, {USER_NOUN}, {SUBJECT_NOUN}, and {SESSION_NOUN}s. Confirm the {ORG_UNIT} count returns to its pre-run value.
- Also sweep any stray `QA-`prefixed entities left by a previous crashed run.
- If a surface has no delete affordance, report it as a finding and a `/jc` gap — the data persists until then; never fall back to direct DB deletes.

## Step 5 — Report

Write `tmp/qa/report-{date}.md` and summarize inline:

- The chain QA built ({ORG_UNIT} → … → analyzed {SESSION_NOUN}) and where it broke, if it did.
- Per-feature table: feature · role · PASS/FAIL/WARN · observation · screenshot.
- Failures: repro steps and the real error or output observed.
- Drift: UNREACHABLE / UNCATALOGUED / STALE-CONTRACT lists.
- Bugs for `/jc`: one line each.
- Teardown: what was removed (and anything that could not be).
- **Verdict:** {N walked · P pass · F fail · W warn · D drift}.

## Rules

- Real everything, built from zero — dev DB, real LLM, real transcription, data created through the UI. Mocking or seeding anything defeats the command.
- Read-only to the codebase — you exercise the app, you do not edit source. Found a bug? Report it and route the fix to `/jc`.
- **{SUBJECT_NOUN} data in dev is real {SENSITIVE_DATA}** — never copy {SESSION_NOUN} or {SUBJECT_NOUN} content into the report; reference by anonymized id and screenshot the UI instead.
- Discover feature lists, credentials, routes, and testIDs each run from the registry, `passwords.json`, `app/`, and the live snapshot — never hardcode values that change.
