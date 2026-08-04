// CLAIM EXTRACTOR — mines a wave manifest's load-bearing claims for the manifest-verify panel
// (source lines 68-77).
import { CONFIG } from '../../config.js';
import { buildClaimExtractor } from './prompts.js';
import type { Agent, ClaimExtractorArgs, Schema } from '../../types/index.js';

export const EXTRACT: Schema = {
  type: 'object',
  properties: {
    claims: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          taskId: { type: 'string' },
          kind: { type: 'string', description: 'existence | behavior | contract | dep' },
          statement: { type: 'string', description: 'self-contained, refutable, <=200 chars' },
          files: { type: 'array', items: { type: 'string' } },
          context: { type: 'string' },
        },
        required: ['id', 'statement'],
      },
    },
    conflictChecks: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          tasks: { type: 'array', items: { type: 'string' } },
          what: { type: 'string' },
        },
        required: ['id', 'what'],
      },
    },
  },
  required: ['claims', 'conflictChecks'],
};

export const claimExtractor: Agent<ClaimExtractorArgs> = {
  tier: CONFIG.TIER.claimExtractor,
  effort: CONFIG.EFFORT.claimExtractor,
  schema: EXTRACT,
  buildPrompt: buildClaimExtractor,
};
