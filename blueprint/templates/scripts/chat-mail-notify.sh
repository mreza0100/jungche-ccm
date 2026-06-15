#!/usr/bin/env bash
set -euo pipefail

# chat-mail-notify.sh — SessionStart hook entry. Surfaces this chat's unread
# chat-mail (its by-session inbox) as a one-line nudge. Read-only; never blocks
# session start. Safe to run on every session across all projects and accounts.
#
# Resolves the session id from the hook's stdin JSON (`session_id`), falling
# back to $CLAUDE_CODE_SESSION_ID — a SessionStart hook may not export the env
# var, but always delivers the id on stdin.

payload=""
[ -t 0 ] || payload="$(cat 2>/dev/null || true)"
sid="${CLAUDE_CODE_SESSION_ID:-}"
if [[ -z "$sid" && -n "$payload" ]]; then
  sid="$(printf '%s' "$payload" | jq -r '.session_id // empty' 2>/dev/null || true)"
fi

[[ -n "$sid" ]] || exit 0

export CLAUDE_CODE_SESSION_ID="$sid"
script_dir="$(cd "$(dirname "$0")" && pwd)"
log="${CHAT_MAILBOX_DIR:-$HOME/.claude-sessions/.mailbox}/.notify.err"
mkdir -p "$(dirname "$log")" 2>/dev/null || true
"$script_dir/chat-mail.sh" register 2>>"$log" || true
"$script_dir/chat-mail.sh" notify 2>>"$log" || true
exit 0
