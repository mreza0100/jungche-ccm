#!/usr/bin/env zsh
# Professor — VSCode tmux launcher (shell side).
# Append this to your shell rc (~/.zshrc). Pairs with terminal-profile.json:
# the "tmux + claude" VSCode profile sets VSCODE_AUTO_CC; this runs `cc` once on
# startup, and on /exit you drop back to a normal interactive shell.

# -- cc: Claude Code in tmux (reuses an existing cc if one is already defined) --
if ! typeset -f cc >/dev/null; then
  cc() {
    if [[ -n "$TMUX" ]]; then
      claude "$@"
    else
      tmux new-session "claude $*"   # claude exits -> tmux ends -> back to shell
    fi
  }
fi

# -- VSCode: new terminals open straight into tmux + cc --
# The "tmux + claude" profile sets VSCODE_AUTO_CC. Run cc once, then unset it so
# the tmux/claude children never re-trigger. On /exit you land back in a shell.
if [[ -o interactive && -n "$VSCODE_AUTO_CC" ]]; then
  unset VSCODE_AUTO_CC
  cc
fi
