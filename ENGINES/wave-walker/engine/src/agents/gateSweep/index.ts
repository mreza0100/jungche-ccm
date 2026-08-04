// GATE SWEEP — one call per resolver file: a PURE EXTRACTOR emitting one gate card per GraphQL entry
// point in it (source lines 358-363, 469-478). Escalates to CONFIG.SENSOR_ESCALATE (see run.ts).
import { CONFIG } from '../../config.js';
import { buildGateSweep } from './prompts.js';
import type { Agent, Schema, GateSweepArgs } from '../../types/index.js';

export const GATE_SWEEP: Schema = {
  type: 'object',
  properties: {
    file: { type: 'string' },
    gates: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          kind: { type: 'string' },
          resource: { type: 'string' },
          anchor: { type: 'string' },
          idArgs: { type: 'array', items: { type: 'string' } },
          rolesAllowed: { type: 'array', items: { type: 'string' } },
          chain: { type: 'array', items: { type: 'string' } },
          orgFence: { type: 'boolean' },
          ownershipFence: { type: 'boolean' },
          notes: { type: 'string' },
        },
        required: ['id', 'anchor'],
      },
    },
  },
  required: ['file', 'gates'],
};

export const gateSweep: Agent<GateSweepArgs> = {
  tier: CONFIG.TIER.gateSweep,
  effort: CONFIG.EFFORT.gateSweep,
  schema: GATE_SWEEP,
  buildPrompt: buildGateSweep,
};
