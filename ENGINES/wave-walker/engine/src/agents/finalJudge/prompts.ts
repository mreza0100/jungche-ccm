// finalJudge prompt — byte-identical to the source's inline construction (wave-walker.js lines 673-685). A non-empty charter appends the Professor-authored WALK CHARTER block (zero bytes otherwise). INVARIANT REGISTRY FEATURE (§ 2.4) — a non-empty coverageGaps list appends the critic's named holes to the Coverage line (zero bytes when empty/absent).
import { RO } from '../shared.js';
import type { FinalJudgeArgs } from '../../types/index.js';

export const buildFinalJudge = ({
  walksBrief,
  confirmed,
  unproven,
  killedWithAnomaly,
  digests,
  securityDoc,
  security,
  walksLen,
  threadsLen,
  cardsLen,
  unsensed,
  charter,
  coverageGaps,
  contradictions,
}: FinalJudgeArgs): string =>
  'You are the FINAL JUDGE of this wave walk — one Opus ruling over the WHOLE result before the review is written. Complete inputs: ' +
  'THREAD WALKS: ' +
  JSON.stringify(walksBrief) +
  ' · CONFIRMED anomalies: ' +
  JSON.stringify(confirmed) +
  ' · UNPROVEN: ' +
  JSON.stringify(unproven) +
  ' · KILLED as FALSE (re-examine — a wrong kill hides here): ' +
  JSON.stringify(killedWithAnomaly) +
  ' · Territory digests: ' +
  JSON.stringify(digests) +
  ' · SECURITY AUDIT (diff-scoped ' +
  securityDoc +
  '): ' +
  (security
    ? JSON.stringify(security.findings || []) +
      ' (auditors ' +
      security.auditorsReturned +
      '/' +
      security.auditorsDispatched +
      ' · opened ' +
      security.filesOpened.length +
      '/' +
      security.filesInScope +
      ' files · swept everywhere: ' +
      (security.categoriesSwept || []).join(',') +
      (security.filesUnswept.length
        ? ' · UNSWEPT files — coverage holes, never clean: ' +
          security.filesUnswept.slice(0, 12).join(', ') +
          (security.filesUnswept.length > 12
            ? ' (+' + (security.filesUnswept.length - 12) + ' more)'
            : '')
        : '') +
      ')'
    : 'AUDIT DIED — a coverage hole') +
  ' · Coverage: threads ' +
  walksLen +
  '/' +
  threadsLen +
  ', fields sensed ' +
  cardsLen +
  ', UNSENSED: ' +
  (unsensed.length ? unsensed.join(', ') : 'none') +
  ((coverageGaps || []).length ? ', coverage-critic gaps: ' + JSON.stringify(coverageGaps) : '') +
  '\n' +
  'Rule the wave: (1) the authoritative verdict on the SMOOTH SAILING | MOSTLY GOOD | ROUGH SEAS | SHIPWRECK scale — weigh broken threads, confirmed severity, security findings, and coverage holes; (2) reinstate any killed anomaly whose kill reasoning does not hold (open the files yourself before reinstating); (3) missedRisks — cross-cutting hazards only the whole picture shows (a pattern repeating across threads, an unsensed-field cluster over sensitive surface, digest smells that compound). Judge evidence, not vibes.' +
  RO +
  ' Structured output: verdict, reinstated, missedRisks, rationale.' +
  ((contradictions || []).length
    ? '\nNAMED CONTRADICTIONS (two or more seats walked ONE file and returned opposite verdicts): ' +
      JSON.stringify(contradictions) +
      '\nRule EACH one explicitly in your rationale — open the file yourself first. Name which seat is wrong about the file in front of it and VOID any verdict resting on evidence the file does not contain (a line count, a parity claim, text reported as removed that is still there). A clean verdict never wins for being calmer, and a flagged one never wins for being louder; averaging, merging, or letting the more optimistic verdict stand is forbidden. A contradiction you cannot resolve from the files is a coverage hole — say so, and weigh it in the verdict.'
    : '') +
  (charter
    ? '\nWALK CHARTER (caller-supplied duty): ' +
      charter +
      '\nAnswer the charter explicitly: what the walk found for it and whether its concern is satisfied — inside your existing fields; the verdict scale and schema unchanged.'
    : '');
