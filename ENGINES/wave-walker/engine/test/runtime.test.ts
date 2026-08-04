// runtime.test.ts — WALK TELEMETRY (DEBUG STEP, tmp/walker-debug-design.md §4/§6/§10). First dedicated
// test file for runtime.ts in EITHER engine (neither rr nor walker has one) — SEAT_TALLY is new,
// self-contained, and worth its own file rather than folding into engine.test.ts's pipeline-smoke style.
// SEAT_TALLY is a module-level singleton (mirrors rr's IO_LOG/LOG_BUFFER — the flat bundle concatenates
// every module into ONE scope, so it's the SAME object every seat's retryAgent call mutates), so every
// test resets it via vi.resetModules() + a fresh dynamic import, exactly like engine.test.ts's own
// freshEngine() pattern.
import { beforeEach, describe, expect, it, vi } from 'vitest';

async function freshRuntime(args: Record<string, unknown>) {
  vi.resetModules();
  globalThis.args = args;
  globalThis.log = () => {};
  globalThis.phase = () => {};
  const { retryAgent } = await import('../src/runtime.js');
  return retryAgent;
}

describe('retryAgent — WALK TELEMETRY seat tally (SEAT_TALLY)', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it('a clean first-attempt success records calls: 1, every death counter 0', async () => {
    globalThis.args = { reportPath: 'r.md' };
    globalThis.log = () => {};
    globalThis.phase = () => {};
    globalThis.agent = async () => ({ ok: true });
    const runtime = await import('../src/runtime.js');
    const r = await runtime.retryAgent('do the thing', {
      label: 'scout',
      model: 'sonnet',
      effort: 'high',
    });
    expect(r).toEqual({ ok: true });
    expect(runtime.SEAT_TALLY.scout).toEqual({
      calls: 1,
      diedFirstAttempt: 0,
      retried: 0,
      diedAfterRetry: 0,
    });
  });

  it('a death-then-recovery sequence records diedFirstAttempt: 1, retried: 1, diedAfterRetry: 0', async () => {
    globalThis.args = { reportPath: 'r.md' };
    globalThis.log = () => {};
    globalThis.phase = () => {};
    let calls = 0;
    globalThis.agent = async () => {
      calls++;
      return calls === 1 ? null : { ok: true };
    };
    const runtime = await import('../src/runtime.js');
    const r = await runtime.retryAgent('do the thing', {
      label: 'walk · t1',
      model: 'sonnet',
      effort: 'high',
    });
    expect(r).toEqual({ ok: true });
    expect(runtime.SEAT_TALLY.walk).toEqual({
      calls: 1,
      diedFirstAttempt: 1,
      retried: 1,
      diedAfterRetry: 0,
    });
  });

  it('a death-then-death sequence records diedAfterRetry: 1', async () => {
    globalThis.args = { reportPath: 'r.md' };
    globalThis.log = () => {};
    globalThis.phase = () => {};
    globalThis.agent = async () => null;
    const runtime = await import('../src/runtime.js');
    const r = await runtime.retryAgent('do the thing', {
      label: 'fold',
      model: 'sonnet',
      effort: 'high',
    });
    expect(r).toBeNull();
    expect(runtime.SEAT_TALLY.fold).toEqual({
      calls: 1,
      diedFirstAttempt: 1,
      retried: 1,
      diedAfterRetry: 1,
    });
  });

  it('CONFIG.debug === false → SEAT_TALLY stays empty — pins "zero cost when off" as behavior, not prose', async () => {
    globalThis.args = { reportPath: 'r.md', debug: false };
    globalThis.log = () => {};
    globalThis.phase = () => {};
    let calls = 0;
    globalThis.agent = async () => {
      calls++;
      return calls === 1 ? null : { ok: true };
    };
    const runtime = await import('../src/runtime.js');
    await runtime.retryAgent('do the thing', { label: 'scout', model: 'sonnet', effort: 'high' });
    expect(runtime.SEAT_TALLY).toEqual({});
  });

  it("keys by the label PREFIX before the first ' · ' or '#' — a retried call tallies under the SAME key as its first attempt", async () => {
    globalThis.args = { reportPath: 'r.md' };
    globalThis.log = () => {};
    globalThis.phase = () => {};
    globalThis.agent = async () => null;
    const runtime = await import('../src/runtime.js');
    await runtime.retryAgent('x', { label: 'judge · R3#2', model: 'sonnet', effort: 'high' });
    expect(Object.keys(runtime.SEAT_TALLY)).toEqual(['judge']);
    expect(runtime.SEAT_TALLY.judge.diedAfterRetry).toBe(1);
  });

  it('sliceSensor labels fan out into separate producer/consumer/cortex buckets (kind-based, not one blended "sliceSensor" row)', async () => {
    globalThis.args = { reportPath: 'r.md' };
    globalThis.log = () => {};
    globalThis.phase = () => {};
    globalThis.agent = async () => ({ ok: true });
    const runtime = await import('../src/runtime.js');
    await runtime.retryAgent('x', { label: 'producer · p1', model: 'haiku', effort: 'medium' });
    await runtime.retryAgent('x', { label: 'consumer · f1', model: 'haiku', effort: 'medium' });
    await runtime.retryAgent('x', { label: 'cortex · x1', model: 'haiku', effort: 'medium' });
    expect(Object.keys(runtime.SEAT_TALLY).sort()).toEqual(['consumer', 'cortex', 'producer']);
  });

  it('multiple calls under the same seat accumulate rather than overwrite', async () => {
    globalThis.args = { reportPath: 'r.md' };
    globalThis.log = () => {};
    globalThis.phase = () => {};
    globalThis.agent = async () => ({ ok: true });
    const runtime = await import('../src/runtime.js');
    await runtime.retryAgent('x', { label: 'walk · t1', model: 'sonnet', effort: 'high' });
    await runtime.retryAgent('x', { label: 'walk · t2', model: 'sonnet', effort: 'high' });
    await runtime.retryAgent('x', { label: 'walk · t3', model: 'sonnet', effort: 'high' });
    expect(runtime.SEAT_TALLY.walk.calls).toBe(3);
  });

  it('resolves to null exactly like before this feature — the retry mechanics themselves are unchanged', async () => {
    const retryAgent = await freshRuntime({ reportPath: 'r.md' });
    globalThis.agent = async () => null;
    const r = await retryAgent('x', { label: 'scout', model: 'sonnet', effort: 'high' });
    expect(r).toBeNull();
  });
});
