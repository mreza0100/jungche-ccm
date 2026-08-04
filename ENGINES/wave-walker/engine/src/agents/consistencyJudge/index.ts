// CONSISTENCY JUDGE — the manifest consistency judge: cross-task conflicts, refuted premises,
// freeloader tasks (source lines 118-130).
import { CONFIG } from '../../config.js';
import { buildConsistencyJudge } from './prompts.js';
import type { Agent, ConsistencyJudgeArgs, Schema } from '../../types/index.js';

export const CONFLICT: Schema = {
  type: 'object',
  properties: {
    conflicts: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          kind: { type: 'string', description: 'conflict | refuted-premise | freeloader' },
          tasks: { type: 'array', items: { type: 'string' } },
          what: { type: 'string' },
          evidence: { type: 'string' },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] },
          fix: { type: 'string', description: 'concrete manifest correction' },
        },
        required: ['id', 'kind', 'what', 'severity'],
      },
    },
    summary: { type: 'string' },
  },
  required: ['conflicts', 'summary'],
};

export const consistencyJudge: Agent<ConsistencyJudgeArgs> = {
  tier: CONFIG.TIER.consistencyJudge,
  effort: CONFIG.EFFORT.consistencyJudge,
  schema: CONFLICT,
  buildPrompt: buildConsistencyJudge,
};
