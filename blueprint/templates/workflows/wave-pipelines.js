export const meta = {
  name: 'wave-pipelines',
  description: 'Wave group scheduler — sequences groups, runs pipelines within a group in parallel via the wave-build workflow, handles dependsOn deferral and the durable STATE.md scribe. Per-pipeline build mechanics live in the wave-build workflow.',
  phases: [{title:'Setup'},{title:'Plan'},{title:'Architecture'},{title:'Develop'},{title:'QA'},{title:'Code Review'},{title:'GATE-1'},{title:'Merge'},{title:'Post-Merge'},{title:'Docs'}],
}

// args arrive verbatim from docs/dev/waves/{waveName}/workflow.json — template + field law in /wave Step 0d:
// { waveName, epicName, carryWip, timestamp, total,
//   groups: [[{ pipelineName, idx, description, routing: ['{project}',...], dependsOn: [] }]] }
// The harness may deliver `args` JSON-STRING-encoded instead of parsed — parse before validating,
// else the guard below throws "requires args.groups" before any workflow runs.
// Flow graph + spawn briefs are declared copies of .claude/commands/wave/build.md § Wave workflow mode — update both together.

if (typeof args === 'string') {
  try { args = JSON.parse(args) } catch (e) {
    throw new Error('wave-pipelines: args arrived as a string but is not valid JSON: ' + e.message)
  }
}

if (!args || !Array.isArray(args.groups) || !args.groups.length) {
  throw new Error('wave-pipelines requires args.groups — see /wave Step 1 for the invocation contract')
}

const tag = p => args.total > 1 ? ' · ' + p.pipelineName : ''

// One exclusive lock ONLY serializes STATE.md scribes — parallel children within a group would
// race the single STATE.md file. The per-pipeline build (wave-build) owns all other serialization.
let lock = Promise.resolve()
function exclusive(fn) {
  const run = lock.then(fn, fn)
  lock = run.then(() => {}, () => {})
  return run
}

const deferred = new Set()

// Durable mid-wave state: append each pipeline outcome to STATE.md as it lands (serialized — no write races).
function scribed(p, result) {
  const line = '- ' + args.timestamp + ' wave-workflow: ' + p.pipelineName + ' → ' + result.status +
    (result.sha ? ' (merge ' + result.sha + ')' : '') +
    (result.trigger ? ' (trigger: ' + result.trigger + ')' : '') +
    (result.dependsOn ? ' (depends on ' + result.dependsOn + ')' : '') +
    (result.flags && result.flags.length ? ' — flags: ' + result.flags.join(' | ') : '')
  return exclusive(() => agent(
    'Scribe task (mechanical append, no judgment): in docs/dev/waves/' + args.waveName + '/STATE.md, append exactly this line at the very END of the file — it is below the append-only marker; modify NOTHING above it:\n' + line,
    { label: 'state-scribe' + tag(p), phase: 'Post-Merge', model: 'haiku', schema: { type: 'object', properties: { status: { type: 'string', enum: ['OK', 'FAIL'] }, summary: { type: 'string' } }, required: ['status', 'summary'] } }
  )).then(() => result, () => result)
}

async function runPipeline(p) {
  const dep = (p.dependsOn || []).find(d => deferred.has(d))
  if (dep) {
    deferred.add(p.pipelineName)
    log(p.pipelineName + ': ⊘ skipped · depends on deferred/failed ' + dep)
    return scribed(p, { pipeline: p.pipelineName, status: 'SKIPPED-DEPENDENCY', dependsOn: dep })
  }

  const r = await workflow('wave-build', {
    pipelineName: p.pipelineName,
    idx: p.idx,
    total: args.total,
    description: p.description,
    routing: p.routing,
    dependsOn: p.dependsOn,
    waveName: args.waveName,
    epicName: args.epicName,
    carryWip: args.carryWip,
    timestamp: args.timestamp,
  })

  if (!r || ['FAILED', 'BLOCKED-DEFERRED', 'MERGE-FAILED'].includes(r.status)) {
    deferred.add(p.pipelineName)
  }
  return scribed(p, r || { pipeline: p.pipelineName, status: 'FAILED', detail: 'wave-build child died' })
}

const results = []
for (let gi = 0; gi < args.groups.length; gi++) {
  const group = args.groups[gi]
  log('▶ Group ' + (gi + 1) + '/' + args.groups.length + ' · ' + group.length + ' pipelines: ' + group.map(p => p.pipelineName).join(', '))
  const groupResults = await parallel(group.map(p => () => runPipeline(p)))
  groupResults.forEach((r, i) => {
    if (!r) deferred.add(group[i].pipelineName)
    results.push(r || { pipeline: group[i].pipelineName, status: 'FAILED', detail: 'pipeline runner crashed' })
  })
}
return { wave: args.waveName, results }
