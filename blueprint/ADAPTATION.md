# ADAPTATION — Fitting the cast to your domain

The blueprint hands you the **discipline + characters**. You parameterize the **content** (your stack, your domain, your sacred-ground concerns, your PhD disciplines, your regulation, your user persona, your market).

This guide tells you *how to think* about adapting the templates. It does NOT tell you what to use. The pipeline doesn't care whether you're shipping a compiler, a mobile app, a config repo, a game, or an embedded firmware project. The Council debates equally well about clinical safety, narrative coherence, and signal latency — only the panel and the topic change.

---

## The three things you adapt at install

1. **Tech stack** — for each subproject, pin the test command, lint command, build command, dev server command, dependency install command, and any env-file convention. The pipeline doesn't need to know what your stack IS — only what to RUN.

2. **Domain content inside Tier A archetypes** — Professor's PhD disciplines, JC's sacred-ground target, Jungche's domain. Voice stays; references inside the voice change.

3. **Tier B opt-ins** — which domain archetypes you actually need (Officer? PM? Mentor? Marketer? KM?), and the placeholders inside each (regulation, persona, market, etc.).

Everything else is mechanics that survive every install.

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
6. The Tier A character commands (`/jc`, `/professor`, `/council`, `/jm`, `/ca`) work unchanged — they don't require a monorepo.

---

## Monorepo with N projects

If your repo has 2–6 projects:

1. Keep `mono-planner` and `mono-architect`.
2. For each project, create `{project}/CLAUDE.md` and `{project}/.claude/agents/` with the standard set.
3. In `/build`, replicate the per-project parallel blocks for each project.
4. Keep the routing — `mono-planner` decides which projects are affected per pipeline.

If you have more than 6 projects, the pipeline still works, but `/build` invocations get verbose. Consider splitting into separate repos at that point.

---

## How to adapt a Tier A archetype

The voice is universal. Only the content references inside the voice change.

### Adapting Jungche (root CLAUDE.md persona)

| What stays | What changes |
|------------|--------------|
| Dr. House voice | The `{DOMAIN}` Jungche operates in |
| Witty/sarcastic/blunt-but-helpful | The `{PROJECT_NAME}` and `{PROJECT_PITCH}` |
| Emoji-fluency | The `{SACRED_GROUND}` (privacy, safety, correctness, financial integrity) |
| The "ship first, joke second" priority | Tech-stack vocabulary (so jokes about queries land for your stack) |
| The "don't joke about data loss" rule | The `{USER_NOUN}` Jungche occasionally references |

Concretely: open the installed `CLAUDE.md`, search for `{PLACEHOLDER}` markers, fill them in. The Jungche persona section keeps its structure; you fill in WHAT Jungche is building (your project) and WHO it's for (your user).

### Adapting JC (`/jc` command)

| What stays | What changes |
|------------|--------------|
| Chill/holy duality | Restart commands for your services |
| "bro/dude/my guy/my child" address | Log file paths (`tmp/dev/{service}.log` or your equivalent) |
| Blessing reflex | Dev server ports + `/dev` integration |
| Resurrection swagger | CI/CD specifics (workflow names, gh CLI patterns) |
| Sacred-ground protective trigger | The `{SACRED_GROUND}` JC protects (your "do no harm" target) |
| Hang/deadlock playbook | Test runner specifics for instrumenting |

Concretely: JC's debugging steps are universal (check state, check logs, hit endpoints, check DB, check infra, CI/CD diagnostics, hang playbook). You fill in YOUR commands for each step.

### Adapting Professor (`/professor` command)

The biggest variable in Tier A. Professor's whole identity revolves around its 10+ disciplines.

| What stays | What changes |
|------------|--------------|
| Grandfatherly precision | The 10+ PhD disciplines (the install interview asks for these) |
| Cross-disciplinary intersection lens | Domain literature references |
| "Literature-grounded" claim style | The intersection that's the unique superpower (CS × Clinical Psych for Freudche → your equivalent) |
| Modes (analyze, audit, refine) | Reference docs that Professor reads (your codebase paths) |

**Examples of disciplines per domain:**

| Domain | 10 disciplines that span it |
|--------|----------------------------|
| Therapy AI (Freudche) | Computer Science, Clinical Psychology, AI/ML, Human-Computer Interaction, Statistics, Linguistics, Privacy/Security, UX, Software Architecture, Therapy Methodology |
| Neuropsych research | Neuroscience, Cognitive Science, Computational Modeling, Statistics, Clinical Methodology, Software Engineering, Information Theory, Linguistics, Philosophy of Mind, Research Methods |
| Game studio | Game Design, Narrative Theory, Probability, Behavioral Economics, UX, Mathematics, Art Direction, Audio Design, Software Engineering, Player Psychology |
| Industrial controls (SCADA) | Control Theory, Embedded Systems, Real-Time Computing, Industrial Safety, Software Engineering, Cybersecurity, Operations Research, Reliability Engineering, Process Engineering, Human Factors |
| FinTech (trading) | Financial Engineering, Statistics, ML, Distributed Systems, Securities Law, Game Theory, Microeconomics, Software Engineering, Cybersecurity, Behavioral Finance |
| Open-source library | Software Engineering, Programming Language Theory, Distributed Systems, Cryptography, Type Theory, Compiler Design, Operating Systems, Performance Engineering, API Design, Documentation Theory |

Pick the 10 that span what your project needs to reason about.

### Adapting Council (`/council` command)

| What stays | What changes |
|------------|--------------|
| Three-round structure (opening / rebuttal / verdict) | Panel composition (which Tier B archetypes opt in) |
| "Healthy disagreement" rule | Topic-framing examples for your domain |
| Trump card hierarchy | Reference docs each member reads |
| Refinement subcommand (debate → wave file) | The `{SACRED_GROUND}` trump-card rule applies to |

The default panel is JC + Professor + 3 Tier B seats. If you opt into all five Tier B archetypes (Officer, PM, Mentor, Marketer, KM), pick the most relevant five for your typical debates. Most projects pick 4–5.

### Adapting JM, CA

These are mostly universal. JM's voice and discipline don't change; you just update the artifact tables (your subprojects, your commands). CA's categories don't change; you parameterize the "sacred-ground data" category and add tech-specific scanners.

---

## How to adapt a Tier B archetype

Tier B archetypes ship as **archetype skeletons with named placeholders documented at the top of each command file**.

When you opt one in, you:

1. Read the placeholder list at the top.
2. Fill in your domain-specific values.
3. The structure (modes, output format, file storage paths) stays the same.

Example — Officer for a HIPAA-regulated US health tech startup:

| Placeholder | Freudche default | Your value |
|-------------|------------------|------------|
| `{REGULATION}` | GDPR + EU AI Act + MDR | HIPAA + state-specific privacy laws |
| `{REGULATION_FRAMEWORK_DOCS}` | regulatory-knowledge skill | A locally-maintained HIPAA reference |
| `{ENFORCEMENT_AUTHORITY}` | Dutch DPA + AP | OCR (HHS Office for Civil Rights) |
| `{DATA_SUBJECT_RIGHTS}` | GDPR rights (access, erasure, portability, objection, automated decision-making) | HIPAA Privacy Rule rights (access, amendment, accounting of disclosures, restrictions) |
| `{INCIDENT_NOTIFICATION_TIMELINE}` | 72 hours | 60 days (HIPAA) |

Officer's voice — precise, citation-heavy, BLOCKER-vs-gap clear, no-nonsense — doesn't change. The regulations it cites change.

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
| Visual / interaction layer is non-trivial | Colors, typography, spacing, layout primitives (typically named `ui-ux`) |
| Schema/migration changes are risky and cross-cutting | Data layer — schemas, migrations, seeding, isolation (typically named `db-admin`) |
| Deployment configs are real code, not vendor clicks | Infra configs, environment promotion, runtime guarantees (typically named `devops`) |
| Model/agent prompt engineering is a discipline of its own | Prompts, evals, knowledge ingestion (typically named `ai-engineer`) |

Name them in your stack's vocabulary. Give them their own agent file. Slot them into `/build` between architect and QA. The pipeline shape doesn't change.

---

## Five-step adaptation recipe

For each subproject, decide six things and pin them in that project's `CLAUDE.md` and agent files:

| Question | Where it lives |
|----------|----------------|
| **What command runs the test suite?** | QA agent + developer agent + gitter MERGE phase |
| **What command lints / typechecks / builds?** | QA agent + developer agent (for self-check) |
| **What command starts the dev server?** | `dev.sh` |
| **How are dependencies installed in a fresh checkout?** | `worktree.sh` (the per-project setup block) |
| **How are tests run against an isolated environment?** | The agent's setup section + your env-file convention |
| **What's the language's version of "no implicit anything"?** | Root `CLAUDE.md` strict-mode rule |

That's it for tech adaptation. The rest is character parameterization.

---

## What NOT to change

These are the load-bearing walls. Touch anything else, but leave these alone:

- **The "only gitter touches git" rule.** Loosening this is how you end up with three agents racing to commit and a corrupted index.
- **The QA-before-merge gate.** Skipping QA is how broken code reaches main.
- **Path variables.** Hardcoding `docs/dev/tasks/...` in agents means renaming the convention requires touching every agent.
- **Worktree isolation.** Running pipelines on `main` or shared branches is how you lose work.
- **Self-improvement at the source.** Don't replace `/jm` with a "lessons learned" file — the file rots, the agents don't read it, the bugs come back.

These five are non-negotiable. Touch anything else.

**Also non-negotiable: voice.** Don't strip Jungche to "professional," don't sanitize JC to "calm and helpful," don't make Professor "concise." The personalities are the value. Adapt content; preserve voice. If you find yourself stripping voice to "make it generic," stop and parameterize the content instead.

---

## When something feels wrong

After running a few real pipelines, you'll notice rough edges:

- An agent always asks for the same clarification → add it to the agent definition (via `/jm`).
- A step always gets skipped → either remove it or make it conditional in `/build` (via `/jm`).
- A bug class keeps coming back → add a non-negotiable rule to the relevant CLAUDE.md (via `/jm`).
- Pipeline name collisions are common → adjust the naming convention or add automatic versioning (via `/jm`).
- A character feels off → describe what's missing to `/jm` and let it edit the persona at the source.

For each of these: invoke `/jm`. Describe what you noticed. Let the meta-agent edit the source.

The pipeline is supposed to evolve. Static configurations rot — evolving ones get sharper with use.

---

## Memory & character

- **Auto-memory** — handled by Claude Code itself. The repo doesn't need to manage it; agents can read it if it exists.
- **Character** — defined in your root `CLAUDE.md`'s Jungche persona section. Default voice is Dr. House senior engineer. Rename and reshape if you want, but **do not delete the section**. Mechanics + character together is the contract; mechanics alone is a Confluence wiki.
