// runClaimVerifier — one SOLO verifier call for one (claim, vote) pair (source lines 94-105).
// `claim.opus` escalates the model to CONFIG.TIER.secondOpinion (source's OPUS_CLAIM_MODEL =
// args.securityEscalateModel — the SAME legacy knob secondOpinion reads; see config.ts's per-seat
// comment). This path runs whenever the panel is ≤ CONFIG.SOLO_THRESHOLD — byte-identical to the
// source's schedule.
//
// runClaimVerifierBatch — the E2 batch call (panel > SOLO_THRESHOLD): one verifier over ≤4
// file-clustered claims, returning {verdicts:[...]} (one VERIFY item per claim). The batch STAYS on
// the verifier tier (sonnet/xhigh) — never haiku. Per-claim `claim.opus` escalation cannot survive
// batching (one model per batch) — accepted E2 trade-off, engine.ts notes it in the log.
import { CONFIG } from '../../config.js';
import { retryAgent } from '../../runtime.js';
import { claimVerifier, VERIFY_BATCH } from './index.js';
import { buildClaimVerifierBatch } from './prompts.js';
import type { ClaimIn, VerifyOut } from '../../types/index.js';

export function runClaimVerifier(
  claim: ClaimIn,
  question: string,
  repoRoot: string,
  voteIndex: number,
  votes: number,
): Promise<VerifyOut | null> {
  return retryAgent<VerifyOut>(claimVerifier.buildPrompt({ claim, question, repoRoot }), {
    label: 'verify · ' + claim.id + (votes > 1 ? ' #' + (voteIndex + 1) : ''),
    phase: 'Verify',
    model: claim.opus ? CONFIG.TIER.secondOpinion : claimVerifier.tier,
    effort: claimVerifier.effort,
    schema: claimVerifier.schema,
  });
}

export function runClaimVerifierBatch(
  batch: ClaimIn[],
  question: string,
  repoRoot: string,
  batchIndex: number,
  voteIndex: number,
  votes: number,
): Promise<{ verdicts: VerifyOut[] } | null> {
  return retryAgent<{ verdicts: VerifyOut[] }>(buildClaimVerifierBatch(batch, question, repoRoot), {
    label: 'verify-batch · b' + (batchIndex + 1) + (votes > 1 ? ' #' + (voteIndex + 1) : ''),
    phase: 'Verify',
    model: claimVerifier.tier,
    effort: claimVerifier.effort,
    schema: VERIFY_BATCH,
  });
}
