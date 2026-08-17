// consistencyJudge prompt — the source's inline construction (wave-walker.js lines 125-126) with the
// E2 payload diet: CONFIRMED claims travel as the consensus map ONLY; full verdict objects ride only
// for non-CONFIRMED claims (REFUTED/PARTIAL/UNPROVEN/NO-VERDICT). Wording per the proven reference
// variant; the ruling instructions are otherwise verbatim from the source.
import { RO } from '../shared.js';
import type { ConsistencyJudgeArgs } from '../../types/index.js';

export const buildConsistencyJudge = ({
  manifestPath,
  nonConfirmed,
  consensus,
  conflictChecks,
}: ConsistencyJudgeArgs): string =>
  'You are the MANIFEST CONSISTENCY JUDGE. Re-read the manifest at ' +
  manifestPath +
  ", then rule over the panel's evidence. Confirmed claims are listed in Consensus only (their full evidence is elsewhere); full verdict detail below covers only non-CONFIRMED claims. Consensus: " +
  JSON.stringify(consensus) +
  '\nNon-CONFIRMED verdicts (full detail): ' +
  JSON.stringify(nonConfirmed) +
  '\nConflict checks queued by the extractor: ' +
  JSON.stringify(conflictChecks) +
  '\nFind, evidence-based (open files where needed): (1) kind=conflict — cross-task collisions (File plans touching the same symbols incompatibly, contract shapes disagreeing between tasks, Depends order the file plan violates); (2) kind=refuted-premise — every task step resting on a REFUTED/PARTIAL claim, naming the manifest section it invalidates; (3) kind=freeloader — a task/step that does not earn its place (premise gone, work a sibling task duplicates, scope nothing consumes). Each: tasks, what (Expected/Got), evidence (manifest section + code anchor), severity, fix (concrete manifest correction).' +
  RO +
  ' Structured output: conflicts, summary.';
