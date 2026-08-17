// source-prompts.ts — PROMPT PARITY fixtures: one expected*(args) builder per seat, each transcribing the
// ported src/agents/*/prompts.ts construction independently, for byte-identical assertion against it.
// Any intentional difference is a bug; there should be zero. DEADNESS_BAR below matches the GENERIC
// default src/constants.ts renders when CONFIG.PROJECT is absent — test/setup.ts's globalThis.args carries
// no `project`, so every prompts.test.ts import sees the floor default (universal-bundle refactor).
/* eslint-disable */
const RO =
  ' Read-only: Grep/Glob, Read, and git log/show/diff/rev-parse only — run no other code, write no files, mutate no git.';
const ENC_VOCAB =
  'encoding vocabulary (EXACTLY one): raw | object | json-string | enum-string | number | boolean | unknown';
const DEC_VOCAB =
  'decode vocabulary (EXACTLY one per consumer): direct | render | json-parse | object-index | compare | spread | unknown';
const DEADNESS_BAR =
  'DEADNESS BAR (for any dead/unread/orphan verdict): prove it ALIVE first — a false dead verdict on live code is a production regression. ' +
  'Dead only with zero PRODUCTION consumers AND the surfaces a static grep misses: dynamic/by-name dispatch (registries, plugin loaders), serialized payload fields (queues, APIs), and generated-vs-hand-written type drift, ' +
  'and test/config/reflection consumers. Cannot prove past the bar -> NOT dead: verdict UNPROVEN, keep the code.';
const CATCHBOOK =
  'Catch-book categories (tag findings): DUP (reinvented helper/type/hook; copy-pasted consumer pattern), DEAD (unreachable/orphaned, subject to the deadness bar), ' +
  'GHOST (dual-writes, manual sync, schema<->code field mismatch), SMELL (cross-boundary writes, shallow error handling, N+1, wrong layer, over-engineering), ' +
  'TYPE-GAP (unguarded casts, hand-typed drift, loose String where enum belongs), NAMING (concept drift, scope-dishonest names), ' +
  'QUALITY (magic literals, hardcoded i18n, fetch-in-leaf-component), STALE-DEP (unused/phantom imports).';

// ── scout (source lines 400-415) ── D1 FILE-SET RECONCILIATION (docs/dev/audits/
// wave-walker-instrument-defects-2026-07-28.md) is a deliberate divergence from the source bundle: the
// changedFileCount duty in both mode strings, changedFileCount in the output list, and the optional
// RECONCILIATION RETRY block — pinned here like the threadWalker's always-on addition below.
export const expectedScout = ({
  reportPath,
  branch,
  walkerDoc,
  maxFieldsPerJob,
  reconcile,
  repoRoot,
  authDoc,
  gateResolverPattern,
}: {
  reportPath: string;
  branch: string | null;
  walkerDoc: string;
  maxFieldsPerJob: number;
  reconcile?: { enumerated: number; counted: number | undefined };
  repoRoot: string;
  authDoc?: string;
  gateResolverPattern?: string;
}): string =>
  'You are the SCOUT-SCHEDULER of a wave-walker review. Read the wave report at ' +
  reportPath +
  " and walk the WAVE'S DIFF. Repo root: " +
  repoRoot +
  '.\n' +
  (branch
    ? '1) PRE-MERGE BRANCH MODE: the wave is NOT merged yet. changedFiles = `git diff --name-only main...' +
      branch +
      "` (three-dot; read file contents from the branch's worktree checkout when present, else `git show " +
      branch +
      ':{path}`). mergeShas = []. headSha = `git rev-parse ' +
      branch +
      '`. The report carries the wave manifest + slice list for context. changedFileCount = run `git diff --name-only main...' +
      branch +
      ' | wc -l` as a SEPARATE command and copy its printed integer EXACTLY — never the length of your enumerated list. The engine FAILS the walk when list and count disagree: enumerate EVERY file the diff prints, no salience filtering, no truncation.\n'
    : '1) From the report — a `**Merge SHA:**` line (the dual-chat wave writes one at MERGE) and/or the Final Summary / Grouping / `## JC Pre-flight` sections: list SUCCEEDED pipeline merge SHAs (mergeShas) and any JC commits. Run `git diff {merge}^1 {merge}` per merge SHA (`git show {sha}` for a JC fix) and union into changedFiles (the integrated changed-and-generated set). headSha = git rev-parse HEAD. changedFileCount = re-run the same name-only diffs (`git diff --name-only {merge}^1 {merge}` per merge, `git show --name-only --format= {sha}` per JC commit) in ONE piped command through `sort -u | wc -l` and copy the printed integer EXACTLY — never the length of your enumerated list. The engine FAILS the walk when list and count disagree: enumerate EVERY file, no salience filtering, no truncation.\n') +
  '2) THREADS — the functional/hygiene walk manifest (the proven floor). Read ' +
  walkerDoc +
  ' § Role: Scout for the thread taxonomy; aim for >= 4, one per feature flow plus a thread for each seam, field, schema change, invariant, test-data-discipline, or dead-code-ripple the diff puts at risk. Emit a Field thread with an explicit READ-BACK check for EVERY new persisted field (writer AND reader mapping). Each: id, type, name, scope, files, verify.\n' +
  '3) LEDGER SCHEDULE (the mechanical spine, only if the diff touches the GraphQL contract surface — else return empty fields/jobs and the thread walk carries the wave):\n' +
  '   · operations — GraphQL operations whose resolver/SDL the diff changed OR whose result type the diff touches: id, kind, resolver anchor, resultType.\n' +
  '   · fields — every field of each touched result type, DEDUPED by (ownerType, field); id="OwnerType.fieldName"; fill each field\'s sdl slice {anchor, typeToken} YOURSELF from the schema. Include a field when the diff changed its producer, its SDL, or any consumer.\n' +
  '   · jobs — cluster fields by FILE LOCALITY into sensor jobs (kind producer|consumer|ai), each with the EXACT files to read and <= ' +
  maxFieldsPerJob +
  " fieldIds; follow resolver imports / grep the query call-sites NOW so each job's file list is exact.\n" +
  '4) gateFiles — ' +
  (gateResolverPattern
    ? 'EVERY file matching the pattern ' +
      gateResolverPattern +
      ' (repo-wide; fence-outlier detection needs the full population even when the diff is small).\n'
    : "this project has no configured gate-resolver surface (args.project.gateResolverPattern) — return [].\n") +
  '5) territories — which of BE/FE/AI the diff touches.\n' +
  '6) authRule — ' +
  (authDoc
    ? 'grep ' +
      (authDoc.includes(' § ') ? authDoc.split(' § ')[0] : authDoc) +
      ' for its "' +
      (authDoc.includes(' § ') ? authDoc.split(' § ').slice(1).join(' § ') : 'Auth Pattern') +
      '" heading (locate by heading text, NEVER by line number) and return the relevant fence-rule bullet VERBATIM — the ledger\'s R6 auth-fence rule and the security second-opinion quote it live.'
    : 'this project has no configured auth doc (args.project.authDoc) — return authRule: "".') +
  RO +
  ' Structured output: headSha, territories, changedFiles, changedFileCount, mergeShas, threads, operations, fields, jobs, gateFiles, authRule.' +
  (reconcile
    ? '\nRECONCILIATION RETRY: your previous pass enumerated ' +
      reconcile.enumerated +
      ' changedFiles but its separately-executed git count said ' +
      reconcile.counted +
      '. The enumerated list scopes EVERY downstream lens — security above all — so a file missing from it is invisible to the whole walk. Re-run the git command(s), enumerate EVERY file they print (no filtering, no truncation), re-run the separate count, and return both; they must match.'
    : '');

// ── threadWalker (source lines 443-446) ── WAVE-WALKER INVARIANT-REGISTRY FEATURE
// (tmp/wave-walker-investigation.md § 2.5) carries an ALWAYS-ON addition here — NOT gated by the
// registry, unlike every other piece of that feature; the design names this a deliberate, permanent
// improvement to the proven floor: the reorientation sentences (set-enumeration diffs + the
// broken-mechanism test) and the failureScenario field on defects. The 2026-07-28 audit adds two more
// deliberate divergences: D3 hands the walker the REAL hygiene standard (walker.md § Role: Walker step 4
// cited audit/code-hygiene.md from day one, but no seat was ever handed it), and D4 adds the
// producer-verification clause (four seats certified a shape-fix against a test's fabricated envelope).
export const expectedThreadWalker = ({
  walkerDoc,
  hygieneDoc,
  thread,
}: {
  walkerDoc: string;
  hygieneDoc: string;
  thread: unknown;
}): string =>
  'Read ' +
  walkerDoc +
  ' § Role: Walker. Walk this ONE thread end-to-end in a single pass over its files, returning BOTH the functional verdict AND the integration-delta code-hygiene findings. ' +
  'Per-pipeline hygiene already ran pre-merge (wave/builder.md Step 7) — your wave-level value is the INTEGRATION delta: read ' +
  hygieneDoc +
  " and apply it scope-diff to this thread's files — above all a repo-wide reuse-grep for a helper/type/hook a SIBLING pipeline duplicated, plus dead code the integration orphaned. " +
  'At every step, also name the concrete input/state under which this step corrupts, aborts, or lies — a failure scenario, not a vibe. Any two set-enumerations the flow assumes equal (a wipe set vs its snapshot set, a terminal-status set vs a poll loop\'s terminal set, a required-env list vs a validator\'s list) are diffed member-by-member. Apply the broken-mechanism test: what does this step report when it FAILS — the same as "nothing to do"? Flag it. ' +
  "A step claiming to HANDLE a shape it receives (a response envelope, an error body, a message payload) is verified against the code that EMITS that shape — open the producer and quote it; a test's fabricated envelope is never evidence the two sides agree. " +
  'Thread: ' +
  JSON.stringify(thread) +
  '.' +
  RO +
  ' Structured output: threadId, name, type, flow (INTACT|AT-RISK|BROKEN|N/A), trace (step → step, marking any break), defects (each {what, location=file:line, failureScenario, jc=`/jc {fix}`}), hygiene (each {kind, where=file:line, detail, jc}), notes.';

// ── sliceSensor (source lines 455-467) ──
export const expectedSliceSensor = ({
  jobId,
  kind,
  files,
  hint,
  assigned,
}: {
  jobId: string;
  kind: 'producer' | 'consumer' | 'ai';
  files: string[];
  hint?: string;
  assigned: unknown[];
}): string =>
  'You are a PURE EXTRACTOR (scheduled sensor). NO judgment, NO bug-finding — extract and return, nothing else. ' +
  'Read ONLY these files (Grep to confirm an anchor is fine): ' +
  JSON.stringify(files) +
  '. Hint: ' +
  (hint || 'none') +
  '.\n' +
  'For EACH assigned field, extract its ' +
  kind.toUpperCase() +
  ' slice:\n' +
  (kind === 'producer'
    ? '· producer: where the value is mapped onto the result object — anchor, writer, typeToken, encoding (' +
      ENC_VOCAB +
      '), valueLiterals (EXACT, case-preserved).\n· dbColumn (if from a column): anchor, columnName, columnType, checkLiterals.\n· resolver (if a dedicated field/type resolver exists): anchor.\n'
    : kind === 'consumer'
      ? '· feSelection: where the query selects it — anchor, queryName (omit if never selected).\n· feTypes: generated type AND any hand-written interfaces — anchor, typeToken, kind (generated|hand).\n· consumers: EVERY read to the leaf render — anchor, name, decode (' +
        DEC_VOCAB +
        '), decodeExpr (VERBATIM, <=80 chars), context (production|test|generated|story), comparedLiterals (EXACT, case-preserved), aliasChain.\n· PARSE SITES ARE CONSUMERS: a screen that parses/transforms before drilling down (JSON.parse, mapping, memo) is its own consumer — its verbatim expression is the decodeExpr; a JSON.parse(JSON.stringify(x)) roundtrip MUST appear verbatim, never summarized; record each screen\'s parse separately.\n· undeclaredReads: any property read off the same result object NOT in your assigned field list (side:"fe"), INCLUDING reads in fallback chains (a ?? b, a || b, ternaries) and optional-chained access; plus any field the resolver returns beyond the declared set if a resolver file is listed (side:"be", expand spreads).\n'
      : '· producer (AI/engine writer): where this project\'s AI/compute layer computes/writes this value — anchor, writer:"ai", encoding, valueLiterals (EXACT). Grep the snake_case form if applicable.\n') +
  'A field with nothing to extract here gets a slice with just its fieldId. Every anchor grep-verified file:line. Keep strings SHORT (<=80 chars).\n' +
  'Assigned fields: ' +
  JSON.stringify(assigned) +
  RO +
  ' Structured output: jobId=' +
  jobId +
  ', slices, undeclaredReads.';

// ── gateSweep (source lines 471-474) — resource-class enum and the org-fence label now read off
// args.project (universal-bundle refactor) instead of one project's hardcoded vocabulary. ──
export const expectedGateSweep = ({
  file,
  resourceClasses,
  fenceLabels,
}: {
  file: string;
  resourceClasses: string[];
  fenceLabels: { org: string; ownership: string };
}): string =>
  'You are a PURE EXTRACTOR (gate sweep). NO judgment. Open the resolver file ' +
  file +
  ' and extract ONE gate card per GraphQL entry point in it.\n' +
  'Per entry point: id ("Query.opName"/"Mutation.opName"), kind, resource class (' +
  resourceClasses.join('|') +
  '), anchor, idArgs (client-supplied ID args), ' +
  'chain (IN ORDER, every guard call between entry and first data access; open custom helpers and note what they fence), rolesAllowed (EXPAND role-set constants), ' +
  'orgFence (boolean: ' +
  fenceLabels.org +
  ' access enforced), ownershipFence (boolean: record-owner check enforced). Keep strings SHORT.' +
  RO +
  ' Structured output: file, gates.';

// ── securityAuditor (source line 488) ── D2 SECURITY FAN-OUT (audit 2026-07-28), a deliberate
// divergence replacing the single whole-diff auditor: one auditor per changed-file slice, full set as
// cross-file context, filesOpened/filesSkipped honesty fields, and the D4 producer-verification clause.
// PROJECT PROFILE (universal-bundle refactor) — the category emphasis is a generic constant (the 8A-8K
// taxonomy's own deepest-pass categories, no domain claim); securityStakesLine (args.project) supplies
// the leading sacred-data framing sentence when present, absent otherwise.
const SECURITY_CATEGORY_EMPHASIS =
  'Sensitive data (8F), auth (8C), API surface (8D), LLM/prompt (8E) get the deepest pass.';
export const expectedSecurityAuditor = ({
  securityDoc,
  clusterFiles,
  allChangedFiles,
  clusterIndex,
  clusterCount,
  branch,
  mergeShas,
  securityStakesLine,
}: {
  securityDoc: string;
  clusterFiles: string[];
  allChangedFiles: string[];
  clusterIndex: number;
  clusterCount: number;
  branch: string | null;
  mergeShas: string[];
  securityStakesLine?: string;
}): string =>
  'You are a WAVE SECURITY AUDITOR (slice ' +
  (clusterIndex + 1) +
  '/' +
  clusterCount +
  '). Read ' +
  securityDoc +
  " and apply its FULL category set (8A–8K + Method & Severity) to YOUR SLICE of this wave's diff. Your assigned files — OPEN AND SWEEP EVERY ONE; a file you did not open goes in filesSkipped with why, never silently: " +
  JSON.stringify(clusterFiles) +
  ". The wave's full changed set (context — follow a changed symbol from your slice into its callers/config/emitters wherever the risk crosses a file boundary, inside or outside the slice): " +
  JSON.stringify(allChangedFiles) +
  '. ' +
  (branch
    ? 'Diff: main...' + branch + '.'
    : 'Merge SHAs: ' + JSON.stringify(mergeShas || []) + '.') +
  ' ' +
  (securityStakesLine ? securityStakesLine + ' — ' : '') +
  SECURITY_CATEGORY_EMPHASIS +
  " A guard or fix claiming to handle a response/error/message SHAPE is verified against the code that EMITS that shape — open the producer; a test's fabricated envelope is never evidence. Report ONLY defects the diff introduced or worsened; a pre-existing issue you trip over goes into summary as one line (category + location), never a finding. filesOpened lists every file you ACTUALLY read; categoriesSwept names every category you ACTUALLY swept — honesty over completeness." +
  RO +
  ' Structured output: findings (Expected/Got), categoriesSwept, filesOpened, filesSkipped, summary.';

// ── anomalyJudge (source lines 612-617) ── D4 (audit 2026-07-28), a deliberate divergence: the
// producer-verification clause — judges opened "both ends" of the cited anchors but nobody opened the
// PRODUCER behind a claimed shape-fix, so a fabricated test envelope passed as evidence four times.
export const expectedAnomalyJudge = ({
  rule,
  ruleMeaning,
  sec,
  instances,
  ctxCards,
  authDoc,
}: {
  rule: string;
  ruleMeaning: string;
  sec: boolean;
  instances: unknown[];
  ctxCards: unknown[];
  authDoc?: string;
}): string =>
  'You are an anomaly JUDGE. Rule ' +
  rule +
  ': ' +
  ruleMeaning +
  '\n' +
  "For EACH instance: open the file(s) at the cited anchors (BOTH ends where two are given), confirm the facts, and rule CONFIRMED (severity, one-sentence what, location=file:line, fix=`/jc {fix}`), FALSE (say why), or UNPROVEN (say what is missing). When a claimed fix or consumer handles a produced SHAPE (a response envelope, an error body, a message payload), also open the PRODUCER that actually emits it — middleware, service, emitter — even when it sits outside the cited anchors; a test's fabricated envelope is never evidence the shapes agree. Judge evidence, not vibes.\n" +
  (sec
    ? 'SECURITY: this rule enforces a WRITTEN project invariant. "Every sibling does it the same way" is NOT a defense — a documented-rule violation is CONFIRMED even when it is the file-wide pattern.' +
      (authDoc ? ' Read ' + authDoc + ' before any FALSE.\n' : '\n')
    : '') +
  DEADNESS_BAR +
  '\nInstances: ' +
  JSON.stringify(instances) +
  '\n' +
  (ctxCards.length
    ? 'Extracted cards for context (verify against real files): ' + JSON.stringify(ctxCards) + '\n'
    : '') +
  RO +
  ' Structured output: verdicts (one per instance, anomalyId matching).';

// ── territoryDigest (source lines 632-635) ──
export const expectedTerritoryDigest = ({
  territory,
  slice,
}: {
  territory: string;
  slice: unknown[];
}): string =>
  'You are the ' +
  territory +
  " TERRITORY DIGEST for this wave. You receive this territory's side of every extracted card. " +
  'Mechanical rules AND the thread walk already handled connectivity/contract/gate/flow — do NOT re-report those. Your job is what neither can see: duplication across fields, wrong-layer logic, over-engineering, naming drift, magic literals, shallow error handling, hardcoded i18n. Open files as needed.\n' +
  CATCHBOOK +
  '\nCards: ' +
  JSON.stringify(slice) +
  RO +
  ' Structured output: territory, findings (each {lens, severity, what, location=file:line, fix}), summary (<=3 sentences).';

// ── secondOpinion (source lines 650-652) ──
export const expectedSecondOpinion = ({
  authRule,
  items,
}: {
  authRule: string;
  items: unknown[];
}): string =>
  "You are the SECOND-OPINION judge (a first judge killed these as FALSE, but the rule's evidence is regex/string-exact or a documented security invariant). " +
  authRule +
  '\n' +
  'For each: open the file(s) yourself, re-derive from scratch, rule independently. Be suspicious of a kill that contradicts the verbatim extracted expression (a JSON.parse(JSON.stringify(...)) still present, a literal that truly never matches the produced set).\n' +
  'Killed verdicts with anomalies: ' +
  JSON.stringify(items) +
  RO +
  ' Structured output: verdicts (one per anomalyId).';

// ── finalJudge (source lines 674-682) ── D2 (audit 2026-07-28), a deliberate divergence: the security
// line now carries the sweep's own denominator (auditors returned/dispatched, files opened/in-scope,
// UNSWEPT files as named coverage holes) — the old render let "0 findings (swept: 8A..8K)" read as a
// complete sweep when the auditor had opened 45 of 148 files.
export const expectedFinalJudge = (a: {
  walksBrief: unknown[];
  confirmed: unknown[];
  unproven: unknown[];
  killedWithAnomaly: unknown[];
  digests: unknown[];
  securityDoc: string;
  security: {
    findings?: unknown[];
    categoriesSwept?: string[];
    auditorsReturned: number;
    auditorsDispatched: number;
    filesOpened: string[];
    filesInScope: number;
    filesUnswept: string[];
  } | null;
  walksLen: number;
  threadsLen: number;
  cardsLen: number;
  unsensed: string[];
}): string =>
  'You are the FINAL JUDGE of this wave walk — one Opus ruling over the WHOLE result before the review is written. Complete inputs: ' +
  'THREAD WALKS: ' +
  JSON.stringify(a.walksBrief) +
  ' · CONFIRMED anomalies: ' +
  JSON.stringify(a.confirmed) +
  ' · UNPROVEN: ' +
  JSON.stringify(a.unproven) +
  ' · KILLED as FALSE (re-examine — a wrong kill hides here): ' +
  JSON.stringify(a.killedWithAnomaly) +
  ' · Territory digests: ' +
  JSON.stringify(a.digests) +
  ' · SECURITY AUDIT (diff-scoped ' +
  a.securityDoc +
  '): ' +
  (a.security
    ? JSON.stringify(a.security.findings || []) +
      ' (auditors ' +
      a.security.auditorsReturned +
      '/' +
      a.security.auditorsDispatched +
      ' · opened ' +
      a.security.filesOpened.length +
      '/' +
      a.security.filesInScope +
      ' files · swept everywhere: ' +
      (a.security.categoriesSwept || []).join(',') +
      (a.security.filesUnswept.length
        ? ' · UNSWEPT files — coverage holes, never clean: ' +
          a.security.filesUnswept.slice(0, 12).join(', ') +
          (a.security.filesUnswept.length > 12
            ? ' (+' + (a.security.filesUnswept.length - 12) + ' more)'
            : '')
        : '') +
      ')'
    : 'AUDIT DIED — a coverage hole') +
  ' · Coverage: threads ' +
  a.walksLen +
  '/' +
  a.threadsLen +
  ', fields sensed ' +
  a.cardsLen +
  ', UNSENSED: ' +
  (a.unsensed.length ? a.unsensed.join(', ') : 'none') +
  '\n' +
  'Rule the wave: (1) the authoritative verdict on the SMOOTH SAILING | MOSTLY GOOD | ROUGH SEAS | SHIPWRECK scale — weigh broken threads, confirmed severity, security findings, and coverage holes; (2) reinstate any killed anomaly whose kill reasoning does not hold (open the files yourself before reinstating); (3) missedRisks — cross-cutting hazards only the whole picture shows (a pattern repeating across threads, an unsensed-field cluster over sensitive surface, digest smells that compound). Judge evidence, not vibes.' +
  RO +
  ' Structured output: verdict, reinstated, missedRisks, rationale.';

// ── fold (source lines 701-711) ── D2 (audit 2026-07-28), a deliberate divergence: the security input
// object now carries auditors/filesOpened/filesUnswept, and the HONESTY sentence names the security
// files-opened denominator whenever files went unswept — the review can never render a partial sweep as
// a clean one.
export const expectedFold = (a: {
  reportPath: string;
  walks: unknown[];
  confirmed: unknown[];
  unproven: unknown[];
  killedCount: number;
  digests: unknown[];
  security: {
    findings?: unknown[];
    categoriesSwept?: unknown;
    summary?: unknown;
    auditorsReturned: number;
    auditorsDispatched: number;
    filesOpened: string[];
    filesInScope: number;
    filesUnswept: string[];
  } | null;
  coverageSummary: string;
  finalJudge: { verdict: string; missedRisks: unknown[]; rationale?: string } | null;
}): string =>
  'You are the FOLD of a wave-walker review. Merge the two walks into ONE review and WRITE it into the report at ' +
  a.reportPath +
  " under a `## Professor's Wave Review` section (create/overwrite ONLY that section of that file; run no git).\n" +
  'Inputs:\n· THREAD WALKS (functional flow + hygiene, the floor): ' +
  JSON.stringify(a.walks) +
  '\n' +
  '· LEDGER anomalies CONFIRMED (mechanical, file-verified by judges): ' +
  JSON.stringify(a.confirmed) +
  '\n· Ledger UNPROVEN (needs human eyes): ' +
  JSON.stringify(a.unproven) +
  '\n· Ledger killed-as-false: ' +
  a.killedCount +
  ' (one line)\n' +
  '· Territory digests: ' +
  JSON.stringify(a.digests) +
  '\n· SECURITY AUDIT (diff-scoped): ' +
  (a.security
    ? JSON.stringify({
        findings: a.security.findings || [],
        categoriesSwept: a.security.categoriesSwept,
        summary: a.security.summary,
        auditors: a.security.auditorsReturned + '/' + a.security.auditorsDispatched,
        filesOpened: a.security.filesOpened.length + '/' + a.security.filesInScope,
        filesUnswept: a.security.filesUnswept
          .slice(0, 12)
          .concat(
            a.security.filesUnswept.length > 12
              ? ['+' + (a.security.filesUnswept.length - 12) + ' more']
              : [],
          ),
      })
    : 'AUDIT DIED — name it in Coverage as an explicit hole') +
  '\n· Coverage: ' +
  a.coverageSummary +
  '\n' +
  (a.finalJudge
    ? '· FINAL JUDGMENT (authoritative): verdict=' +
      a.finalJudge.verdict +
      ' · missedRisks: ' +
      JSON.stringify(a.finalJudge.missedRisks) +
      ' · rationale: ' +
      (a.finalJudge.rationale || '') +
      '\n'
    : '') +
  'Fold rules: every functional defect (thread) AND every confirmed ledger anomaly AND every digest fix AND every security finding becomes a `### /jc Action Items` line (deduped — a thread defect and a ledger anomaly at the same anchor are ONE item). ' +
  (a.finalJudge
    ? 'ADOPT the FINAL JUDGMENT verdict verbatim; fold each missedRisk into the review (fixable → an action item, else Unproven/needs-eyes). '
    : '') +
  'The verdict weighs BOTH: a broken thread flow OR a confirmed critical/high ledger anomaly sinks it. HONESTY: the Coverage note MUST name every UNSENSED field as a hole' +
  (a.security && a.security.filesUnswept.length
    ? ', AND the security files-opened denominator with every UNSWEPT file'
    : '') +
  '.\n' +
  "Report format (per wave/walker.md § Report Format): ## Professor's Wave Review (Wave · Date · Verdict); Executive Summary; Thread Walk table; Ledger Anomalies by rule (Expected/Got + anchors + severity); Territory Digests; Security Audit (per-category Expected/Got, or None); ### /jc Action Items; Coverage.\n" +
  'Verdict: SMOOTH SAILING (nothing) | MOSTLY GOOD (minor only) | ROUGH SEAS (a confirmed high or a BROKEN thread) | SHIPWRECK (a confirmed critical / security, or multiple broken flows).' +
  ' Structured output: verdict, actionItems (verbatim /jc lines), review (the full markdown you wrote).';

// ── claimExtractor (source lines 75-77 + the E2 breadth clause — the ONE intended extractor-prompt
// change: "Target ~4-6 claims per task, covering EVERY task — breadth across tasks before depth within
// any task." inserted before the conflictChecks sentence, exactly as the proven reference variant
// tmp/walker-measure/variants/wave-walker.v2-manifest-scale.js ships it) ──
export const expectedClaimExtractor = ({
  manifestPath,
  repoRoot,
}: {
  manifestPath: string;
  repoRoot: string;
}): string =>
  'You are the CLAIM EXTRACTOR of a manifest-verify panel. Read the wave manifest at ' +
  manifestPath +
  ' (repo root ' +
  repoRoot +
  ') and mine EVERY load-bearing factual claim a hallucination could hide in — a claim is load-bearing when refuting it would change a task\'s design or scope. Per task extract: existence claims (a named file/symbol/field/column/enum/env var/prompt the task assumes EXISTS or assumes ABSENT — incl. every Named anchor and File-plan path), behavior premises ("X currently does Y" statements the design rests on — the classic hallucination class), contract claims (SDL/SQS/WS shapes vs live code), dep claims (cross-task Depends and shared symbols). Each: id T{n}-C{k}, taskId, kind, a SELF-CONTAINED refutable statement, exact files to start probing, 1-line context. ORDER MOST LOAD-BEARING FIRST (a cap may drop the tail). Target ~4-6 claims per task, covering EVERY task — breadth across tasks before depth within any task. Also emit conflictChecks: task pairs/sets whose File plans, Contracts, or data models might collide (same file EDIT+DELETE, one field two shapes, duplicated work) — checks only, no verdicts.' +
  RO +
  ' Structured output: claims, conflictChecks.';

// ── claimVerifier (source lines 97-103) ──
export const expectedClaimVerifier = ({
  claim: c,
  question,
  repoRoot,
}: {
  claim: { id: string; statement: string; context?: string; files?: string[] };
  question: string;
  repoRoot: string;
}): string =>
  'You are an INDEPENDENT VERIFIER on a pre-ruling claims panel' +
  (question ? ' grounding this ruling: ' + question : '') +
  '. Repo root: ' +
  repoRoot +
  '.\n' +
  'CLAIM ' +
  c.id +
  ': ' +
  c.statement +
  '\n' +
  (c.context ? 'Context: ' + c.context + '\n' : '') +
  ((c.files || []).length
    ? 'Start from these files (follow imports/greps wherever the evidence leads): ' +
      JSON.stringify(c.files) +
      '\n'
    : '') +
  'Actively try to REFUTE the claim — hunt for the counterexample before accepting confirmation. CONFIRMED only when file evidence proves it AS STATED; REFUTED when evidence contradicts it; PARTIAL when it holds with a material caveat (state it); UNPROVEN when evidence is unfindable. ' +
  'Every evidence anchor grep-verified file:line with a VERBATIM quote (<=120 chars). Judge evidence, not vibes.' +
  RO +
  ' Structured output: claimId=' +
  c.id +
  ', verdict, evidence, reasoning (<=3 sentences).';

// ── consistencyJudge (source lines 125-126 + the E2 payload diet — the SECOND intended prompt change:
// CONFIRMED claims travel in the Consensus map only; full verdict objects ride only for non-CONFIRMED
// claims. Wording transcribed from the proven reference variant.) ──
export const expectedConsistencyJudge = ({
  manifestPath,
  nonConfirmed,
  consensus,
  conflictChecks,
}: {
  manifestPath: string;
  nonConfirmed: unknown[];
  consensus: unknown;
  conflictChecks: unknown[];
}): string =>
  'You are the MANIFEST CONSISTENCY JUDGE. Re-read the manifest at ' +
  manifestPath +
  ", then rule over the panel's evidence. Confirmed claims are listed in Consensus only (their full evidence is elsewhere); full verdict detail below covers only non-CONFIRMED claims. Consensus: " +
  JSON.stringify(consensus) +
  '\nNon-CONFIRMED verdicts (full detail): ' +
  JSON.stringify(nonConfirmed) +
  '\nConflict checks queued by the extractor: ' +
  JSON.stringify(conflictChecks) +
  '\nFind, evidence-based (open files where needed): (1) kind=conflict — cross-task collisions (File plans touching the same symbols incompatibly, contract shapes disagreeing between tasks, Depends order the file plan violates); (2) kind=refuted-premise — every task step resting on a REFUTED/PARTIAL claim, naming the manifest section it invalidates; (3) kind=freeloader — a task/step that does not earn its place (premise gone, work a sibling task duplicates, scope nothing consumes). Each: tasks, what (Expected/Got), evidence (manifest section + code anchor), severity, fix (concrete manifest correction).' +
  RO +
  ' Structured output: conflicts, summary.';

// ── probe (source lines 219-223) ──
export const expectedProbe = ({
  lane,
  goal,
  scopeLine,
}: {
  lane: {
    id: string;
    kind?: string;
    question: string;
    note?: string;
    files?: string[];
    targets?: string[];
  };
  goal: string;
  scopeLine: string;
}): string =>
  'You are lane ' +
  lane.id +
  ' (' +
  (lane.kind || 'pursue') +
  ') of a code investigation. GOAL: ' +
  goal +
  '.' +
  scopeLine +
  '\nQUESTION: ' +
  lane.question +
  (lane.note ? ' — steering: ' + lane.note : '') +
  '\n' +
  ((lane.files || []).length
    ? 'Start files (follow imports/greps wherever evidence leads): ' +
      JSON.stringify(lane.files) +
      '\n'
    : '') +
  (lane.kind === 'attack'
    ? 'ATTACK LANE: actively hunt COUNTER-evidence against claim ids ' +
      JSON.stringify(lane.targets || []) +
      ' (emit kind=counter with targets). A real hunt that finds NOTHING → nothingFound:true — that survival is first-class evidence, not silence.\n'
    : '') +
  'Return quote-pinned claims — SELF-CONTAINED facts, VERBATIM quotes (<=120 chars), grep-verified file:line anchors — plus leads (files/symbols worth a future lane).' +
  RO +
  ' Structured output: laneId=' +
  lane.id +
  ', claims, leads, nothingFound.';

// ── brainer (source lines 237-241) ──
export const expectedBrainer = ({
  goal,
  scopeLine,
  wave,
  maxWaves,
  ledgerRows,
  openLeads,
  maxLanes,
}: {
  goal: string;
  scopeLine: string;
  wave: number;
  maxWaves: number;
  ledgerRows: unknown[];
  openLeads: unknown[];
  maxLanes: number;
}): string =>
  "You are the BRAINER — this investigation's only global reasoner. GOAL: " +
  goal +
  '.' +
  scopeLine +
  ' Wave ' +
  wave +
  '/' +
  maxWaves +
  '.\n' +
  'LEDGER (statuses are COMPUTED from topology — cite ids, never assert status): ' +
  JSON.stringify(ledgerRows) +
  '\n' +
  'OPEN LEADS: ' +
  JSON.stringify(openLeads) +
  '\n' +
  'Return your COORD: resultSoFar + keyClaimIds (the load-bearing ids — confidence is computed over exactly these); lanes ≤' +
  maxLanes +
  ' (pursue|attack; settled REQUIRES a survived challenge, so attack your own emerging answer — an attack lane names targets); dropLeads (dead leads); stop {done, reason} — done ONLY when the goal is answered on settled key claims or further probing cannot change the answer.' +
  ' Structured output: resultSoFar, keyClaimIds, lanes, dropLeads, stop.';

// ── claimAuditor (source line 212) ──
export const expectedClaimAuditor = ({
  rows,
  repoRoot,
}: {
  rows: unknown[];
  repoRoot: string;
}): string =>
  'You are a CLAIM AUDITOR — you are grepping for a pin, not judging truth. For EACH claim id, open/grep the cited anchor file(s) (repo root ' +
  repoRoot +
  ') and verify the VERBATIM quote appears (whitespace-insensitive; within ±5 lines of a cited line number is fine). pass = every quote found; fail = any quote absent. Claims: ' +
  JSON.stringify(rows) +
  RO +
  ' Structured output: audits (one per claim id).';

// synthesiser carries INTENTIONAL DIVERGENCE #4: the trailing anti-degenerate answer line (Professor-authored).
// ── synthesiser (source lines 260-264) ──
export const expectedSynthesiser = (a: {
  goal: string;
  stopReason: string;
  keyIds: unknown[];
  conf: string;
  reportOut: string | null;
  resultSoFarText: string;
  claimsOut: unknown[];
  openLeads: unknown[];
}): string =>
  'You are the SYNTHESISER of a code investigation. GOAL: ' +
  a.goal +
  '. Write the report from the hardened ledger — sections: Answer · Evidence (claims by status, cite ids inline [cN] with anchors) · Counter-evidence & survived challenges · Open leads · Coverage (stopReason: ' +
  a.stopReason +
  '). ' +
  'COMPUTED confidence over key claims ' +
  JSON.stringify(a.keyIds) +
  ' = ' +
  a.conf +
  ' — your stated confidence may be LOWER, never higher. ' +
  (a.reportOut
    ? 'WRITE the full report to ' + a.reportOut + ' (your ONLY file write; run no git). '
    : 'Write no files. ') +
  'resultSoFar: ' +
  a.resultSoFarText +
  '\nLEDGER: ' +
  JSON.stringify(a.claimsOut) +
  '\nOPEN LEADS: ' +
  JSON.stringify(a.openLeads) +
  ' Structured output: answer, confidence, report (full markdown).' +
  "\nThe answer field carries the COMPLETE answer text itself — never a placeholder, never a pointer to the report file; it is the caller's primary deliverable.";
