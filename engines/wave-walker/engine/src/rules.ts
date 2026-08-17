// rules.ts — the ZERO-TOKEN rule engine (wave-walker.js lines 508-589), ported as pure functions over
// plain state (no class, no engine coupling — mirrors rr's store.ts reducer discipline). Two stages:
// zipCards mechanically zips every sliceSensor job's SlicesOut onto one Card per field (lines 508-528);
// computeAnomalies runs the R1-R8 diff over the zipped cards + undeclared reads + gate cards (529-589).
// Both are byte-behavior-identical to the source — same iteration order, same id numbering, same detail
// strings — verified by test/rules.test.ts against hand-traced expected anomaly sets. A third function,
// computeInvariantAnomalies (INVARIANT REGISTRY FEATURE, tmp/wave-walker-investigation.md § 2.2-2.3), is
// NOT a source port — it reshapes invariantHunter findings into the same Anomaly shape under a new rule
// class, R9-INV, so they concatenate onto computeAnomalies's output and ride the identical downstream
// pipeline. It only ever runs when CONFIG.INVARIANTS is non-empty; empty input yields [].
import type {
  Anomaly,
  Card,
  FieldSpec,
  GateCardWithFile,
  InvariantHunterOut,
  RuleId,
  Severity,
  Slice,
  SliceJob,
  SlicesOut,
  UndeclaredRead,
} from './types/index.js';

// ── baseType — normalizes a GraphQL/TS type token down to its structural base (source lines 537-542) ──
export function baseType(t: string | undefined | null): string {
  let s = String(t || '')
    .toLowerCase()
    .replace(/maybe<|scalars\['?|'?\]\['(in|out)put'?\]|>|\?|!/g, '');
  s = s
    .replace(/\|\s*(null|undefined)/g, '')
    .replace(/(null|undefined)\s*\|/g, '')
    .replace(/\s+/g, '');
  const map: Record<string, string> = {
    string: 'string',
    str: 'string',
    id: 'string',
    int: 'number',
    float: 'number',
    number: 'number',
    boolean: 'boolean',
    bool: 'boolean',
    jsonb: 'object',
    json: 'object',
  };
  return (
    map[s] ||
    (s.startsWith('record<') || s.startsWith('{') || s.includes('array<') || s.endsWith('[]')
      ? 'object'
      : s)
  );
}

// R3 encoding-mismatch incompatibility table + the double-encode detector (source lines 543-544).
export const INCOMPAT: Record<string, string[]> = {
  'json-string': ['object-index', 'spread'],
  object: ['json-parse'],
  'enum-string': ['object-index'],
};
export const DOUBLE_ENCODE = /JSON\s*\.\s*parse\s*\(\s*JSON\s*\.\s*stringify/;

// ── zipCards — mechanical, zero-token zip of every sliceSensor job's output onto one Card per field
// (source lines 508-528). `droppedFieldIds` are fields the sensor cap dropped before scheduling (engine
// owns that cap; passed in here so `unsensed` still names them, exactly as the source's inline computation does).
export function zipCards(
  fields: FieldSpec[],
  jobs: SliceJob[],
  sliceResults: SlicesOut[],
  droppedFieldIds: string[] = [],
): { cards: Card[]; unsensed: string[] } {
  const cardMap = new Map<string, Card & { _sides: Set<string> }>(
    fields.map((f) => [
      f.id,
      {
        id: f.id,
        ownerType: f.ownerType,
        field: f.field,
        apis: f.apis || [],
        sdl: f.sdl || null,
        feTypes: [],
        consumers: [],
        danglingRefs: [],
        sidesCovered: [],
        _sides: new Set<string>(),
      },
    ]),
  );
  for (const r of sliceResults) {
    const job = jobs.find((j) => j.jobId === r.jobId);
    for (const s of r.slices || []) {
      const c = cardMap.get(s.fieldId);
      if (!c) continue;
      c._sides.add(job ? job.kind : '?');
      if (s.producer && !c.producer) c.producer = s.producer;
      else if (s.producer && c.producer)
        c.producer.valueLiterals = [
          ...new Set([...(c.producer.valueLiterals || []), ...(s.producer.valueLiterals || [])]),
        ];
      if (s.dbColumn && !c.dbColumn) c.dbColumn = s.dbColumn;
      if (s.resolver && !c.resolver) c.resolver = s.resolver;
      if (s.feSelection && !c.feSelection) c.feSelection = s.feSelection;
      if (s.feTypes) c.feTypes.push(...s.feTypes);
      if (s.consumers) c.consumers.push(...s.consumers);
      if (s.danglingRefs) c.danglingRefs.push(...s.danglingRefs);
      if (s.notes) c.notes = ((c.notes || '') + ' ' + s.notes).trim();
    }
  }
  const cards = [...cardMap.values()].map((c) => {
    const { _sides, ...rest } = c;
    rest.sidesCovered = [..._sides];
    return rest;
  });
  const unsensed = [
    ...new Set([
      ...[...cardMap.values()].filter((c) => c._sides.size === 0).map((c) => c.id),
      ...droppedFieldIds,
    ]),
  ];
  return { cards, unsensed };
}

// ── groupByResource — the gate byResource grouping (source lines 576-577). Plain object, string-key
// insertion order preserved (matches the source's own object accumulation) — later grouping passes
// (gate-outlier, mandated-fence) iterate Object.entries in that same order.
export function groupByResource(gates: GateCardWithFile[]): Record<string, GateCardWithFile[]> {
  const byResource: Record<string, GateCardWithFile[]> = {};
  for (const g of gates) (byResource[g.resource || 'other'] ||= []).push(g);
  return byResource;
}

const a = (x: { anchor?: string } | null | undefined): string | null => (x && x.anchor) || null;

// GateFenceConfig — the project-supplied vocabulary R6's mandated-fence rule and R7's message text read
// (universal-bundle refactor; args.project via config.ts CONFIG.PROJECT). Optional and gracefully
// defaulted: the R6-mandated-fence block simply never fires without an ownerRole (an empty role can never
// `.includes()`-match a rolesAllowed entry), and R7's label falls back to a generic 'org' word — callers
// that never populate `gates` (every non-fence rule test) never need to pass this at all.
export interface GateFenceConfig {
  fencedResourceClasses: string[];
  ownerRole: string;
  fenceLabels: { org: string; ownership: string };
}

// ── computeAnomalies — the R1-R8 ledger diff (source lines 532-589). ONE pass, in the source's exact
// iteration order, so the sequential `id` numbering (R1-1, R2-3, …) is byte-identical to the source for
// the same inputs — never split into independently-numbered sub-functions.
export function computeAnomalies(
  cards: Card[],
  undeclaredReads: UndeclaredRead[],
  gates: GateCardWithFile[],
  authRule: string,
  fenceConfig?: GateFenceConfig,
): Anomaly[] {
  const fencedResourceClasses = (fenceConfig && fenceConfig.fencedResourceClasses) || [];
  const ownerRole = ((fenceConfig && fenceConfig.ownerRole) || '').toUpperCase();
  const fenceLabels = (fenceConfig && fenceConfig.fenceLabels) || { org: 'org', ownership: 'ownership' };
  const anomalies: Anomaly[] = [];
  let aseq = 0;
  const flag = (
    rule: RuleId,
    ruleName: string,
    detail: string,
    anchors: (string | null)[],
    severityHint: Severity,
    cardId: string | null,
  ): void => {
    anomalies.push({
      id: rule + '-' + ++aseq,
      rule,
      ruleName,
      detail,
      anchors: (anchors || []).filter((x): x is string => !!x),
      severityHint,
      cardId: cardId || null,
    });
  };

  for (const c of cards) {
    const consumers = c.consumers || [];
    const prodConsumers = consumers.filter((x) => (x.context || 'production') === 'production');
    if (c.producer && prodConsumers.length === 0) {
      const nonProd = consumers.length - prodConsumers.length;
      const sub = !c.sdl
        ? 'produced but never exposed in SDL'
        : !c.feSelection
          ? 'declared in SDL but never selected by any FE query'
          : 'shipped and selected but read by no production consumer';
      flag(
        'R1',
        'orphan producer',
        c.id + ': ' + sub + (nonProd ? ' (' + nonProd + ' non-production ref(s) only)' : ''),
        [a(c.producer), a(c.sdl), a(c.feSelection)],
        'med',
        c.id,
      );
    }
    if (!c.producer && consumers.length > 0)
      flag(
        'R2',
        'phantom consumer',
        c.id +
          ': consumed at ' +
          consumers.length +
          ' site(s) but no producer emits it' +
          (c.sdl ? ' (declared in SDL yet unfed)' : ' (absent from SDL)'),
        [a(c.sdl), ...consumers.map(a)],
        'high',
        c.id,
      );
    const enc = c.producer && c.producer.encoding;
    for (const cons of consumers) {
      if (cons.decodeExpr && DOUBLE_ENCODE.test(cons.decodeExpr))
        flag(
          'R3',
          'encoding mismatch',
          c.id +
            ': double-encode JSON.parse(JSON.stringify(...)) at ' +
            cons.anchor +
            ' — on a ' +
            (enc || 'unknown') +
            ' value returns the input unchanged, never a parsed object',
          [a(c.producer), cons.anchor],
          'high',
          c.id,
        );
      else if (enc && INCOMPAT[enc] && cons.decode && INCOMPAT[enc].includes(cons.decode))
        flag(
          'R3',
          'encoding mismatch',
          c.id +
            ': produced as ' +
            enc +
            ' (' +
            ((c.producer && c.producer.anchor) || '?') +
            ') but consumed via ' +
            cons.decode +
            ' at ' +
            cons.anchor,
          [a(c.producer), cons.anchor],
          'high',
          c.id,
        );
    }
    const prodLits = [
      ...new Set([
        ...((c.producer && c.producer.valueLiterals) || []),
        ...((c.dbColumn && c.dbColumn.checkLiterals) || []),
      ]),
    ];
    if (prodLits.length)
      for (const cons of consumers) {
        const cl = cons.comparedLiterals || [];
        const missing = cl.filter((l) => !prodLits.includes(l));
        if (cl.length && missing.length) {
          const casing = missing.filter((l) =>
            prodLits.some((p) => p.toLowerCase() === String(l).toLowerCase()),
          );
          flag(
            'R4',
            'value-set mismatch',
            c.id +
              ': consumer at ' +
              cons.anchor +
              ' compares against ' +
              JSON.stringify(missing) +
              ' which no producer emits' +
              (casing.length
                ? ' — CASING mismatch of ' + JSON.stringify(casing) + ', branch permanently dead'
                : '') +
              ' (produced: ' +
              JSON.stringify(prodLits.slice(0, 8)) +
              ')',
            [a(c.producer), a(c.dbColumn), cons.anchor],
            casing.length ? 'critical' : 'high',
            c.id,
          );
        }
      }
    const gen = (c.feTypes || []).find((t) => t.kind === 'generated');
    for (const hand of (c.feTypes || []).filter((t) => t.kind === 'hand')) {
      const ref = gen || (c.sdl ? { typeToken: c.sdl.typeToken, anchor: c.sdl.anchor } : null);
      if (ref && baseType(hand.typeToken) !== baseType(ref.typeToken))
        flag(
          'R5',
          'type drift',
          c.id +
            ': hand-typed "' +
            hand.typeToken +
            '" (' +
            hand.anchor +
            ') vs ' +
            (gen ? 'generated' : 'SDL') +
            ' "' +
            ref.typeToken +
            '" — base ' +
            baseType(hand.typeToken) +
            ' vs ' +
            baseType(ref.typeToken),
          [hand.anchor ?? null, ref.anchor ?? null],
          'med',
          c.id,
        );
    }
    for (const d of c.danglingRefs || [])
      flag(
        'R8',
        'dangling reference',
        c.id + ': "' + d.ref + '" at ' + d.anchor + ' resolves to nothing',
        [d.anchor ?? null],
        'med',
        c.id,
      );
  }
  for (const r of undeclaredReads)
    flag(
      'R2',
      'phantom consumer',
      (r.side === 'be' ? 'resolver returns' : 'FE reads') +
        ' undeclared field "' +
        r.property +
        '" at ' +
        r.anchor +
        (r.expr ? ' (' + r.expr + ')' : ''),
      [r.anchor],
      r.side === 'be' ? 'med' : 'high',
      null,
    );

  const byResource = groupByResource(gates);
  for (const [res, group] of Object.entries(byResource)) {
    const fenced = group.filter((g) => g.ownershipFence);
    const unfenced = group.filter((g) => !g.ownershipFence && (g.idArgs || []).length > 0);
    if (fenced.length && unfenced.length)
      flag(
        'R6',
        'gate outlier',
        'resource "' +
          res +
          '": ' +
          fenced.map((g) => g.id).join(', ') +
          ' enforce an ownership fence but ' +
          unfenced.map((g) => g.id).join(', ') +
          ' do not — same class, weaker chain',
        [...fenced.map((g) => g.anchor), ...unfenced.map((g) => g.anchor)],
        'high',
        null,
      );
  }
  if (ownerRole)
    for (const [res, group] of Object.entries(byResource)) {
      if (!fencedResourceClasses.includes(res)) continue;
      const violators = group.filter(
        (g) =>
          (g.idArgs || []).length > 0 &&
          !g.ownershipFence &&
          (g.rolesAllowed || []).some((r) => String(r).toUpperCase().includes(ownerRole)),
      );
      if (violators.length)
        flag(
          'R6',
          'mandated-fence violation',
          'resource "' +
            res +
            '": ' +
            violators.map((g) => g.id).join(', ') +
            ' admit ' +
            ownerRole +
            ' with client-supplied id but enforce NO ownership fence — direct violation of the documented rule. ' +
            authRule,
          violators.map((g) => g.anchor),
          'critical',
          null,
        );
    }
  for (const g of gates)
    if ((g.idArgs || []).length > 0 && !g.orgFence && !g.ownershipFence)
      flag(
        'R7',
        'unfenced ID flow',
        g.id +
          ': client-supplied ' +
          JSON.stringify(g.idArgs) +
          ' reaches data access with neither ' +
          fenceLabels.org +
          ' nor ownership fence (chain: ' +
          (g.chain || []).join(' → ') +
          ')',
        [g.anchor],
        'critical',
        null,
      );

  return anomalies;
}

// computeInvariantAnomalies — INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.2-2.3):
// converts every invariantHunter's findings into Anomaly-shaped rows of rule class R9-INV, sequentially
// numbered across ALL hunters (one global aseq, same discipline as computeAnomalies's own numbering) so
// engine.ts can simply CONCATENATE this array onto computeAnomalies's R1-R8 output — the merged array
// then rides the EXISTING byRule → judge → escalation → final-judge → fold pipeline completely unchanged.
// Zero tokens; pure data reshaping, no judgment (judgment is the hunter's + the R9-INV anomalyJudge's job).
export function computeInvariantAnomalies(hunterResults: InvariantHunterOut[]): Anomaly[] {
  const anomalies: Anomaly[] = [];
  let aseq = 0;
  for (const r of hunterResults || []) {
    for (const f of r.findings || []) {
      anomalies.push({
        id: 'R9-INV-' + ++aseq,
        rule: 'R9-INV',
        ruleName: 'invariant-registry violation',
        detail:
          '[' +
          r.invariantId +
          '] ' +
          f.what +
          ' — Expected: ' +
          f.expected +
          ' — Got: ' +
          f.got +
          ' — failure scenario: ' +
          f.failureScenario,
        anchors: [f.location].filter((x): x is string => !!x),
        severityHint: f.severity,
        cardId: null,
      });
    }
  }
  return anomalies;
}

// ruleCounts — the per-rule tally the source logs (`anomalies.reduce(...)`, line 588).
export function ruleCounts(anomalies: Anomaly[]): Record<string, number> {
  return anomalies.reduce(
    (m, x) => {
      m[x.rule] = (m[x.rule] || 0) + 1;
      return m;
    },
    {} as Record<string, number>,
  );
}
