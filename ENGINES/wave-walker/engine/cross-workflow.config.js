// Canonical cross-runtime contract for the authoritative Wave Walker program.
// The source hash is derived from the freshly assembled engine bundle, so an
// intentional source edit cannot leave a stale hand-maintained integrity pin.

import { createHash } from 'node:crypto';

import { defineHarnessProgram } from 'cross-workflow';

export function createWaveWalkerDefinition(source) {
  const sourceSha256 = createHash('sha256').update(source).digest('hex');
  return defineHarnessProgram({
    definitionKind: 'harness-program',
    irVersion: '1.0',
    programVersion: '1.0',
    id: 'wave-walker',
    version: '1.0.0',
    description:
      'The authoritative four-mode Professor Wave Walker compiled from one source for Claude Workflow and the Codex SDK.',
    inputSchema: { type: 'object' },
    outputSchema: {
      anyOf: [
        {
          type: 'object',
          properties: { status: { const: 'FAILED' } },
          required: ['status'],
        },
        {
          type: 'object',
          properties: {
            status: { const: 'DONE' },
            mode: { const: 'investigate' },
            claims: { type: 'array', minItems: 1 },
          },
          required: ['status', 'mode', 'claims'],
        },
        {
          type: 'object',
          properties: {
            status: { const: 'DONE' },
            mode: { enum: ['verify', 'manifest-verify'] },
          },
          required: ['status', 'mode'],
        },
        {
          type: 'object',
          properties: {
            status: { const: 'DONE' },
            verdict: { type: 'string' },
            actionItems: { type: 'array' },
            review: { type: 'string' },
            ledger: { type: 'object' },
          },
          required: ['status', 'verdict', 'actionItems', 'review', 'ledger'],
        },
      ],
    },
    models: {
      claude: {
        judgment: { model: 'opus', effort: 'high', agentType: 'general-purpose' },
        execution: { model: 'sonnet', effort: 'high', agentType: 'general-purpose' },
        collector: { model: 'haiku', effort: 'medium', agentType: 'general-purpose' },
      },
      codex: {
        judgment: {
          model: 'gpt-5.6-sol',
          effort: 'high',
          sandboxMode: 'read-only',
          approvalPolicy: 'never',
          networkAccess: false,
          webSearch: 'disabled',
        },
        execution: {
          model: 'gpt-5.6-terra',
          effort: 'high',
          sandboxMode: 'read-only',
          approvalPolicy: 'never',
          networkAccess: false,
          webSearch: 'disabled',
        },
        collector: {
          model: 'gpt-5.6-luna',
          effort: 'medium',
          sandboxMode: 'read-only',
          approvalPolicy: 'never',
          networkAccess: false,
          webSearch: 'disabled',
        },
      },
    },
    modelAliases: { haiku: 'collector', sonnet: 'execution', opus: 'judgment' },
    effortAliases: { low: 'low', medium: 'medium', high: 'high', xhigh: 'xhigh', max: 'xhigh' },
    requires: [
      'agent.repository-read',
      'agent.structured-output',
      'control.parallel',
      'events.progress',
      'usage.logical',
    ],
    source,
    sourceSha256,
    sourceProvenance: 'Wave Walker engine/src assembled by engine/build.js',
  });
}
