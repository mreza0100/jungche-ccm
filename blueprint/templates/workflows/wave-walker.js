export const meta = {
  name: 'wave-walker',
  description: 'Wave Walker — wave verification. Walks the wave\'s diff (merged SHAs, or a pre-merge worktree branch via args.branch) two ways in one pass and folds them. (1) THREAD WALK (the proven thread-walk floor): a scout enumerates feature-flow / seam / invariant threads from the integrated diff, one Sonnet walker per thread confirms the flow reaches its terminal state and catches the integration-delta hygiene. (2) LEDGER SPINE (the mechanical add): the same scout schedules Haiku sensors over the {API_PROTOCOL} type-fields + entry-point gates the diff touches; they extract comparable cards; a zero-token JS rule engine diffs them (orphan producer, phantom consumer, encoding/double-encode mismatch, value-set/casing mismatch, base-type drift, gate-outlier, mandated-fence violation, unfenced ID flow, dangling refs); Sonnet judges only the flagged anomalies, Opus second-opinions killed security/near-certain ones, and one FINAL Opus judge rules the whole walk (authoritative verdict, reinstates wrong kills, names missed cross-cutting risks). A fold merges thread verdicts + confirmed anomalies + hygiene + the final judgment into `## Professor\'s Wave Review` in the report and returns { verdict, actionItems, review }; the ledger travels in the RESULT and the caller persists it. A diff with no {API_PROTOCOL} surface runs pure thread-walk — the floor never regresses. Flow graph is a declared copy of wave/walker.md § Orchestration. (3) SECURITY: the Walk barrier carries one diff-scoped auditor applying audit/security.md (8A–8K) to the wave\'s changed surface; findings ride the final judgment, the review\'s Security Audit section, and the action items. (4) VERIFY MODE (args.claims — no reportPath): skips the walk; a pre-ruling claims panel fact-checks load-bearing claims against named files (one read-only verifier per claim × votes, Sonnet-xhigh pinned, per-claim opus flag) and returns verdicts + evidence for the CALLER to rule over — no file writes. (5) MANIFEST-VERIFY (args.manifestPath): a claim extractor mines the manifest\'s load-bearing claims (hallucinated fields/premises), the panel probes each, and a consistency judge flags cross-task conflicts + refuted premises + freeloader tasks. (6) INVESTIGATE (args.goal) — RR-for-code: lens probes seed a quote-pinned claim ledger; an Opus brainer steers ≤maxWaves of pursue/attack lanes over it (settled REQUIRES a survived challenge); a Haiku auditor greps every quote-pin; status and confidence are COMPUTED from ledger topology, never asserted; a synthesiser writes the cited report with confidence floored by the computed value; every death degrades loudly, never silently.',
  phases: [{ title: 'Scout' }, { title: 'Walk' }, { title: 'Judge' }, { title: 'Fold' }, { title: 'Verify' }, { title: 'Investigate' }],
}

// INSTALL: this workflow's ledger spine assumes a typed API surface ({API_PROTOCOL}) with a resolver/entry-point
// layer and a documented role-fence rule ({BACKEND_PROJECT}/CLAUDE.md § Auth Pattern). Fill every {TOKEN} below with
// your install's concrete value. The domain/role/resource literals ({ROLE_USER}, {ROLE_SUPER}, {SUBJECT_NOUN},
// {SESSION_NOUN}, {ORG_UNIT}, {SENSITIVE_DATA}, {AI_PROJECT}) must be replaced consistently — the scout prompts,
// the schema enums, and the rule-engine string comparisons all use the SAME values and must agree post-install.
// A single-project / no-API-surface install runs pure thread-walk; the ledger schedule comes back empty on its own.

// args: { reportPath, branch?, ledgerPath?, sensorModel?, sensorEffort?, scoutModel?, walkerModel?, judgeModel?,
//         digestModel?, foldModel?, securityEscalateModel?, maxFieldsPerJob?, maxSensors?,
//         claims?, manifestPath?, maxClaims?, question?, votes?, verifierModel?, verifierEffort?, securityModel?, securityEffort? }
//   claims — VERIFY MODE (takes precedence; reportPath not required): [{id, statement, files?, context?, opus?}].
//   manifestPath — MANIFEST-VERIFY: extractor mines ≤maxClaims (24) load-bearing claims from the manifest, the panel
//     probes each, a consistency judge flags cross-task conflicts / refuted premises / freeloaders → {verdicts, consensus, conflicts}.
//   securityModel/securityEffort — walk-mode diff-scoped security auditor (audit/security.md 8A–8K), default sonnet/xhigh.
//   goal — INVESTIGATE (RR-for-code): plus scope? (path fence), lenses? (wave-0 probe angles; default DIRECT/SKEPTIC/BLAST-RADIUS),
//     maxWaves? (3), maxLanes? (5), probeModel/probeEffort? (sonnet/xhigh), brainerModel/brainerEffort? (opus/xhigh),
//     auditModel? (haiku), synthModel? (sonnet), reportOut? (file the synthesiser writes the cited report to).
//     Returns {answer, confidence (computed-floored), claims (quote-pinned ledger), openLeads, report, stopReason, degraded}.
//   extraThreads — walk mode: caller-forced threads appended to the scout's manifest (same {id,type,name,files,verify} shape).
//     One independent read-only verifier per claim (× votes, default 1; mechanical majority → consensus, else SPLIT)
//     fact-checks the claim against the repo; verifierModel/verifierEffort default sonnet/xhigh (claim.opus:true runs
//     securityEscalateModel instead — per-claim frontier-hands logic); question = the ruling being grounded (context only).
//     Returns { verdicts, consensus } for the CALLER to rule over — no file writes, no fold.
//   reportPath — REQUIRED. The wave report.md (a **Merge SHA:** line and/or Final Summary merge SHAs, grouping, JC pre-flight). The {reportPath} contract the post-wave review has always used.
//   branch — OPTIONAL manual pre-merge mode: a worktree branch (e.g. 'pipeline/{wave}'); the scout diffs main...branch instead of parsing merge SHAs. /wave:orchestrator invokes merge-SHA mode post-merge (§ O6), concurrent with GATE-2.
//   The ledger travels in the RESULT; the caller (wave/orchestrator.md § O6) persists it to ledgerPath — no agent ferries file bytes.
if (typeof args === 'string') {
  try { args = JSON.parse(args) } catch (e) { throw new Error('wave-walker: args is a string but not valid JSON: ' + e.message) }
}
if (!args || (!args.reportPath && !args.manifestPath && !args.goal && !(Array.isArray(args.claims) && args.claims.length))) throw new Error('wave-walker requires args.reportPath (walk), args.claims (verify), args.manifestPath (manifest-verify), or args.goal (investigate); see wave/walker.md for the contract')

const REPORT_PATH = args.reportPath
const BRANCH = args.branch || null
const LEDGER_PATH = args.ledgerPath || (REPORT_PATH ? REPORT_PATH.replace(/report\.md$/, '') + 'walker-ledger.json' : null)
const WALKER_DOC = '.claude/commands/wave/walker.md'
const SCOUT_MODEL = args.scoutModel || 'sonnet'
const WALKER_MODEL = args.walkerModel || 'sonnet'
const SENSOR_MODEL = args.sensorModel || 'haiku'
const SENSOR_EFFORT = args.sensorEffort || 'medium'
const SENSOR_ESCALATE = 'sonnet'
const JUDGE_MODEL = args.judgeModel || 'sonnet'
const DIGEST_MODEL = args.digestModel || 'sonnet'
const FOLD_MODEL = args.foldModel || 'sonnet'
// FRONTIER-JUDGMENT SEATS — final judge, security second-opinion, investigate brainer. Durable default = the
// 'opus' alias (the durable frontier fallback). A limited-time frontier model rides ONLY the invocation args
// (finalJudgeModel / securityEscalateModel / brainerModel) per root CLAUDE.md § Model Selection;
// never a model literal in this file. Security/auth judgment seats never downgrade below opus.
const SEC_ESCALATE_MODEL = args.securityEscalateModel || 'opus'
const FINAL_JUDGE_MODEL = args.finalJudgeModel || 'opus'
const SECURITY_MODEL = args.securityModel || 'sonnet'
const SECURITY_EFFORT = args.securityEffort || 'xhigh'
const SECURITY_DOC = '.claude/commands/audit/security.md'
const MAX_FIELDS_PER_JOB = Number.isInteger(args.maxFieldsPerJob) ? args.maxFieldsPerJob : 18
const MAX_SENSORS = Number.isInteger(args.maxSensors) ? args.maxSensors : 60

const RO = ' Read-only: Grep/Glob, Read, and git log/show/diff/rev-parse only — run no other code, write no files, mutate no git.'

// ─── VERIFY / MANIFEST-VERIFY — pre-ruling claims panel: no walk, no writes ──
if (args.manifestPath || (Array.isArray(args.claims) && args.claims.length)) {
  const VERIFIER_MODEL = args.verifierModel || 'sonnet'
  const VERIFIER_EFFORT = args.verifierEffort || 'xhigh'
  const OPUS_CLAIM_MODEL = args.securityEscalateModel || 'opus'
  const VOTES = Number.isInteger(args.votes) && args.votes > 0 ? args.votes : 1
  const QUESTION = args.question || ''
  const MAX_CLAIMS = Number.isInteger(args.maxClaims) ? args.maxClaims : 24
  let claims = Array.isArray(args.claims) ? args.claims : []
  let conflictChecks = []
  if (args.manifestPath && !claims.length) {
    const EXTRACT = {
      type: 'object', properties: {
        claims: { type: 'array', items: { type: 'object', properties: { id: { type: 'string' }, taskId: { type: 'string' }, kind: { type: 'string', description: 'existence | behavior | contract | dep' }, statement: { type: 'string', description: 'self-contained, refutable, <=200 chars' }, files: { type: 'array', items: { type: 'string' } }, context: { type: 'string' } }, required: ['id', 'statement'] } },
        conflictChecks: { type: 'array', items: { type: 'object', properties: { id: { type: 'string' }, tasks: { type: 'array', items: { type: 'string' } }, what: { type: 'string' } }, required: ['id', 'what'] } },
      }, required: ['claims', 'conflictChecks'],
    }
    const ex = await resilient(
      'You are the CLAIM EXTRACTOR of a manifest-verify panel. Read the wave manifest at ' + args.manifestPath + ' (repo root {REPO_ROOT}) and mine EVERY load-bearing factual claim a hallucination could hide in — a claim is load-bearing when refuting it would change a task\'s design or scope. Per task extract: existence claims (a named file/symbol/field/column/enum/env var/prompt the task assumes EXISTS or assumes ABSENT — incl. every Named anchor and File-plan path), behavior premises ("X currently does Y" statements the design rests on — the classic hallucination class), contract claims (SDL/{QUEUE}/{REALTIME_PROTOCOL} shapes vs live code), dep claims (cross-task Depends and shared symbols). Each: id T{n}-C{k}, taskId, kind, a SELF-CONTAINED refutable statement, exact files to start probing, 1-line context. ORDER MOST LOAD-BEARING FIRST (a cap may drop the tail). Also emit conflictChecks: task pairs/sets whose File plans, Contracts, or data models might collide (same file EDIT+DELETE, one field two shapes, duplicated work) — checks only, no verdicts.' + RO
      + ' Structured output: claims, conflictChecks.',
      { label: 'extract-claims', phase: 'Verify', model: VERIFIER_MODEL, effort: VERIFIER_EFFORT, schema: EXTRACT },
    )
    if (!ex) return { status: 'FAILED', detail: 'claim extractor died twice' }
    if ((ex.claims || []).length > MAX_CLAIMS) log('⚠ claim cap ' + MAX_CLAIMS + ': DROPPED ' + (ex.claims.length - MAX_CLAIMS) + ' tail claim(s): ' + ex.claims.slice(MAX_CLAIMS).map(c => c.id).join(', '))
    claims = (ex.claims || []).slice(0, MAX_CLAIMS)
    conflictChecks = ex.conflictChecks || []
    log('Extracted ' + claims.length + ' claim(s) · ' + conflictChecks.length + ' conflict check(s) from ' + args.manifestPath)
    if (!claims.length) return { status: 'DONE', mode: 'manifest-verify', manifest: args.manifestPath, claims: 0, verdicts: [], consensus: {}, conflicts: [], verifiersDied: 0 }
  }
  const VERIFY = {
    type: 'object', properties: {
      claimId: { type: 'string' },
      verdict: { type: 'string', enum: ['CONFIRMED', 'REFUTED', 'PARTIAL', 'UNPROVEN'] },
      evidence: { type: 'array', items: { type: 'object', properties: { anchor: { type: 'string' }, quote: { type: 'string', description: 'VERBATIM, <=120 chars' } }, required: ['anchor'] } },
      reasoning: { type: 'string' },
    }, required: ['claimId', 'verdict', 'reasoning'],
  }
  log('Verify mode · ' + claims.length + ' claim(s) × ' + VOTES + ' vote(s) · ' + VERIFIER_MODEL + '/' + VERIFIER_EFFORT)
  const panel = claims.flatMap(c => Array.from({ length: VOTES }, (_, v) => ({ c, v })))
  const results = await parallel(panel.map(({ c, v }) => () => resilient(
    'You are an INDEPENDENT VERIFIER on a pre-ruling claims panel' + (QUESTION ? ' grounding this ruling: ' + QUESTION : '') + '. Repo root: {REPO_ROOT}.\n'
    + 'CLAIM ' + c.id + ': ' + c.statement + '\n'
    + (c.context ? 'Context: ' + c.context + '\n' : '')
    + ((c.files || []).length ? 'Start from these files (follow imports/greps wherever the evidence leads): ' + JSON.stringify(c.files) + '\n' : '')
    + 'Actively try to REFUTE the claim — hunt for the counterexample before accepting confirmation. CONFIRMED only when file evidence proves it AS STATED; REFUTED when evidence contradicts it; PARTIAL when it holds with a material caveat (state it); UNPROVEN when evidence is unfindable. '
    + 'Every evidence anchor grep-verified file:line with a VERBATIM quote (<=120 chars). Judge evidence, not vibes.' + RO
    + ' Structured output: claimId=' + c.id + ', verdict, evidence, reasoning (<=3 sentences).',
    { label: 'verify · ' + c.id + (VOTES > 1 ? ' #' + (v + 1) : ''), phase: 'Verify', model: c.opus ? OPUS_CLAIM_MODEL : VERIFIER_MODEL, effort: VERIFIER_EFFORT, schema: VERIFY },
  )))
  const verdicts = results.filter(Boolean)
  const consensus = {}
  for (const c of claims) {
    const vs = verdicts.filter(r => r.claimId === c.id)
    if (!vs.length) { consensus[c.id] = 'NO-VERDICT'; continue }
    const tally = vs.reduce((m, r) => { m[r.verdict] = (m[r.verdict] || 0) + 1; return m }, {})
    const top = Object.entries(tally).sort((a, b) => b[1] - a[1])[0]
    consensus[c.id] = (top[1] > vs.length / 2) ? top[0] : 'SPLIT'
  }
  const died = panel.length - verdicts.length
  let conflicts = []
  if (args.manifestPath) {
    const CONFLICT = {
      type: 'object', properties: {
        conflicts: { type: 'array', items: { type: 'object', properties: { id: { type: 'string' }, kind: { type: 'string', description: 'conflict | refuted-premise | freeloader' }, tasks: { type: 'array', items: { type: 'string' } }, what: { type: 'string' }, evidence: { type: 'string' }, severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] }, fix: { type: 'string', description: 'concrete manifest correction' } }, required: ['id', 'kind', 'what', 'severity'] } },
        summary: { type: 'string' },
      }, required: ['conflicts', 'summary'],
    }
    const cj = await resilient(
      'You are the MANIFEST CONSISTENCY JUDGE. Re-read the manifest at ' + args.manifestPath + ', then rule over the panel\'s evidence. Panel verdicts: ' + JSON.stringify(verdicts) + '\nConsensus: ' + JSON.stringify(consensus) + '\nConflict checks queued by the extractor: ' + JSON.stringify(conflictChecks) + '\nFind, evidence-based (open files where needed): (1) kind=conflict — cross-task collisions (File plans touching the same symbols incompatibly, contract shapes disagreeing between tasks, Depends order the file plan violates); (2) kind=refuted-premise — every task step resting on a REFUTED/PARTIAL claim, naming the manifest section it invalidates; (3) kind=freeloader — a task/step that does not earn its place (premise gone, work a sibling task duplicates, scope nothing consumes). Each: tasks, what (Expected/Got), evidence (manifest section + code anchor), severity, fix (concrete manifest correction).' + RO
      + ' Structured output: conflicts, summary.',
      { label: 'conflict-judge', phase: 'Verify', model: VERIFIER_MODEL, effort: VERIFIER_EFFORT, schema: CONFLICT },
    )
    conflicts = cj ? (cj.conflicts || []) : []
    if (cj) log('Consistency: ' + conflicts.length + ' finding(s) — ' + (cj.summary || '').slice(0, 120))
  }
  log('Verify done · ' + Object.entries(consensus).map(([k, x]) => k + '=' + x).join(' · ') + (died ? ' · ⚠ ' + died + ' verifier(s) died' : ''))
  return { status: 'DONE', mode: args.manifestPath ? 'manifest-verify' : 'verify', manifest: args.manifestPath || null, question: QUESTION, claims: claims.length, votes: VOTES, verdicts, consensus, conflicts, verifiersDied: died }
}

// ─── INVESTIGATE — RR-for-code (args.goal): brainer-steered waves over a computed claim ledger ──
// Invariants (from the RR engine design): (1) evidence is a LEDGER, not prose — status/confidence
// computed from topology, never asserted by a model (a model may LOWER confidence, never raise it);
// (2) the script owns every mutation, agents are pure request/response; (3) degrade loudly — a dead
// brainer/synth ships the best surviving deliverable under a DEGRADED flag, never nothing.
if (args.goal) {
  const GOAL = String(args.goal)
  const SCOPE = Array.isArray(args.scope) && args.scope.length ? args.scope : null
  const LENSES = Array.isArray(args.lenses) && args.lenses.length ? args.lenses : ['DIRECT — the goal head-on', 'SKEPTIC — hunt the evidence that would make the obvious answer WRONG', 'BLAST-RADIUS — callers, consumers, config, and tests the goal implicates']
  const MAX_WAVES = Number.isInteger(args.maxWaves) ? args.maxWaves : 3
  const MAX_LANES = Number.isInteger(args.maxLanes) ? args.maxLanes : 5
  const PROBE_MODEL = args.probeModel || 'sonnet'
  const PROBE_EFFORT = args.probeEffort || 'xhigh'
  const BRAINER_MODEL = args.brainerModel || 'opus' // frontier-judgment seat — see the seat note atop the file
  const BRAINER_EFFORT = args.brainerEffort || 'xhigh'
  const AUDIT_MODEL = args.auditModel || 'haiku'
  const SYNTH_MODEL = args.synthModel || 'sonnet'
  const REPORT_OUT = args.reportOut || null
  const scopeLine = SCOPE ? ' SCOPE (stay inside): ' + JSON.stringify(SCOPE) + '.' : ''
  const PROBE = {
    type: 'object', properties: {
      laneId: { type: 'string' },
      claims: { type: 'array', items: { type: 'object', properties: { statement: { type: 'string', description: 'self-contained fact, <=200 chars' }, kind: { type: 'string', description: 'support | counter' }, targets: { type: 'array', items: { type: 'string' }, description: 'counter only: attacked claim ids' }, anchors: { type: 'array', items: { type: 'object', properties: { anchor: { type: 'string' }, quote: { type: 'string', description: 'VERBATIM, <=120 chars' } }, required: ['anchor', 'quote'] } } }, required: ['statement', 'anchors'] } },
      leads: { type: 'array', items: { type: 'object', properties: { what: { type: 'string' }, files: { type: 'array', items: { type: 'string' } } }, required: ['what'] } },
      nothingFound: { type: 'boolean' },
    }, required: ['laneId', 'claims', 'leads'],
  }
  const COORD = {
    type: 'object', properties: {
      resultSoFar: { type: 'string', description: 'best current answer, <=1200 chars' },
      keyClaimIds: { type: 'array', items: { type: 'string' }, description: 'the LOAD-BEARING ledger ids the answer rests on' },
      lanes: { type: 'array', items: { type: 'object', properties: { id: { type: 'string' }, kind: { type: 'string', description: 'pursue | attack' }, question: { type: 'string' }, files: { type: 'array', items: { type: 'string' } }, targets: { type: 'array', items: { type: 'string' }, description: 'attack only: claim ids to challenge' }, note: { type: 'string' } }, required: ['id', 'kind', 'question'] } },
      dropLeads: { type: 'array', items: { type: 'string' } },
      stop: { type: 'object', properties: { done: { type: 'boolean' }, reason: { type: 'string' } }, required: ['done'] },
    }, required: ['resultSoFar', 'keyClaimIds', 'lanes', 'stop'],
  }
  const AUDITS = { type: 'object', properties: { audits: { type: 'array', items: { type: 'object', properties: { id: { type: 'string' }, result: { type: 'string', enum: ['pass', 'fail'] } }, required: ['id', 'result'] } } }, required: ['audits'] }
  const ledger = new Map(), byStmt = new Map(), leads = new Map()
  let cseq = 0, lseq = 0
  const normStmt = s => String(s || '').toLowerCase().replace(/\s+/g, ' ').trim()
  function ingest(probeResults, wave) {
    let fresh = 0
    for (const r of probeResults.filter(Boolean)) {
      const counters = (r.claims || []).filter(c => c.kind === 'counter')
      for (const c of (r.claims || [])) {
        const key = normStmt(c.statement)
        let row = byStmt.get(key)
        if (!row) { row = { id: 'c' + (++cseq), statement: c.statement, anchors: [], files: [], contested: false, survived: 0, audit: 'pending', wave }; byStmt.set(key, row); ledger.set(row.id, row); fresh++ }
        for (const a of (c.anchors || [])) {
          if (a && a.anchor && !row.anchors.some(x => x.anchor === a.anchor)) row.anchors.push({ anchor: a.anchor, quote: a.quote })
          const f = String((a && a.anchor) || '').split(':')[0]
          if (f && !row.files.includes(f)) row.files.push(f)
        }
        if (c.kind === 'counter') for (const t of (c.targets || [])) { const tgt = ledger.get(t); if (tgt) tgt.contested = true }
      }
      if (r._laneKind === 'attack' && (r.nothingFound || counters.length === 0)) for (const t of (r._targets || [])) { const tgt = ledger.get(t); if (tgt) tgt.survived++ }
      for (const l of (r.leads || [])) { const id = 'L' + (++lseq); leads.set(id, { id, what: l.what, files: l.files || [] }) }
    }
    return fresh
  }
  function statusOf(row) {
    if (row.contested) return 'contested'
    if (row.audit === 'fail') return 'tentative'
    if (row.audit === 'pass' && row.files.length >= 2 && (row.survived >= 1 || row.files.length >= 3)) return 'settled'
    return 'tentative'
  }
  function computedConfidence(keyIds) {
    const rows = keyIds.map(id => ledger.get(id)).filter(Boolean)
    if (!rows.length) return 'low'
    if (rows.some(r => statusOf(r) === 'contested' || r.audit === 'fail')) return 'low'
    if (rows.every(r => statusOf(r) === 'settled')) return 'high'
    return 'medium'
  }
  async function auditNew(wave) {
    const rows = [...ledger.values()].filter(r => r.audit === 'pending')
    if (!rows.length) return
    const a = await resilient('You are a CLAIM AUDITOR — you are grepping for a pin, not judging truth. For EACH claim id, open/grep the cited anchor file(s) (repo root {REPO_ROOT}) and verify the VERBATIM quote appears (whitespace-insensitive; within ±5 lines of a cited line number is fine). pass = every quote found; fail = any quote absent. Claims: ' + JSON.stringify(rows.map(r => ({ id: r.id, anchors: r.anchors }))) + RO + ' Structured output: audits (one per claim id).',
      { label: 'audit · w' + wave, phase: 'Investigate', model: AUDIT_MODEL, effort: 'medium', schema: AUDITS })
    if (!a) return
    for (const v of (a.audits || [])) { const r = ledger.get(v.id); if (r) r.audit = v.result }
  }
  function probeAgent(lane) {
    return resilient(
      'You are lane ' + lane.id + ' (' + (lane.kind || 'pursue') + ') of a code investigation. GOAL: ' + GOAL + '.' + scopeLine + '\nQUESTION: ' + lane.question + (lane.note ? ' — steering: ' + lane.note : '') + '\n'
      + ((lane.files || []).length ? 'Start files (follow imports/greps wherever evidence leads): ' + JSON.stringify(lane.files) + '\n' : '')
      + (lane.kind === 'attack' ? 'ATTACK LANE: actively hunt COUNTER-evidence against claim ids ' + JSON.stringify(lane.targets || []) + ' (emit kind=counter with targets). A real hunt that finds NOTHING → nothingFound:true — that survival is first-class evidence, not silence.\n' : '')
      + 'Return quote-pinned claims — SELF-CONTAINED facts, VERBATIM quotes (<=120 chars), grep-verified file:line anchors — plus leads (files/symbols worth a future lane).' + RO
      + ' Structured output: laneId=' + lane.id + ', claims, leads, nothingFound.',
      { label: 'probe · ' + lane.id, phase: 'Investigate', model: PROBE_MODEL, effort: PROBE_EFFORT, schema: PROBE },
    ).then(r => r && Object.assign(r, { _laneKind: lane.kind, _targets: lane.targets || [] }))
  }
  log('Investigate · ' + LENSES.length + ' lenses · ≤' + MAX_WAVES + ' waves × ≤' + MAX_LANES + ' lanes · probes ' + PROBE_MODEL + '/' + PROBE_EFFORT + ' · brainer ' + BRAINER_MODEL)
  const seedLanes = LENSES.map((lens, i) => ({ id: 'w0-' + (i + 1), kind: 'pursue', question: lens }))
  let results = await parallel(seedLanes.map(l => () => probeAgent(l)))
  if (!results.filter(Boolean).length) return { status: 'FAILED', detail: 'all wave-0 probes died — nothing to reason over' }
  ingest(results, 0)
  await auditNew(0)
  let coord = null, stopReason = 'wave-cap', dry = 0
  for (let wave = 1; wave <= MAX_WAVES; wave++) {
    if (budget.total && budget.remaining() < 80000) { stopReason = 'budget'; break }
    coord = await resilient(
      'You are the BRAINER — this investigation\'s only global reasoner. GOAL: ' + GOAL + '.' + scopeLine + ' Wave ' + wave + '/' + MAX_WAVES + '.\n'
      + 'LEDGER (statuses are COMPUTED from topology — cite ids, never assert status): ' + JSON.stringify([...ledger.values()].map(r => ({ id: r.id, s: r.statement, status: statusOf(r), files: r.files.length, survived: r.survived, audit: r.audit }))) + '\n'
      + 'OPEN LEADS: ' + JSON.stringify([...leads.values()]) + '\n'
      + 'Return your COORD: resultSoFar + keyClaimIds (the load-bearing ids — confidence is computed over exactly these); lanes ≤' + MAX_LANES + ' (pursue|attack; settled REQUIRES a survived challenge, so attack your own emerging answer — an attack lane names targets); dropLeads (dead leads); stop {done, reason} — done ONLY when the goal is answered on settled key claims or further probing cannot change the answer.'
      + ' Structured output: resultSoFar, keyClaimIds, lanes, dropLeads, stop.',
      { label: 'brainer · w' + wave, phase: 'Investigate', model: BRAINER_MODEL, effort: BRAINER_EFFORT, schema: COORD },
    )
    if (!coord) { stopReason = 'brainer-dead'; break }
    for (const id of (coord.dropLeads || [])) leads.delete(id)
    if (coord.stop && coord.stop.done) { stopReason = 'brainer-done: ' + (coord.stop.reason || ''); break }
    const lanes = (coord.lanes || []).slice(0, MAX_LANES)
    if (!lanes.length) { stopReason = 'no-lanes'; break }
    results = await parallel(lanes.map(l => () => probeAgent(l)))
    const fresh = ingest(results, wave)
    await auditNew(wave)
    log('Wave ' + wave + ': ' + lanes.length + ' lane(s) → ' + fresh + ' fresh claim(s) · ledger ' + ledger.size)
    if (!fresh) { if (++dry >= 2) { stopReason = 'dry'; break } } else dry = 0
  }
  const keyIds = coord ? (coord.keyClaimIds || []) : []
  const conf = computedConfidence(keyIds)
  const claimsOut = [...ledger.values()].map(r => ({ id: r.id, statement: r.statement, status: statusOf(r), anchors: r.anchors, files: r.files, survived: r.survived, audit: r.audit }))
  const SYNTH = { type: 'object', properties: { answer: { type: 'string' }, confidence: { type: 'string', enum: ['low', 'medium', 'high'] }, report: { type: 'string' } }, required: ['answer', 'confidence', 'report'] }
  const synth = await resilient(
    'You are the SYNTHESISER of a code investigation. GOAL: ' + GOAL + '. Write the report from the hardened ledger — sections: Answer · Evidence (claims by status, cite ids inline [cN] with anchors) · Counter-evidence & survived challenges · Open leads · Coverage (stopReason: ' + stopReason + '). '
    + 'COMPUTED confidence over key claims ' + JSON.stringify(keyIds) + ' = ' + conf + ' — your stated confidence may be LOWER, never higher. '
    + (REPORT_OUT ? 'WRITE the full report to ' + REPORT_OUT + ' (your ONLY file write; run no git). ' : 'Write no files. ')
    + 'resultSoFar: ' + (coord ? coord.resultSoFar : '(brainer dead — reason from the ledger alone)') + '\nLEDGER: ' + JSON.stringify(claimsOut) + '\nOPEN LEADS: ' + JSON.stringify([...leads.values()])
    + ' Structured output: answer, confidence, report (full markdown).',
    { label: 'synth', phase: 'Investigate', model: SYNTH_MODEL, effort: 'xhigh', schema: SYNTH },
  )
  const rank = { low: 0, medium: 1, high: 2 }
  const finalConf = synth ? (rank[synth.confidence] <= rank[conf] ? synth.confidence : conf) : conf
  log('Investigate done · ' + stopReason + ' · ' + ledger.size + ' claims · confidence ' + finalConf + (synth ? '' : ' · ⚠ DEGRADED (synth died)'))
  return {
    status: 'DONE', mode: 'investigate', goal: GOAL, stopReason,
    answer: synth ? synth.answer : ((coord && coord.resultSoFar) || 'DEGRADED: no synthesis and no coord — see claims'),
    confidence: finalConf, computedConfidence: conf, keyClaimIds: keyIds,
    claims: claimsOut, openLeads: [...leads.values()], report: synth ? synth.report : null, reportOut: REPORT_OUT,
    degraded: !synth || !coord,
  }
}

// AUTH_RULE is extracted LIVE by the scout from {BACKEND_PROJECT}/CLAUDE.md § Auth Pattern (heading grep, never line
// numbers). The fallback below fires ONLY when the scout returns no usable extract; it is a declared copy of that
// section's Role-fences bullet — any edit to § Auth Pattern re-syncs this string (grep AUTH_RULE_FALLBACK).
// INSTALL: replace this fallback with your install's actual § Auth Pattern Role-fences rule, in the SAME role/
// resource vocabulary ({ROLE_USER}, {ROLE_SUPER}, {ORG_UNIT}, {SUBJECT_NOUN}) the scout and rule engine use.
const AUTH_RULE_FALLBACK =
  '{BACKEND_PROJECT}/CLAUDE.md § Auth Pattern (FALLBACK COPY — verify against the live file): "Role fences (founder-ruled, SACRED — reads AND writes): '
  + '{ROLE_USER} is fenced to OWNERSHIP (record.ownerId === user.id) — never another {ROLE_USER}\'s {SUBJECT_NOUN}s, even inside the same {ORG_UNIT}. '
  + '{ROLE_SUPER} is a {ROLE_USER} with {ORG_UNIT}-wide access inside their OWN {ORG_UNIT} ONLY (record.{ORG_UNIT}Id === user.{ORG_UNIT}Id) — never another {ORG_UNIT}, never global. '
  + '{ORG_UNIT}-equality alone is NEVER a sufficient fence on a {ROLE_USER}-reachable path (that is the cross-{ROLE_USER} {SENSITIVE_DATA} leak). Every path that loads or mutates a {SUBJECT_NOUN}/{SESSION_NOUN}/record/document '
  + 'by client-supplied id branches by role and applies the matching fence — proof pattern: requireOwnership (the ownership-fence helper). Fence both roles or neither."'

const DEADNESS_BAR =
  'DEADNESS BAR (for any dead/unread/orphan verdict): prove it ALIVE first — a false dead in a {DOMAIN_ADJ} product is a live-{SESSION_NOUN} regression. '
  + 'Dead only with zero PRODUCTION consumers across all roster projects AND the surfaces a static grep misses: {API_PROTOCOL} SDL queried by name, {QUEUE} payload fields, '
  + 'the {AI_PROJECT} prompt registry (its prompt files loaded at runtime), {ORM} migrations, file-route framework routes, schema/JSON (de)serialization, '
  + 'and test/config/reflection consumers. Cannot prove past the bar -> NOT dead: verdict UNPROVEN, keep the code.'

const CATCHBOOK =
  'Catch-book categories (tag findings): DUP (reinvented helper/type/hook; copy-pasted consumer pattern), DEAD (unreachable/orphaned, subject to the deadness bar), '
  + 'GHOST (dual-writes, manual sync, schema<->code field mismatch), SMELL (cross-boundary writes, shallow error handling, N+1, wrong layer, over-engineering), '
  + 'TYPE-GAP (unguarded casts, hand-typed drift, loose String where enum belongs), NAMING (concept drift, scope-dishonest names), '
  + 'QUALITY (magic literals, hardcoded i18n, fetch-in-leaf-component), STALE-DEP (unused/phantom imports).'

const ENC_VOCAB = 'encoding vocabulary (EXACTLY one): raw | object | json-string | enum-string | number | boolean | unknown'
const DEC_VOCAB = 'decode vocabulary (EXACTLY one per consumer): direct | render | json-parse | object-index | compare | spread | unknown'

// ─── Schemas ──────────────────────────────────────────────────────────────────
const SLICE_PROPS = {
  fieldId: { type: 'string' },
  producer: { type: 'object', properties: { anchor: { type: 'string' }, writer: { type: 'string' }, typeToken: { type: 'string' }, encoding: { type: 'string' }, valueLiterals: { type: 'array', items: { type: 'string' } } } },
  dbColumn: { type: 'object', properties: { anchor: { type: 'string' }, columnName: { type: 'string' }, columnType: { type: 'string' }, checkLiterals: { type: 'array', items: { type: 'string' } } } },
  resolver: { type: 'object', properties: { anchor: { type: 'string' } } },
  feSelection: { type: 'object', properties: { anchor: { type: 'string' }, queryName: { type: 'string' } } },
  feTypes: { type: 'array', items: { type: 'object', properties: { anchor: { type: 'string' }, typeToken: { type: 'string' }, kind: { type: 'string', description: 'generated | hand' } } } },
  consumers: {
    type: 'array', items: {
      type: 'object', properties: {
        anchor: { type: 'string' }, name: { type: 'string' }, decode: { type: 'string' },
        decodeExpr: { type: 'string', description: 'VERBATIM read/parse expression, <=80 chars' },
        context: { type: 'string', description: 'production | test | generated | story' },
        comparedLiterals: { type: 'array', items: { type: 'string' } }, aliasChain: { type: 'array', items: { type: 'string' } },
      }, required: ['anchor'],
    },
  },
  danglingRefs: { type: 'array', items: { type: 'object', properties: { ref: { type: 'string' }, anchor: { type: 'string' } } } },
  notes: { type: 'string' },
}
const SLICES = {
  type: 'object', properties: {
    jobId: { type: 'string' },
    slices: { type: 'array', items: { type: 'object', properties: SLICE_PROPS, required: ['fieldId'] } },
    undeclaredReads: { type: 'array', items: { type: 'object', properties: { side: { type: 'string' }, property: { type: 'string' }, anchor: { type: 'string' }, expr: { type: 'string' } }, required: ['property', 'anchor'] } },
  }, required: ['jobId', 'slices'],
}
const SCOUT = {
  type: 'object', properties: {
    headSha: { type: 'string' },
    territories: { type: 'array', items: { type: 'string' } },
    changedFiles: { type: 'array', items: { type: 'string' } },
    mergeShas: { type: 'array', items: { type: 'string' } },
    threads: {
      type: 'array', description: 'the functional/hygiene walk manifest — feature flow | seam | field | schema/db | invariant | test-data | dead-code-ripple',
      items: { type: 'object', properties: { id: { type: 'string' }, type: { type: 'string' }, name: { type: 'string' }, scope: { type: 'string' }, files: { type: 'array', items: { type: 'string' } }, verify: { type: 'string' } }, required: ['id', 'type', 'name', 'verify'] },
    },
    operations: { type: 'array', items: { type: 'object', properties: { id: { type: 'string' }, kind: { type: 'string' }, anchor: { type: 'string' }, resultType: { type: 'string' } }, required: ['id', 'anchor'] } },
    fields: { type: 'array', items: { type: 'object', properties: { id: { type: 'string' }, ownerType: { type: 'string' }, field: { type: 'string' }, apis: { type: 'array', items: { type: 'string' } }, sdl: { type: 'object', properties: { anchor: { type: 'string' }, typeToken: { type: 'string' } } } }, required: ['id', 'ownerType', 'field'] } },
    jobs: { type: 'array', items: { type: 'object', properties: { jobId: { type: 'string' }, kind: { type: 'string', description: 'producer | consumer | ai' }, files: { type: 'array', items: { type: 'string' } }, fieldIds: { type: 'array', items: { type: 'string' } }, hint: { type: 'string' } }, required: ['jobId', 'kind', 'files', 'fieldIds'] } },
    gateFiles: { type: 'array', items: { type: 'string' }, description: 'EVERY resolver file in {BACKEND_PROJECT}, repo-wide — fence-outlier context' },
    authRule: { type: 'string', description: 'VERBATIM Role-fences bullet from {BACKEND_PROJECT}/CLAUDE.md § Auth Pattern' },
  }, required: ['headSha', 'changedFiles', 'threads', 'fields', 'jobs', 'gateFiles'],
}
const WALK = {
  type: 'object', properties: {
    threadId: { type: 'string' }, name: { type: 'string' }, type: { type: 'string' },
    flow: { type: 'string', enum: ['INTACT', 'AT-RISK', 'BROKEN', 'N/A'] }, trace: { type: 'string' },
    defects: { type: 'array', items: { type: 'object', properties: { what: { type: 'string' }, location: { type: 'string' }, jc: { type: 'string' } }, required: ['what', 'location'] } },
    hygiene: { type: 'array', items: { type: 'object', properties: { kind: { type: 'string' }, where: { type: 'string' }, detail: { type: 'string' }, jc: { type: 'string' } }, required: ['kind', 'where', 'detail'] } },
    notes: { type: 'string' },
  }, required: ['threadId', 'flow', 'trace', 'defects', 'hygiene'],
}
const GATE_SWEEP = {
  type: 'object', properties: {
    file: { type: 'string' },
    gates: { type: 'array', items: { type: 'object', properties: { id: { type: 'string' }, kind: { type: 'string' }, resource: { type: 'string' }, anchor: { type: 'string' }, idArgs: { type: 'array', items: { type: 'string' } }, rolesAllowed: { type: 'array', items: { type: 'string' } }, chain: { type: 'array', items: { type: 'string' } }, orgFence: { type: 'boolean' }, ownershipFence: { type: 'boolean' }, notes: { type: 'string' } }, required: ['id', 'anchor'] } },
  }, required: ['file', 'gates'],
}
const JUDGE = {
  type: 'object', properties: {
    verdicts: { type: 'array', items: { type: 'object', properties: { anomalyId: { type: 'string' }, verdict: { type: 'string', enum: ['CONFIRMED', 'FALSE', 'UNPROVEN'] }, severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] }, what: { type: 'string' }, location: { type: 'string' }, fix: { type: 'string' }, why: { type: 'string' } }, required: ['anomalyId', 'verdict', 'severity', 'what'] } },
  }, required: ['verdicts'],
}
const DIGEST = {
  type: 'object', properties: {
    territory: { type: 'string' },
    findings: { type: 'array', items: { type: 'object', properties: { lens: { type: 'string' }, severity: { type: 'string', enum: ['info', 'low', 'med', 'high'] }, what: { type: 'string' }, location: { type: 'string' }, fix: { type: 'string' } }, required: ['lens', 'severity', 'what', 'location'] } },
    summary: { type: 'string' },
  }, required: ['territory', 'findings', 'summary'],
}
const FOLD = {
  type: 'object', properties: {
    verdict: { type: 'string', enum: ['SMOOTH SAILING', 'MOSTLY GOOD', 'ROUGH SEAS', 'SHIPWRECK'] },
    actionItems: { type: 'array', items: { type: 'string' } },
    review: { type: 'string' },
  }, required: ['verdict', 'actionItems', 'review'],
}

// ─── Resilience with tier escalation ─────────────────────────────────────────
async function resilient(prompt, opts, escalateModel) {
  // '[label]' prompt prefix: the token ledger's snippet fallback attributes workflow spend per stage.
  let r = await agent('[' + (opts.label || 'agent') + '] ' + prompt, opts)
  if (r === null) {
    const retryModel = escalateModel || opts.model
    log('⚠ ' + (opts.label || 'agent') + ' died · respawning once on ' + retryModel)
    r = await agent('[' + (opts.label || 'agent') + '-retry] RESUME: a prior agent for this exact role died mid-task (often on structured-output). Redo from scratch — idempotent. Keep output values SHORT and schema-exact. ' + prompt,
      { ...opts, model: retryModel, label: (opts.label || 'agent') + '-retry' })
  }
  return r
}

// ─── Phase 0: Scout — diff-scoped; emits BOTH the thread manifest AND the ledger schedule ─────
log('Wave walker · report=' + REPORT_PATH + ' · sensors=' + SENSOR_MODEL + '/' + SENSOR_EFFORT + '→' + SENSOR_ESCALATE + ' · walkers=' + WALKER_MODEL + ' · judges=' + JUDGE_MODEL)

const scout = await resilient(
  'You are the SCOUT-SCHEDULER of a wave-walker review. Read the wave report at ' + REPORT_PATH + ' and walk the WAVE\'S DIFF. Repo root: {REPO_ROOT}.\n'
  + (BRANCH
    ? '1) PRE-MERGE BRANCH MODE: the wave is NOT merged yet. changedFiles = `git diff --name-only main...' + BRANCH + '` (three-dot; read file contents from the branch\'s worktree checkout when present, else `git show ' + BRANCH + ':{path}`). mergeShas = []. headSha = `git rev-parse ' + BRANCH + '`. The report carries the wave manifest + slice list for context.\n'
    : '1) From the report — a `**Merge SHA:**` line (the dual-chat wave writes one at MERGE) and/or the Final Summary / Grouping / `## JC Pre-flight` sections: list SUCCEEDED pipeline merge SHAs (mergeShas) and any JC commits. Run `git diff {merge}^1 {merge}` per merge SHA (`git show {sha}` for a JC fix) and union into changedFiles (the integrated changed-and-generated set). headSha = git rev-parse HEAD.\n')
  + '2) THREADS — the functional/hygiene walk manifest (the proven floor). Read ' + WALKER_DOC + ' § Role: Scout for the thread taxonomy; aim for >= 4, one per feature flow plus a thread for each seam, field, schema change, invariant, test-data-discipline, or dead-code-ripple the diff puts at risk. Emit a Field thread with an explicit READ-BACK check for EVERY new persisted field (writer AND reader mapping). Each: id, type, name, scope, files, verify.\n'
  + '3) LEDGER SCHEDULE (the mechanical spine, only if the diff touches the {API_PROTOCOL} contract surface — else return empty fields/jobs and the thread walk carries the wave):\n'
  + '   · operations — {API_PROTOCOL} operations whose resolver/SDL the diff changed OR whose result type the diff touches: id, kind, resolver anchor, resultType.\n'
  + '   · fields — every field of each touched result type, DEDUPED by (ownerType, field); id="OwnerType.fieldName"; fill each field\'s sdl slice {anchor, typeToken} YOURSELF from the schema. Include a field when the diff changed its producer, its SDL, or any consumer.\n'
  + '   · jobs — cluster fields by FILE LOCALITY into sensor jobs (kind producer|consumer|ai), each with the EXACT files to read and <= ' + MAX_FIELDS_PER_JOB + ' fieldIds; follow resolver imports / grep the query call-sites NOW so each job\'s file list is exact.\n'
  + '4) gateFiles — EVERY resolver / API entry-point file under {BACKEND_PROJECT}\'s resolver directory (repo-wide; fence-outlier detection needs the full population even when the diff is small).\n'
  + '5) territories — which of BE/FE/{AI_PROJECT} the diff touches.\n'
  + '6) authRule — grep {BACKEND_PROJECT}/CLAUDE.md for its "Auth Pattern" heading (locate by heading text, NEVER by line number) and return the "Role fences" bullet VERBATIM — the ledger\'s R6 auth-fence rule and the security second-opinion quote it live.' + RO
  + ' Structured output: headSha, territories, changedFiles, mergeShas, threads, operations, fields, jobs, gateFiles, authRule.',
  { label: 'scout', phase: 'Scout', model: SCOUT_MODEL, schema: SCOUT },
)
if (!scout) return { status: 'FAILED', detail: 'scout died twice' }
if (!(scout.changedFiles || []).length) return { status: 'FAILED', detail: 'scout resolved an EMPTY changed-file set — no merge SHA found in ' + REPORT_PATH + (BRANCH ? ' / empty branch diff' : '') + '; a walk over nothing must never return a verdict' }
const threads = (scout.threads || []).concat(Array.isArray(args.extraThreads) ? args.extraThreads : [])
let fields = scout.fields || []
const gateFiles = scout.gateFiles || []
log('Scout: ' + threads.length + ' threads · ' + fields.length + ' type-fields · ' + (scout.jobs || []).length + ' slice jobs · ' + gateFiles.length + ' gate files · changed: ' + (scout.changedFiles || []).length + ' files')

// INSTALL: '{ROLE_USER}' / '{ROLE_SUPER}' are the auth-extract sanity check — the scout's live extract must contain
// both your install's fenced-owner role and its org-scoped supervisor role, or the fallback fires.
const authOk = typeof scout.authRule === 'string' && scout.authRule.includes('{ROLE_USER}') && scout.authRule.includes('{ROLE_SUPER}') && scout.authRule.length >= 120
if (!authOk) log('⚠ scout returned no usable § Auth Pattern extract — R6/second-opinion run on AUTH_RULE_FALLBACK (verify it against {BACKEND_PROJECT}/CLAUDE.md)')
const AUTH_RULE = authOk ? '{BACKEND_PROJECT}/CLAUDE.md § Auth Pattern (live, scout-extracted): "' + scout.authRule + '"' : AUTH_RULE_FALLBACK

// Enforce the sensor cap; name any dropped fields (honest coverage).
let jobs = (scout.jobs || []).flatMap(j => (j.fieldIds || []).length <= MAX_FIELDS_PER_JOB ? [j]
  : j.fieldIds.reduce((acc, id, i) => { const b = Math.floor(i / MAX_FIELDS_PER_JOB); (acc[b] = acc[b] || { ...j, jobId: j.jobId + '-' + (b + 1), fieldIds: [] }).fieldIds.push(id); return acc }, []))
let droppedFieldIds = []
if (jobs.length + gateFiles.length > MAX_SENSORS) {
  const keep = Math.max(0, MAX_SENSORS - gateFiles.length)
  droppedFieldIds = jobs.slice(keep).flatMap(j => j.fieldIds || [])
  jobs = jobs.slice(0, keep)
  if (droppedFieldIds.length) log('⚠ sensor cap ' + MAX_SENSORS + ': DROPPED slice jobs — fields reported UNSENSED: ' + droppedFieldIds.join(', '))
}

// ─── Phase 1: Walk (thread walkers) + Sense (ledger sensors + gate sweeps) — one parallel barrier ─────
const fieldById = new Map(fields.map(f => [f.id, f]))

function walkerAgent(t) {
  return resilient(
    'Read ' + WALKER_DOC + ' § Role: Walker. Walk this ONE thread end-to-end in a single pass over its files, returning BOTH the functional verdict AND the integration-delta code-hygiene findings. '
    + 'Per-pipeline hygiene already ran pre-merge (wave/builder.md Step 7) — your wave-level value is the INTEGRATION delta: a repo-wide reuse-grep for a helper/type/hook a SIBLING pipeline duplicated, plus dead code the integration orphaned. '
    + 'Thread: ' + JSON.stringify(t) + '.' + RO
    + ' Structured output: threadId, name, type, flow (INTACT|AT-RISK|BROKEN|N/A), trace (step → step, marking any break), defects (each {what, location=file:line, jc=`/jc {fix}`}), hygiene (each {kind, where=file:line, detail, jc}), notes.',
    { label: 'walk · ' + (t.id || t.name || '?'), phase: 'Walk', model: WALKER_MODEL, schema: WALK },
  )
}

function sliceSensor(job) {
  const kind = job.kind
  const assigned = job.fieldIds.map(id => { const f = fieldById.get(id) || { id }; return { fieldId: id, field: f.field, sdlTypeToken: f.sdl && f.sdl.typeToken } })
  return resilient(
    'You are a PURE EXTRACTOR (scheduled sensor). NO judgment, NO bug-finding — extract and return, nothing else. '
    + 'Read ONLY these files (Grep to confirm an anchor is fine): ' + JSON.stringify(job.files) + '. Hint: ' + (job.hint || 'none') + '.\n'
    + 'For EACH assigned field, extract its ' + kind.toUpperCase() + ' slice:\n'
    + (kind === 'producer'
      ? '· producer: where the value is mapped onto the result object — anchor, writer, typeToken, encoding (' + ENC_VOCAB + '), valueLiterals (EXACT, case-preserved).\n· dbColumn (if from a column): anchor, columnName, columnType, checkLiterals.\n· resolver (if a dedicated field/type resolver exists): anchor.\n'
      : kind === 'consumer'
        ? '· feSelection: where the query selects it — anchor, queryName (omit if never selected).\n· feTypes: generated type AND any hand-written interfaces — anchor, typeToken, kind (generated|hand).\n· consumers: EVERY read to the leaf render — anchor, name, decode (' + DEC_VOCAB + '), decodeExpr (VERBATIM, <=80 chars), context (production|test|generated|story), comparedLiterals (EXACT, case-preserved), aliasChain.\n· PARSE SITES ARE CONSUMERS: a screen that parses/transforms before drilling down (JSON.parse, mapping, memo) is its own consumer — its verbatim expression is the decodeExpr; a JSON.parse(JSON.stringify(x)) roundtrip MUST appear verbatim, never summarized; record each screen\'s parse separately.\n· undeclaredReads: any property read off the same result object NOT in your assigned field list (side:"fe"), INCLUDING reads in fallback chains (a ?? b, a || b, ternaries) and optional-chained access; plus any field the resolver returns beyond the declared set if a resolver file is listed (side:"be", expand spreads).\n'
        : '· producer ({AI_PROJECT} writer): where the AI/worker layer computes/writes this value — anchor, writer:"ai", encoding, valueLiterals (EXACT). Grep the snake_case form.\n')
    + 'A field with nothing to extract here gets a slice with just its fieldId. Every anchor grep-verified file:line. Keep strings SHORT (<=80 chars).\n'
    + 'Assigned fields: ' + JSON.stringify(assigned) + RO
    + ' Structured output: jobId=' + job.jobId + ', slices, undeclaredReads.',
    { label: kind + ' · ' + job.jobId, phase: 'Walk', model: SENSOR_MODEL, effort: SENSOR_EFFORT, schema: SLICES }, SENSOR_ESCALATE,
  )
}
function gateSweep(file) {
  return resilient(
    'You are a PURE EXTRACTOR (gate sweep). NO judgment. Open the resolver file ' + file + ' and extract ONE gate card per {API_PROTOCOL} entry point in it.\n'
    + 'Per entry point: id ("Query.opName"/"Mutation.opName"), kind, resource class ({SESSION_NOUN}|{SUBJECT_NOUN}|{ORG_UNIT}|user|other), anchor, idArgs (client-supplied ID args), '
    + 'chain (IN ORDER, every guard call between entry and first data access; open custom helpers and note what they fence), rolesAllowed (EXPAND role-set constants), '
    + 'orgFence (boolean: {ORG_UNIT}-equality check enforced), ownershipFence (boolean: record-owner check enforced). Keep strings SHORT.' + RO
    + ' Structured output: file, gates.',
    { label: 'gates · ' + file.split('/').pop(), phase: 'Walk', model: SENSOR_MODEL, effort: SENSOR_EFFORT, schema: GATE_SWEEP }, SENSOR_ESCALATE,
  )
}
const SECURITY = {
  type: 'object', properties: {
    findings: { type: 'array', items: { type: 'object', properties: { id: { type: 'string' }, category: { type: 'string', description: '8A-8K per the audit doc' }, severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] }, what: { type: 'string' }, location: { type: 'string' }, expected: { type: 'string' }, got: { type: 'string' }, fix: { type: 'string' } }, required: ['id', 'category', 'severity', 'what', 'location'] } },
    categoriesSwept: { type: 'array', items: { type: 'string' } },
    summary: { type: 'string' },
  }, required: ['findings', 'categoriesSwept', 'summary'],
}
function securityAudit() {
  return resilient(
    'You are the WAVE SECURITY AUDITOR. Read ' + SECURITY_DOC + ' and apply its FULL category set (8A–8K + Method & Severity) SCOPED TO THIS WAVE\'S DIFF — the changed files plus every security-relevant surface they touch (follow a changed symbol into its callers/config when the risk crosses the file boundary). Changed files: ' + JSON.stringify(scout.changedFiles) + '. ' + (BRANCH ? 'Diff: main...' + BRANCH + '.' : 'Merge SHAs: ' + JSON.stringify(scout.mergeShas || []) + '.') + ' {DOMAIN_SAFETY} — {SENSITIVE_DATA} (8F), auth (8C), {API_PROTOCOL} (8D), LLM/prompt (8E) get the deepest pass. Report ONLY defects the diff introduced or worsened; a pre-existing issue you trip over goes into summary as one line (category + location), never a finding. categoriesSwept names every category you ACTUALLY swept — honesty over completeness.' + RO
    + ' Structured output: findings (Expected/Got), categoriesSwept, summary.',
    { label: 'security · 8A-8K', phase: 'Walk', model: SECURITY_MODEL, effort: SECURITY_EFFORT, schema: SECURITY },
  )
}

const walked = await parallel([
  ...threads.map(t => () => walkerAgent(t)),
  ...jobs.map(j => () => sliceSensor(j)),
  ...gateFiles.map(f => () => gateSweep(f)),
  () => securityAudit(),
])
const nT = threads.length, nJ = jobs.length, nG = gateFiles.length
const walks = walked.slice(0, nT).filter(Boolean)
const sliceResults = walked.slice(nT, nT + nJ).filter(Boolean)
const gates = walked.slice(nT + nJ, nT + nJ + nG).filter(Boolean).flatMap(s => (s.gates || []).map(g => ({ ...g, file: s.file })))
const security = walked[nT + nJ + nG] || null
const secFindings = security ? (security.findings || []) : []
const undeclaredReads = sliceResults.flatMap(r => r.undeclaredReads || [])

// Zip slices into cards (mechanical, zero tokens)
const cardMap = new Map(fields.map(f => [f.id, { id: f.id, ownerType: f.ownerType, field: f.field, apis: f.apis || [], sdl: f.sdl || null, feTypes: [], consumers: [], danglingRefs: [], _sides: new Set() }]))
for (const r of sliceResults) {
  const job = jobs.find(j => j.jobId === r.jobId)
  for (const s of (r.slices || [])) {
    const c = cardMap.get(s.fieldId); if (!c) continue
    c._sides.add(job ? job.kind : '?')
    if (s.producer && !c.producer) c.producer = s.producer
    else if (s.producer && c.producer) c.producer.valueLiterals = [...new Set([...(c.producer.valueLiterals || []), ...(s.producer.valueLiterals || [])])]
    if (s.dbColumn && !c.dbColumn) c.dbColumn = s.dbColumn
    if (s.resolver && !c.resolver) c.resolver = s.resolver
    if (s.feSelection && !c.feSelection) c.feSelection = s.feSelection
    if (s.feTypes) c.feTypes.push(...s.feTypes)
    if (s.consumers) c.consumers.push(...s.consumers)
    if (s.danglingRefs) c.danglingRefs.push(...s.danglingRefs)
    if (s.notes) c.notes = ((c.notes || '') + ' ' + s.notes).trim()
  }
}
const cards = [...cardMap.values()].map(c => { const { _sides, ...rest } = c; rest.sidesCovered = [..._sides]; return rest })
const unsensed = [...new Set([...[...cardMap.values()].filter(c => c._sides.size === 0).map(c => c.id), ...droppedFieldIds])]
if (unsensed.length) log('⚠ UNSENSED fields (no card): ' + unsensed.join(', '))
log('Walked: ' + walks.length + '/' + nT + ' threads · ' + cards.length + ' cards from ' + sliceResults.length + '/' + nJ + ' jobs · ' + gates.length + ' gates · undeclared reads: ' + undeclaredReads.length + ' · security: ' + (security ? secFindings.length + ' finding(s)' : 'AUDIT DIED'))

// ─── Phase 2: THE LEDGER DIFF — mechanical rules, zero tokens ─────────────────
const anomalies = []
let aseq = 0
function flag(rule, ruleName, detail, anchors, severityHint, cardId) {
  anomalies.push({ id: rule + '-' + (++aseq), rule, ruleName, detail, anchors: (anchors || []).filter(Boolean), severityHint, cardId: cardId || null })
}
function baseType(t) {
  let s = String(t || '').toLowerCase().replace(/maybe<|scalars\['?|'?\]\['(in|out)put'?\]|>|\?|!/g, '')
  s = s.replace(/\|\s*(null|undefined)/g, '').replace(/(null|undefined)\s*\|/g, '').replace(/\s+/g, '')
  const map = { string: 'string', str: 'string', id: 'string', int: 'number', float: 'number', number: 'number', boolean: 'boolean', bool: 'boolean', jsonb: 'object', json: 'object' }
  return map[s] || (s.startsWith('record<') || s.startsWith('{') || s.includes('array<') || s.endsWith('[]') ? 'object' : s)
}
const INCOMPAT = { 'json-string': ['object-index', 'spread'], 'object': ['json-parse'], 'enum-string': ['object-index'] }
const DOUBLE_ENCODE = /JSON\s*\.\s*parse\s*\(\s*JSON\s*\.\s*stringify/
for (const c of cards) {
  const consumers = c.consumers || []
  const prodConsumers = consumers.filter(x => (x.context || 'production') === 'production')
  const a = x => (x && x.anchor) || null
  if (c.producer && prodConsumers.length === 0) {
    const nonProd = consumers.length - prodConsumers.length
    const sub = !c.sdl ? 'produced but never exposed in SDL' : !c.feSelection ? 'declared in SDL but never selected by any FE query' : 'shipped and selected but read by no production consumer'
    flag('R1', 'orphan producer', c.id + ': ' + sub + (nonProd ? ' (' + nonProd + ' non-production ref(s) only)' : ''), [a(c.producer), a(c.sdl), a(c.feSelection)], 'med', c.id)
  }
  if (!c.producer && consumers.length > 0) flag('R2', 'phantom consumer', c.id + ': consumed at ' + consumers.length + ' site(s) but no producer emits it' + (c.sdl ? ' (declared in SDL yet unfed)' : ' (absent from SDL)'), [a(c.sdl), ...consumers.map(a)], 'high', c.id)
  const enc = c.producer && c.producer.encoding
  for (const cons of consumers) {
    if (cons.decodeExpr && DOUBLE_ENCODE.test(cons.decodeExpr)) flag('R3', 'encoding mismatch', c.id + ': double-encode JSON.parse(JSON.stringify(...)) at ' + cons.anchor + ' — on a ' + (enc || 'unknown') + ' value returns the input unchanged, never a parsed object', [a(c.producer), cons.anchor], 'high', c.id)
    else if (enc && INCOMPAT[enc] && INCOMPAT[enc].includes(cons.decode)) flag('R3', 'encoding mismatch', c.id + ': produced as ' + enc + ' (' + (c.producer.anchor || '?') + ') but consumed via ' + cons.decode + ' at ' + cons.anchor, [a(c.producer), cons.anchor], 'high', c.id)
  }
  const prodLits = [...new Set([...((c.producer && c.producer.valueLiterals) || []), ...((c.dbColumn && c.dbColumn.checkLiterals) || [])])]
  if (prodLits.length) for (const cons of consumers) {
    const cl = cons.comparedLiterals || [], missing = cl.filter(l => !prodLits.includes(l))
    if (cl.length && missing.length) {
      const casing = missing.filter(l => prodLits.some(p => p.toLowerCase() === String(l).toLowerCase()))
      flag('R4', 'value-set mismatch', c.id + ': consumer at ' + cons.anchor + ' compares against ' + JSON.stringify(missing) + ' which no producer emits' + (casing.length ? ' — CASING mismatch of ' + JSON.stringify(casing) + ', branch permanently dead' : '') + ' (produced: ' + JSON.stringify(prodLits.slice(0, 8)) + ')', [a(c.producer), a(c.dbColumn), cons.anchor], casing.length ? 'critical' : 'high', c.id)
    }
  }
  const gen = (c.feTypes || []).find(t => t.kind === 'generated')
  for (const hand of (c.feTypes || []).filter(t => t.kind === 'hand')) {
    const ref = gen || (c.sdl ? { typeToken: c.sdl.typeToken, anchor: c.sdl.anchor } : null)
    if (ref && baseType(hand.typeToken) !== baseType(ref.typeToken)) flag('R5', 'type drift', c.id + ': hand-typed "' + hand.typeToken + '" (' + hand.anchor + ') vs ' + (gen ? 'generated' : 'SDL') + ' "' + ref.typeToken + '" — base ' + baseType(hand.typeToken) + ' vs ' + baseType(ref.typeToken), [hand.anchor, ref.anchor], 'med', c.id)
  }
  for (const d of (c.danglingRefs || [])) flag('R8', 'dangling reference', c.id + ': "' + d.ref + '" at ' + d.anchor + ' resolves to nothing', [d.anchor], 'med', c.id)
}
for (const r of undeclaredReads) flag('R2', 'phantom consumer', (r.side === 'be' ? 'resolver returns' : 'FE reads') + ' undeclared field "' + r.property + '" at ' + r.anchor + (r.expr ? ' (' + r.expr + ')' : ''), [r.anchor], r.side === 'be' ? 'med' : 'high', null)
const byResource = {}
for (const g of gates) (byResource[g.resource || 'other'] = byResource[g.resource || 'other'] || []).push(g)
for (const [res, group] of Object.entries(byResource)) {
  const fenced = group.filter(g => g.ownershipFence), unfenced = group.filter(g => !g.ownershipFence && (g.idArgs || []).length > 0)
  if (fenced.length && unfenced.length) flag('R6', 'gate outlier', 'resource "' + res + '": ' + fenced.map(g => g.id).join(', ') + ' enforce an ownership fence but ' + unfenced.map(g => g.id).join(', ') + ' do not — same class, weaker chain', [...fenced.map(g => g.anchor), ...unfenced.map(g => g.anchor)], 'high', null)
}
// INSTALL: the owner-fenced resource classes — the resources a {ROLE_USER}-reachable path must ownership-fence.
// Replace with your install's set (must match the resource-class enum the gate sweep emits).
for (const [res, group] of Object.entries(byResource)) {
  if (!['{SESSION_NOUN}', '{SUBJECT_NOUN}'].includes(res)) continue
  const violators = group.filter(g => (g.idArgs || []).length > 0 && !g.ownershipFence && (g.rolesAllowed || []).some(r => String(r).toUpperCase().includes('{ROLE_USER}')))
  if (violators.length) flag('R6', 'mandated-fence violation', 'resource "' + res + '": ' + violators.map(g => g.id).join(', ') + ' admit {ROLE_USER} with client-supplied id but enforce NO ownership fence — direct violation of the documented rule. ' + AUTH_RULE, violators.map(g => g.anchor), 'critical', null)
}
for (const g of gates) if ((g.idArgs || []).length > 0 && !g.orgFence && !g.ownershipFence) flag('R7', 'unfenced ID flow', g.id + ': client-supplied ' + JSON.stringify(g.idArgs) + ' reaches data access with neither {ORG_UNIT} nor ownership fence (chain: ' + (g.chain || []).join(' → ') + ')', [g.anchor], 'critical', null)
const ruleCounts = anomalies.reduce((m, x) => { m[x.rule] = (m[x.rule] || 0) + 1; return m }, {})
log('Ledger diff: ' + anomalies.length + ' anomalies (' + Object.entries(ruleCounts).map(([k, v]) => k + ':' + v).join(' ') + ')')

// ─── Phase 3: Judges (ledger anomalies) + digests; Opus second opinion on killed security/near-certain ─
const RULE_MEANING = {
  R1: 'orphan producer — produced but no production consumer reads it. Apply the deadness bar.',
  R2: 'phantom consumer — a field is read/returned that no producer/SDL declares; the read silently yields undefined or ships out-of-contract data.',
  R3: 'encoding mismatch — producer encodes one way, consumer decodes another (incl. JSON.parse(JSON.stringify(x)), which returns x unchanged).',
  R4: 'value-set mismatch — a consumer compares against literals no producer emits; that branch is permanently dead. Casing-only difference is a certain bug.',
  R5: 'type drift — a hand-written type disagrees at BASE-type level with the generated/SDL truth.',
  R6: 'gate asymmetry / mandated-fence violation — auth fences unequal across a resource class, or the documented ownership-fence rule violated. ' + AUTH_RULE,
  R7: 'unfenced ID flow — a client-supplied ID reaches data access with no fence at all.',
  R8: 'dangling reference — a reference resolving to nothing.',
}
const SECURITY_RULES = ['R6', 'R7'], NEAR_CERTAIN = ['R3', 'R4']
function chunk(arr, n) { const out = []; for (let i = 0; i < arr.length; i += n) out.push(arr.slice(i, i + n)); return out }
const cardById = new Map(cards.map(c => [c.id, c]))
const byRule = {}
for (const x of anomalies) (byRule[x.rule] = byRule[x.rule] || []).push(x)
const judgeJobs = Object.entries(byRule).flatMap(([rule, list]) => chunk(list, 6).map((grp, i) => ({ rule, grp, i })))
function judgeAgent(job) {
  const ctxCards = [...new Set(job.grp.map(x => x.cardId).filter(Boolean))].map(id => cardById.get(id)).filter(Boolean)
  const sec = SECURITY_RULES.includes(job.rule)
  return resilient(
    'You are an anomaly JUDGE. Rule ' + job.rule + ': ' + RULE_MEANING[job.rule] + '\n'
    + 'For EACH instance: open the file(s) at the cited anchors (BOTH ends where two are given), confirm the facts, and rule CONFIRMED (severity, one-sentence what, location=file:line, fix=`/jc {fix}`), FALSE (say why), or UNPROVEN (say what is missing). Judge evidence, not vibes.\n'
    + (sec ? 'SECURITY: this rule enforces a WRITTEN project invariant. "Every sibling does it the same way" is NOT a defense — a documented-rule violation is CONFIRMED even when it is the file-wide pattern. Read {BACKEND_PROJECT}/CLAUDE.md § Auth Pattern before any FALSE.\n' : '')
    + DEADNESS_BAR + '\nInstances: ' + JSON.stringify(job.grp) + '\n'
    + (ctxCards.length ? 'Extracted cards for context (verify against real files): ' + JSON.stringify(ctxCards) + '\n' : '')
    + RO + ' Structured output: verdicts (one per instance, anomalyId matching).',
    { label: 'judge · ' + job.rule + (job.i ? '#' + (job.i + 1) : ''), phase: 'Judge', model: JUDGE_MODEL, schema: JUDGE },
  )
}
function project(c, side) {
  if (side === 'BE') return { id: c.id, producer: c.producer, dbColumn: c.dbColumn, sdl: c.sdl, resolver: c.resolver, notes: c.notes }
  if (side === '{AI_PROJECT}') return { id: c.id, producer: c.producer, dbColumn: c.dbColumn, notes: c.notes }
  return { id: c.id, sdl: c.sdl && c.sdl.typeToken, feSelection: c.feSelection, feTypes: c.feTypes, consumers: c.consumers, notes: c.notes }
}
const digestJobs = cards.length ? (scout.territories || []).map(t => ({
  territory: t,
  slice: t === '{AI_PROJECT}' ? cards.filter(c => c.producer && String(c.producer.writer || '').toLowerCase().includes('ai')).map(c => project(c, '{AI_PROJECT}')) : cards.map(c => project(c, t === 'BE' ? 'BE' : 'FE')),
})).filter(j => j.slice.length) : []
function digestAgent(job) {
  return resilient(
    'You are the ' + job.territory + ' TERRITORY DIGEST for this wave. You receive this territory\'s side of every extracted card. '
    + 'Mechanical rules AND the thread walk already handled connectivity/contract/gate/flow — do NOT re-report those. Your job is what neither can see: duplication across fields, wrong-layer logic, over-engineering, naming drift, magic literals, shallow error handling, hardcoded i18n. Open files as needed.\n'
    + CATCHBOOK + '\nCards: ' + JSON.stringify(job.slice) + RO
    + ' Structured output: territory, findings (each {lens, severity, what, location=file:line, fix}), summary (<=3 sentences).',
    { label: 'digest · ' + job.territory, phase: 'Judge', model: DIGEST_MODEL, schema: DIGEST },
  )
}
const [judgeResults, digestResults] = await Promise.all([
  parallel(judgeJobs.map(j => () => judgeAgent(j))),
  parallel(digestJobs.map(j => () => digestAgent(j))),
])
let verdicts = judgeResults.filter(Boolean).flatMap(r => r.verdicts || [])
const digests = digestResults.filter(Boolean)
const anomalyById = new Map(anomalies.map(x => [x.id, x]))
const escalatable = verdicts.filter(v => v.verdict === 'FALSE').filter(v => { const x = anomalyById.get(v.anomalyId); if (!x) return false; if (SECURITY_RULES.includes(x.rule) && ['high', 'critical'].includes(x.severityHint)) return true; return NEAR_CERTAIN.includes(x.rule) })
if (escalatable.length) {
  log('Escalation: ' + escalatable.length + ' killed security/near-certain verdict(s) → ' + SEC_ESCALATE_MODEL + ' second opinion')
  const second = await parallel(chunk(escalatable, 4).map((grp, i) => () => resilient(
    'You are the SECOND-OPINION judge (a first judge killed these as FALSE, but the rule\'s evidence is regex/string-exact or a documented security invariant). ' + AUTH_RULE + '\n'
    + 'For each: open the file(s) yourself, re-derive from scratch, rule independently. Be suspicious of a kill that contradicts the verbatim extracted expression (a JSON.parse(JSON.stringify(...)) still present, a literal that truly never matches the produced set).\n'
    + 'Killed verdicts with anomalies: ' + JSON.stringify(grp.map(v => ({ verdict: v, anomaly: anomalyById.get(v.anomalyId) }))) + RO
    + ' Structured output: verdicts (one per anomalyId).',
    { label: '2nd-opinion#' + (i + 1), phase: 'Judge', model: SEC_ESCALATE_MODEL, schema: JUDGE },
  )))
  const overrides = new Map(second.filter(Boolean).flatMap(r => r.verdicts || []).map(v => [v.anomalyId, v]))
  verdicts = verdicts.map(v => { const o = overrides.get(v.anomalyId); return (o && v.verdict === 'FALSE' && o.verdict !== 'FALSE') ? { ...o, why: '[OVERRIDE by ' + SEC_ESCALATE_MODEL + '] ' + (o.why || '') } : v })
}
let confirmed = verdicts.filter(v => v.verdict === 'CONFIRMED')
let unproven = verdicts.filter(v => v.verdict === 'UNPROVEN')
let killed = verdicts.filter(v => v.verdict === 'FALSE')
log('Judged: ' + confirmed.length + ' confirmed · ' + killed.length + ' false · ' + unproven.length + ' unproven · thread walks: ' + walks.length + ' · digest findings: ' + digests.reduce((n, d) => n + d.findings.length, 0))

// ─── Phase 3.5: Final judgment — ONE Opus rules the whole walk before anything is written ─────
const FINAL = {
  type: 'object', properties: {
    verdict: { type: 'string', enum: ['SMOOTH SAILING', 'MOSTLY GOOD', 'ROUGH SEAS', 'SHIPWRECK'] },
    reinstated: { type: 'array', items: { type: 'object', properties: { anomalyId: { type: 'string' }, why: { type: 'string' } }, required: ['anomalyId', 'why'] } },
    missedRisks: { type: 'array', items: { type: 'object', properties: { what: { type: 'string' }, where: { type: 'string' }, severity: { type: 'string' }, jc: { type: 'string' } }, required: ['what', 'where'] } },
    rationale: { type: 'string' },
  }, required: ['verdict', 'missedRisks'],
}
const finalJudge = await resilient(
  'You are the FINAL JUDGE of this wave walk — one Opus ruling over the WHOLE result before the review is written. Complete inputs: '
  + 'THREAD WALKS: ' + JSON.stringify(walks.map(w => ({ id: w.threadId, name: w.name, flow: w.flow, defects: w.defects, notes: w.notes })))
  + ' · CONFIRMED anomalies: ' + JSON.stringify(confirmed)
  + ' · UNPROVEN: ' + JSON.stringify(unproven)
  + ' · KILLED as FALSE (re-examine — a wrong kill hides here): ' + JSON.stringify(killed.map(v => ({ anomalyId: v.anomalyId, why: v.why, anomaly: anomalyById.get(v.anomalyId) })))
  + ' · Territory digests: ' + JSON.stringify(digests)
  + ' · SECURITY AUDIT (diff-scoped ' + SECURITY_DOC + '): ' + (security ? JSON.stringify(secFindings) + ' (swept: ' + (security.categoriesSwept || []).join(',') + ')' : 'AUDIT DIED — a coverage hole')
  + ' · Coverage: threads ' + walks.length + '/' + threads.length + ', fields sensed ' + cards.length + ', UNSENSED: ' + (unsensed.length ? unsensed.join(', ') : 'none') + '\n'
  + 'Rule the wave: (1) the authoritative verdict on the SMOOTH SAILING | MOSTLY GOOD | ROUGH SEAS | SHIPWRECK scale — weigh broken threads, confirmed severity, security findings, and coverage holes; (2) reinstate any killed anomaly whose kill reasoning does not hold (open the files yourself before reinstating); (3) missedRisks — cross-cutting hazards only the whole picture shows (a pattern repeating across threads, an unsensed-field cluster over {DOMAIN_ADJ} surface, digest smells that compound). Judge evidence, not vibes.' + RO
  + ' Structured output: verdict, reinstated, missedRisks, rationale.',
  { label: 'final-judge', phase: 'Fold', model: FINAL_JUDGE_MODEL, schema: FINAL },
)
if (finalJudge && (finalJudge.reinstated || []).length) {
  const re = new Map(finalJudge.reinstated.map(r => [r.anomalyId, r]))
  verdicts = verdicts.map(v => (re.has(v.anomalyId) && v.verdict === 'FALSE') ? { ...v, verdict: 'CONFIRMED', why: '[REINSTATED by final judge] ' + re.get(v.anomalyId).why } : v)
  confirmed = verdicts.filter(v => v.verdict === 'CONFIRMED')
  unproven = verdicts.filter(v => v.verdict === 'UNPROVEN')
  killed = verdicts.filter(v => v.verdict === 'FALSE')
  log('Final judge reinstated ' + finalJudge.reinstated.length + ' killed verdict(s)')
}
if (finalJudge) log('Final judgment: ' + finalJudge.verdict + ' · ' + finalJudge.missedRisks.length + ' missed risk(s)')

// ─── Phase 4: Fold — merge thread walks + confirmed anomalies + digests → the wave review ─────
const coverageSummary = 'threads walked: ' + walks.length + '/' + threads.length + ' · fields sensed: ' + cards.length + ' · UNSENSED: ' + (unsensed.length ? unsensed.join(', ') : 'none')
  + ' · gates: ' + gates.length + ' · ledger anomalies: ' + anomalies.length + ' → confirmed ' + confirmed.length + ', false ' + killed.length + ', unproven ' + unproven.length
  + ' · security: ' + (security ? secFindings.length + ' finding(s) over ' + (security.categoriesSwept || []).length + ' categories' : 'AUDIT DIED')
const fold = await resilient(
  'You are the FOLD of a wave-walker review. Merge the two walks into ONE review and WRITE it into the report at ' + REPORT_PATH + ' under a `## Professor\'s Wave Review` section (create/overwrite ONLY that section of that file; run no git).\n'
  + 'Inputs:\n· THREAD WALKS (functional flow + hygiene, the floor): ' + JSON.stringify(walks) + '\n'
  + '· LEDGER anomalies CONFIRMED (mechanical, file-verified by judges): ' + JSON.stringify(confirmed) + '\n· Ledger UNPROVEN (needs human eyes): ' + JSON.stringify(unproven) + '\n· Ledger killed-as-false: ' + killed.length + ' (one line)\n'
  + '· Territory digests: ' + JSON.stringify(digests) + '\n· SECURITY AUDIT (diff-scoped): ' + (security ? JSON.stringify({ findings: secFindings, categoriesSwept: security.categoriesSwept, summary: security.summary }) : 'AUDIT DIED — name it in Coverage as an explicit hole') + '\n· Coverage: ' + coverageSummary + '\n'
  + (finalJudge ? '· FINAL JUDGMENT (authoritative): verdict=' + finalJudge.verdict + ' · missedRisks: ' + JSON.stringify(finalJudge.missedRisks) + ' · rationale: ' + (finalJudge.rationale || '') + '\n' : '')
  + 'Fold rules: every functional defect (thread) AND every confirmed ledger anomaly AND every digest fix AND every security finding becomes a `### /jc Action Items` line (deduped — a thread defect and a ledger anomaly at the same anchor are ONE item). '
  + (finalJudge ? 'ADOPT the FINAL JUDGMENT verdict verbatim; fold each missedRisk into the review (fixable → an action item, else Unproven/needs-eyes). ' : '')
  + 'The verdict weighs BOTH: a broken thread flow OR a confirmed critical/high ledger anomaly sinks it. HONESTY: the Coverage note MUST name every UNSENSED field as a hole.\n'
  + 'Report format (per wave/walker.md § Report Format): ## Professor\'s Wave Review (Wave · Date · Verdict); Executive Summary; Thread Walk table; Ledger Anomalies by rule (Expected/Got + anchors + severity); Territory Digests; Security Audit (per-category Expected/Got, or None); ### /jc Action Items; Coverage.\n'
  + 'Verdict: SMOOTH SAILING (nothing) | MOSTLY GOOD (minor only) | ROUGH SEAS (a confirmed high or a BROKEN thread) | SHIPWRECK (a confirmed critical / security, or multiple broken flows).'
  + ' Structured output: verdict, actionItems (verbatim /jc lines), review (the full markdown you wrote).',
  { label: 'fold', phase: 'Fold', model: FOLD_MODEL, schema: FOLD },
)
if (!fold) return { status: 'FAILED', detail: 'fold died twice', threads: threads.length, anomalies: anomalies.length, confirmed: confirmed.length }

const ledger = { report: REPORT_PATH, headSha: scout.headSha, territories: scout.territories, changedFiles: scout.changedFiles, mergeShas: scout.mergeShas, threads, walks, cards, gateCards: gates, undeclaredReads, anomalies, verdicts, digests, security: security || null, coverage: coverageSummary }
log('Wave walker complete · ' + fold.verdict + ' · ' + coverageSummary + ' · ledger in result (persist to ' + LEDGER_PATH + ')')
return {
  status: 'DONE', verdict: fold.verdict, actionItems: fold.actionItems, review: fold.review,
  threads: threads.length, threadsWalked: walks.length,
  ledgerAnomalies: anomalies.length, anomaliesByRule: ruleCounts,
  confirmed: confirmed.map(v => ({ id: v.anomalyId, severity: v.severity, what: v.what, location: v.location })),
  unproven: unproven.length, killedAsFalse: killed.length,
  overrides: verdicts.filter(v => (v.why || '').startsWith('[OVERRIDE')).length,
  digestFindings: digests.reduce((n, d) => n + d.findings.length, 0),
  security: security ? { findings: secFindings, categoriesSwept: security.categoriesSwept || [], summary: security.summary || '' } : null,
  finalJudge: finalJudge ? { verdict: finalJudge.verdict, missedRisks: finalJudge.missedRisks.length, reinstated: (finalJudge.reinstated || []).length } : null,
  unsensedFields: unsensed, coverage: coverageSummary,
  reportPath: REPORT_PATH, ledgerTarget: LEDGER_PATH, ledger,
}
