#!/bin/bash
# cc-launch.sh <account: 1|2|3> — create a per-session Claude Code config dir
# and print its path. Called by the cc/cc1/cc2/cc3 launchers in ~/.zshrc.
#
# Why: Claude Code keys its macOS Keychain credential by the config dir path
# ("Claude Code-credentials-" + first 8 hex of sha256(path); unsuffixed when
# CLAUDE_CONFIG_DIR is unset). A per-session dir therefore gets a PRIVATE
# Keychain item, seeded here from the chosen account's master item — which is
# what lets /swap switch the account for ONE chat without touching the others.
#
# The session dir is a symlink shell over the slot's master dir, so settings,
# commands, plugins, and projects (transcripts) stay shared. The `account`
# marker file (format: "<num> <label> <email>") is read by /swap and the
# statusline badge. Prune old session dirs with cc-clean (zshrc).
#
# If seeding ever fails auth (provider rotating refresh tokens), re-seed the
# master: launch plain `claude` (acct 1) / `CLAUDE_CONFIG_DIR=$HOME/.claude2
# claude` (acct 2) / `CLAUDE_CONFIG_DIR=$HOME/.claude3 claude` (acct 3) and
# /login once.
#
# macOS only — uses the macOS Keychain via `security`.
set -euo pipefail

ACCT_NUM="${1:?usage: cc-launch.sh <1|2|3>}"

# ── EDIT: your Keychain account name ────────────────────────────────────────
# This is the -a (account) argument to `security`. On macOS it is typically
# your short username. Run `whoami` if unsure.
KC_ACCT="$(whoami)"

# ── EDIT: your accounts ──────────────────────────────────────────────────────
# num  label       master dir        master Keychain item suffix    email
#  1   work        ~/.claude         (unsuffixed — default item)    you@work.example
#  2   personal    ~/.claude2        <8-hex suffix of ~/.claude2>   you@personal.example
#  3   third       ~/.claude3        <8-hex suffix of ~/.claude3>   you3@example
#
# Find suffixes:
#   printf '%s' "$HOME/.claude2" | shasum -a 256 | cut -c1-8
#   printf '%s' "$HOME/.claude3" | shasum -a 256 | cut -c1-8
#
# Account 1 uses the unsuffixed item "Claude Code-credentials" because it
# runs without CLAUDE_CONFIG_DIR set (the default session).
case "$ACCT_NUM" in
  1) MASTER_DIR="$HOME/.claude"
     MASTER_JSON="$HOME/.claude.json"
     MASTER_ITEM="Claude Code-credentials"
     LABEL="work" EMAIL="you@work.example" ;;
  2) MASTER_DIR="$HOME/.claude2"
     MASTER_JSON="$HOME/.claude2/.claude.json"
     MASTER_ITEM="Claude Code-credentials-XXXXXXXX"   # replace XXXXXXXX with your suffix
     LABEL="personal" EMAIL="you@personal.example" ;;
  3) MASTER_DIR="$HOME/.claude3"
     MASTER_JSON="$HOME/.claude3/.claude.json"
     MASTER_ITEM="Claude Code-credentials-YYYYYYYY"   # replace YYYYYYYY with your suffix
     LABEL="third" EMAIL="you3@example" ;;
  *) echo "cc-launch: unknown account '$ACCT_NUM' (want 1, 2, or 3)" >&2; exit 1 ;;
esac
# ── END EDIT ─────────────────────────────────────────────────────────────────

tok="$(security find-generic-password -s "$MASTER_ITEM" -a "$KC_ACCT" -w)"
case "$tok" in
  '{'*) ;;
  *) echo "cc-launch: master credential for account $ACCT_NUM unreadable — run /login under that account once" >&2; exit 1 ;;
esac

SROOT="$HOME/.claude-sessions"
mkdir -p "$SROOT"
SDIR="$SROOT/s$(date +%Y%m%d-%H%M%S)-$$"
mkdir "$SDIR"

ln -s "$MASTER_DIR"/* "$SDIR"/
ln -sfn "$MASTER_JSON" "$SDIR/.claude.json"
printf '%s %s %s\n' "$ACCT_NUM" "$LABEL" "$EMAIL" > "$SDIR/account"

sfx="$(printf '%s' "$SDIR" | shasum -a 256 | cut -c1-8)"
security add-generic-password -U -s "Claude Code-credentials-$sfx" -a "$KC_ACCT" -w "$tok"

echo "$SDIR"
