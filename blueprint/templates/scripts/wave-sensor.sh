#!/usr/bin/env bash
set -euo pipefail

# wave-sensor.sh — collector-tier watchdog for /wave:orchestrator (§ O4).
# A detached loop that watches the BUILDER chat's tmux pane and escalates
# actionable states to the ORCHESTRATOR chat via chat.sh inject. It CLASSIFIES,
# NEVER RULES: no wave-file reads, no verdicts, no retries of builder work —
# every consequential action stays in the orchestrator chat.
#
# Usage: wave-sensor.sh <builder-tmux-session> <orchestrator-tmux-session> [interval-seconds]
#   interval default 600 (10 min — the orchestrator runs 1h prompt cache
#   (ENABLE_PROMPT_CACHING_1H, set at orchestrator chat launch), so wakes stay cache-warm).
#
# Detection (regex-first, zero tokens — mirrors orchestrator.md § O4):
#   GOAL-DROPPED  — the `◎ /goal active` statusline mark is absent from the pane tail.
#   IDLE          — bare `❯` prompt, no in-progress spinner. File counts, background
#                   agents, `· N shell` are NOT activity.
#   FINISHED-TASK — a finished-turn banner (`✻ <word> for Ns` — no paren-timer, no
#                   token counter) above an idle prompt.
#   FROZEN        — identical statusline fingerprint across 2 consecutive ticks
#                   while `· N shell` shows (the shell may have EXITED).
#   LONG-TURN     — the turn-timer climbing across >= 4 consecutive busy ticks.
#   CTX           — context % >= 70 (latched; re-escalates only on +5% growth).
#   LIMITS        — a statusline account gauge `5h-used:NN%`/`7d-used:NN%` >= 80
#                   (latched; re-escalates only on +5% growth).
#   WORKING       — a BUSY pane while mode=idle-sanctioned (idle is by design there;
#                   work is the incident).
#   DEAD          — no live pane for the builder session.
# BUSY is the clear tick: no inject, sleep, loop. Ambiguous captures ONLY go to a
# one-shot `claude -p --model haiku` returning one word (BUSY|IDLE|FINISHED-TASK|FROZEN);
# a failed classify is logged and treated BUSY (FROZEN/DEAD nets still cover a stuck pane).
#
# Orchestrator-side control files (levers — no chat round-trip):
#   tmp/wave-sensor/<builder>.mode    — `active` (default) | `idle-sanctioned`
#                                       (sanctioned: IDLE/FINISHED-TASK suppressed).
#   tmp/wave-sensor/<builder>.snooze  — integer: suppress IDLE/FINISHED-TASK for n ticks.
# Tick log:  tmp/wave-sensor/<builder>.log
# Heartbeat: tmp/wave-sensor/<builder>.heartbeat (touched every tick — the
#            orchestrator's liveness check; stale >25 min means relaunch).

BUILDER="${1:?usage: wave-sensor.sh <builder-session> <orchestrator-session> [interval]}"
ORCH="${2:?usage: wave-sensor.sh <builder-session> <orchestrator-session> [interval]}"
# Default tick assumes the orchestrator chat was launched with 1h prompt cache
# (ENABLE_PROMPT_CACHING_1H=1 — orchestrator-only; NEVER project-wide, write-heavy
# chats pay 2x cache-writes). Env unset → 270s so wakes stay inside the 5m TTL.
if [[ -n "${3:-}" ]]; then INTERVAL="$3"
elif [[ "${ENABLE_PROMPT_CACHING_1H:-}" == "1" ]]; then INTERVAL=600
else INTERVAL=270; fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHAT="$ROOT/.claude/commands/chat/chat.sh"
DIR="$ROOT/tmp/wave-sensor"
mkdir -p "$DIR"
LOG="$DIR/$BUILDER.log"
HB="$DIR/$BUILDER.heartbeat"
MODE_F="$DIR/$BUILDER.mode"
SNOOZE_F="$DIR/$BUILDER.snooze"
STATE_F="$DIR/$BUILDER.state"

# Register the orchestrator session — limits-hook.sh scopes itself to this file.
printf '%s' "$ORCH" > "$DIR/$BUILDER.orch"

# Harvested rate-limit file (written by statusline-command.sh, keyed by account;
# inherited CLAUDE_CONFIG_DIR = the launching orchestrator's account = the builder's).
# Single-account setups always resolve to acct-1. Multi-account setups mirror the
# statusline badge by writing the account number (1|2|3) into "$CLAUDE_CONFIG_DIR/account".
_acct=1
_cfg="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
if [[ -f "$_cfg/account" ]]; then
  read -r _acct _ < "$_cfg/account" || _acct=1
fi
case "$_acct" in 1|2|3) ;; *) _acct=1 ;; esac
RL_PREFIX="/tmp/cc-rate-limits/acct-${_acct}."

log() { printf '%s %s\n' "$(date '+%F %T')" "$*" >> "$LOG"; }

# Scratch persisted across ticks (restart-safe): fingerprint, streaks, latches.
prev_fp=""; frozen_hits=0; busy_streak=0; prev_timer=-1
ctx_latch=0; limits_latch=0; last_escalated=""; dead_reported=0; longturn_fired=0
# shellcheck disable=SC1090
if [[ -f "$STATE_F" ]]; then source "$STATE_F" || true; fi

save_state() {
  printf 'prev_fp=%q\nfrozen_hits=%q\nbusy_streak=%q\nprev_timer=%q\nctx_latch=%q\nlimits_latch=%q\nlast_escalated=%q\ndead_reported=%q\nlongturn_fired=%q\n' \
    "$prev_fp" "$frozen_hits" "$busy_streak" "$prev_timer" "$ctx_latch" "$limits_latch" \
    "$last_escalated" "$dead_reported" "$longturn_fired" > "$STATE_F"
}

escalate() { # $1 = space-joined state set, $2 = statusline payload
  "$CHAT" inject "$ORCH" "sensor $1: $2" >/dev/null 2>&1 || log "inject FAILED ($1)"
}

log "sensor up — builder=$BUILDER orch=$ORCH interval=${INTERVAL}s pid=$$"

while :; do
  touch "$HB"
  states=()

  if ! pane="$("$CHAT" capture "$BUILDER" 2>/dev/null | tail -60)"; then
    if [[ "$dead_reported" != 1 ]]; then
      escalate "DEAD" "no live pane for $BUILDER"
      dead_reported=1; log "tick DEAD — escalated"
    else
      log "tick DEAD — already escalated"
    fi
    save_state; sleep "$INTERVAL"; continue
  fi
  dead_reported=0

  tail_txt="$(printf '%s\n' "$pane" | grep -v '^[[:space:]]*$' | tail -40 || true)"
  statusline="$(printf '%s\n' "$tail_txt" | grep -E '🔖|🌿' | tail -1 || true)"
  [[ -n "$statusline" ]] || statusline="$(printf '%s\n' "$tail_txt" | tail -1)"
  fp="$statusline"

  mode="active"; [[ -f "$MODE_F" ]] && mode="$(cat "$MODE_F" 2>/dev/null || echo active)"
  snooze=0; [[ -f "$SNOOZE_F" ]] && snooze="$(cat "$SNOOZE_F" 2>/dev/null || echo 0)"
  [[ "$snooze" =~ ^[0-9]+$ ]] || snooze=0

  # -- main-loop state: busy / idle / finished-task / ambiguous (regex-first) --
  busy=0; idle=0; finished=0
  if printf '%s\n' "$tail_txt" | grep -qiE 'esc to interrupt|\([0-9]+s ·|· [0-9]+s|[0-9]+ tokens'; then
    busy=1
  elif printf '%s\n' "$tail_txt" | grep -qE '^[[:space:]]*(│[[:space:]]*)?❯'; then
    idle=1
    printf '%s\n' "$tail_txt" | grep -qE '✻ .+ for [0-9]+m?s' && finished=1
  else
    # Ambiguous capture ONLY → one-shot collector-tier classify (one word back).
    word="$(timeout 90 claude -p --model haiku "Classify this Claude Code tmux pane tail. Reply EXACTLY one word — BUSY (a turn is actively generating/working), IDLE (turn ended, prompt waiting), FINISHED-TASK (a finished-turn banner above an idle prompt), or FROZEN (looks mid-turn but visibly stuck). Pane tail:
$tail_txt" 2>/dev/null | tr -d '[:space:]' | tr '[:lower:]' '[:upper:]' || true)"
    case "$word" in
      IDLE) idle=1 ;;
      FINISHED-TASK|FINISHEDTASK) idle=1; finished=1 ;;
      FROZEN) states+=("FROZEN") ;;
      BUSY) busy=1 ;;
      *) busy=1; log "ambiguous classify failed (got '${word:-}') — treating BUSY" ;;
    esac
  fi

  # -- goal marker (statusline `◎ /goal active`) --
  printf '%s\n' "$tail_txt" | grep -q '◎' || states+=("GOAL-DROPPED")

  # -- frozen: same fingerprint 2 consecutive ticks while `· N shell` shows --
  if [[ $busy -eq 1 && "$fp" == "$prev_fp" && -n "$fp" ]] \
     && printf '%s\n' "$tail_txt" | grep -qE '· [0-9]+ shell'; then
    frozen_hits=$((frozen_hits + 1))
    [[ $frozen_hits -ge 1 ]] && states+=("FROZEN")
  else
    frozen_hits=0
  fi

  # -- long-turn: turn-timer climbing across >= 4 consecutive busy ticks --
  timer="$(printf '%s\n' "$tail_txt" | grep -oE '[0-9]+s' | tail -1 | tr -d 's' || true)"
  [[ "$timer" =~ ^[0-9]+$ ]] || timer=-1
  if [[ $busy -eq 1 ]]; then
    busy_streak=$((busy_streak + 1))
    if [[ $busy_streak -ge 4 && $timer -gt $prev_timer && $prev_timer -ge 0 && $longturn_fired -eq 0 ]]; then
      states+=("LONG-TURN"); longturn_fired=1
    fi
  else
    busy_streak=0; longturn_fired=0
  fi
  prev_timer=$timer; prev_fp="$fp"

  # -- context %: >= 70 latched, re-escalate on +5% growth --
  pct="$(printf '%s\n' "$statusline" | grep -oE '[0-9]+%' | tail -1 | tr -d '%' || true)"
  if [[ "$pct" =~ ^[0-9]+$ && $pct -ge 70 ]]; then
    if [[ $ctx_latch -eq 0 || $pct -ge $((ctx_latch + 5)) ]]; then
      states+=("CTX"); ctx_latch=$pct
    fi
  fi

  # -- account limits: harvested per-session JSONs first (exact, modal-immune) — max
  #    across fresh window-alive files (sessions carry different snapshot vintages;
  #    max errs toward pausing), pane-regex fallback when none. >= 80 latched. --
  lim=""
  rl_files=( "${RL_PREFIX}"*.json )
  if [[ ${#rl_files[@]} -gt 0 && -f "${rl_files[0]}" ]]; then
    rl="$(jq -rs '[.[] | select((now - (.ts // 0)) < 900 and ((.five_hour_resets_at // 0) > now))] | if length == 0 then empty else ([.[].five_hour_used, .[].seven_day_used] | max) end' "${rl_files[@]}" 2>/dev/null || true)"
    [[ "$rl" =~ ^[0-9]+$ ]] && lim="$rl"
  fi
  [[ -n "$lim" ]] || lim="$(printf '%s\n' "$tail_txt" | grep -oE '(5h|7d)-used:[0-9]+%' | grep -oE '[0-9]+' | sort -n | tail -1 || true)"
  if [[ "$lim" =~ ^[0-9]+$ && $lim -ge 80 ]]; then
    if [[ $limits_latch -eq 0 || $lim -ge $((limits_latch + 5)) ]]; then
      states+=("LIMITS"); limits_latch=$lim
    fi
  fi

  # -- idle polarity: mode + snooze govern escalation, never detection --
  if [[ $idle -eq 1 ]]; then
    if [[ "$mode" == "idle-sanctioned" ]]; then
      : # sanctioned idle is by design — silent
    elif [[ $snooze -gt 0 ]]; then
      echo $((snooze - 1)) > "$SNOOZE_F"
    elif [[ $finished -eq 1 ]]; then
      states+=("FINISHED-TASK")
    else
      states+=("IDLE")
    fi
  elif [[ $busy -eq 1 && "$mode" == "idle-sanctioned" ]]; then
    states+=("WORKING")
  fi

  # -- one inject per tick, all current states batched; repeat sets stay quiet --
  if [[ ${#states[@]} -gt 0 ]]; then
    set_str="${states[*]}"
    if [[ "$set_str" == "$last_escalated" ]]; then
      log "tick $set_str — unchanged, holding (orchestrator already pinged)"
    else
      escalate "$set_str" "$statusline"
      last_escalated="$set_str"
      log "tick $set_str — escalated"
    fi
  else
    last_escalated=""
    log "tick clear (busy=$busy idle=$idle mode=$mode snooze=$snooze ctx=${pct:-?})"
  fi

  save_state
  sleep "$INTERVAL"
done
