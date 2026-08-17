// verify.js — rebuilds every cross-workflow target in a disposable directory and byte-diffs it against
// the committed side-by-side candidates. `npm test` runs this first, so Claude code, Codex code, and both
// manifests cannot silently drift from engine/src, the model map, or the pinned compiler package.

import { createHash } from 'node:crypto';
import { lstatSync, readFileSync, mkdtempSync, readlinkSync, rmSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { tmpdir } from 'node:os';
import { runBuild } from './build.js';
import { assertEquivalenceEvidence, assertHeadlessEvidence } from './headless-equivalence-lib.js';
import { assertProductionWalkEvidence } from './production-walk-evidence.js';

const HERE = dirname(fileURLToPath(import.meta.url));
const LEGACY = join(HERE, 'dist', 'workflow.js');
const ACTIVE = join(HERE, 'dist', 'active-workflow.js');
const EQUIVALENCE = join(HERE, 'dist', 'cross-workflow', 'equivalence.json');
const RUNTIME_SMOKE = join(HERE, 'dist', 'cross-workflow', 'runtime-smoke.json');
const HEADLESS_EQUIVALENCE = join(HERE, 'dist', 'cross-workflow', 'headless-equivalence.json');
const PRODUCTION_WALK = join(HERE, 'dist', 'cross-workflow', 'production-walk.json');
const HEADLESS_RUNNER = join(HERE, 'headless-equivalence.js');
const HEADLESS_LIBRARY = join(HERE, 'headless-equivalence-lib.js');
const HEADLESS_INPUT = join(HERE, 'headless-equivalence-input.json');
const HEADLESS_PROMPT = join(HERE, 'headless-equivalence.prompt.md');
const LEGACY_SHA256 = 'ccc0ca322a5b82c90b5b1a1ae575410507e666db650fbccc1ef2f5f3e666094b';
const CANDIDATE_ROOT = join('cross-workflow');
const TARGETS = [
  join(CANDIDATE_ROOT, 'claude', 'workflow.js'),
  join(CANDIDATE_ROOT, 'claude', 'manifest.json'),
  join(CANDIDATE_ROOT, 'codex', 'runner.mjs'),
  join(CANDIDATE_ROOT, 'codex', 'manifest.json'),
];

const fail = (msg) => {
  console.error('✗ verify failed — ' + msg);
  process.exit(1);
};

const scratchDir = mkdtempSync(join(tmpdir(), 'wave-walker-verify-'));
try {
  const legacySha = createHash('sha256').update(readFileSync(LEGACY)).digest('hex');
  if (legacySha !== LEGACY_SHA256)
    fail(
      'legacy production dist/workflow.js moved before the equivalence-gated pointer switch: ' +
        legacySha,
    );
  if (!lstatSync(ACTIVE).isSymbolicLink())
    fail('dist/active-workflow.js must be an explicit symlink pointer');
  const activeTarget = readlinkSync(ACTIVE);
  if (!['workflow.js', join('cross-workflow', 'claude', 'workflow.js')].includes(activeTarget))
    fail('active pointer names an unsupported target: ' + activeTarget);
  await runBuild(join(scratchDir, CANDIDATE_ROOT, 'claude', 'workflow.js'), {
    emitAllTargets: true,
  });
  for (const target of TARGETS) {
    const committedPath = join(HERE, 'dist', target);
    const freshPath = join(scratchDir, target);
    let committed;
    try {
      committed = readFileSync(committedPath, 'utf8');
    } catch (e) {
      fail(committedPath + ' does not exist — run `npm run build` first.\n' + e.message);
    }
    const fresh = readFileSync(freshPath, 'utf8');
    if (fresh !== committed)
      fail(
        'dist/' +
          target +
          ' is STALE — it does not match a fresh cross-workflow compile.\n' +
          '  Run `npm run build` in engine/ and commit every regenerated target alongside the source change.',
      );
  }
  if (activeTarget !== 'workflow.js') {
    let evidence;
    try {
      evidence = JSON.parse(readFileSync(EQUIVALENCE, 'utf8'));
    } catch (e) {
      fail('candidate pointer requires readable equivalence evidence: ' + e.message);
    }
    const candidateSha = createHash('sha256')
      .update(readFileSync(join(HERE, 'dist', activeTarget)))
      .digest('hex');
    try {
      assertEquivalenceEvidence(evidence, { legacySha256: legacySha, candidateSha256: candidateSha });
    } catch (e) {
      fail('active candidate equivalence evidence is stale or incomplete: ' + e.message);
    }
    let runtimeSmoke;
    try {
      runtimeSmoke = JSON.parse(readFileSync(RUNTIME_SMOKE, 'utf8'));
    } catch (e) {
      fail('active candidate requires readable real-runtime smoke evidence: ' + e.message);
    }
    const manifest = JSON.parse(
      readFileSync(join(HERE, 'dist', CANDIDATE_ROOT, 'claude', 'manifest.json'), 'utf8'),
    );
    if (
      runtimeSmoke.format !== 'wave-walker-runtime-smoke/1' ||
      runtimeSmoke.candidateSha256 !== candidateSha ||
      runtimeSmoke.workflowHash !== manifest.workflowHash ||
      runtimeSmoke.codex?.authentication !== 'Logged in using ChatGPT' ||
      runtimeSmoke.codex?.unsafeHostDiagnostic?.consensus !== 'CONFIRMED' ||
      runtimeSmoke.codex?.unsafeHostDiagnostic?.sandboxEvidence !== false ||
      runtimeSmoke.claude?.consensus !== 'CONFIRMED' ||
      runtimeSmoke.claude?.permissionsBypassed !== false
    )
      fail('active candidate real-runtime smoke evidence is stale or incomplete');
    try {
      assertHeadlessEvidence(JSON.parse(readFileSync(HEADLESS_EQUIVALENCE, 'utf8')), {
        legacySha256: legacySha,
        candidateSha256: candidateSha,
        workflowHash: manifest.workflowHash,
        runnerSha256: createHash('sha256').update(readFileSync(HEADLESS_RUNNER)).digest('hex'),
        librarySha256: createHash('sha256').update(readFileSync(HEADLESS_LIBRARY)).digest('hex'),
        inputFileSha256: createHash('sha256').update(readFileSync(HEADLESS_INPUT)).digest('hex'),
        promptTemplateSha256: createHash('sha256')
          .update(readFileSync(HEADLESS_PROMPT))
          .digest('hex'),
      });
    } catch (e) {
      fail('active candidate headless old-vs-new evidence is stale or incomplete: ' + e.message);
    }
    try {
      assertProductionWalkEvidence(JSON.parse(readFileSync(PRODUCTION_WALK, 'utf8')), {
        legacySha256: legacySha,
        candidateSha256: candidateSha,
        workflowHash: manifest.workflowHash,
      });
    } catch (e) {
      fail('active candidate production walk evidence is failed, stale, or incomplete: ' + e.message);
    }
  }
  console.log(
    '✓ legacy pinned; candidates current; active pointer deterministic+headless+production-gated → ' +
      activeTarget,
  );
} finally {
  rmSync(scratchDir, { recursive: true, force: true });
}
