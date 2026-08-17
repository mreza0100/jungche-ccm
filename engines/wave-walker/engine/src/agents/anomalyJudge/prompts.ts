// anomalyJudge prompt — byte-identical to the source's inline construction (wave-walker.js lines 608-620)
// for every R1-R8 rule. INVARIANT REGISTRY FEATURE (§ 2.3) adds one conditional block, gated on
// `rule === 'R9-INV'` — never true for any R1-R8 call, so those stay byte-identical.
import { RO } from '../shared.js';
import { DEADNESS_BAR } from '../../constants.js';
import type { AnomalyJudgeArgs } from '../../types/index.js';

export const buildAnomalyJudge = ({
  rule,
  ruleMeaning,
  sec,
  instances,
  ctxCards,
  authDoc,
}: AnomalyJudgeArgs): string =>
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
  (rule === 'R9-INV'
    ? 'R9-INV — this anomaly came from an ADVERSARIAL invariant hunter, not the mechanical rule engine: fold three lens duties into one adjudication — (1) reproduce the concrete failure scenario yourself or kill it; (2) hunt for the guard/compensation the finder may have missed; (3) kill anything with no real-world harm. Default to FALSE/UNPROVEN when uncertain — CONFIRMED is reserved for a scenario you can actually walk.\n'
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
