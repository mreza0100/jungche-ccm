#!/usr/bin/env bash
set -euo pipefail

# Session-keyed guard-marker maintenance (pcm/km gates).
#   read (default) — PostToolUse(Read): stamps this session's quality marker when
#                    .claude/commands/quality/prompt.md is read, making the
#                    mandatory /quality:prompt load deterministic, not advisory.
#   stop           — Stop: clears THIS session's markers at turn end (never
#                    another live session's — markers are session-keyed).
# Both modes reap abandoned markers (age > 1h) so tmp/ never accumulates stale keys.

MODE="${1:-read}"
INPUT=$(cat 2>/dev/null || true)
SID=$(printf '%s' "$INPUT" | jq -r '.session_id // empty' 2>/dev/null || true)

reap() {
  local root="$1" now
  now=$(date +%s)
  for m in "$root"/tmp/professor_pcm_active* "$root"/tmp/professor_km_active* "$root"/tmp/professor_quality_loaded*; do
    [[ -f "$m" ]] || continue
    local age=$(( now - $(cat "$m" 2>/dev/null || echo 0) ))
    (( age > 3600 )) && rm -f "$m"
  done
  return 0
}

case "$MODE" in
  read)
    FILE_PATH=$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null || true)
    [[ -z "$FILE_PATH" ]] && exit 0
    case "$FILE_PATH" in
      */.claude/commands/quality/prompt.md) ;;
      *) exit 0 ;;
    esac
    REPO_ROOT=$(git -C "$(dirname "$FILE_PATH")" rev-parse --show-toplevel 2>/dev/null) || exit 0
    mkdir -p "$REPO_ROOT/tmp"
    date +%s > "$REPO_ROOT/tmp/professor_quality_loaded${SID:+.$SID}"
    reap "$REPO_ROOT"
    ;;
  stop)
    REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
    rm -f "$REPO_ROOT/tmp/professor_pcm_active${SID:+.$SID}" \
          "$REPO_ROOT/tmp/professor_km_active${SID:+.$SID}" \
          "$REPO_ROOT/tmp/professor_quality_loaded${SID:+.$SID}"
    reap "$REPO_ROOT"
    ;;
esac
exit 0
