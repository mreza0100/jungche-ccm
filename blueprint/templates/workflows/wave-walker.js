export const meta = {
  name: 'wave-walker',
  description: 'Wave Walker — wave verification. Walks the wave\'s diff (merged SHAs, or a pre-merge worktree branch via args.branch) two ways in one pass and folds them. (1) THREAD WALK (the proven thread-walk floor): a scout enumerates feature-flow / seam / invariant threads from the integrated diff, one Sonnet walker per thread confirms the flow reaches its terminal state and catches the integration-delta hygiene. (2) LEDGER SPINE (the mechanical add): the same scout schedules Haiku sensors over the GraphQL type-fields + entry-point gates the diff touches; they extract comparable cards; a zero-token JS rule engine diffs them (orphan producer, phantom consumer, encoding/double-encode mismatch, value-set/casing mismatch, base-type drift, gate-outlier, mandated-fence violation, unfenced ID flow, dangling refs); Sonnet judges only the flagged anomalies, Opus second-opinions killed security/near-certain ones, Sonnet territory digests catch the un-mechanizable smells the rules and the walk cannot see, and one FINAL Opus judge rules the whole walk (authoritative verdict, reinstates wrong kills, names missed cross-cutting risks). A fold merges thread verdicts + confirmed anomalies + hygiene + digest findings + the final judgment into `## Professor\'s Wave Review` in the report and returns { verdict, actionItems, review }; the ledger travels in the RESULT and the caller persists it. A diff with no GraphQL surface runs pure thread-walk — the floor never regresses. Flow graph is a declared copy of wave/walker.md § Orchestration. (3) SECURITY: the Walk barrier carries one diff-scoped auditor applying audit/security.md (8A–8K) to the wave\'s changed surface; findings ride the final judgment, the review\'s Security Audit section, and the action items. (4) VERIFY MODE (args.claims — no reportPath): skips the walk; a pre-ruling claims panel fact-checks load-bearing claims against named files (one read-only verifier per claim × votes, Sonnet-xhigh pinned, per-claim opus flag) and returns verdicts + evidence for the CALLER to rule over — no file writes. (5) MANIFEST-VERIFY (args.manifestPath): a claim extractor mines the manifest\'s load-bearing claims (hallucinated fields/premises), the panel probes each, and a consistency judge flags cross-task conflicts + refuted premises + freeloader tasks. (6) INVESTIGATE (args.goal) — RR-for-code: lens probes seed a quote-pinned claim ledger; an Opus brainer steers ≤maxWaves of pursue/attack lanes over it (settled REQUIRES a survived challenge); a Haiku auditor greps every quote-pin; status and confidence are COMPUTED from ledger topology, never asserted; a synthesiser writes the cited report with confidence floored by the computed value; every death degrades loudly, never silently.',
  phases: [{ title: 'Scout' }, { title: 'Walk' }, { title: 'Judge' }, { title: 'Fold' }, { title: 'Verify' }, { title: 'Investigate' }],
}
// ╔══ module: src/agents/shared.ts ════════════════════════════════════════
// Shared cross-seat prompt fragment — RO (source line 55), appended verbatim by 14 of the 17 seats
// (every read-only extractor/judge/auditor/probe; NOT synth/fold, which write files, and NOT brainer,
// which reasons over supplied text only). One definition, never duplicated per seat.
const RO =
  ' Read-only: Grep/Glob, Read, and git log/show/diff/rev-parse only — run no other code, write no files, mutate no git.';
// ╔══ module: src/constants.ts ════════════════════════════════════════════
// constants.ts — verbatim shared string constants ported from the source's inline module-level
// declarations (wave-walker.js lines 279-302, 592-601). AUTH_RULE_FALLBACK/DEADNESS_BAR/CATCHBOOK/
// ENC_VOCAB/DEC_VOCAB are pure static strings, byte-identical to the source. RULE_MEANING's R6 entry
// interpolates the scout-extracted (or fallback) AUTH_RULE at prompt-build time in the source (it is
// built AFTER AUTH_RULE is resolved) — ported here as `ruleMeaning(authRule)`, a pure function
// returning the exact same object the source builds inline (source lines 592-601).

// AUTH_RULE is extracted LIVE by the scout from {project}/CLAUDE.md § Auth Pattern (heading grep, never line
// numbers). The fallback below fires ONLY when the scout returns no usable extract; it is a declared copy of that
// section's Role-fences bullet — any edit to § Auth Pattern re-syncs this string (grep AUTH_RULE_FALLBACK).
const AUTH_RULE_FALLBACK =
  '{project}/CLAUDE.md § Auth Pattern (FALLBACK COPY — verify against the live file): "Role fences (founder-ruled, SACRED — reads AND writes): '
  + 'THERAPIST is fenced to OWNERSHIP (record.therapistId === user.id) — never another therapist\'s patients, even inside the same clinic. '
  + 'SUPERVISOR is a therapist with clinic-wide access inside their OWN clinic ONLY (record.clinicId === user.clinicId) — never another clinic, never global. '
  + 'Clinic-equality alone is NEVER a sufficient fence on a THERAPIST-reachable path (that is the cross-therapist PHI leak). Every path that loads or mutates a patient/session/couple/note/document '
  + 'by client-supplied id branches by role and applies the matching fence — proof pattern: requireTherapistOwnsPatient (treatment-plan.resolvers.ts). Fence both roles or neither."'

const DEADNESS_BAR =
  'DEADNESS BAR (for any dead/unread/orphan verdict): prove it ALIVE first — a false dead in a clinical product is a live-session regression. '
  + 'Dead only with zero PRODUCTION consumers across all five projects AND the surfaces a static grep misses: GraphQL SDL queried by name, SQS payload fields, '
  + 'the Cortex prompt registry (knowledge/prompts/*.md via load_prompt), {ORM} migrations, Expo Router file-routes, Pydantic/JSON (de)serialization, '
  + 'and test/config/reflection consumers. Cannot prove past the bar -> NOT dead: verdict UNPROVEN, keep the code.'

const CATCHBOOK =
  'Catch-book categories (tag findings): DUP (reinvented helper/type/hook; copy-pasted consumer pattern), DEAD (unreachable/orphaned, subject to the deadness bar), '
  + 'GHOST (dual-writes, manual sync, schema<->code field mismatch), SMELL (cross-boundary writes, shallow error handling, N+1, wrong layer, over-engineering), '
  + 'TYPE-GAP (unguarded casts, hand-typed drift, loose String where enum belongs), NAMING (concept drift, scope-dishonest names), '
  + 'QUALITY (magic literals, hardcoded i18n, fetch-in-leaf-component), STALE-DEP (unused/phantom imports).'

const ENC_VOCAB = 'encoding vocabulary (EXACTLY one): raw | object | json-string | enum-string | number | boolean | unknown'
const DEC_VOCAB = 'decode vocabulary (EXACTLY one per consumer): direct | render | json-parse | object-index | compare | spread | unknown'

function ruleMeaning(authRule        )                         {
  return {
    R1: 'orphan producer — produced but no production consumer reads it. Apply the deadness bar.',
    R2: 'phantom consumer — a field is read/returned that no producer/SDL declares; the read silently yields undefined or ships out-of-contract data.',
    R3: 'encoding mismatch — producer encodes one way, consumer decodes another (incl. JSON.parse(JSON.stringify(x)), which returns x unchanged).',
    R4: 'value-set mismatch — a consumer compares against literals no producer emits; that branch is permanently dead. Casing-only difference is a certain bug.',
    R5: 'type drift — a hand-written type disagrees at BASE-type level with the generated/SDL truth.',
    R6: 'gate asymmetry / mandated-fence violation — auth fences unequal across a resource class, or the documented ownership-fence rule violated. ' + authRule,
    R7: 'unfenced ID flow — a client-supplied ID reaches data access with no fence at all.',
    R8: 'dangling reference — a reference resolving to nothing.',
    'R9-INV':
      'invariant-registry violation — a hunter-found breach of a cross-cutting sacred rule, found by TERRITORY (not diff scope). Adversarial, not mechanical: REFUTE-FIRST — reproduce the sequence or kill, hunt the guard/compensation the finder missed, kill anything with no real-world harm.',
  }
}
// ╔══ module: src/agents/anomalyJudge/prompts.ts ══════════════════════════
// anomalyJudge prompt — byte-identical to the source's inline construction (wave-walker.js lines 608-620)
// for every R1-R8 rule. INVARIANT REGISTRY FEATURE (§ 2.3) adds one conditional block, gated on
// `rule === 'R9-INV'` — never true for any R1-R8 call, so those stay byte-identical.


                                                             

const buildAnomalyJudge = ({ rule, ruleMeaning, sec, instances, ctxCards }                  )         =>
  'You are an anomaly JUDGE. Rule ' + rule + ': ' + ruleMeaning + '\n'
  + 'For EACH instance: open the file(s) at the cited anchors (BOTH ends where two are given), confirm the facts, and rule CONFIRMED (severity, one-sentence what, location=file:line, fix=`/jc {fix}`), FALSE (say why), or UNPROVEN (say what is missing). Judge evidence, not vibes.\n'
  + (sec ? 'SECURITY: this rule enforces a WRITTEN project invariant. "Every sibling does it the same way" is NOT a defense — a documented-rule violation is CONFIRMED even when it is the file-wide pattern. Read {project}/CLAUDE.md § Auth Pattern before any FALSE.\n' : '')
  + (rule === 'R9-INV' ? 'R9-INV — this anomaly came from an ADVERSARIAL invariant hunter, not the mechanical rule engine: fold three lens duties into one adjudication — (1) reproduce the concrete failure scenario yourself or kill it; (2) hunt for the guard/compensation the finder may have missed; (3) kill anything with no real-world harm. Default to FALSE/UNPROVEN when uncertain — CONFIRMED is reserved for a scenario you can actually walk.\n' : '')
  + DEADNESS_BAR + '\nInstances: ' + JSON.stringify(instances) + '\n'
  + (ctxCards.length ? 'Extracted cards for context (verify against real files): ' + JSON.stringify(ctxCards) + '\n' : '')
  + RO + ' Structured output: verdicts (one per instance, anomalyId matching).';
// ╔══ module: src/config.ts ═══════════════════════════════════════════════
// ─────────────────────────────────────────────────────────────────────────────
// Configs — the single source of truth for the whole engine, mirroring rr's config.ts discipline:
// every tunable knob, the per-seat model TIER + reasoning EFFORT maps, and the mode-derived args live
// HERE. Nothing is hardcoded elsewhere — engine.ts / rules.ts / the agent modules READ every value off
// the CONFIG singleton.
//
// Mode dispatch (source lines 26-29, 57-58, 141): VERIFY/MANIFEST-VERIFY (args.manifestPath or
// args.claims) takes precedence, then INVESTIGATE (args.goal), else WALK (args.reportPath required).
// The 17-seat TIER/EFFORT defaults are read verbatim off the source's per-call `resilient(...)` /
// `agent(...)` option objects — where the source passes NO explicit `effort` key (threadWalker,
// anomalyJudge, territoryDigest, secondOpinion, finalJudge, fold), the seat defaults to 'high', root
// CLAUDE.md's stated default effort (the source relies on the harness's own default in that case;
// there is no other value to port). claimAuditor/synthesiser have a HARDCODED effort in the source
// (no arg) — ported as fixed defaults, still overridable through `agents.<seat>.effort` like every
// other seat.
// ─────────────────────────────────────────────────────────────────────────────
                                                                             

                                                                         

const DEFAULT_LENSES = [
  'DIRECT — the goal head-on',
  'SKEPTIC — hunt the evidence that would make the obvious answer WRONG',
  'BLAST-RADIUS — callers, consumers, config, and tests the goal implicates',
];

class Configs {
  rawArgs         ;
  mode      ;

  // ── fixed doc/path constants (source lines 34, 51) ──
  WALKER_DOC = '.claude/commands/wave/walker.md';
  SECURITY_DOC = '.claude/commands/audit/security.md';
  // INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — provenance pointer cited in
  // the scout prompt; the JS engine never reads this file itself (no fs access in src/ — confirmed by
  // grep). The registry's DATA arrives structured via args.invariants (see INVARIANTS below); this doc
  // path is what the /pcm promotion step's orchestrator wiring parses into that JSON.
  INVARIANTS_DOC = '.claude/commands/wave/walker-invariants.md';

  // ── WALK mode ──
  REPORT_PATH               ;
  BRANCH               ;
  LEDGER_PATH               ;
  MAX_FIELDS_PER_JOB        ;
  MAX_SENSORS        ;
  EXTRA_THREADS           ;
  // E3 gate-conditional dispatch override: `fullGateSweep: true` forces the repo-wide gate-file sweep
  // regardless of the diff classifier (engine.ts § isGateRelevant). Ported from the proven v3 variant.
  FULL_GATE_SWEEP         ;
  // CHARTER — the walk's caller-supplied duty note (walk mode only). Non-empty → a clearly-delimited
  // 'WALK CHARTER' block is appended to the scout / thread-walker / territory-digest / final-judge
  // prompts (conditional-append, zero bytes when empty — the same pattern as verify-mode's QUESTION).
  // The charter ADDS focus on top of the standard enumeration/judgment, never replaces it, and never
  // touches the security-auditor, gate-sweep, or sensor prompts. No output schema changes anywhere.
  CHARTER        ;
  // INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — args.invariants, validated and
  // structured. [] (the default — no args.invariants) is THE FLOOR: no hunter/coverageCritic dispatched,
  // scout prompt gets zero added bytes, review/ledger byte-identical to today. Malformed shape throws
  // loudly (never a silent partial registry), same discipline as `charter`/`agents`.
  INVARIANTS                 ;
  // WALK TELEMETRY (DEBUG STEP) — mirrors rr's config.ts:383 debugger flag exactly: DEFAULT ON. A
  // structured `debugRecord` + a mechanically-rendered `### Walk Telemetry` block give "good data of
  // what's half broken" after a few runs, at zero LLM cost (see runtime.ts SEAT_TALLY, engine.ts
  // assembleDebugRecord, utils/index.ts renderTelemetryMd). `debug:false` reproduces today's behavior
  // byte-for-byte — no record, no telemetry block, zero added bytes anywhere.
  debug         ;
  DEBUG_PATH               ;

  // ── VERIFY / MANIFEST-VERIFY mode ──
  MANIFEST_PATH               ;
  CLAIMS           ;
  VOTES        ;
  QUESTION        ;
  MAX_CLAIMS        ;
  SOLO_THRESHOLD        ;

  // ── INVESTIGATE mode ──
  GOAL        ;
  SCOPE                 ;
  LENSES          ;
  MAX_WAVES        ;
  MAX_LANES        ;
  REPORT_OUT               ;

  // ── per-seat model TIER + reasoning EFFORT — keyed by seat name (types/agents.ts Seat). Frontier
  // seats (brainer, finalJudge, secondOpinion) default to 'opus' and warn loudly on any downgrade —
  // see the `agents` override loop below. ──
  TIER                      ;
  EFFORT                        ;
  // SENSOR_ESCALATE — the sliceSensor/gateSweep dead-agent escalate model (source line 39: `const
  // SENSOR_ESCALATE = 'sonnet'`). NOT an arg in the source (no `sensorEscalateModel` knob exists) and
  // NOT part of the `agents.<seat>` override system — a fixed internal constant, ported verbatim.
  SENSOR_ESCALATE       = 'sonnet';

  constructor(rawArgs         ) {
    let parsed          = rawArgs;
    if (typeof parsed === 'string') {
      try {
        parsed = JSON.parse(parsed);
      } catch (e) {
        throw new Error('wave-walker: args is a string but not valid JSON: ' + ((e         ).message || e));
      }
    }
    const arg = (parsed || {})           ;
    const hasClaims = Array.isArray(arg.claims) && (arg.claims             ).length > 0;
    if (!parsed || (!arg.reportPath && !arg.manifestPath && !arg.goal && !hasClaims))
      throw new Error(
        'wave-walker requires args.reportPath (walk), args.claims (verify), args.manifestPath (manifest-verify), or args.goal (investigate); see wave/walker.md for the contract',
      );
    this.rawArgs = arg;

    // ── mode (source lines 58, 141; the returned `mode` field for verify/manifest-verify is gated on
    // args.manifestPath truthiness alone — see source line 133) ──
    this.MANIFEST_PATH = typeof arg.manifestPath === 'string' ? arg.manifestPath : null;
    this.CLAIMS = Array.isArray(arg.claims) ? (arg.claims             ) : [];
    this.mode = this.MANIFEST_PATH
      ? 'manifest-verify'
      : this.CLAIMS.length
        ? 'verify'
        : arg.goal
          ? 'investigate'
          : 'walk';

    // ── WALK mode config (source lines 31-53) ──
    this.REPORT_PATH = typeof arg.reportPath === 'string' ? arg.reportPath : null;
    this.BRANCH = typeof arg.branch === 'string' ? arg.branch : null;
    this.LEDGER_PATH =
      typeof arg.ledgerPath === 'string'
        ? arg.ledgerPath
        : this.REPORT_PATH
          ? this.REPORT_PATH.replace(/report\.md$/, '') + 'walker-ledger.json'
          : null;
    this.MAX_FIELDS_PER_JOB = Number.isInteger(arg.maxFieldsPerJob) ? (arg.maxFieldsPerJob          ) : 18;
    this.MAX_SENSORS = Number.isInteger(arg.maxSensors) ? (arg.maxSensors          ) : 60;
    this.EXTRA_THREADS = Array.isArray(arg.extraThreads) ? (arg.extraThreads             ) : [];
    this.FULL_GATE_SWEEP = arg.fullGateSweep === true; // strict — exactly the v3 variant's `!== true` gate inverted
    // charter — absent/null → '' (no-op); anything else must be a string (a mis-typed duty note must
    // fail loudly, never silently walk without its charter).
    if (arg.charter !== undefined && arg.charter !== null && typeof arg.charter !== 'string')
      throw new Error('wave-walker: charter must be a string (the walk\'s caller-supplied duty note), got ' + JSON.stringify(arg.charter));
    this.CHARTER = typeof arg.charter === 'string' ? arg.charter : '';
    // INVARIANT REGISTRY FEATURE — absent/null → [] (THE FLOOR: no hunter/critic, byte-identical walk).
    // A non-array, or an array with a malformed entry, throws loudly — never a silently-partial registry.
    this.INVARIANTS = Configs.parseInvariants(arg.invariants);
    // WALK TELEMETRY (DEBUG STEP) — mirrors rr config.ts:383 `this.debug = bool(arg.debug, true)`,
    // DEFAULT ON. DEBUG_PATH derives from REPORT_PATH the same way LEDGER_PATH does (walker-debug.json
    // sibling); an explicit args.debugPath overrides it; both null when there's no REPORT_PATH to derive from.
    this.debug = Configs.bool(arg.debug, true);
    this.DEBUG_PATH =
      typeof arg.debugPath === 'string'
        ? arg.debugPath
        : this.REPORT_PATH
          ? this.REPORT_PATH.replace(/report\.md$/, '') + 'walker-debug.json'
          : null;

    // ── VERIFY / MANIFEST-VERIFY config (source lines 59-64, plus the E2 manifest-coverage lever) ──
    this.VOTES = Number.isInteger(arg.votes) && (arg.votes          ) > 0 ? (arg.votes          ) : 1;
    this.QUESTION = typeof arg.question === 'string' ? arg.question : '';
    // E2 DEFAULT CHANGE (the port's ONE behavioral delta from the source): maxClaims 24 → 96. Proven on
    // wave.md: 96 gave full 16/16-task coverage; 24 silently dropped 55 claims / 12 tasks. args.maxClaims
    // still overrides.
    this.MAX_CLAIMS = Number.isInteger(arg.maxClaims) ? (arg.maxClaims          ) : 96;
    // E2 batching gate: a panel (claims × votes) ≤ SOLO_THRESHOLD runs SOLO exactly like the source
    // (small-panel latency + per-claim opus escalation preserved); above it, claims batch ≤4 by
    // file-cluster affinity — verifiers STAY on the verifier tier (sonnet/xhigh), never haiku
    // (measured: haiku did 2.5× the tool calls → +21% tokens, +235% latency; rejected).
    this.SOLO_THRESHOLD = Number.isInteger(arg.soloThreshold) ? (arg.soloThreshold          ) : 8;

    // ── INVESTIGATE config (source lines 142-153) ──
    this.GOAL = arg.goal != null ? String(arg.goal) : '';
    this.SCOPE = Array.isArray(arg.scope) && (arg.scope             ).length ? (arg.scope            ) : null;
    this.LENSES = Array.isArray(arg.lenses) && (arg.lenses             ).length ? (arg.lenses            ) : DEFAULT_LENSES;
    this.MAX_WAVES = Number.isInteger(arg.maxWaves) ? (arg.maxWaves          ) : 3;
    this.MAX_LANES = Number.isInteger(arg.maxLanes) ? (arg.maxLanes          ) : 5;
    this.REPORT_OUT = typeof arg.reportOut === 'string' ? arg.reportOut : null;

    // ── per-seat defaults, seeded verbatim off the source's per-call option objects. Several seats
    // deliberately SHARE one legacy arg (sensorModel/sensorEffort → sliceSensor + gateSweep;
    // verifierModel/verifierEffort → claimExtractor + claimVerifier + consistencyJudge;
    // securityEscalateModel → secondOpinion, and also the verify-mode per-claim opus escalation — see
    // agents/claimVerifier/run.ts) — exactly the source's own knob reuse, not a porting shortcut. ──
    const str = (v         , d        )         => (typeof v === 'string' && v.length ? v : d);
    const scoutModel = str(arg.scoutModel, 'sonnet')        ;
    const walkerModel = str(arg.walkerModel, 'sonnet')        ;
    const sensorModel = str(arg.sensorModel, 'haiku')        ;
    const sensorEffort = str(arg.sensorEffort, 'medium')          ;
    const judgeModel = str(arg.judgeModel, 'sonnet')        ;
    const digestModel = str(arg.digestModel, 'sonnet')        ;
    const foldModel = str(arg.foldModel, 'sonnet')        ;
    const securityModel = str(arg.securityModel, 'sonnet')        ;
    const securityEffort = str(arg.securityEffort, 'xhigh')          ;
    const verifierModel = str(arg.verifierModel, 'sonnet')        ;
    const verifierEffort = str(arg.verifierEffort, 'xhigh')          ;
    const probeModel = str(arg.probeModel, 'sonnet')        ;
    const probeEffort = str(arg.probeEffort, 'xhigh')          ;
    const auditModel = str(arg.auditModel, 'haiku')        ;
    const synthModel = str(arg.synthModel, 'sonnet')        ;
    // INVARIANT REGISTRY FEATURE — invariantHunter/coverageCritic default sonnet/high, same tier as
    // securityAuditor (its closest precedent: adversarial, diff/territory-scoped, non-frontier by root
    // CLAUDE.md § Model Selection's own taxonomy — spec-execution work with a defined territory+brief,
    // not open-ended judgment). No dedicated legacy `<seat>Model` arg (neither seat existed pre-feature)
    // — retuned only via `agents.<seat>`, like every seat added after the original 17.
    const invariantHunterModel       = 'sonnet';
    const coverageCriticModel       = 'sonnet';
    // FRONTIER-JUDGMENT SEATS — final judge, security second-opinion, investigate brainer. Durable
    // default = the 'opus' alias, per root CLAUDE.md § Model Selection; never a model literal here. A
    // limited-time frontier model rides ONLY the invocation args (finalJudgeModel / securityEscalateModel
    // / brainerModel). Security/auth judgment seats never silently downgrade below opus — see the
    // `agents` override loop below.
    const finalJudgeModel = str(arg.finalJudgeModel, 'opus')        ;
    const securityEscalateModel = str(arg.securityEscalateModel, 'opus')        ;
    const brainerModel = str(arg.brainerModel, 'opus')        ;
    const brainerEffort = str(arg.brainerEffort, 'xhigh')          ;

    this.TIER = {
      scout: scoutModel,
      threadWalker: walkerModel,
      sliceSensor: sensorModel,
      gateSweep: sensorModel,
      securityAuditor: securityModel,
      anomalyJudge: judgeModel,
      territoryDigest: digestModel,
      secondOpinion: securityEscalateModel,
      finalJudge: finalJudgeModel,
      fold: foldModel,
      claimExtractor: verifierModel,
      claimVerifier: verifierModel,
      consistencyJudge: verifierModel,
      probe: probeModel,
      brainer: brainerModel,
      claimAuditor: auditModel,
      synthesiser: synthModel,
      invariantHunter: invariantHunterModel,
      coverageCritic: coverageCriticModel,
    };
    this.EFFORT = {
      scout: 'high',
      threadWalker: 'high',
      sliceSensor: sensorEffort,
      gateSweep: sensorEffort,
      securityAuditor: securityEffort,
      anomalyJudge: 'high',
      territoryDigest: 'high',
      secondOpinion: 'high',
      finalJudge: 'high',
      fold: 'high',
      claimExtractor: verifierEffort,
      claimVerifier: verifierEffort,
      consistencyJudge: verifierEffort,
      probe: probeEffort,
      brainer: brainerEffort,
      claimAuditor: 'medium', // hardcoded in the source (no auditEffort arg) — source line 213
      synthesiser: 'xhigh', // hardcoded in the source (no synthEffort arg) — source line 265
      invariantHunter: 'high',
      coverageCritic: 'high',
    };

    // ── PER-SEAT OVERRIDE (`agents` arg) — retune any seat's model/effort without touching source,
    // mirroring rr's config.ts override loop exactly. Unknown seat / bad model / bad effort throw
    // loudly. brainer/finalJudge/secondOpinion downgrading below opus logs a loud warning, never throws
    // — those three are the frontier-judgment seats named in root CLAUDE.md § Model Selection. ──
    const VALID_TIERS         = ['haiku', 'sonnet', 'opus'];
    const VALID_EFFORTS           = ['low', 'medium', 'high', 'xhigh', 'max'];
    const FRONTIER_SEATS = ['brainer', 'finalJudge', 'secondOpinion'];
    const seats = Object.keys(this.TIER); // the canonical seat names — read off the default map itself (17 original + invariantHunter/coverageCritic, INVARIANT REGISTRY FEATURE)
    if (arg.agents !== undefined && arg.agents !== null) {
      if (typeof arg.agents !== 'object' || Array.isArray(arg.agents))
        throw new Error('wave-walker: agents must be an object keyed by seat name, e.g. { scout: { model: "opus" } }');
      const overrides = arg.agents                           ;
      for (const seat of Object.keys(overrides)) {
        if (!seats.includes(seat))
          throw new Error('wave-walker: unknown agent seat "' + seat + '" in `agents` — valid seats: ' + seats.join(', '));
        const o = overrides[seat];
        if (typeof o !== 'object' || o === null || Array.isArray(o))
          throw new Error('wave-walker: agents.' + seat + ' must be an object { model?, effort? }');
        const { model, effort } = o                                         ;
        if (model !== undefined) {
          if (!VALID_TIERS.includes(model        ))
            throw new Error('wave-walker: agents.' + seat + '.model must be one of ' + VALID_TIERS.join(', ') + ', got ' + JSON.stringify(model));
          this.TIER[seat] = model        ;
          if (FRONTIER_SEATS.includes(seat) && model !== 'opus') {
            try {
              if (typeof log === 'function')
                log(
                  '⚠ wave-walker: agents.' +
                    seat +
                    '.model overridden to "' +
                    model +
                    '" (below opus) — ' +
                    seat +
                    ' is a frontier-judgment seat (final verdict / security second-opinion / investigate brain); a downgrade risks a wrong ruling',
                );
            } catch (e) {
              /* log not available at construction (unit test) → skip the warning */
            }
          }
        }
        if (effort !== undefined) {
          if (!VALID_EFFORTS.includes(effort          ))
            throw new Error('wave-walker: agents.' + seat + '.effort must be one of ' + VALID_EFFORTS.join(', ') + ', got ' + JSON.stringify(effort));
          this.EFFORT[seat] = effort          ;
        }
      }
    }
  }

  // INVARIANT REGISTRY FEATURE — validates args.invariants into InvariantSpec[]. Absent/null → [] (the
  // floor). Any other shape is validated ENTRY-BY-ENTRY and throws loudly on the first defect, naming it
  // — same "fail loud, never a silent partial" discipline as `charter`/`agents` above. A malformed entry
  // here would otherwise either crash a later JSON.stringify into a prompt or silently disarm territory
  // matching (an empty `territory` array can never glob-match anything) — both are worse than a
  // construction-time throw.
          static parseInvariants(raw         )                  {
    if (raw === undefined || raw === null) return [];
    if (!Array.isArray(raw)) throw new Error('wave-walker: invariants must be an array of registry entries, got ' + JSON.stringify(raw));
    const isStringArray = (v         )                => Array.isArray(v) && v.every((x) => typeof x === 'string');
    return raw.map((entry, i) => {
      if (typeof entry !== 'object' || entry === null || Array.isArray(entry))
        throw new Error('wave-walker: invariants[' + i + '] must be an object, got ' + JSON.stringify(entry));
      const e = entry                           ;
      for (const field of ['id', 'law', 'huntBrief'])
        if (typeof e[field] !== 'string' || !e[field])
          throw new Error('wave-walker: invariants[' + i + '].' + field + ' must be a non-empty string, got ' + JSON.stringify(e[field]));
      if (!isStringArray(e.territory) || !(e.territory            ).length)
        throw new Error('wave-walker: invariants[' + i + '].territory must be a non-empty array of glob strings, got ' + JSON.stringify(e.territory));
      if (e.triggers !== undefined && !isStringArray(e.triggers))
        throw new Error('wave-walker: invariants[' + i + '].triggers must be an array of strings, got ' + JSON.stringify(e.triggers));
      if (e.exemplars !== undefined && !isStringArray(e.exemplars))
        throw new Error('wave-walker: invariants[' + i + '].exemplars must be an array of strings, got ' + JSON.stringify(e.exemplars));
      return {
        id: e.id          ,
        law: e.law          ,
        territory: e.territory            ,
        triggers: isStringArray(e.triggers) ? e.triggers : [],
        exemplars: isStringArray(e.exemplars) ? e.exemplars : [],
        huntBrief: e.huntBrief          ,
      };
    });
  }

  // WALK TELEMETRY (DEBUG STEP) — a strict-typed boolean reader, mirroring rr's local `bool()` helper
  // (rr engine/src/config.ts:119: `(v: unknown, d: boolean): boolean => (typeof v === 'boolean' ? v :
  // d)`) verbatim. A static method (not a local const like the file's own `str` helper) because
  // CONFIG.debug is set early in the constructor — before `str` is declared — so a same-scope local
  // const would be a temporal-dead-zone reference; a static method has the same zero-ceremony call
  // shape and matches this file's own `Configs.parseInvariants` precedent. Strict-typed: a non-boolean
  // truthy value (e.g. the string "false") never accidentally flips the flag.
          static bool(v         , d         )          {
    return typeof v === 'boolean' ? v : d;
  }
}

const CONFIG = new Configs(args);
// ╔══ module: src/agents/anomalyJudge/index.ts ════════════════════════════
// ANOMALY JUDGE — one call per chunk-of-6 same-rule ledger anomalies (source lines 364-368, 608-620).
// Its JUDGE schema is reused verbatim (same object reference) by the secondOpinion seat.


                                                                            

const JUDGE         = {
  type: 'object',
  properties: {
    verdicts: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          anomalyId: { type: 'string' },
          verdict: { type: 'string', enum: ['CONFIRMED', 'FALSE', 'UNPROVEN'] },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] },
          what: { type: 'string' },
          location: { type: 'string' },
          fix: { type: 'string' },
          why: { type: 'string' },
        },
        required: ['anomalyId', 'verdict', 'severity', 'what'],
      },
    },
  },
  required: ['verdicts'],
};

const anomalyJudge                          = {
  tier: CONFIG.TIER.anomalyJudge,
  effort: CONFIG.EFFORT.anomalyJudge,
  schema: JUDGE,
  buildPrompt: buildAnomalyJudge,
};
// ╔══ module: src/runtime.ts ══════════════════════════════════════════════

                                                             

// ─────────────────────────────────────────────────────────────────────────────
// retryAgent() — the shared sub-agent caller (rr calls its analog `retryAgent`; this one is the
// source's `resilient()` lifted VERBATIM/behavior-identical, wave-walker.js lines 385-395 — NOT rr's
// own N-uniform-retry semantics). Exactly ONE respawn on a dead agent (agent() resolving null — a
// terminal API error / safety-classifier block, never a throw): the retry prompt is prefixed
// '[label-retry] RESUME: …' and rides on `escalateModel` when the caller passed one, else the SAME
// model. Every call (first attempt and retry) is prefixed '[label] ' — the token ledger's snippet
// fallback attributes workflow spend per stage. Lives in its own module (bundled before every agent's
// run.ts) so each run fn imports retryAgent without a cycle back through engine.ts.
// ─────────────────────────────────────────────────────────────────────────────

// WALK TELEMETRY (DEBUG STEP) — a lightweight per-seat call tally, mirroring rr's IO_LOG/LOG_BUFFER
// gating pattern (rr engine/src/runtime.ts:14-16, 23,43,63: every increment `if (CONFIG.debug)`) but
// COUNTS ONLY — never the full prompt/output capture rr's IO_LOG does (that forensic layer already
// exists one hop down, in the harness's own journal.jsonl; see tmp/walker-debug-design.md §0/§3). A
// module-level singleton: the bundle concatenates every module into one flat scope (build.js), so this
// is the SAME object every seat's retryAgent call mutates — exactly like rr's own module-level buffers.
const SEAT_TALLY                            = {};

// seatKey — derives the tally key as the label's prefix before the FIRST ' · ' or '#', whichever comes
// first (tmp/walker-debug-design.md §4.1). Needs zero changes to any agents/*/run.ts file — every
// seat's `label` already carries this shape verbatim (e.g. 'walk · t1', 'judge · R3#2',
// '2nd-opinion#1', 'scout'). A '-retry' suffix (added by the retry branch below) always lands AFTER
// the first delimiter, so both attempts of a retried call tally under the SAME key.
function seatKey(label        )         {
  const dot = label.indexOf(' · ');
  const hash = label.indexOf('#');
  const idx = [dot, hash].filter((i) => i >= 0).sort((a, b) => a - b)[0];
  return idx === undefined ? label : label.slice(0, idx);
}

function recordSeat(label        , patch                    )       {
  const key = seatKey(label);
  const t = (SEAT_TALLY[key] = SEAT_TALLY[key] || { calls: 0, diedFirstAttempt: 0, retried: 0, diedAfterRetry: 0 });
  t.calls += patch.calls || 0;
  t.diedFirstAttempt += patch.diedFirstAttempt || 0;
  t.retried += patch.retried || 0;
  t.diedAfterRetry += patch.diedAfterRetry || 0;
}

async function retryAgent   (
  prompt        ,
  opts           ,
  escalateModel         ,
)                    {
  const label = opts.label || 'agent';
  let r = (await agent('[' + label + '] ' + prompt, opts))            ;
  if (CONFIG.debug) recordSeat(label, { calls: 1, diedFirstAttempt: r === null ? 1 : 0 });
  if (r === null) {
    const retryModel = escalateModel || opts.model;
    log('⚠ ' + label + ' died · respawning once on ' + retryModel);
    if (CONFIG.debug) recordSeat(label, { retried: 1 });
    r = (await agent(
      '[' +
        label +
        '-retry] RESUME: a prior agent for this exact role died mid-task (often on structured-output). Redo from scratch — idempotent. Keep output values SHORT and schema-exact. ' +
        prompt,
      { ...opts, model: retryModel                      , label: label + '-retry' },
    ))            ;
    if (CONFIG.debug) recordSeat(label, { diedAfterRetry: r === null ? 1 : 0 });
  }
  return r;
}
// ╔══ module: src/agents/anomalyJudge/run.ts ══════════════════════════════
// runAnomalyJudge — one call per chunk-of-6 same-rule anomalies (source lines 607-620). `chunkIndex` is
// the chunk's position within its rule group — label carries a `#N` suffix from the SECOND chunk on,
// exactly like the source's `job.i ? '#' + (job.i + 1) : ''`.


                                                                       

function runAnomalyJudge(args                  , chunkIndex        )                           {
  return retryAgent          (anomalyJudge.buildPrompt(args), {
    label: 'judge · ' + args.rule + (chunkIndex ? '#' + (chunkIndex + 1) : ''),
    phase: 'Judge',
    model: anomalyJudge.tier,
    effort: anomalyJudge.effort,
    schema: anomalyJudge.schema,
  });
}
// ╔══ module: src/agents/brainer/prompts.ts ═══════════════════════════════
// brainer prompt — byte-identical to the source's inline construction (wave-walker.js lines 237-241).
                                                        

const buildBrainer = ({
  goal,
  scopeLine,
  wave,
  maxWaves,
  ledgerRows,
  openLeads,
  maxLanes,
}             )         =>
  'You are the BRAINER — this investigation\'s only global reasoner. GOAL: ' + goal + '.' + scopeLine + ' Wave ' + wave + '/' + maxWaves + '.\n'
  + 'LEDGER (statuses are COMPUTED from topology — cite ids, never assert status): ' + JSON.stringify(ledgerRows) + '\n'
  + 'OPEN LEADS: ' + JSON.stringify(openLeads) + '\n'
  + 'Return your COORD: resultSoFar + keyClaimIds (the load-bearing ids — confidence is computed over exactly these); lanes ≤' + maxLanes + ' (pursue|attack; settled REQUIRES a survived challenge, so attack your own emerging answer — an attack lane names targets); dropLeads (dead leads); stop {done, reason} — done ONLY when the goal is answered on settled key claims or further probing cannot change the answer.'
  + ' Structured output: resultSoFar, keyClaimIds, lanes, dropLeads, stop.';
// ╔══ module: src/agents/brainer/index.ts ═════════════════════════════════
// BRAINER — the investigation's only global reasoner: steers pursue/attack lanes over the computed
// claim ledger (source lines 163-172, 236-243). Frontier-judgment seat — default opus, warns loudly on
// any downgrade (config.ts's FRONTIER_SEATS).


                                                                       

const COORD         = {
  type: 'object',
  properties: {
    resultSoFar: { type: 'string', description: 'best current answer, <=1200 chars' },
    keyClaimIds: {
      type: 'array',
      items: { type: 'string' },
      description: 'the LOAD-BEARING ledger ids the answer rests on',
    },
    lanes: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          kind: { type: 'string', description: 'pursue | attack' },
          question: { type: 'string' },
          files: { type: 'array', items: { type: 'string' } },
          targets: {
            type: 'array',
            items: { type: 'string' },
            description: 'attack only: claim ids to challenge',
          },
          note: { type: 'string' },
        },
        required: ['id', 'kind', 'question'],
      },
    },
    dropLeads: { type: 'array', items: { type: 'string' } },
    stop: {
      type: 'object',
      properties: { done: { type: 'boolean' }, reason: { type: 'string' } },
      required: ['done'],
    },
  },
  required: ['resultSoFar', 'keyClaimIds', 'lanes', 'stop'],
};

const brainer                     = {
  tier: CONFIG.TIER.brainer,
  effort: CONFIG.EFFORT.brainer,
  schema: COORD,
  buildPrompt: buildBrainer,
};
// ╔══ module: src/agents/brainer/run.ts ═══════════════════════════════════
// runBrainer — the ONE brainer call for one wave (source lines 236-243).


                                                                  

function runBrainer(args             )                           {
  return retryAgent          (brainer.buildPrompt(args), {
    label: 'brainer · w' + args.wave,
    phase: 'Investigate',
    model: brainer.tier,
    effort: brainer.effort,
    schema: brainer.schema,
  });
}
// ╔══ module: src/agents/claimAuditor/prompts.ts ══════════════════════════
// claimAuditor prompt — byte-identical to the source's inline construction (wave-walker.js line 212).

                                                             

const buildClaimAuditor = ({ rows }                  )         =>
  'You are a CLAIM AUDITOR — you are grepping for a pin, not judging truth. For EACH claim id, open/grep the cited anchor file(s) (repo root {REPO_ROOT}) and verify the VERBATIM quote appears (whitespace-insensitive; within ±5 lines of a cited line number is fine). pass = every quote found; fail = any quote absent. Claims: ' + JSON.stringify(rows) + RO + ' Structured output: audits (one per claim id).';
// ╔══ module: src/agents/claimAuditor/index.ts ════════════════════════════
// CLAIM AUDITOR (investigate mode) — mechanically greps every quote-pin the ledger carries; a bounded,
// per-wave batch job over pending claims (source lines 172, 209-216).


                                                                            

const AUDITS         = {
  type: 'object',
  properties: {
    audits: {
      type: 'array',
      items: {
        type: 'object',
        properties: { id: { type: 'string' }, result: { type: 'string', enum: ['pass', 'fail'] } },
        required: ['id', 'result'],
      },
    },
  },
  required: ['audits'],
};

const claimAuditor                          = {
  tier: CONFIG.TIER.claimAuditor,
  effort: CONFIG.EFFORT.claimAuditor,
  schema: AUDITS,
  buildPrompt: buildClaimAuditor,
};
// ╔══ module: src/agents/claimAuditor/run.ts ══════════════════════════════
// runClaimAuditor — the ONE audit call for a wave's pending claims (source lines 212-213). Pure
// request/response: filtering to `audit === 'pending'` rows and writing verdicts back onto the ledger
// are owned by engine.ts.


                                                                        

function runClaimAuditor(args                  )                            {
  return retryAgent           (claimAuditor.buildPrompt(args), {
    label: 'audit · w' + args.wave,
    phase: 'Investigate',
    model: claimAuditor.tier,
    effort: claimAuditor.effort,
    schema: claimAuditor.schema,
  });
}
// ╔══ module: src/agents/claimExtractor/prompts.ts ════════════════════════
// claimExtractor prompt — the source's inline construction (wave-walker.js lines 74-77) plus the E2
// breadth-first clause ("Target ~4-6 claims per task, covering EVERY task — breadth across tasks before
// depth within any task."), the port's ONE intended extractor-prompt change (proven: full 16/16-task
// coverage on wave.md vs depth-first's 12 dropped tasks under the old 24-claim cap).

                                                               

const buildClaimExtractor = ({ manifestPath }                    )         =>
  'You are the CLAIM EXTRACTOR of a manifest-verify panel. Read the wave manifest at ' + manifestPath + ' (repo root {REPO_ROOT}) and mine EVERY load-bearing factual claim a hallucination could hide in — a claim is load-bearing when refuting it would change a task\'s design or scope. Per task extract: existence claims (a named file/symbol/field/column/enum/env var/prompt the task assumes EXISTS or assumes ABSENT — incl. every Named anchor and File-plan path), behavior premises ("X currently does Y" statements the design rests on — the classic hallucination class), contract claims (SDL/SQS/WS shapes vs live code), dep claims (cross-task Depends and shared symbols). Each: id T{n}-C{k}, taskId, kind, a SELF-CONTAINED refutable statement, exact files to start probing, 1-line context. ORDER MOST LOAD-BEARING FIRST (a cap may drop the tail). Target ~4-6 claims per task, covering EVERY task — breadth across tasks before depth within any task. Also emit conflictChecks: task pairs/sets whose File plans, Contracts, or data models might collide (same file EDIT+DELETE, one field two shapes, duplicated work) — checks only, no verdicts.' + RO
  + ' Structured output: claims, conflictChecks.';
// ╔══ module: src/agents/claimExtractor/index.ts ══════════════════════════
// CLAIM EXTRACTOR — mines a wave manifest's load-bearing claims for the manifest-verify panel
// (source lines 68-77).


                                                                              

const EXTRACT         = {
  type: 'object',
  properties: {
    claims: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          taskId: { type: 'string' },
          kind: { type: 'string', description: 'existence | behavior | contract | dep' },
          statement: { type: 'string', description: 'self-contained, refutable, <=200 chars' },
          files: { type: 'array', items: { type: 'string' } },
          context: { type: 'string' },
        },
        required: ['id', 'statement'],
      },
    },
    conflictChecks: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          tasks: { type: 'array', items: { type: 'string' } },
          what: { type: 'string' },
        },
        required: ['id', 'what'],
      },
    },
  },
  required: ['claims', 'conflictChecks'],
};

const claimExtractor                            = {
  tier: CONFIG.TIER.claimExtractor,
  effort: CONFIG.EFFORT.claimExtractor,
  schema: EXTRACT,
  buildPrompt: buildClaimExtractor,
};
// ╔══ module: src/agents/claimExtractor/run.ts ════════════════════════════
// runClaimExtractor — the ONE claim-extractor call (source line 74-78). Pure request/response: the
// MAX_CLAIMS cap + fallback DONE-with-zero-claims short-circuit are owned by engine.ts.


                                                               
                                                       

function runClaimExtractor(args                    )                             {
  return retryAgent            (claimExtractor.buildPrompt(args), {
    label: 'extract-claims',
    phase: 'Verify',
    model: claimExtractor.tier,
    effort: claimExtractor.effort,
    schema: claimExtractor.schema,
  });
}
// ╔══ module: src/agents/claimVerifier/prompts.ts ═════════════════════════
// claimVerifier prompts — buildClaimVerifier is byte-identical to the source's inline construction
// (wave-walker.js lines 97-103) and carries the SOLO path (panel ≤ SOLO_THRESHOLD, byte-identical to
// the source's schedule). buildClaimVerifierBatch is the E2 batch prompt (panel > SOLO_THRESHOLD, ≤4
// file-clustered claims per call) — adapted from the proven reference variant's batchPrompt, minus its
// haiku tiering.

                                                                       

const buildClaimVerifier = ({ claim: c, question }                   )         =>
  'You are an INDEPENDENT VERIFIER on a pre-ruling claims panel' + (question ? ' grounding this ruling: ' + question : '') + '. Repo root: {REPO_ROOT}.\n'
  + 'CLAIM ' + c.id + ': ' + c.statement + '\n'
  + (c.context ? 'Context: ' + c.context + '\n' : '')
  + ((c.files || []).length ? 'Start from these files (follow imports/greps wherever the evidence leads): ' + JSON.stringify(c.files) + '\n' : '')
  + 'Actively try to REFUTE the claim — hunt for the counterexample before accepting confirmation. CONFIRMED only when file evidence proves it AS STATED; REFUTED when evidence contradicts it; PARTIAL when it holds with a material caveat (state it); UNPROVEN when evidence is unfindable. '
  + 'Every evidence anchor grep-verified file:line with a VERBATIM quote (<=120 chars). Judge evidence, not vibes.' + RO
  + ' Structured output: claimId=' + c.id + ', verdict, evidence, reasoning (<=3 sentences).';

const buildClaimVerifierBatch = (batch           , question        )         =>
  'You are an INDEPENDENT VERIFIER on a pre-ruling claims panel' + (question ? ' grounding this ruling: ' + question : '') + '. Repo root: {REPO_ROOT}.\n'
  + 'You are verifying a BATCH of ' + batch.length + ' independent claims — judge EACH one separately, refute-first (actively hunt for the counterexample before accepting confirmation), and return one verdict per claim.\n'
  + batch.map((c) => 'CLAIM ' + c.id + ': ' + c.statement + (c.context ? ' | Context: ' + c.context : '') + ((c.files || []).length ? ' | Files: ' + JSON.stringify(c.files) : '')).join('\n') + '\n'
  + 'CONFIRMED only when file evidence proves it AS STATED; REFUTED when evidence contradicts it; PARTIAL when it holds with a material caveat (state it); UNPROVEN when evidence is unfindable. '
  + 'Every evidence anchor grep-verified file:line with a VERBATIM quote (<=120 chars). Judge evidence, not vibes.' + RO
  + ' Structured output: verdicts — one item per claim above (claimId matching, verdict, evidence, reasoning <=3 sentences).';
// ╔══ module: src/agents/claimVerifier/index.ts ═══════════════════════════
// CLAIM VERIFIER — one independent read-only verifier per claim × vote on the pre-ruling claims panel
// (source lines 86-105).


                                                                             

const VERIFY         = {
  type: 'object',
  properties: {
    claimId: { type: 'string' },
    verdict: { type: 'string', enum: ['CONFIRMED', 'REFUTED', 'PARTIAL', 'UNPROVEN'] },
    evidence: {
      type: 'array',
      items: {
        type: 'object',
        properties: { anchor: { type: 'string' }, quote: { type: 'string', description: 'VERBATIM, <=120 chars' } },
        required: ['anchor'],
      },
    },
    reasoning: { type: 'string' },
  },
  required: ['claimId', 'verdict', 'reasoning'],
};

// E2 batch schema — the VERIFY item shape wrapped in an array, one verdict per batched claim (used
// only when the panel exceeds CONFIG.SOLO_THRESHOLD; the solo path keeps VERIFY verbatim).
const VERIFY_BATCH         = {
  type: 'object',
  properties: { verdicts: { type: 'array', items: VERIFY } },
  required: ['verdicts'],
};

const claimVerifier                           = {
  tier: CONFIG.TIER.claimVerifier,
  effort: CONFIG.EFFORT.claimVerifier,
  schema: VERIFY,
  buildPrompt: buildClaimVerifier,
};
// ╔══ module: src/agents/claimVerifier/run.ts ═════════════════════════════
// runClaimVerifier — one SOLO verifier call for one (claim, vote) pair (source lines 94-105).
// `claim.opus` escalates the model to CONFIG.TIER.secondOpinion (source's OPUS_CLAIM_MODEL =
// args.securityEscalateModel — the SAME legacy knob secondOpinion reads; see config.ts's per-seat
// comment). This path runs whenever the panel is ≤ CONFIG.SOLO_THRESHOLD — byte-identical to the
// source's schedule.
//
// runClaimVerifierBatch — the E2 batch call (panel > SOLO_THRESHOLD): one verifier over ≤4
// file-clustered claims, returning {verdicts:[...]} (one VERIFY item per claim). The batch STAYS on
// the verifier tier (sonnet/xhigh) — never haiku. Per-claim `claim.opus` escalation cannot survive
// batching (one model per batch) — accepted E2 trade-off, engine.ts notes it in the log.




                                                               

function runClaimVerifier(
  claim         ,
  question        ,
  voteIndex        ,
  votes        ,
)                            {
  return retryAgent           (claimVerifier.buildPrompt({ claim, question }), {
    label: 'verify · ' + claim.id + (votes > 1 ? ' #' + (voteIndex + 1) : ''),
    phase: 'Verify',
    model: claim.opus ? CONFIG.TIER.secondOpinion : claimVerifier.tier,
    effort: claimVerifier.effort,
    schema: claimVerifier.schema,
  });
}

function runClaimVerifierBatch(
  batch           ,
  question        ,
  batchIndex        ,
  voteIndex        ,
  votes        ,
)                                            {
  return retryAgent                           (buildClaimVerifierBatch(batch, question), {
    label: 'verify-batch · b' + (batchIndex + 1) + (votes > 1 ? ' #' + (voteIndex + 1) : ''),
    phase: 'Verify',
    model: claimVerifier.tier,
    effort: claimVerifier.effort,
    schema: VERIFY_BATCH,
  });
}
// ╔══ module: src/agents/consistencyJudge/prompts.ts ══════════════════════
// consistencyJudge prompt — the source's inline construction (wave-walker.js lines 125-126) with the
// E2 payload diet: CONFIRMED claims travel as the consensus map ONLY; full verdict objects ride only
// for non-CONFIRMED claims (REFUTED/PARTIAL/UNPROVEN/NO-VERDICT). Wording per the proven reference
// variant; the ruling instructions are otherwise verbatim from the source.

                                                                 

const buildConsistencyJudge = ({
  manifestPath,
  nonConfirmed,
  consensus,
  conflictChecks,
}                      )         =>
  'You are the MANIFEST CONSISTENCY JUDGE. Re-read the manifest at ' + manifestPath + ', then rule over the panel\'s evidence. Confirmed claims are listed in Consensus only (their full evidence is elsewhere); full verdict detail below covers only non-CONFIRMED claims. Consensus: ' + JSON.stringify(consensus) + '\nNon-CONFIRMED verdicts (full detail): ' + JSON.stringify(nonConfirmed) + '\nConflict checks queued by the extractor: ' + JSON.stringify(conflictChecks) + '\nFind, evidence-based (open files where needed): (1) kind=conflict — cross-task collisions (File plans touching the same symbols incompatibly, contract shapes disagreeing between tasks, Depends order the file plan violates); (2) kind=refuted-premise — every task step resting on a REFUTED/PARTIAL claim, naming the manifest section it invalidates; (3) kind=freeloader — a task/step that does not earn its place (premise gone, work a sibling task duplicates, scope nothing consumes). Each: tasks, what (Expected/Got), evidence (manifest section + code anchor), severity, fix (concrete manifest correction).' + RO
  + ' Structured output: conflicts, summary.';
// ╔══ module: src/agents/consistencyJudge/index.ts ════════════════════════
// CONSISTENCY JUDGE — the manifest consistency judge: cross-task conflicts, refuted premises,
// freeloader tasks (source lines 118-130).


                                                                                

const CONFLICT         = {
  type: 'object',
  properties: {
    conflicts: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          kind: { type: 'string', description: 'conflict | refuted-premise | freeloader' },
          tasks: { type: 'array', items: { type: 'string' } },
          what: { type: 'string' },
          evidence: { type: 'string' },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] },
          fix: { type: 'string', description: 'concrete manifest correction' },
        },
        required: ['id', 'kind', 'what', 'severity'],
      },
    },
    summary: { type: 'string' },
  },
  required: ['conflicts', 'summary'],
};

const consistencyJudge                              = {
  tier: CONFIG.TIER.consistencyJudge,
  effort: CONFIG.EFFORT.consistencyJudge,
  schema: CONFLICT,
  buildPrompt: buildConsistencyJudge,
};
// ╔══ module: src/agents/consistencyJudge/run.ts ══════════════════════════
// runConsistencyJudge — the ONE consistency-judge call (source lines 124-128).


                                                                              

function runConsistencyJudge(args                      )                              {
  return retryAgent             (consistencyJudge.buildPrompt(args), {
    label: 'conflict-judge',
    phase: 'Verify',
    model: consistencyJudge.tier,
    effort: consistencyJudge.effort,
    schema: consistencyJudge.schema,
  });
}
// ╔══ module: src/agents/coverageCritic/prompts.ts ════════════════════════
// coverageCritic prompt — ported from the proven rollout-bug-hunt completeness critic (which produced
// 8 concrete, high-value coverage gaps), per tmp/wave-walker-investigation.md § 2.4. Not a source-fidelity
// port — there is no bundle equivalent.

                                                               

const buildCoverageCritic = ({
  changedFiles,
  threadNames,
  hunterCoverage,
  armedIds,
  unarmedInvariants,
  unsensed,
}                    )         =>
  'You are the COVERAGE CRITIC — this walk\'s EXTERNAL denominator. Every other seat reports its OWN coverage; your job is naming what the WALK ITSELF could not see, from outside it. Never re-litigate a finding — only name territory nothing inspected.\n'
  + 'Changed files: ' + JSON.stringify(changedFiles) + '\n'
  + 'Threads walked this pass: ' + JSON.stringify(threadNames) + '\n'
  + 'Invariant hunters that ran, and their own stated coverage: ' + JSON.stringify(hunterCoverage) + '\n'
  + 'Invariants ARMED this walk: ' + JSON.stringify(armedIds) + '. Registered but NOT armed (their territory, unhunted this walk): ' + JSON.stringify(unarmedInvariants) + '\n'
  + 'Fields the sensor cap dropped (UNSENSED): ' + JSON.stringify(unsensed) + '\n'
  + 'Name AT MOST 8 territories this walk could not see: files in the diff\'s blast radius appearing in no thread/hunter scope, a load-bearing named-skip buried inside a coverage line above, a cross-dimension interaction no single seat could see (two armed invariants whose territories overlap but whose hunters never compared notes), or an unarmed invariant whose territory the diff plausibly touches anyway. An empty enumeration is never a verdict — if you truly find nothing, say so explicitly and name what you actually checked (file list / directories) to reach that conclusion; do not render silence as "clean".' + RO
  + ' Structured output: gaps (each {territory, why}, at most 8), summary.';
// ╔══ module: src/agents/coverageCritic/index.ts ══════════════════════════
// COVERAGE CRITIC — INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.4). The walk's
// EXTERNAL denominator: one Sonnet call after Phase 1, before the final judge, whose only job is naming
// what the walk itself did NOT inspect — territories armed but unhunted, dims skipped, thread coverage
// vs the diff's blast radius. Never re-litigates findings. Dispatched only when the registry is non-empty
// (CONFIG.INVARIANTS.length > 0) — an absent/empty registry means no critic runs at all (the floor).


                                                                              

const COVERAGE_CRITIC         = {
  type: 'object',
  properties: {
    gaps: {
      type: 'array',
      maxItems: 8,
      items: {
        type: 'object',
        properties: {
          territory: { type: 'string' },
          why: { type: 'string' },
        },
        required: ['territory', 'why'],
      },
    },
    summary: { type: 'string' },
  },
  required: ['gaps', 'summary'],
};

const coverageCritic                            = {
  tier: CONFIG.TIER.coverageCritic,
  effort: CONFIG.EFFORT.coverageCritic,
  schema: COVERAGE_CRITIC,
  buildPrompt: buildCoverageCritic,
};
// ╔══ module: src/agents/coverageCritic/run.ts ════════════════════════════
// runCoverageCritic — the one call after Phase 1, before the final judge (INVARIANT REGISTRY FEATURE § 2.4).


                                                                                  

function runCoverageCritic(args                    )                                    {
  return retryAgent                   (coverageCritic.buildPrompt(args), {
    label: 'coverage-critic',
    phase: 'Judge',
    model: coverageCritic.tier,
    effort: coverageCritic.effort,
    schema: coverageCritic.schema,
  });
}
// ╔══ module: src/agents/finalJudge/prompts.ts ════════════════════════════
// finalJudge prompt — byte-identical to the source's inline construction (wave-walker.js lines 673-685). A non-empty charter appends the Professor-authored WALK CHARTER block (zero bytes otherwise). INVARIANT REGISTRY FEATURE (§ 2.4) — a non-empty coverageGaps list appends the critic's named holes to the Coverage line (zero bytes when empty/absent).

                                                           

const buildFinalJudge = ({
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
}                )         =>
  'You are the FINAL JUDGE of this wave walk — one Opus ruling over the WHOLE result before the review is written. Complete inputs: '
  + 'THREAD WALKS: ' + JSON.stringify(walksBrief)
  + ' · CONFIRMED anomalies: ' + JSON.stringify(confirmed)
  + ' · UNPROVEN: ' + JSON.stringify(unproven)
  + ' · KILLED as FALSE (re-examine — a wrong kill hides here): ' + JSON.stringify(killedWithAnomaly)
  + ' · Territory digests: ' + JSON.stringify(digests)
  + ' · SECURITY AUDIT (diff-scoped ' + securityDoc + '): ' + (security ? JSON.stringify(security.findings || []) + ' (swept: ' + (security.categoriesSwept || []).join(',') + ')' : 'AUDIT DIED — a coverage hole')
  + ' · Coverage: threads ' + walksLen + '/' + threadsLen + ', fields sensed ' + cardsLen + ', UNSENSED: ' + (unsensed.length ? unsensed.join(', ') : 'none')
  + ((coverageGaps || []).length ? ', coverage-critic gaps: ' + JSON.stringify(coverageGaps) : '') + '\n'
  + 'Rule the wave: (1) the authoritative verdict on the SMOOTH SAILING | MOSTLY GOOD | ROUGH SEAS | SHIPWRECK scale — weigh broken threads, confirmed severity, security findings, and coverage holes; (2) reinstate any killed anomaly whose kill reasoning does not hold (open the files yourself before reinstating); (3) missedRisks — cross-cutting hazards only the whole picture shows (a pattern repeating across threads, an unsensed-field cluster over clinical surface, digest smells that compound). Judge evidence, not vibes.' + RO
  + ' Structured output: verdict, reinstated, missedRisks, rationale.'
  + (charter ? '\nWALK CHARTER (caller-supplied duty): ' + charter + '\nAnswer the charter explicitly: what the walk found for it and whether its concern is satisfied — inside your existing fields; the verdict scale and schema unchanged.' : '');
// ╔══ module: src/agents/finalJudge/index.ts ══════════════════════════════
// FINAL JUDGE — one Opus ruling over the WHOLE walk before the review is written (source lines
// 664-685). FRONTIER-JUDGMENT SEAT — never silently downgrade below opus.


                                                                          

const FINAL         = {
  type: 'object',
  properties: {
    verdict: { type: 'string', enum: ['SMOOTH SAILING', 'MOSTLY GOOD', 'ROUGH SEAS', 'SHIPWRECK'] },
    reinstated: {
      type: 'array',
      items: {
        type: 'object',
        properties: { anomalyId: { type: 'string' }, why: { type: 'string' } },
        required: ['anomalyId', 'why'],
      },
    },
    missedRisks: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          what: { type: 'string' },
          where: { type: 'string' },
          severity: { type: 'string' },
          jc: { type: 'string' },
        },
        required: ['what', 'where'],
      },
    },
    rationale: { type: 'string' },
  },
  required: ['verdict', 'missedRisks'],
};

const finalJudge                        = {
  tier: CONFIG.TIER.finalJudge,
  effort: CONFIG.EFFORT.finalJudge,
  schema: FINAL,
  buildPrompt: buildFinalJudge,
};
// ╔══ module: src/agents/finalJudge/run.ts ════════════════════════════════
// runFinalJudge — the one ruling over the whole walk (source lines 673-685).


                                                                     

function runFinalJudge(args                )                           {
  return retryAgent          (finalJudge.buildPrompt(args), {
    label: 'final-judge',
    phase: 'Fold',
    model: finalJudge.tier,
    effort: finalJudge.effort,
    schema: finalJudge.schema,
  });
}
// ╔══ module: src/agents/fold/prompts.ts ══════════════════════════════════
// fold prompt — byte-identical to the source's inline construction (wave-walker.js lines 700-712). INVARIANT
// REGISTRY FEATURE (§ 2.4) — a non-empty coverageGaps list extends the honesty sentence (zero bytes when
// empty/absent).
                                                     

const buildFold = ({
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
}          )         =>
  'You are the FOLD of a wave-walker review. Merge the two walks into ONE review and WRITE it into the report at ' + reportPath + ' under a `## Professor\'s Wave Review` section (create/overwrite ONLY that section of that file; run no git).\n'
  + 'Inputs:\n· THREAD WALKS (functional flow + hygiene, the floor): ' + JSON.stringify(walks) + '\n'
  + '· LEDGER anomalies CONFIRMED (mechanical, file-verified by judges): ' + JSON.stringify(confirmed) + '\n· Ledger UNPROVEN (needs human eyes): ' + JSON.stringify(unproven) + '\n· Ledger killed-as-false: ' + killedCount + ' (one line)\n'
  + '· Territory digests: ' + JSON.stringify(digests) + '\n· SECURITY AUDIT (diff-scoped): ' + (security ? JSON.stringify({ findings: security.findings || [], categoriesSwept: security.categoriesSwept, summary: security.summary }) : 'AUDIT DIED — name it in Coverage as an explicit hole') + '\n· Coverage: ' + coverageSummary + '\n'
  + (finalJudge ? '· FINAL JUDGMENT (authoritative): verdict=' + finalJudge.verdict + ' · missedRisks: ' + JSON.stringify(finalJudge.missedRisks) + ' · rationale: ' + (finalJudge.rationale || '') + '\n' : '')
  + 'Fold rules: every functional defect (thread) AND every confirmed ledger anomaly AND every digest fix AND every security finding becomes a `### /jc Action Items` line (deduped — a thread defect and a ledger anomaly at the same anchor are ONE item). '
  + (finalJudge ? 'ADOPT the FINAL JUDGMENT verdict verbatim; fold each missedRisk into the review (fixable → an action item, else Unproven/needs-eyes). ' : '')
  + 'The verdict weighs BOTH: a broken thread flow OR a confirmed critical/high ledger anomaly sinks it. HONESTY: the Coverage note MUST name every UNSENSED field as a hole' + ((coverageGaps || []).length ? ', AND every coverage-critic gap (' + JSON.stringify(coverageGaps) + ') as a named hole' : '') + '.\n'
  + (telemetryMd ? '\nAlso append this VERBATIM as a `### Walk Telemetry` subsection at the end of the review, after Coverage — do not summarize, edit, or judge it, just copy it in:\n' + telemetryMd + '\n' : '')
  + 'Report format (per wave/walker.md § Report Format): ## Professor\'s Wave Review (Wave · Date · Verdict); Executive Summary; Thread Walk table; Ledger Anomalies by rule (Expected/Got + anchors + severity); Territory Digests; Security Audit (per-category Expected/Got, or None); ### /jc Action Items; Coverage.\n'
  + 'Verdict: SMOOTH SAILING (nothing) | MOSTLY GOOD (minor only) | ROUGH SEAS (a confirmed high or a BROKEN thread) | SHIPWRECK (a confirmed critical / security, or multiple broken flows).'
  + ' Structured output: verdict, actionItems (verbatim /jc lines), review (the full markdown you wrote).';
// ╔══ module: src/agents/fold/index.ts ════════════════════════════════════
// FOLD — merges thread walks + confirmed anomalies + digests + the final judgment into the wave
// review, written into the report (source lines 376-382, 696-712).


                                                                    

const FOLD         = {
  type: 'object',
  properties: {
    verdict: { type: 'string', enum: ['SMOOTH SAILING', 'MOSTLY GOOD', 'ROUGH SEAS', 'SHIPWRECK'] },
    actionItems: { type: 'array', items: { type: 'string' } },
    review: { type: 'string' },
  },
  required: ['verdict', 'actionItems', 'review'],
};

const fold                  = {
  tier: CONFIG.TIER.fold,
  effort: CONFIG.EFFORT.fold,
  schema: FOLD,
  buildPrompt: buildFold,
};
// ╔══ module: src/agents/fold/run.ts ══════════════════════════════════════
// runFold — the one call merging everything into the report (source lines 700-712).


                                                              

function runFold(args          )                          {
  return retryAgent         (fold.buildPrompt(args), {
    label: 'fold',
    phase: 'Fold',
    model: fold.tier,
    effort: fold.effort,
    schema: fold.schema,
  });
}
// ╔══ module: src/agents/gateSweep/prompts.ts ═════════════════════════════
// gateSweep prompt — byte-identical to the source's inline construction (wave-walker.js lines 469-478).

                                                          

const buildGateSweep = ({ file }               )         =>
  'You are a PURE EXTRACTOR (gate sweep). NO judgment. Open the resolver file ' + file + ' and extract ONE gate card per GraphQL entry point in it.\n'
  + 'Per entry point: id ("Query.opName"/"Mutation.opName"), kind, resource class (session|patient|clinic|couple|user|other), anchor, idArgs (client-supplied ID args), '
  + 'chain (IN ORDER, every guard call between entry and first data access; open custom helpers and note what they fence), rolesAllowed (EXPAND role-set constants), '
  + 'clinicFence (boolean), ownershipFence (boolean: record-owner check enforced). Keep strings SHORT.' + RO
  + ' Structured output: file, gates.';
// ╔══ module: src/agents/gateSweep/index.ts ═══════════════════════════════
// GATE SWEEP — one call per resolver file: a PURE EXTRACTOR emitting one gate card per GraphQL entry
// point in it (source lines 358-363, 469-478). Escalates to CONFIG.SENSOR_ESCALATE (see run.ts).


                                                                         

const GATE_SWEEP         = {
  type: 'object',
  properties: {
    file: { type: 'string' },
    gates: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          kind: { type: 'string' },
          resource: { type: 'string' },
          anchor: { type: 'string' },
          idArgs: { type: 'array', items: { type: 'string' } },
          rolesAllowed: { type: 'array', items: { type: 'string' } },
          chain: { type: 'array', items: { type: 'string' } },
          clinicFence: { type: 'boolean' },
          ownershipFence: { type: 'boolean' },
          notes: { type: 'string' },
        },
        required: ['id', 'anchor'],
      },
    },
  },
  required: ['file', 'gates'],
};

const gateSweep                       = {
  tier: CONFIG.TIER.gateSweep,
  effort: CONFIG.EFFORT.gateSweep,
  schema: GATE_SWEEP,
  buildPrompt: buildGateSweep,
};
// ╔══ module: src/agents/gateSweep/run.ts ═════════════════════════════════
// runGateSweep — one call per gate file (source lines 469-478). Escalates to CONFIG.SENSOR_ESCALATE
// on a dead respawn, same as sliceSensor.



                                                                        

function runGateSweep(args               )                               {
  return retryAgent              (
    gateSweep.buildPrompt(args),
    {
      label: 'gates · ' + (args.file.split('/').pop() || args.file),
      phase: 'Walk',
      model: gateSweep.tier,
      effort: gateSweep.effort,
      schema: gateSweep.schema,
    },
    CONFIG.SENSOR_ESCALATE,
  );
}
// ╔══ module: src/agents/invariantHunter/prompts.ts ═══════════════════════
// invariantHunter prompt — ported near-verbatim from the proven rollout-bug-hunt finder prompts (which
// produced 53 confirmed findings), per tmp/wave-walker-investigation.md § 2.2. Not a source-fidelity
// port — there is no bundle equivalent; this is the new piece.

                                                                

const buildInvariantHunter = ({ invariant, matchedFiles }                     )         =>
  'You are an INVARIANT HUNTER — an adversarial finder, not a confirmer. Your only question is: does anything in this TERRITORY violate the LAW below, right now? REFUTE-FIRST: hunt for a violation before you accept that the code is clean; do not conclude "looks fine" from a skim.\n'
  + 'LAW (' + invariant.id + '): ' + invariant.law + '\n'
  + 'TERRITORY (walk this — the whole territory, not just the files below; matchedFiles is only where the diff first tripped the trigger): ' + JSON.stringify(invariant.territory) + '. Files the diff touched in this territory: ' + JSON.stringify(matchedFiles) + '.\n'
  + (invariant.triggers.length ? 'DIFF TRIGGERS that armed this hunt: ' + JSON.stringify(invariant.triggers) + '\n' : '')
  + (invariant.exemplars.length ? 'CLASS EXEMPLARS (already-confirmed bugs of exactly this shape — use them to calibrate what you are hunting for, not as an exhaustive list): ' + JSON.stringify(invariant.exemplars) + '\n' : '')
  + 'HUNT BRIEF: ' + invariant.huntBrief + '\n'
  + 'METHOD: WALK the code — open files, trace call paths end-to-end, read every branch. Never conclude from grep hits alone. For EVERY finding: file, line, Expected vs Got, and a CONCRETE failure scenario (inputs/state -> wrong outcome -> who is harmed) — a finding with no failure scenario is not a finding, it is a hunch; drop it. Verify by ENUMERATION — walk every instance of the class the hunt brief names, not a sample. Your coverage line MUST name what you walked AND what you skipped; an empty enumeration ("nothing found") is never a verdict unless you name what you inspected to reach it.' + RO
  + ' Structured output: invariantId="' + invariant.id + '", findings (each {what, location=file:line, expected, got, failureScenario, severity, fix=`/jc {fix}`}), coverage.';
// ╔══ module: src/agents/invariantHunter/index.ts ═════════════════════════
// INVARIANT HUNTER — INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.2). One call per
// ARMED registry entry, territory-scoped (registry globs, NOT just the diff — this is what catches bucket
// C: pre-existing code no wave diff ever touches). Adversarial, REFUTE-FIRST: hunt for violations of the
// invariant's law, never confirm the happy path. Findings ride the existing judge path as rule class
// R9-INV (rules.ts computeInvariantAnomalies), never adjudicated by this seat itself.


                                                                               

const INVARIANT_HUNT         = {
  type: 'object',
  properties: {
    invariantId: { type: 'string' },
    findings: {
      type: 'array',
      maxItems: 12,
      items: {
        type: 'object',
        properties: {
          what: { type: 'string' },
          location: { type: 'string' },
          expected: { type: 'string' },
          got: { type: 'string' },
          failureScenario: { type: 'string', description: 'concrete: inputs/state -> wrong outcome -> who is harmed. REQUIRED, never a vibe.' },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] },
          fix: { type: 'string' },
        },
        required: ['what', 'location', 'expected', 'got', 'failureScenario', 'severity'],
      },
    },
    coverage: { type: 'string', description: 'what you walked AND what you skipped — an empty enumeration is never a verdict' },
  },
  required: ['invariantId', 'findings', 'coverage'],
};

const invariantHunter                             = {
  tier: CONFIG.TIER.invariantHunter,
  effort: CONFIG.EFFORT.invariantHunter,
  schema: INVARIANT_HUNT,
  buildPrompt: buildInvariantHunter,
};
// ╔══ module: src/agents/invariantHunter/run.ts ═══════════════════════════
// runInvariantHunter — one call per ARMED registry entry (INVARIANT REGISTRY FEATURE § 2.2), dispatched
// inside the existing Phase-1 barrier alongside threadWalker/sliceSensor/gateSweep/securityAuditor.


                                                                                    

function runInvariantHunter(args                     )                                     {
  return retryAgent                    (invariantHunter.buildPrompt(args), {
    label: 'invariant-hunt · ' + args.invariant.id,
    phase: 'Walk',
    model: invariantHunter.tier,
    effort: invariantHunter.effort,
    schema: invariantHunter.schema,
  });
}
// ╔══ module: src/agents/probe/prompts.ts ═════════════════════════════════
// probe prompt — byte-identical to the source's inline construction (wave-walker.js lines 219-223).

                                                      

const buildProbe = ({ lane, goal, scopeLine }           )         =>
  'You are lane ' + lane.id + ' (' + (lane.kind || 'pursue') + ') of a code investigation. GOAL: ' + goal + '.' + scopeLine + '\nQUESTION: ' + lane.question + (lane.note ? ' — steering: ' + lane.note : '') + '\n'
  + ((lane.files || []).length ? 'Start files (follow imports/greps wherever evidence leads): ' + JSON.stringify(lane.files) + '\n' : '')
  + (lane.kind === 'attack' ? 'ATTACK LANE: actively hunt COUNTER-evidence against claim ids ' + JSON.stringify(lane.targets || []) + ' (emit kind=counter with targets). A real hunt that finds NOTHING → nothingFound:true — that survival is first-class evidence, not silence.\n' : '')
  + 'Return quote-pinned claims — SELF-CONTAINED facts, VERBATIM quotes (<=120 chars), grep-verified file:line anchors — plus leads (files/symbols worth a future lane).' + RO
  + ' Structured output: laneId=' + lane.id + ', claims, leads, nothingFound.';
// ╔══ module: src/agents/probe/index.ts ═══════════════════════════════════
// PROBE — one investigation lane (pursue or attack) over the quote-pinned claim ledger
// (source lines 155-162, 217-225).


                                                                     

const PROBE         = {
  type: 'object',
  properties: {
    laneId: { type: 'string' },
    claims: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          statement: { type: 'string', description: 'self-contained fact, <=200 chars' },
          kind: { type: 'string', description: 'support | counter' },
          targets: {
            type: 'array',
            items: { type: 'string' },
            description: 'counter only: attacked claim ids',
          },
          anchors: {
            type: 'array',
            items: {
              type: 'object',
              properties: { anchor: { type: 'string' }, quote: { type: 'string', description: 'VERBATIM, <=120 chars' } },
              required: ['anchor', 'quote'],
            },
          },
        },
        required: ['statement', 'anchors'],
      },
    },
    leads: {
      type: 'array',
      items: {
        type: 'object',
        properties: { what: { type: 'string' }, files: { type: 'array', items: { type: 'string' } } },
        required: ['what'],
      },
    },
    nothingFound: { type: 'boolean' },
  },
  required: ['laneId', 'claims', 'leads'],
};

const probe                   = {
  tier: CONFIG.TIER.probe,
  effort: CONFIG.EFFORT.probe,
  schema: PROBE,
  buildPrompt: buildProbe,
};
// ╔══ module: src/agents/probe/run.ts ═════════════════════════════════════
// runProbe — one probe call for one lane (source lines 217-226), post-annotated with `_laneKind`/
// `_targets` exactly as the source's `.then(r => r && Object.assign(r, {...}))` does — ledger.ts's
// `ingest()` reads both off the returned ProbeOut.


                                                           

async function runProbe(lane      , goal        , scopeLine        )                           {
  const r = await retryAgent          (probe.buildPrompt({ lane, goal, scopeLine }), {
    label: 'probe · ' + lane.id,
    phase: 'Investigate',
    model: probe.tier,
    effort: probe.effort,
    schema: probe.schema,
  });
  return r && Object.assign(r, { _laneKind: lane.kind, _targets: lane.targets || [] });
}
// ╔══ module: src/agents/scout/prompts.ts ═════════════════════════════════
// scout prompt — byte-identical to the source's inline construction (wave-walker.js lines 400-415) when charter is '' ; a non-empty charter appends the Professor-authored WALK CHARTER block (zero bytes otherwise). INVARIANT REGISTRY FEATURE (§ 2.1): a non-empty `invariants` list appends step 7 (zero bytes when empty/absent — the floor).

                                                      

const buildScout = ({ reportPath, branch, walkerDoc, maxFieldsPerJob, charter, invariants, invariantsDoc }           )         =>
  'You are the SCOUT-SCHEDULER of a wave-walker review. Read the wave report at ' + reportPath + ' and walk the WAVE\'S DIFF. Repo root: {REPO_ROOT}.\n'
  + (branch
    ? '1) PRE-MERGE BRANCH MODE: the wave is NOT merged yet. changedFiles = `git diff --name-only main...' + branch + '` (three-dot; read file contents from the branch\'s worktree checkout when present, else `git show ' + branch + ':{path}`). mergeShas = []. headSha = `git rev-parse ' + branch + '`. The report carries the wave manifest + slice list for context.\n'
    : '1) From the report — a `**Merge SHA:**` line (the dual-chat wave writes one at MERGE) and/or the Final Summary / Grouping / `## JC Pre-flight` sections: list SUCCEEDED pipeline merge SHAs (mergeShas) and any JC commits. Run `git diff {merge}^1 {merge}` per merge SHA (`git show {sha}` for a JC fix) and union into changedFiles (the integrated changed-and-generated set). headSha = git rev-parse HEAD.\n')
  + '2) THREADS — the functional/hygiene walk manifest (the proven floor). Read ' + walkerDoc + ' § Role: Scout for the thread taxonomy; aim for >= 4, one per feature flow plus a thread for each seam, field, schema change, invariant, test-data-discipline, or dead-code-ripple the diff puts at risk. Emit a Field thread with an explicit READ-BACK check for EVERY new persisted field (writer AND reader mapping). Each: id, type, name, scope, files, verify.\n'
  + '3) LEDGER SCHEDULE (the mechanical spine, only if the diff touches the GraphQL contract surface — else return empty fields/jobs and the thread walk carries the wave):\n'
  + '   · operations — GraphQL operations whose resolver/SDL the diff changed OR whose result type the diff touches: id, kind, resolver anchor, resultType.\n'
  + '   · fields — every field of each touched result type, DEDUPED by (ownerType, field); id="OwnerType.fieldName"; fill each field\'s sdl slice {anchor, typeToken} YOURSELF from the schema. Include a field when the diff changed its producer, its SDL, or any consumer.\n'
  + '   · jobs — cluster fields by FILE LOCALITY into sensor jobs (kind producer|consumer|cortex), each with the EXACT files to read and <= ' + maxFieldsPerJob + ' fieldIds; follow resolver imports / grep the query call-sites NOW so each job\'s file list is exact.\n'
  + '4) gateFiles — EVERY resolver file under {project}/src/infrastructure/graphql/resolvers (repo-wide; fence-outlier detection needs the full population even when the diff is small).\n'
  + '5) territories — which of BE/FE/Cortex the diff touches.\n'
  + '6) authRule — grep {project}/CLAUDE.md for its "Auth Pattern" heading (locate by heading text, NEVER by line number) and return the "Role fences" bullet VERBATIM — the ledger\'s R6 auth-fence rule and the security second-opinion quote it live.' + RO
  + ' Structured output: headSha, territories, changedFiles, mergeShas, threads, operations, fields, jobs, gateFiles, authRule.'
  + (charter ? '\nWALK CHARTER (caller-supplied duty): ' + charter + '\nShape the thread manifest to serve this charter IN ADDITION to the standard enumeration — add charter-driven threads; never drop or merge a standard thread for it.' : '')
  + ((invariants && invariants.length)
    ? '\n7) INVARIANT REGISTRY (' + (invariantsDoc || '') + ') — a durable registry of sacred cross-cutting semantics, seeded by a proven adversarial bug-hunt. For EACH entry below, test its `triggers` against this diff (does the diff touch its territory, add/modify a reuse-skip-cache gate, touch an engine-stamped column, etc. — read the triggers literally). Registry: ' + JSON.stringify(invariants) + '. Return armedInvariants: one entry per registry id whose trigger fires, each {id, matchedFiles (the diff files that armed it), reason}. Arm generously — a missed arm is a missed hunt; when genuinely uncertain, arm it. Do NOT drop or merge a standard thread to make room for this — it is additive.'
    : '');
// ╔══ module: src/agents/scout/index.ts ═══════════════════════════════════
// SCOUT — the scout-scheduler seat: diffs the wave, emits the thread manifest AND the ledger schedule
// in one pass (source lines 332-348, 400-415).


                                                                     

const SCOUT         = {
  type: 'object',
  properties: {
    headSha: { type: 'string' },
    territories: { type: 'array', items: { type: 'string' } },
    changedFiles: { type: 'array', items: { type: 'string' } },
    mergeShas: { type: 'array', items: { type: 'string' } },
    threads: {
      type: 'array',
      description:
        'the functional/hygiene walk manifest — feature flow | seam | field | schema/db | invariant | test-data | dead-code-ripple',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          type: { type: 'string' },
          name: { type: 'string' },
          scope: { type: 'string' },
          files: { type: 'array', items: { type: 'string' } },
          verify: { type: 'string' },
        },
        required: ['id', 'type', 'name', 'verify'],
      },
    },
    operations: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          kind: { type: 'string' },
          anchor: { type: 'string' },
          resultType: { type: 'string' },
        },
        required: ['id', 'anchor'],
      },
    },
    fields: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          ownerType: { type: 'string' },
          field: { type: 'string' },
          apis: { type: 'array', items: { type: 'string' } },
          sdl: {
            type: 'object',
            properties: { anchor: { type: 'string' }, typeToken: { type: 'string' } },
          },
        },
        required: ['id', 'ownerType', 'field'],
      },
    },
    jobs: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          jobId: { type: 'string' },
          kind: { type: 'string', description: 'producer | consumer | cortex' },
          files: { type: 'array', items: { type: 'string' } },
          fieldIds: { type: 'array', items: { type: 'string' } },
          hint: { type: 'string' },
        },
        required: ['jobId', 'kind', 'files', 'fieldIds'],
      },
    },
    gateFiles: {
      type: 'array',
      items: { type: 'string' },
      description: 'EVERY resolver file in {project}, repo-wide — fence-outlier context',
    },
    authRule: {
      type: 'string',
      description: 'VERBATIM Role-fences bullet from {project}/CLAUDE.md § Auth Pattern',
    },
    // INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — optional, absent when no
    // registry was supplied (CONFIG.INVARIANTS empty). engine.ts's computeArmedInvariants unions this
    // with a zero-token territory-glob fail-safe, so a scout omission can never silently disarm a hunt.
    armedInvariants: {
      type: 'array',
      description: 'registry entries whose triggers fired against this diff',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          matchedFiles: { type: 'array', items: { type: 'string' } },
          reason: { type: 'string' },
        },
        required: ['id'],
      },
    },
  },
  required: ['headSha', 'changedFiles', 'threads', 'fields', 'jobs', 'gateFiles'],
};

const scout                   = {
  tier: CONFIG.TIER.scout,
  effort: CONFIG.EFFORT.scout,
  schema: SCOUT,
  buildPrompt: buildScout,
};
// ╔══ module: src/agents/scout/run.ts ═════════════════════════════════════
// runScout — the ONE scout-scheduler call (source lines 400-415). Pure request/response: every mutation
// (threads concat, job splitting/capping, authRule derivation) is owned by engine.ts's runWalk().


                                                                

function runScout(args           )                           {
  return retryAgent          (scout.buildPrompt(args), {
    label: 'scout',
    phase: 'Scout',
    model: scout.tier,
    effort: scout.effort,
    schema: scout.schema,
  });
}
// ╔══ module: src/agents/secondOpinion/prompts.ts ═════════════════════════
// secondOpinion prompt — byte-identical to the source's inline construction (wave-walker.js lines 649-655)
// when `direction` is omitted (the ONLY caller before this feature, and still the default). INVARIANT
// REGISTRY FEATURE (§ 2.3, escalation symmetry) adds the 'confirmed' branch: a first judge CONFIRMED an
// R9-INV high/critical finding — re-examine for a wrongful confirm, symmetric to the existing wrongful-kill
// re-examination. `direction === 'confirmed'` is the only path that changes any byte of the output.

                                                              

const buildSecondOpinion = ({ authRule, items, direction }                   )         =>
  (direction === 'confirmed'
    ? 'You are the SECOND-OPINION judge (a first judge CONFIRMED these at high/critical severity from an adversarial invariant-hunter finding — this is the last gate before the founder sees it). '
    : 'You are the SECOND-OPINION judge (a first judge killed these as FALSE, but the rule\'s evidence is regex/string-exact or a documented security invariant). ')
  + authRule + '\n'
  + 'For each: open the file(s) yourself, re-derive from scratch, rule independently. '
  + (direction === 'confirmed'
    ? 'Be suspicious of a confirm with no reproducible failure scenario, or where a guard/compensating control the finder missed already defeats it — override to FALSE when the confirm does not hold.\n'
    : 'Be suspicious of a kill that contradicts the verbatim extracted expression (a JSON.parse(JSON.stringify(...)) still present, a literal that truly never matches the produced set).\n')
  + (direction === 'confirmed' ? 'Confirmed verdicts with anomalies: ' : 'Killed verdicts with anomalies: ') + JSON.stringify(items) + RO
  + ' Structured output: verdicts (one per anomalyId).';
// ╔══ module: src/agents/secondOpinion/index.ts ═══════════════════════════
// SECOND OPINION — Opus re-examines killed security/near-certain verdicts (source lines 646-658).
// FRONTIER-JUDGMENT SEAT — never silently downgrade below opus (root CLAUDE.md § Model Selection).
// Reuses anomalyJudge's JUDGE schema BY REFERENCE (source calls the same JS `JUDGE` constant for both).



                                                                     

const secondOpinion                           = {
  tier: CONFIG.TIER.secondOpinion,
  effort: CONFIG.EFFORT.secondOpinion,
  schema: JUDGE,
  buildPrompt: buildSecondOpinion,
};
// ╔══ module: src/agents/secondOpinion/run.ts ═════════════════════════════
// runSecondOpinion — one call per chunk-of-4 escalatable killed verdicts (source lines 649-655).
// `chunkIndex` is 0-based; the label suffix is `#(chunkIndex+1)` on EVERY chunk (unlike anomalyJudge,
// which omits the suffix on chunk 0) — matches the source's `'2nd-opinion#' + (i + 1)` exactly.


                                                                        

function runSecondOpinion(args                   , chunkIndex        )                           {
  return retryAgent          (secondOpinion.buildPrompt(args), {
    label: '2nd-opinion#' + (chunkIndex + 1),
    phase: 'Judge',
    model: secondOpinion.tier,
    effort: secondOpinion.effort,
    schema: secondOpinion.schema,
  });
}
// ╔══ module: src/agents/securityAuditor/prompts.ts ═══════════════════════
// securityAuditor prompt — byte-identical to the source's inline construction (wave-walker.js lines 486-492).

                                                                

const buildSecurityAuditor = ({ securityDoc, changedFiles, branch, mergeShas }                     )         =>
  'You are the WAVE SECURITY AUDITOR. Read ' + securityDoc + ' and apply its FULL category set (8A–8K + Method & Severity) SCOPED TO THIS WAVE\'S DIFF — the changed files plus every security-relevant surface they touch (follow a changed symbol into its callers/config when the risk crosses the file boundary). Changed files: ' + JSON.stringify(changedFiles) + '. ' + (branch ? 'Diff: main...' + branch + '.' : 'Merge SHAs: ' + JSON.stringify(mergeShas || []) + '.') + ' Therapy data is sacred — PHI (8F), auth (8C), GraphQL (8D), LLM/prompt (8E) get the deepest pass. Report ONLY defects the diff introduced or worsened; a pre-existing issue you trip over goes into summary as one line (category + location), never a finding. categoriesSwept names every category you ACTUALLY swept — honesty over completeness.' + RO
  + ' Structured output: findings (Expected/Got), categoriesSwept, summary.';
// ╔══ module: src/agents/securityAuditor/index.ts ═════════════════════════
// SECURITY AUDITOR — the one diff-scoped wave-level security sweep, audit/security.md 8A-8K (source
// lines 479-492).


                                                                               

const SECURITY         = {
  type: 'object',
  properties: {
    findings: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          category: { type: 'string', description: '8A-8K per the audit doc' },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high', 'critical'] },
          what: { type: 'string' },
          location: { type: 'string' },
          expected: { type: 'string' },
          got: { type: 'string' },
          fix: { type: 'string' },
        },
        required: ['id', 'category', 'severity', 'what', 'location'],
      },
    },
    categoriesSwept: { type: 'array', items: { type: 'string' } },
    summary: { type: 'string' },
  },
  required: ['findings', 'categoriesSwept', 'summary'],
};

const securityAuditor                             = {
  tier: CONFIG.TIER.securityAuditor,
  effort: CONFIG.EFFORT.securityAuditor,
  schema: SECURITY,
  buildPrompt: buildSecurityAuditor,
};
// ╔══ module: src/agents/securityAuditor/run.ts ═══════════════════════════
// runSecurityAuditor — the one diff-scoped auditor call, part of the Walk barrier (source lines 486-492).


                                                                             

function runSecurityAuditor(args                     )                              {
  return retryAgent             (securityAuditor.buildPrompt(args), {
    label: 'security · 8A-8K',
    phase: 'Walk',
    model: securityAuditor.tier,
    effort: securityAuditor.effort,
    schema: securityAuditor.schema,
  });
}
// ╔══ module: src/agents/sliceSensor/prompts.ts ═══════════════════════════
// sliceSensor prompt — byte-identical to the source's inline construction (wave-walker.js lines 451-468).


                                                            

const buildSliceSensor = ({ jobId, kind, files, hint, assigned }                 )         =>
  'You are a PURE EXTRACTOR (scheduled sensor). NO judgment, NO bug-finding — extract and return, nothing else. '
  + 'Read ONLY these files (Grep to confirm an anchor is fine): ' + JSON.stringify(files) + '. Hint: ' + (hint || 'none') + '.\n'
  + 'For EACH assigned field, extract its ' + kind.toUpperCase() + ' slice:\n'
  + (kind === 'producer'
    ? '· producer: where the value is mapped onto the result object — anchor, writer, typeToken, encoding (' + ENC_VOCAB + '), valueLiterals (EXACT, case-preserved).\n· dbColumn (if from a column): anchor, columnName, columnType, checkLiterals.\n· resolver (if a dedicated field/type resolver exists): anchor.\n'
    : kind === 'consumer'
      ? '· feSelection: where the query selects it — anchor, queryName (omit if never selected).\n· feTypes: generated type AND any hand-written interfaces — anchor, typeToken, kind (generated|hand).\n· consumers: EVERY read to the leaf render — anchor, name, decode (' + DEC_VOCAB + '), decodeExpr (VERBATIM, <=80 chars), context (production|test|generated|story), comparedLiterals (EXACT, case-preserved), aliasChain.\n· PARSE SITES ARE CONSUMERS: a screen that parses/transforms before drilling down (JSON.parse, mapping, memo) is its own consumer — its verbatim expression is the decodeExpr; a JSON.parse(JSON.stringify(x)) roundtrip MUST appear verbatim, never summarized; record each screen\'s parse separately.\n· undeclaredReads: any property read off the same result object NOT in your assigned field list (side:"fe"), INCLUDING reads in fallback chains (a ?? b, a || b, ternaries) and optional-chained access; plus any field the resolver returns beyond the declared set if a resolver file is listed (side:"be", expand spreads).\n'
      : '· producer (Cortex writer): where Cortex computes/writes this value — anchor, writer:"cortex", encoding, valueLiterals (EXACT). Grep the snake_case form.\n')
  + 'A field with nothing to extract here gets a slice with just its fieldId. Every anchor grep-verified file:line. Keep strings SHORT (<=80 chars).\n'
  + 'Assigned fields: ' + JSON.stringify(assigned) + RO
  + ' Structured output: jobId=' + jobId + ', slices, undeclaredReads.';
// ╔══ module: src/agents/sliceSensor/index.ts ═════════════════════════════
// SLICE SENSOR — one call per scheduled producer/consumer/cortex job: a PURE EXTRACTOR, zero judgment
// (source lines 305-331, 451-468). Escalates to CONFIG.SENSOR_ESCALATE on a dead respawn (see run.ts).


                                                                           

const SLICE_PROPS                         = {
  fieldId: { type: 'string' },
  producer: {
    type: 'object',
    properties: {
      anchor: { type: 'string' },
      writer: { type: 'string' },
      typeToken: { type: 'string' },
      encoding: { type: 'string' },
      valueLiterals: { type: 'array', items: { type: 'string' } },
    },
  },
  dbColumn: {
    type: 'object',
    properties: {
      anchor: { type: 'string' },
      columnName: { type: 'string' },
      columnType: { type: 'string' },
      checkLiterals: { type: 'array', items: { type: 'string' } },
    },
  },
  resolver: { type: 'object', properties: { anchor: { type: 'string' } } },
  feSelection: { type: 'object', properties: { anchor: { type: 'string' }, queryName: { type: 'string' } } },
  feTypes: {
    type: 'array',
    items: {
      type: 'object',
      properties: {
        anchor: { type: 'string' },
        typeToken: { type: 'string' },
        kind: { type: 'string', description: 'generated | hand' },
      },
    },
  },
  consumers: {
    type: 'array',
    items: {
      type: 'object',
      properties: {
        anchor: { type: 'string' },
        name: { type: 'string' },
        decode: { type: 'string' },
        decodeExpr: { type: 'string', description: 'VERBATIM read/parse expression, <=80 chars' },
        context: { type: 'string', description: 'production | test | generated | story' },
        comparedLiterals: { type: 'array', items: { type: 'string' } },
        aliasChain: { type: 'array', items: { type: 'string' } },
      },
      required: ['anchor'],
    },
  },
  danglingRefs: { type: 'array', items: { type: 'object', properties: { ref: { type: 'string' }, anchor: { type: 'string' } } } },
  notes: { type: 'string' },
};

const SLICES         = {
  type: 'object',
  properties: {
    jobId: { type: 'string' },
    slices: { type: 'array', items: { type: 'object', properties: SLICE_PROPS, required: ['fieldId'] } },
    undeclaredReads: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          side: { type: 'string' },
          property: { type: 'string' },
          anchor: { type: 'string' },
          expr: { type: 'string' },
        },
        required: ['property', 'anchor'],
      },
    },
  },
  required: ['jobId', 'slices'],
};

const sliceSensor                         = {
  tier: CONFIG.TIER.sliceSensor,
  effort: CONFIG.EFFORT.sliceSensor,
  schema: SLICES,
  buildPrompt: buildSliceSensor,
};
// ╔══ module: src/agents/sliceSensor/run.ts ═══════════════════════════════
// runSliceSensor — one call per scheduled job (source lines 451-468). Escalates to CONFIG.SENSOR_ESCALATE
// on a dead respawn — the source's third `resilient()` arg (SENSOR_ESCALATE, fixed 'sonnet', not an arg).



                                                                       

function runSliceSensor(args                 )                            {
  return retryAgent           (
    sliceSensor.buildPrompt(args),
    {
      label: args.kind + ' · ' + args.jobId,
      phase: 'Walk',
      model: sliceSensor.tier,
      effort: sliceSensor.effort,
      schema: sliceSensor.schema,
    },
    CONFIG.SENSOR_ESCALATE,
  );
}
// ╔══ module: src/agents/synthesiser/prompts.ts ═══════════════════════════
// synthesiser prompt — byte-identical to the source's inline construction (wave-walker.js lines 259-264). Divergence #4: the trailing anti-degenerate answer line (Professor-authored).
                                                            

const buildSynthesiser = ({
  goal,
  stopReason,
  keyIds,
  conf,
  reportOut,
  resultSoFarText,
  claimsOut,
  openLeads,
}                 )         =>
  'You are the SYNTHESISER of a code investigation. GOAL: ' + goal + '. Write the report from the hardened ledger — sections: Answer · Evidence (claims by status, cite ids inline [cN] with anchors) · Counter-evidence & survived challenges · Open leads · Coverage (stopReason: ' + stopReason + '). '
  + 'COMPUTED confidence over key claims ' + JSON.stringify(keyIds) + ' = ' + conf + ' — your stated confidence may be LOWER, never higher. '
  + (reportOut ? 'WRITE the full report to ' + reportOut + ' (your ONLY file write; run no git). ' : 'Write no files. ')
  + 'resultSoFar: ' + resultSoFarText + '\nLEDGER: ' + JSON.stringify(claimsOut) + '\nOPEN LEADS: ' + JSON.stringify(openLeads)
  + ' Structured output: answer, confidence, report (full markdown).'
  + '\nThe answer field carries the COMPLETE answer text itself — never a placeholder, never a pointer to the report file; it is the caller\'s primary deliverable.';
// ╔══ module: src/agents/synthesiser/index.ts ═════════════════════════════
// SYNTHESISER (investigate mode) — writes the cited report from the hardened ledger, confidence floored
// by the computed value (source lines 258-266).


                                                                           

// INTENTIONAL DIVERGENCE #4 (anti-degenerate guard): answer.minLength 80 — a live run returned {"answer":"test"} after writing a full report to reportOut; structured-output validation now forces a real answer. The report field stays unconstrained (legitimately vestigial when reportOut is used).
const SYNTH         = {
  type: 'object',
  properties: {
    answer: { type: 'string', minLength: 80 },
    confidence: { type: 'string', enum: ['low', 'medium', 'high'] },
    report: { type: 'string' },
  },
  required: ['answer', 'confidence', 'report'],
};

const synthesiser                         = {
  tier: CONFIG.TIER.synthesiser,
  effort: CONFIG.EFFORT.synthesiser,
  schema: SYNTH,
  buildPrompt: buildSynthesiser,
};
// ╔══ module: src/agents/synthesiser/run.ts ═══════════════════════════════
// runSynthesiser — the ONE closing synthesiser call (source lines 259-265).


                                                                      

function runSynthesiser(args                 )                           {
  return retryAgent          (synthesiser.buildPrompt(args), {
    label: 'synth',
    phase: 'Investigate',
    model: synthesiser.tier,
    effort: synthesiser.effort,
    schema: synthesiser.schema,
  });
}
// ╔══ module: src/agents/territoryDigest/prompts.ts ═══════════════════════
// territoryDigest prompt — byte-identical to the source's inline construction (wave-walker.js lines 630-638). A non-empty charter appends the Professor-authored WALK CHARTER block (zero bytes otherwise).


                                                                

const buildTerritoryDigest = ({ territory, slice, charter }                     )         =>
  'You are the ' + territory + ' TERRITORY DIGEST for this wave. You receive this territory\'s side of every extracted card. '
  + 'Mechanical rules AND the thread walk already handled connectivity/contract/gate/flow — do NOT re-report those. Your job is what neither can see: duplication across fields, wrong-layer logic, over-engineering, naming drift, magic literals, shallow error handling, hardcoded i18n. Open files as needed.\n'
  + CATCHBOOK + '\nCards: ' + JSON.stringify(slice) + RO
  + ' Structured output: territory, findings (each {lens, severity, what, location=file:line, fix}), summary (<=3 sentences).'
  + (charter ? '\nWALK CHARTER (caller-supplied duty): ' + charter + '\nHunt charter-relevant smells in your territory on top of the standard digest.' : '');
// ╔══ module: src/agents/territoryDigest/index.ts ═════════════════════════
// TERRITORY DIGEST — one call per territory (BE/FE/Cortex), catch-book smells the mechanical rules and
// thread walk can't see (source lines 369-375, 630-638).


                                                                               

const DIGEST         = {
  type: 'object',
  properties: {
    territory: { type: 'string' },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          lens: { type: 'string' },
          severity: { type: 'string', enum: ['info', 'low', 'med', 'high'] },
          what: { type: 'string' },
          location: { type: 'string' },
          fix: { type: 'string' },
        },
        required: ['lens', 'severity', 'what', 'location'],
      },
    },
    summary: { type: 'string' },
  },
  required: ['territory', 'findings', 'summary'],
};

const territoryDigest                             = {
  tier: CONFIG.TIER.territoryDigest,
  effort: CONFIG.EFFORT.territoryDigest,
  schema: DIGEST,
  buildPrompt: buildTerritoryDigest,
};
// ╔══ module: src/agents/territoryDigest/run.ts ═══════════════════════════
// runTerritoryDigest — one call per territory (source lines 626-638).


                                                                           

function runTerritoryDigest(args                     )                            {
  return retryAgent           (territoryDigest.buildPrompt(args), {
    label: 'digest · ' + args.territory,
    phase: 'Judge',
    model: territoryDigest.tier,
    effort: territoryDigest.effort,
    schema: territoryDigest.schema,
  });
}
// ╔══ module: src/agents/threadWalker/prompts.ts ══════════════════════════
// threadWalker prompt — byte-identical to the source's inline construction (wave-walker.js lines 441-449)
// EXCEPT for one deliberate, ALWAYS-ON addition: the reorientation sentences below (INVARIANT REGISTRY
// FEATURE, tmp/wave-walker-investigation.md § 2.5 — "cheap, high-yield prompt change", NOT gated by the
// registry; the design names this an unconditional improvement, unlike every other piece of this
// feature). A non-empty charter still appends the Professor-authored WALK CHARTER block (zero bytes
// otherwise) — that part is unchanged.

                                                             

const buildThreadWalker = ({ walkerDoc, thread, charter }                  )         =>
  'Read ' + walkerDoc + ' § Role: Walker. Walk this ONE thread end-to-end in a single pass over its files, returning BOTH the functional verdict AND the integration-delta code-hygiene findings. '
  + 'Per-pipeline hygiene already ran pre-merge (wave/builder.md Step 7) — your wave-level value is the INTEGRATION delta: a repo-wide reuse-grep for a helper/type/hook a SIBLING pipeline duplicated, plus dead code the integration orphaned. '
  + 'At every step, also name the concrete input/state under which this step corrupts, aborts, or lies — a failure scenario, not a vibe. Any two set-enumerations the flow assumes equal (a wipe set vs its snapshot set, a terminal-status set vs a poll loop\'s terminal set, a required-env list vs a validator\'s list) are diffed member-by-member. Apply the broken-mechanism test: what does this step report when it FAILS — the same as "nothing to do"? Flag it. '
  + 'Thread: ' + JSON.stringify(thread) + '.' + RO
  + ' Structured output: threadId, name, type, flow (INTACT|AT-RISK|BROKEN|N/A), trace (step → step, marking any break), defects (each {what, location=file:line, failureScenario, jc=`/jc {fix}`}), hygiene (each {kind, where=file:line, detail, jc}), notes.'
  + (charter ? '\nWALK CHARTER (caller-supplied duty): ' + charter + '\nWeigh this thread against the charter and report charter-relevant findings explicitly in notes — on top of the standard verdict, never instead of it.' : '');
// ╔══ module: src/agents/threadWalker/index.ts ════════════════════════════
// THREAD WALKER — one thread-walker call per scout thread: functional verdict + integration-delta
// hygiene (source lines 349-357, 441-449).


                                                                            

const WALK         = {
  type: 'object',
  properties: {
    threadId: { type: 'string' },
    name: { type: 'string' },
    type: { type: 'string' },
    flow: { type: 'string', enum: ['INTACT', 'AT-RISK', 'BROKEN', 'N/A'] },
    trace: { type: 'string' },
    defects: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          what: { type: 'string' },
          location: { type: 'string' },
          jc: { type: 'string' },
          // INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.5) — optional, so the field
          // is additive: a defect with none named stays a valid defect.
          failureScenario: { type: 'string', description: 'the concrete input/state under which this step corrupts, aborts, or lies' },
        },
        required: ['what', 'location'],
      },
    },
    hygiene: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          kind: { type: 'string' },
          where: { type: 'string' },
          detail: { type: 'string' },
          jc: { type: 'string' },
        },
        required: ['kind', 'where', 'detail'],
      },
    },
    notes: { type: 'string' },
  },
  required: ['threadId', 'flow', 'trace', 'defects', 'hygiene'],
};

const threadWalker                          = {
  tier: CONFIG.TIER.threadWalker,
  effort: CONFIG.EFFORT.threadWalker,
  schema: WALK,
  buildPrompt: buildThreadWalker,
};
// ╔══ module: src/agents/threadWalker/run.ts ══════════════════════════════
// runThreadWalker — one call per thread (source lines 441-449). Pure request/response; the parallel
// fan-out across all threads + jobs + gate files + the security auditor is owned by engine.ts.


                                                                      

function runThreadWalker(args                  )                          {
  const t = args.thread;
  return retryAgent         (threadWalker.buildPrompt(args), {
    label: 'walk · ' + (t.id || t.name || '?'),
    phase: 'Walk',
    model: threadWalker.tier,
    effort: threadWalker.effort,
    schema: threadWalker.schema,
  });
}
// ╔══ module: src/batching.ts ═════════════════════════════════════════════
// batching.ts — the E2 verify-panel batching reducers, pure functions over plain claim lists (same
// discipline as rules.ts/ledger.ts). Ported from the PROVEN reference variant
// (tmp/walker-measure/variants/wave-walker.v2-manifest-scale.js) with its haiku existence/behavior
// tiering DELIBERATELY dropped — every batch rides the verifier tier (sonnet/xhigh); measured haiku
// verifiers did 2.5× the tool calls (+21% tokens, +235% latency) and were rejected.
//
// Clustering rule: claims sharing a file-path prefix of depth ≥2 (e.g. '{project}/src') land in one
// cluster; a claim with NO files opens its own cluster (never merged — no affinity evidence). Batches
// are greedy ≤4-claim slices of each cluster, cluster order preserved.
                                                

function prefixDepth2(f                           )         {
  const parts = String(f || '').split('/');
  return parts.slice(0, 2).join('/');
}

                   
                     
                    
 

function clusterClaims(list           )              {
  const clusters            = [];
  for (const c of list) {
    const prefixes = (c.files || []).map(prefixDepth2).filter(Boolean);
    let cluster = prefixes.length
      ? clusters.find((cl) => cl.prefixes.some((p) => prefixes.includes(p)))
      : null;
    if (!cluster) {
      cluster = { prefixes: [], claims: [] };
      clusters.push(cluster);
    }
    cluster.claims.push(c);
    for (const p of prefixes) if (!cluster.prefixes.includes(p)) cluster.prefixes.push(p);
  }
  return clusters.map((cl) => cl.claims);
}

function batchClaims(list           )              {
  const out              = [];
  for (const cluster of clusterClaims(list))
    for (let i = 0; i < cluster.length; i += 4) out.push(cluster.slice(i, i + 4));
  return out;
}
// ╔══ module: src/ledger.ts ═══════════════════════════════════════════════
// ledger.ts — the INVESTIGATE-mode claim ledger reducers (wave-walker.js lines 173-208), ported as pure
// functions over a plain `LedgerState` object (mirrors rr's store.ts: state is the first arg, never a
// closure). `ingest` mutates the ledger/byStmt/leads maps in place and returns the count of FRESH claims
// this call ledgered — byte-behavior-identical to the source's closure-based `ingest`, including id
// numbering ('c1', 'c2', … / 'L1', 'L2', …) and iteration order.
                                                                        
                                                   

                              
                                 
                                 
                                 
               
               
 

function createLedgerState()              {
  return { ledger: new Map(), byStmt: new Map(), leads: new Map(), cseq: 0, lseq: 0 };
}

// normStmt — the dedupe key: lowercase, whitespace-collapsed, trimmed (source line 175).
const normStmt = (s                           )         =>
  String(s || '')
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim();

// ingest — folds a wave's probe results into the ledger: dedupe-by-normStmt, merge anchors/files onto the
// existing row, mark counter-attack targets contested, credit a real attack-lane's survival, and mint
// fresh lead ids. Returns the number of NEW (never-before-seen) claim rows this call created.
function ingest(state             , probeResults                     , wave        )         {
  let fresh = 0;
  for (const r of probeResults.filter((x)                => !!x)) {
    const counters = (r.claims || []).filter((c) => c.kind === 'counter');
    for (const c of r.claims || []) {
      const key = normStmt(c.statement);
      let row = state.byStmt.get(key);
      if (!row) {
        row = {
          id: 'c' + ++state.cseq,
          statement: c.statement,
          anchors: [],
          files: [],
          contested: false,
          survived: 0,
          audit: 'pending',
          wave,
        };
        state.byStmt.set(key, row);
        state.ledger.set(row.id, row);
        fresh++;
      }
      for (const a of c.anchors || []) {
        if (a && a.anchor && !row.anchors.some((x) => x.anchor === a.anchor))
          row.anchors.push({ anchor: a.anchor, quote: a.quote });
        const f = String((a && a.anchor) || '').split(':')[0];
        if (f && !row.files.includes(f)) row.files.push(f);
      }
      if (c.kind === 'counter')
        for (const t of c.targets || []) {
          const tgt = state.ledger.get(t);
          if (tgt) tgt.contested = true;
        }
    }
    if (r._laneKind === 'attack' && (r.nothingFound || counters.length === 0))
      for (const t of r._targets || []) {
        const tgt = state.ledger.get(t);
        if (tgt) tgt.survived++;
      }
    for (const l of r.leads || []) {
      const id = 'L' + ++state.lseq;
      state.leads.set(id, { id, what: l.what, files: l.files || [] });
    }
  }
  return fresh;
}

// statusOf — COMPUTED from ledger topology, never asserted (source lines 196-201). settled REQUIRES a
// mechanical audit pass AND (a survived challenge OR a third independent anchor file).
function statusOf(row           )                                        {
  if (row.contested) return 'contested';
  if (row.audit === 'fail') return 'tentative';
  if (row.audit === 'pass' && row.files.length >= 2 && (row.survived >= 1 || row.files.length >= 3))
    return 'settled';
  return 'tentative';
}

// computedConfidence — over exactly the brainer's key claim ids (source lines 202-208). No key ids →
// low; any contested/audit-failed key claim → low; every key claim settled → high; else medium.
function computedConfidence(state             , keyIds          )             {
  const rows = keyIds.map((id) => state.ledger.get(id)).filter((r)                 => !!r);
  if (!rows.length) return 'low';
  if (rows.some((r) => statusOf(r) === 'contested' || r.audit === 'fail')) return 'low';
  if (rows.every((r) => statusOf(r) === 'settled')) return 'high';
  return 'medium';
}
// ╔══ module: src/rules.ts ════════════════════════════════════════════════
// rules.ts — the ZERO-TOKEN rule engine (wave-walker.js lines 508-589), ported as pure functions over
// plain state (no class, no engine coupling — mirrors rr's store.ts reducer discipline). Two stages:
// zipCards mechanically zips every sliceSensor job's SlicesOut onto one Card per field (lines 508-528);
// computeAnomalies runs the R1-R8 diff over the zipped cards + undeclared reads + gate cards (529-589).
// Both are byte-behavior-identical to the source — same iteration order, same id numbering, same detail
// strings — verified by test/rules.test.ts against hand-traced expected anomaly sets. A third function,
// computeInvariantAnomalies (INVARIANT REGISTRY FEATURE, tmp/wave-walker-investigation.md § 2.2-2.3), is
// NOT a source port — it reshapes invariantHunter findings into the same Anomaly shape under a new rule
// class, R9-INV, so they concatenate onto computeAnomalies's output and ride the identical downstream
// pipeline. It only ever runs when CONFIG.INVARIANTS is non-empty; empty input yields [].
             
          
       
            
                   
                     
         
           
        
           
            
                 
                          

// ── baseType — normalizes a GraphQL/TS type token down to its structural base (source lines 537-542) ──
function baseType(t                           )         {
  let s = String(t || '')
    .toLowerCase()
    .replace(/maybe<|scalars\['?|'?\]\['(in|out)put'?\]|>|\?|!/g, '');
  s = s
    .replace(/\|\s*(null|undefined)/g, '')
    .replace(/(null|undefined)\s*\|/g, '')
    .replace(/\s+/g, '');
  const map                         = {
    string: 'string',
    str: 'string',
    id: 'string',
    int: 'number',
    float: 'number',
    number: 'number',
    boolean: 'boolean',
    bool: 'boolean',
    jsonb: 'object',
    json: 'object',
  };
  return (
    map[s] ||
    (s.startsWith('record<') || s.startsWith('{') || s.includes('array<') || s.endsWith('[]')
      ? 'object'
      : s)
  );
}

// R3 encoding-mismatch incompatibility table + the double-encode detector (source lines 543-544).
const INCOMPAT                           = {
  'json-string': ['object-index', 'spread'],
  object: ['json-parse'],
  'enum-string': ['object-index'],
};
const DOUBLE_ENCODE = /JSON\s*\.\s*parse\s*\(\s*JSON\s*\.\s*stringify/;

// ── zipCards — mechanical, zero-token zip of every sliceSensor job's output onto one Card per field
// (source lines 508-528). `droppedFieldIds` are fields the sensor cap dropped before scheduling (engine
// owns that cap; passed in here so `unsensed` still names them, exactly as the source's inline computation does).
function zipCards(
  fields             ,
  jobs            ,
  sliceResults             ,
  droppedFieldIds           = [],
)                                        {
  const cardMap = new Map                                        (
    fields.map((f) => [
      f.id,
      {
        id: f.id,
        ownerType: f.ownerType,
        field: f.field,
        apis: f.apis || [],
        sdl: f.sdl || null,
        feTypes: [],
        consumers: [],
        danglingRefs: [],
        sidesCovered: [],
        _sides: new Set        (),
      },
    ]),
  );
  for (const r of sliceResults) {
    const job = jobs.find((j) => j.jobId === r.jobId);
    for (const s of r.slices || []) {
      const c = cardMap.get(s.fieldId);
      if (!c) continue;
      c._sides.add(job ? job.kind : '?');
      if (s.producer && !c.producer) c.producer = s.producer;
      else if (s.producer && c.producer)
        c.producer.valueLiterals = [
          ...new Set([...(c.producer.valueLiterals || []), ...(s.producer.valueLiterals || [])]),
        ];
      if (s.dbColumn && !c.dbColumn) c.dbColumn = s.dbColumn;
      if (s.resolver && !c.resolver) c.resolver = s.resolver;
      if (s.feSelection && !c.feSelection) c.feSelection = s.feSelection;
      if (s.feTypes) c.feTypes.push(...s.feTypes);
      if (s.consumers) c.consumers.push(...s.consumers);
      if (s.danglingRefs) c.danglingRefs.push(...s.danglingRefs);
      if (s.notes) c.notes = ((c.notes || '') + ' ' + s.notes).trim();
    }
  }
  const cards = [...cardMap.values()].map((c) => {
    const { _sides, ...rest } = c;
    rest.sidesCovered = [..._sides];
    return rest;
  });
  const unsensed = [
    ...new Set([
      ...[...cardMap.values()].filter((c) => c._sides.size === 0).map((c) => c.id),
      ...droppedFieldIds,
    ]),
  ];
  return { cards, unsensed };
}

// ── groupByResource — the gate byResource grouping (source lines 576-577). Plain object, string-key
// insertion order preserved (matches the source's own object accumulation) — later grouping passes
// (gate-outlier, mandated-fence) iterate Object.entries in that same order.
function groupByResource(gates                    )                                     {
  const byResource                                     = {};
  for (const g of gates) (byResource[g.resource || 'other'] ||= []).push(g);
  return byResource;
}

const a = (x                                        )                => (x && x.anchor) || null;

// ── computeAnomalies — the R1-R8 ledger diff (source lines 532-589). ONE pass, in the source's exact
// iteration order, so the sequential `id` numbering (R1-1, R2-3, …) is byte-identical to the source for
// the same inputs — never split into independently-numbered sub-functions.
function computeAnomalies(
  cards        ,
  undeclaredReads                  ,
  gates                    ,
  authRule        ,
)            {
  const anomalies            = [];
  let aseq = 0;
  const flag = (
    rule        ,
    ruleName        ,
    detail        ,
    anchors                   ,
    severityHint          ,
    cardId               ,
  )       => {
    anomalies.push({
      id: rule + '-' + ++aseq,
      rule,
      ruleName,
      detail,
      anchors: (anchors || []).filter((x)              => !!x),
      severityHint,
      cardId: cardId || null,
    });
  };

  for (const c of cards) {
    const consumers = c.consumers || [];
    const prodConsumers = consumers.filter((x) => (x.context || 'production') === 'production');
    if (c.producer && prodConsumers.length === 0) {
      const nonProd = consumers.length - prodConsumers.length;
      const sub = !c.sdl
        ? 'produced but never exposed in SDL'
        : !c.feSelection
          ? 'declared in SDL but never selected by any FE query'
          : 'shipped and selected but read by no production consumer';
      flag(
        'R1',
        'orphan producer',
        c.id + ': ' + sub + (nonProd ? ' (' + nonProd + ' non-production ref(s) only)' : ''),
        [a(c.producer), a(c.sdl), a(c.feSelection)],
        'med',
        c.id,
      );
    }
    if (!c.producer && consumers.length > 0)
      flag(
        'R2',
        'phantom consumer',
        c.id +
          ': consumed at ' +
          consumers.length +
          ' site(s) but no producer emits it' +
          (c.sdl ? ' (declared in SDL yet unfed)' : ' (absent from SDL)'),
        [a(c.sdl), ...consumers.map(a)],
        'high',
        c.id,
      );
    const enc = c.producer && c.producer.encoding;
    for (const cons of consumers) {
      if (cons.decodeExpr && DOUBLE_ENCODE.test(cons.decodeExpr))
        flag(
          'R3',
          'encoding mismatch',
          c.id +
            ': double-encode JSON.parse(JSON.stringify(...)) at ' +
            cons.anchor +
            ' — on a ' +
            (enc || 'unknown') +
            ' value returns the input unchanged, never a parsed object',
          [a(c.producer), cons.anchor],
          'high',
          c.id,
        );
      else if (enc && INCOMPAT[enc] && cons.decode && INCOMPAT[enc].includes(cons.decode))
        flag(
          'R3',
          'encoding mismatch',
          c.id +
            ': produced as ' +
            enc +
            ' (' +
            ((c.producer && c.producer.anchor) || '?') +
            ') but consumed via ' +
            cons.decode +
            ' at ' +
            cons.anchor,
          [a(c.producer), cons.anchor],
          'high',
          c.id,
        );
    }
    const prodLits = [
      ...new Set([
        ...((c.producer && c.producer.valueLiterals) || []),
        ...((c.dbColumn && c.dbColumn.checkLiterals) || []),
      ]),
    ];
    if (prodLits.length)
      for (const cons of consumers) {
        const cl = cons.comparedLiterals || [];
        const missing = cl.filter((l) => !prodLits.includes(l));
        if (cl.length && missing.length) {
          const casing = missing.filter((l) => prodLits.some((p) => p.toLowerCase() === String(l).toLowerCase()));
          flag(
            'R4',
            'value-set mismatch',
            c.id +
              ': consumer at ' +
              cons.anchor +
              ' compares against ' +
              JSON.stringify(missing) +
              ' which no producer emits' +
              (casing.length
                ? ' — CASING mismatch of ' + JSON.stringify(casing) + ', branch permanently dead'
                : '') +
              ' (produced: ' +
              JSON.stringify(prodLits.slice(0, 8)) +
              ')',
            [a(c.producer), a(c.dbColumn), cons.anchor],
            casing.length ? 'critical' : 'high',
            c.id,
          );
        }
      }
    const gen = (c.feTypes || []).find((t) => t.kind === 'generated');
    for (const hand of (c.feTypes || []).filter((t) => t.kind === 'hand')) {
      const ref = gen || (c.sdl ? { typeToken: c.sdl.typeToken, anchor: c.sdl.anchor } : null);
      if (ref && baseType(hand.typeToken) !== baseType(ref.typeToken))
        flag(
          'R5',
          'type drift',
          c.id +
            ': hand-typed "' +
            hand.typeToken +
            '" (' +
            hand.anchor +
            ') vs ' +
            (gen ? 'generated' : 'SDL') +
            ' "' +
            ref.typeToken +
            '" — base ' +
            baseType(hand.typeToken) +
            ' vs ' +
            baseType(ref.typeToken),
          [hand.anchor ?? null, ref.anchor ?? null],
          'med',
          c.id,
        );
    }
    for (const d of c.danglingRefs || [])
      flag(
        'R8',
        'dangling reference',
        c.id + ': "' + d.ref + '" at ' + d.anchor + ' resolves to nothing',
        [d.anchor ?? null],
        'med',
        c.id,
      );
  }
  for (const r of undeclaredReads)
    flag(
      'R2',
      'phantom consumer',
      (r.side === 'be' ? 'resolver returns' : 'FE reads') +
        ' undeclared field "' +
        r.property +
        '" at ' +
        r.anchor +
        (r.expr ? ' (' + r.expr + ')' : ''),
      [r.anchor],
      r.side === 'be' ? 'med' : 'high',
      null,
    );

  const byResource = groupByResource(gates);
  for (const [res, group] of Object.entries(byResource)) {
    const fenced = group.filter((g) => g.ownershipFence);
    const unfenced = group.filter((g) => !g.ownershipFence && (g.idArgs || []).length > 0);
    if (fenced.length && unfenced.length)
      flag(
        'R6',
        'gate outlier',
        'resource "' +
          res +
          '": ' +
          fenced.map((g) => g.id).join(', ') +
          ' enforce an ownership fence but ' +
          unfenced.map((g) => g.id).join(', ') +
          ' do not — same class, weaker chain',
        [...fenced.map((g) => g.anchor), ...unfenced.map((g) => g.anchor)],
        'high',
        null,
      );
  }
  for (const [res, group] of Object.entries(byResource)) {
    if (!['session', 'patient', 'couple'].includes(res)) continue;
    const violators = group.filter(
      (g) =>
        (g.idArgs || []).length > 0 &&
        !g.ownershipFence &&
        (g.rolesAllowed || []).some((r) => String(r).toUpperCase().includes('THERAPIST')),
    );
    if (violators.length)
      flag(
        'R6',
        'mandated-fence violation',
        'resource "' +
          res +
          '": ' +
          violators.map((g) => g.id).join(', ') +
          ' admit THERAPIST with client-supplied id but enforce NO ownership fence — direct violation of the documented rule. ' +
          authRule,
        violators.map((g) => g.anchor),
        'critical',
        null,
      );
  }
  for (const g of gates)
    if ((g.idArgs || []).length > 0 && !g.clinicFence && !g.ownershipFence)
      flag(
        'R7',
        'unfenced ID flow',
        g.id +
          ': client-supplied ' +
          JSON.stringify(g.idArgs) +
          ' reaches data access with neither clinic nor ownership fence (chain: ' +
          (g.chain || []).join(' → ') +
          ')',
        [g.anchor],
        'critical',
        null,
      );

  return anomalies;
}

// computeInvariantAnomalies — INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.2-2.3):
// converts every invariantHunter's findings into Anomaly-shaped rows of rule class R9-INV, sequentially
// numbered across ALL hunters (one global aseq, same discipline as computeAnomalies's own numbering) so
// engine.ts can simply CONCATENATE this array onto computeAnomalies's R1-R8 output — the merged array
// then rides the EXISTING byRule → judge → escalation → final-judge → fold pipeline completely unchanged.
// Zero tokens; pure data reshaping, no judgment (judgment is the hunter's + the R9-INV anomalyJudge's job).
function computeInvariantAnomalies(hunterResults                      )            {
  const anomalies            = [];
  let aseq = 0;
  for (const r of hunterResults || []) {
    for (const f of r.findings || []) {
      anomalies.push({
        id: 'R9-INV-' + ++aseq,
        rule: 'R9-INV',
        ruleName: 'invariant-registry violation',
        detail:
          '[' +
          r.invariantId +
          '] ' +
          f.what +
          ' — Expected: ' +
          f.expected +
          ' — Got: ' +
          f.got +
          ' — failure scenario: ' +
          f.failureScenario,
        anchors: [f.location].filter((x)              => !!x),
        severityHint: f.severity,
        cardId: null,
      });
    }
  }
  return anomalies;
}

// ruleCounts — the per-rule tally the source logs (`anomalies.reduce(...)`, line 588).
function ruleCounts(anomalies           )                         {
  return anomalies.reduce(
    (m, x) => {
      m[x.rule] = (m[x.rule] || 0) + 1;
      return m;
    },
    {}                          ,
  );
}
// ╔══ module: src/utils/index.ts ══════════════════════════════════════════
// utils/index.ts — small pure helpers shared by engine.ts. `chunk` matches the source's inline helper
// (wave-walker.js line 603) with one defensive addition: size<=0 degrades to one whole-array chunk
// instead of an infinite loop — the source never calls it with a non-positive size (always the literal
// 6 or 4), so this never changes observed behavior, only guards against future misuse.
                                                     
function chunk   (items     , size        )        {
  if (!items.length) return [];
  if (size <= 0) return [items];
  const out        = [];
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
function globMatch(pattern        , filePath        )          {
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
function renderTelemetryMd(rec             )         {
  try {
    return renderTelemetryMdInner(rec);
  } catch (e) {
    return (
      '### Walk Telemetry\n\n_(telemetry render failed: ' +
      ((e         ) && (e         ).message ? (e         ).message : String(e)) +
      ' — see debugRecord in the workflow result for raw data)_'
    );
  }
}

function renderTelemetryMdInner(rec             )         {
  const lines           = ['### Walk Telemetry', ''];
  if (rec.degraded) lines.push('**DEGRADED** — ' + (rec.gaps.length ? rec.gaps.join('; ') : 'a section failed to assemble'), '');

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
        (deaths ? ' · died-first ' + t.diedFirstAttempt + ' · retried ' + t.retried + ' · died-after-retry ' + t.diedAfterRetry : ''),
    );
  }
  if (rec.seatsExpectedButAbsent.length) lines.push('- **MISSING (expected, never dispatched):** ' + rec.seatsExpectedButAbsent.join(', '));

  lines.push(
    '',
    '**Invariant registry:** ' +
      rec.armedInvariants.registered +
      ' registered, ' +
      rec.armedInvariants.armed.length +
      ' armed' +
      (rec.armedInvariants.armed.length ? ' (' + rec.armedInvariants.armed.map((a) => a.id).join(', ') + ')' : ''),
  );
  if (rec.armedInvariants.unarmed.length) lines.push('- unarmed: ' + rec.armedInvariants.unarmed.map((a) => a.id).join(', '));

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
  if (rec.coverage.unsensedFields.length) lines.push('- unsensed fields: ' + rec.coverage.unsensedFields.join(', '));
  if (rec.coverage.gateSweepSkipped) lines.push('- gate sweep: SKIPPED (diff-scoped)');
  if (rec.coverage.coverageGaps.length)
    lines.push('- coverage-critic gaps: ' + rec.coverage.coverageGaps.map((g) => g.territory + ' (' + g.why + ')').join('; '));

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
      (js.finalJudgeDied ? 'DIED' : (js.finalJudgeVerdict || '?') + ', reinstated ' + js.finalJudgeReinstated),
  );

  lines.push('', '_tokenAttribution: not available at the engine layer (agent() returns no usage metadata) — see the p:tokens skill for post-hoc spend analysis._');
  return lines.join('\n');
}
// ╔══ module: src/engine.ts ═══════════════════════════════════════════════








             
          
                 
                   
       
          
                  
             
           
                    
              
              
            
                 
               
           
          
                   
               
                     
                
                    
               
       
               
           
            
              
           
            
             
            
               
             
          
             
                   
                          

// project — the digest's per-territory card projection (source lines 621-625): a BE/Cortex/FE side sees
// only the fields relevant to it.
function project(c      , side                        )                          {
  if (side === 'BE')
    return { id: c.id, producer: c.producer, dbColumn: c.dbColumn, sdl: c.sdl, resolver: c.resolver, notes: c.notes };
  if (side === 'Cortex') return { id: c.id, producer: c.producer, dbColumn: c.dbColumn, notes: c.notes };
  return {
    id: c.id,
    sdl: c.sdl && c.sdl.typeToken,
    feSelection: c.feSelection,
    feTypes: c.feTypes,
    consumers: c.consumers,
    notes: c.notes,
  };
}

const SECURITY_RULES = ['R6', 'R7'];
const NEAR_CERTAIN = ['R3', 'R4'];

// E3 gate-conditional dispatch (ported from the proven v3 variant, dispatch-only — no security seat's
// prompt or rule logic changes): the repo-wide gate-file sweep feeding R6/R7 spawns ONLY when the diff
// touches gate-relevant surface — resolver/auth/graphql-infra/application(service) files — or the scout
// scheduled any GraphQL fields/jobs. FAIL-SAFE: any hit → full sweep, byte-identical to the source;
// when in doubt, sweep. CONFIG.FULL_GATE_SWEEP (args.fullGateSweep:true) forces the sweep regardless.
function isGateRelevant(changedFiles          , fieldsLen        , jobsLen        )          {
  return (
    (changedFiles || []).some(
      (f) =>
        /{project}\/src\/infrastructure\/graphql\/resolvers\//.test(f) ||
        /{project}\/src\/(infrastructure\/(auth|graphql)|application)\//.test(f),
    ) ||
    fieldsLen > 0 ||
    jobsLen > 0
  );
}

// INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — computeArmedInvariants: the
// zero-token fail-safe, same philosophy as isGateRelevant ("any hit → sweep; when in doubt, sweep"). The
// scout's own semantic judgment (scoutArmed — it can arm on a trigger no glob could express, e.g. "diff
// adds a reuse/skip/cache gate") is UNIONED with a deterministic territory-glob match against
// changedFiles, so a scout that forgets to arm an invariant whose territory the diff plainly touches
// cannot silently disarm the hunt. Registry order is preserved (stable, testable output order).
function computeArmedInvariants(
  invariants                 ,
  changedFiles          ,
  scoutArmed                                           ,
)                   {
  const scoutById = new Map((scoutArmed || []).filter((a) => a && a.id).map((a) => [a.id, a]));
  const out                   = [];
  for (const inv of invariants || []) {
    const scoutHit = scoutById.get(inv.id);
    const territoryMatches = (changedFiles || []).filter((f) => (inv.territory || []).some((g) => globMatch(g, f)));
    if (!scoutHit && !territoryMatches.length) continue;
    const matchedFiles = [...new Set([...((scoutHit && scoutHit.matchedFiles) || []), ...territoryMatches])];
    const reason =
      scoutHit && territoryMatches.length
        ? 'scout + territory glob'
        : scoutHit
          ? 'scout-armed (semantic trigger)'
          : 'territory glob match (fail-safe — scout did not arm)';
    out.push({ id: inv.id, matchedFiles, reason });
  }
  return out;
}

// WALK TELEMETRY (DEBUG STEP, tmp/walker-debug-design.md) — the plain-data bag runWalk() gathers before
// calling assembleDebugRecord. Every field is already computed elsewhere in runWalk() by the time of
// assembly; this interface exists only to keep the assembler a pure, independently-testable function
// (mirrors computeArmedInvariants/isGateRelevant's own pure-function, directly-testable precedent above).
                                     
                     
                              
                                    
                                                           
                                       
                          
                        
                   
                   
                
                                      
                        
                       
                               
                                                 
                            
                         
                           
                                  
                                  
                                        
                              
                               
                     
                            
                              
 

// assembleDebugRecord — the FLOOR-disciplined assembler (tmp/walker-debug-design.md §5 "null economy"):
// every section below is wrapped in its OWN try/catch. One bad section sets `degraded: true`, names
// itself in `gaps`, and falls back to honest zeros/empties — every OTHER section still assembles
// independently ("one bad section never blanks the rest"). Zero LLM cost — pure JS over already-computed
// data. Exported (like computeArmedInvariants/isGateRelevant above) so a unit test can force one
// section to throw directly, without needing a full stubbed pipeline run.
function assembleDebugRecord(input                    )              {
  const gaps           = [];
  let degraded = false;
  const fail = (section        , e         ) => {
    degraded = true;
    gaps.push(section + ' assembly failed: ' + ((e         ) && (e         ).message ? (e         ).message : String(e)));
  };

  let armedInvariantsOut                                 = { registered: 0, armed: [], unarmed: [] };
  try {
    armedInvariantsOut = {
      registered: input.invariants.length,
      armed: input.armedInvariants.map((a) => ({ id: a.id, matchedFiles: a.matchedFiles, reason: a.reason || '' })),
      unarmed: input.unarmedInvariants,
    };
  } catch (e) {
    fail('armedInvariants', e);
  }

  let seats                            = {};
  let seatsExpectedButAbsent           = [];
  try {
    seats = Object.fromEntries(Object.entries(input.seatTally).map(([k, v]) => [k, { ...v }]));
    seatsExpectedButAbsent = input.expectedSeats.filter((s) => !(s in seats));
  } catch (e) {
    fail('seats', e);
  }

  let emptyResults                              = {
    threadsWalked: 0,
    threadsExpected: 0,
    sensorsWithCards: 0,
    sensorsExpected: 0,
    hunterFindingsTotal: 0,
    huntersReturned: 0,
    huntersExpected: 0,
    digestsWithFindings: 0,
    digestsExpected: 0,
    securityDied: false,
    coverageCriticDied: false,
    foldDied: false,
  };
  try {
    const coverageCriticDied = input.invariants.length > 0 && !input.coverageCriticResult;
    emptyResults = {
      threadsWalked: input.walks.length,
      threadsExpected: input.threads.length,
      sensorsWithCards: input.cards.length,
      sensorsExpected: input.jobs.length,
      hunterFindingsTotal: input.hunterResults.reduce((n, r) => n + r.findings.length, 0),
      huntersReturned: input.hunterResults.length,
      huntersExpected: input.armedInvariants.length,
      digestsWithFindings: input.digests.filter((d) => d.findings.length > 0).length,
      digestsExpected: input.digestJobsLen,
      securityDied: !input.security,
      coverageCriticDied,
      foldDied: false, // patched by the caller once runFold() resolves — see engine.ts runWalk()
    };
    // CLAUDE.md "error never renders as absence" — a dead coverage-critic over a non-empty registry is
    // named directly in `gaps`, not just a quiet boolean a reader could miss (tmp/walker-debug-design.md §5).
    if (coverageCriticDied) gaps.push('coverageCriticDied: registry non-empty but coverage-critic returned nothing');
  } catch (e) {
    fail('emptyResults', e);
  }

  let judgeStats                            = {
    confirmed: 0,
    falseCount: 0,
    unproven: 0,
    secondOpinionDispatched: 0,
    secondOpinionOverturned: 0,
    secondOpinionReexaminedKilled: 0,
    finalJudgeReinstated: 0,
    finalJudgeVerdict: null,
    finalJudgeDied: true,
  };
  try {
    judgeStats = {
      confirmed: input.confirmed.length,
      falseCount: input.killed.length,
      unproven: input.unproven.length,
      secondOpinionDispatched: input.secondOpinionDispatched,
      secondOpinionOverturned: input.secondOpinionOverturned,
      secondOpinionReexaminedKilled: input.secondOpinionReexaminedKilled,
      finalJudgeReinstated: input.finalJudgeReinstated,
      finalJudgeVerdict: input.finalJudge ? input.finalJudge.verdict : null,
      finalJudgeDied: !input.finalJudge,
    };
  } catch (e) {
    fail('judgeStats', e);
  }

  let coverage                          = { unsensedFields: [], gateSweepSkipped: false, coverageGaps: [] };
  try {
    coverage = {
      unsensedFields: input.unsensed,
      gateSweepSkipped: input.gateSweepSkipped,
      coverageGaps: input.coverageGaps,
    };
  } catch (e) {
    fail('coverage', e);
  }

  return {
    schemaVersion: 1,
    mode: 'walk',
    reportPath: input.reportPath,
    degraded,
    gaps,
    armedInvariants: armedInvariantsOut,
    seats,
    seatsExpectedButAbsent,
    emptyResults,
    judgeStats,
    coverage,
    tokenAttribution: null,
  };
}

// ─────────────────────────────────────────────────────────────────────────────
// WaveWalker — the pipeline backbone. run() dispatches on CONFIG.mode to one of the four modes below;
// every agent call goes through a seat's run<Seat>() (pure request/response); every mutation (the
// zero-token rule engine, the ledger, chunking, escalation/reinstatement bookkeeping) is owned here.
// ─────────────────────────────────────────────────────────────────────────────
class WaveWalker {
  async run()                            {
    if (CONFIG.mode === 'verify' || CONFIG.mode === 'manifest-verify') return this.runVerify();
    if (CONFIG.mode === 'investigate') return this.runInvestigate();
    return this.runWalk();
  }

  // ─── VERIFY / MANIFEST-VERIFY — pre-ruling claims panel (source lines 57-134, plus the E2
  // manifest-coverage lever: maxClaims 96, breadth-first extraction, conditional ≤4-claim batching
  // above SOLO_THRESHOLD, consistency-judge payload diet, coverage fields on the result). ─────
  async runVerify()                                       {
    const manifestPath = CONFIG.MANIFEST_PATH;
    let claims                               = CONFIG.CLAIMS             ;
    let conflictChecks                                                                                                     = [];
    let claimsMined = claims.length; // E2 coverage: pre-cap claim count (args-supplied claims are never capped)
    let droppedClaimIds           = [];

    if (manifestPath && !claims.length) {
      const ex = await runClaimExtractor({ manifestPath });
      if (!ex) return { status: 'FAILED', detail: 'claim extractor died twice' };
      claimsMined = (ex.claims || []).length;
      if (claimsMined > CONFIG.MAX_CLAIMS)
        log(
          '⚠ claim cap ' +
            CONFIG.MAX_CLAIMS +
            ': DROPPED ' +
            (ex.claims.length - CONFIG.MAX_CLAIMS) +
            ' tail claim(s): ' +
            ex.claims.slice(CONFIG.MAX_CLAIMS).map((c) => c.id).join(', '),
        );
      droppedClaimIds = claimsMined > CONFIG.MAX_CLAIMS ? ex.claims.slice(CONFIG.MAX_CLAIMS).map((c) => c.id) : [];
      claims = (ex.claims || []).slice(0, CONFIG.MAX_CLAIMS);
      conflictChecks = ex.conflictChecks || [];
      log('Extracted ' + claims.length + ' claim(s) · ' + conflictChecks.length + ' conflict check(s) from ' + manifestPath);
      if (!claims.length)
        return {
          status: 'DONE',
          mode: 'manifest-verify',
          manifest: manifestPath,
          claims: 0,
          verdicts: [],
          consensus: {},
          conflicts: [],
          verifiersDied: 0,
          claimsMined,
          claimsVerified: 0,
          droppedClaimIds,
          taskIds: [],
        };
    }

    // E2 conditional batching: a small panel (claims × votes ≤ SOLO_THRESHOLD) runs SOLO exactly as the
    // source does — per-claim calls, per-claim `claim.opus` escalation, identical labels/schema. A large
    // panel batches ≤4 claims by file-cluster affinity, one verifier call per batch × vote; verifiers
    // stay on the verifier tier (sonnet/xhigh) — never haiku (measured: haiku verifiers did 2.5× the
    // tool calls → +21% tokens, +235% latency; rejected). `claim.opus` cannot survive batching (one
    // model per batch) — accepted trade-off, logged when it drops.
    const panelSize = claims.length * CONFIG.VOTES;
    const solo = panelSize <= CONFIG.SOLO_THRESHOLD;
    let verdicts             ;
    let died        ;
    if (solo) {
      log(
        'Verify mode · ' +
          claims.length +
          ' claim(s) × ' +
          CONFIG.VOTES +
          ' vote(s) · ' +
          CONFIG.TIER.claimVerifier +
          '/' +
          CONFIG.EFFORT.claimVerifier,
      );
      const panel = claims.flatMap((c) => Array.from({ length: CONFIG.VOTES }, (_, v) => ({ c: c           , v })));
      const results = await parallel(panel.map(({ c, v }) => () => runClaimVerifier(c, CONFIG.QUESTION, v, CONFIG.VOTES)));
      verdicts = results.filter((r)                 => !!r);
      died = panel.length - verdicts.length;
    } else {
      const batches = batchClaims(claims             );
      const opusDropped = (claims             ).filter((c) => c.opus).length;
      if (opusDropped)
        log('⚠ batching: ' + opusDropped + ' claim.opus flag(s) cannot escalate inside a batch — riding the verifier tier');
      log('batching: ' + batches.length + ' batch(es) (≤4 claims each, file-cluster affinity)');
      log(
        'Verify mode · ' +
          claims.length +
          ' claim(s) in ' +
          batches.length +
          ' batch(es) × ' +
          CONFIG.VOTES +
          ' vote(s) · ' +
          CONFIG.TIER.claimVerifier +
          '/' +
          CONFIG.EFFORT.claimVerifier,
      );
      const panel = batches.flatMap((b, bi) => Array.from({ length: CONFIG.VOTES }, (_, v) => ({ b, bi, v })));
      const batchResults = await parallel(
        panel.map(({ b, bi, v }) => () => runClaimVerifierBatch(b, CONFIG.QUESTION, bi, v, CONFIG.VOTES)),
      );
      verdicts = batchResults.flatMap((r) => (r && Array.isArray(r.verdicts) ? r.verdicts : []));
      // panel is batches×votes here — died counts dead BATCH CALLS, not missing per-claim verdicts.
      died = panel.length - batchResults.filter(Boolean).length;
    }

    const consensus                         = {};
    for (const c of claims) {
      const vs = verdicts.filter((r) => r.claimId === c.id);
      if (!vs.length) {
        consensus[c.id] = 'NO-VERDICT';
        continue;
      }
      const tally = vs.reduce((m                        , r) => {
        m[r.verdict] = (m[r.verdict] || 0) + 1;
        return m;
      }, {});
      const top = Object.entries(tally).sort((a, b) => b[1] - a[1])[0];
      consensus[c.id] = top[1] > vs.length / 2 ? top[0] : 'SPLIT';
    }

    let conflicts                    = [];
    if (manifestPath) {
      // E2 payload diet: full verdict detail rides only for non-CONFIRMED claims; CONFIRMED claims are
      // represented by the consensus map alone.
      const nonConfirmed = verdicts.filter((v) => consensus[v.claimId] !== 'CONFIRMED');
      const cj = await runConsistencyJudge({ manifestPath, nonConfirmed, consensus, conflictChecks });
      conflicts = cj ? cj.conflicts || [] : [];
      if (cj) log('Consistency: ' + conflicts.length + ' finding(s) — ' + (cj.summary || '').slice(0, 120));
    }
    log(
      'Verify done · ' +
        Object.entries(consensus)
          .map(([k, x]) => k + '=' + x)
          .join(' · ') +
        (died ? ' · ⚠ ' + died + ' verifier(s) died' : ''),
    );
    // E2 coverage: which tasks the verified claims cover (unique taskId among verified claims).
    const taskIds = [...new Set((claims             ).map((c) => c.taskId).filter((t)              => !!t))];
    return {
      status: 'DONE',
      mode: manifestPath ? 'manifest-verify' : 'verify',
      manifest: manifestPath || null,
      question: CONFIG.QUESTION,
      claims: claims.length,
      votes: CONFIG.VOTES,
      verdicts,
      consensus,
      conflicts,
      verifiersDied: died,
      claimsMined,
      claimsVerified: claims.length,
      droppedClaimIds,
      taskIds,
    };
  }

  // ─── INVESTIGATE — RR-for-code: brainer-steered waves over a computed claim ledger (source lines 136-277) ─────
  async runInvestigate()                                            {
    const goal = CONFIG.GOAL;
    const scopeLine = CONFIG.SCOPE ? ' SCOPE (stay inside): ' + JSON.stringify(CONFIG.SCOPE) + '.' : '';
    const state = createLedgerState();

    const auditNew = async (wave        )                => {
      const rows = [...state.ledger.values()].filter((r) => r.audit === 'pending');
      if (!rows.length) return;
      const a = await runClaimAuditor({ rows: rows.map((r) => ({ id: r.id, anchors: r.anchors })), wave });
      if (!a) return;
      for (const v of a.audits || []) {
        const r = state.ledger.get(v.id);
        if (r) r.audit = v.result;
      }
    };

    log(
      'Investigate · ' +
        CONFIG.LENSES.length +
        ' lenses · ≤' +
        CONFIG.MAX_WAVES +
        ' waves × ≤' +
        CONFIG.MAX_LANES +
        ' lanes · probes ' +
        CONFIG.TIER.probe +
        '/' +
        CONFIG.EFFORT.probe +
        ' · brainer ' +
        CONFIG.TIER.brainer,
    );
    const seedLanes         = CONFIG.LENSES.map((lens, i) => ({ id: 'w0-' + (i + 1), kind: 'pursue', question: lens }));
    let results = await parallel(seedLanes.map((l) => () => runProbe(l, goal, scopeLine)));
    if (!results.filter(Boolean).length) return { status: 'FAILED', detail: 'all wave-0 probes died — nothing to reason over' };
    ingest(state, results, 0);
    await auditNew(0);

    let coord                  = null;
    let stopReason = 'wave-cap';
    let dry = 0;
    for (let wave = 1; wave <= CONFIG.MAX_WAVES; wave++) {
      if (budget.total && budget.remaining() < 80000) {
        stopReason = 'budget';
        break;
      }
      const ledgerRows                     = [...state.ledger.values()].map((r) => ({
        id: r.id,
        s: r.statement,
        status: statusOf(r),
        files: r.files.length,
        survived: r.survived,
        audit: r.audit,
      }));
      coord = await runBrainer({
        goal,
        scopeLine,
        wave,
        maxWaves: CONFIG.MAX_WAVES,
        ledgerRows,
        openLeads: [...state.leads.values()],
        maxLanes: CONFIG.MAX_LANES,
      });
      if (!coord) {
        stopReason = 'brainer-dead';
        break;
      }
      for (const id of coord.dropLeads || []) state.leads.delete(id);
      if (coord.stop && coord.stop.done) {
        stopReason = 'brainer-done: ' + (coord.stop.reason || '');
        break;
      }
      const lanes = (coord.lanes || []).slice(0, CONFIG.MAX_LANES);
      if (!lanes.length) {
        stopReason = 'no-lanes';
        break;
      }
      results = await parallel(lanes.map((l) => () => runProbe(l, goal, scopeLine)));
      const fresh = ingest(state, results, wave);
      await auditNew(wave);
      log('Wave ' + wave + ': ' + lanes.length + ' lane(s) → ' + fresh + ' fresh claim(s) · ledger ' + state.ledger.size);
      if (!fresh) {
        if (++dry >= 2) {
          stopReason = 'dry';
          break;
        }
      } else dry = 0;
    }

    const keyIds = coord ? coord.keyClaimIds || [] : [];
    const conf = computedConfidence(state, keyIds);
    const claimsOut                 = [...state.ledger.values()].map((r) => ({
      id: r.id,
      statement: r.statement,
      status: statusOf(r),
      anchors: r.anchors,
      files: r.files,
      survived: r.survived,
      audit: r.audit,
    }));
    const synth = await runSynthesiser({
      goal,
      stopReason,
      keyIds,
      conf,
      reportOut: CONFIG.REPORT_OUT,
      resultSoFarText: coord ? coord.resultSoFar : '(brainer dead — reason from the ledger alone)',
      claimsOut,
      openLeads: [...state.leads.values()],
    });
    const rank                             = { low: 0, medium: 1, high: 2 };
    const finalConf             = synth ? (rank[synth.confidence] <= rank[conf] ? synth.confidence : conf) : conf;
    log(
      'Investigate done · ' +
        stopReason +
        ' · ' +
        state.ledger.size +
        ' claims · confidence ' +
        finalConf +
        (synth ? '' : ' · ⚠ DEGRADED (synth died)'),
    );
    return {
      status: 'DONE',
      mode: 'investigate',
      goal,
      stopReason,
      answer: synth ? synth.answer : (coord && coord.resultSoFar) || 'DEGRADED: no synthesis and no coord — see claims',
      confidence: finalConf,
      computedConfidence: conf,
      keyClaimIds: keyIds,
      claims: claimsOut,
      openLeads: [...state.leads.values()],
      report: synth ? synth.report : null,
      reportOut: CONFIG.REPORT_OUT,
      degraded: !synth || !coord,
    };
  }

  // ─── WALK — thread walk + ledger spine, folded into the wave review (source lines 397-731) ─────
  async runWalk()                                     {
    log(
      'Wave walker · report=' +
        CONFIG.REPORT_PATH +
        ' · sensors=' +
        CONFIG.TIER.sliceSensor +
        '/' +
        CONFIG.EFFORT.sliceSensor +
        '→' +
        CONFIG.SENSOR_ESCALATE +
        ' · walkers=' +
        CONFIG.TIER.threadWalker +
        ' · judges=' +
        CONFIG.TIER.anomalyJudge,
    );

    const reportPath = CONFIG.REPORT_PATH          ;
    const branch = CONFIG.BRANCH;
    const scout                  = await runScout({
      reportPath,
      branch,
      walkerDoc: CONFIG.WALKER_DOC,
      maxFieldsPerJob: CONFIG.MAX_FIELDS_PER_JOB,
      charter: CONFIG.CHARTER,
      invariants: CONFIG.INVARIANTS,
      invariantsDoc: CONFIG.INVARIANTS_DOC,
    });
    if (!scout) return { status: 'FAILED', detail: 'scout died twice' };
    if (!(scout.changedFiles || []).length)
      return {
        status: 'FAILED',
        detail:
          'scout resolved an EMPTY changed-file set — no merge SHA found in ' +
          reportPath +
          (branch ? ' / empty branch diff' : '') +
          '; a walk over nothing must never return a verdict',
      };
    const threads               = (scout.threads || []).concat(CONFIG.EXTRA_THREADS                );
    // ZERO-THREAD GUARD — the empty-diff guard above states the law ("a walk over
    // nothing must never return a verdict") and this is the SAME law one altitude up:
    // a scout that enumerates ZERO threads over a NON-EMPTY diff has not proven the
    // diff is safe — it has proven the scout could not read it. Without this, an
    // empty enumeration renders as SMOOTH SAILING: "nothing found" reported as
    // "nothing wrong", by the very instrument the gates trust most.
    if (!threads.length)
      return {
        status: 'FAILED',
        detail:
          'scout enumerated ZERO threads over a NON-EMPTY diff (' +
          (scout.changedFiles || []).length +
          ' changed files) — that is a SCOUT FAILURE, not a clean walk. An empty enumeration is never a verdict.',
      };
    const fields = scout.fields || [];
    let gateFiles = scout.gateFiles || [];
    log(
      'Scout: ' +
        threads.length +
        ' threads · ' +
        fields.length +
        ' type-fields · ' +
        (scout.jobs || []).length +
        ' slice jobs · ' +
        gateFiles.length +
        ' gate files · changed: ' +
        (scout.changedFiles || []).length +
        ' files',
    );

    // INVARIANT REGISTRY FEATURE (§ 2.1) — THE FLOOR: CONFIG.INVARIANTS defaults to [], so
    // computeArmedInvariants trivially returns [] and armedInvariants.length is 0 for every walk that
    // never supplied args.invariants — zero hunters dispatch, zero coverageCritic calls, zero extra log
    // lines below (this ONE line only fires when the registry is non-empty).
    const invariantById = new Map(CONFIG.INVARIANTS.map((inv) => [inv.id, inv]));
    const armedInvariants = computeArmedInvariants(CONFIG.INVARIANTS, scout.changedFiles || [], scout.armedInvariants || []);
    if (CONFIG.INVARIANTS.length)
      log(
        'Invariant registry: ' +
          CONFIG.INVARIANTS.length +
          ' registered · ' +
          armedInvariants.length +
          ' armed' +
          (armedInvariants.length ? ' (' + armedInvariants.map((a) => a.id).join(', ') + ')' : ''),
      );

    // E3: diff-scoped gate sweep — skip R6/R7's repo-wide gate population entirely when the diff touches
    // no resolver/auth/service surface (LOUD skip, fail-safe classifier; the diff-scoped security auditor
    // below ALWAYS runs regardless). args.fullGateSweep:true forces the full sweep.
    const gateRelevant = isGateRelevant(scout.changedFiles || [], fields.length, (scout.jobs || []).length);
    let gateSweepSkipped = false;
    if (!gateRelevant && !CONFIG.FULL_GATE_SWEEP) {
      gateFiles = [];
      gateSweepSkipped = true;
      log('gate sweep SKIPPED (diff touches no resolver/auth/service surface; fullGateSweep to force) — R6/R7 not evaluated this walk');
    }

    const authOk =
      typeof scout.authRule === 'string' &&
      scout.authRule.includes('THERAPIST') &&
      scout.authRule.includes('SUPERVISOR') &&
      scout.authRule.length >= 120;
    if (!authOk)
      log('⚠ scout returned no usable § Auth Pattern extract — R6/second-opinion run on AUTH_RULE_FALLBACK (verify it against {project}/CLAUDE.md)');
    const authRule = authOk
      ? '{project}/CLAUDE.md § Auth Pattern (live, scout-extracted): "' + scout.authRule + '"'
      : AUTH_RULE_FALLBACK;

    // Enforce the sensor cap; name any dropped fields (honest coverage).
    let jobs             = (scout.jobs || []).flatMap((j) =>
      (j.fieldIds || []).length <= CONFIG.MAX_FIELDS_PER_JOB
        ? [j]
        : j.fieldIds.reduce((acc            , id, i) => {
            const b = Math.floor(i / CONFIG.MAX_FIELDS_PER_JOB);
            (acc[b] = acc[b] || { ...j, jobId: j.jobId + '-' + (b + 1), fieldIds: [] }).fieldIds.push(id);
            return acc;
          }, []),
    );
    let droppedFieldIds           = [];
    if (jobs.length + gateFiles.length > CONFIG.MAX_SENSORS) {
      const keep = Math.max(0, CONFIG.MAX_SENSORS - gateFiles.length);
      droppedFieldIds = jobs.slice(keep).flatMap((j) => j.fieldIds || []);
      jobs = jobs.slice(0, keep);
      if (droppedFieldIds.length)
        log('⚠ sensor cap ' + CONFIG.MAX_SENSORS + ': DROPPED slice jobs — fields reported UNSENSED: ' + droppedFieldIds.join(', '));
    }

    // ─── Phase 1: Walk (thread walkers) + Sense (ledger sensors + gate sweeps) + Hunt (armed invariants)
    // — one parallel barrier. invariantHunter dispatch is appended LAST, after the security auditor, so
    // the existing threads/jobs/gates/security slice math (nT/nJ/nG) is untouched — a strictly additive
    // tail, exactly like the floor invariant demands. ─────
    const fieldById = new Map(fields.map((f) => [f.id, f]));
                                                                                                
    const walked            = await parallel([
      ...threads.map((t) => () => runThreadWalker({ walkerDoc: CONFIG.WALKER_DOC, thread: t, charter: CONFIG.CHARTER })                    ),
      ...jobs.map((j) => () => {
        const assigned = j.fieldIds.map((id) => {
          const f = fieldById.get(id);
          return { fieldId: id, field: f && f.field, sdlTypeToken: f && f.sdl && f.sdl.typeToken };
        });
        return runSliceSensor({ jobId: j.jobId, kind: j.kind, files: j.files, hint: j.hint, assigned })                    ;
      }),
      ...gateFiles.map((f) => () => runGateSweep({ file: f })                    ),
      () =>
        runSecurityAuditor({
          securityDoc: CONFIG.SECURITY_DOC,
          changedFiles: scout.changedFiles,
          branch,
          mergeShas: scout.mergeShas || [],
        })                    ,
      ...armedInvariants.map((ai) => () => {
        const inv = invariantById.get(ai.id);
        return (inv ? runInvariantHunter({ invariant: inv, matchedFiles: ai.matchedFiles }) : Promise.resolve(null))                    ;
      }),
    ]);
    const nT = threads.length;
    const nJ = jobs.length;
    const nG = gateFiles.length;
    const nInv = armedInvariants.length;
    const walks = (walked.slice(0, nT)                      ).filter((x)               => !!x);
    const sliceResults = (walked.slice(nT, nT + nJ)                        ).filter((x)                 => !!x);
    const gates                     = (walked.slice(nT + nJ, nT + nJ + nG)                           )
      .filter((x)                    => !!x)
      .flatMap((s) => (s.gates || []).map((g) => ({ ...g, file: s.file })));
    const security = (walked[nT + nJ + nG]                      ) || null;
    const secFindings = security ? security.findings || [] : [];
    const undeclaredReads = sliceResults.flatMap((r) => r.undeclaredReads || []);
    const hunterResults = (walked.slice(nT + nJ + nG + 1, nT + nJ + nG + 1 + nInv)                                 ).filter(
      (x)                          => !!x,
    );
    if (armedInvariants.length)
      log(
        'Invariant hunt: ' +
          hunterResults.length +
          '/' +
          armedInvariants.length +
          ' hunter(s) returned · ' +
          hunterResults.reduce((n, r) => n + (r.findings || []).length, 0) +
          ' finding(s)',
      );

    // Zip slices into cards (mechanical, zero tokens)
    const { cards, unsensed } = zipCards(fields, jobs, sliceResults, droppedFieldIds);
    if (unsensed.length) log('⚠ UNSENSED fields (no card): ' + unsensed.join(', '));
    log(
      'Walked: ' +
        walks.length +
        '/' +
        nT +
        ' threads · ' +
        cards.length +
        ' cards from ' +
        sliceResults.length +
        '/' +
        nJ +
        ' jobs · ' +
        gates.length +
        ' gates · undeclared reads: ' +
        undeclaredReads.length +
        ' · security: ' +
        (security ? secFindings.length + ' finding(s)' : 'AUDIT DIED'),
    );

    // ─── Phase 2: the ledger diff — mechanical rules, zero tokens; R9-INV (hunter findings, zero when
    // the registry is absent/empty) concatenated on — same array, same downstream pipeline ─────
    const anomalies            = computeAnomalies(cards, undeclaredReads, gates, authRule).concat(computeInvariantAnomalies(hunterResults));
    const counts = ruleCounts(anomalies);
    log('Ledger diff: ' + anomalies.length + ' anomalies (' + Object.entries(counts).map(([k, v]) => k + ':' + v).join(' ') + ')');

    // ─── Phase 3: judges (ledger anomalies) + digests; Opus second opinion on killed security/near-certain ─
    const cardById = new Map(cards.map((c) => [c.id, c]));
    const byRule                            = {};
    for (const x of anomalies) (byRule[x.rule] = byRule[x.rule] || []).push(x);
    const meaning = ruleMeaning(authRule);
    const judgeJobs = Object.entries(byRule).flatMap(([rule, list]) =>
      chunk(list, 6).map((grp, i) => ({ rule: rule                   , grp, i })),
    );
    const digestJobs = cards.length
      ? (scout.territories || [])
          .map((t) => ({
            territory: t,
            slice:
              t === 'Cortex'
                ? cards.filter((c) => c.producer && String(c.producer.writer || '').toLowerCase().includes('cortex')).map((c) => project(c, 'Cortex'))
                : cards.map((c) => project(c, t === 'BE' ? 'BE' : 'FE')),
          }))
          .filter((j) => j.slice.length)
      : [];
    // INVARIANT REGISTRY FEATURE (§ 2.4) — the external denominator, dispatched alongside judges/digests
    // (all three depend only on Phase-1 outputs, none on each other) so it completes before the final
    // judge needs it. Only runs when the registry is non-empty — an absent/empty registry dispatches
    // NOTHING here, honoring the floor invariant exactly.
    const armedIds = armedInvariants.map((a) => a.id);
    const unarmedInvariants = CONFIG.INVARIANTS.filter((inv) => !armedIds.includes(inv.id)).map((inv) => ({ id: inv.id, territory: inv.territory }));
    const [judgeResults, digestResults, coverageCriticResult] = await Promise.all([
      parallel(
        judgeJobs.map((j) => () => {
          const ctxCards = [...new Set(j.grp.map((x) => x.cardId).filter((id)               => !!id))]
            .map((id) => cardById.get(id))
            .filter((c)            => !!c);
          return runAnomalyJudge(
            { rule: j.rule, ruleMeaning: meaning[j.rule], sec: SECURITY_RULES.includes(j.rule) || j.rule === 'R9-INV', instances: j.grp, ctxCards },
            j.i,
          );
        }),
      ),
      parallel(digestJobs.map((j) => () => runTerritoryDigest({ territory: j.territory, slice: j.slice, charter: CONFIG.CHARTER }))),
      CONFIG.INVARIANTS.length
        ? runCoverageCritic({
            changedFiles: scout.changedFiles || [],
            threadNames: threads.map((t) => ({ id: t.id, type: t.type, name: t.name })),
            hunterCoverage: hunterResults.map((r) => ({ invariantId: r.invariantId, coverage: r.coverage })),
            armedIds,
            unarmedInvariants,
            unsensed,
          })
        : Promise.resolve(null),
    ]);
    let verdicts = judgeResults.filter((r)                             => !!r).flatMap((r) => r.verdicts || []);
    const digests = digestResults.filter((d)                             => !!d);
    const coverageGaps                = coverageCriticResult ? coverageCriticResult.gaps || [] : [];
    if (CONFIG.INVARIANTS.length)
      log('Coverage critic: ' + (coverageCriticResult ? coverageGaps.length + ' gap(s) named' : 'DIED — a coverage hole'));
    const anomalyById = new Map(anomalies.map((x) => [x.id, x]));
    // killed security/near-certain (existing) — a wrong KILL hides here.
    const killedEscalatable = verdicts
      .filter((v) => v.verdict === 'FALSE')
      .filter((v) => {
        const x = anomalyById.get(v.anomalyId);
        if (!x) return false;
        if (SECURITY_RULES.includes(x.rule) && ['high', 'critical'].includes(x.severityHint)) return true;
        return NEAR_CERTAIN.includes(x.rule);
      });
    // INVARIANT REGISTRY FEATURE (§ 2.3, escalation symmetry) — confirmed R9-INV high/critical (new) —
    // a wrong CONFIRM hides here, the opposite direction. [] whenever no R9-INV anomaly was ever raised.
    const confirmedInvEscalatable = verdicts.filter((v) => {
      if (v.verdict !== 'CONFIRMED') return false;
      const x = anomalyById.get(v.anomalyId);
      return !!x && x.rule === 'R9-INV' && ['high', 'critical'].includes(x.severityHint);
    });
    const escalatable = [...killedEscalatable, ...confirmedInvEscalatable];
    if (escalatable.length) {
      log(
        'Escalation: ' +
          escalatable.length +
          ' killed security/near-certain' +
          (confirmedInvEscalatable.length ? ' or confirmed R9-INV high/critical' : '') +
          ' verdict(s) → ' +
          CONFIG.TIER.secondOpinion +
          ' second opinion',
      );
      const killedChunks = chunk(killedEscalatable, 4);
      const confirmedChunks = chunk(confirmedInvEscalatable, 4);
      const second = await parallel([
        ...killedChunks.map((grp, i) => () =>
          runSecondOpinion({ authRule, items: grp.map((v) => ({ verdict: v, anomaly: anomalyById.get(v.anomalyId) })) }, i),
        ),
        ...confirmedChunks.map((grp, i) => () =>
          runSecondOpinion(
            { authRule, items: grp.map((v) => ({ verdict: v, anomaly: anomalyById.get(v.anomalyId) })), direction: 'confirmed' },
            killedChunks.length + i,
          ),
        ),
      ]);
      const overrides = new Map(
        second
          .filter((r)                             => !!r)
          .flatMap((r) => r.verdicts || [])
          .map((v) => [v.anomalyId, v]         ),
      );
      verdicts = verdicts.map((v) => {
        const o = overrides.get(v.anomalyId);
        if (!o) return v;
        if (v.verdict === 'FALSE' && o.verdict !== 'FALSE')
          return { ...o, why: '[OVERRIDE by ' + CONFIG.TIER.secondOpinion + '] ' + (o.why || '') };
        if (v.verdict === 'CONFIRMED' && o.verdict === 'FALSE')
          return { ...o, why: '[RE-EXAMINED by ' + CONFIG.TIER.secondOpinion + ', KILLED] ' + (o.why || '') };
        return v;
      });
    }
    // WALK TELEMETRY (DEBUG STEP) — snapshot the second-opinion override markers HERE, before Phase 3.5's
    // final-judge reinstatement can potentially overwrite a `why` string that also got reinstated. 0/0
    // naturally when escalation never ran (verdicts never got these markers).
    const secondOpinionOverturned = verdicts.filter((v) => (v.why || '').startsWith('[OVERRIDE')).length;
    const secondOpinionReexaminedKilled = verdicts.filter((v) => (v.why || '').startsWith('[RE-EXAMINED')).length;
    let confirmed = verdicts.filter((v) => v.verdict === 'CONFIRMED');
    let unproven = verdicts.filter((v) => v.verdict === 'UNPROVEN');
    let killed = verdicts.filter((v) => v.verdict === 'FALSE');
    log(
      'Judged: ' +
        confirmed.length +
        ' confirmed · ' +
        killed.length +
        ' false · ' +
        unproven.length +
        ' unproven · thread walks: ' +
        walks.length +
        ' · digest findings: ' +
        digests.reduce((n, d) => n + d.findings.length, 0),
    );

    // ─── Phase 3.5: final judgment — ONE Opus rules the whole walk before anything is written ─────
    const walksBrief = walks.map((w) => ({ id: w.threadId, name: w.name, flow: w.flow, defects: w.defects, notes: w.notes }));
    const killedWithAnomaly = killed.map((v) => ({ anomalyId: v.anomalyId, why: v.why, anomaly: anomalyById.get(v.anomalyId) }));
    let finalJudge                  = await runFinalJudge({
      walksBrief,
      confirmed,
      unproven,
      killedWithAnomaly,
      digests,
      securityDoc: CONFIG.SECURITY_DOC,
      security,
      walksLen: walks.length,
      threadsLen: threads.length,
      cardsLen: cards.length,
      unsensed,
      charter: CONFIG.CHARTER,
      coverageGaps,
    });
    let finalJudgeReinstatedCount = 0;
    if (finalJudge && (finalJudge.reinstated || []).length) {
      const re = new Map(finalJudge.reinstated .map((r) => [r.anomalyId, r]));
      verdicts = verdicts.map((v) =>
        re.has(v.anomalyId) && v.verdict === 'FALSE'
          ? { ...v, verdict: 'CONFIRMED'         , why: '[REINSTATED by final judge] ' + re.get(v.anomalyId) .why }
          : v,
      );
      confirmed = verdicts.filter((v) => v.verdict === 'CONFIRMED');
      unproven = verdicts.filter((v) => v.verdict === 'UNPROVEN');
      killed = verdicts.filter((v) => v.verdict === 'FALSE');
      finalJudgeReinstatedCount = finalJudge.reinstated .length;
      log('Final judge reinstated ' + finalJudge.reinstated .length + ' killed verdict(s)');
    }
    if (finalJudge) log('Final judgment: ' + finalJudge.verdict + ' · ' + finalJudge.missedRisks.length + ' missed risk(s)');

    // ─── Phase 4: Fold — merge thread walks + confirmed anomalies + digests → the wave review ─────
    const coverageSummary =
      'threads walked: ' +
      walks.length +
      '/' +
      threads.length +
      ' · fields sensed: ' +
      cards.length +
      ' · UNSENSED: ' +
      (unsensed.length ? unsensed.join(', ') : 'none') +
      ' · gates: ' +
      (gateSweepSkipped ? 'SKIPPED (diff-scoped)' : gates.length) +
      ' · ledger anomalies: ' +
      anomalies.length +
      ' → confirmed ' +
      confirmed.length +
      ', false ' +
      killed.length +
      ', unproven ' +
      unproven.length +
      ' · security: ' +
      (security ? secFindings.length + ' finding(s) over ' + (security.categoriesSwept || []).length + ' categories' : 'AUDIT DIED') +
      (CONFIG.INVARIANTS.length
        ? ' · invariants: ' +
          CONFIG.INVARIANTS.length +
          ' registered, ' +
          armedInvariants.length +
          ' armed (' +
          (armedIds.length ? armedIds.join(', ') : 'none') +
          ') · coverage gaps: ' +
          coverageGaps.length
        : '');

    // WALK TELEMETRY (DEBUG STEP) — assembled HERE, immediately before the runFold() call, so it is in
    // scope for both the success return below and the `fold died twice` FAILED return (the single
    // highest-value floor fix per tmp/walker-debug-design.md §5: today that path returns nothing but a
    // detail string).
    //
    // computeExpectedSeats — the WALK-mode-relevant subset THIS WALK actually SCHEDULED, derived from
    // real per-walk dispatch decisions (jobs-by-kind, gateFiles, judgeJobs, digestJobs, escalatable,
    // armedInvariants, CONFIG.INVARIANTS), not a static universal seat list — a walk with no GraphQL
    // surface / a gate-free diff / zero anomalies legitimately dispatches zero sensors/gates/judges/
    // digests (the documented floor, walker.md: "the floor never regresses"), so those seats are never
    // flagged as a false "gap." `includeFold` is false pre-fold-call (fold hasn't been dispatched yet at
    // that point) and true in the post-fold patch below, once it genuinely has been.
    const computeExpectedSeats = (includeFold         )           =>
      ['scout', 'walk', 'security', 'final-judge']
        .concat(includeFold ? ['fold'] : [])
        .concat(jobs.some((j) => j.kind === 'producer') ? ['producer'] : [])
        .concat(jobs.some((j) => j.kind === 'consumer') ? ['consumer'] : [])
        .concat(jobs.some((j) => j.kind === 'cortex') ? ['cortex'] : [])
        .concat(gateFiles.length && !gateSweepSkipped ? ['gates'] : [])
        .concat(judgeJobs.length ? ['judge'] : [])
        .concat(digestJobs.length ? ['digest'] : [])
        .concat(escalatable.length ? ['2nd-opinion'] : [])
        .concat(armedInvariants.length ? ['invariant-hunt'] : [])
        .concat(CONFIG.INVARIANTS.length ? ['coverage-critic'] : []);

    let debugRecord                         ;
    if (CONFIG.debug) {
      try {
        debugRecord = assembleDebugRecord({
          reportPath,
          invariants: CONFIG.INVARIANTS,
          armedInvariants,
          unarmedInvariants,
          seatTally: SEAT_TALLY,
          expectedSeats: computeExpectedSeats(false),
          threads,
          walks,
          jobs,
          cards,
          hunterResults,
          digestJobsLen: digestJobs.length,
          digests,
          security,
          coverageCriticResult,
          confirmed,
          killed,
          unproven,
          secondOpinionDispatched: escalatable.length,
          secondOpinionOverturned,
          secondOpinionReexaminedKilled,
          finalJudge,
          finalJudgeReinstated: finalJudgeReinstatedCount,
          unsensed,
          gateSweepSkipped,
          coverageGaps,
        });
      } catch (e) {
        // outermost floor — even a bug in the assembler's own scaffolding must never break the walk.
        log('⚠ debugRecord assembly failed entirely: ' + ((e         ) && (e         ).message ? (e         ).message : String(e)));
        debugRecord = {
          schemaVersion: 1,
          mode: 'walk',
          reportPath,
          degraded: true,
          gaps: ['full assembly failed: ' + ((e         ) && (e         ).message ? (e         ).message : String(e))],
          armedInvariants: { registered: 0, armed: [], unarmed: [] },
          seats: {},
          seatsExpectedButAbsent: [],
          emptyResults: {
            threadsWalked: 0,
            threadsExpected: 0,
            sensorsWithCards: 0,
            sensorsExpected: 0,
            hunterFindingsTotal: 0,
            huntersReturned: 0,
            huntersExpected: 0,
            digestsWithFindings: 0,
            digestsExpected: 0,
            securityDied: false,
            coverageCriticDied: false,
            foldDied: false,
          },
          judgeStats: {
            confirmed: 0,
            falseCount: 0,
            unproven: 0,
            secondOpinionDispatched: 0,
            secondOpinionOverturned: 0,
            secondOpinionReexaminedKilled: 0,
            finalJudgeReinstated: 0,
            finalJudgeVerdict: null,
            finalJudgeDied: true,
          },
          coverage: { unsensedFields: [], gateSweepSkipped: false, coverageGaps: [] },
          tokenAttribution: null,
        };
      }
    }
    // renderTelemetryMd is self-floored (never throws — see utils/index.ts), so no wrapping try/catch
    // is needed here; `debug:false` or an assembly failure both fall through to '' cleanly.
    const telemetryMd = CONFIG.debug && debugRecord ? renderTelemetryMd(debugRecord) : '';

    const fold                 = await runFold({
      reportPath,
      walks,
      confirmed,
      unproven,
      killedCount: killed.length,
      digests,
      security,
      coverageSummary,
      finalJudge,
      coverageGaps,
      telemetryMd,
    });
    if (CONFIG.debug && debugRecord) {
      try {
        // refresh seats/seatsExpectedButAbsent now that fold has genuinely been dispatched (whether it
        // died or succeeded, its tally now exists in SEAT_TALLY) — see computeExpectedSeats above.
        debugRecord.seats = Object.fromEntries(Object.entries(SEAT_TALLY).map(([k, v]) => [k, { ...v }]));
        debugRecord.seatsExpectedButAbsent = computeExpectedSeats(true).filter((s) => !(s in debugRecord .seats));
        debugRecord.emptyResults.foldDied = !fold;
      } catch (e) {
        debugRecord.degraded = true;
        debugRecord.gaps.push('post-fold patch failed: ' + ((e         ) && (e         ).message ? (e         ).message : String(e)));
      }
    }
    if (!fold)
      return {
        status: 'FAILED',
        detail: 'fold died twice',
        threads: threads.length,
        anomalies: anomalies.length,
        confirmed: confirmed.length,
        debugRecord,
      };

    const ledger             = {
      report: reportPath,
      headSha: scout.headSha,
      territories: scout.territories,
      changedFiles: scout.changedFiles,
      mergeShas: scout.mergeShas,
      threads,
      walks,
      cards,
      gateCards: gates,
      undeclaredReads,
      anomalies,
      verdicts,
      digests,
      security: security || null,
      coverage: coverageSummary,
      armedInvariants,
      coverageGaps,
    };
    log('Wave walker complete · ' + fold.verdict + ' · ' + coverageSummary + ' · ledger in result (persist to ' + CONFIG.LEDGER_PATH + ')');
    return {
      status: 'DONE',
      verdict: fold.verdict,
      actionItems: fold.actionItems,
      review: fold.review,
      threads: threads.length,
      threadsWalked: walks.length,
      ledgerAnomalies: anomalies.length,
      anomaliesByRule: counts,
      confirmed: confirmed.map((v) => ({ id: v.anomalyId, severity: v.severity, what: v.what, location: v.location })),
      unproven: unproven.length,
      killedAsFalse: killed.length,
      overrides: verdicts.filter((v) => (v.why || '').startsWith('[OVERRIDE')).length,
      digestFindings: digests.reduce((n, d) => n + d.findings.length, 0),
      security: security ? { findings: secFindings, categoriesSwept: security.categoriesSwept || [], summary: security.summary || '' } : null,
      finalJudge: finalJudge ? { verdict: finalJudge.verdict, missedRisks: finalJudge.missedRisks.length, reinstated: (finalJudge.reinstated || []).length } : null,
      unsensedFields: unsensed,
      coverage: coverageSummary,
      reportPath,
      ledgerTarget: CONFIG.LEDGER_PATH,
      ledger,
      invariantsArmed: armedIds,
      coverageGaps: coverageGaps.length,
      debugRecord,
    };
  }
}

// ── entry — the Workflow harness wraps this file in an async scope and awaits its return ──
const ww = new WaveWalker()
return await ww.run()
