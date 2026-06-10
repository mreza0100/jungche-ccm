---
name: slow-burn
description: Paces long-running multi-round work so the rolling session limit never cuts it short — splits the task into checkpointed rounds, throttles with cache-aware pauses (4-min naps keep the prompt cache warm; 25-min hibernations amortize one cache miss), scales throttle with an intensity dial 0–10 (default 4; 0 removes pacing), cuts per-round burn (sequential subagents, ranged reads, haiku downshift), and keeps a resume file so a hard cutoff loses nothing. Route here for "slow-burn", "time-bump", "pace this so we don't hit the session limit", mid-run intensity changes, and resuming a paced run.
argument-hint: [intensity] <task> | <intensity> | resume <slug>
---

# /slow-burn — session-limit pacing for long-running work

The session limit is token burn per rolling window. slow-burn finishes long tasks anyway by spreading burn over wall-clock time, burning less per round, and checkpointing so even a hard cutoff costs nothing.

## Argument dispatch (`$ARGUMENTS`)

- `<task>` or `<N> <task>` → **Setup** below, at intensity N (default 4 when N is omitted).
- bare `<N>` → **Dial**: rewrite the `intensity:` line in the active checkpoint — the most recently modified `tmp/slow-burn/*/state.md` with `status: RUNNING` — and confirm in one line: `intensity {old} → {new} for {slug}`. Applies from the next round. **`0` removes pacing**: cancel any pending pause, continue unpaced and with normal parallelism; keep writing the checkpoint (free insurance). No active run → say so and stop.
- `resume <slug>` → read `tmp/slow-burn/{slug}/state.md`, announce `resuming {slug} at round {n}/{m}, intensity {i}`, continue the round loop.

## Pause denominations (cache economics)

Prompt cache TTL is ~5 minutes. A 5–10 minute pause pays a full-context cache miss without buying real throttle — the next round re-reads the conversation uncached and burns MORE than not pausing. Two legal denominations:

- **nap** — `sleep 240` (4 min): cache stays warm, light throttle.
- **hibernation** — `sleep 1500` (25 min; `sleep 3000` at intensity 10): one cache miss, amortized.

Run every pause as background Bash (`run_in_background: true`) and end the turn — the harness re-invokes on wake. Never busy-wait or poll through a pause.

## Intensity table (0–10, default 4)

| Intensity | Pause after round — light / medium / heavy | Scheduled extra           | Diet     |
| --------- | ------------------------------------------ | ------------------------- | -------- |
| 0         | – / – / –                                  | —                         | off      |
| 1–2       | – / – / hibernate                          | —                         | normal   |
| 3–4       | – / nap / hibernate                        | hibernate every 4th round | standard |
| 5–6       | nap / nap / hibernate                      | hibernate every 3rd round | standard |
| 7–8       | nap / hibernate / hibernate                | hibernate every 2nd round | strict   |
| 9–10      | hibernate every round                      | —                         | strict   |

Round weight: **light** = ≤5 tool calls, no large reads, no subagent · **medium** = ≤10 tool calls · **heavy** = any subagent, bulk edits, large reads, or test runs.

## Protocol

### Setup

1. Slug the task; write checkpoint `tmp/slow-burn/{slug}/state.md` (template below) with the intensity.
2. Split the task into rounds — each a coherent unit finishable in ≤10 tool calls — and record the round plan.

### Each round

1. Read the checkpoint — it is the working set; prefer its Facts over re-reading files.
2. Do one round of work under the diet.
3. Update the checkpoint: tick the round, rewrite Next, append new Facts.
4. Emit one heartbeat line: `⏱ {n}/{m} · {what happened} · {pause} · next: {next step}`.
5. Start the pause the intensity table dictates in background, end the turn.

### Diet (burn less, not just slower)

- **standard:** subagents run sequentially — never parallel fan-outs; files already read once get ranged reads (offset/limit) — durable facts go in the checkpoint instead; targeted tests per round, full suites at milestones only.
- **strict:** standard, plus all mechanical work (bulk edits, scans, boilerplate) goes to haiku subagents, and no full-suite runs until the final round.

### Limit pressure

On a harness or user warning that the limit is near: raise intensity by 3 (cap 10) and switch to strict diet. On a second warning: write Next as a cold-session resume brief, tell the user how to resume, stop cleanly. The checkpoint is written before every pause, so the worst case is always "resume from the last round."

## Checkpoint template

```
# slow-burn: {task}
status: RUNNING | DONE
intensity: {0-10}
round: {n}/{m}

## Rounds
- [x] 1. {round}
- [ ] 2. {round}

## Next
{exact next action, specific enough for a cold session}

## Facts
- {discoveries, paths, decisions — the working set}
```

## Heartbeat examples

- `⏱ 4/12 · migration 008 written, targeted test green · hibernating 25m · next: wire resolver`
- `⏱ 7/12 · three doc edits (light) · nap 4m · next: update _index.md links`
- `⏱ 9/9 · final suite green · DONE, no pause · checkpoint closed`
