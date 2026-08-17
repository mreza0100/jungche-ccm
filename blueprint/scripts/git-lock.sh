#!/usr/bin/env bash
# git-lock.sh — advisory lock guarding the trunk against two gitter operations
# racing a merge/push. macOS-safe (mkdir is atomic; no flock binary on darwin).
# The lock is held by the directory existing between `acquire` and `release`,
# across the independent Bash invocations a single gitter agent runs. A holder
# older than STALE_AFTER is presumed crashed and reclaimed.
set -euo pipefail

LOCKDIR=".worktrees/.git-merge.lock"
INFOFILE="$LOCKDIR/holder"
STALE_AFTER=1800 # 30 min — longer than any real merge; backstop for a crashed holder

cmd="${1:-}"
holder="${2:-unknown}"

acquire() {
  mkdir -p "$(dirname "$LOCKDIR")"
  local waited=0
  while ! mkdir "$LOCKDIR" 2>/dev/null; do
    local age
    age=$(( $(date +%s) - $(stat -c %Y "$LOCKDIR" 2>/dev/null || stat -f %m "$LOCKDIR" 2>/dev/null || date +%s) ))
    if [ "$age" -gt "$STALE_AFTER" ]; then
      echo "git-lock: clearing stale lock (age ${age}s, was '$(cat "$INFOFILE" 2>/dev/null || echo unknown)')" >&2
      rm -rf "$LOCKDIR"
      continue
    fi
    waited=$((waited + 1))
    if [ "$waited" -ge 60 ]; then
      echo "git-lock: busy — held by '$(cat "$INFOFILE" 2>/dev/null || echo unknown)' (age ${age}s). Retry shortly." >&2
      exit 1
    fi
    sleep 1
  done
  echo "$holder @ $(date -Iseconds)" >"$INFOFILE"
  echo "git-lock: acquired by $holder"
}

release() {
  rm -rf "$LOCKDIR"
  echo "git-lock: released"
}

case "$cmd" in
acquire) acquire ;;
release) release ;;
*)
  echo "usage: git-lock.sh {acquire|release} [holder-label]" >&2
  exit 2
  ;;
esac
