// runClaimExtractor — the ONE claim-extractor call (source line 74-78). Pure request/response: the
// MAX_CLAIMS cap + fallback DONE-with-zero-claims short-circuit are owned by engine.ts.
import { retryAgent } from '../../runtime.js';
import { claimExtractor } from './index.js';
import type { ClaimExtractorArgs } from '../../types/index.js';
import type { ExtractOut } from '../../types/index.js';

export function runClaimExtractor(args: ClaimExtractorArgs): Promise<ExtractOut | null> {
  return retryAgent<ExtractOut>(claimExtractor.buildPrompt(args), {
    label: 'extract-claims',
    phase: 'Verify',
    model: claimExtractor.tier,
    effort: claimExtractor.effort,
    schema: claimExtractor.schema,
  });
}
