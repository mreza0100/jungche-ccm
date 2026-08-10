#!/usr/bin/env zsh
# Post-cutover cc-fleet shell surface. WP10 only prepares this file; WP11 is
# responsible for sourcing it. The binary is installed at ~/.local/bin/cc-fleet.

typeset -gr _CC_FLEET_BIN="$HOME/.local/bin/cc-fleet"
if [[ ! -x "$_CC_FLEET_BIN" ]]; then
  print -u2 -- "cc-fleet: $_CC_FLEET_BIN is missing or not executable"
  return 1
fi

export CLAUDE_DISABLE_ADOPT=1
export CLAUDE_CODE_DISABLE_BG_EXIT_HANDOFF=1
export CLAUDE_CODE_STOP_HOOK_BLOCK_CAP=2

_cc_fleet_eval() {
  local output rc
  output="$("$_CC_FLEET_BIN" "$@")"
  rc=$?
  if (( rc == 0 )) && [[ -n "$output" ]]; then
    eval "$output"
  fi
  return "$rc"
}

cc-ls() {
  case " $* " in
    *" --check "*|*" --plain "*|*" --tsv "*)
      "$_CC_FLEET_BIN" ls "$@"
      return $?
      ;;
  esac
  _cc_fleet_eval ls "$@"
}

cc-open() { _cc_fleet_eval open "$@"; }
cc-revive() { "$_CC_FLEET_BIN" revive "$@"; }

# The launch/account functions below are intentionally retained verbatim from
# oldbox/scripts/cc-fleet.zsh. The Go action protocol emits lines that call them.
_cc_arm1h() {
  if [[ "${CC_ARM_1H:-}" == 1 ]]; then echo 1
  elif [[ "${ENABLE_PROMPT_CACHING_1H:-}" == 1 && -z "${CLAUDECODE:-}" ]]; then echo 1
  else echo 0; fi
}
_cc_run() {
  local acct="$1" use_tmux="$2"; shift 2
  local cfg; case "$acct" in 2|3) cfg="$HOME/.cc/$acct" ;; *) cfg="" ;; esac
  local in_tmux=0; [[ -n "$TMUX" ]] && in_tmux=1
  # ⚡1h-cache is per-launch, NEVER sticky (2× write premium must be a deliberate choice each
  # time) — _cc_arm1h decides; the strip below unsets the leaked flag, and an armed launch
  # re-adds it as an env ARGUMENT (a shell prefix would be re-unset by its own -u)
  local arm1h; arm1h="$(_cc_arm1h)"
  if [[ "$use_tmux" == "1" ]]; then
    # Each chat gets its OWN tmux server (unique socket == session name) so a single
    # tmux SIGSEGV can no longer take down every chat at once. The globally-unique
    # session name preserves the chat: tooling's "address by tmux session name" handle.
    # Inside a tmux already (a vsct bunker, a chat pane), the chat STILL gets its own
    # server — the current pane just becomes a nested client viewing it. Without this,
    # bunker-born chats ran naked on the shared vsct server: no isolation, no cc-ls row.
    local sock="cc-$(date +%s)-$$-$RANDOM"
    # explicit env, always: a launch from INSIDE a chat (Bash tool, nested shell) inherits the
    # host chat's CLAUDE_CONFIG_DIR / session identity / cache mode — env -u makes every one of
    # them the picker's verdict, never the environment's
    local run="env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M"
    if [[ -n "$cfg" ]]; then run+=" CLAUDE_CONFIG_DIR=$cfg"; else run+=" -u CLAUDE_CONFIG_DIR"; fi
    if [[ "$arm1h" == 1 ]]; then run+=" ENABLE_PROMPT_CACHING_1H=1"; else run+=" FORCE_PROMPT_CACHING_5M=1"; fi
    run+=" claude ${(q)@}"
    if [[ "$in_tmux" == "0" ]]; then tmux -L "$sock" new-session -s "$sock" "$run"
    elif _cc_in_bunker; then TMUX= exec tmux -L "$sock" new-session -s "$sock" "$run"   # viewport dies with the tab
    else TMUX= tmux -L "$sock" new-session -s "$sock" "$run"
    fi
  else
    local -a envargs=(-u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M)
    if [[ -n "$cfg" ]]; then envargs+=(CLAUDE_CONFIG_DIR="$cfg"); else envargs+=(-u CLAUDE_CONFIG_DIR); fi
    if [[ "$arm1h" == 1 ]]; then envargs+=(ENABLE_PROMPT_CACHING_1H=1); else envargs+=(FORCE_PROMPT_CACHING_5M=1); fi
    env "${envargs[@]}" claude "$@"
  fi
}
_cc_primary() { local n=1; [[ -f "$HOME/.claude-primary" ]] && n="$(<$HOME/.claude-primary)"; case "$n" in 1|2|3) ;; *) n=1 ;; esac; echo "$n"; }
cc()  { _cc_run "$(_cc_primary)" 1 "$@"; }   # tmux + primary account
cc1() { _cc_run 1 1 "$@"; }                  # tmux + account 1
cc2() { _cc_run 2 1 "$@"; }                  # tmux + account 2
cc3() { _cc_run 3 1 "$@"; }                  # tmux + account 3
# cx — a CODEX chat on the same per-chat-server pattern, socket prefix cx-* instead of cc-*.
# The prefix IS the engine marker: codex writes no statusline breadcrumbs and no ~/.claude
# transcript, so cc-ls recognizes (and lists) a live Codex chat by socket name alone. Claude
# accounts / ⚡1h don't apply — codex has its own single auth (~/.codex).
cx() {
  local sock="cx-$(date +%s)-$$-$RANDOM"
  # same launch hygiene as claude: a codex born inside a Claude chat must not inherit its identity.
  # --dangerously-bypass-approvals-and-sandbox: founder's standing order (2026-07-24) — a fleet
  # codex runs with FULL autonomy, never stopping to ask approval (wave builders stalled on
  # mid-task approval prompts). This box's codex work is already gated by repo trust + the fleet.
  local run="env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u CLAUDE_CONFIG_DIR -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M codex --dangerously-bypass-approvals-and-sandbox ${(q)@}"
  _cx_server "$sock" "$PWD" "$run" || return
  if _cc_selfswitch "$sock"; then :                          # already inside it → switch, never nest
  elif _cc_in_bunker; then TMUX= exec tmux -L "$sock" attach # viewport dies with the tab
  else TMUX= tmux -L "$sock" attach
  fi
}

# _cx_server <sock> <cwd> <run> — create a Codex chat's tmux server DETACHED with the title
# plumbing in place before any client draws: the VS Code tab follows '⬢ <window> · <pane title>'
# (codex owns pane_title — project + busy spinner; cc-ls converges the window name to the codex
# thread_name, so a rename shows up in the tab like Claude's /rename does).
_cx_server() {
  local sock="$1" cwd="$2" run="$3"
  TMUX= tmux -L "$sock" new-session -d -s "$sock" -c "$cwd" -n Codex "$run" || return 1
  tmux -L "$sock" set -g set-titles on \; set -g set-titles-string '⬢ #{window_name} · #{pane_title}' \; setw -g automatic-rename off
  return 0
}

# pretty label per account: medal + the real email pulled from its config dir
_cc_label() {
  local n="$1" dir medal email
  case "$n" in 1) dir="$HOME/.claude"; medal="🥇" ;; 2) dir="$HOME/.cc/2"; medal="🥈" ;; 3) dir="$HOME/.cc/3"; medal="🥉" ;; esac
  email="$(jq -r '.oauthAccount.emailAddress // empty' "$dir/.claude.json" 2>/dev/null)"
  [[ -z "$email" ]] && email="(not logged in — run cc${n} then /login)"
  print -r -- "$medal $email"
}

# cc-swap [1|2|3] — fzf picker (no arg) to set which account bare `cc` opens
cc-swap() {
  local cur n; cur="$(_cc_primary)"
  local -a rows=(
    "1 │ $(_cc_label 1)"
    "2 │ $(_cc_label 2)"
    "3 │ $(_cc_label 3)"
  )
  if [[ "${1:-}" =~ ^[123]$ ]]; then
    n="$1"
  elif command -v fzf >/dev/null; then
    local curi="$cur"                           # row position == account number (rows are 1,2,3)
    local next=$(( curi % ${#rows} + 1 ))       # row position of the other one
    rows[$curi]="${rows[$curi]}  ← current"
    local pick
    pick="$(printf '%s\n' "${rows[@]}" | fzf \
      --no-mouse \
      --height=~6 --reverse --cycle --no-info --header-first \
      --border=rounded --border-label=' Claude Code · primary account ' \
      --header="Enter picks (starts on next) · Esc keeps account $cur" \
      --prompt='cc ❯ ' --pointer='▶' \
      --bind "start:pos($next)" \
      --color='border:cyan,label:bold:cyan,header:dim,prompt:bold,pointer:yellow')" || true
    [[ -z "$pick" ]] && { echo "cc-swap: unchanged — primary stays account $cur"; return 0; }
    n="${pick%% *}"
  else
    echo "cc-swap: fzf not found — pass a number: cc-swap <1|2>"; return 1
  fi
  echo "$n" > "$HOME/.claude-primary"
  echo "Primary → account $n  ($(_cc_label $n))"
  echo "  cc       → account $n"
  echo "  cc1/cc2  → explicit account"
}

# _cc_in_bunker — true when this shell IS a vsct bunker pane. Chat opens from a bunker exec
# INTO the tmux client: the viewport dies with the tab instead of lingering as an orphaned
# husk that the next fresh terminal adopts (the ORCHESTRATOR ambush). Chats themselves live
# on their own cc-* servers and are untouched. Never true inside a chat pane or a plain
# shell — those keep child-spawned clients (a Bash-tool shell must never be exec'd away).
_cc_in_bunker() { [[ "${TMUX%%,*}" == */vsct ]] }

# _cc_selfswitch is retained because cx() depends on the legacy same-server
# recursion guard.
_cc_selfswitch() {
  local sock="$1" w
  [[ -n "${TMUX:-}" && "${TMUX%%,*}" == */"$sock" ]] || return 1
  # the chat's window = the one whose pane runs the engine, in DESCENDING order of certainty and
  # lowest window index first within each tier. An exact engine name is proof; `node` and the bare
  # version string are only hints — codex renders as its node wrapper and claude as a bare version
  # in some states, but a shell window running any node process wears the same shape, and matching
  # it first would switch the founder to that shell, i.e. the very "wrong window" this fixes.
  local panes; panes="$(tmux -L "$sock" list-panes -a -F '#{window_index}'$'\t''#{pane_current_command}' 2>/dev/null | sort -n)"
  w="$(print -r -- "$panes" | awk -F'\t' '$2 ~ /^(claude|codex)$/ { print $1; exit }')"
  [[ -n "$w" ]] || w="$(print -r -- "$panes" | awk -F'\t' '$2 ~ /^node$/ || $2 ~ /^[0-9]+\./ { print $1; exit }')"
  # engine window unidentifiable (an unexpected pane command) → the lowest window index, where every
  # cc-/cx- server puts the chat at birth. Announce only what actually happened: claiming a switch
  # that never ran would send the founder back to a screen that did not change.
  [[ -n "$w" ]] || w="$(print -r -- "$panes" | awk -F'\t' 'NR==1 { print $1 }')"
  if [[ -n "$w" ]] && tmux select-window -t "$w" 2>/dev/null; then
    print -u2 -- "cc: already inside this chat's tmux — switched to its window (a session must never nest inside itself)"
  else
    print -u2 -- "cc: already inside this chat's tmux — refusing to nest it inside itself; switch windows yourself (prefix + w)"
  fi
  return 0
}

# vsct-revive remains zsh: it owns the non-chat bunker dashboard, outside the
# cc-fleet chat database.
vsct-revive() {
  local -a projs=("$@"); (( $# )) || projs=("${PWD:t}")
  local srv="revive-vsct" out="rv-${(j:+:)projs}"
  local s att n=0 pid kids p hit
  tmux -L "$srv" kill-session -t "=$out" 2>/dev/null    # rebuild the dashboard each run
  for s in ${(f)"$(tmux -L vsct list-sessions -F $'#{session_name}\t#{session_attached}' 2>/dev/null)"}; do
    att="${s##*$'\t'}"; s="${s%%$'\t'*}"   # tab-delimited — a session name can contain spaces
    hit=0; for p in "${projs[@]}"; do [[ "$s" == ${p}-* ]] && { hit=1; break; }; done
    (( hit )) && [[ "$att" == 0 ]] || continue
    pid="$(tmux -L vsct list-panes -s -t "=$s" -F '#{pane_pid}' 2>/dev/null)"
    if [[ -n "$pid" && "$pid" != *$'\n'* ]]; then       # husk check — same TWO shapes as vsct.sh:
      # exec'd (pane process IS the cc/cx viewport client) or legacy (shell + sole client child)
      ps -o args= -p "$pid" 2>/dev/null | grep -qE '^tmux -L c[cx]-[^ ]+ (attach|new-session)' && continue
      kids="$(ps -o args= --ppid "$pid" 2>/dev/null)"
      [[ "$(print -r -- "$kids" | grep -c .)" == 1 ]] \
        && print -r -- "$kids" | grep -qE '^tmux -L c[cx]-[^ ]+ (attach|new-session)' && continue
    fi
    if (( n == 0 )); then TMUX= tmux -L "$srv" new-session -d -s "$out" -n "$s" "TMUX= tmux -L vsct attach -t '=$s'"
    else                  TMUX= tmux -L "$srv" new-window  -t "=$out" -n "$s" "TMUX= tmux -L vsct attach -t '=$s'"
    fi
    n=$((n+1))
  done
  if (( n == 0 )); then echo "vsct-revive: no orphaned ${(j:/:)projs} bunkers — everything is already on screen"; return 0; fi
  echo "vsct-revive: $n bunker(s) restored as windows — click the top bar to pick (from a bunker: ⌃B ⌃B w) · open a terminal to adopt one back into its own tab"
  if [[ -t 0 ]]; then TMUX= tmux -L "$srv" attach -t "=$out"   # nested attach — see cc-revive
  else echo "attach with: TMUX= tmux -L $srv attach -t '=$out'"; fi
}
