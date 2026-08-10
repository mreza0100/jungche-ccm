// CLOCK PROBE — the walk's wall-clock instrument. The Workflow runtime forbids in-script clock reads,
// so the graph cannot measure its own elapsed time and cannot reserve a slice of the runtime window for
// terminal judgment. A seat with a shell can: this collector runs `date +%s` and returns the integer.
// Two calls per walk (start + one checkpoint) fund the JUDGE RESERVE — see engine.ts § TIME CHECKPOINT.
// It is an OPTIMISATION, never a gate: a dead or nonsense probe sheds nothing and the walk proceeds
// exactly as it would without it, with the unmeasured window recorded as a named coverage gap.
import { CONFIG } from '../../config.js';
import { buildClockProbe } from './prompts.js';
import type { Agent, ClockProbeArgs, Schema } from '../../types/index.js';

export const CLOCK_PROBE: Schema = {
  type: 'object',
  properties: {
    epochSeconds: { type: 'number' },
  },
  required: ['epochSeconds'],
};

export const clockProbe: Agent<ClockProbeArgs> = {
  tier: CONFIG.TIER.clockProbe,
  effort: CONFIG.EFFORT.clockProbe,
  schema: CLOCK_PROBE,
  buildPrompt: buildClockProbe,
};
