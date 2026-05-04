#!/usr/bin/env bash
# alloc-ports.sh — Allocate unique ports for a worktree pipeline
#
# Usage:
#   ./.claude/scripts/alloc-ports.sh alloc <worktree-id>   -> prints KEY=VAL per line
#   ./.claude/scripts/alloc-ports.sh free  <worktree-id>   -> releases the allocation
#   ./.claude/scripts/alloc-ports.sh list                   -> shows all allocations
#
# === Per-project setup — EDIT FOR YOUR STACK ===
# Port ranges (customize base ports and ranges for your services):
#   Backend:         3001-3099  (main uses 3000)
#   Frontend:        5174-5272  (main uses 5173)
#   Test PostgreSQL: 5434-5532  (shared test uses 5433)
#   Test Services:   4568-4666  (shared test uses 4567)
#   Add more ranges as needed for your services (e.g., web, AI engine, analytics)
#
# Registry: .worktrees/.ports (one line per allocation: id port1 port2 port3 ...)
# ================================================

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REGISTRY="${ROOT}/.worktrees/.ports"
LOCKFILE="${REGISTRY}.lock"

# === Per-project setup — EDIT FOR YOUR STACK ===
# Define base ports for each service. Each pipeline gets base+slot.
BE_BASE=3001
FE_BASE=5174
TEST_PG_BASE=5434
TEST_LS_BASE=4568
# Add more bases as needed:
# WEB_BASE=4001
# CORTEX_BASE=3501
# PULSE_BASE=3302
MAX_SLOTS=99
# ================================================

# Number of port columns in the registry (update when adding new port types)
# This determines the format: id col1 col2 col3 col4
NUM_COLS=4

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

  # Already allocated? Return existing allocation.
  local existing
  existing=$(awk -v id="$id" '$1 == id { print $0 }' "$REGISTRY")
  if [ -n "$existing" ]; then
    local cols
    cols=$(echo "$existing" | awk '{print NF}')
    local be_port fe_port test_pg_port test_ls_port
    be_port=$(echo "$existing" | awk '{print $2}')
    fe_port=$(echo "$existing" | awk '{print $3}')
    local slot=$((be_port - BE_BASE))

    if [ "$cols" -ge "$((NUM_COLS + 1))" ]; then
      # Full entry — read all columns
      test_pg_port=$(echo "$existing" | awk '{print $4}')
      test_ls_port=$(echo "$existing" | awk '{print $5}')
    else
      # Legacy entry — compute missing ports from slot and migrate
      test_pg_port=$((TEST_PG_BASE + slot))
      test_ls_port=$((TEST_LS_BASE + slot))
      grep -v "^${id} " "$REGISTRY" > "${REGISTRY}.tmp" 2>/dev/null || true
      echo "${id} ${be_port} ${fe_port} ${test_pg_port} ${test_ls_port}" >> "${REGISTRY}.tmp"
      mv "${REGISTRY}.tmp" "$REGISTRY"
    fi

    echo "BE_PORT=${be_port}"
    echo "FE_PORT=${fe_port}"
    echo "TEST_PG_PORT=${test_pg_port}"
    echo "TEST_LS_PORT=${test_ls_port}"
    # === Per-project setup — EDIT FOR YOUR STACK ===
    # Echo additional port assignments here
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
    if ! awk -v p="${be_port}" '$2 == p' "$REGISTRY" | grep -q .; then
      # Verify key ports are actually free on the host
      if port_is_free "$be_port" && port_is_free "$fe_port" && port_is_free "$test_pg_port" && port_is_free "$test_ls_port"; then
        echo "${id} ${be_port} ${fe_port} ${test_pg_port} ${test_ls_port}" >> "$REGISTRY"
        echo "BE_PORT=${be_port}"
        echo "FE_PORT=${fe_port}"
        echo "TEST_PG_PORT=${test_pg_port}"
        echo "TEST_LS_PORT=${test_ls_port}"
        # === Per-project setup — EDIT FOR YOUR STACK ===
        # Echo additional port assignments here
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
  # === Per-project setup — EDIT FOR YOUR STACK ===
  # Update the header to match your port columns
  printf "%-40s %-10s %-10s %-10s %-10s\n" "WORKTREE" "BE_PORT" "FE_PORT" "TEST_PG" "TEST_LS"
  while IFS= read -r line; do
    local id be fe tpg tls
    id=$(echo "$line" | awk '{print $1}')
    be=$(echo "$line" | awk '{print $2}')
    fe=$(echo "$line" | awk '{print $3}')
    tpg=$(echo "$line" | awk '{print $4}')
    tls=$(echo "$line" | awk '{print $5}')
    # Handle old entries gracefully
    [ -z "$tpg" ] && tpg="---"
    [ -z "$tls" ] && tls="---"
    printf "%-40s %-10s %-10s %-10s %-10s\n" "$id" "$be" "$fe" "$tpg" "$tls"
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
