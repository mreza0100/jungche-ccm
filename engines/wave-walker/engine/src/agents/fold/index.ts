// FOLD — merges thread walks + confirmed anomalies + digests + the final judgment into the wave
// review, written into the report (source lines 376-382, 696-712).
import { CONFIG } from '../../config.js';
import { buildFold } from './prompts.js';
import type { Agent, Schema, FoldArgs } from '../../types/index.js';

export const FOLD: Schema = {
  type: 'object',
  properties: {
    verdict: { type: 'string', enum: ['SMOOTH SAILING', 'MOSTLY GOOD', 'ROUGH SEAS', 'SHIPWRECK'] },
    actionItems: { type: 'array', items: { type: 'string' } },
    review: { type: 'string' },
  },
  required: ['verdict', 'actionItems', 'review'],
};

export const fold: Agent<FoldArgs> = {
  tier: CONFIG.TIER.fold,
  effort: CONFIG.EFFORT.fold,
  schema: FOLD,
  buildPrompt: buildFold,
};
