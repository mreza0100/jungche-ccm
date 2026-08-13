// Live old-vs-new proof. Both immutable legacy and cross-workflow-generated Claude bundles run through
// separate headless Claude sessions over one byte-frozen repository snapshot. LLM prose is expected to
// vary; the promotion contract is exact agent prompt/model/effort, exact observable topology, and the
// same grounded terminal verdict semantics. Raw provider prose stays in Claude's private run artifacts.

import assert from 'node:assert/strict';
import { execFile, execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import {
  lstatSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  readlinkSync,
  realpathSync,
  renameSync,
  unlinkSync,
  writeFileSync,
} from 'node:fs';
import { dirname, isAbsolute, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';

import {
  compareHeadlessRuns,
  extractWorkflowInvocation,
  sha256Json,
} from './headless-equivalence-lib.js';
// ONE home for "where does the harness write transcripts". Hardcoding ~/.claude here made this script
// look for evidence a run never wrote, under any account that sets CLAUDE_CONFIG_DIR — and then blame
// the run for being unbindable instead of blaming its own assumption.
import { claudeConfigRoot } from './production-caller.js';

const execute = promisify(execFile);
const HERE = dirname(fileURLToPath(import.meta.url));
const LEGACY = join(HERE, 'dist', 'workflow.js');
const CANDIDATE = join(HERE, 'dist', 'cross-workflow', 'claude', 'workflow.js');
const MANIFEST = join(HERE, 'dist', 'cross-workflow', 'claude', 'manifest.json');
const VALIDATOR = join(HERE, 'validate-bundle.js');
const PROMPT = join(HERE, 'headless-equivalence.prompt.md');
const DEFAULT_INPUT = join(HERE, 'headless-equivalence-input.json');
const DEFAULT_OUTPUT = join(HERE, 'dist', 'cross-workflow', 'headless-equivalence.json');
const RUNNER = fileURLToPath(import.meta.url);
const LIBRARY = join(HERE, 'headless-equivalence-lib.js');

function sha256(value) {
  return createHash('sha256').update(value).digest('hex');
}

function parseArguments(argv) {
  const options = { input: DEFAULT_INPUT, output: DEFAULT_OUTPUT };
  for (let index = 0; index < argv.length; index += 1) {
    const name = argv[index];
    const value = argv[index + 1];
    if (!['--input', '--output'].includes(name) || value === undefined)
      throw new Error('Usage: node headless-equivalence.js [--input FILE] [--output FILE]');
    index += 1;
    if (name === '--input') options.input = resolve(value);
    else options.output = resolve(value);
  }
  return options;
}

function loadInput(path) {
  const value = JSON.parse(readFileSync(path, 'utf8'));
  if (typeof value !== 'object' || value === null || Array.isArray(value))
    throw new Error('headless equivalence input must be an object');
  if (typeof value.repository !== 'string' || !isAbsolute(value.repository))
    throw new Error('headless equivalence repository must be an absolute path');
  if (
    typeof value.workflowArgs !== 'object' ||
    value.workflowArgs === null ||
    Array.isArray(value.workflowArgs)
  )
    throw new Error('headless equivalence workflowArgs must be an object');
  return { repository: realpathSync(value.repository), workflowArgs: value.workflowArgs };
}

function repositorySnapshot(repository) {
  const headSha = execFileSync('git', ['-C', repository, 'rev-parse', 'HEAD'], {
    encoding: 'utf8',
  }).trim();
  if (!/^[0-9a-f]{40}$/u.test(headSha)) throw new Error('repository HEAD is not a full SHA-1');
  const listed = execFileSync(
    'git',
    ['-C', repository, 'ls-files', '-z', '--cached', '--others', '--exclude-standard'],
    { encoding: 'buffer', maxBuffer: 128 * 1024 * 1024 },
  )
    .toString('utf8')
    .split('\0')
    .filter(Boolean)
    .sort();
  if (listed.length === 0) throw new Error('repository snapshot has no visible files');
  const hash = createHash('sha256');
  for (const relativePath of listed) {
    const absolute = join(repository, relativePath);
    let stat;
    try {
      stat = lstatSync(absolute);
    } catch (caught) {
      if (caught?.code !== 'ENOENT') throw caught;
      // Indexed but absent from the working tree — an ordinary staged deletion. Record it as ABSENT
      // rather than crashing or skipping: skipping would make a file that vanishes mid-run invisible
      // to the very hash whose only job is to notice change.
      hash.update(relativePath).update('\0absent\0');
      continue;
    }
    hash.update(relativePath).update('\0');
    if (stat.isSymbolicLink()) hash.update('link\0').update(readlinkSync(absolute));
    else if (stat.isFile())
      hash.update('file\0').update(createHash('sha256').update(readFileSync(absolute)).digest());
    else throw new Error('repository snapshot contains unsupported path type: ' + relativePath);
    hash.update('\0');
  }
  return { headSha, files: listed.length, sha256: hash.digest('hex') };
}

function projectSlug(repository) {
  return repository.replaceAll('/', '-');
}

function agentContracts(repository, sessionId, runId, output) {
  const runDirectory = join(
    claudeConfigRoot(),
    'projects',
    projectSlug(repository),
    sessionId,
    'subagents',
    'workflows',
    runId,
  );
  const labels = new Map(
    output.workflowProgress
      .filter((item) => item.type === 'workflow_agent')
      .map((item) => [item.agentId, item.label]),
  );
  const files = readdirSync(runDirectory)
    .filter((name) => /^agent-[a-z0-9]+\.jsonl$/u.test(name))
    .sort();
  if (files.length !== output.agentCount)
    throw new Error('Workflow agent transcript count does not match output agentCount');
  return files.map((name) => {
    const agentId = name.slice('agent-'.length, -'.jsonl'.length);
    const rows = readFileSync(join(runDirectory, name), 'utf8')
      .split('\n')
      .filter(Boolean)
      .map((line) => JSON.parse(line));
    const prompt = rows.find(
      (row) =>
        row.type === 'user' &&
        row.message?.role === 'user' &&
        typeof row.message?.content === 'string',
    )?.message.content;
    const model = rows.find(
      (row) => row.type === 'assistant' && typeof row.message?.model === 'string',
    )?.message.model;
    const effort = rows.find(
      (row) => row.type === 'assistant' && typeof row.effort === 'string',
    )?.effort;
    const label = labels.get(agentId);
    if (
      typeof prompt !== 'string' ||
      typeof model !== 'string' ||
      typeof effort !== 'string' ||
      typeof label !== 'string'
    )
      throw new Error(
        'Workflow agent transcript is missing prompt/model/effort/label contract evidence',
      );
    return { label, prompt, model, effort };
  });
}

async function runHeadless(repository, bundle, workflowArgs, promptTemplate) {
  const prompt = promptTemplate
    .replace('{{SCRIPT_PATH}}', bundle)
    .replace('{{ARGS_JSON}}', JSON.stringify(workflowArgs));
  const { stdout } = await execute(
    'claude',
    [
      '-p',
      prompt,
      '--output-format',
      'json',
      '--model',
      'haiku',
      '--effort',
      'medium',
      '--tools',
      'Workflow',
      '--allowedTools',
      'Workflow',
    ],
    {
      cwd: repository,
      encoding: 'utf8',
      maxBuffer: 32 * 1024 * 1024,
      timeout: 10 * 60 * 1_000,
    },
  );
  const cli = JSON.parse(stdout);
  assert.equal(cli.is_error, false, 'headless Claude CLI reported an error');
  assert.equal(typeof cli.session_id, 'string', 'headless Claude CLI did not return a session ID');
  const transcriptPath = join(
    claudeConfigRoot(),
    'projects',
    projectSlug(repository),
    `${cli.session_id}.jsonl`,
  );
  const transcript = readFileSync(transcriptPath, 'utf8');
  const invocation = extractWorkflowInvocation(transcript, {
    bundle,
    argsJson: JSON.stringify(workflowArgs),
    runRoot: join(
      claudeConfigRoot(),
      'projects',
      projectSlug(repository),
      cli.session_id,
      'subagents',
      'workflows',
    ),
    taskRoot: join(
      '/tmp',
      `claude-${process.getuid()}`,
      projectSlug(repository),
      cli.session_id,
      'tasks',
    ),
  });
  const outputStat = lstatSync(invocation.outputFile);
  if (!outputStat.isFile() || outputStat.isSymbolicLink())
    throw new Error('Workflow task output is not a regular non-symbolic file');
  const output = JSON.parse(readFileSync(invocation.outputFile, 'utf8'));
  const runId = invocation.runId;
  const contracts = agentContracts(repository, cli.session_id, runId, output);
  return {
    result: output.result,
    output,
    agentContracts: contracts,
    sessionId: cli.session_id,
    runId,
  };
}

function runEvidence(run, bundleSha256) {
  return {
    bundleSha256,
    sessionId: run.sessionId,
    runId: run.runId,
    permissionsBypassed: false,
    resultStatus: run.result?.status ?? null,
    resultMode: run.result?.mode ?? null,
    consensus: run.result?.consensus ?? null,
    agentCount: run.output.agentCount,
    totalTokens: run.output.totalTokens,
    totalToolCalls: run.output.totalToolCalls,
    rawResultSha256: sha256Json(run.result),
  };
}

function writeEvidence(path, value) {
  mkdirSync(dirname(path), { recursive: true });
  const temporary = `${path}.${process.pid}.tmp`;
  try {
    writeFileSync(temporary, JSON.stringify(value, null, 2) + '\n', {
      encoding: 'utf8',
      mode: 0o600,
      flag: 'wx',
    });
    renameSync(temporary, path);
  } finally {
    try {
      unlinkSync(temporary);
    } catch (caught) {
      if (caught?.code !== 'ENOENT') throw caught;
    }
  }
}

const options = parseArguments(process.argv.slice(2));
const input = loadInput(options.input);
const promptTemplate = readFileSync(PROMPT, 'utf8');
const legacySource = readFileSync(LEGACY, 'utf8');
const candidateSource = readFileSync(CANDIDATE, 'utf8');
execFileSync(process.execPath, [VALIDATOR, LEGACY], { stdio: 'pipe' });
execFileSync(process.execPath, [VALIDATOR, CANDIDATE], { stdio: 'pipe' });
const manifest = JSON.parse(readFileSync(MANIFEST, 'utf8'));
const before = repositorySnapshot(input.repository);
const legacy = await runHeadless(input.repository, LEGACY, input.workflowArgs, promptTemplate);
const afterLegacy = repositorySnapshot(input.repository);
assert.deepEqual(afterLegacy, before, 'repository changed during the legacy headless run');
const candidate = await runHeadless(
  input.repository,
  CANDIDATE,
  input.workflowArgs,
  promptTemplate,
);
const afterCandidate = repositorySnapshot(input.repository);
assert.deepEqual(afterCandidate, before, 'repository changed during the candidate headless run');
const comparison = compareHeadlessRuns(legacy, candidate);
assert.equal(
  comparison.normalizedBehaviorEqual,
  true,
  'legacy and candidate normalized behavior diverged',
);
assert.equal(
  comparison.observableTopologyEqual,
  true,
  'legacy and candidate Workflow topology diverged',
);
assert.equal(
  comparison.exactAgentContractsEqual,
  true,
  'legacy and candidate agent prompt/model/effort diverged',
);
assert.equal(comparison.normalizedResult.status, 'DONE');
assert.equal(comparison.normalizedResult.mode, 'verify');
assert.ok(
  comparison.normalizedResult.verdicts.every(
    (item) => item.evidencePresent && item.reasoningPresent,
  ),
);

const evidence = {
  format: 'wave-walker-headless-equivalence/1',
  recordedAt: new Date().toISOString(),
  repository: input.repository,
  repositoryHeadSha: before.headSha,
  repositoryFiles: before.files,
  repositorySnapshotScope: 'git-indexed-and-unignored',
  repositorySnapshotSha256: before.sha256,
  sameRepositorySnapshot: true,
  workflowArgsSha256: sha256Json(input.workflowArgs),
  runnerSha256: sha256(readFileSync(RUNNER)),
  librarySha256: sha256(readFileSync(LIBRARY)),
  inputFileSha256: sha256(readFileSync(options.input)),
  promptTemplateSha256: sha256(promptTemplate),
  workflowHash: manifest.workflowHash,
  claudeCliVersion: execFileSync('claude', ['--version'], { encoding: 'utf8' }).trim(),
  comparisonStandard:
    'exact agent prompt/model/effort; exact phase/agent/log topology; normalized grounded terminal semantics',
  exactAgentContractsEqual: comparison.exactAgentContractsEqual,
  observableTopologyEqual: comparison.observableTopologyEqual,
  normalizedBehaviorEqual: comparison.normalizedBehaviorEqual,
  exactFullResultEqual: comparison.exactFullResultEqual,
  agentContractsSha256: comparison.agentContractsSha256,
  observableTopologySha256: comparison.observableTopologySha256,
  normalizedBehaviorSha256: comparison.normalizedBehaviorSha256,
  normalizedResult: comparison.normalizedResult,
  legacy: runEvidence(legacy, sha256(legacySource)),
  candidate: runEvidence(candidate, sha256(candidateSource)),
};
writeEvidence(options.output, evidence);
process.stdout.write(JSON.stringify(evidence, null, 2) + '\n');
