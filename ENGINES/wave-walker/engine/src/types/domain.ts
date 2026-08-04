// Domain types — the WALK-mode + VERIFY-mode + MANIFEST-VERIFY-mode data shapes, mirroring the
// StructuredOutput schemas each agent seat returns (wave-walker.js lines 304-485). Field optionality
// matches the schemas' own `required` arrays — a field absent from `required` is optional here too.

export type Tier = 'haiku' | 'sonnet' | 'opus';
// full CLAUDE.md § Model Selection effort vocabulary — 'low'/'max' are legal override targets even
// though the doc says never default to them ("Low never; Max never unless I say"); validation here
// only rejects an unknown token, not a rare-but-intentional founder override.
export type Effort = 'low' | 'medium' | 'high' | 'xhigh' | 'max';
export type Severity = 'info' | 'low' | 'med' | 'high' | 'critical';

// ── ProjectProfile (args.project) — the ONE runtime handle every project-specific site reads through;
// see config.ts CONFIG.PROJECT. Absent, or missing its gate-relevant subset (roles + fencedResourceClasses
// + both gate patterns), disarms the gate machinery for the whole walk (engine.ts computeGateArming) —
// LOUD, never silent. The thread walk / security fan-out / hygiene / verify / manifest-verify / investigate
// modes never depend on this — they run fully regardless, with repoRoot defaulting to '.' (the working
// directory) when no profile (or no repoRoot) is supplied.
export interface ProjectProfile {
  repoRoot?: string; // prompt-facing "Repo root: X"; default '.' = the working directory
  authDoc?: string; // e.g. "{backend}/CLAUDE.md § Auth Pattern" — doc path + heading text both derive from this one string
  authRuleFallback?: string; // verbatim declared-copy fallback; default '' = no fallback
  authRuleMustContain?: string[]; // sanity tokens a usable scout-extracted authRule must contain; default []
  roles?: { owner: string; elevated: string };
  resourceClasses?: string[]; // full sensor-card enum
  fencedResourceClasses?: string[]; // subset R6's mandated-fence rule protects
  fenceLabels?: { org: string; ownership: string }; // prompt-facing fence names
  gateResolverPattern?: string; // regex SOURCE for resolver files (compiled with try/catch)
  gateSurfacePattern?: string; // regex SOURCE for the gate-relevant surface
  deadnessSurfaces?: string; // this project's registry-style consumer list for the deadness bar
  stakesLine?: string; // domain-stakes clause for the deadness bar; generic default when absent
}

// ── Scout (SCOUT schema) ──
export interface ThreadSpec {
  id: string;
  type: string;
  name: string;
  scope?: string;
  files?: string[];
  verify: string;
}
export interface OperationSpec {
  id: string;
  kind?: string;
  anchor: string;
  resultType?: string;
}
export interface SdlRef {
  anchor?: string;
  typeToken?: string;
}
export interface FieldSpec {
  id: string;
  ownerType: string;
  field: string;
  apis?: string[];
  sdl?: SdlRef;
}
export type JobKind = 'producer' | 'consumer' | 'ai';
export interface SliceJob {
  jobId: string;
  kind: JobKind;
  files: string[];
  fieldIds: string[];
  hint?: string;
}
export interface ScoutOut {
  headSha?: string;
  territories?: string[];
  changedFiles: string[];
  // D1 FILE-SET RECONCILIATION (audit 2026-07-28) — the scout's separately-executed `wc -l` count of
  // the same name-only diff, NEVER derived from the enumerated list. The engine fails the walk when the
  // two disagree (one corrective retry first); optional in the type so a pre-feature cached scout
  // result parses — the engine treats absence as a mismatch.
  changedFileCount?: number;
  mergeShas?: string[];
  threads: ThreadSpec[];
  operations?: OperationSpec[];
  fields: FieldSpec[];
  jobs: SliceJob[];
  gateFiles: string[];
  authRule?: string;
  // INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — optional, [] when no registry
  // was supplied. engine.ts computeArmedInvariants UNIONS this with its own territory-glob fail-safe.
  armedInvariants?: { id: string; matchedFiles?: string[]; reason?: string }[];
}

// ── Walker (WALK schema) ──
export type Flow = 'INTACT' | 'AT-RISK' | 'BROKEN' | 'N/A';
export interface Defect {
  what: string;
  location: string;
  jc?: string;
  // INVARIANT-REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.5) — optional so it's additive to
  // the existing WALK schema; the reoriented thread-walker prompt asks for a concrete failure scenario
  // per defect, but a defect emitted before this field existed (or one where none applies) stays valid.
  failureScenario?: string;
}
export interface Hygiene {
  kind: string;
  where: string;
  detail: string;
  jc?: string;
}
export interface WalkOut {
  threadId: string;
  name?: string;
  type?: string;
  flow: Flow;
  trace: string;
  defects: Defect[];
  hygiene: Hygiene[];
  notes?: string;
}

// ── Slice sensor (SLICES / SLICE_PROPS schema) ──
export interface SliceProducer {
  anchor?: string;
  writer?: string;
  typeToken?: string;
  encoding?: string;
  valueLiterals?: string[];
}
export interface SliceDbColumn {
  anchor?: string;
  columnName?: string;
  columnType?: string;
  checkLiterals?: string[];
}
export interface SliceResolver {
  anchor?: string;
}
export interface SliceFeSelection {
  anchor?: string;
  queryName?: string;
}
export interface SliceFeType {
  anchor?: string;
  typeToken?: string;
  kind?: 'generated' | 'hand';
}
export interface SliceConsumer {
  anchor: string;
  name?: string;
  decode?: string;
  decodeExpr?: string;
  context?: 'production' | 'test' | 'generated' | 'story';
  comparedLiterals?: string[];
  aliasChain?: string[];
}
export interface DanglingRef {
  ref?: string;
  anchor?: string;
}
export interface Slice {
  fieldId: string;
  producer?: SliceProducer;
  dbColumn?: SliceDbColumn;
  resolver?: SliceResolver;
  feSelection?: SliceFeSelection;
  feTypes?: SliceFeType[];
  consumers?: SliceConsumer[];
  danglingRefs?: DanglingRef[];
  notes?: string;
}
export interface UndeclaredRead {
  side?: 'be' | 'fe';
  property: string;
  anchor: string;
  expr?: string;
}
export interface SlicesOut {
  jobId: string;
  slices: Slice[];
  undeclaredReads?: UndeclaredRead[];
}

// ── Gate sweep (GATE_SWEEP schema) ──
export interface GateCard {
  id: string;
  kind?: string;
  resource?: string;
  anchor: string;
  idArgs?: string[];
  rolesAllowed?: string[];
  chain?: string[];
  orgFence?: boolean;
  ownershipFence?: boolean;
  notes?: string;
}
export interface GateSweepOut {
  file: string;
  gates: GateCard[];
}
// a gate card carried out of the parallel barrier with its owning file attached (engine.ts flattens
// GateSweepOut[] → GateCardWithFile[] the same way the source's inline `.flatMap` does).
export interface GateCardWithFile extends GateCard {
  file: string;
}

// ── Zipped card (mechanical, zero-token zip of every slice job's output onto one field) ──
export interface Card {
  id: string;
  ownerType: string;
  field: string;
  apis: string[];
  sdl: SdlRef | null;
  producer?: SliceProducer;
  dbColumn?: SliceDbColumn;
  resolver?: SliceResolver;
  feSelection?: SliceFeSelection;
  feTypes: SliceFeType[];
  consumers: SliceConsumer[];
  danglingRefs: DanglingRef[];
  notes?: string;
  sidesCovered: string[];
}

// ── Ledger diff (R1-R8 mechanical rule engine, plus R9-INV — rules.ts) ── R9-INV is NOT produced by
// computeAnomalies; it is produced by computeInvariantAnomalies from invariantHunter findings (INVARIANT
// REGISTRY FEATURE, tmp/wave-walker-investigation.md § 2.1-2.3) and merged into the SAME anomalies array
// so it rides the existing byRule → judge → escalation → final-judge → fold pipeline unchanged.
export type RuleId = 'R1' | 'R2' | 'R3' | 'R4' | 'R5' | 'R6' | 'R7' | 'R8' | 'R9-INV';
export interface Anomaly {
  id: string;
  rule: RuleId;
  ruleName: string;
  detail: string;
  anchors: string[];
  severityHint: Severity;
  cardId: string | null;
}

// ── Judge (JUDGE schema) ──
export type Verdict = 'CONFIRMED' | 'FALSE' | 'UNPROVEN';
export interface JudgeVerdict {
  anomalyId: string;
  verdict: Verdict;
  severity?: Severity;
  what: string;
  location?: string;
  fix?: string;
  why?: string;
}
export interface JudgeOut {
  verdicts: JudgeVerdict[];
}

// ── Territory digest (DIGEST schema) ──
export interface DigestFinding {
  lens: string;
  severity: 'info' | 'low' | 'med' | 'high';
  what: string;
  location: string;
  fix?: string;
}
export interface DigestOut {
  territory: string;
  findings: DigestFinding[];
  summary: string;
}

// ── Security auditor (SECURITY schema) ──
export interface SecurityFinding {
  id: string;
  category: string;
  severity: Severity;
  what: string;
  location: string;
  expected?: string;
  got?: string;
  fix?: string;
}
export interface SecurityOut {
  findings: SecurityFinding[];
  categoriesSwept: string[];
  summary: string;
  // D2 SECURITY FAN-OUT (audit 2026-07-28) — per-auditor honesty: every file the auditor actually read,
  // and every assigned file it did not (with why). The merge derives the walk's denominator from these.
  filesOpened?: string[];
  filesSkipped?: { file: string; why?: string }[];
}

// The engine's zero-token merge of the per-slice auditor results — ONE SecurityOut-shaped object whose
// headline carries its own denominator: a sweep can never again read as complete without saying what it
// opened. null only when EVERY slice auditor died (the AUDIT DIED path).
export interface MergedSecurity extends SecurityOut {
  filesOpened: string[];
  auditorsDispatched: number;
  auditorsReturned: number;
  filesInScope: number;
  filesUnswept: string[];
}

// ── Final judge (FINAL schema) ──
export interface Reinstated {
  anomalyId: string;
  why: string;
}
export interface MissedRisk {
  what: string;
  where: string;
  severity?: string;
  jc?: string;
}
export interface FinalOut {
  verdict: 'SMOOTH SAILING' | 'MOSTLY GOOD' | 'ROUGH SEAS' | 'SHIPWRECK';
  reinstated?: Reinstated[];
  missedRisks: MissedRisk[];
  rationale?: string;
}

// ── Fold (FOLD schema) ──
export interface FoldOut {
  verdict: 'SMOOTH SAILING' | 'MOSTLY GOOD' | 'ROUGH SEAS' | 'SHIPWRECK';
  actionItems: string[];
  review: string;
}

// ── VERIFY / MANIFEST-VERIFY mode ──
export interface ClaimIn {
  id: string;
  statement: string;
  taskId?: string;
  files?: string[];
  context?: string;
  opus?: boolean;
}
export interface Evidence {
  anchor: string;
  quote?: string;
}
export type ClaimVerdict = 'CONFIRMED' | 'REFUTED' | 'PARTIAL' | 'UNPROVEN';
export interface VerifyOut {
  claimId: string;
  verdict: ClaimVerdict;
  evidence?: Evidence[];
  reasoning: string;
}
export interface ExtractedClaim {
  id: string;
  taskId?: string;
  kind?: string;
  statement: string;
  files?: string[];
  context?: string;
}
export interface ConflictCheck {
  id: string;
  tasks?: string[];
  what: string;
}
export interface ExtractOut {
  claims: ExtractedClaim[];
  conflictChecks: ConflictCheck[];
}
export interface ConflictFinding {
  id: string;
  kind: 'conflict' | 'refuted-premise' | 'freeloader';
  tasks?: string[];
  what: string;
  evidence?: string;
  severity: Severity;
  fix?: string;
}
export interface ConflictOut {
  conflicts: ConflictFinding[];
  summary: string;
}

// ── INVESTIGATE mode ──
export interface ProbeAnchor {
  anchor: string;
  quote: string;
}
export interface ProbeClaim {
  statement: string;
  kind?: 'support' | 'counter';
  targets?: string[];
  anchors: ProbeAnchor[];
}
export interface Lead {
  what: string;
  files?: string[];
}
export interface ProbeOut {
  laneId: string;
  claims: ProbeClaim[];
  leads: Lead[];
  nothingFound?: boolean;
  // engine-attached bookkeeping (mirrors the source's `.then(r => Object.assign(r, {_laneKind, _targets}))`)
  _laneKind?: string;
  _targets?: string[];
}
export type LaneKind = 'pursue' | 'attack';
export interface Lane {
  id: string;
  kind?: LaneKind;
  question: string;
  files?: string[];
  targets?: string[];
  note?: string;
}
export interface Stop {
  done: boolean;
  reason?: string;
}
export interface CoordOut {
  resultSoFar: string;
  keyClaimIds: string[];
  lanes: Lane[];
  dropLeads?: string[];
  stop: Stop;
}
export type AuditVerdict = 'pass' | 'fail';
export interface AuditItem {
  id: string;
  result: AuditVerdict;
}
export interface AuditsOut {
  audits: AuditItem[];
}
export interface LedgerRow {
  id: string;
  statement: string;
  anchors: ProbeAnchor[];
  files: string[];
  contested: boolean;
  survived: number;
  audit: 'pending' | AuditVerdict;
  wave: number;
}
export type Confidence = 'low' | 'medium' | 'high';
export interface SynthOut {
  answer: string;
  confidence: Confidence;
  report: string;
}

// ── INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1-2.4) — the missing piece: a
// durable, machine-readable registry of sacred cross-cutting semantics, hunted by a dedicated adversarial
// seat over TERRITORY (not just the diff), judged by the existing anomaly pipeline as rule class R9-INV,
// with an external coverageCritic naming what the walk itself never inspected. ──

// one registry entry — arrives via args.invariants (see config.ts CONFIG.INVARIANTS); the doc at
// CONFIG.INVARIANTS_DOC is the human-authored source these are transcribed from (see result doc for the
// promotion step that parses the doc into this shape).
export interface InvariantSpec {
  id: string;
  law: string; // the invariant law VERBATIM + its source pointer (e.g. "... — services/worker/CLAUDE.md:159")
  territory: string[]; // glob patterns (supports * and **, no brace expansion) — see utils/index.ts globMatch
  triggers: string[]; // free-text diff predicates the scout judges semantically (territory match is the zero-token fail-safe floor)
  exemplars: string[]; // 2-4 already-confirmed bugs of this class — what makes a hunter sharp
  huntBrief: string; // the enumeration duty handed to the invariantHunter verbatim
}

// an invariant this walk actually armed — the union of the scout's semantic judgment and the engine's
// zero-token territory-glob fail-safe (engine.ts computeArmedInvariants; "a scout omission cannot
// silently disarm a hunt", mirroring isGateRelevant's own fail-safe philosophy).
export interface ArmedInvariant {
  id: string;
  matchedFiles: string[];
  reason?: string;
}

// ── invariantHunter (the adversarial finder seat) ──
export interface InvariantFinding {
  what: string;
  location: string;
  expected: string;
  got: string;
  failureScenario: string; // REQUIRED — refute-first discipline: name the concrete harm, not a vibe
  severity: Severity;
  fix?: string;
}
export interface InvariantHunterOut {
  invariantId: string;
  findings: InvariantFinding[];
  coverage: string; // what this hunter walked AND what it skipped — "an empty enumeration is never a verdict"
}

// ── coverageCritic (the external denominator) ──
export interface CoverageGap {
  territory: string;
  why: string;
}
export interface CoverageCriticOut {
  gaps: CoverageGap[]; // ≤8 — territories THIS WALK could not see
  summary: string;
}

// ── verdict contradictions (walker.md § Orchestration — the contradiction escalation) ──
// Two or more seats walking the SAME file and returning opposite verdicts is not a spread to average:
// one of them is wrong about the file in front of it, and the OPTIMISTIC verdict is the dangerous side
// (a clean verdict built on evidence the file does not contain reads exactly like an earned one).
export interface ContradictionSeat {
  threadId: string;
  name?: string;
  flow: Flow;
  defects: number;
}
export interface VerdictContradiction {
  file: string;
  clean: ContradictionSeat[]; // INTACT
  flagged: ContradictionSeat[]; // AT-RISK | BROKEN
}
// The scan's own COVERAGE LINE — what it compared and what it could NOT, so an empty contradiction list
// is never read as agreement (root CLAUDE.md: "an empty enumeration is never a verdict").
export interface ContradictionScan {
  contradictions: VerdictContradiction[];
  filesCompared: number; // files walked by ≥2 seats — the only files a contradiction CAN surface on
  uncomparableThreads: string[]; // walked threads whose spec named no files: never comparable, never silent
}
