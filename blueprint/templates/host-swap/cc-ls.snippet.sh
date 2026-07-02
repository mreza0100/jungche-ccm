#!/usr/bin/env zsh
# cc-ls.snippet.sh — the cc-ls chat picker for the Claude Code fleet. Append to
# ~/.zshrc (or: source ~/.claude/bin/cc-ls.snippet.sh). zsh-only (glob qualifiers,
# ${(f)…}, assoc arrays, ${var:t:r}); needs fzf for the picker.
#
# cc-ls lists every Claude Code chat as one fzf list and acts on your pick:
#   ● live    — a tmux session                → Enter attaches to it
#   ↻ resume  — a transcript with no live tmux → Enter resumes it in a fresh tmux
#   ⚙ agent   — a live background/forked session (no tmux socket — `--resume` refuses
#               a session that's still running) → Enter opens the `claude agents --cwd`
#               attach view under its OWNING account, so you attach instead of failing
#               to resume it. See _cc_agents below.
# Pairs with a launcher that runs each chat in its OWN `tmux -L cc-*` socket and a
# statusline that drops a /tmp/cc-sid/<socket> → transcript breadcrumb (the zshrc-swap
# multi-account snippet is one such launcher; any cc-* launcher works).

# _cc_primary — which account a resume opens (1/2/3). Guarded: the launcher snippet's
# definition wins if present; else fall back to ~/.claude-primary, then account 1.
typeset -f _cc_primary >/dev/null || _cc_primary() {
  local n=1; [[ -f "$HOME/.claude-primary" ]] && n="$(<$HOME/.claude-primary)"; echo "$n"
}

# _cc_ago <epoch> — compact relative age: 5s / 20m / 4h / 3d ago
_cc_ago() {
  local d=$(( $(date +%s) - $1 )); (( d < 0 )) && d=0
  if   (( d < 60 ));    then echo "${d}s ago"
  elif (( d < 3600 ));  then echo "$(( d / 60 ))m ago"
  elif (( d < 86400 )); then echo "$(( d / 3600 ))h ago"
  else                       echo "$(( d / 86400 ))d ago"
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

# _cc_lastprompt <transcript> — most recent REAL human prompt, flattened to one line
_cc_lastprompt() {
  [[ -r "$1" ]] || return
  tac "$1" 2>/dev/null | jq -rc --arg j "$_CC_JUNK" 'select(.type=="user" and (.message.content|type=="string") and (.message.content|test($j)|not)) | (.message.content|gsub("[\n\t]+";" "))' 2>/dev/null | head -1
}

# _cc_meta <transcript> — "cwd<TAB>first-real-prompt<TAB>prompt-count" in one jq pass (whole file).
# prompt-count = real human turns (same junk filter as naming). cc-ls caches the result by mtime.
_cc_meta() {
  [[ -r "$1" ]] || { print -r -- $'\t\t0'; return; }
  local out title firstline
  out="$(jq -rc --arg j "$_CC_JUNK" 'select(.type=="user" and (.message.content|type=="string") and (.message.content|test($j)|not)) | [(.cwd//""),(.message.content|gsub("[\n\t]+";" "))]|@tsv' "$1" 2>/dev/null)"
  # /rename writes {"type":"custom-title",…} + {"type":"agent-name",…}; the LAST one is the chat's name and wins over the first prompt
  title="$(grep -aE '"type":"(custom-title|agent-name)"' "$1" 2>/dev/null | tail -1 | jq -r '.customTitle // .agentName // empty' 2>/dev/null)"
  [[ -z "$out" ]] && { print -r -- $'\t'"$title"$'\t0'; return; }   # command-only/cleared chat — a /rename title still names it
  firstline="${out%%$'\n'*}"                                         # cwd<TAB>first-prompt of the first real turn
  print -r -- "${firstline%%$'\t'*}"$'\t'"${title:-${firstline#*$'\t'}}"$'\t'"$(print -r -- "$out" | grep -c .)"
}

# _cc_agents — map every LIVE claude session (uuid → owning config dir) into the global CCAGENT.
# A running session can't be plain `--resume`d (it's locked); cc-ls uses this to route "open" to
# the `claude agents` attach view instead. The session id is read from argv: --session-id for
# background/forked agents, else --resume for a plain interactive; the account from /proc/<pid>/environ.
_cc_agents() {
  typeset -gA CCAGENT; CCAGENT=()
  local pid rest a0 sid cfgdir e
  while read -r pid rest; do                       # ps left-pads pid → read trims it, keeps argv intact
    a0="${rest%% *}"                               # argv[0] must be the claude binary, not a shell/tool that merely mentions a uuid
    [[ "${a0:t}" == claude || "$a0" == */claude/versions/* ]] || continue
    if   [[ "$rest" == *"--session-id "* ]]; then sid="${rest#*--session-id }"; sid="${sid%% *}"
    elif [[ "$rest" == *"--resume "* ]];     then sid="${rest#*--resume }";     sid="${sid%% *}"
    else continue; fi
    sid="${sid:t:r}"                               # a --resume path → its uuid; a bare uuid is unchanged
    [[ "$sid" == *-*-*-*-* ]] || continue          # uuid shape guard (4 hyphens)
    [[ -n "${CCAGENT[$sid]}" ]] && continue        # dedup: the agent + its pty-host share one id
    cfgdir="$HOME/.claude"
    for e in "${(@f)$(tr '\0' '\n' < /proc/$pid/environ 2>/dev/null)}"; do
      [[ "$e" == CLAUDE_CONFIG_DIR=* ]] && { cfgdir="${e#CLAUDE_CONFIG_DIR=}"; break; }
    done
    CCAGENT[$sid]="$cfgdir"
  done < <(ps -o pid=,args= -U "$(id -u)" 2>/dev/null)
}

# cc-ls — unified chat picker. Source 1: live tmux sessions (Enter → attach). Source 2: the
# Claude Code chat store (~/.claude/projects/*/*.jsonl) → every transcript NOT already live
# becomes a `cc --resume <uuid>` entry. Deduped by transcript (each tmux maps to one transcript;
# most transcripts have no tmux).  ● = live · ↻ = resumable · ⚙ = live background agent.
# Columns: project │ name (/rename tag, or last/first prompt) │ prompts │ size │ age.
# Default: orphans + hidden + recent resumables; -a shows ALL. ⌃T re-sorts recent⇄size. Enter acts · Esc.
cc-ls() {
  local dir="${TMUX_TMPDIR:-/tmp/tmux-$(id -u)}" siddir=/tmp/cc-sid store="$HOME/.claude/projects"
  local cursock="${${TMUX%%,*}:t}"
  local -a rows=()
  typeset -A live NC                         # live: transcript-uuids already in tmux · NC: cwd/name/count cache
  local s sock name tag proj win att epoch tpath bytes hsize dispname label marks lp
  local cf="$siddir/.namecache.v3" cmeta rest cwd nm ct uuid u lmt stt   # bump vN if _cc_meta logic changes
  local all=0 onlyhidden=0 hidden=0 strict=1   # strict = apply hide/orphan/cap filters (default only)
  case "$1" in -a|--all) all=1 ;; --hidden|-H) onlyhidden=1 ;; esac
  (( all || onlyhidden )) && strict=0
  # cache: uuid -> mtime\tcwd\tname\tcount  (split by param-expansion, not read, so empty fields survive)
  [[ -r "$cf" ]] && while IFS= read -r rest; do u="${rest%%$'\t'*}"; NC[$u]="${rest#*$'\t'}"; done < "$cf"
  # hide list: transcript uuids dropped with ⌃X (reversible — file kept; cc-ls -a reveals, edit "$hf" to undo)
  local hf="$HOME/.claude/.cc-ls-hidden" h; [[ -e "$hf" ]] || : > "$hf" 2>/dev/null
  typeset -A HID; [[ -r "$hf" ]] && while IFS= read -r h; do [[ -n "$h" ]] && HID[$h]=1; done < "$hf"
  # auto-unhide: a hidden chat that GROWS past its baseline = new activity → comes back to the list.
  # Baseline = transcript size, recorded lazily below (so /bb's own exit-writes get baselined in — no false return).
  local af="$hf.at" auuid asize hideskip; typeset -A AT UNHIDE
  [[ -r "$af" ]] && while IFS=$'\t' read -r auuid asize; do [[ -n "$auuid" ]] && AT[$auuid]="$asize"; done < "$af"
  _cc_agents   # CCAGENT: uuid → config dir for LIVE agents (bg/forked). Resuming these fails; cc-ls attaches instead.

  # ── source 1: live tmux sessions (attach) ──
  for s in "$dir"/*(N=); do                 # N=nullglob, ==sockets only
    sock="${s:t}"
    while IFS= read -r rest; do             # tab-safe split — an empty pane_title must not collapse columns
      [[ -z "$rest" ]] && continue
      name="${rest%%$'\t'*}"; rest="${rest#*$'\t'}"
      tag="${rest%%$'\t'*}";  rest="${rest#*$'\t'}"
      proj="${rest%%$'\t'*}"; rest="${rest#*$'\t'}"
      win="${rest%%$'\t'*}";  rest="${rest#*$'\t'}"
      att="${rest%%$'\t'*}";  epoch="${rest#*$'\t'}"
      [[ -z "$name" ]] && continue          # dead/stale socket: server gone
      bytes=0; hsize="-"; tpath=""; ct=0; lmt=""; nm=""
      [[ -r "$siddir/$sock" ]] && tpath="$(<"$siddir/$sock")"
      [[ -n "$tpath" ]] && live[${tpath:t:r}]=1                    # so source 2 won't duplicate it
      uuid="${tpath:t:r}"
      if [[ -n "$tpath" && -r "$tpath" ]]; then
        stt="$(stat -c '%s %Y' "$tpath" 2>/dev/null)"; bytes="${stt%% *}"; lmt="${stt##* }"; hsize="$(_cc_hsize "${bytes:-0}")"
      fi
      hideskip=0                                   # hidden? and did it grow since hidden → auto-unhide
      if [[ -n "$tpath" && -n "${HID[$uuid]}" ]]; then
        if [[ -n "${AT[$uuid]}" ]] && (( ${bytes:-0} > AT[$uuid] )); then UNHIDE[$uuid]=1
        else [[ -z "${AT[$uuid]}" ]] && AT[$uuid]=${bytes:-0}; hideskip=1; fi
      fi
      if (( hideskip )); then (( strict )) && { hidden=$((hidden+1)); continue; }
      elif (( onlyhidden )); then continue; fi   # default hides ⌃X'd · --hidden shows ONLY them
      (( ${bytes:-0} == 0 && strict )) && { hidden=$((hidden+1)); continue; }   # hide orphans (reveal: cc-ls -a)
      if [[ -n "$lmt" ]]; then               # name + prompt count, cached by uuid+mtime
        uuid="${tpath:t:r}"; cmeta="${NC[$uuid]}"
        if [[ -n "$cmeta" && "${cmeta%%$'\t'*}" == "$lmt" ]]; then cmeta="${cmeta#*$'\t'}"   # hit → strip mtime → cwd\tname\tcount
        else cmeta="$(_cc_meta "$tpath")"; NC[$uuid]="$lmt"$'\t'"$cmeta"; fi
        ct="${cmeta##*$'\t'}"; nm="${${cmeta#*$'\t'}%%$'\t'*}"
      fi
      if [[ "$name" == cc-* ]]; then
        dispname="$tag"; [[ -z "$dispname" || "$dispname" == "Claude Code" ]] && dispname="(unnamed)"
      else
        dispname="$name"
      fi
      # pane title empty/unnamed → prefer the /rename title from the transcript, then the last real prompt
      [[ "$dispname" == "(unnamed)" && -n "$nm" ]] && dispname="$nm"
      [[ "$dispname" == "(unnamed)" ]] && lp="$(_cc_lastprompt "$tpath")" && [[ -n "$lp" ]] && dispname="$lp"
      (( ${#dispname} > 30 )) && dispname="${dispname[1,29]}…"
      marks=""; [[ "$att" == "1" ]] && marks+="  ●"; [[ "$sock" == "$cursock" ]] && marks+="  ← here"
      (( all && hideskip )) && marks+="  ·hidden"   # tag still-hidden ones in -a
      label="$(printf '%-14s │ %-30s │ %5s │ %6s │ %-8s' "$proj" "$dispname" "${ct}p" "$hsize" "$(_cc_ago "$epoch")")${marks}"
      rows+=("${proj}"$'\t'"${ct}"$'\t'"${epoch}"$'\t'"L"$'\t'"${sock}"$'\t'"${name}"$'\t'"${label}")
    done < <(tmux -L "$sock" ls -F $'#{session_name}\t#{s/^[^ ]* //:pane_title}\t#{b:pane_current_path}\t#{session_windows}\t#{?session_attached,1,0}\t#{session_created}' 2>/dev/null)
  done

  # ── source 2: resumable chats from the store, deduped, newest first (cache keeps it fast) ──
  local cap=30 shown=0 line mt sz fp isagent acfg
  (( strict )) || cap=99999
  for line in ${(f)"$(find "$store" -maxdepth 2 -name '*.jsonl' -printf '%T@\t%s\t%p\n' 2>/dev/null | sort -rn)"}; do
    mt="${line%%.*}"; sz="${${line#*$'\t'}%%$'\t'*}"; fp="${line##*$'\t'}"; uuid="${fp:t:r}"
    [[ -n "${live[$uuid]}" ]] && continue   # already shown as a live tmux session
    isagent=0; acfg="${CCAGENT[$uuid]}"; [[ -n "$acfg" ]] && isagent=1   # live bg/forked agent → attach (below), never resume
    hideskip=0                                   # hidden? and did it grow since hidden → auto-unhide
    if [[ -n "${HID[$uuid]}" ]]; then
      if [[ -n "${AT[$uuid]}" ]] && (( sz > AT[$uuid] )); then UNHIDE[$uuid]=1
      else [[ -z "${AT[$uuid]}" ]] && AT[$uuid]=$sz; hideskip=1; fi
    fi
    if (( hideskip )); then (( strict )) && { hidden=$((hidden+1)); continue; }
    elif (( onlyhidden )); then continue; fi   # default hides ⌃X'd · --hidden shows ONLY them
    (( sz == 0 && ! isagent )) && continue
    (( shown >= cap && ! isagent )) && { hidden=$((hidden+1)); continue; }   # a live agent is never capped away
    cmeta="${NC[$uuid]}"
    if [[ -n "$cmeta" && "${cmeta%%$'\t'*}" == "$mt" ]]; then cmeta="${cmeta#*$'\t'}"   # hit → strip mtime to cwd\tname\tcount
    else cmeta="$(_cc_meta "$fp")"; NC[$uuid]="$mt"$'\t'"$cmeta"; fi
    cwd="${cmeta%%$'\t'*}"; ct="${cmeta##*$'\t'}"; nm="${${cmeta#*$'\t'}%%$'\t'*}"
    [[ -z "$nm" ]] && (( strict && ! isagent )) && { hidden=$((hidden+1)); continue; }   # promptless → hide; but a live agent always shows
    proj="${cwd:t}"; [[ -z "$proj" ]] && proj="?"
    dispname="$nm"; [[ -z "$dispname" ]] && dispname="(no prompt)"
    (( ${#dispname} > 30 )) && dispname="${dispname[1,29]}…"
    label="$(printf '%-14s │ %-30s │ %5s │ %6s │ %-8s' "$proj" "$dispname" "${ct}p" "$(_cc_hsize "$sz")" "$(_cc_ago "$mt")")"
    if (( isagent )); then
      label="$label  ⚙ agent"; (( all && hideskip )) && label="$label  ·hidden"
      rows+=("${proj}"$'\t'"${ct}"$'\t'"${mt}"$'\t'"A"$'\t'"${uuid}"$'\t'"${cwd}"$'\t'"${label}"$'\t'"${acfg}")   # A=live agent · f6=cwd · f8=owning config dir
    else
      label="$label  ↻"; (( all && hideskip )) && label="$label  ·hidden"
      rows+=("${proj}"$'\t'"${ct}"$'\t'"${mt}"$'\t'"R"$'\t'"${uuid}"$'\t'"${cwd}"$'\t'"${label}")   # field 6 = session's home dir (claude --resume is cwd-scoped)
      shown=$((shown+1))
    fi
  done
  { for u in ${(k)NC}; do print -r -- "$u"$'\t'"${NC[$u]}"; done } > "$cf" 2>/dev/null   # persist cache
  if (( ${#UNHIDE} )); then            # chats that grew since hidden → drop from the hide-list (auto-return)
    grep -vxF -f =(print -l -- ${(k)UNHIDE}) "$hf" > "$hf.t" 2>/dev/null && mv "$hf.t" "$hf"
    for u in ${(k)UNHIDE}; do unset "AT[$u]"; done
  fi
  { for u in ${(k)AT}; do [[ -n "${HID[$u]}" ]] && print -r -- "$u"$'\t'"${AT[$u]}"; done } > "$af" 2>/dev/null   # persist size baselines (still-hidden only)

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

  # pre-sort both ways; ⌃T picks within-group sort, ⌃O rotates which project sits on top.
  # rotate.awk shifts whole project-blocks (files are proj-grouped) by a counter; both keys
  # share one reload so they compose. Cheap: awk+sort over a few dozen rows.
  local tmpd; tmpd="$(mktemp -d "${TMPDIR:-/tmp}/cc-ls.XXXXXX")" || tmpd="/tmp/cc-ls.$$"
  printf '%s\n' "${rows[@]}" | sort -t$'\t' -k1,1 -k2,2nr > "$tmpd/by_prompt"   # project ▸ prompts
  printf '%s\n' "${rows[@]}" | sort -t$'\t' -k1,1 -k3,3nr > "$tmpd/by_time"   # project ▸ recent
  print -r -- time > "$tmpd/mode"; print -r -- 0 > "$tmpd/rot"
  print -r -- '{ if ($1 != p) { b++; p = $1 } blk[NR] = b; ln[NR] = $0 }
END { N = b; if (N < 1) N = 1; Rn = ((R % N) + N) % N
      for (i = 1; i <= NR; i++) { k = ((blk[i] - 1 - Rn) + N) % N; printf "%06d%08d\t%s\n", k, i, ln[i] } }' > "$tmpd/rotate.awk"
  # reload: pick within-sort file by $mode, rotate by $rot, re-sort on the (rank,index) prefix, strip it, filter hidden
  local hgrep=""
  (( strict ))     && hgrep=" | grep -vFf '$hf'"   # default: drop hidden
  (( onlyhidden )) && hgrep=" | grep -Ff '$hf'"    # --hidden: keep ONLY hidden
  local reload="m=\$(cat '$tmpd/mode'); r=\$(cat '$tmpd/rot'); f='$tmpd/by_time'; [ \"\$m\" = prompt ] && f='$tmpd/by_prompt'; awk -F'\t' -v R=\"\$r\" -f '$tmpd/rotate.awk' \"\$f\" | sort | cut -f2-${hgrep}"
  local tog_sort="m=\$(cat '$tmpd/mode'); [ \"\$m\" = prompt ] && echo time > '$tmpd/mode' || echo prompt > '$tmpd/mode'; $reload"
  local tog_proj="echo \$((\$(cat '$tmpd/rot')+1)) > '$tmpd/rot'; $reload"

  local hdr="⏎ open · ⌃T recent/prompts · ⌃R rotate · ⌃X hide⇄show · ● live ↻ resume ⚙ agent"
  local blabel=' tmux + Claude chats · ⌃T sort · ⌃R rotate '
  if (( onlyhidden )); then
    hdr="HIDDEN only — ⌃X restores · ⏎ open · ⌃T recent/prompts · ⌃R rotate"; blabel=' hidden chats · ⌃X restore '
  elif (( hidden )); then
    hdr="$hdr · ${hidden} hidden (-a all · --hidden only)"
  fi
  local pick
  pick="$(fzf \
    --delimiter=$'\t' --with-nth=7 \
    --height=~60% --reverse --cycle --no-info --header-first \
    --border=rounded --border-label="$blabel" \
    --header="$hdr" \
    --prompt='cc ❯ ' --pointer='▶' \
    --bind "ctrl-t:reload:$tog_sort" \
    --bind "ctrl-r:reload:$tog_proj" \
    --bind "ctrl-x:execute-silent(F='$hf'; if grep -qxF -- {5} \"\$F\"; then grep -vxF -- {5} \"\$F\" > \"\$F.t\"; mv \"\$F.t\" \"\$F\"; else printf '%s\n' {5} >> \"\$F\"; fi)+reload:$reload" \
    --color='border:cyan,label:bold:cyan,header:dim,prompt:bold,pointer:yellow' \
    < "$tmpd/by_time")" || true
  rm -rf "$tmpd"

  [[ -z "$pick" ]] && { echo "cc-ls: nothing selected"; return 0; }
  local -a f=("${(@ps:\t:)pick}")
  local kind="$f[4]"
  if [[ "$kind" == L ]]; then               # live → attach across its socket
    sock="$f[5]"; name="$f[6]"
    echo "Attaching → -L $sock · $name"
    TMUX= tmux -L "$sock" attach -t "$name"
  elif [[ "$kind" == A ]]; then             # live background agent → open the agent view to ATTACH
    uuid="$f[5]"; local acwd="$f[6]" acfg="$f[8]"   # --resume can't touch a running session; `claude agents` attaches
    [[ -d "$acwd" ]] || acwd="$PWD"; [[ -n "$acfg" ]] || acfg="$HOME/.claude"
    local as="cc-$(date +%s)-$$-$RANDOM"
    echo "'$uuid' is a live background agent — opening the agent view (pick it to attach) → new tmux -L $as"
    local apfx=""; [[ "$acfg" != "$HOME/.claude" ]] && apfx="CLAUDE_CONFIG_DIR=${(q)acfg} "
    TMUX= tmux -L "$as" new-session -s "$as" -c "$acwd" "${apfx}claude agents --cwd ${(q)acwd}"
  else                                       # resumable → cc --resume in a fresh tmux (like _cc_run)
    uuid="$f[5]"; local rcwd="$f[6]"           # launch in the session's home dir — claude --resume is cwd-scoped
    [[ -d "$rcwd" ]] || rcwd="$PWD"             # fall back if the project dir is gone
    local cfg; case "$(_cc_primary)" in 2) cfg="$HOME/.claude2" ;; 3) cfg="$HOME/.claude3" ;; *) cfg="" ;; esac
    local rs="cc-$(date +%s)-$$-$RANDOM"
    echo "Resuming $uuid in $rcwd → new tmux -L $rs"
    # failure net: a session live OUTSIDE tmux is invisible to _cc_agents when its argv carries no
    # uuid (picker-resume, --continue; pane siblings too) — --resume then refuses. Fall through to
    # the agent view so Enter still lands somewhere useful instead of an instant [exited].
    local rpfx=""; [[ -n "$cfg" ]] && rpfx="CLAUDE_CONFIG_DIR=${(q)cfg} "
    TMUX= tmux -L "$rs" new-session -s "$rs" -c "$rcwd" \
      "${rpfx}claude --resume $uuid || { echo; echo 'resume refused — the session is likely running elsewhere; pick it in the agent view to attach:'; ${rpfx}claude agents; }"
  fi
}
