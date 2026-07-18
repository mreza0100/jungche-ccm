#!/usr/bin/env bash
# Professor statusline — best-of-breed synthesis from 13+ community projects
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
  HR5 D7 HR5R LADD LDEL STYLE TOKIN TOKOUT EFFORT THINK \
  SESSNAME CACHER CACHEC CACHEI D7R SID TPATH < <(
  printf '%s' "$input" | jq -r '[
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
    (.output_style.name // "default"),
    (.context_window.total_input_tokens // 0 | tostring),
    (.context_window.total_output_tokens // 0 | tostring),
    (.effort.level // ""),
    (.thinking.enabled // false | tostring),
    (.session_name // ""),
    (.context_window.current_usage.cache_read_input_tokens // 0 | tostring),
    (.context_window.current_usage.cache_creation_input_tokens // 0 | tostring),
    (.context_window.current_usage.input_tokens // 0 | tostring),
    (.rate_limits.seven_day.resets_at // 0 | tostring),
    (.session_id // ""),
    (.transcript_path // "")
  ] | join("")' 2>/dev/null
) || true

# ── socket → transcript-path map (lets the cc-ls fleet picker size each chat; path,
#    not id, since a resumed/bridged session's id ≠ its transcript filename). Opt-in
#    by nature — a no-op unless you're inside tmux, so it's safe to leave in place
#    even if you never install the host-swap fleet tooling. ──
if [ -n "${TMUX:-}" ] && [ -n "${TPATH:-}" ]; then
  _sock="${TMUX%%,*}"; _sock="${_sock##*/}"
  # 700: breadcrumbs are transcript paths and the namecache carries prompt text — not for other uids
  { mkdir -p /tmp/cc-sid && chmod 700 /tmp/cc-sid && printf '%s' "$TPATH" > "/tmp/cc-sid/${_sock}"; } 2>/dev/null || true
  # pane-keyed breadcrumb too (<sock>.<pane_id>): several chats SPLIT in one window each get their
  # own map entry — cc-ls merges them into one row (the socket-keyed file above is last-writer-wins)
  [ -n "${TMUX_PANE:-}" ] && { printf '%s' "$TPATH" > "/tmp/cc-sid/${_sock}.${TMUX_PANE}"; } 2>/dev/null || true
fi

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

fmttok() {
  local t=${1:-0}
  if   (( t >= 1000000 )); then printf '%d.%dM' "$((t/1000000))" "$(((t%1000000)/100000))"
  elif (( t >= 1000 ));    then printf '%d.%dK' "$((t/1000))" "$(((t%1000)/100))"
  else printf '%d' "$t"; fi
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

# Session tokens (input + output)
total_tok=$(( ${TOKIN:-0} + ${TOKOUT:-0} ))
tok_fmt=$(fmttok "$total_tok")
tok_in_fmt=$(fmttok "${TOKIN:-0}")
tok_out_fmt=$(fmttok "${TOKOUT:-0}")

# Effort / reasoning mode (live per-session from stdin .effort.level: low|medium|high|xhigh|max).
# Gem-rarity ramp 🔹→🔶→💠→💎→👑 (deeper budget = rarer gem) — collision-free with the bar's
# heat family (context-urgency 🟢⚡🔥🚨, agent ⚡); 💤 = extended thinking off.
# ultracode is NOT exposed by the harness (collapses to xhigh in every channel); a session-keyed
# marker file ($HOME/.claude/ultracode/<session_id>) lets us badge it 🚀 when something asserts it.
eff_seg=""
if [ -n "${EFFORT:-}" ]; then
  case "$EFFORT" in
    low)    ec="$D"; ee="🔹" ;;
    medium) ec="$G"; ee="🔶" ;;
    high)   ec="$Y"; ee="💠" ;;
    xhigh)  ec="$M"; ee="💎" ;;
    max)    ec="$R"; ee="👑" ;;
    *)      ec="$C"; ee="🔆" ;;
  esac
  uc_id="${CLAUDE_CODE_SESSION_ID:-${SID:-}}"
  if [ "$EFFORT" = "xhigh" ] && [ -n "$uc_id" ] && [ -f "$HOME/.claude/ultracode/$uc_id" ]; then
    eff_seg="${R}🚀 ultracode${X}"
  elif [ "${THINK:-true}" = "false" ]; then
    eff_seg="${D}💤 ${EFFORT} (off)${X}"
  else
    eff_seg="${ec}${ee} ${EFFORT}${X}"
  fi
fi

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

# Account badge — OPT-IN, OFF BY DEFAULT. If you run Claude Code across multiple
# accounts (e.g. via separate CLAUDE_CONFIG_DIR launchers) and want a per-account
# medal in the statusline, fill the three slots below with YOUR OWN account emails
# and flip ACCOUNT_BADGE=1. The first account that has a per-session `account`
# marker (written by your own launcher) wins; otherwise we fall back to the config
# dir's cached oauthAccount email. Empty by default — no founder/host identity here.
ACCOUNT_BADGE=0
ACCOUNT_EMAIL_1=""   # → 🥇  (e.g. you@example.com)
ACCOUNT_EMAIL_2=""   # → 🥈
ACCOUNT_EMAIL_3=""   # → 🥉
badge=""
if [ "${ACCOUNT_BADGE:-0}" = "1" ]; then
  cfgdir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
  badge="🥇 "
  if [ -f "$cfgdir/account" ]; then
    read -r _an _ < "$cfgdir/account" || true
    [ "${_an:-1}" = "2" ] && badge="🥈 "
    [ "${_an:-1}" = "3" ] && badge="🥉 "
  else
    _aj="$HOME/.claude.json"
    [ "$cfgdir" != "$HOME/.claude" ] && _aj="$cfgdir/.claude.json"
    _email="$(jq -r '.oauthAccount.emailAddress // ""' "$_aj" 2>/dev/null || true)"
    if   [ -n "$ACCOUNT_EMAIL_2" ] && [ "$_email" = "$ACCOUNT_EMAIL_2" ]; then badge="🥈 "
    elif [ -n "$ACCOUNT_EMAIL_3" ] && [ "$_email" = "$ACCOUNT_EMAIL_3" ]; then badge="🥉 "
    elif [ -n "$ACCOUNT_EMAIL_1" ] && [ "$_email" = "$ACCOUNT_EMAIL_1" ]; then badge="🥇 "
    fi
  fi
fi

# ── Rate-limit harvest — persist the gauges for mechanical consumers (e.g. the
#    orchestrator/watcher LIMITS checks and limits-hook.sh) so nothing scrapes them
#    off a rendered pane. Atomic write, keyed by account, one file PER SESSION. The stdin
#    JSON is the ONLY carrier of .rate_limits, so the statusline is the harvester of
#    record. Single-account setups always key to acct-1. ──
_hnow=$(date +%s)
if (( ${HR5:-0} > 0 || ${D7:-0} > 0 )) && (( ${HR5R:-0} > _hnow )); then
  # Freshness gate: an ACTIVE session's 5h resets_at is in the future; an idle chat
  # re-renders a stale snapshot with a fresh write ts — a past resets_at marks dead
  # data, never harvested. One file PER SESSION (sessions carry different snapshot
  # vintages of one account); consumers take max across fresh files. Self-reaps at 1h.
  # Account from the badge (🥇→1 / 🥈→2 / 🥉→3); defaults to 1 when the badge is off.
  _ra=1; [ "${badge:-}" = "🥈 " ] && _ra=2; [ "${badge:-}" = "🥉 " ] && _ra=3
  _rld=/tmp/cc-rate-limits
  _rsid="${SID:-anon}"
  { mkdir -p "$_rld" && printf '{"acct":%s,"five_hour_used":%s,"seven_day_used":%s,"five_hour_resets_at":%s,"seven_day_resets_at":%s,"ts":%s}\n' \
      "$_ra" "${HR5:-0}" "${D7:-0}" "${HR5R:-0}" "${D7R:-0}" "$_hnow" > "$_rld/.acct-${_ra}.${_rsid}.$$" \
    && mv -f "$_rld/.acct-${_ra}.${_rsid}.$$" "$_rld/acct-${_ra}.${_rsid}.json" \
    && find "$_rld" -name "acct-${_ra}.*.json" -mmin +60 -delete; } 2>/dev/null || true
fi

# ── LINE 1: Identity ────────────────────────────────────────────────
l1="${badge}${C}${ms} ${MODEL}${X}"
[ -n "${SESSNAME:-}" ] && l1+="${SEP}${W}🔖 ${SESSNAME}${X}"
[ -n "${eff_seg:-}" ] && l1+="${SEP}${eff_seg}"
l1+="${SEP}${B}${dn}${X}"
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

# Width gate — CC delivers COLUMNS to statusline commands; drop low-priority segments when narrow.
# Unset COLUMNS → WIDE=1 → full layout (no behavior change).
WIDE=1; [ -n "${COLUMNS:-}" ] && (( ${COLUMNS:-999} < 100 )) && WIDE=0

# Chat size in tokens — what the next prompt re-sends: the last request's total input
# (cache read + cache write + fresh). The token twin of the context-% bar.
ctx_tok=$(( ${CACHER:-0} + ${CACHEC:-0} + ${CACHEI:-0} ))
(( WIDE && ctx_tok > 0 )) && l2+="${SEP}${D}🧮$(fmttok "$ctx_tok")${X}"

# Cache window — will the NEXT prompt hit the prompt cache? Shows the TTL this chat runs
# (5m, or 1h when launched with ENABLE_PROMPT_CACHING_1H — set by your launcher; env
# inherited from the claude process) and the time left: every request re-arms the window,
# and the transcript's mtime is the last request. ✓12m = warm, hits; ✗ = window passed,
# next prompt pays full freight.
if (( WIDE )) && [ -n "${TPATH:-}" ] && [ -f "$TPATH" ]; then
  cttl=300; cwl="5m"
  [ "${ENABLE_PROMPT_CACHING_1H:-}" = "1" ] && { cttl=3600; cwl="1h"; }
  # BSD stat first (macOS), GNU fallback (Linux); 0 mtime = unreadable → skip the segment
  cmt=$(stat -f%m "$TPATH" 2>/dev/null || stat -c%Y "$TPATH" 2>/dev/null || echo 0)
  cage=$(( $(date +%s) - cmt ))
  crem=$(( cttl - cage ))
  if (( cmt == 0 )); then :
  elif (( crem > 0 )); then
    crf="${crem}s"; (( crem >= 60 )) && crf="$(( (crem + 59) / 60 ))m"
    l2+="${SEP}${G}💾${cwl}✓${crf}${X}"
  else
    # how long ago the window closed — ✗20m = it expired 20 minutes ago
    cpast=$(( -crem ))
    crf="${cpast}s"; (( cpast >= 60 )) && crf="$(( cpast / 60 ))m"
    (( cpast >= 5400 )) && crf="$(( (cpast + 1800) / 3600 ))h"
    l2+="${SEP}${R}💾${cwl}✗${crf}${X}"
  fi
fi

# Session token count (in→out breakdown), riding on the cache segment's right
if (( total_tok > 0 )); then
  l2+=" ${D}(${tok_in_fmt}→${tok_out_fmt})${X}"
fi

# Cost (float comparison via single awk call)
read -r c1 c2 c3 < <(awk "BEGIN{c=${COST:-0}; print (c>0)?1:0, (c>=2)?1:0, (c>=10)?1:0}" 2>/dev/null) || { c1=0; c2=0; c3=0; }
if [ "$c1" = "1" ]; then
  cc="$D"; [ "$c2" = "1" ] && cc="$Y"; [ "$c3" = "1" ] && cc="$R"
  l2+="${SEP}${cc}💰$(printf '$%.2f' "${COST:-0}")${X}"
fi

# Duration
l2+="${SEP}${D}⏱ ${df}${X}"

# ── LINE 3: Money & account limits ──────────────────────────────────
l3=""

# Local segment modules — OPT-IN extension point: every *.sh under
# ~/.claude/statusline/segments.d/ is sourced here in name order, with the
# helpers (pc/mkbar/fmttok), colors, SEP, and all parsed fields in scope.
# A module appends to l1/l2/l3 (l3 — the money/limits line, empty here — is
# the usual target). Host-personal gauges (cloud spend, quota meters) live
# here as local files, never in this shipped core.
if [ -d "$HOME/.claude/statusline/segments.d" ]; then
  for _seg in "$HOME/.claude/statusline/segments.d"/*.sh; do
    [ -f "$_seg" ] && { . "$_seg"; } 2>/dev/null || true
  done
fi

# Rate limits (Pro/Max — direct from stdin JSON, no API call): 5-hour + 7-day windows.
# Account-scoped, so they live on the money/limits line, not the session line.
if (( ${HR5:-0} > 0 )); then
  [ -n "$l3" ] && l3+="${SEP}"
  l3+="$(mkbar "$HR5" 5) $(pc "$HR5")5h-used:${HR5}%${X}"
  if (( ${HR5R:-0} > 0 )); then
    rem=$(( HR5R - $(date +%s) ))
    (( rem > 0 )) && l3+=" ${D}↻$((rem/3600))h$(((rem%3600)/60))m${X}"
  fi
fi
# 7-day window — the weekly ceiling; pc() flags it red past 80%.
if (( ${D7:-0} > 0 )); then
  [ -n "$l3" ] && l3+="${SEP}"
  l3+="$(mkbar "$D7" 5) $(pc "$D7")7d-used:${D7}%${X}"
  if (( ${D7R:-0} > 0 )); then
    rem7=$(( D7R - $(date +%s) ))
    (( rem7 > 0 )) && l3+=" ${D}↻$((rem7/86400))d$(((rem7%86400)/3600))h${X}"
  fi
fi

# ── Render (prepend reset to fight CC's dimColor wrapper) ────────────
if [ -n "$l3" ]; then
  printf '\033[0m%s\n\033[0m%s\n\033[0m%s\n' "$l1" "$l2" "$l3"
else
  printf '\033[0m%s\n\033[0m%s\n' "$l1" "$l2"
fi
