#!/usr/bin/env sh
# cc-portable.sh — the GNU/BSD seam. Every place this bundle asks the OS a question
# whose answer differs between Linux and macOS is asked HERE, once, and nowhere else.
#
# Sourced, never executed: `. "$PFM_HOME/cc-portable.sh"`. Written in POSIX sh
# with no arrays and no [[ ]], because the callers are bash scripts AND cc-fleet.zsh,
# which is sourced into the founder's interactive shell — a bashism here would surface
# as a syntax error at every new terminal.
#
# THE RULE FOR EVERY FUNCTION BELOW: try the Linux form first, fall back to the BSD
# form, and return a documented empty/zero value when neither can answer. Linux is
# the primary runtime and must not pay for the portability; macOS must not silently
# get a WRONG answer where it can get none. A function that cannot answer says so by
# returning empty — callers branch on that, they never receive a plausible-looking
# fabrication (a 0 mtime read as "epoch 1970" would sort a live chat to the bottom
# of the picker instead of admitting it could not stat the file).
#
# Guard: sourcing twice is free.
[ -n "${CC_PORTABLE_LOADED:-}" ] && return 0 2>/dev/null
CC_PORTABLE_LOADED=1

# cc_os — "linux" | "darwin" | the raw uname otherwise. Computed once; every branch
# below dispatches on tool availability rather than on this string, because a GNU
# coreutils install on a Mac should take the GNU path. This is for the rare case
# where the KERNEL, not the toolchain, is the thing that differs (/proc).
CC_OS="$(uname -s 2>/dev/null | tr 'A-Z' 'a-z')"

# ── file facts ───────────────────────────────────────────────────────────────
# cc_mtime FILE — last-modified epoch seconds, "" if it cannot be read.
# cc_size  FILE — size in bytes, "" if it cannot be read.
# GNU stat and BSD stat share a name and share nothing else: -c '%Y' vs -f '%m'.
cc_mtime() { stat -c %Y "$1" 2>/dev/null || stat -f %m "$1" 2>/dev/null || true; }
cc_size()  { stat -c %s "$1" 2>/dev/null || stat -f %z "$1" 2>/dev/null || true; }

# cc_mtime0 / cc_size0 — the same, but "" collapses to 0 for arithmetic call sites
# that genuinely want "treat unreadable as empty/ancient". Named separately so the
# choice is visible at the call site instead of hidden in a `|| echo 0`.
cc_mtime0() { _v="$(cc_mtime "$1")"; printf '%s' "${_v:-0}"; }
cc_size0()  { _v="$(cc_size  "$1")"; printf '%s' "${_v:-0}"; }

# ── time ─────────────────────────────────────────────────────────────────────
# cc_epoch STAMP — epoch seconds for "@1785832621", "1785832621", or an ISO-8601
# timestamp ("2026-08-02T00:00:11.123Z"). "" when it cannot be parsed.
#
# GNU `date -d` accepts all three. BSD date accepts none of them: it needs -r for an
# epoch and -j -f FORMAT for a stamp, and its -f parser stops at the first character
# the format does not describe — so fractional seconds and the Z must be trimmed
# BEFORE it sees them, or it errors on input GNU handles fine.
cc_epoch() {
  _s="${1:-}"; [ -n "$_s" ] || return 0
  case "$_s" in
    @*) _e="${_s#@}"; case "$_e" in *[!0-9]*) ;; *) printf '%s' "$_e"; return 0 ;; esac ;;
    *[!0-9]*) ;;                          # not a bare integer — fall through to date
    *) printf '%s' "$_s"; return 0 ;;     # already epoch seconds
  esac
  date -d "$_s" +%s 2>/dev/null && return 0
  # BSD: strip the fraction and any trailing zone designator, then parse the ISO core.
  _t="${_s#@}"; _t="${_t%%.*}"; _t="${_t%%Z*}"; _t="${_t%%+*}"
  date -u -j -f '%Y-%m-%dT%H:%M:%S' "$_t" +%s 2>/dev/null && return 0
  date -u -j -f '%Y-%m-%d %H:%M:%S' "$_t" +%s 2>/dev/null || true
}

# cc_timeout SECS CMD... — run CMD with a wall-clock limit. `timeout` is GNU coreutils
# and is NOT on a stock Mac (Homebrew coreutils installs it, and also gtimeout). With
# neither, RUN THE COMMAND ANYWAY rather than failing: every caller in this bundle uses
# a timeout as a guard against a hung `claude agents --json`, so refusing to run at all
# turns a rare hang into a permanent outage. Degrade to slow, never to broken.
cc_timeout() {
  _s="$1"; shift
  if command -v timeout >/dev/null 2>&1; then timeout "$_s" "$@"
  elif command -v gtimeout >/dev/null 2>&1; then gtimeout "$_s" "$@"
  else "$@"; fi
}

# ── processes ────────────────────────────────────────────────────────────────
# cc_ppid PID — parent pid, "" if the process is gone.
# /proc/PID/stat's 4th field is the ppid, but field 2 (comm) can contain spaces and
# parentheses, so it is never safe to cut by whitespace; `ps -o ppid=` is both portable
# and correct, and is what this uses on every platform.
cc_ppid() { ps -o ppid= -p "$1" 2>/dev/null | tr -d ' 	' || true; }

# cc_pstart PID — process start time, epoch seconds. "" if unknown.
# Linux: the /proc/PID directory's mtime IS the process start, and reading it costs one
# stat. macOS has no /proc; `ps -o lstart=` prints a human date that BSD date can parse
# back with the matching format. Used to match a tmux pane to the rollout file born
# alongside it, so a same-directory sibling chat cannot steal its name.
cc_pstart() {
  if [ -d "/proc/$1" ]; then cc_mtime "/proc/$1"; return 0; fi
  _l="$(ps -o lstart= -p "$1" 2>/dev/null)" || return 0
  [ -n "$_l" ] || return 0
  date -j -f '%a %b %e %T %Y' "$_l" +%s 2>/dev/null || true
}

# cc_children_args PID — the argv of every DIRECT child, one per line.
# GNU ps spells this `--ppid`; BSD ps has no such selector, so the pids come from
# pgrep -P (portable) and the argv from a plain -p lookup.
cc_children_args() {
  ps -o args= --ppid "$1" 2>/dev/null && return 0
  _k="$(pgrep -P "$1" 2>/dev/null | tr '\n' ',')"; _k="${_k%,}"
  [ -n "$_k" ] || return 0
  ps -o args= -p "$_k" 2>/dev/null || true
}

# NOTE for everything below that walks a list: never `for x in $(cmd)`. This file is sourced by
# bash scripts AND by cc-fleet.zsh, and zsh does not word-split an unquoted command substitution
# the way bash does — the same loop would iterate once over one giant string there and produce a
# silently empty result. `while IFS= read -r` behaves identically in both shells.

# cc_penv PID VAR — the value of VAR in ANOTHER process's environment. "" when unknown.
#
# THIS IS THE ONE QUESTION macOS CANNOT ANSWER. Linux exposes /proc/PID/environ; macOS
# has restricted `ps -E` to the calling process since SIP, so a Mac gets "" here — always,
# not sometimes. Callers MUST treat "" as "ask another way" and never as "the variable is
# unset", because those mean opposite things: on a Mac, EVERY process looks like it has no
# environment. Where the question is really "which tmux pane is this process in", use
# cc_pane_of instead — it asks tmux, works on both platforms, and is the better question.
cc_penv() {
  [ -r "/proc/$1/environ" ] || return 0
  tr '\0' '\n' < "/proc/$1/environ" 2>/dev/null | sed -n "s/^$2=//p" | head -1
}

# cc_pfiles PID — the regular files this process has open, in fd order, one path per line.
# Linux reads /proc/PID/fd (numeric fd order matters: the codex binary opens its OWN
# rollout first, which is what makes the first hit the right one). macOS uses lsof, whose
# -F field output lists fds in the same ascending order.
cc_pfiles() {
  if [ -d "/proc/$1/fd" ]; then
    ls -1 "/proc/$1/fd" 2>/dev/null | sort -n | while IFS= read -r _fd; do
      readlink "/proc/$1/fd/$_fd" 2>/dev/null
    done
    return 0
  fi
  command -v lsof >/dev/null 2>&1 || return 0
  # -Fn prints one n<path> line per open file; VREG keeps it to regular files, matching
  # what the /proc branch yields once directories and sockets are skipped by the caller.
  lsof -nP -p "$1" -a -d '^cwd,^rtd,^txt' -Fn 2>/dev/null | sed -n 's/^n\//\//p'
}

# cc_pane_of PID — "<socket>	<pane_id>" for the tmux pane this process (or any of its
# ancestors) runs in; "" when it is not under tmux at all.
#
# The portable replacement for reading TMUX / TMUX_PANE out of /proc/PID/environ. It asks
# TMUX which pids it owns instead of asking the kernel what a process was told — which is
# also more truthful: a pane whose command re-execs keeps the pane, and the env of the
# original process may name a socket that no longer holds it. Walks up to 12 ancestors so
# a chat several shells deep still resolves; 12 is far past any real nesting and stops a
# pid-reuse cycle from spinning.
#
# SPACE-DELIMITED, never a tab. tmux hands a control character in a format string back
# as "_" unless the caller's environment carries a UTF-8 locale or merely defines $TMUX.
# This function runs in the one place that has NEITHER — the tool shell a chat engine
# spawns from a scrubbed environment (codex forwards only its core variables, and LANG
# is not among them), reached precisely because $TMUX is absent. The split then never
# happened, no pane pid ever matched, ancestry recovery answered "not in tmux", and
# every codex-origin message went out UNSIGNED. pane_pid is digits and pane_id is %N,
# so a space cannot be ambiguous.
cc_pane_of() {
  _p="$1"; _n=0
  [ -n "$_p" ] || return 0
  _dir="${TMUX_TMPDIR:-/tmp}/tmux-$(id -u 2>/dev/null)"
  while [ -n "$_p" ] && [ "$_p" != 0 ] && [ "$_p" != 1 ] && [ "$_n" -lt 12 ]; do
    for _s in "$_dir"/*; do
      [ -S "$_s" ] || continue
      _sk="${_s##*/}"
      tmux -L "$_sk" list-panes -a -F '#{pane_pid} #{pane_id}' 2>/dev/null | while read -r _pp _pi; do
        [ "$_pp" = "$_p" ] && { printf '%s\t%s\n' "$_sk" "$_pi"; break; }
      done | grep . && return 0
    done
    _p="$(cc_ppid "$_p")"; _n=$((_n + 1))
  done
  return 0
}

# cc_detach — the command PREFIX that survives its parent, for the helpers this bundle
# backgrounds to outlive the very pane they are about to kill (`/bb`'s exit-and-clean,
# `/swap`'s reboot, `/chat:inject --then`'s follow-up turn). Use it as a prefix so the
# call site keeps its own redirections and its own trailing `&`:
#
#   $(cc_detach) env FOO=1 bash -c '…' >>"$log" 2>&1 &
#
# `setsid` is util-linux and is NOT on macOS — a Mac ran none of those helpers, so a
# chat asked to close stayed open with no error to explain it. `nohup` is POSIX, exists
# everywhere, and buys the property that actually matters here: immunity to the SIGHUP
# that arrives when the pane dies. It does not create a new SESSION the way setsid does,
# so the helper stays in the caller's process group — which is why the call sites also
# background it and close stdin. Prefer setsid where it exists; nohup is the floor, and
# there is no third case, since a box with neither has no POSIX shell either.
cc_detach() {
  if command -v setsid >/dev/null 2>&1; then printf 'setsid'
  else printf 'nohup'; fi
}

# cc_session_live UUID — non-empty when a chat with that session id is RUNNING somewhere: the
# pid on Linux, the tmux socket holding it otherwise. Empty means "no evidence it is live", which
# is NOT the same as "proven dead" — read the coverage note below before treating it as one.
#
# Guards a real footgun: appending a turn to a transcript whose process is alive is a dangling
# side-branch that process never reads, so the message vanishes behind a success banner.
#
# Two probes, because they cover different chats and only one is portable:
#   1. the environment scan — every process whose env carries CLAUDE_CODE_SESSION_ID=<uuid>.
#      Exact, and the only probe that sees a pane-less claude.ai background job. LINUX ONLY:
#      it reads /proc/<pid>/environ, and macOS exposes no process's environment but its own.
#   2. the statusline breadcrumbs — /tmp/cc-sid/<socket> holds the transcript path of the chat
#      on that socket, so a crumb naming this uuid whose tmux server still answers proves a live
#      pane chat. Portable, and the only probe a Mac gets.
# Coverage gap, stated rather than papered over: on macOS a background job with no tmux pane is
# invisible to both probes here. The caller's own `claude agents --json` query catches the
# daemon-hosted ones; a claude.ai job is the residue, and it is why the caller must keep asking
# every question it has rather than treating this one answer as final.
cc_session_live() {
  _u="${1:-}"; [ -n "$_u" ] || return 0
  if [ -d /proc ]; then
    for _e in /proc/[0-9]*/environ; do
      if grep -qzF "CLAUDE_CODE_SESSION_ID=$_u" "$_e" 2>/dev/null; then
        _e="${_e#/proc/}"; printf '%s' "${_e%/environ}"; return 0
      fi
    done
  fi
  for _c in /tmp/cc-sid/*; do
    [ -f "$_c" ] || continue
    case "${_c##*/}" in *.%*) continue ;; esac      # pane-keyed crumb — the socket one speaks for it
    _tp="$(cat "$_c" 2>/dev/null)"; [ -n "$_tp" ] || continue
    case "${_tp##*/}" in "$_u.jsonl") ;; *) continue ;; esac
    _sk="${_c##*/}"
    tmux -L "$_sk" list-panes -a >/dev/null 2>&1 && { printf '%s' "$_sk"; return 0; }
  done
  return 0
}

# ── host ─────────────────────────────────────────────────────────────────────
# cc_listening PORT — 0 when something is listening on 127.0.0.1:PORT, 1 otherwise.
# `ss` is iproute2 (Linux only); macOS answers with lsof, and netstat is the last resort
# on a box with neither. Unknown counts as NOT listening — claiming a dead service is up
# sends the operator to debug the wrong end.
cc_listening() {
  _p="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn 2>/dev/null | grep -q "127\.0\.0\.1:$_p" && return 0
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP@127.0.0.1:"$_p" -sTCP:LISTEN >/dev/null 2>&1 && return 0
  fi
  if command -v netstat >/dev/null 2>&1; then
    netstat -an 2>/dev/null | grep -q "127\.0\.0\.1\.$_p .*LISTEN" && return 0
  fi
  return 1
}

# cc_mem — one short line of RAM pressure, for a reap report's before/after. `free` is
# procps (Linux only); macOS derives the same shape from vm_stat's page counts. Prints
# nothing at all where neither exists, so a caller can print it unconditionally.
cc_mem() {
  if command -v free >/dev/null 2>&1; then
    free -m | awk 'NR==1 || /Mem:/'
    return 0
  fi
  command -v vm_stat >/dev/null 2>&1 || return 0
  vm_stat 2>/dev/null | awk -v ps="$(sysctl -n hw.pagesize 2>/dev/null || echo 4096)" '
    /Pages free/       { free  = $3 }
    /Pages active/     { act   = $3 }
    /Pages inactive/   { inact = $3 }
    /Pages wired down/ { wired = $4 }
    /Pages occupied by compressor/ { comp = $5 }
    END {
      mb = ps / 1048576
      gsub(/\./, "", free); gsub(/\./, "", act); gsub(/\./, "", inact)
      gsub(/\./, "", wired); gsub(/\./, "", comp)
      used  = (act + wired + comp) * mb
      avail = (free + inact) * mb
      printf "%18s %8s %8s\n", "", "used", "avail"
      printf "%-18s %8d %8d\n", "Mem(MB):", used, avail
    }'
}

# ── locking ──────────────────────────────────────────────────────────────────
# cc_trylock LOCKPATH — take a non-blocking single-instance lock. 0 = acquired, 1 = someone
# live already holds it. Release with cc_unlock from an EXIT trap.
#
# `flock(1)` is util-linux and is NOT on macOS, which made this the bundle's most expensive
# portability bug: cc-name-sync.sh opened with
#     exec 9>"$LOCK" 2>/dev/null && flock -n 9 || exit 0
# and on a Mac the missing binary failed the `&&`, so `|| exit 0` fired and the ENTIRE script
# exited before doing any work — silently, with status 0, on every single run. Codex windows
# simply never got their names and nothing anywhere said why.
#
# mkdir is atomic on POSIX, which makes it the portable lock primitive (the same reasoning, and
# the same owner-file design, as chat.sh's _inject_lock_acquire — keep the two in step). The
# owner file carries the pid so a lock left behind by a crash is STOLEN rather than honoured
# forever; a lock whose owner is still alive is never touched. One mechanism on both platforms
# deliberately: two lock implementations that cannot see each other is how you get two holders.
#
# THE OWNER PID IS READ INLINE, NEVER THROUGH `$(…)`. A command substitution runs in a subshell
# of its own, so any "what pid am I" helper called that way reports the pid of a process that is
# already gone by the time it returns — the owner file then always names a dead pid, every
# contender judges the lock stale, and the lock silently grants itself to everyone. `$$` is the
# top-level shell and is the right answer for every real caller here (they lock at script top
# level); BASHPID additionally makes it right inside a bash subshell, which is what the fixtures
# use to hold the lock. zsh leaves BASHPID unset and falls back to $$, correct at top level.
cc_trylock() {
  _lk="$1.d"; _me="$$"; [ -n "${BASHPID:-}" ] && _me="$BASHPID"
  mkdir -p "$(dirname "$_lk")" 2>/dev/null || true
  if mkdir "$_lk" 2>/dev/null; then printf '%s' "$_me" > "$_lk/owner" 2>/dev/null; return 0; fi
  _op="$(cat "$_lk/owner" 2>/dev/null || true)"
  # A live owner keeps it. An owner file that is missing or names a dead pid is a crash residue.
  if [ -n "$_op" ] && kill -0 "$_op" 2>/dev/null; then return 1; fi
  rm -rf "$_lk" 2>/dev/null || true
  mkdir "$_lk" 2>/dev/null || return 1
  printf '%s' "$_me" > "$_lk/owner" 2>/dev/null
  return 0
}

# cc_unlock LOCKPATH — drop a lock, but ONLY if we still own it: a contender that judged us dead
# may already hold it, and removing it then would hand a third process a lock two others believe
# they hold.
cc_unlock() {
  _lk="$1.d"; _me="$$"; [ -n "${BASHPID:-}" ] && _me="$BASHPID"
  [ "$(cat "$_lk/owner" 2>/dev/null || true)" = "$_me" ] && rm -rf "$_lk" 2>/dev/null
  return 0
}

# ── find ─────────────────────────────────────────────────────────────────────
# cc_find_meta DIR [find-expr...] — "<mtime>\t<size>\t<path>" for every match, NEWEST FIRST.
#
# The one primitive behind every "show me the most recent transcripts" scan in this bundle. It
# replaces `find … -printf '%T@\t%s\t%p\n'`, which is GNU-only: BSD find rejects -printf outright,
# so on a stock Mac those scans produced NOTHING and the failures were silent and total — cc-ls
# listed no chats, and cc-name-sync could not match a single codex thread to its pane. Nothing
# errored; the lists were simply empty, which reads as "you have no chats".
#
# The BSD arm gets the same three fields from one batched `stat` exec, so it stays one process
# per scan rather than one per file.
#
# FIELD 1 IS AN INTEGER EPOCH ON BOTH ARMS, and that normalization belongs here rather than in
# the callers. GNU's `%T@` carries a fractional part ("1785795885.1234567890"); BSD's `%m` does
# not. Both callers stripped the fraction with `${line%%.*}` — correct on GNU only by accident,
# because there the first `.` in the line is always the fraction separator. On BSD the first `.`
# is in the PATH ("/Users/…/.claude/…"), so `mt` came back as "<mtime><TAB><size><TAB>$HOME/"
# — a tab-bearing string handed to zsh arithmetic and then WRITTEN INTO THE CHAT CACHE, whose
# fields it shifted by one. Every affected chat persisted with prompts=0 and was filtered out of
# the picker on every subsequent run, including runs after the parse itself was fixed. A caller
# cannot be trusted to re-derive a platform difference; the seam hands it a single shape.
#
# Sub-second ordering is preserved where it exists: the GNU arm sorts on the fractional value and
# truncates AFTER the sort, so only the printed field loses the fraction, never the order. On BSD
# two files written in the same second tie and order by size — nothing in this bundle can tell.
#
# The probe result is cached in CC_FIND_PRINTF: the active shell callers and the legacy
# cc-fleet.zsh oracle can load this library repeatedly, and a fork per load to re-learn which
# find is installed is a cost with no payer.
cc_find_meta() {
  _d="$1"; shift
  [ -d "$_d" ] || return 0
  if [ -z "${CC_FIND_PRINTF:-}" ]; then
    if find "$_d" -maxdepth 0 -printf '' >/dev/null 2>&1; then CC_FIND_PRINTF=1; else CC_FIND_PRINTF=0; fi
  fi
  if [ "$CC_FIND_PRINTF" = 1 ]; then
    # awk, not a second `sort` key or a `cut`: sub() rebuilds the record with OFS, so a path
    # carrying anything exotic survives byte-for-byte. One process per SCAN, not per file.
    find "$_d" "$@" -printf '%T@\t%s\t%p\n' 2>/dev/null | sort -rn \
      | awk -F'\t' -v OFS='\t' '{ sub(/\.[0-9]*$/, "", $1); print }'
  else
    find "$_d" "$@" -exec stat -f '%m	%z	%N' {} + 2>/dev/null | sort -rn
  fi
}

# cc_find_newest DIR [find-expr...] — the same scan, paths only, newest first.
cc_find_newest() { cc_find_meta "$@" | cut -f3; }

# ── in-place edit ────────────────────────────────────────────────────────────
# cc_sed_i EXPR FILE — apply a sed script to FILE in place, no backup file left behind.
# GNU takes a bare `-i`; BSD requires an extension argument and reads stdin when it does
# not get one — which is why a GNU-form `sed -i` on a Mac fails with "may not be used with
# stdin" AFTER the caller already believed the edit landed. Writing through a temp file
# sidesteps the incompatibility entirely and is atomic besides.
cc_sed_i() {
  _expr="$1"; _f="$2"
  _t="$_f.cc-sed.$$"
  sed "$_expr" "$_f" > "$_t" 2>/dev/null || { rm -f "$_t"; return 1; }
  mv -f "$_t" "$_f"
}

# ── codex thread identity ────────────────────────────────────────────────────
# A codex chat's identity lives in ~/.codex/state_<N>.sqlite, table `threads` — NOT in a rollout
# file. A thread running in `paginated` history mode may leave no rollout behind at all, so every
# resolver that matches a chat through ~/.codex/sessions/ silently misses the new ones and, worse,
# falls back to a stale sibling instead of failing. The filename is versioned; take the highest.
CX_BIRTH_SLOP="${CX_BIRTH_SLOP:-120}"

# cx_thread_id <cwd> <start-epoch> — the thread the pane owns, empty when the store holds none
# that close (an elder or resumed chat). Matched by BIRTH TIME, not newest-in-cwd: codex records
# a thread within seconds of its process starting, so the user thread in the pane's cwd born
# nearest the pane's own start is that pane's thread. Newest-in-cwd mislabels every chat the
# moment two of them share a directory.
# Read-only and WAL-aware — `mode=ro`, never `immutable`, which hides every row still in the -wal.
cx_thread_id() {
  _cxdb="$(ls -1 "${CODEX_HOME:-$HOME/.codex}"/state_*.sqlite 2>/dev/null \
    | sed 's/.*state_\([0-9]*\)\.sqlite$/\1 &/' | sort -rn | head -1 | cut -d' ' -f2-)"
  [ -n "$_cxdb" ] && [ -n "${2:-}" ] || return 0
  _cxcwd="$(printf '%s' "$1" | sed "s/'/''/g")"
  sqlite3 -readonly "file:$_cxdb?mode=ro" \
    "SELECT id FROM threads
      WHERE thread_source='user' AND archived=0 AND cwd='$_cxcwd'
        AND abs(created_at - $2) <= $CX_BIRTH_SLOP
      ORDER BY abs(created_at - $2) LIMIT 1;" 2>/dev/null
}
