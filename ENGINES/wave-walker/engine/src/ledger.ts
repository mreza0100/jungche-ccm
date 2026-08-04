// ledger.ts — the INVESTIGATE-mode claim ledger reducers (wave-walker.js lines 173-208), ported as pure
// functions over a plain `LedgerState` object (mirrors rr's store.ts: state is the first arg, never a
// closure). `ingest` mutates the ledger/byStmt/leads maps in place and returns the count of FRESH claims
// this call ledgered — byte-behavior-identical to the source's closure-based `ingest`, including id
// numbering ('c1', 'c2', … / 'L1', 'L2', …) and iteration order.
import type { Confidence, LedgerRow, ProbeOut } from './types/index.js';
import type { LeadRecord } from './types/index.js';

export interface LedgerState {
  ledger: Map<string, LedgerRow>;
  byStmt: Map<string, LedgerRow>;
  leads: Map<string, LeadRecord>;
  cseq: number;
  lseq: number;
}

export function createLedgerState(): LedgerState {
  return { ledger: new Map(), byStmt: new Map(), leads: new Map(), cseq: 0, lseq: 0 };
}

// normStmt — the dedupe key: lowercase, whitespace-collapsed, trimmed (source line 175).
export const normStmt = (s: string | undefined | null): string =>
  String(s || '')
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim();

// ingest — folds a wave's probe results into the ledger: dedupe-by-normStmt, merge anchors/files onto the
// existing row, mark counter-attack targets contested, credit a real attack-lane's survival, and mint
// fresh lead ids. Returns the number of NEW (never-before-seen) claim rows this call created.
export function ingest(
  state: LedgerState,
  probeResults: (ProbeOut | null)[],
  wave: number,
): number {
  let fresh = 0;
  for (const r of probeResults.filter((x): x is ProbeOut => !!x)) {
    const counters = (r.claims || []).filter((c) => c.kind === 'counter');
    for (const c of r.claims || []) {
      const key = normStmt(c.statement);
      let row = state.byStmt.get(key);
      if (!row) {
        row = {
          id: 'c' + ++state.cseq,
          statement: c.statement,
          anchors: [],
          files: [],
          contested: false,
          survived: 0,
          audit: 'pending',
          wave,
        };
        state.byStmt.set(key, row);
        state.ledger.set(row.id, row);
        fresh++;
      }
      for (const a of c.anchors || []) {
        if (a && a.anchor && !row.anchors.some((x) => x.anchor === a.anchor))
          row.anchors.push({ anchor: a.anchor, quote: a.quote });
        const f = String((a && a.anchor) || '').split(':')[0];
        if (f && !row.files.includes(f)) row.files.push(f);
      }
      if (c.kind === 'counter')
        for (const t of c.targets || []) {
          const tgt = state.ledger.get(t);
          if (tgt) tgt.contested = true;
        }
    }
    if (r._laneKind === 'attack' && (r.nothingFound || counters.length === 0))
      for (const t of r._targets || []) {
        const tgt = state.ledger.get(t);
        if (tgt) tgt.survived++;
      }
    for (const l of r.leads || []) {
      const id = 'L' + ++state.lseq;
      state.leads.set(id, { id, what: l.what, files: l.files || [] });
    }
  }
  return fresh;
}

// statusOf — COMPUTED from ledger topology, never asserted (source lines 196-201). settled REQUIRES a
// mechanical audit pass AND (a survived challenge OR a third independent anchor file).
export function statusOf(row: LedgerRow): 'contested' | 'tentative' | 'settled' {
  if (row.contested) return 'contested';
  if (row.audit === 'fail') return 'tentative';
  if (row.audit === 'pass' && row.files.length >= 2 && (row.survived >= 1 || row.files.length >= 3))
    return 'settled';
  return 'tentative';
}

// computedConfidence — over exactly the brainer's key claim ids (source lines 202-208). No key ids →
// low; any contested/audit-failed key claim → low; every key claim settled → high; else medium.
export function computedConfidence(state: LedgerState, keyIds: string[]): Confidence {
  const rows = keyIds.map((id) => state.ledger.get(id)).filter((r): r is LedgerRow => !!r);
  if (!rows.length) return 'low';
  if (rows.some((r) => statusOf(r) === 'contested' || r.audit === 'fail')) return 'low';
  if (rows.every((r) => statusOf(r) === 'settled')) return 'high';
  return 'medium';
}
