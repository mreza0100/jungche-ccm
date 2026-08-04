// runThreadWalker — one call per thread (source lines 441-449). Pure request/response; the parallel
// fan-out across all threads + jobs + gate files + the security auditor is owned by engine.ts.
import { retryAgent } from '../../runtime.js';
import { threadWalker } from './index.js';
import type { ThreadWalkerArgs, WalkOut } from '../../types/index.js';

export function runThreadWalker(args: ThreadWalkerArgs): Promise<WalkOut | null> {
  const t = args.thread;
  return retryAgent<WalkOut>(threadWalker.buildPrompt(args), {
    label: 'walk · ' + (t.id || t.name || '?'),
    phase: 'Walk',
    model: threadWalker.tier,
    effort: threadWalker.effort,
    schema: threadWalker.schema,
  });
}
