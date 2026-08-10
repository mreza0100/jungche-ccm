import { CONFIG } from './config.js';
import { ruleMeaning } from './constants.js';
import { chunk, globMatch, renderTelemetryMd } from './utils/index.js';
import { SEAT_TALLY } from './runtime.js';
import { batchClaims } from './batching.js';
import { computeAnomalies, computeInvariantAnomalies, ruleCounts, zipCards } from './rules.js';
import { computedConfidence, createLedgerState, ingest, statusOf } from './ledger.js';
import {
  runAnomalyJudge,
  runBrainer,
  runClaimAuditor,
  runClaimExtractor,
  runClaimVerifier,
  runClaimVerifierBatch,
  runConsistencyJudge,
  runCoverageCritic,
  runFinalJudge,
  runFold,
  runGateSweep,
  runInvariantHunter,
  runProbe,
  runScout,
  runSecondOpinion,
  runSecurityAuditor,
  runSliceSensor,
  runSynthesiser,
  runTerritoryDigest,
  runThreadWalker,
} from './agents/index.js';
import type {
  Anomaly,
  ArmedInvariant,
  BrainerLedgerRow,
  Card,
  ClaimIn,
  ConflictFinding,
  Confidence,
  ContradictionScan,
  ContradictionSeat,
  CoordOut,
  CoverageCriticOut,
  CoverageGap,
  DebugRecord,
  DigestOut,
  ExtractedClaim,
  FailedResult,
  FinalOut,
  FoldOut,
  GateCardWithFile,
  GateSweepOut,
  InvariantHunterOut,
  InvariantSpec,
  InvestigateResult,
  JudgeVerdict,
  Lane,
  LedgerRowOut,
  ProjectProfile,
  ScoutOut,
  SeatTally,
  MergedSecurity,
  SecurityOut,
  SliceJob,
  SlicesOut,
  ThreadSpec,
  VerdictContradiction,
  VerifyOut,
  VerifyResult,
  WalkLedger,
  WalkOut,
  WalkResult,
  WaveWalkerResult,
} from './types/index.js';

// project — the digest's per-territory card projection (source lines 621-625): a BE/AI/FE side sees
// only the fields relevant to it. 'AI' is the generic third territory (an AI/compute layer's produced
// values); see the digestJobs slice below for how a card is attributed to it (job-kind driven, not a
// text-grep on the producer's writer name).
function project(c: Card, side: 'BE' | 'AI' | 'FE'): Record<string, unknown> {
  if (side === 'BE')
    return {
      id: c.id,
      producer: c.producer,
      dbColumn: c.dbColumn,
      sdl: c.sdl,
      resolver: c.resolver,
      notes: c.notes,
    };
  if (side === 'AI')
    return { id: c.id, producer: c.producer, dbColumn: c.dbColumn, notes: c.notes };
  return {
    id: c.id,
    sdl: c.sdl && c.sdl.typeToken,
    feSelection: c.feSelection,
    feTypes: c.feTypes,
    consumers: c.consumers,
    notes: c.notes,
  };
}

const SECURITY_RULES = ['R6', 'R7'];
const NEAR_CERTAIN = ['R3', 'R4'];

// E3 gate-conditional dispatch (ported from the proven v3 variant, dispatch-only — no security seat's
// prompt or rule logic changes): the repo-wide gate-file sweep feeding R6/R7 spawns ONLY when the diff
// touches gate-relevant surface — resolver/auth/graphql-infra/application(service) files, per the
// project's OWN configured patterns (args.project.gateResolverPattern/gateSurfacePattern; see
// computeGateArming below) — or the scout scheduled any GraphQL fields/jobs. FAIL-SAFE: any hit → full
// sweep; when in doubt, sweep. CONFIG.FULL_GATE_SWEEP (args.fullGateSweep:true) forces the sweep regardless.
export function isGateRelevant(
  changedFiles: string[],
  fieldsLen: number,
  jobsLen: number,
  patterns: { resolver: RegExp; surface: RegExp },
): boolean {
  return (
    (changedFiles || []).some((f) => patterns.resolver.test(f) || patterns.surface.test(f)) ||
    fieldsLen > 0 ||
    jobsLen > 0
  );
}

// GATE ARMING (universal-bundle refactor) — the gate machinery (sweep scheduling, R6, R7, fence-based
// rules) runs ONLY when args.project supplies its gate-relevant subset: roles + fencedResourceClasses +
// BOTH gate-pattern regex sources, and those sources actually compile. Any miss disarms the WHOLE walk's
// gates — LOUD, never silent (CLAUDE.md "error never renders as absence"): the caller sees
// `gates: SKIPPED — no project profile supplied` or `— invalid gate pattern: <err>` in the coverage
// line/ledger/telemetry, never a quietly-empty gate sweep indistinguishable from "nothing to fence."
export interface GateArming {
  armed: boolean;
  detail: string; // 'SKIPPED — …' when !armed; '' when armed
  patterns?: { resolver: RegExp; surface: RegExp };
}
const NO_PROJECT_DETAIL = 'SKIPPED — no project profile supplied';
export function computeGateArming(project: ProjectProfile | undefined): GateArming {
  if (!project) return { armed: false, detail: NO_PROJECT_DETAIL };
  const hasRoles = !!(project.roles && project.roles.owner && project.roles.elevated);
  const hasFenced =
    Array.isArray(project.fencedResourceClasses) && project.fencedResourceClasses.length > 0;
  const hasPatterns = !!project.gateResolverPattern && !!project.gateSurfacePattern;
  if (!hasRoles || !hasFenced || !hasPatterns) return { armed: false, detail: NO_PROJECT_DETAIL };
  try {
    return {
      armed: true,
      detail: '',
      patterns: {
        resolver: new RegExp(project.gateResolverPattern as string),
        surface: new RegExp(project.gateSurfacePattern as string),
      },
    };
  } catch (e) {
    return {
      armed: false,
      detail:
        'SKIPPED — invalid gate pattern: ' +
        ((e as Error) && (e as Error).message ? (e as Error).message : String(e)),
    };
  }
}

// D1 FILE-SET RECONCILIATION (audit 2026-07-28) — a scout result whose enumerated list disagrees with
// its own separately-executed git count (or that returned no count at all — a pre-feature shape) has an
// untrusted denominator. Zero tokens; the walk-level policy (one corrective retry, then FAIL) lives in
// runWalk().
export function misreconciled(scout: ScoutOut): boolean {
  return (
    !Number.isInteger(scout.changedFileCount) ||
    (scout.changedFiles || []).length !== scout.changedFileCount
  );
}

// D2 SECURITY FAN-OUT (audit 2026-07-28) — the zero-token merge of per-slice auditor results into ONE
// object whose headline carries its own denominator. Semantics: null ONLY when every slice auditor died
// (the existing AUDIT DIED path); a partial death keeps the survivors' findings and the dead slices'
// files simply never enter filesOpened — they surface in filesUnswept, a named coverage hole, never a
// silent pass. categoriesSwept is the INTERSECTION across returned slices: the only category claim true
// of the whole diff (per-slice detail rides findings/summary). Finding ids are prefixed per slice so
// two auditors' 'SEC-1' never collide downstream.
export function mergeSecurityResults(
  results: SecurityOut[],
  dispatched: number,
  allFiles: string[],
): MergedSecurity | null {
  if (!results.length) return null;
  const findings = results.flatMap((r, i) =>
    (r.findings || []).map((f) => ({ ...f, id: 's' + (i + 1) + '·' + (f.id || '') })),
  );
  const categoriesSwept = results
    .map((r) => r.categoriesSwept || [])
    .reduce((a, b) => a.filter((c) => b.includes(c)));
  const filesOpened = [...new Set(results.flatMap((r) => r.filesOpened || []))];
  const openedSet = new Set(filesOpened);
  const filesUnswept = (allFiles || []).filter((f) => !openedSet.has(f));
  return {
    findings,
    categoriesSwept,
    summary: results
      .map((r) => r.summary || '')
      .filter(Boolean)
      .join(' | '),
    filesOpened,
    auditorsDispatched: dispatched,
    auditorsReturned: results.length,
    filesInScope: (allFiles || []).length,
    filesUnswept,
  };
}

// INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — computeArmedInvariants: the
// zero-token fail-safe, same philosophy as isGateRelevant ("any hit → sweep; when in doubt, sweep"). The
// scout's own semantic judgment (scoutArmed — it can arm on a trigger no glob could express, e.g. "diff
// adds a reuse/skip/cache gate") is UNIONED with a deterministic territory-glob match against
// changedFiles, so a scout that forgets to arm an invariant whose territory the diff plainly touches
// cannot silently disarm the hunt. Registry order is preserved (stable, testable output order).
export function computeArmedInvariants(
  invariants: InvariantSpec[],
  changedFiles: string[],
  scoutArmed: { id: string; matchedFiles?: string[] }[],
): ArmedInvariant[] {
  const scoutById = new Map((scoutArmed || []).filter((a) => a && a.id).map((a) => [a.id, a]));
  const out: ArmedInvariant[] = [];
  for (const inv of invariants || []) {
    const scoutHit = scoutById.get(inv.id);
    const territoryMatches = (changedFiles || []).filter((f) =>
      (inv.territory || []).some((g) => globMatch(g, f)),
    );
    if (!scoutHit && !territoryMatches.length) continue;
    const matchedFiles = [
      ...new Set([...((scoutHit && scoutHit.matchedFiles) || []), ...territoryMatches]),
    ];
    const reason =
      scoutHit && territoryMatches.length
        ? 'scout + territory glob'
        : scoutHit
          ? 'scout-armed (semantic trigger)'
          : 'territory glob match (fail-safe — scout did not arm)';
    out.push({ id: inv.id, matchedFiles, reason });
  }
  return out;
}

// VERDICT CONTRADICTIONS (walker.md § Orchestration) — the zero-token contradiction scan, pure and
// exported for direct unit testing (same precedent as computeArmedInvariants/isGateRelevant above).
// Two seats over ONE file returning opposite verdicts is escalated BY NAME to the final judge, never
// averaged, merged, or resolved in favour of the calmer verdict: a clean verdict built on evidence the
// file does not contain is indistinguishable from an earned one until someone opens the file.
// A seat is paired to its files through its thread spec (the scout's `files`); a walked thread whose
// spec named none is UNCOMPARABLE and is named as such — the scan reports what it compared and what it
// could not, so an empty contradiction list is never read as agreement.
export function computeVerdictContradictions(
  threads: ThreadSpec[],
  walks: WalkOut[],
): ContradictionScan {
  const specById = new Map((threads || []).filter((t) => t && t.id).map((t) => [t.id, t]));
  const norm = (f: string): string => f.trim().replace(/^\.\//, '');
  const byFile = new Map<string, ContradictionSeat[]>();
  const uncomparableThreads: string[] = [];
  for (const w of walks || []) {
    if (!w || !w.threadId) continue;
    const spec = specById.get(w.threadId);
    const files = [
      ...new Set(
        ((spec && spec.files) || []).filter((f) => typeof f === 'string' && f.trim()).map(norm),
      ),
    ];
    if (!files.length) {
      uncomparableThreads.push(w.threadId);
      continue;
    }
    // N/A is an abstention, not a health claim — it joins no side and contradicts nothing.
    if (w.flow !== 'INTACT' && w.flow !== 'AT-RISK' && w.flow !== 'BROKEN') continue;
    const seat: ContradictionSeat = {
      threadId: w.threadId,
      name: w.name || (spec && spec.name) || undefined,
      flow: w.flow,
      defects: (w.defects || []).length,
    };
    for (const f of files) byFile.set(f, (byFile.get(f) || []).concat(seat));
  }
  let filesCompared = 0;
  const contradictions: VerdictContradiction[] = [];
  for (const [file, seats] of [...byFile.entries()].sort((a, b) =>
    a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0,
  )) {
    if (seats.length < 2) continue;
    filesCompared++;
    const clean = seats.filter((s) => s.flow === 'INTACT');
    const flagged = seats.filter((s) => s.flow !== 'INTACT');
    if (clean.length && flagged.length) contradictions.push({ file, clean, flagged });
  }
  return { contradictions, filesCompared, uncomparableThreads };
}

// WALK TELEMETRY (DEBUG STEP, tmp/walker-debug-design.md) — the plain-data bag runWalk() gathers before
// calling assembleDebugRecord. Every field is already computed elsewhere in runWalk() by the time of
// assembly; this interface exists only to keep the assembler a pure, independently-testable function
// (mirrors computeArmedInvariants/isGateRelevant's own pure-function, directly-testable precedent above).
export interface DebugAssemblyInput {
  reportPath: string;
  invariants: InvariantSpec[];
  armedInvariants: ArmedInvariant[];
  unarmedInvariants: { id: string; territory: string[] }[];
  seatTally: Record<string, SeatTally>;
  expectedSeats: string[];
  threads: ThreadSpec[];
  walks: WalkOut[];
  jobs: SliceJob[];
  cards: Card[];
  hunterResults: InvariantHunterOut[];
  digestJobsLen: number;
  digests: DigestOut[];
  security: SecurityOut | null;
  coverageCriticResult: CoverageCriticOut | null;
  confirmed: JudgeVerdict[];
  killed: JudgeVerdict[];
  unproven: JudgeVerdict[];
  secondOpinionDispatched: number;
  secondOpinionOverturned: number;
  secondOpinionReexaminedKilled: number;
  finalJudge: FinalOut | null;
  finalJudgeReinstated: number;
  unsensed: string[];
  gateSweepSkipped: boolean;
  gateSweepSkipDetail: string;
  coverageGaps: CoverageGap[];
  contradictionScan: ContradictionScan;
}

// assembleDebugRecord — the FLOOR-disciplined assembler (tmp/walker-debug-design.md §5 "null economy"):
// every section below is wrapped in its OWN try/catch. One bad section sets `degraded: true`, names
// itself in `gaps`, and falls back to honest zeros/empties — every OTHER section still assembles
// independently ("one bad section never blanks the rest"). Zero LLM cost — pure JS over already-computed
// data. Exported (like computeArmedInvariants/isGateRelevant above) so a unit test can force one
// section to throw directly, without needing a full stubbed pipeline run.
export function assembleDebugRecord(input: DebugAssemblyInput): DebugRecord {
  const gaps: string[] = [];
  let degraded = false;
  const fail = (section: string, e: unknown) => {
    degraded = true;
    gaps.push(
      section +
        ' assembly failed: ' +
        ((e as Error) && (e as Error).message ? (e as Error).message : String(e)),
    );
  };

  let armedInvariantsOut: DebugRecord['armedInvariants'] = {
    registered: 0,
    armed: [],
    unarmed: [],
  };
  try {
    armedInvariantsOut = {
      registered: input.invariants.length,
      armed: input.armedInvariants.map((a) => ({
        id: a.id,
        matchedFiles: a.matchedFiles,
        reason: a.reason || '',
      })),
      unarmed: input.unarmedInvariants,
    };
  } catch (e) {
    fail('armedInvariants', e);
  }

  let seats: Record<string, SeatTally> = {};
  let seatsExpectedButAbsent: string[] = [];
  try {
    seats = Object.fromEntries(Object.entries(input.seatTally).map(([k, v]) => [k, { ...v }]));
    seatsExpectedButAbsent = input.expectedSeats.filter((s) => !(s in seats));
  } catch (e) {
    fail('seats', e);
  }

  let emptyResults: DebugRecord['emptyResults'] = {
    threadsWalked: 0,
    threadsExpected: 0,
    sensorsWithCards: 0,
    sensorsExpected: 0,
    hunterFindingsTotal: 0,
    huntersReturned: 0,
    huntersExpected: 0,
    digestsWithFindings: 0,
    digestsExpected: 0,
    securityDied: false,
    coverageCriticDied: false,
    foldDied: false,
  };
  try {
    const coverageCriticDied = input.invariants.length > 0 && !input.coverageCriticResult;
    emptyResults = {
      threadsWalked: input.walks.length,
      threadsExpected: input.threads.length,
      sensorsWithCards: input.cards.length,
      sensorsExpected: input.jobs.length,
      hunterFindingsTotal: input.hunterResults.reduce((n, r) => n + r.findings.length, 0),
      huntersReturned: input.hunterResults.length,
      huntersExpected: input.armedInvariants.length,
      digestsWithFindings: input.digests.filter((d) => d.findings.length > 0).length,
      digestsExpected: input.digestJobsLen,
      securityDied: !input.security,
      coverageCriticDied,
      foldDied: false, // patched by the caller once runFold() resolves — see engine.ts runWalk()
    };
    // CLAUDE.md "error never renders as absence" — a dead coverage-critic over a non-empty registry is
    // named directly in `gaps`, not just a quiet boolean a reader could miss (tmp/walker-debug-design.md §5).
    if (coverageCriticDied)
      gaps.push('coverageCriticDied: registry non-empty but coverage-critic returned nothing');
  } catch (e) {
    fail('emptyResults', e);
  }

  let judgeStats: DebugRecord['judgeStats'] = {
    confirmed: 0,
    falseCount: 0,
    unproven: 0,
    secondOpinionDispatched: 0,
    secondOpinionOverturned: 0,
    secondOpinionReexaminedKilled: 0,
    finalJudgeReinstated: 0,
    finalJudgeVerdict: null,
    finalJudgeDied: true,
  };
  try {
    judgeStats = {
      confirmed: input.confirmed.length,
      falseCount: input.killed.length,
      unproven: input.unproven.length,
      secondOpinionDispatched: input.secondOpinionDispatched,
      secondOpinionOverturned: input.secondOpinionOverturned,
      secondOpinionReexaminedKilled: input.secondOpinionReexaminedKilled,
      finalJudgeReinstated: input.finalJudgeReinstated,
      finalJudgeVerdict: input.finalJudge ? input.finalJudge.verdict : null,
      finalJudgeDied: !input.finalJudge,
    };
  } catch (e) {
    fail('judgeStats', e);
  }

  let coverage: DebugRecord['coverage'] = {
    unsensedFields: [],
    gateSweepSkipped: false,
    gateSweepSkipDetail: '',
    coverageGaps: [],
    verdictContradictions: [],
    contradictionScan: { filesCompared: 0, uncomparableThreads: [] },
  };
  try {
    coverage = {
      unsensedFields: input.unsensed,
      gateSweepSkipped: input.gateSweepSkipped,
      gateSweepSkipDetail: input.gateSweepSkipDetail,
      coverageGaps: input.coverageGaps,
      verdictContradictions: input.contradictionScan.contradictions.map((c) => ({
        file: c.file,
        clean: c.clean.map((s) => s.threadId),
        flagged: c.flagged.map((s) => s.threadId + ' (' + s.flow + ')'),
      })),
      contradictionScan: {
        filesCompared: input.contradictionScan.filesCompared,
        uncomparableThreads: input.contradictionScan.uncomparableThreads,
      },
    };
  } catch (e) {
    fail('coverage', e);
  }

  return {
    schemaVersion: 1,
    mode: 'walk',
    reportPath: input.reportPath,
    degraded,
    gaps,
    armedInvariants: armedInvariantsOut,
    seats,
    seatsExpectedButAbsent,
    emptyResults,
    judgeStats,
    coverage,
    tokenAttribution: null,
  };
}

// ─────────────────────────────────────────────────────────────────────────────
// WaveWalker — the pipeline backbone. run() dispatches on CONFIG.mode to one of the four modes below;
// every agent call goes through a seat's run<Seat>() (pure request/response); every mutation (the
// zero-token rule engine, the ledger, chunking, escalation/reinstatement bookkeeping) is owned here.
// ─────────────────────────────────────────────────────────────────────────────
export class WaveWalker {
  async run(): Promise<WaveWalkerResult> {
    if (CONFIG.mode === 'verify' || CONFIG.mode === 'manifest-verify') return this.runVerify();
    if (CONFIG.mode === 'investigate') return this.runInvestigate();
    return this.runWalk();
  }

  // ─── VERIFY / MANIFEST-VERIFY — pre-ruling claims panel (source lines 57-134, plus the E2
  // manifest-coverage lever: maxClaims 96, breadth-first extraction, conditional ≤4-claim batching
  // above SOLO_THRESHOLD, consistency-judge payload diet, coverage fields on the result). ─────
  async runVerify(): Promise<FailedResult | VerifyResult> {
    const manifestPath = CONFIG.MANIFEST_PATH;
    let claims: (ClaimIn | ExtractedClaim)[] = CONFIG.CLAIMS as ClaimIn[];
    let conflictChecks: VerifyResult['conflicts'] extends never
      ? never
      : { id: string; tasks?: string[]; what: string }[] = [];
    let claimsMined = claims.length; // E2 coverage: pre-cap claim count (args-supplied claims are never capped)
    let droppedClaimIds: string[] = [];

    if (manifestPath && !claims.length) {
      const ex = await runClaimExtractor({ manifestPath, repoRoot: CONFIG.REPO_ROOT });
      if (!ex) return { status: 'FAILED', detail: 'claim extractor died twice' };
      claimsMined = (ex.claims || []).length;
      if (claimsMined > CONFIG.MAX_CLAIMS)
        log(
          '⚠ claim cap ' +
            CONFIG.MAX_CLAIMS +
            ': DROPPED ' +
            (ex.claims.length - CONFIG.MAX_CLAIMS) +
            ' tail claim(s): ' +
            ex.claims
              .slice(CONFIG.MAX_CLAIMS)
              .map((c) => c.id)
              .join(', '),
        );
      droppedClaimIds =
        claimsMined > CONFIG.MAX_CLAIMS ? ex.claims.slice(CONFIG.MAX_CLAIMS).map((c) => c.id) : [];
      claims = (ex.claims || []).slice(0, CONFIG.MAX_CLAIMS);
      conflictChecks = ex.conflictChecks || [];
      log(
        'Extracted ' +
          claims.length +
          ' claim(s) · ' +
          conflictChecks.length +
          ' conflict check(s) from ' +
          manifestPath,
      );
      if (!claims.length)
        return {
          status: 'DONE',
          mode: 'manifest-verify',
          manifest: manifestPath,
          claims: 0,
          verdicts: [],
          consensus: {},
          conflicts: [],
          verifiersDied: 0,
          claimsMined,
          claimsVerified: 0,
          droppedClaimIds,
          taskIds: [],
        };
    }

    // E2 conditional batching: a small panel (claims × votes ≤ SOLO_THRESHOLD) runs SOLO exactly as the
    // source does — per-claim calls, per-claim `claim.opus` escalation, identical labels/schema. A large
    // panel batches ≤4 claims by file-cluster affinity, one verifier call per batch × vote; verifiers
    // stay on the verifier tier (sonnet/xhigh) — never haiku (measured: haiku verifiers did 2.5× the
    // tool calls → +21% tokens, +235% latency; rejected). `claim.opus` cannot survive batching (one
    // model per batch) — accepted trade-off, logged when it drops.
    const panelSize = claims.length * CONFIG.VOTES;
    const solo = panelSize <= CONFIG.SOLO_THRESHOLD;
    let verdicts: VerifyOut[];
    let died: number;
    if (solo) {
      log(
        'Verify mode · ' +
          claims.length +
          ' claim(s) × ' +
          CONFIG.VOTES +
          ' vote(s) · ' +
          CONFIG.TIER.claimVerifier +
          '/' +
          CONFIG.EFFORT.claimVerifier,
      );
      const panel = claims.flatMap((c) =>
        Array.from({ length: CONFIG.VOTES }, (_, v) => ({ c: c as ClaimIn, v })),
      );
      const results = await parallel(
        panel.map(
          ({ c, v }) =>
            () =>
              runClaimVerifier(c, CONFIG.QUESTION, CONFIG.REPO_ROOT, v, CONFIG.VOTES),
        ),
      );
      verdicts = results.filter((r): r is VerifyOut => !!r);
      died = panel.length - verdicts.length;
    } else {
      const batches = batchClaims(claims as ClaimIn[]);
      const opusDropped = (claims as ClaimIn[]).filter((c) => c.opus).length;
      if (opusDropped)
        log(
          '⚠ batching: ' +
            opusDropped +
            ' claim.opus flag(s) cannot escalate inside a batch — riding the verifier tier',
        );
      log('batching: ' + batches.length + ' batch(es) (≤4 claims each, file-cluster affinity)');
      log(
        'Verify mode · ' +
          claims.length +
          ' claim(s) in ' +
          batches.length +
          ' batch(es) × ' +
          CONFIG.VOTES +
          ' vote(s) · ' +
          CONFIG.TIER.claimVerifier +
          '/' +
          CONFIG.EFFORT.claimVerifier,
      );
      const panel = batches.flatMap((b, bi) =>
        Array.from({ length: CONFIG.VOTES }, (_, v) => ({ b, bi, v })),
      );
      const batchResults = await parallel(
        panel.map(
          ({ b, bi, v }) =>
            () =>
              runClaimVerifierBatch(b, CONFIG.QUESTION, CONFIG.REPO_ROOT, bi, v, CONFIG.VOTES),
        ),
      );
      verdicts = batchResults.flatMap((r) => (r && Array.isArray(r.verdicts) ? r.verdicts : []));
      // panel is batches×votes here — died counts dead BATCH CALLS, not missing per-claim verdicts.
      died = panel.length - batchResults.filter(Boolean).length;
    }

    const consensus: Record<string, string> = {};
    for (const c of claims) {
      const vs = verdicts.filter((r) => r.claimId === c.id);
      if (!vs.length) {
        consensus[c.id] = 'NO-VERDICT';
        continue;
      }
      const tally = vs.reduce((m: Record<string, number>, r) => {
        m[r.verdict] = (m[r.verdict] || 0) + 1;
        return m;
      }, {});
      const top = Object.entries(tally).sort((a, b) => b[1] - a[1])[0];
      consensus[c.id] = top[1] > vs.length / 2 ? top[0] : 'SPLIT';
    }

    let conflicts: ConflictFinding[] = [];
    if (manifestPath) {
      // E2 payload diet: full verdict detail rides only for non-CONFIRMED claims; CONFIRMED claims are
      // represented by the consensus map alone.
      const nonConfirmed = verdicts.filter((v) => consensus[v.claimId] !== 'CONFIRMED');
      const cj = await runConsistencyJudge({
        manifestPath,
        nonConfirmed,
        consensus,
        conflictChecks,
      });
      conflicts = cj ? cj.conflicts || [] : [];
      if (cj)
        log(
          'Consistency: ' + conflicts.length + ' finding(s) — ' + (cj.summary || '').slice(0, 120),
        );
    }
    log(
      'Verify done · ' +
        Object.entries(consensus)
          .map(([k, x]) => k + '=' + x)
          .join(' · ') +
        (died ? ' · ⚠ ' + died + ' verifier(s) died' : ''),
    );
    // E2 coverage: which tasks the verified claims cover (unique taskId among verified claims).
    const taskIds = [
      ...new Set((claims as ClaimIn[]).map((c) => c.taskId).filter((t): t is string => !!t)),
    ];
    return {
      status: 'DONE',
      mode: manifestPath ? 'manifest-verify' : 'verify',
      manifest: manifestPath || null,
      question: CONFIG.QUESTION,
      claims: claims.length,
      votes: CONFIG.VOTES,
      verdicts,
      consensus,
      conflicts,
      verifiersDied: died,
      claimsMined,
      claimsVerified: claims.length,
      droppedClaimIds,
      taskIds,
    };
  }

  // ─── INVESTIGATE — RR-for-code: brainer-steered waves over a computed claim ledger (source lines 136-277) ─────
  async runInvestigate(): Promise<FailedResult | InvestigateResult> {
    const goal = CONFIG.GOAL;
    const scopeLine = CONFIG.SCOPE
      ? ' SCOPE (stay inside): ' + JSON.stringify(CONFIG.SCOPE) + '.'
      : '';
    const state = createLedgerState();

    const auditNew = async (wave: number): Promise<void> => {
      const rows = [...state.ledger.values()].filter((r) => r.audit === 'pending');
      if (!rows.length) return;
      const a = await runClaimAuditor({
        rows: rows.map((r) => ({ id: r.id, anchors: r.anchors })),
        wave,
        repoRoot: CONFIG.REPO_ROOT,
      });
      if (!a) return;
      for (const v of a.audits || []) {
        const r = state.ledger.get(v.id);
        if (r) r.audit = v.result;
      }
    };

    log(
      'Investigate · ' +
        CONFIG.LENSES.length +
        ' lenses · ≤' +
        CONFIG.MAX_WAVES +
        ' waves × ≤' +
        CONFIG.MAX_LANES +
        ' lanes · probes ' +
        CONFIG.TIER.probe +
        '/' +
        CONFIG.EFFORT.probe +
        ' · brainer ' +
        CONFIG.TIER.brainer,
    );
    const seedLanes: Lane[] = CONFIG.LENSES.map((lens, i) => ({
      id: 'w0-' + (i + 1),
      kind: 'pursue',
      question: lens,
    }));
    let results = await parallel(seedLanes.map((l) => () => runProbe(l, goal, scopeLine)));
    if (!results.filter(Boolean).length)
      return { status: 'FAILED', detail: 'all wave-0 probes died — nothing to reason over' };
    ingest(state, results, 0);
    await auditNew(0);
    if (!state.ledger.size)
      return {
        status: 'FAILED',
        detail: 'wave-0 probes returned no auditable claims — an empty ledger is not an investigation',
      };

    let coord: CoordOut | null = null;
    let stopReason = 'wave-cap';
    let dry = 0;
    for (let wave = 1; wave <= CONFIG.MAX_WAVES; wave++) {
      if (budget.total && budget.remaining() < 80000) {
        stopReason = 'budget';
        break;
      }
      const ledgerRows: BrainerLedgerRow[] = [...state.ledger.values()].map((r) => ({
        id: r.id,
        s: r.statement,
        status: statusOf(r),
        files: r.files.length,
        survived: r.survived,
        audit: r.audit,
      }));
      coord = await runBrainer({
        goal,
        scopeLine,
        wave,
        maxWaves: CONFIG.MAX_WAVES,
        ledgerRows,
        openLeads: [...state.leads.values()],
        maxLanes: CONFIG.MAX_LANES,
      });
      if (!coord) {
        stopReason = 'brainer-dead';
        break;
      }
      for (const id of coord.dropLeads || []) state.leads.delete(id);
      if (coord.stop && coord.stop.done) {
        stopReason = 'brainer-done: ' + (coord.stop.reason || '');
        break;
      }
      const lanes = (coord.lanes || []).slice(0, CONFIG.MAX_LANES);
      if (!lanes.length) {
        stopReason = 'no-lanes';
        break;
      }
      results = await parallel(lanes.map((l) => () => runProbe(l, goal, scopeLine)));
      const fresh = ingest(state, results, wave);
      await auditNew(wave);
      log(
        'Wave ' +
          wave +
          ': ' +
          lanes.length +
          ' lane(s) → ' +
          fresh +
          ' fresh claim(s) · ledger ' +
          state.ledger.size,
      );
      if (!fresh) {
        if (++dry >= 2) {
          stopReason = 'dry';
          break;
        }
      } else dry = 0;
    }

    const keyIds = coord ? coord.keyClaimIds || [] : [];
    const conf = computedConfidence(state, keyIds);
    const claimsOut: LedgerRowOut[] = [...state.ledger.values()].map((r) => ({
      id: r.id,
      statement: r.statement,
      status: statusOf(r),
      anchors: r.anchors,
      files: r.files,
      survived: r.survived,
      audit: r.audit,
    }));
    const synth = await runSynthesiser({
      goal,
      stopReason,
      keyIds,
      conf,
      reportOut: CONFIG.REPORT_OUT,
      resultSoFarText: coord ? coord.resultSoFar : '(brainer dead — reason from the ledger alone)',
      claimsOut,
      openLeads: [...state.leads.values()],
    });
    const rank: Record<Confidence, number> = { low: 0, medium: 1, high: 2 };
    const finalConf: Confidence = synth
      ? rank[synth.confidence] <= rank[conf]
        ? synth.confidence
        : conf
      : conf;
    log(
      'Investigate done · ' +
        stopReason +
        ' · ' +
        state.ledger.size +
        ' claims · confidence ' +
        finalConf +
        (synth ? '' : ' · ⚠ DEGRADED (synth died)'),
    );
    return {
      status: 'DONE',
      mode: 'investigate',
      goal,
      stopReason,
      answer: synth
        ? synth.answer
        : (coord && coord.resultSoFar) || 'DEGRADED: no synthesis and no coord — see claims',
      confidence: finalConf,
      computedConfidence: conf,
      keyClaimIds: keyIds,
      claims: claimsOut,
      openLeads: [...state.leads.values()],
      report: synth ? synth.report : null,
      reportOut: CONFIG.REPORT_OUT,
      degraded: !synth || !coord,
    };
  }

  // ─── WALK — thread walk + ledger spine, folded into the wave review (source lines 397-731) ─────
  async runWalk(): Promise<FailedResult | WalkResult> {
    log(
      'Wave walker · report=' +
        CONFIG.REPORT_PATH +
        ' · sensors=' +
        CONFIG.TIER.sliceSensor +
        '/' +
        CONFIG.EFFORT.sliceSensor +
        '→' +
        CONFIG.SENSOR_ESCALATE +
        ' · walkers=' +
        CONFIG.TIER.threadWalker +
        ' · judges=' +
        CONFIG.TIER.anomalyJudge,
    );

    const reportPath = CONFIG.REPORT_PATH as string;
    const branch = CONFIG.BRANCH;
    // GATE ARMING — computed BEFORE the scout dispatch (it depends only on CONFIG.PROJECT, not on
    // anything the scout returns), so the scout's own gateFiles instruction can be honest about whether
    // this project even HAS a configured gate-resolver surface (see computeGateArming above).
    const gateArming = computeGateArming(CONFIG.PROJECT);
    const scoutArgs = {
      reportPath,
      branch,
      walkerDoc: CONFIG.WALKER_DOC,
      maxFieldsPerJob: CONFIG.MAX_FIELDS_PER_JOB,
      charter: CONFIG.CHARTER,
      invariants: CONFIG.INVARIANTS,
      invariantsDoc: CONFIG.INVARIANTS_DOC,
      repoRoot: CONFIG.REPO_ROOT,
      authDoc: CONFIG.PROJECT?.authDoc,
      gateResolverPattern: gateArming.armed ? CONFIG.PROJECT?.gateResolverPattern : undefined,
    };
    let scout: ScoutOut | null = await runScout(scoutArgs);
    // D1 FILE-SET RECONCILIATION (docs/dev/audits/wave-walker-instrument-defects-2026-07-28.md) — the
    // changed-file denominator is LLM-reported and the engine cannot run git, so the scout returns BOTH
    // the enumerated list and a separately-executed `wc -l` count, and the two must agree. The observed
    // failure: two walks each enumerated an incomplete list (148-file diff, ~8 files missing — among
    // them the very file the security lens needed) and every downstream lens was scoped by it, silently.
    // One corrective retry with the mismatch named, then the walk FAILS — the empty-set guard's law, one
    // altitude up: a walk over an untrusted denominator must never render a verdict.
    if (scout && misreconciled(scout)) {
      log(
        '⚠ changed-file reconciliation MISMATCH: scout enumerated ' +
          (scout.changedFiles || []).length +
          ' file(s) but its separately-executed git count says ' +
          scout.changedFileCount +
          ' — one corrective scout retry',
      );
      const retry = await runScout({
        ...scoutArgs,
        reconcile: {
          enumerated: (scout.changedFiles || []).length,
          counted: scout.changedFileCount,
        },
      });
      if (retry) scout = retry;
    }
    if (!scout) return { status: 'FAILED', detail: 'scout died twice' };
    if (!(scout.changedFiles || []).length)
      return {
        status: 'FAILED',
        detail:
          'scout resolved an EMPTY changed-file set — no merge SHA found in ' +
          reportPath +
          (branch ? ' / empty branch diff' : '') +
          '; a walk over nothing must never return a verdict',
      };
    if (misreconciled(scout))
      return {
        status: 'FAILED',
        detail:
          'changed-file reconciliation FAILED twice: scout enumerated ' +
          (scout.changedFiles || []).length +
          ' file(s) but its separately-executed git count says ' +
          scout.changedFileCount +
          ' — every lens (security above all) is scoped by the enumerated list, and a walk over an untrusted denominator must never render a verdict',
      };
    const threads: ThreadSpec[] = (scout.threads || []).concat(
      CONFIG.EXTRA_THREADS as ThreadSpec[],
    );
    // ZERO-THREAD GUARD — the empty-diff guard above states the law ("a walk over
    // nothing must never return a verdict") and this is the SAME law one altitude up:
    // a scout that enumerates ZERO threads over a NON-EMPTY diff has not proven the
    // diff is safe — it has proven the scout could not read it. Without this, an
    // empty enumeration renders as SMOOTH SAILING: "nothing found" reported as
    // "nothing wrong", by the very instrument the gates trust most.
    if (!threads.length)
      return {
        status: 'FAILED',
        detail:
          'scout enumerated ZERO threads over a NON-EMPTY diff (' +
          (scout.changedFiles || []).length +
          ' changed files) — that is a SCOUT FAILURE, not a clean walk. An empty enumeration is never a verdict.',
      };
    const fields = scout.fields || [];
    let gateFiles = scout.gateFiles || [];
    log(
      'Scout: ' +
        threads.length +
        ' threads · ' +
        fields.length +
        ' type-fields · ' +
        (scout.jobs || []).length +
        ' slice jobs · ' +
        gateFiles.length +
        ' gate files · changed: ' +
        (scout.changedFiles || []).length +
        ' files',
    );

    // INVARIANT REGISTRY FEATURE (§ 2.1) — THE FLOOR: CONFIG.INVARIANTS defaults to [], so
    // computeArmedInvariants trivially returns [] and armedInvariants.length is 0 for every walk that
    // never supplied args.invariants — zero hunters dispatch, zero coverageCritic calls, zero extra log
    // lines below (this ONE line only fires when the registry is non-empty).
    const invariantById = new Map(CONFIG.INVARIANTS.map((inv) => [inv.id, inv]));
    const armedInvariants = computeArmedInvariants(
      CONFIG.INVARIANTS,
      scout.changedFiles || [],
      scout.armedInvariants || [],
    );
    if (CONFIG.INVARIANTS.length)
      log(
        'Invariant registry: ' +
          CONFIG.INVARIANTS.length +
          ' registered · ' +
          armedInvariants.length +
          ' armed' +
          (armedInvariants.length ? ' (' + armedInvariants.map((a) => a.id).join(', ') + ')' : ''),
      );

    // GATE ARMING (universal-bundle refactor) — no args.project (or its gate-relevant subset missing, or
    // an unparseable pattern) disarms the gate machinery for the WHOLE walk: the sweep never dispatches
    // and R6/R7/fence rules never fire (gateFiles forced [] below feeds computeAnomalies an empty `gates`
    // array — see rules.ts). LOUD, never silent: gateSweepSkipDetail rides the coverage line/ledger/
    // telemetry wherever gate status is reported.
    let gateSweepSkipped = false;
    let gateSweepSkipDetail = '';
    if (!gateArming.armed) {
      gateFiles = [];
      gateSweepSkipped = true;
      gateSweepSkipDetail = gateArming.detail;
      log('gate sweep ' + gateArming.detail + ' — R6/R7 not evaluated this walk');
    } else {
      // E3: diff-scoped gate sweep — skip R6/R7's repo-wide gate population entirely when the diff
      // touches no resolver/auth/service surface (LOUD skip, fail-safe classifier; the diff-scoped
      // security auditor below ALWAYS runs regardless). args.fullGateSweep:true forces the full sweep.
      const gateRelevant = isGateRelevant(
        scout.changedFiles || [],
        fields.length,
        (scout.jobs || []).length,
        gateArming.patterns!,
      );
      if (!gateRelevant && !CONFIG.FULL_GATE_SWEEP) {
        gateFiles = [];
        gateSweepSkipped = true;
        gateSweepSkipDetail = 'SKIPPED (diff-scoped)';
        log(
          'gate sweep SKIPPED (diff touches no resolver/auth/service surface; fullGateSweep to force) — R6/R7 not evaluated this walk',
        );
      }
    }

    const authDoc = CONFIG.PROJECT?.authDoc || '';
    const authRuleMustContain = CONFIG.PROJECT?.authRuleMustContain || [];
    const authOk =
      typeof scout.authRule === 'string' &&
      scout.authRule.length >= 120 &&
      authRuleMustContain.every((tok) => (scout.authRule as string).includes(tok));
    if (!authOk)
      log(
        '⚠ scout returned no usable auth-pattern extract — R6/second-opinion run on the configured fallback' +
          (authDoc ? ' (verify it against ' + authDoc + ')' : ' (no args.project.authDoc configured)'),
      );
    const authRule = authOk
      ? (authDoc ? authDoc + ' (live, scout-extracted): "' : '(live, scout-extracted): "') +
        scout.authRule +
        '"'
      : CONFIG.PROJECT?.authRuleFallback || '';

    // Enforce the sensor cap; name any dropped fields (honest coverage).
    let jobs: SliceJob[] = (scout.jobs || []).flatMap((j) =>
      (j.fieldIds || []).length <= CONFIG.MAX_FIELDS_PER_JOB
        ? [j]
        : j.fieldIds.reduce((acc: SliceJob[], id, i) => {
            const b = Math.floor(i / CONFIG.MAX_FIELDS_PER_JOB);
            (acc[b] = acc[b] || {
              ...j,
              jobId: j.jobId + '-' + (b + 1),
              fieldIds: [],
            }).fieldIds.push(id);
            return acc;
          }, []),
    );
    let droppedFieldIds: string[] = [];
    if (jobs.length + gateFiles.length > CONFIG.MAX_SENSORS) {
      const keep = Math.max(0, CONFIG.MAX_SENSORS - gateFiles.length);
      droppedFieldIds = jobs.slice(keep).flatMap((j) => j.fieldIds || []);
      jobs = jobs.slice(0, keep);
      if (droppedFieldIds.length)
        log(
          '⚠ sensor cap ' +
            CONFIG.MAX_SENSORS +
            ': DROPPED slice jobs — fields reported UNSENSED: ' +
            droppedFieldIds.join(', '),
        );
    }

    // D2 SECURITY FAN-OUT (audit 2026-07-28) — security was the only lens with no fan-out: one auditor
    // over a 148-file diff opened ~45 files and reported "0 findings across 9 categories" as a clean
    // sweep. Coverage now scales like every other seat: the changed files, sorted (directory locality),
    // cluster into slices of ≤ CONFIG.SECURITY_FILES_PER_AUDITOR, one auditor per slice, each carrying
    // the full set as cross-file context; mergeSecurityResults folds them into one object whose headline
    // carries filesOpened/filesInScope.
    const securityClusters = chunk(
      [...(scout.changedFiles || [])].sort(),
      CONFIG.SECURITY_FILES_PER_AUDITOR,
    );
    log(
      'Security fan-out: ' +
        securityClusters.length +
        ' auditor slice(s) × ≤' +
        CONFIG.SECURITY_FILES_PER_AUDITOR +
        ' files',
    );

    // ─── Phase 1: Walk (thread walkers) + Sense (ledger sensors + gate sweeps) + Hunt (armed invariants)
    // — one parallel barrier. Order: threads · jobs · gates · security slices (nS) · hunters — the slice
    // math below indexes by these counts, in this order. ─────
    const fieldById = new Map(fields.map((f) => [f.id, f]));
    type Barrier = WalkOut | SlicesOut | GateSweepOut | SecurityOut | InvariantHunterOut | null;
    const walked: Barrier[] = await parallel([
      ...threads.map(
        (t) => () =>
          runThreadWalker({
            walkerDoc: CONFIG.WALKER_DOC,
            hygieneDoc: CONFIG.HYGIENE_DOC,
            thread: t,
            charter: CONFIG.CHARTER,
          }) as Promise<Barrier>,
      ),
      ...jobs.map((j) => () => {
        const assigned = j.fieldIds.map((id) => {
          const f = fieldById.get(id);
          return { fieldId: id, field: f && f.field, sdlTypeToken: f && f.sdl && f.sdl.typeToken };
        });
        return runSliceSensor({
          jobId: j.jobId,
          kind: j.kind,
          files: j.files,
          hint: j.hint,
          assigned,
        }) as Promise<Barrier>;
      }),
      // gateFiles is non-empty ONLY when the gate machinery is armed (see the gate-arming block above),
      // so CONFIG.PROJECT is guaranteed populated here — resourceClasses/fenceLabels still fall back
      // gracefully since only roles/fencedResourceClasses/gate-patterns are required to arm.
      ...gateFiles.map(
        (f) => () =>
          runGateSweep({
            file: f,
            resourceClasses: CONFIG.PROJECT?.resourceClasses ||
              CONFIG.PROJECT?.fencedResourceClasses || [],
            fenceLabels: CONFIG.PROJECT?.fenceLabels || { org: 'org', ownership: 'ownership' },
          }) as Promise<Barrier>,
      ),
      ...securityClusters.map(
        (cf, ci) => () =>
          runSecurityAuditor({
            securityDoc: CONFIG.SECURITY_DOC,
            clusterFiles: cf,
            allChangedFiles: scout!.changedFiles,
            clusterIndex: ci,
            clusterCount: securityClusters.length,
            branch,
            mergeShas: scout!.mergeShas || [],
            securityStakesLine: CONFIG.PROJECT?.securityStakesLine,
          }) as Promise<Barrier>,
      ),
      ...armedInvariants.map((ai) => () => {
        const inv = invariantById.get(ai.id);
        return (
          inv
            ? runInvariantHunter({ invariant: inv, matchedFiles: ai.matchedFiles })
            : Promise.resolve(null)
        ) as Promise<Barrier>;
      }),
    ]);
    const nT = threads.length;
    const nJ = jobs.length;
    const nG = gateFiles.length;
    const nS = securityClusters.length;
    const nInv = armedInvariants.length;
    const walks = (walked.slice(0, nT) as (WalkOut | null)[]).filter((x): x is WalkOut => !!x);
    const sliceResults = (walked.slice(nT, nT + nJ) as (SlicesOut | null)[]).filter(
      (x): x is SlicesOut => !!x,
    );
    const gates: GateCardWithFile[] = (
      walked.slice(nT + nJ, nT + nJ + nG) as (GateSweepOut | null)[]
    )
      .filter((x): x is GateSweepOut => !!x)
      .flatMap((s) => (s.gates || []).map((g) => ({ ...g, file: s.file })));
    const securityResults = (
      walked.slice(nT + nJ + nG, nT + nJ + nG + nS) as (SecurityOut | null)[]
    ).filter((x): x is SecurityOut => !!x);
    const security: MergedSecurity | null = mergeSecurityResults(
      securityResults,
      nS,
      scout.changedFiles || [],
    );
    const secFindings = security ? security.findings || [] : [];
    const undeclaredReads = sliceResults.flatMap((r) => r.undeclaredReads || []);
    const hunterResults = (
      walked.slice(nT + nJ + nG + nS, nT + nJ + nG + nS + nInv) as (InvariantHunterOut | null)[]
    ).filter((x): x is InvariantHunterOut => !!x);
    if (armedInvariants.length)
      log(
        'Invariant hunt: ' +
          hunterResults.length +
          '/' +
          armedInvariants.length +
          ' hunter(s) returned · ' +
          hunterResults.reduce((n, r) => n + (r.findings || []).length, 0) +
          ' finding(s)',
      );

    // VERDICT CONTRADICTIONS (walker.md § Orchestration) — mechanical, zero tokens, ALWAYS run: two
    // seats over one file with opposite verdicts are escalated BY NAME to the final judge below, never
    // averaged and never settled by the calmer verdict. The log line is unconditional so a zero result
    // states its own coverage rather than passing silently.
    const contradictionScan = computeVerdictContradictions(threads, walks);
    log(
      'Verdict contradictions: ' +
        contradictionScan.contradictions.length +
        ' over ' +
        contradictionScan.filesCompared +
        ' file(s) walked by 2+ seats' +
        (contradictionScan.uncomparableThreads.length
          ? ' · ' +
            contradictionScan.uncomparableThreads.length +
            ' thread(s) uncomparable (no files in spec): ' +
            contradictionScan.uncomparableThreads.join(', ')
          : '') +
        (contradictionScan.contradictions.length
          ? ' → ' +
            contradictionScan.contradictions
              .map(
                (c) =>
                  c.file +
                  ': ' +
                  c.clean.map((s) => s.threadId).join('/') +
                  ' INTACT vs ' +
                  c.flagged.map((s) => s.threadId + ' ' + s.flow).join('/'),
              )
              .join(' · ')
          : ''),
    );

    // Zip slices into cards (mechanical, zero tokens)
    const { cards, unsensed } = zipCards(fields, jobs, sliceResults, droppedFieldIds);
    if (unsensed.length) log('⚠ UNSENSED fields (no card): ' + unsensed.join(', '));
    log(
      'Walked: ' +
        walks.length +
        '/' +
        nT +
        ' threads · ' +
        cards.length +
        ' cards from ' +
        sliceResults.length +
        '/' +
        nJ +
        ' jobs · ' +
        gates.length +
        ' gates · undeclared reads: ' +
        undeclaredReads.length +
        ' · security: ' +
        (security
          ? secFindings.length +
            ' finding(s) · auditors ' +
            security.auditorsReturned +
            '/' +
            security.auditorsDispatched +
            ' · opened ' +
            security.filesOpened.length +
            '/' +
            security.filesInScope
          : 'AUDIT DIED (all ' + nS + ' slice auditors)'),
    );

    // ─── Phase 2: the ledger diff — mechanical rules, zero tokens; R9-INV (hunter findings, zero when
    // the registry is absent/empty) concatenated on — same array, same downstream pipeline ─────
    // `gates` is [] whenever the gate machinery is unarmed (see above), so R6/R7 naturally produce zero
    // anomalies without any extra guard here — fenceConfig only matters on the armed path.
    const fenceConfig = CONFIG.PROJECT
      ? {
          fencedResourceClasses: CONFIG.PROJECT.fencedResourceClasses || [],
          ownerRole: (CONFIG.PROJECT.roles && CONFIG.PROJECT.roles.owner) || '',
          fenceLabels: CONFIG.PROJECT.fenceLabels || { org: 'org', ownership: 'ownership' },
        }
      : undefined;
    const anomalies: Anomaly[] = computeAnomalies(
      cards,
      undeclaredReads,
      gates,
      authRule,
      fenceConfig,
    ).concat(computeInvariantAnomalies(hunterResults));
    const counts = ruleCounts(anomalies);
    log(
      'Ledger diff: ' +
        anomalies.length +
        ' anomalies (' +
        Object.entries(counts)
          .map(([k, v]) => k + ':' + v)
          .join(' ') +
        ')',
    );

    // ─── Phase 3: judges (ledger anomalies) + digests; Opus second opinion on killed security/near-certain ─
    const cardById = new Map(cards.map((c) => [c.id, c]));
    const byRule: Record<string, Anomaly[]> = {};
    for (const x of anomalies) (byRule[x.rule] = byRule[x.rule] || []).push(x);
    const meaning = ruleMeaning(authRule);
    const judgeJobs = Object.entries(byRule).flatMap(([rule, list]) =>
      chunk(list, 6).map((grp, i) => ({ rule: rule as Anomaly['rule'], grp, i })),
    );
    // 'AI' territory slice — driven by the STRUCTURED sidesCovered a card already carries (zipCards tags
    // every card with the job kinds that touched it), a strictly more reliable signal than grepping
    // extracted free text for a magic string.
    const digestJobs = cards.length
      ? (scout.territories || [])
          .map((t) => ({
            territory: t,
            slice:
              t === 'AI'
                ? cards.filter((c) => (c.sidesCovered || []).includes('ai')).map((c) => project(c, 'AI'))
                : cards.map((c) => project(c, t === 'BE' ? 'BE' : 'FE')),
          }))
          .filter((j) => j.slice.length)
      : [];
    // INVARIANT REGISTRY FEATURE (§ 2.4) — the external denominator, dispatched alongside judges/digests
    // (all three depend only on Phase-1 outputs, none on each other) so it completes before the final
    // judge needs it. Only runs when the registry is non-empty — an absent/empty registry dispatches
    // NOTHING here, honoring the floor invariant exactly.
    const armedIds = armedInvariants.map((a) => a.id);
    const unarmedInvariants = CONFIG.INVARIANTS.filter((inv) => !armedIds.includes(inv.id)).map(
      (inv) => ({ id: inv.id, territory: inv.territory }),
    );
    const [judgeResults, digestResults, coverageCriticResult] = await Promise.all([
      parallel(
        judgeJobs.map((j) => () => {
          const ctxCards = [
            ...new Set(j.grp.map((x) => x.cardId).filter((id): id is string => !!id)),
          ]
            .map((id) => cardById.get(id))
            .filter((c): c is Card => !!c);
          return runAnomalyJudge(
            {
              rule: j.rule,
              ruleMeaning: meaning[j.rule],
              sec: SECURITY_RULES.includes(j.rule) || j.rule === 'R9-INV',
              instances: j.grp,
              ctxCards,
              authDoc: CONFIG.PROJECT?.authDoc,
            },
            j.i,
          );
        }),
      ),
      parallel(
        digestJobs.map(
          (j) => () =>
            runTerritoryDigest({ territory: j.territory, slice: j.slice, charter: CONFIG.CHARTER }),
        ),
      ),
      CONFIG.INVARIANTS.length
        ? runCoverageCritic({
            changedFiles: scout.changedFiles || [],
            threadNames: threads.map((t) => ({ id: t.id, type: t.type, name: t.name })),
            hunterCoverage: hunterResults.map((r) => ({
              invariantId: r.invariantId,
              coverage: r.coverage,
            })),
            armedIds,
            unarmedInvariants,
            unsensed,
          })
        : Promise.resolve(null),
    ]);
    let verdicts = judgeResults
      .filter((r): r is NonNullable<typeof r> => !!r)
      .flatMap((r) => r.verdicts || []);
    const digests = digestResults.filter((d): d is NonNullable<typeof d> => !!d);
    const coverageGaps: CoverageGap[] = coverageCriticResult ? coverageCriticResult.gaps || [] : [];
    if (CONFIG.INVARIANTS.length)
      log(
        'Coverage critic: ' +
          (coverageCriticResult ? coverageGaps.length + ' gap(s) named' : 'DIED — a coverage hole'),
      );
    const anomalyById = new Map(anomalies.map((x) => [x.id, x]));
    // killed security/near-certain (existing) — a wrong KILL hides here.
    const killedEscalatable = verdicts
      .filter((v) => v.verdict === 'FALSE')
      .filter((v) => {
        const x = anomalyById.get(v.anomalyId);
        if (!x) return false;
        if (SECURITY_RULES.includes(x.rule) && ['high', 'critical'].includes(x.severityHint))
          return true;
        return NEAR_CERTAIN.includes(x.rule);
      });
    // INVARIANT REGISTRY FEATURE (§ 2.3, escalation symmetry) — confirmed R9-INV high/critical (new) —
    // a wrong CONFIRM hides here, the opposite direction. [] whenever no R9-INV anomaly was ever raised.
    const confirmedInvEscalatable = verdicts.filter((v) => {
      if (v.verdict !== 'CONFIRMED') return false;
      const x = anomalyById.get(v.anomalyId);
      return !!x && x.rule === 'R9-INV' && ['high', 'critical'].includes(x.severityHint);
    });
    const escalatable = [...killedEscalatable, ...confirmedInvEscalatable];
    if (escalatable.length) {
      log(
        'Escalation: ' +
          escalatable.length +
          ' killed security/near-certain' +
          (confirmedInvEscalatable.length ? ' or confirmed R9-INV high/critical' : '') +
          ' verdict(s) → ' +
          CONFIG.TIER.secondOpinion +
          ' second opinion',
      );
      const killedChunks = chunk(killedEscalatable, 4);
      const confirmedChunks = chunk(confirmedInvEscalatable, 4);
      const second = await parallel([
        ...killedChunks.map(
          (grp, i) => () =>
            runSecondOpinion(
              {
                authRule,
                items: grp.map((v) => ({ verdict: v, anomaly: anomalyById.get(v.anomalyId) })),
              },
              i,
            ),
        ),
        ...confirmedChunks.map(
          (grp, i) => () =>
            runSecondOpinion(
              {
                authRule,
                items: grp.map((v) => ({ verdict: v, anomaly: anomalyById.get(v.anomalyId) })),
                direction: 'confirmed',
              },
              killedChunks.length + i,
            ),
        ),
      ]);
      const overrides = new Map(
        second
          .filter((r): r is NonNullable<typeof r> => !!r)
          .flatMap((r) => r.verdicts || [])
          .map((v) => [v.anomalyId, v] as const),
      );
      verdicts = verdicts.map((v) => {
        const o = overrides.get(v.anomalyId);
        if (!o) return v;
        if (v.verdict === 'FALSE' && o.verdict !== 'FALSE')
          return { ...o, why: '[OVERRIDE by ' + CONFIG.TIER.secondOpinion + '] ' + (o.why || '') };
        if (v.verdict === 'CONFIRMED' && o.verdict === 'FALSE')
          return {
            ...o,
            why: '[RE-EXAMINED by ' + CONFIG.TIER.secondOpinion + ', KILLED] ' + (o.why || ''),
          };
        return v;
      });
    }
    // WALK TELEMETRY (DEBUG STEP) — snapshot the second-opinion override markers HERE, before Phase 3.5's
    // final-judge reinstatement can potentially overwrite a `why` string that also got reinstated. 0/0
    // naturally when escalation never ran (verdicts never got these markers).
    const secondOpinionOverturned = verdicts.filter((v) =>
      (v.why || '').startsWith('[OVERRIDE'),
    ).length;
    const secondOpinionReexaminedKilled = verdicts.filter((v) =>
      (v.why || '').startsWith('[RE-EXAMINED'),
    ).length;
    let confirmed = verdicts.filter((v) => v.verdict === 'CONFIRMED');
    let unproven = verdicts.filter((v) => v.verdict === 'UNPROVEN');
    let killed = verdicts.filter((v) => v.verdict === 'FALSE');
    log(
      'Judged: ' +
        confirmed.length +
        ' confirmed · ' +
        killed.length +
        ' false · ' +
        unproven.length +
        ' unproven · thread walks: ' +
        walks.length +
        ' · digest findings: ' +
        digests.reduce((n, d) => n + d.findings.length, 0),
    );

    // ─── Phase 3.5: final judgment — ONE Opus rules the whole walk before anything is written ─────
    const walksBrief = walks.map((w) => ({
      id: w.threadId,
      name: w.name,
      flow: w.flow,
      defects: w.defects,
      notes: w.notes,
    }));
    const killedWithAnomaly = killed.map((v) => ({
      anomalyId: v.anomalyId,
      why: v.why,
      anomaly: anomalyById.get(v.anomalyId),
    }));
    let finalJudge: FinalOut | null = await runFinalJudge({
      walksBrief,
      confirmed,
      unproven,
      killedWithAnomaly,
      digests,
      securityDoc: CONFIG.SECURITY_DOC,
      security,
      walksLen: walks.length,
      threadsLen: threads.length,
      cardsLen: cards.length,
      unsensed,
      charter: CONFIG.CHARTER,
      coverageGaps,
      contradictions: contradictionScan.contradictions,
    });
    let finalJudgeReinstatedCount = 0;
    if (finalJudge && (finalJudge.reinstated || []).length) {
      const re = new Map(finalJudge.reinstated!.map((r) => [r.anomalyId, r]));
      verdicts = verdicts.map((v) =>
        re.has(v.anomalyId) && v.verdict === 'FALSE'
          ? {
              ...v,
              verdict: 'CONFIRMED' as const,
              why: '[REINSTATED by final judge] ' + re.get(v.anomalyId)!.why,
            }
          : v,
      );
      confirmed = verdicts.filter((v) => v.verdict === 'CONFIRMED');
      unproven = verdicts.filter((v) => v.verdict === 'UNPROVEN');
      killed = verdicts.filter((v) => v.verdict === 'FALSE');
      finalJudgeReinstatedCount = finalJudge.reinstated!.length;
      log('Final judge reinstated ' + finalJudge.reinstated!.length + ' killed verdict(s)');
    }
    if (finalJudge)
      log(
        'Final judgment: ' +
          finalJudge.verdict +
          ' · ' +
          finalJudge.missedRisks.length +
          ' missed risk(s)',
      );

    // ─── Phase 4: Fold — merge thread walks + confirmed anomalies + digests → the wave review ─────
    const coverageSummary =
      'threads walked: ' +
      walks.length +
      '/' +
      threads.length +
      ' · fields sensed: ' +
      cards.length +
      ' · UNSENSED: ' +
      (unsensed.length ? unsensed.join(', ') : 'none') +
      ' · gates: ' +
      (gateSweepSkipped ? gateSweepSkipDetail : gates.length) +
      ' · ledger anomalies: ' +
      anomalies.length +
      ' → confirmed ' +
      confirmed.length +
      ', false ' +
      killed.length +
      ', unproven ' +
      unproven.length +
      ' · security: ' +
      (security
        ? secFindings.length +
          ' finding(s) over ' +
          (security.categoriesSwept || []).length +
          ' categories swept everywhere · auditors ' +
          security.auditorsReturned +
          '/' +
          security.auditorsDispatched +
          ' · files opened ' +
          security.filesOpened.length +
          '/' +
          security.filesInScope +
          (security.filesUnswept.length
            ? ' · UNSWEPT: ' +
              security.filesUnswept.slice(0, 12).join(', ') +
              (security.filesUnswept.length > 12
                ? ' (+' + (security.filesUnswept.length - 12) + ' more)'
                : '')
            : '')
        : 'AUDIT DIED') +
      (CONFIG.INVARIANTS.length
        ? ' · invariants: ' +
          CONFIG.INVARIANTS.length +
          ' registered, ' +
          armedInvariants.length +
          ' armed (' +
          (armedIds.length ? armedIds.join(', ') : 'none') +
          ') · coverage gaps: ' +
          coverageGaps.length
        : '') +
      ' · verdict contradictions: ' +
      contradictionScan.contradictions.length +
      ' over ' +
      contradictionScan.filesCompared +
      ' multi-seat file(s)' +
      (contradictionScan.contradictions.length
        ? ' (' + contradictionScan.contradictions.map((c) => c.file).join(', ') + ')'
        : '') +
      (contradictionScan.uncomparableThreads.length
        ? ' · uncomparable threads (no files in spec): ' +
          contradictionScan.uncomparableThreads.join(', ')
        : '');

    // WALK TELEMETRY (DEBUG STEP) — assembled HERE, immediately before the runFold() call, so it is in
    // scope for both the success return below and the `fold died twice` FAILED return (the single
    // highest-value floor fix per tmp/walker-debug-design.md §5: today that path returns nothing but a
    // detail string).
    //
    // computeExpectedSeats — the WALK-mode-relevant subset THIS WALK actually SCHEDULED, derived from
    // real per-walk dispatch decisions (jobs-by-kind, gateFiles, judgeJobs, digestJobs, escalatable,
    // armedInvariants, CONFIG.INVARIANTS), not a static universal seat list — a walk with no GraphQL
    // surface / a gate-free diff / zero anomalies legitimately dispatches zero sensors/gates/judges/
    // digests (the documented floor, walker.md: "the floor never regresses"), so those seats are never
    // flagged as a false "gap." `includeFold` is false pre-fold-call (fold hasn't been dispatched yet at
    // that point) and true in the post-fold patch below, once it genuinely has been.
    const computeExpectedSeats = (includeFold: boolean): string[] =>
      ['scout', 'walk', 'security', 'final-judge']
        .concat(includeFold ? ['fold'] : [])
        .concat(jobs.some((j) => j.kind === 'producer') ? ['producer'] : [])
        .concat(jobs.some((j) => j.kind === 'consumer') ? ['consumer'] : [])
        .concat(jobs.some((j) => j.kind === 'ai') ? ['ai'] : [])
        .concat(gateFiles.length && !gateSweepSkipped ? ['gates'] : [])
        .concat(judgeJobs.length ? ['judge'] : [])
        .concat(digestJobs.length ? ['digest'] : [])
        .concat(escalatable.length ? ['2nd-opinion'] : [])
        .concat(armedInvariants.length ? ['invariant-hunt'] : [])
        .concat(CONFIG.INVARIANTS.length ? ['coverage-critic'] : []);

    let debugRecord: DebugRecord | undefined;
    if (CONFIG.debug) {
      try {
        debugRecord = assembleDebugRecord({
          reportPath,
          invariants: CONFIG.INVARIANTS,
          armedInvariants,
          unarmedInvariants,
          seatTally: SEAT_TALLY,
          expectedSeats: computeExpectedSeats(false),
          threads,
          walks,
          jobs,
          cards,
          hunterResults,
          digestJobsLen: digestJobs.length,
          digests,
          security,
          coverageCriticResult,
          confirmed,
          killed,
          unproven,
          secondOpinionDispatched: escalatable.length,
          secondOpinionOverturned,
          secondOpinionReexaminedKilled,
          finalJudge,
          finalJudgeReinstated: finalJudgeReinstatedCount,
          unsensed,
          gateSweepSkipped,
          gateSweepSkipDetail,
          coverageGaps,
          contradictionScan,
        });
      } catch (e) {
        // outermost floor — even a bug in the assembler's own scaffolding must never break the walk.
        log(
          '⚠ debugRecord assembly failed entirely: ' +
            ((e as Error) && (e as Error).message ? (e as Error).message : String(e)),
        );
        debugRecord = {
          schemaVersion: 1,
          mode: 'walk',
          reportPath,
          degraded: true,
          gaps: [
            'full assembly failed: ' +
              ((e as Error) && (e as Error).message ? (e as Error).message : String(e)),
          ],
          armedInvariants: { registered: 0, armed: [], unarmed: [] },
          seats: {},
          seatsExpectedButAbsent: [],
          emptyResults: {
            threadsWalked: 0,
            threadsExpected: 0,
            sensorsWithCards: 0,
            sensorsExpected: 0,
            hunterFindingsTotal: 0,
            huntersReturned: 0,
            huntersExpected: 0,
            digestsWithFindings: 0,
            digestsExpected: 0,
            securityDied: false,
            coverageCriticDied: false,
            foldDied: false,
          },
          judgeStats: {
            confirmed: 0,
            falseCount: 0,
            unproven: 0,
            secondOpinionDispatched: 0,
            secondOpinionOverturned: 0,
            secondOpinionReexaminedKilled: 0,
            finalJudgeReinstated: 0,
            finalJudgeVerdict: null,
            finalJudgeDied: true,
          },
          coverage: {
            unsensedFields: [],
            gateSweepSkipped: false,
            gateSweepSkipDetail: '',
            coverageGaps: [],
            verdictContradictions: [],
            contradictionScan: { filesCompared: 0, uncomparableThreads: [] },
          },
          tokenAttribution: null,
        };
      }
    }
    // renderTelemetryMd is self-floored (never throws — see utils/index.ts), so no wrapping try/catch
    // is needed here; `debug:false` or an assembly failure both fall through to '' cleanly.
    const telemetryMd = CONFIG.debug && debugRecord ? renderTelemetryMd(debugRecord) : '';

    const fold: FoldOut | null = await runFold({
      reportPath,
      walks,
      confirmed,
      unproven,
      killedCount: killed.length,
      digests,
      security,
      coverageSummary,
      finalJudge,
      coverageGaps,
      telemetryMd,
      contradictions: contradictionScan.contradictions,
    });
    if (CONFIG.debug && debugRecord) {
      try {
        // refresh seats/seatsExpectedButAbsent now that fold has genuinely been dispatched (whether it
        // died or succeeded, its tally now exists in SEAT_TALLY) — see computeExpectedSeats above.
        debugRecord.seats = Object.fromEntries(
          Object.entries(SEAT_TALLY).map(([k, v]) => [k, { ...v }]),
        );
        debugRecord.seatsExpectedButAbsent = computeExpectedSeats(true).filter(
          (s) => !(s in debugRecord!.seats),
        );
        debugRecord.emptyResults.foldDied = !fold;
      } catch (e) {
        debugRecord.degraded = true;
        debugRecord.gaps.push(
          'post-fold patch failed: ' +
            ((e as Error) && (e as Error).message ? (e as Error).message : String(e)),
        );
      }
    }
    if (!fold)
      return {
        status: 'FAILED',
        detail: 'fold died twice',
        threads: threads.length,
        anomalies: anomalies.length,
        confirmed: confirmed.length,
        debugRecord,
      };

    const ledger: WalkLedger = {
      report: reportPath,
      headSha: scout.headSha,
      territories: scout.territories,
      changedFiles: scout.changedFiles,
      mergeShas: scout.mergeShas,
      threads,
      walks,
      cards,
      gateCards: gates,
      undeclaredReads,
      anomalies,
      verdicts,
      digests,
      security: security || null,
      coverage: coverageSummary,
      armedInvariants,
      coverageGaps,
      contradictions: contradictionScan.contradictions,
    };
    log(
      'Wave walker complete · ' +
        fold.verdict +
        ' · ' +
        coverageSummary +
        ' · ledger in result (persist to ' +
        CONFIG.LEDGER_PATH +
        ')',
    );
    return {
      status: 'DONE',
      verdict: fold.verdict,
      actionItems: fold.actionItems,
      review: fold.review,
      threads: threads.length,
      threadsWalked: walks.length,
      ledgerAnomalies: anomalies.length,
      anomaliesByRule: counts,
      confirmed: confirmed.map((v) => ({
        id: v.anomalyId,
        severity: v.severity,
        what: v.what,
        location: v.location,
      })),
      unproven: unproven.length,
      killedAsFalse: killed.length,
      overrides: verdicts.filter((v) => (v.why || '').startsWith('[OVERRIDE')).length,
      digestFindings: digests.reduce((n, d) => n + d.findings.length, 0),
      security: security
        ? {
            findings: secFindings,
            categoriesSwept: security.categoriesSwept || [],
            summary: security.summary || '',
            auditorsDispatched: security.auditorsDispatched,
            auditorsReturned: security.auditorsReturned,
            filesOpened: security.filesOpened.length,
            filesInScope: security.filesInScope,
            filesUnswept: security.filesUnswept,
          }
        : null,
      finalJudge: finalJudge
        ? {
            verdict: finalJudge.verdict,
            missedRisks: finalJudge.missedRisks.length,
            reinstated: (finalJudge.reinstated || []).length,
          }
        : null,
      unsensedFields: unsensed,
      coverage: coverageSummary,
      reportPath,
      ledgerTarget: CONFIG.LEDGER_PATH,
      ledger,
      invariantsArmed: armedIds,
      coverageGaps: coverageGaps.length,
      verdictContradictions: contradictionScan.contradictions.length,
      debugRecord,
    };
  }
}
