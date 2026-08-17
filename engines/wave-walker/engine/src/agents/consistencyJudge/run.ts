// runConsistencyJudge — the ONE consistency-judge call (source lines 124-128).
import { retryAgent } from '../../runtime.js';
import { consistencyJudge } from './index.js';
import type { ConflictOut, ConsistencyJudgeArgs } from '../../types/index.js';

export function runConsistencyJudge(args: ConsistencyJudgeArgs): Promise<ConflictOut | null> {
  return retryAgent<ConflictOut>(consistencyJudge.buildPrompt(args), {
    label: 'conflict-judge',
    phase: 'Verify',
    model: consistencyJudge.tier,
    effort: consistencyJudge.effort,
    schema: consistencyJudge.schema,
  });
}
