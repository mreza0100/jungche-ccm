// BRAINER — the investigation's only global reasoner: steers pursue/attack lanes over the computed
// claim ledger (source lines 163-172, 236-243). Frontier-judgment seat — default opus, warns loudly on
// any downgrade (config.ts's FRONTIER_SEATS).
import { CONFIG } from '../../config.js';
import { buildBrainer } from './prompts.js';
import type { Agent, BrainerArgs, Schema } from '../../types/index.js';

export const COORD: Schema = {
  type: 'object',
  properties: {
    resultSoFar: { type: 'string', description: 'best current answer, <=1200 chars' },
    keyClaimIds: {
      type: 'array',
      items: { type: 'string' },
      description: 'the LOAD-BEARING ledger ids the answer rests on',
    },
    lanes: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          kind: { type: 'string', description: 'pursue | attack' },
          question: { type: 'string' },
          files: { type: 'array', items: { type: 'string' } },
          targets: {
            type: 'array',
            items: { type: 'string' },
            description: 'attack only: claim ids to challenge',
          },
          note: { type: 'string' },
        },
        required: ['id', 'kind', 'question'],
      },
    },
    dropLeads: { type: 'array', items: { type: 'string' } },
    stop: {
      type: 'object',
      properties: { done: { type: 'boolean' }, reason: { type: 'string' } },
      required: ['done'],
    },
  },
  required: ['resultSoFar', 'keyClaimIds', 'lanes', 'stop'],
};

export const brainer: Agent<BrainerArgs> = {
  tier: CONFIG.TIER.brainer,
  effort: CONFIG.EFFORT.brainer,
  schema: COORD,
  buildPrompt: buildBrainer,
};
