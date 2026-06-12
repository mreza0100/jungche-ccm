#!/usr/bin/env bash
set -euo pipefail

# save-transcript.sh — append this session's verbatim chat to a /save file.
# Dumps visible chat text only (founder messages + assistant replies + tool-name
# trail). Thinking blocks, tool outputs, and harness records are excluded.
#
# Usage: save-transcript.sh <target-file> [transcript-jsonl]
#   <target-file>      file to append the transcript to (created if missing)
#   [transcript-jsonl] explicit transcript path; default resolves from
#                      $CLAUDE_CONFIG_DIR + cwd + $CLAUDE_CODE_SESSION_ID

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <target-file> [transcript-jsonl]" >&2
  exit 1
fi

target="$1"
script_dir="$(cd "$(dirname "$0")" && pwd)"

if [[ $# -ge 2 ]]; then
  transcript="$2"
else
  config_dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
  session_id="${CLAUDE_CODE_SESSION_ID:-}"
  if [[ -z "$session_id" ]]; then
    echo "ERROR: CLAUDE_CODE_SESSION_ID not set and no transcript path given" >&2
    exit 1
  fi
  munged_cwd="$(pwd | tr '/.' '--')"
  transcript="$config_dir/projects/$munged_cwd/$session_id.jsonl"
fi

if [[ ! -f "$transcript" ]]; then
  echo "ERROR: transcript not found: $transcript" >&2
  exit 1
fi

mkdir -p "$(dirname "$target")"
touch "$target"

{
  echo ""
  echo "---"
  echo ""
  echo "# FULL TRANSCRIPT (script-dumped, verbatim)"
  echo ""
  echo "Visible chat text only — thinking and tool outputs are not recorded here."
  echo "Source: $transcript"
  echo ""

  jq -rf "$script_dir/transcript-extract.jq" "$transcript"

  echo "---"
  echo ""
  echo "# ENVIRONMENT SNAPSHOT (script-dumped)"
  echo ""
  if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "Branch: $(git branch --show-current)"
    echo ""
    echo '```'
    git status --short
    echo '```'
    echo ""
    echo "Worktrees:"
    echo '```'
    git worktree list
    echo '```'
  else
    echo "(not a git repository)"
  fi
} >> "$target"

user_count=$(jq -r 'select(.type == "user" and .isSidechain != true) | 1' "$transcript" | wc -l | tr -d ' ')
total_lines=$(wc -l < "$target" | tr -d ' ')
echo "Appended transcript ($user_count user records scanned) + env snapshot -> $target ($total_lines lines total)"
