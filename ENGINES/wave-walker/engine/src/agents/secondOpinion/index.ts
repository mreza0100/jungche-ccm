// SECOND OPINION — Opus re-examines killed security/near-certain verdicts (source lines 646-658).
// FRONTIER-JUDGMENT SEAT — never silently downgrade below opus (root CLAUDE.md § Model Selection).
// Reuses anomalyJudge's JUDGE schema BY REFERENCE (source calls the same JS `JUDGE` constant for both).
import { CONFIG } from '../../config.js';
import { buildSecondOpinion } from './prompts.js';
import { JUDGE } from '../anomalyJudge/index.js';
import type { Agent, SecondOpinionArgs } from '../../types/index.js';

export const secondOpinion: Agent<SecondOpinionArgs> = {
  tier: CONFIG.TIER.secondOpinion,
  effort: CONFIG.EFFORT.secondOpinion,
  schema: JUDGE,
  buildPrompt: buildSecondOpinion,
};
