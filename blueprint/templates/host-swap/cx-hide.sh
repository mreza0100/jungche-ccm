#!/usr/bin/env bash
# cx-hide.sh [--exit] — the CODEX twin of cc-hide.sh. Add the CURRENT Codex chat to cc-ls's hide
# list (run from inside the chat via the global /bb codex prompt). Non-destructive: the rollout is
# kept; it just stops showing in cc-ls. A Codex chat has no ~/.claude transcript and no cc-sid
# breadcrumb — its identity is the USER-thread rollout the live codex process holds open, so we
# find that from /proc the same way cc-fleet's _cx_scan does, and hide by its rollout id (cc-ls's
# codex rows are keyed on exactly that id).
#   --exit  after hiding, gracefully close the chat: type /quit into its tmux pane, then kill that
#           PANE so nothing idle is left behind (detached, ~1.5s later, so this turn finishes
#           first). Never kill-server — a manual split would put a sibling on the same cx-* socket;
#           killing the last pane ends the server anyway, so a standalone chat still fully closes.
# Undo: cc-ls --hidden then ⌃X, or remove the line from ~/.claude/.cc-ls-hidden.
set -u
do_exit=0; [ "${1:-}" = "--exit" ] && do_exit=1
hf="$HOME/.claude/.cc-ls-hidden"
# CC_FLEET_HOME — this bundle's OWN directory, resolved THROUGH symlinks. install.sh links these
# scripts into ~/.claude/bin, so $BASH_SOURCE is the link, not the file: taking its dirname
# straight would hunt for siblings in the link's directory. Plain `readlink` (never -f) because
# macOS ships BSD readlink, which has no -f.
_ccfs="${BASH_SOURCE[0]}"; while [ -L "$_ccfs" ]; do _ccfd="$(cd -P "$(dirname "$_ccfs")" && pwd)"; _ccfs="$(readlink "$_ccfs")"; case "$_ccfs" in /*) ;; *) _ccfs="$_ccfd/$_ccfs" ;; esac; done
CC_FLEET_HOME="${CC_FLEET_HOME:-$(cd -P "$(dirname "$_ccfs")" && pwd)}"
CC_DB="$CC_FLEET_HOME/cc-db.sh"   # fleet state store; falls back to $hf on its own
. "$CC_FLEET_HOME/cc-portable.sh" # GNU/BSD seam — cc_pane_of, cc_penv, cc_detach

# RECOVER $TMUX FROM ANCESTRY. Codex spawns its tool shell WITHOUT passing tmux context through,
# so a /bb run from inside a Codex chat measures TMUX=unset and this script used to abort with
# "not in tmux — nothing to hide" even though the chat plainly sits in a cx-* pane. Identity stays
# the SCRIPT's job: it is derived from our OWN process chain, never accepted from a caller flag.
# Same repair chat.sh makes for the same reason (tmux_from_ancestry there) — keep the two in step,
# since a codex tool shell breaks BOTH the same way.
#
# ASK TMUX, NOT THE KERNEL. This used to walk /proc/<pid>/environ up the ancestry looking for a
# TMUX= line, which no macOS can answer: SIP has restricted `ps -E` to the calling process for a
# decade, so every Mac read "no ancestor is in tmux" and /bb died on a Codex chat with the wrong
# diagnosis. cc_pane_of asks the tmux servers which pids they own and walks the same ancestry —
# one answer, both platforms, and truer besides: a pane whose command re-execs keeps its pane
# while the original env may name a server that no longer holds it.
if [ -z "${TMUX:-}" ] || [ -z "${TMUX_PANE:-}" ]; then
  _hit="$(cc_pane_of "$PPID")"
  if [ -n "$_hit" ]; then
    _sk="${_hit%%	*}"; _pi="${_hit##*	}"
    # TRUST IT ONLY IF IT IS A CODEX SOCKET. The walk climbs OUR process chain, and when this
    # script is invoked from another chat's shell that chain leads to THAT chat's tmux — adopting
    # it would hide, and with --exit close, the wrong chat. A cx-* name is the proof it is ours;
    # anything else is someone else's server and we fall through to cwd resolution below.
    case "$_sk" in
      cx-*)
        # Only the socket PATH component of $TMUX is ever read back (`${TMUX%%,*}`), so the
        # server-pid and session-id fields are filled with zeros rather than invented.
        [ -z "${TMUX:-}" ]      && { TMUX="${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)/$_sk,0,0"; export TMUX; }
        [ -z "${TMUX_PANE:-}" ] && { TMUX_PANE="$_pi"; export TMUX_PANE; }
        ;;
    esac
  fi
  unset _hit _sk _pi
fi

# CWD IS THE IDENTITY OF LAST RESORT. Two assumptions this script was built on have both
# expired in current codex (v0.146): it strips $TMUX/$TMUX_PANE from the tool shell AND from
# every ancestor the walk above can see, and it no longer holds its rollout file open — it
# appends and closes, so the /proc-fd scan below finds nothing. Either alone breaks /bb; together
# they made it unfixable by env repair. What codex does still guarantee is that a chat runs in one
# working directory and records that directory in its rollout's meta line, so $PWD identifies both
# the chat's rollout AND its tmux pane without reading one environment variable or one file
# descriptor. Same fact cc-fleet's _cx_scan needs — keep the two in step.
_cx_sock_by_cwd() {                    # the cx-* socket whose pane sits in $PWD → "sockpath<TAB>pane"
  # Ask tmux to do the matching with -f, and return ONE field. Every separator-based approach
  # failed here: this box's awk reads -F'\t' as the literal characters backslash-t, and tmux
  # SANITISES a real tab in a -F format to "_", so "path<TAB>pane" came back as "path_%0" and no
  # comparison could ever match. A filter plus a single-field format needs no separator at all.
  local s pane
  for s in "${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)"/cx-*; do
    [ -S "$s" ] || continue
    pane="$(tmux -S "$s" list-panes -a -f "#{==:#{pane_current_path},$PWD}" -F '#{pane_id}' 2>/dev/null | head -1)"
    [ -n "$pane" ] && { printf '%s\t%s\n' "$s" "$pane"; return 0; }
  done
  return 1
}
_cx_rollout_by_cwd() {                 # newest USER-thread rollout recorded in $PWD
  local f meta
  while IFS= read -r f; do
    meta="$(head -1 "$f" 2>/dev/null)" || continue
    case "$meta" in *'"thread_source":"user"'*) : ;; *) continue ;; esac
    case "$meta" in *"\"cwd\":\"$PWD\""*) printf '%s\n' "$f"; return 0 ;; esac
  done < <(cc_find_newest "$HOME/.codex/sessions" -name 'rollout-*.jsonl' | head -60)
  return 1
}

# Must be inside a Codex cx-* tmux pane (that is what /bb runs in). A Claude chat uses cc-hide.sh.
if [ -z "${TMUX:-}" ]; then
  _row="$(_cx_sock_by_cwd || true)"
  if [ -n "$_row" ]; then
    sockpath="${_row%%	*}"; pane="${_row#*	}"; sock="${sockpath##*/}"
    echo "cx-hide: \$TMUX unset (codex strips it) — resolved this chat by cwd → ${sock} ${pane}" >&2
  else
    echo "cx-hide: not in tmux and no cx-* pane is running in $PWD — nothing to hide"; exit 1
  fi
else
  sockpath="${TMUX%%,*}"; sock="${sockpath##*/}"; pane="${TMUX_PANE:-}"
  if [ -z "$pane" ]; then                       # TMUX survived but TMUX_PANE did not
    _row="$(_cx_sock_by_cwd || true)"; [ -n "$_row" ] && pane="${_row#*	}"
  fi
fi
case "$sock" in
  cx-*) : ;;
  *) echo "cx-hide: this is not a Codex (cx-*) chat — for Claude use /bb (cc-hide.sh)"; exit 1 ;;
esac

# Identify THIS chat's rollout id. The pane's foreground pid leads up to the codex process; that
# process holds its OWN user-thread rollout at its lowest fd (subagent threads are thread_source
# != "user" and are skipped). Mirror _cx_scan: map each live codex's user rollout onto the codex
# pid AND its ≤4 ancestors (binary → node wrapper → shell → pane), then look up THIS pane's pid.
pane_pid="$(tmux -S "$sockpath" list-panes -t "$pane" -F '#{pane_pid}' 2>/dev/null | head -1)"
[ -n "$pane_pid" ] || pane_pid=0   # no pane pid → the /proc map simply misses; the cwd fallback covers it

declare -A CXRL
while IFS= read -r p; do
  [ -n "$p" ] || continue
  crl=""
  # fd order — the binary opens its own rollout before any subagent thread's. cc_pfiles yields
  # that order from /proc/<pid>/fd on Linux and from lsof on macOS, which lists fds ascending too.
  while IFS= read -r tgt; do
    case "$tgt" in "$HOME"/.codex/sessions/*/rollout-*.jsonl) : ;; *) continue ;; esac
    head -1 "$tgt" 2>/dev/null | grep -q '"thread_source":"user"' || continue
    crl="$tgt"; break
  done <<EOF
$(cc_pfiles "$p")
EOF
  [ -n "$crl" ] || continue
  pp="$p"
  for _ in 1 2 3 4; do
    [ -z "${CXRL[$pp]:-}" ] && CXRL[$pp]="$crl"
    # cc_ppid, not /proc/<pid>/stat: that file's comm field can contain spaces and parentheses,
    # so its ppid is only safe to read positionally AFTER the last ")" — and macOS has no /proc
    # at all. `ps -o ppid=` is both portable and immune to the comm-parsing trap.
    pp="$(cc_ppid "$pp")"
    case "$pp" in ''|0|1) break ;; esac
  done
done < <(pgrep -x codex 2>/dev/null)

rl="${CXRL[$pane_pid]:-}"
# current codex closes the rollout after each append, so the fd scan above finds nothing — fall
# back to the newest user thread recorded in this cwd. Only truly give up if BOTH miss.
[ -n "$rl" ] || rl="$(_cx_rollout_by_cwd || true)"
[ -n "$rl" ] || { echo "cx-hide: no codex rollout found for this pane or for $PWD — is this a Codex chat?"; exit 1; }
# id = the trailing uuid of rollout-YYYY-MM-DDThh-mm-ss-<id>.jsonl (same key cc-ls hides on)
u="$(basename -- "$rl" .jsonl | sed -E 's/^rollout-[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}-[0-9]{2}-[0-9]{2}-//')"
case "$u" in
  *-*-*-*-*) : ;;
  *) echo "cx-hide: could not parse a rollout id from $rl"; exit 1 ;;
esac

mkdir -p "$(dirname "$hf")"
if bash "$CC_DB" hidden-has "$u" 2>/dev/null; then
  echo "cx-hide: already hidden ($u)"
else
  bash "$CC_DB" hidden-add "$u"   # transactional — no flock, no lost update
  echo "cx-hide: hidden $u — gone from cc-ls (cc-ls --hidden to manage · ⌃X to restore)"
fi

if [ "$do_exit" = 1 ]; then
  if [ -z "$pane" ]; then
    echo "cx-hide: in tmux but \$TMUX_PANE unset — type /quit yourself"
    exit 0
  fi
  # Close THIS chat by killing ONLY its own PANE — never kill-server. A cx-* server normally holds
  # one Codex pane, but a manual split puts a SIBLING in the same server, and kill-server would drop
  # it too (the same bug cc-hide.sh already fixes this way). Killing the last pane ends the server on
  # its own, so a standalone chat still fully closes. Detached + delayed so this turn finishes first;
  # a graceful /quit lets codex flush its rollout, polled until the pane closes itself (up to 20s),
  # then kill-pane is the backstop.
  echo "cx-hide: closing this Codex chat (auto /quit, then kill-pane $pane)…"
  $(cc_detach) env SOCKPATH="$sockpath" PANE="$pane" bash -c '
    sleep 1.5
    tmux -S "$SOCKPATH" send-keys -t "$PANE" -l -- /quit
    tmux -S "$SOCKPATH" send-keys -t "$PANE" Enter
    n=0
    while [ "$n" -lt 20 ] && tmux -S "$SOCKPATH" list-panes -a -F "#{pane_id}" 2>/dev/null | grep -qx -- "$PANE"; do
      sleep 1; n=$((n+1))
    done
    tmux -S "$SOCKPATH" kill-pane -t "$PANE" 2>/dev/null
  ' >/dev/null 2>&1 &
fi
