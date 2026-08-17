#!/usr/bin/env bash
# history.sh — read a chat's conversation from its on-disk transcript, as deep as you like.
# Pane captures are bounded to the visible screen (~54 rows, no tmux scrollback — the TUI
# redraws in place); the .jsonl transcript is the unbounded record. Signatures carry the
# sid prefix this script resolves ("sid 3737775b" → 3737775b*.jsonl in any account pool).
#
# usage: history.sh <sid-prefix|jsonl-path> [messages=20] [project-slug=this repo's]
#
# The project slug is Claude Code's own naming for a project's transcript dir: the absolute
# working directory with every "/" turned into "-". Defaulting it to $PWD's slug means the
# lookup follows you from repo to repo instead of pinning one project's pool.
set -euo pipefail
sid="${1:?usage: history.sh <sid-prefix|jsonl-path> [messages] [project-slug]}"
n="${2:-20}"
slug="${3:-${PWD//\//-}}"

if [ -f "$sid" ]; then
  f="$sid"
else
  f=""
  pools=()
  if [ -n "${PFM_HISTORY_ROOTS_JSON:-}" ]; then
    jq -e 'type == "array" and all(.[]; type == "string")' \
      <<<"$PFM_HISTORY_ROOTS_JSON" >/dev/null
    while IFS= read -r -d '' pool; do
      pools+=("$pool")
    done < <(jq -j '.[] | ., "\u0000"' <<<"$PFM_HISTORY_ROOTS_JSON")
  else
    pools=("$HOME/.claude/projects" "$HOME"/.cc/*/projects)
  fi
  for pool in "${pools[@]}"; do
    m=$(ls -t "$pool/$slug/$sid"*.jsonl 2>/dev/null | head -1) || true
    [ -n "${m:-}" ] && { f="$m"; break; }
  done
  [ -n "$f" ] || { echo "no transcript matching sid '$sid' under $slug in any account pool" >&2; exit 1; }
fi

echo "== $f · last $n messages =="
# tail generously, drop the (possibly partial) first line, then keep the last N real messages.
tail -n 800 "$f" | sed 1d | jq -rj --argjson n "$n" -s '
  [ .[]
    | select(.type=="user" or .type=="assistant")
    | {t:(.timestamp // "?"), y:.type,
       x:(.message.content
          | if type=="string" then .
            elif type=="array" then ([.[] | select(.type=="text") | .text] | join("\n"))
            else "" end)}
    | select((.x|length) > 0)
    | select(.x | startswith("<system-reminder") | not)
    | select(.x | startswith("Caveat: The messages below") | not)
  ] | .[-$n:] | .[]
  | "\n───── \(.t) · \(.y) ─────\n\(.x)\n"'
