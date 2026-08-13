#!/usr/bin/env bash
# Codex SubagentStart injection: lane-scoped, plain map surface. The role name IS
# the lane slug, so a role without a generated surface receives nothing.
# Wire contract (proven 2026-08-12 on codex-cli 0.147.0): stdin = event JSON
# carrying .cwd/.agent_type; stdout = Claude-shaped envelope
# {"hookSpecificOutput":{"hookEventName":"SubagentStart","additionalContext":...}}.
# Isolation: the cwd repo must carry an organ with a generated agents/{lane}.md
# for that role — otherwise exit silent. Drift annotations are intentionally NOT
# duplicated here; they live in dreamer-agent-inject.sh.
set -euo pipefail

in=$(cat)
agent_type=$(jq -r '.agent_type // empty' <<<"$in" 2>/dev/null || true)
cwd=$(jq -r '.cwd // empty' <<<"$in" 2>/dev/null || true)
[ -n "$agent_type" ] && [ -n "$cwd" ] || exit 0
lane=$(tr '[:upper:]' '[:lower:]' <<<"$agent_type" | tr -c 'a-z0-9-' '-')
lane=${lane%-}
[[ "$lane" =~ ^[a-z0-9][a-z0-9-]*$ ]] || exit 0
repo_root="${cwd%%/.worktrees/*}"
organ="$repo_root/.professor/stm"
index="$organ/agents/$lane.md"
[ -s "$index" ] || exit 0
surface=$(grep -E '^- [^[:space:]].* -> maps/[a-z0-9][a-z0-9-]*\.md$' "$index" || true)
[ -n "$surface" ] || exit 0
[ "$surface" = "$(cat "$index")" ] || exit 1

header="Cached maps for this repository (bodies under $organ/maps/). Consult a covering map before re-deriving its subject and cite it when used:"
jq -cn --arg ctx "$header"$'\n'"$surface" \
  '{hookSpecificOutput:{hookEventName:"SubagentStart",additionalContext:$ctx}}'
