// scout prompt — byte-identical to the source's inline construction (wave-walker.js lines 400-415) when charter is '' ; a non-empty charter appends the Professor-authored WALK CHARTER block (zero bytes otherwise). INVARIANT REGISTRY FEATURE (§ 2.1): a non-empty `invariants` list appends step 7 (zero bytes when empty/absent — the floor).
import { RO } from '../shared.js';
import type { ScoutArgs } from '../../types/index.js';

export const buildScout = ({
  reportPath,
  branch,
  walkerDoc,
  maxFieldsPerJob,
  charter,
  invariants,
  invariantsDoc,
  reconcile,
  repoRoot,
  authDoc,
  gateResolverPattern,
}: ScoutArgs): string =>
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
    : '') +
  (charter
    ? '\nWALK CHARTER (caller-supplied duty): ' +
      charter +
      '\nShape the thread manifest to serve this charter IN ADDITION to the standard enumeration — add charter-driven threads; never drop or merge a standard thread for it.'
    : '') +
  (invariants && invariants.length
    ? '\n7) INVARIANT REGISTRY (' +
      (invariantsDoc || '') +
      ') — a durable registry of sacred cross-cutting semantics, seeded by a proven adversarial bug-hunt. For EACH entry below, test its `triggers` against this diff (does the diff touch its territory, add/modify a reuse-skip-cache gate, touch an engine-stamped column, etc. — read the triggers literally). Registry: ' +
      JSON.stringify(invariants) +
      '. Return armedInvariants: one entry per registry id whose trigger fires, each {id, matchedFiles (the diff files that armed it), reason}. Arm generously — a missed arm is a missed hunt; when genuinely uncertain, arm it. Do NOT drop or merge a standard thread to make room for this — it is additive.'
    : '');
