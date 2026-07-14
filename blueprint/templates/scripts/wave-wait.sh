#!/usr/bin/env bash
set -euo pipefail

# wave-wait.sh — the orchestrator's blocking waiter (orchestrator.md § O4 Liveness).
# Run as the ORCHESTRATOR chat's OWN background Bash task (run_in_background):
# blocks until an event lands in the spool, then prints it and EXITS — the
# harness task-notification of that exit wakes the orchestrator through the
# conversation itself (guaranteed; a send-keys inject dies silently against a
# busy pane or an open menu). No event within max-wait → prints HEARTBEAT-TIMEOUT
# and exits: the unconditional 10m wake, on which the orchestrator checks every
# lane itself. Reads state, never rules — the orchestrator sweeps and decides.
#
# Usage: wave-wait.sh [max-wait-seconds]   (default 600 = 10m)
# Spool:  tmp/wave-sensor/events.log    (builder pings + watcher alarms append)
# Cursor: tmp/wave-sensor/events.cursor (byte offset of last delivered event)

MAX_WAIT="${1:-600}"
# ROOT is the MAIN worktree, ALWAYS — never the checkout this script happens to live in.
# Every worktree is a full checkout carrying its OWN copy of this script, so resolving ROOT
# from BASH_SOURCE points a worktree-launched waiter at that worktree's empty spool: it
# watches the wrong file, delivers nothing, and reports perfectly healthy while the real
# builder pings pile up unconsumed in MAIN. git-common-dir returns MAIN's .git from inside
# any worktree, so the spool cannot be mis-addressed by a stray CWD or a stray launch dir.
# No git → the substitution fails under `set -e` and the waiter dies LOUDLY, which beats a
# waiter silently watching nothing.
ROOT="$(dirname "$(git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --path-format=absolute --git-common-dir)")"
DIR="$ROOT/tmp/wave-sensor"
SPOOL="$DIR/events.log"
CURSOR="$DIR/events.cursor"
PIDF="$DIR/wait.pid"
mkdir -p "$DIR"
touch "$SPOOL"

# Singleton — a second waiter would race the cursor and double-deliver.
if [[ -f "$PIDF" ]] && kill -0 "$(cat "$PIDF" 2>/dev/null)" 2>/dev/null; then
  echo "WAITER-ALREADY-RUNNING pid=$(cat "$PIDF")"
  exit 0
fi
printf '%s' "$$" > "$PIDF"
trap 'rm -f "$PIDF"' EXIT TERM INT

# First launch starts at EOF — history is the orchestrator's to sweep, not to replay.
if [[ ! -f "$CURSOR" ]]; then stat -c %s "$SPOOL" > "$CURSOR"; fi

deadline=$(( $(date +%s) + MAX_WAIT ))
while :; do
  size="$(stat -c %s "$SPOOL")"
  cur="$(cat "$CURSOR" 2>/dev/null || echo 0)"
  [[ "$cur" =~ ^[0-9]+$ ]] || cur=0
  if (( cur > size )); then cur=0; fi # spool rotated/truncated — deliver from start
  if (( size > cur )); then
    tail -c +"$((cur + 1))" "$SPOOL"
    printf '%s' "$size" > "$CURSOR"
    exit 0
  fi
  now="$(date +%s)"
  if (( now >= deadline )); then
    echo "HEARTBEAT-TIMEOUT: no spool events in ${MAX_WAIT}s — check every lane (full-screen) + sweep \$TASKS, re-arm"
    exit 0
  fi
  sleep 10
done
