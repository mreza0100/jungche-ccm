#!/usr/bin/env bash
# limits-hook.sh — UserPromptSubmit hook: whispers the account rate-limit gauges into
# the WAVE ORCHESTRATOR's turns when either window is >= 80% used, so the GENTLE PAUSE
# ruling never depends on a pane capture.
#
# Scope — ORCHESTRATOR ONLY: fires only when THIS session's tmux name matches a live
# wave-sensor registration (tmp/wave-sensor/*.orch, written by wave-sensor.sh at start).
# Every other session, every sub-80% read, a stale harvest (>15 min), or any resolve
# failure → silent exit 0. A hook must never block or noise a prompt.
#
# Data source: /tmp/cc-rate-limits/acct-{n}.*.json — harvested by statusline-command.sh
# from the stdin JSON (the only carrier of .rate_limits). Same account resolution as
# the statusline badge.
set -uo pipefail

# 1) This session's tmux identity; not in tmux → not a wave chat.
[ -n "${TMUX:-}" ] || exit 0
self="$(tmux -S "${TMUX%%,*}" display-message -p '#S' 2>/dev/null || true)"
[ -n "$self" ] || exit 0

# 2) Orchestrator check against the sensor registrations.
root="${CLAUDE_PROJECT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
match=0
for f in "$root"/tmp/wave-sensor/*.orch; do
  [ -f "$f" ] || continue
  [ "$(cat "$f" 2>/dev/null)" = "$self" ] && { match=1; break; }
done
[ "$match" = 1 ] || exit 0

# 3) Read the harvested gauges. Single-account setups always resolve to acct-1.
#    Multi-account setups mirror the statusline badge by writing the account number
#    (1|2|3) into "$CLAUDE_CONFIG_DIR/account".
acct=1
cfgdir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
if [ -f "$cfgdir/account" ]; then
  read -r acct _ < "$cfgdir/account" || acct=1
fi
case "$acct" in 1|2|3) ;; *) acct=1 ;; esac
rl_files=( /tmp/cc-rate-limits/acct-${acct}.*.json )
{ [ ${#rl_files[@]} -gt 0 ] && [ -f "${rl_files[0]}" ]; } || exit 0
# Max across fresh, window-alive per-session files — sessions carry different
# snapshot vintages of one account; max errs toward pausing, never under-reads.
read -r h5 d7 h5r d7r ts < <(jq -rs '
  [.[] | select((now - (.ts // 0)) < 900 and ((.five_hour_resets_at // 0) > now))]
  | if length == 0 then empty else
      [([.[].five_hour_used] | max), ([.[].seven_day_used] | max),
       ([.[].five_hour_resets_at] | max), ([.[].seven_day_resets_at] | max),
       ([.[].ts] | max)] | map(tostring) | join(" ")
    end' "${rl_files[@]}" 2>/dev/null) || exit 0
for v in "${h5:-x}" "${d7:-x}" "${h5r:-x}" "${d7r:-x}" "${ts:-x}"; do
  case "$v" in '' | *[!0-9]*) exit 0 ;; esac
done
now=$(date +%s)
(( h5 >= 80 || d7 >= 80 )) || exit 0  # below threshold → silent

fmt() { local s=$(( $1 - now )); if (( s <= 0 )); then echo "now"; elif (( s >= 86400 )); then echo "$((s/86400))d$(( (s%86400)/3600 ))h"; else echo "$((s/3600))h$(( (s%3600)/60 ))m"; fi; }
echo "⚠ ACCOUNT LIMITS ≥80% (harvested $(( now - ts ))s ago): 5h-used:${h5}% (resets in $(fmt "$h5r")) · 7d-used:${d7}% (resets in $(fmt "$d7r")) — GENTLE PAUSE law applies (orchestrator.md § O4 LIMITS)."
exit 0
