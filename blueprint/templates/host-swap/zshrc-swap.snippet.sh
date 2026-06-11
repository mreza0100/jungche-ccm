# ── Claude Code: multi-account per-chat billing swap ─────────────────────────
# Masters: account 1 "work"     → ~/.claude  + Keychain "Claude Code-credentials"
#          account 2 "personal" → ~/.claude2 + "Claude Code-credentials-XXXXXXXX"
#          account 3 "third"    → ~/.claude3 + "Claude Code-credentials-YYYYYYYY"
# (Replace XXXXXXXX/YYYYYYYY with your real suffixes — see README for the
# shasum command.)
#
# Every launch gets its own config dir under ~/.claude-sessions (symlink shell
# over the master; private Keychain item seeded from it), so /swap inside a chat
# switches THAT chat's account only. Logic: ~/.claude/bin/cc-launch.sh.
# Keychain suffix = first 8 hex of sha256(config dir path); unsuffixed when unset.
#
# Marker file ~/.claude-primary holds "1".."3" (default "1"). cc-swap opens an
# fzf picker (pointer starts on the next account, so plain Enter = old cycle);
# cc-swap <1|2|3> jumps without the menu.
# Commands: cc (tmux + primary), cc1/cc2/cc3 (tmux + that account), cc-clean [days].
#
# macOS only — uses the macOS Keychain via `security`.
# ─────────────────────────────────────────────────────────────────────────────

_cc_primary() {
  local n="1"
  [[ -f "$HOME/.claude-primary" ]] && n="$(<$HOME/.claude-primary)"
  echo "$n"
}

# _cc_run <account-num: 1|2|3> <tmux: 0|1> [claude args...]
_cc_run() {
  local acct="$1" use_tmux="$2"; shift 2
  local in_tmux=0; [[ -n "${TMUX:-}" ]] && in_tmux=1
  local need_tmux=0; [[ "$use_tmux" == "1" && "$in_tmux" == "0" ]] && need_tmux=1

  local sdir
  sdir="$("$HOME/.claude/bin/cc-launch.sh" "$acct")" || { echo "cc: session setup failed" >&2; return 1; }

  if (( need_tmux )); then
    tmux new-session "CLAUDE_CONFIG_DIR=$sdir claude $*"
  else
    CLAUDE_CONFIG_DIR="$sdir" claude "$@"
  fi
}

# Guard: if the caller already defines `cc`, leave it untouched.
if ! typeset -f cc > /dev/null 2>&1; then
  cc()  { _cc_run "$(_cc_primary)" 1 "$@"; }                                  # tmux + primary
fi

# ── EDIT: update labels in comments to match your accounts ──────────────────
cc1() { _cc_run 1 1 "$@"; }   # tmux + account 1 (work)
cc2() { _cc_run 2 1 "$@"; }   # tmux + account 2 (personal)
cc3() { _cc_run 3 1 "$@"; }   # tmux + account 3 (third)
# ── END EDIT ─────────────────────────────────────────────────────────────────

cc-swap() {
  local cur n; cur="$(_cc_primary)"

  # ── EDIT: your accounts ──────────────────────────────────────────────────
  local -a rows=(
    "1 │ 🥇 work     │ you@work.example"
    "2 │ 🥈 personal │ you@personal.example"
    "3 │ 🥉 third    │ you3@example"
  )
  # ── END EDIT ───────────────────────────────────────────────────────────────

  if [[ "${1:-}" =~ ^[123]$ ]]; then
    n="$1"
  elif command -v fzf >/dev/null; then
    local next; case "$cur" in 1) next="2" ;; 2) next="3" ;; *) next="1" ;; esac
    rows[$cur]="${rows[$cur]}  ← current"
    local pick
    pick="$(printf '%s\n' "${rows[@]}" | fzf \
      --height=~9 --reverse --cycle --no-info --header-first \
      --border=rounded --border-label=' Claude Code · primary account ' \
      --header="Enter picks (starts on next) · Esc keeps account $cur" \
      --prompt='cc ❯ ' --pointer='▶' \
      --bind "start:pos($next)" \
      --color='border:cyan,label:bold:cyan,header:dim,prompt:bold,pointer:yellow')" || true
    [[ -z "$pick" ]] && { echo "cc-swap: unchanged — primary stays account $cur"; return 0; }
    n="${pick%% *}"
  else
    echo "cc-swap: fzf not found — pass a number: cc-swap <1|2|3>"; return 1
  fi
  echo "$n" > "$HOME/.claude-primary"

  # ── EDIT: update labels to match your accounts ───────────────────────────
  local lbl; case "$n" in 1) lbl="🥇 work" ;; 2) lbl="🥈 personal" ;; 3) lbl="🥉 third" ;; esac
  # ── END EDIT ───────────────────────────────────────────────────────────────

  echo "Primary → account $n ($lbl)"
  echo "  cc          → account $n"
  echo "  cc1/cc2/cc3 → explicit account"
}

# cc-clean [days] — prune per-session config dirs older than N days (default 7)
# plus their private Keychain items. Long-running sessions older than the
# cutoff would need a /login after pruning, so keep the window generous.
cc-clean() {
  local days="${1:-7}" d sfx
  find "$HOME/.claude-sessions" -maxdepth 1 -type d -name 's*' -mtime +"$days" 2>/dev/null | while read -r d; do
    sfx="$(printf '%s' "$d" | shasum -a 256 | cut -c1-8)"
    security delete-generic-password -s "Claude Code-credentials-$sfx" >/dev/null 2>&1
    rm -rf "$d"
    echo "pruned $d"
  done
}
# ── end multi-account swap ────────────────────────────────────────────────────
