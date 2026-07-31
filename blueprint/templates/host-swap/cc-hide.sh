#!/usr/bin/env bash
# cc-hide.sh [--exit] — add the CURRENT Claude chat to cc-ls's hide list (run from inside the chat,
# e.g. via the global /hide command). Non-destructive: the transcript is kept; it just stops showing
# in cc-ls. Identifies this chat via the tmux socket → transcript-path map the statusline maintains
# (/tmp/cc-sid/<socket>), falling back to the pane-keyed breadcrumb.
#   --exit  after hiding, gracefully close the chat by typing /exit into its tmux pane, then
#           kill-server its OWN tmux so nothing idle is left behind (detached, ~1.5s later, so
#           this turn finishes first). Per-chat -L socket → kill-server can't touch a sibling.
#           No-op if not in tmux.
# Undo: cc-ls --hidden then ⌃X, or remove the line from ~/.claude/.cc-ls-hidden.
set -u
do_exit=0; [ "${1:-}" = "--exit" ] && do_exit=1
hf="$HOME/.claude/.cc-ls-hidden"
sock=""; pane=""
if [ -n "${TMUX:-}" ]; then
  sock="${TMUX%%,*}"; sock="${sock##*/}"          # this chat's -L socket
  pane="${TMUX_PANE:-}"                            # this chat's OWN pane (not the active one)
fi
# Identify THIS chat's transcript uuid. The env session id comes first, but ONLY when a
# transcript by that name exists — a bridged session's id can differ from its file, and
# hiding a phantom id hides nothing. Then the pane-keyed breadcrumb (precise even when
# several chats share one tmux socket as panes), then the socket breadcrumb.
u=""
# env session id first — but ONLY when a transcript by that name exists, across ALL accounts
# (an account-2/3 chat's transcript lives under ~/.claudeN; checking only the account-1 glob
# made every account-2/3 chat fail this rung). Test STDOUT, not ls's exit code: passing the
# account-1 glob AND the ~/.claudeN glob together makes ls exit nonzero whenever one side has
# no match (the literal unmatched pattern errors), so `if ls …` would wrongly fail for an
# account-2/3 chat even though the ~/.claudeN glob matched. `| grep -q .` is true iff ls
# printed at least one real path.
if [ -n "${CLAUDE_CODE_SESSION_ID:-}" ] && \
   ls "$HOME"/.claude/projects/*/"$CLAUDE_CODE_SESSION_ID".jsonl "$HOME"/.claude[0-9]*/projects/*/"$CLAUDE_CODE_SESSION_ID".jsonl 2>/dev/null | grep -q .; then
  u="$CLAUDE_CODE_SESSION_ID"
fi
if [ -z "$u" ]; then
  tp=""
  [ -n "$sock" ] && [ -n "$pane" ] && tp="$(cat "/tmp/cc-sid/$sock.$pane" 2>/dev/null)"
  [ -z "$tp" ] && [ -n "$sock" ] && tp="$(cat "/tmp/cc-sid/$sock" 2>/dev/null)"
  # NO newest-transcript fallback: `ls -t …/*.jsonl | head -1` returned the most-recently
  # written transcript BOX-WIDE (no relation to this chat), and with --exit that wrong uuid
  # drove kill-server/kill-pane over ANOTHER chat's teammates. Fail loud instead — the case
  # below catches an empty/garbage u.
  u="$(basename -- "$tp" .jsonl 2>/dev/null)"
fi
case "$u" in
  *-*-*-*-*) : ;;                                  # looks like a session uuid
  *) echo "cc-hide: couldn't identify this chat — set CLAUDE_CODE_SESSION_ID or run the statusline"; exit 1 ;;
esac
mkdir -p "$(dirname "$hf")"
if grep -qxF -- "$u" "$hf" 2>/dev/null; then
  echo "cc-hide: already hidden ($u)"
else
  printf '%s\n' "$u" >> "$hf"
  echo "cc-hide: hidden $u — gone from cc-ls (cc-ls --hidden to manage · ⌃X to restore)"
fi
if [ "$do_exit" = 1 ]; then
  # Reap detached teammates this chat spawned via /chat:new --detach — each runs on its
  # OWN cc-new-* tmux server and would otherwise be orphaned headless. They were
  # registered at spawn under this chat's session uuid; kill each, then clear the list.
  children="$HOME/.claude/.cc-new-children/$u"
  if [ -f "$children" ]; then
    while IFS= read -r tsock; do
      [ -n "$tsock" ] || continue
      tmux -L "$tsock" kill-server 2>/dev/null && echo "cc-hide: reaped detached teammate ($tsock)"
      rm -f "${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)/$tsock" 2>/dev/null   # sweep the stale socket file so it never joins the graveyard
    done < "$children"
    rm -f "$children"
  fi
  # Reap PANE teammates this chat spawned via /chat:branch or /chat:new (pane mode) — they
  # live in panes of this chat's window (a SHARED tmux server), so kill each PANE, never the
  # server. Registered at spawn as "<socket>\t<pane>" under this chat's session uuid.
  pchildren="$HOME/.claude/.cc-pane-children/$u"
  if [ -f "$pchildren" ]; then
    while IFS="$(printf '\t')" read -r csock cpane; do
      [ -n "$cpane" ] || continue
      tmux -L "$csock" kill-pane -t "$cpane" 2>/dev/null && echo "cc-hide: reaped pane teammate ($cpane)"
    done < "$pchildren"
    rm -f "$pchildren"
  fi
  if [ -n "$pane" ]; then
    # Close THIS chat by killing ONLY its own pane — never kill-server, since /chat:branch
    # and /chat:new put sibling chats in OTHER panes of the SAME tmux server (kill-server
    # would drop them all — the bug this replaces). Killing the last pane ends the server, so
    # a standalone chat still fully closes. Detached + delayed so this turn finishes first;
    # graceful /exit lets Claude flush its transcript and run Stop hooks — polled until the
    # pane closes itself (up to 20s; compaction can blow past a fixed grace), then kill-pane
    # is the backstop. Sweep the cc-sid breadcrumb only if this was the last pane on the
    # server. Finally record the POST-exit transcript size as the auto-unhide baseline, so
    # the /exit flush itself can never read as "new activity" and resurrect the chat in cc-ls.
    last=0; [ "$(tmux -L "$sock" list-panes -a 2>/dev/null | wc -l | tr -d ' ')" = "1" ] && last=1
    echo "cc-hide: closing this chat (auto /exit, then kill-pane $pane)…"
    setsid env SOCK="$sock" PANE="$pane" LAST="$last" U="$u" \
      AF="$HOME/.claude/.cc-ls-hidden.at" PROJ="$HOME/.claude/projects" bash -c '
      sleep 1.5
      tmux -L "$SOCK" send-keys -t "$PANE" -l -- /exit
      tmux -L "$SOCK" send-keys -t "$PANE" Enter
      n=0
      while [ "$n" -lt 20 ] && tmux -L "$SOCK" list-panes -a -F "#{pane_id}" 2>/dev/null | grep -qx -- "$PANE"; do
        sleep 1; n=$((n+1))
      done
      tmux -L "$SOCK" kill-pane -t "$PANE" 2>/dev/null
      rm -f "/tmp/cc-sid/$SOCK.$PANE"                     # the pane-keyed breadcrumb
      [ "$LAST" = 1 ] && rm -f "/tmp/cc-sid/$SOCK"
      tp="$(ls "$PROJ"/*/"$U".jsonl 2>/dev/null | head -1)"
      if [ -n "$tp" ]; then
        sz="$(stat -c%s "$tp" 2>/dev/null || echo 0)"
        { grep -v "^$U$(printf "\t")" "$AF" 2>/dev/null; printf "%s\t%s\n" "$U" "$sz"; } > "$AF.t" && mv "$AF.t" "$AF"
      fi
    ' >/dev/null 2>&1 &
  elif [ -n "$sock" ]; then
    echo "cc-hide: in tmux but \$TMUX_PANE unset — type /exit yourself"
  else
    echo "cc-hide: not in tmux — type /exit yourself"
  fi
fi
