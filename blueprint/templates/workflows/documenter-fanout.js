export const meta = {
  name: 'documenter-fanout',
  description: 'Documentation consolidation engine — scouts a pipeline/hotfix blast radius into DISJOINT doc scopes (one Sonnet pass), then fans out one Sonnet documenter per scope in parallel, each merging only its own write-set. The parallel replacement for the single serial mono-documenter. Invoked for ARCHIVE (a completed pipeline) and JC-UPDATE (a /jc hotfix); a small blast radius yields one or two workers, a wide one yields many. Flow graph is a declared copy of .claude/commands/documenter.md § Orchestration.',
  phases: [{ title: 'Scout' }, { title: 'Consolidate' }],
}

// args: { mode: 'ARCHIVE'|'JC-UPDATE', pipelineName, docsPath?, epicName?, waveOwned?, changeSummary?, projects?, timestamp }
//  - ARCHIVE   → docsPath is the pipeline's $DOCS dir (1-plan, 3-architecture*, 5-dev-report*, …); epicName/waveOwned gate the epic scope.
//  - JC-UPDATE → changeSummary describes the hotfix; projects lists the touched project keys (no $DOCS pipeline docs).
// The harness may deliver args JSON-STRING-encoded — parse before validating.
// Flow graph + spawn briefs are a declared copy of .claude/commands/documenter.md § Orchestration — update both together.

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
    steps: { type: 'string' },          // the documenter.md merge steps this scope runs (e.g. "2b + 2h + 2i-{project}")
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
  ? 'Mode ARCHIVE. Pipeline ' + args.pipelineName + ' just shipped; its decisions live in ' + args.docsPath + '/ (1-plan.md, 3-architecture*.md, 4-*.md, 5-dev-report-*.md, 6-*.md, 7-post-merge-qa.md — read only what exists). ' +
    'Epic scope: ' + (args.waveOwned ? 'EXCLUDE it — this is a wave-owned build and the wave consolidates the epic.' : 'include an `epic` scope only when epicName is set (' + (args.epicName || 'none') + ') and resolves to an IN_PROGRESS manifest.')
  : 'Mode JC-UPDATE. A /jc hotfix shipped on main: ' + args.changeSummary + '. Touched projects: ' + ((args.projects || []).join(', ') || 'derive from `git diff` of the last commit') + '. There is no pipeline $DOCS dir — verify the blast radius against the changed source itself (read-only git diff is fine). No epic scope in JC-UPDATE.'

function scoutAgent() {
  return resilient(
    'Read ' + CMD + ' § Orchestration (the scope partition table) and the matching mode section (' + args.mode + '). ' + sourceBrief + ' ' +
    'Examine the blast radius and return ONLY the scopes this change actually touches — each with its exact documenter.md `steps`, its DISJOINT `writeTargets` (no two scopes may name the same file), and the `sources` that feed it. ' +
    'A small change is one or two scopes; do not manufacture scopes to parallelize. Read-only: change no docs.' +
    ' Structured output: scopes (the manifest), summary.',
    { label: 'doc-scout', phase: 'Scout', agentType: 'mono-documenter', schema: SCOUT }
  )
}

function workerAgent(s) {
  return resilient(
    'BEFORE you touch any document, read and apply .claude/commands/quality/doc.md — the doc-quality contract that governs every doc you write (cluster model, ≤500-line topic files, _index format, grep-true naming, current-state-only, no byline). ' +
    'Then read ' + CMD + ' — it is your merge spec — and execute the ' + args.mode + ' merge for ONLY this one scope, writing ONLY its targets; another worker owns every other scope, so touching anything outside your write-set is a write race. ' +
    'Scope: ' + JSON.stringify(s) + '. ' + (args.mode === 'ARCHIVE' ? 'Pipeline docs: ' + args.docsPath + '/. ' : 'Hotfix: ' + args.changeSummary + '. ') +
    'Permanent docs are CURRENT-STATE, not changelogs: add what this change introduced and delete/rewrite what it removed or renamed. ' +
    'After writing, re-run the /quality:doc Approval gate over every doc you touched (emit APPROVED:{path} or fix-and-recheck), then `npx prettier --write --prose-wrap preserve` each file you wrote. Write no other files; run no git.' +
    ' Structured output: status (OK | SKIP if nothing in your scope actually changed | FAIL), summary (the files you wrote).',
    { label: 'doc · ' + s.key, phase: 'Consolidate', model: 'sonnet', schema: STATUS }
  )
}

log('Documenter ' + args.mode + ': scouting ' + (args.mode === 'ARCHIVE' ? args.docsPath : 'hotfix') + ' blast radius')
const scout = await scoutAgent()
if (!scout) return { status: 'FAILED', detail: 'doc-scout died twice', mode: args.mode }
const scopes = (scout.scopes || []).filter(Boolean)
if (!scopes.length) { log('⚠ scout found no touched doc scopes — nothing to consolidate'); return { status: 'DONE', mode: args.mode, scopes: 0, summary: 'no doc changes' } }
log('Doc scopes: ' + scopes.length + ' · ' + scopes.map(s => s.key).join(', '))

const results = (await parallel(scopes.map(s => () => workerAgent(s)))).filter(Boolean)
const failed = results.filter(r => r.status === 'FAIL')
log('Documenter ' + args.mode + ' complete · ' + results.filter(r => r.status === 'OK').length + ' merged · ' + failed.length + ' failed')
return {
  status: failed.length ? 'PARTIAL' : 'DONE',
  mode: args.mode,
  scopes: scopes.length,
  merged: results.filter(r => r.status === 'OK').map(r => r.summary),
  failed: failed.map(r => r.summary),
}
