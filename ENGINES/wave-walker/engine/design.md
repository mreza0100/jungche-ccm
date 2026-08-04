# Wave Walker Engine — Design

How the engine works and why it is shaped this way. The source of truth is `src/` plus its tests; `dist/workflow.js` is a generated bundle (its first comment says so) and is never edited by hand. Numbers quoted here explain design decisions — `src/config.ts` is the authoritative table for every default.

## Table of contents

- [What this is](#what-this-is)
- [Design principles](#design-principles)
- [Modes and dispatch](#modes-and-dispatch)
- [Walk-mode flow](#walk-mode-flow)
- [The seat roster](#the-seat-roster)
- [The rule engine](#the-rule-engine)
- [The project profile](#the-project-profile)
- [Honesty mechanisms](#honesty-mechanisms)
- [Build pipeline](#build-pipeline)
- [Test suite](#test-suite)
- [How to change the engine](#how-to-change-the-engine)

## What this is

A multi-agent verification engine that runs as a Claude Code **Workflow script**: one flat JS file, executed in a sandbox whose only APIs are `agent()/parallel()/pipeline()/log()/phase()` plus plain JavaScript — no filesystem, no Node builtins, no module loader. The engine walks a wave's diff after merge (or a branch before merge), fact-checks claim sets, and runs goal-driven code investigations. Its callers are the `wave/walker.md` command and the wave orchestration family; the caller passes everything project-specific at runtime (see [The project profile](#the-project-profile)), which is why one built bundle serves every project.

## Design principles

1. **The judge is never the thing being judged.** Every verdict is produced by a seat that did not produce the evidence: sensors extract cards, a zero-token rule engine diffs them, judges adjudicate only flagged anomalies, an independent final judge rules the whole walk and may overturn any of them. Where two seats disagree about the same file, the disagreement is escalated by name — never averaged (`computeVerdictContradictions`).
2. **Coverage is stated, never implied.** Every lens reports its denominator: the security merge names every unswept file, the sensor cap names every dropped field (`unsensedFields`), gate machinery that did not run says exactly why (three distinct `SKIPPED` markers). An empty result is a claim about the world; a failure to look is a claim about the engine — the two never render the same.
3. **A walk over an untrusted denominator never renders a verdict.** The scout's changed-file list is reconciled against an independently-executed count; an empty diff or a zero-thread enumeration over a non-empty diff FAILS the walk rather than passing it.
4. **Zero-token where JS suffices.** Everything mechanical — card zipping, rule evaluation, chunking, contradiction detection, arming, telemetry assembly — is plain code between agent barriers. Tokens are spent only on judgment.
5. **Degrade loudly, partially, and in the safe direction.** A dead seat gets exactly one respawn (`retryAgent`); a dead slice keeps its survivors and names its hole; a failed telemetry section marks `degraded: true` and names itself while every other section still assembles.

## Modes and dispatch

Mode is computed in the `Configs` constructor, precedence top to bottom; construction throws if no selector is present:

| Selector arg | Mode | What runs |
|---|---|---|
| `manifestPath` | manifest-verify | claimExtractor mines the manifest → claim panel → consistencyJudge |
| `claims[]` | verify | claim panel only (one verifier per claim × `votes`, or file-clustered batches past `soloThreshold`) |
| `goal` | investigate | lens probes → brainer-steered pursue/attack waves over a quote-pinned claim ledger → synthesiser |
| `reportPath` | walk (default) | the full thread walk + ledger spine + security fan-out described below |

## Walk-mode flow

`runWalk` in `src/engine.ts`. Phases separated by **barriers** (parallel agent batches); everything between barriers is zero-token JS.

```
computeGateArming(profile)          zero-token: arms/disarms gate machinery
        |
SCOUT (1, corrective retry ×1)      threads + sensor jobs + gate files + auth rule
        |                           misreconciled() → retry → FAIL; empty diff → FAIL
computeArmedInvariants()            scout's semantic arming ∪ territory-glob fail-safe
        |
BARRIER 1 — Walk+Sense+Hunt        threadWalker × threads
                                    sliceSensor × jobs (≤18 fields each, ≤60 jobs)
                                    gateSweep × gate files (only when armed)
                                    securityAuditor × file clusters (≤12 files each)
                                    invariantHunter × armed invariants
        |
mergeSecurityResults()              intersection of categories; unswept files NAMED
zipCards() → computeAnomalies()     the ledger diff: R1–R8, plus R9-INV from hunters
computeVerdictContradictions()      same-file disagreements escalated by name
        |
BARRIER 2 — Judge                   anomalyJudge × chunks-of-6 (same-rule)
                                    territoryDigest × non-empty territories
                                    coverageCritic (when invariants supplied)
        |
escalation filter                   killed R6/R7 high+critical, killed R3/R4,
        |                           AND confirmed R9-INV high+critical (both directions)
SECOND OPINION (opus, chunks of 4)  overrides marked [OVERRIDE …] / [RE-EXAMINED …, KILLED]
        |
FINAL JUDGE (1, opus)               authoritative verdict; reinstatements marked
        |
assembleDebugRecord()               per-section try/catch telemetry
FOLD (1)                            writes ## Professor's Wave Review; returns ledger
```

## The seat roster

Nineteen seats, closed by the `Seat` union in `src/types/agents.ts` and mirrored one-to-one by `Configs.TIER`/`Configs.EFFORT`. Per-seat overrides ride `args.agents.<seat>.{model,effort}`; unknown seat or tier throws.

| Seat | Tier/effort | Cardinality | Duty |
|---|---|---|---|
| scout | sonnet/high | 1 (+1 retry) | enumerate threads, sensor jobs, gate files; extract the live auth rule |
| threadWalker | sonnet/high | per thread | confirm one flow reaches its terminal state |
| sliceSensor | haiku/medium | per job | extract producer/consumer/dbColumn/SDL cards |
| gateSweep | haiku/medium | per gate file | extract resolver guard-chain cards |
| securityAuditor | sonnet/xhigh | per file cluster | 8A–8K sweep of its slice, full set as context |
| invariantHunter | sonnet/high | per armed invariant | refute-first hunt of one registered invariant |
| coverageCritic | sonnet/high | 0–1 | name what the invariant hunt did NOT cover |
| anomalyJudge | sonnet/high | per 6-chunk | CONFIRMED / FALSE / UNPROVEN on flagged anomalies |
| territoryDigest | sonnet/high | per territory | the un-mechanizable smells rules cannot see |
| secondOpinion | opus/high | per 4-chunk | re-examine escalated verdicts, both directions |
| finalJudge | opus/high | 1 | rule the walk; reinstate wrong kills; name missed risks |
| fold | sonnet/high | 1 | merge everything into the written review |
| claimExtractor | sonnet/xhigh | 0–1 | mine a manifest's load-bearing claims |
| claimVerifier | sonnet/xhigh | per claim×vote | fact-check one claim (or a ≤4-claim batch) |
| consistencyJudge | sonnet/xhigh | 1 | cross-task conflicts, refuted premises, freeloaders |
| probe | sonnet/xhigh | per lane | pursue/attack one investigate lane, quote-pinned |
| brainer | opus/xhigh | per wave | steer the investigate ledger |
| claimAuditor | haiku/medium | per wave | grep every quote-pin; pass/fail mechanically |
| synthesiser | sonnet/xhigh | 1 | cited closing report, confidence floored by computed value |

## The rule engine

`computeAnomalies` (R1–R8, one sequential pass) plus `computeInvariantAnomalies` (R9-INV) in `src/rules.ts`; meanings in `ruleMeaning` (`src/constants.ts`). All zero-token — judges see only what these flag.

| Rule | Detects |
|---|---|
| R1 orphan producer | produced field with zero production consumers (deadness-bar gated) |
| R2 phantom consumer | consumed field nothing produces, or an undeclared field read |
| R3 encoding mismatch | producer encoding vs consumer decode incompatible, incl. double-encode |
| R4 value-set mismatch | compared literals no producer emits (casing-only = critical) |
| R5 type drift | hand-typed base type ≠ generated/SDL base type |
| R6 gate outlier | fenced+unfenced gates on one resource class; owner role with client id and no ownership fence |
| R7 unfenced ID flow | client-supplied id reaches data access with neither org nor ownership fence |
| R8 dangling reference | a reference that resolves to nothing |
| R9-INV invariant violation | hunter-confirmed breach of a registered cross-cutting invariant |

## The project profile

Nothing project-specific is baked into this engine. `args.project` (validated in `Configs.parseProject`, shape `ProjectProfile` in `src/config.ts`) carries: `repoRoot`, `authDoc`, `authRuleFallback`, `authRuleMustContain`, `roles{owner,elevated}`, `resourceClasses`, `fencedResourceClasses`, `fenceLabels{org,ownership}`, `gateResolverPattern`, `gateSurfacePattern`, `deadnessSurfaces`, `stakesLine`, `securityStakesLine`. Each consuming project writes its profile once in its `wave/walker.md` § Engine profile and passes it on every invocation.

`computeGateArming` arms the gate machinery only when the profile supplies both roles, a non-empty `fencedResourceClasses`, and both gate patterns compiling as regexes. Any miss disarms gate sweeps and R6/R7 for the whole walk — loudly:

- `SKIPPED — no project profile supplied` — profile absent or gate fields missing
- `SKIPPED — invalid gate pattern: <err>` — a pattern failed to compile
- `SKIPPED (diff-scoped)` — armed, but the diff touches no gate-relevant surface and `fullGateSweep` was not forced

All three render in the coverage summary, the ledger, and telemetry. The thread walk, security fan-out, and panel modes run fully regardless — the floor never regresses.

## Honesty mechanisms

The greppable inventory; each name is the function to read:

- `misreconciled` — scout's enumerated list vs its independently-executed count; one corrective retry, then FAIL.
- `computeVerdictContradictions` — same-file opposite verdicts escalated by name; no-file threads reported uncomparable, never silently agreeing.
- `computeArmedInvariants` — semantic arming ∪ deterministic territory-glob; a forgetful scout cannot silently disarm a hunt.
- `mergeSecurityResults` — categories intersected, unswept files named, `null` only when every slice died.
- `assembleDebugRecord` — five sections, each in its own try/catch; one failure degrades itself by name, never the record.
- `retryAgent` — exactly one respawn per dead seat, idempotency-asserting RESUME prefix, optional tier escalation.
- Escalation is symmetric — killed security/near-certain verdicts AND confirmed R9-INV verdicts both get an opus second opinion; overrides and reinstatements carry visible markers in the output.
- Verify-mode `claim.opus` — a per-claim escalation that cannot survive batching is logged when dropped, never silently lost.

## Build pipeline

The Workflow sandbox demands a single flat file beginning with `export const meta = {…pure literal…}` and ending in a top-level return — a shape esbuild cannot emit. So `build.js` is a **dependency-derived concatenating bundler**:

1. TS types are blanked in place with Node's `stripTypeScriptTypes` (never reflows code — `meta` stays byte-identical).
2. Module order is a stable topological DFS of the real import graph from `src/engine.ts`, edges sorted before recursion for determinism; `src/meta.ts` is pinned first.
3. Import/export statements are cut so all modules share one flat scope; only `meta` keeps its `export`.
4. Three loud guards: **reachability** (an orphan module fails the build by name), **cycles** (named), **post-sort order** (every dependency must precede its dependent — the actual invariant flat concatenation relies on, verified mechanically).
5. A deterministic `GENERATED FILE — DO NOT EDIT` banner lands immediately after `meta`'s closing brace. No timestamps, no hashes — builds are byte-reproducible.

`verify.js` rebuilds to a temp path and byte-diffs against the committed `dist/workflow.js`; it runs as `pretest`, so a stale bundle can never pass the suite. `validate-bundle.js` then checks: parses as JS *and* as the body of an `AsyncFunction` with exactly the harness globals; no surviving module system; `meta` is the literal first statement and a pure data literal (AST-walked); no nondeterminism (`Date.now`, `Math.random`, `process`, `fetch`, …); and a permanent **project-leak denylist** (terms built by string construction so the guard never trips itself) — a bundle naming the source project fails the build by the exact hit.

## Test suite

337 tests, 10 files. The load-bearing ones:

- `config.test.ts` — mode precedence, validation throws, all 19 seat tier/effort defaults, override plumbing.
- `engine.test.ts` — full walk pipeline, reconciliation, security fan-out, gate-conditional dispatch, **profile arming**, investigate loop, the invariant-registry feature end to end.
- `rules.test.ts` — R1–R8 cases, id numbering, sparse-input fallbacks, R9-INV synthesis.
- `prompts.test.ts` / `schemas.test.ts` — every seat's prompt and StructuredOutput schema pinned against transcribed source; conditional blocks are zero-byte when absent.
- `build.test.ts` — banner determinism and the leak guard observed FAILING on a planted term (the guard's broken state is itself tested).

## How to change the engine

1. Edit `src/` and its tests — never `dist/workflow.js` (generated, banner says so) and never a consuming project's copy (a symlink to `dist/` anyway).
2. `npm run typecheck && npm test` — `pretest` runs `verify.js`, so the committed bundle must match a fresh build.
3. `npm run build` — regenerates `dist/workflow.js` through the guards above.
4. Commit src + dist together in the blueprint repo. Consumers receive the change on their next `git pull` of the clone — that is the entire distribution story.
5. Anything project-specific belongs in `args.project`, never in `src/`. The bundle leak guard enforces this mechanically at every build.
