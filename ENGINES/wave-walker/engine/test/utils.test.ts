// utils.test.ts — chunk() (the source's inline helper, line 603) plus its one defensive addition,
// globMatch() (INVARIANT REGISTRY FEATURE, tmp/wave-walker-investigation.md § 2.1 — the zero-token
// territory-glob fail-safe primitive; new, no source equivalent), and renderTelemetryMd() (WALK
// TELEMETRY / DEBUG STEP, tmp/walker-debug-design.md §4/§6 — mechanically renders a DebugRecord into
// the `### Walk Telemetry` markdown block).
import { describe, expect, it } from 'vitest';
import { chunk, globMatch, renderTelemetryMd } from '../src/utils/index.js';
import type { DebugRecord } from '../src/types/index.js';

describe('chunk', () => {
  it('splits into fixed-size slices, last slice short', () => {
    expect(chunk([1, 2, 3, 4, 5], 2)).toEqual([[1, 2], [3, 4], [5]]);
    expect(chunk([1, 2, 3], 6)).toEqual([[1, 2, 3]]);
  });
  it('empty input → no chunks', () => expect(chunk([], 4)).toEqual([]));
  it('size <= 0 degrades to one whole-array chunk (defensive; the engine only calls 6 and 4)', () => {
    expect(chunk([1, 2], 0)).toEqual([[1, 2]]);
    expect(chunk([1, 2], -1)).toEqual([[1, 2]]);
  });
});

describe('globMatch', () => {
  it('`**` matches any run of chars including `/`', () => {
    expect(
      globMatch(
        'app-cortex/src/app_cortex/db/analysis/**',
        'app-cortex/src/app_cortex/db/analysis/chunk_cache.py',
      ),
    ).toBe(true);
    expect(
      globMatch(
        'app-cortex/src/app_cortex/chains/**',
        'app-cortex/src/app_cortex/chains/action_reaction_extraction.py',
      ),
    ).toBe(true);
    expect(
      globMatch(
        'app-be/**',
        'app-be/src/infrastructure/persistence/drizzle/analysis/regeneration.queries.ts',
      ),
    ).toBe(true);
  });
  it('`**` at the tail requires the prefix to still match — a sibling directory does not', () => {
    expect(globMatch('app-fe/**', 'app-be/src/a.ts')).toBe(false);
    expect(
      globMatch(
        'app-cortex/src/app_cortex/chains/**',
        'app-cortex/src/app_cortex/graphs/nodes/insight.py',
      ),
    ).toBe(false);
  });
  it('`*` matches within one path segment only (never crosses `/`)', () => {
    expect(globMatch('app-be/src/*.ts', 'app-be/src/a.ts')).toBe(true);
    expect(globMatch('app-be/src/*.ts', 'app-be/src/sub/a.ts')).toBe(false);
  });
  it('an exact literal path matches only itself', () => {
    expect(
      globMatch(
        'app-be/src/infrastructure/http/recap-chat.ts',
        'app-be/src/infrastructure/http/recap-chat.ts',
      ),
    ).toBe(true);
    expect(
      globMatch(
        'app-be/src/infrastructure/http/recap-chat.ts',
        'app-be/src/infrastructure/http/other.ts',
      ),
    ).toBe(false);
  });
  it('regex-special characters in a path (e.g. a dot) are treated literally, never as regex metachars', () => {
    expect(globMatch('app-be/src/*.ts', 'app-be/src/aXts')).toBe(false); // '.' must not act as regex any-char
  });
  it('a bare `*` does not implicitly anchor across `/` even mid-pattern', () => {
    expect(globMatch('app-be/*/index.ts', 'app-be/src/sub/index.ts')).toBe(false);
    expect(globMatch('app-be/*/index.ts', 'app-be/src/index.ts')).toBe(true);
  });
});

describe('renderTelemetryMd', () => {
  const REC: DebugRecord = {
    schemaVersion: 1,
    mode: 'walk',
    reportPath: 'docs/dev/waves/w/report.md',
    degraded: false,
    gaps: [],
    armedInvariants: {
      registered: 1,
      armed: [{ id: 'ENGINE-REGRADE', matchedFiles: ['a.py'], reason: 'scout + territory glob' }],
      unarmed: [],
    },
    seats: {
      scout: { calls: 1, diedFirstAttempt: 0, retried: 0, diedAfterRetry: 0 },
      walk: { calls: 3, diedFirstAttempt: 1, retried: 1, diedAfterRetry: 0 },
      fold: { calls: 1, diedFirstAttempt: 0, retried: 0, diedAfterRetry: 0 },
    },
    seatsExpectedButAbsent: [],
    emptyResults: {
      threadsWalked: 3,
      threadsExpected: 3,
      sensorsWithCards: 2,
      sensorsExpected: 2,
      hunterFindingsTotal: 1,
      huntersReturned: 1,
      huntersExpected: 1,
      digestsWithFindings: 1,
      digestsExpected: 2,
      securityDied: false,
      coverageCriticDied: false,
      foldDied: false,
    },
    judgeStats: {
      confirmed: 2,
      falseCount: 1,
      unproven: 0,
      secondOpinionDispatched: 1,
      secondOpinionOverturned: 1,
      secondOpinionReexaminedKilled: 0,
      finalJudgeReinstated: 0,
      finalJudgeVerdict: 'ROUGH SEAS',
      finalJudgeDied: false,
    },
    coverage: {
      unsensedFields: [],
      gateSweepSkipped: false,
      gateSweepSkipDetail: '',
      coverageGaps: [],
      verdictContradictions: [],
      contradictionScan: { filesCompared: 2, uncomparableThreads: [] },
    },
    tokenAttribution: null,
  };

  it('renders the `### Walk Telemetry` header, every seat, and the section labels', () => {
    const md = renderTelemetryMd(REC);
    expect(md).toMatch(/^### Walk Telemetry/);
    expect(md).toContain('scout: 1 call(s)');
    expect(md).toContain('walk: 3 call(s)');
    expect(md).toContain('died-first 1');
    expect(md).toContain('retried 1');
    expect(md).toContain('**Invariant registry:** 1 registered, 1 armed (ENGINE-REGRADE)');
    expect(md).toContain('**Coverage:**');
    expect(md).toContain('**Judgment:**');
  });

  it('never renders a MISSING line when seatsExpectedButAbsent is empty', () => {
    expect(renderTelemetryMd(REC)).not.toContain('MISSING');
  });

  it('names an absent-but-expected seat when seatsExpectedButAbsent is non-empty', () => {
    const md = renderTelemetryMd({ ...REC, seatsExpectedButAbsent: ['judge'] });
    expect(md).toContain('**MISSING (expected, never dispatched):** judge');
  });

  it('renders the verdict-contradiction line even at ZERO — the count comes with what was compared, so a 0 reads as a scan result, not silence', () => {
    const md = renderTelemetryMd(REC);
    expect(md).toContain('- verdict contradictions: 0 over 2 file(s) walked by 2+ seats');
  });

  it('names each contradicting pair and says it was escalated, and names threads the scan could not compare', () => {
    const md = renderTelemetryMd({
      ...REC,
      coverage: {
        ...REC.coverage,
        verdictContradictions: [
          {
            file: 'app-web/content/privacy/en.mdx',
            clean: ['T12'],
            flagged: ['CT5 (AT-RISK)'],
          },
        ],
        contradictionScan: { filesCompared: 3, uncomparableThreads: ['T7'] },
      },
    });
    expect(md).toContain(
      '- verdict contradictions: 1 over 3 file(s) walked by 2+ seats · uncomparable (no files in thread spec): T7',
    );
    expect(md).toContain(
      '  - app-web/content/privacy/en.mdx: T12 INTACT vs CT5 (AT-RISK) — escalated to the final judge',
    );
  });

  it('a degraded:true record renders the honest-gap line, never an empty section', () => {
    const md = renderTelemetryMd({
      ...REC,
      degraded: true,
      gaps: ['emptyResults assembly failed: boom'],
    });
    expect(md).toContain('**DEGRADED**');
    expect(md).toContain('emptyResults assembly failed: boom');
    // every other section still renders — never an empty section standing in for the failure
    expect(md).toContain('**Seats:**');
    expect(md).toContain('**Invariant registry:**');
  });

  it('a degraded:true record with an empty gaps array still names the degradation honestly (never a blank line)', () => {
    const md = renderTelemetryMd({ ...REC, degraded: true, gaps: [] });
    expect(md).toContain('**DEGRADED**');
  });

  it('renders honest zeros for an empty seats map, never omitting the section', () => {
    const md = renderTelemetryMd({ ...REC, seats: {} });
    expect(md).toContain('**Seats:**');
    expect(md).toContain('(none recorded)');
  });

  it('a malformed/partial record never throws — falls back to the honest render-failed line, never an empty string', () => {
    const malformed = { degraded: false, gaps: [] } as unknown as DebugRecord; // missing seats/armedInvariants/etc.
    let out = '';
    expect(() => {
      out = renderTelemetryMd(malformed);
    }).not.toThrow();
    expect(out).not.toBe('');
    expect(out).toContain('### Walk Telemetry');
    expect(out).toContain('telemetry render failed');
  });

  it('a completely empty object never throws either', () => {
    const empty = {} as unknown as DebugRecord;
    let out = '';
    expect(() => {
      out = renderTelemetryMd(empty);
    }).not.toThrow();
    expect(out).not.toBe('');
    expect(out).toContain('telemetry render failed');
  });
});
