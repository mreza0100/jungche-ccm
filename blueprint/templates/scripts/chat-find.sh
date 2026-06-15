#!/usr/bin/env bash
set -euo pipefail

# chat-find.sh — resolve a pasted text excerpt to the past session that
# contains it, searching every account's transcript registry. Shared finder
# for the chat: command family: chat-read.sh extracts the match, chat:send
# messages it.
#
# Usage: chat-find.sh <excerpt-file>
#   <excerpt-file>  file holding a distinctive chunk of the target chat
#
# stdout (on match): "<session-id>\t<transcript-path>"  (single line, tab-separated)
# stderr: match report + other candidates (human-readable)
# exit:   0 match · 1 usage error · 2 no match

if [[ $# -lt 1 || ! -f "${1:-}" ]]; then
  echo "usage: $0 <excerpt-file>" >&2
  exit 1
fi
excerpt="$1"
current_session="${CLAUDE_CODE_SESSION_ID:-__none__}"

# Registries: projects/ under every Claude config dir, deduped by real path.
# The per-account config dirs symlink projects/ to one shared dir, so resolving
# and uniq'ing keeps each physical transcript counted once (not once per account).
registries=()
while IFS= read -r proj; do
  [[ -n "$proj" ]] && registries+=("$proj")
done < <(
  for cfg in "$HOME"/.claude "$HOME"/.claude[0-9]*; do
    [[ -d "$cfg/projects" ]] || continue
    for proj in "$cfg/projects"/*/; do
      [[ -d "$proj" ]] || continue
      ( cd "$proj" && pwd -P )
    done
  done | sort -u
)
if [[ ${#registries[@]} -eq 0 ]]; then
  echo "ERROR: no transcript registry found under \$HOME/.claude*/projects" >&2
  exit 2
fi

# Needles: the 5 longest excerpt lines (>=20 chars), JSON-escaped so they match
# how the text is stored inside the JSONL.
needles_file="$(mktemp)"
matches_file="$(mktemp)"
ranked_file="$(mktemp)"
trap 'rm -f "$needles_file" "$matches_file" "$ranked_file"' EXIT

# Rank to a file first, then slice with head — piping a streaming producer
# (sort) straight into head trips SIGPIPE under pipefail and aborts the script.
awk '{
  sub(/\r$/, ""); sub(/^[ \t>#*-]+/, ""); sub(/[ \t]+$/, "");
  if (length($0) >= 20) print length($0) "\t" $0
}' "$excerpt" | sort -rn > "$ranked_file"
head -5 "$ranked_file" | cut -f2- > "$needles_file"

if [[ ! -s "$needles_file" ]]; then
  echo "ERROR: excerpt contains no line of 20+ characters to search for" >&2
  exit 1
fi

while IFS= read -r line; do
  needle="$(printf '%s' "$line" | jq -Rr @json)"
  needle="${needle#\"}"
  needle="${needle%\"}"
  for reg in "${registries[@]}"; do
    grep -lF -- "$needle" "$reg"/*.jsonl 2>/dev/null || true
  done
done < "$needles_file" | grep -v "/$current_session\.jsonl$" | sort | uniq -c | sort -rn > "$matches_file" || true

if [[ ! -s "$matches_file" ]]; then
  echo "NO MATCH: no session under any account contains the excerpt." >&2
  echo "Try a longer or more distinctive chunk (exact lines from the target chat)." >&2
  exit 2
fi

best_file="$(awk 'NR==1 { $1=""; sub(/^ /,""); print }' "$matches_file")"
best_hits="$(awk 'NR==1 { print $1 }' "$matches_file")"
session_id="$(basename "$best_file" .jsonl)"
needle_count="$(wc -l < "$needles_file" | tr -d ' ')"

ts_all="$(jq -r 'select(.timestamp != null) | .timestamp' "$best_file")"
first_ts="${ts_all%%$'\n'*}"
last_ts="${ts_all##*$'\n'}"

{
  echo "Matched session: $session_id ($best_hits/$needle_count needles hit)"
  echo "Range: $first_ts -> $last_ts"
  if [[ "$(wc -l < "$matches_file" | tr -d ' ')" -gt 1 ]]; then
    echo "Other candidates (hits, file):"
    head -5 "$matches_file" | tail -n +2
  fi
} >&2

printf '%s\t%s\n' "$session_id" "$best_file"
