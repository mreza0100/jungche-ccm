// CLAIM VERIFIER — one independent read-only verifier per claim × vote on the pre-ruling claims panel
// (source lines 86-105).
import { CONFIG } from '../../config.js';
import { buildClaimVerifier } from './prompts.js';
import type { Agent, ClaimVerifierArgs, Schema } from '../../types/index.js';

export const VERIFY: Schema = {
  type: 'object',
  properties: {
    claimId: { type: 'string' },
    verdict: { type: 'string', enum: ['CONFIRMED', 'REFUTED', 'PARTIAL', 'UNPROVEN'] },
    evidence: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          anchor: { type: 'string' },
          quote: { type: 'string', description: 'VERBATIM, <=120 chars' },
        },
        required: ['anchor'],
      },
    },
    reasoning: { type: 'string' },
  },
  required: ['claimId', 'verdict', 'reasoning'],
};

// E2 batch schema — the VERIFY item shape wrapped in an array, one verdict per batched claim (used
// only when the panel exceeds CONFIG.SOLO_THRESHOLD; the solo path keeps VERIFY verbatim).
export const VERIFY_BATCH: Schema = {
  type: 'object',
  properties: { verdicts: { type: 'array', items: VERIFY } },
  required: ['verdicts'],
};

export const claimVerifier: Agent<ClaimVerifierArgs> = {
  tier: CONFIG.TIER.claimVerifier,
  effort: CONFIG.EFFORT.claimVerifier,
  schema: VERIFY,
  buildPrompt: buildClaimVerifier,
};
