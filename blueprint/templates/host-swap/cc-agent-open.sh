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
# Default: takeover when the agent is idle/blocked/done · attach when it is actively working.
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

cc() {  # run claude under a config dir ("" = the default account / unset)
  local cfg="$1"; shift
  if [ -n "$cfg" ]; then CLAUDE_CONFIG_DIR="$cfg" claude "$@"
  else env -u CLAUDE_CONFIG_DIR claude "$@"; fi
}

# find this session in the agent registry (owning account if known, else every configured account).
# Match .sessionId OR the short .id prefix — the registry shows the RESUMED transcript's id
# for forked sessions, while cc-ls keys by transcript filename.
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
st="$(printf '%s' "$hit" | jq -r '.state // .status // "unknown"')"
oacct="$(_acct_of "$hitcfg")"
def=t; case "$st" in working|busy) def=a ;; esac   # never default to killing in-flight work

echo "⚙ $name — live background agent on account $oacct · state: $st"
echo
echo "  [t] take over — stop the agent, reopen this chat fresh under account $prim$( [ "$def" = t ] && echo '   (default)')"
echo "  [a] attach — join the running agent (keeps its original account/model/permissions)$( [ "$def" = a ] && echo '   (default)')"
echo "  [q] quit"
echo
read -r -n1 -p "choice ❯ " ch; echo
[ -z "$ch" ] && ch="$def"

case "$ch" in
  t|T)
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      echo "stopping agent pid $pid (graceful)…"
      kill -TERM "$pid" 2>/dev/null
      for _ in $(seq 1 15); do kill -0 "$pid" 2>/dev/null || break; sleep 1; done
      kill -0 "$pid" 2>/dev/null && { echo "still up after 15s — SIGKILL"; kill -KILL "$pid" 2>/dev/null; sleep 1; }
    else
      echo "registry entry has no live process (stale) — nothing to stop"
    fi
    resume_fresh
    ;;
  a|A) cc "$hitcfg" agents --cwd "$cwd"; exit $? ;;
  *)   echo "cancelled"; exit 0 ;;
esac
