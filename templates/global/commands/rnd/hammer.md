---
name: rnd:hammer
description: "Two-layer RND loop — /rnd:hammer {goal}: scaffold a deterministic run-dir, freeze the measurement gate, then hammer the target round by round (one change, ≥3 reps, scored through the frozen gate) under a timed fresh-context referee (/rnd:referee). Triggers: 'rnd hammer <goal>', 'hammer this prompt/call/config', 'two-layer RND'."
argument-hint: "[goal]"
---

# /rnd:hammer — the driver

Take a measurable goal against one production target — an LLM call, a prompt, a query, any code — and improve it by structured rounds inside a sandboxed run-dir, scored by a frozen gate, supervised by `/rnd:referee`. You are the **driver**; the **referee** is a separate fresh-context agent with authority over your results. Vocabulary used throughout: **arm** (one candidate variant) · **round** (one change + its reps + its scoring) · **champion** (best arm so far) · **gate** (the frozen measurement apparatus) · **judge** (a gate-level LLM measurement agent) · **wake** (one referee activation) · **amendment** (a ratified gate change).

## Sacred boundary

Work ONLY inside the run-dir. NEVER edit a real project file — the deliverable is `PROPOSED_DIFF.md`, applied by others after user ratification. Production enters the harness ONLY as a black box: invoked whole, or monkey-patched at the single substitution point `GOAL.md ## Harness` names — never as copied or replicated fragments. Every other transformation an input undergoes is written in the sandbox, visible there; an inherited production mechanic is an invisible confound.

## Run-dir — the deterministic structure

`.professor/RND/{family}/{N}-{goal}/` — `{family}` = short name of the system under study, `{N}` = next integer in that family dir. Scaffold ALL members before round 0; every path handed to a seat or agent is ABSOLUTE.

```
GOAL.md               goal verbatim + knobs (schema below)
STATE.md              resume file: round ledger · champion · findings · amendments
REFEREE.md            referee's judgment log — you create the 2-line header, never write it again
corpus/manifest.json  the selectable input list
gate/                 scorer + goldens + judge spec — FROZEN at baseline
defects.md            known-defect registry D1…Dn, diffed every round
runs/{round}-{arm}/{case_id}.json   per-case parsed output (raw dumps beside as {case_id}.raw)
results/{arm}.json    per-arm scored summaries
ledger.jsonl          append-only audit trail, one JSON row per call/score
PROPOSED_DIFF.md      the deliverable, written at exit
```

Seat-mode scaffold refusal: grep `.professor/RND/*/*/GOAL.md` for a `## Seats` block naming your driver address with no delivered `PROPOSED_DIFF.md` — a hit refuses the scaffold and names the live campaign (one whose `STATE.md ## Next` is marked `abandoned` or `disarmed` does not block; self-timed campaigns are exempt).

## GOAL.md schema

```
# GOAL — {slug}
## Goal (verbatim)        user's words, immutable; edited only on user instruction, quoted
## Target under change    file/symbol anchors of the production thing under study
## Bar                    metric + threshold + denominator (what counts, what's excluded, who ruled)
## Stop
stall_rounds: 10          consecutive rounds without champion improvement
referee_silent: false     true: the stall exit also requires the latest wake's steering: NONE
budget_usd:               optional
max_rounds:               optional — target-met always exits
## Cadence
referee_interval_minutes: 30
referee_tier: opus        sonnet on the user's say
max_wakes: 48
## Seats
driver: {chat_whoami address | self-timed}
referee_seat: {address | self-timed}
## Harness
language · adapter: black-box | monkey-patch @ {named point} | own-client · run command
## Corpus
manifest: corpus/manifest.json · selection: all | {id list} | tag:{t} | sample:{n}:{seed}
## Round rules
changes_per_round: 1 · reps: 3 (minimum) · scoring unit (e.g. equivalence class, never raw instance)
```

## STATE.md skeleton

```
# STATE — {slug} (resume file)
**Owner:** {driver address}. **Mode:** hammer. **Started:** {ISO ts}. **Skill:** /rnd:hammer.
{goal quoted verbatim} · {target anchors}
## Round ledger      | round | arm | change | change_class | artifacts | result | champion? |
## Champion          | arm | score | status: PROVISIONAL / RATIFIED / REVERTED |
## Standing findings
## Instrument defects  | # | instrument | what it faked | found by |
## Amendments        | # | instrument | change | status: PROPOSED / RATIFIED / REJECTED | wake |
## Next
```

`change_class` = a short stable tag reused for same-mechanism changes (the referee's rut-detection key); `artifacts` = the arm/prompt files the round touched. `REVERTED` is a referee-only demotion: the pointer returns to the last RATIFIED arm (`a0-baseline` when none), the row stays as history. The instrument-defects table is never pruned — the pattern is the finding.

## ledger.jsonl row

```json
{"ts": "ISO8601", "phase": "baseline|round|score|audit", "round": 0, "arm": "a0-baseline", "prompt_version": null, "case_id": "…", "model": "…|null", "config": {}, "latency_ms": null, "tokens_in": null, "tokens_out": null, "cost_usd": null, "metrics": {}, "gate_hash": "…", "note": null}
```

Baseline rows carry `phase: "baseline", arm: "a0-baseline"`. Nullable fields are null when unknowable, never omitted; `model`/`latency_ms` are null and `config` is `{}` for a non-model event (a deterministic scoring row). `gate_hash` is the recipe value below or the literal `"ABSENT"` — a row never carries `"ABSENT"` together with score metrics: scoring refuses to run without a computable hash.

## corpus/manifest.json

```json
{"cases": [{"id": "…", "path": "…", "tags": ["…"], "notes": null}], "authoritative_fields": {}}
```

`authoritative_fields` = corpus-level ground-truth facts you must not re-derive (`{}` when none). Sample selection always carries its seed (`sample:{n}:{seed}`) — an unseeded sample destroys round comparability. Record the selection in force in `GOAL.md ## Corpus`.

## The gate — frozen at baseline

`gate/` holds the scorer (any language), the judge spec, and the goldens. It freezes the moment the first `a0-baseline` row is written — complete it before round 0 — and after that instant you NEVER edit it. Every score row carries the gate hash:

```
cd {run-dir absolute} || exit 1
[ -d gate ] || { echo "gate_hash ABSENT: gate/ missing — hash NOT computed" >&2; exit 1; }
[ -n "$(find gate -type f -print -quit)" ] || { echo "gate_hash ABSENT: gate/ empty — hash NOT computed" >&2; exit 1; }
[ -z "$(find gate -type l -print -quit)" ] || { echo "gate_hash ABSENT: gate/ contains symlinks — regular files only" >&2; exit 1; }
[ -z "$(find gate -type f ! -readable -print -quit)" ] || { echo "gate_hash ABSENT: gate/ has unreadable file(s) — hash NOT computed" >&2; exit 1; }
set -o pipefail
find gate -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum | cut -d' ' -f1
```

The hash is this recipe's stdout ONLY when its exit status is 0 — any nonzero exit is `ABSENT` in the ledger, and the stdout of a failed run is discarded. Run it from the run-dir root (relative paths are part of the digest); reimplementations reproduce `sha256sum`'s byte format: `{64 lowercase hex}` + two spaces + `{relative path}` + `\n` per file, final hash = SHA-256 of that concatenation. The hash pins the judge SPEC, not the stochastic measurement.

A hash change without a RATIFIED amendment makes every later score INVALID: stop scoring, file a PROPOSED amendment row, trigger an out-of-cadence wake; only the referee ratifies. A RATIFIED amendment re-runs `a0-baseline` under the new gate; old rows keep the old hash, and comparisons hold only within one hash era.

Golden truth is extracted ONCE into machine-readable form and validated before first scoring: every referenced input unit exists, no two goldens claim the same unit, every golden row carries a verbatim quote of its source.

## Harness

Copy an existing harness or build your own — any language the target speaks. Three adapters (declare one in `GOAL.md ## Harness`): `black-box` (production invoked whole), `monkey-patch` (production factory patched at the one named point), `own-client` (pinned standalone client). Round 0 is the baseline: production invoked whole; nothing scores before the `a0-baseline` rows exist — a reconstructed baseline measures harness drift, not gain.

## The round

1. Read `REFEREE.md` for new rulings — it is the referee's delivery channel; the file missing = a scaffold defect: stop and report, never "no rulings".
2. Evaluate `GOAL.md ## Stop`. `budget_usd` sums non-null `cost_usd`; when >20% of `baseline|round|audit` rows carry null cost, mark `UNENFORCEABLE — {pct}% rows costless` in `STATE.md` (an error, never silence).
3. ONE change, per `## Round rules`.
4. Run ≥ reps over the selected corpus through the harness.
5. Score through the frozen gate — deterministic scorer, and/or blind judges (§ Integrity instruments) — hash into every row.
6. One full-column round-ledger line in `STATE.md`.
7. Champion: a better Bar score = PROVISIONAL promotion, ratified retroactively by the referee. **No-promotion** on a losing arm: its files and ledger row stay (`champion?` = no), the pointer does not move, no artifact is deleted or edited. You never demote — `REVERTED` is the referee's.

Checkpoint `STATE.md` after every referee-RATIFIED milestone; where the host offers a self-compact, attempt it once and work on from the checkpoint — `STATE.md` is the resume file, and the campaign never lives in your rolling context.

## Referee arming

**Seat mode** (chat MCP present): SPAWN a dedicated observer seat — `chat_new {engine: "claude", name: "referee-{slug}", cwd: {absolute project root}}` — never reuse an existing seat; inject `/loop {referee_interval_minutes}m /rnd:referee {run-dir absolute}`. Arming is complete only when verified: on a busy refusal retry the inject once after 60s, then confirm a `## Wake 1` heading appears in `REFEREE.md` within `referee_interval_minutes + 5m`; failing that, `chat_kill` the seat (a live seat plus a self-timed driver would both append wakes), record `seat killed — arming unverified` in `STATE.md ## Next`, and continue self-timed. Out-of-cadence triggers — a pending PROPOSED amendment, an exit sign-off request — fire by injecting `/rnd:referee {run-dir absolute}` into the seat; a twice-refused trigger inject → dispatch the wake yourself as a fresh-context agent and record `out-of-cadence inject undelivered — self-dispatched` (a frozen scoring halt never waits on a busy seat).

**Self-timed** (no fleet, or a Codex driver — which reads these commands at `~/.codex/prompts/rnd-hammer.md` + `rnd-referee.md`): before each round compare now − the last `## Wake` ts (or `STATE.md` `**Started:**` when none) against the interval; when elapsed, or on either out-of-cadence trigger, dispatch the wake yourself as ONE fresh-context agent executing `/rnd:referee` per its own file. The weaker mode — a stalled driver never gets nudged — named in `GOAL.md ## Seats`, never a silent substitute.

## Integrity instruments

Build these beside the harness (campaign's own language); each refuses an empty comparison set rather than report a meaningless zero, and reports its counts beside its verdict:

- Leak check: before any judge reads a prompt, no scored input material appears verbatim in its instruction block (≥8-real-word match window). Contrastive examples are drawn from failures and REWRITTEN onto invented material teaching the identical rule.
- `defects.md`: stable D-ids diffed every round — do the KNOWN defects still exist? Never an accuracy measure; ids never renumber.
- Blind judge packets: input + prompt(s) + output under randomized keys (`AB_KEY.json` at run-dir root: `{"{key}": {"arm","case_id"}}` — you build packets and hold the key; judges never read it; unblind after all verdicts land). Judges derive their own expected result BEFORE reading the system output; forbidden reads: any scorer, spec, `STATE.md`, golden card (until after derivation), a rival arm's output. One judge per input-cluster, each ending with a cross-cluster pattern section; judge floor = a mid-tier reasoning model or better.
- Adversarial audit on any headline claim (a 100%, a bar cleared): independent judges briefed to BREAK it — checking benchmark contamination, recall-that-cannot-fail, a thinned denominator, and safeguards defined but exercised zero times.

## Exit

Any stop condition → final report (goal · champion with the Bar numbers per hash era · every arm tried and what it taught · `PROPOSED_DIFF.md` written) — plus the referee's `exit-signoff: SIGNED`. Two consecutive `REFUSED` sign-offs (counted over non-`N/A` wakes) → close as Best Effort with the refusal findings attached. Then disarm any spawned seat (`chat_kill referee-{slug}`, including after a fallback) and record `disarmed` in `STATE.md ## Next`. A dead campaign is cleared, on user instruction only, by marking `STATE.md ## Next` `abandoned — {reason}`.
