#!/usr/bin/env bash
# bb-hook.sh — UserPromptSubmit hook: `/bb` closes the chat MECHANICALLY.
#
# The prompt is intercepted before the model ever sees it: the hook hides and ends the chat
# itself and exits 2, which blocks prompt processing and erases the prompt. No turn is taken,
# so `/bb` costs nothing and cannot be reinterpreted, deferred, or "helpfully" rephrased.
#
# The match is the WHOLE prompt and nothing else. "/bb doesn't work!" is a sentence ABOUT the
# command and must reach the model like any other sentence — an anchored comparison against the
# trimmed prompt is what separates the two, so a substring match is never used here.
#
# Everything outside the intentional exit 2 fails OPEN: a hook that errors on a prompt it does
# not own would eat that prompt, and a lost prompt is worse than an unhidden chat.
set -u

MODE="${CC_BB_MODE:-enforce}"                               # enforce | log (log = inert, for probing)
LOG="${CC_BB_LOG:-$HOME/.claude/.bb-hook.log}"
HIDE="${CC_BB_HIDE:-$HOME/.local/bin/cc-fleet}"

payload="$(cat 2>/dev/null)" || exit 0
prompt="$(printf '%s' "$payload" | jq -r '.prompt // ""' 2>/dev/null)" || exit 0

# trim leading/trailing whitespace — `/bb\n` and `  /bb  ` are still a bare /bb
trimmed="${prompt#"${prompt%%[![:space:]]*}"}"
trimmed="${trimmed%"${trimmed##*[![:space:]]}"}"

verdict=pass
[ "$trimmed" = "/bb" ] && verdict=trigger

if [ "$MODE" = log ]; then
  # Phase-one instrument: prove what a slash command actually delivers in .prompt before this
  # hook is ever allowed to block one. Never writes to stdout — on UserPromptSubmit stdout is
  # injected into the model's context, so a chatty inert hook is not inert.
  {
    printf '%s\tverdict=%s\tlen=%s\traw=%q\n' \
      "$(date -Is)" "$verdict" "${#prompt}" "$(printf '%.200s' "$prompt")"
  } >>"$LOG" 2>/dev/null
  lines="$(wc -l <"$LOG" 2>/dev/null || echo 0)"
  if [ "$lines" -gt 400 ] 2>/dev/null; then
    tail -n 200 "$LOG" >"$LOG.trim" 2>/dev/null && mv "$LOG.trim" "$LOG" 2>/dev/null
  fi
  exit 0
fi

[ "$verdict" = trigger ] || exit 0

# hide --self identifies THIS chat from its own TMUX/TMUX_PANE, which the hook inherits, and
# --exit hands the closing choreography to a detached finisher — so this returns at once.
"$HIDE" hide --self --exit >/dev/null 2>&1
print_status=$?
if [ "$print_status" -ne 0 ]; then
  printf 'bb: hide --self --exit failed (%s) — chat left open\n' "$print_status" >&2
  exit 2
fi
printf 'bye-bye — hidden from cc-ls, closing\n' >&2   # stderr is what the user sees on exit 2
exit 2
