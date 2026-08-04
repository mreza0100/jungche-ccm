// CLAIM AUDITOR (investigate mode) — mechanically greps every quote-pin the ledger carries; a bounded,
// per-wave batch job over pending claims (source lines 172, 209-216).
import { CONFIG } from '../../config.js';
import { buildClaimAuditor } from './prompts.js';
import type { Agent, ClaimAuditorArgs, Schema } from '../../types/index.js';

export const AUDITS: Schema = {
  type: 'object',
  properties: {
    audits: {
      type: 'array',
      items: {
        type: 'object',
        properties: { id: { type: 'string' }, result: { type: 'string', enum: ['pass', 'fail'] } },
        required: ['id', 'result'],
      },
    },
  },
  required: ['audits'],
};

export const claimAuditor: Agent<ClaimAuditorArgs> = {
  tier: CONFIG.TIER.claimAuditor,
  effort: CONFIG.EFFORT.claimAuditor,
  schema: AUDITS,
  buildPrompt: buildClaimAuditor,
};
