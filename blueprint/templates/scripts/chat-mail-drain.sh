#!/usr/bin/env bash
set -euo pipefail

# chat-mail-drain.sh — UserPromptSubmit hook entry. Drains this chat's chat-mail
# inbox into context at the turn boundary (gt's queue-drain lever), then archives
# it. Its stdout is injected as additional context for the turn. Never blocks a
# turn: always exit 0. Resolves session_id from the hook's stdin JSON, falling
# back to $CLAUDE_CODE_SESSION_ID.

payload=""
[ -t 0 ] || payload="$(cat 2>/dev/null || true)"
sid="${CLAUDE_CODE_SESSION_ID:-}"
if [[ -z "$sid" && -n "$payload" ]]; then
  sid="$(printf '%s' "$payload" | jq -r '.session_id // empty' 2>/dev/null || true)"
fi
[[ -n "$sid" ]] || exit 0

export CLAUDE_CODE_SESSION_ID="$sid"
script_dir="$(cd "$(dirname "$0")" && pwd)"
log="${CHAT_MAILBOX_DIR:-$HOME/.claude-sessions/.mailbox}/.drain.err"
mkdir -p "$(dirname "$log")" 2>/dev/null || true
"$script_dir/chat-mail.sh" drain 2>>"$log" || true
exit 0
