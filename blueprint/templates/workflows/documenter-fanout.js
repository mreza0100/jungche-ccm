export const meta = {
  name: 'documenter-fanout',
  description: 'Documentation consolidation engine — CANONICAL here (documenter.md § Orchestration is the pointer + scope table). Scouts a pipeline/hotfix blast radius into DISJOINT doc scopes (one Sonnet pass), a collector-tier no-op check drops zero-hit scopes pre-spawn, then fans out one spec-execution documenter per scope in parallel, each merging only its own write-set from its scope card. The parallel replacement for the single serial mono-documenter. Invoked for ARCHIVE (a completed pipeline; args.pipelineName + args.docsPath) and JC-UPDATE (a /jc hotfix; args.changeSummary); a small blast radius yields one or two workers, a wide one yields many.',
  phases: [{ title: 'Scout' }, { title: 'Consolidate' }],
}

// args: { mode: 'ARCHIVE'|'JC-UPDATE', pipelineName, docsPath?, epicName?, waveOwned?, changeSummary?, projects?, timestamp }
//  - ARCHIVE   → docsPath is the pipeline's $DOCS dir (0-task, 4-*, 5-dev-report*, legacy 1-plan/3-architecture*, …); epicName/waveOwned gate the epic scope.
//  - JC-UPDATE → changeSummary describes the hotfix; projects lists the touched project keys (no $DOCS pipeline docs).
// The harness may deliver args JSON-STRING-encoded — parse before validating.
// Flow graph + spawn briefs are canonical here — documenter.md § Orchestration is the pointer + scope table; update both together.

if (typeof args === 'string') {
  try { args = JSON.parse(args) } catch (e) {
    throw new Error('documenter: args arrived as a string but is not valid JSON: ' + e.message)
  }
}
if (!args || !args.mode || (args.mode === 'ARCHIVE' && !args.docsPath) || (args.mode === 'JC-UPDATE' && !args.changeSummary)) {
  throw new Error('documenter requires args.mode plus ARCHIVE→docsPath or JC-UPDATE→changeSummary; see documenter.md § Orchestration for the contract')
}

const CMD = '.claude/commands/documenter.md'

// One scope = one DISJOINT write-set. key partitions the doc tree so no two workers touch the same file.
const SCOPE = {
  type: 'object', properties: {
    key: { type: 'string' },            // INSTALL: one roster key per project ({project-a}|{project-b}|…) plus root-arch|root-api|root-map|root-features|root-db|epic
    steps: { type: 'string' },          // the scope card path: docs/commands/documenter/references/scopes/{key}.md
    writeTargets: { type: 'string' },    // the doc paths this scope OWNS — must not overlap any other scope
    sources: { type: 'array', items: { type: 'string' } }, // pipeline docs / changed files feeding it
    note: { type: 'string' },
  }, required: ['key', 'steps', 'writeTargets'],
}
const SCOUT = {
  type: 'object', properties: {
    scopes: { type: 'array', items: SCOPE },
    summary: { type: 'string' },
  }, required: ['scopes'],
}
const STATUS = {
  type: 'object', properties: {
    status: { type: 'string', enum: ['OK', 'SKIP', 'FAIL'] },
    summary: { type: 'string' },
  }, required: ['status', 'summary'],
}

// Respawn-once on silent agent death (house pattern from wave-build). Scout + workers are idempotent —
// a worker rewrites its own current-state docs from the same sources; a retry redoes the slice from scratch.
async function resilient(prompt, opts) {
  let r = await agent(prompt, opts)
  if (r === null) {
    log('⚠ ' + (opts.label || 'agent') + ' died silently · respawning once')
    r = await agent('RESUME: a prior agent for this exact role died mid-task. Redo it from scratch — the task is idempotent. ' + prompt, { ...opts, label: (opts.label || 'agent') + '-retry' })
  }
  return r
}

const sourceBrief = args.mode === 'ARCHIVE'
  ? 'Mode ARCHIVE. Pipeline ' + args.pipelineName + ' just shipped; its decisions live in ' + args.docsPath + '/ (0-task.md, 4-*.md, 5-dev-report-*.md, 6-*.md, 7-post-merge-qa.md; legacy trails: 1-plan.md, 3-architecture*.md — read only what exists). ' +
    'Epic scope: ' + (args.waveOwned ? 'EXCLUDE it — this is a wave-owned build and the wave consolidates the epic.' : 'include an `epic` scope only when epicName is set (' + (args.epicName || 'none') + ') and resolves to an IN_PROGRESS manifest.')
  : 'Mode JC-UPDATE. A /jc hotfix shipped on main: ' + args.changeSummary + '. Touched projects: ' + ((args.projects || []).join(', ') || 'derive from `git diff` of the last commit') + '. There is no pipeline $DOCS dir — verify the blast radius against the changed source itself (read-only git diff is fine). No epic scope in JC-UPDATE.'

function scoutAgent() {
  return resilient(
    'Read ' + CMD + ' § Orchestration (the scope partition table) and the matching mode section (' + args.mode + '). ' + sourceBrief + ' ' +
    'Examine the blast radius and return ONLY the scopes this change actually touches — each with `steps` = its scope card path (docs/commands/documenter/references/scopes/{key}.md), its DISJOINT `writeTargets` (no two scopes may name the same file), and the `sources` that feed it. ' +
    'A small change is one or two scopes; do not manufacture scopes to parallelize. Read-only: change no docs.' +
    ' Structured output: scopes (the manifest), summary.',
    { label: 'doc-scout', phase: 'Scout', agentType: 'mono-documenter', schema: SCOUT }
  )
}

// DOC_BRIEF — the per-scope worker brief (single copy; the scope cards + doc-approval.md carry the contract).
const DOC_BRIEF = (s, sourceLine) =>
  'Per-scope doc consolidation. Read your two cards FIRST, then execute: ' +
  '(1) docs/commands/documenter/references/scopes/' + s.key + '.md — your merge steps and write set (yours alone; writing outside it is a write race); ' +
  '(2) docs/commands/documenter/references/doc-approval.md — write rules, sacred boundaries, Approval gate (emit APPROVED:{path} or fix-and-recheck), finish steps. ' +
  'Scope manifest: ' + JSON.stringify(s) + '. ' + sourceLine +
  ' Structured output: status (OK | SKIP if nothing in your scope actually changed | FAIL), summary (the files you wrote).'

function workerAgent(s) {
  const sourceLine = args.mode === 'ARCHIVE' ? 'Mode ARCHIVE — pipeline docs: ' + args.docsPath + '/.' : 'Mode JC-UPDATE — hotfix: ' + args.changeSummary + '.'
  return resilient(DOC_BRIEF(s, sourceLine), { label: 'doc · ' + s.key, phase: 'Consolidate', model: 'sonnet', schema: STATUS })
}

log('Documenter ' + args.mode + ': scouting ' + (args.mode === 'ARCHIVE' ? args.docsPath : 'hotfix') + ' blast radius')
const scout = await scoutAgent()
if (!scout) return { status: 'FAILED', detail: 'doc-scout died twice', mode: args.mode }
const scopes = (scout.scopes || []).filter(Boolean)
if (!scopes.length) { log('⚠ scout found no touched doc scopes — nothing to consolidate'); return { status: 'DONE', mode: args.mode, scopes: 0, summary: 'no doc changes' } }
log('Doc scopes: ' + scopes.length + ' · ' + scopes.map(s => s.key).join(', '))

const NOOP = { type: 'object', properties: { drop: { type: 'array', items: { type: 'string' } }, keep: { type: 'array', items: { type: 'string' } } }, required: ['drop', 'keep'] }

// No-op detector — collector-tier, mechanical grep, no judgment. Zero symbol hits + no new-surface signal → drop the scope
// before paying a worker spawn. Doubt, detector death, or `epic` → keep.
let live = scopes
if (scopes.length > 1) {
  const noop = await agent(
    'Mechanical zero-hit check (report only, change nothing): sources = ' +
    (args.mode === 'ARCHIVE' ? args.docsPath + '/ dev reports + 0-task.md' : 'the hotfix diff (' + args.changeSummary + ')') + '. ' +
    'For each scope, extract the changed code/DB/{API_PROTOCOL} symbols from its sources and grep its writeTargets doc paths for them. ' +
    'DROP a scope ONLY when zero symbols hit its docs AND its sources add no new endpoint/table/component/feature/flow needing a NEW entry; any doubt → keep. Never drop `epic`. ' +
    'Scopes: ' + JSON.stringify(scopes.map(s => ({ key: s.key, writeTargets: s.writeTargets, sources: s.sources }))) +
    '. Structured output: drop (scope keys to skip), keep.',
    { label: 'noop-detect', phase: 'Consolidate', model: 'haiku', schema: NOOP }
  )
  if (noop && Array.isArray(noop.drop) && noop.drop.length) {
    live = scopes.filter(s => s.key === 'epic' || !noop.drop.includes(s.key))
    if (live.length < scopes.length) log('No-op scopes dropped pre-spawn: ' + scopes.filter(s => !live.includes(s)).map(s => s.key).join(', '))
  }
}

const results = (await parallel(live.map(s => () => workerAgent(s)))).filter(Boolean)
const failed = results.filter(r => r.status === 'FAIL')
log('Documenter ' + args.mode + ' complete · ' + results.filter(r => r.status === 'OK').length + ' merged · ' + failed.length + ' failed')
return {
  status: failed.length ? 'PARTIAL' : 'DONE',
  mode: args.mode,
  scopes: live.length,
  merged: results.filter(r => r.status === 'OK').map(r => r.summary),
  failed: failed.map(r => r.summary),
}
