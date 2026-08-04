// FINAL JUDGE — one Opus ruling over the WHOLE walk before the review is written (source lines
// 664-685). FRONTIER-JUDGMENT SEAT — never silently downgrade below opus.
import { CONFIG } from '../../config.js';
import { buildFinalJudge } from './prompts.js';
import type { Agent, Schema, FinalJudgeArgs } from '../../types/index.js';

export const FINAL: Schema = {
  type: 'object',
  properties: {
    verdict: { type: 'string', enum: ['SMOOTH SAILING', 'MOSTLY GOOD', 'ROUGH SEAS', 'SHIPWRECK'] },
    reinstated: {
      type: 'array',
      items: {
        type: 'object',
        properties: { anomalyId: { type: 'string' }, why: { type: 'string' } },
        required: ['anomalyId', 'why'],
      },
    },
    missedRisks: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          what: { type: 'string' },
          where: { type: 'string' },
          severity: { type: 'string' },
          jc: { type: 'string' },
        },
        required: ['what', 'where'],
      },
    },
    rationale: { type: 'string' },
  },
  required: ['verdict', 'missedRisks'],
};

export const finalJudge: Agent<FinalJudgeArgs> = {
  tier: CONFIG.TIER.finalJudge,
  effort: CONFIG.EFFORT.finalJudge,
  schema: FINAL,
  buildPrompt: buildFinalJudge,
};
