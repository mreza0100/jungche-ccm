# SETUP — Installing the pipeline in your project

Step-by-step. Assumes you have a git repository and Claude Code installed.

The blueprint is **technology-agnostic** — nothing here will tell you which package manager, test runner, language, or framework to use. You bring those. The pipeline brings the discipline.

---

## Prerequisites

- Git repository (at least one commit on `main` or `master`)
- Claude Code CLI installed and configured
- A clear list of your "subprojects" (or one if it's a single-project repo)
- Knowledge of your existing dev/test ports (so worktree allocations don't collide)

---

## Step 1 — Decide your structure

**Single-project repo** (one codebase): all agents live at `.claude/agents/`. Skip child CLAUDE.md files. You can drop `mono-planner` and `mono-architect` since there's no cross-project consolidation.

**Monorepo** (multiple projects): root agents at `.claude/agents/`, project agents at `{project}/.claude/agents/`.

For each subproject, write down (in your own terms — not in this doc):

- Directory name (you choose)
- One-line description of what it does
- The commands you already use to: install dependencies, run tests, lint, typecheck, build, run a dev server
- The default dev port (if it has one)

Keep this list — you'll plug it into multiple files.

---

## Step 2 — Copy the scaffolding

From the blueprint directory, copy:

```bash
# from your project root
cp blueprint/templates/CLAUDE.md ./CLAUDE.md
cp -r blueprint/templates/agents .claude/
cp -r blueprint/templates/commands .claude/
cp -r blueprint/templates/scripts .claude/
chmod +x .claude/scripts/*.sh
mkdir -p docs/agents docs/commands docs/dev/tasks/archive docs/dev/waves
echo ".worktrees/" >> .gitignore
echo "tmp/" >> .gitignore
```

---

## Step 3 — Edit `CLAUDE.md` (root)

Open the copied `CLAUDE.md` and replace placeholders:

| Placeholder | Replace with |
|-------------|--------------|
| `{PROJECT_NAME}` | Your project name |
| `{PROJECT_PITCH}` | One-line description |
| `{SUBPROJECT_LIST}` | Bulleted list of subprojects with one-line descriptions |
| `{REPO_TREE}` | Your actual repo structure |
| `{LANGUAGE_STRICT_MODE_RULE}` | Whatever strictness rule fits your stack (or delete the bullet) |
| `{CHARACTER_OR_DELETE}` | Either define a character or delete the section entirely |

Keep all the **Non-Negotiable Rules** sections verbatim unless you have a specific reason to change them.

---

## Step 4 — Edit the scripts

### `worktree.sh`

Open `.claude/scripts/worktree.sh` and find the section labeled `# === Per-project setup — EDIT FOR YOUR STACK ===`. Replace the placeholder with the commands your projects need to bootstrap a fresh checkout — install dependencies, generate types, copy env files, whatever you do today by hand. The script just needs to leave a runnable checkout behind.

Also adjust the `.env.local` / `.env.test` rewriting blocks (or your equivalent) to swap port numbers per worktree, so two parallel pipelines don't collide on the same DB or HTTP port.

### `alloc-ports.sh`

Open `.claude/scripts/alloc-ports.sh` and adjust the port ranges at the top to match the slots your stack needs (one variable per logical port). Each range should be 99 slots wide and not overlap with anything you currently run.

The script is concurrency-safe — multiple pipelines can `alloc` simultaneously without racing.

### `dev.sh`

Open `.claude/scripts/dev.sh` and replace the placeholder `start_project` calls with one line per subproject — the directory and the command you'd run in a terminal to start it.

---

## Step 5 — Edit child agent files (if monorepo)

For each subproject, create `{project}/.claude/agents/` with these files (copied and adapted):

- `planner.md`
- `architect.md`
- `developer.md`
- `qa.md`

Adapt:
- The agent's "Context" / "Tech context" line — one sentence describing the project in your stack's terms
- The test / lint / typecheck / build commands the agent will run
- Any project-specific conventions worth pinning (e.g., directory layout, import rules, schema locations)

The templates use placeholders like `{PROJECT_DIR}`, `{TEST_CMD}`, `{LINT_CMD}` — replace them.

---

## Step 6 — Edit `/build` command

Open `.claude/commands/build.md` and:

1. Replace the project list in **Step 1a** (parallel codebase analysis) with your actual subprojects.
2. Replicate the architect / developer / QA invocation steps with one block per subproject.
3. Replace any placeholder directory names with your real ones.

Read the entire file once. The pipeline structure is the same; only the project names and paths change.

---

## Step 7 — Edit `gitter` agent

Open `.claude/agents/gitter.md` and update:

- The **Pipeline context** section's project list
- The **MERGE phase** section's per-project test commands (if you want gitter to verify before merging)
- The **POST-MERGE QA** section's project loop

Gitter is the most complex agent; touch it last and read the full file before editing.

---

## Step 8 — Test the install

Run a smoke test. From the project root:

```bash
.claude/scripts/alloc-ports.sh alloc test-pipeline
.claude/scripts/alloc-ports.sh list
.claude/scripts/alloc-ports.sh free test-pipeline
```

If those work, try a tiny `/build`:

```
/build add-readme-section
```

When Claude Code prompts you, walk through the steps. The first run will reveal anything you missed in adaptation.

---

## Step 9 — Tune and iterate

After your first real `/build`:

- If the pipeline asked for something it didn't need, invoke `/ccm` to remove it.
- If a step was missing, invoke `/ccm` to add it.
- If an agent was confused about your stack, edit its "Context" block.

The pipeline is designed to evolve. `/ccm` is the meta-tool for that — not "add more docs," but **edit the source**.

---

## Common gotchas

1. **Worktree script can't find your tools.** Make sure your shell environment is loaded inside the script — `source ~/.zshrc`, use absolute paths, or pin tool versions in a script-local `PATH`.
2. **Port allocation false positives.** `lsof -i :PORT` checks aren't always reliable across IPv4/IPv6 — adjust the script if you see false positives on your OS.
3. **Gitter tries to merge with conflicts unresolved.** That's a gap in your gitter setup; the template handles it, but if you simplified, restore the conflict-detection block.
4. **Agents writing to permanent docs.** Only `mono-documenter` should write to `docs/agents/` or `{project}/docs/`. If another agent tries, that's a `/ccm` fix at the source agent.
5. **`.worktrees/.ports` corrupted.** Manually edit; the format is one whitespace-separated line per pipeline.

---

## Optional: domain-specific commands

The base set (`/build`, `/jc`, `/ccm`, `/dev`) is the technology-agnostic core. Some teams add narrower commands that own a single domain — a compliance advisor, a knowledge curator, a multi-agent debate, an auditor, a wave runner that batches `/build`s. The blueprint deliberately ships none of those: they leak domain assumptions, and what's right for one project is noise in another.

If you want one, model it on `/build` or `/jc` (whichever has the right shape) and put its docs under `docs/commands/{your-cmd}/`. Mention it in your root `CLAUDE.md`'s commands table.
