---
name: qa-{project}
description: QA gate for {project} — spawned pre-merge (GATE-1) and post-merge (GATE-2) by /wave:builder and the /wave:orchestrator wave; full protocol lives in {project}/.claude/agents/qa.md
model: opus
tools: Read, Write, Edit, Bash, Glob, Grep, Agent
hooks:
  PostToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "$CLAUDE_PROJECT_DIR/.claude/scripts/filter-test-output.sh"
---

You are the {PROJECT_ROLE} QA engineer. Read and follow `{project}/.claude/agents/qa.md` — it is your complete protocol.

The spawn prompt carries Mode (PRE-MERGE | POST-MERGE), Pipeline, worktree path, ports, and doc-path variables; follow the Common spawn contract it references.

<!--
ROSTER PATTERN — the QA variant of `role-wrapper.md`, expressed ONCE, expanded by
SETUP into one `qa-{project}` wrapper per roster entry that has a QA protocol.
Never ship a file per concrete project; the pattern IS the template. Read
`role-wrapper.md` for why wrappers exist at all (registration is the whole job)
and for the naming law — everything there applies here unchanged.

WHAT THIS VARIANT ADDS. The PostToolUse hook on `Bash`: a subagent never inherits
the `settings.json` hook, so without this block a QA run's raw test output floods
the transcript. The hook is the reason a gate role needs its own wrapper shape.

KEEP IT THIN. Two body lines — point at the child protocol, name the spawn
contract. A wrapper that grows a third paragraph has started duplicating the
protocol it points at. Exception, the one shape variant: when the child protocol
is NOT readable from where the agent runs (a contractor-visible sub-repo), inline
the protocol in the wrapper body and end the description with "the full protocol
is this file's body" instead of the pointer.
-->

