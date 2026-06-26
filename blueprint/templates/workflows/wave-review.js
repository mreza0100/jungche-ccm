export const meta = {
  name: 'wave-review',
  description: 'Post-wave review workflow — scouts the wave\'s integrated diff into threads, then one agent per thread walks it end-to-end for BOTH functional correctness and code-hygiene (merged in a single read), and a synthesizer folds in the operational review and consolidates cross-pipeline duplication into the Professor\'s Wave Review. Read-only except the synthesizer\'s review write. Invoked by /wave Step 3 and standalone /wave:review.',
  phases: [{ title: 'Scout' }, { title: 'Walk' }, { title: 'Synthesize' }],
}

// args: { reportPath, waveName? } — reportPath is the wave's report.md (grouping, merge SHAs, JC pre-flight).
// The harness may deliver args JSON-STRING-encoded — parse before validating.
// Flow graph + spawn briefs are a declared copy of .claude/commands/wave/review.md § Orchestration — update both together.

if (typeof args === 'string') {
  try { args = JSON.parse(args) } catch (e) {
    throw new Error('wave-review: args arrived as a string but is not valid JSON: ' + e.message)
  }
}
if (!args || !args.reportPath) {
  throw new Error('wave-review requires args.reportPath — the wave report.md to review; see wave/review.md for the contract')
}

const REVIEW_DOC = '.claude/commands/wave/review.md'
const RO = ' Read-only: git log/show/diff, Read, Grep only — run no code, write no files, touch no git.'

const SCOUT = {
  type: 'object', properties: {
    threads: {
      type: 'array', items: {
        type: 'object', properties: {
          id: { type: 'string' }, type: { type: 'string' }, name: { type: 'string' },
          scope: { type: 'string' }, files: { type: 'array', items: { type: 'string' } }, verify: { type: 'string' },
        }, required: ['id', 'type', 'name', 'scope', 'verify'],
      },
    },
    changedFiles: { type: 'array', items: { type: 'string' } },
    mergeShas: { type: 'array', items: { type: 'string' } },
  }, required: ['threads', 'changedFiles'],
}

// One walker per thread returns BOTH the functional verdict AND the code-hygiene findings for its
// files — merged, so each changed file is read once. defects = functional breaks; hygiene = quality.
const WALK = {
  type: 'object', properties: {
    threadId: { type: 'string' }, name: { type: 'string' }, type: { type: 'string' },
    flow: { type: 'string', enum: ['INTACT', 'AT-RISK', 'BROKEN', 'N/A'] },
    trace: { type: 'string' },
    defects: {
      type: 'array', items: {
        type: 'object', properties: { what: { type: 'string' }, location: { type: 'string' }, jc: { type: 'string' } }, required: ['what', 'location'],
      },
    },
    hygiene: {
      type: 'array', items: {
        type: 'object', properties: { kind: { type: 'string' }, where: { type: 'string' }, detail: { type: 'string' }, jc: { type: 'string' } }, required: ['kind', 'where', 'detail'],
      },
    },
    notes: { type: 'string' },
  }, required: ['threadId', 'flow', 'trace', 'defects', 'hygiene'],
}

const SYNTH = {
  type: 'object', properties: {
    verdict: { type: 'string', enum: ['SMOOTH SAILING', 'MOSTLY GOOD', 'ROUGH SEAS', 'SHIPWRECK'] },
    actionItems: { type: 'array', items: { type: 'string' } },
    review: { type: 'string' },
  }, required: ['verdict', 'actionItems', 'review'],
}

// Respawn-once on silent agent death (house pattern from wave-build). Roles are read-only and idempotent —
// a retry redoes the work from scratch; the synthesizer's retry simply rewrites the review section.
async function resilient(prompt, opts) {
  let r = await agent(prompt, opts)
  if (r === null) {
    log('⚠ ' + (opts.label || 'agent') + ' died silently · respawning once')
    r = await agent('RESUME: a prior agent for this exact role died mid-task. Redo it from scratch — the task is idempotent. ' + prompt, { ...opts, label: (opts.label || 'agent') + '-retry' })
  }
  return r
}

function scoutAgent() {
  return resilient(
    'Read ' + REVIEW_DOC + ' § Role: Scout, then enumerate the wave\'s threads from the report at ' + args.reportPath + '. ' +
    'Also return the integrated changed-and-generated file set — the union of `git diff {merge}^1 {merge}` for every SUCCEEDED pipeline merge SHA plus any /jc `git show {sha}` — and the merge SHAs you used.' + RO +
    ' Structured output: threads (the manifest, one entry per thread), changedFiles, mergeShas.',
    { label: 'scout', phase: 'Scout', model: 'opus', schema: SCOUT }
  )
}

function walkerAgent(t) {
  return resilient(
    'Read ' + REVIEW_DOC + ' § Role: Walker. Walk this ONE thread end-to-end in a single pass over its files, returning BOTH the functional verdict AND the code-hygiene findings — the two lenses share one read. ' +
    'Per-pipeline hygiene already ran pre-merge (wave/build.md Step 7), so your wave-level hygiene value is the INTEGRATION delta: above all a repo-wide Cat 8 reuse-grep for a helper/type/hook a sibling pipeline duplicated, plus any dead code the integration orphaned. ' +
    'Thread: ' + JSON.stringify(t) + '.' + RO +
    ' Structured output: threadId, name, type, flow (INTACT|AT-RISK|BROKEN, or N/A for a hygiene-only thread), trace (step → step, marking any break), defects (functional breaks, each {what, location=file:line, jc=`/jc {fix}`}), hygiene (quality findings, each {kind, where=file:line, detail, jc=`/jc {fix}` or empty when not a fixable code defect}), notes.',
    { label: 'walk · ' + t.id, phase: 'Walk', model: 'opus', schema: WALK }
  )
}

function synthAgent(walks) {
  return resilient(
    'Read ' + REVIEW_DOC + ' § Role: Synthesizer and § Report Format. Given the report at ' + args.reportPath + ' and these thread-walk findings, ' +
    'run the operational review, fold every walker defect AND every hygiene finding into `### /jc Action Items` (dedup across threads; merge duplication findings — especially CROSS-PIPELINE: the same helper written independently in two pipelines, which no single per-pipeline Step-7 review could see), and WRITE the review into the report under `## Professor\'s Wave Review` per the Report Format. ' +
    'Walks: ' + JSON.stringify(walks) + '. ' +
    'You are the one agent permitted to write — edit ONLY the report file to add the review section; change nothing else, run no code or git. ' +
    'Structured output: verdict (SMOOTH SAILING|MOSTLY GOOD|ROUGH SEAS|SHIPWRECK), actionItems (the `/jc` one-liners verbatim), review (the full markdown you wrote).',
    { label: 'synthesize', phase: 'Synthesize', model: 'opus', schema: SYNTH }
  )
}

log('Wave review: scouting ' + args.reportPath)
const scout = await scoutAgent()
if (!scout) return { status: 'FAILED', detail: 'scout died twice' }
const threads = scout.threads || []
log('Threads: ' + threads.length + ' · integrated changed files: ' + (scout.changedFiles || []).length)
if (!threads.length) log('⚠ scout enumerated no threads — synthesizer will still write the operational review')

const walks = (await parallel(threads.map(t => () => walkerAgent(t)))).filter(Boolean)

const review = await synthAgent(walks)
if (!review) return { status: 'FAILED', detail: 'synthesizer died twice', threads: threads.length }
log('Wave review complete · ' + review.verdict)
return { status: 'DONE', verdict: review.verdict, actionItems: review.actionItems, review: review.review, threads: threads.length }
