#!/bin/sh
# Auto-backup Claude Code's project auto-memory to a private git repo.
# Installed as a SessionEnd hook (~/.claude/settings.json). Plain git — no LLM, no tokens.
# Self-healing: pushes whenever local is ahead of remote, so a push cut off by a hard
# window-close is recovered on the next session end.
export GIT_TERMINAL_PROMPT=0          # never hang on a credential prompt — fail fast + log
REPO="$HOME/work/{PROJECT_NAME}-memory"
LOG="$HOME/.claude/{PROJECT_NAME}-memory-sync.log"
cd "$REPO" || exit 0
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
