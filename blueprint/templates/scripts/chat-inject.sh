#!/usr/bin/env bash
set -euo pipefail

# chat-inject.sh — force a turn into another chat, auto-picking how it lands:
#   * LIVE   — target is a live tmux pane  -> chat-nudge.sh types it in and
#              submits now (what you just saw); the chat answers immediately.
#   * RESUME — target is dormant / by file -> append it to the transcript; the
#              chat answers on its next resume. Backs up first, tail-append only.
#
# Target may be a tmux session name (live), a session-id (live via the
# self-registered registry, else its transcript), or a transcript path.
#
# Usage: chat-inject.sh <tmux-session|session-id|transcript-path> <message...>

MBOX="${CHAT_MAILBOX_DIR:-$HOME/.claude-sessions/.mailbox}"

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <tmux-session|session-id|transcript-path> <message...>" >&2
  exit 1
fi
target="$1"; shift
msg="$*"
[[ -n "$msg" ]] || { echo "ERROR: refusing to inject an empty message" >&2; exit 1; }
script_dir="$(cd "$(dirname "$0")" && pwd)"

# --- Resolve: a live tmux pane (preferred), else a transcript to append ---
live_tmux=""
transcript=""
if tmux has-session -t "$target" 2>/dev/null; then
  live_tmux="$target"                                   # a live tmux session name
elif [[ -f "$target" ]]; then
  transcript="$(cd "$(dirname "$target")" && pwd -P)/$(basename "$target")"
elif [[ "$target" =~ ^[A-Za-z0-9_-]+$ ]]; then
  # session-id: prefer a live pane from the registry, else fall back to its file.
  if [[ -f "$MBOX/registry/$target" ]]; then
    reg_tmux="$(grep '^tmux=' "$MBOX/registry/$target" | cut -d= -f2- || true)"
    if [[ -n "$reg_tmux" ]] && tmux has-session -t "$reg_tmux" 2>/dev/null; then
      live_tmux="$reg_tmux"
    fi
  fi
  if [[ -z "$live_tmux" ]]; then
    transcript="$(
      for p in "$HOME"/.claude/projects/*/"$target".jsonl "$HOME"/.claude[0-9]*/projects/*/"$target".jsonl; do
        [[ -f "$p" ]] && ( cd "$(dirname "$p")" && printf '%s\n' "$(pwd -P)/$(basename "$p")" )
      done | sort -u | head -1 || true
    )"
    [[ -n "$transcript" ]] || { echo "ERROR: no live tmux pane and no transcript for session-id '$target'" >&2; exit 1; }
  fi
else
  echo "ERROR: target is not a live tmux session, a transcript path, or a session-id" >&2
  exit 1
fi

# --- LIVE arm: delegate to chat-nudge.sh (tmux send-keys, auto-submit) ---
if [[ -n "$live_tmux" ]]; then
  "$script_dir/chat-nudge.sh" "$live_tmux" "$msg" >/dev/null
  echo "injected LIVE into tmux session '$live_tmux' — answered now (pane must be idle at its prompt)"
  exit 0
fi

# --- RESUME arm: append a user turn to the transcript ---
tail_event="$(jq -c 'select(.uuid != null)' "$transcript" | tail -1)"
[[ -n "$tail_event" ]] || { echo "ERROR: no uuid-bearing event in $transcript" >&2; exit 1; }
parent_uuid="$(printf '%s' "$tail_event" | jq -r '.uuid')"
session_id="$(printf '%s' "$tail_event" | jq -r '.sessionId // empty')"
new_uuid="$(uuidgen | tr 'A-Z' 'a-z')"
prompt_id="$(uuidgen | tr 'A-Z' 'a-z')"
ts="$(date -u +%Y-%m-%dT%H:%M:%S.000Z)"

backup_dir="$MBOX/inject-backups"
mkdir -p "$backup_dir"
backup="$backup_dir/${session_id:-unknown}-$(date -u +%Y%m%dT%H%M%SZ).jsonl"
cp -p "$transcript" "$backup"

new_event="$(printf '%s' "$tail_event" | jq -c \
  --arg uuid "$new_uuid" --arg parent "$parent_uuid" --arg pid "$prompt_id" \
  --arg ts "$ts" --arg msg "$msg" '{
    type: "user", userType: "external", entrypoint: "cli",
    cwd: .cwd, sessionId: .sessionId, version: .version, gitBranch: (.gitBranch // ""),
    parentUuid: $parent, uuid: $uuid, promptId: $pid, timestamp: $ts,
    isSidechain: false, isMeta: false,
    message: { role: "user", content: $msg }
  }')"
[[ -n "$new_event" ]] || { echo "ERROR: failed to build injected event" >&2; exit 1; }

printf '%s\n' "$new_event" >> "$transcript"
echo "injected user turn $new_uuid (parent $parent_uuid) into session ${session_id:-?} — answered on that chat's next RESUME (no live pane found)"
echo "backup: $backup"
