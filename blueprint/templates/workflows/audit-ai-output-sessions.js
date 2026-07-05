export const meta = {
  name: 'audit-ai-output-sessions',
  description: 'Per-unit AI-output faithfulness audit — discover every unit with channel output, fan out one auditor per unit (data-first/code-last paired walk), then a synthesizer that quantifies failure rates and WRITES the report to .professor/AUDIT/ai-output/{date}-{channel}.md (returns only a pointer). Declared copy of audit/ai-output.md Step 3.',
  phases: [
    { title: 'Discover', detail: 'enumerate every unit of every subject with output for the chosen channel(s)' },
    { title: 'Audit', detail: 'one auditor per unit — paired source-vs-output walk, in parallel' },
    { title: 'Synthesize', detail: 'quantify failure rates, write the report to .professor/AUDIT/ai-output/, return a pointer' },
  ],
}

// DOMAIN-HYDRATED SHELL: the flow, schemas, and fan-out are universal. The per-project
// detail — how to query the store ({DB_QUERY_CMD}), the source table + its locator/actor
// columns, the channel→output-table mapping, and the category taxonomy — is filled in at
// install by mapping this onto your AI pipeline. Placeholders below marked {LIKE_THIS}.

// args: { channel: "<channel-name>", exclude?: [unitId], frontierModel?: alias } — defensive against args arriving stringified.
const A = typeof args === 'string' ? JSON.parse(args) : args || {}
const CHANNEL = A.channel || '{DEFAULT_CHANNEL}'
const EXCLUDE = A.exclude || []
// Faithfulness verdict seats — never below the frontier tier; a limited-time frontier alias rides per-invocation, durable default opus (frontier ladder: root CLAUDE.md § Model Selection).
const FRONTIER = A.frontierModel || 'opus'

const DISCOVER_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['table', 'units'],
  properties: {
    table: { type: 'string', description: 'the output table/collection this channel writes to' },
    units: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['id', 'subject', 'rows'],
        properties: { id: { type: 'string' }, subject: { type: 'string' }, rows: { type: 'number' } },
      },
    },
  },
}

const FINDING_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  required: ['unit_id', 'subject', 'verdict', 'rows_audited', 'findings', 'missed', 'notes'],
  properties: {
    unit_id: { type: 'string' },
    subject: { type: 'string' },
    verdict: { type: 'string', enum: ['FAITHFUL', 'MOSTLY_FAITHFUL', 'UNFAITHFUL', 'EMPTY'] },
    rows_audited: { type: 'number' },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        additionalProperties: false,
        required: ['index', 'field', 'got', 'expected', 'severity', 'evidence'],
        properties: {
          index: { type: 'string' },
          field: { type: 'string' },
          got: { type: 'string' },
          expected: { type: 'string' },
          severity: { type: 'string', enum: ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW'] },
          evidence: { type: 'string', description: 'source snippet proving the correct value' },
        },
      },
    },
    missed: { type: 'array', description: 'codable content in the source the model failed to output', items: { type: 'string' } },
    notes: { type: 'string' },
  },
}

phase('Discover')
const disc = await agent(
  `Enumerate the audit universe for the AI pipeline "${CHANNEL}" channel. READ-ONLY.
- Find the channel's output table/collection: read the pipeline's store-write layer and locate the module's write statement for this channel.
- Query the store with the project's sanctioned DB/query command (single-line query only): return EVERY unit of EVERY real subject with its output-row count for that table — INCLUDE 0-row units (they get a completeness check). Resolve each unit's subject name. Exclude orphaned placeholder units that have no real subject.
Return {table, units:[{id, subject, rows}]}.`,
  { model: 'sonnet', schema: DISCOVER_SCHEMA, phase: 'Discover', label: `discover:${CHANNEL}` }
)

const units = (disc.units || []).filter((u) => !EXCLUDE.includes(u.id))
log(`Channel ${CHANNEL} -> table ${disc.table}: ${units.length} units to audit (${EXCLUDE.length} excluded). Fanning out one auditor per unit...`)

phase('Audit')
const results = (
  await parallel(
    units.map((u) => () =>
      agent(
        `You are an AI-output faithfulness auditor for ONE unit, auditing the "${CHANNEL}" channel (table ${disc.table}). READ-ONLY: report findings, change nothing. Method = data-first, code-last.

UNIT: ${u.id} (subject ${u.subject}, ~${u.rows} output rows).
DATA ACCESS: the store only, via the project's sanctioned DB/query command (single-line query; never a raw client).
- A. Source (what went IN): read the source input for this unit from the source table — preserve its stable locator (index/id) and resolve any coded actor/speaker fields via the unit's mapping.
- B. Output (what came OUT): read this unit's rows from ${disc.table} ordered by locator — confirm columns + created-at freshness.
- C. Contract (read LAST): the channel's chain code (deterministic guards/post-filters) + its prompt under the knowledge/prompt registry — open ONLY to localize a discrepancy the data walk already exposed, never to pre-judge.

WALK: pair source with output FIRST. For each output row, find its source unit(s), read what was actually there, and judge — faithfulness (no fabricated quotes/labels), grounding (the label is justified by the content), actor accuracy, instruction/scope compliance (no forbidden output — sacred ground), and any label whose code contradicts its own stated reason. Then completeness: codable content the model MISSED (a 0-row unit is a pure completeness check — read the source and report whether anything codable was dropped). Open the code only to root-cause what the data already flagged.

Return the structured object. verdict: FAITHFUL (clean), MOSTLY_FAITHFUL (minor), UNFAITHFUL (a clear or CRITICAL error), EMPTY (0 rows and correctly so). Put each flagged row in findings with severity + Expected-vs-Got + the source evidence; put missed codable content in missed.`,
        { model: FRONTIER, schema: FINDING_SCHEMA, phase: 'Audit', label: `audit:${u.subject}:${u.id.slice(0, 8)}` }
      )
    )
  )
).filter(Boolean)

const flagged = results.filter((r) => r.verdict === 'UNFAITHFUL' || r.verdict === 'MOSTLY_FAITHFUL')
log(`Audited ${results.length} units — ${flagged.length} flagged. Synthesizing...`)

phase('Synthesize')
const synth = await agent(
  `You are the SYNTHESIZER for a per-unit AI-output faithfulness audit of the "${CHANNEL}" channel across ${results.length} units (output table ${disc.table}). READ-ONLY on the data; you WRITE exactly one report file.

1. AGGREGATE the per-unit findings (JSON below). Separate confirmed CRITICAL/HIGH from MEDIUM/LOW/borderline.

2. QUANTIFY — this is the headline the user reads FIRST:
   - OVERALL: total audited output rows, total confirmed-wrong rows, and the failure % (wrong / total).
   - PER CATEGORY: cluster the findings by failure type (the channel's own label confusions, miscodes, missed items, fabrication, gate violations). For EACH category report: failures, the category's POPULATION (its denominator — query the store for it, single-line query), the % WITHIN the category (failures / population), and the category's SHARE of all failures. State small n explicitly (a 1/1 = 100% is one edge case, not an epidemic — say so).

3. WRITE the full report to a file YOURSELF:
   - Stamp the date: run \`date +%Y-%m-%d\` via Bash.
   - \`mkdir -p .professor/AUDIT/ai-output\`, then Write the full markdown to \`.professor/AUDIT/ai-output/<date>-${CHANNEL}.md\`.
   - The file leads with the OVERALL-% line and the PER-CATEGORY % table (user reads these first), then: verdict, per-subject roll-up, the consolidated CRITICAL/HIGH findings table (subject·unit·index·field·Got·Expected·severity·evidence), completeness (missed content per subject), the recurring-pattern read (which ONE root confusion explains the most failures — where a single example would help most), and recommendations (prompt fixes -> /km, code/guard fixes -> /jc; do NOT remediate).

4. RETURN ONLY a pointer + the headline numbers — never the full report (the requesting chat reads the file for detail).

PER-UNIT RESULTS (JSON):
${JSON.stringify(results, null, 2)}`,
  {
    model: FRONTIER,
    schema: {
      type: 'object',
      additionalProperties: false,
      required: ['report_path', 'overall_verdict', 'total_outputs', 'total_failures', 'overall_failure_pct', 'categories', 'units_audited', 'units_flagged'],
      properties: {
        report_path: { type: 'string', description: '.professor/AUDIT/ai-output/<date>-<channel>.md' },
        overall_verdict: { type: 'string', enum: ['FAITHFUL', 'MOSTLY_FAITHFUL', 'UNFAITHFUL'] },
        total_outputs: { type: 'number' },
        total_failures: { type: 'number' },
        overall_failure_pct: { type: 'number' },
        categories: {
          type: 'array',
          items: {
            type: 'object',
            additionalProperties: false,
            required: ['name', 'failures', 'population', 'pct_within_category', 'pct_of_all_failures'],
            properties: {
              name: { type: 'string' },
              failures: { type: 'number' },
              population: { type: 'number' },
              pct_within_category: { type: 'number' },
              pct_of_all_failures: { type: 'number' },
            },
          },
        },
        units_audited: { type: 'number' },
        units_flagged: { type: 'number' },
      },
    },
    phase: 'Synthesize',
    label: 'synthesize',
  }
)
return synth
