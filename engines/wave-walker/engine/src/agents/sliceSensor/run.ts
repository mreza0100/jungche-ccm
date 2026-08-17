// runSliceSensor — one call per scheduled job (source lines 451-468). Escalates to CONFIG.SENSOR_ESCALATE
// on a dead respawn — the source's third `resilient()` arg (SENSOR_ESCALATE, fixed 'sonnet', not an arg).
import { retryAgent } from '../../runtime.js';
import { sliceSensor } from './index.js';
import { CONFIG } from '../../config.js';
import type { SliceSensorArgs, SlicesOut } from '../../types/index.js';

export function runSliceSensor(args: SliceSensorArgs): Promise<SlicesOut | null> {
  return retryAgent<SlicesOut>(
    sliceSensor.buildPrompt(args),
    {
      label: args.kind + ' · ' + args.jobId,
      phase: 'Walk',
      model: sliceSensor.tier,
      effort: sliceSensor.effort,
      schema: sliceSensor.schema,
    },
    CONFIG.SENSOR_ESCALATE,
  );
}
