#!/usr/bin/env bash
# dev.sh — Fast dev environment manager
#
# Usage:
#   ./dev.sh up           Start full dev environment (default)
#   ./dev.sh kill         Stop all dev servers
#   ./dev.sh restart      Kill then start fresh
#   ./dev.sh drop         Nuke Docker containers, rebuild, restart if servers were running
#   ./dev.sh fresh        Kill + drop + start — full clean slate, always starts servers
#   ./dev.sh status       Show what's running
#   ./dev.sh log [service] [N]  Show last N log lines (default: 50)
#   ./dev.sh clear-logs   Delete all logs
#   ./dev.sh snapshot     Export DB tables to seed data directory
#
# This script replaces slow interpreted /dev commands with
# native bash that runs steps in parallel where possible.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DEV_DIR="${ROOT}/tmp/dev"
PID_FILE="${DEV_DIR}/dev-servers.pid"
ARCHIVE_DIR="${DEV_DIR}/archive"

# === Per-project setup — EDIT FOR YOUR STACK ===
# Define your project directories and default ports.
# If .dev-ports exists at project root, source it (isolated env / worktree).
# Otherwise use defaults (normal local dev).
DEV_PORTS_FILE="${ROOT}/.dev-ports"
if [ -f "$DEV_PORTS_FILE" ]; then
  # shellcheck disable=SC1090
  source "$DEV_PORTS_FILE"
  ISO_MODE=true
  ISO_PROFILE=$(grep "^# Profile:" "$DEV_PORTS_FILE" | sed 's/# Profile: //' | tr -d '[:space:]')
else
  # Defaults — EDIT these for your stack
  BE_PORT=3000
  FE_PORT=8081
  WEB_PORT=4000
  # CORTEX_PORT=3500  # Uncomment if you have an AI engine HTTP server
  ISO_MODE=false
  ISO_PROFILE=""
fi
# ================================================

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
fail()  { echo -e "${RED}[FAIL]${NC}  $*"; }
header(){ echo -e "\n${BOLD}=== $* ===${NC}"; }

# --- Helpers --------------------------------------------------------

ensure_dirs() {
  mkdir -p "$DEV_DIR" "$ARCHIVE_DIR"
}

archive_logs() {
  local next_seq=1
  for f in "$ARCHIVE_DIR"/*.log; do
    [ -f "$f" ] || continue
    local num
    num=$(echo "$f" | sed 's/.*\.\([0-9]*\)\.log/\1/' | sed 's/^0*//')
    [ -z "$num" ] && num=0
    if [ "$num" -ge "$next_seq" ]; then
      next_seq=$((num + 1))
    fi
  done
  local seq_pad
  seq_pad=$(printf "%03d" "$next_seq")
  # === Per-project setup — EDIT FOR YOUR STACK ===
  # List your service log files
  for log in "$DEV_DIR"/be.log "$DEV_DIR"/fe.log "$DEV_DIR"/web.log; do
    if [ -f "$log" ] && [ -s "$log" ]; then
      local base
      base=$(basename "$log" .log)
      mv "$log" "${ARCHIVE_DIR}/${base}.${seq_pad}.log"
    fi
  done
}

check_prereqs() {
  local missing=0
  # === Per-project setup — EDIT FOR YOUR STACK ===
  # List the tools your dev environment needs
  for cmd in node docker; do
    if ! command -v "$cmd" &>/dev/null; then
      fail "Missing: $cmd"
      missing=1
    fi
  done
  if [ "$missing" -eq 1 ]; then
    echo "Install missing tools and retry."
    exit 1
  fi
}

wait_for() {
  local label="$1" check_cmd="$2" max_wait="${3:-15}"
  local elapsed=0
  while [ "$elapsed" -lt "$max_wait" ]; do
    if eval "$check_cmd" &>/dev/null; then
      ok "$label ready (${elapsed}s)"
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done
  fail "$label not ready after ${max_wait}s"
  return 1
}

kill_tree() {
  local pid="$1"
  # Guard against invalid PIDs
  if [[ ! "$pid" =~ ^[0-9]+$ ]] || [ "$pid" -le 1 ]; then
    return 0
  fi
  if kill -0 "$pid" 2>/dev/null; then
    pkill -TERM -P "$pid" 2>/dev/null || true
    kill -TERM "$pid" 2>/dev/null || true
  fi
}

force_kill_tree() {
  local pid="$1"
  if [[ ! "$pid" =~ ^[0-9]+$ ]] || [ "$pid" -le 1 ]; then
    return 0
  fi
  pkill -9 -P "$pid" 2>/dev/null || true
  kill -9 "$pid" 2>/dev/null || true
}

clean_ports() {
  # === Per-project setup — EDIT FOR YOUR STACK ===
  # List all ports your services use
  for port in $BE_PORT $FE_PORT $WEB_PORT; do
    lsof -ti :"$port" 2>/dev/null | xargs kill -9 2>/dev/null || true
  done
}

# --- MODE: KILL ---------------------------------------------------

cmd_kill() {
  header "Stopping dev servers"

  if [ -f "$PID_FILE" ]; then
    while read -r name pid port; do
      [ -z "$name" ] && continue
      if kill -0 "$pid" 2>/dev/null; then
        kill_tree "$pid"
        info "SIGTERM -> $name (PID $pid, port $port)"
      else
        info "$name (PID $pid) already stopped"
      fi
    done < "$PID_FILE"

    sleep 2

    while read -r name pid port; do
      [ -z "$name" ] && continue
      force_kill_tree "$pid"
    done < "$PID_FILE"
  fi

  clean_ports
  rm -f "$PID_FILE"

  # Verify
  local all_clean=true
  # === Per-project setup — EDIT FOR YOUR STACK ===
  for port in $BE_PORT $FE_PORT $WEB_PORT; do
    if lsof -ti :"$port" &>/dev/null; then
      warn "Port $port still occupied"
      all_clean=false
    fi
  done

  if $all_clean; then
    ok "All ports clean"
  fi

  echo ""
  echo "KILL_RESULT=success"
}

# --- MODE: UP -----------------------------------------------------

cmd_up() {
  ensure_dirs

  # Check for already running servers
  if [ -f "$PID_FILE" ]; then
    local alive_count=0
    while read -r name pid port; do
      [ -z "$name" ] && continue
      if kill -0 "$pid" 2>/dev/null; then
        alive_count=$((alive_count + 1))
      fi
    done < "$PID_FILE"

    if [ "$alive_count" -gt 0 ]; then
      echo "ALREADY_RUNNING=true"
      return 0
    else
      rm -f "$PID_FILE"
    fi
  fi

  archive_logs
  check_prereqs

  # -- Step 1: Infrastructure --
  header "Infrastructure"
  if $ISO_MODE; then
    info "Isolated mode — checking existing containers..."
    # Verify infrastructure is running
  else
    info "Starting infrastructure..."
    # === Per-project setup — EDIT FOR YOUR STACK ===
    # Start your infrastructure (databases, message queues, etc.)
    # make -C "$ROOT/{project-infra}" up-local 2>&1 | tail -3
    #
    # local infra_ok=true
    # wait_for "PostgreSQL ({PORT})" "make -C '$ROOT/{project-infra}' pg-ready-local" 20 || infra_ok=false
    #
    # if ! $infra_ok; then
    #   fail "Infrastructure not ready — aborting"
    #   exit 1
    # fi
    :
  fi

  # -- Step 2: Dependencies (parallel) --
  header "Dependencies"
  local dep_pids=()

  # === Per-project setup — EDIT FOR YOUR STACK ===
  # Install dependencies for each project in parallel.
  # Example:
  # (cd "$ROOT/{project-be}" && {PACKAGE_MANAGER} install 2>&1 | tail -1 && echo "BE_DEPS=ok") &
  # dep_pids+=($!)
  # (cd "$ROOT/{project-fe}" && npm install 2>&1 | tail -1 && echo "FE_DEPS=ok") &
  # dep_pids+=($!)

  local deps_ok=true
  for pid in "${dep_pids[@]}"; do
    if ! wait "$pid"; then
      deps_ok=false
    fi
  done

  if $deps_ok; then
    ok "All dependencies installed"
  else
    warn "Some dependency installs had issues — continuing"
  fi

  # -- Step 3: Backend env file --
  if ! $ISO_MODE; then
    # === Per-project setup — EDIT FOR YOUR STACK ===
    # Create default env file if missing
    # if [ ! -f "$ROOT/{project-be}/.env.local" ]; then
    #   warn "Creating default {project-be}/.env.local"
    #   cat > "$ROOT/{project-be}/.env.local" << 'EOF'
    #   DATABASE_URL=postgresql://postgres@localhost:{PORT}/{DB_NAME}
    #   JWT_SECRET=dev-secret-change-me
    #   PORT={DEFAULT_PORT}
    #   EOF
    # fi
    :
  fi

  # -- Step 4: Database --
  if $ISO_MODE; then
    header "Database"
    ok "ISO mode — schema applied during init (skipping migrations)"
  else
    header "Database"
    info "Checking schema + migrations..."
    # === Per-project setup — EDIT FOR YOUR STACK ===
    # make -C "$ROOT/{project-infra}" db-create-local
    # make -C "$ROOT/{project-infra}" db-migrate-local 2>&1 | tail -2
    ok "Database ready"
  fi

  # -- Step 5: Start servers --
  header "Starting servers"

  # Write PID file atomically
  : > "$PID_FILE"

  # === Per-project setup — EDIT FOR YOUR STACK ===
  # Start each service and record its PID.
  # Example:

  # Backend
  # (cd "$ROOT/{project-be}" && NO_COLOR=1 {PACKAGE_MANAGER} run dev > "$DEV_DIR/be.log" 2>&1) &
  # disown $!
  # echo "backend $! $BE_PORT" >> "$PID_FILE"
  # info "Backend starting (PID $!, port $BE_PORT)"

  # Frontend
  # (cd "$ROOT/{project-fe}" && NO_COLOR=1 npx expo start --web --port "$FE_PORT" > "$DEV_DIR/fe.log" 2>&1) &
  # disown $!
  # echo "frontend $! $FE_PORT" >> "$PID_FILE"
  # info "Frontend starting (PID $!, port $FE_PORT)"

  # Web (marketing site)
  # (cd "$ROOT/{project-web}" && NO_COLOR=1 npx next dev --port "$WEB_PORT" > "$DEV_DIR/web.log" 2>&1) &
  # disown $!
  # echo "web $! $WEB_PORT" >> "$PID_FILE"
  # info "Web starting (PID $!, port $WEB_PORT)"
  # ================================================

  # -- Step 6: Health checks --
  header "Health checks"
  sleep 4

  local be_status="RED"
  local be_http_code
  be_http_code=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$BE_PORT/health" 2>/dev/null || echo "000")
  if [ "$be_http_code" = "200" ]; then
    be_status="GREEN"
    ok "Backend healthy"
  elif [ "$be_http_code" = "503" ]; then
    be_status="YELLOW"
    warn "Backend degraded (may still be warming up)"
  else
    fail "Backend health check failed (HTTP $be_http_code)"
  fi

  local fe_status="RED"
  if curl -sf http://localhost:$FE_PORT -o /dev/null 2>/dev/null; then
    fe_status="GREEN"
    ok "Frontend responding"
  else
    sleep 3
    if curl -sf http://localhost:$FE_PORT -o /dev/null 2>/dev/null; then
      fe_status="GREEN"
      ok "Frontend responding"
    else
      warn "Frontend not yet responding (may still be bundling)"
      fe_status="YELLOW"
    fi
  fi

  local web_status="RED"
  if curl -sf http://localhost:$WEB_PORT -o /dev/null 2>/dev/null; then
    web_status="GREEN"
    ok "Web responding"
  else
    sleep 3
    if curl -sf http://localhost:$WEB_PORT -o /dev/null 2>/dev/null; then
      web_status="GREEN"
      ok "Web responding"
    else
      warn "Web not yet responding (may still be compiling)"
      web_status="YELLOW"
    fi
  fi

  # -- Step 7: Scan logs for errors --
  local errors=""
  # === Per-project setup — EDIT FOR YOUR STACK ===
  # List your service log file names
  for svc in be fe web; do
    local logfile="$DEV_DIR/${svc}.log"
    if [ -f "$logfile" ]; then
      local svc_errors
      svc_errors=$(grep -a -iE '(ERR|Error|FATAL|Exception|Traceback|ECONNREFUSED|EADDRINUSE|ModuleNotFoundError|Cannot find module)' "$logfile" 2>/dev/null | grep -v "^Binary file" | grep -viE '(WARN.*swallowing|Warning:|DeprecationWarning|ExperimentalWarning)' | head -3 || true)
      if [ -n "$svc_errors" ]; then
        errors="${errors}\n  ${svc}: $(echo "$svc_errors" | head -1)"
      fi
    fi
  done

  # -- Step 8: Output report --
  echo ""
  echo "---REPORT---"
  echo "BE_STATUS=$be_status"
  echo "FE_STATUS=$fe_status"
  echo "WEB_STATUS=$web_status"

  if [ -n "$errors" ]; then
    echo -e "ERRORS=$errors"
  else
    echo "ERRORS=none"
  fi
  echo "---END---"
}

# --- MODE: DROP ---------------------------------------------------

cmd_drop() {
  ensure_dirs

  # Check if servers were running before we nuke everything
  local were_running=false
  if [ -f "$PID_FILE" ]; then
    while read -r name pid port; do
      [ -z "$name" ] && continue
      if kill -0 "$pid" 2>/dev/null; then
        were_running=true
        break
      fi
    done < "$PID_FILE"
  fi

  # Step 1: Kill servers if running
  if $were_running; then
    header "Killing servers before drop"
    cmd_kill
    echo ""
  fi

  # Step 2: Nuke Docker containers + volumes
  header "Nuking Docker containers"
  # === Per-project setup — EDIT FOR YOUR STACK ===
  # make -C "$ROOT/{project-infra}" nuke-local 2>&1 | tail -5
  ok "Docker containers nuked"

  # Step 3: Bring fresh containers up
  header "Rebuilding infrastructure"
  # === Per-project setup — EDIT FOR YOUR STACK ===
  # make -C "$ROOT/{project-infra}" up-local 2>&1 | tail -3
  # wait_for "PostgreSQL ({PORT})" "make -C '$ROOT/{project-infra}' pg-ready-local" 30 || true
  ok "Infrastructure rebuilt"

  # Step 4: Recreate database + migrate
  header "Database"
  # === Per-project setup — EDIT FOR YOUR STACK ===
  # make -C "$ROOT/{project-infra}" db-create-local
  # make -C "$ROOT/{project-infra}" db-migrate-local 2>&1 | tail -2
  ok "Database migrated"

  # Step 5: Restart servers if they were running before
  if $were_running; then
    header "Restarting servers (were running before drop)"
    cmd_up
  else
    echo ""
    echo "---REPORT---"
    echo "WERE_RUNNING=false"
    echo "NUKE_RESULT=success"
    echo "INFRA_RESULT=success"
    echo "SERVERS_SKIPPED=true"
    echo "---END---"
  fi
}

# --- MODE: FRESH --------------------------------------------------

cmd_fresh() {
  ensure_dirs

  # Step 1: Kill servers unconditionally
  header "Killing servers"
  cmd_kill
  echo ""

  # Step 2: Nuke Docker containers + volumes
  header "Nuking Docker containers"
  # === Per-project setup — EDIT FOR YOUR STACK ===
  # make -C "$ROOT/{project-infra}" nuke-local 2>&1 | tail -5
  ok "Docker containers nuked"

  # Step 3: Bring fresh containers up
  header "Rebuilding infrastructure"
  # === Per-project setup — EDIT FOR YOUR STACK ===
  # make -C "$ROOT/{project-infra}" up-local 2>&1 | tail -3
  # wait_for "PostgreSQL ({PORT})" "make -C '$ROOT/{project-infra}' pg-ready-local" 30 || true
  ok "Infrastructure rebuilt"

  # Step 4: Recreate database + migrate
  header "Database"
  # === Per-project setup — EDIT FOR YOUR STACK ===
  # make -C "$ROOT/{project-infra}" db-create-local
  # make -C "$ROOT/{project-infra}" db-migrate-local 2>&1 | tail -2
  ok "Database migrated"

  # Step 5: Always start servers
  header "Starting servers"
  cmd_up
}

# --- MODE: STATUS -------------------------------------------------

cmd_status() {
  header "Dev server status"

  if [ ! -f "$PID_FILE" ] || [ ! -s "$PID_FILE" ]; then
    echo "NO_SERVERS=true"
    return 0
  fi

  echo "---REPORT---"
  # Deduplicate PID entries — keep last entry per service name
  local _deduped
  _deduped=$(awk '{lines[NR]=$0} END {for(i=NR;i>=1;i--) print lines[i]}' "$PID_FILE" | awk 'NF && !seen[$1]++ {print}')
  while read -r name pid port; do
    [ -z "$name" ] && continue
    local alive="dead" responding="no"
    if kill -0 "$pid" 2>/dev/null; then alive="running"; fi
    if curl -sf "http://localhost:$port" -o /dev/null 2>/dev/null; then responding="yes"; fi
    echo "SVC=${name}|PID=${pid}|PORT=${port}|ALIVE=${alive}|RESPONDING=${responding}"
  done <<< "$_deduped"

  # Health checks
  # === Per-project setup — EDIT FOR YOUR STACK ===
  local be_health="fail"
  curl -s --max-time 15 http://localhost:$BE_PORT/health &>/dev/null && be_health="ok"
  local fe_health="fail"
  curl -sf http://localhost:$FE_PORT -o /dev/null &>/dev/null && fe_health="ok"
  local web_health="fail"
  curl -sf http://localhost:$WEB_PORT -o /dev/null &>/dev/null && web_health="ok"

  echo "BE_HEALTH=$be_health"
  echo "FE_HEALTH=$fe_health"
  echo "WEB_HEALTH=$web_health"
  echo "---END---"
}

# --- MODE: LOG ----------------------------------------------------

cmd_log() {
  local service="${1:-all}"
  local lines="${2:-50}"

  # Handle numeric first arg (e.g., /dev log 100)
  if [[ "$service" =~ ^[0-9]+$ ]]; then
    lines="$service"
    service="all"
  fi

  # Handle second arg as lines count
  if [ -n "${2:-}" ] && [[ "${2}" =~ ^[0-9]+$ ]]; then
    lines="$2"
  fi

  # === Per-project setup — EDIT FOR YOUR STACK ===
  # Map service names to log files
  local services=()
  case "$service" in
    be|backend)  services=(be) ;;
    fe|frontend) services=(fe) ;;
    web)         services=(web) ;;
    all|*)       services=(be fe web) ;;
  esac

  for svc in "${services[@]}"; do
    local logfile="$DEV_DIR/${svc}.log"
    local label
    case $svc in
      be)  label="Backend" ;;
      fe)  label="Frontend" ;;
      web) label="Web" ;;
      *)   label="$svc" ;;
    esac
    echo "--- $label ($logfile) --- [last $lines lines]"
    if [ -f "$logfile" ] && [ -s "$logfile" ]; then
      tail -n "$lines" "$logfile"
    else
      echo "(no output yet)"
    fi
    echo ""
  done

  # Error summary
  echo "Error summary:"
  for svc in "${services[@]}"; do
    local logfile="$DEV_DIR/${svc}.log"
    local label
    case $svc in
      be)  label="Backend" ;;
      fe)  label="Frontend" ;;
      web) label="Web" ;;
      *)   label="$svc" ;;
    esac
    if [ -f "$logfile" ]; then
      local count
      count=$(grep -ciE '(ERR|Error|FATAL|Exception|Traceback)' "$logfile" 2>/dev/null || echo "0")
      if [ "$count" -gt 0 ]; then
        local first_err
        first_err=$(grep -iE '(ERR|Error|FATAL|Exception|Traceback)' "$logfile" 2>/dev/null | head -1)
        echo "  $label: $count error(s) — $first_err"
      else
        echo "  $label: no errors"
      fi
    else
      echo "  $label: (no log file)"
    fi
  done
}

# --- MODE: LOG-CLEAR ----------------------------------------------

cmd_log_clear() {
  local cleared=0
  # === Per-project setup — EDIT FOR YOUR STACK ===
  for f in "$DEV_DIR"/be.log "$DEV_DIR"/fe.log "$DEV_DIR"/web.log; do
    [ -f "$f" ] && rm -f "$f" && cleared=$((cleared + 1))
  done
  local archived
  archived=$(find "$ARCHIVE_DIR" -name "*.log" 2>/dev/null | wc -l | tr -d ' ')
  rm -f "$ARCHIVE_DIR"/*.log 2>/dev/null || true

  echo "CLEARED_CURRENT=$cleared"
  echo "CLEARED_ARCHIVE=$archived"
}

# --- MODE: SNAPSHOT -----------------------------------------------

cmd_snapshot() {
  header "Exporting seed data"

  # === Per-project setup — EDIT FOR YOUR STACK ===
  # Customize the snapshot/export command for your project.
  # Example:
  # local env_file="$ROOT/{project-be}/.env.local"
  # cd "$ROOT/{project-be}" && {PACKAGE_MANAGER} db:export
  echo "Snapshot command not configured — edit dev.sh to add your export logic."
}

# --- MAIN ---------------------------------------------------------

ensure_dirs

case "${1:-up}" in
  up|start)        cmd_up ;;
  kill|stop|down)  cmd_kill ;;
  restart)         cmd_kill; cmd_up ;;
  drop)            cmd_drop ;;
  fresh)           cmd_fresh ;;
  status)          cmd_status ;;
  log|logs)        shift; cmd_log "$@" ;;
  clear-logs|cl)   cmd_log_clear ;;
  snapshot)        cmd_snapshot ;;
  *)
    echo "Usage: $0 {up|kill|restart|drop|fresh|status|log|clear-logs|snapshot}"
    exit 1
    ;;
esac
