// Atomically point dist/active-workflow.js at the pinned legacy bundle or at a deterministic,
// live-smoke, headless-equivalence, and full-production-walk proven cross-workflow candidate.
// Production dist/workflow.js is immutable; rollback is `npm run activate:legacy`.

import { createHash } from 'node:crypto';
import { existsSync, lstatSync, readFileSync, renameSync, symlinkSync, unlinkSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { assertEquivalenceEvidence, assertHeadlessEvidence } from './headless-equivalence-lib.js';
import { assertProductionWalkEvidence } from './production-walk-evidence.js';

const HERE = dirname(fileURLToPath(import.meta.url));
const DIST = join(HERE, 'dist');
const ACTIVE = join(DIST, 'active-workflow.js');
const LEGACY = join(DIST, 'workflow.js');
const CANDIDATE = join(DIST, 'cross-workflow', 'claude', 'workflow.js');
const EVIDENCE = join(DIST, 'cross-workflow', 'equivalence.json');
const RUNTIME_SMOKE = join(DIST, 'cross-workflow', 'runtime-smoke.json');
const HEADLESS_EQUIVALENCE = join(DIST, 'cross-workflow', 'headless-equivalence.json');
const PRODUCTION_WALK = join(DIST, 'cross-workflow', 'production-walk.json');
const MANIFEST = join(DIST, 'cross-workflow', 'claude', 'manifest.json');
const HEADLESS_RUNNER = join(HERE, 'headless-equivalence.js');
const HEADLESS_LIBRARY = join(HERE, 'headless-equivalence-lib.js');
const HEADLESS_INPUT = join(HERE, 'headless-equivalence-input.json');
const HEADLESS_PROMPT = join(HERE, 'headless-equivalence.prompt.md');
const LEGACY_SHA256 = 'ccc0ca322a5b82c90b5b1a1ae575410507e666db650fbccc1ef2f5f3e666094b';

function sha256(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex');
}

function assertCandidateProven() {
  if (!existsSync(EVIDENCE))
    throw new Error('candidate activation requires dist/cross-workflow/equivalence.json');
  assertEquivalenceEvidence(JSON.parse(readFileSync(EVIDENCE, 'utf8')), {
    legacySha256: sha256(LEGACY),
    candidateSha256: sha256(CANDIDATE),
  });
  if (!existsSync(HEADLESS_EQUIVALENCE))
    throw new Error('candidate activation requires dist/cross-workflow/headless-equivalence.json');
  const manifest = JSON.parse(readFileSync(MANIFEST, 'utf8'));
  assertHeadlessEvidence(JSON.parse(readFileSync(HEADLESS_EQUIVALENCE, 'utf8')), {
    legacySha256: sha256(LEGACY),
    candidateSha256: sha256(CANDIDATE),
    workflowHash: manifest.workflowHash,
    runnerSha256: sha256(HEADLESS_RUNNER),
    librarySha256: sha256(HEADLESS_LIBRARY),
    inputFileSha256: sha256(HEADLESS_INPUT),
    promptTemplateSha256: sha256(HEADLESS_PROMPT),
  });
  if (!existsSync(RUNTIME_SMOKE))
    throw new Error('candidate activation requires dist/cross-workflow/runtime-smoke.json');
  const runtimeSmoke = JSON.parse(readFileSync(RUNTIME_SMOKE, 'utf8'));
  if (
    runtimeSmoke.format !== 'wave-walker-runtime-smoke/1' ||
    runtimeSmoke.candidateSha256 !== sha256(CANDIDATE) ||
    runtimeSmoke.workflowHash !== manifest.workflowHash ||
    runtimeSmoke.codex?.authentication !== 'Logged in using ChatGPT' ||
    runtimeSmoke.codex?.unsafeHostDiagnostic?.consensus !== 'CONFIRMED' ||
    runtimeSmoke.codex?.unsafeHostDiagnostic?.sandboxEvidence !== false ||
    runtimeSmoke.claude?.consensus !== 'CONFIRMED' ||
    runtimeSmoke.claude?.permissionsBypassed !== false
  )
    throw new Error('candidate real-runtime smoke evidence is stale or incomplete');
  if (!existsSync(PRODUCTION_WALK))
    throw new Error('candidate activation requires dist/cross-workflow/production-walk.json');
  assertProductionWalkEvidence(JSON.parse(readFileSync(PRODUCTION_WALK, 'utf8')), {
    legacySha256: sha256(LEGACY),
    candidateSha256: sha256(CANDIDATE),
    workflowHash: manifest.workflowHash,
  });
}

const target = process.argv[2];
if (!['legacy', 'candidate'].includes(target))
  throw new Error('Usage: node activate.js legacy|candidate');
if (sha256(LEGACY) !== LEGACY_SHA256)
  throw new Error('legacy production bundle does not match its immutable pin');
if (target === 'candidate') assertCandidateProven();
if (existsSync(ACTIVE) && !lstatSync(ACTIVE).isSymbolicLink())
  throw new Error('refusing to replace a non-symlink active-workflow.js');
const linkTarget = target === 'legacy' ? 'workflow.js' : 'cross-workflow/claude/workflow.js';
const temporary = join(DIST, `.active-workflow.${process.pid}.tmp`);
try {
  symlinkSync(linkTarget, temporary);
  renameSync(temporary, ACTIVE);
} finally {
  if (existsSync(temporary)) unlinkSync(temporary);
}
process.stdout.write(`active Wave Walker pointer → ${linkTarget}\n`);
