// runBrainer — the ONE brainer call for one wave (source lines 236-243).
import { retryAgent } from '../../runtime.js';
import { brainer } from './index.js';
import type { BrainerArgs, CoordOut } from '../../types/index.js';

export function runBrainer(args: BrainerArgs): Promise<CoordOut | null> {
  return retryAgent<CoordOut>(brainer.buildPrompt(args), {
    label: 'brainer · w' + args.wave,
    phase: 'Investigate',
    model: brainer.tier,
    effort: brainer.effort,
    schema: brainer.schema,
  });
}
