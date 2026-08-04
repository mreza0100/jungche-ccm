// runCoverageCritic — the one call after Phase 1, before the final judge (INVARIANT REGISTRY FEATURE § 2.4).
import { retryAgent } from '../../runtime.js';
import { coverageCritic } from './index.js';
import type { CoverageCriticArgs, CoverageCriticOut } from '../../types/index.js';

export function runCoverageCritic(args: CoverageCriticArgs): Promise<CoverageCriticOut | null> {
  return retryAgent<CoverageCriticOut>(coverageCritic.buildPrompt(args), {
    label: 'coverage-critic',
    phase: 'Judge',
    model: coverageCritic.tier,
    effort: coverageCritic.effort,
    schema: coverageCritic.schema,
  });
}
