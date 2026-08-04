// Agent<Args> — the seat contract every agents/<seat>/index.ts exports: the seat's CONFIG-resolved
// tier/effort, its StructuredOutput schema, and its pure prompt builder. Mirrors rr's Agent<Args>.
// Below: one `<Seat>Args` interface per seat — exactly what its buildPrompt receives. run.ts does any
// shaping (.map/.filter over engine state) BEFORE calling buildPrompt, so every builder stays a pure,
// already-shaped-input → string function (mirrors rr's agents/*/prompts.ts discipline).
import type {
  Anomaly,
  Card,
  ClaimIn,
  ConflictCheck,
  Confidence,
  CoverageGap,
  DigestOut,
  Effort,
  FinalOut,
  Flow,
  InvariantSpec,
  JudgeVerdict,
  Lane,
  RuleId,
  MergedSecurity,
  ThreadSpec,
  Tier,
  VerdictContradiction,
  VerifyOut,
  WalkOut,
} from './domain.js';
import type { LeadRecord, LedgerRowOut } from './run.js';
import type { Schema } from './schema.js';

export interface Agent<Args> {
  tier: Tier;
  effort: Effort;
  schema: Schema;
  buildPrompt: (args: Args) => string;
}

// the canonical seat names (17 original + invariantHunter/coverageCritic, INVARIANT REGISTRY FEATURE) —
// matches Configs.TIER/EFFORT's keys (config.ts) and the agents/<seat>/
// directory names verbatim. Declared here (not derived from Object.keys at runtime) so the type system
// and the runtime validator agree on the exact set.
export type Seat =
  | 'scout'
  | 'threadWalker'
  | 'sliceSensor'
  | 'gateSweep'
  | 'securityAuditor'
  | 'anomalyJudge'
  | 'territoryDigest'
  | 'secondOpinion'
  | 'finalJudge'
  | 'fold'
  | 'claimExtractor'
  | 'claimVerifier'
  | 'consistencyJudge'
  | 'probe'
  | 'brainer'
  | 'claimAuditor'
  | 'synthesiser'
  | 'invariantHunter'
  | 'coverageCritic';

// ── scout (source lines 400-415) ──
export interface ScoutArgs {
  reportPath: string;
  branch: string | null;
  walkerDoc: string;
  maxFieldsPerJob: number;
  charter: string; // caller's duty note — '' appends zero prompt bytes
  // INVARIANT REGISTRY FEATURE (§ 2.1) — optional so an omitted/empty registry appends ZERO prompt
  // bytes (floor invariant); embedded verbatim rather than "go read this file" because engine.ts's
  // zero-token fail-safe arming check (computeArmedInvariants) needs the SAME structured data the
  // scout reasons over, and the JS engine has no fs access (confirmed: zero node:fs imports in src/).
  invariants?: InvariantSpec[];
  invariantsDoc?: string; // provenance pointer only, cited in the prompt — default '.claude/commands/wave/walker-invariants.md'
  // D1 FILE-SET RECONCILIATION (docs/dev/audits/wave-walker-instrument-defects-2026-07-28.md) — set
  // ONLY on the corrective retry after the first scout's enumerated list disagreed with its own
  // separately-executed git count; appends the RECONCILIATION RETRY block (zero bytes when absent).
  reconcile?: { enumerated: number; counted: number | undefined };
  // args.project profile (config.ts CONFIG.PROJECT) — repoRoot always present (default '.'); the other
  // two are raw, possibly-absent project strings the prompt renders honestly either way.
  repoRoot: string;
  authDoc?: string; // "{path} § {heading}" — buildScout splits it; absent → authRule instruction asks for ''
  gateResolverPattern?: string; // regex SOURCE for the gate-relevant surface; absent → gateFiles instruction asks for []
}

// ── threadWalker — one thread-walker call per scout thread (source lines 441-449) ──
export interface ThreadWalkerArgs {
  walkerDoc: string;
  // D3 (audit 2026-07-28): the hygiene standard the walker's integration-delta lens applies —
  // walker.md § Role: Walker step 4 always CITED audit/code-hygiene.md, but no seat was ever handed it.
  hygieneDoc: string;
  thread: ThreadSpec;
  charter: string;
}

// ── sliceSensor — one slice-sensor call per scheduled job (source lines 451-468) ──
export interface SliceSensorAssignedField {
  fieldId: string;
  field?: string;
  sdlTypeToken?: string;
}
export interface SliceSensorArgs {
  jobId: string;
  kind: 'producer' | 'consumer' | 'ai';
  files: string[];
  hint?: string;
  assigned: SliceSensorAssignedField[];
}

// ── gateSweep — one call per gate file (source lines 469-478). Only ever dispatched when the gate
// machinery is ARMED (engine.ts computeGateArming) — resourceClasses/fenceLabels are guaranteed to be
// project-supplied (or the engine's own generic defaults) by the time this runs. ──
export interface GateSweepArgs {
  file: string;
  resourceClasses: string[];
  fenceLabels: { org: string; ownership: string };
}

// ── securityAuditor — one call per changed-file slice (D2 fan-out, audit 2026-07-28: security was the
// only lens with no fan-out — one auditor over a 148-file diff opened ~45 files and reported a clean
// sweep). Each auditor sweeps its slice deeply and carries the full set as cross-file context. ──
export interface SecurityAuditorArgs {
  securityDoc: string;
  clusterFiles: string[];
  allChangedFiles: string[];
  clusterIndex: number;
  clusterCount: number;
  branch: string | null;
  mergeShas: string[];
}

// ── anomalyJudge — one call per chunk-of-6 same-rule anomalies (source lines 608-620) ──
export interface AnomalyJudgeArgs {
  rule: RuleId;
  ruleMeaning: string;
  sec: boolean;
  instances: Anomaly[];
  ctxCards: Card[];
  authDoc?: string; // args.project.authDoc — '' → the sec-framing block drops its doc-read instruction
}

// ── territoryDigest — one call per territory (source lines 630-638) ──
export interface TerritoryDigestArgs {
  territory: string;
  slice: unknown[];
  charter: string;
}

// ── secondOpinion — one call per chunk-of-4 escalatable killed verdicts (source lines 649-655) ──
export interface SecondOpinionItem {
  verdict: JudgeVerdict;
  anomaly: Anomaly | undefined;
}
export interface SecondOpinionArgs {
  authRule: string;
  items: SecondOpinionItem[];
  // INVARIANT REGISTRY FEATURE (§ 2.3, escalation symmetry) — optional, defaults to the ORIGINAL
  // 'a first judge killed these as FALSE' framing (byte-identical when omitted). 'confirmed' is the new
  // direction: a first judge CONFIRMED an R9-INV high/critical finding — re-examine before it reaches
  // the founder, symmetric to the existing killed-verdict escalation.
  direction?: 'killed' | 'confirmed';
}

// ── finalJudge — one call ruling the whole walk (source lines 673-685) ──
export interface WalkBrief {
  id: string;
  name?: string;
  flow: Flow;
  defects: WalkOut['defects'];
  notes?: string;
}
export interface KilledWithAnomaly {
  anomalyId: string;
  why?: string;
  anomaly: Anomaly | undefined;
}
export interface FinalJudgeArgs {
  walksBrief: WalkBrief[];
  confirmed: JudgeVerdict[];
  unproven: JudgeVerdict[];
  killedWithAnomaly: KilledWithAnomaly[];
  digests: DigestOut[];
  securityDoc: string;
  security: MergedSecurity | null;
  walksLen: number;
  threadsLen: number;
  cardsLen: number;
  unsensed: string[];
  charter: string;
  // INVARIANT REGISTRY FEATURE (§ 2.4) — optional, defaults to [] (zero added prompt bytes, same
  // `.length ?` idiom as ctxCards elsewhere in this file).
  coverageGaps?: CoverageGap[];
  // VERDICT CONTRADICTIONS (walker.md § Orchestration) — the named clean-vs-flagged pairs this walk
  // found over one file; optional, [] ⇒ zero added prompt bytes.
  contradictions?: VerdictContradiction[];
}

// ── fold — the one call merging everything into the report (source lines 700-712) ──
export interface FoldArgs {
  reportPath: string;
  walks: WalkOut[];
  confirmed: JudgeVerdict[];
  unproven: JudgeVerdict[];
  killedCount: number;
  digests: DigestOut[];
  security: MergedSecurity | null;
  coverageSummary: string;
  finalJudge: FinalOut | null;
  // INVARIANT REGISTRY FEATURE (§ 2.4) — optional, defaults to [] (zero added prompt bytes).
  coverageGaps?: CoverageGap[];
  // WALK TELEMETRY (DEBUG STEP) — a pre-built, mechanically-rendered `### Walk Telemetry` markdown
  // block; fold copies it VERBATIM into its own review output (see agents/fold/prompts.ts). Optional,
  // empty/absent → zero added prompt bytes (CONFIG.debug:false, or every walk before this feature).
  telemetryMd?: string;
  // VERDICT CONTRADICTIONS (walker.md § Orchestration) — the named pairs the review must carry in
  // Coverage alongside the final judge's ruling; optional, [] ⇒ zero added prompt bytes.
  contradictions?: VerdictContradiction[];
}

// ── claimExtractor — the manifest claim extractor (source lines 68-77) ──
export interface ClaimExtractorArgs {
  manifestPath: string;
  repoRoot: string; // args.project.repoRoot — default '.' (the working directory)
}

// ── claimVerifier — one call per claim × vote (source lines 96-105) ──
export interface ClaimVerifierArgs {
  claim: ClaimIn;
  question: string;
  repoRoot: string; // args.project.repoRoot — default '.' (the working directory)
}

// ── consistencyJudge — the manifest consistency judge (source lines 118-128), E2 payload diet:
// the consensus map covers every claim; full verdict detail rides ONLY for non-CONFIRMED claims
// (REFUTED/PARTIAL/UNPROVEN/NO-VERDICT). ──
export interface ConsistencyJudgeArgs {
  manifestPath: string;
  nonConfirmed: VerifyOut[];
  consensus: Record<string, string>;
  conflictChecks: ConflictCheck[];
}

// ── probe — one call per investigate lane (source lines 217-225) ──
export interface ProbeArgs {
  lane: Lane;
  goal: string;
  scopeLine: string;
}

// ── brainer (investigate) — one call per wave (source lines 236-242) ──
export interface BrainerLedgerRow {
  id: string;
  s: string;
  status: string;
  files: number;
  survived: number;
  audit: string;
}
export interface BrainerArgs {
  goal: string;
  scopeLine: string;
  wave: number;
  maxWaves: number;
  ledgerRows: BrainerLedgerRow[];
  openLeads: LeadRecord[];
  maxLanes: number;
}

// ── claimAuditor (investigate claim auditor) — one call per wave with pending claims (source lines 209-216) ──
export interface ClaimAuditorRow {
  id: string;
  anchors: { anchor: string; quote?: string }[];
}
export interface ClaimAuditorArgs {
  rows: ClaimAuditorRow[];
  wave: number;
  repoRoot: string; // args.project.repoRoot — default '.' (the working directory)
}

// ── synthesiser (investigate synthesiser) — the one closing call (source lines 259-266) ──
export interface SynthesiserArgs {
  goal: string;
  stopReason: string;
  keyIds: string[];
  conf: Confidence;
  reportOut: string | null;
  resultSoFarText: string;
  claimsOut: LedgerRowOut[];
  openLeads: LeadRecord[];
}

// ── invariantHunter — one call per ARMED registry entry, territory-scoped (INVARIANT REGISTRY FEATURE
// § 2.2), dispatched inside the existing Phase-1 barrier alongside threadWalker/sliceSensor/gateSweep. ──
export interface InvariantHunterArgs {
  invariant: InvariantSpec;
  matchedFiles: string[];
}

// ── coverageCritic — one call after Phase 1, before the final judge (§ 2.4); only dispatched when the
// registry is non-empty (CONFIG.INVARIANTS.length > 0) — the floor invariant's "no critic dispatched
// on an absent/empty registry" holds by construction. ──
export interface CoverageCriticArgs {
  changedFiles: string[];
  threadNames: { id: string; type?: string; name?: string }[];
  hunterCoverage: { invariantId: string; coverage: string }[];
  armedIds: string[];
  unarmedInvariants: { id: string; territory: string[] }[];
  unsensed: string[];
}
