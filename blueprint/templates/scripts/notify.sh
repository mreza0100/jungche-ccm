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
    if [ -f "$STAMP" ]; then
      started=$(cat "$STAMP")
      elapsed=$(( $(date +%s) - started ))
      rm -f "$STAMP"
      if (( elapsed >= 30 )); then
        osascript -e 'display notification "{CHARACTER_NAME} is done — your turn" with title "{CHARACTER_NAME} 🎓" sound name "Glass"'
      fi
    fi
    ;;
esac
