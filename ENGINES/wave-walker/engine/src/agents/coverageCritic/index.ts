// COVERAGE CRITIC — INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.4). The walk's
// EXTERNAL denominator: one Sonnet call after Phase 1, before the final judge, whose only job is naming
// what the walk itself did NOT inspect — territories armed but unhunted, dims skipped, thread coverage
// vs the diff's blast radius. Never re-litigates findings. Dispatched only when the registry is non-empty
// (CONFIG.INVARIANTS.length > 0) — an absent/empty registry means no critic runs at all (the floor).
import { CONFIG } from '../../config.js';
import { buildCoverageCritic } from './prompts.js';
import type { Agent, CoverageCriticArgs, Schema } from '../../types/index.js';

export const COVERAGE_CRITIC: Schema = {
  type: 'object',
  properties: {
    gaps: {
      type: 'array',
      maxItems: 8,
      items: {
        type: 'object',
        properties: {
          territory: { type: 'string' },
          why: { type: 'string' },
        },
        required: ['territory', 'why'],
      },
    },
    summary: { type: 'string' },
  },
  required: ['gaps', 'summary'],
};

export const coverageCritic: Agent<CoverageCriticArgs> = {
  tier: CONFIG.TIER.coverageCritic,
  effort: CONFIG.EFFORT.coverageCritic,
  schema: COVERAGE_CRITIC,
  buildPrompt: buildCoverageCritic,
};
