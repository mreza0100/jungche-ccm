// synthesiser prompt — byte-identical to the source's inline construction (wave-walker.js lines 259-264). Divergence #4: the trailing anti-degenerate answer line (Professor-authored).
import type { SynthesiserArgs } from '../../types/index.js';

export const buildSynthesiser = ({
  goal,
  stopReason,
  keyIds,
  conf,
  reportOut,
  resultSoFarText,
  claimsOut,
  openLeads,
}: SynthesiserArgs): string =>
  'You are the SYNTHESISER of a code investigation. GOAL: ' +
  goal +
  '. Write the report from the hardened ledger — sections: Answer · Evidence (claims by status, cite ids inline [cN] with anchors) · Counter-evidence & survived challenges · Open leads · Coverage (stopReason: ' +
  stopReason +
  '). ' +
  'COMPUTED confidence over key claims ' +
  JSON.stringify(keyIds) +
  ' = ' +
  conf +
  ' — your stated confidence may be LOWER, never higher. ' +
  (reportOut
    ? 'WRITE the full report to ' + reportOut + ' (your ONLY file write; run no git). '
    : 'Write no files. ') +
  'resultSoFar: ' +
  resultSoFarText +
  '\nLEDGER: ' +
  JSON.stringify(claimsOut) +
  '\nOPEN LEADS: ' +
  JSON.stringify(openLeads) +
  ' Structured output: answer, confidence, report (full markdown).' +
  "\nThe answer field carries the COMPLETE answer text itself — never a placeholder, never a pointer to the report file; it is the caller's primary deliverable.";
