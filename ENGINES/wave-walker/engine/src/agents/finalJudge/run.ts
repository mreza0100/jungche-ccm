// runFinalJudge — the one ruling over the whole walk (source lines 673-685).
import { retryAgent } from '../../runtime.js';
import { finalJudge } from './index.js';
import type { FinalJudgeArgs, FinalOut } from '../../types/index.js';

export function runFinalJudge(args: FinalJudgeArgs): Promise<FinalOut | null> {
  return retryAgent<FinalOut>(finalJudge.buildPrompt(args), {
    label: 'final-judge',
    phase: 'Fold',
    model: finalJudge.tier,
    effort: finalJudge.effort,
    schema: finalJudge.schema,
  });
}
