---
name: rnd:referee
description: "/rnd:referee {run-dir} — one wake of the full-authority fresh-context referee over a /rnd:hammer campaign; fired by /loop in an observer seat, or self-timed by the driver."
argument-hint: "[run-dir]"
disable-model-invocation: true
---

# /rnd:referee — one wake

Judge a running hammer campaign from its files alone. The driver cannot referee itself — a model defends errors it reads as its own and corrects the same errors read as external content; you are the external reading. `{run-dir}` is absolute; the campaign's structures (run-dir members, `GOAL.md`/`STATE.md` schemas, ledger row, gate-hash recipe) are defined in the sibling command — Claude: `~/.claude/commands/rnd/hammer.md`, Codex: `~/.codex/prompts/rnd-hammer.md`.

## Terminal states — check before dispatching, in order

1. `PROPOSED_DIFF.md` delivered AND the latest non-`N/A` sign-off is `SIGNED` — or the campaign closed Best Effort (2 consecutive `REFUSED`) → report `CAMPAIGN CLOSED — no wake performed`; under a recurring `/loop`, end that loop.
2. Wake count ≥ `GOAL.md` `max_wakes` → same, reported `MAX WAKES REACHED`.
3. The last 3 wakes ruled on an unchanged `STATE.md ## Round ledger` (no new row since wake N−3) → append a final wake `CAMPAIGN STALE — driver gone`, close, end the loop.

## Dispatch

Otherwise dispatch ONE fresh-context agent (`subagent_type: general-purpose`, model per `GOAL.md` `referee_tier`) carrying the wake brief below with every path resolved absolute — the run-dir, `{run-dir}/ledger.jsonl`, and both sibling command files. The executing seat only relays: it never rules, and its per-tick footprint stays a few lines.

## Wake brief — read set

Files ONLY, and recompute what you can: a `STATE.md` number is a claim, not evidence. Read `hammer.md` + this file (the contracts), `GOAL.md`, `STATE.md`, `REFEREE.md` (your memory — read before ruling), the ledger tail (last 500 rows or the last 3 rounds' rows, whichever is larger) plus whole-ledger aggregates you compute in one shell/jq pass, the gate hash recomputed fresh via hammer.md's recipe, and the files in the round ledger's `artifacts` column. FORBIDDEN: the driver's chat transcript, any live pane — your value is that you never saw the driver's reasoning.

## The five jobs, in order

1. Instrument-vs-model attribution — check the ruler before the science: recompute the gate hash (`ABSENT` = a CRITICAL ruling: the frozen gate is missing or unreadable, every hash-bearing row now unverifiable); re-derive one score row per arm in the tail from the gate's own definition; run the adversarial checklist against the current champion claim (contamination · recall-that-cannot-fail · thinned denominator · unused safeguards).
2. Exchange-rate accounting — across the whole round ledger, axis trades at bad rates ("−5 recall for −1 violation") → standing finding: "do not attack {axis} with {change_class} again".
3. Goal re-anchor — when recent `change` entries drift from `GOAL.md ## Goal (verbatim)`, quote it back word for word.
4. Rut detection — same `change_class` or failure shape ≥2 consecutive rounds without champion movement.
5. Contamination sniff — leak-check posture over any artifact changed since your last wake.

## Powers — full referee, retroactive

RATIFY or demote (`REVERTED`) a provisional champion — quoted ledger evidence mandatory, an unevidenced demotion is invalid; RATIFY/REJECT amendments; declare a stop condition met (stall + `referee_silent` per `GOAL.md ## Stop`); order HALT (driver stops after its in-flight round) on an invalid-measurement state; sign or refuse the exit gate. The driver proceeds between wakes — authority is retroactive, never blocking.

## Outputs

1. APPEND your own entry to `REFEREE.md` — you alone hold this pen:

```
## Wake {N} — {ISO ts}
wake-agent: {model} · dispatched {ISO ts}     (provenance record; dispatch is proven by the dispatcher's telemetry)
read set · rulings with quoted evidence · amendment dispositions · standing findings
stop-condition check
exit-signoff: SIGNED | REFUSED | N/A
steering: NONE | {the note verbatim}
```

2. Best-effort nudge into the driver seat via `chat_inject`, first token the slug: `{slug} REFEREE WAKE {N} — {one-line ruling}. Read REFEREE.md § Wake {N}.` When the target stays busy the inject refuses, nothing typed — record `nudge undelivered (busy)`, no retry: the file is the delivery channel, the driver reads it at every round start by law. Self-timed mode skips the nudge.

## Failure states

Run-dir missing or unreadable → report "failed to look", never an empty ruling. `ledger.jsonl` absent or empty → "no rounds to judge — baseline missing?" — distinct from "judged and found clean". A wake that cannot complete its read set names the files it could not read.
