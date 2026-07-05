---
name: wave:schedule
description: Wave scheduler — sole writer of root wave.md. Collects every QUEUED /wave:refine spec from docs/dev/waves/queue/, builds the cross-spec dependency + conflict graph, gets founder rulings on conflicts and the train map, and emits ONE consolidated dependency-ordered wave train for /wave:orchestrator. Triggers: "schedule the waves", "/wave:schedule", "build the train".
---

# Schedule — Wave Train Scheduler

> Refine specifies, schedule sequences, orchestrator executes. N independently-refined specs become ONE conflict-free, dependency-ordered, aggressively-batched train.

**Trigger:** `/wave:schedule` — no args. Run S0 → S4 in order; every gate blocks. A single-spec queue runs the same protocol (S1–S2 trivial, S3 one question).

## Division of labor

- `/wave:refine` writes ZERO-GAP specs to `docs/dev/waves/queue/{YYYY-MM-DD}-{slug}.md` — each R4-founder-approved, each blind to the others.
- `/wave:schedule` sequences: ordering, batching, conflict resolution, staleness triage. It NEVER authors or rewrites spec content — task bodies copy byte-identical; its only pen is wave header blocks, sequential task renumbering (with every in-spec `#N` reference remapped), and reconciliation tables. A needed content change routes BACK to `/wave:refine` as a delta.
- `/wave:orchestrator` executes root `wave.md` unchanged — it never reads the queue.

Root `wave.md` is written by THIS command ONLY.

## S0 — Lock + intake

- Singleton lock `docs/dev/waves/queue/.schedule.lock` (own tmux/session id + epoch). Fresh lock held by another session → STOP and report; stale >2h → take over, note it in the report.
- A non-stub root `wave.md` (pre-queue spec) is adopted: move it to `docs/dev/waves/queue/` with a `**Status:** QUEUED` + `**Refined:**` header (sha = the commit it was refined against, best-effort from git log), reset root to the `# Tasks` stub, then proceed.
- Snapshot the QUEUED set; a spec arriving mid-run waits for the next schedule.

## S1 — Extract cards (fan-out)

One `Explore` reader per spec returns per-partition cards (whole spec = one partition when no `## Wave` headings): Touches, Depends, Epic, task list (# + title), file-plan paths with CREATE/EDIT/DELETE verbs, data-model symbols, contract symbols ({API_PROTOCOL} types/operations, tables, {QUEUE}/{REALTIME_PROTOCOL} events), rules blocks + the task ranges they bind. Per spec, the mechanical staleness probe: `git diff --name-only {Refined-sha}..main` ∩ its file-plan + named anchors → touched-anchor list. Readers retrieve; judgment stays yours.

## S2 — Graph (frontier judgment + walker verification)

- **Dependencies:** explicit `Depends` + inferred producer→consumer (a partition consuming a symbol/file/table another partition creates or reshapes) → edges for the topo sort.
- **Staleness — walker claims panel, never hand-judged:** each touched anchor becomes a refute-first claim ("this spec's premise at {anchor} still holds on main") for `Workflow({ scriptPath: '{REPO_ROOT}/.claude/workflows/wave-walker.js', args: { claims } })` — scriptPath, never `{name}` (stale snapshot). REFUTED = structural drift → **RE-REFINE** flag carrying the panel's evidence — a stale spec is never silently patched here; survived = cosmetic → note for the orchestrator's O2 reconcile.
- **Overlap pairs** (any shared file/symbol/table across specs), graded: `INDEPENDENT` (orthogonal symbols → ordering constraint only) · `COMPOSABLE` (stackable — sequence them; the later partition inherits the earlier's deltas via O2 reconcile, noted in its wave header) · `CONFLICTING` (contradictory intent on one feature/surface — e.g. one spec adjusts what another redesigns).
- **Draft-train MANIFEST-VERIFY:** assemble the candidate train at `tmp/wave-schedule/{train-name}-draft.md` (full S4 dialect), then run the walker panel on it — `args: { manifestPath: '{draft path}' }`. Once the specs sit in ONE manifest its consistency judge covers cross-spec conflicts natively: fold returned `conflicts` into the CONFLICTING set, refuted premises into RE-REFINE, and flag freeloader tasks for the gate. Keep `maxClaims` modest — cross-spec conflict + premise checks only; per-wave claim depth stays the orchestrator's O2 MANIFEST-VERIFY at cut time.

## S3 — Founder gate (ALWAYS — never auto-emit)

Present in ONE message: a mermaid train map (waves as nodes tagged Touches, dependency edges, which specs merge into which wave, drops/holds) + the walker's verdicts (conflicts, refuted premises, freeloaders) + rulings via `AskUserQuestion`. Every `CONFLICTING` pair gets a recommendation and these options: **SUPERSEDE** (winner ships; loser's tasks DROP, recorded) · **SEQUENCE** (both ship, earlier first — state the throwaway cost honestly) · **MERGE** (both partitions leave this train and route to ONE `/wave:refine` delta for a unified spec) · **HOLD** (spec stays QUEUED for a later train). Also rule: RE-REFINE flags, batch merges, and order ties. Loop until approved. This gate approves sequencing only — spec content was R4's gate and never re-opens here.

## S4 — Emit the train

- **Batching laws:** dependency topo-order; ties broken founder priority > unblocking fan-out > spec age. Merge same-Touches, conflict-free partitions into ONE wave section — aggressively few (every boundary costs two compactions + reconcile + full gates) yet single-impact-area each; a merged wave grown too big splits HERE, never mid-train.
- Promote the approved draft (S3 adjustments applied) to root `wave.md` in the exact refine train dialect: top preamble (`**Epic:**` — `none` when sources differ — + ALL-task rules blocks only) + `## Wave {k}: {kebab-area}` sections, each with the four header fields (+ per-wave `**Epic:**` override where source epics differ), its scoped rules blocks, its tasks. Task bodies byte-identical; renumber sequentially across the train and remap EVERY in-spec `#N` reference per the map — grep-verify zero stale numbers. Open with a **Source Reconciliation** table: queue file → original # → train # / disposition.
- Stamp each consumed spec `**Status:** SCHEDULED → {train-name} ({date})`; DROP / HOLD / MERGE→re-refine stamped likewise. Release the lock.
- Report: specs consumed/held, waves, tasks, conflicts ruled, RE-REFINE flags outstanding. Running `/wave:orchestrator {builder}` is the founder's separate decision.

## Constraints

- Files you write: root `wave.md`, queue header stamps, the lock — nothing else. No git (gitter law).
- **Legal fence inherits from refine:** you NEVER introduce a task, clause, or routing over legal/compliance documents; a founder-owned paper-trail item in a spec is carried verbatim into your report, never into the train.
- Task identity is sacred: every queued task traces to a train # / DROP (founder-ruled) / MERGE→re-refine / HOLD.
- ZERO GAP never lowers: a gap found in a spec (missing section, undecided field) is a RE-REFINE flag, never something schedule fills.
