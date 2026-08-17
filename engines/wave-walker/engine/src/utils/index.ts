// utils/index.ts — small pure helpers shared by engine.ts. `chunk` matches the source's inline helper
// (wave-walker.js line 603) with one defensive addition: size<=0 degrades to one whole-array chunk
// instead of an infinite loop — the source never calls it with a non-positive size (always the literal
// 6 or 4), so this never changes observed behavior, only guards against future misuse.
import type { DebugRecord } from '../types/index.js';
export function chunk<T>(items: T[], size: number): T[][] {
  if (!items.length) return [];
  if (size <= 0) return [items];
  const out: T[][] = [];
  for (let i = 0; i < items.length; i += size) out.push(items.slice(i, i + size));
  return out;
}

// globMatch — a minimal, dependency-free glob matcher for INVARIANT REGISTRY territory patterns
// (tmp/wave-walker-investigation.md section 2.1). No npm glob/minimatch package is vendored (build.js's
// resolveSpec rejects any bare-specifier import - the Workflow sandbox has no module loader), so this is
// the zero-token fail-safe's own primitive, in the same spirit as isGateRelevant's hand-rolled regex
// tests (engine.ts). Supports `*` (any run of non-slash chars) and `**` (any run of chars, incl. `/`) -
// no brace expansion; a territory needing alternatives lists each as its own glob string instead.
// Implemented by walking the pattern char-by-char (never a placeholder-and-replace pass — no marker
// string could be proven absent from an arbitrary caller-supplied glob) and building the regex source
// directly, so a literal '*' inside the pattern can never be misread as part of a longer '**' run twice.
export function globMatch(pattern: string, filePath: string): boolean {
  let out = '';
  for (let i = 0; i < pattern.length; i++) {
    const c = pattern[i];
    if (c === '*') {
      if (pattern[i + 1] === '*') {
        out += '.*';
        i++;
      } else {
        out += '[^/]*';
      }
    } else if ('.+^${}()|[]\\'.includes(c)) {
      out += '\\' + c;
    } else {
      out += c;
    }
  }
  return new RegExp('^' + out + '$').test(filePath);
}

// WALK TELEMETRY (DEBUG STEP) — mechanically renders a DebugRecord into the `### Walk Telemetry`
// markdown block fold copies VERBATIM into the review (tmp/walker-debug-design.md §4/§6). Plain data
// in, plain string out, zero engine coupling — fully unit-testable without stubbing `agent()`, mirroring
// rr's compactCheckpoint discipline. COUNTS ONLY, bounded regardless of wave size (design §9: target
// ≤40 lines / ~1.5KB) — never the raw prompt/output capture rr's `_debug.md` carries.
//
// Self-floored: wraps its own body in a try/catch and NEVER throws — a malformed/partial record (a
// future field shape change, a caller passing something not fully DebugRecord-shaped) falls back to one
// honest line rather than crashing the walk or the caller's own try/catch (tmp/walker-debug-design.md
// §5 row 2). This lets the function be called directly (engine.ts, or a unit test) with no wrapping
// required at every call site.
export function renderTelemetryMd(rec: DebugRecord): string {
  try {
    return renderTelemetryMdInner(rec);
  } catch (e) {
    return (
      '### Walk Telemetry\n\n_(telemetry render failed: ' +
      ((e as Error) && (e as Error).message ? (e as Error).message : String(e)) +
      ' — see debugRecord in the workflow result for raw data)_'
    );
  }
}

function renderTelemetryMdInner(rec: DebugRecord): string {
  const lines: string[] = ['### Walk Telemetry', ''];
  if (rec.degraded)
    lines.push(
      '**DEGRADED** — ' + (rec.gaps.length ? rec.gaps.join('; ') : 'a section failed to assemble'),
      '',
    );

  lines.push('**Seats:**');
  const seatKeys = Object.keys(rec.seats).sort();
  if (!seatKeys.length) lines.push('- (none recorded)');
  for (const k of seatKeys) {
    const t = rec.seats[k];
    const deaths = t.diedFirstAttempt || t.retried || t.diedAfterRetry;
    lines.push(
      '- ' +
        k +
        ': ' +
        t.calls +
        ' call(s)' +
        (deaths
          ? ' · died-first ' +
            t.diedFirstAttempt +
            ' · retried ' +
            t.retried +
            ' · died-after-retry ' +
            t.diedAfterRetry
          : ''),
    );
  }
  if (rec.seatsExpectedButAbsent.length)
    lines.push(
      '- **MISSING (expected, never dispatched):** ' + rec.seatsExpectedButAbsent.join(', '),
    );

  lines.push(
    '',
    '**Invariant registry:** ' +
      rec.armedInvariants.registered +
      ' registered, ' +
      rec.armedInvariants.armed.length +
      ' armed' +
      (rec.armedInvariants.armed.length
        ? ' (' + rec.armedInvariants.armed.map((a) => a.id).join(', ') + ')'
        : ''),
  );
  if (rec.armedInvariants.unarmed.length)
    lines.push('- unarmed: ' + rec.armedInvariants.unarmed.map((a) => a.id).join(', '));

  const er = rec.emptyResults;
  lines.push(
    '',
    '**Coverage:** threads ' +
      er.threadsWalked +
      '/' +
      er.threadsExpected +
      ' · sensors ' +
      er.sensorsWithCards +
      '/' +
      er.sensorsExpected +
      ' · hunters ' +
      er.huntersReturned +
      '/' +
      er.huntersExpected +
      ' (' +
      er.hunterFindingsTotal +
      ' finding(s)) · digests ' +
      er.digestsWithFindings +
      '/' +
      er.digestsExpected +
      (er.securityDied ? ' · security: DIED' : '') +
      (er.coverageCriticDied ? ' · coverage-critic: DIED' : '') +
      (er.foldDied ? ' · fold: DIED' : ''),
  );
  if (rec.coverage.unsensedFields.length)
    lines.push('- unsensed fields: ' + rec.coverage.unsensedFields.join(', '));
  if (rec.coverage.gateSweepSkipped)
    lines.push('- gate sweep: ' + (rec.coverage.gateSweepSkipDetail || 'SKIPPED'));
  if (rec.coverage.coverageGaps.length)
    lines.push(
      '- coverage-critic gaps: ' +
        rec.coverage.coverageGaps.map((g) => g.territory + ' (' + g.why + ')').join('; '),
    );
  // VERDICT CONTRADICTIONS — printed on EVERY walk, zero included: the line states what was compared and
  // what could not be, so "0" reads as a scan result, never as "nobody looked" (walker.md § Orchestration).
  const scan = rec.coverage.contradictionScan || { filesCompared: 0, uncomparableThreads: [] };
  const contras = rec.coverage.verdictContradictions || [];
  lines.push(
    '- verdict contradictions: ' +
      contras.length +
      ' over ' +
      scan.filesCompared +
      ' file(s) walked by 2+ seats' +
      (scan.uncomparableThreads.length
        ? ' · uncomparable (no files in thread spec): ' + scan.uncomparableThreads.join(', ')
        : ''),
  );
  for (const c of contras)
    lines.push(
      '  - ' +
        c.file +
        ': ' +
        c.clean.join('/') +
        ' INTACT vs ' +
        c.flagged.join('/') +
        ' — escalated to the final judge',
    );

  const js = rec.judgeStats;
  lines.push(
    '',
    '**Judgment:** confirmed ' +
      js.confirmed +
      ' · false ' +
      js.falseCount +
      ' · unproven ' +
      js.unproven +
      ' · 2nd-opinion: ' +
      js.secondOpinionDispatched +
      ' dispatched, ' +
      js.secondOpinionOverturned +
      ' overturned, ' +
      js.secondOpinionReexaminedKilled +
      ' re-examined-killed · final judge: ' +
      (js.finalJudgeDied
        ? 'DIED'
        : (js.finalJudgeVerdict || '?') + ', reinstated ' + js.finalJudgeReinstated),
  );

  lines.push(
    '',
    '_tokenAttribution: not available at the engine layer (agent() returns no usage metadata) — see the p:tokens skill for post-hoc spend analysis._',
  );
  return lines.join('\n');
}
