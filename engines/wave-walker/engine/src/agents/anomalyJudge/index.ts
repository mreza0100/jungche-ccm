// ANOMALY JUDGE — one call per chunk-of-6 same-rule ledger anomalies (source lines 364-368, 608-620).
// Its JUDGE schema is reused verbatim (same object reference) by the secondOpinion seat.
import { CONFIG } from '../../config.js';
import { buildAnomalyJudge } from './prompts.js';
import type { Agent, Schema, AnomalyJudgeArgs } from '../../types/index.js';

export const JUDGE: Schema = {
  type: 'object',
  properties: {
    verdicts: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          anomalyId: { type: 'string' },
          verdict: { type: 'string', enum: ['CONFIRMED', 'FALSE', 'UNPROVEN'] },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] },
          what: { type: 'string' },
          location: { type: 'string' },
          fix: { type: 'string' },
          why: { type: 'string' },
        },
        required: ['anomalyId', 'verdict', 'severity', 'what'],
      },
    },
  },
  required: ['verdicts'],
};

export const anomalyJudge: Agent<AnomalyJudgeArgs> = {
  tier: CONFIG.TIER.anomalyJudge,
  effort: CONFIG.EFFORT.anomalyJudge,
  schema: JUDGE,
  buildPrompt: buildAnomalyJudge,
};
