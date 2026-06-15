#!/usr/bin/env bash
set -euo pipefail

# chat-nudge.sh — live send-keys into another chat's tmux pane (gt's immediate
# lever). Resolves a session-id to its tmux session via the self-registered
# registry (no process-env scanning), or takes a tmux session name directly.
# Invasive: it types a submitted prompt into a running chat. Like gt, it cannot
# interrupt a mid-tool-call agent — it lands cleanly only when the pane is idle
# at its input.
#
# Usage: chat-nudge.sh <tmux-session|session-id> <message...>

MBOX="${CHAT_MAILBOX_DIR:-$HOME/.claude-sessions/.mailbox}"
[[ $# -ge 2 ]] || { echo "usage: $0 <tmux-session|session-id> <message...>" >&2; exit 1; }
target="$1"; shift
msg="$*"
[[ -n "$msg" ]] || { echo "ERROR: refusing to nudge an empty message" >&2; exit 1; }

# Resolve to a live tmux session name: a direct session name, or a session-id
# looked up in the registry the SessionStart hook self-populates.
tmux_sess=""
if tmux has-session -t "$target" 2>/dev/null; then
  tmux_sess="$target"
elif [[ -f "$MBOX/registry/$target" ]]; then
  tmux_sess="$(grep '^tmux=' "$MBOX/registry/$target" | cut -d= -f2- || true)"
fi
[[ -n "$tmux_sess" ]] || { echo "ERROR: no tmux session for target '$target' (registry has no map — pass the tmux session name)" >&2; exit 1; }
tmux has-session -t "$tmux_sess" 2>/dev/null || { echo "ERROR: tmux session '$tmux_sess' is not live" >&2; exit 1; }

# Deliver the text, then submit with a discrete Enter after a short delay.
# Claude Code's TUI uses bracketed paste: text and Enter sent together are taken
# as one multi-line paste (the newline stays literal, nothing submits — you'd
# have to press Enter yourself). Sending them separately, with a beat for the
# TUI to close the paste, makes the Enter register as submit. Delay is tunable.
tmux send-keys -t "$tmux_sess" -l -- "$msg"
sleep "${CHAT_NUDGE_SUBMIT_DELAY:-0.4}"
tmux send-keys -t "$tmux_sess" Enter
echo "nudged tmux session '$tmux_sess' (target $target)"
