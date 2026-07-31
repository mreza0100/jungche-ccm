# ── Claude Code: multi-account per-chat billing swap ─────────────────────────
# Masters: account 1 "work"     → ~/.claude   (CLAUDE_CONFIG_DIR unset — Claude's own default)
#          account 2 "personal" → ~/.claude2
#          account 3 "third"    → ~/.claude3  (optional — this reads correctly with just 1 and 2;
#                                               drop every "3" line below if you only use two)
# One /login each, ever — no credential copying (a copied OAuth token forks and dies). Each
# account IS its own config dir directly — nothing forks a per-launch session dir, so there is
# nothing to prune later.
#
# Every launch gets its OWN tmux server (-L cc-<epoch>-<pid>-<rand>) so a single crashed tmux
# can't take every chat down at once, and so a chat picker / the /swap command can address one
# chat's pane precisely instead of guessing which pane in a shared server is which chat.
#
# Marker file ~/.claude-primary holds "1".."3" (default "1"). cc-swap opens an fzf picker
# (pointer starts on the next account, so plain Enter = old cycle); cc-swap <1|2|3> jumps
# without the menu — this only changes which account a FUTURE bare `cc` opens. To move an
# ALREADY-RUNNING chat to another account (or flip its cache-window mode) without losing the
# conversation, use /swap from inside the chat (blueprint/templates/host-swap/swap.command.md)
# — it reboots that one chat in place; see that command file for the install note.
# Commands: cc (tmux + primary), cc1/cc2/cc3 (tmux + that account).
#
# Linux and macOS — plain config-dir files, no Keychain/credential-store dependency.
# ─────────────────────────────────────────────────────────────────────────────

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

# ── cache-window flag (⚡1h vs 🪫5m) ────────────────────────────────────────
# _cc_arm1h — 1 when THIS launch should be born with the ⚡1h prompt-cache TTL: CC_ARM_1H=1 (an
# explicit per-launch verdict, e.g. from a picker) or ENABLE_PROMPT_CACHING_1H=1 set from a
# NON-chat shell (the documented `ENABLE_PROMPT_CACHING_1H=1 cc` interface). Inside a chat's own
# tool shell (CLAUDECODE set) an inherited flag is the HOST chat's birth env leaking downstream —
# it must never arm a new launch on its own. The harness defaults every session to the 1h
# window; an un-armed launch must actively set FORCE_PROMPT_CACHING_5M=1 to get 5m instead.
_cc_arm1h() {
  if [[ "${CC_ARM_1H:-}" == 1 ]]; then echo 1
  elif [[ "${ENABLE_PROMPT_CACHING_1H:-}" == 1 && -z "${CLAUDECODE:-}" ]]; then echo 1
  else echo 0; fi
}

_cc_primary() {
  local n="1"
  [[ -f "$HOME/.claude-primary" ]] && n="$(<$HOME/.claude-primary)"
  case "$n" in 1|2|3) ;; *) n=1 ;; esac
  echo "$n"
}

# config dir for account N: 1 (or unset) → "" (the default ~/.claude); else ~/.claudeN
_cc_cfgdir() { case "$1" in 1|"") echo "" ;; *) echo "$HOME/.claude$1" ;; esac }

# _cc_run <account-num: 1|2|3> <tmux: 0|1> [claude args...]
_cc_run() {
  local acct="$1" use_tmux="$2"; shift 2
  local cfg; cfg="$(_cc_cfgdir "$acct")"
  local in_tmux=0; [[ -n "${TMUX:-}" ]] && in_tmux=1
  # ⚡1h-cache is per-launch, NEVER sticky (2× write premium must be a deliberate choice each
  # time) — _cc_arm1h decides; env -u strips a leaked flag first, and an armed launch re-adds
  # it as an env ARGUMENT (a shell prefix would be re-unset by its own -u below)
  local arm1h; arm1h="$(_cc_arm1h)"

  # explicit env, always: a launch from INSIDE a chat (Bash tool, nested shell) inherits the
  # host chat's CLAUDE_CONFIG_DIR / session identity / cache mode — env -u makes every one of
  # them THIS launch's own verdict, never the environment's
  local -a envargs=(-u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u ENABLE_PROMPT_CACHING_1H -u FORCE_PROMPT_CACHING_5M)
  if [[ -n "$cfg" ]]; then envargs+=(CLAUDE_CONFIG_DIR="$cfg"); else envargs+=(-u CLAUDE_CONFIG_DIR); fi
  if [[ "$arm1h" == 1 ]]; then envargs+=(ENABLE_PROMPT_CACHING_1H=1); else envargs+=(FORCE_PROMPT_CACHING_5M=1); fi

  if [[ "$use_tmux" == "1" ]]; then
    # Each chat gets its OWN tmux server (unique socket == session name) so a single tmux
    # crash can no longer take down every chat at once. The globally-unique session name
    # preserves the "address a chat by its tmux session name" handle other fleet tooling
    # (a chat picker, /bb, /swap) relies on. Inside a tmux already, the chat STILL gets its
    # own server — the current pane just becomes a nested client viewing it.
    local sock="cc-$(date +%s)-$$-$RANDOM"
    # per-arg quoting via printf %q (bash/zsh builtin) so a multi-word claude argument (e.g.
    # `cc --model x`) survives being flattened into tmux's one shell-command string.
    local -a q=() a
    for a in "${envargs[@]}" claude "$@"; do q+=("$(printf '%q' "$a")"); done
    local run="env ${q[*]}"
    if (( ! in_tmux )); then tmux -L "$sock" new-session -s "$sock" "$run"
    else TMUX= tmux -L "$sock" new-session -s "$sock" "$run"
    fi
  else
    env "${envargs[@]}" claude "$@"
  fi
}

# Guard: if the caller already defines `cc`, leave it untouched.
if ! typeset -f cc > /dev/null 2>&1; then
  cc()  { _cc_run "$(_cc_primary)" 1 "$@"; }                                  # tmux + primary
fi

cc1() { _cc_run 1 1 "$@"; }   # tmux + account 1
cc2() { _cc_run 2 1 "$@"; }   # tmux + account 2
cc3() { _cc_run 3 1 "$@"; }   # tmux + account 3 (delete this line if you only use 2 accounts)

# pretty label per account: medal + the real email pulled from its config dir — nothing to
# keep in sync by hand when an account's login changes.
_cc_label() {
  local n="$1" dir medal email
  case "$n" in
    1) dir="$HOME/.claude";   medal="🥇" ;;
    2) dir="$HOME/.claude2";  medal="🥈" ;;
    3) dir="$HOME/.claude3";  medal="🥉" ;;
    *) dir="$HOME/.claude$n"; medal="●" ;;
  esac
  email="$(jq -r '.oauthAccount.emailAddress // empty' "$dir/.claude.json" 2>/dev/null)"
  [[ -z "$email" ]] && email="(not logged in — run cc${n} then /login)"
  printf '%s\n' "$medal $email"
}

# cc-swap [1|2|3] — fzf picker (no arg) to set which account bare `cc` opens
cc-swap() {
  local cur n; cur="$(_cc_primary)"

  # ── EDIT: which account numbers you actually use (2 accounts works fine — just drop the "3") ──
  local -a accts=(1 2 3)
  # ── END EDIT ─────────────────────────────────────────────────────────────

  local -a rows=() ; local a
  for a in "${accts[@]}"; do rows+=("$a │ $(_cc_label "$a")"); done

  if [[ "${1:-}" =~ ^[0-9]+$ ]] && [[ " ${accts[*]} " == *" ${1} "* ]]; then
    n="$1"
  elif command -v fzf >/dev/null; then
    local curi=1 i=0 x next
    for x in "${accts[@]}"; do i=$((i+1)); [[ "$x" == "$cur" ]] && curi=$i; done
    next=$(( curi % ${#accts[@]} + 1 ))
    rows[$curi]="${rows[$curi]}  ← current"
    local pick
    pick="$(printf '%s\n' "${rows[@]}" | fzf \
      --height=~$(( ${#accts[@]} + 3 )) --reverse --cycle --no-info --header-first \
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
  echo "Primary → account $n  ($(_cc_label "$n"))"
  echo "  cc          → account $n"
  echo "  cc1/cc2/cc3 → explicit account"
}
# ── end multi-account swap ────────────────────────────────────────────────────
