// runInvariantHunter — one call per ARMED registry entry (INVARIANT REGISTRY FEATURE § 2.2), dispatched
// inside the existing Phase-1 barrier alongside threadWalker/sliceSensor/gateSweep/securityAuditor.
import { retryAgent } from '../../runtime.js';
import { invariantHunter } from './index.js';
import type { InvariantHunterArgs, InvariantHunterOut } from '../../types/index.js';

export function runInvariantHunter(args: InvariantHunterArgs): Promise<InvariantHunterOut | null> {
  return retryAgent<InvariantHunterOut>(invariantHunter.buildPrompt(args), {
    label: 'invariant-hunt · ' + args.invariant.id,
    phase: 'Walk',
    model: invariantHunter.tier,
    effort: invariantHunter.effort,
    schema: invariantHunter.schema,
  });
}
