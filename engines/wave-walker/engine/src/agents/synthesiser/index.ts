// SYNTHESISER (investigate mode) — writes the cited report from the hardened ledger, confidence floored
// by the computed value (source lines 258-266).
import { CONFIG } from '../../config.js';
import { buildSynthesiser } from './prompts.js';
import type { Agent, Schema, SynthesiserArgs } from '../../types/index.js';

// INTENTIONAL DIVERGENCE #4 (anti-degenerate guard): answer.minLength 80 — a live run returned {"answer":"test"} after writing a full report to reportOut; structured-output validation now forces a real answer. The report field stays unconstrained (legitimately vestigial when reportOut is used).
export const SYNTH: Schema = {
  type: 'object',
  properties: {
    answer: { type: 'string', minLength: 80 },
    confidence: { type: 'string', enum: ['low', 'medium', 'high'] },
    report: { type: 'string' },
  },
  required: ['answer', 'confidence', 'report'],
};

export const synthesiser: Agent<SynthesiserArgs> = {
  tier: CONFIG.TIER.synthesiser,
  effort: CONFIG.EFFORT.synthesiser,
  schema: SYNTH,
  buildPrompt: buildSynthesiser,
};
