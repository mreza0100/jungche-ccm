#!/usr/bin/env bash
set -euo pipefail

# chat.sh — the chat: family engine, one script with subcommands. Lives in
# .claude/commands/chat/ ; each chat/<name>.md command calls `chat.sh <name>`.
#
#   whoami                                    print THIS chat's own tmux session
#   find    <excerpt-file>                    resolve a pasted excerpt to a session
#   read    <excerpt-file>                    extract a matched chat's transcript
#   inject  <self|tmux|session-id|path> <message...>   force a turn (live / resume)
#   ls                                        list live chats with cwd in this repo
#   capture <tmux-session> [scrollback]       snapshot a live chat's screen
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
    [[ $# -ge 2 ]] || { echo "usage: chat.sh inject <self|tmux-session|session-id|transcript-path> <message...>" >&2; exit 1; }
    target="$1"; shift; msg="$*"
    [[ -n "$msg" ]] || { echo "ERROR: refusing to inject an empty message" >&2; exit 1; }
    if [[ "$target" == "self" || "$target" == "me" ]]; then target="$(self_tmux)" || exit 1; fi
    live_tmux=""; transcript=""
    if tmux has-session -t "$target" 2>/dev/null; then live_tmux="$target"
    elif [[ -f "$target" ]]; then transcript="$(cd "$(dirname "$target")" && pwd -P)/$(basename "$target")"
    elif [[ "$target" =~ ^[A-Za-z0-9_-]+$ ]]; then
      transcript="$(
        for p in "$HOME"/.claude/projects/*/"$target".jsonl "$HOME"/.claude[0-9]*/projects/*/"$target".jsonl; do
          [[ -f "$p" ]] && ( cd "$(dirname "$p")" && printf '%s\n' "$(pwd -P)/$(basename "$p")" )
        done | sort -u | head -1 || true
      )"
      [[ -n "$transcript" ]] || { echo "ERROR: no live tmux pane and no transcript for '$target'" >&2; exit 1; }
    else echo "ERROR: target is not self, a live tmux session, a transcript path, or a session-id" >&2; exit 1; fi

    # Sender signature — every injected message ends with who sent it and the
    # exact command to answer with. The /rename chat title isn't readable from a
    # script, so identity = the sender's own tmux session (the reply handle) +
    # short session id; an optional human name comes from the caller via
    # $CHAT_INJECT_FROM_NAME (the model's own 🔖 chat name). The reply line hands
    # the recipient a runnable `/chat:inject {handle} <message>`.
    # Two footer forms: a one-line one for the LIVE typed path (a bare newline
    # submits in Claude Code, so the typed message must stay single-line), and a
    # block one for pure text — the RESUME transcript and the long-message file.
    sender_handle="$(self_tmux 2>/dev/null || true)"
    sender_uuid="${CLAUDE_CODE_SESSION_ID:-}"; sender_uuid8="${sender_uuid:0:8}"
    sender_name="${CHAT_INJECT_FROM_NAME:-}"
    sigparts=()
    [[ -n "$sender_name" ]]   && sigparts+=("from ${sender_name}")
    [[ -n "$sender_uuid8" ]]  && sigparts+=("sid ${sender_uuid8}")
    [[ -n "$sender_handle" ]] && sigparts+=("to reply: /chat:inject ${sender_handle} <message>")
    sig=""; for p in "${sigparts[@]:-}"; do [[ -n "$p" ]] || continue; [[ -n "$sig" ]] && sig="$sig · "; sig="$sig$p"; done
    footer_inline=""; footer_block=""
    [[ -n "$sig" ]] && { footer_inline="  — ${sig}"; footer_block=$'\n\n'"— ${sig}"; }
    sender_short="${sender_name:-${sender_handle:-${sender_uuid8:-unknown}}}"

    if [[ -n "$live_tmux" ]]; then
      note=""
      if [[ "$msg" =~ ^[[:space:]]*/ ]]; then
        : # the message IS a slash command — send verbatim so the target runs it:
          # no signature (it would land as command args) and no file-cap (a file
          # pointer would be read, never executed).
      else
        limit="${CHAT_INJECT_MAXLEN:-280}"; msg="${msg}${footer_inline}"
        if [[ ${#msg} -gt $limit ]]; then
          msgdir="$HOME/.claude-sessions/.chat-inject-msgs"; mkdir -p "$msgdir"
          msgfile="$msgdir/$(date -u +%Y%m%dT%H%M%SZ)-$$.md"; printf '%s%s\n' "$*" "$footer_block" > "$msgfile"
          msg="📨 Injected message from ${sender_short} (too long to type inline). Read it and act on it: $msgfile"; note=" (long message saved to $msgfile)"
        fi
      fi
      tmux send-keys -t "$live_tmux" -l -- "$msg"
      sleep "${CHAT_INJECT_SUBMIT_DELAY:-0.4}"
      tmux send-keys -t "$live_tmux" Enter
      echo "injected LIVE into tmux session '$live_tmux'$note — answered now (pane must be idle at its prompt)"
      exit 0
    fi

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
    repo="$(repo_root)"; self=""
    [[ -n "${TMUX:-}" ]] && self="$(tmux display-message -p '#{session_name}' 2>/dev/null || true)"
    echo "live chats in this repo (session · state · last activity):"
    found=0
    while IFS='|' read -r sess path pcmd wactive pactive; do
      [[ "$wactive" == "1" && "$pactive" == "1" ]] || continue
      case "$path" in "$repo" | "$repo"/*) ;; *) continue ;; esac
      case "$pcmd" in [0-9]* | claude) ;; *) continue ;; esac
      cap="$(tmux capture-pane -t "$sess" -p 2>/dev/null | sed 's/[[:space:]]*$//' || true)"
      state="$(printf '%s\n' "$cap" | grep -oE '🟢|⚡|🔴' | tail -1 || true)"; state="${state:-·}"
      topic="$(printf '%s\n' "$cap" | grep -vE '^$|🟢|⚡|🔴|auto mode on|shift\+tab|esc to interrupt|to scroll|for agents|^[─╭╮╰╯│]|^❯$|💾|💰|⏱' | tail -1 | cut -c1-64 || true)"
      mark=""; [[ "$sess" == "$self" ]] && mark="  <- this chat"
      printf '  %-6s %s  %s%s\n' "$sess" "$state" "$topic" "$mark"; found=$((found + 1))
    done < <(tmux list-panes -a -F '#{session_name}|#{pane_current_path}|#{pane_current_command}|#{window_active}|#{pane_active}' 2>/dev/null | sort -t'|' -k1 -n)
    [[ "$found" -gt 0 ]] || echo "  (none — no live claude chats with cwd inside $repo)"
    ;;

  capture)
    [[ $# -ge 1 ]] || { echo "usage: chat.sh capture <tmux-session> [scrollback-lines]" >&2; exit 1; }
    target="$1"; lines="${2:-}"
    tmux has-session -t "$target" 2>/dev/null || { echo "ERROR: no live tmux session '$target' — capture needs a live window; use 'read' for a dormant chat's transcript" >&2; exit 1; }
    if [[ -n "$lines" ]]; then tmux capture-pane -t "$target" -p -S "-$lines"; else tmux capture-pane -t "$target" -p; fi
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
