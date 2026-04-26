#!/usr/bin/env bash
# alloc-ports.sh — Allocate unique ports for a worktree pipeline
#
# Usage:
#   ./.claude/scripts/alloc-ports.sh alloc <worktree-id>   → prints KEY=VAL per line
#   ./.claude/scripts/alloc-ports.sh free  <worktree-id>   → releases the allocation
#   ./.claude/scripts/alloc-ports.sh list                  → shows all allocations
#
# === EDIT FOR YOUR STACK ===
# Each port range is 99 slots wide. Adjust BASE values to free ranges on your machine.
# main checkout uses BASE-1 (e.g., main API on 3000, worktrees get 3001-3099).
#
# Registry: .worktrees/.ports (one whitespace-separated line per allocation)

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
REGISTRY="${ROOT}/.worktrees/.ports"
LOCKFILE="${REGISTRY}.lock"

# === Port range bases — EDIT FOR YOUR STACK ===
# One base per logical port your stack needs. Each range is MAX_SLOTS slots wide,
# so pick bases that don't overlap. Keep PORT_FIELDS and PORT_BASES in sync.
PORT_A_BASE={CHOOSE_A_BASE_PORT}     # e.g., a primary service port
PORT_B_BASE={CHOOSE_A_BASE_PORT}     # e.g., a secondary service port
PORT_C_BASE={CHOOSE_A_BASE_PORT}     # e.g., an isolated test backing-service port

PORT_FIELDS=("PORT_A" "PORT_B" "PORT_C")
PORT_BASES=("$PORT_A_BASE" "$PORT_B_BASE" "$PORT_C_BASE")

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

emit_ports_for_slot() {
  local slot="$1"
  local i=0
  for field in "${PORT_FIELDS[@]}"; do
    local base="${PORT_BASES[$i]}"
    echo "${field}=$((base + slot))"
    i=$((i + 1))
  done
}

cmd_alloc() {
  local id="$1"
  acquire_lock

  # Already allocated?
  local existing
  existing=$(awk -v id="$id" '$1 == id { print $0 }' "$REGISTRY")
  if [ -n "$existing" ]; then
    local first_port slot
    first_port=$(echo "$existing" | awk '{print $2}')
    slot=$((first_port - ${PORT_BASES[0]}))
    emit_ports_for_slot "$slot"
    return 0
  fi

  # Helper: is this port actually free on the host?
  port_is_free() {
    ! lsof -i ":${1}" -sTCP:LISTEN >/dev/null 2>&1
  }

  # Find next free slot
  local slot=0
  while [ "$slot" -lt "$MAX_SLOTS" ]; do
    local first_port=$((${PORT_BASES[0]} + slot))
    # Slot is taken in registry?
    if awk -v p="$first_port" '$2 == p' "$REGISTRY" | grep -q .; then
      slot=$((slot + 1))
      continue
    fi
    # All ports for this slot actually free on the host?
    local all_free=1
    for base in "${PORT_BASES[@]}"; do
      if ! port_is_free "$((base + slot))"; then
        all_free=0
        break
      fi
    done
    if [ "$all_free" = 1 ]; then
      # Write registry line: <id> <port1> <port2> ...
      local line="$id"
      for base in "${PORT_BASES[@]}"; do
        line="$line $((base + slot))"
      done
      echo "$line" >> "$REGISTRY"
      emit_ports_for_slot "$slot"
      return 0
    fi
    slot=$((slot + 1))
  done

  echo "Error: no free port slots (all $MAX_SLOTS in use)" >&2
  exit 1
}

cmd_free() {
  local id="$1"
  acquire_lock

  grep -v "^${id} " "$REGISTRY" > "${REGISTRY}.tmp" 2>/dev/null || true
  mv "${REGISTRY}.tmp" "$REGISTRY"
  echo "Freed ports for: $id"
}

cmd_list() {
  if [ ! -s "$REGISTRY" ]; then
    echo "No port allocations."
    return 0
  fi
  # Header
  printf "%-40s" "WORKTREE"
  for field in "${PORT_FIELDS[@]}"; do
    printf " %-12s" "$field"
  done
  printf "\n"
  # Rows
  while IFS= read -r line; do
    local id
    id=$(echo "$line" | awk '{print $1}')
    printf "%-40s" "$id"
    local i=2
    for _ in "${PORT_FIELDS[@]}"; do
      local val
      val=$(echo "$line" | awk -v col="$i" '{print $col}')
      [ -z "$val" ] && val="—"
      printf " %-12s" "$val"
      i=$((i + 1))
    done
    printf "\n"
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
