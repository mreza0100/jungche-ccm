// securityAuditor prompt — D2 SECURITY FAN-OUT (audit 2026-07-28): one auditor per changed-file slice,
// replacing the single whole-diff auditor that opened ~45 of 148 files and reported a clean sweep. Each
// slice is swept exhaustively; the full changed set rides along as cross-file context; filesOpened/
// filesSkipped make the sweep's denominator reportable. Carries the D4 producer-verification clause.
import { RO } from '../shared.js';
import type { SecurityAuditorArgs } from '../../types/index.js';

// Universal category weighting — the 8A-8K taxonomy itself names these as the deepest-pass categories;
// carries no domain claim, so this is exactly what ships when args.project supplies no
// securityStakesLine (the universal-bundle refactor's last hardcoded domain string: this line used to
// open with the seeding project's own sacred-data framing sentence, steering every linked project's
// auditor at another domain's sensitive-data class).
const SECURITY_CATEGORY_EMPHASIS =
  'Sensitive data (8F), auth (8C), API surface (8D), LLM/prompt (8E) get the deepest pass.';

export const buildSecurityAuditor = ({
  securityDoc,
  clusterFiles,
  allChangedFiles,
  clusterIndex,
  clusterCount,
  branch,
  mergeShas,
  securityStakesLine,
}: SecurityAuditorArgs): string =>
  'You are a WAVE SECURITY AUDITOR (slice ' +
  (clusterIndex + 1) +
  '/' +
  clusterCount +
  '). Read ' +
  securityDoc +
  " and apply its FULL category set (8A–8K + Method & Severity) to YOUR SLICE of this wave's diff. Your assigned files — OPEN AND SWEEP EVERY ONE; a file you did not open goes in filesSkipped with why, never silently: " +
  JSON.stringify(clusterFiles) +
  ". The wave's full changed set (context — follow a changed symbol from your slice into its callers/config/emitters wherever the risk crosses a file boundary, inside or outside the slice): " +
  JSON.stringify(allChangedFiles) +
  '. ' +
  (branch
    ? 'Diff: main...' + branch + '.'
    : 'Merge SHAs: ' + JSON.stringify(mergeShas || []) + '.') +
  ' ' +
  (securityStakesLine ? securityStakesLine + ' — ' : '') +
  SECURITY_CATEGORY_EMPHASIS +
  " A guard or fix claiming to handle a response/error/message SHAPE is verified against the code that EMITS that shape — open the producer; a test's fabricated envelope is never evidence. Report ONLY defects the diff introduced or worsened; a pre-existing issue you trip over goes into summary as one line (category + location), never a finding. filesOpened lists every file you ACTUALLY read; categoriesSwept names every category you ACTUALLY swept — honesty over completeness." +
  RO +
  ' Structured output: findings (Expected/Got), categoriesSwept, filesOpened, filesSkipped, summary.';
