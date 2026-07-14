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
- ORPHANED train remainders are adopted the same way: a `docs/dev/waves/{name}/manifest.md` carrying a `**Train:**` stamp but no `STATE.md`, whose stamped source is the stub or superseded, is an un-run partition of a dead train — move it byte-identical to `docs/dev/waves/queue/{YYYY-MM-DD}-{name}.md` with the `**Status:** QUEUED` + `**Refined:**` header prepended and remove the emptied wave dir; it reschedules as a spec beside new work.
- Snapshot the QUEUED set; a spec arriving mid-run waits for the next schedule.

## S1 — Extract cards (fan-out)

One `Explore` (Sonnet) reader per spec returns per-partition cards (whole spec = one partition when no `## Wave` headings): Touches, Depends, Epic, task list (# + title), file-plan paths with CREATE/EDIT/DELETE verbs, data-model symbols, contract symbols ({API_PROTOCOL} types/operations, tables, {QUEUE}/{REALTIME_PROTOCOL} events), rules blocks + the task ranges they bind. Per spec, the mechanical staleness probe: `git diff --name-only {Refined-sha}..main` ∩ its file-plan + named anchors → touched-anchor list. Readers retrieve; judgment stays yours.

## S2 — Graph (frontier judgment + walker verification)

- **Dependencies:** explicit `Depends` + inferred producer→consumer (a partition consuming a symbol/file/table another partition creates or reshapes) → edges for the topo sort.
- **Staleness — walker claims panel, never hand-judged:** each touched anchor becomes a refute-first claim ("this spec's premise at {anchor} still holds on main") for `Workflow({ scriptPath: '{REPO_ROOT}/.claude/workflows/wave-walker.js', args: { claims } })` — scriptPath, never `{name}` (stale snapshot). REFUTED = structural drift → **RE-REFINE** flag carrying the panel's evidence — a stale spec is never silently patched here; survived = cosmetic → note for the orchestrator's O2 reconcile.
- **Overlap pairs** (any shared file/symbol/table across specs), graded: `INDEPENDENT` (orthogonal symbols → ordering constraint only) · `COMPOSABLE` (stackable — sequence them; the later partition inherits the earlier's deltas via O2 reconcile, noted in its wave header) · `CONFLICTING` (contradictory intent on one feature/surface — e.g. one spec adjusts what another redesigns). Same-FUNCTION overlap (two specs editing one hot function body) is graded explicitly — COMPOSABLE at best, its SYNC conflict named in the later wave's header, never left for the second lane's SYNC to discover.
- **Draft-train MANIFEST-VERIFY:** assemble the candidate train at `tmp/wave-schedule/{train-name}-draft.md` (full S4 dialect), then run the walker panel on it — `args: { manifestPath: '{draft path}' }`. Once the specs sit in ONE manifest its consistency judge covers cross-spec conflicts natively: fold returned `conflicts` into the CONFLICTING set, refuted premises into RE-REFINE, and flag freeloader tasks for the gate. Leave `maxClaims` at its default (96) — the extractor covers every spec breadth-first, and returned `droppedClaimIds` must come back empty (a dropped spec = an unchecked premise); per-wave claim depth stays the orchestrator's O2 MANIFEST-VERIFY at cut time.

## S3 — Founder gate (ALWAYS — never auto-emit)

Present in ONE message: a mermaid train map (waves as nodes tagged Touches, dependency edges, which specs merge into which wave, drops/holds) + the walker's verdicts (conflicts, refuted premises, freeloaders) + rulings via `AskUserQuestion`. Every `CONFLICTING` pair gets a recommendation and these options: **SUPERSEDE** (winner ships; loser's tasks DROP, recorded) · **SEQUENCE** (both ship, earlier first — state the throwaway cost honestly) · **MERGE** (both partitions leave this train and route to ONE `/wave:refine` delta for a unified spec) · **HOLD** (spec stays QUEUED for a later train). Also rule: RE-REFINE flags, batch merges, and order ties. When the founder runs multiple builders (`/wave:orchestrator lanes`), the gate rules the POOL — assignment happens at the orchestrator's dispatch time, so no lane map is drawn: approve (a) the conflict graph — every wave pair graded INDEPENDENT / COMPOSABLE (producer's merge precedes the consumer's dispatch) / CONFLICTING (never build concurrently; their SEQUENCE order restated here), (b) the `## Priority` dispatch order over wave names — critical-path-first: the wave heading the longest remaining `Depends:` chain outranks (ties per S4 Batching laws) — presented WITH the critical path, expected concurrency width, and any graph-forced dry spells (idle no dispatcher can fill is a refine-shape question, surfaced here), (c) each rare `**Lane:** pinned-to:{wave}` exception with its stated reason (dispatched to whichever builder built {wave}). Loop until approved. This gate approves sequencing only — spec content was R4's gate and never re-opens here.

## S4 — Emit the train

- **Batching laws:** dependency topo-order; ties broken founder priority > unblocking fan-out > spec age. Merge same-Touches, conflict-free partitions into ONE wave section — aggressively few (every boundary costs two compactions + reconcile + full gates) yet single-impact-area each; a merged wave grown too big splits HERE, never mid-train.
- Promote the approved draft (S3 adjustments applied) to root `wave.md` in the exact refine train dialect: top preamble (`**Epic:**` — `none` when sources differ — + ALL-task rules blocks only) + `## Wave {k}: {kebab-area}` sections, each with the four header fields (+ per-wave `**Epic:**` override where source epics differ; + `**Lane:** pinned-to:{wave}` only on S3-pinned waves — multi-builder trains are pool, single-lane trains carry no lane headers), its scoped rules blocks, its tasks. Task bodies byte-identical; renumber sequentially across the train and remap EVERY in-spec `#N` reference per the map — grep-verify zero stale numbers. Open with a **Source Reconciliation** table: queue file → original # → train # / disposition; multi-builder trains follow it with `## Priority` (wave names in dispatch order) + `## Conflict graph` (wave pair → grade → overlapping write-paths).
- Stamp each consumed spec `**Status:** SCHEDULED → {train-name} ({date})`; DROP / HOLD / MERGE→re-refine stamped likewise. Release the lock.
- Report: specs consumed/held, waves, tasks, conflicts ruled, RE-REFINE flags outstanding. Running `/wave:orchestrator {builder}` is the founder's separate decision.

## Rebalance — mid-train re-entry (lane-mapped trains only)

Lane-mapped is the LEGACY dialect — read for trains predating pool, S3 draws no new lane maps; pool trains assign at dispatch and never rebalance. `/wave:schedule rebalance`: when a live lane-mapped train's queues go lopsided (one lane dry or ending while a sibling holds ≥2 unstarted waves), re-map UNSTARTED waves only — every S3 grade and merged-`Depends:` edge holds, task bodies and numbering untouched, a started wave never moves. Before moving any wave, grep the delta for cross-wave PAIRED tasks — one side removing or renaming what the other side cleans up or consumes; a split pair moves TOGETHER, or the removal side gains the minimal unbreak in-wave. The delta table (wave → old lane → new lane → why safe) is WATCHER-ruled — grades + Depends held is mechanical verification, and the ruling lands in the decision ledger for the founder, who can pull any gate to himself; on approval the ORCHESTRATOR applies the delta — re-stamps `**Lane:**` in the affected manifests (the one sanctioned post-split manifest edit) + rewrites lanes.md; schedule writes nothing mid-train.

## Constraints

- Files you write: root `wave.md`, queue header stamps, the lock — nothing else. No git (gitter law).
- **Legal fence inherits from refine:** you NEVER introduce a task, clause, or routing over legal/compliance documents; a founder-owned paper-trail item in a spec is carried verbatim into your report, never into the train.
- Task identity is sacred: every queued task traces to a train # / DROP (founder-ruled) / MERGE→re-refine / HOLD.
- ZERO GAP never lowers: a gap found in a spec (missing section, undecided field) is a RE-REFINE flag, never something schedule fills.
