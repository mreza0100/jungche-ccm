import { describe, expect, it } from 'vitest';
import {
  computedConfidence,
  createLedgerState,
  ingest,
  normStmt,
  statusOf,
} from '../src/ledger.js';
import type { LedgerRow, ProbeOut } from '../src/types/index.js';

describe('normStmt', () => {
  it('lowercases, collapses whitespace, and trims', () => {
    expect(normStmt('  The   Cat Sat  ')).toBe('the cat sat');
    expect(normStmt('A\nB\tC')).toBe('a b c');
  });
  it('degrades gracefully on null/undefined', () => {
    expect(normStmt(null)).toBe('');
    expect(normStmt(undefined)).toBe('');
  });
});

const probe = (over: Partial<ProbeOut>): ProbeOut => ({
  laneId: 'w0-1',
  claims: [],
  leads: [],
  ...over,
});

describe('ingest', () => {
  it('mints a fresh ledger row per distinct statement, id-numbered c1, c2, …', () => {
    const state = createLedgerState();
    const fresh = ingest(
      state,
      [
        probe({
          claims: [
            { statement: 'Alpha', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] },
            { statement: 'Beta', anchors: [{ anchor: 'b.ts:1', quote: 'q' }] },
          ],
        }),
      ],
      0,
    );
    expect(fresh).toBe(2);
    expect([...state.ledger.keys()]).toEqual(['c1', 'c2']);
    expect(state.ledger.get('c1')?.statement).toBe('Alpha');
  });

  it('dedupes a repeated statement (normalized) across probes into ONE row, merging anchors', () => {
    const state = createLedgerState();
    ingest(
      state,
      [
        probe({
          claims: [{ statement: '  Alpha  ', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] }],
        }),
      ],
      0,
    );
    const fresh = ingest(
      state,
      [probe({ claims: [{ statement: 'alpha', anchors: [{ anchor: 'b.ts:2', quote: 'q2' }] }] })],
      1,
    );
    expect(fresh).toBe(0); // no NEW row — same normalized statement
    expect(state.ledger.size).toBe(1);
    const row = state.ledger.get('c1')!;
    expect(row.anchors).toEqual([
      { anchor: 'a.ts:1', quote: 'q' },
      { anchor: 'b.ts:2', quote: 'q2' },
    ]);
    expect(row.files.sort()).toEqual(['a.ts', 'b.ts']);
  });

  it('never duplicates the same anchor on a row', () => {
    const state = createLedgerState();
    ingest(
      state,
      [probe({ claims: [{ statement: 'X', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] }] })],
      0,
    );
    ingest(
      state,
      [probe({ claims: [{ statement: 'X', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] }] })],
      1,
    );
    expect(state.ledger.get('c1')!.anchors).toHaveLength(1);
  });

  it('a counter claim marks its targets contested', () => {
    const state = createLedgerState();
    ingest(
      state,
      [
        probe({
          claims: [{ statement: 'X is true', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] }],
        }),
      ],
      0,
    );
    ingest(
      state,
      [
        probe({
          laneId: 'a1',
          claims: [
            {
              statement: 'X is false',
              kind: 'counter',
              targets: ['c1'],
              anchors: [{ anchor: 'b.ts:1', quote: 'q' }],
            },
          ],
        }),
      ],
      1,
    );
    expect(state.ledger.get('c1')!.contested).toBe(true);
  });

  it("credits survival to an attack lane's targets when it finds nothing (nothingFound:true)", () => {
    const state = createLedgerState();
    ingest(
      state,
      [probe({ claims: [{ statement: 'X', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] }] })],
      0,
    );
    const attackResult = probe({ nothingFound: true });
    Object.assign(attackResult, { _laneKind: 'attack', _targets: ['c1'] });
    ingest(state, [attackResult], 1);
    expect(state.ledger.get('c1')!.survived).toBe(1);
  });

  it('credits survival when an attack lane returns zero counter claims (even without nothingFound)', () => {
    const state = createLedgerState();
    ingest(
      state,
      [probe({ claims: [{ statement: 'X', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] }] })],
      0,
    );
    const attackResult = probe({
      claims: [
        {
          statement: 'irrelevant support',
          kind: 'support',
          anchors: [{ anchor: 'z.ts:1', quote: 'q' }],
        },
      ],
    });
    Object.assign(attackResult, { _laneKind: 'attack', _targets: ['c1'] });
    ingest(state, [attackResult], 1);
    expect(state.ledger.get('c1')!.survived).toBe(1);
  });

  it('does NOT credit survival when an attack lane actually lands a counter', () => {
    const state = createLedgerState();
    ingest(
      state,
      [probe({ claims: [{ statement: 'X', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] }] })],
      0,
    );
    const attackResult = probe({
      claims: [
        {
          statement: 'X is false',
          kind: 'counter',
          targets: ['c1'],
          anchors: [{ anchor: 'b.ts:1', quote: 'q' }],
        },
      ],
    });
    Object.assign(attackResult, { _laneKind: 'attack', _targets: ['c1'] });
    ingest(state, [attackResult], 1);
    expect(state.ledger.get('c1')!.survived).toBe(0);
  });

  it("mints fresh lead ids L1, L2, … from every probe's leads, across calls", () => {
    const state = createLedgerState();
    ingest(state, [probe({ leads: [{ what: 'follow up A', files: ['a.ts'] }] })], 0);
    ingest(state, [probe({ leads: [{ what: 'follow up B' }] })], 1);
    expect([...state.leads.keys()]).toEqual(['L1', 'L2']);
    expect(state.leads.get('L2')).toEqual({ id: 'L2', what: 'follow up B', files: [] });
  });

  it('skips null probe results (a died lane)', () => {
    const state = createLedgerState();
    const fresh = ingest(
      state,
      [null, probe({ claims: [{ statement: 'X', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] }] })],
      0,
    );
    expect(fresh).toBe(1);
  });

  // sparse-input fallbacks — the reducer's defensive `|| []` / `a && a.anchor` alternates, hand-traced.
  it('tolerates undefined claims/leads arrays on a probe result', () => {
    const state = createLedgerState();
    const bare = { laneId: 'w0-1' } as unknown as ProbeOut;
    expect(ingest(state, [bare], 0)).toBe(0);
    expect(state.ledger.size).toBe(0);
    expect(state.leads.size).toBe(0);
  });
  it('tolerates null/anchor-less anchor entries — no anchor added, no file derived', () => {
    const state = createLedgerState();
    ingest(
      state,
      [
        probe({
          claims: [
            {
              statement: 'X',
              anchors: [null, { quote: 'q' }, { anchor: '', quote: 'q' }] as never,
            },
          ],
        }),
      ],
      0,
    );
    const row = state.ledger.get('c1')!;
    expect(row.anchors).toEqual([]);
    expect(row.files).toEqual([]);
  });
  it('a claim with undefined anchors still ledgers (defensive `c.anchors || []`)', () => {
    const state = createLedgerState();
    ingest(state, [probe({ claims: [{ statement: 'X' } as never] })], 0);
    expect(state.ledger.get('c1')!.anchors).toEqual([]);
  });
  it('a counter with undefined targets, or targeting an unknown id, contests nothing', () => {
    const state = createLedgerState();
    ingest(
      state,
      [probe({ claims: [{ statement: 'X', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] }] })],
      0,
    );
    ingest(
      state,
      [
        probe({
          claims: [
            { statement: 'C1', kind: 'counter', anchors: [{ anchor: 'b.ts:1', quote: 'q' }] },
          ],
        }),
        probe({
          claims: [
            {
              statement: 'C2',
              kind: 'counter',
              targets: ['c99'],
              anchors: [{ anchor: 'c.ts:1', quote: 'q' }],
            },
          ],
        }),
      ],
      1,
    );
    expect(state.ledger.get('c1')!.contested).toBe(false);
  });
  it('an attack lane with undefined _targets, or targeting an unknown id, credits no survival', () => {
    const state = createLedgerState();
    ingest(
      state,
      [probe({ claims: [{ statement: 'X', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] }] })],
      0,
    );
    const noTargets = probe({ nothingFound: true });
    Object.assign(noTargets, { _laneKind: 'attack' });
    const ghostTarget = probe({ nothingFound: true });
    Object.assign(ghostTarget, { _laneKind: 'attack', _targets: ['c99'] });
    ingest(state, [noTargets, ghostTarget], 1);
    expect(state.ledger.get('c1')!.survived).toBe(0);
  });
});

const row = (over: Partial<LedgerRow>): LedgerRow => ({
  id: 'c1',
  statement: 's',
  anchors: [],
  files: [],
  contested: false,
  survived: 0,
  audit: 'pending',
  wave: 0,
  ...over,
});

describe('statusOf — COMPUTED from ledger topology, never asserted', () => {
  it('contested wins over everything else', () => {
    expect(
      statusOf(
        row({ contested: true, audit: 'pass', files: ['a.ts', 'b.ts', 'c.ts'], survived: 5 }),
      ),
    ).toBe('contested');
  });
  it('an audit failure is tentative (even with independent files)', () => {
    expect(statusOf(row({ audit: 'fail', files: ['a.ts', 'b.ts', 'c.ts'] }))).toBe('tentative');
  });
  it('settled requires audit pass AND (>=2 files AND (a survived challenge OR >=3 files))', () => {
    expect(statusOf(row({ audit: 'pass', files: ['a.ts', 'b.ts'], survived: 1 }))).toBe('settled');
    expect(statusOf(row({ audit: 'pass', files: ['a.ts', 'b.ts', 'c.ts'], survived: 0 }))).toBe(
      'settled',
    );
  });
  it('two files with no survived challenge is only tentative', () => {
    expect(statusOf(row({ audit: 'pass', files: ['a.ts', 'b.ts'], survived: 0 }))).toBe(
      'tentative',
    );
  });
  it('a pending audit is tentative regardless of file count', () => {
    expect(statusOf(row({ audit: 'pending', files: ['a.ts', 'b.ts', 'c.ts'], survived: 1 }))).toBe(
      'tentative',
    );
  });
});

describe('computedConfidence — over exactly the given key claim ids', () => {
  it('no key ids → low', () => {
    const state = createLedgerState();
    expect(computedConfidence(state, [])).toBe('low');
  });
  it('an unresolvable key id → low', () => {
    const state = createLedgerState();
    expect(computedConfidence(state, ['c99'])).toBe('low');
  });
  it('any contested or audit-failed key claim → low', () => {
    const state = createLedgerState();
    state.ledger.set('c1', row({ id: 'c1', contested: true }));
    state.ledger.set('c2', row({ id: 'c2', audit: 'fail' }));
    expect(computedConfidence(state, ['c1'])).toBe('low');
    expect(computedConfidence(state, ['c2'])).toBe('low');
  });
  it('every key claim settled → high', () => {
    const state = createLedgerState();
    state.ledger.set('c1', row({ id: 'c1', audit: 'pass', files: ['a.ts', 'b.ts', 'c.ts'] }));
    state.ledger.set('c2', row({ id: 'c2', audit: 'pass', files: ['x.ts', 'y.ts'], survived: 1 }));
    expect(computedConfidence(state, ['c1', 'c2'])).toBe('high');
  });
  it('a mix of settled and tentative (no contested/failed) → medium', () => {
    const state = createLedgerState();
    state.ledger.set('c1', row({ id: 'c1', audit: 'pass', files: ['a.ts', 'b.ts', 'c.ts'] }));
    state.ledger.set('c2', row({ id: 'c2', audit: 'pending' }));
    expect(computedConfidence(state, ['c1', 'c2'])).toBe('medium');
  });
});
