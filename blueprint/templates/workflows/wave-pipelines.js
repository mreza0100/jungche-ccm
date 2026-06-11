export const meta = {
  name: 'wave-pipelines',
  description: 'Wave execution engine — schedules every /build spawn directly per build.md § Wave workflow mode: plan/arch/dev parallel across a group, SETUP/QA/merge serialized, groups sequential',
}

// args arrive verbatim from docs/dev/waves/{waveName}/workflow.json — template + field law in /wave Step 0d:
// { waveName, epicName, carryWip, timestamp, total,
//   groups: [[{ pipelineName, idx, description, routing: ['{project}',...], dependsOn: [] }]] }
// Flow graph + spawn briefs are declared copies of .claude/commands/build.md — update both together.

if (!args || !Array.isArray(args.groups) || !args.groups.length) {
  throw new Error('wave-pipelines requires args.groups — see /wave Step 1 for the invocation contract')
}

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
const PLAN = { type: 'object', properties: { routing: { type: 'array', items: { type: 'string', enum: PROJECTS_ALL } }, needsMonoArchitect: { type: 'boolean' }, addPlanners: { type: 'array', items: { type: 'string', enum: PROJECTS_ALL } }, summary: { type: 'string' } }, required: ['routing', 'needsMonoArchitect', 'addPlanners', 'summary'] }
const DETECT = { type: 'object', properties: { dbAdmin: { type: 'boolean' }, uiUx: { type: 'boolean' } }, required: ['dbAdmin', 'uiUx'] }
const QA = { type: 'object', properties: { status: { type: 'string', enum: ['NONE', 'OPEN'] }, bugIds: { type: 'array', items: { type: 'string' } }, hungTest: { type: 'boolean' }, summary: { type: 'string' }, flags: FLAGS }, required: ['status', 'bugIds', 'hungTest', 'summary'] }
const REVIEW = { type: 'object', properties: { verdict: { type: 'string', enum: ['CLEAN', 'FINDINGS', 'RESIDUAL'] }, projects: { type: 'array', items: { type: 'string', enum: PROJECTS_ALL } }, summary: { type: 'string' }, flags: FLAGS }, required: ['verdict', 'projects', 'summary'] }
const MERGE = { type: 'object', properties: { status: { type: 'string', enum: ['OK', 'FAIL'] }, sha: { type: 'string' }, summary: { type: 'string' }, flags: FLAGS }, required: ['status', 'sha', 'summary'] }
const PMQA = { type: 'object', properties: { status: { type: 'string', enum: ['PASS', 'FAIL'] }, summary: { type: 'string' }, flags: FLAGS }, required: ['status', 'summary'] }

// One exclusive lock serializes gitter SETUP, the QA/review gate (shared test infra),
// the merge tail (single main), and STATE.md scribes. Everything else runs concurrently.
let lock = Promise.resolve()
function exclusive(fn) {
  const run = lock.then(fn, fn)
  lock = run.then(() => {}, () => {})
  return run
}

// Silent agent death is routine — respawn once with a continuation brief before treating it as an orphan.
async function resilient(prompt, opts) {
  let r = await agent(prompt, opts)
  if (r === null) {
    log((opts.label || 'agent') + ' died silently — respawning once with continuation brief')
    r = await agent(
      'RESUME: a prior agent for this exact role died mid-task. Check existing artifacts first — partial report files and their Continuation sections — and complete ONLY the remainder; never redo finished work. ' + prompt,
      { ...opts, label: (opts.label || 'agent') + '-retry' }
    )
  }
  return r
}

const deferred = new Set()
const bad = r => !r || r.status === 'FAIL'
const take = (flags, r) => { if (r && r.flags && r.flags.length) flags.push(...r.flags); return r }
// INSTALL: adapt the infra project key and test-port references below to match your install's {INFRA_PROJECT} and {PORT_B_TEST}/{QUEUE_PORT_TEST}.
const COMMON = ' Follow build.md § Common spawn contract: NEVER run git (gitter owns every commit); write reports to the root $DOCS path given here, never inside the worktree; ZERO GAP — implement the spec, never re-decide it; doc-awareness via the grep-true doc clusters. Report carry-forward items (/jc candidates, SPEC-CONFLICTs, pre-existing defects) in your structured flags, one line each.'
const TEST_INFRA = ' Test infra discipline: the shared test stack runs on the DEFAULT test ports — port reservations in ports.md are aspirational; the wave lock guarantees you are sole occupant. If you hit orphaned worker queues from a prior interrupted run, sanctioned recovery is make -C {INFRA_PROJECT} nuke-test → up-test → db-setup-test, then rerun — log it as environmental, not a code bug.'

const docs = p => 'docs/dev/builds/' + p.pipelineName
const wt = p => '.worktrees/' + p.pipelineName
const header = p =>
  'Pipeline: ' + p.pipelineName + ' (build ' + p.idx + '/' + args.total + ', wave ' + args.waveName + ', epic ' + args.epicName + '). ' +
  'Task spec: ' + docs(p) + '/0-task.md (pre-placed — read it first). $DOCS = ' + docs(p) + '. $WORKTREE = ' + wt(p) + '. Branch: pipeline/' + p.pipelineName + '.'

function plannerAgent(p, proj) {
  return resilient(
    'You are the ' + proj + ' planner. Read and follow {project-prefix}-' + proj + '/.claude/agents/planner.md. Mode: ANALYSIS. ' + header(p) +
    ' Feature: ' + p.description + '. Analyze the {project-prefix}-' + proj + '/ codebase and write ' + docs(p) + '/1-analysis-' + proj + '.md.' + COMMON,
    { label: p.pipelineName + ':planner-' + proj, phase: p.pipelineName, model: 'opus', schema: STATUS }
  )
}

function architectAgent(p, proj, brief) {
  return resilient(
    'You are the ' + proj + ' architect. Read and follow {project-prefix}-' + proj + '/.claude/agents/architect.md. ' + header(p) +
    ' Doc reads: ' + docs(p) + '/1-plan.md + 3-architecture.md (if present) + 1-analysis-' + proj + '.md. ' + brief + COMMON,
    { label: p.pipelineName + ':architect-' + proj, phase: p.pipelineName, model: 'opus', schema: STATUS }
  )
}

function devAgent(p, proj, brief) {
  return resilient(
    'You are the ' + DEV_ROLE[proj] + '. Read and follow {project-prefix}-' + proj + '/.claude/agents/' + DEV_FILE[proj] + '. ' + header(p) +
    ' Worktree: ' + wt(p) + '/{project-prefix}-' + proj + '. Read dev-server ports from ' + docs(p) + '/ports.md. ' + brief + COMMON,
    { label: p.pipelineName + ':dev-' + proj, phase: p.pipelineName, model: 'opus', schema: STATUS }
  )
}

function monoPlannerAgent(p, note) {
  return resilient(
    header(p) + ' ' + note + 'Read the analysis reports at ' + docs(p) + '/1-analysis-*.md and consolidate into ' + docs(p) + '/1-plan.md. ' +
    'Structured output: routing = project keys this pipeline touches; needsMonoArchitect = false only when routing is single-project with no integration changes; ' +
    'addPlanners = projects whose analysis is missing but needed (normally empty).',
    { label: p.pipelineName + ':mono-planner', phase: p.pipelineName, agentType: 'mono-planner', schema: PLAN }
  )
}

async function planStage(p) {
  const initial = p.routing && p.routing.length ? p.routing : PROJECTS_ALL
  await parallel(initial.map(proj => () => plannerAgent(p, proj)))
  let plan = await monoPlannerAgent(p, '')
  if (plan && plan.addPlanners && plan.addPlanners.length) {
    log(p.pipelineName + ': mono-planner demands planners for ' + plan.addPlanners.join(', '))
    await parallel(plan.addPlanners.map(proj => () => plannerAgent(p, proj)))
    plan = await monoPlannerAgent(p, 'Re-consolidation: the demanded analyses now exist. ')
  }
  return plan
}

async function buildStage(p, plan, routing, flags) {
  if (plan.needsMonoArchitect) {
    const ma = await resilient(
      header(p) + ' Read ' + docs(p) + '/1-plan.md. Write ' + docs(p) + '/3-architecture.md — API contracts, shared types, integration patterns; no code-level decisions or TODO stubs.',
      { label: p.pipelineName + ':mono-architect', phase: p.pipelineName, agentType: 'mono-architect', schema: STATUS }
    )
    if (bad(ma)) return false
  }
  const archs = await parallel(routing.map(proj => () => architectAgent(p, proj,
    'Write your architecture doc to ' + docs(p) + '/3-architecture-' + proj + '.md. Architecture doc ONLY — no code stubs; the developer derives their work queue from your doc.')))
  archs.forEach(r => take(flags, r))
  if (archs.some(bad)) return false

  // Mechanical detection: schema signals → db-admin; UI/visual work → ui-ux designer.
  // INSTALL: adapt the grep signals to match your project's ORM/schema conventions.
  const det = await resilient(
    'Mechanical detection for pipeline ' + p.pipelineName + ' (report only, change nothing): grep ' + docs(p) + '/1-plan.md and ' + docs(p) + '/3-architecture*.md for ' +
    '(a) schema signals — table, schema, column, index, enum, migration, database — dbAdmin=true if ANY hit; ' +
    '(b) frontend visual/UI work — uiUx=true if the plan includes UI visual tasks.' + (routing.some(p => p.includes('fe') || p.includes('web') || p.includes('ui')) ? '' : ' uiUx must be false — no UI project in routing.'),
    { label: p.pipelineName + ':detect', phase: p.pipelineName, model: 'haiku', schema: DETECT }
  )
  const extras = []
  if (det && det.uiUx) extras.push(() => resilient(
    'You are the UI/UX designer. Read and follow {project-ui}/.claude/agents/ui-ux.md. ' + header(p) +
    ' Read ' + docs(p) + '/3-architecture.md and ' + docs(p) + '/3-architecture-{project-ui}.md. Write your spec to ' + docs(p) + '/4-ui-ux-spec.md.' + COMMON,
    { label: p.pipelineName + ':ui-ux', phase: p.pipelineName, model: 'opus', schema: STATUS }))
  if (det && det.dbAdmin) extras.push(() => resilient(
    'You are the database admin. Read and follow {project-db}/.claude/agents/db-admin.md. ' + header(p) +
    ' Read ' + docs(p) + '/1-plan.md + 3-architecture docs. Worktrees: ' + wt(p) + '/{project-db} (and other routed projects as applicable). Implement schema changes from the architecture docs. ' +
    'Verify the migration slot is free (ls the worktree schema dir) before numbering. ' +
    'Every column in the schema definition MUST have a corresponding SQL migration — run your column-level completeness check before finishing, it is BLOCKING. ' +
    'Write your database architecture doc to ' + docs(p) + '/4-db-architecture.md.' + COMMON,
    { label: p.pipelineName + ':db-admin', phase: p.pipelineName, model: 'opus', schema: STATUS }))
  if (extras.length) {
    const x = await parallel(extras)
    x.forEach(r => take(flags, r))
    if (x.some(bad)) return false
  }

  const devs = await parallel(routing.map(proj => () => devAgent(p, proj,
    'Implement per ' + docs(p) + '/1-plan.md, 3-architecture.md and 3-architecture-' + proj + '.md (+ 4-db-architecture.md / 4-ui-ux-spec.md if present). ' +
    'Write your report to ' + docs(p) + '/5-dev-report-' + proj + '.md.')))
  devs.forEach(r => take(flags, r))
  return !devs.some(bad)
}

function qaRound(p, routing, iter) {
  return parallel(routing.map(proj => () => resilient(
    'Mode: PRE-MERGE. ' + header(p) + ' Worktree: ' + wt(p) + '/{project-prefix}-' + proj + '.' +
    ' Run every infra make target from the WORKTREE infra — make -C ' + wt(p) + '/{INFRA_PROJECT} ... — worktree-only migrations must reach the test template.' + TEST_INFRA +
    ' Write findings into ' + docs(p) + '/6-bugs.md under a `## ' + proj.toUpperCase() + '` section (create the file if absent) — own only that section, never touch another\'s; your structured return must match what you wrote there. ' +
    'Wrap every test command in timeout 600s.' + COMMON +
    ' Structured output: status NONE|OPEN for your section; bugIds = stable ids of bugs still OPEN; hungTest = true if any test deadlocked at 0% CPU for >2 minutes (kill it and report BUG-HUNG-TEST in your section).',
    { label: p.pipelineName + ':qa-' + proj + '-r' + iter, phase: p.pipelineName, agentType: 'qa-' + proj, schema: QA }
  )))
}

// Step 7 + Fix Loop — caps and escalation triggers per build.md § Fix Loop
async function gateStage(p, routing, flags) {
  let prev = null
  for (let iter = 0; iter <= 3; iter++) {
    const qa = await qaRound(p, routing, iter)
    qa.forEach(r => take(flags, r))
    if (qa.some(r => r === null)) return 'sub-agent-orphan'
    if (qa.some(r => r.hungTest)) return 'hung-test'
    if (qa.every(r => r.status === 'NONE')) return null
    const cur = new Set(qa.flatMap(r => r.bugIds))
    if (prev && [...cur].some(id => prev.has(id))) return 'repeat-bug'
    if (iter === 3) return 'iteration-cap'
    prev = cur
    log(p.pipelineName + ': fix loop ' + (iter + 1) + '/3 — bugs open, spawning developers')
    const fixes = await parallel(routing.map(proj => () => devAgent(p, proj,
      'Fix loop: read ' + docs(p) + '/6-bugs.md — fix every bug with Status OPEN in your `## ' + proj.toUpperCase() + '` section; the failing adversarial test is the reproduction. ' +
      'If your section has no open bugs, return immediately with summary "no open bugs for ' + proj + '". Wrap test runs in timeout 600s.')))
    if (fixes.some(r => r === null)) return 'sub-agent-orphan'
  }
  return 'iteration-cap'
}

// Pre-merge hygiene gate — cap 2 per build.md § Code Review
async function reviewStage(p, routing, flags) {
  for (let iter = 0; iter <= 2; iter++) {
    const last = iter === 2
    const rev = await resilient(
      header(p) + ' Pre-merge hygiene gate: read .claude/skills/p:audit:code-hygiene/SKILL.md and execute it with scope `diff` over the changed set from ' +
      '`git -C ' + wt(p) + ' diff --name-only main...pipeline/' + p.pipelineName + '` (read-only git permitted for this audit only). ' +
      'Scope strictly to that committed three-dot range. Category 8 (Duplication) first. ' +
      'Write findings to ' + docs(p) + '/6-code-review.md ending with a verdict line.' +
      (last ? ' Final pass: if findings remain, move them under `## Residual` and return verdict RESIDUAL.' : '') +
      ' Structured output: verdict; projects = project keys named in findings.',
      { label: p.pipelineName + ':code-review-r' + iter, phase: p.pipelineName, model: 'opus', schema: REVIEW }
    )
    if (!rev) return 'FAIL'
    take(flags, rev)
    if (rev.verdict !== 'FINDINGS') return rev.verdict
    const affected = rev.projects.length ? rev.projects : routing
    await parallel(affected.map(proj => () => architectAgent(p, proj,
      'Read ' + docs(p) + '/6-code-review.md. For each finding in your project decide the fix — which existing symbol to reuse, where to extract the shared helper, which copy to delete. ' +
      'Append a `## Fix Plan` section. Decisions only, no code edits.')))
    await parallel(affected.map(proj => () => devAgent(p, proj,
      'Apply every fix for your project in the `## Fix Plan` of ' + docs(p) + '/6-code-review.md. Re-run your project tests (timeout 600s) — the worktree must stay test-green.')))
  }
  return 'RESIDUAL'
}

// Steps 8–11 per build.md § Merge Phase / Post-Merge / Documentation
async function shipStage(p, routing, flags) {
  const projectsCsv = routing.join(',')
  const m = await resilient(
    'Pipeline: ' + p.pipelineName + '. Wave: ' + args.waveName + '. Phase: MERGE. Projects: ' + projectsCsv + '. Structured output: status, sha = merge commit sha.',
    { label: p.pipelineName + ':gitter-merge', phase: p.pipelineName, agentType: 'gitter', schema: MERGE }
  )
  if (bad(m)) return { status: 'MERGE-FAILED', detail: m ? m.summary : 'gitter merge agent died twice' }
  take(flags, m)

  const pm = await parallel(routing.map(proj => () => resilient(
    'Mode: POST-MERGE. Pipeline: ' + p.pipelineName + '. Run against {project-prefix}-' + proj + '/ on main (NOT the worktree), infra targets via make -C {INFRA_PROJECT}.' + TEST_INFRA +
    ' Pipeline docs: docs/dev/builds/' + p.pipelineName + '/.' +
    (RUNBOOK[proj] ? ' Runbook: ' + RUNBOOK[proj] + '.' : '') + ' Wrap every test command in timeout 600s.',
    { label: p.pipelineName + ':pmqa-' + proj, phase: p.pipelineName, agentType: 'qa-' + proj, schema: PMQA }
  )))
  pm.forEach(r => take(flags, r))
  await resilient(
    'Scribe task (mechanical write, no judgment): write ' + docs(p) + '/7-post-merge-qa.md consolidating these post-merge QA results for pipeline ' + p.pipelineName +
    ' — one section per project with status and summary: ' + JSON.stringify(routing.map((proj, i) => ({ project: proj, result: pm[i] }))),
    { label: p.pipelineName + ':pmqa-scribe', phase: p.pipelineName, model: 'haiku', schema: STATUS }
  )
  if (pm.some(r => !r || r.status === 'FAIL')) return { status: 'POSTMERGE-FIX-NEEDED', sha: m.sha }

  await resilient(
    'Pipeline: ' + p.pipelineName + '. Phase: ARCHIVE. Epic: ' + args.epicName + '. Docs: ' + docs(p) + '. Wave-owned build — skip the epic write; the wave consolidates it.',
    { label: p.pipelineName + ':documenter', phase: p.pipelineName, agentType: 'mono-documenter', schema: STATUS }
  )
  await resilient(
    'Pipeline: ' + p.pipelineName + '. Wave: ' + args.waveName + '. Phase: DOCS-COMMIT. Projects: ' + projectsCsv + '. Archive: none.',
    { label: p.pipelineName + ':gitter-docs', phase: p.pipelineName, agentType: 'gitter', schema: STATUS }
  )
  return { status: 'DONE', sha: m.sha }
}

async function blockPipeline(p, trigger) {
  deferred.add(p.pipelineName)
  await resilient(
    header(p) + ' The fix loop escalated with trigger `' + trigger + '`. Write ' + docs(p) + '/BLOCKED.md exactly per the template in ' +
    'docs/commands/build/references/build-reference.md § BLOCKED.md — status BLOCKED-DEFERRED, the trigger, root cause from ' + docs(p) + '/6-bugs.md, state preserved, resume protocol. ' +
    'Date: ' + args.timestamp + '. Do NOT delete the worktree, do NOT touch git, do NOT release ports.',
    { label: p.pipelineName + ':blocked-md', phase: p.pipelineName, model: 'sonnet', schema: STATUS }
  )
  return { pipeline: p.pipelineName, status: 'BLOCKED-DEFERRED', trigger }
}

// Durable mid-wave state: append each pipeline outcome to STATE.md as it lands (serialized — no write races).
function scribed(p, result) {
  const line = '- ' + args.timestamp + ' wave-workflow: ' + p.pipelineName + ' → ' + result.status +
    (result.sha ? ' (merge ' + result.sha + ')' : '') +
    (result.trigger ? ' (trigger: ' + result.trigger + ')' : '') +
    (result.dependsOn ? ' (depends on ' + result.dependsOn + ')' : '') +
    (result.flags && result.flags.length ? ' — flags: ' + result.flags.join(' | ') : '')
  return exclusive(() => agent(
    'Scribe task (mechanical append, no judgment): in docs/dev/waves/' + args.waveName + '/STATE.md, append exactly this line at the very END of the file — it is below the append-only marker; modify NOTHING above it:\n' + line,
    { label: p.pipelineName + ':state-scribe', phase: p.pipelineName, model: 'haiku', schema: STATUS }
  )).then(() => result, () => result)
}

async function runPipeline(p) {
  const flags = []
  const dep = (p.dependsOn || []).find(d => deferred.has(d))
  if (dep) {
    deferred.add(p.pipelineName)
    log(p.pipelineName + ': skipped — depends on deferred/failed ' + dep)
    return scribed(p, { pipeline: p.pipelineName, status: 'SKIPPED-DEPENDENCY', dependsOn: dep })
  }

  log(p.pipelineName + ': planning')
  const plan = await planStage(p)
  if (!plan) { deferred.add(p.pipelineName); return scribed(p, { pipeline: p.pipelineName, status: 'FAILED', detail: 'planning stage died', flags }) }
  const routing = plan.routing && plan.routing.length ? plan.routing : (p.routing || [])
  if (!routing.length) { deferred.add(p.pipelineName); return scribed(p, { pipeline: p.pipelineName, status: 'FAILED', detail: 'no routing resolved', flags }) }

  const setup = await exclusive(() => resilient(
    'Pipeline: ' + p.pipelineName + '. Phase: SETUP. CarryWIP: ' + args.carryWip + '.',
    { label: p.pipelineName + ':gitter-setup', phase: p.pipelineName, agentType: 'gitter', schema: STATUS }
  ))
  if (bad(setup)) { deferred.add(p.pipelineName); return scribed(p, { pipeline: p.pipelineName, status: 'FAILED', detail: 'gitter SETUP failed', flags }) }

  log(p.pipelineName + ': building (' + routing.join(', ') + ')')
  const built = await buildStage(p, plan, routing, flags)
  if (!built) return scribed(p, { ...(await blockPipeline(p, 'sub-agent-orphan')), flags })

  log(p.pipelineName + ': QA gate (serialized — shared test infra)')
  const blocked = await exclusive(() => gateStage(p, routing, flags))
  if (blocked) return scribed(p, { ...(await blockPipeline(p, blocked)), flags })
  const review = await exclusive(() => reviewStage(p, routing, flags))
  if (review === 'FAIL') return scribed(p, { ...(await blockPipeline(p, 'sub-agent-orphan')), flags })

  log(p.pipelineName + ': shipping (serialized merge tail)')
  const ship = await exclusive(() => shipStage(p, routing, flags))
  if (ship.status !== 'DONE' && ship.status !== 'POSTMERGE-FIX-NEEDED') deferred.add(p.pipelineName)
  return scribed(p, { pipeline: p.pipelineName, status: ship.status, sha: ship.sha, codeReview: review, detail: ship.detail, flags })
}

const results = []
for (let gi = 0; gi < args.groups.length; gi++) {
  const group = args.groups[gi]
  log('Group ' + (gi + 1) + '/' + args.groups.length + ': ' + group.map(p => p.pipelineName).join(', '))
  const groupResults = await parallel(group.map(p => () => runPipeline(p)))
  groupResults.forEach((r, i) => {
    if (!r) deferred.add(group[i].pipelineName)
    results.push(r || { pipeline: group[i].pipelineName, status: 'FAILED', detail: 'pipeline runner crashed' })
  })
}
return { wave: args.waveName, results }
