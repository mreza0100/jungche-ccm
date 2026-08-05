#!/usr/bin/env bash
# cc-reap.sh — list or reap the cc-* tmux "socket graveyard" (see docs/cc-fleet.md § Reaping).
#
# Closing a VS Code terminal tab DETACHES the tmux client but leaves the chat's own tmux server
# (and its `claude` node process, ~0.5-1 GB RSS) alive forever. Crashed servers (the tmux SIGSEGV
# SPOF) leave 0-RAM stale socket files. Over weeks this piles up — 100+ dead sockets, several GB held.
#
# This walks every cc-* and cx-* (Codex) socket under /tmp/tmux-$UID and classifies each:
#   KEEP  = attached (a live tab is showing it) OR this very chat's own socket ($TMUX)
#           OR a cc-new-* detached teammate (headless BY DESIGN — its parent's /bb reaps it)
#           OR a session Claude itself reports BUSY (deliberately detached, still working).
#   KILL  = unattached idle orphan (kill-server frees its RAM) OR dead socket file (just rm it).
# It NEVER touches an attached chat, your own socket, or sockets outside the cc-*/cx-* sweep
# domain (dev / vscode / default / cctest*). When the agents query fails, --kill skips
# every breadcrumbed socket (busy-unknown, fail closed). Codex (cx-*) has no breadcrumb and no
# busy signal — a deliberately detached working Codex chat needs a tab or dashboard window to
# survive a sweep. Default run is a DRY-RUN report with per-socket RAM; --kill performs the reap.
#
#   cc-reap.sh            # dry run: classify + show RAM, change nothing
#   cc-reap.sh --kill     # reap: kill-server unattached orphans, rm stale socket files
#
# RAM caveat: per-socket RAM is summed RSS of the server's process subtree. RSS over-counts shared
# node runtime pages across chats (~1.5x), so the column is an upper bound — read the TRUE reclaim
# from the memory before/after delta the reap prints, not the summed column.
set -u

# CC_FLEET_HOME — this bundle's own directory, resolved THROUGH symlinks, because install.sh
# links this script into ~/.claude/bin and $BASH_SOURCE is then the link. Plain `readlink`
# (never -f) because macOS ships BSD readlink.
_ccfs="${BASH_SOURCE[0]}"; while [ -L "$_ccfs" ]; do _ccfd="$(cd -P "$(dirname "$_ccfs")" && pwd)"; _ccfs="$(readlink "$_ccfs")"; case "$_ccfs" in /*) ;; *) _ccfs="$_ccfd/$_ccfs" ;; esac; done
CC_FLEET_HOME="${CC_FLEET_HOME:-$(cd -P "$(dirname "$_ccfs")" && pwd)}"
. "$CC_FLEET_HOME/cc-portable.sh"   # GNU/BSD seam — cc_timeout, cc_mtime0, cc_mem

DO_KILL=0
case "${1:-}" in
  --kill) DO_KILL=1 ;;
  ""|--list|-l) DO_KILL=0 ;;
  -h|--help) sed -n '2,20p' "$0"; exit 0 ;;
  *) echo "cc-reap: unknown arg '$1' (use --kill or --list)"; exit 2 ;;
esac

TMUXDIR="/tmp/tmux-$(id -u)"
MYSOCK=""
[ -n "${TMUX:-}" ] && { MYSOCK="${TMUX%%,*}"; MYSOCK="${MYSOCK##*/}"; }

# live-work guard: sessions Claude itself reports BUSY are kept even when detached — a
# deliberately detached chat grinding a long task must survive a sweep. Sockets map to
# sessions via the /tmp/cc-sid breadcrumb (socket → transcript path → uuid).
# FAIL CLOSED: each account is queried and validated separately (one account's error output
# must not poison the others' jq), and any failed query flips AGENTS_OK=0 — with the busy set
# unknown, --kill SKIPS every breadcrumbed socket instead of treating silence as idle.
CLAUDE_BIN="$(command -v claude || echo "$HOME/.local/bin/claude")"
BUSY_IDS=""; AGENTS_OK=1
for _cfg in "" "$HOME/.cc/2"; do
  # A subshell sets (or unsets) the account for the probe rather than an `env` prefix: cc_timeout
  # is a shell FUNCTION, and `env` execs a binary — it cannot see one. The subshell scopes the
  # change just as tightly and works with either.
  if [ -n "$_cfg" ]; then _out="$(export CLAUDE_CONFIG_DIR="$_cfg"; cc_timeout 20 "$CLAUDE_BIN" agents --json 2>/dev/null)"
  else _out="$(unset CLAUDE_CONFIG_DIR; cc_timeout 20 "$CLAUDE_BIN" agents --json 2>/dev/null)"; fi
  if printf '%s' "$_out" | jq -e 'type=="array"' >/dev/null 2>&1; then
    _ids="$(printf '%s' "$_out" | jq -r '.[] | select(.status=="busy") | .sessionId' 2>/dev/null)"
    [ -n "$_ids" ] && BUSY_IDS="${BUSY_IDS}${BUSY_IDS:+
}${_ids}"
  else
    AGENTS_OK=0
  fi
done
BUSY_IDS="$(printf '%s\n' "$BUSY_IDS" | sort -u)"
[ "$AGENTS_OK" = 0 ] && echo "cc-reap: WARNING — an agents query failed; busy state unknown, --kill skips breadcrumbed sockets"

NOW="$(date +%s)"
# kill-time busy re-verify (defense against a stale one-time BUSY_IDS snapshot): a session
# idle at snapshot time can start work while detached (a queued --then steer, a /loop turn) —
# and a running turn writes its transcript continuously, so a transcript touched within the
# last BUSY_RECENT_S seconds is treated as busy even if it missed the snapshot.
BUSY_RECENT_S="${CC_REAP_BUSY_RECENT_S:-60}"

PS_SNAP="$(ps -e -o pid=,ppid=,rss=)"   # one snapshot; subtree sums read from it
_subtree_kb() {                          # $1 = root pid -> KB (0 if unknown)
  [ -z "${1:-}" ] && { echo 0; return; }
  awk -v root="$1" '
    { rss[$1]=$3; kids[$2]=kids[$2]" "$1 }
    END { n=0; stack[n++]=root; t=0
          while (n>0) { p=stack[--n]; if (seen[p]++) continue; t+=rss[p]+0
            m=split(kids[p],c," "); for(i=1;i<=m;i++) stack[n++]=c[i] }
          print t }' <<<"$PS_SNAP"
}

FMT=$'#{?session_attached,1,0}\t#{pid}\t#{s/^[^ ]* //:pane_title}\t#{b:pane_current_path}'
keep_n=0 kill_live_n=0 kill_dead_n=0 keep_kb=0 kill_kb=0 freed_kb=0

shopt -s nullglob
SOCKS=("$TMUXDIR"/cc-* "$TMUXDIR"/cx-*)   # cx-* = Codex chats, same per-chat-server pattern.
# Tradeoff: codex writes no breadcrumb and has no busy signal, so a deliberately detached
# still-working Codex chat looks like any orphan — keep a tab (or dashboard window) on it.
shopt -u nullglob
[ ${#SOCKS[@]} -eq 0 ] && echo "cc-reap: no cc-*/cx-* sockets under $TMUXDIR"   # vsct sweep below still runs

(( DO_KILL )) && { echo "RAM before:"; cc_mem; echo; }
printf '%-38s %-5s %8s  %s\n' "SOCKET" "STATE" "RAM(MB)" "LABEL [cwd]"

for path in "${SOCKS[@]}"; do
  sock="${path##*/}"
  line="$(tmux -L "$sock" ls -F "$FMT" 2>/dev/null | head -1)"
  if [ -z "$line" ]; then            # empty tmux ls — server gone OR mid-startup (a forking
    # tmux binds the socket BEFORE its session exists, and that reads identically to dead).
    # Only treat (and count) a socket file older than 1h as dead — mirrors cc-fleet.zsh's
    # corpse sweep ("never touch a server mid-startup whose socket exists but isn't
    # answering yet"). A young empty socket is reported and left alone in BOTH modes.
    if [ -n "$(find "$path" -mmin +60 2>/dev/null)" ]; then
      kill_dead_n=$((kill_dead_n+1))
      if (( DO_KILL )); then
        rm -f "$path"; printf '%-38s %-5s %8s  %s\n' "$sock" "dead" "0" "(stale socket file — rm'd)"
      else
        printf '%-38s %-5s %8s  %s\n' "$sock" "dead" "0" "(stale socket file)"
      fi
    else
      printf '%-38s %-5s %8s  %s\n' "$sock" "SKIP" "0" "(empty socket <1h — mid-startup? left alone)"
    fi
    continue
  fi
  IFS=$'\t' read -r att pid label cwd <<<"$line"
  [ -z "${label:-}" ] && label="(unnamed)"
  ram_kb="$(_subtree_kb "${pid:-}")"

  case "$sock" in cc-new-*)                 # detached teammate (/chat:new --detach): headless by
    keep_n=$((keep_n+1)); keep_kb=$((keep_kb+ram_kb))        # design — its parent's /bb reaps it
    printf '%-38s %-5s %8s  %s\n' "$sock" "mate" "$((ram_kb/1024))" "$label [$cwd]"
    continue ;;
  esac
  # Resolve EVERY uuid this socket hosts — the socket crumb AND each pane crumb
  # (/tmp/cc-sid/<sock>.%<pane>). A /chat:branch or /chat:new split puts two claudes on one
  # socket, and the socket crumb is last-writer-wins, so the busy pane may not own it —
  # reading only the socket crumb could reap a busy worker beside an idle parent. has_crumb
  # records whether ANY crumb file exists (the fail-closed signal below).
  buuids=""; has_crumb=0
  for cf in "/tmp/cc-sid/$sock" "/tmp/cc-sid/$sock".%*; do
    [ -e "$cf" ] || continue
    has_crumb=1
    cu="$(basename -- "$(cat "$cf" 2>/dev/null)" .jsonl 2>/dev/null)"
    case "$cu" in *-*-*-*-*) buuids="${buuids}${buuids:+ }$cu" ;; esac
  done
  # KEEP if ANY resolved uuid is busy in the snapshot, OR its transcript was written within
  # the last BUSY_RECENT_S seconds (turned busy AFTER the one-time snapshot).
  busy=0; busy_why="busy"
  for cu in $buuids; do
    if printf '%s\n' "$BUSY_IDS" | grep -qxF -- "$cu"; then busy=1; busy_why="busy"; break; fi
    tf="$(ls "$HOME"/.claude/projects/*/"$cu".jsonl "$HOME"/.cc/[0-9]*/projects/*/"$cu".jsonl 2>/dev/null | head -1)"
    [ -n "$tf" ] || continue
    if [ "$(( NOW - $(cc_mtime0 "$tf") ))" -lt "$BUSY_RECENT_S" ]; then busy=1; busy_why="active<${BUSY_RECENT_S}s"; break; fi
  done
  if [ "$busy" = 1 ]; then   # KEEP: still working
    keep_n=$((keep_n+1)); keep_kb=$((keep_kb+ram_kb))
    printf '%-38s %-5s %8s  %s\n' "$sock" "$busy_why" "$((ram_kb/1024))" "$label [$cwd]"
    continue
  fi

  if [ "$sock" = "$MYSOCK" ] || [ "${att:-0}" = "1" ]; then   # KEEP
    keep_n=$((keep_n+1)); keep_kb=$((keep_kb+ram_kb))
    mark="keep"; [ "$sock" = "$MYSOCK" ] && mark="self"
    printf '%-38s %-5s %8s  %s\n' "$sock" "$mark" "$((ram_kb/1024))" "$label [$cwd]"
    continue
  fi

  # KILL: unattached live orphan
  kill_live_n=$((kill_live_n+1)); kill_kb=$((kill_kb+ram_kb))
  if (( DO_KILL )); then
    # fail closed: busy set unknown + this socket has a breadcrumb (a real chat) → never kill
    if [ "$AGENTS_OK" = 0 ] && [ "$has_crumb" = 1 ]; then
      printf '%-38s %-5s %8s  %s\n' "$sock" "SKIP" "$((ram_kb/1024))" "busy-unknown (agents query failed) — $label"
      continue
    fi
    # fail closed: a live cc-* server with NO breadcrumb is busy-UNKNOWN — its busy state
    # can't be read (statusline never rendered, or /tmp/cc-sid was cleared), so a detached
    # chat grinding a task there would look like an idle orphan. Skip it. cx-* (Codex) has
    # no breadcrumb BY DESIGN — the tab-or-die tradeoff — so this guard is cc-* only.
    case "$sock" in
      cc-*) if [ "$has_crumb" = 0 ]; then
              printf '%-38s %-5s %8s  %s\n' "$sock" "SKIP" "$((ram_kb/1024))" "busy-unknown (no breadcrumb) — $label"
              continue
            fi ;;
    esac
    # defense-in-depth: re-check attached at kill time (fleet spawns chats mid-sweep)
    if tmux -L "$sock" ls -F '#{?session_attached,1,0}' 2>/dev/null | grep -qx 1; then
      printf '%-38s %-5s %8s  %s\n' "$sock" "SKIP" "$((ram_kb/1024))" "now attached — $label"
      continue
    fi
    tmux -L "$sock" kill-server 2>/dev/null
    rm -f "$path"                    # kill-server may leave the socket file
    freed_kb=$((freed_kb+ram_kb))
    printf '%-38s %-5s %8s  %s\n' "$sock" "KILL" "$((ram_kb/1024))" "$label [$cwd]"
  else
    printf '%-38s %-5s %8s  %s\n' "$sock" "orph" "$((ram_kb/1024))" "$label [$cwd]"
  fi
done

# ── vsct plain-terminal bunkers (docs/cc-fleet.md § vsct) ──
# attached or recently-active sessions are KEPT; detached AND idle >7 days are reaped with --kill.
VSCT_MAX_IDLE=$((7*24*3600))   # NOW is set once at the top of the run
while IFS=$'\t' read -r vname vatt vact; do
  [ -n "$vname" ] || continue
  if [ "$vatt" = "1" ] || [ $((NOW - vact)) -lt "$VSCT_MAX_IDLE" ]; then
    keep_n=$((keep_n+1))
    printf '%-38s %-5s %8s  %s\n' "vsct:$vname" "keep" "-" "(plain terminal)"
  elif (( DO_KILL )); then
    kill_live_n=$((kill_live_n+1))
    tmux -L vsct kill-session -t "=$vname" 2>/dev/null && \
      printf '%-38s %-5s %8s  %s\n' "vsct:$vname" "KILL" "-" "(idle $(( (NOW-vact)/86400 ))d)"
  else
    kill_live_n=$((kill_live_n+1))
    printf '%-38s %-5s %8s  %s\n' "vsct:$vname" "orph" "-" "(idle $(( (NOW-vact)/86400 ))d)"
  fi
done < <(tmux -L vsct ls -F $'#{session_name}\t#{?session_attached,1,0}\t#{session_activity}' 2>/dev/null)

echo
echo "KEEP: $keep_n live chats (~$((keep_kb/1024)) MB)   KILL: $kill_live_n orphans (~$((kill_kb/1024)) MB summed RSS) + $kill_dead_n dead files"
if (( DO_KILL )); then
  echo "reaped ~$((freed_kb/1024)) MB summed RSS"
  echo; echo "RAM after:"; cc_mem
else
  echo "dry run — nothing changed. Run 'cc-reap.sh --kill' to reap."
fi
