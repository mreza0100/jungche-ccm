// Run infra types — the untrusted injected args, agent-call options, and the per-mode result shapes
// WaveWalker.run() resolves to (mirroring the FOUR distinct literal shapes the source `return`s).
import type { Schema } from './schema.js';
import type {
  Anomaly,
  ArmedInvariant,
  Card,
  Confidence,
  ConflictFinding,
  CoverageGap,
  Effort,
  GateCardWithFile,
  JudgeVerdict,
  ProbeAnchor,
  SecurityFinding,
  MergedSecurity,
  Severity,
  Tier,
  ThreadSpec,
  UndeclaredRead,
  VerdictContradiction,
  VerifyOut,
  WalkOut,
  DigestOut,
} from './domain.js';

// the injected JSON args — anything until Configs validates it, so every field is `unknown`.
export interface RawArgs {
  reportPath?: unknown;
  branch?: unknown;
  ledgerPath?: unknown;
  sensorModel?: unknown;
  sensorEffort?: unknown;
  scoutModel?: unknown;
  walkerModel?: unknown;
  judgeModel?: unknown;
  digestModel?: unknown;
  foldModel?: unknown;
  securityEscalateModel?: unknown;
  maxFieldsPerJob?: unknown;
  maxSensors?: unknown;
  claims?: unknown;
  manifestPath?: unknown;
  maxClaims?: unknown;
  soloThreshold?: unknown;
  question?: unknown;
  votes?: unknown;
  verifierModel?: unknown;
  verifierEffort?: unknown;
  securityModel?: unknown;
  securityEffort?: unknown;
  goal?: unknown;
  scope?: unknown;
  lenses?: unknown;
  maxWaves?: unknown;
  maxLanes?: unknown;
  probeModel?: unknown;
  probeEffort?: unknown;
  brainerModel?: unknown;
  brainerEffort?: unknown;
  auditModel?: unknown;
  synthModel?: unknown;
  reportOut?: unknown;
  extraThreads?: unknown;
  finalJudgeModel?: unknown;
  fullGateSweep?: unknown;
  charter?: unknown;
  agents?: unknown;
  // D2 SECURITY FAN-OUT (audit 2026-07-28) — changed files per security-auditor slice (default 12).
  securityFilesPerAuditor?: unknown;
  // INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — args.invariants is the
  // structured registry (see config.ts CONFIG.INVARIANTS); absent/empty ⇒ the floor (no hunter/critic
  // dispatched, byte-identical to today). args.invariantsDoc overrides the doc-path provenance pointer
  // cited in the scout prompt — never re-read by the JS engine itself (no fs access in src/).
  invariants?: unknown;
  invariantsDoc?: unknown;
  // WALK TELEMETRY (DEBUG STEP) — args.debug mirrors rr's config.ts debugger flag, DEFAULT ON (see
  // config.ts CONFIG.debug). args.debugPath overrides the derived walker-debug.json sibling path.
  debug?: unknown;
  debugPath?: unknown;
  // PROJECT PROFILE (universal-bundle refactor) — the caller's per-invocation project profile; see
  // config.ts CONFIG.PROJECT / types/domain.ts ProjectProfile. Absent ⇒ every project-specific site
  // (gate machinery, auth-rule extraction, resource-class vocabulary) runs generic/degraded-loud —
  // never silently baked to one project.
  project?: unknown;
}

// per-call options handed to agent()/resilient() alongside the prompt.
export interface AgentOpts {
  label: string;
  phase?: string;
  model?: Tier;
  effort?: Effort;
  schema?: Schema;
  agentType?: string;
}

// ── result shapes — one per mode/exit-point, mirroring the source's distinct literal `return`s ──
export interface FailedResult {
  status: 'FAILED';
  detail: string;
  threads?: number;
  anomalies?: number;
  confirmed?: number;
  // WALK TELEMETRY — populated ONLY on the fold-death path, where Phase 1-3.5 data is already in
  // scope (see engine.ts runWalk) — the single highest-value floor fix: today this path returns
  // nothing but a detail string. Absent on every earlier WALK-mode FAILED return (scout died, empty
  // diff, zero threads) — those precede any telemetry worth reporting.
  debugRecord?: DebugRecord;
}

export interface WalkLedger {
  report: string;
  headSha?: string;
  territories?: string[];
  changedFiles: string[];
  mergeShas?: string[];
  threads: ThreadSpec[];
  walks: WalkOut[];
  cards: Card[];
  gateCards: GateCardWithFile[];
  undeclaredReads: UndeclaredRead[];
  anomalies: Anomaly[];
  verdicts: JudgeVerdict[];
  digests: DigestOut[];
  security: MergedSecurity | null;
  coverage: string;
  // INVARIANT REGISTRY FEATURE — armed invariants (empty when the registry is absent/empty) and the
  // coverageCritic's named gaps travel in the ledger for auditability, same as `security`.
  armedInvariants: ArmedInvariant[];
  coverageGaps: CoverageGap[];
  // VERDICT CONTRADICTIONS — the escalated clean-vs-flagged pairs, in the ledger for auditability so a
  // reader can see which seat the final judge ruled against, and on which file.
  contradictions: VerdictContradiction[];
}

// WALK TELEMETRY (DEBUG STEP) — a structured, mechanically-computed self-report of THIS walk's own
// health: per-seat retry tallies and the coverage-declaration checks (armed invariants, unsensed
// fields, dead seats). Zero LLM cost to compute; rides the RETURN VALUE exactly like `ledger` (no new
// fs/persistence mechanism — see engine.ts assembleDebugRecord / utils/index.ts renderTelemetryMd).
// NO PHASE TIMINGS: the design's Date.now()-based phase-timing plan is dropped — DEVIATION from
// tmp/walker-debug-design.md §4/§6, forced by this project's OWN existing sandbox contract
// (validate-bundle.js's determinism guard forbids `Date.now()` in the bundled Workflow script, "mirrors
// the harness static guard" — confirmed: globals.d.ts declares no clock primitive among the ambient
// harness globals `agent`/`parallel`/`phase`/`log`/`workflow`/`args`/`budget`). Building this would fail
// `npm run build`'s own bundle-validity gate, and likely misbehave at the real harness layer too — a
// pre-existing, unrelated-to-this-feature constraint that source reality wins against the design doc.
export interface SeatTally {
  calls: number; // total agent() dispatches under this label prefix
  diedFirstAttempt: number; // agent() returned null on the FIRST call (runtime.ts retryAgent)
  retried: number; // the retry branch fired
  diedAfterRetry: number; // still null after the ONE retry — degraded to null for good
}

export interface DebugRecord {
  schemaVersion: 1;
  mode: 'walk';
  reportPath: string;
  degraded: boolean; // true iff ANY section below failed to assemble
  gaps: string[]; // named, human-readable — never an empty array standing in for "nothing wrong"
  armedInvariants: {
    registered: number;
    armed: { id: string; matchedFiles: string[]; reason: string }[];
    unarmed: { id: string; territory: string[] }[];
  };
  seats: Record<string, SeatTally>; // keyed by the label PREFIX — populated dynamically, a seat never dispatched simply never appears
  seatsExpectedButAbsent: string[]; // the coverage-declaration check — a seat THIS WALK scheduled but never actually called
  emptyResults: {
    threadsWalked: number;
    threadsExpected: number;
    sensorsWithCards: number;
    sensorsExpected: number;
    hunterFindingsTotal: number;
    huntersReturned: number;
    huntersExpected: number;
    digestsWithFindings: number;
    digestsExpected: number;
    securityDied: boolean;
    coverageCriticDied: boolean; // registry non-empty but coverageCriticResult null
    foldDied: boolean;
  };
  judgeStats: {
    confirmed: number;
    falseCount: number;
    unproven: number;
    secondOpinionDispatched: number;
    secondOpinionOverturned: number; // FALSE -> non-FALSE on second opinion
    secondOpinionReexaminedKilled: number; // CONFIRMED -> FALSE on second opinion
    finalJudgeReinstated: number;
    finalJudgeVerdict: string | null;
    finalJudgeDied: boolean;
  };
  coverage: {
    unsensedFields: string[];
    gateSweepSkipped: boolean;
    // the exact skip-reason fragment rendered after "gates: " in coverageSummary (e.g. 'SKIPPED — no
    // project profile supplied', 'SKIPPED — invalid gate pattern: …', 'SKIPPED (diff-scoped)'); '' when
    // gateSweepSkipped is false — the gate machinery is never reported skipped without saying why.
    gateSweepSkipDetail: string;
    coverageGaps: { territory: string; why: string }[];
    // VERDICT CONTRADICTIONS — named pairs (clean seat vs flagged seat over ONE file) plus the scan's
    // own coverage line: a zero here means "compared N multi-seat files and found none", never "nobody
    // looked" (walker.md § Orchestration).
    verdictContradictions: { file: string; clean: string[]; flagged: string[] }[];
    contradictionScan: { filesCompared: number; uncomparableThreads: string[] };
  };
  tokenAttribution: null; // NAMED GAP — not available at the engine layer (agent() returns no usage metadata); use the p:tokens skill post-hoc
}

export interface WalkResult {
  status: 'DONE';
  verdict: string;
  actionItems: string[];
  review: string;
  threads: number;
  threadsWalked: number;
  ledgerAnomalies: number;
  anomaliesByRule: Record<string, number>;
  confirmed: { id: string; severity?: Severity; what: string; location?: string }[];
  unproven: number;
  killedAsFalse: number;
  overrides: number;
  digestFindings: number;
  security: {
    findings: SecurityFinding[];
    categoriesSwept: string[];
    summary: string;
    // D2 SECURITY FAN-OUT — the sweep's own denominator rides the result, so a caller can never quote
    // "0 findings" without the coverage that earned it.
    auditorsDispatched: number;
    auditorsReturned: number;
    filesOpened: number;
    filesInScope: number;
    filesUnswept: string[];
  } | null;
  finalJudge: { verdict: string; missedRisks: number; reinstated: number } | null;
  unsensedFields: string[];
  coverage: string;
  reportPath: string;
  ledgerTarget: string | null;
  ledger: WalkLedger;
  // INVARIANT REGISTRY FEATURE — quick-access summary (full detail rides in `ledger`); [] when the
  // registry is absent/empty, honoring the floor invariant.
  invariantsArmed: string[];
  coverageGaps: number;
  // VERDICT CONTRADICTIONS — quick-access count (the named pairs ride in `ledger.contradictions`); a
  // non-zero value means the final judge was handed a contradiction to rule, never that one was averaged.
  verdictContradictions: number;
  // WALK TELEMETRY — undefined when CONFIG.debug is false (never a fake empty object masquerading as
  // "collected and clean" — see config.ts CONFIG.debug, default true).
  debugRecord?: DebugRecord;
}

export interface VerifyResult {
  status: 'DONE';
  mode: 'verify' | 'manifest-verify';
  manifest: string | null;
  question?: string;
  claims: number;
  votes?: number;
  verdicts: VerifyOut[];
  consensus: Record<string, string>;
  conflicts: ConflictFinding[];
  verifiersDied: number;
  // E2 coverage fields — mined vs verified vs dropped, and which tasks the verified claims cover.
  claimsMined: number;
  claimsVerified: number;
  droppedClaimIds: string[];
  taskIds: string[];
}

export interface LeadRecord {
  id: string;
  what: string;
  files: string[];
}
export interface LedgerRowOut {
  id: string;
  statement: string;
  status: string;
  anchors: ProbeAnchor[];
  files: string[];
  survived: number;
  audit: string;
}
export interface InvestigateResult {
  status: 'DONE';
  mode: 'investigate';
  goal: string;
  stopReason: string;
  answer: string;
  confidence: Confidence;
  computedConfidence: Confidence;
  keyClaimIds: string[];
  claims: LedgerRowOut[];
  openLeads: LeadRecord[];
  report: string | null;
  reportOut: string | null;
  degraded: boolean;
}

export type WaveWalkerResult = FailedResult | WalkResult | VerifyResult | InvestigateResult;
