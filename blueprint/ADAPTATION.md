# ADAPTATION — Fitting the discipline to your stack

The blueprint is **technology-agnostic on purpose**. You will not find a single language, framework, package manager, runtime, database, build tool, or cloud provider named in this document — and that is the feature, not the omission.

This guide tells you *how to think* about adapting the templates. It does not tell you *what to use*; that is your call, and the pipeline does not care.

---

## Single-project repo

If your repo has only one project (no monorepo):

1. Skip child CLAUDE.md files entirely.
2. Skip child agent directories. All agents live at `.claude/agents/`.
3. In `.claude/agents/`, you have:
   - `gitter.md`
   - `mono-documenter.md`
   - `planner.md` (no project prefix)
   - `architect.md`
   - `developer.md`
   - `qa.md`
4. Drop `mono-planner.md` and `mono-architect.md` — there's no cross-project consolidation needed. The orchestrator goes straight from `planner` → `architect` → `developer` → `qa`.
5. In `/build`, remove the parallel-fan-out steps. Keep the linear pipeline.

---

## Monorepo with N projects

If your repo has 2–6 projects:

1. Keep `mono-planner` and `mono-architect`.
2. For each project, create `{project}/CLAUDE.md` and `{project}/.claude/agents/` with the standard set.
3. In `/build`, replicate the per-project parallel blocks for each project.
4. Keep the `$PROJECTS` routing — let `mono-planner` decide which projects are affected per pipeline.

---

## How to adapt to your stack (generic recipe)

For each subproject, decide six things and pin them in that project's `CLAUDE.md` and agent files:

| Question | Where it lives |
|----------|----------------|
| **What command runs the test suite?** | QA agent + developer agent + gitter MERGE phase |
| **What command lints / typechecks / builds?** | QA agent + developer agent (for self-check) |
| **What command starts the dev server?** | `dev.sh` |
| **How are dependencies installed in a fresh checkout?** | `worktree.sh` (the per-project setup block) |
| **How are tests run against an isolated environment?** | The agent's setup section + your env-file convention |
| **What's the language's version of "no implicit anything"?** | Root `CLAUDE.md` strict-mode rule |

That's it. The pipeline doesn't need to know what your stack *is* — only what to *run*. Pin the commands, and the agents fill in the rest.

---

## Test environment isolation (the discipline, not the tools)

The template assumes two environment files per project:

- `.env.local` — local development
- `.env.test` — integration tests

The contract is what matters, not the specific tools:

- Tests load `.env.test`. Never `.env.local` for DB/port config.
- For credentials that only exist in `.env.local` (paid API keys, etc.), load them separately *without* overriding `.env.test`.
- One canonical command resets the test environment between runs. Agents call that command, never reach around it.

If your stack uses different file names or a different convention, fine — keep the *contract* and rename to taste.

---

## Mock policy (universal — keep verbatim)

This rule is technology-independent and load-bearing:

- **Mock external dependencies** — anything you don't own and that costs money, has rate limits, or is flaky. Paid APIs, third-party SaaS, model providers, transactional email, anything outside your trust boundary.
- **Never mock internal dependencies within 1 hop.** A frontend's integration test hits the *real* backend. A backend's integration test hits the *real* database and the *real* queue. A worker's integration test uses a *fake* model provider but the *real* queue.

The distinction is **external vs internal**, not "mock vs no mock." This rule survives every stack, every domain, every layer.

---

## Specialist agents (when, not which)

Beyond the standard four (`planner`, `architect`, `developer`, `qa`), some projects benefit from a specialist that owns a narrow concern. The pipeline doesn't tell you which to add — it tells you *when* you'd want one:

| You'd add a specialist when… | Concern they'd own |
|------------------------------|--------------------|
| Visual / interaction layer is non-trivial | Colors, typography, spacing, layout primitives |
| Schema/migration changes are risky and cross-cutting | Data layer — schemas, migrations, seeding, isolation |
| Deployment configs are real code, not vendor clicks | Infra configs, environment promotion, runtime guarantees |
| Model/agent prompt engineering is a discipline of its own | Prompts, evals, knowledge ingestion |

Name them in your stack's vocabulary. Give them their own agent file. Slot them into `/build` between architect and QA. The pipeline shape doesn't change.

---

## Optional: domain-specific commands

The blueprint ships only the technology-agnostic core: `/build`, `/jc`, `/ccm`, `/dev`. Some teams find narrower commands useful — a compliance advisor that owns regulatory questions, a knowledge curator that owns a research corpus, a multi-agent debate, an auditor, a wave runner. We deliberately do not ship templates for these: they leak domain assumptions, and the right shape for one project is noise in another.

If you want one, the abstract pattern is:

1. **Single concern.** A command earns its place when it owns a coherent piece of the project that doesn't belong to `/build` or `/jc`.
2. **Idempotent on docs.** It writes only to `docs/commands/{cmd}/` (`$REFS`, `$RESEARCH`, `$RESOURCES`). Permanent docs are still mono-documenter's territory.
3. **Pipeline-respectful.** If it produces code changes, it routes through `/build` (for new work) or `/jc` (for fixes), never directly to `main`.
4. **Discoverable.** It appears in the root `CLAUDE.md` commands table with a one-line description.

Model it on `/build` or `/jc` — whichever has the right shape — and adapt.

---

## Memory & character

The blueprint does NOT include the auto-memory directory or a character system. Both are user-specific:

- **Auto-memory** — handled by Claude Code itself. The repo doesn't need to manage it; agents can read it if it exists.
- **Character** — define in your root `CLAUDE.md` if you want personality. Be specific about tone, when to drop the persona, and what NOT to do (e.g., "no jokes about data loss"). Or delete the section entirely. Mechanics first; personality is taste.

---

## What NOT to change

These are the load-bearing walls. Touch anything else, but leave these alone:

- **The "only gitter touches git" rule.** Loosening this is how you end up with three agents racing to commit and a corrupted index.
- **The QA-before-merge gate.** Skipping QA is how broken code reaches main.
- **Path variables.** Hardcoding `docs/dev/tasks/...` in agents means renaming the convention requires touching every agent.
- **Worktree isolation.** Running pipelines on `main` or shared branches is how you lose work.
- **Self-improvement at the source.** Don't replace `/ccm` with a "lessons learned" file — the file rots, the agents don't read it, the bugs come back.

These five are non-negotiable. Touch anything else.

---

## When something feels wrong

After running a few real pipelines, you'll notice rough edges:

- An agent always asks for the same clarification → add it to the agent definition.
- A step always gets skipped → either remove it or make it conditional in `/build`.
- A bug class keeps coming back → add a non-negotiable rule to the relevant CLAUDE.md.
- Pipeline name collisions are common → adjust the naming convention or add automatic versioning.

For each of these: invoke `/ccm`. Describe what you noticed. Let the meta-agent edit the source.

The pipeline is supposed to evolve. Static configurations rot — evolving ones get sharper with use.
