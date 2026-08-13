# SPEC — port the dreamer engine from bash to TypeScript

**Owner of this spec:** Professor. **Builder:** STM_BUILDER.
**Engine root:** `/home/user/.professor/dreamer/` (moved out of `ENGINES/` — no path may reference `ENGINES/dreamer`).

## 1. What the dreamer is

A memory organ builder. It reads an agent's past transcripts, distills durable *maps* from them, has a second model refine those maps against the live repository, gates the result mechanically, and — only on a supervisor's explicit apply — writes them into that repository's `.professor/stm/` organ, where an injection hook feeds them back to the same agent type on its next spawn.

Two model seats do the thinking. Everything else is deterministic machinery, and that machinery is what you are porting.

## 2. Scope

**PORT to TypeScript:**

| File | Lines | What it is |
| --- | ---: | --- |
| `dreamer-night.sh` | 1261 | the engine — corpus enumeration, seat invocation, four gates, apply |
| `dreamer-morning.sh` | 57 | multi-repo/multi-lane sequential launcher (cron entry) |
| `dreamer-agent-inject.sh` | 113 | Claude `PreToolUse` hook — injects a lane surface into a matching spawn |
| `dreamer-codex-subagent-inject.sh` | 29 | Codex `SubagentStart` equivalent |
| `dreamer-nudge.sh` | 55 | `UserPromptSubmit` hook — the 🌙 staleness/failure line |
| `tests/test-dreamer-night.sh` | 488 | the battery — port to the same runner, see § 7 |

**DO NOT TOUCH — these are prompts, and prompts are Professor's:**
`dreamer-distill.prompt.md`, `dreamer-refiner.prompt.md`, `lanes/*.md`, `dreamer.command.md`, `agent-lane.command.md`. Read them to understand the contract; changing a word of them is out of scope. If a port forces a prompt change, STOP and report it — do not edit.

## 3. Runtime and shape

- Node ≥ 20, TypeScript strict, ESM. No `any` without a justifying comment.
- Zero runtime dependencies unless one is argued for and approved. Node built-ins only (`node:fs`, `node:child_process`, `node:crypto`, `node:path`).
- Entry points stay executable and keep their exact current **argument surface**. Their FILENAMES drop the `.sh` — a compiled TypeScript program named `.sh` is a lie: `dreamer-night`, `dreamer-morning`, `dreamer-agent-inject`, `dreamer-codex-subagent-inject`, `dreamer-nudge`. Ship them as thin executables with a shebang; the logic lives in modules.
- **The rename has four external callers, and Professor updates all four — not you, and not with a shim.** They are `dreamer.command.md` (lines 11, 15–18), `agent-lane.command.md` if it names a script, `~/.claude/settings.json` (the inject and nudge hooks), and the 07:00 crontab entry. "Prompts are Professor's" means only Professor edits them; it does not mean they are frozen. Report when your executables are proven and Professor lands all four in the same cutover, so no window exists where a caller points at a deleted file. Do not write a `.sh` compatibility shim — that is the dual implementation § 7 forbids.
- Build must produce something runnable directly from a cron line and a settings.json hook with no wrapper and no install step at call time.

Current CLI, which must be preserved exactly:

```
dreamer-night [--repo ROOT] [--agent TYPE] [--bootstrap-count N | --corpus-file FILE] [supervise]
dreamer-night [--repo ROOT] [--agent TYPE] apply STAGE
dreamer-night [--repo ROOT] [--agent TYPE] inspect-repo
dreamer-night gate-pin PATHS SHA256
dreamer-night gate-coverage PATHS COVERAGE [SHA256]
dreamer-night [--repo ROOT] gate-anchors MAPS RESULTS SURVIVORS
dreamer-night gate-verdicts SURVIVORS VERDICTS NORMALIZED
dreamer-night test-surfaces ORGAN
dreamer-night migrate-anchors ORGAN
dreamer-night [--repo ROOT] [--agent TYPE] lane-membership MAPS EXISTING OUT
```

## 4. The flow, end to end

```
PREFLIGHT   seat-law check → stage dir in the organ → dedup surface → corpus
            enumeration → PIN gate
   ↓
① DISTILL   one codex seat, gpt-5.6-luna, xhigh, sandboxed to the stage.
            Writes maps/*.md and coverage.md.
   ↓
GATES       PIN re-check · COVERAGE · ANCHORS
   ↓
② REFINER   one codex seat, gpt-5.6-luna, xhigh. Repairs or rules each map;
            may rewrite a map in place. Writes verdicts.md.
   ↓
GATES       PIN re-check · ANCHORS re-check · VERDICTS
   ↓
HOLD        READY | ZERO-SURVIVORS | ZERO-YIELD — then stop.
   ↓
APPLY       separate invocation: survivors → maps/, refuted → archive/ with the
            refutation note, lane surfaces re-rendered, lanes.tsv written,
            sweep log written. NO git.
```

Seat invocation shape (preserve exactly): `codex exec --ephemeral --skip-git-repo-check --sandbox workspace-write --cd {stage}`, with a per-seat timeout of 2700s and **one attempt per seat — no retry loop**.

## 5. Invariants — a port that breaks one of these is a failed port

These are laws, each bought with a failed night. Port them as *mechanisms*, not comments.

1. **Both seats run `gpt-5.6-luna`.** `requireSeatLaw()` runs before any other check and refuses any other distill or refiner model. The store is built by one dreamer; two models is two stores.
2. **Fail closed, everywhere.** Any gate failure aborts the night and applies nothing. A check that cannot run is a FAILURE, never a pass — an empty result is never a verdict.
3. **PIN.** `paths.txt` is SHA-256 pinned at preflight and re-verified after each seat. A corpus that changed mid-run invalidates the night.
4. **Coverage is index-keyed.** One line per supplied transcript, `{index}\tREAD|SKIP\t{reason}`, every index exactly once, never a retyped path (a seat mistyping two of thirteen absolute paths cost a whole night). The engine expands indices back to paths for the ledger.
5. **CONDUCT accounting.** Coverage also carries exactly three lines, `CONDUCT\t{technique|prior|baseline}\t{slug|NONE}\t{reason}`, before `END-OF-RUN`. All three kinds must be present or the gate fails — a kind gone silent is an unexamined class, not an empty one.
6. **Anchors.** Row grammar is exactly `` - `{path}[:lines]` — {blob|tree} `{12 lowercase hex}` ``. 2–8 rows per map. At most one terminal `:N` or `:N-N`. Never a 40-char hash, never a commit sha. Hashes verified against the pinned tree; a map that fails is rejected.
7. **Verdicts are the seat's.** `verdicts-normalized.tsv` is derived from the seat's raw `verdicts.md` at apply time and MUST NOT be editable as a back door — a supervisor who hand-edits the normalized file must find it discarded (this is deliberate; it caught the spec author overriding a refutation).
8. **Lane isolation.** One map pool, per-lane membership in `lanes.tsv`, one surface per lane at `agents/{lane}.md`, per-lane sweep windows, organ-first lane-profile resolution (`{organ}/lanes/{lane}.md` then the engine's `lanes/`). A lane with no profile cannot run. A spawn whose type has no surface receives nothing — isolation by construction, no allow-list.
9. **The engine writes NO git.** Read-only git (`show`, `grep`, `rev-parse`, `cat-file`, `status`) only. The organ is tracked by its repository now; a `git -C "$ORGAN" commit` would walk up and commit to the parent branch. Committing belongs to the repository's own flow.
10. **Everything lives in the organ.** Stage at `{organ}/dreamer/staging/{lane}-{stamp}/`, logs at `{organ}/dreamer/logs/`, corpus at `{organ}/corpus/`, sweep logs at `{organ}/dreamer/{date}.md`. Nothing is written to `/tmp` or to the repository's root `tmp/`.
11. **Two organ shapes are legal**: tracked inside its repository (proja), or its own nested ledger (projc, host-ops — not yet migrated). Both must keep working.
12. **Nothing is ever deleted.** Refuted maps go to `archive/` with their refutation note appended; name collisions gain a `{date}-` prefix.
13. **HOLD states are distinct.** `READY` (survivors exist), `ZERO-SURVIVORS` (no map was written), `ZERO-YIELD` (maps were written and all were refuted). Survivors are not yield; a night whose every survivor died must not read green.
14. **Dedup surface carries Questions.** The cached-map surface supplied to the distill seat is `{title} — {the Question that map answers}`, never bare titles — the seat may not read map bodies, and a title made it guess at containment and silently decline real lessons.

## 6. What to improve while porting

Port behaviour first, then take these — each is a real defect the bash shape invited:

- **Types for the artifacts.** `CoverageLine`, `ConductLine`, `AnchorRow`, `Verdict`, `Census`, `HoldState`, `LaneProfile`, `StageLayout` as real types with parsers that validate at the boundary and return discriminated results. No `as` casts on anything a seat produced.
- **One parser per artifact, shared by gate and consumer.** In bash, `awk` parsed the same file two different ways in two places; that drift is the bug class to eliminate.
- **Counts derived, never literal.** `sorted(actual) === sorted(declared)`, and a guard whose scan can return empty asserts non-empty first.
- **Structured run log.** One append-only JSONL per night beside the human log — phase, timestamps, gate verdicts, seat durations, exit reason — so a post-mortem does not require reading 600KB of seat output. The human-readable log line format stays as it is; the cron and the nudge parse it.
- **A process this engine spawns must not outlive it.** Seat timeouts kill the whole process group.
- **The runner lock stays** (`flock` on self today) — two nights on one organ must not interleave.

## 7. Test parity — the acceptance bar

`tests/test-dreamer-night.sh` is 488 lines and currently ALL PASS. Port every check to the TypeScript test runner (`node --test`), keeping each assertion's *intent* and its name. The battery is the specification of behaviour the prose above only summarises — where this spec and the battery disagree, **the battery wins and you report the discrepancy**.

Specifically preserved: the luna-law refusal test (build an engine copy with a non-luna model, assert it dies), lane isolation, corpus-file pinning with a ghost path failing closed, coverage index typos failing closed, the anchor grammar rejections, the hermetic second-repo hook test, byte-stable surface regeneration, the positive drift-marker test (a moved anchor renders `⚠ DRIFTED` — assert on a fixture, never on the live repo being clean).

Add: a test that a hand-edited `verdicts-normalized.tsv` does not survive apply (invariant 7), and a test that a missing CONDUCT kind fails the coverage gate (invariant 5).

**Definition of done:** the TypeScript battery passes; a real night runs end to end on the proja `qa-projb-cortex` lane against `{organ}/corpus/qa-projb-cortex-2026-08-12.txt` and reaches a HOLD state; `dreamer-morning` runs across all three repos in `repos.list`; both injection hooks return valid hook JSON; and the bash originals are deleted — no dual implementation, no compatibility shim.

**Cutover order** (the deletion is last, and it is not yours alone):

1. You build the TypeScript entry points alongside the untouched `.sh` originals and prove them — battery green, one real night to HOLD, both hooks emitting valid JSON.
2. You report: proven, ready to cut.
3. Professor lands the four caller updates (§ 3) in one pass.
4. You delete the `.sh` originals and confirm nothing still resolves to them.

Coexistence is allowed ONLY inside that window and only because nothing calls the new files yet. Do not point any caller at the TypeScript build before step 3.

## 8. Working rules

- Ask before adding a dependency, before changing any CLI surface, and before touching a `.md` file in this directory.
- Do not commit. Report what you changed and let the commit route through the repository's normal flow.
- Report progress against the file table in § 2, and name any invariant you could not port faithfully rather than porting it loosely.
