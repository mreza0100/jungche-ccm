#!/usr/bin/env bash
# install.sh — wire this bundle into the host by SYMLINK, so one `git pull` updates every
# command, script and launcher at once. Nothing is copied: `~/.claude/commands/chat/ls.md` and
# friends become links back into this clone, which makes the clone the single home for the
# fleet. Edit a file here and the live command changes; there is no second copy to drift.
#
#   install.sh                    dry run — print exactly what would change, touch nothing
#   install.sh --apply            make the changes
#   install.sh --uninstall        replace every link this script made with its backup (or drop it)
#   install.sh --config-dir DIR   target DIR instead of ~/.claude (rarely wanted; see below)
#
# DRY RUN IS THE DEFAULT, matching cc-reap.sh and cc-archive.sh in this bundle: a run that
# rewrites files under ~/.claude should have to be asked for twice.
#
# Idempotent. A link already pointing at the right file is reported "ok" and left alone, so
# re-running after a pull is free. A REAL file in the way is moved to <name>.pre-professor-<ts>
# before the link replaces it — this never destroys something you wrote.
set -uo pipefail

# CC_FLEET_HOME — this bundle's own directory, resolved THROUGH symlinks, so the installer works
# whether it is run from the clone, from a link, or from another directory entirely.
_ccfs="${BASH_SOURCE[0]}"; while [ -L "$_ccfs" ]; do _ccfd="$(cd -P "$(dirname "$_ccfs")" && pwd)"; _ccfs="$(readlink "$_ccfs")"; case "$_ccfs" in /*) ;; *) _ccfs="$_ccfd/$_ccfs" ;; esac; done
BUNDLE="${CC_FLEET_HOME:-$(cd -P "$(dirname "$_ccfs")" && pwd)}"

MODE=dry
CLAUDE_DIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --apply)       MODE=apply ;;
    --uninstall)   MODE=uninstall ;;
    --dry-run)     MODE=dry ;;
    --config-dir)  shift; CLAUDE_DIR="${1:?--config-dir needs a path}" ;;
    *) echo "install.sh: unknown argument '$1' (want --apply, --uninstall, --config-dir DIR)" >&2; exit 2 ;;
  esac
  shift
done

# THE TARGET IS THE HOST'S DEFAULT CONFIG DIR, NEVER $CLAUDE_CONFIG_DIR. This bundle is
# host-level — one copy shared by every account — and it will usually be installed from inside a
# running chat, where CLAUDE_CONFIG_DIR names THAT CHAT'S account. Honouring it would drop the
# scripts into, say, ~/.cc/2/bin while the /bb command still looks in ~/.claude/bin, and the
# breakage is silent. --config-dir is the deliberate override.
CLAUDE_DIR="${CLAUDE_DIR:-$HOME/.claude}"
BIN="$CLAUDE_DIR/bin"
CMD="$CLAUDE_DIR/commands"
ZSHRC="$HOME/.zshrc"
TS="$(date +%Y%m%d-%H%M%S)"
n_ok=0; n_link=0; n_backup=0; n_skip=0

say() { printf '%s\n' "$*"; }
act() { [ "$MODE" = apply ] || return 1; return 0; }

# link SRC DST — point DST at SRC, preserving anything real that was already there.
link() {
  local src="$1" dst="$2" cur
  if [ ! -e "$src" ]; then
    say "  skip    $dst  (missing in bundle: ${src#$BUNDLE/})"; n_skip=$((n_skip+1)); return
  fi
  if [ -L "$dst" ]; then
    cur="$(cd -P "$(dirname "$dst")" 2>/dev/null && readlink "$dst")"
    case "$cur" in /*) ;; *) cur="$(dirname "$dst")/$cur" ;; esac
    if [ "$(readlink -f "$dst" 2>/dev/null || echo "$cur")" = "$(readlink -f "$src" 2>/dev/null || echo "$src")" ]; then
      say "  ok      $dst"; n_ok=$((n_ok+1)); return
    fi
    say "  relink  $dst  (was -> $cur)"
    act && ln -sfn "$src" "$dst"
    n_link=$((n_link+1)); return
  fi
  if [ -e "$dst" ]; then
    # A real file: it may be the operator's own copy, and it may be NEWER than the bundle's.
    # Keep it beside the link rather than deleting it, and say where it went.
    say "  backup  $dst -> $(basename "$dst").pre-professor-$TS"
    act && mv "$dst" "$dst.pre-professor-$TS"
    n_backup=$((n_backup+1))
  else
    say "  link    $dst"
  fi
  act && { mkdir -p "$(dirname "$dst")"; ln -sfn "$src" "$dst"; }
  n_link=$((n_link+1))
}

# unlink_one DST — undo a link this script made, restoring the newest backup if one exists.
unlink_one() {
  local dst="$1" bk
  [ -L "$dst" ] || { say "  skip    $dst  (not a link — left alone)"; n_skip=$((n_skip+1)); return; }
  bk="$(ls -1d "$dst".pre-professor-* 2>/dev/null | tail -1 || true)"
  if [ -n "$bk" ]; then
    say "  restore $dst  <- $(basename "$bk")"
    act && { rm -f "$dst"; mv "$bk" "$dst"; }
  else
    say "  remove  $dst"
    act && rm -f "$dst"
  fi
  n_link=$((n_link+1))
}

say "Professor fleet bundle: $BUNDLE"
say "Target config dir:      $CLAUDE_DIR"
[ "$MODE" = dry ] && say "MODE: dry run — nothing will change. Re-run with --apply to commit."
[ "$MODE" = uninstall ] && say "MODE: uninstall"
say ""

# ── the fleet scripts: one stable address, ~/.claude/bin, independent of where the clone lives ──
FLEET_SCRIPTS="cc-db.sh cc-hide.sh cx-hide.sh cc-agent-open.sh cc-swap-chat.sh cc-archive.sh cc-reap.sh"
say "fleet scripts -> $BIN"
act && mkdir -p "$BIN"
for f in $FLEET_SCRIPTS; do
  if [ "$MODE" = uninstall ]; then unlink_one "$BIN/$f"; else link "$BUNDLE/$f" "$BIN/$f"; fi
done
say ""

# ── the slash commands: host-level, so they resolve in every repo AND every worktree ──
say "commands -> $CMD"
if [ "$MODE" = uninstall ]; then
  unlink_one "$CMD/bb.md"; unlink_one "$CMD/swap.md"
else
  link "$BUNDLE/bb.command.md"   "$CMD/bb.md"
  link "$BUNDLE/swap.command.md" "$CMD/swap.md"
fi

# chat/*.command.md in the bundle installs as chat/*.md — the bundle keeps the .command.md
# suffix so the blueprint's template tree stays self-describing; the link drops it.
for src in "$BUNDLE"/chat/*.command.md; do
  [ -e "$src" ] || continue
  base="$(basename "$src" .command.md)"
  if [ "$MODE" = uninstall ]; then unlink_one "$CMD/chat/$base.md"; else link "$src" "$CMD/chat/$base.md"; fi
done
for src in "$BUNDLE"/chat/self/*.command.md; do
  [ -e "$src" ] || continue
  base="$(basename "$src" .command.md)"
  if [ "$MODE" = uninstall ]; then unlink_one "$CMD/chat/self/$base.md"; else link "$src" "$CMD/chat/self/$base.md"; fi
done
for f in chat.sh history.sh; do
  if [ "$MODE" = uninstall ]; then unlink_one "$CMD/chat/$f"; else link "$BUNDLE/chat/$f" "$CMD/chat/$f"; fi
done
say ""

# ── ~/.zshrc: source the launchers. One line, rewritten in place when the clone moves. ──
SRC_LINE="[[ -r \"$BUNDLE/cc-fleet.zsh\" ]] && source \"$BUNDLE/cc-fleet.zsh\""
say "shell -> $ZSHRC"
if [ "$MODE" = uninstall ]; then
  if [ -f "$ZSHRC" ] && grep -q 'cc-fleet\.zsh' "$ZSHRC"; then
    say "  remove  the cc-fleet.zsh source line"
    act && { cp -p "$ZSHRC" "$ZSHRC.pre-professor-$TS"; sed -i.bak '/cc-fleet\.zsh/d' "$ZSHRC" && rm -f "$ZSHRC.bak"; }
  else
    say "  ok      no source line present"; n_ok=$((n_ok+1))
  fi
elif [ ! -f "$ZSHRC" ]; then
  say "  create  $ZSHRC with the source line"
  act && printf '%s\n' "$SRC_LINE" > "$ZSHRC"
  n_link=$((n_link+1))
elif grep -qF "$SRC_LINE" "$ZSHRC"; then
  say "  ok      source line already points here"; n_ok=$((n_ok+1))
elif grep -q 'cc-fleet\.zsh' "$ZSHRC"; then
  # An older install pointed somewhere else. Rewrite in place — never append a second line, or
  # the shell sources two copies of the fleet and the later one silently wins.
  say "  rewrite the existing cc-fleet.zsh source line to this bundle"
  act && { cp -p "$ZSHRC" "$ZSHRC.pre-professor-$TS"; awk -v repl="$SRC_LINE" '/cc-fleet\.zsh/ && !done {print repl; done=1; next} {print}' "$ZSHRC" > "$ZSHRC.tmp.$$" && mv "$ZSHRC.tmp.$$" "$ZSHRC"; }
  n_backup=$((n_backup+1))
else
  say "  append  the source line"
  act && { cp -p "$ZSHRC" "$ZSHRC.pre-professor-$TS"; printf '\n# Professor fleet — launchers + the cc-ls chat picker\n%s\n' "$SRC_LINE" >> "$ZSHRC"; }
  n_link=$((n_link+1))
fi

say ""
say "ok=$n_ok  linked/changed=$n_link  backed-up=$n_backup  skipped=$n_skip"
if [ "$MODE" = dry ]; then
  say ""
  say "Dry run — nothing changed. Run: $0 --apply"
elif [ "$MODE" = apply ]; then
  say ""
  say "Installed. Open a new shell (or: source $ZSHRC) then run  cc-ls."
  say "Backups from this run carry the suffix .pre-professor-$TS"
fi
exit 0
