#!/usr/bin/env bash
set -euo pipefail

# chat-load.sh — locate the past session whose transcript contains a pasted
# excerpt, then extract its full visible chat for /chat:load to read.
#
# Usage: chat-load.sh <excerpt-file> [registry-dir]
#   <excerpt-file>  file holding the founder's pasted chunk of the old chat
#   [registry-dir]  transcript dir; default resolves from $CLAUDE_CONFIG_DIR + cwd
#
# Output: tmp/chat-loads/{session-id}.md + a match report on stdout.

if [[ $# -lt 1 || ! -f "${1:-}" ]]; then
  echo "usage: $0 <excerpt-file> [registry-dir]" >&2
  exit 1
fi
excerpt="$1"
script_dir="$(cd "$(dirname "$0")" && pwd)"

config_dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
registry="${2:-$config_dir/projects/$(pwd | tr '/.' '--')}"
if [[ ! -d "$registry" ]]; then
  echo "ERROR: registry dir not found: $registry" >&2
  exit 1
fi

current_session="${CLAUDE_CODE_SESSION_ID:-__none__}"

# Needles: the 5 longest excerpt lines (>=20 chars), JSON-escaped so they match
# how the text is stored inside the JSONL.
needles_file="$(mktemp)"
matches_file="$(mktemp)"
trap 'rm -f "$needles_file" "$matches_file"' EXIT

awk '{
  sub(/\r$/, ""); sub(/^[ \t>#*-]+/, ""); sub(/[ \t]+$/, "");
  if (length($0) >= 20) print length($0) "\t" $0
}' "$excerpt" | sort -rn | head -5 | cut -f2- > "$needles_file"

if [[ ! -s "$needles_file" ]]; then
  echo "ERROR: excerpt contains no line of 20+ characters to search for" >&2
  exit 1
fi

while IFS= read -r line; do
  needle="$(printf '%s' "$line" | jq -Rr @json)"
  needle="${needle#\"}"
  needle="${needle%\"}"
  grep -lF -- "$needle" "$registry"/*.jsonl 2>/dev/null || true
done < "$needles_file" | grep -v "/$current_session\.jsonl$" | sort | uniq -c | sort -rn > "$matches_file"

if [[ ! -s "$matches_file" ]]; then
  echo "NO MATCH: no past session in $registry contains the excerpt." >&2
  echo "Try a longer or more distinctive chunk (exact lines from the old chat)." >&2
  exit 2
fi

best_file="$(awk 'NR==1 { $1=""; sub(/^ /,""); print }' "$matches_file")"
best_hits="$(awk 'NR==1 { print $1 }' "$matches_file")"
session_id="$(basename "$best_file" .jsonl)"

first_ts="$(jq -r 'select(.timestamp != null) | .timestamp' "$best_file" | head -1)"
last_ts="$(jq -r 'select(.timestamp != null) | .timestamp' "$best_file" | tail -1)"

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

echo "Matched session: $session_id ($best_hits/$(wc -l < "$needles_file" | tr -d ' ') needles hit)"
echo "Range: $first_ts -> $last_ts"
if [[ "$(wc -l < "$matches_file" | tr -d ' ')" -gt 1 ]]; then
  echo "Other candidates (hits, file):"
  tail -n +2 "$matches_file" | head -4
fi
echo "Extracted -> $out ($(wc -l < "$out" | tr -d ' ') lines)"
