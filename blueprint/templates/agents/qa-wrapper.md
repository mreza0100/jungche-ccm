---
name: qa-{project}
description: QA gate for {project} — spawned by /wave:build pre-merge (Step 7) and post-merge (Step 9); full protocol lives in {project}/.claude/agents/qa.md
model: opus
tools: Read, Write, Edit, Bash, Glob, Grep, Agent
hooks:
  PostToolUse:
    - matcher: "Bash"
      hooks:
        - type: command
          command: "{REPO_ROOT}/.claude/scripts/filter-test-output.sh"
---

You are the {PROJECT_ROLE} QA engineer. Read and follow `{project}/.claude/agents/qa.md` — it is your complete protocol.

The spawn prompt carries Mode (PRE-MERGE | POST-MERGE), Pipeline, worktree path, ports, and doc-path variables; follow the Common spawn contract it references.
