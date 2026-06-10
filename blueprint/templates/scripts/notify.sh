#!/bin/bash
set -euo pipefail

STAMP="/tmp/{CHARACTER_NAME_LOWER}_turn_start"

case "${1:-stop}" in
  start)
    # Called by PreToolUse — record when the turn started (first tool only)
    [ -f "$STAMP" ] || date +%s > "$STAMP"
    ;;
  stop)
    # Called by Stop — notify only if the turn took 30+ seconds
    ROOT=$(git rev-parse --show-toplevel 2>/dev/null || true)
    # Hook JSON arrives on stdin (session_id, cwd, ...). Tolerate it missing —
    # manual invocations have no stdin payload.
    HOOK_INPUT=$(cat 2>/dev/null || true)
    SESSION=$(printf '%s' "$HOOK_INPUT" | sed -n 's/.*"session_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | cut -c1-8)
    PROJECT=$(basename "${ROOT:-$PWD}")
    # Read-then-remove the turn stamp without a [ -f ] TOCTOU: the global and the
    # project Stop hooks share this one stamp, so it can vanish between test and
    # read. Tolerate a missing/empty stamp instead of erroring (set -e safe).
    started=$(cat "$STAMP" 2>/dev/null || true)
    rm -f "$STAMP"
    if [ -n "$started" ]; then
      elapsed=$(( $(date +%s) - started ))
      if (( elapsed >= 30 )); then
        # Title carries the project; body carries the session id so the chat is
        # findable via /resume (ids are listed there).
        osascript -e "display notification \"Done — your turn ☕${SESSION:+ · session $SESSION}\" with title \"{CHARACTER_NAME} 🎓 — $PROJECT\" sound name \"Glass\""
      fi
    fi
    ;;
esac
