---
name: animate
description: Build a research-grounded educational HTML animation of any structure or flow — pipeline, architecture, protocol, lifecycle, data flow, algorithm — where every animated step is cited to a real source. Output is a single self-contained HTML file (optional GIF/video recording) under tmp/animate/. Triggered by "animate {subject}", "make an animation of", "animated walkthrough of", "visualize the flow of".
argument-hint: [subject — a flow, system, doc path, or concept]
---

# Animate — Educational Flow Animation

Turn `$ARGUMENTS` into a single-file HTML animation that teaches the subject. Two qualities are non-negotiable: **accuracy** (every label, caption, and tooltip traces to a cited fact — nothing invented for visual effect) and **teaching depth** (every beat explains what happens AND why it matters).

Engine contract, UI chrome, study mode, recording recipe, pitfalls: `docs/commands/animate/references/engine.md`. The build agent reads all of it; you read § Verification before Step 4.

## Step 0 — Scope

1. Resolve the subject and its sources: repo code/docs (start at `docs/agents/_index.md` for cross-project subjects), URLs or web research for external concepts, or the conversation itself. All source types reduce to the same fact sheet in Step 1.
2. Output dir `tmp/animate/{kebab-name}/` — on collision append `-v2`.
3. Audience default: an engineer new to the subject. Honor any audience/depth/length the user stated.

## Step 1 — Research → `facts.md`

Spawn fresh agent(s) — parallel per domain when the subject spans projects — to write `tmp/animate/{name}/facts.md`:

- Capture actors/components, ordered steps, artifacts produced and consumed, decision points, gates, loops, failure and recovery paths, cardinalities, and real timings/orderings.
- One fact per line with a stable ID (`F1`, `F2`, …) and a citation — `file:line`, doc § anchor, or URL. **A fact without a citation does not exist.**
- Names verbatim from the source — labels must grep back to the code or doc they came from.

The fact sheet is the single source of truth downstream; anything not in it stays out of the animation.

## Step 2 — Storyboard → `storyboard.md`

You write this yourself — the teaching judgment stays with the orchestrator:

- 3–5 acts; within each, beats: `{t, actor/lane, what appears, caption, tooltip, fact IDs}`.
- Two teaching layers per beat: the **caption** narrates why this moment matters; the **tooltip** states precisely what happens, ending with its fact ID(s).
- **Coverage table:** every fact ID maps to beat(s) or `CUT — {reason}`. Silent drops are how animations drift from truth.
- Pacing: ~90–120 s at 1×; insert holds at gates and decision moments; pick study-mode stop points (one per teaching moment).

## Step 3 — Build

Spawn one general-purpose agent with a complete brief: read `engine.md`, `facts.md`, `storyboard.md`; write `tmp/animate/{name}/{name}.html` — single self-contained file, no external assets. Visual design is free; the engine behaviors marked ⚓ in the contract and the storyboard's exact strings are not.

## Step 4 — Verify (both passes blocking)

1. **Behavior** — serve the output dir with `python3 -m http.server` (the Playwright MCP browser refuses `file:` URLs) and drive it with Playwright: console clean (favicon 404 exempt); act jump from a **paused** state lands mid-act and resumes playback; full run reaches the end card; speed toggle works; study mode steps through every stop; one screenshot mid-act per act.
2. **Accuracy** — spawn a fresh agent (never the builder grading its own work): extract every rendered string from the HTML — labels, captions, tooltips — and diff against `facts.md`. Any string without a backing fact is a finding; an "Expected vs Got" list comes back.

Fix findings, re-verify. Loop until both passes are clean.

## Step 5 — Deliver

`SendUserFile` the HTML plus one representative screenshot, with a one-line guide (controls + study mode). Offer recording only if the user asked for a GIF/video — recipe in `engine.md` § Recording.
