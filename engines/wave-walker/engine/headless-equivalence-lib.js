import { createHash } from 'node:crypto';
import { isDeepStrictEqual } from 'node:util';

function record(value, label) {
  if (typeof value !== 'object' || value === null || Array.isArray(value))
    throw new Error(label + ' must be an object');
  return value;
}

function sortedObject(value) {
  return Object.fromEntries(
    Object.entries(record(value, 'object')).sort(([left], [right]) => left.localeCompare(right)),
  );
}

function canonicalValue(value) {
  if (Array.isArray(value)) return value.map(canonicalValue);
  if (typeof value === 'object' && value !== null)
    return Object.fromEntries(
      Object.entries(value)
        .sort(([left], [right]) => left.localeCompare(right))
        .map(([key, child]) => [key, canonicalValue(child)]),
    );
  return value;
}

export function canonicalJson(value) {
  return JSON.stringify(canonicalValue(value));
}

export function sha256Json(value) {
  return createHash('sha256').update(canonicalJson(value)).digest('hex');
}

function messageBlocks(row) {
  return Array.isArray(row?.message?.content) ? row.message.content : [];
}

function taggedLine(line, tag) {
  const prefix = `<${tag}>`;
  const suffix = `</${tag}>`;
  if (!line.startsWith(prefix) || !line.endsWith(suffix))
    throw new Error(`Workflow notification has no structural ${tag} field`);
  return line.slice(prefix.length, -suffix.length);
}

export function extractWorkflowInvocation(transcript, expected) {
  if (typeof transcript !== 'string' || transcript.trim().length === 0)
    throw new Error('Claude transcript is empty');
  const rows = transcript
    .split('\n')
    .filter(Boolean)
    .map((line, index) => {
      try {
        return JSON.parse(line);
      } catch (caught) {
        throw new Error(`Claude transcript row ${index} is not JSON`, { cause: caught });
      }
    });
  // A tool_use whose result came back as an error LAUNCHED NOTHING — it is an attempt, not an invocation.
  // Counting attempts made a live model's ordinary self-correction (calling Workflow by name, being told
  // no such workflow exists, then calling it correctly) look like two production runs. What must be
  // unique is the run that actually STARTED, so errored attempts are excluded — and the survivor must
  // still be exactly one, which is the property this check exists to hold.
  const erroredToolUseIds = new Set(
    rows.flatMap((row) =>
      messageBlocks(row)
        .filter((block) => block?.type === 'tool_result' && block.is_error === true)
        .map((block) => block.tool_use_id),
    ),
  );
  const attempts = rows.flatMap((row) =>
    messageBlocks(row).filter((block) => block?.type === 'tool_use' && block.name === 'Workflow'),
  );
  const toolUses = attempts.filter((block) => !erroredToolUseIds.has(block.id));
  if (toolUses.length !== 1)
    throw new Error(
      'Claude transcript must contain exactly one launched Workflow tool use ' +
        `(${toolUses.length} launched, ${attempts.length - toolUses.length} errored attempt(s))`,
    );
  const toolUse = toolUses[0];
  let actualArgs;
  let expectedArgs;
  try {
    actualArgs = JSON.parse(toolUse.input?.args);
    expectedArgs = JSON.parse(expected.argsJson);
  } catch (caught) {
    throw new Error('Workflow tool use args are not valid JSON', { cause: caught });
  }
  if (
    typeof toolUse.id !== 'string' ||
    toolUse.input?.scriptPath !== expected.bundle ||
    canonicalJson(actualArgs) !== canonicalJson(expectedArgs)
  )
    throw new Error('Workflow tool use does not match the exact bundle and args contract');

  const toolResults = rows.flatMap((row) =>
    messageBlocks(row).filter(
      (block) => block?.type === 'tool_result' && block.tool_use_id === toolUse.id,
    ),
  );
  if (toolResults.length !== 1) throw new Error('Workflow tool use must have exactly one bound result');
  const toolResult = toolResults[0];
  if (toolResult.is_error === true || typeof toolResult.content !== 'string')
    throw new Error('Workflow tool result is missing or failed');
  const resultLines = toolResult.content.split('\n');
  const taskPrefix = 'Workflow launched in background. Task ID: ';
  if (!resultLines[0]?.startsWith(taskPrefix))
    throw new Error('Workflow tool result has no structural task ID');
  const taskId = resultLines[0].slice(taskPrefix.length);
  if (!/^[a-z0-9]+$/u.test(taskId)) throw new Error('Workflow task ID is malformed');
  const runLines = resultLines.filter((line) => /^Run ID: wf_[a-z0-9-]+$/u.test(line));
  if (runLines.length !== 1) throw new Error('Workflow tool result must expose exactly one run ID');
  const runId = runLines[0].slice('Run ID: '.length);
  const expectedTranscriptDirectory = `${expected.runRoot}/${runId}`;
  if (!resultLines.includes(`Transcript dir: ${expectedTranscriptDirectory}`))
    throw new Error('Workflow run ID is not bound to the expected transcript directory');

  const notifications = rows.filter(
    (row) => row?.origin?.kind === 'task-notification' && typeof row?.message?.content === 'string',
  );
  if (notifications.length !== 1)
    throw new Error('Claude transcript must contain exactly one structural task notification');
  const notificationLines = notifications[0].message.content.split('\n');
  if (notificationLines[0] !== '<task-notification>')
    throw new Error('Workflow task notification has an invalid structural prefix');
  if (taggedLine(notificationLines[1] ?? '', 'task-id') !== taskId)
    throw new Error('Workflow task notification does not match the launched task');
  if (taggedLine(notificationLines[2] ?? '', 'tool-use-id') !== toolUse.id)
    throw new Error('Workflow task notification does not match the Workflow tool use');
  const outputFile = taggedLine(notificationLines[3] ?? '', 'output-file');
  if (taggedLine(notificationLines[4] ?? '', 'status') !== 'completed')
    throw new Error('Workflow task did not complete successfully');
  const expectedOutputFile = `${expected.taskRoot}/${taskId}.output`;
  if (outputFile !== expectedOutputFile)
    throw new Error('Workflow task notification points outside its bound task output');
  return { runId, outputFile, taskId, toolUseId: toolUse.id };
}

export function normalizeVerifyResult(value) {
  const result = record(value, 'Workflow result');
  if (!Array.isArray(result.verdicts)) throw new Error('Workflow result verdicts must be an array');
  if (!Array.isArray(result.conflicts))
    throw new Error('Workflow result conflicts must be an array');
  const verdicts = result.verdicts
    .map((item, index) => {
      const verdict = record(item, `Workflow verdict ${index}`);
      return {
        claimId: verdict.claimId,
        verdict: verdict.verdict,
        evidencePresent: Array.isArray(verdict.evidence) && verdict.evidence.length > 0,
        reasoningPresent:
          typeof verdict.reasoning === 'string' && verdict.reasoning.trim().length > 0,
      };
    })
    .sort((left, right) => String(left.claimId).localeCompare(String(right.claimId)));
  return {
    status: result.status,
    mode: result.mode,
    claims: result.claims,
    votes: result.votes,
    verdicts,
    consensus: sortedObject(result.consensus),
    conflicts: result.conflicts.length,
    verifiersDied: result.verifiersDied,
    claimsMined: result.claimsMined,
    claimsVerified: result.claimsVerified,
    droppedClaimIds: result.droppedClaimIds,
    taskIds: result.taskIds,
  };
}

export function normalizeWorkflowTopology(value) {
  const output = record(value, 'Workflow output');
  if (!Array.isArray(output.logs)) throw new Error('Workflow output logs must be an array');
  if (!Array.isArray(output.workflowProgress))
    throw new Error('Workflow progress must be an array');
  return {
    summary: output.summary,
    logs: output.logs,
    agentCount: output.agentCount,
    progress: output.workflowProgress.map((item, index) => {
      const progress = record(item, `Workflow progress ${index}`);
      if (progress.type === 'workflow_phase')
        return { type: progress.type, index: progress.index, title: progress.title };
      if (progress.type === 'workflow_agent')
        return {
          type: progress.type,
          index: progress.index,
          label: progress.label,
          phaseIndex: progress.phaseIndex,
          phaseTitle: progress.phaseTitle,
          model: progress.model,
          state: progress.state,
          attempt: progress.attempt,
        };
      throw new Error(`unsupported Workflow progress type at index ${index}`);
    }),
  };
}

export function normalizeAgentContracts(value) {
  if (!Array.isArray(value)) throw new Error('agent contracts must be an array');
  return value
    .map((item, index) => {
      const contract = record(item, `agent contract ${index}`);
      if (typeof contract.prompt !== 'string' || contract.prompt.length === 0)
        throw new Error(`agent contract ${index} has no prompt`);
      return {
        label: contract.label,
        promptSha256: createHash('sha256').update(contract.prompt).digest('hex'),
        model: contract.model,
        effort: contract.effort,
      };
    })
    .sort((left, right) => String(left.label).localeCompare(String(right.label)));
}

export function compareHeadlessRuns(legacy, candidate) {
  const legacyResult = normalizeVerifyResult(record(legacy, 'legacy run').result);
  const candidateResult = normalizeVerifyResult(record(candidate, 'candidate run').result);
  const legacyTopology = normalizeWorkflowTopology(legacy.output);
  const candidateTopology = normalizeWorkflowTopology(candidate.output);
  const legacyContracts = normalizeAgentContracts(legacy.agentContracts);
  const candidateContracts = normalizeAgentContracts(candidate.agentContracts);
  return {
    exactFullResultEqual: isDeepStrictEqual(legacy.result, candidate.result),
    normalizedBehaviorEqual: isDeepStrictEqual(legacyResult, candidateResult),
    observableTopologyEqual: isDeepStrictEqual(legacyTopology, candidateTopology),
    exactAgentContractsEqual: isDeepStrictEqual(legacyContracts, candidateContracts),
    normalizedResult: legacyResult,
    normalizedBehaviorSha256: sha256Json(legacyResult),
    observableTopologySha256: sha256Json(legacyTopology),
    agentContractsSha256: sha256Json(legacyContracts),
  };
}

export function assertHeadlessEvidence(value, expected) {
  const evidence = record(value, 'headless equivalence evidence');
  const legacy = record(evidence.legacy, 'headless legacy evidence');
  const candidate = record(evidence.candidate, 'headless candidate evidence');
  const result = record(evidence.normalizedResult, 'headless normalized result');
  if (
    evidence.format !== 'wave-walker-headless-equivalence/1' ||
    evidence.workflowHash !== expected.workflowHash ||
    evidence.runnerSha256 !== expected.runnerSha256 ||
    evidence.librarySha256 !== expected.librarySha256 ||
    evidence.inputFileSha256 !== expected.inputFileSha256 ||
    evidence.promptTemplateSha256 !== expected.promptTemplateSha256 ||
    evidence.repositorySnapshotScope !== 'git-indexed-and-unignored' ||
    evidence.sameRepositorySnapshot !== true ||
    evidence.exactAgentContractsEqual !== true ||
    evidence.observableTopologyEqual !== true ||
    evidence.normalizedBehaviorEqual !== true ||
    legacy.bundleSha256 !== expected.legacySha256 ||
    candidate.bundleSha256 !== expected.candidateSha256 ||
    legacy.permissionsBypassed !== false ||
    candidate.permissionsBypassed !== false ||
    legacy.resultStatus !== 'DONE' ||
    candidate.resultStatus !== 'DONE' ||
    legacy.resultMode !== 'verify' ||
    candidate.resultMode !== 'verify' ||
    result.status !== 'DONE' ||
    result.mode !== 'verify' ||
    !Array.isArray(result.verdicts) ||
    result.verdicts.length === 0 ||
    result.verdicts.some((item) => item.evidencePresent !== true || item.reasoningPresent !== true)
  )
    throw new Error('headless Claude equivalence evidence is stale or incomplete');
  return evidence;
}

// ONE home for "is the deterministic equivalence evidence good enough to move the pointer". This lived
// as three inline copies (equivalence.js, verify.js, activate.js) that all demanded byte-identical agent
// calls — which meant a candidate could never carry a deliberate fix, because fixing a legacy defect
// necessarily changes the call sequence. The gate was not wrong to refuse divergence; it was wrong that
// it could not tell a DECLARED divergence from an undeclared one.
//
// So a divergence is admissible only if it was named IN ADVANCE, by agent label, in the evidence itself:
// the candidate may ADD the declared seats and change nothing else. Everything outside that declaration
// still fails, and a declaration that goes unused fails too — a standing allowance nobody exercises is
// how a gate quietly becomes decorative.
export function assertEquivalenceEvidence(value, expected) {
  const evidence = record(value, 'deterministic equivalence evidence');
  const declared = evidence.declaredAddedCalls ?? [];
  const observed = evidence.observedAddedCalls ?? [];
  const wellFormedDeclaration =
    Array.isArray(declared) &&
    Array.isArray(observed) &&
    declared.every((label) => typeof label === 'string' && label.length > 0) &&
    observed.every((label) => typeof label === 'string' && label.length > 0);
  const callsAdmissible = wellFormedDeclaration
    ? evidence.exactCallsEqual === true
      ? declared.length === 0 && observed.length === 0
      : declared.length > 0 &&
        observed.length > 0 &&
        observed.every((label) => declared.includes(label)) &&
        declared.every((label) => observed.includes(label)) &&
        evidence.callsEqualIgnoringDeclaredAdditions === true
    : false;
  if (
    evidence.format !== 'wave-walker-equivalence/1' ||
    callsAdmissible !== true ||
    evidence.exactResultEqual !== true ||
    evidence.verdictShapeEqual !== true ||
    evidence.legacySha256 !== expected.legacySha256 ||
    evidence.candidateSha256 !== expected.candidateSha256
  )
    throw new Error('candidate equivalence evidence is stale or incomplete');
  return evidence;
}
