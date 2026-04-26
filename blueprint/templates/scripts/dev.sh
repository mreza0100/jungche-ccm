#!/usr/bin/env bash
# dev.sh — Manage the local development environment
#
# Usage:
#   ./.claude/scripts/dev.sh start          → boot infrastructure + dev servers
#   ./.claude/scripts/dev.sh kill | stop    → kill all dev servers + infra
#   ./.claude/scripts/dev.sh restart        → kill, then start
#   ./.claude/scripts/dev.sh status         → show running processes + ports
#   ./.claude/scripts/dev.sh log [project]  → tail logs
#
# === EDIT FOR YOUR STACK ===
# Replace the per-project start/stop blocks with your actual commands.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
LOG_DIR="${ROOT}/tmp/dev-logs"
PID_DIR="${ROOT}/tmp/dev-pids"

mkdir -p "$LOG_DIR" "$PID_DIR"

# === Per-project commands — EDIT FOR YOUR STACK ===

start_infrastructure() {
  # Replace this with whatever brings your local infra up
  # (databases, queues, container stacks, mock services, etc.)
  # Example shape:
  #   (cd "${ROOT}/{your-infra-dir}" && {your up command})
  echo "Starting infrastructure..."
}

stop_infrastructure() {
  # Replace with the corresponding shutdown command.
  # Example shape:
  #   (cd "${ROOT}/{your-infra-dir}" && {your down command})
  echo "Stopping infrastructure..."
}

start_project() {
  local name="$1"
  local dir="$2"
  local cmd="$3"
  if [ -d "${ROOT}/${dir}" ]; then
    echo "Starting $name..."
    (cd "${ROOT}/${dir}" && nohup $cmd >"${LOG_DIR}/${name}.log" 2>&1 &)
    echo $! > "${PID_DIR}/${name}.pid"
  fi
}

stop_project() {
  local name="$1"
  local pidfile="${PID_DIR}/${name}.pid"
  if [ -f "$pidfile" ]; then
    local pid
    pid=$(cat "$pidfile")
    if kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      echo "Killed $name (pid $pid)"
    fi
    rm -f "$pidfile"
  fi
}

cmd_start() {
  start_infrastructure

  # One line per subproject. Args: name, directory, dev command.
  # start_project "{name}" "{dir}" "{your dev command}"
  # start_project "{name}" "{dir}" "{your dev command}"
  # start_project "{name}" "{dir}" "{your dev command}"

  sleep 2
  cmd_status
}

cmd_stop() {
  for pidfile in "${PID_DIR}"/*.pid; do
    [ -f "$pidfile" ] || continue
    local name
    name="$(basename "$pidfile" .pid)"
    stop_project "$name"
  done
  stop_infrastructure
  echo "Dev environment stopped."
}

cmd_restart() {
  cmd_stop
  cmd_start
}

cmd_status() {
  echo "=== Dev environment status ==="
  for pidfile in "${PID_DIR}"/*.pid; do
    [ -f "$pidfile" ] || continue
    local name pid
    name="$(basename "$pidfile" .pid)"
    pid="$(cat "$pidfile")"
    if kill -0 "$pid" 2>/dev/null; then
      echo "  $name: running (pid $pid)"
    else
      echo "  $name: STALE pid file (process gone)"
    fi
  done
  if [ -z "$(ls -A "$PID_DIR" 2>/dev/null)" ]; then
    echo "  No dev servers running."
  fi
}

cmd_log() {
  local project="${1:-}"
  if [ -n "$project" ]; then
    local logfile="${LOG_DIR}/${project}.log"
    if [ -f "$logfile" ]; then
      tail -f "$logfile"
    else
      echo "No log for $project at $logfile"
      exit 1
    fi
  else
    tail -f "${LOG_DIR}"/*.log
  fi
}

case "${1:-start}" in
  start)         cmd_start ;;
  kill | stop)   cmd_stop ;;
  restart)       cmd_restart ;;
  status)        cmd_status ;;
  log)           cmd_log "${2:-}" ;;
  *)
    echo "Usage: $0 {start|kill|stop|restart|status|log [project]}"
    exit 1
    ;;
esac
