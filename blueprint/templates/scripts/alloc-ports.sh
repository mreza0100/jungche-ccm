#!/usr/bin/env bash
# alloc-ports.sh — Allocate unique ports for a worktree pipeline
#
# Usage:
#   ./.claude/scripts/alloc-ports.sh alloc <worktree-id>   → prints BE_PORT=N FE_PORT=M TEST_PG_PORT=P TEST_LS_PORT=Q PULSE_PORT=R WEB_PORT=S CORTEX_PORT=T
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
# Registry: .worktrees/.ports (one line per allocation: id be_port fe_port test_pg_port test_ls_port pulse_port web_port cortex_port)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REGISTRY="${ROOT}/.worktrees/.ports"
LOCKFILE="${REGISTRY}.lock"

BE_BASE=3001
FE_BASE=5174
TEST_PG_BASE=5434
TEST_LS_BASE=4568
PULSE_BASE=3302
WEB_BASE=4001
CORTEX_BASE=3501
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

  # Already allocated? Support old (3-col), mid (5-col), 6-col, 7-col, and current (8-col) formats
  local existing
  existing=$(awk -v id="$id" '$1 == id { print $0 }' "$REGISTRY")
  if [ -n "$existing" ]; then
    local cols
    cols=$(echo "$existing" | awk '{print NF}')
    local be_port fe_port test_pg_port test_ls_port pulse_port web_port cortex_port
    be_port=$(echo "$existing" | awk '{print $2}')
    fe_port=$(echo "$existing" | awk '{print $3}')
    local slot=$((be_port - BE_BASE))
    if [ "$cols" -ge 8 ]; then
      test_pg_port=$(echo "$existing" | awk '{print $4}')
      test_ls_port=$(echo "$existing" | awk '{print $5}')
      pulse_port=$(echo "$existing" | awk '{print $6}')
      web_port=$(echo "$existing" | awk '{print $7}')
      cortex_port=$(echo "$existing" | awk '{print $8}')
    elif [ "$cols" -ge 7 ]; then
      test_pg_port=$(echo "$existing" | awk '{print $4}')
      test_ls_port=$(echo "$existing" | awk '{print $5}')
      pulse_port=$(echo "$existing" | awk '{print $6}')
      web_port=$(echo "$existing" | awk '{print $7}')
      cortex_port=$((CORTEX_BASE + slot))
      # Migrate 7-column entry to 8-column
      grep -v "^${id} " "$REGISTRY" > "${REGISTRY}.tmp" 2>/dev/null || true
      echo "${id} ${be_port} ${fe_port} ${test_pg_port} ${test_ls_port} ${pulse_port} ${web_port} ${cortex_port}" >> "${REGISTRY}.tmp"
      mv "${REGISTRY}.tmp" "$REGISTRY"
    elif [ "$cols" -ge 6 ]; then
      test_pg_port=$(echo "$existing" | awk '{print $4}')
      test_ls_port=$(echo "$existing" | awk '{print $5}')
      pulse_port=$(echo "$existing" | awk '{print $6}')
      web_port=$((WEB_BASE + slot))
      cortex_port=$((CORTEX_BASE + slot))
      # Migrate 6-column entry to 8-column
      grep -v "^${id} " "$REGISTRY" > "${REGISTRY}.tmp" 2>/dev/null || true
      echo "${id} ${be_port} ${fe_port} ${test_pg_port} ${test_ls_port} ${pulse_port} ${web_port} ${cortex_port}" >> "${REGISTRY}.tmp"
      mv "${REGISTRY}.tmp" "$REGISTRY"
    elif [ "$cols" -ge 5 ]; then
      test_pg_port=$(echo "$existing" | awk '{print $4}')
      test_ls_port=$(echo "$existing" | awk '{print $5}')
      pulse_port=$((PULSE_BASE + slot))
      web_port=$((WEB_BASE + slot))
      cortex_port=$((CORTEX_BASE + slot))
      # Migrate 5-column entry to 8-column
      grep -v "^${id} " "$REGISTRY" > "${REGISTRY}.tmp" 2>/dev/null || true
      echo "${id} ${be_port} ${fe_port} ${test_pg_port} ${test_ls_port} ${pulse_port} ${web_port} ${cortex_port}" >> "${REGISTRY}.tmp"
      mv "${REGISTRY}.tmp" "$REGISTRY"
    else
      # Migrate old 3-column entry to 8-column
      test_pg_port=$((TEST_PG_BASE + slot))
      test_ls_port=$((TEST_LS_BASE + slot))
      pulse_port=$((PULSE_BASE + slot))
      web_port=$((WEB_BASE + slot))
      cortex_port=$((CORTEX_BASE + slot))
      grep -v "^${id} " "$REGISTRY" > "${REGISTRY}.tmp" 2>/dev/null || true
      echo "${id} ${be_port} ${fe_port} ${test_pg_port} ${test_ls_port} ${pulse_port} ${web_port} ${cortex_port}" >> "${REGISTRY}.tmp"
      mv "${REGISTRY}.tmp" "$REGISTRY"
    fi
    echo "BE_PORT=${be_port}"
    echo "FE_PORT=${fe_port}"
    echo "TEST_PG_PORT=${test_pg_port}"
    echo "TEST_LS_PORT=${test_ls_port}"
    echo "PULSE_PORT=${pulse_port}"
    echo "WEB_PORT=${web_port}"
    echo "CORTEX_PORT=${cortex_port}"
    return 0
  fi

  # Check if a port is actually free on the host (not just in registry)
  port_is_free() {
    ! lsof -i ":${1}" -sTCP:LISTEN >/dev/null 2>&1
  }

  # Find next free slot (checks both registry AND host)
  local slot=0
  while [ "$slot" -lt "$MAX_SLOTS" ]; do
    local be_port=$((BE_BASE + slot))
    local fe_port=$((FE_BASE + slot))
    local test_pg_port=$((TEST_PG_BASE + slot))
    local test_ls_port=$((TEST_LS_BASE + slot))
    local pulse_port=$((PULSE_BASE + slot))
    local web_port=$((WEB_BASE + slot))
    local cortex_port=$((CORTEX_BASE + slot))
    if ! awk -v p="${be_port}" '$2 == p' "$REGISTRY" | grep -q .; then
      # Verify key ports are actually free on the host
      if port_is_free "$be_port" && port_is_free "$fe_port" && port_is_free "$test_pg_port" && port_is_free "$test_ls_port" && port_is_free "$cortex_port"; then
        echo "${id} ${be_port} ${fe_port} ${test_pg_port} ${test_ls_port} ${pulse_port} ${web_port} ${cortex_port}" >> "$REGISTRY"
        echo "BE_PORT=${be_port}"
        echo "FE_PORT=${fe_port}"
        echo "TEST_PG_PORT=${test_pg_port}"
        echo "TEST_LS_PORT=${test_ls_port}"
        echo "PULSE_PORT=${pulse_port}"
        echo "WEB_PORT=${web_port}"
        echo "CORTEX_PORT=${cortex_port}"
        return 0
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
  printf "%-40s %-10s %-10s %-10s %-10s %-10s %-10s %-12s\n" "WORKTREE" "BE_PORT" "FE_PORT" "TEST_PG" "TEST_LS" "PULSE" "WEB" "CORTEX_HTTP"
  while IFS= read -r line; do
    local id be fe tpg tls pulse web cortex
    id=$(echo "$line" | awk '{print $1}')
    be=$(echo "$line" | awk '{print $2}')
    fe=$(echo "$line" | awk '{print $3}')
    tpg=$(echo "$line" | awk '{print $4}')
    tls=$(echo "$line" | awk '{print $5}')
    pulse=$(echo "$line" | awk '{print $6}')
    web=$(echo "$line" | awk '{print $7}')
    cortex=$(echo "$line" | awk '{print $8}')
    # Handle old entries gracefully
    [ -z "$tpg" ] && tpg="—"
    [ -z "$tls" ] && tls="—"
    [ -z "$pulse" ] && pulse="—"
    [ -z "$web" ] && web="—"
    [ -z "$cortex" ] && cortex="—"
    printf "%-40s %-10s %-10s %-10s %-10s %-10s %-10s %-12s\n" "$id" "$be" "$fe" "$tpg" "$tls" "$pulse" "$web" "$cortex"
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
