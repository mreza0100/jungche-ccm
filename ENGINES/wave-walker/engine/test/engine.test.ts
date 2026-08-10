// engine.test.ts — SMOKE: `new WaveWalker().run()` completes without throwing, for each of the four
// modes, under one deliberately-generic stubbed `agent()` (every schema's required field, safe empty
// defaults — the engine's own `|| []` / `|| ''` guards absorb the rest, same discipline the source relies
// on). Deep per-phase behavior is covered by rules.test.ts / ledger.test.ts / prompts.test.ts /
// schemas.test.ts; this only proves the mode dispatch + the whole pipeline wires together.
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { DebugAssemblyInput } from '../src/engine.js';

// one object satisfying every seat schema's REQUIRED fields at once — see file header.
const BLOB = {
  headSha: 'abc123',
  territories: [] as string[],
  changedFiles: ['a.ts'],
  changedFileCount: 1,
  mergeShas: ['abc123'],
  threads: [{ id: 't1', type: 'flow', name: 'Flow', verify: 'v' }] as unknown[],
  operations: [] as unknown[],
  fields: [] as unknown[],
  jobs: [] as unknown[],
  gateFiles: [] as string[],
  authRule: '',
  threadId: 't1',
  flow: 'INTACT',
  trace: '',
  defects: [] as unknown[],
  hygiene: [] as unknown[],
  name: '',
  type: '',
  notes: '',
  file: 'f.ts',
  gates: [] as unknown[],
  jobId: 'j1',
  slices: [] as unknown[],
  undeclaredReads: [] as unknown[],
  verdicts: [] as unknown[],
  territory: 'BE',
  findings: [] as unknown[],
  summary: '',
  verdict: 'SMOOTH SAILING',
  actionItems: [] as string[],
  review: '',
  reinstated: [] as unknown[],
  missedRisks: [] as unknown[],
  rationale: '',
  categoriesSwept: [] as string[],
  filesOpened: [] as string[],
  claims: [
    {
      id: 'c1',
      statement: 'X holds',
      anchors: [
        { anchor: 'a.ts:1', quote: 'q1' },
        { anchor: 'b.ts:2', quote: 'q2' },
      ],
    },
  ] as unknown[],
  conflictChecks: [] as unknown[],
  claimId: 'c1',
  evidence: [] as unknown[],
  reasoning: '',
  conflicts: [] as unknown[],
  laneId: 'w0-1',
  leads: [] as unknown[],
  nothingFound: false,
  resultSoFar: '',
  keyClaimIds: [] as string[],
  lanes: [] as unknown[],
  dropLeads: [] as string[],
  stop: { done: true, reason: 'smoke' },
  audits: [] as unknown[],
  answer: '',
  confidence: 'low',
  report: '',
};

async function freshEngine(args: Record<string, unknown>) {
  vi.resetModules();
  globalThis.args = args;
  globalThis.log = () => {};
  globalThis.phase = () => {};
  globalThis.agent = async () => ({ ...BLOB });
  globalThis.parallel = async <T>(thunks: Array<() => Promise<T>>): Promise<T[]> =>
    Promise.all(thunks.map((t) => t()));
  globalThis.budget = { total: null, spent: () => 0, remaining: () => Infinity };
  const { WaveWalker } = await import('../src/engine.js');
  return new WaveWalker();
}

describe('WaveWalker — mode dispatch smoke test', () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it('constructs and runs to DONE under WALK mode (reportPath)', async () => {
    const ww = await freshEngine({ reportPath: 'docs/dev/waves/w/report.md' });
    const result = await ww.run();
    expect(result.status).toBe('DONE');
    if (result.status === 'DONE' && 'ledger' in result) {
      expect(result.verdict).toBe('SMOOTH SAILING');
      expect(result.reportPath).toBe('docs/dev/waves/w/report.md');
    }
  });

  it('constructs and runs to DONE under VERIFY mode (claims)', async () => {
    const ww = await freshEngine({ claims: [{ id: 'c1', statement: 'X exists' }] });
    const result = await ww.run();
    expect(result.status).toBe('DONE');
    if (result.status === 'DONE' && 'mode' in result && result.mode !== 'investigate') {
      expect(result.mode).toBe('verify');
      expect(result.claims).toBe(1);
    }
  });

  it('constructs and runs to DONE under MANIFEST-VERIFY mode (manifestPath)', async () => {
    const ww = await freshEngine({ manifestPath: 'docs/dev/waves/w/manifest.md' });
    const result = await ww.run();
    expect(result.status).toBe('DONE');
    if (result.status === 'DONE' && 'mode' in result && result.mode !== 'investigate') {
      expect(result.mode).toBe('manifest-verify');
    }
  });

  it('constructs and runs to DONE under INVESTIGATE mode (goal)', async () => {
    const ww = await freshEngine({ goal: 'why does the login flow fail intermittently' });
    const result = await ww.run();
    expect(result.status).toBe('DONE');
    if (result.status === 'DONE' && 'mode' in result && result.mode === 'investigate') {
      expect(result.goal).toBe('why does the login flow fail intermittently');
    }
  });

  it('WALK mode returns FAILED (never a verdict) when the scout resolves an empty changed-file set', async () => {
    vi.resetModules();
    globalThis.args = { reportPath: 'r.md' };
    globalThis.log = () => {};
    globalThis.phase = () => {};
    globalThis.agent = async () => ({ ...BLOB, changedFiles: [] });
    globalThis.parallel = async <T>(thunks: Array<() => Promise<T>>): Promise<T[]> =>
      Promise.all(thunks.map((t) => t()));
    globalThis.budget = { total: null, spent: () => 0, remaining: () => Infinity };
    const { WaveWalker } = await import('../src/engine.js');
    const result = await new WaveWalker().run();
    expect(result.status).toBe('FAILED');
  });

  it('WALK mode returns FAILED (SCOUT FAILURE) when the scout enumerates ZERO threads over a NON-EMPTY diff', async () => {
    vi.resetModules();
    globalThis.args = { reportPath: 'r.md' };
    globalThis.log = () => {};
    globalThis.phase = () => {};
    globalThis.agent = async () => ({
      ...BLOB,
      threads: [],
      changedFiles: ['a.ts', 'b.ts'],
      changedFileCount: 2,
    });
    globalThis.parallel = async <T>(thunks: Array<() => Promise<T>>): Promise<T[]> =>
      Promise.all(thunks.map((t) => t()));
    globalThis.budget = { total: null, spent: () => 0, remaining: () => Infinity };
    const { WaveWalker } = await import('../src/engine.js');
    const result = await new WaveWalker().run();
    expect(result.status).toBe('FAILED');
    if (result.status === 'FAILED') {
      expect(result.detail).toMatch(/SCOUT FAILURE/);
      expect(result.detail).toMatch(/2 changed files/);
    }
  });

  it('WALK mode returns FAILED when the scout dies twice (agent resolves null)', async () => {
    vi.resetModules();
    globalThis.args = { reportPath: 'r.md' };
    globalThis.log = () => {};
    globalThis.phase = () => {};
    globalThis.agent = async () => null;
    globalThis.parallel = async <T>(thunks: Array<() => Promise<T>>): Promise<T[]> =>
      Promise.all(thunks.map((t) => t()));
    globalThis.budget = { total: null, spent: () => 0, remaining: () => Infinity };
    const { WaveWalker } = await import('../src/engine.js');
    const result = await new WaveWalker().run();
    expect(result.status).toBe('FAILED');
    if (result.status === 'FAILED') expect(result.detail).toMatch(/scout died twice/);
  });
});

// ─── E2 manifest-coverage lever — solo/batch gating, coverage fields, judge payload diet ───
interface CapturedCall {
  prompt: string;
  opts: {
    label: string;
    model?: string;
    effort?: string;
    schema?: { properties?: Record<string, unknown> };
  };
}

// verify-mode stub: answers the extractor, solo verifiers, batch verifiers, and the consistency judge
// off the '[label]' prompt prefix; records every call for shape assertions.
function verifyStub(calls: CapturedCall[], opts?: { verdictFor?: (claimId: string) => string }) {
  const verdictFor = opts?.verdictFor ?? (() => 'CONFIRMED');
  return async (prompt: string, callOpts: unknown) => {
    calls.push({ prompt, opts: callOpts as CapturedCall['opts'] });
    if (prompt.startsWith('[verify · ')) {
      const claimId = /CLAIM (\S+):/.exec(prompt)![1];
      return { claimId, verdict: verdictFor(claimId), reasoning: 'REASONING-' + claimId };
    }
    if (prompt.startsWith('[verify-batch · ')) {
      const ids = [...prompt.matchAll(/CLAIM (\S+):/g)].map((m) => m[1]);
      return {
        verdicts: ids.map((claimId) => ({
          claimId,
          verdict: verdictFor(claimId),
          reasoning: 'REASONING-' + claimId,
        })),
      };
    }
    if (prompt.startsWith('[conflict-judge]')) return { conflicts: [], summary: 'ok' };
    // TIME CHECKPOINT — both readings return the SAME instant, so elapsed is 0 and no lens is ever
    // shed. Every pre-existing expectation therefore holds unchanged; the shed path is exercised by
    // its own tests, which vary the two readings deliberately.
    if (prompt.startsWith('[clock · ')) return { epochSeconds: 1_000_000 };
    throw new Error('unexpected agent call in verify stub: ' + prompt.slice(0, 60));
  };
}

async function runVerifyWith(
  args: Record<string, unknown>,
  calls: CapturedCall[],
  stub: (p: string, o: unknown) => Promise<unknown>,
) {
  vi.resetModules();
  globalThis.args = args;
  globalThis.log = () => {};
  globalThis.phase = () => {};
  globalThis.agent = stub;
  globalThis.parallel = async <T>(thunks: Array<() => Promise<T>>): Promise<T[]> =>
    Promise.all(thunks.map((t) => t()));
  globalThis.budget = { total: null, spent: () => 0, remaining: () => Infinity };
  const { WaveWalker } = await import('../src/engine.js');
  return new WaveWalker().run();
}

describe('E2 — solo/batch gating on SOLO_THRESHOLD', () => {
  const soloClaims = [
    { id: 'c1', statement: 'X exists', files: ['app-be/src/a.ts'] },
    { id: 'c2', statement: 'Y exists', files: ['app-be/src/b.ts'] },
    { id: 'c3', statement: 'Z exists' },
  ];

  it('a panel ≤ SOLO_THRESHOLD runs SOLO — one call per claim×vote, byte-identical prompt+schema to the source', async () => {
    const calls: CapturedCall[] = [];
    const result = await runVerifyWith({ claims: soloClaims }, calls, verifyStub(calls));
    const verifierCalls = calls.filter((c) => c.prompt.startsWith('[verify · '));
    expect(verifierCalls).toHaveLength(3); // 3 claims × 1 vote, no batch calls anywhere
    expect(calls.some((c) => c.prompt.startsWith('[verify-batch'))).toBe(false);
    // call shape unchanged vs the source: label, solo VERIFY schema (claimId at top level), prompt text
    const { expectedClaimVerifier } = await import('./fixtures/source-prompts.js');
    expect(verifierCalls[0].prompt).toBe(
      '[verify · c1] ' +
        expectedClaimVerifier({ claim: soloClaims[0], question: '', repoRoot: '.' }),
    );
    expect(verifierCalls[0].opts.label).toBe('verify · c1');
    expect(verifierCalls[0].opts.schema?.properties).toHaveProperty('claimId');
    expect(verifierCalls[0].opts.schema?.properties).not.toHaveProperty('verdicts');
    if (result.status === 'DONE' && 'consensus' in result) {
      expect(result.consensus).toEqual({ c1: 'CONFIRMED', c2: 'CONFIRMED', c3: 'CONFIRMED' });
      expect(result.claimsVerified).toBe(3);
      expect(result.claimsMined).toBe(3);
      expect(result.droppedClaimIds).toEqual([]);
    }
  });

  it('solo mode preserves per-claim `claim.opus` escalation to the secondOpinion tier', async () => {
    const calls: CapturedCall[] = [];
    await runVerifyWith(
      {
        claims: [
          { id: 'c1', statement: 'S', opus: true },
          { id: 'c2', statement: 'T' },
        ],
      },
      calls,
      verifyStub(calls),
    );
    const byLabel = Object.fromEntries(calls.map((c) => [c.opts.label, c.opts.model]));
    expect(byLabel['verify · c1']).toBe('opus');
    expect(byLabel['verify · c2']).toBe('sonnet');
  });

  it('a panel > SOLO_THRESHOLD batches ≤4 claims by file-cluster; verifiers stay sonnet (never haiku)', async () => {
    // 10 claims × 1 vote > 8 → batch. Two depth-2 clusters: 6× app-be/src, 4× app-fe/app.
    const claims = Array.from({ length: 10 }, (_, i) => ({
      id: 'c' + (i + 1),
      statement: 'S' + (i + 1),
      taskId: 'T' + ((i % 5) + 1),
      files: [i < 6 ? 'app-be/src/f' + i + '.ts' : 'app-fe/app/f' + i + '.tsx'],
    }));
    const calls: CapturedCall[] = [];
    const result = await runVerifyWith({ claims }, calls, verifyStub(calls));
    const batchCalls = calls.filter((c) => c.prompt.startsWith('[verify-batch'));
    expect(calls.some((c) => c.prompt.startsWith('[verify · '))).toBe(false); // no solo calls
    expect(batchCalls).toHaveLength(3); // be-cluster [4,2] + fe-cluster [4]
    for (const bc of batchCalls) {
      const n = [...bc.prompt.matchAll(/CLAIM (\S+):/g)].length;
      expect(n).toBeLessThanOrEqual(4);
      expect(bc.opts.model).toBe('sonnet'); // verifier tier — never haiku
      expect(bc.opts.effort).toBe('xhigh');
      expect(bc.opts.schema?.properties).toHaveProperty('verdicts'); // VERIFY_BATCH shape
    }
    // batches never mix clusters
    const batchIds = batchCalls.map((bc) =>
      [...bc.prompt.matchAll(/CLAIM (\S+):/g)].map((m) => m[1]),
    );
    expect(batchIds).toEqual([
      ['c1', 'c2', 'c3', 'c4'],
      ['c5', 'c6'],
      ['c7', 'c8', 'c9', 'c10'],
    ]);
    if (result.status === 'DONE' && 'consensus' in result) {
      expect(result.verdicts).toHaveLength(10); // flattened — one verdict per claim
      expect(result.verifiersDied).toBe(0);
      expect(result.taskIds.sort()).toEqual(['T1', 'T2', 'T3', 'T4', 'T5']);
    }
  });

  it('args.soloThreshold moves the gate (a 3-claim panel batches when the threshold is 2)', async () => {
    const calls: CapturedCall[] = [];
    await runVerifyWith({ claims: soloClaims, soloThreshold: 2 }, calls, verifyStub(calls));
    expect(calls.some((c) => c.prompt.startsWith('[verify-batch'))).toBe(true);
    expect(calls.some((c) => c.prompt.startsWith('[verify · '))).toBe(false);
  });

  it('votes multiply the panel: 5 claims × 2 votes = 10 > 8 → batches, one call per batch × vote', async () => {
    const claims = Array.from({ length: 5 }, (_, i) => ({
      id: 'c' + (i + 1),
      statement: 'S',
      files: ['x/y/f.ts'],
    }));
    const calls: CapturedCall[] = [];
    await runVerifyWith({ claims, votes: 2 }, calls, verifyStub(calls));
    const batchCalls = calls.filter((c) => c.prompt.startsWith('[verify-batch'));
    expect(batchCalls).toHaveLength(4); // 2 batches (4+1, one cluster) × 2 votes
    expect(batchCalls.map((c) => c.opts.label).sort()).toEqual([
      'verify-batch · b1 #1',
      'verify-batch · b1 #2',
      'verify-batch · b2 #1',
      'verify-batch · b2 #2',
    ]);
  });
});

describe('E2 — manifest-verify coverage fields + consistency-judge payload diet', () => {
  const manifestStub = (
    calls: CapturedCall[],
    minedClaims: unknown[],
    verdictFor?: (id: string) => string,
  ) => {
    const inner = verifyStub(calls, { verdictFor });
    return async (prompt: string, callOpts: unknown) => {
      if (prompt.startsWith('[extract-claims]')) {
        calls.push({ prompt, opts: callOpts as CapturedCall['opts'] });
        return { claims: minedClaims, conflictChecks: [{ id: 'cc1', what: 'w' }] };
      }
      return inner(prompt, callOpts);
    };
  };

  it('reports claimsMined / claimsVerified / droppedClaimIds when the cap drops the tail', async () => {
    const mined = [
      { id: 'T1-C1', taskId: 'T1', statement: 'A', files: ['be/src/a.ts'] },
      { id: 'T2-C1', taskId: 'T2', statement: 'B', files: ['be/src/b.ts'] },
      { id: 'T3-C1', taskId: 'T3', statement: 'C', files: ['be/src/c.ts'] },
    ];
    const calls: CapturedCall[] = [];
    const result = await runVerifyWith(
      { manifestPath: 'm.md', maxClaims: 2 },
      calls,
      manifestStub(calls, mined),
    );
    expect(result.status).toBe('DONE');
    if (result.status === 'DONE' && 'consensus' in result) {
      expect(result.mode).toBe('manifest-verify');
      expect(result.claimsMined).toBe(3);
      expect(result.claimsVerified).toBe(2);
      expect(result.droppedClaimIds).toEqual(['T3-C1']);
      expect(result.taskIds).toEqual(['T1', 'T2']); // only VERIFIED claims count toward coverage
    }
  });

  it('feeds the consistency judge the consensus map + full detail ONLY for non-CONFIRMED claims', async () => {
    const mined = [
      { id: 'T1-C1', taskId: 'T1', statement: 'A', files: ['be/src/a.ts'] },
      { id: 'T2-C1', taskId: 'T2', statement: 'B', files: ['be/src/b.ts'] },
    ];
    const calls: CapturedCall[] = [];
    const result = await runVerifyWith(
      { manifestPath: 'm.md' },
      calls,
      manifestStub(calls, mined, (id) => (id === 'T2-C1' ? 'REFUTED' : 'CONFIRMED')),
    );
    const judgeCall = calls.find((c) => c.prompt.startsWith('[conflict-judge]'));
    expect(judgeCall).toBeDefined();
    // the diet: consensus names both claims; full verdict detail carries ONLY the refuted one
    expect(judgeCall!.prompt).toContain('Confirmed claims are listed in Consensus only');
    expect(judgeCall!.prompt).toContain('"T1-C1":"CONFIRMED"');
    expect(judgeCall!.prompt).toContain('REASONING-T2-C1');
    expect(judgeCall!.prompt).not.toContain('REASONING-T1-C1');
    if (result.status === 'DONE' && 'consensus' in result) {
      expect(result.consensus).toEqual({ 'T1-C1': 'CONFIRMED', 'T2-C1': 'REFUTED' });
      expect(result.claimsMined).toBe(2);
      expect(result.droppedClaimIds).toEqual([]);
    }
  });

  it('the empty-claims early return carries the coverage fields too', async () => {
    const calls: CapturedCall[] = [];
    const result = await runVerifyWith({ manifestPath: 'm.md' }, calls, manifestStub(calls, []));
    expect(result.status).toBe('DONE');
    if (result.status === 'DONE' && 'consensus' in result) {
      expect(result.claims).toBe(0);
      expect(result.claimsMined).toBe(0);
      expect(result.claimsVerified).toBe(0);
      expect(result.droppedClaimIds).toEqual([]);
      expect(result.taskIds).toEqual([]);
    }
  });

  it('a dead batch call counts in verifiersDied and its claims fall to NO-VERDICT', async () => {
    const claims = Array.from({ length: 9 }, (_, i) => ({
      id: 'c' + (i + 1),
      statement: 'S',
      files: ['x/y/f' + i + '.ts'],
    }));
    const calls: CapturedCall[] = [];
    let killFirst = 2; // retryAgent respawns once — kill BOTH attempts of the first batch call
    const stub = async (prompt: string, callOpts: unknown) => {
      calls.push({ prompt, opts: callOpts as CapturedCall['opts'] });
      if (prompt.includes('verify-batch · b1') && killFirst > 0) {
        killFirst--;
        return null;
      }
      const ids = [...prompt.matchAll(/CLAIM (\S+):/g)].map((m) => m[1]);
      return {
        verdicts: ids.map((claimId) => ({ claimId, verdict: 'CONFIRMED', reasoning: 'r' })),
      };
    };
    const result = await runVerifyWith({ claims }, calls, stub);
    if (result.status === 'DONE' && 'consensus' in result) {
      expect(result.verifiersDied).toBe(1); // one dead BATCH call (batches: 4+4+1 → 3 calls)
      expect(result.consensus.c1).toBe('NO-VERDICT');
      expect(result.consensus.c5).toBe('CONFIRMED');
      expect(result.verdicts).toHaveLength(5);
    }
  });
});

// ─── WALK mode — full pipeline with a populated diff: sensors → rule engine → judges → escalation
// (near-certain R3 only — never an authored security scenario) → final-judge reinstatement → fold ───
describe('WaveWalker — WALK mode, populated pipeline', () => {
  // Owner.a: producer only → R1-1 (orphan producer, judged FALSE, NOT escalatable → reinstated by final judge).
  // Owner.b: producer(object) + production consumer with a double-encode decodeExpr → R3-2 (NEAR-CERTAIN;
  // judged FALSE → second-opinion override to CONFIRMED).
  const AUTH =
    'Role fences (founder-ruled, SACRED — reads AND writes): THERAPIST is fenced to OWNERSHIP — never another therapist. SUPERVISOR is clinic-wide inside their OWN clinic ONLY.';
  const scoutOut = {
    headSha: 'abc123',
    territories: ['BE', 'FE', 'Cortex'],
    changedFiles: ['app-be/src/a.ts', 'app-fe/app/b.tsx'],
    changedFileCount: 2,
    mergeShas: ['abc123'],
    threads: [{ id: 't1', type: 'flow', name: 'Flow', verify: 'v' }],
    operations: [],
    fields: [
      { id: 'Owner.a', ownerType: 'Owner', field: 'a' },
      {
        id: 'Owner.b',
        ownerType: 'Owner',
        field: 'b',
        sdl: { anchor: 's:1', typeToken: 'String' },
      },
    ],
    jobs: [
      {
        jobId: 'p1',
        kind: 'producer',
        files: ['app-be/src/a.ts'],
        fieldIds: ['Owner.a', 'Owner.b'],
      },
      { jobId: 'f1', kind: 'consumer', files: ['app-fe/app/b.tsx'], fieldIds: ['Owner.b'] },
      {
        jobId: 'x1',
        kind: 'cortex',
        files: ['app-cortex/src/c.py'],
        fieldIds: ['Owner.b'],
        hint: 'h',
      },
    ],
    gateFiles: ['app-be/src/infrastructure/graphql/resolvers/r.resolvers.ts'],
    authRule: AUTH,
  };

  // retryAgent respawns a dead seat once under a '[label-retry] RESUME: …' prompt — dispatch on the
  // NORMALIZED label (retry suffix stripped) so a deliberate kill stays dead across both attempts.
  const labelOf = (prompt: string): string =>
    (/^\[(.+?)\] /.exec(prompt)?.[1] ?? '').replace(/-retry$/, '');

  function walkStub(
    calls: CapturedCall[],
    overrides?: { authRule?: string; finalJudgeDies?: boolean; scout?: Record<string, unknown> },
  ) {
    return async (rawPrompt: string, callOpts: unknown) => {
      calls.push({ prompt: rawPrompt, opts: callOpts as CapturedCall['opts'] });
      const prompt = '[' + labelOf(rawPrompt) + '] ' + rawPrompt.slice(rawPrompt.indexOf('] ') + 2);
      if (prompt.startsWith('[scout]'))
        return {
          ...scoutOut,
          ...(overrides?.scout || {}),
          authRule: overrides?.authRule ?? scoutOut.authRule,
        };
      if (prompt.startsWith('[walk · '))
        return {
          threadId: 't1',
          flow: 'AT-RISK',
          trace: 'a → b',
          defects: [{ what: 'w', location: 'l:1' }],
          hygiene: [{ kind: 'DUP', where: 'l:2', detail: 'd' }],
        };
      if (prompt.startsWith('[producer · '))
        return {
          jobId: 'p1',
          slices: [
            { fieldId: 'Owner.a', producer: { anchor: 'be:1', encoding: 'raw' }, notes: 'n1' },
            {
              fieldId: 'Owner.b',
              producer: { anchor: 'be:2', encoding: 'object', writer: 'cortex' },
              dbColumn: { anchor: 'db:1' },
              resolver: { anchor: 'rv:1' },
            },
          ],
        };
      if (prompt.startsWith('[consumer · '))
        return {
          jobId: 'f1',
          slices: [
            {
              fieldId: 'Owner.b',
              feSelection: { anchor: 'q:1', queryName: 'GetOwner' },
              feTypes: [{ anchor: 'gen:1', typeToken: 'string', kind: 'generated' }],
              consumers: [
                {
                  anchor: 'fe:1',
                  decodeExpr: 'JSON.parse(JSON.stringify(x))',
                  context: 'production',
                },
              ],
              notes: 'n2',
            },
          ],
          undeclaredReads: [],
        };
      if (prompt.startsWith('[cortex · ')) return { jobId: 'x1', slices: [{ fieldId: 'Owner.b' }] };
      if (prompt.startsWith('[gates · ')) return { file: scoutOut.gateFiles[0], gates: [] };
      if (prompt.startsWith('[security · '))
        return {
          findings: [],
          categoriesSwept: ['8A'],
          filesOpened: ['app-be/src/a.ts'],
          summary: '',
        };
      if (prompt.startsWith('[judge · ')) {
        const ids = [...prompt.matchAll(/"id":"(R\d-\d+)"/g)].map((m) => m[1]);
        return {
          verdicts: [...new Set(ids)].map((anomalyId) => ({
            anomalyId,
            verdict: 'FALSE',
            severity: 'med',
            what: 'killed',
            why: 'looks fine',
          })),
        };
      }
      if (prompt.startsWith('[2nd-opinion')) {
        const ids = [...prompt.matchAll(/"anomalyId":"(R\d-\d+)"/g)].map((m) => m[1]);
        return {
          verdicts: [...new Set(ids)].map((anomalyId) => ({
            anomalyId,
            verdict: 'CONFIRMED',
            severity: 'high',
            what: 'real',
            why: 'verbatim expression present',
          })),
        };
      }
      if (prompt.startsWith('[digest · '))
        return {
          territory: 'BE',
          findings: [{ lens: 'QUALITY', severity: 'low', what: 'w', location: 'l:3' }],
          summary: 's',
        };
      if (prompt.startsWith('[final-judge]')) {
        if (overrides?.finalJudgeDies) return null;
        const killed = [
          ...(prompt.split('KILLED as FALSE')[1] || '')
            .split('· Territory digests')[0]
            .matchAll(/"anomalyId":"(R\d-\d+)"/g),
        ].map((m) => m[1]);
        return {
          verdict: 'ROUGH SEAS',
          reinstated: [...new Set(killed)].map((anomalyId) => ({
            anomalyId,
            why: 'kill reasoning does not hold',
          })),
          missedRisks: [{ what: 'mr', where: 'w:1' }],
          rationale: 'because',
        };
      }
      if (prompt.startsWith('[fold]'))
        return { verdict: 'ROUGH SEAS', actionItems: ['/jc fix w'], review: '# review' };
      if (rawPrompt.startsWith('[clock · ')) return { epochSeconds: 1_000_000 };
      throw new Error('unexpected agent call in walk stub: ' + rawPrompt.slice(0, 40));
    };
  }

  it('runs sensors → rule engine → judges → near-certain escalation override → reinstatement → fold', async () => {
    const calls: CapturedCall[] = [];
    const result = await runVerifyWith(
      { reportPath: 'docs/dev/waves/w/report.md' },
      calls,
      walkStub(calls),
    );
    expect(result.status).toBe('DONE');
    if (result.status !== 'DONE' || !('ledger' in result)) throw new Error('expected walk result');
    // rule engine (zero tokens): R1-1 on Owner.a, R3-2 on Owner.b — global aseq numbering
    expect(result.anomaliesByRule).toEqual({ R1: 1, R3: 1 });
    // judge killed both; R3 (near-certain) escalated to second opinion → CONFIRMED override
    expect(result.overrides).toBe(1);
    expect(calls.some((c) => c.prompt.startsWith('[2nd-opinion'))).toBe(true);
    // R1 stayed killed → final judge reinstated it
    expect(result.finalJudge).toEqual({ verdict: 'ROUGH SEAS', missedRisks: 1, reinstated: 1 });
    expect(result.confirmed).toHaveLength(2);
    expect(result.killedAsFalse).toBe(0);
    // fold adopted; ledger carries the full walk state
    expect(result.verdict).toBe('ROUGH SEAS');
    expect(result.actionItems).toEqual(['/jc fix w']);
    expect(result.ledger.cards).toHaveLength(2);
    expect(result.ledger.walks).toHaveLength(1);
    expect(result.threadsWalked).toBe(1);
    expect(result.digestFindings).toBe(3); // BE + FE + Cortex digests (stub returns 1 finding each)
    expect(result.unsensedFields).toEqual([]);
    // the scout-extracted auth rule (not the fallback) rode into the R6 rule meaning surface — checked
    // here only via the ledger's coverage string being well-formed; no fence scenarios are authored.
    expect(result.coverage).toContain('threads walked: 1/1');
    // D2 — the result's security block carries the sweep's own denominator, and the unopened changed
    // file is NAMED unswept (never folded into a clean sweep).
    expect(result.security).toEqual({
      findings: [],
      categoriesSwept: ['8A'],
      summary: '',
      auditorsDispatched: 1,
      auditorsReturned: 1,
      filesOpened: 1,
      filesInScope: 2,
      filesUnswept: ['app-fe/app/b.tsx'],
    });
    expect(result.coverage).toContain('UNSWEPT: app-fe/app/b.tsx');
  });

  it('splits oversized jobs, enforces the sensor cap (dropped fields reported UNSENSED), and runs on the configured auth-rule fallback when the scout extract is unusable', async () => {
    const calls: CapturedCall[] = [];
    const logs: string[] = [];
    vi.resetModules();
    globalThis.args = { reportPath: 'r.md', maxFieldsPerJob: 1, maxSensors: 2 };
    globalThis.log = (m?: unknown) => logs.push(String(m));
    globalThis.phase = () => {};
    // The judge LIVES here: this test's subject is job splitting, the sensor cap, and the auth-rule
    // fallback, all of which need the walk to reach DONE. A dead judge now FAILS the walk by design
    // (engine.ts § DEAD-JUDGE GUARD) and has its own test below.
    globalThis.agent = walkStub(calls, { authRule: 'too short' });
    globalThis.parallel = async <T>(thunks: Array<() => Promise<T>>): Promise<T[]> =>
      Promise.all(thunks.map((t) => t()));
    globalThis.budget = { total: null, spent: () => 0, remaining: () => Infinity };
    const { WaveWalker } = await import('../src/engine.js');
    const result = await new WaveWalker().run();
    expect(result.status).toBe('DONE');
    if (result.status !== 'DONE' || !('ledger' in result)) throw new Error('expected walk result');
    // maxFieldsPerJob=1 splits p1 (2 fieldIds) into p1-1/p1-2 → 4 jobs; no args.project → gates SKIPPED
    // (gateFiles forced []), keep = maxSensors(2) - 0 → 2 jobs kept, the rest drop, their fields reported
    // UNSENSED (Owner.b is carried by more than one dropped job, so it is named regardless of which
    // specific jobs survive the cap).
    expect(logs.some((l) => l.includes('sensor cap 2: DROPPED'))).toBe(true);
    expect(result.unsensedFields).toContain('Owner.b');
    // unusable scout authRule + no args.project → loud fallback warning naming the missing profile
    expect(logs.some((l) => l.includes('no usable auth-pattern extract'))).toBe(true);
    expect(logs.some((l) => l.includes('no args.project.authDoc configured'))).toBe(true);
    // the judge ruled, so the walk reaches a verdict at all — the dead-judge path is FAILED by design
    // now (§ DEAD-JUDGE GUARD) and is pinned separately below, never incidentally from this fixture
    expect(result.finalJudge).not.toBeNull();
    expect(result.verdict).toBe('ROUGH SEAS');
  });

  it('returns FAILED when the fold dies (never a silent success)', async () => {
    const calls: CapturedCall[] = [];
    const base = walkStub(calls);
    const stub = async (prompt: string, callOpts: unknown) =>
      labelOf(prompt) === 'fold' ? null : base(prompt, callOpts);
    const result = await runVerifyWith({ reportPath: 'r.md' }, calls, stub);
    expect(result.status).toBe('FAILED');
    if (result.status === 'FAILED') expect(result.detail).toMatch(/fold died twice/);
  });

  // ─── DEAD-JUDGE GUARD + TIME CHECKPOINT — the final judge is the ONE seat that rules the whole wave,
  // and it runs LAST, so it is the seat the runtime window starves first. Two laws are pinned here: a
  // walk that lost its judge must FAIL rather than fold a verdict without one, and a walk that sees its
  // window running short must shed discretionary lenses LOUDLY — as a coverage gap the judge is told
  // about — rather than spend the reserve and lose the ruling. ───
  it('returns FAILED when the final judge dies — an unjudged walk is never a verdict', async () => {
    const calls: CapturedCall[] = [];
    const base = walkStub(calls);
    const stub = async (prompt: string, callOpts: unknown) =>
      labelOf(prompt) === 'final-judge' ? null : base(prompt, callOpts);
    const result = await runVerifyWith({ reportPath: 'r.md' }, calls, stub);
    expect(result.status).toBe('FAILED');
    if (result.status !== 'FAILED') throw new Error('expected a FAILED walk');
    expect(result.detail).toMatch(/final judge died/);
    expect(result.detail).toMatch(/An unjudged walk is never a verdict/);
    // the failure carries its accounting — a walk that did real work says how much of it survived
    expect(result.detail).toMatch(/completed \d+\/\d+ threads and \d+ ledger anomalies/);
    // and it never reached the fold: no review is written over an unruled wave
    expect(calls.some((c) => c.opts.label === 'fold')).toBe(false);
  });

  it('sheds discretionary lenses when the window runs short, and NAMES the shed to the final judge', async () => {
    const calls: CapturedCall[] = [];
    const base = walkStub(calls);
    let clockReadings = 0;
    // t0 = 1_000_000; checkpoint = +520s → 80s left of a 600s window, under the 150s judge reserve.
    const stub = async (prompt: string, callOpts: unknown) => {
      if (labelOf(prompt).startsWith('clock · ')) {
        clockReadings++;
        return { epochSeconds: clockReadings === 1 ? 1_000_000 : 1_000_520 };
      }
      return base(prompt, callOpts);
    };
    const result = await runVerifyWith({ reportPath: 'r.md' }, calls, stub);
    expect(clockReadings).toBe(2);
    // the discretionary seat never ran...
    expect(calls.some((c) => c.opts.label === 'digest')).toBe(false);
    // ...but the judge DID, and was told the walk had been narrowed — a shed lens is a named hole,
    // never a silent omission.
    const judge = calls.find((c) => c.opts.label === 'final-judge');
    expect(judge).toBeDefined();
    expect(judge?.prompt).toMatch(/SHED/);
    expect(judge?.prompt).toMatch(/NARROWER than a full walk/);
    expect(result.status).toBe('DONE');
  });

  it('a clock reading that fails sheds NOTHING — an unmeasurable window never costs a lens', async () => {
    const calls: CapturedCall[] = [];
    const base = walkStub(calls);
    const stub = async (prompt: string, callOpts: unknown) =>
      labelOf(prompt).startsWith('clock · ') ? null : base(prompt, callOpts);
    const result = await runVerifyWith({ reportPath: 'r.md' }, calls, stub);
    // failing open: the judge is told nothing about a shed that never happened
    const judge = calls.find((c) => c.opts.label === 'final-judge');
    expect(judge).toBeDefined();
    expect(judge?.prompt).not.toMatch(/SHED/);
    expect(result.status).toBe('DONE');
  });

  // ─── AUDIT 2026-07-28 (docs/dev/audits/wave-walker-instrument-defects-2026-07-28.md) — engine-level
  // pins for D1 (file-set reconciliation) and D2 (security fan-out). Failure class, not instance: any
  // future scout that enumerates fewer files than git printed, and any security sweep whose headline
  // hides its denominator, must fail HERE before it can lie to a walk again. ───
  describe('audit 2026-07-28 — D1 reconciliation + D2 security fan-out', () => {
    it('D1: a mismatched first scout gets ONE corrective retry naming both numbers; a consistent retry walks to DONE', async () => {
      const calls: CapturedCall[] = [];
      const base = walkStub(calls);
      let scoutCalls = 0;
      const stub = async (prompt: string, callOpts: unknown) => {
        if (labelOf(prompt) === 'scout') {
          scoutCalls++;
          if (scoutCalls === 1) {
            calls.push({ prompt, opts: callOpts as CapturedCall['opts'] });
            return { ...scoutOut, changedFileCount: 3 }; // enumerated 2, git says 3
          }
        }
        return base(prompt, callOpts);
      };
      const result = await runVerifyWith({ reportPath: 'r.md' }, calls, stub);
      expect(result.status).toBe('DONE');
      expect(scoutCalls).toBe(2);
      const retryCall = calls.filter((c) => labelOf(c.prompt) === 'scout')[1];
      expect(retryCall.prompt).toContain(
        'RECONCILIATION RETRY: your previous pass enumerated 2 changedFiles but its separately-executed git count said 3.',
      );
    });

    it('D1: a twice-mismatched scout FAILS the walk naming both numbers — never a verdict over an untrusted denominator', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md' },
        calls,
        walkStub(calls, { scout: { changedFileCount: 5 } }),
      );
      expect(result.status).toBe('FAILED');
      if (result.status === 'FAILED') {
        expect(result.detail).toContain('reconciliation FAILED twice');
        expect(result.detail).toContain('enumerated 2 file(s)');
        expect(result.detail).toContain('says 5');
      }
      expect(calls.filter((c) => labelOf(c.prompt) === 'scout')).toHaveLength(2);
    });

    it('D1: a scout with NO changedFileCount (the pre-feature shape) is mismatched by definition', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md' },
        calls,
        walkStub(calls, { scout: { changedFileCount: undefined } }),
      );
      expect(result.status).toBe('FAILED');
    });

    it('D2: the security lens fans out one auditor per file slice and the barrier slice-math stays aligned', async () => {
      const files = Array.from(
        { length: 30 },
        (_, i) => 'app-be/src/domain/f' + String(i).padStart(2, '0') + '.ts',
      );
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md', securityFilesPerAuditor: 12 },
        calls,
        walkStub(calls, {
          scout: { changedFiles: files, changedFileCount: 30, fields: [], jobs: [], gateFiles: [] },
        }),
      );
      expect(result.status).toBe('DONE');
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      const secLabels = calls
        .map((c) => labelOf(c.prompt))
        .filter((l) => l.startsWith('security · '));
      expect(secLabels).toEqual([
        'security · 8A-8K s1/3',
        'security · 8A-8K s2/3',
        'security · 8A-8K s3/3',
      ]);
      expect(result.security?.auditorsDispatched).toBe(3);
      expect(result.security?.auditorsReturned).toBe(3);
      expect(result.security?.filesInScope).toBe(30);
      expect(result.threadsWalked).toBe(1); // thread results still land in their slots around the widened security band
    });

    it('D2: ALL slice auditors dead → security null, coverage says AUDIT DIED — a dead sweep is a hole, never a pass', async () => {
      const calls: CapturedCall[] = [];
      const base = walkStub(calls);
      const stub = async (prompt: string, callOpts: unknown) =>
        labelOf(prompt).startsWith('security · ') ? null : base(prompt, callOpts);
      const result = await runVerifyWith({ reportPath: 'r.md' }, calls, stub);
      expect(result.status).toBe('DONE');
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(result.security).toBeNull();
      expect(result.coverage).toContain('AUDIT DIED');
    });

    it('D2 merge semantics: category INTERSECTION, filesOpened dedup, unswept naming, per-slice id prefixes; D1 misreconciled pins', async () => {
      const { mergeSecurityResults, misreconciled } = await import('../src/engine.js');
      expect(mergeSecurityResults([], 3, ['a.ts'])).toBeNull(); // all dead → null, the AUDIT DIED path
      const merged = mergeSecurityResults(
        [
          {
            findings: [{ id: 'SEC-1', category: '8C', severity: 'high', what: 'w', location: 'l' }],
            categoriesSwept: ['8A', '8C'],
            filesOpened: ['a.ts', 'b.ts'],
            summary: 's1',
          },
          {
            findings: [
              { id: 'SEC-1', category: '8F', severity: 'med', what: 'w2', location: 'l2' },
            ],
            categoriesSwept: ['8C', '8F'],
            filesOpened: ['b.ts', 'c.ts'],
            summary: 's2',
          },
        ] as never,
        3,
        ['a.ts', 'b.ts', 'c.ts', 'd.ts'],
      );
      expect(merged).not.toBeNull();
      expect(merged!.findings.map((f) => f.id)).toEqual(['s1·SEC-1', 's2·SEC-1']); // two slices' SEC-1 never collide
      expect(merged!.categoriesSwept).toEqual(['8C']); // intersection — the only claim true of the WHOLE diff
      expect([...merged!.filesOpened].sort()).toEqual(['a.ts', 'b.ts', 'c.ts']);
      expect(merged!.filesUnswept).toEqual(['d.ts']); // the never-opened file is a NAMED hole
      expect(merged!.auditorsDispatched).toBe(3);
      expect(merged!.auditorsReturned).toBe(2); // partial death keeps the survivors
      expect(merged!.summary).toBe('s1 | s2');
      expect(misreconciled({ changedFiles: ['a'], changedFileCount: 1 } as never)).toBe(false);
      expect(misreconciled({ changedFiles: ['a'], changedFileCount: 2 } as never)).toBe(true);
      expect(misreconciled({ changedFiles: ['a'] } as never)).toBe(true); // absent count = untrusted
    });
  });

  // ─── E3 gate-conditional dispatch (v3 variant) — the repo-wide gate sweep spawns only on
  // gate-relevant diffs; the skip is LOUD; the security auditor ALWAYS runs; fullGateSweep forces. ───
  describe('E3 — gate-conditional dispatch', () => {
    const GATE_FREE_SCOUT = {
      fields: [],
      jobs: [],
      changedFiles: ['app-fe/app/screen.tsx', 'docs/notes.md'],
      territories: ['FE'],
    };
    // GATE ARMING (universal-bundle refactor) — these tests exercise the diff-scoped SKIP classifier
    // itself, which only runs once the gate machinery is armed (see engine.ts computeGateArming); an
    // absent args.project would report 'no project profile supplied' instead and never reach the
    // diff-scoped classifier this describe block is about.
    const TEST_PROJECT = {
      roles: { owner: 'OWNER', elevated: 'ELEVATED' },
      fencedResourceClasses: ['alpha', 'beta', 'gamma'],
      gateResolverPattern: 'app-be/src/infrastructure/graphql/resolvers/',
      gateSurfacePattern: 'app-be/src/(infrastructure/(auth|graphql)|application)/',
    };

    async function runWalkCapturingLogs(
      args: Record<string, unknown>,
      stub: (p: string, o: unknown) => Promise<unknown>,
      logs: string[],
    ) {
      vi.resetModules();
      globalThis.args = args;
      globalThis.log = (m?: unknown) => logs.push(String(m));
      globalThis.phase = () => {};
      globalThis.agent = stub;
      globalThis.parallel = async <T>(thunks: Array<() => Promise<T>>): Promise<T[]> =>
        Promise.all(thunks.map((t) => t()));
      globalThis.budget = { total: null, spent: () => 0, remaining: () => Infinity };
      const { WaveWalker } = await import('../src/engine.js');
      return new WaveWalker().run();
    }

    it('SKIPS the gate sweep on a gate-free diff — loudly, with the security auditor still running', async () => {
      const calls: CapturedCall[] = [];
      const logs: string[] = [];
      const result = await runWalkCapturingLogs(
        { reportPath: 'r.md', project: TEST_PROJECT },
        walkStub(calls, { scout: GATE_FREE_SCOUT }),
        logs,
      );
      expect(result.status).toBe('DONE');
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(calls.some((c) => labelOf(c.prompt).startsWith('gates · '))).toBe(false); // sweep skipped
      expect(calls.some((c) => labelOf(c.prompt).startsWith('security · '))).toBe(true); // auditor UNCONDITIONAL
      expect(
        logs.some((l) =>
          l.includes(
            'gate sweep SKIPPED (diff touches no resolver/auth/service surface; fullGateSweep to force) — R6/R7 not evaluated this walk',
          ),
        ),
      ).toBe(true);
      expect(result.coverage).toContain('· gates: SKIPPED (diff-scoped) ·'); // never a silent narrowing
      expect(result.ledger.gateCards).toEqual([]);
    });

    it('FAIL-SAFE: a buried gate-relevant service file in the diff runs the FULL sweep (fields/jobs empty)', async () => {
      const calls: CapturedCall[] = [];
      const logs: string[] = [];
      const result = await runWalkCapturingLogs(
        { reportPath: 'r.md', project: TEST_PROJECT },
        walkStub(calls, {
          scout: {
            ...GATE_FREE_SCOUT,
            changedFiles: ['app-be/src/application/services/session.service.ts'],
            changedFileCount: 1,
          },
        }),
        logs,
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(calls.some((c) => labelOf(c.prompt).startsWith('gates · '))).toBe(true); // full sweep ran
      expect(logs.some((l) => l.includes('gate sweep SKIPPED'))).toBe(false);
      expect(result.coverage).toContain('· gates: 0 ·'); // sweep ran (stub returns zero cards), not SKIPPED
    });

    it('scheduled GraphQL fields/jobs alone make the diff gate-relevant (sweep runs)', async () => {
      const calls: CapturedCall[] = [];
      const logs: string[] = [];
      await runWalkCapturingLogs(
        { reportPath: 'r.md', project: TEST_PROJECT },
        walkStub(calls),
        logs,
      ); // full scoutOut: fields+jobs present
      expect(calls.some((c) => labelOf(c.prompt).startsWith('gates · '))).toBe(true);
      expect(logs.some((l) => l.includes('gate sweep SKIPPED'))).toBe(false);
    });

    it('args.fullGateSweep:true forces the full sweep on a gate-free diff', async () => {
      const calls: CapturedCall[] = [];
      const logs: string[] = [];
      await runWalkCapturingLogs(
        { reportPath: 'r.md', fullGateSweep: true, project: TEST_PROJECT },
        walkStub(calls, { scout: GATE_FREE_SCOUT }),
        logs,
      );
      expect(calls.some((c) => labelOf(c.prompt).startsWith('gates · '))).toBe(true);
      expect(logs.some((l) => l.includes('gate sweep SKIPPED'))).toBe(false);
    });

    it('isGateRelevant — the v3 classifier verbatim: resolver/auth/graphql-infra/application paths (per the CALLER-supplied patterns), or any scheduled fields/jobs', async () => {
      const { isGateRelevant } = await import('../src/engine.js');
      const patterns = {
        resolver: /app-be\/src\/infrastructure\/graphql\/resolvers\//,
        surface: /app-be\/src\/(infrastructure\/(auth|graphql)|application)\//,
      };
      expect(
        isGateRelevant(
          ['app-be/src/infrastructure/graphql/resolvers/session.resolvers.ts'],
          0,
          0,
          patterns,
        ),
      ).toBe(true);
      expect(isGateRelevant(['app-be/src/infrastructure/auth/jwt.ts'], 0, 0, patterns)).toBe(true);
      expect(isGateRelevant(['app-be/src/infrastructure/graphql/schema.ts'], 0, 0, patterns)).toBe(
        true,
      );
      expect(
        isGateRelevant(['app-be/src/application/services/patient.service.ts'], 0, 0, patterns),
      ).toBe(true);
      expect(isGateRelevant(['app-fe/app/screen.tsx'], 1, 0, patterns)).toBe(true); // scheduled fields
      expect(isGateRelevant(['app-fe/app/screen.tsx'], 0, 1, patterns)).toBe(true); // scheduled jobs
      expect(
        isGateRelevant(
          ['app-fe/app/screen.tsx', 'docs/x.md', 'app-be/src/domain/session.ts'],
          0,
          0,
          patterns,
        ),
      ).toBe(false);
      expect(isGateRelevant([], 0, 0, patterns)).toBe(false);
    });
  });

  it('charter rides EXACTLY four seats: scout, thread-walker, territory digest, final judge — and nowhere else', async () => {
    const calls: CapturedCall[] = [];
    const CHARTER = 'confirm the export path never touches PHI columns';
    await runVerifyWith({ reportPath: 'r.md', charter: CHARTER }, calls, walkStub(calls));
    const withCharter = calls.filter((c) =>
      c.prompt.includes('WALK CHARTER (caller-supplied duty): ' + CHARTER),
    );
    const seats = new Set(withCharter.map((c) => labelOf(c.prompt).split(' · ')[0]));
    expect(seats).toEqual(new Set(['scout', 'walk', 'digest', 'final-judge']));
    for (const c of calls) {
      const seat = labelOf(c.prompt).split(' · ')[0];
      if (!['scout', 'walk', 'digest', 'final-judge'].includes(seat))
        expect(c.prompt).not.toContain('WALK CHARTER');
    }
  });

  // ─── PROJECT PROFILE (universal-bundle refactor) — the gate machinery (sweep scheduling, R6, R7,
  // fence rules) arms strictly from args.project; a missing/incomplete/unparseable profile disarms it
  // LOUD, never silent (CLAUDE.md "error never renders as absence"). Regression pins for the three floor
  // behaviors: absence, a working armed profile, and an unparseable pattern. ───
  describe('PROJECT PROFILE — gate-machinery arming (universal-bundle refactor)', () => {
    const TEST_PROJECT = {
      roles: { owner: 'OWNER', elevated: 'ELEVATED' },
      fencedResourceClasses: ['alpha'],
      gateResolverPattern: 'app-be/src/infrastructure/graphql/resolvers/',
      gateSurfacePattern: 'app-be/src/(infrastructure/(auth|graphql)|application)/',
    };

    it('(a) no args.project: the gates-SKIPPED marker rides the result AND telemetry, and no R6/R7 anomaly is ever produced', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith({ reportPath: 'r.md' }, calls, walkStub(calls));
      expect(result.status).toBe('DONE');
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(result.coverage).toContain('gates: SKIPPED — no project profile supplied');
      expect(result.debugRecord?.coverage.gateSweepSkipped).toBe(true);
      expect(result.debugRecord?.coverage.gateSweepSkipDetail).toBe(
        'SKIPPED — no project profile supplied',
      );
      expect(result.ledger.anomalies.some((a) => a.rule === 'R6' || a.rule === 'R7')).toBe(false);
      expect(calls.some((c) => labelOf(c.prompt).startsWith('gates · '))).toBe(false);
    });

    it('(b) a valid armed profile (roles + fencedResourceClasses + valid patterns): R6 fires on a crafted mandated-fence-violation gate card', async () => {
      const calls: CapturedCall[] = [];
      const gateFile = 'app-be/src/infrastructure/graphql/resolvers/alpha.resolvers.ts';
      const stub = async (rawPrompt: string, callOpts: unknown) => {
        calls.push({ prompt: rawPrompt, opts: callOpts as CapturedCall['opts'] });
        const prompt =
          '[' + labelOf(rawPrompt) + '] ' + rawPrompt.slice(rawPrompt.indexOf('] ') + 2);
        if (prompt.startsWith('[scout]'))
          return {
            headSha: 'abc',
            territories: [],
            changedFiles: [gateFile],
            changedFileCount: 1,
            mergeShas: ['abc'],
            threads: [{ id: 't1', type: 'flow', name: 'Flow', verify: 'v' }],
            operations: [],
            fields: [],
            jobs: [],
            gateFiles: [gateFile],
            authRule: '',
          };
        if (prompt.startsWith('[walk · '))
          return { threadId: 't1', flow: 'INTACT', trace: 'a', defects: [], hygiene: [] };
        if (prompt.startsWith('[gates · '))
          return {
            file: gateFile,
            gates: [
              {
                id: 'Query.alpha',
                anchor: 'a:1',
                resource: 'alpha',
                idArgs: ['alphaId'],
                ownershipFence: false,
                rolesAllowed: ['OWNER'],
              },
            ],
          };
        if (prompt.startsWith('[security · '))
          return { findings: [], categoriesSwept: ['8A'], filesOpened: [gateFile], summary: '' };
        if (prompt.startsWith('[judge · ')) {
          const ids = [...prompt.matchAll(/"id":"(R\d-\d+)"/g)].map((m) => m[1]);
          return {
            verdicts: [...new Set(ids)].map((anomalyId) => ({
              anomalyId,
              verdict: 'CONFIRMED',
              severity: 'critical',
              what: 'mandated fence missing',
            })),
          };
        }
        if (prompt.startsWith('[final-judge]'))
          return { verdict: 'SHIPWRECK', missedRisks: [], rationale: 'r' };
        if (prompt.startsWith('[fold]'))
          return { verdict: 'SHIPWRECK', actionItems: [], review: '# review' };
        if (rawPrompt.startsWith('[clock · ')) return { epochSeconds: 1_000_000 };
        throw new Error('unexpected agent call in R6-pin stub: ' + rawPrompt.slice(0, 60));
      };
      const result = await runVerifyWith(
        { reportPath: 'r.md', project: TEST_PROJECT },
        calls,
        stub,
      );
      expect(result.status).toBe('DONE');
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(
        result.ledger.anomalies.some(
          (a) => a.rule === 'R6' && a.ruleName === 'mandated-fence violation',
        ),
      ).toBe(true);
      expect(result.coverage).not.toContain('SKIPPED — no project profile supplied');
      expect(calls.some((c) => labelOf(c.prompt).startsWith('gates · '))).toBe(true);
    });

    it('(c) an invalid gateResolverPattern regex source: SKIPPED with the compile error named — never a throw', async () => {
      const calls: CapturedCall[] = [];
      const badProject = { ...TEST_PROJECT, gateResolverPattern: '(unterminated[' };
      const result = await runVerifyWith(
        { reportPath: 'r.md', project: badProject },
        calls,
        walkStub(calls),
      );
      expect(result.status).toBe('DONE');
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(result.coverage).toContain('gates: SKIPPED — invalid gate pattern:');
      expect(calls.some((c) => labelOf(c.prompt).startsWith('gates · '))).toBe(false);
      expect(result.ledger.anomalies.some((a) => a.rule === 'R6' || a.rule === 'R7')).toBe(false);
    });
  });
});

// ─── INVESTIGATE mode — brainer loop, claim auditor, computed confidence flooring, loud degradation ───
describe('WaveWalker — INVESTIGATE mode, brainer-steered loop', () => {
  const labelOf = (prompt: string): string =>
    (/^\[(.+?)\] /.exec(prompt)?.[1] ?? '').replace(/-retry$/, '');

  function investigateStub(
    calls: CapturedCall[],
    opts?: {
      brainerDies?: boolean;
      synthDies?: boolean;
      brainerNoLanes?: boolean;
      emptyClaims?: boolean;
      synthConfidence?: string;
    },
  ) {
    return async (rawPrompt: string, callOpts: unknown) => {
      calls.push({ prompt: rawPrompt, opts: callOpts as CapturedCall['opts'] });
      const prompt = '[' + labelOf(rawPrompt) + '] ' + rawPrompt.slice(rawPrompt.indexOf('] ') + 2);
      if (prompt.startsWith('[probe · '))
        return {
          laneId: 'w0-1',
          claims: opts?.emptyClaims
            ? []
            : [
                {
                  statement: 'X holds',
                  anchors: [
                    { anchor: 'a.ts:1', quote: 'q1' },
                    { anchor: 'b.ts:2', quote: 'q2' },
                  ],
                },
              ],
          leads: [{ what: 'follow up', files: ['c.ts'] }],
        };
      if (prompt.startsWith('[audit · ')) {
        const ids = [...prompt.matchAll(/"id":"(c\d+)"/g)].map((m) => m[1]);
        return { audits: [...new Set(ids)].map((id) => ({ id, result: 'pass' })) };
      }
      if (prompt.startsWith('[brainer · ')) {
        if (opts?.brainerDies) return null;
        return {
          resultSoFar: 'X because Y',
          keyClaimIds: ['c1'],
          lanes: opts?.brainerNoLanes
            ? []
            : [{ id: 'w1-1', kind: 'attack', question: 'attack c1', targets: ['c1'] }],
          dropLeads: ['L1'],
          stop: { done: prompt.includes('Wave 2/') },
        };
      }
      if (prompt.startsWith('[synth]')) {
        if (opts?.synthDies) return null;
        return {
          answer: 'X because Y (final)',
          confidence: opts?.synthConfidence ?? 'low',
          report: '# report',
        };
      }
      throw new Error('unexpected agent call in investigate stub: ' + prompt.slice(0, 40));
    };
  }

  it('runs waves, audits quote-pins, floors synth confidence at the COMPUTED value', async () => {
    const calls: CapturedCall[] = [];
    // wave-0 probes seed the ledger; wave-1 brainer sends an attack lane; the attack probe returns the
    // SAME statement (dedup) with no counters → survival credit; wave-2 brainer stops done.
    const result = await runVerifyWith(
      { goal: 'why X', maxWaves: 3, lenses: ['L1 — one'] },
      calls,
      investigateStub(calls, { synthConfidence: 'high' }),
    );
    expect(result.status).toBe('DONE');
    if (result.status !== 'DONE' || !('goal' in result) || result.mode !== 'investigate')
      throw new Error('expected investigate result');
    expect(result.stopReason).toMatch(/brainer-done/);
    expect(calls.some((c) => c.prompt.startsWith('[audit · '))).toBe(true); // claim auditor ran on pending rows
    // computed: c1 audit=pass, files [a.ts,b.ts] =2, survived 1 (attack found nothing) → settled → 'high'
    expect(result.computedConfidence).toBe('high');
    expect(result.confidence).toBe('high'); // synth said high; computed high — not floored
    expect(result.claims[0]).toMatchObject({
      id: 'c1',
      status: 'settled',
      survived: 1,
      audit: 'pass',
    });
    // L1 dropped by the brainer's dropLeads; the wave-1 probe re-minted the same lead as L2
    expect(result.openLeads).toEqual([{ id: 'L2', what: 'follow up', files: ['c.ts'] }]);
    expect(result.degraded).toBe(false);
    expect(result.report).toBe('# report');
  });

  it('a synth claiming HIGHER than computed is floored to the computed value', async () => {
    const calls: CapturedCall[] = [];
    // maxWaves 0 → loop never runs → coord null → keyIds [] → computed 'low'; synth says 'high' → floored to 'low'
    const result = await runVerifyWith(
      { goal: 'why X', maxWaves: 0, lenses: ['L1 — one'] },
      calls,
      investigateStub(calls, { synthConfidence: 'high' }),
    );
    if (result.status !== 'DONE' || !('goal' in result) || result.mode !== 'investigate')
      throw new Error('expected investigate result');
    expect(result.computedConfidence).toBe('low');
    expect(result.confidence).toBe('low');
    expect(result.stopReason).toBe('wave-cap');
    expect(result.degraded).toBe(true); // no coord
  });

  it('degrades LOUDLY when brainer and synth both die — DEGRADED answer, never nothing', async () => {
    const calls: CapturedCall[] = [];
    const result = await runVerifyWith(
      { goal: 'why X', lenses: ['L1 — one'] },
      calls,
      investigateStub(calls, { brainerDies: true, synthDies: true }),
    );
    if (result.status !== 'DONE' || !('goal' in result) || result.mode !== 'investigate')
      throw new Error('expected investigate result');
    expect(result.stopReason).toBe('brainer-dead');
    expect(result.answer).toMatch(/^DEGRADED: no synthesis and no coord/);
    expect(result.degraded).toBe(true);
    expect(result.report).toBeNull();
  });

  it('stops with no-lanes when the brainer returns zero lanes without done', async () => {
    const calls: CapturedCall[] = [];
    const result = await runVerifyWith(
      { goal: 'why X', lenses: ['L1 — one'] },
      calls,
      investigateStub(calls, { brainerNoLanes: true }),
    );
    if (result.status !== 'DONE' || !('goal' in result) || result.mode !== 'investigate')
      throw new Error('expected investigate result');
    expect(result.stopReason).toBe('no-lanes');
  });

  it('returns FAILED when every wave-0 probe dies', async () => {
    const calls: CapturedCall[] = [];
    const stub = async (prompt: string, callOpts: unknown) => {
      calls.push({ prompt, opts: callOpts as CapturedCall['opts'] });
      return null;
    };
    const result = await runVerifyWith({ goal: 'why X' }, calls, stub);
    expect(result.status).toBe('FAILED');
    if (result.status === 'FAILED') expect(result.detail).toMatch(/wave-0 probes died/);
  });

  it('returns FAILED when live wave-0 probes produce no auditable claims', async () => {
    const calls: CapturedCall[] = [];
    const result = await runVerifyWith(
      { goal: 'why X', lenses: ['L1 — one'] },
      calls,
      investigateStub(calls, { emptyClaims: true }),
    );
    expect(result.status).toBe('FAILED');
    if (result.status === 'FAILED') expect(result.detail).toMatch(/no auditable claims/);
    expect(calls.some((c) => c.prompt.startsWith('[brainer'))).toBe(false);
    expect(calls.some((c) => c.prompt.startsWith('[synth'))).toBe(false);
  });

  it('stops on budget before a wave when remaining() < 80000', async () => {
    const calls: CapturedCall[] = [];
    vi.resetModules();
    globalThis.args = { goal: 'why X', lenses: ['L1 — one'] };
    globalThis.log = () => {};
    globalThis.phase = () => {};
    globalThis.agent = investigateStub(calls);
    globalThis.parallel = async <T>(thunks: Array<() => Promise<T>>): Promise<T[]> =>
      Promise.all(thunks.map((t) => t()));
    globalThis.budget = { total: 100000, spent: () => 50000, remaining: () => 50000 };
    const { WaveWalker } = await import('../src/engine.js');
    const result = await new WaveWalker().run();
    if (result.status !== 'DONE' || !('goal' in result) || result.mode !== 'investigate')
      throw new Error('expected investigate result');
    expect(result.stopReason).toBe('budget');
    expect(calls.some((c) => c.prompt.startsWith('[brainer'))).toBe(false);
  });
});

// ─── INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1-2.4) — the missing piece: end-
// to-end coverage of registry arming (scout semantic + territory-glob fail-safe union), invariantHunter
// dispatch (one per armed invariant, territory-scoped), R9-INV judge flow + escalation symmetry
// (confirmed-direction second opinion, new — mirrors the existing killed-direction one), coverageCritic
// dispatch (gated on a non-empty registry), and the FLOOR (an absent/empty registry ⇒ zero of any of
// this dispatches, byte-identical to today). ───
describe('WaveWalker — INVARIANT REGISTRY FEATURE', () => {
  const labelOf = (prompt: string): string =>
    (/^\[(.+?)\] /.exec(prompt)?.[1] ?? '').replace(/-retry$/, '');

  const ENGINE_REGRADE = {
    id: 'ENGINE-REGRADE',
    law: 'Narrative clinical record is engine-frozen — app-cortex/CLAUDE.md:159',
    territory: ['app-cortex/src/app_cortex/db/analysis/**'],
    triggers: ['diff touches territory'],
    exemplars: ['chunk_cache.py:38 engine-less key'],
    huntBrief: 'walk every reuse/skip/cache gate for CURRENCY, not existence',
  };
  const FROZEN_NARRATIVE = {
    id: 'FROZEN-NARRATIVE',
    law: 'Regeneration must never wipe a column it does not snapshot — app-cortex/CLAUDE.md:159',
    territory: ['app-be/src/infrastructure/persistence/drizzle/analysis/**'],
    triggers: ['diff touches a wipe/TRUNCATE/regeneration path'],
    exemplars: ['regeneration.queries.ts:76 wipe/snapshot mismatch'],
    huntBrief: 'diff every wipe set against its snapshot set member-by-member',
  };

  const CHUNK_CACHE_FILE = 'app-cortex/src/app_cortex/db/analysis/chunk_cache.py';
  const invScoutOut = (overrides?: Record<string, unknown>) => ({
    headSha: 'sha1',
    territories: ['Cortex'],
    changedFiles: [CHUNK_CACHE_FILE],
    changedFileCount: 1,
    mergeShas: ['sha1'],
    threads: [{ id: 't1', type: 'flow', name: 'Flow', verify: 'v' }],
    operations: [],
    fields: [],
    jobs: [],
    gateFiles: [],
    authRule: 'irrelevant here (no gate-fence anomalies authored in this suite)',
    armedInvariants: [{ id: 'ENGINE-REGRADE', matchedFiles: [CHUNK_CACHE_FILE] }],
    ...overrides,
  });

  function invStub(
    calls: CapturedCall[],
    opts?: {
      scout?: Record<string, unknown>;
      findings?: unknown[];
      judgeVerdict?: string;
      judgeSeverity?: string;
      secondVerdict?: string;
      gaps?: unknown[];
      extraJudgeHandler?: (prompt: string) => unknown | undefined;
    },
  ) {
    return async (rawPrompt: string, callOpts: unknown) => {
      calls.push({ prompt: rawPrompt, opts: callOpts as CapturedCall['opts'] });
      const label = labelOf(rawPrompt);
      if (label === 'scout') return invScoutOut(opts?.scout);
      if (label === 'walk · t1')
        return { threadId: 't1', flow: 'INTACT', trace: 't', defects: [], hygiene: [] };
      if (label.startsWith('security'))
        return { findings: [], categoriesSwept: ['8A'], summary: '' };
      if (label.startsWith('invariant-hunt · '))
        return {
          invariantId: label.replace('invariant-hunt · ', ''),
          findings:
            opts?.findings ??
            (label === 'invariant-hunt · ENGINE-REGRADE'
              ? [
                  {
                    what: 'chunk cache key has no engine dimension',
                    location: CHUNK_CACHE_FILE + ':38',
                    expected: 'cache key includes engine version',
                    got: '(session_id, step_name, chunk_hash)',
                    failureScenario:
                      'engine bump -> redelivery replays old-engine slice results stamped as new engine',
                    severity: opts?.judgeSeverity ?? 'high',
                  },
                ]
              : []),
          coverage: 'walked db/analysis/** — nothing skipped',
        };
      if (label.startsWith('judge · R9-INV')) {
        if (opts?.extraJudgeHandler) {
          const r = opts.extraJudgeHandler(rawPrompt);
          if (r !== undefined) return r;
        }
        const ids = [...rawPrompt.matchAll(/"id":"(R9-INV-\d+)"/g)].map((m) => m[1]);
        return {
          verdicts: [...new Set(ids)].map((anomalyId) => ({
            anomalyId,
            verdict: opts?.judgeVerdict ?? 'CONFIRMED',
            severity: opts?.judgeSeverity ?? 'high',
            what: 'engine-less cache key confirmed',
            why: 'walked chunk_cache.py:38 myself',
          })),
        };
      }
      if (label.startsWith('2nd-opinion')) {
        const ids = [...rawPrompt.matchAll(/"anomalyId":"(R9-INV-\d+)"/g)].map((m) => m[1]);
        return {
          verdicts: [...new Set(ids)].map((anomalyId) => ({
            anomalyId,
            verdict: opts?.secondVerdict ?? 'CONFIRMED',
            severity: 'high',
            what: 're-derived independently',
            why: 're-opened the file myself',
          })),
        };
      }
      if (label === 'coverage-critic')
        return {
          gaps: opts?.gaps ?? [{ territory: 'app-fe/**', why: 'no finder walked one FE line' }],
          summary: 'sc',
        };
      if (label === 'final-judge')
        return { verdict: 'ROUGH SEAS', missedRisks: [], rationale: 'r' };
      if (label === 'fold')
        return {
          verdict: 'ROUGH SEAS',
          actionItems: ['/jc fix chunk_cache.py'],
          review: '# review',
        };
      if (rawPrompt.startsWith('[clock · ')) return { epochSeconds: 1_000_000 };
      throw new Error('unexpected call in invStub: ' + rawPrompt.slice(0, 80));
    };
  }

  describe('THE FLOOR — absent/empty registry', () => {
    it('no args.invariants: zero invariant-hunt/coverage-critic calls, even though the scout STUB claims an arm (the registry, not the scout, is the gate)', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith({ reportPath: 'r.md' }, calls, invStub(calls));
      expect(result.status).toBe('DONE');
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(calls.some((c) => labelOf(c.prompt).startsWith('invariant-hunt'))).toBe(false);
      expect(calls.some((c) => labelOf(c.prompt) === 'coverage-critic')).toBe(false);
      expect(result.ledger.armedInvariants).toEqual([]);
      expect(result.ledger.coverageGaps).toEqual([]);
      expect(result.invariantsArmed).toEqual([]);
      expect(result.coverageGaps).toBe(0);
      expect(result.coverage).not.toContain('invariants:');
      // R9-INV never appears in anomaliesByRule when nothing was ever hunted
      expect(Object.keys(result.anomaliesByRule)).not.toContain('R9-INV');
    });
    it('args.invariants: [] is the same floor as omitting it entirely', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md', invariants: [] },
        calls,
        invStub(calls),
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(calls.some((c) => labelOf(c.prompt).startsWith('invariant-hunt'))).toBe(false);
      expect(calls.some((c) => labelOf(c.prompt) === 'coverage-critic')).toBe(false);
    });
  });

  describe('arming — scout semantic judgment UNION territory-glob fail-safe', () => {
    it('scout arms it: one invariant-hunt call dispatched, carrying the full registry entry + matchedFiles', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md', invariants: [ENGINE_REGRADE] },
        calls,
        invStub(calls),
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      const huntCalls = calls.filter((c) => labelOf(c.prompt).startsWith('invariant-hunt'));
      expect(huntCalls).toHaveLength(1);
      expect(huntCalls[0].prompt).toContain('ENGINE-REGRADE');
      expect(huntCalls[0].prompt).toContain(ENGINE_REGRADE.huntBrief);
      expect(huntCalls[0].prompt).toContain(CHUNK_CACHE_FILE);
      expect(result.invariantsArmed).toEqual(['ENGINE-REGRADE']);
      expect(result.ledger.armedInvariants).toEqual([
        {
          id: 'ENGINE-REGRADE',
          matchedFiles: [CHUNK_CACHE_FILE],
          reason: 'scout + territory glob',
        },
      ]);
    });
    it('FAIL-SAFE: scout does NOT arm it, but the diff touches its territory — the hunt STILL dispatches (a scout omission cannot silently disarm a hunt)', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md', invariants: [ENGINE_REGRADE] },
        calls,
        invStub(calls, { scout: { armedInvariants: [] } }), // scout claims nothing armed
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(calls.some((c) => labelOf(c.prompt) === 'invariant-hunt · ENGINE-REGRADE')).toBe(true);
      expect(result.ledger.armedInvariants[0].reason).toBe(
        'territory glob match (fail-safe — scout did not arm)',
      );
    });
    it('a registered invariant whose territory the diff never touches, and the scout never arms, stays UNARMED — no hunt', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md', invariants: [ENGINE_REGRADE, FROZEN_NARRATIVE] },
        calls,
        invStub(calls), // scout only arms ENGINE-REGRADE; FROZEN-NARRATIVE's territory (app-be/...) never appears in changedFiles
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(result.invariantsArmed).toEqual(['ENGINE-REGRADE']);
      expect(calls.some((c) => labelOf(c.prompt) === 'invariant-hunt · FROZEN-NARRATIVE')).toBe(
        false,
      );
    });
  });

  describe('computeArmedInvariants — the zero-token fail-safe primitive, tested directly', () => {
    it('unions scout-armed and territory-glob-matched entries, deduping matchedFiles', async () => {
      const { computeArmedInvariants } = await import('../src/engine.js');
      const out = computeArmedInvariants(
        [ENGINE_REGRADE],
        [CHUNK_CACHE_FILE, 'app-cortex/src/app_cortex/db/analysis/gottman.py'],
        [{ id: 'ENGINE-REGRADE', matchedFiles: [CHUNK_CACHE_FILE] }],
      );
      expect(out).toHaveLength(1);
      expect(out[0].matchedFiles.sort()).toEqual(
        [CHUNK_CACHE_FILE, 'app-cortex/src/app_cortex/db/analysis/gottman.py'].sort(),
      );
      expect(out[0].reason).toBe('scout + territory glob');
    });
    it('an invariant with no scout arm and no territory match is absent from the result entirely', async () => {
      const { computeArmedInvariants } = await import('../src/engine.js');
      const out = computeArmedInvariants([FROZEN_NARRATIVE], [CHUNK_CACHE_FILE], []);
      expect(out).toEqual([]);
    });
    it('empty registry always returns [] regardless of scout claims — the registry is the sole authority', async () => {
      const { computeArmedInvariants } = await import('../src/engine.js');
      const out = computeArmedInvariants(
        [],
        [CHUNK_CACHE_FILE],
        [{ id: 'ENGINE-REGRADE', matchedFiles: [CHUNK_CACHE_FILE] }],
      );
      expect(out).toEqual([]);
    });
  });

  // VERDICT CONTRADICTIONS (walker.md § Orchestration) — the T12-vs-CT5 shape: two seats over one file,
  // opposite verdicts, the clean one built on evidence the file does not contain.
  describe('computeVerdictContradictions — the zero-token contradiction scan, tested directly', () => {
    const PRIV = 'app-web/content/privacy/en.mdx';
    const threads = [
      {
        id: 't12',
        type: 'invariant',
        name: 'COMPLIANCE-CLAIM INTEGRITY',
        verify: 'v',
        files: [PRIV, 'app-web/content/privacy/nl.mdx'],
      },
      {
        id: 'ct5',
        type: 'invariant',
        name: 'Compliance claims substantiated',
        verify: 'v',
        files: [PRIV],
      },
    ];
    const walk = (threadId: string, flow: string, defects: unknown[] = []) =>
      ({ threadId, flow, trace: '', defects, hygiene: [] }) as never;

    it('one file, INTACT from one seat and AT-RISK from another: a NAMED contradiction carrying both seats', async () => {
      const { computeVerdictContradictions } = await import('../src/engine.js');
      const scan = computeVerdictContradictions(threads as never, [
        walk('t12', 'INTACT'),
        walk('ct5', 'AT-RISK', [{ what: 'NL policy is a stub', location: 'nl.mdx:1' }]),
      ]);
      expect(scan.contradictions).toHaveLength(1);
      expect(scan.contradictions[0].file).toBe(PRIV);
      expect(scan.contradictions[0].clean.map((s) => s.threadId)).toEqual(['t12']);
      expect(scan.contradictions[0].flagged.map((s) => s.threadId)).toEqual(['ct5']);
      expect(scan.contradictions[0].flagged[0].defects).toBe(1);
      expect(scan.filesCompared).toBe(1);
    });

    it('two seats AGREEING on one file: zero contradictions, but the scan still reports the file it compared — a zero is a result, never silence', async () => {
      const { computeVerdictContradictions } = await import('../src/engine.js');
      const scan = computeVerdictContradictions(threads as never, [
        walk('t12', 'INTACT'),
        walk('ct5', 'INTACT'),
      ]);
      expect(scan.contradictions).toEqual([]);
      expect(scan.filesCompared).toBe(1);
      expect(scan.uncomparableThreads).toEqual([]);
    });

    it('a walked thread whose spec names no files is UNCOMPARABLE and named — never counted as agreement', async () => {
      const { computeVerdictContradictions } = await import('../src/engine.js');
      const scan = computeVerdictContradictions(
        [{ id: 'tX', type: 'flow', name: 'no files', verify: 'v' }, ...threads] as never,
        [walk('tX', 'INTACT'), walk('t12', 'INTACT'), walk('ct5', 'BROKEN')],
      );
      expect(scan.uncomparableThreads).toEqual(['tX']);
      expect(scan.contradictions).toHaveLength(1);
      expect(scan.contradictions[0].flagged[0].flow).toBe('BROKEN');
    });

    it('N/A abstains — it joins neither side, so an N/A beside a BROKEN verdict is no contradiction', async () => {
      const { computeVerdictContradictions } = await import('../src/engine.js');
      const scan = computeVerdictContradictions(threads as never, [
        walk('t12', 'N/A'),
        walk('ct5', 'BROKEN'),
      ]);
      expect(scan.contradictions).toEqual([]);
      expect(scan.filesCompared).toBe(0); // only ONE seat made a health claim about the file
    });

    it('path spellings converge: `./x` and `x` are the same file, so the pair still collides', async () => {
      const { computeVerdictContradictions } = await import('../src/engine.js');
      const scan = computeVerdictContradictions(
        [
          { id: 'a', type: 'seam', name: 'A', verify: 'v', files: ['./' + PRIV] },
          { id: 'b', type: 'seam', name: 'B', verify: 'v', files: [PRIV] },
        ] as never,
        [walk('a', 'INTACT'), walk('b', 'AT-RISK')],
      );
      expect(scan.contradictions).toHaveLength(1);
      expect(scan.contradictions[0].file).toBe(PRIV);
    });
  });

  describe('contradiction escalation — the walk hands every named contradiction to the final judge', () => {
    const SHARED = 'app-web/content/privacy/en.mdx';
    // per-thread verdicts: t1 clean, t2 flagged over ONE file — the T12/CT5 shape. `agree: true` makes
    // both seats INTACT, the control case.
    const contradictionStub = (calls: CapturedCall[], agree = false) => {
      return async (rawPrompt: string, callOpts: unknown) => {
        calls.push({ prompt: rawPrompt, opts: callOpts as CapturedCall['opts'] });
        const label = labelOf(rawPrompt);
        if (label === 'scout')
          return {
            headSha: 'sha1',
            territories: ['FE'],
            changedFiles: [SHARED],
            changedFileCount: 1,
            mergeShas: ['sha1'],
            threads: [
              {
                id: 't1',
                type: 'invariant',
                name: 'compliance claims',
                verify: 'v',
                files: [SHARED],
              },
              {
                id: 't2',
                type: 'invariant',
                name: 'compliance claims (independent)',
                verify: 'v',
                files: [SHARED],
              },
            ],
            operations: [],
            fields: [],
            jobs: [],
            gateFiles: [],
            authRule: 'irrelevant here',
          };
        if (label.startsWith('walk · ')) {
          const threadId = /"id":"(t\d)"/.exec(rawPrompt)![1];
          return threadId === 't1' || agree
            ? { threadId, flow: 'INTACT', trace: 'read both locales', defects: [], hygiene: [] }
            : {
                threadId,
                flow: 'AT-RISK',
                trace: 'nl is a stub',
                defects: [{ what: 'NL policy deleted', location: SHARED + ':1' }],
                hygiene: [],
              };
        }
        if (label.startsWith('security'))
          return { findings: [], categoriesSwept: ['8A'], summary: '' };
        if (label === 'final-judge')
          return { verdict: 'ROUGH SEAS', missedRisks: [], rationale: 'r' };
        if (label === 'fold')
          return {
            verdict: 'ROUGH SEAS',
            actionItems: ['/jc restore the NL policy'],
            review: '# review',
          };
        if (rawPrompt.startsWith('[clock · ')) return { epochSeconds: 1_000_000 };
        throw new Error('unexpected agent call in contradiction stub: ' + rawPrompt.slice(0, 40));
      };
    };

    it('the final-judge prompt names the contradiction and forbids averaging; the fold and the ledger carry it too', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith({ reportPath: 'r.md' }, calls, contradictionStub(calls));
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      const judge = calls.find((c) => labelOf(c.prompt) === 'final-judge')!;
      expect(judge.prompt).toContain('NAMED CONTRADICTIONS');
      expect(judge.prompt).toContain(SHARED);
      expect(judge.prompt).toMatch(
        /averaging, merging, or letting the more optimistic verdict stand is forbidden/,
      );
      const fold = calls.find((c) => labelOf(c.prompt) === 'fold')!;
      expect(fold.prompt).toContain('NAMED CONTRADICTIONS');
      expect(result.verdictContradictions).toBe(1);
      expect(result.ledger.contradictions).toHaveLength(1);
      expect(result.ledger.contradictions[0].clean.map((s) => s.threadId)).toEqual(['t1']);
      expect(result.coverage).toContain('verdict contradictions: 1');
    });

    it('no contradiction: the final-judge and fold prompts stay byte-clean of the block, and Coverage still states the scan ran', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md' },
        calls,
        contradictionStub(calls, true),
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(calls.find((c) => labelOf(c.prompt) === 'final-judge')!.prompt).not.toContain(
        'NAMED CONTRADICTIONS',
      );
      expect(calls.find((c) => labelOf(c.prompt) === 'fold')!.prompt).not.toContain(
        'NAMED CONTRADICTIONS',
      );
      expect(result.verdictContradictions).toBe(0);
      expect(result.coverage).toContain('verdict contradictions: 0');
    });
  });

  describe('R9-INV judge flow — hunter finding -> anomaly -> judge (REFUTE-FIRST) -> escalation symmetry', () => {
    it('a hunter finding becomes an R9-INV anomaly, judged with the R9-INV refute-first block, confirmed high escalates to a CONFIRMED-direction second opinion', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md', invariants: [ENGINE_REGRADE] },
        calls,
        invStub(calls),
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(result.anomaliesByRule['R9-INV']).toBe(1);
      const judgeCall = calls.find((c) => labelOf(c.prompt).startsWith('judge · R9-INV'));
      expect(judgeCall).toBeDefined();
      expect(judgeCall!.prompt).toContain(
        'R9-INV — this anomaly came from an ADVERSARIAL invariant hunter',
      );
      expect(judgeCall!.prompt).toContain(
        'SECURITY: this rule enforces a WRITTEN project invariant',
      ); // sec framing also applies to R9-INV
      // confirmed + high -> escalation symmetry (§ 2.3) fires a CONFIRMED-direction second opinion
      const secondCall = calls.find((c) => labelOf(c.prompt).startsWith('2nd-opinion'));
      expect(secondCall).toBeDefined();
      expect(secondCall!.prompt).toContain(
        'a first judge CONFIRMED these at high/critical severity from an adversarial invariant-hunter finding',
      );
      expect(result.confirmed).toHaveLength(1);
      expect(result.confirmed[0].id).toBe('R9-INV-1');
    });
    it('the second opinion can RE-EXAMINE and KILL a wrongful confirm (the new direction — symmetric to the existing wrongful-kill reinstatement path)', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md', invariants: [ENGINE_REGRADE] },
        calls,
        invStub(calls, { secondVerdict: 'FALSE' }),
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(result.confirmed).toHaveLength(0);
      expect(result.killedAsFalse).toBe(1);
    });
    it('a MEDIUM-severity R9-INV confirm does NOT escalate (severity gate — only high/critical)', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md', invariants: [ENGINE_REGRADE] },
        calls,
        invStub(calls, { judgeSeverity: 'med' }),
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(calls.some((c) => labelOf(c.prompt).startsWith('2nd-opinion'))).toBe(false);
      expect(result.confirmed).toHaveLength(1); // stays confirmed, just never escalated
    });
    it('a killed (FALSE) R9-INV verdict does NOT trigger the confirm-direction escalation (R9-INV is not in SECURITY_RULES/NEAR_CERTAIN, by design — the refute-first judge is the kill-side guardrail)', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md', invariants: [ENGINE_REGRADE] },
        calls,
        invStub(calls, { judgeVerdict: 'FALSE' }),
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(calls.some((c) => labelOf(c.prompt).startsWith('2nd-opinion'))).toBe(false);
      expect(result.killedAsFalse).toBe(1);
    });
    it('a dead hunter (agent dies twice) yields zero findings, never crashes the walk', async () => {
      const calls: CapturedCall[] = [];
      const stub = async (rawPrompt: string, callOpts: unknown) => {
        calls.push({ prompt: rawPrompt, opts: callOpts as CapturedCall['opts'] });
        if (labelOf(rawPrompt).startsWith('invariant-hunt')) return null;
        return invStub(calls)(rawPrompt, callOpts);
      };
      const result = await runVerifyWith(
        { reportPath: 'r.md', invariants: [ENGINE_REGRADE] },
        calls,
        stub,
      );
      expect(result.status).toBe('DONE');
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(result.anomaliesByRule['R9-INV']).toBeUndefined();
      expect(result.ledger.armedInvariants).toHaveLength(1); // still armed — dying is a coverage hole, not a disarm
    });
  });

  describe('coverageCritic — the external denominator', () => {
    it('dispatched once when the registry is non-empty; its args carry armedIds and the UNARMED registry entries (lightweight — id+territory only)', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md', invariants: [ENGINE_REGRADE, FROZEN_NARRATIVE] },
        calls,
        invStub(calls),
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      const criticCalls = calls.filter((c) => labelOf(c.prompt) === 'coverage-critic');
      expect(criticCalls).toHaveLength(1);
      expect(criticCalls[0].prompt).toContain('"ENGINE-REGRADE"');
      expect(criticCalls[0].prompt).toContain('FROZEN-NARRATIVE'); // named as unarmed
      expect(result.ledger.coverageGaps).toEqual([
        { territory: 'app-fe/**', why: 'no finder walked one FE line' },
      ]);
      expect(result.coverageGaps).toBe(1);
    });
    it('gaps ride the finalJudge input and the fold honesty line', async () => {
      const calls: CapturedCall[] = [];
      await runVerifyWith(
        { reportPath: 'r.md', invariants: [ENGINE_REGRADE] },
        calls,
        invStub(calls),
      );
      const finalJudgeCall = calls.find((c) => labelOf(c.prompt) === 'final-judge');
      expect(finalJudgeCall!.prompt).toContain('coverage-critic gaps');
      expect(finalJudgeCall!.prompt).toContain('app-fe/**');
      const foldCall = calls.find((c) => labelOf(c.prompt) === 'fold');
      expect(foldCall!.prompt).toContain('every coverage-critic gap');
    });
    it('a dead coverage critic is a named coverage hole, never a silent pass', async () => {
      const calls: CapturedCall[] = [];
      const stub = async (rawPrompt: string, callOpts: unknown) => {
        calls.push({ prompt: rawPrompt, opts: callOpts as CapturedCall['opts'] });
        if (labelOf(rawPrompt) === 'coverage-critic') return null;
        return invStub(calls)(rawPrompt, callOpts);
      };
      const logs: string[] = [];
      vi.resetModules();
      globalThis.args = { reportPath: 'r.md', invariants: [ENGINE_REGRADE] };
      globalThis.log = (m?: unknown) => logs.push(String(m));
      globalThis.phase = () => {};
      globalThis.agent = stub;
      globalThis.parallel = async <T>(thunks: Array<() => Promise<T>>): Promise<T[]> =>
        Promise.all(thunks.map((t) => t()));
      globalThis.budget = { total: null, spent: () => 0, remaining: () => Infinity };
      const { WaveWalker } = await import('../src/engine.js');
      const result = await new WaveWalker().run();
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(result.ledger.coverageGaps).toEqual([]);
      expect(logs.some((l) => l.includes('Coverage critic: DIED'))).toBe(true);
    });
  });

  describe('coverage summary + logging — invariants fold into the SAME honest coverage line', () => {
    it('names registered/armed counts and ids when the registry is non-empty', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md', invariants: [ENGINE_REGRADE, FROZEN_NARRATIVE] },
        calls,
        invStub(calls),
      );
      if (result.status !== 'DONE' || !('ledger' in result))
        throw new Error('expected walk result');
      expect(result.coverage).toContain('invariants: 2 registered, 1 armed (ENGINE-REGRADE)');
      expect(result.coverage).toContain('coverage gaps: 1');
    });
  });
});

// ─── WALK TELEMETRY (DEBUG STEP, tmp/walker-debug-design.md) — CONFIG.debug default TRUE, a structured
// debugRecord riding WalkResult/FailedResult, the mechanically-rendered `### Walk Telemetry` block fold
// copies verbatim into the review, per-seat retryAgent tallies, and the floor discipline: telemetry
// failure NEVER breaks a walk, debug:false reproduces today's behavior byte-for-byte. ───
describe('WaveWalker — WALK TELEMETRY (debug step)', () => {
  const labelOf = (prompt: string): string =>
    (/^\[(.+?)\] /.exec(prompt)?.[1] ?? '').replace(/-retry$/, '');

  // A minimal WALK scout with zero GraphQL surface (no fields/jobs/gates) — the smallest scenario that
  // still exercises the full Phase 1-4 spine (scout, one thread walk, the unconditional security
  // auditor, the unconditional final judge, fold). Deliberately does NOT touch sensors/gates/judges/
  // digests/invariants — those get their own coverage in the INVARIANT REGISTRY FEATURE suite above and
  // in the E2/E3 suites; this block is scoped to the telemetry mechanism itself.
  const TELE_SCOUT = {
    headSha: 'sha1',
    territories: [] as string[],
    changedFiles: ['app-be/src/a.ts'],
    changedFileCount: 1,
    mergeShas: ['sha1'],
    threads: [{ id: 't1', type: 'flow', name: 'Flow', verify: 'v' }],
    operations: [] as unknown[],
    fields: [] as unknown[],
    jobs: [] as unknown[],
    gateFiles: [] as string[],
    authRule: '',
  };

  // fold's stubbed review ECHOES whether its own prompt carried the telemetry appendix — this is what
  // lets a test assert on the RESULT (review string) that the appendix rode through, without re-parsing
  // the prompt itself; a real fold agent does the same "copy it in verbatim" duty (fold/prompts.ts).
  function teleStub(calls: CapturedCall[], opts?: { foldDies?: boolean }) {
    return async (rawPrompt: string, callOpts: unknown) => {
      calls.push({ prompt: rawPrompt, opts: callOpts as CapturedCall['opts'] });
      const label = labelOf(rawPrompt);
      if (label === 'scout') return TELE_SCOUT;
      if (label === 'walk · t1')
        return { threadId: 't1', flow: 'INTACT', trace: 't', defects: [], hygiene: [] };
      if (label.startsWith('security'))
        return { findings: [], categoriesSwept: ['8A'], summary: '' };
      if (label === 'final-judge')
        return { verdict: 'SMOOTH SAILING', reinstated: [], missedRisks: [], rationale: 'r' };
      if (label === 'fold') {
        if (opts?.foldDies) return null;
        return {
          verdict: 'SMOOTH SAILING',
          actionItems: [],
          review:
            '# review' + (rawPrompt.includes('### Walk Telemetry') ? '\n[TELEMETRY APPENDED]' : ''),
        };
      }
      // TIME CHECKPOINT — one fixed instant for both readings: elapsed 0, nothing shed.
      if (label.startsWith('clock · ')) return { epochSeconds: 1_000_000 };
      throw new Error('unexpected call in teleStub: ' + rawPrompt.slice(0, 80));
    };
  }

  it('a full runWalk() smoke run returns debugRecord present, schemaVersion 1, and seats containing at least scout/walk/fold', async () => {
    const ww = await freshEngine({ reportPath: 'docs/dev/waves/w/report.md' });
    const result = await ww.run();
    expect(result.status).toBe('DONE');
    if (result.status !== 'DONE' || !('ledger' in result)) throw new Error('expected walk result');
    expect(result.debugRecord).toBeDefined();
    const dr = result.debugRecord!;
    expect(dr.schemaVersion).toBe(1);
    expect(dr.mode).toBe('walk');
    expect(dr.reportPath).toBe('docs/dev/waves/w/report.md');
    expect(dr.degraded).toBe(false);
    expect(dr.gaps).toEqual([]);
    expect(Object.keys(dr.seats)).toEqual(expect.arrayContaining(['scout', 'walk', 'fold']));
    expect(dr.seatsExpectedButAbsent).toEqual([]);
    expect(dr.tokenAttribution).toBeNull();
  });

  it("debug: false → debugRecord is undefined and the fold prompt carries ZERO added telemetry bytes; debug: true (default) vs debug: false on the SAME stubbed inputs leave verdict/actionItems/confirmed/killedAsFalse/unproven byte-identical — only review's appendix and debugRecord differ", async () => {
    const callsOn: CapturedCall[] = [];
    const callsOff: CapturedCall[] = [];
    const resultOn = await runVerifyWith({ reportPath: 'r.md' }, callsOn, teleStub(callsOn));
    const resultOff = await runVerifyWith(
      { reportPath: 'r.md', debug: false },
      callsOff,
      teleStub(callsOff),
    );
    if (resultOn.status !== 'DONE' || !('ledger' in resultOn))
      throw new Error('expected walk result');
    if (resultOff.status !== 'DONE' || !('ledger' in resultOff))
      throw new Error('expected walk result');

    expect(resultOff.debugRecord).toBeUndefined();
    expect(resultOn.debugRecord).toBeDefined();

    // the "pure observation" pin — every verdict-determining field is byte-identical either way
    expect(resultOn.verdict).toBe(resultOff.verdict);
    expect(resultOn.actionItems).toEqual(resultOff.actionItems);
    expect(resultOn.confirmed).toEqual(resultOff.confirmed);
    expect(resultOn.killedAsFalse).toBe(resultOff.killedAsFalse);
    expect(resultOn.unproven).toBe(resultOff.unproven);

    // only the review's telemetry appendix (and debugRecord) differ
    expect(resultOff.review).toBe('# review');
    expect(resultOn.review).toBe('# review\n[TELEMETRY APPENDED]');

    // direct diff on the fold prompt itself — zero added bytes when off, matching fold/prompts.ts's
    // conditional-append discipline exactly (byte-identical to pre-feature when telemetryMd is empty).
    const foldOn = callsOn.find((c) => labelOf(c.prompt) === 'fold')!;
    const foldOff = callsOff.find((c) => labelOf(c.prompt) === 'fold')!;
    expect(foldOn.prompt).toContain('### Walk Telemetry');
    expect(foldOff.prompt).not.toContain('Walk Telemetry');
  });

  describe('FLOOR — telemetry failure never breaks a walk', () => {
    it('runFold dying twice: the FAILED result now carries a non-null debugRecord with Phase 1-3.5 data populated (today this path returns nothing but a detail string)', async () => {
      const calls: CapturedCall[] = [];
      const result = await runVerifyWith(
        { reportPath: 'r.md' },
        calls,
        teleStub(calls, { foldDies: true }),
      );
      expect(result.status).toBe('FAILED');
      if (result.status !== 'FAILED') throw new Error('expected FAILED');
      expect(result.detail).toMatch(/fold died twice/);
      expect(result.debugRecord).toBeDefined();
      const dr = result.debugRecord!;
      expect(dr.armedInvariants).toEqual({ registered: 0, armed: [], unarmed: [] });
      expect(dr.judgeStats.finalJudgeVerdict).toBe('SMOOTH SAILING');
      expect(dr.judgeStats.finalJudgeDied).toBe(false);
      // fold's own tally is captured too — it WAS dispatched (died twice), so it is never reported absent
      expect(dr.seats.fold).toEqual({
        calls: 1,
        diedFirstAttempt: 1,
        retried: 1,
        diedAfterRetry: 1,
      });
      expect(dr.emptyResults.foldDied).toBe(true);
    });

    // BASE_ASSEMBLY_INPUT — every required DebugAssemblyInput field at its honest-zero default. Each
    // test below overrides only the fields relevant to the row of tmp/walker-debug-design.md §5 it pins.
    const BASE_ASSEMBLY_INPUT: DebugAssemblyInput = {
      reportPath: 'r.md',
      invariants: [],
      armedInvariants: [],
      unarmedInvariants: [],
      seatTally: {},
      expectedSeats: [],
      threads: [],
      walks: [],
      jobs: [],
      cards: [],
      hunterResults: [],
      digestJobsLen: 0,
      digests: [],
      security: null,
      coverageCriticResult: null,
      confirmed: [],
      killed: [],
      unproven: [],
      secondOpinionDispatched: 0,
      secondOpinionOverturned: 0,
      secondOpinionReexaminedKilled: 0,
      finalJudge: null,
      finalJudgeReinstated: 0,
      unsensed: [],
      gateSweepSkipped: false,
      gateSweepSkipDetail: '',
      coverageGaps: [],
      contradictionScan: { contradictions: [], filesCompared: 0, uncomparableThreads: [] },
    };

    it('one section throwing (a malformed hunterResults shape) sets degraded: true, names the section in gaps, AND leaves an unrelated section (armedInvariants) fully populated — one bad section never blanks the rest', async () => {
      const { assembleDebugRecord } = await import('../src/engine.js');
      const rec = assembleDebugRecord({
        ...BASE_ASSEMBLY_INPUT,
        invariants: [
          { id: 'X', law: 'L', territory: ['a/**'], triggers: [], exemplars: [], huntBrief: 'H' },
        ],
        armedInvariants: [{ id: 'X', matchedFiles: ['a.ts'], reason: 'scout-armed' }],
        // malformed — a future seat's shape change; forces emptyResults' `.reduce` to throw. The cast is
        // the deliberate point of this test: proving the floor holds against a shape TypeScript would
        // normally reject, exactly the "e.g. a future seat's shape changes" scenario §5 names.
        hunterResults: null as unknown as DebugAssemblyInput['hunterResults'],
      });
      expect(rec.degraded).toBe(true);
      expect(rec.gaps.some((g) => g.startsWith('emptyResults assembly failed'))).toBe(true);
      // the unrelated section, assembled in its OWN try/catch, is untouched by the emptyResults failure
      expect(rec.armedInvariants).toEqual({
        registered: 1,
        armed: [{ id: 'X', matchedFiles: ['a.ts'], reason: 'scout-armed' }],
        unarmed: [],
      });
    });

    it('CONFIG.INVARIANTS empty (the existing registry floor) → armedInvariants = { registered: 0, armed: [], unarmed: [] }, present with honest zeros, never omitted', async () => {
      const { assembleDebugRecord } = await import('../src/engine.js');
      const rec = assembleDebugRecord(BASE_ASSEMBLY_INPUT);
      expect(rec.armedInvariants).toEqual({ registered: 0, armed: [], unarmed: [] });
      expect(rec.degraded).toBe(false);
    });

    it('registry non-empty but coverageCriticResult is null (critic died) → emptyResults.coverageCriticDied === true AND it appears in gaps — the direct pin for "error never renders as absence" against this feature\'s closest analog', async () => {
      const { assembleDebugRecord } = await import('../src/engine.js');
      const rec = assembleDebugRecord({
        ...BASE_ASSEMBLY_INPUT,
        invariants: [
          { id: 'X', law: 'L', territory: ['a/**'], triggers: [], exemplars: [], huntBrief: 'H' },
        ],
        coverageCriticResult: null,
      });
      expect(rec.emptyResults.coverageCriticDied).toBe(true);
      expect(rec.gaps.some((g) => g.includes('coverageCriticDied'))).toBe(true);
    });

    it('a seat in expectedSeats that never got a tally entry is named in seatsExpectedButAbsent — a raw seats diff alone would have missed it (the coverage-declaration gap this field exists to close)', async () => {
      const { assembleDebugRecord } = await import('../src/engine.js');
      const rec = assembleDebugRecord({
        ...BASE_ASSEMBLY_INPUT,
        seatTally: { scout: { calls: 1, diedFirstAttempt: 0, retried: 0, diedAfterRetry: 0 } },
        expectedSeats: ['scout', 'walk', 'judge'],
      });
      expect(rec.seatsExpectedButAbsent).toEqual(['walk', 'judge']);
      // 'scout', which DID get a tally entry, is never flagged
      expect(rec.seatsExpectedButAbsent).not.toContain('scout');
    });

    it('a never-called seat is silently ABSENT from `seats`, not a zero row — a raw diff against seats alone would misread that as "healthy, 0 calls"', async () => {
      const { assembleDebugRecord } = await import('../src/engine.js');
      const rec = assembleDebugRecord(BASE_ASSEMBLY_INPUT);
      expect(rec.seats).toEqual({});
      expect('judge' in rec.seats).toBe(false);
    });
  });
});
