// Agents barrel — re-exports every seat's Agent object + run function so engine.ts (and tests) import
// from one path. The bundler (build.js) concatenates the seat modules directly (see build.js's ORDER
// discovery) and strips this barrel's re-exports — at runtime every seat const already lives in the flat
// scope. This file only serves the dev/test import path (mirrors rr's agents/index.ts).
export { scout } from './scout/index.js';
export { runScout } from './scout/run.js';
export { threadWalker, WALK } from './threadWalker/index.js';
export { runThreadWalker } from './threadWalker/run.js';
export { sliceSensor, SLICE_PROPS, SLICES } from './sliceSensor/index.js';
export { runSliceSensor } from './sliceSensor/run.js';
export { gateSweep, GATE_SWEEP } from './gateSweep/index.js';
export { runGateSweep } from './gateSweep/run.js';
export { securityAuditor, SECURITY } from './securityAuditor/index.js';
export { runSecurityAuditor } from './securityAuditor/run.js';
export { anomalyJudge, JUDGE } from './anomalyJudge/index.js';
export { runAnomalyJudge } from './anomalyJudge/run.js';
export { territoryDigest, DIGEST } from './territoryDigest/index.js';
export { runTerritoryDigest } from './territoryDigest/run.js';
export { secondOpinion } from './secondOpinion/index.js';
export { runSecondOpinion } from './secondOpinion/run.js';
export { finalJudge, FINAL } from './finalJudge/index.js';
export { runFinalJudge } from './finalJudge/run.js';
export { fold, FOLD } from './fold/index.js';
export { runFold } from './fold/run.js';
export { claimExtractor, EXTRACT } from './claimExtractor/index.js';
export { runClaimExtractor } from './claimExtractor/run.js';
export { claimVerifier, VERIFY, VERIFY_BATCH } from './claimVerifier/index.js';
export { runClaimVerifier, runClaimVerifierBatch } from './claimVerifier/run.js';
export { consistencyJudge, CONFLICT } from './consistencyJudge/index.js';
export { runConsistencyJudge } from './consistencyJudge/run.js';
export { probe, PROBE } from './probe/index.js';
export { runProbe } from './probe/run.js';
export { brainer, COORD } from './brainer/index.js';
export { runBrainer } from './brainer/run.js';
export { claimAuditor, AUDITS } from './claimAuditor/index.js';
export { runClaimAuditor } from './claimAuditor/run.js';
export { synthesiser, SYNTH } from './synthesiser/index.js';
export { runSynthesiser } from './synthesiser/run.js';
export { invariantHunter, INVARIANT_HUNT } from './invariantHunter/index.js';
export { runInvariantHunter } from './invariantHunter/run.js';
export { coverageCritic, COVERAGE_CRITIC } from './coverageCritic/index.js';
export { runCoverageCritic } from './coverageCritic/run.js';
export { RO } from './shared.js';
