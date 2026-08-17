#!/usr/bin/env bash
# alloc-ports.sh — Allocate unique ports for a worktree pipeline
#
# Usage:
#   ./.claude/scripts/alloc-ports.sh alloc <worktree-id>   → prints BE_PORT=N FE_PORT=M TEST_PG_PORT=P TEST_LS_PORT=Q PULSE_PORT=R WEB_PORT=S AI_PORT=T
#   ./.claude/scripts/alloc-ports.sh free  <worktree-id>   → releases the allocation
#   ./.claude/scripts/alloc-ports.sh list                   → shows all allocations
#
# Port ranges:
#   Backend:        3001–3099  (main uses {BACKEND_PORT})
#   Frontend:       5174–5272  (main uses 5173)
#   Test {DATABASE}: 5434–5532  (shared test uses {DB_PORT_TEST})
#   Test {QUEUE}: 4568–4666  (shared test uses {QUEUE_PORT_TEST})
#   Pulse (analytics):  3302–3400  (main uses 3300, shared test uses 3301)
#   {WEB_PROJECT}:  4001–4099  (main uses {WEB_PORT})
#   {AI_SERVICE_NAME} HTTP:    3501–3599  (main uses 3500)
#
# Registry: .worktrees/.ports (one line per allocation: id be_port fe_port test_pg_port test_ls_port pulse_port web_port ai_port)

set -euo pipefail

# ROOT resolves to the ONE canonical checkout — the shared common repo, never a worktree's
# own physical location — because ports are a machine-global resource: several pipeline
# lanes on one devbox all allocate from this SAME file, and each git worktree carries its
# own copy of this script. `git rev-parse --path-format=absolute --git-common-dir` returns
# the shared main checkout's .git dir from ANY worktree (same idiom the Makefile's
# PORTS_REGISTRY already uses); a physical
# dirname($0)-based resolution instead gives each worktree its OWN nested
# .worktrees/.ports — invisible to Make's canonical resolution and to every other worktree.
SCRIPT_CHECKOUT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GIT_COMMON_DIR="$(git -C "$SCRIPT_CHECKOUT_ROOT" rev-parse --path-format=absolute --git-common-dir 2>/dev/null || echo "$SCRIPT_CHECKOUT_ROOT/.git")"
ROOT="$(dirname "$GIT_COMMON_DIR")"
REGISTRY="${ROOT}/.worktrees/.ports"
LOCKFILE="${REGISTRY}.lock"

BE_BASE=3001
FE_BASE=5174
TEST_PG_BASE=5434
TEST_LS_BASE=4568
PULSE_BASE=3302
WEB_BASE=4001
AI_BASE=3501
MAX_SLOTS=99

mkdir -p "$(dirname "$REGISTRY")"
touch "$REGISTRY"

acquire_lock() {
  local tries=0
  while ! mkdir "$LOCKFILE" 2>/dev/null; do
    tries=$((tries + 1))
    if [ "$tries" -gt 50 ]; then
      echo "Error: could not acquire port lock after 5s" >&2
      exit 1
    fi
    sleep 0.1
  done
  trap 'rmdir "$LOCKFILE" 2>/dev/null' EXIT
}

cmd_alloc() {
  local id="$1"
  acquire_lock

  # Already allocated? (registry is always 8-col — this script is the sole writer)
  local existing
  existing=$(awk -v id="$id" '$1 == id { print $0 }' "$REGISTRY")
  if [ -n "$existing" ]; then
    local be_port fe_port test_pg_port test_ls_port pulse_port web_port ai_port
    be_port=$(echo "$existing" | awk '{print $2}')
    fe_port=$(echo "$existing" | awk '{print $3}')
    test_pg_port=$(echo "$existing" | awk '{print $4}')
    test_ls_port=$(echo "$existing" | awk '{print $5}')
    pulse_port=$(echo "$existing" | awk '{print $6}')
    web_port=$(echo "$existing" | awk '{print $7}')
    ai_port=$(echo "$existing" | awk '{print $8}')
    echo "BE_PORT=${be_port}"
    echo "FE_PORT=${fe_port}"
    echo "TEST_PG_PORT=${test_pg_port}"
    echo "TEST_LS_PORT=${test_ls_port}"
    echo "PULSE_PORT=${pulse_port}"
    echo "WEB_PORT=${web_port}"
    echo "AI_PORT=${ai_port}"
    return 0
  fi

  # An existing registry entry is an idempotent reservation, not a liveness probe.
  # A later host bind may still make its consumer fail, but reallocation would break callers.

  host_tcp_port_state() {
    local port="$1"
    local output status

    if command -v ss >/dev/null 2>&1; then
      if output=$(ss -H -ltn "sport = :${port}" 2>&1); then
        [ -n "$output" ] && return 1
        return 0
      fi
      echo "Error: could not inspect TCP listener occupancy for port ${port} with ss: ${output}" >&2
      return 2
    fi

    if command -v lsof >/dev/null 2>&1; then
      if output=$(lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>&1); then
        return 1
      else
        status=$?
      fi
      if [ "$status" -eq 1 ] && [ -z "$output" ]; then
        return 0
      fi
      echo "Error: could not inspect TCP listener occupancy for port ${port} with lsof: ${output}" >&2
      return 2
    fi

    echo "Error: cannot inspect TCP listener occupancy: neither ss nor lsof is available" >&2
    return 2
  }

  tuple_is_host_free() {
    local port state
    for port in "$@"; do
      if host_tcp_port_state "$port"; then
        continue
      else
        state=$?
      fi
      case "$state" in
        1) return 1 ;;
        2) return 2 ;;
        *)
          echo "Error: unexpected TCP listener inspection state for port ${port}: ${state}" >&2
          return 2
          ;;
      esac
    done
  }

  tuple_is_unreserved() {
    awk -v be="$1" -v fe="$2" -v test_pg="$3" -v test_ls="$4" \
      -v pulse="$5" -v web="$6" -v ai="$7" '
      $2 == be || $3 == fe || $4 == test_pg || $5 == test_ls ||
      $6 == pulse || $7 == web || $8 == ai { found = 1 }
      END { exit found }
    ' "$REGISTRY"
  }

  # Pin: an adversarial test starts an unregistered host listener and proves its tuple is
  # rejected. This remains a TOCTOU snapshot: the registry lock serializes allocators, not
  # later OS binds.
  local slot=0 inspection_state
  while [ "$slot" -lt "$MAX_SLOTS" ]; do
    local be_port=$((BE_BASE + slot))
    local fe_port=$((FE_BASE + slot))
    local test_pg_port=$((TEST_PG_BASE + slot))
    local test_ls_port=$((TEST_LS_BASE + slot))
    local pulse_port=$((PULSE_BASE + slot))
    local web_port=$((WEB_BASE + slot))
    local ai_port=$((AI_BASE + slot))
    if tuple_is_unreserved "$be_port" "$fe_port" "$test_pg_port" "$test_ls_port" "$pulse_port" "$web_port" "$ai_port"; then
      if tuple_is_host_free "$be_port" "$fe_port" "$test_pg_port" "$test_ls_port" "$pulse_port" "$web_port" "$ai_port"; then
        echo "${id} ${be_port} ${fe_port} ${test_pg_port} ${test_ls_port} ${pulse_port} ${web_port} ${ai_port}" >> "$REGISTRY"
        echo "BE_PORT=${be_port}"
        echo "FE_PORT=${fe_port}"
        echo "TEST_PG_PORT=${test_pg_port}"
        echo "TEST_LS_PORT=${test_ls_port}"
        echo "PULSE_PORT=${pulse_port}"
        echo "WEB_PORT=${web_port}"
        echo "AI_PORT=${ai_port}"
        return 0
      else
        inspection_state=$?
      fi
      if [ "$inspection_state" -eq 2 ]; then
        exit 1
      fi
    fi
    slot=$((slot + 1))
  done

  echo "Error: no free port slots (all $MAX_SLOTS in use)" >&2
  exit 1
}

cmd_free() {
  local id="$1"
  acquire_lock

  # Use grep -v instead of sed to avoid delimiter issues with / in ids
  grep -v "^${id} " "$REGISTRY" > "${REGISTRY}.tmp" 2>/dev/null || true
  mv "${REGISTRY}.tmp" "$REGISTRY"
  echo "Freed ports for: $id"
}

cmd_list() {
  if [ ! -s "$REGISTRY" ]; then
    echo "No port allocations."
    return 0
  fi
  printf "%-40s %-10s %-10s %-10s %-10s %-10s %-10s %-12s\n" "WORKTREE" "BE_PORT" "FE_PORT" "TEST_PG" "TEST_LS" "PULSE" "WEB" "AI_HTTP"
  while IFS= read -r line; do
    local id be fe tpg tls pulse web ai
    id=$(echo "$line" | awk '{print $1}')
    be=$(echo "$line" | awk '{print $2}')
    fe=$(echo "$line" | awk '{print $3}')
    tpg=$(echo "$line" | awk '{print $4}')
    tls=$(echo "$line" | awk '{print $5}')
    pulse=$(echo "$line" | awk '{print $6}')
    web=$(echo "$line" | awk '{print $7}')
    ai=$(echo "$line" | awk '{print $8}')
    printf "%-40s %-10s %-10s %-10s %-10s %-10s %-10s %-12s\n" "$id" "$be" "$fe" "$tpg" "$tls" "$pulse" "$web" "$ai"
  done < "$REGISTRY"
}

case "${1:-help}" in
  alloc) cmd_alloc "${2:?worktree-id required}" ;;
  free)  cmd_free  "${2:?worktree-id required}" ;;
  list)  cmd_list ;;
  *)
    echo "Usage: $0 {alloc|free|list} [worktree-id]"
    exit 1
    ;;
esac
