// claimVerifier prompts — buildClaimVerifier is byte-identical to the source's inline construction
// (wave-walker.js lines 97-103) and carries the SOLO path (panel ≤ SOLO_THRESHOLD, byte-identical to
// the source's schedule). buildClaimVerifierBatch is the E2 batch prompt (panel > SOLO_THRESHOLD, ≤4
// file-clustered claims per call) — adapted from the proven reference variant's batchPrompt, minus its
// haiku tiering.
import { RO } from '../shared.js';
import type { ClaimIn, ClaimVerifierArgs } from '../../types/index.js';

export const buildClaimVerifier = ({ claim: c, question, repoRoot }: ClaimVerifierArgs): string =>
  'You are an INDEPENDENT VERIFIER on a pre-ruling claims panel' +
  (question ? ' grounding this ruling: ' + question : '') +
  '. Repo root: ' +
  repoRoot +
  '.\n' +
  'CLAIM ' +
  c.id +
  ': ' +
  c.statement +
  '\n' +
  (c.context ? 'Context: ' + c.context + '\n' : '') +
  ((c.files || []).length
    ? 'Start from these files (follow imports/greps wherever the evidence leads): ' +
      JSON.stringify(c.files) +
      '\n'
    : '') +
  'Actively try to REFUTE the claim — hunt for the counterexample before accepting confirmation. CONFIRMED only when file evidence proves it AS STATED; REFUTED when evidence contradicts it; PARTIAL when it holds with a material caveat (state it); UNPROVEN when evidence is unfindable. ' +
  'Every evidence anchor grep-verified file:line with a VERBATIM quote (<=120 chars). Judge evidence, not vibes.' +
  RO +
  ' Structured output: claimId=' +
  c.id +
  ', verdict, evidence, reasoning (<=3 sentences).';

export const buildClaimVerifierBatch = (
  batch: ClaimIn[],
  question: string,
  repoRoot: string,
): string =>
  'You are an INDEPENDENT VERIFIER on a pre-ruling claims panel' +
  (question ? ' grounding this ruling: ' + question : '') +
  '. Repo root: ' +
  repoRoot +
  '.\n' +
  'You are verifying a BATCH of ' +
  batch.length +
  ' independent claims — judge EACH one separately, refute-first (actively hunt for the counterexample before accepting confirmation), and return one verdict per claim.\n' +
  batch
    .map(
      (c) =>
        'CLAIM ' +
        c.id +
        ': ' +
        c.statement +
        (c.context ? ' | Context: ' + c.context : '') +
        ((c.files || []).length ? ' | Files: ' + JSON.stringify(c.files) : ''),
    )
    .join('\n') +
  '\n' +
  'CONFIRMED only when file evidence proves it AS STATED; REFUTED when evidence contradicts it; PARTIAL when it holds with a material caveat (state it); UNPROVEN when evidence is unfindable. ' +
  'Every evidence anchor grep-verified file:line with a VERBATIM quote (<=120 chars). Judge evidence, not vibes.' +
  RO +
  ' Structured output: verdicts — one item per claim above (claimId matching, verdict, evidence, reasoning <=3 sentences).';
