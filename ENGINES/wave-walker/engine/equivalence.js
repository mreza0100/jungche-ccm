// Run the immutable legacy and side-by-side cross-workflow Claude bundles against the exact same
// deterministic walk input. This is the promotion gate: same merge commit, same report path, same
// agent outputs, then exact prompt/options and result comparison. It does not pretend to be a live-LLM
// quality test; it proves adapter/bundle semantics before a reversible pointer can move.

import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync, realpathSync, writeFileSync } from 'node:fs';
import { dirname, isAbsolute, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { isDeepStrictEqual } from 'node:util';

import { assertClaudeBundle, executeHarnessBundle } from 'cross-workflow';

const HERE = dirname(fileURLToPath(import.meta.url));
const LEGACY = resolve(HERE, 'dist', 'workflow.js');
const CANDIDATE = resolve(HERE, 'dist', 'cross-workflow', 'claude', 'workflow.js');

function parseArguments(argv) {
  const options = { repository: undefined, mergeSha: undefined, reportPath: undefined, output: undefined };
  for (let index = 0; index < argv.length; index += 1) {
    const name = argv[index];
    const value = argv[index + 1];
    if (!['--repository', '--merge-sha', '--report-path', '--output'].includes(name) || value === undefined)
      throw new Error('Usage: node equivalence.js --repository DIR --merge-sha SHA --report-path FILE --output FILE');
    index += 1;
    if (name === '--repository') options.repository = resolve(value);
    else if (name === '--merge-sha') options.mergeSha = value;
    else if (name === '--report-path') options.reportPath = value;
    else options.output = resolve(value);
  }
  if (!options.repository || !options.mergeSha || !options.reportPath || !options.output)
    throw new Error('repository, merge SHA, report path, and output are required');
  if (!/^[0-9a-f]{40}$/u.test(options.mergeSha)) throw new Error('merge SHA must be a full lowercase SHA-1');
  return options;
}

function sha256(value) {
  return createHash('sha256').update(value).digest('hex');
}

function shape(value) {
  if (value === null) return 'null';
  if (Array.isArray(value)) return value.length === 0 ? [] : [shape(value[0])];
  if (typeof value !== 'object') return typeof value;
  return Object.fromEntries(Object.keys(value).sort().map((key) => [key, shape(value[key])]));
}

function assertInside(repository, path) {
  const absolute = realpathSync(isAbsolute(path) ? path : resolve(repository, path));
  const rel = relative(repository, absolute);
  if (rel === '..' || rel.startsWith('../') || isAbsolute(rel)) throw new Error('report path escapes repository');
  return rel;
}

function globals(calls, blob) {
  return {
    async agent(prompt, options) {
      calls.push({ prompt, options });
      return structuredClone(blob);
    },
    parallel(thunks) { return Promise.all(thunks.map((thunk) => thunk())); },
    async pipeline(steps, initial) {
      let value = initial;
      for (const step of steps) value = await step(value);
      return value;
    },
    log() {},
    phase() {},
    workflow: {},
    budget: { total: null, spent: () => 0, remaining: () => Number.MAX_SAFE_INTEGER },
  };
}

async function run(bundle, args, blob) {
  const calls = [];
  const result = await executeHarnessBundle(bundle, args, globals(calls, blob));
  return { result, calls };
}

const options = parseArguments(process.argv.slice(2));
if (!existsSync(LEGACY) || !existsSync(CANDIDATE)) throw new Error('both legacy and candidate bundles must exist');
const parents = execFileSync('git', ['-C', options.repository, 'rev-list', '--parents', '-n', '1', options.mergeSha], { encoding: 'utf8' }).trim().split(/\s+/u);
if (parents[0] !== options.mergeSha || parents.length < 3) throw new Error('the supplied SHA is not a merge commit in the repository');
const changedFiles = execFileSync('git', ['-C', options.repository, 'diff', '--name-only', `${options.mergeSha}^1`, options.mergeSha], { encoding: 'utf8' })
  .split('\n').filter(Boolean).sort();
if (changedFiles.length === 0) throw new Error('the merge commit has no first-parent changed-file denominator');
const reportPath = assertInside(options.repository, options.reportPath);

const blob = {
  headSha: options.mergeSha,
  territories: [],
  changedFiles,
  changedFileCount: changedFiles.length,
  mergeShas: [options.mergeSha],
  threads: [{ id: 'merge-flow', type: 'flow', name: `Merge ${options.mergeSha.slice(0, 12)}`, verify: 'deterministic equivalence' }],
  operations: [], fields: [], jobs: [], gateFiles: [], authRule: '',
  threadId: 'merge-flow', flow: 'INTACT', trace: '', defects: [], hygiene: [],
  name: '', type: '', notes: '', file: changedFiles[0], gates: [], jobId: 'equivalence',
  slices: [], undeclaredReads: [], verdicts: [], territory: 'BE', findings: [], summary: '',
  verdict: 'SMOOTH SAILING', actionItems: [], review: '', reinstated: [], missedRisks: [], rationale: '',
  categoriesSwept: [], filesOpened: changedFiles, claims: [], conflictChecks: [], claimId: '',
  evidence: [], reasoning: '', conflicts: [], laneId: '', leads: [], nothingFound: false,
  resultSoFar: '', keyClaimIds: [], lanes: [], dropLeads: [], stop: { done: true, reason: 'equivalence' },
  audits: [], answer: '', confidence: 'low', report: '',
};
const args = { reportPath, branch: options.mergeSha, debug: false };
const legacySource = readFileSync(LEGACY, 'utf8');
const candidateSource = readFileSync(CANDIDATE, 'utf8');
assertClaudeBundle(legacySource);
assertClaudeBundle(candidateSource);
const legacy = await run(legacySource, args, blob);
const candidate = await run(candidateSource, args, blob);
const exactCallsEqual = isDeepStrictEqual(legacy.calls, candidate.calls);
const exactResultEqual = isDeepStrictEqual(legacy.result, candidate.result);
const legacyShape = shape(legacy.result);
const candidateShape = shape(candidate.result);
const verdictShapeEqual = isDeepStrictEqual(legacyShape, candidateShape);
if (!exactCallsEqual || !exactResultEqual || !verdictShapeEqual)
  throw new Error('legacy and candidate walk behavior diverged');

const evidence = {
  format: 'wave-walker-equivalence/1',
  repository: realpathSync(options.repository),
  mergeSha: options.mergeSha,
  reportPath,
  changedFileCount: changedFiles.length,
  legacySha256: sha256(legacySource),
  candidateSha256: sha256(candidateSource),
  exactCallsEqual,
  exactResultEqual,
  verdictShapeEqual,
  agentCalls: legacy.calls.length,
  resultSha256: sha256(JSON.stringify(legacy.result)),
  resultStatus: legacy.result?.status ?? null,
  verdict: legacy.result?.verdict ?? null,
  verdictShape: legacyShape,
};
writeFileSync(options.output, JSON.stringify(evidence, null, 2) + '\n', { encoding: 'utf8', mode: 0o600 });
process.stdout.write(JSON.stringify(evidence, null, 2) + '\n');
