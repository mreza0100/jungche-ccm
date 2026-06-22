#!/bin/sh
# ---------------------------------------------------------------------------
# cc-memory-wire.sh — Claude Code SessionStart hook.
# ONE git repo (the memory vault) holds EVERY project's memory, each in its own
# subdir. On session start this: (1) pulls the shared vault current, (2) ensures
# the CURRENT project's Claude memory dir is a symlink into <vault>/<project>/.
# Subdir name = the project folder's basename, so the SAME project on two machines
# maps to the SAME subdir → memory is shared across machines, per project, no
# intermingling, zero manual setup. SessionEnd (memory-sync.sh) commits + pushes.
#
# Vault path: $CLAUDE_MEMORY_REPO if set, else the install-baked default below.
# ---------------------------------------------------------------------------
REPO="${CLAUDE_MEMORY_REPO:-$HOME/work/{MEMORY_VAULT_DIR}}"
[ -d "$REPO/.git" ] || exit 0

# project dir: prefer the hook's stdin JSON (.cwd), fall back to PWD. timeout
# guards against a hang if ever invoked with no stdin.
INPUT="$(timeout 2 cat 2>/dev/null)"
CWD="$(printf '%s' "$INPUT" | jq -r '.cwd // empty' 2>/dev/null)"
[ -z "$CWD" ] && CWD="$PWD"

# keep the shared vault current (rebase local commits on top; never block on creds)
( cd "$REPO" && GIT_TERMINAL_PROMPT=0 git pull --rebase --autostash -q origin main 2>/dev/null ) || true

PROJ="$(basename "$CWD")"
ENC="$(printf '%s' "$CWD" | sed 's#/#-#g')"          # Claude's project-dir encoding
MEMDIR="$HOME/.claude/projects/$ENC/memory"
SUB="$REPO/$PROJ"

# root-guard: a memory dir already symlinked to the vault ROOT (the legacy
# single-project "main brain") is left exactly as-is, never re-homed into a subdir.
if [ -L "$MEMDIR" ]; then
  _tgt="$(readlink -f "$MEMDIR" 2>/dev/null || readlink "$MEMDIR" 2>/dev/null)"
  _root="$(cd "$REPO" 2>/dev/null && pwd -P)"
  if [ "$_tgt" = "$REPO" ] || [ "$_tgt" = "$_root" ]; then exit 0; fi
fi

mkdir -p "$SUB"

# already wired to the right subdir? done.
if [ -L "$MEMDIR" ] && [ "$(readlink "$MEMDIR")" = "$SUB" ]; then exit 0; fi

mkdir -p "$(dirname "$MEMDIR")"
# migrate a pre-existing REAL memory dir into the subdir (non-destructive: never
# clobbers a file already in the vault), then replace it with the symlink.
if [ -d "$MEMDIR" ] && [ ! -L "$MEMDIR" ]; then
  cp -an "$MEMDIR"/. "$SUB"/ 2>/dev/null || true
fi
rm -rf "$MEMDIR" 2>/dev/null
ln -s "$SUB" "$MEMDIR"
exit 0
