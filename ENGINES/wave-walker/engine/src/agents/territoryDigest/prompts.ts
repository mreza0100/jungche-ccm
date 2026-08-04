// territoryDigest prompt — byte-identical to the source's inline construction (wave-walker.js lines 630-638). A non-empty charter appends the Professor-authored WALK CHARTER block (zero bytes otherwise).
import { RO } from '../shared.js';
import { CATCHBOOK } from '../../constants.js';
import type { TerritoryDigestArgs } from '../../types/index.js';

export const buildTerritoryDigest = ({ territory, slice, charter }: TerritoryDigestArgs): string =>
  'You are the ' +
  territory +
  " TERRITORY DIGEST for this wave. You receive this territory's side of every extracted card. " +
  'Mechanical rules AND the thread walk already handled connectivity/contract/gate/flow — do NOT re-report those. Your job is what neither can see: duplication across fields, wrong-layer logic, over-engineering, naming drift, magic literals, shallow error handling, hardcoded i18n. Open files as needed.\n' +
  CATCHBOOK +
  '\nCards: ' +
  JSON.stringify(slice) +
  RO +
  ' Structured output: territory, findings (each {lens, severity, what, location=file:line, fix}), summary (<=3 sentences).' +
  (charter
    ? '\nWALK CHARTER (caller-supplied duty): ' +
      charter +
      '\nHunt charter-relevant smells in your territory on top of the standard digest.'
    : '');
