# ── Account badge ────────────────────────────────────────────────────────────
# Merge this block into ~/.claude/statusline-command.sh just before the
# "LINE 1: Identity" section. The `badge` variable is already consumed by
# the l1 line in the Professor statusline — you only need this computation.
#
# Primary signal: the oauthAccount email cached in each account's own .claude.json (works on
# any platform, no launcher-specific plumbing required). The `<cfgdir>/account` marker file
# below is optional legacy support for a launcher that writes one (format: "<n> <label>
# <email>") — the bundle's own cc-fleet.zsh launcher does not write this file, so that branch
# is a no-op fallback unless your own launcher populates it.
#
# ── EDIT: update labels and fallback emails to match your accounts ───────────
# Account 1 gets 🥇, account 2 gets 🥈, account 3 gets 🥉.
# The fallback case block uses the oauthAccount email from .claude.json for
# sessions launched without the cc launcher (e.g., plain `claude`).
#
# Optional — canonical per-account config-dir paths. If your launcher gives each
# account ONE fixed, never-changing config dir, fill these in and they're checked
# FIRST (before the marker file or the email fallback) — the fastest, least
# ambiguous match. Leave empty (default) to skip straight to the marker file.
ACCOUNT_CFGDIR_2=""   # e.g. $HOME/.cc/2
ACCOUNT_CFGDIR_3=""   # e.g. $HOME/.cc/3
cfgdir="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"
badge="🥇 "
if [ -n "$ACCOUNT_CFGDIR_3" ] && [ "$cfgdir" = "$ACCOUNT_CFGDIR_3" ]; then
  badge="🥉 "
elif [ -n "$ACCOUNT_CFGDIR_2" ] && [ "$cfgdir" = "$ACCOUNT_CFGDIR_2" ]; then
  badge="🥈 "
elif [ -f "$cfgdir/account" ]; then
  read -r _an _ < "$cfgdir/account" || true
  [ "${_an:-1}" = "2" ] && badge="🥈 "
  [ "${_an:-1}" = "3" ] && badge="🥉 "
else
  _aj="$HOME/.claude.json"
  [ "$cfgdir" != "$HOME/.claude" ] && _aj="$cfgdir/.claude.json"
  case "$(jq -r '.oauthAccount.emailAddress // ""' "$_aj" 2>/dev/null || true)" in
    you@personal.example)  badge="🥈 " ;;   # account 2
    you3@example)          badge="🥉 " ;;   # account 3
    you@work.example)      badge="🥇 " ;;   # account 1 (explicit match)
    *) [ "$cfgdir" = "$HOME/.claude2" ] && badge="🥈 " ;;   # master dir fallback
  esac
fi
# ── END EDIT ──────────────────────────────────────────────────────────────────
# After this block, `badge` holds the correct emoji prefix ("🥇 ", "🥈 ", or "🥉 ").
# Reference it in your l1 line, e.g.:   l1="${badge}${C}${ms} ${MODEL}${X}..."
