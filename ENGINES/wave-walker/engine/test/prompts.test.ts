// prompts.test.ts — PROMPT PARITY: every ported buildX(args) produces a BYTE-IDENTICAL string to the
// independently-transcribed source-prompts.ts fixture, for the same representative inputs (covering
// every conditional branch the source's inline construction takes).
import { describe, expect, it } from 'vitest';

import { buildScout } from '../src/agents/scout/prompts.js';
import { buildThreadWalker } from '../src/agents/threadWalker/prompts.js';
import { buildSliceSensor } from '../src/agents/sliceSensor/prompts.js';
import { buildGateSweep } from '../src/agents/gateSweep/prompts.js';
import { buildSecurityAuditor } from '../src/agents/securityAuditor/prompts.js';
import { buildAnomalyJudge } from '../src/agents/anomalyJudge/prompts.js';
import { buildTerritoryDigest } from '../src/agents/territoryDigest/prompts.js';
import { buildSecondOpinion } from '../src/agents/secondOpinion/prompts.js';
import { buildFinalJudge } from '../src/agents/finalJudge/prompts.js';
import { buildFold } from '../src/agents/fold/prompts.js';
import { buildClaimExtractor } from '../src/agents/claimExtractor/prompts.js';
import { buildClaimVerifier } from '../src/agents/claimVerifier/prompts.js';
import { buildConsistencyJudge } from '../src/agents/consistencyJudge/prompts.js';
import { buildProbe } from '../src/agents/probe/prompts.js';
import { buildBrainer } from '../src/agents/brainer/prompts.js';
import { buildClaimAuditor } from '../src/agents/claimAuditor/prompts.js';
import { buildSynthesiser } from '../src/agents/synthesiser/prompts.js';

import {
  expectedAnomalyJudge,
  expectedBrainer,
  expectedClaimAuditor,
  expectedClaimExtractor,
  expectedClaimVerifier,
  expectedConsistencyJudge,
  expectedFinalJudge,
  expectedFold,
  expectedGateSweep,
  expectedProbe,
  expectedScout,
  expectedSecondOpinion,
  expectedSecurityAuditor,
  expectedSliceSensor,
  expectedSynthesiser,
  expectedTerritoryDigest,
  expectedThreadWalker,
} from './fixtures/source-prompts.js';

describe('prompt parity — scout', () => {
  it('merge-SHA mode (no branch)', () => {
    const args = {
      reportPath: 'docs/dev/waves/w/report.md',
      branch: null,
      walkerDoc: '.claude/commands/wave/walker.md',
      maxFieldsPerJob: 18,
      charter: '',
      repoRoot: '.',
    };
    expect(buildScout(args)).toBe(expectedScout(args));
  });
  it('pre-merge branch mode', () => {
    const args = {
      reportPath: 'r.md',
      branch: 'pipeline/x',
      walkerDoc: '.claude/commands/wave/walker.md',
      maxFieldsPerJob: 10,
      charter: '',
      repoRoot: '.',
    };
    expect(buildScout(args)).toBe(expectedScout(args));
  });
});

// PROJECT PROFILE (universal-bundle refactor) — repoRoot/authDoc/gateResolverPattern are read off
// args.project (config.ts CONFIG.PROJECT); repoRoot always resolves (default '.'), the other two render
// honestly present-or-absent — no project profile is ever silently assumed.
describe('prompt parity — scout project-profile fields', () => {
  const base = {
    reportPath: 'r.md',
    branch: null,
    walkerDoc: '.claude/commands/wave/walker.md',
    maxFieldsPerJob: 18,
    charter: '',
  };
  it('a non-"." repoRoot renders verbatim', () => {
    const args = { ...base, repoRoot: 'apps/backend' };
    expect(buildScout(args)).toBe(expectedScout(args));
    expect(buildScout(args)).toContain('Repo root: apps/backend.');
  });
  it('authDoc present: splits "{path} § {heading}" into the grep instruction', () => {
    const args = { ...base, repoRoot: '.', authDoc: 'apps/backend/CLAUDE.md § Auth Pattern' };
    expect(buildScout(args)).toBe(expectedScout(args));
    expect(buildScout(args)).toContain(
      'grep apps/backend/CLAUDE.md for its "Auth Pattern" heading',
    );
  });
  it('authDoc absent: the authRule instruction asks for "" instead of a doc grep', () => {
    const args = { ...base, repoRoot: '.' };
    expect(buildScout(args)).toBe(expectedScout(args));
    expect(buildScout(args)).toContain(
      'this project has no configured auth doc (args.project.authDoc) — return authRule: "".',
    );
  });
  it('gateResolverPattern present: names the pattern in the gateFiles instruction', () => {
    const args = { ...base, repoRoot: '.', gateResolverPattern: 'apps/backend/.*/resolvers/' };
    expect(buildScout(args)).toBe(expectedScout(args));
    expect(buildScout(args)).toContain(
      'EVERY file matching the pattern apps/backend/.*/resolvers/',
    );
  });
  it('gateResolverPattern absent: the gateFiles instruction asks for [] instead of a sweep', () => {
    const args = { ...base, repoRoot: '.' };
    expect(buildScout(args)).toBe(expectedScout(args));
    expect(buildScout(args)).toContain(
      'this project has no configured gate-resolver surface (args.project.gateResolverPattern) — return [].',
    );
  });
});

describe('prompt parity — threadWalker', () => {
  it('one thread', () => {
    const args = {
      walkerDoc: '.claude/commands/wave/walker.md',
      hygieneDoc: '.claude/commands/audit/code-hygiene.md',
      thread: {
        id: 't1',
        type: 'flow',
        name: 'Login flow',
        scope: 'be',
        files: ['a.ts'],
        verify: 'log in as X',
      },
      charter: '',
    };
    expect(buildThreadWalker(args)).toBe(expectedThreadWalker(args));
  });
});

describe('prompt parity — sliceSensor', () => {
  const assigned = [{ fieldId: 'Owner.field', field: 'field', sdlTypeToken: 'String' }];
  it('kind=producer', () => {
    const args = {
      jobId: 'p1',
      kind: 'producer' as const,
      files: ['be.ts'],
      hint: 'follow the resolver',
      assigned,
    };
    expect(buildSliceSensor(args)).toBe(expectedSliceSensor(args));
  });
  it('kind=consumer', () => {
    const args = { jobId: 'c1', kind: 'consumer' as const, files: ['fe.tsx'], assigned };
    expect(buildSliceSensor(args)).toBe(expectedSliceSensor(args));
  });
  it('kind=ai', () => {
    const args = { jobId: 'x1', kind: 'ai' as const, files: ['engine.py'], assigned };
    expect(buildSliceSensor(args)).toBe(expectedSliceSensor(args));
  });
});

describe('prompt parity — gateSweep', () => {
  it('one file', () => {
    const args = {
      file: 'apps/backend/src/infrastructure/graphql/resolvers/session.resolvers.ts',
      resourceClasses: ['alpha', 'beta', 'gamma', 'user', 'other'],
      fenceLabels: { org: 'org', ownership: 'ownership' },
    };
    expect(buildGateSweep(args)).toBe(expectedGateSweep(args));
  });
});

describe('prompt parity — securityAuditor (D2 slice shape)', () => {
  it('merge-SHA mode', () => {
    const args = {
      securityDoc: '.claude/commands/audit/security.md',
      clusterFiles: ['a.ts', 'b.ts'],
      allChangedFiles: ['a.ts', 'b.ts', 'c.ts'],
      clusterIndex: 0,
      clusterCount: 2,
      branch: null,
      mergeShas: ['abc123'],
    };
    expect(buildSecurityAuditor(args)).toBe(expectedSecurityAuditor(args));
  });
  it('branch mode', () => {
    const args = {
      securityDoc: '.claude/commands/audit/security.md',
      clusterFiles: ['a.ts'],
      allChangedFiles: ['a.ts'],
      clusterIndex: 0,
      clusterCount: 1,
      branch: 'pipeline/x',
      mergeShas: [],
    };
    expect(buildSecurityAuditor(args)).toBe(expectedSecurityAuditor(args));
  });
});

describe('prompt parity — anomalyJudge', () => {
  const instances = [
    {
      id: 'R6-1',
      rule: 'R6',
      ruleName: 'gate outlier',
      detail: 'd',
      anchors: ['a:1'],
      severityHint: 'high',
      cardId: null,
    },
  ];
  it('sec=true, with ctxCards and an authDoc', () => {
    const args = {
      rule: 'R6',
      ruleMeaning: 'gate asymmetry — auth fences unequal',
      sec: true,
      instances,
      ctxCards: [{ id: 'Owner.a' }],
      authDoc: 'apps/backend/CLAUDE.md § Auth Pattern',
    };
    expect(buildAnomalyJudge(args as never)).toBe(expectedAnomalyJudge(args));
    expect(buildAnomalyJudge(args as never)).toContain(
      'Read apps/backend/CLAUDE.md § Auth Pattern before any FALSE.',
    );
  });
  it('sec=true, no authDoc — the doc-read instruction drops cleanly', () => {
    const args = {
      rule: 'R6',
      ruleMeaning: 'gate asymmetry — auth fences unequal',
      sec: true,
      instances,
      ctxCards: [],
    };
    expect(buildAnomalyJudge(args as never)).toBe(expectedAnomalyJudge(args));
    expect(buildAnomalyJudge(args as never)).not.toContain('before any FALSE');
  });
  it('sec=false, no ctxCards', () => {
    const args = {
      rule: 'R1',
      ruleMeaning: 'orphan producer',
      sec: false,
      instances,
      ctxCards: [],
    };
    expect(buildAnomalyJudge(args as never)).toBe(expectedAnomalyJudge(args));
  });
});

describe('prompt parity — territoryDigest', () => {
  it('one territory', () => {
    const args = { territory: 'BE', slice: [{ id: 'Owner.a' }], charter: '' };
    expect(buildTerritoryDigest(args)).toBe(expectedTerritoryDigest(args));
  });
});

describe('prompt parity — secondOpinion', () => {
  it('one chunk of killed verdicts', () => {
    const args = {
      authRule: 'app-be/CLAUDE.md § Auth Pattern: "..."',
      items: [
        {
          verdict: { anomalyId: 'R7-1', verdict: 'FALSE' as const, what: 'w' },
          anomaly: {
            id: 'R7-1',
            rule: 'R7' as const,
            ruleName: 'unfenced ID flow',
            detail: 'd',
            anchors: [],
            severityHint: 'critical' as const,
            cardId: null,
          },
        },
      ],
    };
    expect(buildSecondOpinion(args)).toBe(expectedSecondOpinion(args));
  });
});

describe('prompt parity — finalJudge', () => {
  const base = {
    walksBrief: [{ id: 't1', flow: 'INTACT' as const, defects: [] }],
    confirmed: [],
    unproven: [],
    killedWithAnomaly: [],
    digests: [],
    securityDoc: '.claude/commands/audit/security.md',
    walksLen: 1,
    threadsLen: 1,
    cardsLen: 0,
    unsensed: [],
    charter: '',
  };
  it('security died (null)', () => {
    const args = { ...base, security: null };
    expect(buildFinalJudge(args)).toBe(expectedFinalJudge(args));
  });
  it('security present — the denominator (auditors, opened/in-scope, UNSWEPT) rides the line', () => {
    const args = {
      ...base,
      security: {
        findings: [
          { id: 'S1', category: '8C', severity: 'high' as const, what: 'w', location: 'l' },
        ],
        categoriesSwept: ['8A', '8C'],
        summary: 's',
        filesOpened: ['a.ts', 'b.ts'],
        auditorsDispatched: 2,
        auditorsReturned: 2,
        filesInScope: 3,
        filesUnswept: ['c.ts'],
      },
    };
    expect(buildFinalJudge(args)).toBe(expectedFinalJudge(args));
    expect(buildFinalJudge(args)).toContain('auditors 2/2 · opened 2/3 files');
    expect(buildFinalJudge(args)).toContain('UNSWEPT files — coverage holes, never clean: c.ts');
  });
});

describe('prompt parity — fold', () => {
  const base = {
    reportPath: 'r.md',
    walks: [],
    confirmed: [],
    unproven: [],
    killedCount: 0,
    digests: [],
    coverageSummary:
      'threads walked: 1/1 · fields sensed: 0 · UNSENSED: none · gates: 0 · ledger anomalies: 0 → confirmed 0, false 0, unproven 0 · security: AUDIT DIED',
  };
  it('no security, no final judge', () => {
    const args = { ...base, security: null, finalJudge: null };
    expect(buildFold(args)).toBe(expectedFold(args));
  });
  it('with security AND final judge — the denominator and the UNSWEPT honesty clause ride along', () => {
    const args = {
      ...base,
      security: {
        findings: [{ id: 'S1' }],
        categoriesSwept: ['8A'],
        summary: 'swept',
        filesOpened: ['a.ts'],
        auditorsDispatched: 1,
        auditorsReturned: 1,
        filesInScope: 2,
        filesUnswept: ['b.ts'],
      },
      finalJudge: {
        verdict: 'ROUGH SEAS',
        missedRisks: [{ what: 'w', where: 'x' }],
        rationale: 'because',
      },
    };
    expect(buildFold(args as never)).toBe(expectedFold(args as never));
    expect(buildFold(args as never)).toContain('"filesOpened":"1/2"');
    expect(buildFold(args as never)).toContain(
      ', AND the security files-opened denominator with every UNSWEPT file',
    );
  });
});

describe('prompt parity — claimExtractor', () => {
  it('one manifest', () => {
    const args = { manifestPath: 'docs/dev/waves/w/manifest.md', repoRoot: '.' };
    expect(buildClaimExtractor(args)).toBe(expectedClaimExtractor(args));
  });
});

describe('prompt parity — claimVerifier', () => {
  it('full claim (context + files), with a grounding question', () => {
    const args = {
      claim: { id: 'T1-C1', statement: 'X exists', context: 'ctx', files: ['a.ts'] },
      question: 'is the manifest sound?',
      repoRoot: '.',
    };
    expect(buildClaimVerifier(args)).toBe(expectedClaimVerifier(args));
  });
  it('bare claim (no context, no files, no question)', () => {
    const args = { claim: { id: 'c1', statement: 'Y is true' }, question: '', repoRoot: '.' };
    expect(buildClaimVerifier(args)).toBe(expectedClaimVerifier(args));
  });
});

describe('prompt parity — consistencyJudge (E2 payload diet)', () => {
  it('one manifest — consensus map + non-CONFIRMED detail only', () => {
    const args = {
      manifestPath: 'm.md',
      nonConfirmed: [{ claimId: 'c2', verdict: 'REFUTED', reasoning: 'r' }],
      consensus: { c1: 'CONFIRMED', c2: 'REFUTED' },
      conflictChecks: [{ id: 'cc1', what: 'w' }],
    };
    expect(buildConsistencyJudge(args as never)).toBe(expectedConsistencyJudge(args));
  });
});

describe('prompt parity — probe', () => {
  it('pursue lane, no files', () => {
    const args = {
      lane: { id: 'w0-1', kind: 'pursue' as const, question: 'DIRECT — the goal head-on' },
      goal: 'why does X fail',
      scopeLine: '',
    };
    expect(buildProbe(args)).toBe(expectedProbe(args));
  });
  it('attack lane, with files, targets, note, and scope', () => {
    const args = {
      lane: {
        id: 'w1-2',
        kind: 'attack' as const,
        question: 'attack claim c3',
        files: ['a.ts'],
        targets: ['c3'],
        note: 'try the fallback path',
      },
      goal: 'why does X fail',
      scopeLine: ' SCOPE (stay inside): ["src/a.ts"].',
    };
    expect(buildProbe(args)).toBe(expectedProbe(args));
  });
});

describe('prompt parity — brainer', () => {
  it('one wave', () => {
    const args = {
      goal: 'why does X fail',
      scopeLine: '',
      wave: 1,
      maxWaves: 3,
      ledgerRows: [
        { id: 'c1', s: 'X', status: 'tentative', files: 1, survived: 0, audit: 'pending' },
      ],
      openLeads: [{ id: 'L1', what: 'follow up', files: [] }],
      maxLanes: 5,
    };
    expect(buildBrainer(args)).toBe(expectedBrainer(args));
  });
});

describe('prompt parity — claimAuditor', () => {
  it('pending rows', () => {
    const args = {
      rows: [{ id: 'c1', anchors: [{ anchor: 'a.ts:1', quote: 'q' }] }],
      wave: 0,
      repoRoot: '.',
    };
    expect(buildClaimAuditor(args)).toBe(expectedClaimAuditor(args));
  });
});

describe('prompt parity — synthesiser', () => {
  const base = {
    goal: 'why does X fail',
    stopReason: 'brainer-done: goal answered',
    keyIds: ['c1'],
    conf: 'high' as const,
    claimsOut: [
      {
        id: 'c1',
        statement: 'X',
        status: 'settled',
        anchors: [],
        files: ['a.ts'],
        survived: 1,
        audit: 'pass',
      },
    ],
    openLeads: [],
  };
  it('no reportOut (write no files)', () => {
    const args = { ...base, reportOut: null, resultSoFarText: 'X because Y' };
    expect(buildSynthesiser(args)).toBe(expectedSynthesiser(args));
  });
  it('with reportOut', () => {
    const args = {
      ...base,
      reportOut: 'out.md',
      resultSoFarText: '(brainer dead — reason from the ledger alone)',
    };
    expect(buildSynthesiser(args)).toBe(expectedSynthesiser(args));
  });
});

describe('charter append — Professor-authored WALK CHARTER blocks (walk-mode seats only)', () => {
  const CHARTER = 'confirm the export path never touches PHI columns';
  const DELIM = '\nWALK CHARTER (caller-supplied duty): ';
  it('scout: empty charter is byte-identical to source; non-empty appends the exact block at the very end', () => {
    const base = {
      reportPath: 'r.md',
      branch: null,
      walkerDoc: '.claude/commands/wave/walker.md',
      maxFieldsPerJob: 18,
      repoRoot: '.',
    };
    expect(buildScout({ ...base, charter: '' })).toBe(expectedScout(base));
    expect(buildScout({ ...base, charter: CHARTER })).toBe(
      expectedScout(base) +
        DELIM +
        CHARTER +
        '\nShape the thread manifest to serve this charter IN ADDITION to the standard enumeration — add charter-driven threads; never drop or merge a standard thread for it.',
    );
  });
  it('threadWalker: exact block appended', () => {
    const base = {
      walkerDoc: '.claude/commands/wave/walker.md',
      hygieneDoc: '.claude/commands/audit/code-hygiene.md',
      thread: { id: 't1', type: 'flow', name: 'F', verify: 'v' },
    };
    expect(buildThreadWalker({ ...base, charter: '' })).toBe(expectedThreadWalker(base));
    expect(buildThreadWalker({ ...base, charter: CHARTER })).toBe(
      expectedThreadWalker(base) +
        DELIM +
        CHARTER +
        '\nWeigh this thread against the charter and report charter-relevant findings explicitly in notes — on top of the standard verdict, never instead of it.',
    );
  });
  it('territoryDigest: exact block appended', () => {
    const base = { territory: 'BE', slice: [{ id: 'Owner.a' }] };
    expect(buildTerritoryDigest({ ...base, charter: '' })).toBe(expectedTerritoryDigest(base));
    expect(buildTerritoryDigest({ ...base, charter: CHARTER })).toBe(
      expectedTerritoryDigest(base) +
        DELIM +
        CHARTER +
        '\nHunt charter-relevant smells in your territory on top of the standard digest.',
    );
  });
  it('finalJudge: exact block appended', () => {
    const base = {
      walksBrief: [],
      confirmed: [],
      unproven: [],
      killedWithAnomaly: [],
      digests: [],
      securityDoc: '.claude/commands/audit/security.md',
      security: null,
      walksLen: 0,
      threadsLen: 0,
      cardsLen: 0,
      unsensed: [],
    };
    expect(buildFinalJudge({ ...base, charter: '' })).toBe(expectedFinalJudge(base));
    expect(buildFinalJudge({ ...base, charter: CHARTER })).toBe(
      expectedFinalJudge(base) +
        DELIM +
        CHARTER +
        '\nAnswer the charter explicitly: what the walk found for it and whether its concern is satisfied — inside your existing fields; the verdict scale and schema unchanged.',
    );
  });
});

// INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — the registry block rides ONLY
// the scout prompt (never sliceSensor/gateSweep/securityAuditor, per the design's explicit boundary), and
// only when `invariants` is non-empty — same additive, zero-bytes-when-absent discipline as `charter`.
describe('invariant registry append — scout ONLY, zero bytes when absent/empty', () => {
  const REGISTRY = [
    {
      id: 'ENGINE-REGRADE',
      law: 'engine-frozen law text',
      territory: ['app-cortex/src/app_cortex/db/analysis/**'],
      triggers: ['diff touches territory'],
      exemplars: ['chunk_cache.py:38'],
      huntBrief: 'walk every reuse/skip/cache gate for currency',
    },
  ];
  const base = {
    reportPath: 'r.md',
    branch: null,
    walkerDoc: '.claude/commands/wave/walker.md',
    maxFieldsPerJob: 18,
    charter: '',
    repoRoot: '.',
  };

  it('absent invariants is byte-identical to source parity (already proven above); explicit [] is the same', () => {
    expect(buildScout({ ...base, invariants: [] })).toBe(expectedScout(base));
    expect(buildScout({ ...base })).toBe(expectedScout(base));
  });
  it('a non-empty registry appends step 7, embedding the law/territory/huntBrief verbatim and the doc pointer', () => {
    const withDoc = buildScout({
      ...base,
      invariants: REGISTRY,
      invariantsDoc: '.claude/commands/wave/walker-invariants.md',
    });
    expect(withDoc).toContain(expectedScout(base)); // the standard enumeration is untouched, registry block is a pure tail
    expect(withDoc).toContain('.claude/commands/wave/walker-invariants.md');
    expect(withDoc).toContain('ENGINE-REGRADE');
    expect(withDoc).toContain('engine-frozen law text');
    expect(withDoc).toContain('walk every reuse/skip/cache gate for currency');
    expect(withDoc).toContain('armedInvariants');
  });
  it('charter and the invariant registry compose (both additive, both tails) without corrupting either', () => {
    const both = buildScout({ ...base, charter: 'audit the export path', invariants: REGISTRY });
    expect(both).toContain('WALK CHARTER (caller-supplied duty): audit the export path');
    expect(both).toContain('ENGINE-REGRADE');
  });
});

// R9-INV judge extension (§ 2.3) — gated strictly on `rule === 'R9-INV'`; every R1-R8 call is untouched
// (already proven by the unmodified anomalyJudge parity tests above, which never pass rule='R9-INV').
describe('anomalyJudge R9-INV extension — REFUTE-FIRST block, gated on rule', () => {
  const instances = [
    {
      id: 'R9-INV-1',
      rule: 'R9-INV' as const,
      ruleName: 'invariant-registry violation',
      detail: 'd',
      anchors: ['a:1'],
      severityHint: 'high' as const,
      cardId: null,
    },
  ];
  it('rule=R9-INV appends the refute-first block AND the sec framing (a hunter finding is exactly the class the sec framing was written for)', () => {
    const out = buildAnomalyJudge({
      rule: 'R9-INV',
      ruleMeaning: 'invariant-registry violation — ...',
      sec: true,
      instances,
      ctxCards: [],
    } as never);
    expect(out).toContain('R9-INV — this anomaly came from an ADVERSARIAL invariant hunter');
    expect(out).toContain('reproduce the concrete failure scenario yourself or kill it');
    expect(out).toContain('SECURITY: this rule enforces a WRITTEN project invariant');
  });
  it('rule=R1 never renders the R9-INV block, byte-identical to the existing parity fixture', () => {
    const r1Instances = [
      {
        id: 'R1-1',
        rule: 'R1' as const,
        ruleName: 'orphan producer',
        detail: 'd',
        anchors: [],
        severityHint: 'med' as const,
        cardId: 'Owner.a',
      },
    ];
    const args = {
      rule: 'R1' as const,
      ruleMeaning: 'orphan producer',
      sec: false,
      instances: r1Instances,
      ctxCards: [],
    };
    expect(buildAnomalyJudge(args as never)).toBe(expectedAnomalyJudge(args));
    expect(buildAnomalyJudge(args as never)).not.toContain('ADVERSARIAL invariant hunter');
  });
});

// secondOpinion `direction` (§ 2.3, escalation symmetry) — 'confirmed' is the ONLY path that changes any
// byte; `direction` omitted (the sole caller before this feature) already proven byte-identical above.
describe('secondOpinion direction — escalation symmetry, additive', () => {
  const items = [
    {
      verdict: { anomalyId: 'R9-INV-1', verdict: 'CONFIRMED' as const, what: 'w' },
      anomaly: {
        id: 'R9-INV-1',
        rule: 'R9-INV' as const,
        ruleName: 'invariant-registry violation',
        detail: 'd',
        anchors: [],
        severityHint: 'high' as const,
        cardId: null,
      },
    },
  ];
  it('direction omitted stays the ORIGINAL killed-framing text, byte-identical to the parity fixture above', () => {
    const args = { authRule: 'AUTH', items };
    expect(buildSecondOpinion(args)).toBe(expectedSecondOpinion(args));
  });
  it('direction="confirmed" swaps the framing to the wrongful-confirm re-examination text', () => {
    const out = buildSecondOpinion({ authRule: 'AUTH', items, direction: 'confirmed' });
    expect(out).toContain(
      'a first judge CONFIRMED these at high/critical severity from an adversarial invariant-hunter finding',
    );
    expect(out).toContain('Confirmed verdicts with anomalies: ');
    expect(out).not.toContain('a first judge killed these as FALSE');
  });
});

// VERDICT CONTRADICTIONS (walker.md § Orchestration) — the named-contradiction block rides the finalJudge
// and fold prompts ONLY, and only when `contradictions` is non-empty: same additive, zero-bytes-when-absent
// discipline as `charter` and the invariant registry.
describe('verdict-contradiction append — finalJudge + fold only, zero bytes when absent/empty', () => {
  const CONTRA = [
    {
      file: 'app-web/content/privacy/en.mdx',
      clean: [
        {
          threadId: 'T12',
          name: 'COMPLIANCE-CLAIM INTEGRITY',
          flow: 'INTACT' as const,
          defects: 0,
        },
      ],
      flagged: [
        {
          threadId: 'CT5',
          name: 'compliance claims substantiated',
          flow: 'AT-RISK' as const,
          defects: 2,
        },
      ],
    },
  ];
  const judgeBase = {
    walksBrief: [],
    confirmed: [],
    unproven: [],
    killedWithAnomaly: [],
    digests: [],
    securityDoc: '.claude/commands/audit/security.md',
    security: null,
    walksLen: 0,
    threadsLen: 0,
    cardsLen: 0,
    unsensed: [],
  };
  const foldBase = {
    reportPath: 'r.md',
    walks: [],
    confirmed: [],
    unproven: [],
    killedCount: 0,
    digests: [],
    security: null,
    coverageSummary: 'c',
    finalJudge: null,
  };

  it('finalJudge: absent or [] is byte-identical to the source parity fixture', () => {
    expect(buildFinalJudge({ ...judgeBase, charter: '' })).toBe(expectedFinalJudge(judgeBase));
    expect(buildFinalJudge({ ...judgeBase, charter: '', contradictions: [] })).toBe(
      expectedFinalJudge(judgeBase),
    );
  });

  it('finalJudge: a non-empty list appends the named block with the no-averaging duty, before any charter block', () => {
    const out = buildFinalJudge({ ...judgeBase, charter: '', contradictions: CONTRA });
    expect(out).toContain(
      'NAMED CONTRADICTIONS (two or more seats walked ONE file and returned opposite verdicts): ' +
        JSON.stringify(CONTRA),
    );
    expect(out).toContain('VOID any verdict resting on evidence the file does not contain');
    expect(out).toContain(
      'averaging, merging, or letting the more optimistic verdict stand is forbidden',
    );
    const withCharter = buildFinalJudge({
      ...judgeBase,
      charter: 'audit the copy',
      contradictions: CONTRA,
    });
    expect(withCharter.indexOf('NAMED CONTRADICTIONS')).toBeLessThan(
      withCharter.indexOf('WALK CHARTER'),
    );
  });

  it('fold: absent or [] is byte-identical; a non-empty list demands the pair in Coverage and a VOID mark in the table', () => {
    expect(buildFold({ ...foldBase })).toBe(expectedFold(foldBase));
    expect(buildFold({ ...foldBase, contradictions: [] })).toBe(expectedFold(foldBase));
    const out = buildFold({ ...foldBase, contradictions: CONTRA });
    expect(out).toContain(
      'NAMED CONTRADICTIONS (seats disagreeing over ONE file): ' + JSON.stringify(CONTRA),
    );
    expect(out).toContain('never silently keep the cleaner verdict');
  });
});

// WALKER INSTRUMENT DEFECTS (docs/dev/audits/wave-walker-instrument-defects-2026-07-28.md) — one pin
// per defect CLASS, anchored to the audit so a future prompt rewrite cannot silently drop the cure:
// D1 the scout's separately-executed count + the corrective-retry block; D3 the hygiene standard named
// by path in the walker prompt (the phantom-citation class: a doc cited but handed to no seat); D4 the
// producer-verification clause on every seat that certified the fabricated-envelope fix.
describe('audit 2026-07-28 regression pins — the four instrument defects stay fixed', () => {
  const scoutBase = {
    reportPath: 'r.md',
    branch: null,
    walkerDoc: '.claude/commands/wave/walker.md',
    maxFieldsPerJob: 18,
    charter: '',
    repoRoot: '.',
  };

  it('D1: both scout modes demand the separately-executed count, and the output list carries changedFileCount', () => {
    for (const branch of [null, 'pipeline/x']) {
      const out = buildScout({ ...scoutBase, branch });
      expect(out).toContain('changedFileCount');
      expect(out).toContain('never the length of your enumerated list');
      expect(out).toContain('The engine FAILS the walk when list and count disagree');
      expect(out).toContain(
        'Structured output: headSha, territories, changedFiles, changedFileCount,',
      );
    }
  });

  it('D1: the reconcile arg appends the RECONCILIATION RETRY block naming both numbers; absent appends zero bytes', () => {
    expect(buildScout({ ...scoutBase })).not.toContain('RECONCILIATION RETRY');
    const out = buildScout({ ...scoutBase, reconcile: { enumerated: 140, counted: 148 } });
    expect(out).toContain(
      'RECONCILIATION RETRY: your previous pass enumerated 140 changedFiles but its separately-executed git count said 148.',
    );
    expect(out).toContain(expectedScout(scoutBase)); // the standard enumeration is untouched — a pure tail
  });

  it('D3: the walker prompt hands the REAL hygiene standard by path — never a paraphrase', () => {
    const out = buildThreadWalker({
      walkerDoc: 'w.md',
      hygieneDoc: '.claude/commands/audit/code-hygiene.md',
      thread: { id: 't1', type: 'flow', name: 'F', verify: 'v' },
      charter: '',
    });
    expect(out).toContain('read .claude/commands/audit/code-hygiene.md and apply it scope-diff');
  });

  it('D4: walker, anomaly judge, and security auditor each carry the producer-verification clause', () => {
    const walker = buildThreadWalker({
      walkerDoc: 'w.md',
      hygieneDoc: 'h.md',
      thread: { id: 't1', type: 'flow', name: 'F', verify: 'v' },
      charter: '',
    });
    const judge = buildAnomalyJudge({
      rule: 'R1',
      ruleMeaning: 'orphan producer',
      sec: false,
      instances: [],
      ctxCards: [],
    } as never);
    const auditor = buildSecurityAuditor({
      securityDoc: 's.md',
      clusterFiles: ['a.ts'],
      allChangedFiles: ['a.ts'],
      clusterIndex: 0,
      clusterCount: 1,
      branch: null,
      mergeShas: [],
    });
    for (const prompt of [walker, judge, auditor]) {
      expect(prompt.toLowerCase()).toContain('emits');
      expect(prompt.toLowerCase()).toContain('open the producer');
      expect(prompt.toLowerCase()).toContain('fabricated envelope is never evidence');
    }
  });

  it('D2: the auditor prompt is slice-scoped with the full set as context, and demands filesOpened/filesSkipped', () => {
    const out = buildSecurityAuditor({
      securityDoc: 's.md',
      clusterFiles: ['a.ts'],
      allChangedFiles: ['a.ts', 'z.ts'],
      clusterIndex: 1,
      clusterCount: 3,
      branch: null,
      mergeShas: [],
    });
    expect(out).toContain('(slice 2/3)');
    expect(out).toContain('OPEN AND SWEEP EVERY ONE');
    expect(out).toContain('filesOpened lists every file you ACTUALLY read');
    expect(out).toContain(
      'Structured output: findings (Expected/Got), categoriesSwept, filesOpened, filesSkipped, summary.',
    );
  });
});
