export const meta = {
  name: 'wave-build',
  description: 'Single-pipeline build workflow — runs ONE pipeline end-to-end (SETUP → conditional design (db-admin/ui-ux) → develop → QA → review → GATE-1 → merge → GATE-2 → docs) straight from the ZERO-GAP 0-task.md spec, with per-pipeline isolated test infra. Standalone /wave:builder invokes it; /wave:orchestrator runs the dual-chat wave instead.',
  phases: [{title:'Setup'},{title:'Design'},{title:'Develop'},{title:'QA'},{title:'Code Review'},{title:'GATE-1'},{title:'Merge'},{title:'Post-Merge'},{title:'Docs'}],
}

// args is ONE pipeline (standalone /wave:builder passes a lean object):
// { pipelineName, idx, total, description, routing: ['{project-a}',...] REQUIRED non-empty, waveName, epicName, carryWip, timestamp }
// The harness may deliver `args` JSON-STRING-encoded instead of parsed — parse before validating.
// Flow graph + spawn briefs are declared copies of .claude/commands/wave/builder.md — update both together.

if (typeof args === 'string') {
  try { args = JSON.parse(args) } catch (e) {
    throw new Error('wave-build: args arrived as a string but is not valid JSON: ' + e.message)
  }
}

if (!args || !args.pipelineName) {
  throw new Error('wave-build requires args.pipelineName — it runs exactly one pipeline; see /wave:builder for the invocation contract')
}

if (args.idx == null) args.idx = 1
if (args.total == null) args.total = 1
if (!Array.isArray(args.routing)) args.routing = []

// The single pipeline this build runs. All stage functions take `p`; here p IS args.
const p = args

// INSTALL: Replace this roster with the actual project keys for this install.
// Each key must match the routing keys used in workflow.json and the qa-{project} wrapper names.
// Single-project install: one entry only (e.g. ['app']).
const PROJECTS_ALL = ['{project-a}', '{project-b}']
// INSTALL: Map each project key to the developer agent file it uses.
// Use 'developer.md' for standard projects; specialist names (e.g. 'devops.md', 'ai-engineer.md') where applicable.
const DEV_FILE = { '{project-a}': 'developer.md', '{project-b}': 'developer.md' }
// INSTALL: Map each project key to a human-readable role label for spawn briefs.
const DEV_ROLE = { '{project-a}': '{project-a} developer', '{project-b}': '{project-b} developer' }
// INSTALL: Map project keys to their runbook paths (used by post-merge QA). Omit if a project has no runbook.
const RUNBOOK = { '{project-a}': '{project-a}/docs/runbook.md' }

const FLAGS = { type: 'array', items: { type: 'string' }, description: 'carry-forward items, verbatim one-liners: /jc candidates, SPEC-CONFLICTs, pre-existing defects observed' }
const STATUS = { type: 'object', properties: { status: { type: 'string', enum: ['OK', 'FAIL'] }, summary: { type: 'string' }, flags: FLAGS }, required: ['status', 'summary'] }
const DETECT = { type: 'object', properties: { dbAdmin: { type: 'boolean' }, uiUx: { type: 'boolean' } }, required: ['dbAdmin', 'uiUx'] }
const QA = { type: 'object', properties: { status: { type: 'string', enum: ['NONE', 'OPEN'] }, bugIds: { type: 'array', items: { type: 'string' } }, hungTest: { type: 'boolean' }, preExistingOrthogonal: { type: 'boolean' }, envBlocked: { type: 'boolean' }, summary: { type: 'string' }, flags: FLAGS }, required: ['status', 'bugIds', 'hungTest', 'summary'] }
const REVIEW = { type: 'object', properties: { verdict: { type: 'string', enum: ['CLEAN', 'FINDINGS', 'RESIDUAL'] }, projects: { type: 'array', items: { type: 'string', enum: PROJECTS_ALL } }, summary: { type: 'string' }, flags: FLAGS }, required: ['verdict', 'projects', 'summary'] }
const MERGE = { type: 'object', properties: { status: { type: 'string', enum: ['OK', 'FAIL'] }, sha: { type: 'string' }, summary: { type: 'string' }, flags: FLAGS }, required: ['status', 'sha', 'summary'] }
const PMQA = { type: 'object', properties: { status: { type: 'string', enum: ['PASS', 'FAIL'] }, envBlocked: { type: 'boolean' }, summary: { type: 'string' }, flags: FLAGS }, required: ['status', 'summary'] }
// Docs fan-out (sync: documenter-fanout.js is the canonical engine): scout maps disjoint doc scopes, one worker per scope.
const DOC_SCOPE = { type: 'object', properties: { key: { type: 'string' }, steps: { type: 'string' }, writeTargets: { type: 'string' }, sources: { type: 'array', items: { type: 'string' } } }, required: ['key', 'steps', 'writeTargets'] }
const DOCSCOUT = { type: 'object', properties: { scopes: { type: 'array', items: DOC_SCOPE }, summary: { type: 'string' } }, required: ['scopes'] }

// Silent agent death is routine (gitter SETUP, devs died in past waves) — respawn once
// with a continuation brief before treating it as an orphan.
async function resilient(prompt, opts) {
  // '[label]' prompt prefix: the token ledger's snippet fallback attributes workflow spend per stage.
  let r = await agent('[' + (opts.label || 'agent') + '] ' + prompt, opts)
  if (r === null) {
    log('⚠ ' + (opts.label || 'agent') + ' died silently · respawning once with continuation brief')
    r = await agent(
      '[' + (opts.label || 'agent') + '-retry] RESUME: a prior agent for this exact role died mid-task. Check existing artifacts first — partial report files and their Continuation sections — and complete ONLY the remainder; never redo finished work. ' + prompt,
      { ...opts, label: (opts.label || 'agent') + '-retry' }
    )
  }
  return r
}

const bad = r => !r || r.status === 'FAIL'
const take = (flags, r) => { if (r && r.flags && r.flags.length) flags.push(...r.flags); return r }
const COMMON = ' Common spawn contract (wave/builder.md): NEVER run git — gitter owns every commit; reports go to the root $DOCS path given here, never inside the worktree; ZERO GAP — implement the spec, never re-decide it; doc-awareness via the grep-true doc clusters; report carry-forward items (/jc candidates, SPEC-CONFLICTs, pre-existing defects) in your structured flags, one line each.'
const TEST_PIPE = ' Wrap every test command in timeout 600s and pipe through ../.claude/scripts/filter-test-output.sh -p (the settings.json hook misses subagents) — report failures + summaries only, never tail/head/grep test output.'
// INSTALL: adapt the infra project key, test-port references, worker-DB and message-queue-segment naming below
// to match your install's {INFRA_PROJECT}, TEST_PG_PORT/TEST_LS_PORT allocated test ports, and message-queue conventions.
// Constant prefix, pipeline-name tail — the shared byte-identical prefix stays long for prompt caching.
const TEST_INFRA = ' Test stack: ONE shared per-pipeline container (DB + message-queue services), orchestrator-owned — ALREADY UP, shared with the parallel dev/QA agents, torn down ONCE after ALL agents finish. NEVER run up-/down-/nuke-test-pipeline — a nuke drops the shared template + sibling worker DBs mid-run; db-setup-test-template-pipeline / db-setup-test-pipeline are fine (idempotent, no DROP). Isolate INSIDE the container: your worker DB {project}_test_{worker} (cloned from the shared template, which carries the worktree migrations) + message-queue segment {base}-{project}-{worker} — a bare {base}-{worker} name collides with a sibling project. Ports TEST_PG_PORT/TEST_LS_PORT from <worktree>/.env.ports (post-merge GATE-2: docs/dev/builds/' + p.pipelineName + '/test-stack.env); export TEST_PG_PORT/TEST_LS_PORT/PIPELINE=' + p.pipelineName + ' and run against it.'
const SELF_QA = ' Self-QA TARGETED — unit + typecheck + lint + only the affected/own integration (or e2e) profile, never the full suite (it runs at the two gates only).' + TEST_PIPE

const docs = p => 'docs/dev/builds/' + p.pipelineName
const wt = p => '.worktrees/' + p.pipelineName
const header = p =>
  'Pipeline: ' + p.pipelineName + ' (build ' + p.idx + '/' + args.total + ', wave ' + args.waveName + ', epic ' + args.epicName + '). ' +
  'Task spec: ' + docs(p) + '/0-task.md (pre-placed — read it first). $DOCS = ' + docs(p) + '. $WORKTREE = ' + wt(p) + '. Branch: pipeline/' + p.pipelineName + '.'
const tag = p => args.total > 1 ? ' · ' + p.pipelineName : ''

// Orchestrator-owned test-stack lifecycle (one shared stack per pipeline; dev + QA agents
// share it with dedicated per-project worker DBs + namespaced queues; cleaned ONCE at the end).
// INSTALL: adapt the {INFRA_PROJECT} key and the *-pipeline make targets to your install's infra conventions.
function stackSetup(p) {
  return resilient(
    header(p) + ' Phase: TEST-STACK-UP — orchestrator-owned, ONE shared stack for the whole pipeline (dev self-QA + QA gates). From the WORKTREE infra, stand up a FRESH isolated stack — run each command wrapped in the Bash 600000ms timeout: `make -C ' + wt(p) + '/{INFRA_PROJECT} nuke-test-pipeline PIPELINE=' + p.pipelineName + '` (clear any stale stack), then `up-test-pipeline PIPELINE=' + p.pipelineName + '`, then `db-setup-test-template-pipeline PIPELINE=' + p.pipelineName + '` (the worktree migrations MUST reach the template), then `db-setup-test-pipeline PIPELINE=' + p.pipelineName + '`, then `health-test-pipeline PIPELINE=' + p.pipelineName + '`. Then record the test ports durably for post-merge GATE-2 (the worktree is removed at merge): `cp ' + wt(p) + '/.env.ports ' + docs(p) + '/test-stack.env 2>/dev/null || true`. Do NOT run any tests — you only stand the stack up. Report OK only when the DB + message-queue services are healthy.',
    { label: 'stack-up' + tag(p), phase: 'Develop', model: 'haiku', schema: STATUS }
  )
}
function stackTeardown(p) {
  return resilient(
    header(p) + ' Phase: TEST-STACK-DOWN — orchestrator-owned, run ONCE now that ALL dev + QA agents are finished. Tear the pipeline stack down + wipe volumes: `make -C ' + wt(p) + '/{INFRA_PROJECT} nuke-test-pipeline PIPELINE=' + p.pipelineName + '`. If the worktree is already gone (post-merge), run it from main: `make -C {INFRA_PROJECT} nuke-test-pipeline PIPELINE=' + p.pipelineName + '`. Best-effort — report OK even if already torn down.',
    { label: 'stack-down' + tag(p), phase: 'Docs', model: 'haiku', schema: STATUS }
  )
}

function devAgent(p, proj, brief, phase = 'Develop') {
  return resilient(
    'You are the ' + DEV_ROLE[proj] + '. Read and follow {project-prefix}-' + proj + '/.claude/agents/' + DEV_FILE[proj] + '. ' + header(p) +
    ' Worktree: ' + wt(p) + '/{project-prefix}-' + proj + '. Read dev-server ports from ' + docs(p) + '/ports.md. ' + brief + COMMON,
    { label: 'dev · ' + proj + tag(p), phase, model: 'sonnet', schema: STATUS }
  )
}

async function buildStage(p, routing, flags) {
  // Conditional design roster: the refined spec declares it (**Build agents:**); the grep is the fallback for a bare description.
  // INSTALL: adapt the grep signals to match your project's ORM/schema conventions.
  const det = await resilient(
    'Mechanical detection for pipeline ' + p.pipelineName + ' (report only, change nothing): read ' + docs(p) + '/0-task.md. ' +
    'If it declares `**Build agents:**` lines, dbAdmin=true iff any names db-admin, uiUx=true iff any names ui-ux. ' +
    'Otherwise grep it for (a) schema signals — table, schema, column, index, enum, migration, database — dbAdmin=true if ANY hit; ' +
    '(b) frontend visual/UI work — uiUx=true if it includes UI visual tasks.' + (routing.some(r => r.includes('fe') || r.includes('web') || r.includes('ui')) ? '' : ' uiUx must be false — no UI project in routing.'),
    { label: 'detect' + tag(p), phase: 'Design', model: 'haiku', schema: DETECT }
  )
  const extras = []
  if (det && det.uiUx) extras.push(() => resilient(
    'You are the UI/UX designer. Read and follow {project-ui}/.claude/agents/ui-ux.md. ' + header(p) +
    ' Read ' + docs(p) + '/0-task.md — the ZERO-GAP spec; its behaviors, file plan, and boundaries scope your design. Write your spec to ' + docs(p) + '/4-ui-ux-spec.md.' + COMMON,
    { label: 'ui-ux' + tag(p), phase: 'Design', model: 'opus', schema: STATUS }))
  if (det && det.dbAdmin) extras.push(() => resilient(
    'You are the database admin. Read and follow {project-db}/.claude/agents/db-admin.md. ' + header(p) +
    ' Read ' + docs(p) + '/0-task.md — its Data model + Contracts sections are your work queue. Worktrees: ' + wt(p) + '/{project-prefix}-{routed projects}. Implement the schema changes. ' +
    'Verify the migration slot is free (ls the worktree schema-migrations dir) before numbering. ' +
    'Every column in the schema definition MUST have a corresponding SQL migration (CREATE TABLE or ALTER TABLE ADD COLUMN) — run your column-level completeness check before finishing, it is BLOCKING. ' +
    'Write your database architecture doc to ' + docs(p) + '/4-db-architecture.md.' + COMMON,
    { label: 'db-admin' + tag(p), phase: 'Design', model: 'opus', schema: STATUS }))
  if (extras.length) {
    const x = await parallel(extras)
    x.forEach(r => take(flags, r))
    if (x.some(bad)) return false
  }

  const devs = await parallel(routing.map(proj => () => devAgent(p, proj,
    'Implement per ' + docs(p) + '/0-task.md — the ZERO-GAP spec; its File plan + Contracts sections are your work queue (+ 4-db-architecture.md / 4-ui-ux-spec.md if present). ' +
    SELF_QA + ' Write your report to ' + docs(p) + '/5-dev-report-' + proj + '.md.')))
  devs.forEach(r => take(flags, r))
  return !devs.some(bad)
}

// scope 'TARGETED' = fix-loop round (failing+affected+adversarial+unit); 'FULL' = GATE-1/GATE-2 full suite.
function qaRound(p, routing, iter, scope) {
  const scopeBrief = scope === 'FULL'
    ? ' Scope: FULL — the pre-merge full-suite gate (unit + integration/e2e), zero-tolerance all-green.'
    : ' Scope: TARGETED — re-run ONLY failing + affected profiles + the pipeline\'s adversarial tests + unit, NOT the full suite.'
  return parallel(routing.map(proj => () => resilient(
    'Mode: PRE-MERGE.' + scopeBrief + ' ' + header(p) + ' Worktree: ' + wt(p) + '/{project-prefix}-' + proj + '.' +
    ' Run every infra make target from the WORKTREE infra — make -C ' + wt(p) + '/{INFRA_PROJECT} ... — worktree-only migrations must reach the test template (running main\'s infra builds a template missing them).' + TEST_INFRA +
    ' Write findings into ' + docs(p) + '/6-bugs.md under a `## ' + proj.toUpperCase() + '` section (create the file if absent) — own only that section, never touch another\'s; your structured return must match what you wrote there.' + TEST_PIPE + COMMON +
    ' Structured output: status NONE|OPEN for your section; bugIds = stable ids of bugs still OPEN; hungTest = true if any test deadlocked at 0% CPU for >2 minutes (kill it and report BUG-HUNG-TEST in your section); ' +
    'preExistingOrthogonal = true ONLY when EVERY open bug either reproduces on main OR lives in a file this pipeline never changed (confirm against `git -C ' + wt(p) + ' diff --name-only main...pipeline/' + p.pipelineName + '`) — name each as a `/jc on main` candidate in flags; a failure in code this pipeline changed keeps it false; ' +
    'envBlocked = true when a whole test TIER could not run because its infra would not provision (an environment blocker, not a code failure) — add an `INTEGRATION-UNRUN: {tier}` flag and report status NONE only for tiers that actually executed, never for one that was skipped.',
    { label: 'qa ' + (scope === 'FULL' ? 'gate1' : 'r' + iter) + ' · ' + proj + tag(p), phase: scope === 'FULL' ? 'GATE-1' : 'QA', agentType: 'qa-' + proj, schema: QA }
  )))
}

// Spawn developers against a QA round's OPEN sections (shared by the targeted fix loop and the GATE-1 fix pass).
// `qa` = that round's structured results, index-aligned with `routing` (qaRound maps routing):
// clean projects spawn nothing; each brief carries QA's bugIds + summary — the dev fixes the known set, never re-derives scope.
function fixRound(p, routing, qa) {
  const open = routing.map((proj, i) => ({ proj, r: qa[i] })).filter(x => x.r && x.r.status === 'OPEN')
  return parallel(open.map(({ proj, r }) => () => devAgent(p, proj,
    'Fix loop: read ' + docs(p) + '/6-bugs.md `## ' + proj.toUpperCase() + '` and fix every bug with Status OPEN' +
    (r.bugIds.length ? ' — QA reported: ' + r.bugIds.join(', ') + ' (' + r.summary + ')' : '') +
    '; the failing adversarial test is the reproduction.' + SELF_QA, 'QA')))
}

// Step 4 — Targeted Fix Loop, cap 3, escalation per wave/builder.md § Fix Loop Escalation. Returns null when green; else the trigger.
async function fixLoopStage(p, routing, flags) {
  let prev = null
  for (let iter = 0; iter <= 3; iter++) {
    const qa = await qaRound(p, routing, iter, 'TARGETED')
    qa.forEach(r => take(flags, r))
    if (qa.some(r => r === null)) return 'sub-agent-orphan'
    if (qa.some(r => r.hungTest)) return 'hung-test'
    if (qa.every(r => r.status === 'NONE')) { log(p.pipelineName + ': ✓ QA targeted r' + iter + ' green'); return null }
    // Every open bug reproduces on main or sits outside this pipeline's diff → not ours to fix-loop;
    // defer it straight to /jc-on-main rather than spending the iteration cap (BLOCKED.md routes it).
    if (qa.filter(r => r.status === 'OPEN').every(r => r.preExistingOrthogonal)) return 'pre-existing-orthogonal'
    const cur = new Set(qa.flatMap(r => r.bugIds))
    if (prev && [...cur].some(id => prev.has(id))) return 'repeat-bug'
    if (iter === 3) return 'iteration-cap'
    prev = cur
    log(p.pipelineName + ': ⚙ Fix ' + (iter + 1) + '/3 · ' + cur.size + ' bugs')
    if ((await fixRound(p, routing, qa)).some(r => r === null)) return 'sub-agent-orphan'
  }
  return 'iteration-cap'
}

// Step 6 — GATE-1 pre-merge full suite. Runs AFTER Code Review (declared order — wave/builder.md § Pipeline flow, meta.phases)
// so review fixes sit INSIDE the gate: no code change reaches Merge without a full-suite run over it.
// One bounded fix pass + one re-run. Returns null only when green.
async function gate1Stage(p, routing, flags) {
  for (let g = 0; g <= 1; g++) {
    log(p.pipelineName + ': 🔒 GATE-1 full suite (pre-merge)' + (g ? ' · re-run' : ''))
    const gate = await qaRound(p, routing, 0, 'FULL')
    gate.forEach(r => take(flags, r))
    if (gate.some(r => r === null)) return 'sub-agent-orphan'
    if (gate.some(r => r.hungTest)) return 'hung-test'
    if (gate.every(r => r.status === 'NONE')) { log(p.pipelineName + ': ✓ GATE-1 green'); return null }
    if (gate.filter(r => r.status === 'OPEN').every(r => r.preExistingOrthogonal)) return 'pre-existing-orthogonal'
    if (g === 1) return 'iteration-cap'
    log(p.pipelineName + ': ⚙ GATE-1 fix pass · ' + new Set(gate.flatMap(r => r.bugIds)).size + ' bugs')
    if ((await fixRound(p, routing, gate)).some(r => r === null)) return 'sub-agent-orphan'
  }
  return 'iteration-cap'
}

// Step 5 — pre-merge hygiene gate, cap 2 (wave/builder.md § Code Review). Runs BEFORE GATE-1 so its fixes land inside the full-suite gate.
// Pass 0 executes the full checklist; re-audits verify the recorded findings without reloading the checklist.
async function reviewStage(p, routing, flags) {
  for (let iter = 0; iter <= 2; iter++) {
    const last = iter === 2
    const brief = iter === 0
      ? 'Pre-merge hygiene gate: read .claude/commands/audit/code-hygiene.md and execute it with scope `diff` over the changed set from `git -C ' + wt(p) + ' diff --name-only main...pipeline/' + p.pipelineName + '` (read-only git permitted for this audit only). Scope strictly to that committed three-dot range — files inherited from main\'s dirty working tree are OUT of scope. Category 8 (Duplication) first. Write findings to ' + docs(p) + '/6-code-review.md ending with a verdict line.'
      : 'Pre-merge hygiene RE-AUDIT (pass ' + iter + ' — the full checklist ran in pass 0, do not reload it): read ' + docs(p) + '/6-code-review.md, verify each recorded finding is resolved in the worktree code, and inspect the fix diffs (read-only `git -C ' + wt(p) + ' diff main...pipeline/' + p.pipelineName + '`) for newly introduced duplication. Update 6-code-review.md ending with a verdict line.'
    const rev = await resilient(
      header(p) + ' ' + brief +
      (last ? ' Final pass: if findings remain, move them under `## Residual` and return verdict RESIDUAL.' : '') +
      ' Structured output: verdict; projects = project keys named in findings.',
      { label: 'review r' + iter + tag(p), phase: 'Code Review', model: 'opus', schema: REVIEW }
    )
    if (!rev) return 'FAIL'
    take(flags, rev)
    if (rev.verdict !== 'FINDINGS') return rev.verdict
    const affected = rev.projects.length ? rev.projects : routing
    await parallel(affected.map(proj => () => devAgent(p, proj,
      'Read ' + docs(p) + '/6-code-review.md. For each finding in your project decide the fix — which existing symbol to reuse, where to extract the shared helper, which copy to delete — and apply it.' + SELF_QA + ' The worktree must stay test-green.', 'Code Review')))
  }
  return 'RESIDUAL'
}

// Steps 8–11 per wave/builder.md § Merge Phase / Post-Merge / Documentation
async function shipStage(p, routing, flags) {
  const projectsCsv = routing.join(',')
  const m = await resilient(
    'Pipeline: ' + p.pipelineName + '. Wave: ' + args.waveName + '. Phase: MERGE. Projects: ' + projectsCsv + '. ' +
    'Serialize against main via your own git-lock.sh — it guarantees a single merge to main at a time across concurrent pipelines. ' +
    'Structured output: status, sha = merge commit sha.',
    { label: 'merge' + tag(p), phase: 'Merge', agentType: 'gitter', schema: MERGE }
  )
  if (bad(m)) return { status: 'MERGE-FAILED', detail: m ? m.summary : 'gitter merge agent died twice' }
  take(flags, m)
  log(p.pipelineName + ': ✅ merged ' + (m.sha ? m.sha.slice(0, 7) : '?'))

  log(p.pipelineName + ': 🔒 GATE-2 post-merge full suite')
  const pm = await parallel(routing.map(proj => () => resilient(
    'Mode: POST-MERGE — GATE-2 full suite (always FULL), zero-tolerance all-green. Pipeline: ' + p.pipelineName + '. Run against {project-prefix}-' + proj + '/ on main (NOT the worktree), infra targets via make -C {INFRA_PROJECT}.' + TEST_INFRA +
    ' Pipeline docs: docs/dev/builds/' + p.pipelineName + '/.' +
    (RUNBOOK[proj] ? ' Runbook: ' + RUNBOOK[proj] + '.' : '') + TEST_PIPE +
    ' Structured output: status PASS|FAIL; envBlocked = true when a whole test TIER could not run because its infra would not provision (add an `INTEGRATION-UNRUN: {tier}` flag) — never report a tier that was skipped as PASS.',
    { label: 'pmqa · ' + proj + tag(p), phase: 'Post-Merge', agentType: 'qa-' + proj, schema: PMQA }
  )))
  pm.forEach(r => take(flags, r))
  await resilient(
    'Scribe task (mechanical write, no judgment): write ' + docs(p) + '/7-post-merge-qa.md consolidating these post-merge QA results for pipeline ' + p.pipelineName +
    ' — one section per project with status and summary: ' + JSON.stringify(routing.map((proj, i) => ({ project: proj, result: pm[i] }))),
    { label: 'pmqa-scribe' + tag(p), phase: 'Post-Merge', model: 'haiku', schema: STATUS }
  )
  if (pm.some(r => !r || r.status === 'FAIL')) return { status: 'POSTMERGE-FIX-NEEDED', sha: m.sha }

  // Docs — scout maps disjoint scopes, one worker per scope, each on its scope card + doc-approval card.
  // Stays INLINE (one-level workflow() law — wave-build never calls workflow()). Canonical engine: documenter-fanout.js.
  // SYNC: DOC_BRIEF duplicated verbatim from documenter-fanout.js — edit both together.
  const DOC_BRIEF = (s, sourceLine) =>
    'Per-scope doc consolidation. Read your two cards FIRST, then execute: ' +
    '(1) docs/commands/documenter/references/scopes/' + s.key + '.md — your merge steps and write set (yours alone; writing outside it is a write race); ' +
    '(2) docs/commands/documenter/references/doc-approval.md — write rules, sacred boundaries, Approval gate (emit APPROVED:{path} or fix-and-recheck), finish steps. ' +
    'Scope manifest: ' + JSON.stringify(s) + '. ' + sourceLine +
    ' Structured output: status (OK | SKIP if nothing in your scope actually changed | FAIL), summary (the files you wrote).'
  const docScout = await resilient(
    'Phase: ARCHIVE-SCOUT. Pipeline: ' + p.pipelineName + '. Docs: ' + docs(p) + '. Wave-owned build — EXCLUDE the epic scope (the wave consolidates it). ' +
    'Read .claude/commands/documenter.md § Orchestration, examine the blast radius, and return the DISJOINT scope manifest (each scope: steps + writeTargets + sources; a small change is one or two scopes).',
    { label: 'doc-scout' + tag(p), phase: 'Docs', agentType: 'mono-documenter', schema: DOCSCOUT }
  )
  const docScopes = ((docScout && docScout.scopes) || []).filter(Boolean)
  if (docScopes.length) await parallel(docScopes.map(s => () => resilient(
    'Phase: ARCHIVE. ' + DOC_BRIEF(s, 'Mode ARCHIVE — pipeline docs: ' + docs(p) + '/.'),
    { label: 'doc · ' + s.key + tag(p), phase: 'Docs', model: 'sonnet', schema: STATUS })))
  await resilient(
    'Pipeline: ' + p.pipelineName + '. Wave: ' + args.waveName + '. Phase: DOCS-COMMIT. Projects: ' + projectsCsv + '. Archive: none.',
    { label: 'docs-commit' + tag(p), phase: 'Docs', agentType: 'gitter', schema: STATUS }
  )
  return { status: 'DONE', sha: m.sha }
}

async function blockPipeline(p, trigger) {
  await resilient(
    header(p) + ' The fix loop escalated with trigger `' + trigger + '`. Write ' + docs(p) + '/BLOCKED.md exactly per the template in ' +
    'docs/commands/build/references/build-reference.md § BLOCKED.md — status BLOCKED-DEFERRED, the trigger, root cause from ' + docs(p) + '/6-bugs.md, state preserved, resume protocol. ' +
    'Date: ' + args.timestamp + '. Do NOT delete the worktree, do NOT touch git, do NOT release ports.',
    { label: 'blocked-md' + tag(p), phase: 'QA', model: 'sonnet', schema: STATUS }
  )
  return { pipeline: p.pipelineName, status: 'BLOCKED-DEFERRED', trigger }
}

// Single-pipeline build: SETUP (gitter) → conditional design + develop → QA gate → review → ship.
// Infra is isolated per pipeline (own test stack via *-pipeline make targets), so SETUP/QA/GATE-1/merge
// run inline — no cross-pipeline lock. gitter MERGE serializes against main via its own git-lock.sh.
const flags = []

const setup = await resilient(
  'Pipeline: ' + p.pipelineName + '. Phase: SETUP. CarryWIP: ' + args.carryWip + '.',
  { label: 'setup' + tag(p), phase: 'Setup', agentType: 'gitter', schema: STATUS }
)
if (bad(setup)) return { pipeline: p.pipelineName, status: 'FAILED', detail: 'gitter SETUP failed', flags }

const routing = p.routing || []
if (!routing.length) return { pipeline: p.pipelineName, status: 'FAILED', detail: 'no routing declared — 0-task.md must carry **Routing:** (refine the spec or state it in the description)', flags }

// Orchestrator owns the per-pipeline test stack: stand it up ONCE here (fresh); the dev + QA
// agents SHARE it (dedicated per-project worker DB + per-project queue segment); it is torn down
// ONCE in the finally — never per-agent (a per-agent nuke drops the shared template + sibling
// worker DBs mid-run, the GATE-1 teardown collision this fix eliminates).
const stackOk = await stackSetup(p)
if (bad(stackOk)) return { pipeline: p.pipelineName, status: 'FAILED', detail: 'test-stack setup failed', flags }

try {
  log(p.pipelineName + ': ▶ Develop · ' + routing.length + ' projects (' + routing.join(', ') + ')')
  const built = await buildStage(p, routing, flags)
  if (!built) return { ...(await blockPipeline(p, 'sub-agent-orphan')), flags }

  log(p.pipelineName + ': ▶ QA gate (shared stack · per-agent dedicated DB · orchestrator owns teardown)')
  const blocked = await fixLoopStage(p, routing, flags)
  if (blocked) return { ...(await blockPipeline(p, blocked)), flags }
  const review = await reviewStage(p, routing, flags)
  if (review === 'FAIL') return { ...(await blockPipeline(p, 'sub-agent-orphan')), flags }
  const gated = await gate1Stage(p, routing, flags)
  if (gated) return { ...(await blockPipeline(p, gated)), flags }

  log(p.pipelineName + ': ▶ Ship (merge serialized against main via gitter git-lock.sh)')
  const ship = await shipStage(p, routing, flags)
  return { pipeline: p.pipelineName, status: ship.status, sha: ship.sha, trigger: ship.trigger, codeReview: review, detail: ship.detail, flags }
} finally {
  await stackTeardown(p)
}
