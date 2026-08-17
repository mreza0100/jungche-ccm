// Promotion boundary for the required real active-pointer walk. Deterministic and verify-mode proofs
// are necessary but insufficient: candidate activation also requires one current full walk with an
// explicit terminal result. A failed/empty task can never be reinterpreted as promotion evidence.

export function assertProductionWalkEvidence(evidence, expected) {
  if (
    typeof evidence !== 'object' ||
    evidence === null ||
    evidence.format !== 'wave-walker-production-walk/1' ||
    evidence.status !== 'PASS' ||
    evidence.invocationCount !== 1 ||
    evidence.candidateSha256 !== expected.candidateSha256 ||
    evidence.workflowHash !== expected.workflowHash ||
    evidence.legacySha256 !== expected.legacySha256 ||
    evidence.permissionsBypassed !== false ||
    evidence.resultStatus !== 'DONE' ||
    typeof evidence.verdict !== 'string' ||
    evidence.agentCount <= 0 ||
    evidence.agentFailures < 0 ||
    evidence.repositoryUnchangedOutsideArtifacts !== true
  ) {
    throw new Error('production walk evidence is failed, stale, or incomplete');
  }
}
