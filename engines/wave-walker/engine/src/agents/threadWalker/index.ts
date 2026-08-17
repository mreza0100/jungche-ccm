// THREAD WALKER — one thread-walker call per scout thread: functional verdict + integration-delta
// hygiene (source lines 349-357, 441-449).
import { CONFIG } from '../../config.js';
import { buildThreadWalker } from './prompts.js';
import type { Agent, Schema, ThreadWalkerArgs } from '../../types/index.js';

export const WALK: Schema = {
  type: 'object',
  properties: {
    threadId: { type: 'string' },
    name: { type: 'string' },
    type: { type: 'string' },
    flow: { type: 'string', enum: ['INTACT', 'AT-RISK', 'BROKEN', 'N/A'] },
    trace: { type: 'string' },
    defects: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          what: { type: 'string' },
          location: { type: 'string' },
          jc: { type: 'string' },
          // INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.5) — optional, so the field
          // is additive: a defect with none named stays a valid defect.
          failureScenario: {
            type: 'string',
            description: 'the concrete input/state under which this step corrupts, aborts, or lies',
          },
        },
        required: ['what', 'location'],
      },
    },
    hygiene: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          kind: { type: 'string' },
          where: { type: 'string' },
          detail: { type: 'string' },
          jc: { type: 'string' },
        },
        required: ['kind', 'where', 'detail'],
      },
    },
    notes: { type: 'string' },
  },
  required: ['threadId', 'flow', 'trace', 'defects', 'hygiene'],
};

export const threadWalker: Agent<ThreadWalkerArgs> = {
  tier: CONFIG.TIER.threadWalker,
  effort: CONFIG.EFFORT.threadWalker,
  schema: WALK,
  buildPrompt: buildThreadWalker,
};
