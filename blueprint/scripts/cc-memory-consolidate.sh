#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# cc-memory-consolidate.sh — one-time consolidation. Moves every ~/work/<project>
# Claude memory dir into a per-project subdir of the single shared memory vault.
# Non-destructive: copies content in, VERIFIES file-for-file, only THEN swaps the
# local dir for a symlink. Idempotent. A memory dir already symlinked into the
# vault (including the legacy ROOT brain) is left untouched.
# Run once per machine; the SessionStart hook (cc-memory-wire.sh) handles new projects.
#
# Vault path: $CLAUDE_MEMORY_REPO if set, else the install-baked default below.
# ---------------------------------------------------------------------------
set -euo pipefail
REPO="${CLAUDE_MEMORY_REPO:-$HOME/work/{MEMORY_VAULT_DIR}}"
PROJROOT="$HOME/.claude/projects"
PREFIX="$(printf '%s' "$HOME/work" | sed 's#/#-#g')"     # e.g. -Users-you-work
[ -d "$REPO/.git" ] || { echo "ERROR: $REPO is not a git repo"; exit 1; }
REPO_REAL="$(cd "$REPO" && pwd -P)"

migrated=0; skipped=0
for memdir in "$PROJROOT"/*/memory; do
  [ -e "$memdir" ] || continue
  enc="$(basename "$(dirname "$memdir")")"

  # already linked into the vault (incl. the legacy root brain)? leave it.
  if [ -L "$memdir" ]; then
    tgt="$(readlink -f "$memdir" 2>/dev/null || readlink "$memdir")"
    case "$tgt" in
      "$REPO_REAL"|"$REPO_REAL"/*|"$REPO"|"$REPO"/*) echo "skip (already in vault): $enc"; skipped=$((skipped+1)); continue ;;
    esac
  fi
  # only single-level ~/work/<proj> projects (unambiguous subdir name)
  case "$enc" in "$PREFIX-"*) proj="${enc#"$PREFIX"-}" ;; *) echo "skip (not under ~/work): $enc"; skipped=$((skipped+1)); continue ;; esac

  SUB="$REPO/$proj"
  mkdir -p "$SUB"
  cp -an "$memdir"/. "$SUB"/ 2>/dev/null || cp -Rn "$memdir"/. "$SUB"/ 2>/dev/null || true

  # VERIFY every top-level entry copied before destroying the original
  ok=1
  while IFS= read -r f; do
    b="$(basename "$f")"
    [ -e "$SUB/$b" ] || { ok=0; echo "  !! $proj: missing after copy: $b"; }
  done < <(find "$memdir" -mindepth 1 -maxdepth 1)
  if [ "$ok" != 1 ]; then echo "ABORT $proj — copy incomplete; original left intact"; continue; fi

  rm -rf "$memdir"
  ln -s "$SUB" "$memdir"
  echo "migrated: $proj ($(find "$SUB" -mindepth 1 -maxdepth 1 | wc -l | tr -d ' ') entries)"
  migrated=$((migrated+1))
done

echo "=== done: $migrated migrated, $skipped skipped ==="
echo "vault subdirs: $(find "$REPO" -mindepth 1 -maxdepth 1 -type d ! -name .git -exec basename {} \; | sort | tr '\n' ' ')"
