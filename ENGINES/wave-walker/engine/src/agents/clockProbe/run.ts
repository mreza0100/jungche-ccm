// runClockProbe — one wall-clock reading. Returns null on ANY doubt (dead seat, missing field,
// non-finite, non-positive): the caller then sheds nothing, because a clock you cannot trust must never
// be the reason a lens is skipped. Failing open here is safe — the walk keeps its full coverage; failing
// closed would trade real lenses for a guess.
import { retryAgent } from '../../runtime.js';
import { clockProbe } from './index.js';
import type { ClockProbeOut } from '../../types/index.js';

// NOTE: the second parameter is `phaseName`, never `phase`. `phase` is an ambient Workflow function,
// and the bundle validator rejects that identifier anywhere it is not a direct call (WF_PROGRAM_011) —
// a parameter of that name, or the `{ phase }` shorthand, fails the build.
export async function runClockProbe(label: string, phaseName: string): Promise<number | null> {
  const out = await retryAgent<ClockProbeOut>(clockProbe.buildPrompt({}), {
    label,
    phase: phaseName,
    model: clockProbe.tier,
    effort: clockProbe.effort,
    schema: clockProbe.schema,
  });
  if (!out) return null;
  const seconds = out.epochSeconds;
  if (typeof seconds !== 'number' || !Number.isFinite(seconds) || seconds <= 0) return null;
  return seconds;
}
