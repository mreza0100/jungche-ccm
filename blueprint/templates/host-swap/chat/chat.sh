#!/usr/bin/env bash
set -euo pipefail

# CC_FLEET_HOME — the fleet bundle this script belongs to. chat.sh sits one level under it
# (bundle/chat/chat.sh alongside bundle/cc-db.sh), and install.sh symlinks it into
# ~/.claude/commands/chat/, so resolve THROUGH the link before stepping up a directory —
# $BASH_SOURCE is the link, and its dirname is the install path, not the bundle. Plain
# `readlink` (never -f) because macOS ships BSD readlink, which has no -f.
_ccfs="${BASH_SOURCE[0]}"; while [ -L "$_ccfs" ]; do _ccfd="$(cd -P "$(dirname "$_ccfs")" && pwd)"; _ccfs="$(readlink "$_ccfs")"; case "$_ccfs" in /*) ;; *) _ccfs="$_ccfd/$_ccfs" ;; esac; done
CC_FLEET_HOME="${CC_FLEET_HOME:-$(cd -P "$(dirname "$_ccfs")/.." && pwd)}"
. "$CC_FLEET_HOME/cc-portable.sh"   # GNU/BSD seam — cc_pane_of, cc_session_live, cc_timeout, cc_detach

# chat.sh — the chat: family engine, one script with subcommands. Lives in
# $HOME/.claude/commands/chat/ (global — shared by every repo; repo .claude/commands/chat
# entries are symlinks here); each chat/<name>.md command calls `chat.sh <name>`.
#
#   whoami                                    print THIS chat's own tmux session
#   find    <excerpt-file>                    resolve a pasted excerpt to a session
#   read    <excerpt-file>                    extract a matched chat's transcript
#   inject  [--force-now] [--then <steer>]... <self|tmux|label|session-id|path> <message...>  force a turn (live / resume; steers chain in order)
#   ls      [--all]                           list live chats with cwd in this repo (--all: every dir/project)
#   capture <tmux-session>                    snapshot a live chat's full scrollback
#   extract <transcript-jsonl>                render a known transcript to text
#   save    <target-file> [transcript-jsonl]  dump THIS session's transcript
#   tail    [N]                               render THIS session's last N lines
#   load    <dir-or-file>...                  enumerate a file set to force-read in full
#   branch  [name]                            fork THIS session into a side-by-side pane (inherits model, names the fork)
#   new     [--detach] [name]                  spawn a FRESH teammate chat — a side-by-side pane, or --detach for a headless bg session
#   modal   <tmux-session> deny <down-count>  answer a permission modal DENY-ONLY (N Downs + Enter); modal deadlock rescue

# transcript-extract.jq, inlined — renders a session JSONL as readable chat.
read -r -d '' JQ_EXTRACT <<'JQ' || true
def textblocks:
  (if (.message.content | type) == "string"
   then [.message.content]
   else [.message.content[]? | select(.type == "text") | .text]
   end)
  | map(select(startswith("<system-reminder>") | not))
  | map(select(length > 0));

select(.isSidechain != true)
| if .type == "summary" then
    "## [COMPACTION SUMMARY]\n\n" + (.summary // empty)
  elif .type == "user" then
    (textblocks | select(length > 0) | "## USER\n\n" + join("\n\n"))
  elif .type == "assistant" then
    ((textblocks | join("\n\n")) as $t
     | ([.message.content[]? | select(.type == "tool_use") | .name] | unique) as $tools
     | if ($t | length) > 0 then
         "## ASSISTANT\n\n"
         + (if ($tools | length) > 0 then "> [tools: " + ($tools | join(", ")) + "]\n\n" else "" end)
         + $t
       elif ($tools | length) > 0 then "> [tools: " + ($tools | join(", ")) + "]"
       else empty end)
  else empty end
| . + "\n"
JQ

# repo_root: the CALLING chat's repo root — derived from $PWD (Claude Code runs this
# from the project's cwd), never from this script's own on-disk location, since the
# script is a single global copy shared by every repo. Falls back to $PWD itself
# outside a git repo.
repo_root() { git rev-parse --show-toplevel 2>/dev/null || pwd -P; }

# tmux_from_ancestry: recover this chat's TMUX from the nearest ancestor that carries it.
# A chat engine may spawn its tool-shell WITHOUT passing tmux context through — the codex
# path measures as TMUX=unset / TMUX_PANE=unset / PPID=codex, which left every codex-origin
# message unsigned (no handle, no label, and CLAUDE_CODE_SESSION_ID is unset at codex launch
# too, so all three signature parts came back empty at once). Identity stays the SCRIPT's job:
# the value is read from the sender's OWN process chain, never accepted from a caller flag.
#
# ASK TMUX, NOT THE KERNEL. This walked /proc/<pid>/environ for a TMUX= line, which no macOS can
# answer — SIP has restricted a process to its OWN environment for a decade, so every Mac read
# "no ancestor is in tmux" and left codex-origin messages unsigned exactly as the bug above
# describes. cc_pane_of asks the tmux servers which pids they own and walks the same ancestry:
# one answer, both platforms. It returns socket and pane; this returns the $TMUX-shaped string
# the callers already parse (only the socket path component is ever read back).
tmux_from_ancestry() {
  local hit sk
  hit="$(cc_pane_of "${1:-$PPID}")"
  [[ -n "$hit" ]] || return 1
  sk="${hit%%$'\t'*}"
  printf '%s\n' "${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)/$sk,0,0"
}

# NEVER export a recovered value globally. An empty $TMUX is not always a defect to repair:
# this script CLEARS it deliberately when probing another chat's socket, and a global re-export
# points every such probe at the SENDER's own session instead (measured: every codex chat
# vanished from `ls` and became uninjectable). The fallback is scoped per call, to the one
# command that needs it, and is never exported.
self_tmux() {
  local t="${TMUX:-}"
  [[ -n "$t" ]] || t="$(tmux_from_ancestry 2>/dev/null || true)"
  [[ -n "$t" ]] || { echo "ERROR: this chat is not inside tmux (\$TMUX unset, and no ancestor carries it)" >&2; return 1; }
  TMUX="$t" tmux display-message -p '#{session_name}'
}

# self_label: this chat's own 🔖 label (its /rename name), read from its own statusline —
# the line carrying both 🔖 and the account badge (🥇/🥈/🥉/🍀). Empty when not in tmux or
# unlabeled. Anchored on the badge (always emitted) rather than 🌿 (only when git
# symbolic-ref succeeds — absent outside a repo or in detached HEAD, which made a labeled
# chat unaddressable). Still collision-proof: this script prints 🔖 but never a badge, so
# no command-echo line in our own pane can satisfy both.
# CODEX FALLBACK: a codex chat has no 🔖 statusline, which left codex-origin signatures
# opaque (bare cx-… handle, no name). When the scrape finds nothing AND this chat's own
# socket basename is cx-*, fall back to the tmux window name — the human thread name, set
# by cc-fleet — returned BARE (no ⬢ prefix: the signature must carry exactly the label the
# founder gave the chat, and a recipient replying by that label must resolve; the cx-…
# reply handle already says which engine it is). The 🔖+badge anchor always wins when
# present — the Claude path is unchanged.
self_label() {
  local s lbl sockbase
  s="$(self_tmux 2>/dev/null || true)"; [[ -n "$s" ]] || return 0
  lbl="$(tmux capture-pane -t "$s" -p -J 2>/dev/null | grep -F '🔖' | grep -E '🥇|🥈|🥉|🍀' | tail -1 \
    | sed 's/.*🔖 *//; s/ *│.*//; s/[[:space:]]*$//' || true)"
  if [[ -z "$lbl" && -n "${TMUX:-}" ]]; then
    sockbase="${TMUX%%,*}"; sockbase="${sockbase##*/}"
    if [[ "$sockbase" == cx-* ]]; then
      lbl="$(tmux display-message -p '#{window_name}' 2>/dev/null || true)"
    fi
  fi
  printf '%s\n' "$lbl"
}

# _pane_busy <tmux-session> [socketpath]: true while Claude Code is actively
# generating. The live indicator is the spinner detail line — "<word>… (Ns · ↓ N
# tokens · …)" or the "esc to interrupt" hint. A FINISHED turn shows "✻ <word> for Ns"
# (no paren-timer, no token counter), which must NOT match — so we key on the
# in-progress paren-timer, token counter, or esc hint, never a bare spinner glyph.
# An optional socket path routes the capture to the target's own socket (cross-socket
# inject); omitted, it uses the connected/default socket as before.
_pane_busy() {
  local -a tm=(tmux); [[ -n "${2:-}" ]] && tm=(tmux -S "$2")
  "${tm[@]}" capture-pane -t "$1" -p -J 2>/dev/null | grep -qiE 'esc to interrupt|\([0-9]+s ·|· [0-9]+s|[0-9]+ tokens'
}

# _inject_lock_acquire <target>: serialize LIVE injects to the SAME target across
# processes — several chats may inject into one pane at once and their keystrokes
# would interleave into a mangled turn. mkdir is atomic on POSIX, so it is the
# portable lock primitive (stock macOS has no flock(1)). The owner file holds
# "PID EPOCH"; a held lock is STOLEN only when its owner process is dead or it has
# been held past CHAT_INJECT_LOCK_MAXHOLD (a wedged holder) — never otherwise.
# Sets INJECT_LOCKDIR on success; times out after CHAT_INJECT_LOCK_TIMEOUT.
_inject_lock_acquire() {
  local target="$1" safe lockdir line opid ots now deadline
  safe="$(printf '%s' "$target" | tr '/:. ' '____')"
  lockdir="${TMPDIR:-/tmp}/chat-inject-locks/${safe}.lock"
  mkdir -p "$(dirname "$lockdir")" 2>/dev/null || true
  deadline=$(( $(date +%s) + ${CHAT_INJECT_LOCK_TIMEOUT:-30} ))
  while :; do
    if mkdir "$lockdir" 2>/dev/null; then
      printf '%s %s\n' "$$" "$(date +%s)" > "$lockdir/owner" 2>/dev/null || true
      # DOUBLE-STEAL GUARD: two contenders can both see the same stale owner and both run
      # rm-rf — the second rm can erase the first's fresh mkdir+owner, then mkdir over it,
      # leaving BOTH believing they own the pane (the interleaved-keystroke outcome the lock
      # exists to prevent). mkdir is atomic but rm-rf+mkdir is not: re-read the owner file
      # after a short settle and only proceed if it still carries OUR pid.
      sleep 0.05
      line="$(cat "$lockdir/owner" 2>/dev/null || true)"
      if [[ "${line%% *}" != "$$" ]]; then continue; fi   # clobbered by a racing steal — re-contend
      INJECT_LOCKDIR="$lockdir"
      return 0
    fi
    line="$(cat "$lockdir/owner" 2>/dev/null || true)"; opid="${line%% *}"; ots="${line##* }"; now="$(date +%s)"
    if { [[ -n "$opid" ]] && ! kill -0 "$opid" 2>/dev/null; } || { [[ "$ots" =~ ^[0-9]+$ ]] && (( now - ots > ${CHAT_INJECT_LOCK_MAXHOLD:-60} )); }; then
      rm -rf "$lockdir" 2>/dev/null || true
      continue
    fi
    if (( now >= deadline )); then return 1; fi
    sleep "${CHAT_INJECT_LOCK_POLL:-0.1}"
  done
}

# _inject_lock_release: drop the lock held in INJECT_LOCKDIR (run from an EXIT trap).
# OWNERSHIP-CHECKED: only remove the lock if WE still own it. If a contender decided we
# were wedged and STOLE the lock (owner file now carries its PID), our EXIT trap must NOT
# delete it — that would unlock the pane out from under the thief mid-delivery, admitting a
# third writer. Compare the owner PID to ours before removing.
_inject_lock_release() {
  [[ -n "${INJECT_LOCKDIR:-}" ]] || { INJECT_LOCKDIR=""; return 0; }
  local _line _opid
  _line="$(cat "$INJECT_LOCKDIR/owner" 2>/dev/null || true)"; _opid="${_line%% *}"
  [[ "$_opid" == "$$" ]] && rm -rf "$INJECT_LOCKDIR" 2>/dev/null
  INJECT_LOCKDIR=""
  return 0
}

# _inject_lock_beat: refresh the owner epoch so the steal test (held-past-MAXHOLD) measures
# STALL, not total hold. A live holder legitimately runs past 60s under fleet load (~120
# tmux roundtrips + bounded sleeps); without a heartbeat a contender would steal the lock
# from a working holder. Called at each long-loop iteration below. OWNERSHIP-AWARE: if the
# owner file no longer carries our pid, the lock was stolen — do NOT rewrite it (that would
# clobber the thief's claim and re-open the pane to a third writer); just stop beating.
_inject_lock_beat() {
  [[ -n "${INJECT_LOCKDIR:-}" ]] || return 0
  local _line; _line="$(cat "$INJECT_LOCKDIR/owner" 2>/dev/null || true)"
  [[ -z "$_line" || "${_line%% *}" == "$$" ]] || return 0
  printf '%s %s\n' "$$" "$(date +%s)" > "$INJECT_LOCKDIR/owner" 2>/dev/null || true
}

# _sockets: print every tmux socket path under the per-user tmux dir, one per line.
# Each chat now runs on its OWN `tmux -L <name>` socket so a single tmux server
# SIGSEGV can no longer take down every chat at once; cross-chat ops must therefore
# search every socket, not just the connected/default one. Fail-safe — if the dir is
# absent, print nothing and return 0; only real sockets ([ -S ]) are emitted.
_sockets() {
  local dir="${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)" f
  [[ -d "$dir" ]] || return 0
  for f in "$dir"/*; do
    [[ -S "$f" ]] && printf '%s\n' "$f"
  done
}

# _config_dirs: every account's Claude config dir — ~/.claude (account 1) plus each
# ~/.cc/<n> plus the legacy ~/.claude[0-9]* aliases — as PHYSICAL paths, deduped, one
# per line. Cross-account transcript discovery must walk all of these: ~/.cc/3 is a
# real directory with no ~/.claudeN alias, so the old .claude[0-9]* glob silently
# missed every chat born under account 3. Symlinked accounts (~/.cc/1 → ~/.claude,
# ~/.cc/2 → ~/.claude3) collapse to one physical path via pwd -P + sort -u.
_config_dirs() {
  local cfg
  for cfg in "$HOME/.claude" "$HOME"/.cc/[0-9]* "$HOME"/.claude[0-9]*; do
    [[ -d "$cfg/projects" ]] && ( cd "$cfg" && pwd -P )
  done | sort -u
}

# _all_panes: enumerate every pane on EVERY socket. Each output row is
# "socketpath<TAB>session<TAB>pane_id<TAB>pane_current_command". The 4th field lets
# resolvers tell an AUTHORITATIVE pane from a VIEWPORT: a vsct bunker pane runs a tmux
# CLIENT attached to a chat's server, so its screen mirrors the inner statusline (🔖 and
# badge included) — pcmd==tmux marks the mirror. EVERY caller must read all 4 fields
# (read -r sock sess pane pcmd) or the command folds into $pane and breaks targeting.
# A dead or unreadable socket is skipped (the `|| true` swallows its error) and never
# aborts the scan.
_all_panes() {
  local sock line
  while IFS= read -r sock; do
    [[ -n "$sock" ]] || continue
    while IFS= read -r line; do
      [[ -n "$line" ]] && printf '%s\t%s\n' "$sock" "$line"
    done < <(tmux -S "$sock" list-panes -a -F '#{session_name}'$'\t''#{pane_id}'$'\t''#{pane_current_command}' 2>/dev/null || true)
  done < <(_sockets)
}

# _resolve_label <label>: resolve a destination by its Claude 🔖 label (the /rename
# name) to a (socket, pane) — scanning live panes on EVERY socket the way `ls` does.
# Match is case-insensitive and exact on the 🔖 text. Prints "socketpath<TAB>pane_id"
# on a UNIQUE match (return 0); prints nothing on no match (return 1); on several
# matches lists the candidates on stderr (return 2). The 🔖 name lives on the
# statusline — the line carrying both 🔖 and 🌿 — so we key on that to avoid matching
# conversation text. Returning the PANE (not its session) lets a backgrounded teammate
# that shares its orchestrator's tmux session still be addressed precisely: send-keys
# to a pane id needs no select-pane, so an inject never steals focus or breaks a zoom.
_resolve_label() {
  local want="$1" sock sess pane pcmd cap name wantlc namelc seen m; local -a matches=()
  wantlc="$(printf '%s' "$want" | tr '[:upper:]' '[:lower:]')"
  # Scan EVERY pane on EVERY socket (not just each session's active pane) — a
  # session can hold several shells, and only the Claude pane carries the 🔖
  # statusline. Per-pane capture must hit the pane's OWN socket via -S "$sock".
  # MIRROR SKIP: a pane running a tmux CLIENT (a vsct/VS Code viewport attached to a
  # chat's server) mirrors the inner chat's statusline pixel-for-pixel, so it matches
  # the 🔖 scrape and made EVERY attached chat resolve ambiguous (rc=2). A viewport is
  # never the authoritative pane — the real chat runs claude — so skip pcmd==tmux.
  while IFS=$'\t' read -r sock sess pane pcmd; do
    [[ -n "$pane" ]] || continue
    [[ "$pcmd" == tmux ]] && continue
    cap="$(tmux -S "$sock" capture-pane -t "$pane" -p -J 2>/dev/null || true)"
    # anchor on 🔖 + account badge (🥇/🥈/🥉/🍀, always emitted), NOT 🌿 (git-only — a labeled
    # chat outside a repo or in detached HEAD renders 🔖 without 🌿 and became unaddressable).
    # 🍀 is account 4 (the GPT account): it was missing here, so every cc4 chat WAS listed by
    # `ls` yet could not be addressed by its 🔖 label — the badge set must be identical in
    # every anchor (self_label, _resolve_label, and the `new` numbering scan).
    name="$(printf '%s\n' "$cap" | grep -F '🔖' | grep -E '🥇|🥈|🥉|🍀' | tail -1 | sed 's/.*🔖 *//; s/ *│.*//; s/[[:space:]]*$//' || true)"
    [[ -n "$name" ]] || continue
    namelc="$(printf '%s' "$name" | tr '[:upper:]' '[:lower:]')"
    [[ "$namelc" == "$wantlc" ]] || continue
    seen=0; for m in "${matches[@]:-}"; do [[ "$m" == "$sock"$'\t'"$pane" ]] && { seen=1; break; }; done
    [[ "$seen" == 0 ]] && matches+=("$sock"$'\t'"$pane")
  done < <(_all_panes)
  # SAME-CHAT TIE-BREAK: several matches whose crumbs all point at ONE transcript are not
  # several chats — they are one chat multi-hosted by leftover servers (an account-switch
  # relaunch leaves the elder running; cc-ls shows it as ⚠Nsrv). Refusing there made the
  # chat unaddressable by its label exactly while duplicated (WATCHER, live). Resolve to
  # the NEWEST-born server (epoch in the cc-<epoch>-… socket name) — the launch the founder
  # is typing into; Enter/⌃B in cc-ls reaps the elders. Different transcripts (or any pane
  # without a crumb) stay genuinely ambiguous — fail loud as before.
  if [[ ${#matches[@]} -gt 1 ]]; then
    local tb_uuid="" tb_ok=1 tb_best="" tb_beste=-1 tb_sock tb_pane tb_base tb_tp tb_u tb_e
    for m in "${matches[@]}"; do
      tb_sock="${m%%$'\t'*}"; tb_pane="${m#*$'\t'}"; tb_base="${tb_sock##*/}"
      tb_tp="$(cat "/tmp/cc-sid/$tb_base.$tb_pane" 2>/dev/null || cat "/tmp/cc-sid/$tb_base" 2>/dev/null || true)"
      tb_u="$(basename -- "$tb_tp" .jsonl 2>/dev/null)"
      [[ -n "$tb_u" ]] || { tb_ok=0; break; }
      if [[ -z "$tb_uuid" ]]; then tb_uuid="$tb_u"; elif [[ "$tb_u" != "$tb_uuid" ]]; then tb_ok=0; break; fi
      tb_e="$(printf '%s' "$tb_base" | cut -d- -f2)"
      [[ "$tb_e" =~ ^[0-9]+$ ]] || tb_e=0
      if [[ "$tb_e" -gt "$tb_beste" ]]; then tb_beste="$tb_e"; tb_best="$m"; fi
    done
    if [[ "$tb_ok" == 1 && -n "$tb_best" ]]; then
      echo "note: 🔖 '$want' is hosted by ${#matches[@]} servers for ONE chat ($tb_uuid) — resolving to the newest (cc-ls shows it ⚠${#matches[@]}srv; open it there to reap the elders)" >&2
      printf '%s\n' "$tb_best"; return 0
    fi
  fi
  case "${#matches[@]}" in
    1) printf '%s\n' "${matches[0]}"; return 0;;
    0) return 1;;
    *) { echo "ambiguous 🔖 label '$want' — matches panes:"; for m in "${matches[@]}"; do echo "  pane ${m#*$'\t'}  (socket ${m%%$'\t'*})"; done; } >&2; return 2;;
  esac
}

# _resolve_session <name>: resolve a destination by EXACT tmux session name across
# EVERY socket. Session names are globally unique (the launcher names each chat's
# session after its unique socket basename), so a hit is normally unique. Prints
# "socketpath<TAB>session" on a unique match (return 0); nothing on no match
# (return 1); on several lists candidates on stderr (return 2).
_resolve_session() {
  local want="$1" sock sess pane pcmd key seen m np; local -a matches=() panerows=()
  while IFS=$'\t' read -r sock sess pane pcmd; do
    [[ "$sess" == "$want" ]] || continue
    key="$sock"$'\t'"$sess"
    seen=0; for m in "${matches[@]:-}"; do [[ "$m" == "$key" ]] && { seen=1; break; }; done
    [[ "$seen" == 0 ]] && matches+=("$key")
    panerows+=("$key"$'\t'"$pane")   # keep every matching PANE, to detect a split session
  done < <(_all_panes)
  case "${#matches[@]}" in
    1)
      # A single (socket,session) — but /chat:branch and pane-mode /chat:new split a SECOND
      # claude pane INTO the same session, and send-keys to a bare session name targets the
      # session's ACTIVE pane (so a reply could land on the fork, not the parent). With more
      # than one pane, count the CLAUDE panes (comm "claude" or a bare version string —
      # same discriminator as cc-swap-chat's tty scan): exactly one → resolve to that PANE
      # id (precise, like _resolve_label — a manual shell split must not break addressing);
      # two or more (or none identifiable) → ambiguous, address by 🔖 label or %pane-id.
      np=0; for m in "${panerows[@]}"; do [[ "$m" == "${matches[0]}"$'\t'* ]] && np=$((np+1)); done
      if (( np > 1 )); then
        local msock="${matches[0]%%$'\t'*}" prow ppcmd; local -a cpanes=()
        while IFS=$'\t' read -r prow ppcmd; do
          [[ "$ppcmd" =~ ^(claude|[0-9]+\.) ]] && cpanes+=("$prow")
        done < <(tmux -S "$msock" list-panes -s -t "=$want" -F '#{pane_id}'$'\t''#{pane_current_command}' 2>/dev/null || true)
        if [[ ${#cpanes[@]} -eq 1 ]]; then
          printf '%s\t%s\n' "$msock" "${cpanes[0]}"; return 0
        fi
        echo "ambiguous session '$want' — it holds $np panes, ${#cpanes[@]} running claude (a /chat:branch or /chat:new sibling shares this session); a bare session name would target whichever pane is active. Address by 🔖 label or %pane-id instead (run /chat:ls)." >&2
        return 2
      fi
      printf '%s\n' "${matches[0]}"; return 0;;
    0) return 1;;
    *) { echo "ambiguous session '$want' — matches:"; for m in "${matches[@]}"; do echo "  ${m#*$'\t'}  (socket ${m%%$'\t'*})"; done; } >&2; return 2;;
  esac
}

# _resolve_cxwin <name>: resolve a destination CODEX chat by its human thread name.
# Codex has no 🔖 statusline (the label rung never matches) and its tmux SESSION name is
# the opaque cx-<epoch>-<pid>-<rand> socket id — the human name lives in tmux
# #{window_name}, set at spawn and kept converged with the codex thread name by cc-ls.
# Only sockets whose basename starts cx- are scanned; do NOT key on the pane command
# (codex flips between `codex` and `node` with state). Match is case-insensitive and
# exact — plus a truncation rule: cc-ls clips the window name to 24 chars, so a longer
# wanted name matches a window holding exactly its first 24. Same contract as
# _resolve_label: prints "socketpath<TAB>pane_id" on a unique match (rc 0), nothing on
# none (rc 1), candidates on stderr on several (rc 2).
_resolve_cxwin() {
  local want="$1" wantlc sock base sess pane win winlc m seen hit; local -a matches=()
  wantlc="$(printf '%s' "$want" | tr '[:upper:]' '[:lower:]')"
  while IFS= read -r sock; do
    base="${sock##*/}"
    [[ "$base" == cx-* ]] || continue
    while IFS=$'\t' read -r sess pane win; do
      [[ -n "$pane" && -n "$win" ]] || continue
      winlc="$(printf '%s' "$win" | tr '[:upper:]' '[:lower:]')"
      hit=0
      [[ "$winlc" == "$wantlc" ]] && hit=1
      [[ ${#wantlc} -gt 24 && "$winlc" == "${wantlc:0:24}" ]] && hit=1
      [[ "$hit" == 1 ]] || continue
      seen=0; for m in "${matches[@]:-}"; do [[ "$m" == "$sock"$'\t'"$pane" ]] && { seen=1; break; }; done
      [[ "$seen" == 0 ]] && matches+=("$sock"$'\t'"$pane")
    done < <(tmux -S "$sock" list-panes -a -F '#{session_name}'$'\t''#{pane_id}'$'\t''#{window_name}' 2>/dev/null || true)
  done < <(_sockets)
  case "${#matches[@]}" in
    1) printf '%s\n' "${matches[0]}"; return 0;;
    0) return 1;;
    *) { echo "ambiguous codex thread name '$want' — matches panes:"; for m in "${matches[@]}"; do echo "  pane ${m#*$'\t'}  (socket ${m%%$'\t'*})"; done; } >&2; return 2;;
  esac
}

# _self_model: map THIS session's recorded model to the launch alias for a new pane.
# The transcript records only the base id (…/claude-opus-4-8), never the [1m] suffix,
# and a bare alias drops the 1M context — so emit the alias + [1m] variant this
# environment runs (opus[1m]/sonnet[1m]); haiku/fable have no [1m] tier. Empty when
# undetectable, so the caller omits --model and the new chat boots the default.
_self_model() {
  local sid="${CLAUDE_CODE_SESSION_ID:-}" tx base
  [[ -n "$sid" ]] || return 0
  tx="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/projects/$(pwd | tr '/.' '--')/$sid.jsonl"
  [[ -f "$tx" ]] || return 0
  # Exclude the "<synthetic>" sentinel CC writes on slash-command canned replies: it matches
  # no case arm below, so landing on it would make _self_model echo nothing and branch/new
  # silently drop --model (the fork boots the harness default, not the parent's pinned model).
  base="$(jq -r 'select(.isSidechain != true and .message.model != null and .message.model != "<synthetic>") | .message.model' "$tx" 2>/dev/null | tail -1 || true)"
  case "$base" in
    *opus*)   echo "opus[1m]" ;;
    *sonnet*) echo "sonnet[1m]" ;;
    *haiku*)  echo "haiku" ;;
    *fable*)  echo "fable" ;;
  esac
}

# _spawn_envpfx: the `env …` prefix that bakes THIS chat's account + cache mode into a
# split-pane spawn (branch / pane-mode new). A tmux split inherits the tmux SERVER
# environment (account 1, harness-default 1h cache), NOT the parent chat's process env — so
# without this a fork/teammate of an account-2/3 chat silently boots on account 1 and drains
# the wrong budget, and also loses the parent's ⚡ cache mode. Also drops the parent's session
# identity so a fresh/forked pane never re-uses it. The config dir is our own $HOME path
# (single-quote-safe); the cache flags are the exact ones the launcher sets at birth.
_spawn_envpfx() {
  # GNU env parses ALL -u/options FIRST, then NAME=VALUE assignments, then the command — the
  # first NAME=VALUE ends option parsing, so a -u placed AFTER an assignment is read as the
  # command ("env: '-u': No such file or directory"). Emit every -u first, then the sets
  # (mirrors cc-open/cc-swap-chat's rpfx ordering).
  local unsets="-u CLAUDE_CODE_SESSION_ID -u CLAUDECODE" sets=""
  if [[ -n "${CLAUDE_CONFIG_DIR:-}" ]]; then sets="$sets CLAUDE_CONFIG_DIR='${CLAUDE_CONFIG_DIR}'"; else unsets="$unsets -u CLAUDE_CONFIG_DIR"; fi
  if [[ "${FORCE_PROMPT_CACHING_5M:-}" == 1 ]]; then unsets="$unsets -u ENABLE_PROMPT_CACHING_1H"; sets="$sets FORCE_PROMPT_CACHING_5M=1"
  elif [[ "${ENABLE_PROMPT_CACHING_1H:-}" == 1 ]]; then unsets="$unsets -u FORCE_PROMPT_CACHING_5M"; sets="$sets ENABLE_PROMPT_CACHING_1H=1"; fi
  printf 'env %s%s' "$unsets" "$sets"
}

# _register_pane_child <pane-id>: record a teammate PANE this chat spawned (via branch or
# new pane mode) under this chat's session uuid, as "<socket>\t<pane>", so /bb (cc-hide.sh)
# kills that pane when this chat closes. A pane teammate shares this chat's tmux server, so
# /bb must take it down by PANE — never kill-server, which would drop this chat's neighbours.
_register_pane_child() {
  local cp="$1" sock_name
  [[ -n "$cp" && -n "${CLAUDE_CODE_SESSION_ID:-}" && -n "${TMUX:-}" ]] || return 0
  sock_name="${TMUX%%,*}"; sock_name="${sock_name##*/}"
  bash "$CC_FLEET_HOME/cc-db.sh" child-add pane "$CLAUDE_CODE_SESSION_ID" "$(printf '%s\t%s' "$sock_name" "$cp")" 2>/dev/null || true
}

# _find <excerpt-file>: resolve to the past session containing the excerpt across
# every account. stdout: "<session-id>\t<transcript-path>"; report on stderr.
_find() {
  local excerpt="$1"
  [[ -f "${excerpt:-}" ]] || { echo "usage: chat.sh find <excerpt-file>" >&2; return 1; }
  local current="${CLAUDE_CODE_SESSION_ID:-__none__}"
  local registries=()
  while IFS= read -r proj; do [[ -n "$proj" ]] && registries+=("$proj"); done < <(
    while IFS= read -r cfg; do
      [[ -d "$cfg/projects" ]] || continue
      for p in "$cfg/projects"/*/; do [[ -d "$p" ]] && ( cd "$p" && pwd -P ); done
    done < <(_config_dirs) | sort -u
  )
  [[ ${#registries[@]} -gt 0 ]] || { echo "ERROR: no transcript registry under \$HOME/.claude*/projects" >&2; return 2; }

  local needles matches ranked
  needles="$(mktemp)"; matches="$(mktemp)"; ranked="$(mktemp)"
  awk '{ sub(/\r$/,""); sub(/^[ \t>#*-]+/,""); sub(/[ \t]+$/,""); if (length($0)>=20) print length($0)"\t"$0 }' "$excerpt" | sort -rn > "$ranked"
  head -5 "$ranked" | cut -f2- > "$needles"
  if [[ ! -s "$needles" ]]; then echo "ERROR: excerpt has no line of 20+ characters to search for" >&2; rm -f "$needles" "$matches" "$ranked"; return 1; fi
  local line needle reg
  while IFS= read -r line; do
    needle="$(printf '%s' "$line" | jq -Rr @json)"; needle="${needle#\"}"; needle="${needle%\"}"
    for reg in "${registries[@]}"; do grep -lF -- "$needle" "$reg"/*.jsonl 2>/dev/null || true; done
  done < "$needles" | grep -v "/$current\.jsonl$" | sort | uniq -c | sort -rn > "$matches" || true
  if [[ ! -s "$matches" ]]; then echo "NO MATCH: no session under any account contains the excerpt. Try a longer/more distinctive chunk." >&2; rm -f "$needles" "$matches" "$ranked"; return 2; fi

  local best_file best_hits sid ncount ts_all
  best_file="$(awk 'NR==1 { $1=""; sub(/^ /,""); print }' "$matches")"
  best_hits="$(awk 'NR==1 { print $1 }' "$matches")"
  sid="$(basename "$best_file" .jsonl)"
  ncount="$(wc -l < "$needles" | tr -d ' ')"
  ts_all="$(jq -r 'select(.timestamp != null) | .timestamp' "$best_file")"
  {
    echo "Matched session: $sid ($best_hits/$ncount needles hit)"
    echo "Range: ${ts_all%%$'\n'*} -> ${ts_all##*$'\n'}"
    if [[ "$(wc -l < "$matches" | tr -d ' ')" -gt 1 ]]; then echo "Other candidates (hits, file):"; head -5 "$matches" | tail -n +2; fi
  } >&2
  printf '%s\t%s\n' "$sid" "$best_file"
  rm -f "$needles" "$matches" "$ranked"
}

cmd="${1:-}"; shift || true
case "$cmd" in
  whoami)
    self_tmux
    ;;

  find)
    _find "${1:-}"
    ;;

  read)
    excerpt="${1:-}"; limit="${2:-}"
    [[ -f "$excerpt" ]] || { echo "usage: chat.sh read <excerpt-file> [last-N-lines]" >&2; exit 1; }
    match="$(_find "$excerpt")"
    sid="${match%%$'\t'*}"; best_file="${match#*$'\t'}"
    ts_all="$(jq -r 'select(.timestamp != null) | .timestamp' "$best_file")"
    mkdir -p tmp/chat-loads
    out="tmp/chat-loads/$sid.md"
    {
      echo "# Loaded chat — session $sid"; echo
      echo "Source: $best_file"
      echo "Range: ${ts_all%%$'\n'*} -> ${ts_all##*$'\n'}"
      echo "Visible chat text only — thinking and tool outputs are not recorded here."
      [[ "$limit" =~ ^[0-9]+$ ]] && echo "(last $limit lines)"
      echo
      if [[ "$limit" =~ ^[0-9]+$ ]]; then jq -r "$JQ_EXTRACT" "$best_file" | tail -n "$limit"; else jq -r "$JQ_EXTRACT" "$best_file"; fi
    } > "$out"
    echo "Extracted${limit:+ (last $limit lines)} -> $out ($(wc -l < "$out" | tr -d ' ') lines)"
    ;;

  inject)
    force_now=0; then_steers=(); msg_file=""
    while [[ "${1:-}" == --* ]]; do
      case "$1" in
        --no-sig)    echo "WARNING: --no-sig is retired — the message PREFIX decides (/-prefixed = unsigned, everything else signed); flag ignored" >&2; shift;;
        --force-now) force_now=1; shift;;
        --then)      shift; [[ -n "${1:-}" ]] || { echo "ERROR: --then needs a non-empty steer" >&2; exit 1; }; then_steers+=("$1"); shift;;
        --file)      shift; msg_file="${1:-}"; shift;;
        *) echo "ERROR: unknown inject flag '$1'" >&2; exit 1;;
      esac
    done
    # --file: read the message body from a file instead of argv. The CALLER's shell —
    # not tmux, not this script — is what mangles a message carrying shell syntax
    # (redirects, pipes, backticks, $): one imperfect quote and the caller's shell eats
    # or executes part of it. A file never crosses a shell, so command examples travel
    # intact. Use it for any message carrying shell metacharacters or spanning lines.
    if [[ -n "$msg_file" ]]; then
      [[ -f "$msg_file" ]] || { echo "ERROR: --file '$msg_file' not found" >&2; exit 1; }
      [[ $# -ge 1 ]] || { echo "usage: chat.sh inject --file <path> [--force-now] [--then <steer>]... <target>" >&2; exit 1; }
      target="$1"; shift
      msg="$(cat "$msg_file")"
    else
      [[ $# -ge 2 ]] || { echo "usage: chat.sh inject [--force-now] [--then <steer>]... [--file <path>] <self|tmux-session|🔖-label|session-id|transcript-path> <message...>" >&2; exit 1; }
      target="$1"; shift; msg="$*"
    fi
    [[ -n "$msg" ]] || { echo "ERROR: refusing to inject an empty message" >&2; exit 1; }
    # Resolve the LIVE destination to (socketpath, session). Cross-chat ops span every
    # tmux socket now (one socket per chat), so we never assume the connected/default
    # one. self/me is this chat — its socket is ${TMUX%%,*} and bare tmux is correct.
    live_tmux=""; transcript=""; socketpath=""
    if [[ "$target" == "self" || "$target" == "me" ]]; then
      [[ -n "${TMUX:-}" ]] || { echo "ERROR: this chat is not inside tmux (\$TMUX unset)" >&2; exit 1; }
      socketpath="${TMUX%%,*}"; live_tmux="$(tmux display-message -p '#{session_name}')"
    elif [[ -f "$target" ]]; then
      transcript="$(cd "$(dirname "$target")" && pwd -P)/$(basename "$target")"
    elif [[ "$target" =~ ^%[0-9]+$ ]]; then
      # A raw tmux pane id (e.g. %1) — target that exact pane. Pane ids are unique per
      # tmux server, not globally, so the socket comes from CHAT_INJECT_SOCKET (set by
      # the __then waiter re-delivering to the pane it watched) or this chat's own.
      socketpath="${CHAT_INJECT_SOCKET:-${TMUX%%,*}}"; live_tmux="$target"
    else
      # Try an exact tmux session name first (across every socket), then a 🔖 label
      # (the destination's /rename name), then fall through to a transcript/session-id
      # RESUME. Session resolution prints "socketpath<TAB>session".
      srow=""; srrc=0; srow="$(_resolve_session "$target")" || srrc=$?
      if [[ $srrc -eq 0 && -n "$srow" ]]; then
        socketpath="${srow%%$'\t'*}"; live_tmux="${srow#*$'\t'}"
      elif [[ $srrc -eq 2 ]]; then
        echo "ERROR: session '$target' is ambiguous — pass the tmux session id instead (run /chat:ls to list them)" >&2; exit 1
      else
        lrow=""; lrc=0; lrow="$(_resolve_label "$target")" || lrc=$?
        if [[ $lrc -eq 0 && -n "$lrow" ]]; then
          socketpath="${lrow%%$'\t'*}"; live_tmux="${lrow#*$'\t'}"
          echo "resolved 🔖 label '$target' → pane '$live_tmux'" >&2
        elif [[ $lrc -eq 2 ]]; then
          echo "ERROR: 🔖 label '$target' is ambiguous — pass the tmux session id instead (run /chat:ls to list them)" >&2; exit 1
        else
          # Codex rung: no 🔖 matched — try the codex thread name (tmux window name on
          # cx-* sockets) before falling through to the transcript/session-id RESUME.
          xrow=""; xrc=0; xrow="$(_resolve_cxwin "$target")" || xrc=$?
          if [[ $xrc -eq 0 && -n "$xrow" ]]; then
            socketpath="${xrow%%$'\t'*}"; live_tmux="${xrow#*$'\t'}"
            echo "resolved ⬢ codex thread '$target' → pane '$live_tmux'" >&2
          elif [[ $xrc -eq 2 ]]; then
            echo "ERROR: codex thread name '$target' is ambiguous — pass the cx-* tmux session id instead (run cc-ls to list them)" >&2; exit 1
          elif [[ "$target" =~ ^[A-Za-z0-9_-]+$ ]]; then
            transcript="$(
              while IFS= read -r cfg; do
                for p in "$cfg"/projects/*/"$target".jsonl; do
                  [[ -f "$p" ]] && ( cd "$(dirname "$p")" && printf '%s\n' "$(pwd -P)/$(basename "$p")" )
                done
              done < <(_config_dirs) | sort -u | head -1 || true
            )"
            [[ -n "$transcript" ]] || { echo "ERROR: '$target' matched no live chat — not a 🔖 label (run /chat:ls), not a codex thread name (run cc-ls), not a live tmux session, a transcript path, or a session-id — nothing to inject into" >&2; exit 1; }
          else
            echo "ERROR: '$target' matched no live chat — not a 🔖 label (run /chat:ls), not a codex thread name (run cc-ls), not a live tmux session, a transcript path, or a session-id — nothing to inject into" >&2; exit 1
          fi
        fi
      fi
    fi

    # EVERY /compact inject must carry a --then steer (founder law; chat/self/compact.md
    # owns the self dialect): compaction returns to an idle prompt — no turn fires — so
    # a steerless compact strands the target command-less. The --then waiter (__then)
    # rides out the busy→idle compaction and delivers on a quiet pane. The steer must
    # not itself be a /compact (recursion loses the thread).
    # No steer, wherever it sits in the chain, may itself be a /compact — compact-
    # steering-into-compact recurses and loses the thread. Checked upfront for ANY
    # primary (not just a /compact one): a bad chain must die here, at the caller,
    # not later in a detached waiter's log where nobody is watching.
    for s in "${then_steers[@]:-}"; do
      [[ -n "$s" ]] || continue
      if printf '%s' "$s" | grep -qE '^[[:space:]]*/compact([[:space:]]|$)'; then
        echo "ERROR: a --then steer must not itself start with /compact — compact-steering-into-compact recurses and loses the thread" >&2
        exit 1
      fi
    done
    if printf '%s' "$msg" | grep -qE '^[[:space:]]*/compact([[:space:]]|$)'; then
      if [[ ${#then_steers[@]} -eq 0 ]]; then
        echo "ERROR: a /compact inject requires --then <steer> — compaction ends at an idle prompt with no turn fired, stranding the target. Re-run: chat.sh inject --then '<post-compact steer>' <target> '/compact <focus>'" >&2
        exit 1
      fi
      # LONG-FOCUS GUARD (exit 6). A long /compact body is typed as a bracketed
      # PASTE, which the TUI collapses to "[Pasted text #N] · paste again to
      # expand" — the Enter lands on the collapsed block, the compaction never
      # fires, and the message sits in the composer as queue-limbo. Twice live.
      # The fix is also the better practice: a /compact focus is a POINTER plus
      # the few facts that must survive VERBATIM — the long hold goes to a file
      # the target reads back after compacting. A file on disk survives the
      # summary; a 2,000-character focus competes with it for the same budget.
      COMPACT_FOCUS_MAX="${COMPACT_FOCUS_MAX:-600}"
      if (( ${#msg} > COMPACT_FOCUS_MAX )); then
        echo "ABORT: /compact focus is ${#msg} chars (max ${COMPACT_FOCUS_MAX}) — a body this long is typed as a PASTE, the TUI collapses it, and the compaction never fires (queue-limbo, seen twice). Write the hold to a file and make the focus a POINTER: chat.sh inject --then '<steer>' <target> '/compact hold: read <abs path> — {2-3 facts that must survive verbatim}'. Nothing was typed." >&2
        exit 6
      fi
    fi

    # Sender signature — every injected message ends with who sent it and the
    # exact command to answer with. Identity is the SCRIPT's job, never the
    # caller's: it self-derives from this chat's own tmux session (= chat.sh
    # whoami, also the reply handle) + short session id + the 🔖 label (via
    # self_label — readable mid-run since the script never prints 🌿). The reply line
    # hands the recipient a runnable `/chat:inject {handle} <message>`, and the 🔖
    # label sits next to it so the recipient sees who sent it and can reply by label.
    # Two footer forms: a one-line one for the LIVE typed path (a bare newline
    # submits in Claude Code, so the typed message must stay single-line), and a
    # block one for the RESUME transcript (pure text, not typed).
    sender_handle="$(self_tmux 2>/dev/null || true)"
    sender_lbl="$(self_label 2>/dev/null || true)"
    sender_uuid="${CLAUDE_CODE_SESSION_ID:-}"; sender_uuid8="${sender_uuid:0:8}"
    sigparts=()
    [[ -n "$sender_uuid8" ]]  && sigparts+=("sid ${sender_uuid8}")
    [[ -n "$sender_handle" ]] && sigparts+=("to reply: /chat:inject ${sender_handle} <message>")
    [[ -n "$sender_lbl" ]]    && sigparts+=("🔖 ${sender_lbl}")
    sig=""; for p in "${sigparts[@]:-}"; do [[ -n "$p" ]] || continue; [[ -n "$sig" ]] && sig="$sig · "; sig="$sig$p"; done
    footer_inline=""; footer_block=""
    if [[ -n "$sig" ]]; then
      footer_inline="  — ${sig}"; footer_block=$'\n\n'"— ${sig}"
    else
      # Identity underivable — every derivation source came back empty (no TMUX in
      # this shell, no CLAUDE_CODE_SESSION_ID). Dropping the footer silently makes an
      # unsigned message indistinguishable from a signed one at the recipient, so the
      # absence is STATED instead: the failure renders as a failure, never as absence.
      unsig="UNSIGNED — sender identity underivable"
      [[ -z "${TMUX:-}" ]] && unsig="${unsig}; no tmux context"
      [[ -z "${CLAUDE_CODE_SESSION_ID:-}" ]] && unsig="${unsig}; no session id"
      footer_inline="  — ${unsig}"; footer_block=$'\n\n'"— ${unsig}"
      echo "WARNING: sender signature underivable (TMUX='${TMUX:-}' CLAUDE_CODE_SESSION_ID='${CLAUDE_CODE_SESSION_ID:+set}') — message sent, marked UNSIGNED." >&2
    fi
    # Signature policy (founder law): the sender signature is MANDATORY on every
    # normal prompt — an unsigned message hides who is speaking. The MESSAGE PREFIX
    # alone decides, never a caller flag: a /-prefixed prompt is a harness command
    # and travels bare (a trailing footer would corrupt its args); everything else
    # is signed. There is no opt-out flag.
    exempt=0
    msg_trim="${msg#"${msg%%[![:space:]]*}"}"
    case "$msg_trim" in
      /*) exempt=1; footer_inline=""; footer_block="";;
    esac

    if [[ -n "$live_tmux" ]]; then
      # Every tmux call in this critical section must hit the TARGET's socket, not the
      # connected/default one — so route them all through TM. live_tmux still holds the
      # session name for log lines and -t targeting (session names are globally unique).
      # (Plain array, not `local` — the case body runs at top level, not in a function.)
      TM=(tmux -S "$socketpath")
      # Serialize delivery to this target across processes — several chats may inject
      # into one pane at once and their keystrokes would interleave. Hold a per-target
      # lock for the whole interrupt/type/submit critical section; the EXIT trap frees
      # it on every exit path (success, warning, or error). The lock key is
      # socket-scoped so identically-named sessions on different sockets don't collide.
      if ! _inject_lock_acquire "${socketpath}:${live_tmux}"; then
        echo "WARNING: could not acquire the inject lock for '$live_tmux' within ${CHAT_INJECT_LOCK_TIMEOUT:-30}s — another inject is mid-delivery into it. Re-run when it frees." >&2
        exit 4
      fi
      trap '_inject_lock_release' EXIT

      # --force-now: interrupt the target's running tool/flow so it reads this NOW.
      # Press Esc only while the pane is busy (re-checking each round) — a single
      # Esc interrupts and may rewind the running turn into the input box, which the
      # empty-box guard below stashes and clears before we type. An idle target has
      # nothing to interrupt, gets no Esc, and the marker below is not appended.
      did_intr=0
      if [[ "$force_now" == 1 ]]; then
        for _ in $(seq 1 "${CHAT_INJECT_INTR_TRIES:-8}"); do
          _pane_busy "$live_tmux" "$socketpath" || break
          "${TM[@]}" send-keys -t "$live_tmux" Escape; did_intr=1
          sleep "${CHAT_INJECT_POLL:-0.2}"
        done
      fi
      # CODEX TALL-MESSAGE GUARD (2026-07-26, second false-green repro): a ~2500-char
      # inline send to a cx-* pane rendered ~40 rows against a ~50-row pane, truncated
      # MID-WORD in the composer, and defeated every composer-line scrape — the submit
      # confirm matched a stale › echo above and reported DELIVERED on an unsent brief.
      # The composer window shows the TAIL of a tall message while the prefix test keys
      # on its HEAD, so no scrape is reliable there. Founder standing law already says
      # long messages travel as FILE PATHS + a pointer line; enforce it mechanically for
      # codex targets (same shape as the /compact long-focus guard, same exit 6).
      # Claude panes keep unlimited inline — their [Pasted text] collapse is handled.
      if [[ "${socketpath##*/}" == cx-* ]]; then
        CX_INLINE_MAX="${CHAT_INJECT_CX_INLINE_MAX:-1500}"
        if (( ${#msg} > CX_INLINE_MAX )); then
          echo "ABORT: message is ${#msg} chars — over the ${CX_INLINE_MAX}-char inline cap for codex panes. A tall message overflows the codex composer and every submit-confirm scrape goes blind (false DELIVERED on an unsent brief, seen live twice). Write the body to a file and inject a short pointer: chat.sh inject $target 'READ AND EXECUTE: <abs path> — <one-line summary>'. Nothing was typed." >&2
          exit 6
        fi
      fi
      # Everything else goes inline — no file, no pointer, no length cap. A /-prefixed
      # command sends verbatim (a footer/marker would corrupt its args); plain text
      # gets the force marker (only if we actually interrupted) plus
      # the one-line footer appended. The settle-poll below confirms the whole
      # message — however long — actually rendered before Enter, so a long inline
      # send is reliable and a genuine jam is caught instead of half-sent.
      force_mark=""
      [[ "$did_intr" == 1 ]] && force_mark=" — ⚠ FORCE-DELIVERED via Esc (your running flow was interrupted; re-check any in-progress action)"
      # Founder law: /-prefixed harness commands (decided above) travel bare;
      # every plain-text message is signed.
      if [[ "$exempt" == 1 ]]; then :; else msg="${msg}${force_mark}${footer_inline}"; fi

      # Normalize the input surface so keystrokes actually land — a pane in copy/scroll
      # mode or sitting in the Rewind menu silently eats input (a common "Enter didn't
      # land" cause). Exit copy-mode; cancel a Rewind menu with a single Esc.
      if [[ "$("${TM[@]}" display-message -t "$live_tmux" -p '#{pane_in_mode}' 2>/dev/null || echo 0)" == 1 ]]; then
        "${TM[@]}" send-keys -t "$live_tmux" -X cancel 2>/dev/null || true
      fi
      if "${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null | grep -qiF 'Restore the code'; then
        "${TM[@]}" send-keys -t "$live_tmux" Escape; sleep "${CHAT_INJECT_POLL:-0.2}"
      fi

      # CODEX PLAN-OVERLAY GUARD (2026-07-26): codex sometimes raises a "Create a
      # plan?" prompt-overlay that SWALLOWS the next Enter (it answers the overlay
      # instead of submitting the composer) — three briefs sat unsent behind it in
      # one dispatch. Dismiss with a single Esc before typing, same shape as the
      # Rewind guard above. Bounded to the last 12 lines so an old conversation
      # echo of the phrase can't trigger a spurious Esc; Esc on an idle composer
      # is a no-op.
      if "${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null | tail -12 | grep -qiE 'create a plan\?'; then
        "${TM[@]}" send-keys -t "$live_tmux" Escape; sleep "${CHAT_INJECT_POLL:-0.2}"
      fi

      # COPY-MODE GUARD: a pane sitting in tmux copy-mode (the scrollback view — a
      # scroll-wheel or stray key opens it) EATS every typed key as a copy-mode
      # binding: the inject would vanish, and stray letters raise jump prompts
      # ("(jump to forward)") instead of reaching the composer. Cancel the view
      # first — it only closes the scrollback overlay; the underlying composer
      # and any draft are untouched.
      if [[ "$("${TM[@]}" display-message -t "$live_tmux" -p '#{pane_in_mode}' 2>/dev/null || true)" == "1" ]]; then
        "${TM[@]}" send-keys -t "$live_tmux" -X cancel
        sleep "${CHAT_INJECT_POLL:-0.2}"
      fi

      # SELECTOR GUARD (both engines): an open AskUserQuestion/permission/approval menu
      # treats Enter as SELECT — injecting would forge an answer on the target's behalf
      # (menus gate security authorizations, incl. Codex's exec/patch-approval modals).
      # Abort BEFORE typing; exit 4 = selector-open, retry after the menu's owner answers/
      # dismisses it (distinct from exit 3 typed-not-submitted).
      #   Claude marks the selected option with ❯; the LAST '❯' on a numbered option row
      #   (not a bare input line) is a menu. A pane with NO '❯' is a foreign/idle UI.
      #   Codex marks it with › (U+203A) — the SAME glyph as its normal composer prompt
      #   AND its sent-message echo, so a bare › is NOT a menu (a lone "› 1. do this" draft
      #   must inject fine). Discriminate a real Codex menu: its LAST › sits on a numbered
      #   option AND either a menu confirm/cancel hint is present (every Codex modal prints
      #   one — "Press enter to confirm … esc to cancel") OR ≥2 numbered option rows are
      #   stacked (a menu always offers several; a numbered draft is one line). The hint OR
      #   the option-count keeps the guard robust if Codex reword the hint. || true keeps a
      #   failing grep from killing the script (set -e).
      guard_cap="$("${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null || true)"
      cc_sel="$(printf '%s\n' "$guard_cap" | grep -F '❯' | tail -1 || true)"
      cx_sel="$(printf '%s\n' "$guard_cap" | grep -F '›' | tail -1 || true)"
      menu_hit=""
      if printf '%s' "$cc_sel" | grep -qE '❯[[:space:]]*[0-9]+\.[[:space:]]'; then
        menu_hit="$cc_sel"
      elif printf '%s' "$cx_sel" | grep -qE '›[[:space:]]*[0-9]+\.[[:space:]]'; then
        cx_nopt="$(printf '%s\n' "$guard_cap" | grep -cE '^[[:space:]]*[›]?[[:space:]]*[0-9]+\.[[:space:]]' || true)"
        if printf '%s\n' "$guard_cap" | grep -qiE 'enter to (confirm|continue|select)|esc to (cancel|go back)' || (( ${cx_nopt:-0} >= 2 )); then
          menu_hit="$cx_sel"
        fi
      fi
      if [[ -n "$menu_hit" ]]; then
        echo "ABORT: '$live_tmux' has an OPEN selector menu (marker on a numbered option: $(printf '%s' "$menu_hit" | sed 's/^[[:space:]]*//')) — an inject would press Enter INTO the menu and forge an answer. Nothing was typed. Re-run after the menu's owner answers or dismisses it." >&2
        exit 4
      fi

      # PRE-INJECT draft safety: send-keys -l appends at the cursor, so typing onto an
      # unsent draft would submit the mash-up. Ctrl+S stashes any draft (the box clears
      # and the draft auto-restores after the next submit) and is a NO-OP on an empty
      # box — so we press it unconditionally and never need to read the input. A draft
      # was stashed iff the "stashed" indicator then appears near the prompt; that
      # decides single vs double Enter below (a restored draft must not be re-submitted).
      "${TM[@]}" send-keys -t "$live_tmux" C-s
      sleep "${CHAT_INJECT_POLL:-0.2}"
      saved_draft=0
      "${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null | tail -8 | grep -qi 'stashed' && saved_draft=1

      # MASH GUARD: one C-s is not proof — a stash-restore from the PREVIOUS inject can
      # race this one (the restored draft pops back after our C-s already fired), and
      # transient pane states can leave the binding inert. Typing onto a live draft
      # submits a mash-up as the TARGET's words. Verify the composer is EMPTY (bare ❯,
      # the pane's LAST ❯ line) before typing; a lingering draft gets stash retries,
      # then a hard abort — never typed over. ❯-less (foreign) panes skip: no Claude
      # composer to guard.
      draft_line="$("${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null | grep -F '❯' | tail -1 || true)"
      if [[ -n "$draft_line" ]]; then
        # A draft is REAL only if it carries a printable ASCII char — the empty
        # composer's line can hold invisible artifacts (cursor glyph, NBSP) that
        # [[:space:]] never strips in the C locale — and the queued-messages hint
        # ("Press up to edit queued messages") is placeholder text on the ❯ line,
        # never a draft: strip it before the test or every inject to a target with
        # a queued backlog false-aborts exit 5.
        _has_draft() { printf '%s' "$1" | sed 's/^[[:space:]]*❯[[:space:]]*//; s/Press up to edit queued messages//' | LC_ALL=C grep -qE '[[:graph:]]'; }
        # Dim-styled (SGR 2) ❯-line text is a HARNESS PLACEHOLDER (contextual suggested
        # reply, CC ≥2.1.205) — never a buffer draft: C-s cannot stash it (the buffer is
        # empty) and typing replaces it, so it must not trip the guard. Discriminate by
        # styling in an escape-preserving capture: a real typed draft renders undimmed.
        # Placeholder iff the styled line carries an SGR-2 span AND stripping the dim
        # spans + all SGRs + the queued hint leaves no printable text after ❯.
        # 2026-07-13: dynamic placeholders false-aborted (exit 5) every inject to any
        # pane showing one, while the unconditional first C-s had already stashed real
        # drafts underneath — a self-deepening delivery trap.
        _is_placeholder() {
          local styled esc; esc=$'\x1b'
          styled="$("${TM[@]}" capture-pane -t "$live_tmux" -p -e -J 2>/dev/null | grep -F '❯' | tail -1 || true)"
          [[ "$styled" == *"${esc}[2m"* ]] || return 1
          printf '%s' "$styled" | LC_ALL=C sed -E "s/${esc}\[2m[^${esc}]*//g; s/${esc}\[[0-9;]*m//g; s/^[[:space:]]*❯[[:space:]]*//; s/Press up to edit queued messages//" | LC_ALL=C grep -qE '[[:graph:]]' && return 1
          return 0
        }
        for _ in $(seq 1 "${CHAT_INJECT_STASH_TRIES:-4}"); do
          _has_draft "$draft_line" || break
          _is_placeholder && break
          "${TM[@]}" send-keys -t "$live_tmux" C-s
          sleep "${CHAT_INJECT_POLL:-0.2}"
          "${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null | tail -8 | grep -qi 'stashed' && saved_draft=1
          draft_line="$("${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null | grep -F '❯' | tail -1 || true)"
        done
        if _has_draft "$draft_line" && ! _is_placeholder; then
          rest="$(printf '%s' "$draft_line" | sed 's/^[[:space:]]*❯[[:space:]]*//; s/[[:space:]]*$//')"
          echo "ABORT: '$live_tmux' composer holds a draft that will not stash (mash guard): ${rest:0:80} — nothing typed; the draft is preserved, re-run when the composer clears. (Dim SGR-2 placeholder text is exempt — this text rendered UNDIMMED, i.e. a real draft.)" >&2
          exit 5
        fi
      fi

      "${TM[@]}" send-keys -t "$live_tmux" -l -- "$msg"
      # A distinctive tail of the message — used to confirm the text rendered AND
      # to tell "still in the input box" from "submitted". On submit the input line
      # clears, but the message stays visible UP in the conversation, so the
      # submit check looks at the input line only (the LAST '❯'), never the whole pane.
      needle="$(printf '%s' "$msg" | tr -s '[:space:]' ' ' | sed 's/^ //; s/ *$//')"; _fullneedle="$needle"; needle="${needle: -40}"
      # a message under 40 chars slices to EMPTY, and *""* matches any capture — the settle
      # loop would break on iteration 1 without ever confirming the render. Fall back to the
      # whole (normalized) message as the needle (mirrors cc-swap-chat.sh's guard).
      [[ -n "$needle" ]] || needle="$_fullneedle"
      # 1) Wait for the full text to render before the first Enter (advisory). NEVER
      #    bail on a miss — a typed-but-unsubmitted message is the worst outcome, and
      #    detection can flake on wrapping/footer glyphs even when the text is there.
      for _ in $(seq 1 "${CHAT_INJECT_SETTLE_TRIES:-40}"); do
        _inject_lock_beat   # keep the lock fresh so a slow-but-live hold isn't stolen
        cap="$("${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null | tr -s '[:space:]' ' ')"
        [[ "$cap" == *"$needle"* ]] && break
        sleep "${CHAT_INJECT_POLL:-0.2}"
      done
      # 2) Submit and CONFIRM the Enter actually landed (positive safety net). Press
      #    Enter; with NO stashed draft press a second Enter CHAT_INJECT_ENTER_GAP
      #    (0.15s) later to defeat a swallowed first one (an empty box ignores it);
      #    with a stashed draft press ONCE (the stash restores in <0.3s — a second
      #    Enter would re-submit it). Then settle CHAT_INJECT_ENTER_SETTLE (0.4s) and
      #    check the PLAIN input line (color-stripped, so syntax-highlighting of the
      #    message/footer can't fool it) for the message's LEADING text: while that
      #    prefix is still sitting in the input the turn did NOT submit, so press
      #    again. Test the INPUT LINE only (last ❯) — the submitted message stays
      #    visible UP in the conversation. The CALLER never presses Enter — this owns it.
      prefix="$(printf '%s' "$msg" | tr -s '[:space:]' ' ' | sed 's/^ //')"; prefix="${prefix:0:24}"
      submitted=0
      for _ in $(seq 1 "${CHAT_INJECT_ENTER_TRIES:-12}"); do
        _inject_lock_beat   # keep the lock fresh so a slow-but-live hold isn't stolen
        "${TM[@]}" send-keys -t "$live_tmux" Enter
        if [[ "$saved_draft" == 0 ]]; then
          sleep "${CHAT_INJECT_ENTER_GAP:-0.15}"
          "${TM[@]}" send-keys -t "$live_tmux" Enter
        fi
        sleep "${CHAT_INJECT_ENTER_SETTLE:-0.4}"
        _cap="$("${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null || true)"
        input_line="$(printf '%s\n' "$_cap" | grep -F '❯' | tail -1 || true)"
        # The Codex TUI composer prompt is › (U+203A), not Claude's ❯. When no ❯ is
        # present the submit-confirm was blind: input_line came back EMPTY, the prefix
        # check passed on iteration 1, and the loop declared success while the message
        # sat UNSENT in the codex composer (the founder had to press Enter every time).
        # ❯-first / ›-fallback keeps Claude behaviour byte-identical and only extends
        # the confirm to codex panes.
        [[ -n "$input_line" ]] || input_line="$(printf '%s\n' "$_cap" | grep -F '›' | tail -1 || true)"
        # EMPTY prompt-line match is UNKNOWN, never SUBMITTED (2026-07-26, live in
        # fleet): a message TALLER THAN THE PANE scrolls the ❯/› prompt off the
        # visible screen, both greps return empty, and the old guard read that
        # absence as "prefix gone → submitted" while the whole brief sat UNSENT in
        # the composer — an error rendering as absence inside the delivery mechanism
        # itself. Re-capture WITH SCROLLBACK (-S) to find the real composer line
        # (-J joins wrapped lines, so the composer reads as one "›<full message>"
        # row the prefix test can see). Still nothing → we know NOTHING: retry, and
        # let the exit-3 warning report honestly. This also retires the old
        # foreign-pane best-effort (empty ⇒ submitted) — an unconfirmable pane now
        # exits 3 with the text still in its input, which is the truthful outcome.
        if [[ -z "$input_line" ]]; then
          _cap="$("${TM[@]}" capture-pane -t "$live_tmux" -p -J -S "-${CHAT_INJECT_SCROLLBACK:-300}" 2>/dev/null || true)"
          input_line="$(printf '%s\n' "$_cap" | grep -F '❯' | tail -1 || true)"
          [[ -n "$input_line" ]] || input_line="$(printf '%s\n' "$_cap" | grep -F '›' | tail -1 || true)"
          [[ -n "$input_line" ]] || continue
        fi
        # A long message collapses in the composer to "[Pasted text #N]" — that line
        # does NOT contain the message prefix, so the prefix check alone reads it as
        # submitted while the text sits UNSENT. A collapsed paste on the input line is
        # NOT submitted: keep pressing Enter until the placeholder leaves the composer.
        [[ "$input_line" == *"[Pasted text"* ]] && continue
        [[ "$input_line" != *"$prefix"* ]] && { submitted=1; break; }
      done
      if [[ "$submitted" == 1 ]]; then
        note=""
        [[ "$did_intr" == 1 ]]    && note="${note} — FORCE-NOW: interrupted the target's running flow"
        [[ "$saved_draft" == 1 ]] && note="${note} — stashed and restored the target's unsent draft"
        if [[ ${#then_steers[@]} -gt 0 ]]; then
          # --then: deliver follow-up steers once the primary turn settles, IN ORDER —
          # each steer is its own turn. The primary is built for /compact, which
          # leaves the pane idle — a steer typed while compaction runs is swallowed,
          # so the waiter rides out the busy→idle transition first, delivers the FIRST
          # steer, and passes the remainder as --then flags on that recursive inject:
          # the chain re-arms itself one confirmed delivery at a time, so steer N+1
          # always waits out steer N's whole turn. It must be DETACHED: for a
          # self-inject the waiter runs inside the very turn it waits on, and that
          # turn cannot end until we return, so a synchronous wait would hang. A
          # reparented background process outlives the turn and delivers via a fresh
          # inject (its own per-target lock). Release our lock now so the waiter is
          # never blocked by us.
          _inject_lock_release; trap - EXIT
          steer_log="${TMPDIR:-/tmp}/chat-then-$(printf '%s' "$live_tmux" | tr -c 'A-Za-z0-9' _).log"
          # A fresh chain truncates the log; a HOP (this inject was exec'd by a __then
          # waiter, marked by CHAT_THEN_CHAIN) appends — truncating here would wipe the
          # chain's earlier hops WHILE the exec'ing inject still writes into the same
          # file, garbling the record (seen in fixture).
          if [[ -n "${CHAT_THEN_CHAIN:-}" ]]; then
            $(cc_detach) bash "$0" __then "$socketpath" "$live_tmux" "${then_steers[@]}" >>"$steer_log" 2>&1 </dev/null &
          else
            $(cc_detach) bash "$0" __then "$socketpath" "$live_tmux" "${then_steers[@]}" >"$steer_log" 2>&1 </dev/null &
          fi
          note="${note} — ${#then_steers[@]} --then steer(s) queued; deliver in order, one settled turn apart (log: $steer_log)"
        fi
        # Delivery proof — capture the target pane right after the send so the caller
        # (often an orchestrator that has zoomed this teammate into the background)
        # sees the message landed without un-zooming to look. A brief settle lets the
        # new turn register first. The proof VERIFIES, never just displays: if the
        # composer line in this very capture still holds the collapsed paste or the
        # message prefix, the submit verdict above was a false positive — report
        # NOT-DELIVERED loudly and exit nonzero instead of printing a success banner
        # over a screen that contradicts it.
        sleep "${CHAT_INJECT_PROOF_SETTLE:-0.5}"
        proof_cap="$("${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null)"
        proof_input="$(printf '%s\n' "$proof_cap" | grep -F '❯' | tail -1 || true)"
        # ›-fallback + scrollback here too (2026-07-26): the proof read only ❯, so a
        # codex pane's proof was ALWAYS empty and the contradiction check could never
        # fire — the same blindness the submit loop had. Empty visible screen → look
        # into scrollback before concluding; a proof that finds NO composer line
        # anywhere reports its own blindness in the banner instead of implying green.
        [[ -n "$proof_input" ]] || proof_input="$(printf '%s\n' "$proof_cap" | grep -F '›' | tail -1 || true)"
        if [[ -z "$proof_input" ]]; then
          _pcap2="$("${TM[@]}" capture-pane -t "$live_tmux" -p -J -S "-${CHAT_INJECT_SCROLLBACK:-300}" 2>/dev/null || true)"
          proof_input="$(printf '%s\n' "$_pcap2" | grep -F '❯' | tail -1 || true)"
          [[ -n "$proof_input" ]] || proof_input="$(printf '%s\n' "$_pcap2" | grep -F '›' | tail -1 || true)"
        fi
        proof_note=""
        [[ -z "$proof_input" ]] && proof_note=" (proof caveat: no composer line found in capture+scrollback — submit was positively confirmed above, but this proof is blind; verify the target pane if in doubt)"
        if [[ "$proof_input" == *"[Pasted text"* || ( -n "$proof_input" && "$proof_input" == *"$prefix"* ) ]]; then
          echo "PROOF-CONTRADICTION: '$live_tmux' composer STILL holds the message (${proof_input:0:80}) — NOT delivered despite the submit check. Text remains queued in its input; re-run inject when the pane is idle, or press Enter in that pane." >&2
          echo "--- delivery proof (FAILED): screen of '$live_tmux' ---" >&2
          printf '%s\n' "$proof_cap" | sed 's/[[:space:]]*$//' | grep -v '^$' | tail -"${CHAT_INJECT_PROOF_LINES:-20}" >&2
          echo "--- end proof ---" >&2
          exit 4
        fi
        echo "injected LIVE into '$live_tmux' — typed inline and submitted (Enter confirmed, input cleared)${note}${proof_note}"
        echo "--- delivery proof: screen of '$live_tmux' ---"
        printf '%s\n' "$proof_cap" | sed 's/[[:space:]]*$//' | grep -v '^$' | tail -"${CHAT_INJECT_PROOF_LINES:-20}"
        echo "--- end proof ---"
        exit 0
      fi
      echo "WARNING: typed the text into '$live_tmux' but could not confirm submission after ${CHAT_INJECT_ENTER_TRIES:-12} Enter attempts — the pane is likely busy mid-turn or in a selector. The text is in its input and will send on the next Enter; re-run when it is idle at '❯'." >&2
      exit 3
    fi

    [[ "$force_now" == 1 ]] && echo "WARNING: --force-now ignored — '$target' is not a live pane (a dormant chat has no running flow to interrupt); delivering via transcript RESUME." >&2
    [[ ${#then_steers[@]} -gt 0 ]] && echo "WARNING: --then ignored (${#then_steers[@]} steer(s)) — '$target' is not a live pane; there is no turn to wait on. Deliver the steers yourself after the chat resumes and compacts." >&2
    msg="${msg}${footer_block}"
    # DAEMON-AGENT GUARD: a background-agent session (claude daemon-hosted, listed in
    # `claude agents`) is LIVE but has no tmux pane, so it fell through to this dormant path.
    # A running CC process reads stdin, not its jsonl, and keeps chaining events off its own
    # in-memory tail — so appending a turn here is a dangling side-branch it never reads: the
    # steer is silently lost behind a success banner. Refuse if the uuid is in ANY account's
    # agent registry, and point the caller at the takeover path (cc-open → cc-agent-open.sh).
    dm_uuid="$(basename -- "$transcript" .jsonl 2>/dev/null)"
    if [[ "$dm_uuid" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; then
      # LIVE-PROCESS GUARD: the daemon registry below only knows `claude agents` daemons —
      # a claude.ai BACKGROUND JOB (~/.claude/jobs/*) or a pane chat targeted by raw uuid
      # is just as live and just as pane-less on this path, and the append below would be
      # a dangling side-branch its running process never reads: the message is silently
      # lost behind a success banner. Detect liveness directly — cc_session_live returns the pid
      # whose environment carries CLAUDE_CODE_SESSION_ID=<target uuid> on Linux, or the tmux
      # socket still hosting it on any platform. Refuse loudly instead. Its answer is evidence
      # of life, never proof of death, which is why the daemon-registry query below still runs.
      lp="$(cc_session_live "$dm_uuid")"
      if [[ -n "$lp" ]]; then
        echo "ERROR: '$target' is a LIVE session (running at $lp) with no resolvable tmux pane on this path — a running Claude reads its input stream, not its transcript, so appending a turn here would be silently lost behind a success banner. If it is a pane chat, inject by its tmux session or 🔖 label (/chat:ls); if it is a background job or daemon agent, reach it through its own surface (claude.ai / cc-open). Nothing was appended." >&2
        exit 1
      fi
      dm_bin="$(command -v claude || echo "$HOME/.local/bin/claude")"
      while IFS= read -r dm_cfg; do
        [[ -n "$dm_cfg" ]] || continue
        dm_json="$(export CLAUDE_CONFIG_DIR="$dm_cfg"; cc_timeout 15 "$dm_bin" agents --json 2>/dev/null || true)"
        if printf '%s' "$dm_json" | jq -e --arg u "$dm_uuid" '(type=="array") and (map(select((.sessionId==$u) or (.id!=null and (.id as $i|$u|startswith($i))))) | length > 0)' >/dev/null 2>&1; then
          echo "ERROR: '$target' is a LIVE background agent (daemon-hosted, config $dm_cfg) — it has no tmux pane and reads stdin, not its transcript, so appending a turn here would be a dangling side-branch it never sees. Take it over instead: cc-open $dm_uuid (routes through cc-agent-open.sh), or attach its agent view. Nothing was appended." >&2
          exit 1
        fi
      done < <(_config_dirs)
    fi
    tail_event="$(jq -c 'select(.uuid != null)' "$transcript" | tail -1)"
    [[ -n "$tail_event" ]] || { echo "ERROR: no uuid-bearing event in $transcript" >&2; exit 1; }
    parent_uuid="$(printf '%s' "$tail_event" | jq -r '.uuid')"
    session_id="$(printf '%s' "$tail_event" | jq -r '.sessionId // empty')"
    new_uuid="$(uuidgen | tr 'A-Z' 'a-z')"; prompt_id="$(uuidgen | tr 'A-Z' 'a-z')"
    ts="$(date -u +%Y-%m-%dT%H:%M:%S.000Z)"
    backup_dir="$HOME/.claude-sessions/.chat-inject-backups"; mkdir -p "$backup_dir"
    backup="$backup_dir/${session_id:-unknown}-$(date -u +%Y%m%dT%H%M%SZ).jsonl"
    cp -p "$transcript" "$backup"
    new_event="$(printf '%s' "$tail_event" | jq -c \
      --arg uuid "$new_uuid" --arg parent "$parent_uuid" --arg pid "$prompt_id" --arg ts "$ts" --arg msg "$msg" '{
        type:"user", userType:"external", entrypoint:"cli",
        cwd:.cwd, sessionId:.sessionId, version:.version, gitBranch:(.gitBranch // ""),
        parentUuid:$parent, uuid:$uuid, promptId:$pid, timestamp:$ts,
        isSidechain:false, isMeta:false, message:{role:"user", content:$msg} }')"
    [[ -n "$new_event" ]] || { echo "ERROR: failed to build injected event" >&2; exit 1; }
    printf '%s\n' "$new_event" >> "$transcript"
    echo "injected user turn $new_uuid (parent $parent_uuid) into session ${session_id:-?} — answered on that chat's next RESUME (no live pane found)"
    echo "backup: $backup"
    ;;

  __then)
    # internal (detached): deliver --then follow-up steers once the target's current
    # turn — typically a /compact compaction — has settled to idle. Args:
    # <socketpath> <target> <steer> [steer...], where <target> is whatever the parent
    # inject resolved — a pane id for a label/pane target, a session name otherwise.
    # Waits for busy→stable-idle so the FIRST steer lands on a quiet pane, then
    # re-injects it (own lock) with any REMAINING steers passed through as --then
    # flags: that inject spawns the NEXT waiter only after its own delivery confirms,
    # so an N-steer chain delivers in order, one settled turn apart, each hop a fresh
    # detached process. The socket rides env so a pane-id target hits the right server.
    socketpath="${1:-}"; target="${2:-}"
    shift 2 2>/dev/null || true
    [[ -n "$target" && $# -ge 1 && -n "${1:-}" ]] || { echo "ERROR: __then needs <socketpath> <target> <steer> [steer...]" >&2; exit 1; }
    then_steer="$1"; shift
    # 1) let the primary turn take hold (ride past any turn-end → /compact-start gap)
    sleep "${CHAT_THEN_MIN:-1.5}"
    # 2) wait for it to start (pane busy), bounded so a no-op turn can't stall us
    for _ in $(seq 1 "${CHAT_THEN_BUSY_TRIES:-25}"); do
      _pane_busy "$target" "$socketpath" && break
      sleep "${CHAT_INJECT_POLL:-0.2}"
    done
    # 3) wait for it to finish — idle must hold steady, since compaction shows brief
    #    stalls that would otherwise read as done
    stable=0; need="${CHAT_THEN_IDLE_STABLE:-3}"
    for _ in $(seq 1 "${CHAT_THEN_IDLE_TRIES:-1500}"); do
      if _pane_busy "$target" "$socketpath"; then stable=0; else stable=$((stable+1)); fi
      (( stable >= need )) && break
      sleep "${CHAT_THEN_IDLE_POLL:-0.4}"
    done
    sleep "${CHAT_THEN_SETTLE:-0.4}"
    # CHAT_THEN_CHAIN marks the exec'd inject as a chain hop: its own --then spawn
    # (for the remaining steers) APPENDS to the chain log instead of truncating it.
    if [[ $# -gt 0 ]]; then
      rest=(); for s in "$@"; do rest+=(--then "$s"); done
      CHAT_INJECT_SOCKET="$socketpath" CHAT_THEN_CHAIN=1 exec "$0" inject "${rest[@]}" "$target" "$then_steer"
    fi
    CHAT_INJECT_SOCKET="$socketpath" CHAT_THEN_CHAIN=1 exec "$0" inject "$target" "$then_steer"
    ;;

  ls)
    # Default: chats whose cwd is inside THIS repo. --all: every live chat on the box,
    # each row carrying its dir — cross-project discovery (inject/capture already
    # resolve globally, so any session listed here is directly addressable).
    all=0
    case "${1:-}" in --all|-a|all) all=1;; esac
    repo="$(repo_root)"; self_sock=""; self_sess=""
    if [[ -n "${TMUX:-}" ]]; then
      self_sock="${TMUX%%,*}"; self_sess="$(tmux display-message -p '#{session_name}' 2>/dev/null || true)"
    fi
    if [[ "$all" == 1 ]]; then echo "live chats everywhere (session · state · dir · last activity):"
    else echo "live chats in this repo (session · state · last activity):"; fi
    found=0; elsewhere=0; hiddenrows=0
    # A hidden chat is hidden EVERYWHERE, not just in cc-ls's picker: hiding says "this one is
    # done with me", and a listing that keeps offering it makes the hide meaningless. Read the
    # same set cc-ls reads, in the same precedence — the file is the hidden set as it stands
    # between picker runs and can be one ⌃X ahead of the db, so it wins when it exists.
    hidefile="$HOME/.claude/.cc-ls-hidden"
    if [[ -f "$hidefile" ]]; then hidden_ids="$(grep . "$hidefile" 2>/dev/null || true)"
    else hidden_ids="$(bash "$CC_FLEET_HOME/cc-db.sh" hidden-list 2>/dev/null || true)"; fi
    # Enumerate panes on EVERY socket (one socket per chat now); each row is prefixed
    # with its socket so per-pane capture/display hit the right server. A dead socket
    # is skipped (|| true), never aborting the listing. Self-match compares BOTH socket
    # and session — session names are globally unique, so the session stays the handle.
    while IFS='|' read -r sock sess path pcmd wactive pactive paneid panepid win; do
      [[ "$wactive" == "1" && "$pactive" == "1" ]] || continue
      # Codex rows: gated on the cx-* SOCKET basename, never on bare `node` (that would
      # list unrelated node processes); the codex TUI renders as `codex` or `node` (its
      # wrapper) with state. Claude rows keep the original filter untouched.
      codexrow=0; sockbase="${sock##*/}"
      if [[ "$sockbase" == cx-* ]]; then
        case "$pcmd" in node | codex) codexrow=1 ;; *) continue ;; esac
      elif [[ "$sockbase" == cc-* ]]; then
        # A cc-* socket hosts exactly one Claude chat by launcher convention, and the
        # pane command is NOT a reliable discriminator for it: a chat launched under a
        # non-exec'ing shell wrapper reports its parent shell (zsh) forever, and the
        # old claude-only filter dropped that row — `ls` denied a LIVE attached chat
        # existed while whoami/capture/label-resolve all found it (2026-07-26: the
        # WATCHER itself, nearly ruled dead on the empty listing). The socket naming
        # is the discriminator; only a tmux client (viewport mirror) is excluded,
        # mirroring _resolve_label's pcmd==tmux skip.
        case "$pcmd" in tmux) continue ;; *) ;; esac
      else
        case "$pcmd" in [0-9]* | claude) ;; *) continue ;; esac
      fi
      # Drop hidden chats before anything else counts them — a hidden chat must not even swell
      # the "+N live in other dirs" tally. Each engine answers "which chat is this pane" its own
      # way: Claude leaves a pane-keyed breadcrumb, codex records the thread in its store and is
      # matched by birth time (cx_thread_id). A pane whose id will not resolve is LISTED, never
      # dropped — failing open keeps a live chat visible, and that is the safe direction.
      rowid=""
      if [[ "$sockbase" == cx-* ]]; then
        rowid="$(cx_thread_id "$path" "$(cc_pstart "$panepid" 2>/dev/null)" 2>/dev/null || true)"
      else
        for crumb in "/tmp/cc-sid/$sockbase.$paneid" "/tmp/cc-sid/$sockbase"; do
          [[ -r "$crumb" ]] || continue
          rowid="$(basename -- "$(cat "$crumb" 2>/dev/null)" .jsonl 2>/dev/null)"
          [[ -n "$rowid" ]] && break
        done
      fi
      if [[ -n "$rowid" ]] && printf '%s\n' "$hidden_ids" | grep -qxF -- "$rowid"; then
        hiddenrows=$((hiddenrows + 1)); continue
      fi
      in_repo=1; case "$path" in "$repo" | "$repo"/*) ;; *) in_repo=0 ;; esac
      [[ "$all" == 0 && "$in_repo" == 0 ]] && { elsewhere=$((elsewhere + 1)); continue; }
      if [[ "$codexrow" == 1 ]]; then
        # Codex: no Claude statusline to scrape — the human thread name lives in the
        # tmux window name (cc-fleet sets and converges it). ⬢ marks the engine.
        state="⬢"; topic="$win"
      else
        cap="$(tmux -S "$sock" capture-pane -t "$sess" -p 2>/dev/null | sed 's/[[:space:]]*$//' || true)"
        state="$(printf '%s\n' "$cap" | grep -oE '🟢|⚡|🔴' | tail -1 || true)"; state="${state:-·}"
        # Statusline rows must never become the "topic" — they are chrome, not conversation.
        # ☁️ (cost) / 🔖 (label) / 🧮 (tokens) / the account badges join the older markers:
        # the cost row carries none of 💰|⏱|🟢 and an UNLABELED chat's line 1 carries no 🔖,
        # so both survived the filter and rows printed statusline chrome instead of the
        # chat's last line. Every badge is listed — 🥇🥈🥉 (Claude accounts 1-3) and 🍀
        # (account 4, GPT) — since line 1 always carries exactly one of them.
        topic="$(printf '%s\n' "$cap" | grep -vE '^$|🟢|⚡|🔴|☁️|🔖|🧮|🥇|🥈|🥉|🍀|auto mode on|shift\+tab|esc to interrupt|to scroll|for agents|^[─╭╮╰╯│]|^❯$|💾|💰|⏱' | tail -1 | cut -c1-64 || true)"
      fi
      mark=""; [[ "$sock" == "$self_sock" && "$sess" == "$self_sess" ]] && mark="  <- this chat"
      loc=""; [[ "$all" == 1 ]] && loc="${path/#$HOME/\~}  "
      printf '  %-6s %s  %s%s%s\n' "$sess" "$state" "$loc" "$topic" "$mark"; found=$((found + 1))
    done < <(
      while IFS= read -r sock; do
        [[ -n "$sock" ]] || continue
        tmux -S "$sock" list-panes -a -F "$sock"'|#{session_name}|#{pane_current_path}|#{pane_current_command}|#{window_active}|#{pane_active}|#{pane_id}|#{pane_pid}|#{window_name}' 2>/dev/null || true
      done < <(_sockets) | sort -t'|' -k2 -n
    )
    if [[ "$found" -eq 0 ]]; then
      if [[ "$all" == 1 ]]; then echo "  (none — no live claude chats anywhere)"
      else echo "  (none — no live claude chats with cwd inside $repo)"; fi
    fi
    if [[ "$all" == 0 && "$elsewhere" -gt 0 ]]; then
      echo "  (+$elsewhere live in other dirs — chat.sh ls --all to see them)"
    fi
    # Say what was withheld. A listing that quietly drops rows reads as "this is everything".
    [[ "$hiddenrows" -gt 0 ]] && echo "  (+$hiddenrows hidden — cc-ls --hidden to manage)"
    ;;

  capture)
    [[ $# -ge 1 ]] || { echo "usage: chat.sh capture <tmux-session>" >&2; exit 1; }
    target="$1"
    # Resolve across EVERY socket (one socket per chat): exact session name first,
    # then 🔖 label. Either yields "socketpath<TAB>session"; capture then hits that
    # pane's own socket. rc2 is an ambiguous match (candidates already on stderr).
    socketpath=""; session=""; srow=""; srrc=0; srow="$(_resolve_session "$target")" || srrc=$?
    if [[ $srrc -eq 0 && -n "$srow" ]]; then
      socketpath="${srow%%$'\t'*}"; session="${srow#*$'\t'}"
    elif [[ $srrc -eq 2 ]]; then
      echo "ERROR: session '$target' is ambiguous — pass the tmux session id (run /chat:ls to list them)" >&2; exit 1
    else
      lrow=""; lrc=0; lrow="$(_resolve_label "$target")" || lrc=$?
      if [[ $lrc -eq 0 && -n "$lrow" ]]; then
        socketpath="${lrow%%$'\t'*}"; session="${lrow#*$'\t'}"
      elif [[ $lrc -eq 2 ]]; then
        echo "ERROR: 🔖 label '$target' is ambiguous — pass the tmux session id (run /chat:ls to list them)" >&2; exit 1
      else
        xrow=""; xrc=0; xrow="$(_resolve_cxwin "$target")" || xrc=$?
        if [[ $xrc -eq 0 && -n "$xrow" ]]; then
          socketpath="${xrow%%$'\t'*}"; session="${xrow#*$'\t'}"
        elif [[ $xrc -eq 2 ]]; then
          echo "ERROR: codex thread name '$target' is ambiguous — pass the cx-* tmux session id (run cc-ls to list them)" >&2; exit 1
        else
          echo "ERROR: no live tmux session, 🔖 label, or codex thread named '$target' — capture needs a live window; use 'read' for a dormant chat's transcript" >&2; exit 1
        fi
      fi
    fi
    # -J joins wrapped lines so a long input line / footer reads as one whole
    # line instead of "empty-looking" fragments split at the wrap column.
    # -S - starts at the top of the scrollback so the whole retained buffer is
    # captured, not just the visible fold — bounded only by tmux history-limit.
    tmux -S "$socketpath" capture-pane -t "$session" -p -J -S -
    ;;

  save)
    [[ $# -ge 1 ]] || { echo "usage: chat.sh save <target-file> [transcript-jsonl]" >&2; exit 1; }
    target="$1"
    if [[ $# -ge 2 ]]; then transcript="$2"
    else
      config_dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"; session_id="${CLAUDE_CODE_SESSION_ID:-}"
      [[ -n "$session_id" ]] || { echo "ERROR: CLAUDE_CODE_SESSION_ID not set and no transcript path given" >&2; exit 1; }
      transcript="$config_dir/projects/$(pwd | tr '/.' '--')/$session_id.jsonl"
    fi
    [[ -f "$transcript" ]] || { echo "ERROR: transcript not found: $transcript" >&2; exit 1; }
    mkdir -p "$(dirname "$target")"; touch "$target"
    {
      echo; echo "---"; echo; echo "# FULL TRANSCRIPT (script-dumped, verbatim)"; echo
      echo "Visible chat text only — thinking and tool outputs are not recorded here."
      echo "Source: $transcript"; echo
      jq -r "$JQ_EXTRACT" "$transcript"
      echo "---"; echo; echo "# ENVIRONMENT SNAPSHOT (script-dumped)"; echo
      if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        echo "Branch: $(git branch --show-current)"; echo; echo '```'; git status --short; echo '```'; echo
        echo "Worktrees:"; echo '```'; git worktree list; echo '```'
      else echo "(not a git repository)"; fi
    } >> "$target"
    user_count=$(jq -r 'select(.type == "user" and .isSidechain != true) | 1' "$transcript" | wc -l | tr -d ' ')
    echo "Appended transcript ($user_count user records) + env snapshot -> $target ($(wc -l < "$target" | tr -d ' ') lines total)"
    ;;

  tail)
    n="${1:-50}"
    [[ "$n" =~ ^[0-9]+$ ]] || { echo "usage: chat.sh tail [last-N-lines]" >&2; exit 1; }
    config_dir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"; session_id="${CLAUDE_CODE_SESSION_ID:-}"
    [[ -n "$session_id" ]] || { echo "ERROR: CLAUDE_CODE_SESSION_ID not set" >&2; exit 1; }
    transcript="$config_dir/projects/$(pwd | tr '/.' '--')/$session_id.jsonl"
    [[ -f "$transcript" ]] || { echo "ERROR: transcript not found: $transcript" >&2; exit 1; }
    jq -r "$JQ_EXTRACT" "$transcript" | tail -n "$n"
    ;;

  load)
    [[ $# -ge 1 ]] || { echo "usage: chat.sh load <dir-or-file>..." >&2; exit 1; }
    n=0; total=0
    while IFS= read -r f; do
      [[ -n "$f" ]] || continue
      grep -Iq . "$f" 2>/dev/null || continue
      l="$(wc -l < "$f" | tr -d ' ')"
      printf '%7s  %s\n' "$l" "$f"; n=$((n + 1)); total=$((total + l))
    done < <(
      for t in "$@"; do
        if [[ -f "$t" ]]; then printf '%s\n' "$t"
        elif [[ -d "$t" ]]; then find "$t" -type f -not -path '*/.git/*' -not -path '*/node_modules/*' -not -path '*/.venv/*' -not -path '*/__pycache__/*' 2>/dev/null
        else echo "WARN: not found: $t" >&2; fi
      done | sort -u
    )
    echo "---"
    echo "$n files, $total total lines. READ EVERY ONE in full with the Read tool — no skim, no sampling. Write nothing."
    ;;

  branch)
    # Split THIS pane side-by-side (the tmux `Ctrl+b %` split) and run a fork of
    # the current session in the new pane. --fork-session resumes into a fresh
    # session id, so the original transcript is never mutated; the original chat
    # stays visible in the left pane. The fork is pinned to THIS session's model
    # at the 1M-context [1m] variant and takes the given name (the fork can't
    # rename itself) — both on the launch command.
    sid="${CLAUDE_CODE_SESSION_ID:-}"
    [[ -n "$sid" ]] || { echo "ERROR: \$CLAUDE_CODE_SESSION_ID unset — run /chat:branch from inside a live Claude Code session" >&2; exit 1; }
    [[ -n "${TMUX:-}" ]] || { echo "ERROR: not inside tmux — /chat:branch splits the current tmux pane" >&2; exit 1; }
    command -v claude >/dev/null 2>&1 || { echo "ERROR: 'claude' not found on PATH" >&2; exit 1; }
    name="$*"
    # $name is spliced into a shell-parsed tmux run-string ($fork below → sh -c) — strip every
    # shell metacharacter (quotes, ; $ ` etc.) so a name can neither break the quoting nor inject
    # a command; a chat label is only ever letters/digits/space/._-. (sid is a uuid, model a
    # fixed alias — neither is user-influenced.)
    name="${name//[^A-Za-z0-9 ._-]/}"
    model="$(_self_model)"
    # bake THIS chat's account + cache mode into the fork (a split pane otherwise inherits the
    # tmux server env = account 1, harness-default cache) — see _spawn_envpfx.
    fork="$(_spawn_envpfx) claude --resume '$sid' --fork-session"
    [[ -n "$model" ]] && fork="$fork --model '$model'"
    [[ -n "$name" ]]  && fork="$fork --name '$name'"
    child_pane="$(tmux split-window -h -P -F '#{pane_id}' -c "$PWD" "$fork")"
    _register_pane_child "$child_pane"   # so this chat's /bb takes the fork down with it
    echo "Branched ${sid:0:8}…${name:+ as '$name'}${model:+ on $model} into a new pane beside this one — original chat still live in the left pane (tmux prefix + ←/→)."
    ;;

  new)
    # Spawn a FRESH (empty) teammate chat — by DEFAULT in a side-by-side pane like
    # `branch` (but a new chat, not a fork), or with --detach as a headless background
    # session on its own socket. Either way it is a new empty session (no --resume /
    # --fork-session), inherits THIS session's model, and is auto-named from this chat's
    # 🔖 prefix with the next free number (RR → RR_1, RR_2 …); an argument overrides the
    # prefix. The model can't rename itself, so --name sets it. Drive it with
    # /chat:inject <name> (which returns its screen as proof).
    #   default  — split-window -h: a visible pane in THIS window; shares this chat's
    #              tmux server, so it closes when this chat does.
    #   --detach — a detached session on its own cc-new-* socket: headless, off-screen,
    #              found by 🔖 name across sockets; registered so /bb reaps it on bye-bye.
    detach=0
    while [[ "${1:-}" == --* ]]; do
      case "$1" in
        --detach) detach=1; shift;;
        *) echo "ERROR: unknown new flag '$1' (only --detach)" >&2; exit 1;;
      esac
    done
    command -v claude >/dev/null 2>&1 || { echo "ERROR: 'claude' not found on PATH" >&2; exit 1; }
    [[ -n "${TMUX:-}" ]] || { echo "ERROR: not inside tmux — /chat:new needs tmux" >&2; exit 1; }
    # Prefix: an explicit arg wins; else this chat's 🔖 label with a trailing _<n>
    # stripped; else 'chat'.
    prefix="$*"
    [[ -n "$prefix" ]] || prefix="$(self_label 2>/dev/null || true)"
    [[ "$prefix" =~ ^(.+)_[0-9]+$ ]] && prefix="${BASH_REMATCH[1]}"
    # $prefix flows into $name → the shell-parsed $spawn run-string AND into tmux session/socket
    # names — strip shell metacharacters (the self_label source is text scraped off a pane, i.e.
    # whatever the chat was /rename'd to, which can carry a payload). No spaces: prefix names a
    # session/socket. Re-default if sanitizing emptied it.
    prefix="${prefix//[^A-Za-z0-9._-]/}"
    [[ -n "$prefix" ]] || prefix="chat"
    # Next free number in the prefix_<n> family, reading 🔖 labels across all panes.
    used="$(_all_panes | while IFS=$'\t' read -r s sess pane pcmd; do
        [[ "$pcmd" == tmux ]] && continue   # vsct viewport mirrors the inner 🔖 — skip
        nm="$(tmux -S "$s" capture-pane -t "$pane" -p -J 2>/dev/null | grep -F '🔖' | grep -E '🥇|🥈|🥉|🍀' | tail -1 | sed 's/.*🔖 *//; s/ *│.*//; s/[[:space:]]*$//' || true)"
        [[ "$nm" =~ ^${prefix}_([0-9]+)$ ]] && echo "${BASH_REMATCH[1]}" || true
      done | sort -n || true)"
    n=1; while printf '%s\n' "$used" | grep -qx "$n"; do n=$((n+1)); done
    name="${prefix}_${n}"
    model="$(_self_model)"
    # bake THIS chat's account + cache mode into the teammate — a split pane inherits the tmux
    # server env (account 1), and the --detach path also gets it explicitly (harmless, since it
    # already births from the parent's tool shell). See _spawn_envpfx.
    spawn="$(_spawn_envpfx) claude"
    [[ -n "$model" ]] && spawn="$spawn --model '$model'"
    spawn="$spawn --name '$name'"
    if [[ "$detach" == 1 ]]; then
      # Detached background session on its OWN socket (cc-prefixed so the cross-socket
      # scan in /chat:ls and inject picks it up) + a wide pane so the 🔖 statusline
      # renders un-truncated for name resolution. -d keeps it headless; -c opens in repo.
      socket="cc-new-${name}"
      tmux -L "$socket" new-session -d -s "$name" -c "$PWD" -x "${CHAT_NEW_COLS:-220}" -y "${CHAT_NEW_ROWS:-50}" "$spawn"
      # Register it under THIS chat so /bb (cc-hide.sh) reaps it on bye-bye — a detached
      # teammate is its own tmux server and would otherwise outlive its orchestrator
      # headless. Keyed by session uuid, which is exactly the id cc-hide.sh resolves.
      if [[ -n "${CLAUDE_CODE_SESSION_ID:-}" ]]; then
        bash "$CC_FLEET_HOME/cc-db.sh" child-add new "$CLAUDE_CODE_SESSION_ID" "$socket" 2>/dev/null || true
      fi
      echo "Spawned background teammate '$name'${model:+ on $model} as a detached session on socket '$socket' — headless, not in this terminal. Give it work: /chat:inject $name <task>  (inject returns its screen as proof). Watch it live: tmux -L $socket attach -t $name. Reaped when this chat runs /bb."
    else
      # Visible side-by-side pane in THIS window (like branch, but a fresh empty chat).
      # It shares this chat's tmux server, so it closes when this chat does.
      child_pane="$(tmux split-window -h -P -F '#{pane_id}' -c "$PWD" "$spawn")"
      _register_pane_child "$child_pane"   # so this chat's /bb takes the teammate down with it
      echo "Spawned teammate '$name'${model:+ on $model} in a new pane beside this one (fresh empty chat — closes with this chat). Give it work: /chat:inject $name <task>  (inject returns its screen as proof)."
    fi
    ;;

  extract)
    [[ $# -ge 1 && -f "${1:-}" ]] || { echo "usage: chat.sh extract <transcript-jsonl>" >&2; exit 1; }
    jq -r "$JQ_EXTRACT" "$1"
    ;;

  modal)
    # modal <tmux-session> deny <down-count> — answer a permission MODAL in a live chat pane.
    # SANCTIONED DENY-ONLY (wave/orchestrator.md § tmux-Escape lever): a blocked destructive action
    # raises a modal that swallows the statusline (label-resolve breaks — hence session-name-only)
    # and deadlocks the chat; inject cannot answer a menu. This navigates <down-count> Downs + Enter
    # to select the deny/"No" option. NEVER used to accept — blind-accepting defeats the guard.
    [[ $# -ge 3 && "${2:-}" == "deny" && "${3:-}" =~ ^[0-9]+$ ]] || { echo "usage: chat.sh modal <tmux-session> deny <down-count>   # deny-only: N Downs + Enter onto the deny option" >&2; exit 1; }
    target="$1"; downs="$3"
    sock="/tmp/tmux-$(id -u)/$target"
    [[ -S "$sock" ]] || { echo "ERROR: no tmux socket for session '$target'" >&2; exit 1; }
    # Lock on the SAME socket-scoped key inject uses ("${sock}:${target}") — inject locks on
    # "${socketpath}:${live_tmux}", so a bare-"$target" key computed a DIFFERENT lockdir and
    # the two never excluded each other (modal's Down/Down/Enter could interleave with an
    # inject and land the deny on the wrong menu option). Abort on timeout like inject does.
    if ! _inject_lock_acquire "${sock}:${target}"; then
      echo "WARNING: could not acquire the inject lock for '$target' within ${CHAT_INJECT_LOCK_TIMEOUT:-30}s — another inject/modal is mid-delivery. Re-run when it frees." >&2
      exit 4
    fi
    trap _inject_lock_release EXIT
    for ((i = 0; i < downs; i++)); do tmux -S "$sock" send-keys Down; sleep 0.2; done
    tmux -S "$sock" send-keys Enter
    sleep 1
    echo "--- modal answered (deny, ${downs} Down) : screen of '$target' ---"
    tmux -S "$sock" capture-pane -p | tail -25
    echo "--- end proof ---"
    ;;

  *)
    echo "usage: chat.sh {whoami|find|read|inject|ls|capture|save|extract|tail|load|branch|new|modal} ..." >&2
    exit 1
    ;;
esac
