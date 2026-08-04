import { describe, expect, it } from 'vitest';
import { Configs } from '../src/config.js';

describe('Configs validation', () => {
  it('throws when none of reportPath/manifestPath/goal/claims is present', () =>
    expect(() => new Configs({})).toThrow(/requires args.reportPath/));
  it('throws on invalid JSON string args', () =>
    expect(() => new Configs('{not json')).toThrow(/not valid JSON/));
  it('parses a JSON string args', () =>
    expect(new Configs(JSON.stringify({ reportPath: 'r.md' })).REPORT_PATH).toBe('r.md'));
  it('accepts args.claims alone (no reportPath) — verify mode', () =>
    expect(() => new Configs({ claims: [{ id: 'c1', statement: 's' }] })).not.toThrow());
  it('accepts args.manifestPath alone — manifest-verify mode', () =>
    expect(() => new Configs({ manifestPath: 'm.md' })).not.toThrow());
  it('accepts args.goal alone — investigate mode', () =>
    expect(() => new Configs({ goal: 'why does X happen' })).not.toThrow());
  it('an empty claims array does not satisfy the requirement on its own', () =>
    expect(() => new Configs({ claims: [] })).toThrow(/requires args.reportPath/));
});

describe('Configs mode dispatch (precedence: manifest-verify > verify > investigate > walk)', () => {
  it('walk when only reportPath is given', () =>
    expect(new Configs({ reportPath: 'r.md' }).mode).toBe('walk'));
  it('verify when claims is given', () =>
    expect(new Configs({ claims: [{ id: 'c1', statement: 's' }] }).mode).toBe('verify'));
  it('manifest-verify when manifestPath is given', () =>
    expect(new Configs({ manifestPath: 'm.md' }).mode).toBe('manifest-verify'));
  it('manifest-verify wins over verify when BOTH manifestPath and claims are given', () =>
    expect(new Configs({ manifestPath: 'm.md', claims: [{ id: 'c1', statement: 's' }] }).mode).toBe(
      'manifest-verify',
    ));
  it('investigate when goal is given (and none of the above)', () =>
    expect(new Configs({ goal: 'g' }).mode).toBe('investigate'));
  it('investigate wins over walk when both reportPath and goal are given (mode keys off manifestPath > claims > goal > walk)', () =>
    expect(new Configs({ reportPath: 'r.md', goal: 'g' }).mode).toBe('investigate'));
});

describe('Configs — WALK mode config', () => {
  it('defaults BRANCH to null, LEDGER_PATH derived from reportPath', () => {
    const c = new Configs({ reportPath: 'docs/dev/waves/w/report.md' });
    expect(c.BRANCH).toBeNull();
    expect(c.LEDGER_PATH).toBe('docs/dev/waves/w/walker-ledger.json');
  });
  it('an explicit ledgerPath overrides the derived one', () => {
    const c = new Configs({ reportPath: 'r.md', ledgerPath: 'custom-ledger.json' });
    expect(c.LEDGER_PATH).toBe('custom-ledger.json');
  });
  it('reads branch for pre-merge mode', () =>
    expect(new Configs({ reportPath: 'r.md', branch: 'pipeline/x' }).BRANCH).toBe('pipeline/x'));
  it('defaults MAX_FIELDS_PER_JOB to 18, MAX_SENSORS to 60', () => {
    const c = new Configs({ reportPath: 'r.md' });
    expect(c.MAX_FIELDS_PER_JOB).toBe(18);
    expect(c.MAX_SENSORS).toBe(60);
  });
  it('honors explicit maxFieldsPerJob/maxSensors overrides', () => {
    const c = new Configs({ reportPath: 'r.md', maxFieldsPerJob: 5, maxSensors: 10 });
    expect(c.MAX_FIELDS_PER_JOB).toBe(5);
    expect(c.MAX_SENSORS).toBe(10);
  });
  // D2 SECURITY FAN-OUT (audit 2026-07-28)
  it('defaults SECURITY_FILES_PER_AUDITOR to 12; honors a positive override; rejects zero/negative/non-integer', () => {
    expect(new Configs({ reportPath: 'r.md' }).SECURITY_FILES_PER_AUDITOR).toBe(12);
    expect(
      new Configs({ reportPath: 'r.md', securityFilesPerAuditor: 8 }).SECURITY_FILES_PER_AUDITOR,
    ).toBe(8);
    expect(
      new Configs({ reportPath: 'r.md', securityFilesPerAuditor: 0 }).SECURITY_FILES_PER_AUDITOR,
    ).toBe(12);
    expect(
      new Configs({ reportPath: 'r.md', securityFilesPerAuditor: -3 }).SECURITY_FILES_PER_AUDITOR,
    ).toBe(12);
    expect(
      new Configs({ reportPath: 'r.md', securityFilesPerAuditor: 'many' })
        .SECURITY_FILES_PER_AUDITOR,
    ).toBe(12);
  });
  // D3 (audit 2026-07-28) — the hygiene standard is a HANDED doc, no longer a phantom citation
  it('holds HYGIENE_DOC alongside WALKER_DOC/SECURITY_DOC', () => {
    expect(new Configs({ reportPath: 'r.md' }).HYGIENE_DOC).toBe(
      '.claude/commands/audit/code-hygiene.md',
    );
  });
  it('defaults EXTRA_THREADS to []', () =>
    expect(new Configs({ reportPath: 'r.md' }).EXTRA_THREADS).toEqual([]));
  it('carries extraThreads verbatim', () => {
    const t = [{ id: 't1', type: 'flow', name: 'n', verify: 'v' }];
    expect(new Configs({ reportPath: 'r.md', extraThreads: t }).EXTRA_THREADS).toEqual(t);
  });
  it('FULL_GATE_SWEEP (E3) defaults false and only strict `true` enables it — the v3 `!== true` gate', () => {
    expect(new Configs({ reportPath: 'r.md' }).FULL_GATE_SWEEP).toBe(false);
    expect(new Configs({ reportPath: 'r.md', fullGateSweep: true }).FULL_GATE_SWEEP).toBe(true);
    expect(new Configs({ reportPath: 'r.md', fullGateSweep: 'yes' }).FULL_GATE_SWEEP).toBe(false);
    expect(new Configs({ reportPath: 'r.md', fullGateSweep: 1 }).FULL_GATE_SWEEP).toBe(false);
  });
});

describe('Configs — VERIFY / MANIFEST-VERIFY mode config', () => {
  it('defaults VOTES to 1, QUESTION to "", MAX_CLAIMS to 96 (E2 full-coverage default — was 24 in the source)', () => {
    const c = new Configs({ manifestPath: 'm.md' });
    expect(c.VOTES).toBe(1);
    expect(c.QUESTION).toBe('');
    expect(c.MAX_CLAIMS).toBe(96);
  });
  it('defaults SOLO_THRESHOLD to 8 (E2 batching gate); args.soloThreshold overrides', () => {
    expect(new Configs({ manifestPath: 'm.md' }).SOLO_THRESHOLD).toBe(8);
    expect(new Configs({ manifestPath: 'm.md', soloThreshold: 20 }).SOLO_THRESHOLD).toBe(20);
    expect(new Configs({ manifestPath: 'm.md', soloThreshold: 0 }).SOLO_THRESHOLD).toBe(0);
    expect(new Configs({ manifestPath: 'm.md', soloThreshold: 'nope' }).SOLO_THRESHOLD).toBe(8);
  });
  it('clamps votes to a positive integer, defaulting otherwise', () => {
    expect(new Configs({ manifestPath: 'm.md', votes: 3 }).VOTES).toBe(3);
    expect(new Configs({ manifestPath: 'm.md', votes: 0 }).VOTES).toBe(1);
    expect(new Configs({ manifestPath: 'm.md', votes: -1 }).VOTES).toBe(1);
  });
  it('reads question and maxClaims (override still honored)', () => {
    const c = new Configs({ manifestPath: 'm.md', question: 'is X safe?', maxClaims: 5 });
    expect(c.QUESTION).toBe('is X safe?');
    expect(c.MAX_CLAIMS).toBe(5);
  });
});

describe('Configs — INVESTIGATE mode config', () => {
  it('defaults SCOPE to null, MAX_WAVES to 3, MAX_LANES to 5, REPORT_OUT to null', () => {
    const c = new Configs({ goal: 'g' });
    expect(c.SCOPE).toBeNull();
    expect(c.MAX_WAVES).toBe(3);
    expect(c.MAX_LANES).toBe(5);
    expect(c.REPORT_OUT).toBeNull();
  });
  it('defaults LENSES to the 3 canonical DIRECT/SKEPTIC/BLAST-RADIUS lenses', () => {
    const c = new Configs({ goal: 'g' });
    expect(c.LENSES).toHaveLength(3);
    expect(c.LENSES[0]).toMatch(/DIRECT/);
    expect(c.LENSES[1]).toMatch(/SKEPTIC/);
    expect(c.LENSES[2]).toMatch(/BLAST-RADIUS/);
  });
  it('honors an explicit non-empty lenses array', () => {
    const c = new Configs({ goal: 'g', lenses: ['ONE — a', 'TWO — b'] });
    expect(c.LENSES).toEqual(['ONE — a', 'TWO — b']);
  });
  it('honors scope/maxWaves/maxLanes/reportOut overrides', () => {
    const c = new Configs({
      goal: 'g',
      scope: ['src/a.ts'],
      maxWaves: 5,
      maxLanes: 2,
      reportOut: 'out.md',
    });
    expect(c.SCOPE).toEqual(['src/a.ts']);
    expect(c.MAX_WAVES).toBe(5);
    expect(c.MAX_LANES).toBe(2);
    expect(c.REPORT_OUT).toBe('out.md');
  });
});

describe('Configs — the 17-seat TIER/EFFORT defaults (verbatim off the source)', () => {
  it('seeds every seat model default', () => {
    const c = new Configs({ reportPath: 'r.md' });
    expect(c.TIER.scout).toBe('sonnet');
    expect(c.TIER.threadWalker).toBe('sonnet');
    expect(c.TIER.sliceSensor).toBe('haiku');
    expect(c.TIER.gateSweep).toBe('haiku'); // shares sensorModel with sliceSensor
    expect(c.TIER.securityAuditor).toBe('sonnet');
    expect(c.TIER.anomalyJudge).toBe('sonnet');
    expect(c.TIER.territoryDigest).toBe('sonnet');
    expect(c.TIER.secondOpinion).toBe('opus');
    expect(c.TIER.finalJudge).toBe('opus');
    expect(c.TIER.fold).toBe('sonnet');
    expect(c.TIER.claimExtractor).toBe('sonnet');
    expect(c.TIER.claimVerifier).toBe('sonnet');
    expect(c.TIER.consistencyJudge).toBe('sonnet'); // shares verifierModel with claimExtractor/claimVerifier
    expect(c.TIER.probe).toBe('sonnet');
    expect(c.TIER.brainer).toBe('opus');
    expect(c.TIER.claimAuditor).toBe('haiku');
    expect(c.TIER.synthesiser).toBe('sonnet');
  });
  it('seeds every seat effort default — "high" where the source passes no explicit effort key', () => {
    const c = new Configs({ reportPath: 'r.md' });
    expect(c.EFFORT.scout).toBe('high');
    expect(c.EFFORT.threadWalker).toBe('high');
    expect(c.EFFORT.sliceSensor).toBe('medium');
    expect(c.EFFORT.gateSweep).toBe('medium');
    expect(c.EFFORT.securityAuditor).toBe('xhigh');
    expect(c.EFFORT.anomalyJudge).toBe('high');
    expect(c.EFFORT.territoryDigest).toBe('high');
    expect(c.EFFORT.secondOpinion).toBe('high');
    expect(c.EFFORT.finalJudge).toBe('high');
    expect(c.EFFORT.fold).toBe('high');
    expect(c.EFFORT.claimExtractor).toBe('xhigh');
    expect(c.EFFORT.claimVerifier).toBe('xhigh');
    expect(c.EFFORT.consistencyJudge).toBe('xhigh');
    expect(c.EFFORT.probe).toBe('xhigh');
    expect(c.EFFORT.brainer).toBe('xhigh');
    expect(c.EFFORT.claimAuditor).toBe('medium'); // hardcoded in source (no arg)
    expect(c.EFFORT.synthesiser).toBe('xhigh'); // hardcoded in source (no arg)
  });
  it('honors every legacy per-seat arg', () => {
    const c = new Configs({
      reportPath: 'r.md',
      scoutModel: 'opus',
      walkerModel: 'haiku',
      sensorModel: 'sonnet',
      sensorEffort: 'high',
      judgeModel: 'opus',
      digestModel: 'haiku',
      foldModel: 'opus',
      securityModel: 'haiku',
      securityEffort: 'high',
      verifierModel: 'opus',
      verifierEffort: 'high',
      probeModel: 'haiku',
      probeEffort: 'medium',
      auditModel: 'sonnet',
      synthModel: 'opus',
      finalJudgeModel: 'opus',
      securityEscalateModel: 'opus',
      brainerModel: 'opus',
      brainerEffort: 'high',
    });
    expect(c.TIER.scout).toBe('opus');
    expect(c.TIER.threadWalker).toBe('haiku');
    expect(c.TIER.sliceSensor).toBe('sonnet');
    expect(c.TIER.gateSweep).toBe('sonnet');
    expect(c.EFFORT.sliceSensor).toBe('high');
    expect(c.TIER.anomalyJudge).toBe('opus');
    expect(c.TIER.territoryDigest).toBe('haiku');
    expect(c.TIER.fold).toBe('opus');
    expect(c.TIER.securityAuditor).toBe('haiku');
    expect(c.EFFORT.securityAuditor).toBe('high');
    expect(c.TIER.claimExtractor).toBe('opus');
    expect(c.TIER.claimVerifier).toBe('opus');
    expect(c.TIER.consistencyJudge).toBe('opus');
    expect(c.EFFORT.claimVerifier).toBe('high');
    expect(c.TIER.probe).toBe('haiku');
    expect(c.EFFORT.probe).toBe('medium');
    expect(c.TIER.claimAuditor).toBe('sonnet');
    expect(c.TIER.synthesiser).toBe('opus');
    expect(c.TIER.brainer).toBe('opus');
    expect(c.EFFORT.brainer).toBe('high');
  });
  it('holds the fixed SENSOR_ESCALATE constant (sonnet, no arg knob)', () =>
    expect(new Configs({ reportPath: 'r.md' }).SENSOR_ESCALATE).toBe('sonnet'));
  it('holds the fixed doc path constants', () => {
    const c = new Configs({ reportPath: 'r.md' });
    expect(c.WALKER_DOC).toBe('.claude/commands/wave/walker.md');
    expect(c.SECURITY_DOC).toBe('.claude/commands/audit/security.md');
  });
});

describe('Configs — per-seat `agents` override', () => {
  it('applies no overrides when agents is absent, null, or undefined', () => {
    expect(new Configs({ reportPath: 'r.md' }).TIER.scout).toBe('sonnet');
    expect(new Configs({ reportPath: 'r.md', agents: null }).TIER.scout).toBe('sonnet');
    expect(new Configs({ reportPath: 'r.md', agents: undefined }).TIER.scout).toBe('sonnet');
  });
  it('overrides exactly the named seat/field, leaving every other seat at its default', () => {
    const c = new Configs({
      reportPath: 'r.md',
      agents: { scout: { model: 'opus' }, anomalyJudge: { effort: 'xhigh' } },
    });
    expect(c.TIER.scout).toBe('opus');
    expect(c.EFFORT.anomalyJudge).toBe('xhigh');
    expect(c.EFFORT.scout).toBe('high');
    expect(c.TIER.anomalyJudge).toBe('sonnet');
    expect(c.TIER.threadWalker).toBe('sonnet');
  });
  it('a seat override may set model and effort together', () => {
    const c = new Configs({
      reportPath: 'r.md',
      agents: { probe: { model: 'haiku', effort: 'low' } },
    });
    expect(c.TIER.probe).toBe('haiku');
    expect(c.EFFORT.probe).toBe('low');
  });
  it('a seat override accepts the "max" effort (rare, founder-invoked per CLAUDE.md, never a default)', () => {
    const c = new Configs({ reportPath: 'r.md', agents: { probe: { effort: 'max' } } });
    expect(c.EFFORT.probe).toBe('max');
  });
  it('precedence: agents.<seat>.model > legacy <seat>Model arg > TIER default', () => {
    const c = new Configs({
      reportPath: 'r.md',
      scoutModel: 'haiku',
      agents: { scout: { model: 'opus' } },
    });
    expect(c.TIER.scout).toBe('opus');
  });
  it('throws on an unknown seat name, listing the valid seats', () => {
    expect(() => new Configs({ reportPath: 'r.md', agents: { bogus: { model: 'opus' } } })).toThrow(
      /unknown agent seat "bogus"/,
    );
    expect(() => new Configs({ reportPath: 'r.md', agents: { bogus: { model: 'opus' } } })).toThrow(
      /scout/,
    );
  });
  it('throws on an invalid model value', () =>
    expect(() => new Configs({ reportPath: 'r.md', agents: { scout: { model: 'gpt4' } } })).toThrow(
      /model must be one of/,
    ));
  it('throws on an invalid effort value', () =>
    expect(
      () => new Configs({ reportPath: 'r.md', agents: { scout: { effort: 'ultra' } } }),
    ).toThrow(/effort must be one of/));
  it('throws when agents is not a plain object (string/array)', () => {
    expect(() => new Configs({ reportPath: 'r.md', agents: 'nope' })).toThrow(
      /agents must be an object/,
    );
    expect(() => new Configs({ reportPath: 'r.md', agents: [] })).toThrow(
      /agents must be an object/,
    );
  });
  it('throws when a seat override is not a plain object', () => {
    expect(() => new Configs({ reportPath: 'r.md', agents: { scout: 'sonnet' } })).toThrow(
      /agents\.scout must be an object/,
    );
    expect(() => new Configs({ reportPath: 'r.md', agents: { scout: null } })).toThrow(
      /agents\.scout must be an object/,
    );
  });
  it('warns loudly (never throws) when a FRONTIER seat is downgraded below opus', () => {
    const warnings: unknown[] = [];
    (globalThis as unknown as { log: (m?: unknown) => void }).log = (m) => warnings.push(m);
    try {
      for (const seat of ['brainer', 'finalJudge', 'secondOpinion']) {
        warnings.length = 0;
        const c = new Configs({
          reportPath: 'r.md',
          goal: 'g',
          agents: { [seat]: { model: 'sonnet' } },
        });
        expect(c.TIER[seat]).toBe('sonnet');
        expect(
          warnings.some((w) => typeof w === 'string' && /frontier-judgment seat/.test(w)),
        ).toBe(true);
      }
    } finally {
      (globalThis as unknown as { log: () => void }).log = () => {};
    }
  });
  it('never warns for a non-frontier seat downgrade, or when a frontier seat keeps opus', () => {
    const warnings: unknown[] = [];
    (globalThis as unknown as { log: (m?: unknown) => void }).log = (m) => warnings.push(m);
    try {
      new Configs({ reportPath: 'r.md', agents: { scout: { model: 'haiku' } } });
      new Configs({ reportPath: 'r.md', agents: { brainer: { model: 'opus' } } });
      expect(warnings.length).toBe(0);
    } finally {
      (globalThis as unknown as { log: () => void }).log = () => {};
    }
  });
});

describe("Configs — charter (the walk's caller-supplied duty note)", () => {
  it('defaults to "" (absent or null)', () => {
    expect(new Configs({ reportPath: 'r.md' }).CHARTER).toBe('');
    expect(new Configs({ reportPath: 'r.md', charter: null }).CHARTER).toBe('');
  });
  it('carries a string charter verbatim', () =>
    expect(new Configs({ reportPath: 'r.md', charter: 'audit the export path' }).CHARTER).toBe(
      'audit the export path',
    ));
  it('throws on a non-string charter', () => {
    expect(() => new Configs({ reportPath: 'r.md', charter: 42 })).toThrow(
      /charter must be a string/,
    );
    expect(() => new Configs({ reportPath: 'r.md', charter: ['x'] })).toThrow(
      /charter must be a string/,
    );
    expect(() => new Configs({ reportPath: 'r.md', charter: { note: 'x' } })).toThrow(
      /charter must be a string/,
    );
  });
});

// INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — registry validation is the load-
// bearing piece: a malformed entry must throw loudly, never silently disarm territory matching.
describe('Configs — INVARIANTS (the invariant registry, args.invariants)', () => {
  const VALID_ENTRY = {
    id: 'ENGINE-REGRADE',
    law: 'Narrative clinical record is engine-frozen — app-cortex/CLAUDE.md:159',
    territory: ['app-cortex/src/app_cortex/db/analysis/**'],
    triggers: ['diff touches territory'],
    exemplars: ['chunk_cache.py:38'],
    huntBrief: 'walk every reuse/skip/cache gate for currency, not existence',
  };

  it('THE FLOOR: defaults to [] when args.invariants is absent or null', () => {
    expect(new Configs({ reportPath: 'r.md' }).INVARIANTS).toEqual([]);
    expect(new Configs({ reportPath: 'r.md', invariants: null }).INVARIANTS).toEqual([]);
  });
  it('an empty array is also the floor', () =>
    expect(new Configs({ reportPath: 'r.md', invariants: [] }).INVARIANTS).toEqual([]));
  it('parses a well-formed entry, defaulting absent triggers/exemplars to []', () => {
    const c = new Configs({
      reportPath: 'r.md',
      invariants: [{ id: 'X', law: 'L', territory: ['a/**'], huntBrief: 'H' }],
    });
    expect(c.INVARIANTS).toEqual([
      { id: 'X', law: 'L', territory: ['a/**'], triggers: [], exemplars: [], huntBrief: 'H' },
    ]);
  });
  it('carries a fully-populated entry verbatim', () => {
    const c = new Configs({ reportPath: 'r.md', invariants: [VALID_ENTRY] });
    expect(c.INVARIANTS).toEqual([VALID_ENTRY]);
  });
  it('parses multiple entries, preserving order', () => {
    const c = new Configs({
      reportPath: 'r.md',
      invariants: [VALID_ENTRY, { ...VALID_ENTRY, id: 'FROZEN-NARRATIVE' }],
    });
    expect(c.INVARIANTS.map((i) => i.id)).toEqual(['ENGINE-REGRADE', 'FROZEN-NARRATIVE']);
  });
  it('throws when invariants is not an array', () => {
    expect(() => new Configs({ reportPath: 'r.md', invariants: 'nope' })).toThrow(
      /invariants must be an array/,
    );
    expect(() => new Configs({ reportPath: 'r.md', invariants: { id: 'X' } })).toThrow(
      /invariants must be an array/,
    );
  });
  it('throws naming the index when an entry is not an object', () => {
    expect(() => new Configs({ reportPath: 'r.md', invariants: ['nope'] })).toThrow(
      /invariants\[0\] must be an object/,
    );
    expect(() => new Configs({ reportPath: 'r.md', invariants: [VALID_ENTRY, 42] })).toThrow(
      /invariants\[1\] must be an object/,
    );
  });
  it('throws when id/law/huntBrief is missing or empty', () => {
    expect(
      () => new Configs({ reportPath: 'r.md', invariants: [{ ...VALID_ENTRY, id: '' }] }),
    ).toThrow(/invariants\[0\]\.id must be a non-empty string/);
    expect(
      () => new Configs({ reportPath: 'r.md', invariants: [{ ...VALID_ENTRY, law: undefined }] }),
    ).toThrow(/invariants\[0\]\.law must be a non-empty string/);
    expect(
      () => new Configs({ reportPath: 'r.md', invariants: [{ ...VALID_ENTRY, huntBrief: 123 }] }),
    ).toThrow(/invariants\[0\]\.huntBrief must be a non-empty string/);
  });
  it('throws when territory is missing, empty, or not an array of strings — an empty territory can never glob-match anything', () => {
    expect(
      () =>
        new Configs({ reportPath: 'r.md', invariants: [{ ...VALID_ENTRY, territory: undefined }] }),
    ).toThrow(/invariants\[0\]\.territory must be a non-empty array/);
    expect(
      () => new Configs({ reportPath: 'r.md', invariants: [{ ...VALID_ENTRY, territory: [] }] }),
    ).toThrow(/invariants\[0\]\.territory must be a non-empty array/);
    expect(
      () =>
        new Configs({ reportPath: 'r.md', invariants: [{ ...VALID_ENTRY, territory: 'a/**' }] }),
    ).toThrow(/invariants\[0\]\.territory must be a non-empty array/);
    expect(
      () =>
        new Configs({ reportPath: 'r.md', invariants: [{ ...VALID_ENTRY, territory: [1, 2] }] }),
    ).toThrow(/invariants\[0\]\.territory must be a non-empty array/);
  });
  it('throws when triggers or exemplars is present but not an array of strings', () => {
    expect(
      () => new Configs({ reportPath: 'r.md', invariants: [{ ...VALID_ENTRY, triggers: 'nope' }] }),
    ).toThrow(/invariants\[0\]\.triggers must be an array of strings/);
    expect(
      () => new Configs({ reportPath: 'r.md', invariants: [{ ...VALID_ENTRY, exemplars: [1] }] }),
    ).toThrow(/invariants\[0\]\.exemplars must be an array of strings/);
  });
});

describe('Configs — INVARIANTS_DOC constant + invariantHunter/coverageCritic TIER/EFFORT defaults', () => {
  it('holds the fixed doc path constant, alongside WALKER_DOC/SECURITY_DOC', () =>
    expect(new Configs({ reportPath: 'r.md' }).INVARIANTS_DOC).toBe(
      '.claude/commands/wave/walker-invariants.md',
    ));
  it("defaults invariantHunter/coverageCritic to sonnet/high (securityAuditor's own tier precedent)", () => {
    const c = new Configs({ reportPath: 'r.md' });
    expect(c.TIER.invariantHunter).toBe('sonnet');
    expect(c.EFFORT.invariantHunter).toBe('high');
    expect(c.TIER.coverageCritic).toBe('sonnet');
    expect(c.EFFORT.coverageCritic).toBe('high');
  });
  it('retunable via the generic `agents.<seat>` override, like every other non-legacy seat', () => {
    const c = new Configs({
      reportPath: 'r.md',
      agents: {
        invariantHunter: { model: 'opus', effort: 'xhigh' },
        coverageCritic: { model: 'haiku' },
      },
    });
    expect(c.TIER.invariantHunter).toBe('opus');
    expect(c.EFFORT.invariantHunter).toBe('xhigh');
    expect(c.TIER.coverageCritic).toBe('haiku');
  });
  it('neither seat is a FRONTIER seat — no warning on a non-opus override', () => {
    const warnings: unknown[] = [];
    (globalThis as unknown as { log: (m?: unknown) => void }).log = (m) => warnings.push(m);
    try {
      new Configs({ reportPath: 'r.md', agents: { invariantHunter: { model: 'haiku' } } });
      expect(warnings.length).toBe(0);
    } finally {
      (globalThis as unknown as { log: () => void }).log = () => {};
    }
  });
});

// WALK TELEMETRY (DEBUG STEP, tmp/walker-debug-design.md) — CONFIG.debug mirrors rr's config.ts:383
// debugger flag: DEFAULT ON. Strict-typed reader discipline (Configs.bool) matches this file's own
// FULL_GATE_SWEEP/charter conventions — a non-boolean truthy value never accidentally flips the flag.
describe('Configs — debug (WALK TELEMETRY debug step)', () => {
  it('defaults to true when args.debug is absent', () =>
    expect(new Configs({ reportPath: 'r.md' }).debug).toBe(true));
  it('debug: false is honored', () =>
    expect(new Configs({ reportPath: 'r.md', debug: false }).debug).toBe(false));
  it('a non-boolean truthy value (e.g. the string "false") does NOT accidentally flip it — strict-typed-reader discipline', () => {
    expect(new Configs({ reportPath: 'r.md', debug: 'false' }).debug).toBe(true);
    expect(new Configs({ reportPath: 'r.md', debug: 0 }).debug).toBe(true);
    expect(new Configs({ reportPath: 'r.md', debug: 1 }).debug).toBe(true);
    expect(new Configs({ reportPath: 'r.md', debug: null }).debug).toBe(true);
  });
  it('DEBUG_PATH derives from REPORT_PATH the same way LEDGER_PATH does', () => {
    const c = new Configs({ reportPath: 'docs/dev/waves/w/report.md' });
    expect(c.DEBUG_PATH).toBe('docs/dev/waves/w/walker-debug.json');
  });
  it('an explicit debugPath overrides the derived one', () => {
    const c = new Configs({ reportPath: 'r.md', debugPath: 'custom-debug.json' });
    expect(c.DEBUG_PATH).toBe('custom-debug.json');
  });
  it('DEBUG_PATH is null when there is no REPORT_PATH to derive from and no explicit debugPath', () => {
    expect(new Configs({ goal: 'g' }).DEBUG_PATH).toBeNull();
  });
});

// PROJECT PROFILE (universal-bundle refactor) — securityStakesLine mirrors stakesLine exactly: an
// optional string field on ProjectProfile, validated by parseProject's shared `str()` helper (absent →
// undefined, the floor; wrong type → throws loudly, same discipline as every other project.* field).
describe('Configs — PROJECT PROFILE securityStakesLine (args.project)', () => {
  it('undefined when args.project is absent', () =>
    expect(new Configs({ reportPath: 'r.md' }).PROJECT).toBeUndefined());
  it('carries a supplied securityStakesLine verbatim, alongside stakesLine', () => {
    const c = new Configs({
      reportPath: 'r.md',
      project: { stakesLine: 'a dead line', securityStakesLine: 'Therapy data is sacred' },
    });
    expect(c.PROJECT?.stakesLine).toBe('a dead line');
    expect(c.PROJECT?.securityStakesLine).toBe('Therapy data is sacred');
  });
  it('defaults to undefined when the profile omits it (other fields still parse)', () => {
    const c = new Configs({ reportPath: 'r.md', project: { repoRoot: 'apps/backend' } });
    expect(c.PROJECT?.securityStakesLine).toBeUndefined();
    expect(c.PROJECT?.repoRoot).toBe('apps/backend');
  });
  it('throws on a wrong-type securityStakesLine, same as stakesLine\'s own validation', () => {
    expect(
      () => new Configs({ reportPath: 'r.md', project: { securityStakesLine: 42 } }),
    ).toThrow(/args\.project\.securityStakesLine must be a string/);
    expect(
      () => new Configs({ reportPath: 'r.md', project: { securityStakesLine: ['x'] } }),
    ).toThrow(/args\.project\.securityStakesLine must be a string/);
    expect(
      () => new Configs({ reportPath: 'r.md', project: { stakesLine: 42 } }),
    ).toThrow(/args\.project\.stakesLine must be a string/);
  });
});
