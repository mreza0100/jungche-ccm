---
name: wave:builder
description: The wave builder — two modes. ORCHESTRATED — /wave:orchestrator injects `/wave:builder {BRIEF path}` into the builder chat, which implements the wave task-by-task in the worktree under per-task verification. STANDALONE — run one full pipeline (worktree, conditional design, develop, QA gates, merge, docs) from a spec-ready description; optional — /jc delivers any change live on main.
argument-hint: [BRIEF path | feature description]
---

# Wave Builder

$ARGUMENTS

**Mode dispatch:** a `BRIEF.md` path (or an orchestrator inject) → § Orchestrated mode. A feature description → § Standalone mode.

## Orchestrated mode — the wave builder role

You are the **wave_builder**, steered by the `/wave:orchestrator` chat over the /chat protocol; the whole wave lives in ONE worktree. The TRAIN goal arrives as one `/goal` at train start (harness-durable — it survives your compacts and spans every wave); each wave then arrives as this command with a BRIEF path. Read the BRIEF (identity + env card, spec + corrections pointers, task map, orchestrator chat name), stand up the pipeline test stack it names, then run the task loop until the orchestrator calls the end-of-wave gates. **Goal discipline (the allow-list is absolute):** act ONLY on injected turns — a task verdict, a `/wave:builder` BRIEF, or a boundary brief; awaiting one IS goal progress: end the turn empty-handed. Never start work unprompted, never open `$WAVES` manifests yourself, never run `/jc` unprompted, never self-schedule timers or background waiters — deferred Action Items are the orchestrator's to dispatch. You run in one of two modes: **TASK mode** (the loop below — worktree-only) and **BOUNDARY mode** (§ Boundary duties — entered ONLY by the orchestrator's boundary brief, exited at its post-green `/compact`).

**Per task:** implement through **Sonnet sub-agent hands — they write the code, never you**: one `general-purpose` spawn per coherent span — **default `model: "sonnet"`; the task map's stamped frontier alias (durable default `"opus"`) ONLY for spans the task map or a verdict marks `frontier-hands:`** — briefed with its exact task section + the project's dev protocol (`{project}/.claude/agents/developer.md`, `{AI_PROJECT}/.claude/agents/ai-engineer.md`, `{INFRA_PROJECT}/.claude/agents/devops.md`) + two HARD BANS in every hand brief: NO git of any kind (gitter-only; `stash`/`reset`/`commit` are worktree-wide destructive — a blocked git attempt raises a permission modal that deadlocks THIS chat; stash is NEVER your tool — leave uncommitted worktree state as-is) and NO full-suite/isolation runs (hands build + targeted checks; you verify the combined tree; the gates own the full suite); spawn db-admin — `{INFRA_PROJECT}/.claude/agents/db-admin.md` — or ui-ux — `{project-ui}/.claude/agents/ui-ux.md` — (Opus) when the task's `Build agents:` line says so. Your own turns are coordination, seam judgment, and reviewing the hands' diffs — spec-following implementation is the Sonnet tier, and delegating it keeps your context lean across a long wave. You never self-upgrade a hand's tier: believing a span needs a frontier hand is a ping (`frontier-request: {span} — {reason}`), built only on an orchestrator grant → targeted self-QA (affected-first: unit + typecheck + lint + affected profile — affected = trace the call graph INWARD from every changed symbol; symbol-grep alone misses indirect dispatch paths; the full suite runs only at the two gates) → write the § Task report card → ping the orchestrator with the absolute report path → **END YOUR TURN**. The verdict arrives as an injected fresh turn — a foreground wait-loop keeps the pane busy and deadlocks against the very inject it waits for (idle means END THE TURN, nothing else). No verdict ~10 minutes after a ping → capture the orchestrator's screen and re-ping once (idempotent: same report path, same echoed verdict id). Echo the last verdict id in your next ping. Start task N+1 file writes only after task N's checkpoint commit returns. Mid-task ambiguity or spec-reality conflict → ping the question with a SPEC-CONFLICT tag, end turn; on `proceed-disjoint`, work the named disjoint task meanwhile; a `proceed-span` verdict pre-authorizes the named disjoint span — still ping + checkpoint per task, the combined review lands at the span's last task. Non-blocking protocol observations — a brief ambiguity you resolved yourself, machinery friction — ride as `retro: {one line}` on a ping AND the card's `Retro:` line (the durable copy the retro ledger is audited against).

**Task report card** — `$TASKS/task-{n}-report.md`, fixed headers, ≤1KB; the orchestrator and its diff-card reader parse fields, never prose. Format binds at BRIEF launch — a live wave finishes on the format it launched with.

```
# T{n} {title} — DONE | DEVIATIONS | BLOCKED
Files: {path — symbols touched}, one line each
Tests: {profiles run → result}
Deviations: none | Expected: … Got: … each
Conflicts: none | SPEC-CONFLICT: {question}
Facts: none | discovered facts binding later tasks
Retro: none | {one-liners}
```

**Dispatch discipline (thrash-proven):** every hand's brief carries the EXACT file+symbol work-list — a scope-name brief ("do the test surgery") makes the hand re-investigate the whole change and thrash (zero files at 200k+ tokens). Work you already hold the context for — applying a change you made or fully mapped (test-fix after your own re-key, an N-file mechanical sweep) — execute DIRECTLY; when context pressure is the urge to delegate, compact-then-direct, never a fresh hand that must re-load it all. Hands are for work you'd otherwise have to load: new investigation, or an exact-briefed bounded span. A dispatch blocked >~15 min with no files landed → interrupt it and take the work over yourself.

**Open-ended/foundational task (harness, scaffold, corpus):** ship incrementally — skeleton (corpus + entrypoint) first, request its checkpoint, then iterate; treat dependencies as black boxes (score their outputs, never spelunk internals); ~15 min / ~150k tokens with zero files written is a stall — checkpoint what exists or ping.

**Destructive migration (drop column/enum/table):** map the FULL coupling before writing it — grep every WHERE / ON CONFLICT / caller of the dropped symbol AND every raw-SQL string (`grep -rn '{col}' src tests` — column refs inside SQL strings are invisible to the typechecker/linter, so typecheck-green is NOT re-key-complete); slow careful planning is protective here, not over-deliberation. Removing a Settings/config FIELD scrubs its ENV VARS across every source in the SAME change (`.env.test` / `.env.local` / `.env.*.example` / `.env.demo` + infra/deploy configs) — a strict-config schema (extra-keys-forbidden) makes an orphan var a fatal boot error, and the worktree's generated env MASKS it (a GATE-2-only class).

**End-of-wave duties** (on the orchestrator's call): remediate hygiene findings in the worktree; run GATE-1 — spawn `qa-{proj}` per routed project, Mode PRE-MERGE, full suite, zero-tolerance; QA also writes the wave's regression + adversarial tests; verdicts persist to `$WAVES/{wave-name}/gate1.md` as per-project PASS/FAIL (a tier that cannot provision → `envBlocked`/`INTEGRATION-UNRUN`, never green). Fix-loop re-runs are failing-project failed+affected only, the full suite once on final green-confirm; any § Fix Loop Escalation trigger → BLOCKED-DEFERRED, worktree preserved. Gates on a schema-changing wave run from the WORKTREE infra (`make -C $WT/{INFRA_PROJECT} … PIPELINE={wave-name}` — main's Makefile resolves REPO_ROOT to the un-migrated tree) after a template nuke-rebuild (`{TEST_DB_NAME}_template` is IF-NOT-EXISTS-guarded; stale = the gate validates the wrong schema).

**Boundary duties (BOUNDARY mode — ALL real post-merge work is yours; the orchestrator only rules):** the worktree is gone — gitter removed it at MERGE — so the write scope narrows to: files under `$WAVES/{wave-name}/` only; source changes ONLY through the `/jc` Skill (its commits land via gitter JC-COMMIT); `make -C {INFRA_PROJECT}` teardown targets. In order: (1) launch the walker — `Workflow({ scriptPath: '{REPO_ROOT}/.claude/workflows/wave-walker.js', args: { reportPath: '$WAVES/{wave-name}/report.md' } })` (scriptPath, never `{name}` — name-lookup serves a stale session-start snapshot), background — its static trace never touches the test stack — and record its run-id in STATE.md at launch; (2) run GATE-2 foreground-serial on the canonical stack (`up-test` → `db-setup-test`; schema-changing wave → nuke-rebuild the canonical test template first, it is IF-NOT-EXISTS-guarded and a stale one validates the pre-merge schema; every touched project's full suite SERIALLY, reset between projects), appending each project verdict to `gate2.md` AS IT LANDS and pinging it; (3) persist `walker-ledger.json` when the walker returns, ping it; (4) implement the orchestrator's `fix-now` rulings via `/jc` boundary-lite (args declare: caller owns gates — gate2.md re-run scope + the independent diff judge + wave docs) at inter-suite turn boundaries ONLY — never mid-suite, the canonical stack is single-tenant; echo every boundary-verdict id in your next ping; a halt-inject (stop-the-train) means: abandon the in-flight `/jc`, list your dirty files in the ack, END TURN; (5) after GATE-2 concludes: teardown (`nuke-test-pipeline PIPELINE={wave-name}`), final ping `boundary done — gate2 {v} · walker {v}`. Then the orchestrator's `/compact` lands — accept it and idle SANCTIONED (the allow-list from the train goal governs verbatim) until the next BRIEF.

**Guard rails (sacred):** NEVER run git — request checkpoints through the orchestrator; TASK mode writes only inside the worktree and its `tasks/` dir (BOUNDARY mode narrows per § Boundary duties); pipe every test through `timeout 600s` + `../.claude/scripts/filter-test-output.sh -p`; parallel-4 stays; tests own their data — no DDL/`.sql` fixtures; no {SENSITIVE_DATA}, {RECORD_NOUN} content, or real DB-row values in any report; SPEC-CONFLICT goes UP via ping, never re-decided; a knowledge/`.claude` file in scope is a stop-and-ping, never an edit.

---

## Standalone mode — one pipeline

**Autonomous execution contract:** once started, a standalone build runs to completion without stopping to ask questions or wait for approval. The only defined stops are pre-flight failure (before any work) and Fix Loop Escalation → BLOCKED-DEFERRED. Any other mid-run stop is a contract violation. A costly/external/production action a task requires (paid API call, live deploy) is not a stop: take the safest reversible path and log it. Raise a true blocker as a pre-flight fail-fast.

**Execution is a saved workflow.** The end-to-end pipeline (Setup → Conditional design → Develop → QA → Code Review → GATE-1 → Merge → GATE-2 → Docs) runs as the saved workflow `.claude/workflows/wave-build.js`. A standalone build does its pre-flight + naming here, then LAUNCHES that workflow (Step 1) rather than hand-running each stage in the expensive main loop. The stage-by-stage flow documented below (Pipeline flow) is the **declared copy** of `wave-build.js` — when a stage, spawn brief, loop cap, or escalation trigger changes in one, change both.

---

## Pre-flight — validate before starting any work

Check `$ARGUMENTS` for fatal unrunnability before creating any directory or allocating any port:

- **Coherence:** Is the description specific enough to route? A bare "fix things" or "improve stuff" with zero context cannot be planned. → `PRE-FLIGHT FAILED: Description too vague — what specifically needs to change? No pipeline started.`
- **Routing declared:** the description or the pre-placed `0-task.md` names the project set (`**Routing:**` — one routing key per roster entry). Missing → `PRE-FLIGHT FAILED: no routing declared — state it or run /wave:refine first.` The engine implements straight from the spec; nothing re-derives routing.
- **Self-consistency:** Does the description contradict itself? → Stop with the contradiction noted.
- **Uncommitted work on main:** read-only `git status --porcelain`. If non-empty AND no `[CarryWIP: ...]` was passed (standalone), warn the founder — list the files — and ask: **commit & carry** (gitter commits it to main; the branch builds on it and merges back as a shared base) or **leave on main** (excluded from the build). Set `$CARRYWIP`. Pre-flight is the only place this is asked.

If pre-flight fails: STOP. Return the diagnostic. Do NOT proceed.

---

## Progress reporting

`wave-build.js` owns live progress (`log()` stream + per-stage phase events; `/workflows` shows it live). Launch (Step 1), wait for the completion notification, then report the returned result — no own phase lines.

On return, summarize the result object `{ pipeline, status, sha, trigger, codeReview, detail, flags }` in one block:

```
{✓ DONE | ✗ FAILED | ⚠ BLOCKED-DEFERRED} $PIPELINE → {status}
Merge: {sha or —} · Code review: {codeReview} · {trigger / detail when not DONE}
Flags: {flags joined, or none}
```

On `BLOCKED-DEFERRED`, point to `$DOCS/BLOCKED.md` for the resume protocol.

---

## Step 0 — Name the pipeline, clean up stale dirs, and resolve paths

### 0a. Stale pipeline cleanup (MANDATORY pre-flight)

Run `bash .claude/scripts/worktree.sh prune` to remove orphaned worktrees, then sweep `docs/dev/builds/` for stale dirs (no matching `.worktrees/{name}`) and archive abandoned/completed ones. Invariants: `BLOCKED.md` dirs are preserved (never archived/deleted), wave-owned builds (matched by `grep -rl "$name" docs/dev/waves/*/report.md`) are skipped, stale standalone dirs move to gitignored cold storage `tmp/dev/archive/builds/`. **Read and execute the full procedure: `docs/commands/build/references/build-reference.md` § 0a.**

### 0b. Name the pipeline

**If `$ARGUMENTS` contains `[Pipeline: {name}]`:** Extract and use that name — the wave runner pre-assigned it, pre-placed the task manifest at `docs/dev/builds/{name}/0-task.md`, and already ran uniqueness checks. Skip name generation and uniqueness check below; proceed directly to path variable resolution.

**Otherwise (standalone invocation):** Choose a short, descriptive kebab-case name based on the feature (e.g., `session-notes`, `audio-streaming`).

**Name uniqueness check (standalone only — skip when `[Pipeline: ...]` is present):** Before proceeding, verify the chosen name does NOT already exist in:

- `tmp/dev/archive/builds/` — archived pipelines (gitignored cold storage)
- `docs/dev/builds/` — active pipelines
- `.worktrees/` — active worktrees

```bash
ls tmp/dev/archive/builds/ 2>/dev/null | sed 's/^[0-9]*-//' | grep -x "{name}"
ls docs/dev/builds/ .worktrees/ 2>/dev/null | grep -x "{name}"
```

The first `ls` strips legacy counter prefixes (e.g., `003-radar-surfaces` → `radar-surfaces`) before matching. If the name exists anywhere, append a version suffix (e.g., `session-notes-v2`) or choose a more specific name. **NEVER reuse an archived pipeline name.**

Resolve path variables:

- **`$PIPELINE`** = `{name}` — pipeline name (kebab-case, unique across active + archived). From `[Pipeline: {name}]` in `$ARGUMENTS` when wave-invoked, otherwise chosen by build.
- **`$WAVE`** = wave name from `[Wave: {wave-name}]` in `$ARGUMENTS`, otherwise `none`. Forwarded to gitter so merge + docs commits carry a `Wave:` trailer.
- **`$EPIC`** = epic name from `[Epic: {epic-name}]` in `$ARGUMENTS`, otherwise `none`. Forwarded as the workflow's `epicName` so a standalone build's documenter routes progress to `docs/epics/{name}/`. Wave-owned builds inherit it but skip the epic write — the wave consolidates it.
- **`$CARRYWIP`** = `commit` or `leave` from `[CarryWIP: ...]` in `$ARGUMENTS` (passed by an orchestrating command), otherwise `ask`. Forwarded as the workflow's `carryWip` — governs whether main's uncommitted work is carried into the pipeline's worktree at SETUP.
- **`$DOCS`** = `docs/dev/builds/{name}` — pipeline docs from repo root (where Step 2 archives from).
- **`$WORKTREE`** = `.worktrees/{name}` — pipeline worktree directory (full monorepo checkout), branch `pipeline/{name}`.

The workflow derives the per-agent paths internally (worktree-relative `$DOCS_REL`, post-merge `$DOCS_POST`, per-project dirs `$WORKTREE/{project}` for each roster entry) from the pipeline name — the orchestrator passes only the name.

```bash
mkdir -p docs/dev/builds/{name}
```

**Write the task manifest** — idempotent, an orchestrating command pre-places this when composing the engine:

```bash
[ -f docs/dev/builds/{name}/0-task.md ] && echo "manifest exists — wave pre-placed it" || echo "manifest missing — standalone build"
```

- **Exists** → read it as-is, do NOT overwrite. Wave wrote the pipeline-specific task spec here.
- **Missing** (standalone build only) → write it now:

  ```markdown
  # Task: {name}

  {verbatim $ARGUMENTS — stripped of [Wave: ...] and [Pipeline: ...] tokens}

  Wave: {$WAVE or none}
  ```

Step 0 resolves only the NAME and pre-places `0-task.md`. The workflow handles worktree creation (gitter SETUP) and per-agent path passing.

---

## Common spawn contract

Every spawned agent inherits these. Each spawn block carries only its role, worktree path, report file, ports, and role-specific additions, plus "follow the Common spawn contract."

- **NEVER run git** — gitter owns every commit.
- **Write reports to root docs, never inside the worktree.** From a worktree, `$DOCS_REL` resolves to the ROOT docs directory (`docs/dev/builds/{name}/`), NOT to `docs/` inside the worktree — e.g. from `$WORKTREE/{project}/`, `$DOCS_REL = ../../../docs/dev/builds/{name}/`. NEVER write to `.worktrees/{name}/docs/` — it is inside the worktree and will be lost.
- **ZERO GAP** — when `$DOCS/0-task.md` is a `/wave:refine` spec, IMPLEMENT and VALIDATE it; never re-decide routing, data model, or contracts; a bare-description standalone build designs as normal. Surface a genuine spec flaw to the orchestrator; never silently change it.
- **Doc-awareness** — consult the grep-true doc clusters: read the project's `docs/architecture/_index.md`, then `grep` for the exact symbol; the full DB schema is `docs/agents/graph/db/postgres.mmd`.

**Per-role doc reads** (each role reads only what it needs from `$DOCS/`):

- **Developers / engineers** → `0-task.md` + `4-db-architecture.md` (if present) + `4-ui-ux-spec.md` (UI role only) + `ports.md`
- **Pre-merge QA** → `0-task.md` + `5-dev-report-{proj}.md` + `6-bugs.md`
- **Post-merge QA** → `0-task.md` + `5-dev-report-{proj}.md` + `6-bugs.md` + project runbook

---

## Step 1 — Launch the build workflow

After pre-flight and Step 0 (name resolved, `0-task.md` pre-placed), launch the saved single-pipeline workflow. It runs cheap workflow orchestration — not the expensive main loop — and owns every stage from SETUP to docs-commit:

`Workflow({name: 'wave-build', args: { pipelineName: '<build-name>', idx: 1, total: 1, description: '<feature>', routing: [<declared project keys, or [] when none declared>], waveName: '<build-name>', epicName: '<$EPIC or none>', carryWip: '<$CARRYWIP>', timestamp: '<YYYY-MM-DD>' }})`

- `routing` = the declared project keys from `0-task.md`'s `**Routing:**` (one routing key per roster entry) — REQUIRED non-empty; the workflow fails fast without it (pre-flight guarantees it).
- `waveName` for a standalone build is the build name itself (no wave); `epicName` carries `$EPIC` so the workflow's documenter routes progress into the epic.
- Runs in the background; do NOT poll (§ Progress reporting).

**On return**, the result is `{ pipeline, status, sha, trigger, codeReview, detail, flags }`:

- `DONE` → proceed to Step 2 (archive), then announce completion.
- `BLOCKED-DEFERRED` → the workflow already wrote `$DOCS/BLOCKED.md`, preserved the worktree, and skipped merge. Report the `trigger` and the resume hint; do NOT archive.
- `FAILED` / `MERGE-FAILED` → report `detail`; do NOT archive.
- `POSTMERGE-FIX-NEEDED` → run § If Post-Merge QA fails for this pipeline, then archive.

Surface `flags` (carry-forward /jc candidates, SPEC-CONFLICTs, pre-existing defects) to the founder.

**Resume:** same session — relaunch with the SAME `args` AND `resumeFromRunId: {runId}` (args are NOT restored from the journal — omit them and the args-guard throws before any cached agent runs); completed agents return cached, in-flight ones re-run. A machine crash mid-run recovers the same way: the journal, the worktree, and `main` all survive — assess them, then relaunch with args + `resumeFromRunId`. New session — resume a BLOCKED-DEFERRED pipeline per its `BLOCKED.md` protocol.

---

## Step 2 — Standalone archive tail (after DONE)

The workflow's internal docs-commit passes `Archive: none` (a wave archives its builds together at wave end). A standalone build archives its own build dir here — the standalone equivalent of the orchestrator's § O6 archive.

Invoke the `gitter` agent in **DOCS-COMMIT** phase (**Model: sonnet**): "Pipeline: {name}. Wave: none. Phase: DOCS-COMMIT. Projects: none. Archive: docs/dev/builds/{name}."

Gitter moves `docs/dev/builds/{name}` to `tmp/dev/archive/builds/` (gitignored cold storage; git history keeps the permanent record) and commits the removal. Skip this step on any non-DONE outcome — BLOCKED-DEFERRED dirs stay in place for resume.

---

## Pipeline flow (declared copy of wave-build.js)

This is the contract `.claude/workflows/wave-build.js` executes — invariant: flow graph + spawn briefs are declared copies, update both together. Pipeline Reference (at-a-glance step map): `docs/commands/build/references/build-reference.md` § Pipeline step map. The two-gate discipline and per-pipeline isolated test infra below live in the engine; this prose must match it, never contradict.

**Stage flow:** Setup → Conditional design → Develop (targeted self-QA) → Targeted QA + Fix Loop → Code Review → **GATE-1 (pre-merge full)** → Merge → **GATE-2 (post-merge full)** → Docs. Model tiers per CLAUDE.md § Model Selection (single source).

1. **Setup** — gitter SETUP (sonnet) with `CarryWIP: $CARRYWIP`: one monorepo worktree, one branch `pipeline/{name}`, allocated ports + isolated test ports written to `.env.ports` (`TEST_PG_PORT`/`TEST_LS_PORT`).
2. **Conditional design** — mechanical detection (haiku) reads `0-task.md`: an explicit `**Build agents:**` declaration wins; otherwise it greps for schema signals (`table`, `schema`, `column`, `index`, `enum`, `migration`, `{SCHEMA_DEFINITION}`, `{ORM}`, `database`) → db-admin (opus), and UI visual work → ui-ux (opus). Every column in `{SCHEMA_DEFINITION}` needs a matching SQL migration; db-admin's column-level completeness check is BLOCKING.
3. **Develop** — developers/ai-engineer/devops (sonnet, parallel over routing) implement straight from `0-task.md` — its File plan + Contracts are the work queue (+ `4-db-architecture.md` / `4-ui-ux-spec.md` when present). **Self-QA is TARGETED** — unit + typecheck + lint + only the affected/own integration (or e2e) profile, never the full suite (the full suite runs at the two gates only).
4. **Targeted QA + Fix Loop** — qa-{proj} wrappers (opus, parallel) run failing + affected profiles + the pipeline's adversarial tests + unit against the worktree branches, writing each project's `## {PROJECT}` section of `$DOCS/6-bugs.md`. Developers fix OPEN bugs (only projects whose section is OPEN spawn a fix dev; the brief carries QA's bug ids + summary from the round's structured result), QA re-runs targeted. **Cap: 3 iterations** → escalation triggers below.
5. **Code Review** — pre-merge hygiene gate on the pipeline's own diff (`git -C $WORKTREE diff --name-only main...pipeline/{name}`), `audit/code-hygiene.md` scope `diff`, Category 8 (Duplication) first → `$DOCS/6-code-review.md` verdict. On `FINDINGS`: developers decide + apply the fixes → re-audit (verifies the recorded findings + fix diffs; the full checklist loads only in pass 1). **Cap: 2 iterations** → log `## Residual` and proceed (never block shipping on hygiene; a standalone build surfaces residual to the founder).
6. **GATE-1 (pre-merge full)** — one full-suite run (unit + integration/e2e), zero-tolerance all-green, against the worktree branches; it follows Code Review, so hygiene fixes sit inside the gate — no code change reaches Merge without a full-suite run over it. On bugs: one targeted fix pass + one re-run; still failing → BLOCKED-DEFERRED. A whole tier that cannot provision its infra is reported `envBlocked` (UNRUN-ENV) with an `INTEGRATION-UNRUN` flag — never counted as green; the flag rides to the wave-level gate, which re-runs that tier on integrated `main`.
7. **Merge** — gitter MERGE (sonnet) once GATE-1 is green; serializes against `main` via its own `git-lock.sh`. Returns the merge sha.
8. **GATE-2 (post-merge full)** — qa-{proj} wrappers run the full suite from the project dirs on `main` (not worktrees), zero-tolerance all-green; a scribe consolidates `$DOCS/7-post-merge-qa.md`. A code failure returns `POSTMERGE-FIX-NEEDED`; a tier that cannot provision is reported `envBlocked`/UNRUN-ENV with an `INTEGRATION-UNRUN` flag (distinct from a code failure) — the wave-level gate covers it on integrated `main`.
9. **Docs** — a mono-documenter scout (sonnet) maps the blast radius into disjoint doc scopes, then one documenter worker per scope (sonnet, parallel) merges its own slice into permanent root + child docs (a wave-owned build excludes the epic scope — the wave consolidates it; the scout + fan-out is inline, a declared copy of `documenter.md` § Orchestration, because a workflow can't nest inside this one); gitter DOCS-COMMIT (sonnet) commits with `Archive: none` (standalone archive happens in Step 2 above).

**Per-pipeline isolated test infra — the orchestrator owns the lifecycle.** Each pipeline runs ONE isolated test stack ({DATABASE} + {QUEUE} at the worktree's allocated `TEST_PG_PORT`/`TEST_LS_PORT`, NOT the shared `{DB_PORT_TEST}/{QUEUE_PORT_TEST}` default). The engine stands it up ONCE before Develop (`stackSetup`: nuke→up→template→db→health from the worktree infra, recording the ports to `$DOCS/test-stack.env` for post-merge GATE-2) and tears it down ONCE at the very end (`stackTeardown`, in a `finally`). The parallel dev + QA agents SHARE that one container and NEVER run `up-/down-/nuke-test-pipeline` — a per-agent nuke drops the shared template + every sibling's worker DBs mid-run. Each agent isolates INSIDE the container via a dedicated per-project worker DB (`{project}_test_{worker}`, cloned from the shared template) and per-project {QUEUE}-queue segments (`{base}-{project}-{worker}`), so all projects' QA runs in parallel with no collision. Every test command is wrapped in `timeout 600s` and piped through `../.claude/scripts/filter-test-output.sh -p` (failures + summaries only) — the `settings.json` hook does not reach subagents, so each agent pipes explicitly.

### Fix Loop Escalation — BLOCKED-DEFERRED

The engine aborts the QA/GATE-1 loop and marks the pipeline `BLOCKED-DEFERRED` on ANY of:

- **Pre-existing / orthogonal failure** (trigger `pre-existing-orthogonal`) — QA confirms every OPEN bug reproduces on `main` or sits outside this pipeline's diff (`git diff --name-only main...pipeline/{name}`), so it is not this pipeline's bug to fix-loop. The engine short-circuits immediately — no iteration cap spent — and `BLOCKED.md` routes it to `/jc` on main (resume branch A). QA signals this via `preExistingOrthogonal`; a failure in code this pipeline changed never qualifies.
- **Iteration cap reached** — 3 targeted fix-loop iterations (or the bounded GATE-1 re-run) passed, bugs still OPEN.
- **Hung test** — a QA report flags `hungTest` / `BUG-HUNG-TEST` (a test deadlocked at 0% CPU for >2 minutes — the agent kills it and reports it; no fix-loop iteration fixes code that hangs). A hung test is a code bug for `/jc` on main, not a fix-loop bug.
- **Same bug returns** — a bug id reappears in two consecutive QA rounds despite a developer fix between them (the fix is wrong; retrying is futile).
- **Sub-agent orphan** — a developer or QA sub-agent returns no output even after the single resilient respawn.

On any trigger the engine writes `$DOCS/BLOCKED.md` (template: `docs/commands/build/references/build-reference.md` § BLOCKED.md), preserves the worktree + ports, skips gitter MERGE, and returns `BLOCKED-DEFERRED` with the trigger. The `0a` stale-cleanup rule recognizes `BLOCKED.md` and never auto-archives a deferred pipeline.

### If Post-Merge QA fails

On `POSTMERGE-FIX-NEEDED`, spawn a new fix pipeline `{name}-postmerge-fix`: create it (new `$DOCS`), write a plan scoped to the bugs found, run the full cycle (gitter SETUP → architects → developers → QA → fix loop → gitter MERGE), re-run Post-Merge QA, repeat until clean.

---

## Wave mode

`wave-build` serves standalone `/wave:builder` only — the dual-chat wave is `/wave:orchestrator` + § Orchestrated mode; `/wave:live` batches on `main` without worktrees. A `[Pipeline: {name}]`-tagged invocation with pre-placed `0-task.md` launches this engine directly (Step 0b's manifest-exists branch).

---

## Done

Once the workflow returns DONE and Step 2 archives the build dir, say: "Build complete ({name}). All tests pass on main. Code review clean. Docs committed; pipeline archived to tmp (git history keeps the record)."
