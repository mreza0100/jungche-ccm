#!/usr/bin/env bash
# cc-swap-chat.sh [<1|2>] [--then "<prompt>"] [--sock <cc-socket>] [--1h on|off] — move a
# chat to another account and/or flip its ⚡1h-cache mode: gracefully /exit its session, then
# respawn `claude --resume <its transcript>` IN PLACE (same pane, same socket) under the target
# env. The account is optional when --1h is given — a cache-only reboot keeps the chat's
# CURRENT account (read from the live process, since env binds at birth). A live session can't change accounts
# (the OAuth token binds to the config dir it was born with), so close-and-reopen IS the
# clean swap. Invoked by the global /swap (self-swap, target from $TMUX) and by cc-ls's
# mismatched-account gate (--sock names another chat's server; single-pane servers only —
# a split could hide an innocent sibling). With --then, the follow-up prompt is typed into
# the reborn chat once it reaches its input box — the chat continues working unattended
# (the limit-rescue path: a chat swaps ITSELF to a fresh account and hands itself the baton).
set -u
# CC_FLEET_HOME — this bundle's OWN directory, resolved THROUGH symlinks. install.sh links these
# scripts into ~/.claude/bin, so $BASH_SOURCE is the link, not the file: taking its dirname
# straight would hunt for siblings in the link's directory. Plain `readlink` (never -f) because
# macOS ships BSD readlink, which has no -f.
_ccfs="${BASH_SOURCE[0]}"; while [ -L "$_ccfs" ]; do _ccfd="$(cd -P "$(dirname "$_ccfs")" && pwd)"; _ccfs="$(readlink "$_ccfs")"; case "$_ccfs" in /*) ;; *) _ccfs="$_ccfd/$_ccfs" ;; esac; done
CC_FLEET_HOME="${CC_FLEET_HOME:-$(cd -P "$(dirname "$_ccfs")" && pwd)}"
. "$CC_FLEET_HOME/cc-portable.sh"   # GNU/BSD seam — cc_size0, cc_penv, cc_detach
n=""; then_prompt=""; sock_arg=""; c1h_arg=""
while [ $# -gt 0 ]; do
  case "$1" in
    --then) shift; then_prompt="${1:?cc-swap-chat: --then needs a prompt}" ;;
    --sock) shift; sock_arg="${1:?cc-swap-chat: --sock needs a cc-* socket}" ;;
    --1h)   shift; case "${1:-}" in on|1) c1h_arg=1 ;; off|0) c1h_arg=0 ;;
              *) echo "cc-swap-chat: --1h needs on|off"; exit 2 ;; esac ;;
    1|2)  n="$1" ;;
    *) echo "usage: cc-swap-chat.sh [<1|2>] [--then \"<prompt>\"] [--sock <cc-socket>] [--1h on|off]   # account optional with --1h (cache-only reboot keeps the account)"; exit 2 ;;
  esac
  shift
done
[ -n "$n" ] || [ -n "$c1h_arg" ] || { echo "usage: cc-swap-chat.sh [<1|2>] [--then \"<prompt>\"] [--sock <cc-socket>] [--1h on|off]   # account optional with --1h (cache-only reboot keeps the account)"; exit 2; }
# send-keys -l types newlines as Enter (would submit mid-message) — flatten to spaces
then_prompt="$(printf '%s' "$then_prompt" | tr '\n\r' '  ')"
case "$n" in
  2) cfg="$HOME/.cc/$n" ;;
  *) cfg="" ;;   # account 1 — or --1h-only, resolved below once the chat is identified
esac

# identify the chat. --sock targets ANOTHER chat's server (cc-ls's gate): the caller's env
# (session id, $TMUX) describes the CALLER, so only the target's breadcrumbs may speak here.
# Self-swap (/swap): same chain as `cc-fleet hide --self` — env session id (only when its transcript
# exists), then the pane-keyed breadcrumb, then the socket breadcrumb.
sock=""; pane=""; u=""
if [ -n "$sock_arg" ]; then
  sock="$sock_arg"
  pane="$(tmux -L "$sock" list-panes -a -F '#{pane_id}' 2>/dev/null)"
  case "$pane" in
    "") echo "cc-swap-chat: no live server on -L $sock"; exit 1 ;;
    *"
"*) echo "cc-swap-chat: -L $sock has multiple panes — run /swap inside the chat instead"; exit 1 ;;
  esac
  tp="$(cat "/tmp/cc-sid/$sock.$pane" 2>/dev/null)"
  [ -z "$tp" ] && tp="$(cat "/tmp/cc-sid/$sock" 2>/dev/null)"
  u="$(basename -- "$tp" .jsonl 2>/dev/null)"
else
  if [ -n "${TMUX:-}" ]; then sock="${TMUX%%,*}"; sock="${sock##*/}"; pane="${TMUX_PANE:-}"; fi
  if [ -n "${CLAUDE_CODE_SESSION_ID:-}" ] && ls "$HOME"/.claude/projects/*/"$CLAUDE_CODE_SESSION_ID".jsonl "$HOME"/.cc/[0-9]*/projects/*/"$CLAUDE_CODE_SESSION_ID".jsonl >/dev/null 2>&1; then
    u="$CLAUDE_CODE_SESSION_ID"
  fi
  if [ -z "$u" ]; then
    tp=""
    [ -n "$sock" ] && [ -n "$pane" ] && tp="$(cat "/tmp/cc-sid/$sock.$pane" 2>/dev/null)"
    [ -z "$tp" ] && [ -n "$sock" ] && tp="$(cat "/tmp/cc-sid/$sock" 2>/dev/null)"
    u="$(basename -- "$tp" .jsonl 2>/dev/null)"
  fi
fi
case "$u" in
  *-*-*-*-*) : ;;
  *) echo "cc-swap-chat: couldn't identify this chat — set CLAUDE_CODE_SESSION_ID or run the statusline"; exit 1 ;;
esac
[ -n "$pane" ] || { echo "cc-swap-chat: not in tmux — /exit yourself, then: CLAUDE_CONFIG_DIR=${cfg:-<unset>} claude --resume $u"; exit 1; }
# resume is cwd-scoped — take the cwd from the transcript itself, not the caller's $PWD
# (the FIRST line is often a summary record without .cwd — scan the head for a real one).
# Search EVERY account's registry: an account-2/3 chat's transcript lives under ~/.cc/<n>,
# and missing it here silently rebooted the chat in the CALLER's cwd (wrong project).
tpf="$(ls "$HOME"/.claude/projects/*/"$u".jsonl "$HOME"/.cc/[0-9]*/projects/*/"$u".jsonl 2>/dev/null | head -1)"
if [ -z "$tpf" ]; then
  # no transcript = a chat with no messages yet — nothing to --resume (respawning a resume
  # of a nonexistent session dies and takes the pane with it). Reboot FRESH instead: the
  # empty conversation loses nothing and the swap still changes the account / cache mode.
  echo "cc-swap-chat: no transcript yet (chat has no messages) — rebooting FRESH under the new env"
  u=""
  rcwd="$(tmux -L "$sock" display-message -p -t "$pane" '#{pane_current_path}' 2>/dev/null)"
  [ -d "$rcwd" ] || rcwd="$PWD"
else
  rcwd="$(head -40 "$tpf" 2>/dev/null | jq -r 'select(.cwd) | .cwd' 2>/dev/null | head -1)"
  [ -d "$rcwd" ] || rcwd="$PWD"
fi
tpmb=$(( $(cc_size0 "$tpf") / 1048576 ))   # scales the --then input-box wait

# ⚡1h-cache: preserve the CHAT'S OWN mode across the swap — read from the live process's
# environment (there is no sticky global toggle; a reboot must not silently change the chat's
# cache profile). comm is "claude" or a bare version string, depending on the install.
# Since CC 2.1.215 the harness defaults every session to 1h, so only an explicit
# FORCE_PROMPT_CACHING_5M=1 birth reads as 5m — a flagless elder runs (and keeps) 1h.
#
# READABLE ON LINUX ONLY, AND THAT IS SAFE. cc_penv answers from /proc/<pid>/environ; macOS gives
# a process no way to read another's environment at all (SIP), so a Mac finds nothing and the
# default stands. The default IS the harness default — a chat with no FORCE_PROMPT_CACHING_5M in
# its birth env runs 1h — so an unreadable env and a flagless chat produce the same verdict, and
# the one case a Mac gets wrong (a chat deliberately born 5m) is rebooted 1h rather than
# silently billed the 2x write premium it never asked for. Wrong only in the cheap direction.
c1h=1; cp=""
ptty="$(tmux -L "$sock" display-message -p -t "$pane" '#{pane_tty}' 2>/dev/null)"
for cp in $(ps -o pid=,comm= -t "${ptty#/dev/}" 2>/dev/null | awk '$2=="claude" || $2 ~ /^[0-9]+\./ {print $1}'); do
  [ "$(cc_penv "$cp" FORCE_PROMPT_CACHING_5M)" = "1" ] && { c1h=0; break; }
done
[ -n "$c1h_arg" ] && c1h="$c1h_arg"   # --1h on|off overrides (the picker's reboot-to-match gate)

# account: an explicit <n> wins; else (--1h-only reboot) KEEP the chat's current account —
# from the live process env (same claude pid the c1h loop above found, via cc_penv — reused
# rather than a second /proc read), falling back to the caller's own (a self-swap's tool shell
# carries the chat's config dir). Symlinked accounts collapse via pwd -P (~/.cc/2 → ~/.claude3).
if [ -z "$n" ]; then
  tcfg=""; [ -n "$cp" ] && tcfg="$(cc_penv "$cp" CLAUDE_CONFIG_DIR)"
  [ -n "$tcfg" ] || tcfg="${CLAUDE_CONFIG_DIR:-}"
  p=""; [ -n "$tcfg" ] && [ -d "$tcfg" ] && p="$(cd "$tcfg" && pwd -P)"
  case "$p" in
    "$HOME/.claude3"|"$HOME/.cc/2") n=2 ;;
    *)                              n=1 ;;
  esac
  case "$n" in 2) cfg="$HOME/.cc/$n" ;; *) cfg="" ;; esac
  echo "cc-swap-chat: no account given — keeping the chat's current account $n"
fi

echo "cc-swap-chat: swapping this chat to account $n IN PLACE — it reboots right here under the new account"
[ -n "$then_prompt" ] && echo "cc-swap-chat: --then queued — the follow-up is typed into the reborn chat once it reaches its prompt"
# The swap happens INSIDE the chat's own pane: remain-on-exit holds the pane open through the
# graceful /exit (transcript flush, Stop hooks), then respawn-pane boots the new-account claude
# in the SAME window on the SAME socket — no client ever disconnects, so it's seamless for raw
# VS Code tabs, bunkers, and splits alike; detached chats swap silently. Socket name and
# breadcrumbs stay stable (the new statusline refreshes them). Delayed so this turn finishes.
$(cc_detach) env SOCK="$sock" PANE="$pane" U="$u" CFG="$cfg" CWD="$rcwd" THEN="$then_prompt" C1H="$c1h" TPMB="$tpmb" ACCT="$n" bash -c '
  sleep 1.5
  # per-pane mutual exclusion: two overlapping swaps of one pane both /exit and both
  # respawn -k, the second killing a freshly resumed claude with a stale env snapshot.
  # A non-blocking flock bails loudly rather than colliding. Held for the whole reboot
  # (fd 9 stays open for the life of this subshell).
  # dot-prefixed: "$SOCK.%N.swaplock" would MATCH the "$sock".%* pane-crumb globs in
  # cc-reap and _cc_solo, reading as a phantom pane crumb (husk servers, fabricated has_crumb)
  mkdir -p /tmp/cc-sid 2>/dev/null
  exec 9>"/tmp/cc-sid/.$SOCK.$PANE.swaplock" 2>/dev/null || true
  if command -v flock >/dev/null 2>&1 && ! flock -n 9; then
    tmux -L "$SOCK" display-message -t "$PANE" "swap skipped — another swap of this pane is already in flight" 2>/dev/null
    echo "swap: another swap of $SOCK $PANE is in flight — aborting this one"; exit 1
  fi
  tmux -L "$SOCK" set-option -p -t "$PANE" remain-on-exit on   # -p: pane-scoped — -w leaked to split siblings (an exiting sibling became an uncloseable husk)
  # Prep the input surface the way chat.sh inject does before typing — else the /exit is
  # swallowed or corrupted: copy-mode eats keys as bindings, a draft turns it into
  # "draft/exit", and an OPEN selector/permission menu would make Enter forge a menu
  # selection (e.g. "No, exit!" or a permission answer). Cancel copy-mode; on an open
  # menu, bail VISIBLY rather than force it; stash any draft with C-s (no-op on empty box).
  [ "$(tmux -L "$SOCK" display-message -p -t "$PANE" "#{pane_in_mode}" 2>/dev/null || echo 0)" = 1 ] && \
    tmux -L "$SOCK" send-keys -t "$PANE" -X cancel 2>/dev/null
  selline="$(tmux -L "$SOCK" capture-pane -t "$PANE" -p -J 2>/dev/null | grep -F "❯" | tail -1)"
  if printf "%s" "$selline" | grep -qE "❯[[:space:]]*[0-9]+\.[[:space:]]"; then
    tmux -L "$SOCK" set-option -p -t "$PANE" -u remain-on-exit
    tmux -L "$SOCK" display-message -t "$PANE" "swap ABORTED — answer the open menu first, then /swap again" 2>/dev/null
    echo "swap: open selector menu on the pane — refusing to /exit (Enter would forge a selection). Aborted."; exit 1
  fi
  tmux -L "$SOCK" send-keys -t "$PANE" C-s; sleep 0.2
  tmux -L "$SOCK" send-keys -t "$PANE" -l -- /exit
  tmux -L "$SOCK" send-keys -t "$PANE" Enter
  i=0; gone=0; empties=0
  while [ "$i" -lt 20 ]; do
    d="$(tmux -L "$SOCK" list-panes -a -F "#{pane_id} #{pane_dead}" 2>/dev/null | awk -v p="$PANE" "\$1==p{print \$2}")"
    [ "$d" = 1 ] && { gone=1; break; }
    if [ -z "$d" ]; then                                  # pane not listed — a transient empty
      empties=$((empties+1)); [ "$empties" -ge 3 ] && { gone=1; break; }   # read is not proof; retry
    else
      empties=0
    fi
    sleep 1; i=$((i+1))
  done
  # If the graceful /exit did NOT complete (timeout with claude still alive on the tty), do
  # NOT silently respawn-pane -k — that force-kills a live turn with no Stop hooks. Surface a
  # visible failure and leave the chat running; the operator retries when it is idle.
  if [ "$gone" != 1 ]; then
    ptty0="$(tmux -L "$SOCK" display-message -p -t "$PANE" "#{pane_tty}" 2>/dev/null)"
    if ps -o comm= -t "${ptty0#/dev/}" 2>/dev/null | grep -qE "^(claude|[0-9]+\.)"; then
      tmux -L "$SOCK" set-option -p -t "$PANE" -u remain-on-exit
      tmux -L "$SOCK" display-message -t "$PANE" "swap NOT done — /exit did not complete (pane busy?); chat left running. /swap again when idle" 2>/dev/null
      echo "swap: /exit did not complete after 20s and claude is still alive — refusing respawn -k; chat left running."; exit 1
    fi
  fi
  # env -u, always: respawn-pane inherits the SERVER environment — a server born from a
  # poisoned shell (chat-in-chat launch) carries CLAUDE_CONFIG_DIR and would silently defeat
  # a swap to account 1; stale session identity must not ride along either
  run="env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M"
  # strip any inherited API endpoint: a server born from a shell pointed at a translating proxy
  # would carry a base URL that silently redirects the reborn chat away from Anthropic.
  run="$run -u ANTHROPIC_BASE_URL -u ANTHROPIC_AUTH_TOKEN -u ANTHROPIC_MODEL -u ANTHROPIC_SMALL_FAST_MODEL"
  run="$run -u CLAUDE_CODE_AUTO_COMPACT_WINDOW -u CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC -u CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK"
  if [ -n "$CFG" ]; then run="$run CLAUDE_CONFIG_DIR=$CFG"; else run="$run -u CLAUDE_CONFIG_DIR"; fi
  # the chat keeps (or the --1h override sets) its ⚡ mode — env ARG, after its own -u (a shell
  # prefix would be re-unset); 5m must be FORCED since the harness default is 1h
  if [ "$C1H" = 1 ]; then run="$run ENABLE_PROMPT_CACHING_1H=1"; else run="$run FORCE_PROMPT_CACHING_5M=1"; fi
  if [ -n "$U" ]; then run="$run claude --resume $U"; else run="$run claude"; fi   # no transcript yet → fresh boot

  tmux -L "$SOCK" respawn-pane -k -t "$PANE" -c "$CWD" "$run"
  echo "respawned in place: $SOCK $PANE rc=$?"
  tmux -L "$SOCK" set-option -p -t "$PANE" -u remain-on-exit
  [ -n "$THEN" ] || exit 0
  # --then: hand the reborn chat its next prompt (chat.sh-inject style, simplified — a fresh
  # boot has no drafts or copy-mode to defuse). Wait for the resume to reach the input box —
  # scaled to transcript size (a multi-MB resume replays for minutes), capped at 15min.
  # Failure is VISIBLE: a /tmp/cc-sid/$SOCK.then-failed sentinel + a pane display-message,
  # never just a line in this unwatched log. The baton must not drop silently.
  then_fail() {
    printf "%s\n" "$THEN" > "/tmp/cc-sid/$SOCK.then-failed" 2>/dev/null
    tmux -L "$SOCK" display-message -t "$PANE" "swap --then NOT delivered ($1) — prompt saved: /tmp/cc-sid/$SOCK.then-failed" 2>/dev/null
    echo "then: $1 — follow-up NOT delivered: $THEN"; exit 1
  }
  wait_max=$((90 + 30 * ${TPMB:-0})); [ "$wait_max" -gt 900 ] && wait_max=900
  ptty="$(tmux -L "$SOCK" display-message -p -t "$PANE" "#{pane_tty}" 2>/dev/null)"
  i=0
  while [ "$i" -lt "$wait_max" ]; do
    cap="$(tmux -L "$SOCK" capture-pane -t "$PANE" -p -J 2>/dev/null)"
    # A config-dir/cwd pair not yet trust-accepted opens the trust prompt. CC 2.1.215 emits
    # "Trust this directory?" / "Yes, I trust this folder" (the old "trust the files" needle
    # matched NO installed version, so --then typed into the trust MENU). Its default is Yes:
    # a single Enter accepts. Handle it FIRST and continue, so the dialog own ❯ pointer can
    # never satisfy the input-box check below (which would type the prompt into the menu).
    case "$cap" in
      *"Trust this directory?"*|*"trust this folder"*|*"trust these settings"*)
        tmux -L "$SOCK" send-keys -t "$PANE" Enter; sleep 2; i=$((i+1)); continue ;;
    esac
    printf "%s" "$cap" | tail -15 | grep -qF "❯" && break    # bottom lines only — a ❯ in replayed scrollback is not the input box
    sleep 1; i=$((i+1))
  done
  [ "$i" -lt "$wait_max" ] || then_fail "input box never appeared after ${wait_max}s"
  # never type into a bare shell or dead pane — the reborn claude must be alive on this tty
  ps -o comm= -t "${ptty#/dev/}" 2>/dev/null | grep -qE "^(claude|[0-9]+\.)" || then_fail "no live claude on the pane"
  sleep 2
  tmux -L "$SOCK" send-keys -t "$PANE" -l -- "$THEN"
  flat="$(printf %s "$THEN" | tr -s "[:space:]" " ")"
  needle="${flat: -40}"; [ -n "$needle" ] || needle="$flat"   # a sub-40-char prompt slices to empty — vacuous match
  prefix="${flat:0:24}"
  typed=0; i=0
  while [ "$i" -lt 40 ]; do
    cap="$(tmux -L "$SOCK" capture-pane -t "$PANE" -p -J 2>/dev/null | tr -s "[:space:]" " ")"
    case "$cap" in *"$needle"*) typed=1; break ;; esac
    sleep 0.2; i=$((i+1))
  done
  [ "$typed" = 1 ] || then_fail "typed text never rendered — blind Enter presses would submit garbage"
  ok=0; i=0
  while [ "$i" -lt 12 ]; do
    tmux -L "$SOCK" send-keys -t "$PANE" Enter
    sleep 0.15
    tmux -L "$SOCK" send-keys -t "$PANE" Enter              # empty-box no-op; defeats a swallowed first Enter
    sleep 0.4
    line="$(tmux -L "$SOCK" capture-pane -t "$PANE" -p -J 2>/dev/null | grep -F "❯" | tail -1)"
    case "$line" in *"$prefix"*) i=$((i+1)) ;; *) ok=1; break ;; esac
  done
  if [ "$ok" = 1 ]; then echo "then: follow-up delivered and submitted"
  else
    tmux -L "$SOCK" display-message -t "$PANE" "swap --then typed but submit unconfirmed — press Enter" 2>/dev/null
    echo "then: typed but submit unconfirmed — press Enter in the pane"
  fi
' 2>&1 | bash "$CC_FLEET_HOME/cc-db.sh" swap-log &   # append: concurrent swaps must not truncate each other's log
exit 0
