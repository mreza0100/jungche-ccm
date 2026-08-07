#!/usr/bin/env bash
# cc-name-sync.sh — the single writer of chat WINDOW names, both engines. The tmux window name
# is the fleet's DNS record: chat.sh resolves codex chats by it, the VS Code tab renders it
# (cx servers title as '⬢ <window> · <pane title>'), and a human picking a pane reads it.
#
#   codex  window <- the thread's name in the codex state store (`threads` table), else the
#          thread_name in ~/.codex/session_index.jsonl (what `codex resume <name>` accepts),
#          else its first real prompt. The pane is matched to its thread by BIRTH TIME, not
#          newest-in-cwd: codex records the thread within seconds of the process starting, so
#          the user thread in the pane's cwd born closest to the pane's own start is that pane's
#          thread. Newest-in-cwd mislabeled every chat when two shared a directory.
#   claude window <- the 🔖 label scraped off the pane's own statusline (the /rename name),
#          anchored on 🔖 + an account badge exactly as chat.sh resolves labels. The tab needs
#          no help here — Claude Code sets it by OSC — the window name is for the DNS.
#
# Sessions are never renamed: session name == socket name is the chat discriminator everywhere
# in the fleet (anything else on a socket is a squatter and is left alone).
#
# Fired by: the cc-name-sync.path systemd user unit (a codex rename appends to
# session_index.jsonl and lands on the tab in under a second), the cc-name-sync.timer
# (claude /rename drift), and cc-ls in the background on every picker run.
#
#   cc-name-sync.sh              converge every live chat window
#   cc-name-sync.sh --dry-run    print what would change, touch nothing
set -uo pipefail

# CC_FLEET_HOME — this bundle's own directory, resolved THROUGH symlinks, because install.sh
# links this script into ~/.claude/bin and $BASH_SOURCE is then the link. Plain `readlink`
# (never -f) because macOS ships BSD readlink.
_ccfs="${BASH_SOURCE[0]}"; while [ -L "$_ccfs" ]; do _ccfd="$(cd -P "$(dirname "$_ccfs")" && pwd)"; _ccfs="$(readlink "$_ccfs")"; case "$_ccfs" in /*) ;; *) _ccfs="$_ccfd/$_ccfs" ;; esac; done
CC_FLEET_HOME="${CC_FLEET_HOME:-$(cd -P "$(dirname "$_ccfs")" && pwd)}"
. "$CC_FLEET_HOME/cc-portable.sh"   # GNU/BSD seam


DRY=0
case "${1:-}" in
  --dry-run) DRY=1 ;;
  "") ;;
  *) echo "usage: cc-name-sync.sh [--dry-run]" >&2; exit 2 ;;
esac

CODEX_DIR="${CODEX_HOME:-$HOME/.codex}"
TMUXDIR="${TMUX_TMPDIR:-/tmp/tmux-$(id -u)}"
LOCK="${CC_NAME_SYNC_LOCK:-${XDG_RUNTIME_DIR:-/tmp}/cc-name-sync.lock}"
MAXLEN=24            # window names feed a tab title; longer is noise there
BIRTH_SLOP=120       # a rollout may be stamped a moment before its pane's clock — allow this much

# One sync at a time; a concurrent run's work is this run's work. cc_trylock rather than flock(1),
# which macOS does not ship — see the note on cc_trylock for how the flock form silently exited
# this whole script on every Mac run.
cc_trylock "$LOCK" || exit 0
trap 'cc_unlock "$LOCK"' EXIT

# ── the codex name index: id -> LAST thread_name (the index is append-only; a rename appends) ──
declare -A CXNM
if [ -r "$CODEX_DIR/session_index.jsonl" ]; then
  while IFS=$'\t' read -r i n; do [ -n "$i" ] && CXNM[$i]="$n"; done < <(
    jq -r 'select(.id != null) | [.id, .thread_name // ""] | @tsv' "$CODEX_DIR/session_index.jsonl" 2>/dev/null)
fi

# ── the codex thread store: ~/.codex/state_<N>.sqlite, table `threads` ────────────────────────
# Codex moved a thread's identity into sqlite: a thread running in `paginated` history mode keeps
# its history in the store and may leave NO rollout file behind at all. Matching a pane through
# rollout files therefore missed every new chat, and the miss was silent — cx_rollout_for fell
# back to "newest rollout in this cwd" and stamped a long-dead sibling's name on the window, so a
# codex rename never reached the tab. The table answers the same question exactly, in one query:
# id, name, cwd, created_at. The filename is versioned, so take the highest; a codex too old to
# have the store leaves CXDB empty and the rollout scan below serves alone.
CXDB="$(ls -1 "$CODEX_DIR"/state_*.sqlite 2>/dev/null \
  | sed 's/.*state_\([0-9]*\)\.sqlite$/\1 &/' | sort -rn | head -1 | cut -d' ' -f2-)"

# cx_db_name <cwd> <start-epoch> — the display name of the thread that pane owns, "" if the store
# has none that close (an elder or resumed thread) and the rollout scan should try. Read-only and
# WAL-aware: never `immutable`, which would hide every row still sitting in the -wal.
# Name precedence: the store's own `name` (what codex writes on a rename), then the index — which
# still holds the renames of threads that predate that column — then the thread's first prompt.
cx_db_name() {
  [ -n "$CXDB" ] || return 0
  local cwd start id nm fum ttl row
  cwd=${1//\'/\'\'}; start="$2"
  row="$(sqlite3 -readonly -separator "$(printf '\037')" "file:$CXDB?mode=ro" \
    "select id, coalesce(name,''),
            replace(replace(coalesce(first_user_message,''),char(9),' '),char(10),' '),
            replace(replace(coalesce(title,''),char(9),' '),char(10),' ')
       from threads
      where thread_source='user' and archived=0 and cwd='$cwd'
        and abs(created_at - $start) <= $BIRTH_SLOP
      order by abs(created_at - $start) limit 1;" 2>/dev/null)" || return 0
  [ -n "$row" ] || return 0
  # \037 (unit separator), not a tab: tab is IFS whitespace, so `read` would fold the run of
  # separators an empty column produces and shift every later field one to the left.
  IFS="$(printf '\037')" read -r id nm fum ttl <<EOF
$row
EOF
  [ -n "${nm:-}" ] && { printf '%s' "$nm"; return 0; }
  nm="${CXNM[${id:-}]:-}"
  [ -n "$nm" ] && { printf '%s' "$nm"; return 0; }
  printf '%s' "${fum:-${ttl:-}}"
}

# cx_rollout_for <cwd> <start-epoch> — the pane's own rollout: among user-thread rollouts whose
# meta cwd matches, the one whose birth (meta timestamp, UTC) lies closest to the pane's start
# and not before start-BIRTH_SLOP. None that close (an elder thread, clock trouble) → the newest
# match, the pre-disambiguation behavior.
cx_rollout_for() {
  local cwd="$1" start="$2" f meta ts birth d best="" bestd=999999999 newest=""
  while IFS= read -r f; do
    meta="$(head -1 "$f" 2>/dev/null)" || continue
    case "$meta" in *'"thread_source":"user"'*) ;; *) continue ;; esac
    case "$meta" in *"\"cwd\":\"$cwd\""*) ;; *) continue ;; esac
    [ -n "$newest" ] || newest="$f"
    # the birth is payload.timestamp. Codex wraps the meta record in an envelope carrying its own
    # WRITE time (seen 35 minutes past the thread's birth, and rewritten as the thread runs), so
    # reading the first "timestamp" in the line dated the thread by when it was last written.
    # A record with no payload leaves the string untouched and the first timestamp still serves.
    ts="${meta#*\"payload\":\{}"; ts="${ts#*\"timestamp\":\"}"; ts="${ts%%\"*}"
    birth="$(cc_epoch "$ts")"; [ -n "$birth" ] || continue
    (( birth >= start - BIRTH_SLOP )) || continue
    d=$(( birth - start )); (( d < 0 )) && d=$(( -d ))
    (( d < bestd )) && { bestd=$d; best="$f"; }
  done < <(cc_find_newest "$CODEX_DIR/sessions" -name 'rollout-*.jsonl' | head -60)
  [ -n "$best" ] || best="$newest"
  [ -n "$best" ] && printf '%s' "$best"
}

# cx_display_name <rollout> — the index thread_name for any id in the thread's lineage (own id,
# session_id, parent_thread_id — a resumed thread carries a fresh id but the name sticks to an
# ancestor), else the first real user prompt, else "".
cx_display_name() {
  local rl="$1" meta id k x nm
  local -a ids=()
  id="${rl##*/}"; id="${id%.jsonl}"; id="${id#rollout-????-??-??T??-??-??-}"
  ids+=("$id")
  meta="$(head -1 "$rl" 2>/dev/null)"
  for k in session_id parent_thread_id; do
    while IFS= read -r x; do [ -n "$x" ] && ids+=("$x"); done < <(
      printf '%s' "$meta" | grep -o "\"$k\":\"[0-9a-f-]*\"" | cut -d'"' -f4)
  done
  for id in "${ids[@]}"; do
    nm="${CXNM[$id]:-}"
    [ -n "$nm" ] && { printf '%s' "$nm"; return 0; }
  done
  nm="$(grep -m1 '"type":"user_message"' "$rl" 2>/dev/null | jq -r '.payload.message // empty' 2>/dev/null | tr '\t\n' '  ')"
  printf '%s' "${nm:0:60}"
}

# rename <sockpath> <window-id> <old> <new> <engine> — one converged window, reported.
rename() {
  local sockpath="$1" win="$2" old="$3" new="$4" engine="$5"
  new="${new%"${new##*[![:space:]]}"}"   # the 24-char cut can land on a space — trim it
  [ -n "$new" ] || return 0
  [ "$old" = "$new" ] && return 0
  if [ "$DRY" = 1 ]; then
    printf 'would rename %s %s %s: %s -> %s\n' "$engine" "${sockpath##*/}" "$win" "$old" "$new"
    return 0
  fi
  if [ "$engine" = codex ]; then
    # elder cx servers born before the title override get their options converged here too
    tmux -S "$sockpath" set -g set-titles on \; set -g set-titles-string '⬢ #{window_name} · #{pane_title}' \; \
      setw -t "$win" automatic-rename off \; rename-window -t "$win" -- "$new" 2>/dev/null
  else
    tmux -S "$sockpath" setw -t "$win" automatic-rename off \; rename-window -t "$win" -- "$new" 2>/dev/null
  fi
  printf 'renamed %s %s %s: %s -> %s\n' "$engine" "${sockpath##*/}" "$win" "$old" "$new"
}

for sockpath in "$TMUXDIR"/cc-* "$TMUXDIR"/cx-*; do
  [ -S "$sockpath" ] || continue
  sock="${sockpath##*/}"
  declare -A WINLBL=() WINSKIP=() WINNAME=() WINSOCK=()
  while IFS=$'\t' read -r sess win wname pane ppid pcwd pcmd; do
    [ "$sess" = "$sock" ] || continue          # squatters on a chat's socket are not chats
    [ "$pcmd" = "tmux" ] && continue           # a viewport mirrors an inner statusline — skip
    case "$sock" in
      cx-*)
        # The pane process's START TIME is what matches it to the rollout file born beside it,
        # so two codex chats in one directory keep their own names. cc_pstart reads it from
        # /proc's directory mtime on Linux and from `ps -o lstart=` on macOS — the liveness
        # check rides along, since a dead pid yields no start time on either platform (the old
        # `[ -d /proc/$ppid ]` gate was Linux's spelling of exactly that check).
        start="$(cc_pstart "$ppid")"; [ -n "$start" ] || continue
        # the store first — it knows the threads that write no rollout file; the rollout scan
        # then covers the elder threads recorded before the store existed.
        nm="$(cx_db_name "$pcwd" "$start")"
        if [ -z "$nm" ]; then
          rl="$(cx_rollout_for "$pcwd" "$start")" || true
          [ -n "$rl" ] && [ -r "$rl" ] || continue
          nm="$(cx_display_name "$rl")"
        fi
        [ -n "$nm" ] || continue
        rename "$sockpath" "$win" "$wname" "${nm:0:$MAXLEN}" codex
        ;;
      cc-*)
        # the 🔖+badge statusline anchor, exactly as chat.sh resolves labels (badge, not 🌿:
        # a labeled chat outside a repo renders no 🌿 and would go unaddressable). The badge set
        # is whatever the STATUSLINE can emit, which outlives the fleet's account roster: a chat
        # born under a retired account still renders its old badge until it exits, and dropping
        # that badge here would silently stop syncing its name.
        lbl="$(tmux -S "$sockpath" capture-pane -t "$pane" -p -J 2>/dev/null \
               | grep -F '🔖' | grep -E '🥇|🥈|🥉|🍀' | tail -1 \
               | sed 's/.*🔖 *//; s/ *│.*//; s/[[:space:]]*$//' || true)"
        [ -n "$lbl" ] || continue
        lbl="${lbl:0:$MAXLEN}"
        # two differently-labeled chats can share a window (/chat:branch siblings) — a window
        # name cannot carry both, so a conflicted window is left alone
        if [ -n "${WINLBL[$win]:-}" ] && [ "${WINLBL[$win]}" != "$lbl" ]; then
          WINSKIP[$win]=1; continue
        fi
        WINLBL[$win]="$lbl"; WINNAME[$win]="$wname"; WINSOCK[$win]="$sockpath"
        ;;
    esac
  done < <(tmux -S "$sockpath" list-panes -a -F \
    $'#{session_name}\t#{window_id}\t#{window_name}\t#{pane_id}\t#{pane_pid}\t#{pane_current_path}\t#{pane_current_command}' 2>/dev/null)
  for win in "${!WINLBL[@]}"; do
    [ -n "${WINSKIP[$win]:-}" ] && continue
    rename "${WINSOCK[$win]}" "$win" "${WINNAME[$win]}" "${WINLBL[$win]}" claude
  done
done
exit 0
