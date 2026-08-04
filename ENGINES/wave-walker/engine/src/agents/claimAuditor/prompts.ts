// claimAuditor prompt — byte-identical to the source's inline construction (wave-walker.js line 212).
import { RO } from '../shared.js';
import type { ClaimAuditorArgs } from '../../types/index.js';

export const buildClaimAuditor = ({ rows, repoRoot }: ClaimAuditorArgs): string =>
  'You are a CLAIM AUDITOR — you are grepping for a pin, not judging truth. For EACH claim id, open/grep the cited anchor file(s) (repo root ' +
  repoRoot +
  ') and verify the VERBATIM quote appears (whitespace-insensitive; within ±5 lines of a cited line number is fine). pass = every quote found; fail = any quote absent. Claims: ' +
  JSON.stringify(rows) +
  RO +
  ' Structured output: audits (one per claim id).';
