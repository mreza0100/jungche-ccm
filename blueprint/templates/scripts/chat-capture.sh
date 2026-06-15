#!/usr/bin/env bash
set -euo pipefail

# chat-capture.sh — snapshot another chat's LIVE tmux window: the on-screen
# footer, spinner/idle state, task list, and statusline (what it's doing NOW).
# For conversation history use chat-read.sh instead. Capture needs a LIVE pane;
# a dormant chat has no window to snapshot.
#
# Target: a tmux session name, or a session-id (resolved to its live pane via
# the self-registered registry). Optional second arg: scrollback lines to
# include above the visible screen (default: the visible pane only).
#
# Usage: chat-capture.sh <tmux-session|session-id> [scrollback-lines]

MBOX="${CHAT_MAILBOX_DIR:-$HOME/.claude-sessions/.mailbox}"

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <tmux-session|session-id> [scrollback-lines]" >&2
  exit 1
fi
target="$1"
lines="${2:-}"

# --- Resolve to a live tmux session (same spine as chat-inject.sh's LIVE arm) ---
live_tmux=""
if tmux has-session -t "$target" 2>/dev/null; then
  live_tmux="$target"
elif [[ "$target" =~ ^[A-Za-z0-9_-]+$ && -f "$MBOX/registry/$target" ]]; then
  reg_tmux="$(grep '^tmux=' "$MBOX/registry/$target" | cut -d= -f2- || true)"
  if [[ -n "$reg_tmux" ]] && tmux has-session -t "$reg_tmux" 2>/dev/null; then
    live_tmux="$reg_tmux"
  fi
fi
[[ -n "$live_tmux" ]] || {
  echo "ERROR: no live tmux pane for '$target' — capture needs a LIVE window; use chat-read.sh for a dormant chat's transcript" >&2
  exit 1
}

# --- Capture the pane ---
if [[ -n "$lines" ]]; then
  tmux capture-pane -t "$live_tmux" -p -S "-$lines"
else
  tmux capture-pane -t "$live_tmux" -p
fi
