// fold prompt — byte-identical to the source's inline construction (wave-walker.js lines 700-712). INVARIANT
// REGISTRY FEATURE (§ 2.4) — a non-empty coverageGaps list extends the honesty sentence (zero bytes when
// empty/absent).
import type { FoldArgs } from '../../types/index.js';

export const buildFold = ({
  reportPath,
  walks,
  confirmed,
  unproven,
  killedCount,
  digests,
  security,
  coverageSummary,
  finalJudge,
  coverageGaps,
  telemetryMd,
  contradictions,
}: FoldArgs): string =>
  'You are the FOLD of a wave-walker review. Merge the two walks into ONE review and WRITE it into the report at ' +
  reportPath +
  " under a `## Professor's Wave Review` section (create/overwrite ONLY that section of that file; run no git).\n" +
  'Inputs:\n· THREAD WALKS (functional flow + hygiene, the floor): ' +
  JSON.stringify(walks) +
  '\n' +
  '· LEDGER anomalies CONFIRMED (mechanical, file-verified by judges): ' +
  JSON.stringify(confirmed) +
  '\n· Ledger UNPROVEN (needs human eyes): ' +
  JSON.stringify(unproven) +
  '\n· Ledger killed-as-false: ' +
  killedCount +
  ' (one line)\n' +
  '· Territory digests: ' +
  JSON.stringify(digests) +
  '\n· SECURITY AUDIT (diff-scoped): ' +
  (security
    ? JSON.stringify({
        findings: security.findings || [],
        categoriesSwept: security.categoriesSwept,
        summary: security.summary,
        auditors: security.auditorsReturned + '/' + security.auditorsDispatched,
        filesOpened: security.filesOpened.length + '/' + security.filesInScope,
        filesUnswept: security.filesUnswept
          .slice(0, 12)
          .concat(
            security.filesUnswept.length > 12
              ? ['+' + (security.filesUnswept.length - 12) + ' more']
              : [],
          ),
      })
    : 'AUDIT DIED — name it in Coverage as an explicit hole') +
  '\n· Coverage: ' +
  coverageSummary +
  '\n' +
  (finalJudge
    ? '· FINAL JUDGMENT (authoritative): verdict=' +
      finalJudge.verdict +
      ' · missedRisks: ' +
      JSON.stringify(finalJudge.missedRisks) +
      ' · rationale: ' +
      (finalJudge.rationale || '') +
      '\n'
    : '') +
  'Fold rules: every functional defect (thread) AND every confirmed ledger anomaly AND every digest fix AND every security finding becomes a `### /jc Action Items` line (deduped — a thread defect and a ledger anomaly at the same anchor are ONE item). ' +
  (finalJudge
    ? 'ADOPT the FINAL JUDGMENT verdict verbatim; fold each missedRisk into the review (fixable → an action item, else Unproven/needs-eyes). '
    : '') +
  'The verdict weighs BOTH: a broken thread flow OR a confirmed critical/high ledger anomaly sinks it. HONESTY: the Coverage note MUST name every UNSENSED field as a hole' +
  (security && security.filesUnswept.length
    ? ', AND the security files-opened denominator with every UNSWEPT file'
    : '') +
  ((coverageGaps || []).length
    ? ', AND every coverage-critic gap (' + JSON.stringify(coverageGaps) + ') as a named hole'
    : '') +
  '.\n' +
  ((contradictions || []).length
    ? '· NAMED CONTRADICTIONS (seats disagreeing over ONE file): ' +
      JSON.stringify(contradictions) +
      "\nName each one in Coverage with the final judgment's ruling on it, and mark a voided thread verdict as VOID in the Thread Walk table — never silently keep the cleaner verdict.\n"
    : '') +
  (telemetryMd
    ? '\nAlso append this VERBATIM as a `### Walk Telemetry` subsection at the end of the review, after Coverage — do not summarize, edit, or judge it, just copy it in:\n' +
      telemetryMd +
      '\n'
    : '') +
  "Report format (per wave/walker.md § Report Format): ## Professor's Wave Review (Wave · Date · Verdict); Executive Summary; Thread Walk table; Ledger Anomalies by rule (Expected/Got + anchors + severity); Territory Digests; Security Audit (per-category Expected/Got, or None); ### /jc Action Items; Coverage.\n" +
  'Verdict: SMOOTH SAILING (nothing) | MOSTLY GOOD (minor only) | ROUGH SEAS (a confirmed high or a BROKEN thread) | SHIPWRECK (a confirmed critical / security, or multiple broken flows).' +
  ' Structured output: verdict, actionItems (verbatim /jc lines), review (the full markdown you wrote).';
