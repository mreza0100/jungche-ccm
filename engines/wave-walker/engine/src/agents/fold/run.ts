// runFold — the one call merging everything into the report (source lines 700-712).
import { retryAgent } from '../../runtime.js';
import { fold } from './index.js';
import type { FoldArgs, FoldOut } from '../../types/index.js';

export function runFold(args: FoldArgs): Promise<FoldOut | null> {
  return retryAgent<FoldOut>(fold.buildPrompt(args), {
    label: 'fold',
    phase: 'Fold',
    model: fold.tier,
    effort: fold.effort,
    schema: fold.schema,
  });
}
