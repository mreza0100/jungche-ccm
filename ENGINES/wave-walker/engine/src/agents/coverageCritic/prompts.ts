// coverageCritic prompt — ported from the proven rollout-bug-hunt completeness critic (which produced
// 8 concrete, high-value coverage gaps), per tmp/wave-walker-investigation.md § 2.4. Not a source-fidelity
// port — there is no bundle equivalent.
import { RO } from '../shared.js';
import type { CoverageCriticArgs } from '../../types/index.js';

export const buildCoverageCritic = ({
  changedFiles,
  threadNames,
  hunterCoverage,
  armedIds,
  unarmedInvariants,
  unsensed,
}: CoverageCriticArgs): string =>
  "You are the COVERAGE CRITIC — this walk's EXTERNAL denominator. Every other seat reports its OWN coverage; your job is naming what the WALK ITSELF could not see, from outside it. Never re-litigate a finding — only name territory nothing inspected.\n" +
  'Changed files: ' +
  JSON.stringify(changedFiles) +
  '\n' +
  'Threads walked this pass: ' +
  JSON.stringify(threadNames) +
  '\n' +
  'Invariant hunters that ran, and their own stated coverage: ' +
  JSON.stringify(hunterCoverage) +
  '\n' +
  'Invariants ARMED this walk: ' +
  JSON.stringify(armedIds) +
  '. Registered but NOT armed (their territory, unhunted this walk): ' +
  JSON.stringify(unarmedInvariants) +
  '\n' +
  'Fields the sensor cap dropped (UNSENSED): ' +
  JSON.stringify(unsensed) +
  '\n' +
  'Name AT MOST 8 territories this walk could not see: files in the diff\'s blast radius appearing in no thread/hunter scope, a load-bearing named-skip buried inside a coverage line above, a cross-dimension interaction no single seat could see (two armed invariants whose territories overlap but whose hunters never compared notes), or an unarmed invariant whose territory the diff plausibly touches anyway. An empty enumeration is never a verdict — if you truly find nothing, say so explicitly and name what you actually checked (file list / directories) to reach that conclusion; do not render silence as "clean".' +
  RO +
  ' Structured output: gaps (each {territory, why}, at most 8), summary.';
