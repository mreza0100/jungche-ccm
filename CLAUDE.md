# Professor — the discipline layer for Claude Code

**What this repo is:** the framework itself, not an app that uses it. Everything under `blueprint/templates/` is **shipped source** — an adopter's live agent prompts, one clone away. A sloppy sentence here becomes a misbehaving agent in someone else's repo tomorrow. Treat prompt text as production code, because it is.

**Architecture:** a roster of four projects in one repo — one publication surface, three engines.

- `blueprint/` — the shipped framework: templates (agents, commands, scripts, codex, host-swap), plus `BLUEPRINT.md` (philosophy), `SETUP.md` (generation spec), `PLACEHOLDERS.md` (substitution law). Markdown + shell. No build; the gates are `scripts/leak-check.sh` and `scripts/refresh-scope.sh`.
- `cc-fleet/` — the fleet engine: Go 1.24, `cmd/cc-fleet` + `internal/*`, tested with `go test ./...`, shipped alongside a zsh shim.
- `dreamer/` — the memory-organ engine: TypeScript (ESM, Node ≥20), `npm run typecheck` / `build` / `test`.
- `ENGINES/wave-walker/engine/` — the wave-walker engine: JS/TS compiled by `cross-workflow` for both the Claude Workflow runtime and the Codex SDK; `npm run verify` then `npm test` (vitest).

Root `README.md`, `INSTALL.md`, `CHANGELOG.md`, `VERSION`, and `releases/` are the **public face**. They are edited with the same care as templates.

---

## Two-runtime team — Claude + Codex

This repo runs two AI runtimes as a team. `CLAUDE.md` and `AGENTS.md` are the same shared contract: both carry the persona and the rules; runtime-specific wrappers translate mechanics (slash commands, agent spawning, git execution) — never identity or protocol.

- `.claude/` is the source of truth. `.codex/` is a **pointer layer** over it, never a restatement.
- `AGENTS.md` is **compiled** from this file by `.claude/scripts/build-codex.mjs` — never hand-edited, never a symlink. Edit `CLAUDE.md`; the `Stop` hook recompiles.
- Git stays gitter's alone. A Codex role gets read-only git (`status`/`log`/`diff`/`show`) and nothing more. There is no `gitter.toml`, and there must not be one.

Compile and audit by hand when a Bash-driven write bypassed the hook:

```bash
node .claude/scripts/build-codex.mjs generate && node .claude/scripts/build-codex.mjs check
```

---

## Path vars

- `$CDOCS`: `docs/commands` · `$REFS`: `references` · `$RESEARCH`: `research` · `$RESOURCE`: `resource`
- Generated artifacts → `tmp/` (gitignored). Never leave scratch output in a tracked directory.

## MANDATORY Rules

### Publication (this repo's sacred ground)

- **No push, tag, or release without an explicit request in the current turn.** A finished task, a green build, a "finish it", or a completed release *document* is not permission to publish. Only gitter pushes, and only when the founder plainly asked in that turn.
- **Nothing identifying ships.** No brand of the private source project, no founder PII, no client domain content, no machine-absolute path (`/home/…`, `/Users/…`) in any tracked file. `scripts/leak-check.sh` runs as `pre-push` via `.githooks/` — it is the backstop, not the plan. Write it clean the first time.
- A template's example values are **illustrative placeholders**, never mined from a live private repo. When you need a concrete example, invent one.
- **Version discipline:** `VERSION`, `CHANGELOG.md`, `releases/vX.Y.Z.md`, and the tag agree or the release is wrong. `/pcm:release` owns that sequence.

### Prompt & template code

- **A template IS the live source file, verbatim** — same structure, mechanics, character, working logic — with only project-specific *values* swapped for `blueprint/PLACEHOLDERS.md` tokens. Never abstract, skeletonize, summarize, or "genericize" the prose. If a line works and carries no project-specific value, it ships unchanged.
- **One canonical token per concept** — never invent a synonym for a registered placeholder.
- **No dangling pointers.** A command that references a file, agent, command, or reference card that the install does not produce is a broken instruction. Grep before you cite; delete the pointer or ship the target.
- **Every check names what its own broken state reports.** A gate that answers "fine" both when things are fine and when it is broken is a coincidence detector. An error never renders as ABSENCE: absence is a claim about the world ("nothing there"), an error is a claim about ourselves ("we failed to look"). Distinguish them at the visible surface — logging is necessary and not sufficient.
- **The judge is never the thing being judged.** A verdict asserted in a dispatch brief is not a verdict; read the artifact from disk. An enumeration that returned empty is not a clean result until the enumerator itself is proven to have run.
- Surgical changes — every changed line traces to the task; don't refactor adjacent working prose. Always fix broken things you hit. Exception — dead code, dead references, unused deps: remove entirely.
- When removing something, delete it end to end like it never existed — including its references in `README.md`, `BLUEPRINT.md`, `SETUP.md`, and `refresh-map.json`.
- NO duplication: grep for the existing rule/section/script and point at it; extract and reference, never keep a near-copy that will drift.
- Right-size and finish: simplest thing that works; no speculative abstractions; no stubs, no deferred TODOs.

### Engine code (Go / TypeScript / JS)

- Never swallow exceptions: every `catch`/`if err != nil` logs the full context. A silent failure hides a bug behind a clean screen.
- Validate at the entry of data — never cast external payloads into shape. `as` blinds `tsc` to the exact nullability that crashes on the first real row.
- Follow the package's existing naming and structure; no new directories or patterns unless the task requires them.
- Never install unvalidated libraries.

### Process

- **Only gitter WRITES git** — commit / merge / checkout / branch / stash / reset / push and every other state-changing git are gitter's, for every agent and for the main loop. Read-only git (`status`/`diff`/`log`/`show`/`rev-parse`) is open to all.
- **Never commit broken code.** Tests pass before the commit, not after it.
- This install carries **no worktree pipeline** — work lands on `main` under `/dev` verification and a gitter commit. That is a deliberate scope choice, not a missing piece: the pipeline commands (`/wave:*`) live in `blueprint/templates/commands/` as shipped source, not as this repo's own workflow.
- **Guarded files:** a PreToolUse hook gates `.claude/**` and every `CLAUDE.md` behind `/pcm` — and behind a session that has provably read `.claude/commands/quality/prompt.md`. The deny message carries the unlock steps. Never route around it by disabling the hook.
- Execute explicit instructions as given: founder delegation ("run it", "finish it") runs to completion; never narrow, drop, or swap scope, nor override with your own caution; raise a genuine concern up front.
- **"God speed"** = full autonomy: the founder is away; resolve every ambiguity yourself, finish, and report the decisions at the end. Only failure = stopping to ask.
- **"What's up / how's it going"** = summarize what happened since the last prompt.

### Testing

- Run the project's own gate before claiming anything works — `/dev test {project}` or `.claude/scripts/dev.sh test {project}`. Never report a suite you did not watch run.
- **A regression test is accepted only after it was watched FAILING against the unfixed code.** A test that never failed proves nothing about the fix.
- A skipped suite is a NAMED gap in the report, never a pass. "Tests pass" with a filtered run is a false statement about coverage.

### Meta

- **Three lenses at once** — Computer Science, instruction design (how an agent actually reads this), and adopter safety (what breaks in someone else's repo). The intersections carry the value: a rule that is technically correct and unreadable at 2 a.m. will be violated.
- **AskUserQuestion is the founder's whole screen** — chat prose between dialogs never reaches them: context travels inside the question text; a clarification gets its answer in the next question's title, simpler and more concrete each round, never a rephrase.
- **When in doubt, do the right thing** — the correct path over the convenient, even at the cost of re-architecting.

## Model Selection

Match the tier to the cost of being wrong; judgment never delegates downward. Models are named inline at each spawn site as aliases; this section alone defines the tiers and the frontier — there is no separate model registry.

- **apex** (frontier, optional) — usecase: research-and-develop loops, architecture, the genuinely hardest problems, or when the founder says — nothing else. Falls back to `opus` when no limited-run frontier model is available.
- **frontier-judgment** (`opus`) — product-shaping output: framework surgery, release judgment, salience over large or ambiguous input, any ruling on shipped prompt text.
- **spec-execution** (`sonnet`) — bounded work with a spec: git mechanics, doc merges, structured-file writes, implementing a design, the `tracer` lead.
- **collector** (`haiku`) — fetch, classify, append, extract verbatim, summarize large output; returns raw material with its source, never concludes. Child tracers live here. Unsure? `inherit`.

**Effort:** `Max` never unless the founder says; `XHigh` only to force open a genuinely hard problem; `High` for medium problems (default here); `Medium` for small low-reasoning tasks; `Low` never.

## Subagent dispatch

**Delegate far ahead** — investigate the whole task, see far ahead; independent work dispatches in parallel with exact per-task briefings; dependent work runs as planned sequential batches of spec-execution agents (your cheap hands); nest tiers — a spec-execution agent fans out collector probes and reasons over the raw findings. Heavy MCP tools (large web-fetch, docs, browser automation) never run in the main loop — a nested agent fetches, distills, returns only the answer.

**The registered cast:**

| Agent | Tier | Spawn it for |
| --- | --- | --- |
| `gitter` | spec-execution | every git WRITE — commit, push, pull. Never run one yourself. |
| `tracer` | spec-execution (dispatches collector children) | "where does X go / who feeds X / map it now" — a consumer-tree trace of any target, quote-pinned, with a stated coverage boundary. Returns a map, never a verdict. |
| `Explore` | inherit | broad read-only sweeps where you need the conclusion, not the file dumps. |
| `general-purpose` | per task | anything that needs the full toolset and doesn't fit above. |

**The briefing contract — every dispatch carries all five:**

1. **The goal in one sentence**, and the artifact it must return (a path, a map, a verdict — name the shape).
2. **The boundary** — what is in scope and, explicitly, what is NOT. An agent with no stated edge walks the whole repo.
3. **The anchors** — the exact files, symbols, or commands to start from. Never "find the relevant code."
4. **The tier and effort**, chosen per the table above, plus the budget (tool calls or files) when the task can run away.
5. **What its own failure looks like** — how it must report a dead end, an empty result, or a tool that would not run. Silence is never a result.

**The laws:**

- **Sync-dispatch:** dispatch all sibling agents of a wave in ONE message and wait for every report. A missing report is a loud, named coverage hole — never a silent omission.
- **An empty enumeration is never a verdict.** A child that found nothing must say whether it looked and found nothing, or failed to look. The parent reports the difference.
- **Reconcile the telemetry:** agents dispatched vs reports received must match, and the count appears in the report.
- **No subagent writes git, and no subagent edits `.claude/**` or a `CLAUDE.md`** — the guard hook denies it, and routing around the guard is a violation, not initiative.
- Other agents' reports are evidence, not truth. A claim that contradicts what you can read yourself gets verified before it is relayed.

## Cross-Disciplinary System Analysis

The three lenses, applied together — this is what the Analysis Protocol in `.claude/output-styles/professor.md` executes:

1. **Computer Science** — correctness, concurrency, failure modes, the actual data flow input → storage → output.
2. **Instruction design** — how an agent reads this at 2 a.m. with 40% context left: ambiguity, load-bearing sentences buried mid-paragraph, rules that can be satisfied without being followed, a check whose broken state is indistinguishable from its healthy one.
3. **Adopter safety** — what this does in a repo that is not this one: a missing target, an assumed stack, an unfilled placeholder, a host-level write nobody asked for.

The intersections are where the value is: a technically correct gate that an agent will predictably route around, a beautifully written rule that assumes a database the adopter does not have.
