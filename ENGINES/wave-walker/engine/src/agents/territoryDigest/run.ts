// runTerritoryDigest — one call per territory (source lines 626-638).
import { retryAgent } from '../../runtime.js';
import { territoryDigest } from './index.js';
import type { TerritoryDigestArgs, DigestOut } from '../../types/index.js';

export function runTerritoryDigest(args: TerritoryDigestArgs): Promise<DigestOut | null> {
  return retryAgent<DigestOut>(territoryDigest.buildPrompt(args), {
    label: 'digest · ' + args.territory,
    phase: 'Judge',
    model: territoryDigest.tier,
    effort: territoryDigest.effort,
    schema: territoryDigest.schema,
  });
}
