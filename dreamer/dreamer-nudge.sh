#!/usr/bin/env bash
# UserPromptSubmit hook: one line for a failed or stale files-only Dreamer.
set -uo pipefail

main() {
  local root organ failure_root failed failed_stage failed_repo sweep base candidate_date candidate_sequence
  local latest='' latest_date='' latest_sequence=0 applied_at applied_epoch now age_days

  root=${CLAUDE_PROJECT_DIR:-}
  [ -n "$root" ] || return 0
  root=${root%%/.worktrees/*}
  organ="$root/.professor/stm"
  [ -d "$organ" ] || return 0

  failure_root=/tmp
  shopt -s nullglob
  for failed in "$failure_root"/dreamer-night-*/FAILED; do
    [ -f "$failed" ] || continue
    failed_stage=${failed%/FAILED}
    failed_repo=$(sed -n '1p' "$failed_stage/meta/repo-root.txt" 2>/dev/null) || continue
    [ "$failed_repo" = "$root" ] || continue
    printf '%s\n' '🌙 dreamer-night failed — inspect /tmp/dreamer-night-*/FAILED'
    shopt -u nullglob
    return 0
  done
  shopt -u nullglob

  [ -d "$organ/dreamer" ] || return 0
  shopt -s nullglob
  for sweep in "$organ"/dreamer/*.md; do
    base=$(basename "$sweep")
    [[ "$base" =~ ^([0-9]{4}-[0-9]{2}-[0-9]{2})(-([0-9]+))?\.md$ ]] || continue
    candidate_date=${BASH_REMATCH[1]}
    candidate_sequence=${BASH_REMATCH[3]:-1}
    grep -qx 'END-OF-SWEEP' "$sweep" || continue
    if [ -z "$latest" ] || [[ "$candidate_date" > "$latest_date" ]] || \
      { [ "$candidate_date" = "$latest_date" ] && [ "$candidate_sequence" -gt "$latest_sequence" ]; }; then
      latest=$sweep
      latest_date=$candidate_date
      latest_sequence=$candidate_sequence
    fi
  done
  shopt -u nullglob
  [ -n "$latest" ] || return 0

  applied_at=$(sed -n 's/^Applied: //p' "$latest" | tail -n 1)
  applied_epoch=$(date -d "$applied_at" +%s 2>/dev/null) || \
    applied_epoch=$(date -d "$latest_date 00:00:00" +%s 2>/dev/null) || return 0
  now=$(date +%s) || return 0
  (( now - applied_epoch > 172800 )) || return 0
  age_days=$(( (now - applied_epoch) / 86400 ))
  printf '🌙 dreamer-night stale — newest applied sweep is %sd old; run /dreamer\n' "$age_days"
}

main 2>/dev/null || exit 0
