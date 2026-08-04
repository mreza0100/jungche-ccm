import { CONFIG } from './config.js';
import type { AgentOpts, SeatTally } from './types/index.js';

// ─────────────────────────────────────────────────────────────────────────────
// retryAgent() — the shared sub-agent caller (rr calls its analog `retryAgent`; this one is the
// source's `resilient()` lifted VERBATIM/behavior-identical, wave-walker.js lines 385-395 — NOT rr's
// own N-uniform-retry semantics). Exactly ONE respawn on a dead agent (agent() resolving null — a
// terminal API error / safety-classifier block, never a throw): the retry prompt is prefixed
// '[label-retry] RESUME: …' and rides on `escalateModel` when the caller passed one, else the SAME
// model. Every call (first attempt and retry) is prefixed '[label] ' — the token ledger's snippet
// fallback attributes workflow spend per stage. Lives in its own module (bundled before every agent's
// run.ts) so each run fn imports retryAgent without a cycle back through engine.ts.
// ─────────────────────────────────────────────────────────────────────────────

// WALK TELEMETRY (DEBUG STEP) — a lightweight per-seat call tally, mirroring rr's IO_LOG/LOG_BUFFER
// gating pattern (rr engine/src/runtime.ts:14-16, 23,43,63: every increment `if (CONFIG.debug)`) but
// COUNTS ONLY — never the full prompt/output capture rr's IO_LOG does (that forensic layer already
// exists one hop down, in the harness's own journal.jsonl; see tmp/walker-debug-design.md §0/§3). A
// module-level singleton: the bundle concatenates every module into one flat scope (build.js), so this
// is the SAME object every seat's retryAgent call mutates — exactly like rr's own module-level buffers.
export const SEAT_TALLY: Record<string, SeatTally> = {};

// seatKey — derives the tally key as the label's prefix before the FIRST ' · ' or '#', whichever comes
// first (tmp/walker-debug-design.md §4.1). Needs zero changes to any agents/*/run.ts file — every
// seat's `label` already carries this shape verbatim (e.g. 'walk · t1', 'judge · R3#2',
// '2nd-opinion#1', 'scout'). A '-retry' suffix (added by the retry branch below) always lands AFTER
// the first delimiter, so both attempts of a retried call tally under the SAME key.
function seatKey(label: string): string {
  const dot = label.indexOf(' · ');
  const hash = label.indexOf('#');
  const idx = [dot, hash].filter((i) => i >= 0).sort((a, b) => a - b)[0];
  return idx === undefined ? label : label.slice(0, idx);
}

function recordSeat(label: string, patch: Partial<SeatTally>): void {
  const key = seatKey(label);
  const t = (SEAT_TALLY[key] = SEAT_TALLY[key] || {
    calls: 0,
    diedFirstAttempt: 0,
    retried: 0,
    diedAfterRetry: 0,
  });
  t.calls += patch.calls || 0;
  t.diedFirstAttempt += patch.diedFirstAttempt || 0;
  t.retried += patch.retried || 0;
  t.diedAfterRetry += patch.diedAfterRetry || 0;
}

export async function retryAgent<T>(
  prompt: string,
  opts: AgentOpts,
  escalateModel?: string,
): Promise<T | null> {
  const label = opts.label || 'agent';
  let r = (await agent('[' + label + '] ' + prompt, opts)) as T | null;
  if (CONFIG.debug) recordSeat(label, { calls: 1, diedFirstAttempt: r === null ? 1 : 0 });
  if (r === null) {
    const retryModel = escalateModel || opts.model;
    log('⚠ ' + label + ' died · respawning once on ' + retryModel);
    if (CONFIG.debug) recordSeat(label, { retried: 1 });
    r = (await agent(
      '[' +
        label +
        '-retry] RESUME: a prior agent for this exact role died mid-task (often on structured-output). Redo from scratch — idempotent. Keep output values SHORT and schema-exact. ' +
        prompt,
      { ...opts, model: retryModel as AgentOpts['model'], label: label + '-retry' },
    )) as T | null;
    if (CONFIG.debug) recordSeat(label, { diedAfterRetry: r === null ? 1 : 0 });
  }
  return r;
}
