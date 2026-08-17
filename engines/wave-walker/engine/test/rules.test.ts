import { describe, expect, it } from 'vitest';
import {
  baseType,
  computeAnomalies,
  computeInvariantAnomalies,
  groupByResource,
  ruleCounts,
  zipCards,
} from '../src/rules.js';
import type {
  Anomaly,
  Card,
  GateCardWithFile,
  InvariantHunterOut,
  Slice,
  SliceJob,
  SlicesOut,
  UndeclaredRead,
} from '../src/types/index.js';

const AUTH_RULE =
  'auth-doc.md § Auth Pattern (live, scout-extracted): "Role fences (SACRED...) OWNER ... ELEVATED ..."';

// GateFenceConfig fixture — neutral vocab (not tied to any project), threaded through every
// computeAnomalies call below via computeAnomaliesT so R6-mandated-fence/R7 label wording stay exercised
// without any single call site repeating it.
const FENCE_CONFIG = {
  fencedResourceClasses: ['alpha', 'beta', 'gamma'],
  ownerRole: 'OWNER',
  fenceLabels: { org: 'org', ownership: 'ownership' },
};
const computeAnomaliesT = (
  cards: Card[],
  reads: UndeclaredRead[],
  gates: GateCardWithFile[],
): Anomaly[] => computeAnomalies(cards, reads, gates, AUTH_RULE, FENCE_CONFIG);

const emptyCard = (id: string): Card => ({
  id,
  ownerType: 'Owner',
  field: 'field',
  apis: [],
  sdl: null,
  feTypes: [],
  consumers: [],
  danglingRefs: [],
  sidesCovered: [],
});

describe('baseType — GraphQL/TS type-token normalization', () => {
  it('maps scalar synonyms to their structural base', () => {
    expect(baseType('String')).toBe('string');
    expect(baseType('ID')).toBe('string');
    expect(baseType('Int')).toBe('number');
    expect(baseType('Float')).toBe('number');
    expect(baseType('Boolean')).toBe('boolean');
    expect(baseType('JSONB')).toBe('object');
  });
  it('strips Maybe<>, Scalars[...], nullability markers, and null/undefined unions', () => {
    expect(baseType("Maybe<Scalars['String']['output']>")).toBe('string');
    expect(baseType('string | null')).toBe('string');
    expect(baseType('string?')).toBe('string');
    expect(baseType('string!')).toBe('string');
  });
  it('classifies object-shaped tokens', () => {
    expect(baseType('Record<string, number>')).toBe('object');
    expect(baseType('{ a: string }')).toBe('object');
    expect(baseType('Array<string>')).toBe('object');
    expect(baseType('string[]')).toBe('object');
  });
  it('passes through an unrecognized token unchanged', () => {
    expect(baseType('CustomEnum')).toBe('customenum');
  });
});

describe('groupByResource', () => {
  it('groups gates by resource, defaulting to "other", preserving insertion order', () => {
    const gates: GateCardWithFile[] = [
      { id: 'Query.a', anchor: 'a:1', file: 'a.ts', resource: 'session' },
      { id: 'Query.b', anchor: 'b:1', file: 'b.ts' },
      { id: 'Query.c', anchor: 'c:1', file: 'c.ts', resource: 'session' },
    ];
    const g = groupByResource(gates);
    expect(Object.keys(g)).toEqual(['session', 'other']);
    expect(g.session.map((x) => x.id)).toEqual(['Query.a', 'Query.c']);
    expect(g.other.map((x) => x.id)).toEqual(['Query.b']);
  });
});

describe('ruleCounts', () => {
  it('tallies anomalies per rule', () => {
    const counts = ruleCounts([
      {
        id: 'R1-1',
        rule: 'R1',
        ruleName: '',
        detail: '',
        anchors: [],
        severityHint: 'med',
        cardId: null,
      },
      {
        id: 'R1-2',
        rule: 'R1',
        ruleName: '',
        detail: '',
        anchors: [],
        severityHint: 'med',
        cardId: null,
      },
      {
        id: 'R7-3',
        rule: 'R7',
        ruleName: '',
        detail: '',
        anchors: [],
        severityHint: 'critical',
        cardId: null,
      },
    ]);
    expect(counts).toEqual({ R1: 2, R7: 1 });
  });
});

describe('zipCards — mechanical zero-token zip', () => {
  it('zips a producer + a consumer slice from separate jobs onto one card, tagging sidesCovered', () => {
    const fields = [{ id: 'Owner.field', ownerType: 'Owner', field: 'field' }];
    const jobs: SliceJob[] = [
      { jobId: 'p1', kind: 'producer', files: ['be.ts'], fieldIds: ['Owner.field'] },
      { jobId: 'c1', kind: 'consumer', files: ['fe.ts'], fieldIds: ['Owner.field'] },
    ];
    const producerSlice: Slice = {
      fieldId: 'Owner.field',
      producer: { anchor: 'be.ts:1', encoding: 'raw', valueLiterals: ['A'] },
    };
    const consumerSlice: Slice = {
      fieldId: 'Owner.field',
      consumers: [{ anchor: 'fe.ts:1', context: 'production' }],
    };
    const sliceResults: SlicesOut[] = [
      { jobId: 'p1', slices: [producerSlice] },
      { jobId: 'c1', slices: [consumerSlice] },
    ];
    const { cards, unsensed } = zipCards(fields, jobs, sliceResults);
    expect(cards).toHaveLength(1);
    expect(cards[0].producer?.anchor).toBe('be.ts:1');
    expect(cards[0].consumers).toEqual([{ anchor: 'fe.ts:1', context: 'production' }]);
    expect(cards[0].sidesCovered.sort()).toEqual(['consumer', 'producer']);
    expect(unsensed).toEqual([]);
  });

  it('merges valueLiterals across multiple producer slices for the same field (deduped)', () => {
    const fields = [{ id: 'Owner.field', ownerType: 'Owner', field: 'field' }];
    const jobs: SliceJob[] = [
      { jobId: 'p1', kind: 'producer', files: ['a.ts'], fieldIds: ['Owner.field'] },
      { jobId: 'p2', kind: 'producer', files: ['b.ts'], fieldIds: ['Owner.field'] },
    ];
    const sliceResults: SlicesOut[] = [
      {
        jobId: 'p1',
        slices: [
          { fieldId: 'Owner.field', producer: { anchor: 'a.ts:1', valueLiterals: ['A', 'B'] } },
        ],
      },
      {
        jobId: 'p2',
        slices: [
          { fieldId: 'Owner.field', producer: { anchor: 'b.ts:1', valueLiterals: ['B', 'C'] } },
        ],
      },
    ];
    const { cards } = zipCards(fields, jobs, sliceResults);
    // FIRST producer anchor wins (source: `if (s.producer && !c.producer) c.producer = s.producer`); only
    // valueLiterals merge on a SECOND producer hit.
    expect(cards[0].producer?.anchor).toBe('a.ts:1');
    expect(cards[0].producer?.valueLiterals?.sort()).toEqual(['A', 'B', 'C']);
  });

  it('reports a field with zero sensor coverage as unsensed', () => {
    const fields = [{ id: 'Owner.dead', ownerType: 'Owner', field: 'dead' }];
    const { cards, unsensed } = zipCards(fields, [], []);
    expect(cards).toHaveLength(1);
    expect(cards[0].sidesCovered).toEqual([]);
    expect(unsensed).toEqual(['Owner.dead']);
  });

  it('unions droppedFieldIds (the sensor-cap casualties) into unsensed, deduped', () => {
    const fields = [{ id: 'Owner.dead', ownerType: 'Owner', field: 'dead' }];
    const { unsensed } = zipCards(fields, [], [], ['Owner.dead', 'Owner.other']);
    expect(unsensed.sort()).toEqual(['Owner.dead', 'Owner.other']);
  });
});

describe('computeAnomalies — R1 orphan producer', () => {
  it('flags "produced but never exposed in SDL" when sdl is absent', () => {
    const c = { ...emptyCard('Owner.a'), producer: { anchor: 'a.ts:1' } };
    const [x] = computeAnomaliesT([c], [], []);
    expect(x.rule).toBe('R1');
    expect(x.severityHint).toBe('med');
    expect(x.detail).toBe('Owner.a: produced but never exposed in SDL');
    expect(x.cardId).toBe('Owner.a');
  });
  it('flags "declared in SDL but never selected" when sdl is present but no feSelection', () => {
    const c = {
      ...emptyCard('Owner.b'),
      producer: { anchor: 'a.ts:1' },
      sdl: { anchor: 's:1', typeToken: 'String' },
    };
    const [x] = computeAnomaliesT([c], [], []);
    expect(x.detail).toBe('Owner.b: declared in SDL but never selected by any FE query');
  });
  it('flags "shipped and selected but read by no production consumer", naming non-production refs', () => {
    const c: Card = {
      ...emptyCard('Owner.c'),
      producer: { anchor: 'a.ts:1' },
      sdl: { anchor: 's:1', typeToken: 'String' },
      feSelection: { anchor: 'q:1' },
      consumers: [{ anchor: 'test.ts:1', context: 'test' }],
    };
    const [x] = computeAnomaliesT([c], [], []);
    expect(x.detail).toBe(
      'Owner.c: shipped and selected but read by no production consumer (1 non-production ref(s) only)',
    );
  });
  it('does NOT flag when a production consumer exists', () => {
    const c: Card = {
      ...emptyCard('Owner.d'),
      producer: { anchor: 'a.ts:1' },
      consumers: [{ anchor: 'fe.ts:1', context: 'production' }],
    };
    expect(computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R1')).toHaveLength(0);
  });
});

describe('computeAnomalies — R2 phantom consumer', () => {
  it('flags a card with consumers but no producer, absent from SDL', () => {
    const c: Card = { ...emptyCard('Owner.e'), consumers: [{ anchor: 'fe.ts:1' }] };
    const [x] = computeAnomaliesT([c], [], []);
    expect(x.rule).toBe('R2');
    expect(x.severityHint).toBe('high');
    expect(x.detail).toBe(
      'Owner.e: consumed at 1 site(s) but no producer emits it (absent from SDL)',
    );
  });
  it('flags an undeclared BE return as med severity, resolver-flavored wording', () => {
    const reads: UndeclaredRead[] = [{ side: 'be', property: 'secret', anchor: 'r.ts:9' }];
    const [x] = computeAnomaliesT([], reads, []);
    expect(x.rule).toBe('R2');
    expect(x.severityHint).toBe('med');
    expect(x.detail).toBe('resolver returns undeclared field "secret" at r.ts:9');
    expect(x.cardId).toBeNull();
  });
  it('flags an undeclared FE read as high severity, with the verbatim expr appended', () => {
    const reads: UndeclaredRead[] = [
      { side: 'fe', property: 'status', anchor: 'c.tsx:4', expr: 'x.status ?? y.status' },
    ];
    const [x] = computeAnomaliesT([], reads, []);
    expect(x.severityHint).toBe('high');
    expect(x.detail).toBe('FE reads undeclared field "status" at c.tsx:4 (x.status ?? y.status)');
  });
});

describe('computeAnomalies — R3 encoding mismatch', () => {
  it('flags a double-encode JSON.parse(JSON.stringify(...)) regardless of the INCOMPAT table', () => {
    const c: Card = {
      ...emptyCard('Owner.f'),
      producer: { anchor: 'a.ts:1', encoding: 'object' },
      consumers: [{ anchor: 'fe.ts:2', decodeExpr: 'JSON.parse(JSON.stringify(x))' }],
    };
    const [x] = computeAnomaliesT([c], [], []);
    expect(x.rule).toBe('R3');
    expect(x.severityHint).toBe('high');
    expect(x.detail).toBe(
      'Owner.f: double-encode JSON.parse(JSON.stringify(...)) at fe.ts:2 — on a object value returns the input unchanged, never a parsed object',
    );
  });
  it('flags an INCOMPAT-table mismatch (json-string produced, object-index consumed)', () => {
    const c: Card = {
      ...emptyCard('Owner.g'),
      producer: { anchor: 'a.ts:1', encoding: 'json-string' },
      consumers: [{ anchor: 'fe.ts:3', decode: 'object-index' }],
    };
    const [x] = computeAnomaliesT([c], [], []);
    expect(x.detail).toBe(
      'Owner.g: produced as json-string (a.ts:1) but consumed via object-index at fe.ts:3',
    );
  });
  it('does not flag a compatible encoding/decode pair', () => {
    const c: Card = {
      ...emptyCard('Owner.h'),
      producer: { anchor: 'a.ts:1', encoding: 'raw' },
      consumers: [{ anchor: 'fe.ts:4', decode: 'direct' }],
    };
    expect(computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R3')).toHaveLength(0);
  });
});

describe('computeAnomalies — R4 value-set mismatch', () => {
  it('flags a CASING-only mismatch as critical, naming it explicitly', () => {
    const c: Card = {
      ...emptyCard('Owner.i'),
      producer: { anchor: 'a.ts:1', valueLiterals: ['Active', 'Inactive'] },
      consumers: [{ anchor: 'fe.ts:5', comparedLiterals: ['active'] }],
    };
    const [x] = computeAnomaliesT([c], [], []);
    expect(x.rule).toBe('R4');
    expect(x.severityHint).toBe('critical');
    expect(x.detail).toBe(
      'Owner.i: consumer at fe.ts:5 compares against ["active"] which no producer emits — CASING mismatch of ["active"], branch permanently dead (produced: ["Active","Inactive"])',
    );
  });
  it('flags a non-casing mismatch as high, without the CASING clause', () => {
    const c: Card = {
      ...emptyCard('Owner.j'),
      producer: { anchor: 'a.ts:1', valueLiterals: ['Active'] },
      consumers: [{ anchor: 'fe.ts:6', comparedLiterals: ['Pending'] }],
    };
    const [x] = computeAnomaliesT([c], [], []);
    expect(x.severityHint).toBe('high');
    expect(x.detail).toBe(
      'Owner.j: consumer at fe.ts:6 compares against ["Pending"] which no producer emits (produced: ["Active"])',
    );
  });
  it('does not flag when every compared literal is produced', () => {
    const c: Card = {
      ...emptyCard('Owner.k'),
      producer: { anchor: 'a.ts:1', valueLiterals: ['Active'] },
      consumers: [{ anchor: 'fe.ts:7', comparedLiterals: ['Active'] }],
    };
    expect(computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R4')).toHaveLength(0);
  });
});

describe('computeAnomalies — R5 type drift', () => {
  it('flags a hand type disagreeing with the generated type at base-type level', () => {
    const c: Card = {
      ...emptyCard('Owner.l'),
      feTypes: [
        { anchor: 'gen.ts:1', typeToken: 'string', kind: 'generated' },
        { anchor: 'hand.ts:1', typeToken: 'number', kind: 'hand' },
      ],
    };
    const [x] = computeAnomaliesT([c], [], []);
    expect(x.rule).toBe('R5');
    expect(x.severityHint).toBe('med');
    expect(x.detail).toBe(
      'Owner.l: hand-typed "number" (hand.ts:1) vs generated "string" — base number vs string',
    );
  });
  it('falls back to the SDL type token when no generated type exists', () => {
    const c: Card = {
      ...emptyCard('Owner.m'),
      sdl: { anchor: 'sdl.ts:1', typeToken: 'Boolean' },
      feTypes: [{ anchor: 'hand.ts:2', typeToken: 'string', kind: 'hand' }],
    };
    const [x] = computeAnomaliesT([c], [], []);
    expect(x.detail).toBe(
      'Owner.m: hand-typed "string" (hand.ts:2) vs SDL "Boolean" — base string vs boolean',
    );
  });
  it('does not flag when base types agree', () => {
    const c: Card = {
      ...emptyCard('Owner.n'),
      feTypes: [
        { anchor: 'gen.ts:3', typeToken: 'String', kind: 'generated' },
        { anchor: 'hand.ts:3', typeToken: 'string', kind: 'hand' },
      ],
    };
    expect(computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R5')).toHaveLength(0);
  });
});

describe('computeAnomalies — R8 dangling reference', () => {
  it('flags a dangling ref', () => {
    const c: Card = {
      ...emptyCard('Owner.o'),
      danglingRefs: [{ ref: 'Foo.bar', anchor: 'x.ts:1' }],
    };
    const [x] = computeAnomaliesT([c], [], []);
    expect(x.rule).toBe('R8');
    expect(x.severityHint).toBe('med');
    expect(x.detail).toBe('Owner.o: "Foo.bar" at x.ts:1 resolves to nothing');
  });
});

describe('computeAnomalies — R6 gate outlier + mandated-fence violation', () => {
  it('flags a gate outlier when one gate in a resource class is fenced and a sibling is not', () => {
    const gates: GateCardWithFile[] = [
      {
        id: 'Query.a',
        anchor: 'a:1',
        file: 'f.ts',
        resource: 'alpha',
        ownershipFence: true,
        idArgs: ['id'],
      },
      {
        id: 'Query.b',
        anchor: 'b:1',
        file: 'f.ts',
        resource: 'alpha',
        ownershipFence: false,
        idArgs: ['id'],
      },
    ];
    const outliers = computeAnomaliesT([], [], gates).filter(
      (a) => a.ruleName === 'gate outlier',
    );
    expect(outliers).toHaveLength(1);
    expect(outliers[0].severityHint).toBe('high');
    expect(outliers[0].detail).toBe(
      'resource "alpha": Query.a enforce an ownership fence but Query.b do not — same class, weaker chain',
    );
  });
  it('flags a mandated-fence violation on a FENCED resource class when the configured owner role is admitted with no ownership fence', () => {
    const gates: GateCardWithFile[] = [
      {
        id: 'Query.alpha',
        anchor: 'p:1',
        file: 'f.ts',
        resource: 'alpha',
        idArgs: ['alphaId'],
        ownershipFence: false,
        rolesAllowed: ['OWNER', 'ELEVATED'],
      },
    ];
    const violations = computeAnomaliesT([], [], gates).filter(
      (a) => a.ruleName === 'mandated-fence violation',
    );
    expect(violations).toHaveLength(1);
    expect(violations[0].severityHint).toBe('critical');
    expect(violations[0].detail).toContain(
      'resource "alpha": Query.alpha admit OWNER with client-supplied id but enforce NO ownership fence',
    );
    expect(violations[0].detail).toContain(AUTH_RULE);
  });
  it('does not flag mandated-fence on a resource class outside fencedResourceClasses', () => {
    const gates: GateCardWithFile[] = [
      {
        id: 'Query.report',
        anchor: 'r:1',
        file: 'f.ts',
        resource: 'other',
        idArgs: ['id'],
        ownershipFence: false,
        rolesAllowed: ['OWNER'],
      },
    ];
    expect(
      computeAnomaliesT([], [], gates).filter(
        (a) => a.ruleName === 'mandated-fence violation',
      ),
    ).toHaveLength(0);
  });
  it('never fires mandated-fence when fenceConfig is absent (no ownerRole to match)', () => {
    const gates: GateCardWithFile[] = [
      {
        id: 'Query.alpha',
        anchor: 'p:1',
        file: 'f.ts',
        resource: 'alpha',
        idArgs: ['alphaId'],
        ownershipFence: false,
        rolesAllowed: ['OWNER'],
      },
    ];
    expect(
      computeAnomalies([], [], gates, AUTH_RULE).filter(
        (a) => a.ruleName === 'mandated-fence violation',
      ),
    ).toHaveLength(0);
  });
});

describe('computeAnomalies — R7 unfenced ID flow', () => {
  it('flags a client-supplied id reaching data access with neither fence, naming the configured org-fence label', () => {
    const gates: GateCardWithFile[] = [
      {
        id: 'Query.x',
        anchor: 'x:1',
        file: 'f.ts',
        idArgs: ['id'],
        orgFence: false,
        ownershipFence: false,
        chain: ['auth', 'load'],
      },
    ];
    const [x] = computeAnomaliesT([], [], gates).filter((a) => a.rule === 'R7');
    expect(x.severityHint).toBe('critical');
    expect(x.detail).toBe(
      'Query.x: client-supplied ["id"] reaches data access with neither org nor ownership fence (chain: auth → load)',
    );
  });
  it('does not flag when either fence is present', () => {
    const gates: GateCardWithFile[] = [
      {
        id: 'Query.y',
        anchor: 'y:1',
        file: 'f.ts',
        idArgs: ['id'],
        orgFence: true,
        ownershipFence: false,
      },
    ];
    expect(computeAnomaliesT([], [], gates).filter((a) => a.rule === 'R7')).toHaveLength(
      0,
    );
  });
  it('falls back to the generic "org" label when fenceConfig is absent', () => {
    const gates: GateCardWithFile[] = [
      { id: 'Query.z', anchor: 'z:1', file: 'f.ts', idArgs: ['id'], orgFence: false },
    ];
    const [x] = computeAnomalies([], [], gates, AUTH_RULE).filter((a) => a.rule === 'R7');
    expect(x.detail).toContain('neither org nor ownership fence');
  });
});

describe('computeAnomalies — sequential id numbering across the whole pass', () => {
  it('numbers ids off ONE global sequence (aseq), not a per-rule counter — id = rule + "-" + the Nth flag() call overall, in the source\'s exact iteration order (cards, then undeclaredReads, then gates)', () => {
    const cards: Card[] = [
      { ...emptyCard('Owner.a'), producer: { anchor: 'a:1' } }, // 1st flag() call → R1-1
      { ...emptyCard('Owner.b'), consumers: [{ anchor: 'b:1' }] }, // 2nd flag() call → R2-2
    ];
    const reads: UndeclaredRead[] = [{ side: 'be', property: 'p', anchor: 'r:1' }]; // 3rd flag() call → R2-3
    const gates: GateCardWithFile[] = [
      {
        id: 'Query.z',
        anchor: 'z:1',
        file: 'f.ts',
        idArgs: ['id'],
        orgFence: false,
        ownershipFence: false,
      },
    ]; // 4th flag() call → R7-4
    const anomalies = computeAnomaliesT(cards, reads, gates);
    expect(anomalies.map((a) => a.id)).toEqual(['R1-1', 'R2-2', 'R2-3', 'R7-4']);
  });
});

// Sparse-input fallbacks — the source's defensive `|| []` / `?? null` alternates on NON-security rules
// (R1-R5, R8, baseType, zipCards), hand-traced same-input→same-output. R6/R7 gate inputs are exercised
// ONLY by the mechanical scenarios above — no new fence scenarios are authored here.
describe('computeAnomalies — sparse-input fallbacks (non-security rules)', () => {
  it('baseType degrades to "" on undefined/null', () => {
    expect(baseType(undefined)).toBe('');
    expect(baseType(null)).toBe('');
  });
  it('a card with undefined consumers/feTypes/danglingRefs arrays takes every `|| []` alternate without flagging', () => {
    const c = {
      ...emptyCard('Owner.sparse'),
      producer: { anchor: 'a:1' },
      consumers: undefined,
      feTypes: undefined,
      danglingRefs: undefined,
    } as unknown as Card;
    const [x, ...rest] = computeAnomaliesT([c], [], []);
    expect(x.rule).toBe('R1'); // producer with zero consumers still flags R1
    expect(rest).toEqual([]);
  });
  it('R3 double-encode on a card with NO producer reads encoding "unknown" and drops the null producer anchor', () => {
    const c: Card = {
      ...emptyCard('Owner.p'),
      consumers: [{ anchor: 'fe:9', decodeExpr: 'JSON.parse(JSON.stringify(x))' }],
    };
    const r3 = computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R3');
    expect(r3).toHaveLength(1);
    expect(r3[0].detail).toContain('on a unknown value returns the input unchanged');
    expect(r3[0].anchors).toEqual(['fe:9']); // a(c.producer) → null → filtered
  });
  it('R3 INCOMPAT mismatch with a producer missing its anchor falls back to "?"', () => {
    const c: Card = {
      ...emptyCard('Owner.q'),
      producer: { encoding: 'json-string' },
      consumers: [{ anchor: 'fe:3', decode: 'object-index', context: 'production' }],
    };
    const [x] = computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R3');
    expect(x.detail).toBe(
      'Owner.q: produced as json-string (?) but consumed via object-index at fe:3',
    );
  });
  it('R4 draws produced literals from dbColumn.checkLiterals when the producer has none', () => {
    const c: Card = {
      ...emptyCard('Owner.r'),
      producer: { anchor: 'a:1' },
      dbColumn: { anchor: 'db:1', checkLiterals: ['A'] },
      consumers: [{ anchor: 'fe:4', comparedLiterals: ['B'], context: 'production' }],
    };
    const [x] = computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R4');
    expect(x.detail).toBe(
      'Owner.r: consumer at fe:4 compares against ["B"] which no producer emits (produced: ["A"])',
    );
  });
  it('R5 tolerates a hand type and an SDL reference both missing anchors (`?? null` alternates)', () => {
    const c: Card = {
      ...emptyCard('Owner.s'),
      sdl: { typeToken: 'Boolean' },
      feTypes: [{ typeToken: 'string', kind: 'hand' }],
    };
    const [x] = computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R5');
    expect(x.detail).toBe(
      'Owner.s: hand-typed "string" (undefined) vs SDL "Boolean" — base string vs boolean',
    );
    expect(x.anchors).toEqual([]); // both `?? null` → filtered
  });
  it('R8 tolerates a dangling ref without an anchor', () => {
    const c: Card = { ...emptyCard('Owner.t'), danglingRefs: [{ ref: 'Ghost' }] };
    const [x] = computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R8');
    expect(x.detail).toBe('Owner.t: "Ghost" at undefined resolves to nothing');
    expect(x.anchors).toEqual([]);
  });
});

describe('computeAnomalies — remaining non-security alternates', () => {
  it('R2 phantom consumer names "(declared in SDL yet unfed)" when the card HAS an SDL slice', () => {
    const c: Card = {
      ...emptyCard('Owner.u'),
      sdl: { anchor: 's:1', typeToken: 'String' },
      consumers: [{ anchor: 'fe:1' }],
    };
    const [x] = computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R2');
    expect(x.detail).toBe(
      'Owner.u: consumed at 1 site(s) but no producer emits it (declared in SDL yet unfed)',
    );
  });
  it('R4 skips a consumer with no comparedLiterals even when the producer emits literals', () => {
    const c: Card = {
      ...emptyCard('Owner.v'),
      producer: { anchor: 'a:1', valueLiterals: ['A'] },
      consumers: [{ anchor: 'fe:1', context: 'production' }],
    };
    expect(computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R4')).toEqual([]);
  });
  it('R5 stays silent for a hand type with NEITHER a generated type NOR an SDL reference', () => {
    const c: Card = {
      ...emptyCard('Owner.w'),
      feTypes: [{ anchor: 'hand:1', typeToken: 'string', kind: 'hand' }],
    };
    expect(computeAnomaliesT([c], [], []).filter((a) => a.rule === 'R5')).toEqual([]);
  });
});

describe('zipCards — sparse-input fallbacks', () => {
  it('a slice result whose jobId matches no job tags the side "?" (still zips)', () => {
    const fields = [{ id: 'Owner.f', ownerType: 'Owner', field: 'f' }];
    const sliceResults: SlicesOut[] = [
      { jobId: 'ghost', slices: [{ fieldId: 'Owner.f', producer: { anchor: 'a:1' } }] },
    ];
    const { cards } = zipCards(fields, [], sliceResults);
    expect(cards[0].sidesCovered).toEqual(['?']);
    expect(cards[0].producer?.anchor).toBe('a:1');
  });
  it('appends notes across slices with a single space (the `(c.notes || "") + " " + s.notes` chain)', () => {
    const fields = [{ id: 'Owner.f', ownerType: 'Owner', field: 'f' }];
    const jobs: SliceJob[] = [{ jobId: 'j1', kind: 'producer', files: [], fieldIds: ['Owner.f'] }];
    const sliceResults: SlicesOut[] = [
      { jobId: 'j1', slices: [{ fieldId: 'Owner.f', notes: 'first' }] },
      { jobId: 'j1', slices: [{ fieldId: 'Owner.f', notes: 'second' }] },
    ];
    const { cards } = zipCards(fields, jobs, sliceResults);
    expect(cards[0].notes).toBe('first second');
  });
  it('a slice for an unknown fieldId is skipped; a result with undefined slices is tolerated', () => {
    const fields = [{ id: 'Owner.f', ownerType: 'Owner', field: 'f' }];
    const sliceResults = [
      { jobId: 'j1', slices: [{ fieldId: 'Owner.GHOST', producer: { anchor: 'a:1' } }] },
      { jobId: 'j2' } as unknown as SlicesOut,
    ];
    const { cards, unsensed } = zipCards(fields, [], sliceResults);
    expect(cards[0].producer).toBeUndefined();
    expect(unsensed).toEqual(['Owner.f']);
  });
  it('merging a second producer tolerates a missing valueLiterals on EITHER side, and slices carry danglingRefs through', () => {
    const fields = [{ id: 'Owner.f', ownerType: 'Owner', field: 'f' }];
    const jobs: SliceJob[] = [
      { jobId: 'p1', kind: 'producer', files: [], fieldIds: ['Owner.f'] },
      { jobId: 'p2', kind: 'producer', files: [], fieldIds: ['Owner.f'] },
      { jobId: 'p3', kind: 'producer', files: [], fieldIds: ['Owner.f'] },
    ];
    const sliceResults: SlicesOut[] = [
      { jobId: 'p1', slices: [{ fieldId: 'Owner.f', producer: { anchor: 'a:1' } }] }, // first producer, NO literals
      {
        jobId: 'p2',
        slices: [{ fieldId: 'Owner.f', producer: { anchor: 'b:1', valueLiterals: ['A'] } }],
      }, // merge onto missing
      {
        jobId: 'p3',
        slices: [
          {
            fieldId: 'Owner.f',
            producer: { anchor: 'c:1' },
            danglingRefs: [{ ref: 'G', anchor: 'g:1' }],
          },
        ],
      }, // merge FROM missing
    ];
    const { cards } = zipCards(fields, jobs, sliceResults);
    expect(cards[0].producer?.anchor).toBe('a:1'); // first producer wins
    expect(cards[0].producer?.valueLiterals).toEqual(['A']);
    expect(cards[0].danglingRefs).toEqual([{ ref: 'G', anchor: 'g:1' }]);
  });
});

// computeInvariantAnomalies — INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.2-2.3).
// Not a source port: reshapes invariantHunter findings into rule class R9-INV Anomaly rows so they
// concatenate onto computeAnomalies's R1-R8 output and ride the same downstream pipeline unchanged.
describe('computeInvariantAnomalies — R9-INV anomaly synthesis from hunter findings', () => {
  it('empty hunter results → empty anomalies (the floor: no registry, no hunters, nothing to convert)', () => {
    expect(computeInvariantAnomalies([])).toEqual([]);
  });
  it('one finding → one R9-INV anomaly carrying invariantId, severity, and a composed detail string', () => {
    const hunterResults: InvariantHunterOut[] = [
      {
        invariantId: 'ENGINE-REGRADE',
        coverage: 'walked db/analysis/**',
        findings: [
          {
            what: 'chunk cache key has no engine dimension',
            location: 'app-cortex/src/app_cortex/db/analysis/chunk_cache.py:38',
            expected: 'cache key includes engine version',
            got: '(session_id, step_name, chunk_hash)',
            failureScenario:
              'engine bump → redelivery replays old-engine slice results stamped as new engine',
            severity: 'high',
          },
        ],
      },
    ];
    const anomalies = computeInvariantAnomalies(hunterResults);
    expect(anomalies).toHaveLength(1);
    expect(anomalies[0].id).toBe('R9-INV-1');
    expect(anomalies[0].rule).toBe('R9-INV');
    expect(anomalies[0].severityHint).toBe('high');
    expect(anomalies[0].cardId).toBeNull();
    expect(anomalies[0].anchors).toEqual([
      'app-cortex/src/app_cortex/db/analysis/chunk_cache.py:38',
    ]);
    expect(anomalies[0].detail).toContain('[ENGINE-REGRADE]');
    expect(anomalies[0].detail).toContain('Expected: cache key includes engine version');
    expect(anomalies[0].detail).toContain('Got: (session_id, step_name, chunk_hash)');
    expect(anomalies[0].detail).toContain('failure scenario: engine bump');
  });
  it('numbers sequentially ACROSS multiple hunters, in hunter-then-finding order (one global sequence, never per-hunter)', () => {
    const hunterResults: InvariantHunterOut[] = [
      {
        invariantId: 'A',
        coverage: 'c',
        findings: [
          {
            what: 'w1',
            location: 'f1:1',
            expected: 'e1',
            got: 'g1',
            failureScenario: 'fs1',
            severity: 'low',
          },
          {
            what: 'w2',
            location: 'f2:2',
            expected: 'e2',
            got: 'g2',
            failureScenario: 'fs2',
            severity: 'med',
          },
        ],
      },
      {
        invariantId: 'B',
        coverage: 'c',
        findings: [
          {
            what: 'w3',
            location: 'f3:3',
            expected: 'e3',
            got: 'g3',
            failureScenario: 'fs3',
            severity: 'critical',
          },
        ],
      },
    ];
    const anomalies = computeInvariantAnomalies(hunterResults);
    expect(anomalies.map((a) => a.id)).toEqual(['R9-INV-1', 'R9-INV-2', 'R9-INV-3']);
    expect(anomalies.map((a) => a.detail)).toEqual([
      expect.stringContaining('[A]'),
      expect.stringContaining('[A]'),
      expect.stringContaining('[B]'),
    ]);
  });
  it('a hunter with zero findings contributes zero anomalies but does not break numbering for the next hunter', () => {
    const hunterResults: InvariantHunterOut[] = [
      { invariantId: 'A', coverage: 'walked, found nothing', findings: [] },
      {
        invariantId: 'B',
        coverage: 'c',
        findings: [
          {
            what: 'w',
            location: 'f:1',
            expected: 'e',
            got: 'g',
            failureScenario: 'fs',
            severity: 'high',
          },
        ],
      },
    ];
    const anomalies = computeInvariantAnomalies(hunterResults);
    expect(anomalies).toHaveLength(1);
    expect(anomalies[0].id).toBe('R9-INV-1');
  });
});
