// constants.ts — shared string constants. DEADNESS_BAR/CATCHBOOK/ENC_VOCAB/DEC_VOCAB were ported verbatim
// from the source's inline module-level declarations (wave-walker.js lines 279-302, 592-601);
// DEADNESS_BAR's two domain-specific clauses (stakes, deadness-surfaces) now read off CONFIG.PROJECT (the
// universal-bundle refactor — args.project), falling back to a generic default when no project profile is
// supplied. RULE_MEANING's R6 entry interpolates the scout-extracted (or configured-fallback) AUTH_RULE at
// prompt-build time in the source (it is built AFTER AUTH_RULE is resolved) — ported here as
// `ruleMeaning(authRule)`, a pure function returning the exact same object the source builds inline
// (source lines 592-601). The AUTH_RULE fallback string itself now lives on CONFIG.PROJECT.authRuleFallback
// (default '' — no fallback) and is read directly by engine.ts; there is no module-level constant for it.
import { CONFIG } from './config.js';

const DEFAULT_STAKES_LINE = 'a false dead verdict on live code is a production regression';
const DEFAULT_DEADNESS_SURFACES =
  'dynamic/by-name dispatch (registries, plugin loaders), serialized payload fields (queues, APIs), and generated-vs-hand-written type drift';

export const DEADNESS_BAR =
  'DEADNESS BAR (for any dead/unread/orphan verdict): prove it ALIVE first — ' +
  (CONFIG.PROJECT?.stakesLine || DEFAULT_STAKES_LINE) +
  '. ' +
  'Dead only with zero PRODUCTION consumers AND the surfaces a static grep misses: ' +
  (CONFIG.PROJECT?.deadnessSurfaces || DEFAULT_DEADNESS_SURFACES) +
  ', and test/config/reflection consumers. Cannot prove past the bar -> NOT dead: verdict UNPROVEN, keep the code.';

export const CATCHBOOK =
  'Catch-book categories (tag findings): DUP (reinvented helper/type/hook; copy-pasted consumer pattern), DEAD (unreachable/orphaned, subject to the deadness bar), ' +
  'GHOST (dual-writes, manual sync, schema<->code field mismatch), SMELL (cross-boundary writes, shallow error handling, N+1, wrong layer, over-engineering), ' +
  'TYPE-GAP (unguarded casts, hand-typed drift, loose String where enum belongs), NAMING (concept drift, scope-dishonest names), ' +
  'QUALITY (magic literals, hardcoded i18n, fetch-in-leaf-component), STALE-DEP (unused/phantom imports).';

export const ENC_VOCAB =
  'encoding vocabulary (EXACTLY one): raw | object | json-string | enum-string | number | boolean | unknown';
export const DEC_VOCAB =
  'decode vocabulary (EXACTLY one per consumer): direct | render | json-parse | object-index | compare | spread | unknown';

export function ruleMeaning(authRule: string): Record<string, string> {
  return {
    R1: 'orphan producer — produced but no production consumer reads it. Apply the deadness bar.',
    R2: 'phantom consumer — a field is read/returned that no producer/SDL declares; the read silently yields undefined or ships out-of-contract data.',
    R3: 'encoding mismatch — producer encodes one way, consumer decodes another (incl. JSON.parse(JSON.stringify(x)), which returns x unchanged).',
    R4: 'value-set mismatch — a consumer compares against literals no producer emits; that branch is permanently dead. Casing-only difference is a certain bug.',
    R5: 'type drift — a hand-written type disagrees at BASE-type level with the generated/SDL truth.',
    R6:
      'gate asymmetry / mandated-fence violation — auth fences unequal across a resource class, or the documented ownership-fence rule violated. ' +
      authRule,
    R7: 'unfenced ID flow — a client-supplied ID reaches data access with no fence at all.',
    R8: 'dangling reference — a reference resolving to nothing.',
    'R9-INV':
      'invariant-registry violation — a hunter-found breach of a cross-cutting sacred rule, found by TERRITORY (not diff scope). Adversarial, not mechanical: REFUTE-FIRST — reproduce the sequence or kill, hunt the guard/compensation the finder missed, kill anything with no real-world harm.',
  };
}
