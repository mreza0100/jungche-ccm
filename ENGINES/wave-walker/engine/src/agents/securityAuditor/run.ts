// runSecurityAuditor — one diff-scoped auditor call per changed-file slice (D2 fan-out), part of the
// Walk barrier. The label's ' · ' delimiter keeps every slice under the single 'security' seat tally.
import { retryAgent } from '../../runtime.js';
import { securityAuditor } from './index.js';
import type { SecurityAuditorArgs, SecurityOut } from '../../types/index.js';

export function runSecurityAuditor(args: SecurityAuditorArgs): Promise<SecurityOut | null> {
  return retryAgent<SecurityOut>(securityAuditor.buildPrompt(args), {
    label: 'security · 8A-8K s' + (args.clusterIndex + 1) + '/' + args.clusterCount,
    phase: 'Walk',
    model: securityAuditor.tier,
    effort: securityAuditor.effort,
    schema: securityAuditor.schema,
  });
}
