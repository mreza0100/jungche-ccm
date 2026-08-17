// PROBE — one investigation lane (pursue or attack) over the quote-pinned claim ledger
// (source lines 155-162, 217-225).
import { CONFIG } from '../../config.js';
import { buildProbe } from './prompts.js';
import type { Agent, ProbeArgs, Schema } from '../../types/index.js';

export const PROBE: Schema = {
  type: 'object',
  properties: {
    laneId: { type: 'string' },
    claims: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          statement: { type: 'string', description: 'self-contained fact, <=200 chars' },
          kind: { type: 'string', description: 'support | counter' },
          targets: {
            type: 'array',
            items: { type: 'string' },
            description: 'counter only: attacked claim ids',
          },
          anchors: {
            type: 'array',
            items: {
              type: 'object',
              properties: {
                anchor: { type: 'string' },
                quote: { type: 'string', description: 'VERBATIM, <=120 chars' },
              },
              required: ['anchor', 'quote'],
            },
          },
        },
        required: ['statement', 'anchors'],
      },
    },
    leads: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          what: { type: 'string' },
          files: { type: 'array', items: { type: 'string' } },
        },
        required: ['what'],
      },
    },
    nothingFound: { type: 'boolean' },
  },
  required: ['laneId', 'claims', 'leads'],
};

export const probe: Agent<ProbeArgs> = {
  tier: CONFIG.TIER.probe,
  effort: CONFIG.EFFORT.probe,
  schema: PROBE,
  buildPrompt: buildProbe,
};
