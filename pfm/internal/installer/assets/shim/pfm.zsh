#!/usr/bin/env zsh
# Post-cutover pfm shell surface. WP10 only prepares this file; WP11 is
# responsible for sourcing it. The binary is installed at ~/.local/bin/pfm.

typeset -gr _PFM_BIN="$HOME/.local/bin/pfm"
if [[ ! -x "$_PFM_BIN" ]]; then
  print -u2 -- "pfm: $_PFM_BIN is missing or not executable"
  return 1
fi

export CLAUDE_DISABLE_ADOPT=1
export CLAUDE_CODE_DISABLE_BG_EXIT_HANDOFF=1
export CLAUDE_CODE_STOP_HOOK_BLOCK_CAP=2

_pfm_eval() {
  local output rc
  output="$("$_PFM_BIN" "$@")"
  rc=$?
  if (( rc == 0 )) && [[ -n "$output" ]]; then
    eval "$output"
    local exit_status=$?
    # An action that ATTACHED a chat handed this terminal to the harness, so the terminal ends
    # with it. Escaping the picker emits no line at all and never reaches here — Esc still
    # lands back at the prompt.
    [[ "$output" != *tmux* ]] || _cc_own_terminal "$exit_status"
  fi
  return "$rc"
}

cc-ls() {
  case " $* " in
    *" --plain "*|*" --tsv "*)
      "$_PFM_BIN" ls "$@"
      return $?
      ;;
  esac
  _pfm_eval ls "$@"
}

cc-open() { _pfm_eval open "$@"; }
cc-revive() { "$_PFM_BIN" revive "$@"; }

# The launch/account functions below are the shell owner of fresh interactive
# launches; the Go action protocol emits lines that call them.
# PFM_CLAUDE_PROMPTED and PFM_CODEX_YOLO are emitted by the installer from the
# machine config. A prompted Claude account gets no bypass pair; a Codex account
# with yolo=false gets no approval bypass. The launch functions below are the
# only shell paths that decide these flags.
typeset -gA PFM_CLAUDE_PROMPTED=()
typeset -gA PFM_CODEX_YOLO=()
# CC_ENDPOINT_UNSET — every launch strips any inherited API endpoint. A chat
# born inside another chat's Bash tool inherits that chat's environment, so a shell pointed at a
# local translating proxy would hand the next launch a foreign endpoint and it would answer from a
# foreign model under an Anthropic medal. The launcher's verdict is the account; the environment
# gets no vote.
typeset -ga CC_ENDPOINT_UNSET=(
  -u ANTHROPIC_BASE_URL -u ANTHROPIC_AUTH_TOKEN -u ANTHROPIC_MODEL -u ANTHROPIC_SMALL_FAST_MODEL
  -u CLAUDE_CODE_AUTO_COMPACT_WINDOW -u CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC -u CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK
  -u CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY
)
# CC_SESSION_UNSET — the inherited SESSION IDENTITY, stripped by every path that starts a chat.
# A chat born inside another chat's Bash tool inherits that chat's markers, and each one lies in a
# different way: CLAUDE_CODE_SESSION_ID makes the newborn answer to its parent's id, CLAUDECODE
# makes it believe it is already inside a harness, and CLAUDE_CODE_CHILD_SESSION marks it a
# SUBORDINATE — which silently turns transcript saving OFF. That last one is the quiet one: the
# chat runs perfectly, and only the footer whispers "Transcript saving is off", so the loss is
# discovered when someone goes looking for a conversation that was never written. Stripping the
# marker restores the default; forcing persistence back on with
# CLAUDE_CODE_FORCE_SESSION_PERSISTENCE would paper over an identity the chat should never have
# had. One array, three launch paths — a list written three times is a list that gets fixed twice.
typeset -ga CC_SESSION_UNSET=(-u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u CLAUDE_CODE_CHILD_SESSION)
_cc_arm1h() {
  if [[ "${CC_ARM_1H:-}" == 1 ]]; then echo 1
  elif [[ "${ENABLE_PROMPT_CACHING_1H:-}" == 1 && -z "${CLAUDECODE:-}" ]]; then echo 1
  else echo 0; fi
}
_cc_run() {
  local acct="$1" use_tmux="$2"; shift 2
  local cfg; case "$acct" in 2) cfg="$HOME/.cc/$acct" ;; *) cfg="" ;; esac
  local -a autonomy_flags=()
  if [[ "${PFM_CLAUDE_PROMPTED[$acct]:-0}" != 1 ]]; then
    autonomy_flags=(--allow-dangerously-skip-permissions --dangerously-skip-permissions)
  fi
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
    local run="env ${CC_SESSION_UNSET} -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M ${CC_ENDPOINT_UNSET}"
    if [[ -n "$cfg" ]]; then run+=" CLAUDE_CONFIG_DIR=$cfg"; else run+=" -u CLAUDE_CONFIG_DIR"; fi
    if [[ "$arm1h" == 1 ]]; then run+=" ENABLE_PROMPT_CACHING_1H=1"; else run+=" FORCE_PROMPT_CACHING_5M=1"; fi
    # PER-ELEMENT quoting, then join. "${(q)@}" joins the array into ONE word FIRST and quotes
    # that, so two flags arrive as a single argv element with an escaped space, claude rejects the
    # unknown option, and the tmux session dies at birth.
    run+=" claude ${(j: :)${(@q)autonomy_flags}} ${(j: :)${(@q)@}}"
    # A bare terminal hands itself to the chat (_cc_own_terminal); a bunker pane is EXEC'd into
    # the viewport, which is the same law by another route. Inside another chat, neither: a
    # nested viewport that closed on exit would take its host chat's pane down with it.
    if [[ "$in_tmux" == "0" ]] && _cc_owns_terminal; then TMUX= exec tmux -L "$sock" new-session -s "$sock" "$run"
    elif [[ "$in_tmux" == "0" ]]; then tmux -L "$sock" new-session -s "$sock" "$run"; _cc_own_terminal $?
    elif _cc_in_bunker; then TMUX= exec tmux -L "$sock" new-session -s "$sock" "$run"   # viewport dies with the tab
    else
      TMUX= tmux -L "$sock" new-session -s "$sock" "$run"
      # Reached inside another chat, where _cc_own_terminal declines by design. It is called
      # anyway as the net under the exec above: a bunker pane that somehow spawned its viewport
      # as a CHILD would otherwise outlive the chat and sit there as a bare prompt.
      _cc_own_terminal $?
    fi
  else
    local -a envargs=("${CC_SESSION_UNSET[@]}" -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M "${CC_ENDPOINT_UNSET[@]}")
    if [[ -n "$cfg" ]]; then envargs+=(CLAUDE_CONFIG_DIR="$cfg"); else envargs+=(-u CLAUDE_CONFIG_DIR); fi
    if [[ "$arm1h" == 1 ]]; then envargs+=(ENABLE_PROMPT_CACHING_1H=1); else envargs+=(FORCE_PROMPT_CACHING_5M=1); fi
    env "${envargs[@]}" claude "${autonomy_flags[@]}" "$@"
    _cc_own_terminal $?
  fi
}
_cc_primary() { local n; n="$("$_PFM_BIN" internal primary-get 2>/dev/null)"; case "$n" in 1|2) ;; *) n=1 ;; esac; echo "$n"; }
cc()  { _cc_run "$(_cc_primary)" 1 "$@"; }   # tmux + primary account
cc1() { _cc_run 1 1 "$@"; }                  # tmux + account 1
cc2() { _cc_run 2 1 "$@"; }                  # tmux + account 2
# _cc_run prepends the configured Claude flags for every account, so a launcher must NOT pass them again
# (that would duplicate the flags in argv). A caller's own flags still follow and win, so
# `cc1 --permission-mode manual` remains the escape hatch for a supervised chat.
# cx — a CODEX chat on the same per-chat-server pattern, socket prefix cx-* instead of cc-*.
# The prefix IS the engine marker: codex writes no statusline breadcrumbs and no ~/.claude
# transcript, so cc-ls recognizes (and lists) a live Codex chat by socket name alone. Claude
# accounts / ⚡1h don't apply — codex has its own single auth (~/.codex).
cx() {
  local sock="cx-$(date +%s)-$$-$RANDOM"
  local acct; acct="$(_cc_primary)"
  local -a codex_flags=()
  if [[ "${PFM_CODEX_YOLO[$acct]:-1}" == 1 ]]; then
    codex_flags=(--dangerously-bypass-approvals-and-sandbox)
  fi
  # same launch hygiene as claude: a codex born inside a Claude chat must not inherit its identity.
  # PER-ELEMENT quoting, then join — same fix as _cc_run (lines 89-93): "${(q)@}" joins the
  # array into ONE word FIRST and quotes that, so `cx --resume abc123` arrives as a single
  # escaped argv element ("--resume\ abc123") and codex rejects it as one unknown flag.
  local run="env ${CC_SESSION_UNSET} -u CLAUDE_CONFIG_DIR -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M ${CC_ENDPOINT_UNSET} codex ${(j: :)${(@q)codex_flags}} ${(j: :)${(@q)@}}"
  _cx_server "$sock" "$PWD" "$run" || return
  if _cc_selfswitch "$sock"; then :                          # already inside it → switch, never nest
  elif _cc_in_bunker; then TMUX= exec tmux -L "$sock" attach # viewport dies with the tab
  elif _cc_owns_terminal; then TMUX= exec tmux -L "$sock" attach # bare terminal dies with the harness
  else
    TMUX= tmux -L "$sock" attach
    _cc_own_terminal $?   # bare terminal: the chat took it, the chat closes it
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
  # ACCOUNT 1'S IDENTITY IS NOT INSIDE ITS CONFIG DIR. Claude Code writes
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
  "$_PFM_BIN" internal primary-set "$n" || { print -u2 -- "cc-swap: primary-set $n failed — primary unchanged"; return 1; }
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

# _cc_owns_terminal — true when replacing this shell is safe and gives the
# harness structural ownership of the terminal. Scripts, pipes and shells
# inside another chat never qualify.
_cc_owns_terminal() {
  [[ -o interactive && -t 0 && -t 1 ]] || return 1
  [[ -z "$TMUX" ]] || _cc_in_bunker
}

# _cc_own_terminal <status> — the terminal belongs to the harness that was typed into it, so
# a harness that ENDS takes the terminal with it (the VS Code tab closes) instead of falling
# back to a prompt nobody asked for. Three guards keep it from eating anything else: only an
# interactive shell on a real tty is ever ended, so a script, a Bash-tool shell and a piped
# run return untouched; only a caller that actually ran a harness calls it, so the picker
# (cc-ls) still escapes to the prompt on Esc; and a NON-ZERO exit holds the terminal for a
# keypress first — a crash the tab swallowed is a crash that never happened. zsh's refusal to
# exit while jobs are suspended is the right refusal, and is left alone.
_cc_own_terminal() {
  local exit_status="${1:-0}"
  _cc_owns_terminal || return "$exit_status"
  # A bare terminal or a bunker pane exists to hold a harness, so it goes when the harness
  # goes. Inside any OTHER tmux — a chat pane, an ad-hoc work window — the terminal is
  # someone else's and closing it would take the host down too.
  if (( exit_status )); then
    print -u2 -- "pfm: harness exited $exit_status — press any key to close this terminal"
    read -k1 -s
  fi
  exit "$exit_status"
}

# _cc_tui_call — true when these arguments make the harness take over the screen. A print,
# version or subcommand call writes its answer INTO this terminal and hands it back, so that
# terminal is not the harness's to close. Resume flags are absent on purpose: `claude -r` and
# `codex resume` open the full TUI and own the terminal like any other chat.
_cc_tui_call() {
  local argument
  for argument in "$@"; do
    case "$argument" in
      -p|--print|-v|--version|-h|--help|doctor|update|install|mcp|config|migrate-installer|login|logout|exec|proto|apply)
        return 1 ;;
    esac
  done
  return 0
}

# A harness typed straight into a terminal closes that terminal on exit, exactly as a fleet
# chat does. `command` keeps the wrapper from recursing, and only this shell's own typing is
# affected — pfm, hooks and scripts exec the binary and never see these functions.
claude() {
  command claude "$@"
  local exit_status=$?
  _cc_tui_call "$@" && _cc_own_terminal "$exit_status"
  return "$exit_status"
}

codex() {
  command codex "$@"
  local exit_status=$?
  _cc_tui_call "$@" && _cc_own_terminal "$exit_status"
  return "$exit_status"
}

# _cc_selfswitch is retained because cx() depends on the legacy same-server
# recursion guard.
_cc_selfswitch() {
  local sock="$1" w
  [[ -n "${TMUX:-}" && "${TMUX%%,*}" == */"$sock" ]] || return 1
  # the chat's window = the one whose pane runs the engine, in DESCENDING order of certainty and
  # lowest window index first within each tier. An exact engine name is proof; `node` and the bare
  # version string are only hints — codex renders as its node wrapper and claude as a bare version
  # in some states, but a shell window running any node process wears the same shape, and matching
  # it first would switch the operator to that shell, i.e. the very "wrong window" this fixes.
  local panes; panes="$(tmux -L "$sock" list-panes -a -F '#{window_index}'$'\t''#{pane_current_command}' 2>/dev/null | sort -n)"
  w="$(print -r -- "$panes" | awk -F'\t' '$2 ~ /^(claude|codex)$/ { print $1; exit }')"
  [[ -n "$w" ]] || w="$(print -r -- "$panes" | awk -F'\t' '$2 ~ /^node$/ || $2 ~ /^[0-9]+\./ { print $1; exit }')"
  # engine window unidentifiable (an unexpected pane command) → the lowest window index, where every
  # cc-/cx- server puts the chat at birth. Announce only what actually happened: claiming a switch
  # that never ran would send the operator back to a screen that did not change.
  [[ -n "$w" ]] || w="$(print -r -- "$panes" | awk -F'\t' 'NR==1 { print $1 }')"
  if [[ -n "$w" ]] && tmux select-window -t "$w" 2>/dev/null; then
    print -u2 -- "cc: already inside this chat's tmux — switched to its window (a session must never nest inside itself)"
  else
    print -u2 -- "cc: already inside this chat's tmux — refusing to nest it inside itself; switch windows yourself (prefix + w)"
  fi
  return 0
}

# vsct-revive remains zsh: it owns the non-chat bunker dashboard, outside the
# pfm chat database.
vsct-revive() {
  local -a projs=("$@"); (( $# )) || projs=("${PWD:t}")
  projs=("${projs[@]//[.:]/_}")   # tmux stored '.' and ':' as '_' when the bunker was born
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

# ── auto-open: a new terminal lands straight in a chat ────────────────────
# LAST in this file on purpose, and the reason is worth the paragraph. A terminal profile that
# wants a chat on open — VS Code's "tmux + claude" is the canonical one — used to carry the hook
# in ~/.zshrc itself:  [[ -n "$VSCODE_AUTO_CC" ]] && cc.  But the installer APPENDS its source
# line to the BOTTOM of ~/.zshrc, so that call sat above the line that defines `cc`, and zsh does
# not report that as an error: `cc` is the POSIX C compiler, present on every box. The terminal
# opened by running clang with no arguments —
#     clang: error: no input files
# — and then the fleet loaded a few lines further down, so typing `cc` or `cc-ls` by hand always
# worked. That gap is what made it look like a broken picker rather than a line-ordering bug.
# Owning the hook here settles the ordering for every adopter: nothing can run it before the
# functions exist, because there is nothing after it.
#
# But "after this file" is still not late enough, so the launch is DEFERRED to the first prompt.
# ~/.zshrc keeps going after the source line — a host's PATH edits that make `pfm`, `tmux` or
# `claude` resolvable, its own overrides of anything above — and launching from here would run a
# chat before the shell had finished becoming itself, the same class of ordering bug one layer
# down. A one-shot precmd hook runs when the shell is fully configured and about to draw its
# first prompt, whatever else the rc files do after this line.
if [[ -o interactive && -z "${CLAUDECODE:-}" && -n "${CC_AUTO_OPEN:-}${VSCODE_AUTO_CC:-}" ]]; then
  # Read the value, then unset BEFORE arming, both spellings. The variable is exported by the
  # terminal profile, so it is inherited: a shell opened inside the chat would open a chat
  # inside the chat.
  typeset -g _cc_auto_what="${CC_AUTO_OPEN:-$VSCODE_AUTO_CC}"
  unset CC_AUTO_OPEN VSCODE_AUTO_CC
  autoload -Uz add-zsh-hook
  _cc_auto_open() {
    # Disarm FIRST. The command below RETURNS — when the chat exits, or when the picker is
    # dismissed — and the shell then draws another prompt. A hook still registered at that
    # moment is a terminal that reopens whatever you just closed, forever.
    add-zsh-hook -d precmd _cc_auto_open
    unfunction _cc_auto_open
    # The value chooses, from a WHITELIST — never `eval`, and never run as-is. It arrives from
    # the environment, which a terminal profile, a parent process or an ssh client can set.
    local cmd
    case "$_cc_auto_what" in
      cc|new|chat)  cmd=cc ;;                 # straight into a fresh chat, no picker
      cc1|cc2)      cmd="$_cc_auto_what" ;;   # a fresh chat on that account — the roster is two
      cx|codex)     cmd=cx ;;                 # a fresh Codex chat
      *)            cmd=cc-ls ;;              # 1 / yes / ls / picker — and the default
    esac
    unset _cc_auto_what
    $cmd
  }
  add-zsh-hook precmd _cc_auto_open
fi
