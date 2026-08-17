// runClaimAuditor — the ONE audit call for a wave's pending claims (source lines 212-213). Pure
// request/response: filtering to `audit === 'pending'` rows and writing verdicts back onto the ledger
// are owned by engine.ts.
import { retryAgent } from '../../runtime.js';
import { claimAuditor } from './index.js';
import type { AuditsOut, ClaimAuditorArgs } from '../../types/index.js';

export function runClaimAuditor(args: ClaimAuditorArgs): Promise<AuditsOut | null> {
  return retryAgent<AuditsOut>(claimAuditor.buildPrompt(args), {
    label: 'audit · w' + args.wave,
    phase: 'Investigate',
    model: claimAuditor.tier,
    effort: claimAuditor.effort,
    schema: claimAuditor.schema,
  });
}
