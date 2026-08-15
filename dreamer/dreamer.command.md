---
name: dreamer
description: Run the Dreamer night through cc-fleet. Modes — /dreamer (one night — seats, gates, HOLD before apply), /dreamer apply {stage} (apply a signed night), /dreamer lane {agent-type} (harvest one agent type into its lane), /dreamer morning (every configured repository). Triggers — "dreamer", "dream", "consolidate memory", the 🌙 nudge.
argument-hint: '[apply {stage}|lane {agent-type}|morning]'
---

# Dreamer CLI gateway

$ARGUMENTS

The engine is `cc-fleet dream`. Run it; never reconstruct its flow in chat and never route it through an agent, workflow, or SDK.

Dispatch:

1. No mode → `cc-fleet dream night`. It runs the seats, applies the four gates, and stops at HOLD, printing a signed apply command when maps survive. A night never applies itself, so there is no separate supervise mode.
2. `apply {stage}` → run the signed command the night printed, verbatim.
3. `lane {agent-type}` → add `--agent {agent-type}`; the lane needs a profile at `lanes/{lane}.md`, organ-local first and then the engine's, or the runner refuses.
4. `morning` → `cc-fleet dream morning` walks every repository in `repos.list` and exits non-zero when a listed repository fails to produce a night.

Another repository takes `--repo {root}`.

The corpus defaults to transcripts newer than the last applied sweep. `--bootstrap-count N` takes the N newest regardless of that cutoff; `--corpus-file {path}` takes an explicit list of absolute transcript paths, whose leading `#` comments reach the seat as provenance. Reach for the corpus file whenever recency selects the wrong work: a day spent in another repository fills the newest slots with transcripts this repo cannot anchor, the seat correctly refuses every one of them, and the lane reads as empty when it is only badly sampled. Select by substance — the commands that agent runs, the files it edits — when its typed transcripts are thin.

Report the outcome and the stage path. One seat attempt means one attempt: surface failures with their preserved artifacts and never retry.
