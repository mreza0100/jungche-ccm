// runSynthesiser — the ONE closing synthesiser call (source lines 259-265).
import { retryAgent } from '../../runtime.js';
import { synthesiser } from './index.js';
import type { SynthesiserArgs, SynthOut } from '../../types/index.js';

export function runSynthesiser(args: SynthesiserArgs): Promise<SynthOut | null> {
  return retryAgent<SynthOut>(synthesiser.buildPrompt(args), {
    label: 'synth',
    phase: 'Investigate',
    model: synthesiser.tier,
    effort: synthesiser.effort,
    schema: synthesiser.schema,
  });
}
