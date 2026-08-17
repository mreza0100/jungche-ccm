// schemas.test.ts — SCHEMA PARITY: every ported seat schema deep-equals its source-schemas.ts fixture
// (transcribed independently from the 730-line source). Any drift fails loudly — the schema IS the
// contract each agent's StructuredOutput must satisfy.
import { describe, expect, it } from 'vitest';
import { SCOUT as srcSCOUT } from './fixtures/source-schemas.js';
import { WALK as srcWALK } from './fixtures/source-schemas.js';
import { SLICES as srcSLICES, SLICE_PROPS as srcSLICE_PROPS } from './fixtures/source-schemas.js';
import { GATE_SWEEP as srcGATE_SWEEP } from './fixtures/source-schemas.js';
import { JUDGE as srcJUDGE } from './fixtures/source-schemas.js';
import { DIGEST as srcDIGEST } from './fixtures/source-schemas.js';
import { FOLD as srcFOLD } from './fixtures/source-schemas.js';
import { SECURITY as srcSECURITY } from './fixtures/source-schemas.js';
import { FINAL as srcFINAL } from './fixtures/source-schemas.js';
import { EXTRACT as srcEXTRACT } from './fixtures/source-schemas.js';
import { VERIFY as srcVERIFY } from './fixtures/source-schemas.js';
import { CONFLICT as srcCONFLICT } from './fixtures/source-schemas.js';
import { PROBE as srcPROBE } from './fixtures/source-schemas.js';
import { COORD as srcCOORD } from './fixtures/source-schemas.js';
import { AUDITS as srcAUDITS } from './fixtures/source-schemas.js';
import { SYNTH as srcSYNTH } from './fixtures/source-schemas.js';

import { SCOUT } from '../src/agents/scout/index.js';
import { WALK } from '../src/agents/threadWalker/index.js';
import { SLICE_PROPS, SLICES } from '../src/agents/sliceSensor/index.js';
import { GATE_SWEEP } from '../src/agents/gateSweep/index.js';
import { JUDGE } from '../src/agents/anomalyJudge/index.js';
import { DIGEST } from '../src/agents/territoryDigest/index.js';
import { FOLD } from '../src/agents/fold/index.js';
import { SECURITY } from '../src/agents/securityAuditor/index.js';
import { FINAL } from '../src/agents/finalJudge/index.js';
import { EXTRACT } from '../src/agents/claimExtractor/index.js';
import { VERIFY } from '../src/agents/claimVerifier/index.js';
import { CONFLICT } from '../src/agents/consistencyJudge/index.js';
import { PROBE } from '../src/agents/probe/index.js';
import { COORD } from '../src/agents/brainer/index.js';
import { AUDITS } from '../src/agents/claimAuditor/index.js';
import { SYNTH } from '../src/agents/synthesiser/index.js';
import { secondOpinion } from '../src/agents/secondOpinion/index.js';
import { INVARIANT_HUNT } from '../src/agents/invariantHunter/index.js';
import { COVERAGE_CRITIC } from '../src/agents/coverageCritic/index.js';

describe('schema parity — every ported seat schema deep-equals the transcribed source', () => {
  it('SCOUT', () => expect(SCOUT).toEqual(srcSCOUT));
  it('WALK', () => expect(WALK).toEqual(srcWALK));
  it('SLICE_PROPS', () => expect(SLICE_PROPS).toEqual(srcSLICE_PROPS));
  it('SLICES', () => expect(SLICES).toEqual(srcSLICES));
  it('GATE_SWEEP', () => expect(GATE_SWEEP).toEqual(srcGATE_SWEEP));
  it('JUDGE', () => expect(JUDGE).toEqual(srcJUDGE));
  it('DIGEST', () => expect(DIGEST).toEqual(srcDIGEST));
  it('FOLD', () => expect(FOLD).toEqual(srcFOLD));
  it('SECURITY', () => expect(SECURITY).toEqual(srcSECURITY));
  it('FINAL', () => expect(FINAL).toEqual(srcFINAL));
  it('EXTRACT', () => expect(EXTRACT).toEqual(srcEXTRACT));
  it('VERIFY', () => expect(VERIFY).toEqual(srcVERIFY));
  it('CONFLICT', () => expect(CONFLICT).toEqual(srcCONFLICT));
  it('PROBE', () => expect(PROBE).toEqual(srcPROBE));
  it('COORD', () => expect(COORD).toEqual(srcCOORD));
  it('AUDITS', () => expect(AUDITS).toEqual(srcAUDITS));
  it('SYNTH', () => expect(SYNTH).toEqual(srcSYNTH));
  it('secondOpinion reuses the JUDGE schema BY REFERENCE, exactly like the source reusing its one JUDGE const for both calls', () => {
    expect(secondOpinion.schema).toBe(JUDGE);
  });
});

describe('intentional divergence #4 — synthesiser anti-degenerate guard', () => {
  it('SYNTH.answer requires minLength 80 (never a placeholder answer)', () => {
    expect(SYNTH.properties!.answer.minLength).toBe(80);
  });
});

// INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.2, 2.4) — the two NEW seats have no
// source-bundle equivalent, so there is nothing to parity-check against; these are direct STRUCTURAL
// assertions on the contract each schema enforces (same style as the divergence block above).
describe('INVARIANT_HUNT schema (invariantHunter) — the adversarial finder contract', () => {
  it('requires invariantId, findings, coverage at the top level', () => {
    expect(INVARIANT_HUNT.required).toEqual(['invariantId', 'findings', 'coverage']);
  });
  it('caps findings at 12 and REQUIRES failureScenario + severity per finding — refute-first, never a vibe', () => {
    expect(INVARIANT_HUNT.properties!.findings.maxItems).toBe(12);
    expect(INVARIANT_HUNT.properties!.findings.items!.required).toEqual([
      'what',
      'location',
      'expected',
      'got',
      'failureScenario',
      'severity',
    ]);
  });
  it('severity uses the standard 5-point Severity enum (matches SecurityFinding, not the 4-point DigestFinding one)', () => {
    expect(INVARIANT_HUNT.properties!.findings.items!.properties!.severity.enum).toEqual([
      'info',
      'low',
      'med',
      'high',
      'critical',
    ]);
  });
});

describe('COVERAGE_CRITIC schema (coverageCritic) — the external-denominator contract', () => {
  it('requires gaps, summary at the top level', () => {
    expect(COVERAGE_CRITIC.required).toEqual(['gaps', 'summary']);
  });
  it('caps gaps at 8 (the design\'s "≤8 territories" bound) and requires territory+why per gap', () => {
    expect(COVERAGE_CRITIC.properties!.gaps.maxItems).toBe(8);
    expect(COVERAGE_CRITIC.properties!.gaps.items!.required).toEqual(['territory', 'why']);
  });
});
