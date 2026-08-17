// threadWalker prompt — byte-identical to the source's inline construction (wave-walker.js lines 441-449)
// EXCEPT for one deliberate, ALWAYS-ON addition: the reorientation sentences below (INVARIANT REGISTRY
// FEATURE, tmp/wave-walker-investigation.md § 2.5 — "cheap, high-yield prompt change", NOT gated by the
// registry; the design names this an unconditional improvement, unlike every other piece of this
// feature). A non-empty charter still appends the Professor-authored WALK CHARTER block (zero bytes
// otherwise) — that part is unchanged.
import { RO } from '../shared.js';
import type { ThreadWalkerArgs } from '../../types/index.js';

export const buildThreadWalker = ({
  walkerDoc,
  hygieneDoc,
  thread,
  charter,
}: ThreadWalkerArgs): string =>
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
  ' Structured output: threadId, name, type, flow (INTACT|AT-RISK|BROKEN|N/A), trace (step → step, marking any break), defects (each {what, location=file:line, failureScenario, jc=`/jc {fix}`}), hygiene (each {kind, where=file:line, detail, jc}), notes.' +
  (charter
    ? '\nWALK CHARTER (caller-supplied duty): ' +
      charter +
      '\nWeigh this thread against the charter and report charter-relevant findings explicitly in notes — on top of the standard verdict, never instead of it.'
    : '');
