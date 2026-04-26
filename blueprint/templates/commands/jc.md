# JC — Live Debug, Diagnose & Fix

Hotfix mode. The ONE command allowed to edit code on `main` directly.

Use for: targeted bug fixes, regressions, missing logs, small correctness fixes.
Do NOT use for: new features, architectural changes, multi-project refactors. Those go through `/build`.

Argument: $ARGUMENTS — short description of the bug.

---

## Step 0 — Lock and prepare

### 0a. Acquire merge lock

Determine which projects are likely affected. Acquire merge locks via gitter:

```
Agent(gitter): "Phase: LOCK. Projects: {projects-list}."
```

If gitter reports the lock is held: STOP. Tell the user "another pipeline is touching this project — wait or override."

### 0b. Resolve paths

- `$JC_NAME` = short kebab-case name from `$ARGUMENTS` (e.g., `audio-upload-timeout-fix`)
- `$JC_DOCS` = `docs/dev/tasks/jc-{name}/` — temp working dir (archived after)
- Verify name uniqueness against `docs/dev/tasks/`, `docs/dev/tasks/archive/`, `.worktrees/`.

```bash
mkdir -p $JC_DOCS
```

---

## Step 1 — Locate

Use Grep / Read / Glob to find the relevant code. If you need help understanding the system layout, read `docs/agents/map.md` and `docs/agents/architecture.md`.

Write `$JC_DOCS/1-locate.md`:

```markdown
# Locate — {name}

## Symptom
What the user reported.

## Suspected cause
Hypothesis with file:line evidence.

## Blast radius
What other code/data is affected by this bug or its fix.
```

---

## Step 2 — Diagnose

Read the suspected code. Trace data flow. Check logs / error reports. Confirm the root cause.

> **Hidden-failure mode (load `debug` skill):** if symptoms suggest a hang, deadlock, or mystery failure rather than a visible error — process at 0% CPU but not exited, test never returns, intermittent flake, passes alone but fails in suite, silent crash with no traceback — load the optional `debug` skill via the Skill tool BEFORE guessing. Visible-failure-mode steps below assume there's a stack trace to read; hidden failures need a different playbook (confirm hang → add timeout → isolate → trace prints → query layer below).

Write `$JC_DOCS/2-diagnose.md`:

```markdown
# Diagnose — {name}

## Root cause
The actual bug, not the symptom.

## Why it happened
Missing test? Wrong assumption? Race? Bad library default? Be honest.

## Fix sketch
What needs to change. Files and line ranges. NOT code yet.
```

---

## Step 3 — Fix

Edit the code on `main` directly. Keep the change tight: only what's needed for the fix. No "while I'm here" cleanup.

If the fix touches more than 3 files OR more than ~50 lines of meaningful change, STOP and switch to `/build`. JC is for surgical fixes.

After editing, run:
- The relevant test (existing test that should now pass, or new regression test)
- Lint + typecheck
- Build

If anything fails: fix it. If you can't fix it in `/jc`'s scope: revert and switch to `/build`.

---

## Step 4 — Test

Add a regression test. **No fix ships without a test that proves it stays fixed.**

For each affected project, run the full test suite once. Zero tolerance for "pre-existing failures" — your fix doesn't ship if it leaves main with broken tests.

Write `$JC_DOCS/3-fix.md`:

```markdown
# Fix — {name}

## Files changed
- file:line — what changed

## Regression test
- file — what it tests

## Test results
- Project A: N tests pass
- Project B: N tests pass
```

---

## Step 5 — gitter JC-COMMIT

```
Agent(gitter): "Phase: JC-COMMIT. Name: $JC_NAME. Files: {list}. Message: '{one-line summary}'. Projects: {affected}."
```

Gitter commits, releases the locks acquired in Step 0a.

---

## Step 6 — JC-UPDATE (documentation)

If the fix changes documented behavior, contracts, or non-obvious assumptions, invoke documenter:

```
Agent(mono-documenter): "Phase: JC-UPDATE. Hotfix: $JC_NAME. Docs: $JC_DOCS. Files changed: {list}."
```

Documenter updates the relevant permanent docs and archives `$JC_DOCS`. Then gitter DOCS-COMMIT.

If the fix is purely internal (no doc-relevant change), skip documenter and just archive:

```bash
mv $JC_DOCS docs/dev/tasks/archive/jc-$JC_NAME/
```

---

## Final report

```
Hotfix {JC_NAME} shipped.
- Commit: {sha}
- Files: {list}
- Regression test: {file}
- Locks released: {projects}
```
