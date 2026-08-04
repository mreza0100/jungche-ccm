// probe prompt — byte-identical to the source's inline construction (wave-walker.js lines 219-223).
import { RO } from '../shared.js';
import type { ProbeArgs } from '../../types/index.js';

export const buildProbe = ({ lane, goal, scopeLine }: ProbeArgs): string =>
  'You are lane ' +
  lane.id +
  ' (' +
  (lane.kind || 'pursue') +
  ') of a code investigation. GOAL: ' +
  goal +
  '.' +
  scopeLine +
  '\nQUESTION: ' +
  lane.question +
  (lane.note ? ' — steering: ' + lane.note : '') +
  '\n' +
  ((lane.files || []).length
    ? 'Start files (follow imports/greps wherever evidence leads): ' +
      JSON.stringify(lane.files) +
      '\n'
    : '') +
  (lane.kind === 'attack'
    ? 'ATTACK LANE: actively hunt COUNTER-evidence against claim ids ' +
      JSON.stringify(lane.targets || []) +
      ' (emit kind=counter with targets). A real hunt that finds NOTHING → nothingFound:true — that survival is first-class evidence, not silence.\n'
    : '') +
  'Return quote-pinned claims — SELF-CONTAINED facts, VERBATIM quotes (<=120 chars), grep-verified file:line anchors — plus leads (files/symbols worth a future lane).' +
  RO +
  ' Structured output: laneId=' +
  lane.id +
  ', claims, leads, nothingFound.';
