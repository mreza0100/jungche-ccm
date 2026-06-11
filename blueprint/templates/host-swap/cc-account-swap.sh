#!/bin/bash
# cc-account-swap.sh [1|2|3|status] — PER-CHAT account swap. Rewrites ONLY the
# calling session's private Keychain item (seeded by cc-launch.sh) with the
# chosen account's master token (cycle 1→2→3→1; pass a number to jump straight
# there), then updates the session's `account` marker. `status` just prints the
# marker. Other running sessions and future launches are untouched.
#
# Requires a session launched via cc/cc1/cc2/cc3 (per-session CLAUDE_CONFIG_DIR).
#
# macOS only — uses the macOS Keychain via `security`.
set -euo pipefail

# ── EDIT: your Keychain account name ────────────────────────────────────────
KC_ACCT="$(whoami)"
# ── END EDIT ─────────────────────────────────────────────────────────────────

CFG="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"

if [[ "${1:-}" == "status" ]]; then
  if [[ -f "$CFG/account" && "$CFG" == "$HOME/.claude-sessions/"* ]]; then
    cat "$CFG/account"
  else
    echo "legacy session — no per-chat marker"
  fi
  exit 0
fi

if [[ ! -f "$CFG/account" || "$CFG" != "$HOME/.claude-sessions/"* ]]; then
  echo "ABORT: this session has no private credential — it was launched before the per-chat upgrade."
  echo "Exit and relaunch with cc2 -c (other account) or cc -c (same account) to continue this exact chat with per-chat /swap available."
  exit 1
fi

read -r cur _ _ < "$CFG/account"
if [[ "${1:-}" =~ ^[123]$ ]]; then
  new="$1"
else
  case "$cur" in 1) new="2" ;; 2) new="3" ;; *) new="1" ;; esac
fi

# ── EDIT: your accounts ──────────────────────────────────────────────────────
# Keep in sync with cc-launch.sh.
case "$new" in
  1) MASTER_ITEM="Claude Code-credentials"
     LABEL="work" EMAIL="you@work.example" ;;
  2) MASTER_ITEM="Claude Code-credentials-XXXXXXXX"   # replace XXXXXXXX with your suffix
     LABEL="personal" EMAIL="you@personal.example" ;;
  3) MASTER_ITEM="Claude Code-credentials-YYYYYYYY"   # replace YYYYYYYY with your suffix
     LABEL="third" EMAIL="you3@example" ;;
  *) echo "cc-account-swap: unknown account '$new'" >&2; exit 1 ;;
esac
# ── END EDIT ─────────────────────────────────────────────────────────────────

tok="$(security find-generic-password -s "$MASTER_ITEM" -a "$KC_ACCT" -w)"
case "$tok" in
  '{'*) ;;
  *) echo "ABORT: master credential for account $new unreadable — nothing touched." >&2; exit 1 ;;
esac

sfx="$(printf '%s' "$CFG" | shasum -a 256 | cut -c1-8)"
security add-generic-password -U -s "Claude Code-credentials-$sfx" -a "$KC_ACCT" -w "$tok"
printf '%s %s %s\n' "$new" "$LABEL" "$EMAIL" > "$CFG/account"

echo "THIS chat now bills to $EMAIL (account $new, $LABEL) — all other sessions untouched."
echo "Effective from the next message (~30s Keychain cache). The statusline badge updates on its next refresh."
