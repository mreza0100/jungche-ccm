// runAnomalyJudge — one call per chunk-of-6 same-rule anomalies (source lines 607-620). `chunkIndex` is
// the chunk's position within its rule group — label carries a `#N` suffix from the SECOND chunk on,
// exactly like the source's `job.i ? '#' + (job.i + 1) : ''`.
import { retryAgent } from '../../runtime.js';
import { anomalyJudge } from './index.js';
import type { AnomalyJudgeArgs, JudgeOut } from '../../types/index.js';

export function runAnomalyJudge(
  args: AnomalyJudgeArgs,
  chunkIndex: number,
): Promise<JudgeOut | null> {
  return retryAgent<JudgeOut>(anomalyJudge.buildPrompt(args), {
    label: 'judge · ' + args.rule + (chunkIndex ? '#' + (chunkIndex + 1) : ''),
    phase: 'Judge',
    model: anomalyJudge.tier,
    effort: anomalyJudge.effort,
    schema: anomalyJudge.schema,
  });
}
