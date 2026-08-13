// Transcribe ONE real production walk into the promotion gate's artifact. This is a TRANSCRIBER, not a
// gatekeeper: it copies the walk's verdict faithfully, writes the artifact whether the walk passed or
// failed, and exits nonzero when the result would not satisfy activate.js. The artifact it replaces was
// hand-authored — every fact here now traces to a field in the walk evidence or to a byte on disk.

import { createHash } from 'node:crypto';
import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { assertProductionWalkEvidence } from './production-walk-evidence.js';

const HERE = dirname(fileURLToPath(import.meta.url));
const DIST = join(HERE, 'dist');
const LEGACY = join(DIST, 'workflow.js');
const CANDIDATE = join(DIST, 'cross-workflow', 'claude', 'workflow.js');
const MANIFEST = join(DIST, 'cross-workflow', 'claude', 'manifest.json');
const OUTPUT = join(DIST, 'cross-workflow', 'production-walk.json');

// The escape hatches that would make a walk's tool observations meaningless. Enumerated by NAME so the
// artifact can state what was checked — an unnamed "we looked" is the empty enumeration this codebase
// treats as a non-verdict.
const BYPASS_FLAGS = [
  '--dangerously-skip-permissions',
  '--dangerously-bypass-approvals-and-sandbox',
  'bypassPermissions',
  'acceptEdits',
];

function sha256(value) {
  return createHash('sha256').update(value).digest('hex');
}

function parseArguments(argv) {
  let evidencePath = null;
  let launcherPaths = [];
  for (let index = 0; index < argv.length; index += 1) {
    const name = argv[index];
    const value = argv[index + 1];
    if (value === undefined) throw new Error(`missing value for ${name}`);
    index += 1;
    if (name === '--evidence') evidencePath = resolve(value);
    else if (name === '--launcher') launcherPaths.push(resolve(value));
    else throw new Error('Usage: node production-walk.js --evidence FILE [--launcher FILE ...]');
  }
  if (!evidencePath) throw new Error('--evidence FILE is required');
  return { evidencePath, launcherPaths };
}

// permissionsBypassed is a SECURITY claim, so it names the pins that prove it rather than being asserted.
// Two independent pins: the launcher sources contain no bypass flag (static), and the walk's own Bash
// payloads all stayed inside the declared read grammar (runtime — the fence actually held during THIS
// walk). Static alone could miss a dynamically built argv; runtime alone could miss a walk that made no
// Bash calls at all. Together they are a conjunction, not a guess.
function proveNoPermissionBypass(evidence, launcherPaths) {
  const launchers = launcherPaths.map((path) => {
    if (!existsSync(path)) throw new Error(`launcher source is missing, cannot pin permissions: ${path}`);
    const source = readFileSync(path, 'utf8');
    const present = BYPASS_FLAGS.filter((flag) => source.includes(flag));
    return { path, sha256: sha256(source), bypassFlagsPresent: present };
  });
  if (launchers.length === 0) throw new Error('at least one --launcher is required to pin permissionsBypassed');
  const staticClean = launchers.every((entry) => entry.bypassFlagsPresent.length === 0);
  const outside = evidence.agents?.outsideDeclaredReadGrammar;
  const attempts = Array.isArray(outside) ? outside.length : null;
  // An ATTEMPT outside the grammar is not a BREACH — the fence denies before execution, so a denied
  // command proves the fence WORKED. The evidence schema records attempts but not their per-attempt
  // denial verdict, so the fence's action cannot be established from this artifact's inputs alone.
  // Absent that proof we fail CLOSED, and we say why rather than reporting a bypass that did not happen.
  // The evidence now records a per-attempt fence verdict, so the runtime pin is no longer "nobody tried"
  // — it is "every attempt was refused before execution", which is the stronger claim and the one that
  // actually proves the control acted. A walk that made no attempt at all still passes, trivially.
  const verdicts = Array.isArray(outside) ? outside.map((record) => record?.fenceDenied) : [];
  const allDenied = verdicts.length > 0 && verdicts.every((verdict) => verdict === true);
  const runtimeProven = attempts === 0 || allDenied;
  return {
    bypassed: !(staticClean && runtimeProven),
    proof: {
      flagsChecked: BYPASS_FLAGS,
      launchers,
      staticClean,
      outsideGrammarAttempts: attempts,
      attemptsProvenDenied: verdicts.filter((verdict) => verdict === true).length,
      attemptsWithUnknownVerdict: verdicts.filter((verdict) => verdict !== true).length,
      verdictBasis:
        attempts === 0
          ? 'no out-of-grammar attempt was made; static pin alone carries the verdict'
          : allDenied
            ? 'every out-of-grammar attempt was refused by the pre-execution fence before it ran'
            : 'FAIL-CLOSED: at least one out-of-grammar attempt has no proven denial verdict. This is NOT ' +
              'an observation that permissions were bypassed — it is the absence of proof that they were not.',
      coverage:
        'Static: every launcher source scanned for each named flag. Runtime: EVERY out-of-grammar Bash ' +
        'attempt was paired to its tool_result and judged individually; an unpaired attempt counts as ' +
        'unknown and fails closed.',
    },
  };
}

// The walk evidence's own invocationCount is a CAMPAIGN counter (walks consumed to date). The gate's
// field is per-artifact — sibling evidence files each record 1 for their own single walk. Same name, two
// meanings, so the drift is recorded here in the open instead of being silently reconciled.
function describeInvocations(evidence) {
  const claude = evidence.claude ?? {};
  for (const key of ['sessionId', 'runId', 'taskId']) {
    if (typeof claude[key] !== 'string' || claude[key].length === 0)
      throw new Error(`walk evidence does not identify exactly one invocation: ${key} is absent`);
  }
  const preceding = [];
  for (const prior of [evidence.priorFailedEvidence, evidence.precedingConfirmation]) {
    if (!prior || typeof prior.path !== 'string') continue;
    // A prior failure that has been DELETED cannot be verified, and a campaign that can erase its own
    // failures can re-roll until something looks green. Refuse rather than transcribe an unverifiable past.
    if (!existsSync(prior.path)) throw new Error(`a preceding walk's evidence is missing: ${prior.path}`);
    const actual = sha256(readFileSync(prior.path));
    if (typeof prior.sha256 === 'string' && actual !== prior.sha256)
      throw new Error(`a preceding walk's evidence was modified after the fact: ${prior.path}`);
    preceding.push({ path: prior.path, sha256: actual, status: prior.status ?? null });
  }
  return {
    invocationCount: 1,
    campaign: {
      sourceInvocationCountField: evidence.invocationCount ?? null,
      note: 'source field counts walks consumed campaign-wide; this artifact describes exactly one walk',
      precedingInvocations: preceding,
    },
  };
}

const options = parseArguments(process.argv.slice(2));
const evidence = JSON.parse(readFileSync(options.evidencePath, 'utf8'));
const legacySha256 = sha256(readFileSync(LEGACY));
const candidateSha256 = sha256(readFileSync(CANDIDATE));
const manifest = JSON.parse(readFileSync(MANIFEST, 'utf8'));

// Bind the transcript to the bytes that were actually walked. Without this, a stale evidence file would
// promote a bundle nobody exercised — the exact failure the equivalence gate already learned to catch.
if (evidence.candidateBundle?.sha256 !== candidateSha256)
  throw new Error(
    `walk exercised a different candidate: evidence ${evidence.candidateBundle?.sha256} vs on-disk ${candidateSha256}`,
  );
if (evidence.legacyBundleSha256 !== legacySha256)
  throw new Error(`walk recorded a different legacy bundle: ${evidence.legacyBundleSha256}`);

const filesystem = evidence.filesystemProof ?? {};
const unexpected = filesystem.unexpectedModifiedFiles ?? [];
const walkerWrites = filesystem.ignoredAmbientAttribution?.walkerWriteCapableToolCalls ?? null;
const permissions = proveNoPermissionBypass(evidence, options.launcherPaths);
const invocations = describeInvocations(evidence);

const artifact = {
  format: 'wave-walker-production-walk/1',
  recordedAt: evidence.recordedAt ?? null,
  status: evidence.status ?? 'FAIL',
  invocationCount: invocations.invocationCount,
  candidateSha256,
  workflowHash: manifest.workflowHash,
  legacySha256,
  permissionsBypassed: permissions.bypassed,
  resultStatus: evidence.result?.status ?? null,
  verdict: evidence.result?.verdict ?? null,
  agentCount: evidence.agents?.started ?? 0,
  agentFailures: evidence.agents?.incomplete ?? 0,
  repositoryUnchangedOutsideArtifacts:
    filesystem.protectedScopeUnchanged === true && unexpected.length === 0 && walkerWrites === 0,
  sessionId: evidence.claude.sessionId,
  runId: evidence.claude.runId,
  taskId: evidence.claude.taskId,
  toolCounts: evidence.agents?.toolCounts ?? {},
  filesystem: {
    protectedScopeUnchanged: filesystem.protectedScopeUnchanged ?? null,
    beforeSha256: filesystem.beforeSha256 ?? null,
    afterSha256: filesystem.afterSha256 ?? null,
    unexpectedModifiedFiles: unexpected,
    walkerWriteCapableToolCalls: walkerWrites,
  },
  permissionsBypassProof: permissions.proof,
  campaign: invocations.campaign,
  workflowHashBinding: 'manifest.json of the candidate whose bytes match the walked bundle sha256',
  sourceEvidence: options.evidencePath,
  sourceEvidenceSha256: sha256(readFileSync(options.evidencePath)),
};

// Write FIRST, judge second. A failed walk still deserves a durable, honest artifact — suppressing it
// would leave the gate reading yesterday's file and calling that evidence.
writeFileSync(OUTPUT, `${JSON.stringify(artifact, null, 2)}\n`, { encoding: 'utf8', mode: 0o600 });
try {
  assertProductionWalkEvidence(artifact, { legacySha256, candidateSha256, workflowHash: manifest.workflowHash });
} catch (caught) {
  process.stderr.write(
    `production walk artifact written, but it does NOT satisfy the promotion gate\n` +
      `  status                            : ${artifact.status}\n` +
      `  resultStatus                      : ${artifact.resultStatus}\n` +
      `  verdict                           : ${JSON.stringify(artifact.verdict)}\n` +
      `  invocationCount                   : ${artifact.invocationCount}\n` +
      `  permissionsBypassed               : ${artifact.permissionsBypassed}\n` +
      `  agentCount / agentFailures        : ${artifact.agentCount} / ${artifact.agentFailures}\n` +
      `  repositoryUnchangedOutsideArtifacts: ${artifact.repositoryUnchangedOutsideArtifacts}\n` +
      `  ${caught.message}\n`,
  );
  process.exit(1);
}
process.stdout.write(`${JSON.stringify(artifact, null, 2)}\n`);
