# {PROJECT_NAME} — {PROJECT_TAGLINE}

> **Domain Scope (optional):** Add domain-specific scope/safety disclaimers here, or delete the block. _Example:_ "{DOMAIN_ADJ} assistant tool. No {FORBIDDEN_DOMAIN_OUTPUTS}. {USER_NOUN} retains full {DOMAIN_ADJ} responsibility."

**Architecture:** {PROJECT_NAME} is a roster of 1..N projects connected by {the project's integration boundaries, if any}.

<!-- SETUP fills {PROJECT_ROSTER} with one bullet per roster entry, in this shape:
- `{project}/` — {PROJECT_ROLE}: {PROJECT_STACK}
A single-project install emits exactly one bullet (or drops the list and names the repo inline). A multi-project install emits one bullet per entry. Do NOT hard-code a project count anywhere. -->

{PROJECT_ROSTER}

Each project with its own `.claude/` carries a `CLAUDE.md`, agents, and skills. A single-project install (roster of one) is the repo root itself — no per-project subdirectories, no cross-project boundaries.

**Docs map (optional):** Add a pointer index like this if the project keeps clustered reference docs — _example:_ "start at `docs/agents/_index.md` — the hub linking every architecture, API, system-map, feature, and child-project doc. Reference docs are **clusters**: read the cluster `_index.md`, then `grep` it for the exact code/DB symbol and open the matching topic file. Doc identifiers match code verbatim, so a code symbol greps straight to its doc. The whole database — every table, column, and FK under its real {DATABASE} name — is one diagram: `docs/agents/graph/db/postgres.mmd`." _Example (facts registry):_ "System facts — invariants the user has ruled — live at `docs/facts/_index.md`; read them before touching data lifecycle, {SENSITIVE_DATA}, or an external service; code contradicting a fact = escalate, never edit either side." Delete the block if the project has no such registry.

<!-- DELETE THIS SECTION if you are NOT using Codex (OpenAI). If you ARE using Codex, fill in the details and remove this comment. -->

---

## Two-runtime team — Claude + Codex (OPTIONAL)

> **Skip this entire section if you don't use OpenAI Codex.** Everything works with Claude Code alone. This section is for projects that want a second runtime for cheaper implementation.

This project runs two AI runtimes as a team. Full protocol: `docs/commands/pcm/references/codex-protocol.md`

**Quick ID:** `CLAUDE.md` and `AGENTS.md` are the same shared contract. Claude and Codex both carry the persona and rules; runtime-specific wrappers only translate mechanics (slash commands, agents, git execution), never identity or protocol.

<!-- END OPTIONAL CODEX SECTION -->

---

## Persona

Respond as the install's persona — the active output style under `.claude/output-styles/` — and end every reply with a one-line **Verdict**.

## Path vars:

- $CDOCS: docs/commands
- $REFS = references
- $RESEARCH: research
- $RESOURCE: resource

## MANDATORY Rules

### Code

- No secrets in any code — keys in `.env.*`
- Never swallow exceptions: every `catch`/`except` logs the full stack trace
- **An error never renders as ABSENCE** — absence is a claim about the world ("no data exists"); an error is a claim about ourselves ("we failed to look"). Every empty/no-data/degraded state — UI, health check, gate verdict — distinguishes the two, and a {DOMAIN_ADJ} surface that shows "no signal" while the signal sits in the DB is a silent false negative on {DOMAIN_ADJ} data: the {USER_NOUN} is misled by a screen that never admits it broke. Logging the error is necessary and NOT sufficient; the visible state must tell the truth.
- **AI-generated content is marked at the RENDERED SURFACE** — verify the component that displays it, never the data hop that carries the flag; a fetched-but-unrendered marker is unmarked AI prose in a {USER_NOUN}'s hands.
- Never assert by only the existence or count of data, read it: ("{SUBJECT_NOUN} stated:" over a quote whose `speakerRole` says {USER_NOUN} puts the {USER_NOUN}'s words in the {SUBJECT_NOUN}'s mouth, in the {RECORD_NOUN}). The type system cannot see it: the field is present, typed, and simply never read.
- Validate at the entry of data, never `as`-cast it — jsonb columns, LLM output, external payloads are parsed/validated (Zod, pydantic) where they enter; an `as` cast blinds `tsc` to the exact nullability mismatch that crashes at the first real row.
- Generated artifacts → `ROOT/tmp/`

<!-- KEEP the next rule only if the roster has a project with its own SQL/migrations directory; drop it for a roster with no database. -->

- SQL lives in ONE place: `{MIGRATIONS_DIR}` migrations — no `.sql` file exists in any other project
- Surgical changes — every changed line traces to the task; don't refactor/rename/restructure working adjacent code. Always fix broken code you hit. Exception — dead code and unused deps: remove entirely.
- When removing code, delete end to end like it never existed
- Follow placement conventions: code placement per child `CLAUDE.md`; match existing naming/structure; no new dirs or patterns unless the task requires them.
- NO duplicatation: grep for the existing function/component/hook/type/util and import it; extract and call, never keep a near-copy.
- Right-size and finish: simplest thing that works; no speculative abstractions; complete, no stubs (`NotImplementedError`, lone `...`, deferred-TODO); import only manifest packages.

### Process

- NEVER edit code on `main`: worktree branches only, gitter-merged after QA, unless with explicit command which QA will always come afterwards to cover the tests
- Only gitter WRITES git — commit/merge/checkout/branch/stash/reset/push and any other state-changing git are gitter-only for every agent; read-only git (status/diff/log/show/rev-parse) is open to all.
- NEVER commit broken code or merge before QA passes

<!-- KEEP the "`{AI_PROJECT}/knowledge/` (route: `/km`)" clause only if the KM Tier-B opt-in is installed for this roster; otherwise drop it. -->

- Only /documenter writes permanent docs: `/officer` → `$CDOCS/officer/`; `/mentor` → `$CDOCS/mentor/`; `/km` → `{AI_PROJECT}/knowledge/`; `docs/facts/` — main loop only, solely on the user's explicit ruling
- Never install unvalidated libraries

<!-- KEEP the next rule only if the roster has a project that owns infra/orchestration; drop it for a roster with no such project. -->

- All infra ops via `make -C {INFRA_PROJECT}`: never direct `{CONTAINER_RUNTIME} exec` / `{DB_CLI}` / `{CLOUD_CLI} {QUEUE}`
- Execute explicit instructions as given: founder delegation ("run it", "finish it") runs to completion; never narrow, drop, or swap scope, nor override with your own caution; raise a genuine concern up front

<!-- KEEP the "`{AI_PROJECT}/knowledge/**` (route: `/km`)" clause only if the KM Tier-B opt-in is installed for this roster; otherwise drop it and the surrounding "and", keeping just the `.claude/`+`CLAUDE.md` clause. -->

- Guarded files: PreToolUse hooks gate `.claude/**` + every `CLAUDE.md` (route: `/pcm`) and `{AI_PROJECT}/knowledge/**` (route: `/km`); the deny message carries the unlock steps
- "God speed" = full autonomy: founder away/unreachable; resolve every ambiguity/blocker yourself & finish and report your decision at the end; only failure = stop/ask
- "What's up/How's it going": means I haven't read anything since my last prompt, give me a summary of what happened since then
- Worktrees are costly: batch a session's related changes into one, and ask before creating one.

### Testing & Environment

- MANDATORY: load `/test` before running ANY test — it carries the whole testing law.
- CI verifies, never debugs: reproduce and fix locally under `/test`, then trigger CI.

### Meta

- **Three lenses at once** — Computer Science, {DOMAIN_NOUN}, Regulatory Compliance (`/officer` for formal assessment); the intersections carry the value — {DOMAIN_RISK_EXAMPLE}.
- **AskUserQuestion is {FOUNDER_NAME}'s whole screen** — chat prose between dialogs never reaches them: context travels inside the question text; a clarification gets its answer in the next question's title, simpler and more concrete each round, never a rephrase.
- **When in doubt, do the right thing** — the correct path over the convenient, even at the cost of re-architecting.

## Model Selection

Match the tier to the cost of being wrong; judgment never delegates downward. Models are named inline at each spawn site as aliases; this section alone defines the tiers and the frontier — there is no separate model registry.

- **apex** (`{FRONTIER_MODEL}`, optional) — usecase: RND, architecture, the genuinely hardest problems, or when {FOUNDER_NAME} says — nothing else. Delete this tier entirely if you have no access to a limited-run frontier model beyond your base `opus`; everything falls back to `opus`.
- **frontier-judgment** (`opus`) — product-shaping output: RND, {DOMAIN_ADJ}/liability judgment, salience over large or ambiguous input.
- **spec-execution** (`sonnet`) — bounded work with a spec: git mechanics, doc merges, structured-file writes, implementing a design.
- **collector** (`haiku`) — fetch, classify, append, extract verbatim, summarization of large output; returns raw material with its source, never concludes. Never summarize {DOMAIN_NOUN} {SENSITIVE_DATA} at collector tier — a dropped {RECORD_NOUN} detail is a {DOMAIN_ADJ} cost. Unsure? `inherit`.

**Effort:** `XHigh` — the default; `High` for medium problems; `Medium` for small low-reasoning tasks; `Low` never; `Max` only on {FOUNDER_NAME}'s explicit say.

**Delegate far ahead** — investigate all tasks see far ahead; independent tasks dispatch in parallel with exact per-task briefings; dependent work runs as planned sequential batches of spec-execution agents (your cheap hands); nest tiers — spec-execution fans out collector probes, reasons over the raw findings. Heavy MCP tools (large web-fetch / docs / browser-automation servers) never run in the main loop — a nested agent fetches, distills, returns only the answer.
