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

# CC_DB — the fleet's state store CLI, as install.sh symlinks it into place. Every read and
# write of the primary account goes through it: it validates the roster and mirrors the choice
# into ~/.claude-primary, which the statusline still reads (cc-db.sh:262-275).
typeset -gr _CC_DB="$HOME/.claude/bin/cc-db.sh"

# The launch/account functions below mirror the live fleet's cc-fleet.zsh; the
# Go action protocol emits lines that call them.
# CC_AUTONOMY_FLAGS — the FULL-AUTONOMY posture, applied by EVERY path that starts a chat, on
# every account (cc-fleet.zsh:67-75). Chats run unattended overnight, so a mid-task approval
# prompt is a stalled chat with nobody awake to clear it. `--allow-…` is the enabling half (the
# harness refuses the bypass without it), `--dangerously-…` the acting half; both are required.
# Blast radius is total and deliberate — PreToolUse hooks sit outside the permission system and
# are the only brake left, so a guard that matters belongs in a hook, not in a permission rule.
typeset -ga CC_AUTONOMY_FLAGS=(--allow-dangerously-skip-permissions --dangerously-skip-permissions)
# CC_ENDPOINT_UNSET — every launch strips any inherited API endpoint (cc-fleet.zsh:76-84). A chat
# born inside another chat's Bash tool inherits that chat's environment, so a shell pointed at a
# local translating proxy would hand the next launch a foreign endpoint and it would answer from a
# foreign model under an Anthropic medal. The launcher's verdict is the account; the environment
# gets no vote.
typeset -ga CC_ENDPOINT_UNSET=(
  -u ANTHROPIC_BASE_URL -u ANTHROPIC_AUTH_TOKEN -u ANTHROPIC_MODEL -u ANTHROPIC_SMALL_FAST_MODEL
  -u CLAUDE_CODE_AUTO_COMPACT_WINDOW -u CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC -u CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK
  -u CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY
)
_cc_arm1h() {
  if [[ "${CC_ARM_1H:-}" == 1 ]]; then echo 1
  elif [[ "${ENABLE_PROMPT_CACHING_1H:-}" == 1 && -z "${CLAUDECODE:-}" ]]; then echo 1
  else echo 0; fi
}
_cc_run() {
  local acct="$1" use_tmux="$2"; shift 2
  local cfg; case "$acct" in 2) cfg="$HOME/.cc/$acct" ;; *) cfg="" ;; esac
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
    local run="env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M ${CC_ENDPOINT_UNSET}"
    if [[ -n "$cfg" ]]; then run+=" CLAUDE_CONFIG_DIR=$cfg"; else run+=" -u CLAUDE_CONFIG_DIR"; fi
    if [[ "$arm1h" == 1 ]]; then run+=" ENABLE_PROMPT_CACHING_1H=1"; else run+=" FORCE_PROMPT_CACHING_5M=1"; fi
    # PER-ELEMENT quoting, then join. "${(q)@}" joins the array into ONE word FIRST and quotes
    # that, so two flags arrive as a single argv element with an escaped space, claude rejects the
    # unknown option, and the tmux session dies at birth — which reads as "the launcher doesn't
    # open" (cc-fleet.zsh:108-113).
    run+=" claude ${(j: :)${(@q)CC_AUTONOMY_FLAGS}} ${(j: :)${(@q)@}}"
    if [[ "$in_tmux" == "0" ]]; then tmux -L "$sock" new-session -s "$sock" "$run"
    elif _cc_in_bunker; then TMUX= exec tmux -L "$sock" new-session -s "$sock" "$run"   # viewport dies with the tab
    else TMUX= tmux -L "$sock" new-session -s "$sock" "$run"
    fi
  else
    local -a envargs=(-u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M "${CC_ENDPOINT_UNSET[@]}")
    if [[ -n "$cfg" ]]; then envargs+=(CLAUDE_CONFIG_DIR="$cfg"); else envargs+=(-u CLAUDE_CONFIG_DIR); fi
    if [[ "$arm1h" == 1 ]]; then envargs+=(ENABLE_PROMPT_CACHING_1H=1); else envargs+=(FORCE_PROMPT_CACHING_5M=1); fi
    env "${envargs[@]}" claude "${CC_AUTONOMY_FLAGS[@]}" "$@"
  fi
}
_cc_primary() { local n; n="$(bash "$_CC_DB" primary-get 2>/dev/null)"; case "$n" in 1|2) ;; *) n=1 ;; esac; echo "$n"; }
cc()  { _cc_run "$(_cc_primary)" 1 "$@"; }   # tmux + primary account
cc1() { _cc_run 1 1 "$@"; }                  # tmux + account 1
cc2() { _cc_run 2 1 "$@"; }                  # tmux + account 2
# _cc_run prepends CC_AUTONOMY_FLAGS for every account, so a launcher must NOT pass them again
# (that would duplicate the flags in argv). A caller's own flags still follow and win, so
# `cc1 --permission-mode manual` remains the escape hatch for a supervised chat.
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
  local run="env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u CLAUDE_CONFIG_DIR -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M ${CC_ENDPOINT_UNSET} codex --dangerously-bypass-approvals-and-sandbox ${(q)@}"
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
  case "$n" in 1) dir="$HOME/.claude"; medal="🥇" ;; 2) dir="$HOME/.cc/2"; medal="🥈" ;; esac
  # ACCOUNT 1'S IDENTITY IS NOT INSIDE ITS CONFIG DIR (cc-fleet.zsh:174-182). Claude Code writes
  # the default account's .claude.json BESIDE the config dir; ~/.claude/.claude.json also exists
  # but carries only machine state and no oauthAccount. A uniform "$dir/.claude.json" therefore
  # reads a real file, finds no email, and renders the primary account "(not logged in)" while
  # cc1 logs in perfectly well. Accounts 2+ are explicit CLAUDE_CONFIG_DIRs and DO keep it inside.
  local aj="$dir/.claude.json"; [[ "$dir" == "$HOME/.claude" ]] && aj="$HOME/.claude.json"
  email="$(jq -r '.oauthAccount.emailAddress // empty' "$aj" 2>/dev/null)"
  [[ -z "$email" ]] && email="(not logged in — run cc${n} then /login)"
  print -r -- "$medal $email"
}

# cc-swap [1|2] — fzf picker (no arg) to set which account bare `cc` opens
cc-swap() {
  local cur n; cur="$(_cc_primary)"
  local -a rows=(
    "1 │ $(_cc_label 1)"
    "2 │ $(_cc_label 2)"
  )
  if [[ "${1:-}" =~ ^[12]$ ]]; then
    n="$1"
  elif command -v fzf >/dev/null; then
    local curi="$cur"                           # row position == account number (rows are 1,2)
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
  # primary-set is the only writer: it validates $n against the roster and keeps
  # ~/.claude-primary in lockstep for the statusline (cc-db.sh:268-275). Writing that file
  # directly would skip both.
  bash "$_CC_DB" primary-set "$n" || { print -u2 -- "cc-swap: cc-db.sh primary-set $n failed — primary unchanged"; return 1; }
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
