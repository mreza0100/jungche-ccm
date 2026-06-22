#!/bin/sh
# ---------------------------------------------------------------------------
# memory-sync.sh — Claude Code SessionEnd hook. Syncs the WHOLE memory vault
# (every project's memory, one subdir each) to its private git repo. Plain git —
# no LLM, no tokens. Pull-rebase first (multi-writer-safe across machines), then
# commit + push any change. Self-healing: pushes whenever local is ahead of
# remote, so a push cut off by a hard window-close recovers on the next session.
#
# Vault path: $CLAUDE_MEMORY_REPO if set, else the install-baked default below.
# ---------------------------------------------------------------------------
export GIT_TERMINAL_PROMPT=0          # never hang on a credential prompt — fail fast + log
REPO="${CLAUDE_MEMORY_REPO:-$HOME/work/{MEMORY_VAULT_DIR}}"
LOG="$HOME/.claude/memory-sync.log"
cd "$REPO" || exit 0
[ -d "$REPO/.git" ] || exit 0

# rebase local memory on top of any concurrent writer (another machine), stashing
# uncommitted edits so the rebase never blocks. multi-writer-safe.
git pull --rebase --autostash -q origin main >>"$LOG" 2>&1 || true

# commit any pending memory changes (no-op if clean)
if [ -n "$(git status --porcelain)" ]; then
    git add -A
    git commit -q -m "memory: auto-sync $(date '+%Y-%m-%d %H:%M:%S')" >>"$LOG" 2>&1
fi

# push if local is ahead of remote (also recovers a previously cut-off push)
if [ -n "$(git rev-list origin/main..HEAD 2>/dev/null)" ]; then
    if git push -q origin main >>"$LOG" 2>&1; then
        echo "$(date '+%F %T') pushed" >>"$LOG"
    else
        echo "$(date '+%F %T') PUSH FAILED — retries next session" >>"$LOG"
    fi
fi
