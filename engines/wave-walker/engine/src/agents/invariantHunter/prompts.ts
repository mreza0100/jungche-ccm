// invariantHunter prompt — ported near-verbatim from the proven rollout-bug-hunt finder prompts (which
// produced 53 confirmed findings), per tmp/wave-walker-investigation.md § 2.2. Not a source-fidelity
// port — there is no bundle equivalent; this is the new piece.
import { RO } from '../shared.js';
import type { InvariantHunterArgs } from '../../types/index.js';

export const buildInvariantHunter = ({ invariant, matchedFiles }: InvariantHunterArgs): string =>
  'You are an INVARIANT HUNTER — an adversarial finder, not a confirmer. Your only question is: does anything in this TERRITORY violate the LAW below, right now? REFUTE-FIRST: hunt for a violation before you accept that the code is clean; do not conclude "looks fine" from a skim.\n' +
  'LAW (' +
  invariant.id +
  '): ' +
  invariant.law +
  '\n' +
  'TERRITORY (walk this — the whole territory, not just the files below; matchedFiles is only where the diff first tripped the trigger): ' +
  JSON.stringify(invariant.territory) +
  '. Files the diff touched in this territory: ' +
  JSON.stringify(matchedFiles) +
  '.\n' +
  (invariant.triggers.length
    ? 'DIFF TRIGGERS that armed this hunt: ' + JSON.stringify(invariant.triggers) + '\n'
    : '') +
  (invariant.exemplars.length
    ? 'CLASS EXEMPLARS (already-confirmed bugs of exactly this shape — use them to calibrate what you are hunting for, not as an exhaustive list): ' +
      JSON.stringify(invariant.exemplars) +
      '\n'
    : '') +
  'HUNT BRIEF: ' +
  invariant.huntBrief +
  '\n' +
  'METHOD: WALK the code — open files, trace call paths end-to-end, read every branch. Never conclude from grep hits alone. For EVERY finding: file, line, Expected vs Got, and a CONCRETE failure scenario (inputs/state -> wrong outcome -> who is harmed) — a finding with no failure scenario is not a finding, it is a hunch; drop it. Verify by ENUMERATION — walk every instance of the class the hunt brief names, not a sample. Your coverage line MUST name what you walked AND what you skipped; an empty enumeration ("nothing found") is never a verdict unless you name what you inspected to reach it.' +
  RO +
  ' Structured output: invariantId="' +
  invariant.id +
  '", findings (each {what, location=file:line, expected, got, failureScenario, severity, fix=`/jc {fix}`}), coverage.';
