export type ParseResult<T> =
  | { readonly ok: true; readonly value: T }
  | { readonly ok: false; readonly errors: readonly string[] };

export type CoverageStatus = "READ" | "SKIP";

export interface CoverageLine {
  readonly kind: "coverage";
  readonly index: number;
  readonly status: CoverageStatus;
  readonly reason: string;
}

export type ConductKind = "technique" | "prior" | "baseline";

export interface ConductLine {
  readonly kind: "conduct";
  readonly conductKind: ConductKind;
  readonly slug: string;
  readonly reason: string;
}

export interface CoverageArtifact {
  readonly coverage: readonly CoverageLine[];
  readonly conduct: readonly ConductLine[];
}

export type GitObjectType = "blob" | "tree";

export interface AnchorRow {
  readonly displayPath: string;
  readonly lookupPath: string;
  readonly objectType: GitObjectType;
  readonly hash: string;
}

export type SeatVerdict = "CONFIRM" | "AMEND" | "REFUTE";
export type NormalizedVerdictKind = SeatVerdict | "UNRULED";

export interface Verdict {
  readonly verdict: SeatVerdict;
  readonly mapPath: string;
  readonly evidence: string;
}

export interface NormalizedVerdict {
  readonly verdict: NormalizedVerdictKind;
  readonly mapPath: string;
  readonly evidence: string;
}

export interface Census {
  readonly windowMetaCount: number;
  readonly agentMetaCount: number;
  readonly pairedTranscriptCount: number;
  readonly selectedPairedTranscriptCount: number;
  readonly omittedPairedTranscriptCount: number;
  readonly coverageGapCount: number;
  readonly excludedOtherAgentOrInvalidCount: number;
  readonly invalidMetaCount: number;
}

export type HoldState = "READY" | "ZERO-SURVIVORS" | "ZERO-YIELD";

export interface LaneProfile {
  readonly agentType: string;
  readonly lane: string;
  readonly path: string;
  readonly body: string;
}

export interface StageLayout {
  readonly root: string;
  readonly maps: string;
  readonly meta: string;
  readonly paths: string;
  readonly pin: string;
  readonly coverage: string;
  readonly verdicts: string;
  readonly normalizedVerdicts: string;
  readonly structuredLog: string;
  readonly humanLog: string;
}

export interface RepoContext {
  readonly repoRoot: string;
  readonly organ: string;
  readonly registry: string;
}

export interface LaneContext {
  readonly agentType: string;
  readonly lane: string;
}
