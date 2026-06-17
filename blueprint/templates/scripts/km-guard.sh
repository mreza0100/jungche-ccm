#!/usr/bin/env bash
set -euo pipefail

# ─────────────────────────────────────────────────────────────────────────────
# Gate hook — PreToolUse(Edit|Write) protection for sensitive files.
#
# PROBLEM IT SOLVES
#   Some files are too sensitive to be edited casually during automated work —
#   prompt text injected verbatim into an LLM, schema or migration files,
#   secrets/config, generated contracts. A careless or unauthorized edit
#   ships straight to production. This hook DENIES any Edit/Write whose target
#   matches a protected path UNLESS an explicit, time-bounded marker says the
#   editor has opened the gate on purpose. Every other path passes untouched.
#
# HOW TO CONFIGURE
#   Fill PROTECTED_PATHS below with one or more globs, each matched against the
#   edited file's path RELATIVE to its repo root (e.g. "config/secrets/*").
#   Leave the array empty to make the hook a silent no-op for all paths.
#
# HOW THE MARKER / TTL WORKS
#   The gate opens when a marker file exists and is fresh (younger than
#   MARKER_TTL seconds). The authorized flow stamps it right before its edit
#   pass — `date +%s > "$REPO_ROOT/$MARKER_FILE"` — and a Stop hook (or the flow
#   itself) clears it afterward. The short TTL is a safety bound: if the Stop
#   close is ever missed, the gate auto-closes instead of staying open forever.
# ─────────────────────────────────────────────────────────────────────────────

# ── Config ───────────────────────────────────────────────────────────────────
# Globs, repo-relative, of the files this gate protects. Fill per adoption.
# Example: PROTECTED_PATHS=( "path/to/sensitive/*" )
#   — prompt files, schema/migration dirs, secrets/config, etc.
PROTECTED_PATHS=( "{PROTECTED_PATH_GLOB}" )

# Marker file (repo-relative) the authorized flow stamps to open the gate.
MARKER_FILE="tmp/gate_marker"

# How long (seconds) the marker stays valid after it is stamped.
MARKER_TTL=600
# ───────────────────────────────────────────────────────────────────────────--

INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')
[[ -z "$FILE_PATH" ]] && exit 0

# Anchor to the EDITED FILE's repo root, not the hook's cwd — correct inside
# worktrees and regardless of where the harness spawns the hook.
REPO_ROOT=$(git -C "$(dirname "$FILE_PATH")" rev-parse --show-toplevel 2>/dev/null) || exit 0
REL_PATH="${FILE_PATH#"$REPO_ROOT"/}"
MARKER="$REPO_ROOT/$MARKER_FILE"

# Does the edited file match any protected glob? If not, allow silently.
matched=0
for glob in "${PROTECTED_PATHS[@]}"; do
  [[ -z "$glob" ]] && continue
  # shellcheck disable=SC2053
  if [[ "$REL_PATH" == $glob ]]; then
    matched=1
    break
  fi
done
(( matched == 0 )) && exit 0

# Gate open if the marker is fresh. Short TTL: a Stop hook is the primary close;
# this only bounds a missed close (e.g. a sub-agent opened the gate). The flow
# stamps right before its edit pass, so the tight window never slams shut mid-work.
if [[ -f "$MARKER" ]]; then
  age=$(( $(date +%s) - $(cat "$MARKER" 2>/dev/null || echo 0) ))
  (( age >= 0 && age < MARKER_TTL )) && exit 0
fi

cat <<JSON
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"This path is gate-protected: it matches a configured PROTECTED_PATHS glob, so edits are blocked unless the gate is explicitly open. If you are authorized to change this file, open the gate on purpose: (1) confirm the edit is correct and intentional — protected files ship straight to production; (2) open the gate from the repo root: date +%s > $MARKER_FILE ; (3) retry the edit. The gate auto-closes after $MARKER_TTL seconds. Do NOT route around this guard by editing through an unprotected path or post-processing the output — that hides the change from review."}}
JSON
exit 2
