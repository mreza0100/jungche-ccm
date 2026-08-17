// INVARIANT HUNTER — INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.2). One call per
// ARMED registry entry, territory-scoped (registry globs, NOT just the diff — this is what catches bucket
// C: pre-existing code no wave diff ever touches). Adversarial, REFUTE-FIRST: hunt for violations of the
// invariant's law, never confirm the happy path. Findings ride the existing judge path as rule class
// R9-INV (rules.ts computeInvariantAnomalies), never adjudicated by this seat itself.
import { CONFIG } from '../../config.js';
import { buildInvariantHunter } from './prompts.js';
import type { Agent, InvariantHunterArgs, Schema } from '../../types/index.js';

export const INVARIANT_HUNT: Schema = {
  type: 'object',
  properties: {
    invariantId: { type: 'string' },
    findings: {
      type: 'array',
      maxItems: 12,
      items: {
        type: 'object',
        properties: {
          what: { type: 'string' },
          location: { type: 'string' },
          expected: { type: 'string' },
          got: { type: 'string' },
          failureScenario: {
            type: 'string',
            description:
              'concrete: inputs/state -> wrong outcome -> who is harmed. REQUIRED, never a vibe.',
          },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] },
          fix: { type: 'string' },
        },
        required: ['what', 'location', 'expected', 'got', 'failureScenario', 'severity'],
      },
    },
    coverage: {
      type: 'string',
      description: 'what you walked AND what you skipped — an empty enumeration is never a verdict',
    },
  },
  required: ['invariantId', 'findings', 'coverage'],
};

export const invariantHunter: Agent<InvariantHunterArgs> = {
  tier: CONFIG.TIER.invariantHunter,
  effort: CONFIG.EFFORT.invariantHunter,
  schema: INVARIANT_HUNT,
  buildPrompt: buildInvariantHunter,
};
