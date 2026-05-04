#!/usr/bin/env bash
# Jungche statusline — best-of-breed synthesis from 13+ community projects
# Techniques: daniel3303 (jq/IFS), claudeline (escalation), Mohamed3on (symbols),
#   vtmocanu (git cache), wmoto-ai (rate limits), ccstatusline (reset trick)
#
# Install: copy to ~/.claude/statusline-command.sh
# Config:  add to ~/.claude/settings.json:
#   "statusLine": {
#     "type": "command",
#     "command": "bash ~/.claude/statusline-command.sh",
#     "padding": 0,
#     "refreshInterval": 10,
#     "hideVimModeIndicator": true
#   }
set -euo pipefail

input=$(cat)

# ── Colors (ANSI 16 + bold, combined sequences — max CC compatibility) ──
# Truecolor (\033[38;2;R;G;Bm) broken since CC v2.1.78 — avoid
# Nerd Font PUA glyphs broken in CC UI — use emoji + standard Unicode
G=$'\033[1;32m' Y=$'\033[1;33m' R=$'\033[1;31m'
C=$'\033[1;36m' B=$'\033[1;34m' M=$'\033[1;35m'
D=$'\033[2m'    W=$'\033[1;37m' X=$'\033[0m'
SEP=" ${D}│${X} "

# ── JSON (single jq, unit-separator IFS — one subprocess, all fields) ──
IFS=$'\x1f' read -r MODEL DIR PCT COST DUR VIM AGENT WT GWT \
  HR5 D7 HR5R LADD LDEL STYLE < <(
  echo "$input" | jq -r '[
    (.model.display_name // "Claude"),
    (.workspace.current_dir // .cwd // ""),
    ((.context_window.used_percentage // 0) | floor | tostring),
    (.cost.total_cost_usd // 0 | tostring),
    (.cost.total_duration_ms // 0 | tostring),
    (.vim.mode // ""),
    (.agent.name // ""),
    (.worktree.name // ""),
    (.workspace.git_worktree // ""),
    ((.rate_limits.five_hour.used_percentage // 0) | floor | tostring),
    ((.rate_limits.seven_day.used_percentage // 0) | floor | tostring),
    (.rate_limits.five_hour.resets_at // 0 | tostring),
    (.cost.total_lines_added // 0 | tostring),
    (.cost.total_lines_removed // 0 | tostring),
    (.output_style.name // "default")
  ] | join("")' 2>/dev/null
) || true

# ── Helpers ──────────────────────────────────────────────────────────
pc() {
  local p=${1:-0}
  (( p >= 80 )) && printf '%s' "$R" && return
  (( p >= 50 )) && printf '%s' "$Y" && return
  printf '%s' "$G"
}

mkbar() {
  local p=${1:-0} w=${2:-10} f=$(( ${1:-0} * ${2:-10} / 100 ))
  (( f > w )) && f=$w; (( f < 0 )) && f=0
  local e=$(( w - f )) b
  b="$(pc "$p")"
  for (( i = 0; i < f; i++ )); do b+="▓"; done
  b+="$D"
  for (( i = 0; i < e; i++ )); do b+="░"; done
  printf '%s%s' "$b" "$X"
}

# ── Computed values ──────────────────────────────────────────────────

# Model symbol (◆ Opus ◇ Sonnet ○ Haiku ● other)
case "$MODEL" in *Opus*) ms="◆";; *Sonnet*) ms="◇";; *Haiku*) ms="○";; *) ms="●";; esac

# Directory basename
dn="${DIR##*/}"; [ -z "$dn" ] && dn="~"

# Duration
ds=$(( ${DUR:-0} / 1000 ))
if   (( ds >= 3600 )); then df="$((ds/3600))h$((ds%3600/60))m"
elif (( ds >= 60 ));   then df="$((ds/60))m$((ds%60))s"
else df="${ds}s"; fi

# Context urgency emoji escalation (claudeline pattern)
ce="🟢"; (( ${PCT:-0} >= 50 )) && ce="⚡"; (( ${PCT:-0} >= 80 )) && ce="🔥"; (( ${PCT:-0} >= 95 )) && ce="🚨"

# ── Git (5s cache to avoid subprocess spam) ──────────────────────────
gitseg=""
if [ -n "${DIR:-}" ]; then
  ck=$(printf '%s' "$DIR" | md5 2>/dev/null || printf '%s' "$DIR" | md5sum 2>/dev/null | cut -d' ' -f1 || echo x)
  cf="/tmp/cc-sl-${ck}"
  now=$(date +%s)
  ca=999
  [ -f "$cf" ] && ca=$(( now - $(stat -f%m "$cf" 2>/dev/null || stat -c%Y "$cf" 2>/dev/null || echo 0) ))

  if (( ca > 5 )); then
    _gb=$(git -C "$DIR" --no-optional-locks symbolic-ref --short HEAD 2>/dev/null || echo "")
    if [ -n "$_gb" ]; then
      _gs=$(git -C "$DIR" --no-optional-locks diff --cached --numstat 2>/dev/null | wc -l | tr -d ' ')
      _gm=$(git -C "$DIR" --no-optional-locks diff --numstat 2>/dev/null | wc -l | tr -d ' ')
      printf '%s|%s|%s' "$_gb" "$_gs" "$_gm" > "$cf"
    else
      : > "$cf"
    fi
  fi

  gb="" gs=0 gm=0
  [ -f "$cf" ] && [ -s "$cf" ] && IFS='|' read -r gb gs gm < "$cf" 2>/dev/null || true

  if [ -n "${gb:-}" ]; then
    gc="$G" gx=""
    (( ${gs:-0} > 0 )) && gx+=" ${G}+${gs}" && gc="$Y"
    (( ${gm:-0} > 0 )) && gx+=" ${Y}~${gm}" && gc="$Y"
    gitseg="${gc}🌿 ${gb}${gx}${X}"
  fi
fi

# ── LINE 1: Identity ────────────────────────────────────────────────
l1="${C}${ms} ${MODEL}${X}${SEP}${B}${dn}${X}"
[ -n "${WT:-}" ]                          && l1+="${SEP}${M}🌳 ${WT}${X}"
[ -z "${WT:-}" ] && [ -n "${GWT:-}" ]     && l1+="${SEP}${M}🌳 ${GWT}${X}"
[ -n "$gitseg" ]                          && l1+="${SEP}${gitseg}"
[ -n "${AGENT:-}" ]                       && l1+="${SEP}${M}⚡${AGENT}${X}"
if [ -n "${VIM:-}" ]; then
  vc="$G"; [ "$VIM" = "NORMAL" ] && vc="$C"
  l1+="${SEP}${vc}${VIM}${X}"
fi

# ── LINE 2: Metrics ─────────────────────────────────────────────────
l2="${ce} $(mkbar "${PCT:-0}" 10) $(pc "${PCT:-0}")${PCT:-0}%${X}"

# Cost (float comparison via single awk call)
read -r c1 c2 c3 < <(awk "BEGIN{c=${COST:-0}; print (c>0)?1:0, (c>=2)?1:0, (c>=10)?1:0}" 2>/dev/null) || { c1=0; c2=0; c3=0; }
if [ "$c1" = "1" ]; then
  cc="$D"; [ "$c2" = "1" ] && cc="$Y"; [ "$c3" = "1" ] && cc="$R"
  l2+="${SEP}${cc}💰$(printf '$%.2f' "${COST:-0}")${X}"
fi

# Lines changed
(( ${LADD:-0} > 0 || ${LDEL:-0} > 0 )) && l2+="${SEP}${G}+${LADD:-0}${X} ${R}-${LDEL:-0}${X}"

# Duration
l2+="${SEP}${D}⏱ ${df}${X}"

# Rate limit (Pro/Max — direct from stdin JSON, no API call)
if (( ${HR5:-0} > 0 )); then
  l2+="${SEP}$(mkbar "$HR5" 5) $(pc "$HR5")5h:${HR5}%${X}"
  if (( ${HR5R:-0} > 0 )); then
    rem=$(( HR5R - $(date +%s) ))
    (( rem > 0 )) && l2+=" ${D}↻$((rem/3600))h$(((rem%3600)/60))m${X}"
  fi
fi

# ── Render (prepend reset to fight CC's dimColor wrapper) ────────────
printf '\033[0m%s\n\033[0m%s\n' "$l1" "$l2"
