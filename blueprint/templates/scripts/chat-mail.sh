#!/usr/bin/env bash
set -euo pipefail

# chat-mail.sh — the chat: family message bus. A sender resolves a target chat
# by text (chat-find.sh) to its session-id, then drops a message into that
# session's inbox; the recipient reads its own inbox here.
#
# Usage:
#   chat-mail.sh send <target-session-id> <message...>
#   chat-mail.sh inbox     # read + archive THIS chat's unread
#   chat-mail.sh drain     # UserPromptSubmit hook: surface unread as context + archive
#   chat-mail.sh register  # SessionStart hook: record this chat's tmux/config -> uuid
#   chat-mail.sh count     # print THIS chat's unread count (no side effects)
#   chat-mail.sh notify    # one-line nudge if THIS chat has unread; always exit 0
#
# Bus: $CHAT_MAILBOX_DIR or ~/.claude-sessions/.mailbox ; per-session inbox at
# by-session/<id>/ ; read messages archived to by-session/<id>/.read/.

MBOX="${CHAT_MAILBOX_DIR:-$HOME/.claude-sessions/.mailbox}"

valid_sid() { [[ "$1" =~ ^[A-Za-z0-9_-]+$ ]]; }

# This chat's identity for stamping outgoing mail. The cc session wrapper sets
# CLAUDE_CONFIG_DIR to its session dir, which holds the account file
# ("<num> <label> <email>"); CLAUDE_CODE_SESSION_ID is the UUID keying inboxes.
self_sid="${CLAUDE_CODE_SESSION_ID:-}"
self_sid8="${self_sid:0:8}"
[[ -n "$self_sid8" ]] || self_sid8="unknown"
self_acct="?"
self_label="unknown"
acct_file="${CLAUDE_CONFIG_DIR:-$HOME/.claude}/account"
if [[ -f "$acct_file" ]]; then
  read -r self_acct self_label _rest < "$acct_file" || true
  [[ -n "$self_label" ]] || self_label="unknown"
fi

cmd="${1:-}"
case "$cmd" in
  send)
    shift
    target="${1:-}"
    shift || true
    msg="$*"
    if [[ -z "$target" || -z "$msg" ]]; then
      echo "usage: $0 send <target-session-id> <message...>" >&2
      exit 1
    fi
    if ! valid_sid "$target"; then
      echo "ERROR: invalid target session-id: $target" >&2
      exit 1
    fi
    inbox="$MBOX/by-session/$target"
    mkdir -p "$inbox"
    ts="$(date -u +%Y%m%dT%H%M%SZ)"
    file="$inbox/${ts}__from-${self_label}-${self_sid8}.md"
    {
      echo "To: session $target"
      echo "From: $self_label (acct $self_acct), session $self_sid8"
      echo "Reply-To: session $self_sid"
      echo "Sent: $ts UTC"
      echo ""
      printf '%s\n' "$msg"
    } > "$file"
    echo "sent to session $target ($file)"
    ;;
  inbox)
    if [[ -z "$self_sid" ]]; then
      echo "ERROR: CLAUDE_CODE_SESSION_ID unset — cannot resolve this chat's inbox" >&2
      exit 1
    fi
    inbox="$MBOX/by-session/$self_sid"
    shopt -s nullglob
    unread=("$inbox"/*.md)
    shopt -u nullglob
    if [[ ${#unread[@]} -eq 0 ]]; then
      echo "no unread chat-mail for this chat (session $self_sid8)"
      exit 0
    fi
    mkdir -p "$inbox/.read"
    for f in "${unread[@]}"; do
      echo "----------------------------------------"
      cat "$f"
      echo ""
      mv "$f" "$inbox/.read/"
    done
    echo "----------------------------------------"
    echo "(${#unread[@]} message(s) read and archived to .read/)"
    ;;
  count)
    if [[ -z "$self_sid" ]]; then
      echo 0
      exit 0
    fi
    inbox="$MBOX/by-session/$self_sid"
    shopt -s nullglob
    unread=("$inbox"/*.md)
    shopt -u nullglob
    echo "${#unread[@]}"
    ;;
  notify)
    [[ -n "$self_sid" ]] || exit 0
    inbox="$MBOX/by-session/$self_sid"
    shopt -s nullglob
    unread=("$inbox"/*.md)
    shopt -u nullglob
    if [[ ${#unread[@]} -gt 0 ]]; then
      echo "📬 ${#unread[@]} unread chat-mail for this chat — run /chat:send inbox to read"
    fi
    exit 0
    ;;
  drain)
    # UserPromptSubmit hook path: surface unread as injected context at the turn
    # boundary, then archive. Prints nothing when empty. Always exit 0.
    [[ -n "$self_sid" ]] || exit 0
    inbox="$MBOX/by-session/$self_sid"
    shopt -s nullglob
    unread=("$inbox"/*.md)
    shopt -u nullglob
    [[ ${#unread[@]} -gt 0 ]] || exit 0
    mkdir -p "$inbox/.read"
    echo "[chat-mail] ${#unread[@]} message(s) delivered to this chat from other chats:"
    for f in "${unread[@]}"; do
      echo "---"
      cat "$f"
      mv "$f" "$inbox/.read/"
    done
    echo "---"
    echo "[chat-mail] end of delivered messages (archived). To reply: /chat:send {msg} :: the Reply-To session-id shown above."
    exit 0
    ;;
  register)
    # SessionStart hook path: self-register this chat's identity so other chats
    # can target it by tmux session or config dir without scanning process env.
    [[ -n "$self_sid" ]] || exit 0
    regdir="$MBOX/registry"
    mkdir -p "$regdir/by-tmux"
    tmux_sess=""
    if [[ -n "${TMUX:-}" ]]; then
      tmux_sess="$(tmux display-message -p '#{session_name}' 2>/dev/null | tr -cd 'A-Za-z0-9._-' || true)"
    fi
    {
      echo "uuid=$self_sid"
      echo "tmux=$tmux_sess"
      echo "config_dir=${CLAUDE_CONFIG_DIR:-}"
      echo "account=$self_label"
      echo "updated=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    } > "$regdir/$self_sid"
    [[ -n "$tmux_sess" ]] && printf '%s\n' "$self_sid" > "$regdir/by-tmux/$tmux_sess"
    exit 0
    ;;
  *)
    echo "usage: $0 {send <target-sid> <msg...>|inbox|drain|register|count|notify}" >&2
    exit 1
    ;;
esac
