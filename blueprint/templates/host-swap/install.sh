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
# DRY RUN IS THE DEFAULT: a run that rewrites files under ~/.claude should have to be asked
# for twice.
#
# Idempotent. A link already pointing at the right file is reported "ok" and left alone, so
# re-running after a pull is free. A REAL file in the way is moved to <name>.pre-professor-<ts>
# before the link replaces it — this never destroys something you wrote.
set -uo pipefail

# PFM_HOME — this bundle's own directory, resolved THROUGH symlinks, so the installer works
# whether it is run from the clone, from a link, or from another directory entirely.
_pfms="${BASH_SOURCE[0]}"; while [ -L "$_pfms" ]; do _pfmd="$(cd -P "$(dirname "$_pfms")" && pwd)"; _pfms="$(readlink "$_pfms")"; case "$_pfms" in /*) ;; *) _pfms="$_pfmd/$_pfms" ;; esac; done
BUNDLE="${PFM_HOME:-$(cd -P "$(dirname "$_pfms")" && pwd)}"

# DO (install|uninstall) and APPLY (0|1) are INDEPENDENT — one flag each, order-insensitive.
# A single MODE variable (dry|apply|uninstall) used to carry BOTH "which action" and "dry vs
# real" at once: --uninstall and --apply each overwrote the SAME variable, so whichever came
# LAST won outright. `--uninstall --apply` collapsed to MODE=apply and ran a normal install;
# `--apply --uninstall` collapsed to MODE=uninstall and printed remove/restore lines that
# act() (gated on MODE=apply) never executed. Two variables can't cannibalize each other.
DO=install
APPLY=0
CLAUDE_DIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --apply)       APPLY=1 ;;
    --uninstall)   DO=uninstall ;;
    --dry-run)     APPLY=0 ;;
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
act() { [ "$APPLY" = 1 ] || return 1; return 0; }

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
# Independent again: DO says WHAT (install/uninstall), APPLY says whether it is real — so
# `--uninstall` bare prints BOTH "uninstall" and "dry run" (it is a dry-run uninstall preview),
# matching the same dry-run-by-default contract --apply already had.
[ "$DO" = uninstall ] && say "MODE: uninstall"
[ "$APPLY" = 0 ] && say "MODE: dry run — nothing will change. Re-run with --apply to commit."
say ""

# ── retired fleet scripts: ~/.claude/bin is empty after the Go cutover ──
# RETIRED into the Go engine (each is now a pfm subcommand, and the links
# below are removed from ~/.claude/bin on the next --apply): cc-hide.sh /
# cx-hide.sh → `pfm chat hide self [--exit]`, bb-hook.sh → `pfm chat bb`,
# cc-reap.sh → `pfm reap`, cc-archive.sh → `pfm archive`,
# cc-name-sync.sh → `pfm name-sync`, cx-heal.sh → `pfm heal`.
FLEET_SCRIPTS=""
RETIRED_SCRIPTS="cc-hide.sh cx-hide.sh bb-hook.sh cc-archive.sh cc-reap.sh cc-name-sync.sh cx-heal.sh cc-portable.sh cc-db.sh cx-recover.sh cc-agent-open.sh cc-swap-chat.sh"
RETIRED_ARTIFACTS="cx-recover.sh.pre-professor-20260815-193920"
say "fleet scripts -> $BIN"
act && mkdir -p "$BIN"
for f in $FLEET_SCRIPTS; do
  if [ "$DO" = uninstall ]; then unlink_one "$BIN/$f"; else link "$BUNDLE/$f" "$BIN/$f"; fi
done
# A retired satellite's link is REMOVED, not left dangling: the script is gone
# from the bundle, so the link resolves to nothing and anything still calling
# it fails in a way that reads as "the fleet is broken" rather than "this moved
# into pfm".
for f in $RETIRED_SCRIPTS; do
  [ -L "$BIN/$f" ] || continue
  say "  retire  $BIN/$f  (now a pfm subcommand)"
  act && rm -f "$BIN/$f"
  n_link=$((n_link+1))
done
for f in $RETIRED_ARTIFACTS; do
  [ -e "$BIN/$f" ] || [ -L "$BIN/$f" ] || continue
  say "  retire  $BIN/$f"
  act && rm -f "$BIN/$f"
  n_link=$((n_link+1))
done
say ""

# ── retired statusline implementation: rendering and both detached refreshers now live in
# `pfm statusline`. Remove only the six named predecessor files; an adopter's unrelated local
# segment is never swept. Empty directories are tidied after the files are gone. ──
RETIRED_STATUSLINE_FILES="statusline-command.sh statusline/segments.d/10-vertex-spend.sh statusline/segments.d/40-gpt-account.sh statusline/vertex-spend-refresh.py statusline/gpt-usage.py statusline/vertex_daily_tokens.py"
say "retired statusline files -> $CLAUDE_DIR"
if [ "$DO" = install ]; then
  for f in $RETIRED_STATUSLINE_FILES; do
    [ -e "$CLAUDE_DIR/$f" ] || [ -L "$CLAUDE_DIR/$f" ] || continue
    say "  retire  $CLAUDE_DIR/$f  (now pfm statusline)"
    act && rm -f "$CLAUDE_DIR/$f"
    n_link=$((n_link+1))
  done
  if act; then
    rmdir "$CLAUDE_DIR/statusline/segments.d" "$CLAUDE_DIR/statusline" 2>/dev/null || true
  fi
else
  say "  ok      native statusline files are not recreated on uninstall"; n_ok=$((n_ok+1))
fi
say ""

# ── the slash commands: host-level, so they resolve in every repo AND every worktree ──
say "commands -> $CMD"
if [ "$DO" = uninstall ]; then
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
  if [ "$DO" = uninstall ]; then unlink_one "$CMD/chat/$base.md"; else link "$src" "$CMD/chat/$base.md"; fi
done
for src in "$BUNDLE"/chat/self/*.command.md; do
  [ -e "$src" ] || continue
  base="$(basename "$src" .command.md)"
  if [ "$DO" = uninstall ]; then unlink_one "$CMD/chat/self/$base.md"; else link "$src" "$CMD/chat/self/$base.md"; fi
done
for f in dump.md; do
  [ -L "$CMD/chat/$f" ] || continue
  say "  retire  $CMD/chat/$f  (command removed)"
  act && rm -f "$CMD/chat/$f"
  n_link=$((n_link+1))
done
for f in chat.sh history.sh; do
  if [ "$DO" = uninstall ]; then unlink_one "$CMD/chat/$f"; else link "$BUNDLE/chat/$f" "$CMD/chat/$f"; fi
done
if [ -L "$CMD/chat/chat-ops.sh" ]; then
  say "  retire  $CMD/chat/chat-ops.sh  (chat subtree now lives in pfm)"
  act && rm -f "$CMD/chat/chat-ops.sh"
  n_link=$((n_link+1))
fi
say ""

# ── codex agent skills: codex ≥0.146 dropped ~/.codex/prompts custom prompts entirely; agent
# skills at ~/.agents/skills are the replacement, invoked as $<name>. Codex follows symlinked
# skill folders, so each skill dir links whole — same one-clone model as everything else. ──
SKILLS_DIR="$HOME/.agents/skills"
say "codex skills -> $SKILLS_DIR"
for d in "$BUNDLE"/codex-skills/*/; do
  [ -d "$d" ] || continue
  nm="$(basename "$d")"
  if [ "$DO" = uninstall ]; then unlink_one "$SKILLS_DIR/$nm"; else link "${d%/}" "$SKILLS_DIR/$nm"; fi
done
say ""

# ── systemd user units: the name-sync triggers (a codex rename lands on the tab in under a
# second via the path watch; the timer converges claude /rename drift). They invoke the BINARY —
# `pfm name-sync` — because window naming lives in the engine now and a unit pointing at a
# retired .sh is a trigger that fires into nothing. The predecessor cc-name-sync.* units are
# disabled and unlinked on every run, so a host that had them converges without being asked.
# Skipped cleanly where systemd --user is absent (a jail, a container) — there the sync still
# fires from every cc-ls run. ──
OLD_STATE="$HOME/.local/state/cc-fleet"
STATE="$HOME/.local/state/pfm"
migrate_state() {
  [ "$DO" = install ] || return
  if [ -d "$OLD_STATE" ] && [ ! -e "$STATE" ]; then
    say "  migrate $OLD_STATE -> $STATE"
    act && { mkdir -p "$(dirname "$STATE")"; mv "$OLD_STATE" "$STATE"; }
    n_link=$((n_link+1))
  elif [ -e "$OLD_STATE" ] && [ -e "$STATE" ]; then
    say "  warn    both $OLD_STATE and $STATE exist — left both in place"
    n_skip=$((n_skip+1))
  fi
}

SYSD="$HOME/.config/systemd/user"
UNITS="pfm-name-sync.service pfm-name-sync.path pfm-name-sync.timer"
RETIRED_UNITS="cc-fleet-name-sync.path cc-fleet-name-sync.timer cc-fleet-name-sync.service cc-name-sync.path cc-name-sync.timer cc-name-sync.service"
say "systemd user units -> $SYSD"
if command -v systemctl >/dev/null 2>&1 && systemctl --user show-environment >/dev/null 2>&1; then
  # The retirement runs in BOTH directions: install replaces the old triggers, uninstall
  # removes whatever is left of either generation.
  for u in $RETIRED_UNITS; do
    if [ ! -e "$SYSD/$u" ] && [ ! -L "$SYSD/$u" ] &&
       ! systemctl --user is-active --quiet "$u" &&
       ! systemctl --user is-enabled --quiet "$u" &&
       ! systemctl --user is-failed --quiet "$u"; then
      continue
    fi
    say "  retire  $SYSD/$u  (superseded by pfm-name-sync)"
    # A loaded unit can survive its file, so every old name is disabled,
    # stopped, reset, and unlinked whether or not the file is still visible.
    act && {
      systemctl --user disable --now "$u" >/dev/null 2>&1
      systemctl --user stop "$u" >/dev/null 2>&1
      systemctl --user reset-failed "$u" >/dev/null 2>&1
      rm -f "$SYSD/$u"
    }
    n_link=$((n_link+1))
  done
  migrate_state
  if [ "$DO" = uninstall ]; then
    act && systemctl --user disable --now pfm-name-sync.path pfm-name-sync.timer >/dev/null 2>&1
    for u in $UNITS; do unlink_one "$SYSD/$u"; done
    act && systemctl --user daemon-reload
  else
    act && mkdir -p "$SYSD"
    for u in $UNITS; do link "$BUNDLE/systemd/$u" "$SYSD/$u"; done
    if act; then
      systemctl --user daemon-reload
      systemctl --user enable --now pfm-name-sync.path pfm-name-sync.timer >/dev/null 2>&1 \
        || say "  warn    could not enable pfm-name-sync.path/.timer — run: systemctl --user enable --now pfm-name-sync.path pfm-name-sync.timer"
    fi
  fi
else
  say "  skip    systemd --user unavailable — codex renames land on the next cc-ls run instead"
  n_skip=$((n_skip+1))
  migrate_state
fi
say ""

# ── the /bb hook: settings.json is the ONE place a UserPromptSubmit hook is declared, and this
# installer is the only writer of it. The hook calls the BINARY (`pfm chat bb`), never a .sh —
# the shell hook is retired. jq also migrates every hook command that starts with the old binary
# path while preserving its arguments; without jq the file is left alone and the operator is told. ──
SETTINGS="$CLAUDE_DIR/settings.json"
OLD_BINARY="$HOME/.local/bin/cc-fleet"
PFM_BINARY="$HOME/.local/bin/pfm"
BB_COMMAND="$HOME/.local/bin/pfm chat bb"
say "/bb hook -> $SETTINGS"
if ! command -v jq >/dev/null 2>&1; then
  say "  skip    jq is not installed — wire the UserPromptSubmit hook to '$BB_COMMAND' by hand"
  n_skip=$((n_skip+1))
elif [ ! -f "$SETTINGS" ]; then
  say "  skip    no settings.json yet — it is written by Claude Code, not by this installer"
  n_skip=$((n_skip+1))
else
  bb_state="$(jq -r --arg want "$BB_COMMAND" --arg old "$OLD_BINARY" --arg pfm "$PFM_BINARY" '
    [ .hooks.UserPromptSubmit[]?.hooks[]?.command // empty ] as $commands
    | if ($commands | index($want)) then "ok"
      elif ($commands | map(select(. == ($old + " bb") or . == ($pfm + " bb") or test("bb-hook\\.sh"))) | length) > 0 then "rewire"
      else "add" end' "$SETTINGS" 2>/dev/null)" || bb_state=""
  old_hook_count="$(jq -r --arg old "$OLD_BINARY" '
    [ .hooks[]?[]?.hooks[]?.command // empty
      | select(. == $old or startswith($old + " ")) ] | length
  ' "$SETTINGS" 2>/dev/null)" || old_hook_count=0
  case "$bb_state" in
    ok)
      if [ "$old_hook_count" -gt 0 ]; then
        say "  rewire  $old_hook_count hook command(s) from '$OLD_BINARY' to '$PFM_BINARY'"
        n_link=$((n_link+1))
      else
        say "  ok      the hook already runs '$BB_COMMAND'"; n_ok=$((n_ok+1))
      fi ;;
    rewire|add)
      if [ "$DO" = uninstall ]; then
        say "  remove  the '$BB_COMMAND' UserPromptSubmit hook"
      elif [ "$bb_state" = rewire ]; then
        say "  rewire  the bb-hook.sh UserPromptSubmit hook -> '$BB_COMMAND'"
      else
        say "  add     a UserPromptSubmit hook running '$BB_COMMAND'"
      fi
      n_link=$((n_link+1)) ;;
    *)
      say "  skip    could not read $SETTINGS as JSON — left untouched"; n_skip=$((n_skip+1)) ;;
  esac
  if [ "$DO" = uninstall ]; then
    if act && [ "$bb_state" != "" ] && [ "$bb_state" != "skip" ]; then
      cp -p "$SETTINGS" "$SETTINGS.pre-professor-$TS"
      jq --arg want "$BB_COMMAND" '
        .hooks.UserPromptSubmit = [
          .hooks.UserPromptSubmit[]?
          | .hooks = [ .hooks[]? | select(.command != $want) ]
        ] | .hooks.UserPromptSubmit |= map(select((.hooks | length) > 0))
      ' "$SETTINGS" > "$SETTINGS.tmp.$$" && mv "$SETTINGS.tmp.$$" "$SETTINGS"
    fi
  elif [ "$bb_state" = rewire ] || [ "$bb_state" = add ] || [ "$old_hook_count" -gt 0 ]; then
    if act; then
      cp -p "$SETTINGS" "$SETTINGS.pre-professor-$TS"
      if [ "$bb_state" = rewire ] || [ "$old_hook_count" -gt 0 ]; then
        jq --arg want "$BB_COMMAND" --arg old "$OLD_BINARY" --arg new "$PFM_BINARY" '
          (.hooks[]?[]?.hooks[]?.command) |=
            (if type == "string" and (. == $old or startswith($old + " "))
             then $new + .[($old | length):] else . end)
          |
          (.hooks.UserPromptSubmit[]?.hooks[]?
           | select((.command // "") == ($new + " bb"))
           | .command) = $want
          |
          (.hooks.UserPromptSubmit[]?.hooks[]?
           | select((.command // "") | test("bb-hook\\.sh"))
           | .command) = $want
          | (.hooks.UserPromptSubmit[]?.hooks[]?
             | select(.command == $want) | .type) = "command"
          | if any(.hooks.UserPromptSubmit[]?.hooks[]?; .command == $want)
            then .
            else .hooks.UserPromptSubmit += [{
              "matcher": "",
              "hooks": [{"type": "command", "command": $want}]
            }] end
        ' "$SETTINGS" > "$SETTINGS.tmp.$$" && mv "$SETTINGS.tmp.$$" "$SETTINGS"
      else
        jq --arg want "$BB_COMMAND" '
          .hooks //= {} | .hooks.UserPromptSubmit //= []
          | .hooks.UserPromptSubmit += [{
              "matcher": "",
              "hooks": [{"type": "command", "command": $want}]
            }]
        ' "$SETTINGS" > "$SETTINGS.tmp.$$" && mv "$SETTINGS.tmp.$$" "$SETTINGS"
      fi
    fi
  fi
fi
say ""

# ── the chat group receiver: migrate the existing opt-in hook to the binary surface. The
# receiver itself is fail-open: `pfm chat group hook` drains its payload and returns success
# even when the group backend cannot be reached. An install with no group hook stays opted out. ──
GROUP_COMMAND="$HOME/.local/bin/pfm chat group hook"
say "/chat group hook -> $SETTINGS"
if ! command -v jq >/dev/null 2>&1; then
  say "  skip    jq is not installed — rewire the group hook to '$GROUP_COMMAND' by hand"
  n_skip=$((n_skip+1))
elif [ ! -f "$SETTINGS" ]; then
  say "  skip    no settings.json yet — it is written by Claude Code, not by this installer"
  n_skip=$((n_skip+1))
else
  group_state="$(jq -r --arg want "$GROUP_COMMAND" '
    [ .hooks.UserPromptSubmit[]?.hooks[]?.command // empty ] as $commands
    | if ($commands | index($want)) then "ok"
      elif ($commands | map(select(test("chat/group\\.sh[ ]+hook$"))) | length) > 0 then "rewire"
      else "absent" end
  ' "$SETTINGS" 2>/dev/null)" || group_state=""
  case "$group_state" in
    ok)
      if [ "$DO" = uninstall ]; then
        say "  remove  the '$GROUP_COMMAND' UserPromptSubmit hook"; n_link=$((n_link+1))
      else
        say "  ok      the group hook already runs '$GROUP_COMMAND'"; n_ok=$((n_ok+1))
      fi ;;
    rewire)
      if [ "$DO" = uninstall ]; then
        say "  skip    the legacy group hook is not owned by this install"; n_skip=$((n_skip+1))
      else
        say "  rewire  the group.sh UserPromptSubmit hook -> '$GROUP_COMMAND'"; n_link=$((n_link+1))
      fi ;;
    absent)
      say "  ok      no group hook is configured"; n_ok=$((n_ok+1)) ;;
    *)
      say "  skip    could not read $SETTINGS as JSON — left group hook untouched"; n_skip=$((n_skip+1)) ;;
  esac
  if [ "$DO" = uninstall ] && [ "$group_state" = ok ]; then
    if act; then
      cp -p "$SETTINGS" "$SETTINGS.pre-professor-$TS"
      jq --arg want "$GROUP_COMMAND" '
        .hooks.UserPromptSubmit = [
          .hooks.UserPromptSubmit[]?
          | .hooks = [ .hooks[]? | select(.command != $want) ]
        ] | .hooks.UserPromptSubmit |= map(select((.hooks | length) > 0))
      ' "$SETTINGS" > "$SETTINGS.tmp.$$" && mv "$SETTINGS.tmp.$$" "$SETTINGS"
    fi
  elif [ "$DO" = install ] && [ "$group_state" = rewire ]; then
    if act; then
      cp -p "$SETTINGS" "$SETTINGS.pre-professor-$TS"
      jq --arg want "$GROUP_COMMAND" '
        (.hooks.UserPromptSubmit[]?.hooks[]?
         | select((.command // "") | test("chat/group\\.sh[ ]+hook$"))
         | .command) = $want
        | (.hooks.UserPromptSubmit[]?.hooks[]?
           | select(.command == $want) | .type) = "command"
      ' "$SETTINGS" > "$SETTINGS.tmp.$$" && mv "$SETTINGS.tmp.$$" "$SETTINGS"
    fi
  fi
fi
say ""

# ── native high-frequency wiring: `pfm statusline` replaces the 3-second shell/Python
# pipeline, and `pfm usage-hook` replaces the per-prompt usage shell. The host default plus
# every existing canonical account settings file converge together; physical-path de-duplication
# prevents ~/.cc/1 symlinks from rewriting the same file twice. A custom non-Professor
# statusline is left alone. Both native commands are fail-open at their CLI boundary. ──
STATUSLINE_COMMAND="$HOME/.local/bin/pfm statusline"
USAGE_COMMAND="$HOME/.local/bin/pfm usage-hook"

wire_native_settings() {
  local target="$1" status_command status_state usage_state changed
  say "native statusline + usage hook -> $target"
  if ! command -v jq >/dev/null 2>&1; then
    say "  skip    jq is not installed — wire '$STATUSLINE_COMMAND' and '$USAGE_COMMAND' by hand"
    n_skip=$((n_skip+1)); return
  fi
  if [ ! -f "$target" ]; then
    say "  skip    no settings.json at $target"
    n_skip=$((n_skip+1)); return
  fi
  if ! jq -e . "$target" >/dev/null 2>&1; then
    say "  skip    could not read $target as JSON — left native wiring untouched"
    n_skip=$((n_skip+1)); return
  fi

  status_command="$(jq -r '.statusLine.command // ""' "$target")"
  if [ "$DO" = uninstall ]; then
    [ "$status_command" = "$STATUSLINE_COMMAND" ] && status_state=remove || status_state=keep
  elif [ "$status_command" = "$STATUSLINE_COMMAND" ]; then
    status_state=ok
  elif [ -z "$status_command" ]; then
    status_state=add
  elif printf '%s' "$status_command" | grep -Eq 'statusline-command[.]sh([[:space:]]|$)'; then
    status_state=rewire
  else
    status_state=custom
  fi

  usage_state="$(jq -r --arg want "$USAGE_COMMAND" '
    [ .hooks.UserPromptSubmit[]?.hooks[]?.command // empty ] as $commands
    | if ($commands | index($want)) then "ok"
      elif ($commands | map(select(test("cc-usage-hook[.]sh([ ]|$)"))) | length) > 0 then "rewire"
      else "add" end
  ' "$target")"
  [ "$DO" = uninstall ] && { [ "$usage_state" = ok ] && usage_state=remove || usage_state=keep; }

  case "$status_state" in
    ok) say "  ok      statusline already runs '$STATUSLINE_COMMAND'"; n_ok=$((n_ok+1)) ;;
    add) say "  add     statusline running '$STATUSLINE_COMMAND'"; n_link=$((n_link+1)) ;;
    rewire) say "  rewire  statusline-command.sh -> '$STATUSLINE_COMMAND'"; n_link=$((n_link+1)) ;;
    remove) say "  remove  native statusline wiring"; n_link=$((n_link+1)) ;;
    custom) say "  skip    custom statusline command left untouched: $status_command"; n_skip=$((n_skip+1)) ;;
    keep) say "  ok      no native statusline wiring to remove"; n_ok=$((n_ok+1)) ;;
  esac
  case "$usage_state" in
    ok) say "  ok      usage hook already runs '$USAGE_COMMAND'"; n_ok=$((n_ok+1)) ;;
    add) say "  add     usage hook running '$USAGE_COMMAND'"; n_link=$((n_link+1)) ;;
    rewire) say "  rewire  cc-usage-hook.sh -> '$USAGE_COMMAND'"; n_link=$((n_link+1)) ;;
    remove) say "  remove  native usage hook wiring"; n_link=$((n_link+1)) ;;
    keep) say "  ok      no native usage hook wiring to remove"; n_ok=$((n_ok+1)) ;;
  esac

  changed=0
  case "$status_state" in add|rewire|remove) changed=1 ;; esac
  case "$usage_state" in add|rewire|remove) changed=1 ;; esac
  [ "$changed" = 1 ] || return
  if act; then
    cp -p "$target" "$target.pre-professor-$TS"
    if [ "$DO" = uninstall ]; then
      jq --arg status "$STATUSLINE_COMMAND" --arg usage "$USAGE_COMMAND" '
        if (.statusLine.command // "") == $status then del(.statusLine) else . end
        | .hooks.UserPromptSubmit = [
            .hooks.UserPromptSubmit[]?
            | .hooks = [ .hooks[]? | select(.command != $usage) ]
          ]
        | .hooks.UserPromptSubmit |= map(select((.hooks | length) > 0))
      ' "$target" > "$target.tmp.$$" && mv "$target.tmp.$$" "$target"
    else
      jq --arg status "$STATUSLINE_COMMAND" --arg usage "$USAGE_COMMAND" \
         --arg statusState "$status_state" '
        if $statusState == "add" then
          .statusLine = {
            "type": "command", "command": $status, "padding": 0,
            "refreshInterval": 3, "hideVimModeIndicator": true
          }
        elif $statusState == "rewire" then
          .statusLine.type = "command" | .statusLine.command = $status
        else . end
        | .hooks //= {} | .hooks.UserPromptSubmit //= []
        | (.hooks.UserPromptSubmit[]?.hooks[]?
           | select((.command // "") | test("cc-usage-hook[.]sh([ ]|$)"))
           | .command) = $usage
        | if any(.hooks.UserPromptSubmit[]?.hooks[]?; .command == $usage)
          then .
          else .hooks.UserPromptSubmit += [{
            "matcher": "", "hooks": [{"type": "command", "command": $usage}]
          }] end
        | (.hooks.UserPromptSubmit[]?.hooks[]?
           | select(.command == $usage) | .type) = "command"
      ' "$target" > "$target.tmp.$$" && mv "$target.tmp.$$" "$target"
    fi
  fi
}

settings_candidates="$SETTINGS"
if [ "$CLAUDE_DIR" = "$HOME/.claude" ]; then
  for account_dir in "$HOME/.cc/1" "$HOME/.cc/2" "$HOME/.cc/3" "$HOME/.cc/4"; do
    [ -f "$account_dir/settings.json" ] && settings_candidates="$settings_candidates $account_dir/settings.json"
  done
fi
settings_seen="|"
for native_settings in $settings_candidates; do
  settings_physical="$(readlink -f "$native_settings" 2>/dev/null || printf '%s' "$native_settings")"
  case "$settings_seen" in *"|$settings_physical|"*) continue ;; esac
  settings_seen="$settings_seen$settings_physical|"
  wire_native_settings "$native_settings"
done
say ""

# ── ~/.zshrc: source the launchers. One line, rewritten in place when the clone moves. ──
# The shim evals one-line actions from the Go engine; it aborts loudly if ~/.local/bin/pfm
# is missing.
#
# The engine is a PROGRAM, not a template, so it lives at the repo root rather than inside the
# bundle: $BUNDLE is <repo>/blueprint/templates/host-swap, three levels down from it. A clone
# that is only the bundle (no repo above it) keeps working — the fallback is the old in-bundle
# location, and a missing shim is reported rather than sourced blindly.
ENGINE="$(cd -P "$BUNDLE/../../.." 2>/dev/null && pwd)/pfm"
[ -r "$ENGINE/shim/pfm.zsh" ] || ENGINE="$BUNDLE/pfm"
SRC_LINE="[[ -r \"$ENGINE/shim/pfm.zsh\" ]] && source \"$ENGINE/shim/pfm.zsh\""
FLEET_SOURCE_RE='^[[:space:]]*([^#].*)?source[[:space:]].*(cc-fleet|pfm)[.]zsh'
PFM_SOURCE_RE='^[[:space:]]*([^#].*)?source[[:space:]].*pfm[.]zsh'
STALE_FLEET_COMMENT_RE='^[[:space:]]*# (The shim evals one-line actions from the Go engine \(~/.local/bin/cc-fleet\); the legacy|cc-fleet[.]zsh stays on disk unsourced as the parity checker)'
[ -r "$ENGINE/shim/pfm.zsh" ] || say "  warn    no pfm shim found at $ENGINE/shim/pfm.zsh"
[ -x "$HOME/.local/bin/pfm" ] || say "  warn    ~/.local/bin/pfm is not installed — the shim will refuse"
say "shell -> $ZSHRC"
if [ "$DO" = uninstall ]; then
  if [ -f "$ZSHRC" ] && grep -Eq "$PFM_SOURCE_RE" "$ZSHRC"; then
    say "  remove  the pfm.zsh source line"
    act && { cp -p "$ZSHRC" "$ZSHRC.pre-professor-$TS"; awk '!/^[[:space:]]*#/ && /source[[:space:]]/ && /pfm[.]zsh/ {next} {print}' "$ZSHRC" > "$ZSHRC.tmp.$$" && mv "$ZSHRC.tmp.$$" "$ZSHRC"; }
  else
    say "  ok      no source line present"; n_ok=$((n_ok+1))
  fi
elif [ ! -f "$ZSHRC" ]; then
  say "  create  $ZSHRC with the source line"
  act && printf '%s\n' "$SRC_LINE" > "$ZSHRC"
  n_link=$((n_link+1))
elif [ "$(grep -cF "$SRC_LINE" "$ZSHRC")" -eq 1 ] &&
     [ "$(grep -Ec "$FLEET_SOURCE_RE" "$ZSHRC")" -eq 1 ] &&
     ! grep -Eq "$STALE_FLEET_COMMENT_RE" "$ZSHRC"; then
  say "  ok      source line already points here"; n_ok=$((n_ok+1))
elif grep -Eq "$FLEET_SOURCE_RE" "$ZSHRC"; then
  # An older install pointed somewhere else. Rewrite in place — never append a second line, or
  # the shell sources two copies of the fleet and the later one silently wins.
  say "  rewrite the existing fleet source line to this bundle"
  act && { cp -p "$ZSHRC" "$ZSHRC.pre-professor-$TS"; awk -v repl="$SRC_LINE" '
    /^#[[:space:]]*The shim evals one-line actions from the Go engine \(~\/.local\/bin\/cc-fleet\); the legacy/ {
      print "# The shell launchers delegate to the pfm engine."
      current_comment=1
      next
    }
    /^#[[:space:]]*cc-fleet[.]zsh stays on disk unsourced as the parity checker/ {next}
    !/^[[:space:]]*#/ && /source[[:space:]]/ && /(cc-fleet|pfm)[.]zsh/ {
      if (!done) {
        if (!current_comment) print "# The shell launchers delegate to the pfm engine."
        print repl
      }
      done=1
      next
    }
    {print}
  ' "$ZSHRC" > "$ZSHRC.tmp.$$" && mv "$ZSHRC.tmp.$$" "$ZSHRC"; }
  n_backup=$((n_backup+1))
else
  say "  append  the source line"
  act && { cp -p "$ZSHRC" "$ZSHRC.pre-professor-$TS"; printf '\n# Professor fleet — launchers + the cc-ls chat picker\n%s\n' "$SRC_LINE" >> "$ZSHRC"; }
  n_link=$((n_link+1))
fi

say ""
say "ok=$n_ok  linked/changed=$n_link  backed-up=$n_backup  skipped=$n_skip"
if [ "$APPLY" = 0 ]; then
  say ""
  if [ "$DO" = uninstall ]; then say "Dry run — nothing changed. Run: $0 --uninstall --apply"
  else say "Dry run — nothing changed. Run: $0 --apply"; fi
elif [ "$DO" = uninstall ]; then
  say ""
  say "Uninstalled. Each link was either restored from its newest .pre-professor-<ts> backup or removed outright where none existed."
else
  say ""
  say "Installed. Open a new shell (or: source $ZSHRC) then run  cc-ls."
  say "Backups from this run carry the suffix .pre-professor-$TS"
fi
exit 0
