// TERRITORY DIGEST — one call per territory (BE/FE/AI), catch-book smells the mechanical rules and
// thread walk can't see (source lines 369-375, 630-638).
import { CONFIG } from '../../config.js';
import { buildTerritoryDigest } from './prompts.js';
import type { Agent, Schema, TerritoryDigestArgs } from '../../types/index.js';

export const DIGEST: Schema = {
  type: 'object',
  properties: {
    territory: { type: 'string' },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          lens: { type: 'string' },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high'] },
          what: { type: 'string' },
          location: { type: 'string' },
          fix: { type: 'string' },
        },
        required: ['lens', 'severity', 'what', 'location'],
      },
    },
    summary: { type: 'string' },
  },
  required: ['territory', 'findings', 'summary'],
};

export const territoryDigest: Agent<TerritoryDigestArgs> = {
  tier: CONFIG.TIER.territoryDigest,
  effort: CONFIG.EFFORT.territoryDigest,
  schema: DIGEST,
  buildPrompt: buildTerritoryDigest,
};
