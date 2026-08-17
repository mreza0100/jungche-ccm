// brainer prompt — byte-identical to the source's inline construction (wave-walker.js lines 237-241).
import type { BrainerArgs } from '../../types/index.js';

export const buildBrainer = ({
  goal,
  scopeLine,
  wave,
  maxWaves,
  ledgerRows,
  openLeads,
  maxLanes,
}: BrainerArgs): string =>
  "You are the BRAINER — this investigation's only global reasoner. GOAL: " +
  goal +
  '.' +
  scopeLine +
  ' Wave ' +
  wave +
  '/' +
  maxWaves +
  '.\n' +
  'LEDGER (statuses are COMPUTED from topology — cite ids, never assert status): ' +
  JSON.stringify(ledgerRows) +
  '\n' +
  'OPEN LEADS: ' +
  JSON.stringify(openLeads) +
  '\n' +
  'Return your COORD: resultSoFar + keyClaimIds (the load-bearing ids — confidence is computed over exactly these); lanes ≤' +
  maxLanes +
  ' (pursue|attack; settled REQUIRES a survived challenge, so attack your own emerging answer — an attack lane names targets); dropLeads (dead leads); stop {done, reason} — done ONLY when the goal is answered on settled key claims or further probing cannot change the answer.' +
  ' Structured output: resultSoFar, keyClaimIds, lanes, dropLeads, stop.';
