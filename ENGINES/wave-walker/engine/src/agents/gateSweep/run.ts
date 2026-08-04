// runGateSweep — one call per gate file (source lines 469-478). Escalates to CONFIG.SENSOR_ESCALATE
// on a dead respawn, same as sliceSensor.
import { retryAgent } from '../../runtime.js';
import { gateSweep } from './index.js';
import { CONFIG } from '../../config.js';
import type { GateSweepArgs, GateSweepOut } from '../../types/index.js';

export function runGateSweep(args: GateSweepArgs): Promise<GateSweepOut | null> {
  return retryAgent<GateSweepOut>(
    gateSweep.buildPrompt(args),
    {
      label: 'gates · ' + (args.file.split('/').pop() || args.file),
      phase: 'Walk',
      model: gateSweep.tier,
      effort: gateSweep.effort,
      schema: gateSweep.schema,
    },
    CONFIG.SENSOR_ESCALATE,
  );
}
