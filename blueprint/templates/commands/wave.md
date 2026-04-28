# Wave — Task Runner

> **Tier C with light Jungche voice.** Mechanics-driven; the orchestrator (Jungche) reports in character. Universal across stacks.

Run a wave of `/build` invocations from a task file: $ARGUMENTS

---

## Overview

A **wave** is a batched series of `/build` pipelines driven by a single task file. Use waves when a feature, refactor, or initiative spans multiple `/build`-able units. The wave runner reads the task file, splits tasks into pipelines, runs them in waves (parallel where independent), and produces a wave report.

**When to use a wave:**
- Cross-project initiative with 5+ tasks
- Refinement output from `/professor refine` or `/council refinement`
- Migration with sequenced phases
- Anything that's too big for one `/build` but too coordinated for many ad-hoc invocations

**When NOT to use a wave:**
- Single-feature work — use `/build` directly
- Bug fixes — use `/jc`
- Pipeline infrastructure changes — use `/ccm`
- Ad-hoc exploration — use `/professor` or `/council` for analysis first

---

## Path variables

| Variable | Value | Purpose |
|----------|-------|---------|
| `$WAVES` | `docs/dev/waves` | Wave runner artifacts |
| `$WAVE_DIR` | `$WAVES/{name}` | Per-wave directory (active) |
| `$WAVE_ARCHIVE` | `$WAVES/archive/{name}` | Per-wave directory (after completion) |

---

## Step 0 — Parse the wave task file

The wave file is at one of:
- `docs/dev/waves/council/{name}.md` — produced by `/council refinement`
- `docs/dev/waves/professor/{name}.md` — produced by `/professor refine`
- `docs/dev/waves/marketer/{name}.md` — produced by `/marketer wave`
- `docs/dev/waves/{any other source}.md`

If `$ARGUMENTS` is a path, use that. If `$ARGUMENTS` is a name, search for `{name}.md` under `$WAVES/` recursively.

**Required wave file structure:**

```markdown
# Wave: {feature title}

**Source:** {where this wave came from}

## {Category 1} ({N} tasks)

| # | Task |
|---|------|
| 1 | {task title} — {detailed description with what / why / behaviors / boundaries / compliance flags / UX specs} |
| 2 | ... |

## {Category 2} ({N} tasks)

| # | Task |
|---|------|
| 3 | ... |

## Deferred to V2

| # | Item | Reason | Champion |
|---|------|--------|----------|
| D1 | ... | ... | ... |
```

---

## Step 1 — Set up the wave directory

```bash
mkdir -p $WAVE_DIR
cp {wave-file-path} $WAVE_DIR/wave.md
```

Initialize the report:

```bash
echo "# Wave Report: {name}\n\n**Started:** $(date)\n\n## Pipelines\n" > $WAVE_DIR/report.md
```

---

## Step 2 — Plan the wave (Jungche)

Read the wave file. For each task:

1. **Group related tasks into pipelines** — tasks that share files, models, or services should be in the same pipeline. Tasks that are independent can run in parallel.
2. **Decide pipeline names** — kebab-case, unique within `docs/dev/tasks/` and `docs/dev/tasks/archive/` and `.worktrees/`. Append `-v2` etc. for collisions.
3. **Decide wave order** — pipelines that depend on each other run in serial waves. Independent pipelines run in parallel within a wave.

**Optional consultation:** if the routing is ambiguous, invoke `/professor wave-review` for a second opinion before locking the plan.

Write the plan to `$WAVE_DIR/plan.md`:

```markdown
# Wave Plan: {name}

## Wave 1 (parallel)
- Pipeline `pipeline-name-1`: tasks {1, 3, 5}
- Pipeline `pipeline-name-2`: tasks {2}

## Wave 2 (parallel, after Wave 1)
- Pipeline `pipeline-name-3`: tasks {4, 6}

## Wave 3 (serial, after Wave 2)
- Pipeline `pipeline-name-4`: tasks {7, 8}
```

---

## Step 3 — Execute waves

For each wave, in order:

For each pipeline in the wave, in parallel:

1. Invoke `/build {pipeline-name}` with the relevant task subset
2. The `/build` command runs its full pipeline (planners → architects → developers → QA → merge → post-merge QA → audit → docs → docs-commit)

**Monitor** each pipeline. If one fails:
- If it's recoverable (a fix would resolve it), pause the wave, surface the issue
- If it's unrecoverable (architectural disagreement, blocking compliance issue), abort the wave, capture state in the report

**After each wave completes**, append the pipeline outcomes to `$WAVE_DIR/report.md`.

---

## Step 4 — Wave-level reflection

After ALL waves complete, optionally invoke `/professor wave-review $WAVE_DIR/report.md` for an operational review:
- Did pipelines execute as planned?
- Where did routing decisions go right/wrong?
- What patterns recurred?
- What should the next wave do differently?

The reflection is saved to `$WAVE_DIR/reflection.md`.

---

## Step 5 — Archive

When the wave is complete:

```bash
mv $WAVE_DIR $WAVE_ARCHIVE
```

The archive preserves the full record (wave file, plan, report, reflection) for traceability.

---

## Step 6 — Report

```
🌊 Wave complete: {name}

Waves: {N}
Pipelines: {M}
Tasks delivered: {K} of {total}
Tasks deferred: {D}

Outcomes:
- {pipeline-name-1}: ✅ merged ({sha})
- {pipeline-name-2}: ✅ merged ({sha})
- ...

Issues:
- {anything that surfaced}

Archive: $WAVE_ARCHIVE
```

Light Jungche voice in the report — celebrate wins with 🎉, flag issues with 🚩, mark completions with ✅.

---

## Rules

- **Never reuse wave names** — check `$WAVES/`, `$WAVES/archive/`, and `docs/dev/tasks/{archive,}/` first
- **Wave file is read-only after Step 0** — once the wave starts, the task file is locked. Edits go through `/ccm` or a re-run.
- **Pipelines within a wave run in parallel** — they MUST be independent. If a pipeline depends on another, put it in the next wave.
- **A failing pipeline pauses the wave** — surface the failure; don't auto-skip
- **Archive on completion** — `$WAVE_DIR` → `$WAVE_ARCHIVE` keeps the active waves directory clean
- **Light Jungche voice** — wave reports are mechanics, but the messenger has personality
