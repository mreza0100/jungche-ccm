// runSecondOpinion — one call per chunk-of-4 escalatable killed verdicts (source lines 649-655).
// `chunkIndex` is 0-based; the label suffix is `#(chunkIndex+1)` on EVERY chunk (unlike anomalyJudge,
// which omits the suffix on chunk 0) — matches the source's `'2nd-opinion#' + (i + 1)` exactly.
import { retryAgent } from '../../runtime.js';
import { secondOpinion } from './index.js';
import type { SecondOpinionArgs, JudgeOut } from '../../types/index.js';

export function runSecondOpinion(
  args: SecondOpinionArgs,
  chunkIndex: number,
): Promise<JudgeOut | null> {
  return retryAgent<JudgeOut>(secondOpinion.buildPrompt(args), {
    label: '2nd-opinion#' + (chunkIndex + 1),
    phase: 'Judge',
    model: secondOpinion.tier,
    effort: secondOpinion.effort,
    schema: secondOpinion.schema,
  });
}
