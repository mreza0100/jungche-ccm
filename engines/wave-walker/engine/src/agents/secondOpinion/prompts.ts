// secondOpinion prompt — byte-identical to the source's inline construction (wave-walker.js lines 649-655)
// when `direction` is omitted (the ONLY caller before this feature, and still the default). INVARIANT
// REGISTRY FEATURE (§ 2.3, escalation symmetry) adds the 'confirmed' branch: a first judge CONFIRMED an
// R9-INV high/critical finding — re-examine for a wrongful confirm, symmetric to the existing wrongful-kill
// re-examination. `direction === 'confirmed'` is the only path that changes any byte of the output.
import { RO } from '../shared.js';
import type { SecondOpinionArgs } from '../../types/index.js';

export const buildSecondOpinion = ({ authRule, items, direction }: SecondOpinionArgs): string =>
  (direction === 'confirmed'
    ? 'You are the SECOND-OPINION judge (a first judge CONFIRMED these at high/critical severity from an adversarial invariant-hunter finding — this is the last gate before the founder sees it). '
    : "You are the SECOND-OPINION judge (a first judge killed these as FALSE, but the rule's evidence is regex/string-exact or a documented security invariant). ") +
  authRule +
  '\n' +
  'For each: open the file(s) yourself, re-derive from scratch, rule independently. ' +
  (direction === 'confirmed'
    ? 'Be suspicious of a confirm with no reproducible failure scenario, or where a guard/compensating control the finder missed already defeats it — override to FALSE when the confirm does not hold.\n'
    : 'Be suspicious of a kill that contradicts the verbatim extracted expression (a JSON.parse(JSON.stringify(...)) still present, a literal that truly never matches the produced set).\n') +
  (direction === 'confirmed'
    ? 'Confirmed verdicts with anomalies: '
    : 'Killed verdicts with anomalies: ') +
  JSON.stringify(items) +
  RO +
  ' Structured output: verdicts (one per anomalyId).';
