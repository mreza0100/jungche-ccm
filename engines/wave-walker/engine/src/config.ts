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
import type { Effort, InvariantSpec, ProjectProfile, RawArgs, Tier } from './types/index.js';

export type Mode = 'walk' | 'verify' | 'manifest-verify' | 'investigate';

const DEFAULT_LENSES = [
  'DIRECT — the goal head-on',
  'SKEPTIC — hunt the evidence that would make the obvious answer WRONG',
  'BLAST-RADIUS — callers, consumers, config, and tests the goal implicates',
];

export class Configs {
  rawArgs: RawArgs;
  mode: Mode;

  // ── fixed doc/path constants (source lines 34, 51) ──
  WALKER_DOC = '.claude/commands/wave/walker.md';
  SECURITY_DOC = '.claude/commands/audit/security.md';
  // D3 (audit 2026-07-28): the hygiene standard the thread walkers' integration-delta lens applies.
  // walker.md § Role: Walker step 4 cited this file from day one, but no seat was ever handed it —
  // the 341-line standard had never been read by a walk (a phantom citation, found because the walk
  // certified hygiene it never checked).
  HYGIENE_DOC = '.claude/commands/audit/code-hygiene.md';
  // INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — provenance pointer cited in
  // the scout prompt; the JS engine never reads this file itself (no fs access in src/ — confirmed by
  // grep). The registry's DATA arrives structured via args.invariants (see INVARIANTS below); this doc
  // path is what Professor's caller wiring parses into that JSON.
  INVARIANTS_DOC = '.claude/commands/wave/walker-invariants.md';

  // ── WALK mode ──
  REPORT_PATH: string | null;
  BRANCH: string | null;
  LEDGER_PATH: string | null;
  MAX_FIELDS_PER_JOB: number;
  MAX_SENSORS: number;
  // D2 SECURITY FAN-OUT (audit 2026-07-28) — changed files per security-auditor slice. Security was the
  // only lens with no fan-out; coverage now scales with diff size like every other seat.
  SECURITY_FILES_PER_AUDITOR: number;
  EXTRA_THREADS: unknown[];
  // E3 gate-conditional dispatch override: `fullGateSweep: true` forces the repo-wide gate-file sweep
  // regardless of the diff classifier (engine.ts § isGateRelevant). Ported from the proven v3 variant.
  FULL_GATE_SWEEP: boolean;
  // CHARTER — the walk's caller-supplied duty note (walk mode only). Non-empty → a clearly-delimited
  // 'WALK CHARTER' block is appended to the scout / thread-walker / territory-digest / final-judge
  // prompts (conditional-append, zero bytes when empty — the same pattern as verify-mode's QUESTION).
  // The charter ADDS focus on top of the standard enumeration/judgment, never replaces it, and never
  // touches the security-auditor, gate-sweep, or sensor prompts. No output schema changes anywhere.
  CHARTER: string;
  // INVARIANT REGISTRY FEATURE (tmp/wave-walker-investigation.md § 2.1) — args.invariants, validated and
  // structured. [] (the default — no args.invariants) is THE FLOOR: no hunter/coverageCritic dispatched,
  // scout prompt gets zero added bytes, review/ledger byte-identical to today. Malformed shape throws
  // loudly (never a silent partial registry), same discipline as `charter`/`agents`.
  INVARIANTS: InvariantSpec[];
  // WALK TELEMETRY (DEBUG STEP) — mirrors rr's config.ts:383 debugger flag exactly: DEFAULT ON. A
  // structured `debugRecord` + a mechanically-rendered `### Walk Telemetry` block give "good data of
  // what's half broken" after a few runs, at zero LLM cost (see runtime.ts SEAT_TALLY, engine.ts
  // assembleDebugRecord, utils/index.ts renderTelemetryMd). `debug:false` reproduces today's behavior
  // byte-for-byte — no record, no telemetry block, zero added bytes anywhere.
  debug: boolean;
  DEBUG_PATH: string | null;

  // ── PROJECT PROFILE (universal-bundle refactor) — args.project, validated; undefined when absent (the
  // floor: every project-specific site runs generic/degraded-loud — engine.ts computeGateArming,
  // constants.ts DEADNESS_BAR, every prompts.ts repoRoot/authDoc site). REPO_ROOT is a convenience
  // pre-derivation (repoRoot always resolves — default '.', the working directory) so every call site
  // reads ONE field instead of repeating `PROJECT?.repoRoot || '.'`. ──
  PROJECT: ProjectProfile | undefined;
  REPO_ROOT: string;

  // ── VERIFY / MANIFEST-VERIFY mode ──
  MANIFEST_PATH: string | null;
  CLAIMS: unknown[];
  VOTES: number;
  QUESTION: string;
  MAX_CLAIMS: number;
  SOLO_THRESHOLD: number;

  // ── INVESTIGATE mode ──
  GOAL: string;
  SCOPE: string[] | null;
  LENSES: string[];
  MAX_WAVES: number;
  MAX_LANES: number;
  REPORT_OUT: string | null;

  // ── TIME CHECKPOINT — the runtime window the walk must finish inside, and the slice held back for
  // terminal judgment. The graph cannot read a clock (the Workflow runtime forbids it), so both are
  // declared here and compared against a clockProbe reading. See engine.ts § TIME CHECKPOINT. ──
  WINDOW_SECONDS: number;
  JUDGE_RESERVE_SECONDS: number;

  // ── per-seat model TIER + reasoning EFFORT — keyed by seat name (types/agents.ts Seat). Frontier
  // seats (brainer, finalJudge, secondOpinion) default to 'opus' and warn loudly on any downgrade —
  // see the `agents` override loop below. ──
  TIER: Record<string, Tier>;
  EFFORT: Record<string, Effort>;
  // SENSOR_ESCALATE — the sliceSensor/gateSweep dead-agent escalate model (source line 39: `const
  // SENSOR_ESCALATE = 'sonnet'`). NOT an arg in the source (no `sensorEscalateModel` knob exists) and
  // NOT part of the `agents.<seat>` override system — a fixed internal constant, ported verbatim.
  SENSOR_ESCALATE: Tier = 'sonnet';

  constructor(rawArgs: unknown) {
    let parsed: unknown = rawArgs;
    if (typeof parsed === 'string') {
      try {
        parsed = JSON.parse(parsed);
      } catch (e) {
        throw new Error(
          'wave-walker: args is a string but not valid JSON: ' + ((e as Error).message || e),
        );
      }
    }
    const arg = (parsed || {}) as RawArgs;
    const hasClaims = Array.isArray(arg.claims) && (arg.claims as unknown[]).length > 0;
    if (!parsed || (!arg.reportPath && !arg.manifestPath && !arg.goal && !hasClaims))
      throw new Error(
        'wave-walker requires args.reportPath (walk), args.claims (verify), args.manifestPath (manifest-verify), or args.goal (investigate); see wave/walker.md for the contract',
      );
    this.rawArgs = arg;

    // ── mode (source lines 58, 141; the returned `mode` field for verify/manifest-verify is gated on
    // args.manifestPath truthiness alone — see source line 133) ──
    this.MANIFEST_PATH = Configs.optionalRepoPath(arg.manifestPath, 'manifestPath');
    this.CLAIMS = Array.isArray(arg.claims) ? (arg.claims as unknown[]) : [];
    this.mode = this.MANIFEST_PATH
      ? 'manifest-verify'
      : this.CLAIMS.length
        ? 'verify'
        : arg.goal
          ? 'investigate'
          : 'walk';

    // ── WALK mode config (source lines 31-53) ──
    this.REPORT_PATH = Configs.optionalRepoPath(arg.reportPath, 'reportPath');
    this.BRANCH = Configs.optionalGitRevision(arg.branch, 'branch');
    this.LEDGER_PATH =
      arg.ledgerPath !== undefined && arg.ledgerPath !== null
        ? Configs.repoPath(arg.ledgerPath, 'ledgerPath')
        : this.REPORT_PATH
          ? this.REPORT_PATH.replace(/report\.md$/, '') + 'walker-ledger.json'
          : null;
    this.MAX_FIELDS_PER_JOB = Number.isInteger(arg.maxFieldsPerJob)
      ? (arg.maxFieldsPerJob as number)
      : 18;
    this.MAX_SENSORS = Number.isInteger(arg.maxSensors) ? (arg.maxSensors as number) : 60;
    this.SECURITY_FILES_PER_AUDITOR =
      Number.isInteger(arg.securityFilesPerAuditor) && (arg.securityFilesPerAuditor as number) > 0
        ? (arg.securityFilesPerAuditor as number)
        : 12;
    this.EXTRA_THREADS = Array.isArray(arg.extraThreads) ? (arg.extraThreads as unknown[]) : [];
    this.FULL_GATE_SWEEP = arg.fullGateSweep === true; // strict — exactly the v3 variant's `!== true` gate inverted
    // charter — absent/null → '' (no-op); anything else must be a string (a mis-typed duty note must
    // fail loudly, never silently walk without its charter).
    if (arg.charter !== undefined && arg.charter !== null && typeof arg.charter !== 'string')
      throw new Error(
        "wave-walker: charter must be a string (the walk's caller-supplied duty note), got " +
          JSON.stringify(arg.charter),
      );
    this.CHARTER =
      typeof arg.charter === 'string' ? Configs.promptText(arg.charter, 'charter') : '';
    // INVARIANT REGISTRY FEATURE — absent/null → [] (THE FLOOR: no hunter/critic, byte-identical walk).
    // A non-array, or an array with a malformed entry, throws loudly — never a silently-partial registry.
    this.INVARIANTS = Configs.parseInvariants(arg.invariants);
    // WALK TELEMETRY (DEBUG STEP) — mirrors rr config.ts:383 `this.debug = bool(arg.debug, true)`,
    // DEFAULT ON. DEBUG_PATH derives from REPORT_PATH the same way LEDGER_PATH does (walker-debug.json
    // sibling); an explicit args.debugPath overrides it; both null when there's no REPORT_PATH to derive from.
    this.debug = Configs.bool(arg.debug, true);
    this.DEBUG_PATH =
      arg.debugPath !== undefined && arg.debugPath !== null
        ? Configs.repoPath(arg.debugPath, 'debugPath')
        : this.REPORT_PATH
          ? this.REPORT_PATH.replace(/report\.md$/, '') + 'walker-debug.json'
          : null;

    // PROJECT PROFILE — absent/null → undefined (THE FLOOR: gate machinery disarms, everything else
    // stays generic). A malformed shape throws loudly — same discipline as `charter`/`agents`/`invariants`.
    this.PROJECT = Configs.parseProject(arg.project);
    this.REPO_ROOT = (this.PROJECT && this.PROJECT.repoRoot) || '.';

    // ── VERIFY / MANIFEST-VERIFY config (source lines 59-64, plus the E2 manifest-coverage lever) ──
    this.VOTES =
      Number.isInteger(arg.votes) && (arg.votes as number) > 0 ? (arg.votes as number) : 1;
    this.QUESTION =
      typeof arg.question === 'string' ? Configs.promptText(arg.question, 'question') : '';
    // E2 DEFAULT CHANGE (the port's ONE behavioral delta from the source): maxClaims 24 → 96. Proven on
    // wave.md: 96 gave full 16/16-task coverage; 24 silently dropped 55 claims / 12 tasks. args.maxClaims
    // still overrides.
    this.MAX_CLAIMS = Number.isInteger(arg.maxClaims) ? (arg.maxClaims as number) : 96;
    // E2 batching gate: a panel (claims × votes) ≤ SOLO_THRESHOLD runs SOLO exactly like the source
    // (small-panel latency + per-claim opus escalation preserved); above it, claims batch ≤4 by
    // file-cluster affinity — verifiers STAY on the verifier tier (sonnet/xhigh), never haiku
    // (measured: haiku did 2.5× the tool calls → +21% tokens, +235% latency; rejected).
    this.SOLO_THRESHOLD = Number.isInteger(arg.soloThreshold) ? (arg.soloThreshold as number) : 8;

    // ── INVESTIGATE config (source lines 142-153) ──
    this.GOAL = arg.goal != null ? Configs.promptText(String(arg.goal), 'goal') : '';
    this.SCOPE =
      Array.isArray(arg.scope) && (arg.scope as unknown[]).length
        ? Configs.promptTextArray(arg.scope, 'scope')
        : null;
    this.LENSES =
      Array.isArray(arg.lenses) && (arg.lenses as unknown[]).length
        ? Configs.promptTextArray(arg.lenses, 'lenses')
        : DEFAULT_LENSES;
    this.MAX_WAVES = Number.isInteger(arg.maxWaves) ? (arg.maxWaves as number) : 3;
    this.MAX_LANES = Number.isInteger(arg.maxLanes) ? (arg.maxLanes as number) : 5;
    this.REPORT_OUT = Configs.optionalRepoPath(arg.reportOut, 'reportOut');
    // Defaults measured against the observed starvation: the final judge was reached at 589.9s of an
    // approximately 600s window and was killed 15.9s later. 150s is the smallest reserve that lets one
    // Opus ruling over a whole wave actually land.
    this.WINDOW_SECONDS = Number.isInteger(arg.windowSeconds) ? (arg.windowSeconds as number) : 600;
    this.JUDGE_RESERVE_SECONDS = Number.isInteger(arg.judgeReserveSeconds)
      ? (arg.judgeReserveSeconds as number)
      : 150;

    // ── per-seat defaults, seeded verbatim off the source's per-call option objects. Several seats
    // deliberately SHARE one legacy arg (sensorModel/sensorEffort → sliceSensor + gateSweep;
    // verifierModel/verifierEffort → claimExtractor + claimVerifier + consistencyJudge;
    // securityEscalateModel → secondOpinion, and also the verify-mode per-claim opus escalation — see
    // agents/claimVerifier/run.ts) — exactly the source's own knob reuse, not a porting shortcut. ──
    const str = (v: unknown, d: string): string => (typeof v === 'string' && v.length ? v : d);
    const scoutModel = str(arg.scoutModel, 'sonnet') as Tier;
    const walkerModel = str(arg.walkerModel, 'sonnet') as Tier;
    const sensorModel = str(arg.sensorModel, 'haiku') as Tier;
    const sensorEffort = str(arg.sensorEffort, 'medium') as Effort;
    const judgeModel = str(arg.judgeModel, 'sonnet') as Tier;
    const digestModel = str(arg.digestModel, 'sonnet') as Tier;
    const foldModel = str(arg.foldModel, 'sonnet') as Tier;
    const securityModel = str(arg.securityModel, 'sonnet') as Tier;
    const securityEffort = str(arg.securityEffort, 'xhigh') as Effort;
    const verifierModel = str(arg.verifierModel, 'sonnet') as Tier;
    const verifierEffort = str(arg.verifierEffort, 'xhigh') as Effort;
    const probeModel = str(arg.probeModel, 'sonnet') as Tier;
    const probeEffort = str(arg.probeEffort, 'xhigh') as Effort;
    const auditModel = str(arg.auditModel, 'haiku') as Tier;
    const synthModel = str(arg.synthModel, 'sonnet') as Tier;
    // INVARIANT REGISTRY FEATURE — invariantHunter/coverageCritic default sonnet/high, same tier as
    // securityAuditor (its closest precedent: adversarial, diff/territory-scoped, non-frontier by root
    // CLAUDE.md § Model Selection's own taxonomy — spec-execution work with a defined territory+brief,
    // not open-ended judgment). No dedicated legacy `<seat>Model` arg (neither seat existed pre-feature)
    // — retuned only via `agents.<seat>`, like every seat added after the original 17.
    const invariantHunterModel: Tier = 'sonnet';
    const coverageCriticModel: Tier = 'sonnet';
    // FRONTIER-JUDGMENT SEATS — final judge, security second-opinion, investigate brainer. Durable
    // default = the 'opus' alias, per root CLAUDE.md § Model Selection; never a model literal here. A
    // limited-time frontier model rides ONLY the invocation args (finalJudgeModel / securityEscalateModel
    // / brainerModel). Security/auth judgment seats never silently downgrade below opus — see the
    // `agents` override loop below.
    const finalJudgeModel = str(arg.finalJudgeModel, 'opus') as Tier;
    const securityEscalateModel = str(arg.securityEscalateModel, 'opus') as Tier;
    const brainerModel = str(arg.brainerModel, 'opus') as Tier;
    const brainerEffort = str(arg.brainerEffort, 'xhigh') as Effort;

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
      // COLLECTOR SEAT — one `date +%s` read. No judgment, no repository access, no salience.
      clockProbe: 'haiku',
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
      clockProbe: 'medium', // one command, one integer — reasoning effort buys nothing here
    };

    // ── PER-SEAT OVERRIDE (`agents` arg) — retune any seat's model/effort without touching source,
    // mirroring rr's config.ts override loop exactly. Unknown seat / bad model / bad effort throw
    // loudly. brainer/finalJudge/secondOpinion downgrading below opus logs a loud warning, never throws
    // — those three are the frontier-judgment seats named in root CLAUDE.md § Model Selection. ──
    const VALID_TIERS: Tier[] = ['haiku', 'sonnet', 'opus'];
    const VALID_EFFORTS: Effort[] = ['low', 'medium', 'high', 'xhigh', 'max'];
    const FRONTIER_SEATS = ['brainer', 'finalJudge', 'secondOpinion'];
    const seats = Object.keys(this.TIER); // the canonical seat names — read off the default map itself (17 original + invariantHunter/coverageCritic, INVARIANT REGISTRY FEATURE)
    if (arg.agents !== undefined && arg.agents !== null) {
      if (typeof arg.agents !== 'object' || Array.isArray(arg.agents))
        throw new Error(
          'wave-walker: agents must be an object keyed by seat name, e.g. { scout: { model: "opus" } }',
        );
      const overrides = arg.agents as Record<string, unknown>;
      for (const seat of Object.keys(overrides)) {
        if (!seats.includes(seat))
          throw new Error(
            'wave-walker: unknown agent seat "' +
              seat +
              '" in `agents` — valid seats: ' +
              seats.join(', '),
          );
        const o = overrides[seat];
        if (typeof o !== 'object' || o === null || Array.isArray(o))
          throw new Error('wave-walker: agents.' + seat + ' must be an object { model?, effort? }');
        const { model, effort } = o as { model?: unknown; effort?: unknown };
        if (model !== undefined) {
          if (!VALID_TIERS.includes(model as Tier))
            throw new Error(
              'wave-walker: agents.' +
                seat +
                '.model must be one of ' +
                VALID_TIERS.join(', ') +
                ', got ' +
                JSON.stringify(model),
            );
          this.TIER[seat] = model as Tier;
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
          if (!VALID_EFFORTS.includes(effort as Effort))
            throw new Error(
              'wave-walker: agents.' +
                seat +
                '.effort must be one of ' +
                VALID_EFFORTS.join(', ') +
                ', got ' +
                JSON.stringify(effort),
            );
          this.EFFORT[seat] = effort as Effort;
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
  private static parseInvariants(raw: unknown): InvariantSpec[] {
    if (raw === undefined || raw === null) return [];
    if (!Array.isArray(raw))
      throw new Error(
        'wave-walker: invariants must be an array of registry entries, got ' + JSON.stringify(raw),
      );
    const isStringArray = (v: unknown): v is string[] =>
      Array.isArray(v) && v.every((x) => typeof x === 'string');
    return raw.map((entry, i) => {
      if (typeof entry !== 'object' || entry === null || Array.isArray(entry))
        throw new Error(
          'wave-walker: invariants[' + i + '] must be an object, got ' + JSON.stringify(entry),
        );
      const e = entry as Record<string, unknown>;
      for (const field of ['id', 'law', 'huntBrief'])
        if (typeof e[field] !== 'string' || !e[field])
          throw new Error(
            'wave-walker: invariants[' +
              i +
              '].' +
              field +
              ' must be a non-empty string, got ' +
              JSON.stringify(e[field]),
          );
      if (!isStringArray(e.territory) || !(e.territory as string[]).length)
        throw new Error(
          'wave-walker: invariants[' +
            i +
            '].territory must be a non-empty array of glob strings, got ' +
            JSON.stringify(e.territory),
        );
      if (e.triggers !== undefined && !isStringArray(e.triggers))
        throw new Error(
          'wave-walker: invariants[' +
            i +
            '].triggers must be an array of strings, got ' +
            JSON.stringify(e.triggers),
        );
      if (e.exemplars !== undefined && !isStringArray(e.exemplars))
        throw new Error(
          'wave-walker: invariants[' +
            i +
            '].exemplars must be an array of strings, got ' +
            JSON.stringify(e.exemplars),
        );
      return {
        id: e.id as string,
        law: e.law as string,
        territory: e.territory as string[],
        triggers: isStringArray(e.triggers) ? e.triggers : [],
        exemplars: isStringArray(e.exemplars) ? e.exemplars : [],
        huntBrief: e.huntBrief as string,
      };
    });
  }

  // PROJECT PROFILE — validates args.project into ProjectProfile. Absent/null → undefined (the floor:
  // gate machinery disarms — engine.ts computeGateArming — everything else stays generic). Every field is
  // individually optional (the schema itself has no required subset), so a partially-populated profile is
  // valid; only a WRONG TYPE on a present field throws — same "fail loud on malformed shape, never on
  // absence" discipline as parseInvariants above.
  private static parseProject(raw: unknown): ProjectProfile | undefined {
    if (raw === undefined || raw === null) return undefined;
    if (typeof raw !== 'object' || Array.isArray(raw))
      throw new Error('wave-walker: args.project must be an object, got ' + JSON.stringify(raw));
    const p = raw as Record<string, unknown>;
    const str = (v: unknown, field: string): string | undefined => {
      if (v === undefined) return undefined;
      if (typeof v !== 'string')
        throw new Error(
          'wave-walker: args.project.' + field + ' must be a string, got ' + JSON.stringify(v),
        );
      return Configs.promptText(v, 'project.' + field);
    };
    const strArr = (v: unknown, field: string): string[] | undefined => {
      if (v === undefined) return undefined;
      if (!Array.isArray(v) || !v.every((x) => typeof x === 'string'))
        throw new Error(
          'wave-walker: args.project.' +
            field +
            ' must be an array of strings, got ' +
            JSON.stringify(v),
        );
      return Configs.promptTextArray(v, 'project.' + field);
    };
    const pair = (
      v: unknown,
      field: string,
      keys: [string, string],
    ): { [k: string]: string } | undefined => {
      if (v === undefined) return undefined;
      if (typeof v !== 'object' || v === null || Array.isArray(v))
        throw new Error(
          'wave-walker: args.project.' +
            field +
            ' must be an object { ' +
            keys.join(', ') +
            ' }, got ' +
            JSON.stringify(v),
        );
      const o = v as Record<string, unknown>;
      const out: Record<string, string> = {};
      for (const k of keys) {
        if (typeof o[k] !== 'string' || !o[k])
          throw new Error(
            'wave-walker: args.project.' + field + '.' + k + ' must be a non-empty string',
          );
        out[k] = Configs.promptText(o[k] as string, 'project.' + field + '.' + k);
      }
      return out;
    };
    const roles = pair(p.roles, 'roles', ['owner', 'elevated']) as
      { owner: string; elevated: string } | undefined;
    const fenceLabels = pair(p.fenceLabels, 'fenceLabels', ['org', 'ownership']) as
      { org: string; ownership: string } | undefined;
    const rawRepoRoot = str(p.repoRoot, 'repoRoot');
    const rawAuthDoc = str(p.authDoc, 'authDoc');
    if (rawAuthDoc !== undefined) {
      const [docPath] = rawAuthDoc.split(' § ');
      Configs.repoPath(docPath || '', 'project.authDoc path', true);
    }
    return {
      repoRoot:
        rawRepoRoot === undefined
          ? undefined
          : Configs.repoPath(rawRepoRoot, 'project.repoRoot', true),
      authDoc: rawAuthDoc,
      authRuleFallback: str(p.authRuleFallback, 'authRuleFallback'),
      authRuleMustContain: strArr(p.authRuleMustContain, 'authRuleMustContain'),
      roles,
      resourceClasses: strArr(p.resourceClasses, 'resourceClasses'),
      fencedResourceClasses: strArr(p.fencedResourceClasses, 'fencedResourceClasses'),
      fenceLabels,
      gateResolverPattern: str(p.gateResolverPattern, 'gateResolverPattern'),
      gateSurfacePattern: str(p.gateSurfacePattern, 'gateSurfacePattern'),
      deadnessSurfaces: str(p.deadnessSurfaces, 'deadnessSurfaces'),
      stakesLine: str(p.stakesLine, 'stakesLine'),
      securityStakesLine: str(p.securityStakesLine, 'securityStakesLine'),
    };
  }

  // WALK TELEMETRY (DEBUG STEP) — a strict-typed boolean reader, mirroring rr's local `bool()` helper
  // (rr engine/src/config.ts:119: `(v: unknown, d: boolean): boolean => (typeof v === 'boolean' ? v :
  // d)`) verbatim. A static method (not a local const like the file's own `str` helper) because
  // CONFIG.debug is set early in the constructor — before `str` is declared — so a same-scope local
  // const would be a temporal-dead-zone reference; a static method has the same zero-ceremony call
  // shape and matches this file's own `Configs.parseInvariants` precedent. Strict-typed: a non-boolean
  // truthy value (e.g. the string "false") never accidentally flips the flag.
  private static bool(v: unknown, d: boolean): boolean {
    return typeof v === 'boolean' ? v : d;
  }

  // Runtime args cross both a shell-command prompt and file-write prompts. Mechanical tokens therefore
  // fail closed at construction: relative repo paths only, no parent traversal, separators that change
  // argv shape, shell metacharacters, or prompt-control bytes. The Workflow harness supplies no safe
  // shell-quoting primitive, so accepting a wider grammar would turn caller data into executable syntax.
  private static repoPath(raw: unknown, field: string, allowDot = false): string {
    if (typeof raw !== 'string' || raw.length === 0)
      throw new Error('wave-walker: args.' + field + ' must be a non-empty relative repo path');
    if (raw.length > 512)
      throw new Error('wave-walker: args.' + field + ' exceeds the 512-character path limit');
    if (raw === '.' && allowDot) return raw;
    if (
      raw.startsWith('/') ||
      raw.startsWith('~') ||
      /^[A-Za-z]:/.test(raw) ||
      raw.includes('\\') ||
      /[\u0000-\u001f\u007f`$;&|<>(){}\[\]!?*]/.test(raw)
    )
      throw new Error(
        'wave-walker: args.' + field + ' contains an absolute path, control byte, or unsafe syntax',
      );
    const segments = raw.split('/');
    if (segments.some((segment) => !segment || segment === '.' || segment === '..'))
      throw new Error('wave-walker: args.' + field + ' contains an empty, dot, or parent segment');
    return raw;
  }

  private static optionalRepoPath(raw: unknown, field: string): string | null {
    if (raw === undefined || raw === null) return null;
    return Configs.repoPath(raw, field);
  }

  private static optionalGitRevision(raw: unknown, field: string): string | null {
    if (raw === undefined || raw === null) return null;
    if (
      typeof raw !== 'string' ||
      raw.length === 0 ||
      raw.length > 200 ||
      !/^[A-Za-z0-9][A-Za-z0-9._/-]*$/.test(raw) ||
      raw.includes('..') ||
      raw.includes('//') ||
      raw.includes('@{') ||
      raw.endsWith('/') ||
      raw.endsWith('.') ||
      raw.endsWith('.lock')
    )
      throw new Error('wave-walker: args.' + field + ' is not a safe git revision');
    return raw;
  }

  private static promptText(raw: string, field: string): string {
    const injectionDirective =
      /(?:ignore|disregard|override|forget).{0,48}(?:prior|previous|above|system|developer|instruction)|(?:system|assistant|developer)\s*:/i;
    if (raw.length > 16_384 || /[\u0000-\u001f\u007f`<>]/.test(raw) || injectionDirective.test(raw))
      throw new Error(
        'wave-walker: args.' +
          field +
          ' contains a prompt delimiter, control byte, injection directive, or exceeds 16384 characters',
      );
    return raw;
  }

  private static promptTextArray(raw: unknown, field: string): string[] {
    if (!Array.isArray(raw) || !raw.every((value) => typeof value === 'string'))
      throw new Error('wave-walker: args.' + field + ' must be an array of strings');
    return raw.map((value, index) => Configs.promptText(value, field + '[' + index + ']'));
  }
}

export const CONFIG = new Configs(args);
