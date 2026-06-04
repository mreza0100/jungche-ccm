#!/usr/bin/env bash
# dev.sh — Fast dev environment manager for {PROJECT_NAME}
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
#   ./dev.sh export       Export DB tables to seeding/local/ (runs {DB_EXPORT_CMD})
#   ./dev.sh promote-demo Copy seeding/local/ → seeding/demo/ (review passwords.json after)
#
# This script replaces the slow Claude-interpreted /dev command with
# native bash that runs steps in parallel where possible.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DEV_DIR="${ROOT}/tmp/dev"
PID_FILE="${DEV_DIR}/dev-servers.pid"
ARCHIVE_DIR="${DEV_DIR}/archive"

# Dev servers run on a DEDICATED tmux socket — never the default socket, where an
# interactive Claude Code session lives. Isolating it means the dev lifecycle
# (kill / restart / fresh) can never reach the tmux server Claude runs under. The
# old shared-socket setup once let `/dev fresh` take a live Claude session down
# with it. View / attach dev sessions with:  tmux -L {PROJECT_NAME_LOWER}-dev ls
DEV_TMUX_SOCKET="{PROJECT_NAME_LOWER}-dev"

# Port discovery: if .dev-ports exists at project root, source it (isolated env).
# Otherwise use defaults (normal local dev).
DEV_PORTS_FILE="${ROOT}/.dev-ports"
if [ -f "$DEV_PORTS_FILE" ]; then
  # shellcheck disable=SC1090
  source "$DEV_PORTS_FILE"
  ISO_MODE=true
  # Read profile from .dev-ports comment (e.g., "# Profile: demo")
  ISO_PROFILE=$(grep "^# Profile:" "$DEV_PORTS_FILE" | sed 's/# Profile: //' | tr -d '[:space:]')
else
  # Defaults (main local dev — {AI_SERVICE_NAME} HTTP on 3500, BE on {BACKEND_PORT}, FE on 8081, Web on {WEB_PORT})
  BE_PORT={BACKEND_PORT}
  FE_PORT=8081
  WEB_PORT={WEB_PORT}
  CORTEX_PORT=3500
  ISO_MODE=false
  ISO_PROFILE=""
fi

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

# ─── Helpers ──────────────────────────────────────────────────────

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
  for log in "$DEV_DIR"/be.log "$DEV_DIR"/cortex.log "$DEV_DIR"/fe.log "$DEV_DIR"/web.log; do
    if [ -f "$log" ] && [ -s "$log" ]; then
      local base
      base=$(basename "$log" .log)
      mv "$log" "${ARCHIVE_DIR}/${base}.${seq_pad}.log"
    fi
  done
}

check_prereqs() {
  local missing=0
  for cmd in node pnpm python3 uv docker; do
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
  # Guard against invalid PIDs — kill 0 would signal the entire process group
  # (including this shell's parent), and kill 1 would target init. Both disasters.
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

# Recycling guard. macOS reuses PIDs aggressively, so a stale PID_FILE entry can
# point at an unrelated process by the time we go to kill it. We must NEVER signal
# our own ancestry — the Claude Code session, its tmux server, and this script's
# shell are all ancestors of $$. A real dev server is always a detached child of
# the dev tmux server (or a nohup orphan), never an ancestor, so this skips them
# correctly while making it impossible to kill the session that launched us.
is_ancestor() {
  local target="$1" p="$$"
  [[ "$target" =~ ^[0-9]+$ ]] || return 1
  while [ "$p" -gt 1 ]; do
    [ "$p" = "$target" ] && return 0
    p=$(ps -p "$p" -o ppid= 2>/dev/null | tr -d ' ')
    [[ "$p" =~ ^[0-9]+$ ]] || return 1
  done
  return 1
}

clean_ports() {
  for port in $BE_PORT $FE_PORT $WEB_PORT $CORTEX_PORT; do
    lsof -ti :"$port" 2>/dev/null | xargs kill -9 2>/dev/null || true
  done
  # Kill {PROJECT_NAME_LOWER} processes scoped to THIS environment's directory.
  # Uses $ROOT to ensure main's kill doesn't murder ISO processes and vice versa.
  # IMPORTANT: main's ROOT is a prefix of ISO's ROOT (.worktrees/local/),
  # so we must EXCLUDE .worktrees/ matches when running from main.
  local pids
  pids=$(pgrep -f "{AI_PROCESS_PATTERN}|pnpm run dev|tsx.*src/index|uv run python|expo start|next dev" 2>/dev/null || true)
  for pid in $pids; do
    local cmdline
    cmdline=$(ps -p "$pid" -o args= 2>/dev/null || true)
    if $ISO_MODE; then
      # ISO mode: only kill processes from THIS worktree (exact ROOT match)
      if echo "$cmdline" | grep -qF "$ROOT"; then
        kill -9 "$pid" 2>/dev/null || true
      fi
    else
      # Main mode: kill processes from main repo BUT NOT from any worktree
      if echo "$cmdline" | grep -qF "$ROOT" && ! echo "$cmdline" | grep -qF ".worktrees/"; then
        kill -9 "$pid" 2>/dev/null || true
      fi
    fi
  done
}

start_detached() {
  local name="$1" port="$2" workdir="$3" logfile="$4" cmd="$5" verb="${6:-starting}"
  local pid
  if command -v tmux &>/dev/null; then
    local session="{PROJECT_NAME_LOWER}-dev-${name}-${port}"
    local launch_cmd
    tmux -L "$DEV_TMUX_SOCKET" kill-session -t "$session" 2>/dev/null || true
    printf -v launch_cmd 'cd %q && exec %s >> %q 2>&1' "$workdir" "$cmd" "$logfile"
    tmux -L "$DEV_TMUX_SOCKET" new-session -d -s "$session" "$launch_cmd"
    pid=$(tmux -L "$DEV_TMUX_SOCKET" display-message -p -t "$session" '#{pane_pid}')
  else
    nohup bash -lc "cd \"$workdir\" && exec $cmd" > "$logfile" 2>&1 &
    pid=$!
  fi
  echo "$name $pid $port" >> "$PID_FILE"
  case "$name" in
    backend)  info "Backend $verb (PID $pid, port $port)" ;;
    cortex)   info "{AI_SERVICE_NAME} $verb (PID $pid, port $port)" ;;
    frontend) info "Frontend $verb (PID $pid, port $port)" ;;
    web)      info "Web $verb (PID $pid, port $port)" ;;
  esac
}

# ─── MODE: KILL ───────────────────────────────────────────────────

cmd_kill() {
  header "Stopping dev servers"

  # Logs stay in tmp/dev/ after kill — user may want to inspect them.
  # Archival happens at next server start (cmd_up) so fresh logs begin clean.

  if [ -f "$PID_FILE" ]; then
    while read -r name pid port; do
      [ -z "$name" ] && continue
      if kill -0 "$pid" 2>/dev/null && ! is_ancestor "$pid"; then
        kill_tree "$pid"
        info "SIGTERM → $name (PID $pid, port $port)"
      elif is_ancestor "$pid"; then
        warn "$name (PID $pid) recycled onto this session's own process tree — refusing to signal it"
      else
        info "$name (PID $pid) already stopped"
      fi
    done < "$PID_FILE"

    sleep 2

    while read -r name pid port; do
      [ -z "$name" ] && continue
      is_ancestor "$pid" || force_kill_tree "$pid"
    done < "$PID_FILE"
  fi

  clean_ports
  rm -f "$PID_FILE"

  # Verify
  local all_clean=true
  for port in $BE_PORT $FE_PORT $WEB_PORT $CORTEX_PORT; do
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

# ─── MODE: UP ────────────────────────────────────────────────────

cmd_up() {
  ensure_dirs

  # Check for already running servers — detect partial failures
  if [ -f "$PID_FILE" ]; then
    local alive_count=0 dead_count=0
    local dead_names="" alive_entries=""
    while read -r name pid port; do
      [ -z "$name" ] && continue
      if kill -0 "$pid" 2>/dev/null; then
        alive_count=$((alive_count + 1))
        alive_entries="${alive_entries}${name} ${pid} ${port}\n"
      else
        dead_count=$((dead_count + 1))
        dead_names="${dead_names} ${name}"
      fi
    done < "$PID_FILE"

    if [ "$alive_count" -gt 0 ] && [ "$dead_count" -eq 0 ]; then
      # All registered services alive — nothing to do
      echo "ALREADY_RUNNING=true"
      return 0
    elif [ "$alive_count" -gt 0 ] && [ "$dead_count" -gt 0 ]; then
      # Partial failure — some alive, some dead. Resurrect the fallen.
      # Deduplicate dead names (e.g., two stale cortex entries → one resurrection)
      dead_names=$(echo "$dead_names" | tr ' ' '\n' | sort -u | tr '\n' ' ')
      warn "Partial failure detected: dead:${dead_names}"

      # Kill zombies for dead services — port-scoped only (safe for ISO coexistence)
      for dead_name in $dead_names; do
        case "$dead_name" in
          backend) lsof -ti :$BE_PORT 2>/dev/null | xargs kill -9 2>/dev/null || true ;;
          cortex)  # Kill {AI_SERVICE_NAME} by port first, then by process name scoped to $ROOT
                   lsof -ti :"$CORTEX_PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
                   local cpids
                   cpids=$(pgrep -f "{AI_PROCESS_PATTERN}" 2>/dev/null || true)
                   for cpid in $cpids; do
                     local ccmd
                     ccmd=$(ps -p "$cpid" -o args= 2>/dev/null || true)
                     if $ISO_MODE; then
                       echo "$ccmd" | grep -qF "$ROOT" && kill -9 "$cpid" 2>/dev/null || true
                     else
                       echo "$ccmd" | grep -qF "$ROOT" && ! echo "$ccmd" | grep -qF ".worktrees/" && kill -9 "$cpid" 2>/dev/null || true
                     fi
                   done ;;
          frontend) lsof -ti :$FE_PORT 2>/dev/null | xargs kill -9 2>/dev/null || true ;;
          web)     lsof -ti :$WEB_PORT 2>/dev/null | xargs kill -9 2>/dev/null || true ;;
        esac
      done
      sleep 1

      # Rewrite PID file with only alive entries
      echo -ne "$alive_entries" > "$PID_FILE"

      # Archive the dead service's log before overwriting
      for dead_name in $dead_names; do
        local dead_log=""
        case "$dead_name" in
          backend) dead_log="$DEV_DIR/be.log" ;;
          cortex)  dead_log="$DEV_DIR/cortex.log" ;;
          frontend) dead_log="$DEV_DIR/fe.log" ;;
          web)      dead_log="$DEV_DIR/web.log" ;;
        esac
        if [ -n "$dead_log" ] && [ -f "$dead_log" ] && [ -s "$dead_log" ]; then
          local seq_pad
          seq_pad=$(printf "%03d" "$(( $(ls "$ARCHIVE_DIR"/*.log 2>/dev/null | wc -l | tr -d ' ') + 1 ))")
          local base
          base=$(basename "$dead_log" .log)
          mv "$dead_log" "${ARCHIVE_DIR}/${base}.${seq_pad}.log"
        fi
      done

      # Restart only dead services
      for dead_name in $dead_names; do
        case "$dead_name" in
          backend)
            start_detached "backend" "$BE_PORT" "$ROOT/{BACKEND_PROJECT}" "$DEV_DIR/be.log" \
              "env NO_COLOR=1 NODE_ENV=\"${ISO_PROFILE:-}\" pnpm run dev" "resurrected"
            ;;
          cortex)
            start_detached "cortex" "$CORTEX_PORT" "$ROOT/{AI_PROJECT}" "$DEV_DIR/cortex.log" \
              "env NO_COLOR=1 ${ISO_PROFILE:+ENV_FILE=.env.$ISO_PROFILE} {AI_RUN_CMD}" \
              "resurrected"
            ;;
          frontend)
            start_detached "frontend" "$FE_PORT" "$ROOT/{FRONTEND_PROJECT}" "$DEV_DIR/fe.log" \
              "env NO_COLOR=1 FORCE_COLOR=0 npx expo start --web --port \"$FE_PORT\"" "resurrected"
            ;;
          web)
            start_detached "web" "$WEB_PORT" "$ROOT/{WEB_PROJECT}" "$DEV_DIR/web.log" \
              "env NO_COLOR=1 npx next dev --port \"$WEB_PORT\"" "resurrected"
            ;;
        esac
      done

      echo "PARTIAL_RESURRECT=true"
      echo "RESURRECTED=${dead_names}"
      # Fall through to health checks below
      sleep 4

      # Run health checks on all services (same as fresh start)
      # 503 = degraded ({AI_SERVICE_NAME} warming up) = YELLOW; no response = RED
      local be_status="RED"
      local be_http_code
      be_http_code=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$BE_PORT/health" 2>/dev/null || echo "000")
      if [ "$be_http_code" = "200" ]; then
        be_status="GREEN"
        ok "Backend healthy"
      elif [ "$be_http_code" = "503" ]; then
        be_status="YELLOW"
        warn "Backend degraded ({AI_SERVICE_NAME} still warming up — self-heals in ~30s)"
      else
        fail "Backend health check failed (HTTP $be_http_code)"
      fi

      local cortex_status="RED"
      local cortex_http_code
      cortex_http_code=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$CORTEX_PORT/health" 2>/dev/null || echo "000")
      local cortex_pid_check
      cortex_pid_check=$(awk '/^cortex/ {print $2}' "$PID_FILE" | tail -1)
      if [ "$cortex_http_code" = "200" ]; then
        cortex_status="GREEN"
        ok "{AI_SERVICE_NAME} HTTP healthy (port $CORTEX_PORT)"
      elif [ -n "$cortex_pid_check" ] && kill -0 "$cortex_pid_check" 2>/dev/null; then
        cortex_status="YELLOW"
        warn "{AI_SERVICE_NAME} process alive but HTTP not yet responding (port $CORTEX_PORT)"
      else
        cortex_status="RED"
        fail "{AI_SERVICE_NAME} process dead"
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

      echo ""
      echo "---REPORT---"
      echo "BE_STATUS=$be_status"
      echo "CORTEX_STATUS=$cortex_status"
      echo "FE_STATUS=$fe_status"
      echo "WEB_STATUS=$web_status"
      local _seed_dir="${ISO_PROFILE:-local}"
      if [ -f "$ROOT/{BACKEND_PROJECT}/seeding/$_seed_dir/passwords.json" ]; then
        echo "CREDENTIALS_FILE=$ROOT/{BACKEND_PROJECT}/seeding/$_seed_dir/passwords.json"
      else
        echo "CREDENTIALS_FILE=MISSING"
      fi
      echo "ERRORS=none"
      echo "---END---"
      return 0
    else
      # All dead — clean up and proceed with full startup
      rm -f "$PID_FILE"
    fi
  fi

  archive_logs
  check_prereqs

  # ── Step 1: Infrastructure ──
  header "Infrastructure"
  if $ISO_MODE; then
    info "Isolated mode — checking existing containers..."
    local infra_ok=true
    wait_for "{DATABASE} (${PG_PORT:-{DB_PORT}})" "docker exec ${DOCKER_PG_CONTAINER:-{PROJECT_NAME_LOWER}-postgres} pg_isready -U ${DB_USER:-postgres} -d ${DB_NAME:-{PROJECT_NAME_LOWER}}" 10 || infra_ok=false
    wait_for "{QUEUE} (${LS_PORT:-{QUEUE_PORT}})" "curl -sf http://localhost:${LS_PORT:-{QUEUE_PORT}}/_localstack/health" 10 || infra_ok=false
    if ! $infra_ok; then
      fail "Isolated infrastructure not running — run '/dev iso init' first"
      exit 1
    fi
  else
    info "Starting {DATABASE} + {QUEUE}..."
    make -C "$ROOT/{INFRA_PROJECT}" up-local 2>&1 | tail -3

    local infra_ok=true
    wait_for "{DATABASE} ({DB_PORT})" "make -C '$ROOT/{INFRA_PROJECT}' pg-ready-local" 20 || infra_ok=false
    wait_for "{QUEUE} ({QUEUE_PORT})" "make -C '$ROOT/{INFRA_PROJECT}' ls-ready-local" 20 || infra_ok=false

    if ! $infra_ok; then
      fail "Infrastructure not ready — aborting"
      make -C "$ROOT/{INFRA_PROJECT}" ps-local
      exit 1
    fi
  fi

  # ── Step 2: Dependencies (parallel) ──
  header "Dependencies"
  local dep_pids=()

  (cd "$ROOT/{BACKEND_PROJECT}" && pnpm install --prefer-offline 2>&1 | tail -1 && echo "BE_DEPS=ok") &
  dep_pids+=($!)
  (cd "$ROOT/{FRONTEND_PROJECT}" && npm install --legacy-peer-deps 2>&1 | tail -1 && echo "FE_DEPS=ok") &
  dep_pids+=($!)
  (cd "$ROOT/{AI_PROJECT}" && uv sync --group dev 2>&1 | tail -1 && echo "CORTEX_DEPS=ok") &
  dep_pids+=($!)
  (cd "$ROOT/{WEB_PROJECT}" && npm install 2>&1 | tail -1 && echo "WEB_DEPS=ok") &
  dep_pids+=($!)

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

  # ── Step 2b: FE node_modules integrity check ──
  # npm on Node 24 can silently drop transitive deps during install.
  # Spot-check critical files and clean-reinstall if corrupted.
  local fe_nm="$ROOT/{FRONTEND_PROJECT}/node_modules"
  if [ ! -f "$fe_nm/hermes-parser/dist/generated/ParserVisitorKeys.js" ] || \
     [ ! -f "$fe_nm/ms/package.json" ] || \
     [ ! -f "$fe_nm/@babel/runtime/helpers/objectWithoutPropertiesLoose.js" ]; then
    warn "Corrupted FE node_modules detected — clean reinstalling"
    rm -rf "$fe_nm" "$ROOT/{FRONTEND_PROJECT}/.expo"
    (cd "$ROOT/{FRONTEND_PROJECT}" && npm ci --legacy-peer-deps 2>&1 | tail -1)
    ok "FE dependencies reinstalled (clean)"
  fi

  # Reset Watchman to prevent stale file index after any install
  if command -v watchman &>/dev/null; then
    watchman watch-del "$ROOT/{FRONTEND_PROJECT}" &>/dev/null || true
    watchman shutdown-server &>/dev/null || true
  fi

  # ── Step 3: Backend env file ──
  if ! $ISO_MODE; then
    if [ ! -f "$ROOT/{BACKEND_PROJECT}/.env.local" ]; then
      warn "Creating default {BACKEND_PROJECT}/.env.local"
      cat > "$ROOT/{BACKEND_PROJECT}/.env.local" << 'EOF'
DATABASE_URL=postgresql://postgres@localhost:{DB_PORT}/{PROJECT_NAME_LOWER}
JWT_SECRET=dev-secret-change-me
PORT={BACKEND_PORT}
EOF
      warn "Update API keys ({TRANSCRIPTION_API_KEY}) in .env.local for full functionality"
    fi
  fi

  # ── Step 4: Database ──
  if $ISO_MODE; then
    header "Database"
    ok "ISO mode — schema applied during init (skipping migrations)"
  else
    header "Database"
    info "Checking schema + migrations..."

    # Create DB if needed (ignore error if exists)
    make -C "$ROOT/{INFRA_PROJECT}" db-create-local

    # Apply any pending migrations (non-interactive, file-based)
    make -C "$ROOT/{INFRA_PROJECT}" db-migrate-local 2>&1 | tail -2
    ok "Database ready (seeding handled by BE on boot)"
  fi

  # ── Step 5: Start servers ──
  header "Starting servers"

  # Write PID file atomically — truncate first to prevent stale entries
  : > "$PID_FILE"

  # Backend
  start_detached "backend" "$BE_PORT" "$ROOT/{BACKEND_PROJECT}" "$DEV_DIR/be.log" \
    "env NO_COLOR=1 NODE_ENV=\"${ISO_PROFILE:-}\" pnpm run dev"

  # {AI_SERVICE_NAME}
  start_detached "cortex" "$CORTEX_PORT" "$ROOT/{AI_PROJECT}" "$DEV_DIR/cortex.log" \
    "env NO_COLOR=1 ${ISO_PROFILE:+ENV_FILE=.env.$ISO_PROFILE} {AI_RUN_CMD}"

  # Frontend
  start_detached "frontend" "$FE_PORT" "$ROOT/{FRONTEND_PROJECT}" "$DEV_DIR/fe.log" \
    "env NO_COLOR=1 FORCE_COLOR=0 npx expo start --web --port \"$FE_PORT\""

  # Web (marketing site)
  start_detached "web" "$WEB_PORT" "$ROOT/{WEB_PROJECT}" "$DEV_DIR/web.log" \
    "env NO_COLOR=1 npx next dev --port \"$WEB_PORT\""

  # ── Step 6: Health checks ──
  header "Health checks"
  sleep 4

  # 503 = degraded ({AI_SERVICE_NAME} warming up) = YELLOW; no response = RED
  local be_status="RED"
  local be_http_code
  be_http_code=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$BE_PORT/health" 2>/dev/null || echo "000")
  if [ "$be_http_code" = "200" ]; then
    be_status="GREEN"
    ok "Backend healthy"
  elif [ "$be_http_code" = "503" ]; then
    be_status="YELLOW"
    warn "Backend degraded ({AI_SERVICE_NAME} still warming up — self-heals in ~30s)"
  else
    fail "Backend health check failed (HTTP $be_http_code)"
  fi

  local cortex_status="RED"
  local cortex_http_code_fresh
  cortex_http_code_fresh=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:$CORTEX_PORT/health" 2>/dev/null || echo "000")
  local cortex_pid
  cortex_pid=$(awk '/^cortex/ {print $2}' "$PID_FILE" | tail -1)
  if [ "$cortex_http_code_fresh" = "200" ]; then
    cortex_status="GREEN"
    ok "{AI_SERVICE_NAME} HTTP healthy (port $CORTEX_PORT)"
  elif [ -n "$cortex_pid" ] && kill -0 "$cortex_pid" 2>/dev/null; then
    cortex_status="YELLOW"
    warn "{AI_SERVICE_NAME} process alive but HTTP not yet responding (port $CORTEX_PORT)"
  else
    cortex_status="RED"
    fail "{AI_SERVICE_NAME} process dead"
  fi

  local fe_status="RED"
  if curl -sf http://localhost:$FE_PORT -o /dev/null 2>/dev/null; then
    fe_status="GREEN"
    ok "Frontend responding"
  else
    # Frontend can take longer
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

  # ── Step 7: Scan logs for errors ──
  local errors=""
  for svc in be cortex fe web; do
    local logfile="$DEV_DIR/${svc}.log"
    if [ -f "$logfile" ]; then
      local svc_errors
      svc_errors=$(grep -a -iE '(ERR|Error|FATAL|Exception|Traceback|ECONNREFUSED|EADDRINUSE|ModuleNotFoundError|Cannot find module)' "$logfile" 2>/dev/null | grep -v "^Binary file" | grep -viE '(WARN.*swallowing|Warning:|DeprecationWarning|ExperimentalWarning)' | head -3 || true)
      if [ -n "$svc_errors" ]; then
        errors="${errors}\n  ${svc}: $(echo "$svc_errors" | head -1)"
      fi
    fi
  done

  # ── Step 8: Output JSON-ish report for Claude to parse ──
  echo ""
  echo "---REPORT---"
  echo "BE_STATUS=$be_status"
  echo "CORTEX_STATUS=$cortex_status"
  echo "FE_STATUS=$fe_status"
  echo "WEB_STATUS=$web_status"

  local _seed_dir="${ISO_PROFILE:-local}"
  if [ -f "$ROOT/{BACKEND_PROJECT}/seeding/$_seed_dir/passwords.json" ]; then
    echo "CREDENTIALS_FILE=$ROOT/{BACKEND_PROJECT}/seeding/$_seed_dir/passwords.json"
  else
    echo "CREDENTIALS_FILE=MISSING"
  fi

  if [ -n "$errors" ]; then
    echo -e "ERRORS=$errors"
  else
    echo "ERRORS=none"
  fi
  echo "---END---"
}

# ─── MODE: DROP ──────────────────────────────────────────────────

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
  if make -C "$ROOT/{INFRA_PROJECT}" nuke-local 2>&1 | tail -5; then
    ok "Docker containers nuked"
  else
    fail "Failed to nuke Docker containers"
    echo "---REPORT---"
    echo "WERE_RUNNING=$were_running"
    echo "NUKE_RESULT=fail"
    echo "INFRA_RESULT=fail"
    echo "---END---"
    return 1
  fi

  # Step 3: Bring fresh containers up
  header "Rebuilding infrastructure"
  make -C "$ROOT/{INFRA_PROJECT}" up-local 2>&1 | tail -3

  local infra_ok=true
  wait_for "{DATABASE} ({DB_PORT})" "make -C '$ROOT/{INFRA_PROJECT}' pg-ready-local" 30 || infra_ok=false
  wait_for "{QUEUE} ({QUEUE_PORT})" "make -C '$ROOT/{INFRA_PROJECT}' ls-ready-local" 30 || infra_ok=false

  if ! $infra_ok; then
    fail "Infrastructure not ready after rebuild"
    echo "---REPORT---"
    echo "WERE_RUNNING=$were_running"
    echo "NUKE_RESULT=success"
    echo "INFRA_RESULT=fail"
    echo "---END---"
    return 1
  fi

  ok "Infrastructure rebuilt"

  # Step 4: Recreate database + migrate
  header "Database"
  make -C "$ROOT/{INFRA_PROJECT}" db-create-local
  make -C "$ROOT/{INFRA_PROJECT}" db-migrate-local 2>&1 | tail -2
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

# ─── MODE: FRESH ──────────────────────────────────────────────────

cmd_fresh() {
  ensure_dirs

  # Step 1: Kill servers unconditionally
  header "Killing servers"
  cmd_kill
  echo ""

  # Step 2: Nuke Docker containers + volumes
  header "Nuking Docker containers"
  if make -C "$ROOT/{INFRA_PROJECT}" nuke-local 2>&1 | tail -5; then
    ok "Docker containers nuked"
  else
    fail "Failed to nuke Docker containers"
    echo "---REPORT---"
    echo "NUKE_RESULT=fail"
    echo "INFRA_RESULT=fail"
    echo "---END---"
    return 1
  fi

  # Step 3: Bring fresh containers up
  header "Rebuilding infrastructure"
  make -C "$ROOT/{INFRA_PROJECT}" up-local 2>&1 | tail -3

  local infra_ok=true
  wait_for "{DATABASE} ({DB_PORT})" "make -C '$ROOT/{INFRA_PROJECT}' pg-ready-local" 30 || infra_ok=false
  wait_for "{QUEUE} ({QUEUE_PORT})" "make -C '$ROOT/{INFRA_PROJECT}' ls-ready-local" 30 || infra_ok=false

  if ! $infra_ok; then
    fail "Infrastructure not ready after rebuild"
    echo "---REPORT---"
    echo "NUKE_RESULT=success"
    echo "INFRA_RESULT=fail"
    echo "---END---"
    return 1
  fi

  ok "Infrastructure rebuilt"

  # Step 4: Recreate database + migrate
  header "Database"
  make -C "$ROOT/{INFRA_PROJECT}" db-create-local
  make -C "$ROOT/{INFRA_PROJECT}" db-migrate-local 2>&1 | tail -2
  ok "Database migrated"

  # Step 5: Always start servers
  header "Starting servers"
  cmd_up
}

# ─── MODE: STATUS ────────────────────────────────────────────────

cmd_status() {
  header "Dev server status"

  if [ ! -f "$PID_FILE" ] || [ ! -s "$PID_FILE" ]; then
    echo "NO_SERVERS=true"
    return 0
  fi

  echo "---REPORT---"
  # Deduplicate PID entries — keep last entry per service name (handles stale duplicates)
  # Reverse lines (portable, no tac on macOS) so newest entry wins, awk deduplicates by name
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
  local be_health="fail" gql_health="fail" fe_health="fail" web_health="fail"
  local health_json=""
  health_json=$(curl -s --max-time 15 http://localhost:$BE_PORT/health 2>/dev/null || echo "")
  if [ -n "$health_json" ]; then
    be_health="ok"
  fi
  curl -sf http://localhost:$BE_PORT/graphql -H 'Content-Type: application/json' -d '{"query":"{ __typename }"}' &>/dev/null && gql_health="ok"
  curl -sf http://localhost:$FE_PORT -o /dev/null &>/dev/null && fe_health="ok"
  curl -sf http://localhost:$WEB_PORT -o /dev/null &>/dev/null && web_health="ok"

  echo "BE_HEALTH=$be_health"
  echo "GQL_HEALTH=$gql_health"
  echo "FE_HEALTH=$fe_health"
  echo "WEB_HEALTH=$web_health"

  # Seed progress (extracted from health JSON)
  if [ -n "$health_json" ] && command -v python3 &>/dev/null; then
    local seed_line
    seed_line=$(python3 -c "
import json, sys
try:
    d = json.loads(sys.argv[1])
    sp = d.get('seedProgress', {})
    status = sp.get('status', 'unknown')
    inserted = sp.get('totalInserted', 0)
    analyzed = sp.get('totalAnalyzed', 0)
    expected = sp.get('totalExpected', 0)
    remaining = sp.get('totalRemaining', 0)
    subjects = sp.get('patients', [])
    parts = []
    for p in subjects:
        mark = '✓' if p.get('done') else f\"{p.get('analyzed',0)}/{p.get('total',0)}\"
        parts.append(f\"{p.get('patient','?')}={mark}\")
    detail = ', '.join(parts) if parts else ''
    print(f'SEED_STATUS={status}|SEED_INSERTED={inserted}|SEED_ANALYZED={analyzed}|SEED_EXPECTED={expected}|SEED_REMAINING={remaining}|SEED_DETAIL={detail}')
except Exception:
    print('SEED_STATUS=unknown')
" "$health_json" 2>/dev/null || echo "SEED_STATUS=unknown")
    echo "$seed_line"
  fi

  local _seed_dir="${ISO_PROFILE:-local}"
  if [ -f "$ROOT/{BACKEND_PROJECT}/seeding/$_seed_dir/passwords.json" ]; then
    echo "CREDENTIALS_FILE=$ROOT/{BACKEND_PROJECT}/seeding/$_seed_dir/passwords.json"
  else
    echo "CREDENTIALS_FILE=MISSING"
  fi
  echo "---END---"
}

# ─── MODE: LOG ────────────────────────────────────────────────────

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

  local services=()
  case "$service" in
    be|backend)  services=(be) ;;
    cortex) services=(cortex) ;;
    fe|frontend) services=(fe) ;;
    web)         services=(web) ;;
    all|*)       services=(be cortex fe web) ;;
  esac

  for svc in "${services[@]}"; do
    local logfile="$DEV_DIR/${svc}.log"
    local label
    case $svc in
      be)    label="Backend" ;;
      cortex) label="{AI_SERVICE_NAME}" ;;
      fe)    label="Frontend" ;;
      web)   label="Web" ;;
    esac
    echo "━━━ $label ($logfile) ━━━ [last $lines lines]"
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
      be)    label="Backend" ;;
      cortex) label="{AI_SERVICE_NAME}" ;;
      fe)    label="Frontend" ;;
      web)   label="Web" ;;
    esac
    if [ -f "$logfile" ]; then
      local count
      count=$(grep -ciE '(ERR|Error|FATAL|Exception|Traceback)' "$logfile" 2>/dev/null || true)
      count=${count:-0}
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

# ─── MODE: LOG-CLEAR ─────────────────────────────────────────────

cmd_log_clear() {
  local cleared=0
  for f in "$DEV_DIR"/be.log "$DEV_DIR"/cortex.log "$DEV_DIR"/fe.log "$DEV_DIR"/web.log; do
    [ -f "$f" ] && rm -f "$f" && cleared=$((cleared + 1))
  done
  local archived
  archived=$(find "$ARCHIVE_DIR" -name "*.log" 2>/dev/null | wc -l | tr -d ' ')
  rm -f "$ARCHIVE_DIR"/*.log 2>/dev/null || true

  echo "CLEARED_CURRENT=$cleared"
  echo "CLEARED_ARCHIVE=$archived"
}

# ─── MODE: EXPORT ──────────────────────────────────────────────

cmd_export() {
  header "Exporting seed data"

  local env_file="$ROOT/{BACKEND_PROJECT}/.env.local"
  local seeding_name
  seeding_name=$(grep '^SEEDING_NAME=' "$env_file" 2>/dev/null | cut -d= -f2 | tr -d '"'"'" | head -1)
  seeding_name="${seeding_name:-local}"

  local export_dir="$ROOT/{BACKEND_PROJECT}/seeding/$seeding_name"

  SEEDING_NAME="$seeding_name" cd "$ROOT/{BACKEND_PROJECT}" && SEEDING_NAME="$seeding_name" {DB_EXPORT_CMD}
}

# ─── MODE: PROMOTE-DEMO ──────────────────────────────────────────────

cmd_promote_demo() {
  header "Promoting local → demo dataset"
  local src="$ROOT/{BACKEND_PROJECT}/seeding/local"
  local dest="$ROOT/{BACKEND_PROJECT}/seeding/demo"

  # Copy all JSON files (overwrites demo — developer reviews passwords.json after)
  cp "$src"/*.json "$dest/"

  # Copy audio WAVs
  mkdir -p "$dest/audio"
  if [ -n "$(ls -A "$src/audio"/*.wav 2>/dev/null)" ]; then
    cp "$src/audio"/*.wav "$dest/audio/"
  fi

  local json_count audio_count
  json_count=$(ls -1 "$dest"/*.json 2>/dev/null | wc -l | tr -d ' ')
  audio_count=$(ls -1 "$dest/audio"/*.wav 2>/dev/null | wc -l | tr -d ' ')

  ok "Promoted: $json_count JSON files, $audio_count WAV files"
  warn "IMPORTANT: Review seeding/demo/passwords.json before committing."
  warn "Ensure the magic-login demo account is present."
}

# ─── MAIN ─────────────────────────────────────────────────────────

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
  export)          cmd_export ;;
  promote-demo)    cmd_promote_demo ;;
  *)
    echo "Usage: $0 {up|kill|restart|drop|fresh|status|log|clear-logs|export|promote-demo}"
    exit 1
    ;;
esac
