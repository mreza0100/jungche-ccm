#!/usr/bin/env bash
set -euo pipefail

# chat.sh — the chat: family engine, one script with subcommands. Lives in
# .claude/commands/chat/ ; each chat/<name>.md command calls `chat.sh <name>`.
#
#   whoami                                    print THIS chat's own tmux session
#   find    <excerpt-file>                    resolve a pasted excerpt to a session
#   read    <excerpt-file>                    extract a matched chat's transcript
#   inject  [--no-sig] [--force-now] <self|tmux|label|session-id|path> <message...>  force a turn (live / resume)
#   ls                                        list live chats with cwd in this repo
#   capture <tmux-session>                    snapshot a live chat's full scrollback
#   save    <target-file> [transcript-jsonl]  dump THIS session's transcript
#   extract <transcript-jsonl>                render a known transcript to text
#   tail    [N]                               render THIS session's last N lines
#   load    <dir-or-file>...                  enumerate a file set to force-read in full

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

repo_root() { cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd -P; }

self_tmux() {
  [[ -n "${TMUX:-}" ]] || { echo "ERROR: this chat is not inside tmux (\$TMUX unset)" >&2; return 1; }
  tmux display-message -p '#{session_name}'
}

# self_label: this chat's own 🔖 label (its /rename name), read from its own
# statusline — the line carrying both 🔖 and 🌿. Empty when not in tmux or unlabeled.
# Reliable mid-run because this script never prints 🌿, so no command-echo line in our
# own pane collides with the real statusline.
self_label() {
  local s
  s="$(self_tmux 2>/dev/null || true)"; [[ -n "$s" ]] || return 0
  tmux capture-pane -t "$s" -p -J 2>/dev/null | grep -F '🔖' | grep -F '🌿' | tail -1 \
    | sed 's/.*🔖 *//; s/ *│.*//; s/[[:space:]]*$//' || true
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
_inject_lock_release() {
  [[ -n "${INJECT_LOCKDIR:-}" ]] && rm -rf "$INJECT_LOCKDIR" 2>/dev/null || true
  INJECT_LOCKDIR=""
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

# _all_panes: enumerate every pane on EVERY socket. Each output row is
# "socketpath<TAB>session<TAB>pane_id". A dead or unreadable socket is skipped
# (the `|| true` swallows its error) and never aborts the scan.
_all_panes() {
  local sock line
  while IFS= read -r sock; do
    [[ -n "$sock" ]] || continue
    while IFS= read -r line; do
      [[ -n "$line" ]] && printf '%s\t%s\n' "$sock" "$line"
    done < <(tmux -S "$sock" list-panes -a -F '#{session_name}'$'\t''#{pane_id}' 2>/dev/null || true)
  done < <(_sockets)
}

# _resolve_label <label>: resolve a destination by its Claude 🔖 label (the /rename
# name) to a (socket, session) — scanning live panes on EVERY socket the way `ls`
# does. Match is case-insensitive and exact on the 🔖 text. Prints
# "socketpath<TAB>session" on a UNIQUE match (return 0); prints nothing on no match
# (return 1); on several matches lists the candidates on stderr (return 2). The 🔖
# name lives on the statusline — the line that carries both 🔖 and 🌿 — so we key on
# that to avoid matching conversation text.
_resolve_label() {
  local want="$1" sock sess pane cap name wantlc namelc seen m; local -a matches=()
  wantlc="$(printf '%s' "$want" | tr '[:upper:]' '[:lower:]')"
  # Scan EVERY pane on EVERY socket (not just each session's active pane) — a
  # session can hold several shells, and only the Claude pane carries the 🔖
  # statusline. Per-pane capture must hit the pane's OWN socket via -S "$sock".
  while IFS=$'\t' read -r sock sess pane; do
    [[ -n "$pane" ]] || continue
    cap="$(tmux -S "$sock" capture-pane -t "$pane" -p -J 2>/dev/null || true)"
    name="$(printf '%s\n' "$cap" | grep -F '🔖' | grep -F '🌿' | tail -1 | sed 's/.*🔖 *//; s/ *│.*//; s/[[:space:]]*$//' || true)"
    [[ -n "$name" ]] || continue
    namelc="$(printf '%s' "$name" | tr '[:upper:]' '[:lower:]')"
    [[ "$namelc" == "$wantlc" ]] || continue
    seen=0; for m in "${matches[@]:-}"; do [[ "$m" == "$sock"$'\t'"$sess" ]] && { seen=1; break; }; done
    [[ "$seen" == 0 ]] && matches+=("$sock"$'\t'"$sess")
  done < <(_all_panes)
  case "${#matches[@]}" in
    1) printf '%s\n' "${matches[0]}"; return 0;;
    0) return 1;;
    *) { echo "ambiguous 🔖 label '$want' — matches sessions:"; for m in "${matches[@]}"; do echo "  ${m#*$'\t'}  (socket ${m%%$'\t'*})"; done; } >&2; return 2;;
  esac
}

# _resolve_session <name>: resolve a destination by EXACT tmux session name across
# EVERY socket. Session names are globally unique (the launcher names each chat's
# session after its unique socket basename), so a hit is normally unique. Prints
# "socketpath<TAB>session" on a unique match (return 0); nothing on no match
# (return 1); on several lists candidates on stderr (return 2).
_resolve_session() {
  local want="$1" sock sess pane seen m; local -a matches=()
  while IFS=$'\t' read -r sock sess pane; do
    [[ "$sess" == "$want" ]] || continue
    seen=0; for m in "${matches[@]:-}"; do [[ "$m" == "$sock"$'\t'"$sess" ]] && { seen=1; break; }; done
    [[ "$seen" == 0 ]] && matches+=("$sock"$'\t'"$sess")
  done < <(_all_panes)
  case "${#matches[@]}" in
    1) printf '%s\n' "${matches[0]}"; return 0;;
    0) return 1;;
    *) { echo "ambiguous session '$want' — matches:"; for m in "${matches[@]}"; do echo "  ${m#*$'\t'}  (socket ${m%%$'\t'*})"; done; } >&2; return 2;;
  esac
}

# _find <excerpt-file>: resolve to the past session containing the excerpt across
# every account. stdout: "<session-id>\t<transcript-path>"; report on stderr.
_find() {
  local excerpt="$1"
  [[ -f "${excerpt:-}" ]] || { echo "usage: chat.sh find <excerpt-file>" >&2; return 1; }
  local current="${CLAUDE_CODE_SESSION_ID:-__none__}"
  local registries=()
  while IFS= read -r proj; do [[ -n "$proj" ]] && registries+=("$proj"); done < <(
    for cfg in "$HOME"/.claude "$HOME"/.claude[0-9]*; do
      [[ -d "$cfg/projects" ]] || continue
      for p in "$cfg/projects"/*/; do [[ -d "$p" ]] && ( cd "$p" && pwd -P ); done
    done | sort -u
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
    nosig=0; force_now=0
    while [[ "${1:-}" == --* ]]; do
      case "$1" in
        --no-sig)    nosig=1; shift;;
        --force-now) force_now=1; shift;;
        *) echo "ERROR: unknown inject flag '$1'" >&2; exit 1;;
      esac
    done
    [[ $# -ge 2 ]] || { echo "usage: chat.sh inject [--no-sig] [--force-now] <self|tmux-session|🔖-label|session-id|transcript-path> <message...>" >&2; exit 1; }
    target="$1"; shift; msg="$*"
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
          echo "resolved 🔖 label '$target' → tmux session '$live_tmux'" >&2
        elif [[ $lrc -eq 2 ]]; then
          echo "ERROR: 🔖 label '$target' is ambiguous — pass the tmux session id instead (run /chat:ls to list them)" >&2; exit 1
        elif [[ "$target" =~ ^[A-Za-z0-9_-]+$ ]]; then
          transcript="$(
            for p in "$HOME"/.claude/projects/*/"$target".jsonl "$HOME"/.claude[0-9]*/projects/*/"$target".jsonl; do
              [[ -f "$p" ]] && ( cd "$(dirname "$p")" && printf '%s\n' "$(pwd -P)/$(basename "$p")" )
            done | sort -u | head -1 || true
          )"
          [[ -n "$transcript" ]] || { echo "ERROR: 🔖 label '$target' matched no live chat (run /chat:ls to see live labels), and it is not a live tmux session, a transcript path, or a session-id — nothing to inject into" >&2; exit 1; }
        else
          echo "ERROR: 🔖 label '$target' matched no live chat (run /chat:ls to see live labels), and it is not a live tmux session, a transcript path, or a session-id — nothing to inject into" >&2; exit 1
        fi
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
    [[ -n "$sig" ]] && { footer_inline="  — ${sig}"; footer_block=$'\n\n'"— ${sig}"; }
    # --no-sig drops the footer entirely — for /compact, /goal, /loop and other
    # operational injections that must not carry a signature.
    [[ "$nosig" == 1 ]] && { footer_inline=""; footer_block=""; }

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
      # Everything goes inline — no file, no pointer, no length cap. A slash
      # command sends verbatim (a footer/marker would land as command args);
      # anything else gets the force marker (only if we actually interrupted) plus
      # the one-line footer appended. The settle-poll below confirms the whole
      # message — however long — actually rendered before Enter, so a long inline
      # send is reliable and a genuine jam is caught instead of half-sent.
      force_mark=""
      [[ "$did_intr" == 1 ]] && force_mark=" — ⚠ FORCE-DELIVERED via Esc (your running flow was interrupted; re-check any in-progress action)"
      if [[ "$msg" =~ ^[[:space:]]*/ ]]; then :; else msg="${msg}${force_mark}${footer_inline}"; fi

      # Normalize the input surface so keystrokes actually land — a pane in copy/scroll
      # mode or sitting in the Rewind menu silently eats input (a common "Enter didn't
      # land" cause). Exit copy-mode; cancel a Rewind menu with a single Esc.
      if [[ "$("${TM[@]}" display-message -t "$live_tmux" -p '#{pane_in_mode}' 2>/dev/null || echo 0)" == 1 ]]; then
        "${TM[@]}" send-keys -t "$live_tmux" -X cancel 2>/dev/null || true
      fi
      if "${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null | grep -qiF 'Restore the code'; then
        "${TM[@]}" send-keys -t "$live_tmux" Escape; sleep "${CHAT_INJECT_POLL:-0.2}"
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

      "${TM[@]}" send-keys -t "$live_tmux" -l -- "$msg"
      # A distinctive tail of the message — used to confirm the text rendered AND
      # to tell "still in the input box" from "submitted". On submit the input line
      # clears, but the message stays visible UP in the conversation, so the
      # submit check looks at the input line only (the LAST '❯'), never the whole pane.
      needle="$(printf '%s' "$msg" | tr -s '[:space:]' ' ' | sed 's/^ //; s/ *$//')"; needle="${needle: -40}"
      # 1) Wait for the full text to render before the first Enter (advisory). NEVER
      #    bail on a miss — a typed-but-unsubmitted message is the worst outcome, and
      #    detection can flake on wrapping/footer glyphs even when the text is there.
      for _ in $(seq 1 "${CHAT_INJECT_SETTLE_TRIES:-40}"); do
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
        "${TM[@]}" send-keys -t "$live_tmux" Enter
        if [[ "$saved_draft" == 0 ]]; then
          sleep "${CHAT_INJECT_ENTER_GAP:-0.15}"
          "${TM[@]}" send-keys -t "$live_tmux" Enter
        fi
        sleep "${CHAT_INJECT_ENTER_SETTLE:-0.4}"
        input_line="$("${TM[@]}" capture-pane -t "$live_tmux" -p -J 2>/dev/null | grep -F '❯' | tail -1)"
        [[ "$input_line" != *"$prefix"* ]] && { submitted=1; break; }
      done
      if [[ "$submitted" == 1 ]]; then
        note=""
        [[ "$did_intr" == 1 ]]    && note="${note} — FORCE-NOW: interrupted the target's running flow"
        [[ "$saved_draft" == 1 ]] && note="${note} — stashed and restored the target's unsent draft"
        echo "injected LIVE into tmux session '$live_tmux' — typed inline and submitted (Enter confirmed, input cleared)${note}"
        exit 0
      fi
      echo "WARNING: typed the text into '$live_tmux' but could not confirm submission after ${CHAT_INJECT_ENTER_TRIES:-12} Enter attempts — the pane is likely busy mid-turn or in a selector. The text is in its input and will send on the next Enter; re-run when it is idle at '❯'." >&2
      exit 3
    fi

    [[ "$force_now" == 1 ]] && echo "WARNING: --force-now ignored — '$target' is not a live pane (a dormant chat has no running flow to interrupt); delivering via transcript RESUME." >&2
    msg="${msg}${footer_block}"
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

  ls)
    repo="$(repo_root)"; self_sock=""; self_sess=""
    if [[ -n "${TMUX:-}" ]]; then
      self_sock="${TMUX%%,*}"; self_sess="$(tmux display-message -p '#{session_name}' 2>/dev/null || true)"
    fi
    echo "live chats in this repo (session · state · last activity):"
    found=0
    # Enumerate panes on EVERY socket (one socket per chat now); each row is prefixed
    # with its socket so per-pane capture/display hit the right server. A dead socket
    # is skipped (|| true), never aborting the listing. Self-match compares BOTH socket
    # and session — session names are globally unique, so the session stays the handle.
    while IFS='|' read -r sock sess path pcmd wactive pactive; do
      [[ "$wactive" == "1" && "$pactive" == "1" ]] || continue
      case "$path" in "$repo" | "$repo"/*) ;; *) continue ;; esac
      case "$pcmd" in [0-9]* | claude) ;; *) continue ;; esac
      cap="$(tmux -S "$sock" capture-pane -t "$sess" -p 2>/dev/null | sed 's/[[:space:]]*$//' || true)"
      state="$(printf '%s\n' "$cap" | grep -oE '🟢|⚡|🔴' | tail -1 || true)"; state="${state:-·}"
      topic="$(printf '%s\n' "$cap" | grep -vE '^$|🟢|⚡|🔴|auto mode on|shift\+tab|esc to interrupt|to scroll|for agents|^[─╭╮╰╯│]|^❯$|💾|💰|⏱' | tail -1 | cut -c1-64 || true)"
      mark=""; [[ "$sock" == "$self_sock" && "$sess" == "$self_sess" ]] && mark="  <- this chat"
      printf '  %-6s %s  %s%s\n' "$sess" "$state" "$topic" "$mark"; found=$((found + 1))
    done < <(
      while IFS= read -r sock; do
        [[ -n "$sock" ]] || continue
        tmux -S "$sock" list-panes -a -F "$sock"'|#{session_name}|#{pane_current_path}|#{pane_current_command}|#{window_active}|#{pane_active}' 2>/dev/null || true
      done < <(_sockets) | sort -t'|' -k2 -n
    )
    [[ "$found" -gt 0 ]] || echo "  (none — no live claude chats with cwd inside $repo)"
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
        echo "ERROR: no live tmux session '$target' — capture needs a live window; use 'read' for a dormant chat's transcript" >&2; exit 1
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

  extract)
    [[ $# -ge 1 && -f "${1:-}" ]] || { echo "usage: chat.sh extract <transcript-jsonl>" >&2; exit 1; }
    jq -r "$JQ_EXTRACT" "$1"
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

  *)
    echo "usage: chat.sh {whoami|find|read|inject|ls|capture|save|extract|tail|load} ..." >&2
    exit 1
    ;;
esac
