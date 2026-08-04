#!/usr/bin/env zsh
# cc-fleet.zsh — Claude Code multi-account launchers + the cc-ls chat picker (Claude + Codex).
# Sourced from ~/.zshrc — install.sh writes that source line and symlinks this bundle into place.
# zsh-only: glob qualifiers, ${(f)…}, assoc arrays, ${var:t:r}. One home for the whole fleet.

# ── builtins instead of forks ─────────────────────────────────────────────
# cc-ls's warm cost was never file I/O — it was process creation. One run measured 803 execve
# calls and 7.4s wall, of which only 0.77s was in the named scan phases; the rest was `date`,
# `stat` and `head|grep` spawned once per ROW. These three modules do the same jobs in-process:
#   zsh/datetime → $EPOCHSECONDS instead of `date +%s`
#   zsh/stat     → zstat instead of `stat -c`
#   zsh/system   → sysread for a BOUNDED read instead of `head -c | grep` (never $(<file):
#                  a transcript can be hundreds of MB and slurping one would be far worse)
zmodload -F zsh/datetime b:strftime 2>/dev/null
zmodload zsh/datetime 2>/dev/null
zmodload zsh/stat 2>/dev/null
zmodload zsh/system 2>/dev/null

# ── the GNU/BSD seam ──────────────────────────────────────────────────────
# Every question whose answer differs between Linux and macOS is asked through cc-portable.sh,
# never inline here. It is POSIX sh on purpose so this zsh file and the bundle's bash scripts
# share one implementation instead of drifting two. CC_FLEET_HOME is resolved THROUGH symlinks
# because ~/.zshrc sources this file from wherever the clone lives.
_ccfs="${(%):-%N}"; while [[ -L "$_ccfs" ]]; do _ccfd="${_ccfs:h:A}"; _ccfs="$(readlink "$_ccfs")"; [[ "$_ccfs" == /* ]] || _ccfs="$_ccfd/$_ccfs"; done
CC_FLEET_HOME="${CC_FLEET_HOME:-${_ccfs:h:A}}"
[[ -r "$CC_FLEET_HOME/cc-portable.sh" ]] && source "$CC_FLEET_HOME/cc-portable.sh"

# ── no sneaky agent conversions ───────────────────────────────────────────
# tmux is this fleet's survival layer — a chat killed mid-task must DIE, not resurrect as a
# hidden background agent under its birth account. These two stop the daemon's silent
# conversions (exit handoff + in-flight adoption) while leaving deliberate background work
# (wave workers, RR sub-agents, --bg) untouched. Read at process birth — live chats unaffected.
export CLAUDE_DISABLE_ADOPT=1
export CLAUDE_CODE_DISABLE_BG_EXIT_HANDOFF=1

# ── goal-hook churn cap ───────────────────────────────────────────────────
# An idle chat holding an active /goal argues with the goal-continuation stop hook
# 9 consecutive times per idle turn (~90s + ~1K tokens each cycle, every builder,
# every between-BRIEF hold). Two blocks keep the nag's reminder value; nine is churn.
# Read at process birth — live chats keep the old cap until relaunched.
export CLAUDE_CODE_STOP_HOOK_BLOCK_CAP=2

# ── Claude Code: triple account (Linux-native, file-based creds) ──────────
# ONE canonical path per account: ~/.cc/N (1 🥇 · 2 🥈 · 3 🥉). What ~/.cc/N points at is
# storage trivia — on this box 1 → ~/.claude (Claude's default dir; account 1 launches with
# CLAUDE_CONFIG_DIR unset), 2 → the legacy ~/.claude3 dir (2026-07 renumber; live sessions
# still hold the old path — fold into a real dir once they drain, zero script changes), and
# 3 is a real dir. Scripts speak ACCOUNT NUMBERS only; never reference a ~/.claudeN path.
# One /login each, ever — no credential copying (a copied OAuth token forks
# and dies). Launches inside tmux so agents survive SSH drops. ~/.claude-primary
# holds which acct bare `cc` opens (default 1). cc-swap <1|2|3> changes it.
# cc1/cc2/cc3 force an account.
# _cc_arm1h — 1 when THIS launch should be born with the ⚡1h-cache: CC_ARM_1H=1 (the picker's
# explicit per-pick verdict) or ENABLE_PROMPT_CACHING_1H=1 from a NON-chat shell (the documented
# `ENABLE_PROMPT_CACHING_1H=1 cc` interface). Inside a chat's tool shell (CLAUDECODE set) the
# inherited flag is the HOST chat's birth env leaking downstream — it never arms a new launch.
# SINCE CC 2.1.215 the harness defaults EVERY session to the 1h TTL — merely omitting the
# ENABLE flag no longer yields 5m (that regression shipped silently and burned us). Un-armed
# launches must now actively set FORCE_PROMPT_CACHING_5M=1; armed ones still set ENABLE=1,
# which doubles as the birth marker _cc_c1h_tty reads. Verified 2026-07-19 via -p probes:
# default env → ephemeral_1h writes; FORCE_PROMPT_CACHING_5M=1 → ephemeral_5m writes.
_cc_arm1h() {
  if [[ "${CC_ARM_1H:-}" == 1 ]]; then echo 1
  elif [[ "${ENABLE_PROMPT_CACHING_1H:-}" == 1 && -z "${CLAUDECODE:-}" ]]; then echo 1
  else echo 0; fi
}
# ── account 4: the GPT account ────────────────────────────────────────────
# Account 4 is not an Anthropic login. It is the same Claude Code harness — same commands,
# skills, hooks, subagents, MCP — pointed at a local claude-code-proxy, which translates the
# Anthropic API traffic into ChatGPT/Codex subscription calls. GPT-5.6 answers; everything
# else about the chat is identical, so cc-ls lists it and cc-open resumes it like any other.
# The proxy runs as a systemd --user unit you supply, on loopback
# 18765; if it is down, a cc4 chat simply cannot connect. Quota comes out of the ChatGPT plan
# the cx codex chats already spend — a separate pool from accounts 1-3, which is the point.
typeset -ga CC4_ENV=(
  "ANTHROPIC_BASE_URL=http://127.0.0.1:18765"
  "ANTHROPIC_AUTH_TOKEN=unused"                  # the proxy holds the real credential; this only satisfies the client
  "ANTHROPIC_MODEL=gpt-5.6-sol[1m]"              # [1m] is a local compaction hint, stripped before the upstream call
  "ANTHROPIC_SMALL_FAST_MODEL=gpt-5.6-luna[1m]"  # titles + background work
  "CLAUDE_CODE_AUTO_COMPACT_WINDOW=272000"       # the real ChatGPT context limit for GPT-5.6
  "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1"
  "CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK=1"  # a non-streaming retry of a partial stream duplicates tool calls
  "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1" # /model reads the proxy's catalog instead of the harness's built-in tiers
)
# CC_AUTONOMY_FLAGS — the FULL-AUTONOMY posture, defined once and applied by EVERY path that starts
# a chat: _cc_run (fresh launches, tmux and bare) and both --resume paths, on EVERY account. Standing
# order 2026-07-31 — chats run unattended overnight, so a mid-task approval prompt is a stalled chat
# with nobody awake to clear it. `--allow-…` is the enabling half (the harness refuses the bypass
# without it), `--dangerously-…` the acting half; both are required.
# Blast radius is total and deliberate: no prompt, no classifier, anything this user can do on any
# host it can reach. PreToolUse hooks still fire (they sit outside the permission system) — they are
# the only brake left, so a guard that matters belongs in a hook, not in a permission rule.
typeset -ga CC_AUTONOMY_FLAGS=(--allow-dangerously-skip-permissions --dangerously-skip-permissions)
# Every launch strips these, account 4 included (it re-adds them as env ARGUMENTS below). A
# chat born inside a cc4 chat's Bash tool inherits ANTHROPIC_BASE_URL, and without the strip
# an account-1/2/3 launch would silently answer from GPT under an Anthropic medal.
typeset -ga CC4_UNSET=(
  -u ANTHROPIC_BASE_URL -u ANTHROPIC_AUTH_TOKEN -u ANTHROPIC_MODEL -u ANTHROPIC_SMALL_FAST_MODEL
  -u CLAUDE_CODE_AUTO_COMPACT_WINDOW -u CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC -u CLAUDE_CODE_DISABLE_NONSTREAMING_FALLBACK
  -u CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY
)

_cc_run() {
  local acct="$1" use_tmux="$2"; shift 2
  local cfg; case "$acct" in 2|3|4) cfg="$HOME/.cc/$acct" ;; *) cfg="" ;; esac
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
    local run="env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M ${CC4_UNSET}"
    if [[ -n "$cfg" ]]; then run+=" CLAUDE_CONFIG_DIR=$cfg"; else run+=" -u CLAUDE_CONFIG_DIR"; fi
    if [[ "$arm1h" == 1 ]]; then run+=" ENABLE_PROMPT_CACHING_1H=1"; else run+=" FORCE_PROMPT_CACHING_5M=1"; fi
    [[ "$acct" == 4 ]] && run+=" ${(j: :)${(@q)CC4_ENV}}"   # per-element quoting: [1m] is a glob to the sh that runs this, and (q) on a joined array would collapse all seven into one variable
    # PER-ELEMENT quoting, then join — the same trap CC4_ENV hit above. "${(q)@}" joins the array
    # into ONE word FIRST and quotes that, so two flags arrive as a single argv element with an
    # escaped space ("--allow-…\ --dangerously-…"), claude rejects the unknown option, and the
    # tmux session dies at birth — which reads as "cc4 doesn't open". Latent until a launcher
    # passed more than one argument; `cc --model x` style calls were mis-quoted the same way.
    run+=" claude ${(j: :)${(@q)CC_AUTONOMY_FLAGS}} ${(j: :)${(@q)@}}"
    if [[ "$in_tmux" == "0" ]]; then tmux -L "$sock" new-session -s "$sock" "$run"
    elif _cc_in_bunker; then TMUX= exec tmux -L "$sock" new-session -s "$sock" "$run"   # viewport dies with the tab
    else TMUX= tmux -L "$sock" new-session -s "$sock" "$run"
    fi
  else
    local -a envargs=(-u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M "${CC4_UNSET[@]}")
    if [[ -n "$cfg" ]]; then envargs+=(CLAUDE_CONFIG_DIR="$cfg"); else envargs+=(-u CLAUDE_CONFIG_DIR); fi
    if [[ "$arm1h" == 1 ]]; then envargs+=(ENABLE_PROMPT_CACHING_1H=1); else envargs+=(FORCE_PROMPT_CACHING_5M=1); fi
    [[ "$acct" == 4 ]] && envargs+=("${CC4_ENV[@]}")
    env "${envargs[@]}" claude "${CC_AUTONOMY_FLAGS[@]}" "$@"
  fi
}
# CC_FLEET_HOME — the bundle's OWN directory, resolved THROUGH symlinks (%x is the file being
# sourced, :A canonicalises it). The install layer symlinks this bundle into place, so a plain
# dirname would send every sibling lookup into the link's directory instead of the clone.
typeset -g CC_FLEET_HOME="${CC_FLEET_HOME:-${${(%):-%x}:A:h}}"
# CC_DB — the fleet's state store. Every read/write of hidden chats, the primary account, spawned
# children and the swap log goes through it instead of the sidecar files those used to live in.
# It degrades to those same files when sqlite3 is missing, so the picker is never down.
typeset -g CC_DB="$CC_FLEET_HOME/cc-db.sh"
_cc_primary() { local n; n="$(bash "$CC_DB" primary-get 2>/dev/null)"; case "$n" in 1|2|3|4) ;; *) n=1 ;; esac; echo "$n"; }
cc()  { _cc_run "$(_cc_primary)" 1 "$@"; }   # tmux + primary account
cc1() { _cc_run 1 1 "$@"; }                  # tmux + account 1
cc2() { _cc_run 2 1 "$@"; }                  # tmux + account 2
cc3() { _cc_run 3 1 "$@"; }                  # tmux + account 3
# Account 4 is no longer special here — _cc_run prepends CC_AUTONOMY_FLAGS for every account, so cc4
# must NOT pass them again (that would duplicate the flags in argv). A caller's own flags still
# follow and win, so `cc1 --permission-mode manual` remains the escape hatch for a supervised chat.
cc4() { _cc_run 4 1 "$@"; }
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
  local run="env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u CLAUDE_CONFIG_DIR -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M ${CC4_UNSET} codex --dangerously-bypass-approvals-and-sandbox ${(q)@}"
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
  # account 4 has no Anthropic login to name — label it by what actually answers, and say so
  # when the proxy is down, because that is the only way a cc4 chat fails.
  if [[ "$n" == 4 ]]; then
    if cc_listening 18765; then print -r -- "🍀 GPT-5.6 · codex proxy"
    else print -r -- "🍀 GPT-5.6 · codex proxy (DOWN — systemctl --user start claude-code-proxy)"; fi
    return
  fi
  case "$n" in 1) dir="$HOME/.claude"; medal="🥇" ;; 2) dir="$HOME/.cc/2"; medal="🥈" ;; 3) dir="$HOME/.cc/3"; medal="🥉" ;; esac
  # ACCOUNT 1'S IDENTITY IS NOT INSIDE ITS CONFIG DIR. Claude Code writes the default account's
  # .claude.json BESIDE the config dir (~/.claude.json), not into it — ~/.claude/.claude.json
  # also exists but carries only machine state (machineID, projects, seenNotifications) and no
  # oauthAccount. A uniform "$dir/.claude.json" therefore reads a real file, finds no email, and
  # renders the primary account "(not logged in)" while cc1 logs in perfectly well: a convincing
  # wrong answer, which is the kind worth a special case. Accounts 2+ are explicit
  # CLAUDE_CONFIG_DIRs and DO keep .claude.json inside. Same rule the statusline badge follows,
  # for the same reason — change one, change the other.
  local aj="$dir/.claude.json"; [[ "$dir" == "$HOME/.claude" ]] && aj="$HOME/.claude.json"
  email="$(jq -r '.oauthAccount.emailAddress // empty' "$aj" 2>/dev/null)"
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
    "4 │ $(_cc_label 4)"
  )
  if [[ "${1:-}" =~ ^[1234]$ ]]; then
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
  bash "$CC_DB" primary-set "$n"   # writes the db AND keeps ~/.claude-primary in lockstep for the statusline
  echo "Primary → account $n  ($(_cc_label $n))"
  echo "  cc       → account $n"
  echo "  cc1/cc2  → explicit account"
}

# cc-revive — after VS Code kills the terminals (hard disconnect), ONE command restores every
# live chat: each unattached cc-* server becomes a window (named from its /rename title or last
# prompt) of a single outer "revive" tmux session, nested-attached to the chat's own socket.
# Idempotent — rebuilds the dashboard every run; chats already visible somewhere are skipped.
# The workspace folderOpen task auto-runs this after reloads (12s grace so VS Code's own
# persistent-session revival gets first claim). ⌃B w pick · ⌃B n/p cycle · ⌃B d leave.
cc-revive() {
  local dir="${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)" siddir=/tmp/cc-sid   # tmux appends /tmux-$UID to TMUX_TMPDIR
  local s sock att nm tp t n=0 rpp rl
  _cx_scan                                          # pane pid → rollout map for naming cx windows
  tmux -L revive kill-server 2>/dev/null            # rebuild from scratch each run
  for s in "$dir"/cc-[0-9]*(N) "$dir"/cx-[0-9]*(N); do   # real chats (cc-<epoch>-…) + Codex (cx-…); skips cc-new-* teammates
    sock="${s:t}"
    att="$(tmux -L "$sock" ls -F '#{session_attached}' 2>/dev/null | head -1)"
    [[ -z "$att" || "$att" != 0 ]] && continue      # dead socket file, or already on screen
    nm="$sock"; tp=""
    if [[ "$sock" == cx-* ]]; then                  # Codex: name from its live rollout (thread_name / first prompt)
      nm="Codex"
      rpp="$(tmux -L "$sock" list-panes -a -F '#{pane_pid}' 2>/dev/null | head -1)"
      [[ -n "$rpp" ]] && rl="$(_cx_rollout "$rpp")" && { t="$(_cx_name "$rl")"; [[ -n "$t" ]] && nm="${t[1,24]}"; }
    fi
    [[ -r "$siddir/$sock" ]] && tp="$(<"$siddir/$sock")"
    if [[ -n "$tp" && -r "$tp" ]]; then
      t="$(grep -aE '"type":"(custom-title|agent-name)"' "$tp" 2>/dev/null | tail -1 | jq -r '.customTitle // .agentName // empty' 2>/dev/null)"
      [[ -z "$t" ]] && t="$(grep -a '"type":"ai-title"' "$tp" 2>/dev/null | tail -1 | jq -r '.aiTitle // empty' 2>/dev/null)"   # never renamed → the harness's own title
      [[ -z "$t" ]] && t="$(_cc_lastprompt "$tp")"
      [[ -n "$t" ]] && nm="${t[1,24]}"
    fi
    if (( n == 0 )); then TMUX= tmux -L revive new-session -d -s revive -n "$nm" "TMUX= tmux -L '$sock' attach"
    else                  TMUX= tmux -L revive new-window  -t revive -n "$nm" "TMUX= tmux -L '$sock' attach"
    fi
    n=$((n+1))
  done
  if (( n == 0 )); then echo "cc-revive: every live chat is already on screen — nothing to restore"; return 0; fi
  echo "cc-revive: restored $n chat(s) as tmux windows — click the top bar to pick · ⌃B w (from a bunker: ⌃B ⌃B w)"
  # nested attach is this fleet's native posture — every terminal lives in a vsct bunker now,
  # so a $TMUX guard would mean never attaching at all; a tty is the only real requirement
  if [[ -t 0 ]]; then TMUX= tmux -L revive attach -t revive
  else echo "attach with: TMUX= tmux -L revive attach"; fi
}

# vsct-revive [project] — after a hard reload/disconnect, restore this project's orphaned
# plain-terminal BUNKERS (vsct sessions with no client) as windows of one dashboard session,
# nested-attached — cc-revive's sibling for non-chat terminals. Native tab revival gets first
# claim (the folderOpen task sleeps 12s); bunkers already on screen are skipped; chat-viewport
# husks are skipped too (vsct.sh kills those at adoption — showing one would re-ambush).
# To move a bunker back into its own tab: just open a terminal — adoption steals it from
# the dashboard (attach -d; its window closes). Closing the window first still works too.
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
      kids="$(cc_children_args "$pid")"
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

# _cc_acct <path> — account number a config-dir-anchored path belongs to. Transcript
# breadcrumbs carry the LAUNCHING account's dir (claude reports paths under its own config
# dir), so a live chat's crumb reveals its birth account. Legacy spellings (~/.claude2,
# ~/.claude3 — pre-renumber envs still live in old sessions) map to account 2. "" = unknown.
_cc_acct() {
  case "$1" in
    "$HOME/.cc/2"|"$HOME/.cc/2/"*|"$HOME/.claude3"|"$HOME/.claude3/"*|"$HOME/.claude2"|"$HOME/.claude2/"*) echo 2 ;;
    "$HOME/.cc/3"|"$HOME/.cc/3/"*) echo 3 ;;
    "$HOME/.cc/4"|"$HOME/.cc/4/"*) echo 4 ;;
    "$HOME/.claude"|"$HOME/.claude/"*) echo 1 ;;
    *) echo "" ;;
  esac
}
_cc_medal() { case "$1" in 1) echo 🥇 ;; 2) echo 🥈 ;; 3) echo 🥉 ;; 4) echo 🍀 ;; *) echo "" ;; esac }

# _cc_isgpt <uuid> — was this chat born on account 4 (GPT)? Every launch drops a per-session
# breadcrumb in its own config dir's session-env/, so the account survives even for a chat with
# no live tmux — the shared transcript store cannot answer this on its own.
_cc_isgpt() { [[ -n "$1" && -e "$HOME/.cc/4/session-env/$1" ]] }

# _cc_resume_acct <uuid> — which account may host this resume. GPT and Claude chats NEVER cross:
# a transcript full of GPT turns replayed to Anthropic is a foreign conversation billed to the
# wrong account (and vice versa), so a chat born on 4 resumes only on 4, and a Claude-born chat
# never lands on 4. Among accounts 1-3 the primary still decides, exactly as before.
_cc_resume_acct() {
  local p; p="$(_cc_primary)"
  if _cc_isgpt "$1"; then echo 4; return; fi
  [[ "$p" == 4 ]] && { echo 1; return; }    # Claude chat while the GPT account is primary → account 1
  echo "$p"
}

# _cc_in_bunker — true when this shell IS a vsct bunker pane. Chat opens from a bunker exec
# INTO the tmux client: the viewport dies with the tab instead of lingering as an orphaned
# husk that the next fresh terminal adopts (the ORCHESTRATOR ambush). Chats themselves live
# on their own cc-* servers and are untouched. Never true inside a chat pane or a plain
# shell — those keep child-spawned clients (a Bash-tool shell must never be exec'd away).
_cc_in_bunker() { [[ "${TMUX%%,*}" == */vsct ]] }

# _cc_selfswitch <sock> — 0 (and switches) when THIS shell already lives on that chat's OWN tmux
# server, so the caller must NOT attach. Every attach path clears $TMUX — the fleet's native
# posture, since chats live in vsct bunkers and tmux's nesting refusal would otherwise mean never
# attaching at all — and that refusal is the ONLY thing normally stopping a session being attached
# to ITSELF. Self-attached, the session mirrors recursively and `window-size latest` lets the inner
# client clamp the window: seen live 2026-07-26, a chat's window collapsed to 177x1 and read as
# dead while the chat underneath was perfectly healthy (its shell window had run cc-ls and picked
# the very chat it was sitting in). Switching to the chat's own window is what the founder meant
# anyway — "show me that chat" — so this is the right answer, not merely the safe one.
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

# _cc_c1h_tty <tty[,tty…]> — 1 if the claude living on these ttys runs the ⚡1h-cache,
# else 0. Env binds at birth, so /proc of the live process is the only truth; comm is
# "claude" or a bare version string, depending on the install. The harness defaults to
# 1h since CC 2.1.215, so only an explicit FORCE_PROMPT_CACHING_5M=1 birth reads as 5m —
# a flagless elder (born before the force-5m rewire) runs 1h and reports 1.
# READABLE ON LINUX ONLY, AND THAT IS SAFE. cc_penv answers from /proc/<pid>/environ; macOS lets
# no process read another's environment (SIP), so a Mac finds nothing and 1 stands. That is also
# what a flagless chat reports, and since the harness now defaults to 1h, "unreadable" and
# "flagless" mean the same thing here — the ⚡ badge is wrong only for a chat deliberately born
# 5m, and it errs by NOT promising a cheaper window than the chat actually has.
_cc_c1h_tty() {
  local t="${1:-}" cp
  [[ -n "$t" ]] || { echo 1; return }
  for cp in ${(f)"$(ps -o pid=,comm= -t "$t" 2>/dev/null | awk '$2=="claude" || $2 ~ /^[0-9]+\./ {print $1}')"}; do
    [[ "$(cc_penv "${cp// /}" FORCE_PROMPT_CACHING_5M)" == 1 ]] && { echo 0; return }
  done
  echo 1
}

# ── Codex (cx-*) first-class plumbing ─────────────────────────────────────
# A Codex chat's identity is its USER-thread rollout file (~/.codex/sessions/…/rollout-…-<id>.jsonl).
# No breadcrumbs, no statusline: for a LIVE chat /proc is the map (the codex process holds the
# rollout open); for a resumable one the store is walked directly. Naming is codex-native — the
# thread_name in ~/.codex/session_index.jsonl, the same name `codex resume <name>` accepts.

# _cx_scan — ONE pass over every codex process: map each binary's user-thread rollout onto the
# pid AND its ancestors (≤4 levels, binary → node wrapper → shell → pane) in CXRL, so a pane_pid
# indexes its chat's rollout without per-socket /proc sweeps (three pgreps × a dozen sockets on
# a busy box cost whole seconds). Callers re-scan per listing run — the map ignores later births.
_cx_scan() {
  typeset -gA CXRL; CXRL=()
  local p pp rl l st rest i
  for p in ${(f)"$(pgrep -x codex 2>/dev/null)"}; do
    rl=""
    # fd order — the binary opens its OWN rollout first. cc_pfiles reads /proc/<pid>/fd on Linux
    # and falls back to lsof on macOS, which lists fds in the same ascending order; the glob this
    # replaced existed only under /proc, so a Mac saw no live codex chat at all.
    for l in ${(f)"$(cc_pfiles $p)"}; do
      [[ "$l" == "$HOME/.codex/sessions/"*/rollout-*.jsonl ]] || continue
      head -1 "$l" 2>/dev/null | grep -q '"thread_source":"user"' || continue
      rl="$l"; break
    done
    [[ -n "$rl" ]] || continue
    pp="$p"
    for i in 1 2 3 4; do
      [[ -z "${CXRL[$pp]:-}" ]] && CXRL[$pp]="$rl"
      # cc_ppid, not /proc/<pid>/stat: comm can contain spaces and parentheses, so that field is
      # only safe to read after the last ")" — and macOS has no /proc to read at all.
      pp="$(cc_ppid $pp)"
      [[ "$pp" == <-> && "$pp" != [01] ]] || break
    done
  done
}

# _cx_rollout_by_cwd <dir> [start-epoch] — the USER-thread rollout of the codex running in <dir>.
# THE /proc MAP IS NO LONGER ENOUGH. Codex (v0.146) appends to its rollout and CLOSES it, so the
# fd scan _cx_scan does finds nothing for a live chat: every ⬢ row fell back to the birth name
# "Codex chat" with 0 prompts, the window name never converged to the thread name, and the VS Code
# tab title (⬢ #{window_name} · …) showed that same placeholder. What codex still guarantees is
# that a chat runs in one directory and writes that directory into its rollout's meta line, so the
# pane's cwd identifies the thread with no fd and no env. With <start-epoch> (the pane's own start),
# the match whose meta timestamp lies CLOSEST to it wins — codex writes the rollout within seconds
# of starting, and this is what tells two chats in one directory apart (newest-wins labeled them
# both with the newer thread). No start, or nothing born near it → newest match, as before.
# Twin: cx_rollout_for in cc-name-sync.sh — change one, change both.
_cx_rollout_by_cwd() {
  local dir="$1" start="${2:-}" f meta ts birth d best="" bestd=999999999 newest=""
  [[ -n "$dir" ]] || return 1
  for f in ${(f)"$(cc_find_newest "$HOME/.codex/sessions" -name 'rollout-*.jsonl' | head -60)"}; do
    meta="$(head -1 "$f" 2>/dev/null)" || continue
    [[ "$meta" == *'"thread_source":"user"'* ]] || continue
    [[ "$meta" == *"\"cwd\":\"$dir\""* ]] || continue
    [[ -n "$start" ]] || { print -r -- "$f"; return 0 }
    [[ -n "$newest" ]] || newest="$f"
    ts="${${meta#*\"timestamp\":\"}%%\"*}"
    birth="$(cc_epoch "$ts")"
    [[ -n "$birth" ]] || continue
    (( birth >= start - 120 )) || continue
    d=$(( birth - start )); (( d < 0 )) && d=$(( -d ))
    (( d < bestd )) && { bestd=$d; best="$f"; }
  done
  [[ -n "$best" ]] || best="$newest"
  [[ -n "$best" ]] && { print -r -- "$best"; return 0 }
  return 1
}

# _cx_rollout <pane_pid> — the user-thread rollout of the codex living under this pane, read
# from the CXRL map (callers run _cx_scan first); empty + rc 1 when the pane hosts no codex.
_cx_rollout() {
  local rl="${CXRL[$1]:-}"
  [[ -n "$rl" ]] || return 1
  print -r -- "$rl"
}

# _cx_index — load session_index.jsonl into CXNM (thread id → thread_name), re-read only when
# the file changes; a rename lands on the next picker run.
_cx_index() {
  local idx="$HOME/.codex/session_index.jsonl" m i n
  m="$(cc_mtime "$idx")"; [[ -n "$m" ]] || { typeset -gA CXNM; CXNM=(); return 0 }
  [[ "${_CXNM_MT:-}" == "$m" ]] && return 0
  typeset -gA CXNM; CXNM=(); typeset -g _CXNM_MT="$m"
  while IFS=$'\t' read -r i n; do [[ -n "$i" ]] && CXNM[$i]="$n"; done \
    < <(jq -r '[.id, .thread_name // ""] | @tsv' "$idx" 2>/dev/null)
  return 0
}

# _cx_name <rollout-path> — display name: the index thread_name for any id in the thread's
# lineage (own id, session_id, parent_thread_id — a resumed thread carries a fresh id but the
# name sticks to an ancestor), else the FIRST real user prompt, else "".
_cx_name() {
  local rl="$1" meta i nm
  _cx_index
  local -a idl=("${${rl:t:r}#rollout-????-??-??T??-??-??-}")
  meta="$(head -1 "$rl" 2>/dev/null)"
  for i in session_id parent_thread_id; do
    idl+=(${(f)"$(print -r -- "$meta" | grep -o "\"$i\":\"[0-9a-f-]*\"" | cut -d'"' -f4)"})
  done
  for i in "${idl[@]}"; do
    [[ -n "$i" && -n "${CXNM[$i]:-}" ]] && { print -r -- "${CXNM[$i]}"; return 0 }
  done
  nm="$(grep -m1 '"type":"user_message"' "$rl" 2>/dev/null | jq -r '.payload.message // empty' 2>/dev/null)"
  print -r -- "${${nm//[$'\t\n']/ }[1,60]}"
}

# _cx_newprompts <rollout> <fromsize> — count user_message events in the bytes appended past
# <fromsize>; the codex side of the auto-unhide delta scan (codex event lines need no jq filter).
_cx_newprompts() {
  tail -c +$(($2+1)) "$1" 2>/dev/null \
    | awk 'NR==1 && substr($0,1,1)!="{" {next} {print}' \
    | grep -c '"type":"user_message"'
}

# _cx_metac <id> <rollout> <mtime> <size> — cached codex meta; REPLY="cwd\tcount" (count = real
# user_message events), cwd="§sub§" marks a subagent worker thread (not a chat — callers skip).
# Shares the NC cache in the claude 5-field shape (mt\tsz\tcwd\tname\tcount, name unused) so one
# persist path serves both engines; grown rollouts count only their appended bytes.
_cx_metac() {
  local id="$1" rl="$2" mt="$3" sz="$4" e emt esz rest cwd ct dct meta
  e="${NC[$id]}"
  if [[ -n "$e" ]]; then
    emt="${e%%$'\t'*}"; rest="${e#*$'\t'}"; esz="${rest%%$'\t'*}"; rest="${rest#*$'\t'}"
    cwd="${rest%%$'\t'*}"; ct="${rest##*$'\t'}"
    if (( mt == emt && sz == esz )); then REPLY="$cwd"$'\t'"$ct"; return; fi   # unchanged-only, like _cc_metac
    if (( esz > 0 && sz >= esz )); then
      dct="$(tail -c +$((esz+1)) "$rl" 2>/dev/null | grep -c '"type":"user_message"')"
      ct=$(( ${ct:-0} + dct ))
      NC[$id]="$mt"$'\t'"$sz"$'\t'"$cwd"$'\t\t'"$ct"
      REPLY="$cwd"$'\t'"$ct"; return
    fi
  fi
  meta="$(head -1 "$rl" 2>/dev/null)"                     # cold or rewritten → classify + one full pass
  if print -r -- "$meta" | grep -q '"thread_source":"user"'; then
    cwd="$(print -r -- "$meta" | grep -o '"cwd":"[^"]*"' | head -1 | cut -d'"' -f4)"
    ct="$(grep -c '"type":"user_message"' "$rl" 2>/dev/null)"
  else
    cwd="§sub§"; ct=0
  fi
  NC[$id]="$mt"$'\t'"$sz"$'\t'"$cwd"$'\t\t'"$ct"
  REPLY="$cwd"$'\t'"$ct"
}

# _cc_open_gate <acct> <live-c1h> <want-c1h> <name> — before attaching a LIVE chat whose birth
# env disagrees with the picker (account ≠ primary, or ⚡1h ≠ the ⌃E choice), say so and offer
# the in-place reboot — env binds at birth, attach can't change it. Interactive terminals
# only; returns the decision via $REPLY (s = reboot to match the picker, then attach).
_cc_open_gate() {
  local acct="$1" lc1h="$2" wc1h="$3" name="$4" pm; REPLY=""
  [[ -t 0 ]] || return 0
  pm="$(_cc_primary)"
  # collect ONLY the dimensions that actually disagree, as "<label>\t<born>\t<picker-wants>"
  local -a rows=()
  # GPT↔Claude is not a swap this fleet offers: the transcript belongs to one engine, and
  # rebooting it under the other replays a foreign conversation to the wrong provider. An
  # account-4 chat opened while 1-3 is primary (or the reverse) simply reopens as itself.
  local xeng=0
  [[ ( "$acct" == 4 && "$pm" != 4 ) || ( "$acct" != 4 && "$acct" != "" && "$pm" == 4 ) ]] && xeng=1
  [[ -n "$acct" && "$acct" != "$pm" ]] && (( ! xeng )) && rows+=("account"$'\t'"$(_cc_medal $acct) $acct"$'\t'"$(_cc_medal $pm) $pm")
  if [[ -n "$lc1h" && "$lc1h" != "$wc1h" ]]; then
    local lcs wcs
    [[ "$lc1h" == 1 ]] && lcs="⚡ 1h" || lcs="🪫 5m"
    [[ "$wc1h" == 1 ]] && wcs="⚡ 1h" || wcs="🪫 5m"
    rows+=("cache"$'\t'"$lcs"$'\t'"$wcs")
  fi
  (( ${#rows} )) || return 0

  # a rounded cyan callout that echoes the cc-ls picker's border/label palette. Each mismatch
  # row is a self-aligning "born  →  picker wants" pair (every value leads with one 2-cell
  # glyph, so char-count padding lands the arrows in one column). No right edge = emoji width
  # never breaks the box.
  local X0=$'\e[0m' Xd=$'\e[2m' Xb=$'\e[1m' Xc=$'\e[36m' Xr=$'\e[31m' XY=$'\e[93m'
  local bar="${Xc}│${X0}" topr botr r lbl born want
  topr="$(printf '─%.0s' {1..30})"; botr="$(printf '─%.0s' {1..52})"
  print
  print -r -- "${Xc}╭─${X0} ${Xb}${XY}⚠${X0}  ${Xb}birth env ≠ picker${X0} ${Xc}${topr}${X0}"
  print -r -- "$bar"
  print -r -- "$bar   ${Xb}${name}${X0} ${Xd}is live${X0}"
  print -r -- "$bar"
  print -r -- "$bar   ${Xd}left = born now   ·   right = picker wants${X0}"
  for r in "${rows[@]}"; do
    lbl="${r%%$'\t'*}"; born="${r#*$'\t'}"; want="${born#*$'\t'}"; born="${born%%$'\t'*}"
    print -r -- "$bar   ${Xd}$(printf '%-9s' "$lbl")${X0}$(printf '%-7s' "$born") ${Xd}→${X0}   ${Xb}${want}${X0}"
  done
  print -r -- "$bar"
  print -r -- "$bar   ${Xd}env binds at birth — attach can't change it, only a reboot can.${X0}"
  print -r -- "$bar"
  print -r -- "$bar   ${Xb}${XY}⏎${X0}  attach as-is"
  print -r -- "$bar   ${Xb}${XY}s${X0}  reboot to match the picker, then attach   ${Xr}— in-flight work dies${X0}"
  print -r -- "${Xc}╰${botr}${X0}"
  local k=""
  read -sk 1 "k?  ${Xc}❯${X0} "
  print
  [[ "$k" == [sS] ]] && REPLY=s
  return 0
}

# _cc_cachew <uuid> <fallback-epoch> [ttl] — the prompt-cache window this chat's NEXT turn lands
# in: the picker's twin of the statusline's 💾 segment, and the last column of every row.
#   ✓12m  green — attach or resume now and the turn still hits the cache
#   ✗3h   red   — the window closed; the next turn re-reads the whole context at full freight
#   12m   dim   — no anchor to read: a plain AGE, never a cache claim (Codex rows, background
#                 agents, chats whose statusline never cached an anchor). Our blind spot renders
#                 as what we do know, because ✗ is a claim about the chat and this is not one.
# The anchor is the statusline's own /tmp/cc-sl-anchor-<uuid> — the newest main-chain `user`
# record, i.e. the request that armed the window — read as a file, so a row costs no scan and
# inherits the statusline's correctness rules. A transcript's MTIME is NOT an anchor: CC rewrites
# transcripts on a flush timer with no request behind them, which would re-arm a dormant chat's
# window on a free write. A stale anchor only ever under-states warmth (its own chat's statusline
# refreshes it every few seconds), so ✓ is never a false promise — wrong in the safe direction.
# Sets REPLY (plain text, so the caller's ${(r:N:)} padding stays true) and REPLY_C (its colour).
_cc_cachew() {
  local u="${1:-}" mt="${2:-0}" ttl="${3:-3600}" a="" d g=""
  local now=${EPOCHSECONDS:-0}; (( now == 0 )) && now=$(date +%s)   # zsh/datetime, never a `date` fork — this runs once per row
  REPLY_C=$'\e[2m'                                   # dim = age, no verdict
  [[ -n "$u" && -r "/tmp/cc-sl-anchor-$u" ]] && { a="$(<"/tmp/cc-sl-anchor-$u")"; a="${a##* }"; }
  if [[ -z "$a" || "$a" == - ]]; then                # "-" = the statusline scanned and found nothing
    d=$(( now - mt ))
  else
    d=$(( ttl - ( now - a ) ))
    if (( d > 0 )); then g="✓"; REPLY_C=$'\e[32m'
    else                g="✗"; REPLY_C=$'\e[31m'; d=$(( -d )); fi
  fi
  (( d < 0 )) && d=0
  if   (( d < 60 ));    then REPLY="${g}${d}s"
  elif (( d < 3600 ));  then REPLY="${g}$(( d / 60 ))m"
  elif (( d < 86400 )); then REPLY="${g}$(( d / 3600 ))h"
  else                       REPLY="${g}$(( d / 86400 ))d"
  fi
}

# _cc_hsize <bytes> — human size: 0B / 12K / 1.2M / 3.4G
_cc_hsize() {
  local b=${1:-0}
  if   (( b >= 1073741824 )); then printf '%d.%dG' $(( b / 1073741824 )) $(( b % 1073741824 * 10 / 1073741824 ))
  elif (( b >= 1048576 ));    then printf '%d.%dM' $(( b / 1048576 ))    $(( b % 1048576 * 10 / 1048576 ))
  elif (( b >= 1024 ));       then printf '%dK' $(( b / 1024 ))
  else                             printf '%dB' "$b"
  fi
}

# injected pseudo-prompts CC stores as "user" messages — skip these when naming a chat
_CC_JUNK='^(<[a-z]|Caveat:|\[Request)'   # injected blocks (<system-reminder>, <task-notification>, <bash-input>, …)

# user-turn text extractor shared by _cc_meta/_cc_lastprompt: string content as-is; array content
# joins the TOP-LEVEL text blocks (a slash command's expansion turn, a pasted-image prompt's text) —
# tool_result blocks are not text blocks, so tool returns never count. Junk filter applies after.
# Compact summaries are user-typed records but not prompts — counting them let an auto-compaction
# resurrect a hidden chat, and naming from them titled chats "This session is being continued…".
_CC_TEXTQ='select(.type=="user" and (.isCompactSummary != true)) | (.message.content) as $c
  | (if ($c|type)=="string" then $c
     elif ($c|type)=="array" then ([$c[]? | select(.type=="text") | .text] | join(" "))
     else "" end) as $t
  | select($t != "" and ($t|test($j)|not))'

# _cc_lastprompt <transcript> — most recent REAL human prompt, flattened to one line
_cc_lastprompt() {
  [[ -r "$1" ]] || return
  tac "$1" 2>/dev/null | jq -rc --arg j "$_CC_JUNK" "$_CC_TEXTQ"' | ($t|gsub("[\n\t]+";" "))' 2>/dev/null | head -1
}

# _cc_newprompts <transcript> <fromsize> — count REAL prompts in the bytes appended past
# <fromsize> (the auto-unhide trigger: scan the delta, never the whole file). The first delta
# line may be a record cut mid-append — dropped unless it starts one, like _cc_metac's delta.
_cc_newprompts() {
  tail -c +$(($2+1)) "$1" 2>/dev/null \
    | awk 'NR==1 && substr($0,1,1)!="{" {next} {print}' \
    | jq -rc --arg j "$_CC_JUNK" "$_CC_TEXTQ"' | 1' 2>/dev/null | grep -c .
}

# _cc_isbg <transcript> — true when this is a background SESSION (its records carry
# sessionKind:"bg"), the harness's async twin of a real chat rather than a chat itself. Only the
# head is read: the marker repeats on every record, and reading a fixed 64K keeps the test O(1)
# on a 200MB transcript. A normal chat carries no sessionKind at all, so a miss simply leaves the
# row visible — the safe direction to fail.
# sysread gives a BOUNDED in-process read; `head -c 65536 | grep` cost two processes per row
# (11.2 ms measured) and was the single biggest contributor to cc-ls's warm time.
_cc_isbg() {
  local fd buf
  if zmodload -e zsh/system; then
    exec {fd}<"$1" 2>/dev/null || return 1
    sysread -s 65536 -i $fd buf 2>/dev/null
    exec {fd}<&- 2>/dev/null
    [[ "$buf" == *'"sessionKind":"bg"'* ]]
  else
    head -c 65536 "$1" 2>/dev/null | grep -qm1 '"sessionKind":"bg"'
  fi
}

# _cc_stat <file> — sets REPLY to "<size> <mtime>". zstat is in-process; `stat -c` was a fork
# per row. Sets REPLY rather than printing so callers need no command substitution either.
_cc_stat() {
  local -A _h
  if zmodload -e zsh/stat && zstat -H _h -- "$1" 2>/dev/null; then
    REPLY="${_h[size]} ${_h[mtime]}"; return 0
  fi
  REPLY="$(cc_size "$1") $(cc_mtime "$1")" && [[ "$REPLY" != " " ]]
}

# _cc_meta <transcript> — "cwd<TAB>first-real-prompt<TAB>prompt-count" in one jq pass (whole file).
# prompt-count = real human turns (same junk filter as naming). cc-ls caches the result by mtime.
_cc_meta() {
  [[ -r "$1" ]] || { print -r -- $'\t\t0'; return; }
  local out title firstline
  out="$(jq -rc --arg j "$_CC_JUNK" "$_CC_TEXTQ"' | [(.cwd//""),($t|gsub("[\n\t]+";" "))]|@tsv' "$1" 2>/dev/null)"
  # /rename writes {"type":"custom-title",…} + {"type":"agent-name",…}; the LAST one is the chat's name and wins over the first prompt
  title="$(grep -aE '"type":"(custom-title|agent-name)"' "$1" 2>/dev/null | tail -1 | jq -r '.customTitle // .agentName // empty' 2>/dev/null)"
  # Never /rename'd? The harness titles the chat itself ({"type":"ai-title","aiTitle":…}) — a far
  # better row label than the raw first prompt (which read like "This box is too small for anything
  # we're doing…" on a chat actually titled "Migrate application to larger server"). STRICTLY second:
  # a renamed chat keeps writing ai-title records AFTER its custom-title ones, so folding both into
  # one last-record-wins grep would relabel every /rename'd chat with its AI title.
  [[ -z "$title" ]] && title="$(grep -a '"type":"ai-title"' "$1" 2>/dev/null | tail -1 | jq -r '.aiTitle // empty' 2>/dev/null)"
  title="${title//[$'\t\n']/ }"   # titles are user text — a raw tab/newline would corrupt the TSV cache record
  [[ -z "$out" ]] && { print -r -- $'\t'"$title"$'\t0'; return; }   # command-only/cleared chat — a /rename title still names it
  firstline="${out%%$'\n'*}"                                         # cwd<TAB>first-prompt of the first real turn
  print -r -- "${firstline%%$'\t'*}"$'\t'"${title:-${firstline#*$'\t'}}"$'\t'"$(print -r -- "$out" | grep -c .)"
}

# _cc_metac <uuid> <tpath> <mtime> <size> — cached chat meta; sets REPLY="cwd\tname\tcount".
# Cache value: mtime\tsize\tcwd\tname\tcount. An UNCHANGED file (same mtime+size) returns the
# cache as-is; a GROWN transcript parses only its APPENDED bytes (count += new real turns; a
# fresh /rename title wins; a first prompt arriving names a promptless chat) — an active
# multi-MB chat otherwise re-parsed wholesale every couple of minutes, which is what made
# cc-ls crawl. Unchanged-only, NOT a staleness window: the old `mt-emt<120` compared file
# mtimes, so two writes <120s apart made the cache permanently stale — auto-unhide counts
# ride on this, and a resurrection prompt landing <2min after the last noise write was
# swallowed forever.
_cc_metac() {
  local u="$1" tp="$2" mt="$3" sz="$4" e emt esz rest
  e="${NC[$u]}"
  if [[ -n "$e" ]]; then
    emt="${e%%$'\t'*}"; rest="${e#*$'\t'}"
    esz="${rest%%$'\t'*}"; rest="${rest#*$'\t'}"
    if (( mt == emt && sz == esz )); then REPLY="$rest"; return; fi
    if (( esz > 0 && sz >= esz )); then
      local dseg dtexts dttl cwd0 mid ct0 dct
      # first delta line may be a partial record cut mid-append — drop it unless it starts a record
      dseg="$(tail -c +$((esz+1)) "$tp" 2>/dev/null | awk 'NR==1 && substr($0,1,1)!="{" {next} {print}')"
      dtexts="$(print -r -- "$dseg" | jq -rc --arg j "$_CC_JUNK" "$_CC_TEXTQ"' | ($t|gsub("[\n\t]+";" "))' 2>/dev/null)"
      dct="$(print -r -- "$dtexts" | grep -c .)"
      dttl="$(print -r -- "$dseg" | grep -aE '"type":"(custom-title|agent-name)"' 2>/dev/null | tail -1 | jq -r '.customTitle // .agentName // empty' 2>/dev/null)"
      # same precedence as _cc_meta: an auto ai-title only when the delta carries no manual rename.
      # A /rename'd chat rewrites its manual title on essentially every turn, so any real delta from
      # one carries it — the auto title can only win for a chat that was never named by hand.
      [[ -z "$dttl" ]] && dttl="$(print -r -- "$dseg" | grep -a '"type":"ai-title"' 2>/dev/null | tail -1 | jq -r '.aiTitle // empty' 2>/dev/null)"
      cwd0="${rest%%$'\t'*}"; mid="${${rest#*$'\t'}%%$'\t'*}"; ct0="${rest##*$'\t'}"
      [[ -z "$mid" && -n "$dtexts" ]] && mid="${${dtexts%%$'\n'*}//$'\t'/ }"
      [[ -n "$dttl" ]] && mid="${dttl//[$'\t\n']/ }"
      REPLY="$cwd0"$'\t'"$mid"$'\t'"$(( ${ct0:-0} + dct ))"
      NC[$u]="$mt"$'\t'"$sz"$'\t'"$REPLY"
      return
    fi
  fi
  REPLY="$(_cc_meta "$tp")"                # cold or shrunk (rewritten) → one full parse
  NC[$u]="$mt"$'\t'"$sz"$'\t'"$REPLY"
}

# _cc_agents — map every LIVE claude session (uuid → owning config dir) into the global CCAGENT.
# A running session can't be plain `--resume`d (it's locked); cc-ls uses this to route "open" to
# the `claude agents` attach view instead. The session id is read from argv: --session-id for
# background/forked agents, else --resume for a plain interactive; the account from /proc/<pid>/environ.
_cc_agents() {
  typeset -gA CCAGENT; CCAGENT=()
  local pid rest a0 sid cfgdir e
  local -a sids
  while read -r pid rest; do                       # ps left-pads pid → read trims it, keeps argv intact
    a0="${rest%% *}"                               # argv[0] must be the claude binary, not a shell/tool that merely mentions a uuid
    [[ "${a0:t}" == claude || "$a0" == */claude/versions/* ]] || continue
    # register BOTH ids a process can hold the lock under: its --session-id AND the transcript it
    # --resume'd (a forked bg agent locks the RESUMED file too — that's the uuid cc-ls knows it by)
    sids=()
    [[ "$rest" == *"--session-id "* ]] && { sid="${rest#*--session-id }"; sids+=("${${sid%% *}:t:r}"); }
    [[ "$rest" == *"--resume "* ]]     && { sid="${rest#*--resume }";     sids+=("${${sid%% *}:t:r}"); }
    (( ${#sids} )) || continue
    cfgdir=""
    for sid in "${sids[@]}"; do
      [[ "$sid" =~ '^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$' ]] || continue   # strict uuid (hex only) — a crafted argv must not pass as a session id
      [[ -n "${CCAGENT[$sid]}" ]] && continue      # dedup: the agent + its pty-host share ids
      # WHICH ACCOUNT OWNS THIS AGENT — from the per-session breadcrumb, not the process env.
      # Every launch drops $CLAUDE_CONFIG_DIR/session-env/<session-id>, so the account that owns
      # a session is the one holding that file: an on-disk fact, readable on any platform, and
      # true for a chat whose process this scan cannot even see. The env read it replaced worked
      # only through /proc — on macOS it always came back empty and EVERY agent silently
      # defaulted to account 1, which sends a takeover to the wrong subscription. The env is
      # still consulted first where it can be read, since it is the account the process is
      # actually running under; the breadcrumb is the portable second opinion, and ~/.claude
      # remains the floor when neither answers.
      if [[ -z "$cfgdir" ]]; then
        cfgdir="$(cc_penv $pid CLAUDE_CONFIG_DIR)"
        if [[ -z "$cfgdir" ]]; then
          for e in "$HOME"/.cc/[0-9]* "$HOME/.claude"; do
            [[ -e "$e/session-env/$sid" ]] && { cfgdir="$e"; break; }
          done
        fi
        [[ -n "$cfgdir" ]] || cfgdir="$HOME/.claude"
      fi
      CCAGENT[$sid]="$cfgdir"
    done
  done < <(ps -o pid=,args= -U "$(id -u)" 2>/dev/null)
}

# _cc_solo <uuid> [keep-socket] — ONE tmux instance per chat, enforced at launch time.
# A chat opened twice = two claudes on one transcript: window-size wars wreck the display and
# both processes write the same file. Kill every OTHER live tmux host of <uuid> before opening:
# a pane-keyed breadcrumb kills just its pane (split siblings survive); a socket-level crumb
# kills its server only when it's provably alone (single pane, no pane crumbs — a split could
# hide an innocent sibling). Stray claudes whose argv names the uuid are TERMed too — unless
# the uuid is a live daemon agent, which cc-agent-open.sh takes over gracefully instead.
_cc_solo() {
  local u="$1" keep="${2:-}" siddir=/tmp/cc-sid
  local bc f sock pane panes p a0 w
  [[ -n "$u" ]] || return 0
  for bc in "$siddir"/cc-*(N); do
    [[ -r "$bc" && "$(<"$bc")" == *"/$u.jsonl" ]] || continue
    f="${bc:t}"
    if [[ "$f" == *.%* ]]; then sock="${f%.\%*}"; pane="%${f##*.\%}"; else sock="$f"; pane=""; fi
    [[ -n "$keep" && "$sock" == "$keep" ]] && continue
    panes="$(tmux -L "$sock" list-panes -a -F '#{pane_id}' 2>/dev/null)"
    [[ -z "$panes" ]] && { rm -f "$bc"; continue; }               # dead server → stale crumb
    if [[ -n "$pane" ]]; then
      if [[ $'\n'"$panes"$'\n' == *$'\n'"$pane"$'\n'* ]]; then
        echo "cc: solo — closing duplicate of this chat on $sock $pane"
        tmux -L "$sock" kill-pane -t "$pane" 2>/dev/null
      fi
      rm -f "$bc"
    else
      local -a pcr=("$siddir/$sock".%*(N))
      if (( ${#pcr} == 0 )) && [[ "$panes" != *$'\n'* ]]; then
        echo "cc: solo — closing duplicate of this chat on $sock"
        tmux -L "$sock" kill-server 2>/dev/null
      fi
      rm -f "$bc"
    fi
  done
  [[ -n "${CCAGENT[$u]:-}" ]] && return 0   # live daemon agent — never TERM here; the router handles it
  # keep-socket EXEMPTION for the stray sweep. A chat opened via `claude --resume <uuid>`
  # carries that uuid in its argv for life, so this pgrep sweep would TERM the very chat
  # cc-open is attaching (CCAGENT is empty on a fresh direct-open shell, so the guard above
  # doesn't cover it). Never kill a pid whose controlling tty belongs to the keep socket —
  # that pid IS the chat we're keeping. ps -o tty= prints e.g. "pts/5"; pane_tty is
  # "/dev/pts/5", so strip the /dev/ prefix to compare.
  local -a keepttys=()
  [[ -n "$keep" ]] && keepttys=(${${(f)"$(tmux -L "$keep" list-panes -a -F '#{pane_tty}' 2>/dev/null)"}#/dev/})
  local ptty
  for p in $(pgrep -f -- "$u" 2>/dev/null); do
    a0="$(ps -o args= -p "$p" 2>/dev/null)"; [[ -n "$a0" ]] || continue
    [[ "$a0" == *--bg-pty-host* ]] && continue
    w="${${(z)a0}[1]}"
    [[ "${w:t}" == claude || "$w" == */claude/versions/* ]] || continue
    if (( ${#keepttys} )); then
      ptty="$(ps -o tty= -p "$p" 2>/dev/null | tr -d ' ')"
      [[ -n "$ptty" ]] && (( ${keepttys[(Ie)$ptty]} )) && continue   # on the keep socket — never TERM the chat we're attaching
    fi
    kill -TERM "$p" 2>/dev/null && echo "cc: solo — killed stray claude $p holding $u"
  done
  return 0
}

# _cc_open_lock <uuid> — best-effort serialize of cc-open's check-then-LAUNCH (resume / agent
# takeover) so two simultaneous opens of one uuid don't both clear _cc_solo and both
# `--resume` it (two claudes, one transcript — the corruption _cc_solo guards). Atomic mkdir
# with stale-steal: a dead owner or a hold >8s is reclaimed (an opener that exec'd into the
# chat leaves the dir, so staleness frees it). Waits at most ~2s — long enough for the first
# opener to establish its server, after which _cc_solo/the harness see the live session. Only
# the launch paths take it; the attach path (a live server already exists) never does, so a
# normal re-attach is never delayed.
_cc_open_lock() {
  local ld="/tmp/cc-sid/.open.$1" line opid ots now i=0
  mkdir -p /tmp/cc-sid 2>/dev/null
  while (( i < 20 )); do
    if mkdir "$ld" 2>/dev/null; then print -r -- "$$ $(date +%s)" >| "$ld/owner" 2>/dev/null; return 0; fi
    line="$(cat "$ld/owner" 2>/dev/null)"; opid="${line%% *}"; ots="${line##* }"; now=$(date +%s)   # cat, not $(<): zsh's $(<) prints an error for a vanished owner file
    if { [[ -n "$opid" ]] && ! kill -0 "$opid" 2>/dev/null; } || { [[ "$ots" == <-> ]] && (( now - ots > 8 )); }; then
      rm -rf "$ld" 2>/dev/null; continue
    fi
    sleep 0.1; (( i++ ))
  done
  return 0   # never block an open — best effort
}

# cc-open <transcript-uuid> — open one chat DIRECTLY, no picker: attach its live tmux, route a
# live agent through cc-agent-open.sh, else resume in a fresh cc-* server — the same three paths
# as a cc-ls Enter. `cc-ls <uuid>` forwards here; /swap's seamless re-entry types it for you.
cc-open() {
  local u="$1"                             # strict uuid: 8-4-4-4-12 hex ONLY — this value flows into
  [[ "$u" =~ '^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$' ]] \
    || { echo "cc-open: need a transcript uuid"; return 2; }   # shell-parsed run-strings; a loose *-*-*-*-* let metacharacters through
  local siddir=/tmp/cc-sid bc sock lc1h
  local wc1h; wc1h="$(_cc_arm1h)"   # ⚡ intent — explicit CC_ARM_1H or a non-chat caller's env (leaks never arm)
  for bc in "$siddir"/cc-*(N); do                       # 1) live tmux host → attach
    [[ -r "$bc" && "$(<"$bc")" == *"/$u.jsonl" ]] || continue
    sock="${bc:t}"; sock="${sock%%.\%*}"
    tmux -L "$sock" list-panes -a >/dev/null 2>&1 || continue
    lc1h="$(_cc_c1h_tty "${(j:,:)${${(f)$(tmux -L "$sock" list-panes -a -F '#{pane_tty}' 2>/dev/null)}#/dev/}}")"
    _cc_open_gate "$(_cc_acct "$(<"$bc")")" "$lc1h" "$wc1h" "$u"   # birth env ≠ intent? offer the reboot
    [[ "$REPLY" == s ]] && bash "$CC_FLEET_HOME/cc-swap-chat.sh" --sock "$sock" "$(_cc_primary)" --1h "$wc1h"
    _cc_solo "$u" "$sock"
    tmux -L "$sock" set -g window-size latest 2>/dev/null
    _cc_selfswitch "$sock" && return 0                   # already inside it → switch, never nest
    _cc_in_bunker && TMUX= exec tmux -L "$sock" attach   # viewport dies with the tab
    TMUX= tmux -L "$sock" attach; return $?
  done
  _cc_open_lock "$u"   # serialize check+launch for the resume/takeover paths below (TOCTOU)
  typeset -gA CCAGENT; (( ${#CCAGENT} )) || _cc_agents
  # c1henv sanitizes run-string launches: strip the inherited flags, then pick the mode
  # explicitly — armed re-adds ENABLE, un-armed must FORCE 5m (1h is the harness default)
  local c1henv="env -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M "
  if [[ "$wc1h" == 1 ]]; then c1henv+="ENABLE_PROMPT_CACHING_1H=1 "; else c1henv+="FORCE_PROMPT_CACHING_5M=1 "; fi
  local tpf rcwd
  tpf="$(ls "$HOME"/.claude/projects/*/"$u".jsonl 2>/dev/null | head -1)"
  rcwd="$(head -40 "$tpf" 2>/dev/null | jq -r 'select(.cwd) | .cwd' 2>/dev/null | head -1)"; [[ -d "$rcwd" ]] || rcwd="$PWD"
  if [[ -n "${CCAGENT[$u]:-}" ]]; then                  # 2) live agent → zero-question router
    local as="cc-$(date +%s)-$$-$RANDOM" ocfg="${CCAGENT[$u]}"; [[ "$ocfg" == "$HOME/.claude" ]] && ocfg=""
    _cc_solo "$u"
    _cc_in_bunker && TMUX= exec tmux -L "$as" new-session -s "$as" -c "$rcwd" \
      "${c1henv}bash ${(q)CC_FLEET_HOME}/cc-agent-open.sh $u ${(q)rcwd} ${(q)ocfg}"
    TMUX= tmux -L "$as" new-session -s "$as" -c "$rcwd" \
      "${c1henv}bash ${(q)CC_FLEET_HOME}/cc-agent-open.sh $u ${(q)rcwd} ${(q)ocfg}"
    return $?
  fi
  [[ -n "$tpf" ]] || { echo "cc-open: no transcript for $u"; return 1; }
  local cfg="" rs="cc-$(date +%s)-$$-$RANDOM"           # 3) resume (failure-net → router)
  local ra; ra="$(_cc_resume_acct "$u")"   # GPT chats resume on 4, Claude chats never do
  case "$ra" in 2|3|4) cfg="$HOME/.cc/$ra" ;; esac
  local rpfx="env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M ${CC4_UNSET} "   # never inherit a host chat's identity or cache mode
  if [[ -n "$cfg" ]]; then rpfx+="CLAUDE_CONFIG_DIR=${(q)cfg} "; else rpfx+="-u CLAUDE_CONFIG_DIR "; fi
  if [[ "$wc1h" == 1 ]]; then rpfx+="ENABLE_PROMPT_CACHING_1H=1 "; else rpfx+="FORCE_PROMPT_CACHING_5M=1 "; fi   # env ARGUMENT after its own -u (a shell prefix would be re-unset)
  [[ "$ra" == 4 ]] && rpfx+="${(j: :)${(@q)CC4_ENV}} "   # resume under account 4 → answer from GPT, not a dud Anthropic token
  local rflags=" ${(j: :)${(@q)CC_AUTONOMY_FLAGS}}"   # a resumed chat keeps full autonomy, every account
  _cc_solo "$u"
  _cc_in_bunker && TMUX= exec tmux -L "$rs" new-session -s "$rs" -c "$rcwd" \
    "${rpfx}claude --resume ${(q)u}${rflags} || { echo; echo 'resume refused — session is live elsewhere:'; ${c1henv}bash ${(q)CC_FLEET_HOME}/cc-agent-open.sh ${(q)u} ${(q)rcwd}; }"
  TMUX= tmux -L "$rs" new-session -s "$rs" -c "$rcwd" \
    "${rpfx}claude --resume ${(q)u}${rflags} || { echo; echo 'resume refused — session is live elsewhere:'; ${c1henv}bash ${(q)CC_FLEET_HOME}/cc-agent-open.sh ${(q)u} ${(q)rcwd}; }"
}

# cc-ls — Account 4 (GPT via claude-code-proxy) is deliberately NOT an entry point here:
# `cc4` still launches one and its chats still LIST like any other Claude chat (they ARE
# Claude Code chats — hiding them would orphan their transcripts), but the picker offers no
# ✦ row for starting one. The proxy stays installed and disabled: GPT is out of the daily
# workflow by choice, not removed.
# cc-ls — unified chat picker. Source 1: live tmux sessions, Claude (cc-*) only — Codex is
# not listed here at all (2026-07-29); `cx` still launches it
# alike (Enter → attach). Source 2: the Claude Code chat store (~/.claude/projects/*/*.jsonl) →
# every transcript NOT already live becomes a `cc --resume <uuid>` entry. Source 2b: the Codex
# store (~/.codex/sessions) → every user thread no live codex holds open becomes a
# `codex resume <id>` entry. Deduped by transcript (each tmux maps to one transcript; most
# transcripts have no tmux).  ● = live · ↻ = resumable.
# Columns: project │ name (/rename tag, or last/first prompt) │ prompts │ size │ cache window.
# Default: orphans hidden + recent resumables; -a shows ALL. ⌃T re-sorts recent⇄size. Enter acts · Esc.
cc-ls() {
  local dir="${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)" siddir=/tmp/cc-sid store="$HOME/.claude/projects"   # tmux appends /tmux-$UID to TMUX_TMPDIR
  local cursock="${${TMUX%%,*}:t}"
  local -a rows=()
  typeset -A live SEEN NC cxlive PROJDIR           # live: transcript-uuids already in tmux · NC: cwd/name/count cache · PROJDIR: project basename → a real cwd (for the ✦ new-chat-follows-⌃R-rotation target)
  PROJDIR[${PWD:t}]="$PWD"                     # seed the launch dir first so it wins for PWD's project basename
  local s sock name tag proj win att epoch tpath bytes hsize dispname label marks lp ttl agets acct prim lc1h
  local cxrl cxid cxnm cxct cxbytes cxmt cxst cxpp cxcwd1 cxst1                # live Codex row scratch
  prim="$(_cc_primary)"   # for the birth-account ≠ mark on live rows (border shows the live value)
  local cf="$siddir/.namecache.v5" cmeta rest cwd nm ct uuid u lmt stt   # bump vN if _cc_meta logic changes
  local -a ptp; local panes pbc pane_ mnm mu mst mmeta mmax mnewu ma      # split-window (⊞) merging
  local all=0 onlyhidden=0 hidden=0 strict=1   # strict = apply hide/orphan/cap filters (default only)
  # row palette (fzf --ansi): leading glyph = kind — ● live · ↻ resume · ⚙ agent · ✦ new
  local X0=$'\e[0m' Xd=$'\e[2m' Xb=$'\e[1m' Xc=$'\e[36m' Xg=$'\e[32m' Xy=$'\e[33m' Xm=$'\e[35m' Xu=$'\e[34m'
  local nt=""   # name tint — green marks an account-4 (GPT) row, the way magenta used to mark Codex
  case "$1" in -a|--all) all=1 ;; --hidden|-H) onlyhidden=1 ;; *-*-*-*-*) cc-open "$1"; return $? ;; esac   # uuid arg → direct open
  (( all || onlyhidden )) && strict=0
  # cache: uuid -> mtime\tsize\tcwd\tname\tcount  (split by param-expansion, so empty fields survive)
  # one-time migration from v4 (no size field): size=0 → actives take one full re-parse, done
  if [[ ! -r "$cf" && -r "$siddir/.namecache.v4" ]]; then
    awk -F'\t' -v OFS='\t' 'NF>=2 {print $1, $2, 0, $3, $4, $5}' "$siddir/.namecache.v4" > "$cf" 2>/dev/null
  fi
  # name cache now lives in the db, not /tmp/cc-sid/.namecache.v5 — a /tmp file was wiped on every
  # reboot, so the next cc-ls cold-parsed every transcript on the box. Same record shape either way.
  while IFS= read -r rest; do u="${rest%%$'\t'*}"; NC[$u]="${rest#*$'\t'}"; done < <(bash "$CC_DB" chat-load 2>/dev/null)
  # ⚡1h-cache ⌃E scratch lives at "$tmpd/1h" — keyed to THIS picker run (a fresh tmpd is
  # inherently OFF; concurrent pickers can no longer stomp each other's armed state)
  # hide list: transcript uuids dropped with ⌃X (reversible — file kept; cc-ls -a reveals, edit "$hf" to undo)
  local hf="$HOME/.claude/.cc-ls-hidden" h
  typeset -A HID; while IFS= read -r h; do [[ -n "$h" ]] && HID[$h]=1; done < <(bash "$CC_DB" hidden-list 2>/dev/null)
  # auto-unhide: a hidden chat that gains a NEW REAL PROMPT after hiding = someone is talking to
  # it again → comes back to the list. Byte growth alone NEVER unhides — a live chat's transcript
  # grows from pure noise (tool results, task notifications, its own autonomous grinding), and
  # byte-based unhide resurrected every hidden live chat within minutes. Instead, when a hidden
  # transcript grows past its size baseline, ONLY the appended bytes are scanned for a real
  # junk-filtered prompt (_cc_newprompts); a noise-only delta ratchets the baseline forward, so
  # each byte of a hidden chat is scanned at most once — never a full-file parse (this store is
  # multi-GB; full parses of hidden 70MB transcripts once wedged cc-ls for minutes).
  local af="$hf.at" auuid asize hideskip; typeset -A AT UNHIDE
  # read defensively: extra fields fold into $_ (an interim build shipped 3-field rows) and a
  # non-numeric size is re-baselined — a stray tab in a math value blows up zsh's (( )) parser
  while IFS=$'\t' read -r auuid asize _; do
    [[ -n "$auuid" && "$asize" == <-> ]] && AT[$auuid]="$asize"
  done < <(bash "$CC_DB" hidden-at-list 2>/dev/null)
  _cc_agents   # CCAGENT: uuid → config dir for LIVE agents (bg/forked). Resuming these fails; cc-ls attaches instead.
  _cx_scan     # CXRL: pane/ancestor pid → live codex user-thread rollout, one /proc pass for every cx row
  # chat window names (→ VS Code tabs) converge in the background; cc-name-sync.sh is their
  # single writer, also fired by its systemd path/timer units between picker runs
  [[ -x "$HOME/.claude/bin/cc-name-sync.sh" ]] && ( "$HOME/.claude/bin/cc-name-sync.sh" >/dev/null 2>&1 & )

  # ── source 1: live tmux sessions (attach) ──
  # PARALLEL probe, one call per socket: serial tmux round-trips (~25ms each) dominated cc-ls
  # latency. A single list-panes -a per server carries session fields + pane id + tty, so the
  # crumb sweep and the hygiene check need no second/third call.
  setopt localoptions no_notify no_monitor
  local tls pfile panelist ttylist
  local probed; probed="$(mktemp -d "${TMPDIR:-/tmp}/cc-probe.XXXXXX")" || probed="/tmp/cc-probe.$$"
  local -a psocks=()
  for s in "$dir"/*(N=); do                 # N=nullglob, ==sockets only
    sock="${s:t}"
    case "$sock" in vsct|revive*) continue ;; esac   # infra servers, never chats (bunkers + revive/revive-vsct dashboards)
    psocks+=("$s")
    tmux -L "$sock" list-panes -a -F $'#{session_name}\t#{s/^[^ ]* //:pane_title}\t#{b:pane_current_path}\t#{session_windows}\t#{?session_attached,1,0}\t#{session_created}\t#{pane_id}\t#{pane_tty}\t#{window_name}\t#{pane_pid}' > "$probed/$sock" 2>/dev/null &
  done
  wait
  for s in "${psocks[@]}"; do
    sock="${s:t}"; pfile="$probed/$sock"
    if [[ ! -s "$pfile" ]]; then
      # corpse socket file (server gone) — self-heal the graveyard on sight, cc-reap style.
      # 200 corpses once cost ~7s of dead probes per cc-ls. Age guard (>1h): never touch a
      # server mid-startup whose socket exists but isn't answering yet. Bare glob qualifiers
      # in filename generation, NOT [[ -n "$s"(#q...) ]] — (#q) needs extendedglob (off in
      # production shells), where that test is true for fresh and even nonexistent paths.
      local -a corpse=( ${s}(Nmh+1) )
      (( ${#corpse} )) && rm -f "$s" 2>/dev/null
      continue
    fi
    panelist="$(cut -f7 "$pfile")"                          # every pane id on this SERVER
    ttylist="${(j:,:)${${(f)$(cut -f8 "$pfile")}#/dev/}}"   # their ttys, for the hygiene ps
    tls="$(awk -F'\t' '!seen[$1]++' "$pfile" | cut -f1-6)"  # one line per SESSION (same six fields as before)
    while IFS= read -r rest; do             # tab-safe split — an empty pane_title must not collapse columns
      [[ -z "$rest" ]] && continue
      name="${rest%%$'\t'*}"; rest="${rest#*$'\t'}"
      tag="${rest%%$'\t'*}";  rest="${rest#*$'\t'}"
      proj="${rest%%$'\t'*}"; rest="${rest#*$'\t'}"
      win="${rest%%$'\t'*}";  rest="${rest#*$'\t'}"
      att="${rest%%$'\t'*}";  epoch="${rest#*$'\t'}"
      [[ -z "$name" ]] && continue          # dead/stale socket: server gone
      if [[ "$sock" == cx-* ]]; then        # ⬢ live Codex chat — identity = the user-thread rollout
        # A cx-* SOCKET is not the same thing as a cx-* CHAT. `_cx_server` names a codex chat's
        # session after its socket, and other work can be parked on that same server later — a
        # pair of long-running dev-server sessions once rode a live cx-* socket and rendered as
        # two phantom "⬢ Codex chat · 0p · 0B" rows. The session
        # name IS the discriminator, exactly as it is for cc-* chats: anything else on this socket
        # is a squatter, not a chat, and must not be listed (nor offered for Enter-to-attach).
        [[ "$name" == "$sock" ]] || continue
        # the codex process holds open (no crumbs/transcript exist); it must row HERE or it becomes
        # an invisible orphan server. Named by the codex thread_name, else its first real prompt.
        cxrl=""; cxid=""; cxnm=""; cxct=0; cxbytes=0; cxmt=""
        for cxpp in ${(f)"$(cut -f10 "$pfile")"}; do cxrl="$(_cx_rollout "$cxpp")" && break; done
        # fd map missed (current codex keeps no rollout open) → identify by the pane's own cwd,
        # disambiguated by the pane's start time (two chats can share a directory)
        if [[ -z "$cxrl" ]]; then
          cxcwd1="$(tmux -L "$sock" list-panes -t "=$name" -F '#{pane_current_path}' 2>/dev/null | head -1)"
          cxst1="$(cc_pstart "$(cut -f10 "$pfile" | head -1)")"
          [[ -n "$cxcwd1" ]] && cxrl="$(_cx_rollout_by_cwd "$cxcwd1" "$cxst1")"
        fi
        if [[ -n "$cxrl" && -r "$cxrl" ]]; then
          cxid="${${cxrl:t:r}#rollout-????-??-??T??-??-??-}"
          _cc_stat "$cxrl"; cxst="$REPLY"; cxbytes="${cxst%% *}"; cxmt="${cxst##* }"
          _cx_metac "$cxid" "$cxrl" "${cxmt:-0}" "${cxbytes:-0}"; cxct="${REPLY##*$'\t'}"
          local cxcwd0="${REPLY%%$'\t'*}"; [[ -n "$proj" && -z "${PROJDIR[$proj]}" && -d "$cxcwd0" ]] && PROJDIR[$proj]="$cxcwd0"
          cxnm="$(_cx_name "$cxrl")"
          cxlive[$cxid]=1                   # so the resumable sweep won't duplicate it
        fi
        # window names (→ the VS Code tab) belong to cc-name-sync.sh, fired in the background
        # at sweep start — the single writer; an inline rename here fought it on same-cwd chats
        hideskip=0                          # hidden? same delta-scan rule as Claude rows
        if [[ -n "$cxid" && -n "${HID[$cxid]}" ]]; then
          if [[ -n "${AT[$cxid]}" ]] && (( ${cxbytes:-0} > AT[$cxid] )); then
            if (( $(_cx_newprompts "$cxrl" "${AT[$cxid]}") > 0 )); then UNHIDE[$cxid]=1
            else AT[$cxid]=${cxbytes:-0}; hideskip=1; fi
          else
            [[ -z "${AT[$cxid]}" ]] && AT[$cxid]=${cxbytes:-0}
            (( ${cxbytes:-0} < ${AT[$cxid]:-0} )) && AT[$cxid]=${cxbytes:-0}
            hideskip=1
          fi
        fi
        if (( hideskip )); then (( strict )) && { hidden=$((hidden+1)); continue; }
        elif (( onlyhidden )); then continue; fi
        dispname="⬢ ${cxnm:-Codex chat}"
        (( ${#dispname} > 30 )) && dispname="${dispname[1,29]}…"
        marks=""
        [[ "$att" == "1" ]] && marks+="  ${Xg}⇄${X0}"; [[ "$sock" == "$cursock" ]] && marks+="  ${Xg}← here${X0}"
        (( all && hideskip )) && marks+="  ${Xd}·hidden${X0}"
        agets="${cxmt:-$epoch}"             # age = last rollout write; tmux birth only when unresolved
        # Codex ids own no statusline anchor, so this always lands on the dim-age branch — the
        # Anthropic prompt-cache window is a Claude fact and a ✓/✗ here would be inventing one.
        _cc_cachew "$cxid" "$agets"
        # name column in magenta — the engine tint (matches the ⬢-new row and ⚙ agent tag), so
        # codex rows read apart from Claude's at a glance
        label="${Xg}●${X0} ${Xc}${(r:14:)proj}${X0} ${Xd}│${X0} ${Xm}${Xb}${(r:30:)dispname}${X0} ${Xd}│ ${(l:5:)${:-${cxct}p}} │ ${(l:6:)$(_cc_hsize "${cxbytes:-0}")} │ ${REPLY_C}${(r:6:)REPLY}${X0}${marks}"
        rows+=("${proj}"$'\t'"${cxct}"$'\t'"${agets}"$'\t'"L"$'\t'"${sock}"$'\t'"${name}"$'\t'"${label}"$'\t'"${cxid}")
        continue
      fi
      [[ "$sock" == cx-* ]] && continue     # Codex chats are not listed — see the note at source 2b
      # pane-keyed breadcrumbs (<sock>.<pane_id>, written by the statusline): a window SPLIT into
      # several chats gets ONE merged row — every pane's transcript marked live (no phantom ↻
      # sibling), Enter attaches the whole split. Stale pane files (pane gone) are swept here.
      ptp=()
      panes="$panelist"
      for pbc in "$siddir/$sock".%*(N); do
        pane_="${pbc##*.}"
        if [[ $'\n'"$panes"$'\n' == *$'\n'"%${pane_#'%'}"$'\n'* ]]; then ptp+=("$(<"$pbc")")
        else rm -f "$pbc" 2>/dev/null; fi
      done
      bytes=0; hsize="-"; tpath=""; ct=0; lmt=""; nm=""; acct=""
      if (( ${#ptp} >= 2 )); then           # ⊞ merged split-window row (hide/⌃X not applied — it's live)
        mnm=""; mmax=0; mnewu=""
        for tpath in "${ptp[@]}"; do
          [[ -n "$tpath" ]] || continue
          mu="${tpath:t:r}"; live[$mu]=1
          ma="$(_cc_acct "$tpath")"; [[ -n "$ma" && "$acct" != *"$ma"* ]] && acct+="$ma"   # accounts across the split, unique
          [[ -r "$tpath" ]] || continue
          _cc_stat "$tpath"; mst="$REPLY"
          bytes=$(( bytes + ${mst%% *} ))
          (( ${mst##* } > mmax )) && { mmax="${mst##* }"; mnewu="$mu"; }   # the split's most recently active chat owns the cache cell
          _cc_metac "$mu" "$tpath" "${mst##* }" "${mst%% *}"; mmeta="$REPLY"
          ct=$(( ct + ${mmeta##*$'\t'} ))
          nm="${${mmeta#*$'\t'}%%$'\t'*}"; [[ -z "$nm" ]] && nm="?"
          mnm+="${mnm:+ + }$nm"
        done
        dispname="$mnm"; [[ -z "$dispname" ]] && dispname="(split)"
        (( ${#dispname} > 30 )) && dispname="${dispname[1,29]}…"
        SEEN[${dispname}$'\x1f'${proj}]=1   # claim this name+project so a bg twin below can't repeat it
        agets="$mmax"; (( agets )) || agets="$epoch"   # age = last ACTIVITY (newest pane transcript), not tmux birth
        marks="  ${Xu}⊞${#ptp}${X0}"
        if [[ -n "$acct" ]]; then           # birth account(s) of the split's chats; ≠ = not the primary
          marks+="  ${Xd}"; for ma in ${(s::)acct}; do marks+="$(_cc_medal $ma)"; done; marks+="${X0}"
          [[ "$acct" != "$prim" ]] && marks+="${Xy}≠${X0}"
        fi
        [[ "$att" == "1" ]] && marks+="  ${Xg}⇄${X0}"; [[ "$sock" == "$cursock" ]] && marks+="  ${Xg}← here${X0}"
        nt=""; [[ "$acct" == *4* ]] && nt="$Xg"
        lc1h="$(_cc_c1h_tty "$ttylist")"   # the split shares one server, so one birth mode
        _cc_cachew "$mnewu" "$agets" "$(( lc1h ? 3600 : 300 ))"
        label="${Xg}●${X0} ${Xc}${(r:14:)proj}${X0} ${Xd}│${X0} ${nt}${Xb}${(r:30:)dispname}${X0} ${Xd}│ ${(l:5:)${:-${ct}p}} │ ${(l:6:)$(_cc_hsize "$bytes")} │ ${REPLY_C}${(r:6:)REPLY}${X0}${marks}"
        rows+=("${proj}"$'\t'"${ct}"$'\t'"${agets}"$'\t'"L"$'\t'"${sock}"$'\t'"${name}"$'\t'"${label}")
        continue
      fi
      (( ${#ptp} == 1 )) && [[ -n "${ptp[1]}" ]] && tpath="${ptp[1]}"   # pane-keyed beats last-writer socket file
      if [[ -z "$tpath" && -r "$siddir/$sock" ]]; then
        # trust a socket-level crumb only while a claude still runs on this server — the crumb
        # outlives its pane (RR_CLIENT: a dead pane's crumb steered attach at the wrong window).
        # pane ttys (from the probe) → one ps call; matches claude foregrounded or busy in a tool.
        if ps -o comm= -t "$ttylist" 2>/dev/null | grep -qE '^(claude|[0-9]+\.)'; then   # version-agnostic, like _cc_c1h_tty
          tpath="$(<"$siddir/$sock")"
        else rm -f "$siddir/$sock" 2>/dev/null; fi
      fi
      [[ -n "$tpath" ]] && live[${tpath:t:r}]=1                    # so source 2 won't duplicate it
      uuid="${tpath:t:r}"
      if [[ -n "$tpath" && -r "$tpath" ]]; then
        _cc_stat "$tpath"; stt="$REPLY"; bytes="${stt%% *}"; lmt="${stt##* }"; hsize="$(_cc_hsize "${bytes:-0}")"
      fi
      hideskip=0                                   # hidden? a new REAL prompt in the delta → auto-unhide
      if [[ -n "$tpath" && -n "${HID[$uuid]}" ]]; then
        if [[ -n "${AT[$uuid]}" ]] && (( ${bytes:-0} > AT[$uuid] )) && [[ -r "$tpath" ]]; then
          if (( $(_cc_newprompts "$tpath" "${AT[$uuid]}") > 0 )); then UNHIDE[$uuid]=1
          else AT[$uuid]=${bytes:-0}; hideskip=1; fi   # noise-only delta → ratchet, never rescan it
        else
          [[ -z "${AT[$uuid]}" ]] && AT[$uuid]=${bytes:-0}                          # lazy baseline
          (( ${bytes:-0} < ${AT[$uuid]:-0} )) && AT[$uuid]=${bytes:-0}              # rewritten → re-baseline
          hideskip=1
        fi
      fi
      if (( hideskip )); then (( strict )) && { hidden=$((hidden+1)); continue; }
      elif (( onlyhidden )); then continue; fi   # default hides ⌃X'd · --hidden shows ONLY them
      (( ${bytes:-0} == 0 && strict )) && { hidden=$((hidden+1)); continue; }   # hide orphans (reveal: cc-ls -a)
      if [[ -n "$lmt" ]]; then               # name + prompt count (incremental cache — _cc_metac)
        uuid="${tpath:t:r}"
        _cc_metac "$uuid" "$tpath" "$lmt" "${bytes:-0}"; cmeta="$REPLY"
        ct="${cmeta##*$'\t'}"; nm="${${cmeta#*$'\t'}%%$'\t'*}"
        local lcwd="${cmeta%%$'\t'*}"; [[ -n "$proj" && -z "${PROJDIR[$proj]}" && -d "$lcwd" ]] && PROJDIR[$proj]="$lcwd"   # remember a real dir for ⌃R-aware ✦ new-chat
      fi
      # the transcript's /rename title is the truth, and _cc_meta CACHES it (title wins over the
      # first prompt inside nm) — no per-row transcript grep here; at 400MB of live chats that
      # cost whole seconds per cc-ls
      if [[ -n "$nm" ]]; then
        dispname="$nm"
      elif [[ "$name" == cc-* ]]; then
        dispname="$tag"; [[ -z "$dispname" || "$dispname" == "Claude Code" ]] && dispname="(unnamed)"
      else
        dispname="$name"
      fi
      # still unnamed → the last real prompt as a final resort
      # ZERO PROMPTS → HIDDEN. A chat nobody has spoken to is a husk: it carries a name (birth
      # title, ai-title, tmux session) but no conversation, so it looks like a real row and is
      # never worth opening. Codex rows have always been filtered this way (source 2b); this puts
      # Claude rows — live and resumable — on the same rule. strict only, so `cc-ls -a` reveals.
      (( ${ct:-0} == 0 && strict )) && { hidden=$((hidden+1)); continue; }
      [[ "$dispname" == "(unnamed)" ]] && lp="$(_cc_lastprompt "$tpath")" && [[ -n "$lp" ]] && dispname="$lp"
      (( ${#dispname} > 30 )) && dispname="${dispname[1,29]}…"
      SEEN[${dispname}$'\x1f'${proj}]=1   # a live row owns its name+project (see BG-TWIN DEDUP below)
      agets="${lmt:-$epoch}"                # age = last ACTIVITY (transcript mtime), not tmux birth
      marks=""; acct="$(_cc_acct "$tpath")"
      if [[ -n "$acct" ]]; then             # birth account; ≠ = not the primary (Enter offers the swap)
        marks+="  ${Xd}$(_cc_medal $acct)${X0}"; [[ "$acct" != "$prim" ]] && marks+="${Xy}≠${X0}"
      fi
      lc1h="$(_cc_c1h_tty "$ttylist")"      # born with ⚡1h-cache? Enter offers a reboot to match ⌃E
      [[ "$lc1h" == 1 ]] && marks+="  ${Xy}⚡${X0}"
      [[ "$att" == "1" ]] && marks+="  ${Xg}⇄${X0}"; [[ "$sock" == "$cursock" ]] && marks+="  ${Xg}← here${X0}"
      (( all && hideskip )) && marks+="  ${Xd}·hidden${X0}"   # tag still-hidden ones in -a
      nt=""; [[ "$acct" == *4* ]] && nt="$Xg"
      _cc_cachew "$uuid" "$agets" "$(( lc1h ? 3600 : 300 ))"   # TTL is the chat's BIRTH mode: flagless = 1h, FORCE_PROMPT_CACHING_5M=1 = 5m
      label="${Xg}●${X0} ${Xc}${(r:14:)proj}${X0} ${Xd}│${X0} ${nt}${Xb}${(r:30:)dispname}${X0} ${Xd}│ ${(l:5:)${:-${ct}p}} │ ${(l:6:)hsize} │ ${REPLY_C}${(r:6:)REPLY}${X0}${marks}"
      rows+=("${proj}"$'\t'"${ct}"$'\t'"${agets}"$'\t'"L"$'\t'"${sock}"$'\t'"${name}"$'\t'"${label}"$'\t'"${uuid}"$'\t'"${acct}"$'\t'"${lc1h}")   # f8 = transcript uuid (⌃X hides) · f9 = birth account · f10 = born-with-⚡1h (Enter's reboot gate)
    done <<< "$tls"
  done
  rm -rf "$probed"

  # ── collapse multi-server claims: N live rows sharing one transcript/rollout id (f8) are N
  # tmux servers running ONE chat — the _cc_solo corruption hazard, usually a leftover second
  # resume after an account switch (seen live: WATCHER ×2, one server from each launch). Show
  # ONE row — the newest-born server (epoch in its cc-/cx- socket name) wins, since that's the
  # launch the founder is actually typing into; Enter's _cc_solo then reaps the elder hosts.
  # The collapse is marked ⚠Nsrv in yellow — a silent merge would hide a real two-writer
  # hazard. Split-window ⊞ rows carry no f8 and are never collapsed.
  typeset -A _lidx _lcnt
  local _i _id _oe _ne
  for (( _i=1; _i <= ${#rows}; _i++ )); do
    local -a _rf=("${(@ps:\t:)rows[_i]}")
    [[ "${_rf[4]}" == L && -n "${_rf[8]:-}" ]] || continue
    _id="${_rf[8]}"
    if [[ -z "${_lidx[$_id]}" ]]; then _lidx[$_id]=$_i; _lcnt[$_id]=1; continue; fi
    _lcnt[$_id]=$(( _lcnt[$_id] + 1 ))
    _oe="${${(s:-:)${${(@ps:\t:)rows[${_lidx[$_id]}]}[5]}}[2]:-0}"   # incumbent's birth epoch
    _ne="${${(s:-:)_rf[5]}[2]:-0}"                                    # challenger's birth epoch
    if [[ "$_ne" == <-> && "$_oe" == <-> ]] && (( _ne > _oe )); then
      rows[${_lidx[$_id]}]=""; _lidx[$_id]=$_i
    else
      rows[_i]=""
    fi
  done
  for _id in ${(k)_lcnt}; do
    (( _lcnt[$_id] > 1 )) || continue
    _i=${_lidx[$_id]}
    local -a _pf=("${(@ps:\t:)rows[_i]}")
    _pf[7]+="  ${Xy}⚠${_lcnt[$_id]}srv${X0}"
    rows[_i]="${(pj:\t:)_pf}"
  done
  rows=("${(@)rows:#}")

  # ── source 2: resumable chats from the store, deduped, newest first (cache keeps it fast) ──
  local cap=30 shown=0 line mt sz fp isagent acfg
  (( strict )) || cap=99999
  for line in ${(f)"$(cc_find_meta "$store" -maxdepth 2 -name '*.jsonl')"}; do
    mt="${line%%.*}"; sz="${${line#*$'\t'}%%$'\t'*}"; fp="${line##*$'\t'}"; uuid="${fp:t:r}"
    [[ -n "${live[$uuid]}" ]] && continue   # already shown as a live tmux session
    isagent=0; acfg="${CCAGENT[$uuid]}"; [[ -n "$acfg" ]] && isagent=1   # live bg/forked agent → attach (below), never resume
    # (the sessionKind:"bg" skip moved BELOW, where the prompt count exists — see there)
    hideskip=0                                   # hidden? a new REAL prompt in the delta → auto-unhide
    if [[ -n "${HID[$uuid]}" ]]; then
      if [[ -n "${AT[$uuid]}" ]] && (( sz > AT[$uuid] )); then
        if (( $(_cc_newprompts "$fp" "${AT[$uuid]}") > 0 )); then UNHIDE[$uuid]=1
        else AT[$uuid]=$sz; hideskip=1; fi         # noise-only delta → ratchet, never rescan it
      else
        [[ -z "${AT[$uuid]}" ]] && AT[$uuid]=$sz                                    # lazy baseline
        (( sz < ${AT[$uuid]:-0} )) && AT[$uuid]=$sz                                 # rewritten → re-baseline
        hideskip=1
      fi
    fi
    if (( hideskip )); then (( strict )) && { hidden=$((hidden+1)); continue; }
    elif (( onlyhidden )); then continue; fi   # default hides ⌃X'd · --hidden shows ONLY them
    (( sz == 0 && ! isagent )) && continue
    (( shown >= cap && ! isagent )) && { hidden=$((hidden+1)); continue; }   # a live agent is never capped away
    _cc_metac "$uuid" "$fp" "$mt" "$sz"; cmeta="$REPLY"   # incremental cache — appended bytes only
    cwd="${cmeta%%$'\t'*}"; ct="${cmeta##*$'\t'}"; nm="${${cmeta#*$'\t'}%%$'\t'*}"
    # 0 prompts → hidden, same rule as the live rows and as codex's source 2b. Keyed on the
    # COUNT, not on an empty name: a husk often has a title (ai-title, or a /rename that landed
    # before any prompt) and the old nameless test let exactly those through. A live agent still
    # always shows — it is doing work whether or not a human typed at it.
    (( ${ct:-0} == 0 )) && (( strict && ! isagent )) && { hidden=$((hidden+1)); continue; }
    [[ -z "$nm" ]] && (( strict && ! isagent )) && { hidden=$((hidden+1)); continue; }
    # A background SESSION (records carry sessionKind:"bg") CAN be the harness's async twin of a
    # real chat — it inherits the parent's custom-title, so it lists as a second row under the
    # same name. But sessionKind alone does NOT mean twin: on this fleet it marks any DETACHED
    # launch, and skipping on the marker alone hid 11 real chats from the picker at once —
    # CONTRACTS (17 prompts), SEC (45), LLM_JUNK (55), RR_ENGINEER (34), CC_FLEET (21), WATCHER,
    # RR, RR_CLIENT, RR_STELLAR_TEST … every one a chat the founder had actually talked to, and
    # the only way back to one was knowing its uuid by heart. A real twin is distinguishable by
    # what it does NOT have: prompts of its own. So require BOTH — the bg marker AND a zero
    # prompt count (`ct`, already computed above; string-compared because a corrupt cache row can
    # leave a uuid in there and a (( )) test would abort the whole picker). The test is now
    # strictly narrower than before, so it can only ever reveal a chat, never hide a new one.
    # A live bg AGENT is a different thing — its own ⚙ row — so never skip that; `-a` shows all.
    proj="${cwd:t}"; [[ -z "$proj" ]] && proj="?"
    [[ -n "$proj" && -z "${PROJDIR[$proj]}" && -d "$cwd" ]] && PROJDIR[$proj]="$cwd"   # dir for ⌃R-aware ✦ new-chat
    dispname="$nm"; [[ -z "$dispname" ]] && dispname="(no prompt)"
    (( ${#dispname} > 30 )) && dispname="${dispname[1,29]}…"
    # BG-TWIN DEDUP. A background session is the harness's async twin of a real chat and inherits
    # its custom-title, so it lists as a SECOND row under the same name (live: CC_FLEET 232p plus a
    # twin at 18p). The old rule skipped EVERY sessionKind:"bg" transcript, which hid 11 real chats
    # at once — CONTRACTS, SEC, LLM_JUNK, RR_ENGINEER … each one a chat the founder had talked to,
    # reachable only by typing its uuid. Prompt count does not separate them either: a twin records
    # the same user turns, so it has prompts of its own. What a twin never has is a name of its own
    # — it borrows the parent's. So hide a bg row only when this exact name+project was ALREADY
    # claimed (SEEN, seeded by every live row and every non-bg resumable). A bg chat whose parent is
    # gone keeps its row; two genuinely distinct chats that merely share a title both keep theirs,
    # because only the bg one can ever be suppressed. `-a` still shows everything.
    if (( strict && ! isagent )) && [[ -n "${SEEN[${dispname}$'\x1f'${proj}]}" ]] && _cc_isbg "$fp"; then hidden=$((hidden+1)); continue; fi
    SEEN[${dispname}$'\x1f'${proj}]=1
    # A dead chat's process is gone, so its birth mode is unknowable — take the SHORT window.
    # The entry Anthropic holds was written with the TTL of the last request, and every chat this
    # fleet launches carries FORCE_PROMPT_CACHING_5M=1 unless ⚡ armed it. Assuming 1h here would
    # paint ✓ on a chat whose 5m window shut 55 minutes ago — a green that costs real money.
    _cc_cachew "$uuid" "$mt" 300
    label="${Xc}${(r:14:)proj}${X0} ${Xd}│${X0} ${(r:30:)dispname} ${Xd}│ ${(l:5:)${:-${ct}p}} │ ${(l:6:)$(_cc_hsize "$sz")} │ ${REPLY_C}${(r:6:)REPLY}${X0}"
    if (( isagent )); then
      acct="$(_cc_acct "$acfg")"            # birth account (no ≠ — a takeover resumes under the primary anyway)
      label="${Xm}⚙${X0} $label  ${Xm}agent${X0}"; [[ -n "$acct" ]] && label="$label ${Xd}$(_cc_medal $acct)${X0}"
      (( all && hideskip )) && label="$label  ${Xd}·hidden${X0}"
      rows+=("${proj}"$'\t'"${ct}"$'\t'"${mt}"$'\t'"A"$'\t'"${uuid}"$'\t'"${cwd}"$'\t'"${label}"$'\t'"${acfg}")   # A=live agent · f6=cwd · f8=owning config dir
    else
      label="${Xd}↻${X0} $label"; (( all && hideskip )) && label="$label  ${Xd}·hidden${X0}"
      rows+=("${proj}"$'\t'"${ct}"$'\t'"${mt}"$'\t'"R"$'\t'"${uuid}"$'\t'"${cwd}"$'\t'"${label}")   # field 6 = session's home dir (claude --resume is cwd-scoped)
      shown=$((shown+1))
    fi
  done

  # Codex threads are no longer listed here (2026-07-29) — cc-ls is Claude-only; `cx` still
  # launches Codex chats and cx-hide still prunes them, they simply do not appear in the picker.
  # ── source 2b: resumable CODEX threads — rollouts no live cx chat holds open ──
  # User threads only (subagent rollouts are workers, not chats); Enter → `codex resume <id>` in
  # a fresh cx-* server. Candidates bounded to the newest 120 files — the store grows unbounded
  # and each cold classification reads the rollout's meta line (the NC cache absorbs re-runs).
  local cxcap=15 cxshown=0
  (( strict )) || cxcap=99999
  if command -v codex >/dev/null; then
    for line in ${(f)"$(cc_find_meta "$HOME/.codex/sessions" -name 'rollout-*.jsonl' | head -120)"}; do
      mt="${line%%.*}"; sz="${${line#*$'\t'}%%$'\t'*}"; fp="${line##*$'\t'}"
      uuid="${${fp:t:r}#rollout-????-??-??T??-??-??-}"
      [[ -n "${cxlive[$uuid]}" ]] && continue        # already shown as a live cx row
      (( sz == 0 )) && continue
      _cx_metac "$uuid" "$fp" "$mt" "$sz"; cwd="${REPLY%%$'\t'*}"; ct="${REPLY##*$'\t'}"
      [[ "$cwd" == "§sub§" ]] && continue            # subagent worker thread, not a chat
      hideskip=0
      if [[ -n "${HID[$uuid]}" ]]; then              # hidden? same delta-scan rule as Claude rows
        if [[ -n "${AT[$uuid]}" ]] && (( sz > AT[$uuid] )); then
          if (( $(_cx_newprompts "$fp" "${AT[$uuid]}") > 0 )); then UNHIDE[$uuid]=1
          else AT[$uuid]=$sz; hideskip=1; fi
        else
          [[ -z "${AT[$uuid]}" ]] && AT[$uuid]=$sz
          (( sz < ${AT[$uuid]:-0} )) && AT[$uuid]=$sz
          hideskip=1
        fi
      fi
      if (( hideskip )); then (( strict )) && { hidden=$((hidden+1)); continue; }
      elif (( onlyhidden )); then continue; fi
      (( ct == 0 && strict )) && continue            # promptless husk (reveal: cc-ls -a)
      (( cxshown >= cxcap )) && { hidden=$((hidden+1)); continue; }
      nm="$(_cx_name "$fp")"
      dispname="⬢ ${nm:-(no prompt)}"; (( ${#dispname} > 30 )) && dispname="${dispname[1,29]}…"
      proj="${cwd:t}"; [[ -z "$proj" ]] && proj="?"
      [[ -n "$proj" && -z "${PROJDIR[$proj]}" && -d "$cwd" ]] && PROJDIR[$proj]="$cwd"   # dir for ⌃R-aware ✦ new-chat
      _cc_cachew "$uuid" "$mt"   # codex id → dim age, same as the live ⬢ rows
      label="${Xd}↻${X0} ${Xc}${(r:14:)proj}${X0} ${Xd}│${X0} ${Xm}${(r:30:)dispname}${X0} ${Xd}│ ${(l:5:)${:-${ct}p}} │ ${(l:6:)$(_cc_hsize "$sz")} │ ${REPLY_C}${(r:6:)REPLY}${X0}"   # magenta name = codex, like the live rows
      (( all && hideskip )) && label="$label  ${Xd}·hidden${X0}"
      rows+=("${proj}"$'\t'"${ct}"$'\t'"${mt}"$'\t'"X"$'\t'"${uuid}"$'\t'"${cwd}"$'\t'"${label}")   # X = codex resume · f6 = thread's home dir
      cxshown=$((cxshown+1))
    done
  fi
  { for u in ${(k)NC}; do print -r -- "$u"$'\t'"${NC[$u]}"; done } | bash "$CC_DB" chat-save 2>/dev/null   # persist cache (one transaction)
  if (( ${#UNHIDE} )); then            # chats that grew since hidden → drop from the hide-list (auto-return)
    # grep exits 1 when EVERY line is dropped (the common one-hidden-chat case) — that's a
    # valid empty result, not a failure; only >1 (real error) may veto the mv
    (   # one flock across ALL hide-list writers (⌃X toggle, this auto-unhide, cc-hide append)
        # so a concurrent append landing inside this read-modify-write is not lost; per-pid temp
        flock 9 2>/dev/null || true
        grep -vxF -f =(print -l -- ${(k)UNHIDE}) "$hf" > "$hf.t.$$" 2>/dev/null
        (( $? <= 1 )) && mv "$hf.t.$$" "$hf" || rm -f "$hf.t.$$"
    ) 9>"$hf.lock"
    for u in ${(k)UNHIDE}; do unset "AT[$u]"; done
  fi
  # ONE transaction replaces the old flock'd two-file rewrite, whose own comment conceded
  # "last-writer-wins" — the exact mechanism by which a concurrent picker's hides disappeared.
  # Send the full intended hidden set (uuid + baseline); the db makes it so atomically.
  { for u in ${(k)HID}; do print -r -- "$u"$'\t'"${AT[$u]:-}"; done } | bash "$CC_DB" hidden-sync 2>/dev/null

  if (( ${#rows} == 0 )); then
    if   (( onlyhidden )); then echo "cc-ls: no hidden chats"
    elif (( hidden ));     then echo "cc-ls: $hidden hidden — cc-ls -a (all) · cc-ls --hidden (just those)"
    else echo "cc-ls: no chats found"; fi
    return 0
  fi

  if ! command -v fzf >/dev/null; then
    printf '%s\n' "${rows[@]}" | sort -t$'\t' -k1,1 -k3,3nr | cut -f7
    echo "cc-ls: fzf not found"; return 1
  fi

  # pre-sort project ▸ recent; ⌃R rotates which project sits on top. rotate.awk shifts whole
  # project-blocks (the file is proj-grouped) by a counter. Cheap: awk+sort over a few dozen rows.
  local tmpd; tmpd="$(mktemp -d "${TMPDIR:-/tmp}/cc-ls.XXXXXX")" || tmpd="/tmp/cc-ls.$$"
  printf '%s\n' "${rows[@]}" | sort -t$'\t' -k1,1 -k3,3nr > "$tmpd/by_time"   # project ▸ recent
  # PROJDIR → a file the fzf-side sh reads so the ✦ new-chat rows can target the CURRENTLY-rotated
  # project's real dir (project basename → cwd)
  { for _p in ${(k)PROJDIR}; do print -r -- "$_p"$'\t'"${PROJDIR[$_p]}"; done } > "$tmpd/projdir"
  # start with the CURRENT dir's project group on top (⌃R rotates onward from there)
  local rot0 pj="${PWD:t}"
  rot0="$(cut -f1 "$tmpd/by_time" | uniq | grep -nxF -- "$pj" | head -1 | cut -d: -f1)"
  [[ -n "$rot0" ]] && rot0=$((rot0 - 1)) || rot0=0
  print -r -- "$rot0" > "$tmpd/rot"
  print -r -- '{ if ($1 != p) { b++; p = $1 } blk[NR] = b; ln[NR] = $0 }
END { N = b; if (N < 1) N = 1; Rn = ((R % N) + N) % N
      for (i = 1; i <= NR; i++) { k = ((blk[i] - 1 - Rn) + N) % N; printf "%06d%08d\t%s\n", k, i, ln[i] } }' > "$tmpd/rotate.awk"
  # ✦ new-chat rows (kind N Claude · C GPT/account-4) — regenerated EACH reload so their target dir FOLLOWS
  # the ⌃R rotation: the project on top after rotating by $rot is block (Rn+1) in by_time's project
  # order; its dir comes from projdir (falls back to $PWD). f6 carries that dir; the Enter handler
  # launches there. Kept outside the sort so they always sit on top. Not shown in --hidden.
  local cxok="";  command -v codex >/dev/null && cxok=1   # the Codex row needs the codex binary
  if (( ! onlyhidden )); then
    cat > "$tmpd/newrow.sh" <<NRW
r=\$(cat '$tmpd/rot' 2>/dev/null || echo 0)
projs=\$(cut -f1 '$tmpd/by_time' | awk '!s[\$0]++')
N=\$(printf '%s\n' "\$projs" | grep -c .); [ "\$N" -lt 1 ] && N=1
Rn=\$(( r % N )); [ "\$Rn" -lt 0 ] && Rn=\$(( Rn + N ))
tp=\$(printf '%s\n' "\$projs" | sed -n "\$(( Rn + 1 ))p"); [ -z "\$tp" ] && tp='${PWD:t}'
td=\$(awk -F'\t' -v p="\$tp" '\$1==p{print \$2; exit}' '$tmpd/projdir'); [ -z "\$td" ] && td='$PWD'
pn=\$(printf '%-14.14s' "\$tp")
printf '%s\t0\t0\tN\t\t%s\t${Xy}${Xb}✦${X0} ${Xc}%s${X0} ${Xd}│${X0} ${Xy}%-30s${X0} ${Xd}│ %5s │ %6s │ %-8s${X0}\n' "\$tp" "\$td" "\$pn" '➕ new Claude chat here' '' '' now
[ -n '$cxok' ] && printf '%s\t0\t0\tC\t\t%s\t${Xm}${Xb}✦${X0} ${Xc}%s${X0} ${Xd}│${X0} ${Xm}%-30s${X0} ${Xd}│ %5s │ %6s │ %-8s${X0}\n' "\$tp" "\$td" "\$pn" '⬢ new Codex chat here' '' '' now
NRW
  fi
  # reload: emit the ✦ rows (dir follows ⌃R), then the rotated+sorted list, strip the rank prefix, filter hidden
  local hgrep=""
  (( strict ))     && hgrep=" | grep -vFf '$hf'"   # default: drop hidden
  (( onlyhidden )) && hgrep=" | grep -Ff '$hf'"    # --hidden: keep ONLY hidden
  local reload="[ -r '$tmpd/newrow.sh' ] && sh '$tmpd/newrow.sh'; r=\$(cat '$tmpd/rot'); awk -F'\t' -v R=\"\$r\" -f '$tmpd/rotate.awk' '$tmpd/by_time' | sort | cut -f2-${hgrep}"
  local tog_proj="echo \$((\$(cat '$tmpd/rot')+1)) > '$tmpd/rot'; $reload"
  # as files — binds stay quotable, and the load-transform can re-run the pipeline to FIND a row
  print -r -- "$reload"   > "$tmpd/reload.sh"
  print -r -- "$tog_proj" > "$tmpd/tog_proj.sh"
  # border-label builder — one source of truth for ⌃S (account) and ⌃E (⚡1h-cache) state;
  # $1 = this picker's per-run arm file ("$tmpd/1h")
  cat > "$tmpd/label.sh" <<'LBL'
n=$(cat "$HOME/.claude-primary" 2>/dev/null || echo 1)
case "$n" in 2) m=🥈 ;; 3) m=🥉 ;; *) n=1; m=🥇 ;; esac
c="🪫 5m cache (forced)"; [ "$(cat "${1:-}" 2>/dev/null)" = 1 ] && c="⚡ 1h ON (this pick)"
printf ' tmux + Claude/Codex chats · cc → %s acct %s (⌃S) · %s (⌃E) ' "$m" "$n" "$c"
LBL

  # two-line header: actions on top, glyph legend below (was one long noisy line)
  local hdr=$'⏎ open · ♻⌃O reload · ⇅⌃T sort · ⚡⌃E 1h-cache · 🔄⌃R rotate · 🙈⌃X hide⇄show · 🎖⌃S account\n● live · ↻ resume · ⚙ agent · ✦ new · ⬢ codex · 🍀 GPT · ⇄ attached · last col 💾 ✓warm ✗cold (dim = age)'
  local blabel="$(sh "$tmpd/label.sh" "$tmpd/1h")"
  if (( onlyhidden )); then
    hdr=$'HIDDEN only — ⌃X restores\n⏎ open · 🔄⌃R rotate'; blabel=' hidden chats · ⌃X restore '
  elif (( hidden )); then
    hdr="$hdr"$' · '"${hidden} hidden (-a all · --hidden only)"
  fi
  local pick
  # --no-mouse: a focus-click (or trackpad-inertia scroll) into a fresh terminal lands on a row
  # and silently steals the cursor from "new chat here" — Enter then opens whatever chat sat
  # under the pointer. Keyboard + fuzzy-typing are the picker's only inputs.
  pick="$(fzf \
    --delimiter=$'\t' --with-nth=7 \
    --expect=ctrl-t,ctrl-o \
    --no-mouse \
    --height=~70% --reverse --cycle --no-info --header-first --gap=1 \
    --ansi --highlight-line \
    --border=rounded --border-label="$blabel" \
    --header="$hdr" \
    --prompt='cc ❯ ' --pointer='▌' \
    --bind "ctrl-t:execute-silent(printf %s {5} > '$tmpd/want')+reload:sh '$tmpd/tog_sort.sh'" \
    --bind "ctrl-r:execute-silent(printf %s {5} > '$tmpd/want')+reload:sh '$tmpd/tog_proj.sh'" \
    --bind "ctrl-x:execute-silent(printf %s \"\$FZF_POS\" > '$tmpd/pos'; [ {4} = N ] && exit 0; id={5}; if [ {4} = L ]; then id={8}; fi; [ -n \"\$id\" ] || exit 0; F='$hf'; ( flock 9 2>/dev/null || true; if grep -qxF -- \"\$id\" \"\$F\"; then grep -vxF -- \"\$id\" \"\$F\" > \"\$F.t.\$\$\"; mv \"\$F.t.\$\$\" \"\$F\"; else printf '%s\n' \"\$id\" >> \"\$F\"; fi ) 9>\"\$F.lock\")+reload:sh '$tmpd/reload.sh'" \
    --bind "load:transform(if [ -r '$tmpd/pos' ]; then printf 'pos(%s)' \"\$(cat '$tmpd/pos')\"; rm -f '$tmpd/pos'; elif [ -s '$tmpd/want' ]; then n=\$(sh '$tmpd/reload.sh' | awk -F'\t' -v w=\"\$(cat '$tmpd/want')\" '\$5==w{print NR; exit}'); rm -f '$tmpd/want'; [ -n \"\$n\" ] && printf 'pos(%s)' \"\$n\"; else rm -f '$tmpd/want'; fi || :)" \
    --bind "ctrl-e:execute-silent(f='$tmpd/1h'; if [ \"\$(cat \"\$f\" 2>/dev/null)\" = 1 ]; then rm -f \"\$f\"; else echo 1 > \"\$f\"; fi)+transform-border-label(sh '$tmpd/label.sh' '$tmpd/1h')" \
    --bind "ctrl-s:execute-silent(n=\$(bash '$CC_DB' primary-get); case \"\$n\" in 1) n=2 ;; 2) n=3 ;; *) n=1 ;; esac; bash '$CC_DB' primary-set \"\$n\")+transform-border-label(sh '$tmpd/label.sh' '$tmpd/1h')" \
    --color='border:cyan,label:bold:cyan,header:dim,prompt:bold:cyan,pointer:bright-yellow,fg+:bold,hl:bright-yellow,hl+:bright-yellow:bold' \
    < <(sh "$tmpd/reload.sh"))" || true
  # ⌃E arms the ⚡1h-cache for THIS pick only (shown in the border) — read this run's arm file,
  # then consume it with the tmpd. Env is read at process birth: it applies to the chat LAUNCHED
  # now (new/resume/takeover), never to already-running ones.
  local c1h=0
  [[ "$(cat "$tmpd/1h" 2>/dev/null)" == 1 ]] && c1h=1
  rm -rf "$tmpd"
  # c1henv sanitizes every run-string launch: strip the inherited flags (a chat's leaked
  # birth env), then pick the mode explicitly — armed re-adds ENABLE, un-armed must FORCE
  # 5m (since CC 2.1.215 the harness defaults to 1h, so omission alone means 1h)
  local c1henv="env -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M "
  if (( c1h )); then c1henv+="ENABLE_PROMPT_CACHING_1H=1 "; else c1henv+="FORCE_PROMPT_CACHING_5M=1 "; fi

  [[ -z "$pick" ]] && { echo "cc-ls: nothing selected"; return 0; }
  # --expect=ctrl-o: fzf's first output line is the accepting key ("" for plain Enter),
  # the selected row follows. Split them before field-parsing.
  local key=""
  if [[ "$pick" == *$'\n'* ]]; then key="${pick%%$'\n'*}"; pick="${pick#*$'\n'}"; fi
  [[ -z "$pick" ]] && { echo "cc-ls: nothing selected"; return 0; }
  local -a f=("${(@ps:\t:)pick}")
  local kind="$f[4]"
  # ♻⌃O reload — fully reboot a LIVE chat: kill its tmux session/server, then fall through to the
  # NOT ⌃B. This was bound to ctrl-b and could never fire, for two independent reasons: ctrl-b was
  # missing from --expect (so fzf never returned it as an accepting key), and C-b is tmux's PREFIX
  # — the picker always runs inside tmux, so tmux swallowed the keystroke before fzf saw it. Any
  # replacement must clear both bars: listed in --expect, and not a tmux prefix. ⌃O is free in
  # fzf, free in tmux, and unused by the other binds here (⌃T sort · ⌃R rotate · ⌃X hide · ⌃E
  # 1h-cache · ⌃S account).
  # ordinary resume branch (R for Claude, X for Codex) so the relaunch reuses the exact
  # battle-tested open path (fresh server, cwd-scoped resume, _cc_solo reaping any OTHER
  # server still hosting this chat — so ⌃O also collapses a ⚠Nsrv multi-server claim to one).
  # The chat's home dir comes from the namecache (f6 holds the SESSION name on L rows, not a
  # cwd). On non-live rows ⌃O degrades to plain Enter: R/X already boot fresh; A attaches.
  if [[ "$key" == ctrl-o && "$kind" == L ]]; then
    local rlsock="$f[5]" rlid="${f[8]:-}"
    if [[ -z "$rlid" ]]; then
      echo "⌃O reload: this row is a ⊞ split window (several chats, no single transcript) — not reloadable as one; attach and /exit the panes instead"; return 1
    fi
    # KILL THE SESSION, NOT THE SERVER, when the socket is shared. A cx-* (and occasionally a
    # cc-*) server can host sessions that are not this chat — a parked dev server can share the
    # socket — and kill-server would take them down with the chat being reloaded. Same
    # trap that made the codex reap and the ⬢ row listing unsafe. One session on the socket means
    # kill-server is equivalent and still sweeps the socket file, so prefer it only then.
    local _nsess
    _nsess="$(tmux -L "$rlsock" list-sessions -F x 2>/dev/null | grep -c .)"
    if (( ${_nsess:-0} > 1 )); then
      echo "♻ Reloading $rlid — socket $rlsock hosts ${_nsess} sessions, killing only this chat's…"
      tmux -L "$rlsock" kill-session -t "=$f[6]" 2>/dev/null
    else
      echo "♻ Reloading $rlid — killing tmux -L $rlsock, then resuming fresh…"
      tmux -L "$rlsock" kill-server 2>/dev/null
    fi
    rm -f "$dir/$rlsock" "$siddir/$rlsock" "$siddir/$rlsock".%*(N) 2>/dev/null   # sweep socket + crumbs (N: pane-crumb glob may be empty)
    f[5]="$rlid"
    f[6]="${${(@ps:\t:)NC[$rlid]}[3]:-$PWD}"   # namecache cwd — resume is cwd-scoped
    [[ "$rlsock" == cx-* ]] && kind=X || kind=R
  fi
  if [[ "$kind" == N ]]; then               # ➕ new Claude chat in the dir cc-ls was opened from
    echo "New chat in $PWD (account $(_cc_primary))$( (( c1h )) && printf ' · ⚡1h-cache')"
    if (( c1h )); then CC_ARM_1H=1 cc; else cc; fi
    return $?                                 # CC_ARM_1H = the picker's explicit verdict (_cc_arm1h)
  elif [[ "$kind" == C ]]; then             # ⬢ new Codex chat in the dir cc-ls was opened from
    (( c1h )) && echo "⚡1h-cache is Claude-only — ignored for a Codex launch"
    echo "New Codex chat in $PWD"
    cx
    return $?
  elif [[ "$kind" == X ]]; then             # ⬢ resumable Codex thread → codex resume in a fresh cx server
    uuid="$f[5]"; local xcwd="$f[6]"          # launch in the thread's home dir, like Claude's R rows
    [[ -d "$xcwd" ]] || xcwd="$PWD"
    (( c1h )) && echo "⚡1h-cache is Claude-only — ignored for a Codex resume"
    local xs="cx-$(date +%s)-$$-$RANDOM"
    echo "Resuming Codex thread $uuid in $xcwd → new tmux -L $xs"
    _cx_server "$xs" "$xcwd" "env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u CLAUDE_CONFIG_DIR -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M codex --dangerously-bypass-approvals-and-sandbox resume ${(q)uuid}" || return
    if _cc_in_bunker; then TMUX= exec tmux -L "$xs" attach   # viewport dies with the tab
    else TMUX= tmux -L "$xs" attach; fi
  elif [[ "$kind" == L ]]; then               # live → attach across its socket
    sock="$f[5]"; name="$f[6]"
    _cc_open_gate "${f[9]:-}" "${f[10]:-}" "$c1h" "$name"   # birth env ≠ picker? offer the in-place reboot
    [[ "$REPLY" == s ]] && bash "$CC_FLEET_HOME/cc-swap-chat.sh" --sock "$sock" "$(_cc_primary)" --1h "$c1h"
    echo "Attaching → -L $sock · $name"
    _cc_solo "${f[8]:-}" "$sock"              # one instance per chat — close any other host first
    tmux -L "$sock" set -g window-size latest 2>/dev/null   # a second client must not clamp the split
    if _cc_in_bunker; then TMUX= exec tmux -L "$sock" attach -t "$name"   # viewport dies with the tab
    else TMUX= tmux -L "$sock" attach -t "$name"; fi
  elif [[ "$kind" == A ]]; then             # live background agent → open the agent view to ATTACH
    uuid="$f[5]"; local acwd="$f[6]" acfg="$f[8]"   # --resume can't touch a running session; `claude agents` attaches
    [[ -d "$acwd" ]] || acwd="$PWD"; [[ -n "$acfg" ]] || acfg="$HOME/.claude"
    local as="cc-$(date +%s)-$$-$RANDOM"
    echo "'$uuid' is a live background agent — auto-routing (attach if working, else takeover) → new tmux -L $as"
    local ocfg="$acfg"; [[ "$ocfg" == "$HOME/.claude" ]] && ocfg=""   # acct1 = unset for the helper
    _cc_solo "$uuid"                          # one instance per chat — close any tmux host first
    _cc_in_bunker && TMUX= exec tmux -L "$as" new-session -s "$as" -c "$acwd" \
      "${c1henv}bash ${(q)CC_FLEET_HOME}/cc-agent-open.sh $uuid ${(q)acwd} ${(q)ocfg}"
    TMUX= tmux -L "$as" new-session -s "$as" -c "$acwd" \
      "${c1henv}bash ${(q)CC_FLEET_HOME}/cc-agent-open.sh $uuid ${(q)acwd} ${(q)ocfg}"
  else                                       # resumable → cc --resume in a fresh tmux (like _cc_run)
    uuid="$f[5]"; local rcwd="$f[6]"           # launch in the session's home dir — claude --resume is cwd-scoped
    [[ -d "$rcwd" ]] || rcwd="$PWD"             # fall back if the project dir is gone
    local ra; ra="$(_cc_resume_acct "$uuid")"   # GPT chats resume on 4, Claude chats never do
    local cfg; case "$ra" in 2|3|4) cfg="$HOME/.cc/$ra" ;; *) cfg="" ;; esac
    local rs="cc-$(date +%s)-$$-$RANDOM"
    echo "Resuming $uuid in $rcwd → new tmux -L $rs"
    # failure net: a session live OUTSIDE tmux is invisible to _cc_agents when its argv carries no
    # uuid (picker-resume, --continue; pane siblings too) — --resume then refuses. Fall through to
    # the auto-router so Enter still lands somewhere useful instead of an instant [exited].
    local rpfx="env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M ${CC4_UNSET} "   # never inherit a host chat's identity or cache mode
    if [[ -n "$cfg" ]]; then rpfx+="CLAUDE_CONFIG_DIR=${(q)cfg} "; else rpfx+="-u CLAUDE_CONFIG_DIR "; fi
    if (( c1h )); then rpfx+="ENABLE_PROMPT_CACHING_1H=1 "; else rpfx+="FORCE_PROMPT_CACHING_5M=1 "; fi   # env ARGUMENT after its own -u (a shell prefix would be re-unset)
    [[ "$ra" == 4 ]] && rpfx+="${(j: :)${(@q)CC4_ENV}} "   # resume under account 4 → answer from GPT, not a dud Anthropic token
    local rflags=" ${(j: :)${(@q)CC_AUTONOMY_FLAGS}}"   # a resumed chat keeps full autonomy, every account
    _cc_solo "$uuid"                          # one instance per chat — close any other host first
    _cc_in_bunker && TMUX= exec tmux -L "$rs" new-session -s "$rs" -c "$rcwd" \
      "${rpfx}claude --resume ${(q)uuid}${rflags} || { echo; echo 'resume refused — session is live elsewhere:'; ${c1henv}bash ${(q)CC_FLEET_HOME}/cc-agent-open.sh ${(q)uuid} ${(q)rcwd}; }"
    TMUX= tmux -L "$rs" new-session -s "$rs" -c "$rcwd" \
      "${rpfx}claude --resume ${(q)uuid}${rflags} || { echo; echo 'resume refused — session is live elsewhere:'; ${c1henv}bash ${(q)CC_FLEET_HOME}/cc-agent-open.sh ${(q)uuid} ${(q)rcwd}; }"
  fi
}

