// runProbe — one probe call for one lane (source lines 217-226), post-annotated with `_laneKind`/
// `_targets` exactly as the source's `.then(r => r && Object.assign(r, {...}))` does — ledger.ts's
// `ingest()` reads both off the returned ProbeOut.
import { retryAgent } from '../../runtime.js';
import { probe } from './index.js';
import type { Lane, ProbeOut } from '../../types/index.js';

export async function runProbe(
  lane: Lane,
  goal: string,
  scopeLine: string,
): Promise<ProbeOut | null> {
  const r = await retryAgent<ProbeOut>(probe.buildPrompt({ lane, goal, scopeLine }), {
    label: 'probe · ' + lane.id,
    phase: 'Investigate',
    model: probe.tier,
    effort: probe.effort,
    schema: probe.schema,
  });
  return r && Object.assign(r, { _laneKind: lane.kind, _targets: lane.targets || [] });
}
