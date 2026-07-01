#!/usr/bin/env bash
set -euo pipefail

# PreToolUse(Edit|Write) — guards /pcm territory.
# Edits to .claude/** and any CLAUDE.md (root or child) are /pcm-exclusive: these
# files ARE the framework — agent graph, routing, hooks, conventions — loaded by
# Claude Code at runtime, so a careless edit breaks the pipeline. Allowed only
# when /pcm is active (marker set right before /pcm's edit pass, cleared by the
# Stop hook or bounded by the TTL below). Silent no-op for every other path.
# Knowledge / sensitive files belong to km-guard.sh (its configurable
# PROTECTED_PATHS), not here.

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
[[ -z "$FILE_PATH" ]] && exit 0

# Anchor to the EDITED FILE's repo root, not the hook's cwd — correct inside
# worktrees and regardless of where Claude Code spawns the hook.
REPO_ROOT=$(git -C "$(dirname "$FILE_PATH")" rev-parse --show-toplevel 2>/dev/null) || exit 0
REL_PATH="${FILE_PATH#"$REPO_ROOT"/}"
MARKER="$REPO_ROOT/tmp/professor_pcm_active"

case "$REL_PATH" in
  CLAUDE.md|.claude/*) ;;              # root infrastructure
  */CLAUDE.md|*/.claude/*) ;;         # child-project infrastructure (any nesting)
  *) exit 0 ;;
esac

# Gate open if the marker is fresh. Short TTL: the Stop hook is the primary close;
# this only bounds a missed Stop (e.g. a sub-agent opened the gate). /pcm stamps
# right before its edit pass, so the tight window never slams shut mid-work.
if [[ -f "$MARKER" ]]; then
  age=$(( $(date +%s) - $(cat "$MARKER" 2>/dev/null || echo 0) ))
  (( age >= 0 && age < 600 )) && exit 0
fi

cat <<'JSON'
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"This file is framework INFRASTRUCTURE — a .claude/ prompt or a CLAUDE.md (root or child) that Claude Code loads at runtime, so a careless edit ships straight into the agent pipeline (routing, hooks, conventions) and can break every build. You ARE authorized to edit it as the infra owner: the /pcm flow. To proceed: (1) run /pcm and apply prompt discipline via /quality:prompt (read .claude/commands/quality/prompt.md — cut test, thresholds, one canonical term, positive framing, frontmatter discipline); (2) open the gate from the repo root: date +%s > tmp/professor_pcm_active ; (3) retry the edit. Do NOT route around this by disabling the hook or editing infra outside /pcm — that hides the change from the change protocol and the consistency sweep."}}
JSON
exit 2
