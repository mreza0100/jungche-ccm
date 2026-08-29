#!/usr/bin/env bash
set -euo pipefail

# Codex-mirror auto-compile — the deterministic "always compile after framework edits".
#   mark — PostToolUse(Edit|Write): when the edited file is a Claude source the mirror
#          compiles from (.claude/**, any CLAUDE.md, $HOME/.claude/commands/**), drop
#          the repo-scoped dirty flag tmp/professor_codex_dirty.
#   sync — Stop: if the flag is present, run `pfm codex build` then `pfm codex
#          check`; success clears the flag silently, failure BLOCKS
#          turn end (exit 2, reason on stderr) so a broken mirror is fixed, never
#          silently shipped. Respects stop_hook_active — a block never loops; on a
#          suppressed block the flag stays set so the next turn retries.
# Coverage (declared): sees Edit/Write TOOL calls only. A Bash-driven write (sed,
# redirect) to a Claude source does NOT set the flag — `pfm codex check` in the
# ptm `structure` audit scope remains the backstop for that shape.
#
# `pfm codex build` is the SINGLE writer of the mirror. The legacy JS compiler is
# retained unwired for reference only: two writers stamp different generated
# markers into the same files, so each rewrites the other's output on every run.

MODE="${1:-mark}"
INPUT=$(cat 2>/dev/null || true)

case "$MODE" in
  mark)
    FILE_PATH=$(printf '%s' "$INPUT" | jq -r '.tool_input.file_path // empty' 2>/dev/null || true)
    [[ -z "$FILE_PATH" ]] && exit 0
    case "$FILE_PATH" in
      "$HOME"/.claude/commands/*) ;;
      */.claude/*|*/CLAUDE.md|*/.mcp.json) ;;
      *) exit 0 ;;
    esac
    REPO_ROOT=$(git -C "$(dirname "$FILE_PATH")" rev-parse --show-toplevel 2>/dev/null) \
      || REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
    command -v pfm >/dev/null 2>&1 || exit 0
    mkdir -p "$REPO_ROOT/tmp"
    touch "$REPO_ROOT/tmp/professor_codex_dirty"
    ;;
  sync)
    REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
    FLAG="$REPO_ROOT/tmp/professor_codex_dirty"
    [[ -f "$FLAG" ]] || exit 0
    if OUT=$(pfm codex build "$REPO_ROOT" 2>&1) \
       && CHK=$(pfm codex check "$REPO_ROOT" 2>&1); then
      rm -f "$FLAG"
      exit 0
    fi
    STOP_ACTIVE=$(printf '%s' "$INPUT" | jq -r '.stop_hook_active // false' 2>/dev/null || echo false)
    [[ "$STOP_ACTIVE" == "true" ]] && exit 0
    printf 'codex-sync: the Codex mirror failed to compile after this turn'\''s framework edits — fix before ending the turn.\n%s\n%s\n' "${OUT:-}" "${CHK:-}" >&2
    exit 2
    ;;
esac
exit 0
