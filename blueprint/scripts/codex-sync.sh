#!/usr/bin/env bash
set -euo pipefail

# Codex-mirror auto-compile — the deterministic "always compile after framework edits".
#   mark — PostToolUse(Edit|Write): when the edited file is a Claude source the mirror
#          compiles from (.claude/**, any CLAUDE.md, $HOME/.claude/commands/**), drop
#          the repo-scoped dirty flag tmp/professor_codex_dirty.
#   sync — Stop: if the flag is present, run `node .claude/scripts/build-codex.mjs
#          generate` then `check`; success clears the flag silently, failure BLOCKS
#          turn end (exit 2, reason on stderr) so a broken mirror is fixed, never
#          silently shipped. Respects stop_hook_active — a block never loops; on a
#          suppressed block the flag stays set so the next turn retries.
# Coverage (declared): sees Edit/Write TOOL calls only. A Bash-driven write (sed,
# redirect) to a Claude source does NOT set the flag — `build-codex.mjs check` in the
# pcm `structure` audit scope remains the backstop for that shape.

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
    [[ -x "$REPO_ROOT/.claude/scripts/build-codex.mjs" ]] || exit 0
    mkdir -p "$REPO_ROOT/tmp"
    touch "$REPO_ROOT/tmp/professor_codex_dirty"
    ;;
  sync)
    REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0
    FLAG="$REPO_ROOT/tmp/professor_codex_dirty"
    [[ -f "$FLAG" ]] || exit 0
    if OUT=$(node "$REPO_ROOT/.claude/scripts/build-codex.mjs" generate 2>&1) \
       && CHK=$(node "$REPO_ROOT/.claude/scripts/build-codex.mjs" check 2>&1); then
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
