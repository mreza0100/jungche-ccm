#!/usr/bin/env bash
set -euo pipefail

# chat-read.sh — resolve a pasted excerpt to its source session (via the
# shared chat-find.sh) and extract that session's full visible chat for
# /chat:read.
#
# Usage: chat-read.sh <excerpt-file>
#   <excerpt-file>  file holding the founder's pasted chunk of the target chat
#
# Output: tmp/chat-loads/{session-id}.md + the chat-find.sh match report (stderr).

if [[ $# -lt 1 || ! -f "${1:-}" ]]; then
  echo "usage: $0 <excerpt-file>" >&2
  exit 1
fi
excerpt="$1"
script_dir="$(cd "$(dirname "$0")" && pwd)"

# chat-find.sh prints "<session-id>\t<path>" on stdout, its report on stderr,
# and exits non-zero on no match — set -e then surfaces that report to the user.
match="$("$script_dir/chat-find.sh" "$excerpt")"
session_id="${match%%$'\t'*}"
best_file="${match#*$'\t'}"

ts_all="$(jq -r 'select(.timestamp != null) | .timestamp' "$best_file")"
first_ts="${ts_all%%$'\n'*}"
last_ts="${ts_all##*$'\n'}"

mkdir -p tmp/chat-loads
out="tmp/chat-loads/$session_id.md"
{
  echo "# Loaded chat — session $session_id"
  echo ""
  echo "Source: $best_file"
  echo "Range: $first_ts -> $last_ts"
  echo "Visible chat text only — thinking and tool outputs are not recorded here."
  echo ""
  jq -rf "$script_dir/transcript-extract.jq" "$best_file"
} > "$out"

echo "Extracted -> $out ($(wc -l < "$out" | tr -d ' ') lines)"
