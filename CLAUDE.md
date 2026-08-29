# Professor — the discipline layer for Claude Code

**What this repo is:** the framework itself, not an app that uses it. Everything under `templates/` is **shipped source** — an adopter's live agent prompts, one clone away. Treat every prompt line as production code, because it is.

## Repo structure

- `templates/`: the shipped framework an adopter clones — `templates/project/` (per-install templates) + `templates/global/` (machine-global agents/commands/skills — the originals). Markdown + shell, no build; the gates are `scripts/leak-check.sh` and `scripts/refresh-scope.sh`.
- **Globalization protocol:** `templates/global/` content is globalized — `pfm install` symlinks each original into its engine's global registry (`~/.claude/agents/`, `~/.claude/commands/`); engine twins (Codex `.toml`) generate at release time, live committed beside their originals, and link into `~/.codex/agents/` — one original, every engine, updates propagate through the link.
- `pfm/`: fleet engine — Go 1.24, `cmd/pfm` + `internal/*`. Owns its staged host assets under `pfm/internal/installer/assets/`; `pfm install` stages them. Also owns the memory organ under `internal/dream` and the only harvester under `internal/harvest` + `internal/harvestmcp`, over a pinned Python conversion sidecar in `internal/harvestpy/`.
- `engines/wave-walker/engine/`: wave-walker engine — JS/TS compiled by `cross-workflow` for both the Claude Workflow runtime and the Codex SDK.
- `docs/`: the specs — `BLUEPRINT.md` (philosophy), `SETUP.md` (generation), `PLACEHOLDERS.md` (substitution law) — plus `commands/` reference cards and `dev/` wave trains.
- `scripts/`: repo-level gates (`leak-check.sh`, `refresh-scope.sh`); `.githooks/` runs the leak gate `pre-push`.
- `releases/` + root `README.md` / `INSTALL.md` / `CHANGELOG.md` / `VERSION`: the public face — edited with template-grade care.
- `.claude/`: this repo's live install — commands, skills, agents, scripts — the source of truth. `.codex/` and `.opencode/`: pointer layers compiled over it, never a restatement.
- `.professor/`: ledgers — `drift.md` (keep-local), `release.md` (pending upstream), `retro.md` (steering inbox).
- `tmp/`: gitignored scratch — every generated artifact lands here, never in a tracked dir.

Build/test through `.claude/scripts/dev.sh {status|install|build|typecheck|verify|test} {templates|pfm|walker}`.

## Three-runtime team — Claude + Codex + OpenCode

`CLAUDE.md` and `AGENTS.md` are one shared contract; runtime wrappers translate mechanics, never identity or protocol. `AGENTS.md` is **compiled** from this file — never hand-edited, never a symlink; edit `CLAUDE.md` and the `Stop` hook recompiles both mirrors. OpenCode reads the same compiled `AGENTS.md` (its loader prefers it over `CLAUDE.md`) plus its own `.opencode/` layer, compiled by `build-opencode.mjs`: agents (`.opencode/agent/*.md`), commands (`/flat-name`), skill symlinks, and `opencode.jsonc`, where guarded-file and non-gitter Git-write denies remain pinned. Every `.claude/agents/*.md` role compiles for Codex and OpenCode; only registered `gitter` retains Git-write authority. The active main Codex chat may use the user-authorized fallback under § Process when gitter is unavailable. After a Bash-driven write bypassed the hook:

```bash
pfm codex build . && pfm codex check .
node .claude/scripts/build-opencode.mjs generate && node .claude/scripts/build-opencode.mjs doctor
```

`pfm codex build` is the SINGLE writer of the Codex mirror; the legacy repo-local JS compiler is retired — `templates/project/scripts/build-codex.mjs` lives only in the adopter blueprint.

## Path vars

`$CDOCS`: `docs/commands` · `$REFS`: `references` · `$RESEARCH`: `research` · `$RESOURCE`: `resource`

## MANDATORY Rules

### Publication (this repo's sacred ground)

- **No push, tag, or release without an explicit request in the current turn.** A finished task, a green build, a "finish it", or a completed release document is never permission to publish. The authorized writer publishes only on the user's plain ask in that turn.
- **Nothing identifying ships:** no source-project brand, no user PII, no client domain content, no machine-absolute path (`/home/…`, `/Users/…`) in any tracked file. `scripts/leak-check.sh` (`pre-push`) is the backstop, not the plan — write it clean the first time.
- Template example values are invented placeholders, never mined from a live private repo.
- **Version discipline:** `VERSION`, `CHANGELOG.md`, `releases/vX.Y.Z.md`, and the tag agree or the release is wrong; `/pfm:release` owns the sequence.

### Prompt & template code

- **A template IS the live source file, verbatim** — same structure, mechanics, character, logic; only project-specific values swap for `docs/PLACEHOLDERS.md` tokens. Never abstract, skeletonize, or "genericize" the prose.
- One canonical token per concept — never a synonym for a registered placeholder.
- **No dangling pointers:** grep before you cite; a referenced file/agent/command the install does not produce = delete the pointer or ship the target.
- **Every check names what its own broken state reports.** A gate answering "fine" when healthy AND when broken is a coincidence detector. An error never renders as ABSENCE — absence claims "nothing there", an error claims "we failed to look"; distinguish them at the visible surface, logging alone is not sufficient.
- **The judge is never the thing being judged:** read the artifact from disk, never trust a verdict asserted in a brief; an empty enumeration is clean only once the enumerator provably ran.
- Surgical changes: every changed line traces to the task; fix broken things you hit; dead code/references/deps — remove entirely, end to end (including `README.md`, `BLUEPRINT.md`, `SETUP.md`, `refresh-map.json`).
- NO duplication: grep for the existing rule/section/script and reference it; never keep a near-copy that will drift.
- Right-size and finish: simplest thing that works, no speculative abstractions, no stubs or deferred TODOs.

### Engine code (Go / TS / JS / Python)

- Never swallow exceptions — every `catch` / `if err != nil` logs full context.
- Validate at data entry; an `as`-cast blinds `tsc` to the nullability that crashes on the first real row.
- Follow the package's existing naming and structure; new dirs/patterns only when the task requires them.
- Never install unvalidated libraries.

### Process

- **Git writes use registered gitter.** Every other subagent is read-only. When gitter is unavailable, only the active main Codex chat may perform scoped Git writes, and only after explicit user authorization in the current turn. Publication still requires the separate explicit in-turn request above.
- **Never commit broken code** — tests pass before the commit.
- **Code waves build inside the fence** — a git worktree under `.worktrees/{train}/`, every build/test through `dev.sh iso` (the `infra/` container: fresh machine, own HOME, worktree mounted; design: `docs/dev/isolated-dev-foundation.md`). The live checkout, the host's `~/.local/bin`, and the real `$HOME` are never dev targets. Markdown-only waves (templates/docs/prompts) land on `main` directly. A fenced wave closes in order: QA pass → orchestrator review with issues fixed → authorized Git writer merges to `main` → the host mirror build (`go build -o ~/.local/bin/pfm ./cmd/pfm` + `pfm install --yes`). The installed wave commands (`/wave:refine`, `/wave:live`, `/wave:walker`, `/wave:walker-invariants`, `/wave:ccc`) are rewired to this cast — `dev` builds, `qa` tests, `$git` commits and merges; a task touching `.claude/**`, any `CLAUDE.md`, or `templates/**` routes to `/pfm`. Their `templates/project/commands/wave/` twins keep the adopter pipeline.
- **Guarded files:** a PreToolUse hook gates `.claude/**` and every `CLAUDE.md` behind `/pfm` plus a session that has read `.claude/commands/quality/prompt.md`; the deny message carries the unlock steps. Never route around it by disabling the hook.
- Execute explicit instructions as given: user delegation runs to completion — never narrow, drop, or swap scope; raise a genuine concern up front.
- "God speed" = full autonomy: resolve every ambiguity yourself, finish, report the decisions at the end; only failure = stop/ask.
- **Milestone = compact point:** at every milestone, checkpoint the plan to a `tmp/` file, then give yourself a compact before the next phase (a held turn arms an idle-fired self-inject instead).
- "What's up / how's it going" = summarize everything since the last prompt.
- **AskUserQuestion is the user's whole screen** — context travels inside the question text; each round simpler and more concrete, never a rephrase.
- When in doubt, do the right thing — correct over convenient, even at re-architecting cost.

### Testing

- Run the project's own gate before claiming anything works: `.claude/scripts/dev.sh test {project}` — never report a suite you did not watch run.
- **A regression test counts only after it was watched FAILING against the unfixed code.**
- A skipped or filtered suite is a NAMED gap in the report, never a pass.

## Subagent dispatch

Tiers, effort, and delegation posture live in the fleet prompt's § Model Selection — never restated here. The cast, its triggers, and each agent's pinned model live in the harness registry (`.claude/agents/` frontmatter, injected every session).

**The briefing contract — every dispatch carries all five:**

1. The goal in one sentence, and the artifact it must return (a path, a map, a verdict — name the shape).
2. The boundary — what is in scope and, explicitly, what is NOT.
3. The anchors — exact files, symbols, or commands to start from; never "find the relevant code".
4. The tier and effort per the fleet prompt's § Model Selection, plus a budget when the task can run away.
5. What its own failure looks like — how to report a dead end, an empty result, a tool that would not run. Silence is never a result.

**The laws:**

- Sync-dispatch: all sibling agents of a wave go in ONE message; a missing report is a loud, named coverage hole.
- An empty enumeration is never a verdict: "looked and found nothing" ≠ "failed to look" — the parent reports which.
- Reconcile telemetry: agents dispatched vs reports received must match, and the count appears in the report.
- **Only gitter writes git; no subagent edits `.claude/**` / a `CLAUDE.md`** — the guard denies those framework edits; routing around it is a violation, not initiative.
- Agent reports are evidence, not truth — verify a claim against what you can read yourself before relaying it.

## Cross-Disciplinary System Analysis

The three lenses, applied together — the procedure: **Orient** (read `docs/BLUEPRINT.md` and `docs/SETUP.md` § the relevant phase; for an engine, its own spec before its code) → **Map before judging** (spawn `subagent_type: tracer` on the target; a map is never a verdict, it FEEDS one) → **Deep dive** (implementations + tests — what's tested vs NOT — plus the prompt files that drive them; for a prompt, what it makes an agent DO, not what it claims) → **Report** (verdict HEALTHY | NEEDS ATTENTION | CRITICAL ISSUES; findings per lens; cross-disciplinary insights; recommendations table). The lenses:

1. Computer Science — correctness, concurrency, failure modes, the actual data flow.
2. Instruction design — how an agent reads this at 2 a.m. with 40% context left: ambiguity, buried load-bearing sentences, rules satisfiable without being followed, a check whose broken state looks healthy.
3. Adopter safety — what this does in a repo that is not this one: a missing target, an assumed stack, an unfilled placeholder, a host-level write nobody asked for.

The value is in the intersections: a technically correct gate an agent will predictably route around; a beautiful rule assuming a database the adopter does not have.
