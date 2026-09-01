---
name: {ROLE}-{project}
description: {One line naming what this role builds in {project} — its {PROJECT_STACK} surfaces}. Follows the project CLAUDE.md conventions and runs self-QA before finishing; works in a worktree with allocated ports. Spawn as the {role-noun} for any {project} task span; full protocol lives in {project}/.claude/agents/{ROLE}.md
model: {MODEL_TIER}
tools: Read, Write, Edit, Bash, Glob, Grep
---

You are the {PROJECT_ROLE} {role-noun}. Read and follow `{project}/.claude/agents/{ROLE}.md` — it is your complete protocol.

The spawn prompt carries the task section, worktree path, ports, and the explicit doc paths; follow the dispatching command's Common spawn contract and the HARD BANS it references.

<!--
ROSTER PATTERN — expressed ONCE, expanded by SETUP per roster entry × role.
Never ship a file per concrete {ROLE}-{project} pair; the pattern IS the template.

WHY the wrapper exists. A child agent file that is not registered at root is
spawnable only as `general-purpose` reading a path, so its frontmatter — model
tier, tool allowlist, hooks — never loads. The root wrapper is what makes the
child's declared tier and tool list real. Registration is the whole job.

WHAT SETUP expands. For each roster entry, one wrapper per role that entry
actually has (`ls {project}/.claude/agents/`). Naming is `{ROLE}-{project}` on
every wrapper, no exceptions — one convention, mechanically greppable.
`{MODEL_TIER}` follows the child protocol's own tier (a design/review role runs
a frontier tier, a build hand a mid tier). Tools follow the child's needs — a
design role that writes no code drops `Bash`, adds `Skill`.

VARIANTS the pattern carries:

- Gate roles (QA) use `templates/agents/qa-wrapper.md`, which adds the
  test-output filter hook. Everything else uses this file.
- A role with an ordering constraint states it in the description
  ("Spawn BEFORE the developer", "Spawn AFTER the designer, when the task
  carries a visual spec") — the router reads descriptions, so the constraint
  belongs there, not only in the body.
- A role owning hook-gated or generated files adds ONE body line naming them
  and where they route instead ("artifacts under `artifacts/**` and every
  vendored copy are generated — regenerate them with the emit and vendor
  scripts, never hand-edit them").
- A child repo visible to people outside the team (a contractor-facing
  sub-repo) gets its protocol INLINED in the wrapper rather than pointed at,
  because the pointer target is not readable from where the agent runs.

KEEP IT THIN. Everything above the pattern's own variants belongs in the child
protocol file, not in the wrapper. A wrapper that grows a second paragraph of
procedure has started duplicating the protocol it points at.
-->
