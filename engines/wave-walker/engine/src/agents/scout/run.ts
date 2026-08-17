// runScout — the ONE scout-scheduler call (source lines 400-415). Pure request/response: every mutation
// (threads concat, job splitting/capping, authRule derivation) is owned by engine.ts's runWalk().
import { retryAgent } from '../../runtime.js';
import { scout } from './index.js';
import type { ScoutArgs, ScoutOut } from '../../types/index.js';

export function runScout(args: ScoutArgs): Promise<ScoutOut | null> {
  return retryAgent<ScoutOut>(scout.buildPrompt(args), {
    label: 'scout',
    phase: 'Scout',
    model: scout.tier,
    effort: scout.effort,
    schema: scout.schema,
  });
}
