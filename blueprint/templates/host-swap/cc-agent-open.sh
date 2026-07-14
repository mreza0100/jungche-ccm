#!/usr/bin/env bash
# cc-agent-open.sh <uuid> [cwd] [owning-config-dir] — open a chat whose session is LIVE as a
# Claude Code background agent ("agent mode": hosted headlessly by `claude daemon`, listed in
# `claude agents`). Called by cc-ls inside the fresh tmux window for ⚙ rows and resume-refusals.
#
# A daemon-hosted session refuses a plain `claude --resume`. Two ways in:
#   takeover — stop the agent gracefully, then resume the SAME transcript fresh here, under the
#              CURRENT primary account. The daily driver after cc-swap: a live agent keeps the
#              account/model/effort/permission-mode it was BORN with — only a fresh process
#              picks up the new primary.
#   attach   — connect to the running process via the agent view (keeps its original everything).
# Default: takeover unless the agent is BUSY (computing right now) · busy → attach.
# A lock-holder that lives inside a cc-* tmux is attached via its own window instead.
#
# N-account generic: the current primary is read from ~/.claude-primary (number N → ~/.claudeN;
# 1 → the unset default ~/.claude). The registry is probed across the default account plus every
# ~/.claude[0-9]* config dir found on disk (the same convention cc-reap's busy-scan uses).
set -u
u="${1:?usage: cc-agent-open.sh <uuid> [cwd] [owning-config-dir]}"
cwd="${2:-$PWD}"; owncfg="${3:-}"

# config dir for account N: 1 (or unset) → "" (the default ~/.claude); else ~/.claudeN
_cfg_of() { case "$1" in 1|"") echo "" ;; *) echo "$HOME/.claude$1" ;; esac; }
# account number for a config dir: "" / ~/.claude → 1; else the trailing digits of ~/.claudeN
_acct_of() { case "$1" in ""|"$HOME/.claude") echo 1 ;; *) local n="${1##*/.claude}"; echo "${n:-1}" ;; esac; }

prim=1; [ -f "$HOME/.claude-primary" ] && prim="$(cat "$HOME/.claude-primary")"
pcfg="$(_cfg_of "$prim")"

cc() {  # run claude under a config dir ("" = the default account / unset); never inherit a host chat's identity
  local cfg="$1"; shift
  if [ -n "$cfg" ]; then env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE CLAUDE_CONFIG_DIR="$cfg" claude "$@"
  else env -u CLAUDE_CODE_SESSION_ID -u CLAUDECODE -u CLAUDE_CONFIG_DIR claude "$@"; fi
}

# find this session in the agent registry (owning account if known, else every configured account).
# Match .sessionId OR the short .id prefix — the registry shows the RESUMED transcript's id
# for forked sessions, while cc-ls keys by transcript filename.
# jq gotcha: inside `$u|startswith(...)` the input is $u (a string), so the row's .id must be
# captured as a variable BEFORE the pipe — bare `.id` there indexes the string and jq aborts
# the whole stream, hiding every row after the first id-bearing one.
hit=""; hitcfg=""
if [ -n "$owncfg" ]; then
  [ "$owncfg" = "$HOME/.claude" ] && owncfg=""   # normalize the default account to ""
  cands=("$owncfg")
else
  cands=("")                                          # account 1 / the unset default…
  for d in "$HOME"/.claude[0-9]*; do [ -d "$d" ] && cands+=("$d"); done   # …plus every extra ~/.claudeN
fi
for cfg in "${cands[@]}"; do
  # judge by parseable output, not exit code — a registry that answers is a registry that counts
  j="$(timeout 20 bash -c '
    if [ -n "$1" ]; then CLAUDE_CONFIG_DIR="$1" claude agents --json
    else env -u CLAUDE_CONFIG_DIR claude agents --json; fi' _ "$cfg" 2>/dev/null)"
  row="$(printf '%s' "$j" | jq -c --arg u "$u" \
    '.[] | objects | select((.sessionId==$u) or (.id!=null and (.id as $i | $u|startswith($i))))' 2>/dev/null | head -1)"
  [ -n "$row" ] && { hit="$row"; hitcfg="$cfg"; break; }
done

resume_fresh() {
  cc "$pcfg" --resume "$u" && exit 0
  echo; echo "resume still refused — falling back to the agent view (pick the session to attach):"
  cc "${hitcfg:-$pcfg}" agents --cwd "$cwd"; exit $?
}

if [ -z "$hit" ]; then   # not in any registry (stale ⚙ label) → plain fresh resume
  echo "no live agent found for $u — resuming fresh (account $prim)"
  resume_fresh
fi

name="$(printf '%s' "$hit" | jq -r '.name // "(unnamed)"')"
pid="$(printf '%s' "$hit" | jq -r '.pid // empty')"
# LIVE activity (.status: busy/idle/waiting) outranks task lifecycle (.state: working/blocked/
# done) — a wave agent between turns reads idle+working, and only busy means "computing right
# now". Routing on .state sent idle agents to the attach view, where picking them wedges.
st="$(printf '%s' "$hit" | jq -r '.status // .state // "unknown"')"
oacct="$(_acct_of "$hitcfg")"

# a lock-holder living inside a cc-* tmux is just a CHAT whose breadcrumb went missing (statusline
# never rendered) — attach its own window; the agents view can't reach it and the resume refuses
if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
  tsock="$(tr '\0' '\n' < "/proc/$pid/environ" 2>/dev/null | sed -n 's/^TMUX=//p' | head -1)"
  tsock="${tsock%%,*}"; tsock="${tsock##*/}"
  case "$tsock" in
    cc-*)
      echo "⚙ $name is a tmux-resident chat on $tsock — attaching its window"
      tmux -L "$tsock" set -g window-size latest 2>/dev/null
      exec env -u TMUX tmux -L "$tsock" attach
      ;;
  esac
fi

# zero-question routing — Enter in cc-ls must land INSIDE the chat:
#   busy (computing right now) → attach (never kill in-flight work; pick "$name" in the view)
#   idle/waiting/blocked/done/stale → silent takeover: stop the agent, resume fresh under the primary
case "$st" in
  busy)
    echo "⚙ $name (account $oacct) is BUSY — attaching. Pick '$name' in the view; ⌃C detaches."
    cc "$hitcfg" agents --cwd "$cwd"; exit $?
    ;;
  *)
    echo "⚙ $name (account $oacct, state: $st) — taking over → fresh resume under account $prim"
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      kill -TERM "$pid" 2>/dev/null
      for _ in $(seq 1 15); do kill -0 "$pid" 2>/dev/null || break; sleep 1; done
      kill -0 "$pid" 2>/dev/null && { kill -KILL "$pid" 2>/dev/null; sleep 1; }
    fi
    resume_fresh
    ;;
esac
