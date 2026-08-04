// gateSweep prompt — ported from the source's inline construction (wave-walker.js lines 469-478); the
// resource-class enum and the org-fence field label now read off args.project (universal-bundle refactor)
// instead of a project's hardcoded vocabulary. Only ever dispatched when the gate machinery is ARMED (see
// engine.ts computeGateArming) — resourceClasses/fenceLabels are always populated by the caller.
import { RO } from '../shared.js';
import type { GateSweepArgs } from '../../types/index.js';

export const buildGateSweep = ({ file, resourceClasses, fenceLabels }: GateSweepArgs): string =>
  'You are a PURE EXTRACTOR (gate sweep). NO judgment. Open the resolver file ' +
  file +
  ' and extract ONE gate card per GraphQL entry point in it.\n' +
  'Per entry point: id ("Query.opName"/"Mutation.opName"), kind, resource class (' +
  resourceClasses.join('|') +
  '), anchor, idArgs (client-supplied ID args), ' +
  'chain (IN ORDER, every guard call between entry and first data access; open custom helpers and note what they fence), rolesAllowed (EXPAND role-set constants), ' +
  'orgFence (boolean: ' +
  fenceLabels.org +
  ' access enforced), ownershipFence (boolean: record-owner check enforced). Keep strings SHORT.' +
  RO +
  ' Structured output: file, gates.';
