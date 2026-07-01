#!/usr/bin/env bash
# cc-reap.sh — list or reap the cc-* tmux "socket graveyard".
#
# Closing a VS Code terminal tab DETACHES the tmux client but leaves the chat's own tmux server
# (and its `claude` node process, ~0.5-1 GB RSS) alive forever. Crashed servers (the tmux SIGSEGV
# SPOF) leave 0-RAM stale socket files. Over weeks this piles up — 100+ dead sockets, several GB held.
#
# This walks every cc-* socket under /tmp/tmux-$UID and classifies each:
#   KEEP  = attached (a live tab is showing it) OR this very chat's own socket ($TMUX).
#   KILL  = unattached live orphan (kill-server frees its RAM) OR dead socket file (just rm it).
# It NEVER touches an attached chat, your own socket, or any non-cc socket (dev / vscode /
# default / cctest*). Default run is a DRY-RUN report with per-socket RAM; --kill performs the reap.
#
#   cc-reap.sh            # dry run: classify + show RAM, change nothing
#   cc-reap.sh --kill     # reap: kill-server unattached orphans, rm stale socket files
#
# RAM caveat: per-socket RAM is summed RSS of the server's process subtree. RSS over-counts shared
# node runtime pages across chats (~1.5x), so the column is an upper bound — read the TRUE reclaim
# from the `free` before/after delta the reap prints, not the summed column.
set -u

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
SOCKS=("$TMUXDIR"/cc-*)
shopt -u nullglob
[ ${#SOCKS[@]} -eq 0 ] && { echo "cc-reap: no cc-* sockets under $TMUXDIR"; exit 0; }

(( DO_KILL )) && { echo "RAM before:"; free -m | awk 'NR==1||/Mem:/'; echo; }
printf '%-38s %-5s %8s  %s\n' "SOCKET" "STATE" "RAM(MB)" "LABEL [cwd]"

for path in "${SOCKS[@]}"; do
  sock="${path##*/}"
  line="$(tmux -L "$sock" ls -F "$FMT" 2>/dev/null | head -1)"
  if [ -z "$line" ]; then            # dead/stale socket file (server gone)
    kill_dead_n=$((kill_dead_n+1))
    printf '%-38s %-5s %8s  %s\n' "$sock" "dead" "0" "(stale socket file)"
    if (( DO_KILL )); then rm -f "$path"; fi
    continue
  fi
  IFS=$'\t' read -r att pid label cwd <<<"$line"
  [ -z "${label:-}" ] && label="(unnamed)"
  ram_kb="$(_subtree_kb "${pid:-}")"

  if [ "$sock" = "$MYSOCK" ] || [ "${att:-0}" = "1" ]; then   # KEEP
    keep_n=$((keep_n+1)); keep_kb=$((keep_kb+ram_kb))
    mark="keep"; [ "$sock" = "$MYSOCK" ] && mark="self"
    printf '%-38s %-5s %8s  %s\n' "$sock" "$mark" "$((ram_kb/1024))" "$label [$cwd]"
    continue
  fi

  # KILL: unattached live orphan
  kill_live_n=$((kill_live_n+1)); kill_kb=$((kill_kb+ram_kb))
  if (( DO_KILL )); then
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

echo
echo "KEEP: $keep_n live chats (~$((keep_kb/1024)) MB)   KILL: $kill_live_n orphans (~$((kill_kb/1024)) MB summed RSS) + $kill_dead_n dead files"
if (( DO_KILL )); then
  echo "reaped ~$((freed_kb/1024)) MB summed RSS"
  echo; echo "RAM after:"; free -m | awk 'NR==1||/Mem:/'
else
  echo "dry run — nothing changed. Run 'cc-reap.sh --kill' to reap."
fi
