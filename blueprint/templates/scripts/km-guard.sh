#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# km-guard.sh — PreToolUse(Edit|Write) — guards /km territory (the LLM-prompt /
# knowledge files, and any other too-sensitive-to-edit-casually asset).
#
# PROBLEM IT SOLVES
#   Some files are too sensitive to edit casually during automated work — prompt
#   text injected verbatim into an LLM, schema/migration files, secrets/config,
#   generated contracts. A careless or unauthorized edit ships straight to
#   production. This hook DENIES any Edit/Write whose target matches a protected
#   path UNLESS BOTH of THIS session's markers are fresh. Every other path passes.
#
# HOW TO CONFIGURE
#   Fill PROTECTED_PATHS with one or more globs, each matched against the edited
#   file's path RELATIVE to its repo root. The canonical use is the AI project's
#   prompt registry — e.g. "{AI_PROJECT}/knowledge/*". Leave the array empty (or
#   only the sentinel) to make the hook a silent no-op for all paths.
#
# HOW THE MARKERS / TTL WORK (session-keyed — concurrent sessions never share or
# clear each other's gate):
#   tmp/professor_km_active.<sid>       — /km is active (stamped per the deny message)
#   tmp/professor_quality_loaded.<sid>  — quality/prompt.md was READ this session
#                                         (stamped automatically by guard-stamp.sh)
#   Sliding expiry: every ALLOWED edit re-touches both markers; the TTL reaps only
#   abandoned sessions. guard-stamp.sh clears this session's markers at turn end.
# ─────────────────────────────────────────────────────────────────────────────

# ── Config ───────────────────────────────────────────────────────────────────
# Globs, repo-relative, of the files this gate protects. Fill per adoption.
# Example: PROTECTED_PATHS=( "{AI_PROJECT}/knowledge/*" )
PROTECTED_PATHS=( "{PROTECTED_PATH_GLOB}" )
# ─────────────────────────────────────────────────────────────────────────────

INPUT=$(cat)
FILE_PATH=$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // empty')
[[ -z "$FILE_PATH" ]] && exit 0

# Anchor to the EDITED FILE's repo root, not the hook's cwd — correct inside
# worktrees and regardless of where the harness spawns the hook.
REPO_ROOT=$(git -C "$(dirname "$FILE_PATH")" rev-parse --show-toplevel 2>/dev/null) || exit 0
REL_PATH="${FILE_PATH#"$REPO_ROOT"/}"

# Does the edited file match any protected glob? If not, allow silently.
matched=0
for glob in "${PROTECTED_PATHS[@]}"; do
  [[ -z "$glob" ]] && continue
  # shellcheck disable=SC2053
  if [[ "$REL_PATH" == $glob ]]; then matched=1; break; fi
done
(( matched == 0 )) && exit 0

SID=$(printf '%s' "$INPUT" | jq -r '.session_id // empty')
ACTIVE="$REPO_ROOT/tmp/professor_km_active${SID:+.$SID}"
QUALITY="$REPO_ROOT/tmp/professor_quality_loaded${SID:+.$SID}"
TTL=1500
NOW=$(date +%s)

fresh() {
  [[ -f "$1" ]] || return 1
  local age=$(( NOW - $(cat "$1" 2>/dev/null || echo 0) ))
  (( age >= 0 && age < TTL ))
}

if fresh "$ACTIVE" && fresh "$QUALITY"; then
  # Sliding expiry — an active session never times out mid-batch.
  printf '%s\n' "$NOW" > "$ACTIVE"
  printf '%s\n' "$NOW" > "$QUALITY"
  exit 0
fi

REASON="This path is gate-protected: it matches a configured PROTECTED_PATHS glob — an LLM PROMPT or other sensitive asset that ships straight to production. You ARE authorized as the owner via /km."
if ! fresh "$QUALITY"; then
  REASON+=" DENIED — protected-file edits require /quality:prompt loaded this session: Read .claude/commands/quality/prompt.md (the Read auto-stamps your session), then retry."
fi
if ! fresh "$ACTIVE"; then
  REASON+=" DENIED — these edits route through /km: open this session's gate from the repo root: date +%s > \"tmp/professor_km_active${SID:+.$SID}\" , then retry."
fi
REASON+=" Markers slide on every allowed edit and are cleared at turn end. Do NOT route around this by editing through an unprotected path or disabling the hook."

jq -cn --arg r "$REASON" '{hookSpecificOutput:{hookEventName:"PreToolUse",permissionDecision:"deny",permissionDecisionReason:$r}}'
exit 2
