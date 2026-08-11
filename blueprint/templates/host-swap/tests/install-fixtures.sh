#!/usr/bin/env bash
# Fixtures for install.sh. Runs entirely inside a scratch HOME + scratch CLAUDE_CONFIG_DIR —
# it never reads or writes the operator's real ~/.claude, ~/.zshrc, or fleet state. The cases
# that matter are the destructive ones: a real file already sitting at the destination, a link
# left over from an older clone, and a ~/.zshrc that already sources a different copy.
set -uo pipefail
BUNDLE="$(cd "$(dirname "$0")/.." && pwd)"
T="$(mktemp -d)"
trap 'rm -rf "$T"' EXIT
pass=0; fail=0
ok(){ if [ "$2" = "$3" ]; then pass=$((pass+1)); printf '  ✓ %s\n' "$1"; else fail=$((fail+1)); printf '  ✗ %s\n     want=[%s]\n     got =[%s]\n' "$1" "$3" "$2"; fi; }

export HOME="$T/home"; mkdir -p "$HOME"
# jail systemd too: with HOME faked but the real session bus reachable, the installer's systemd
# section would daemon-reload the REAL user manager from inside the test. A dead runtime dir
# makes `systemctl --user` fail, which is exactly the installer's documented skip path.
export XDG_RUNTIME_DIR="$T/xdg-dead"
CLAUDE_DIR="$HOME/.claude"
CMD="$CLAUDE_DIR/commands"; BIN="$CLAUDE_DIR/bin"
# CLAUDE_CONFIG_DIR is deliberately set to a DIFFERENT account here and stays set for every run:
# the installer is normally invoked from inside a running chat, where that variable names that
# chat's account. It must be ignored — see the dedicated case below.
export CLAUDE_CONFIG_DIR="$HOME/.cc/2"
run() { bash "$BUNDLE/install.sh" "$@" 2>&1; }

echo "=== dry run changes nothing ==="
out="$(run)"
ok "dry run announces itself"      "$(printf '%s' "$out" | grep -c 'dry run')" "1"
ok "no bin dir created"            "$([ -d "$BIN" ] && echo yes || echo no)" "no"
ok "no commands dir created"       "$([ -d "$CMD" ] && echo yes || echo no)" "no"
ok "no zshrc created"              "$([ -f "$HOME/.zshrc" ] && echo yes || echo no)" "no"

echo "=== a chat's CLAUDE_CONFIG_DIR must NOT redirect the install ==="
# The bundle is host-level: one copy shared by every account. Installing into the running chat's
# account would put the scripts in ~/.cc/N/bin while /bb keeps looking in ~/.claude/bin — silent.
out="$(run)"
ok "targets the host default, not \$CLAUDE_CONFIG_DIR" "$(printf '%s' "$out" | grep -c "Target config dir: *$CLAUDE_DIR\$")" "1"
ok "does not name the account dir"  "$(printf '%s' "$out" | grep -c "Target config dir: *$HOME/.cc/2\$")" "0"
ok "--config-dir overrides on purpose" "$(run --config-dir "$T/elsewhere" | grep -c "Target config dir: *$T/elsewhere\$")" "1"

echo "=== --apply wires everything ==="
out="$(run --apply)"
ok "cc-db.sh is a symlink"         "$([ -L "$BIN/cc-db.sh" ] && echo yes || echo no)" "yes"
ok "cc-db.sh resolves to bundle"   "$(readlink -f "$BIN/cc-db.sh")" "$BUNDLE/cc-db.sh"
ok "cx-hide.sh linked"             "$([ -L "$BIN/cx-hide.sh" ] && echo yes || echo no)" "yes"
ok "bb.md linked (suffix dropped)" "$(readlink -f "$CMD/bb.md")" "$BUNDLE/bb.command.md"
ok "chat/ls.md linked"             "$(readlink -f "$CMD/chat/ls.md")" "$BUNDLE/chat/ls.command.md"
ok "chat/self/compact.md linked"   "$(readlink -f "$CMD/chat/self/compact.md")" "$BUNDLE/chat/self/compact.command.md"
ok "chat.sh linked"                "$(readlink -f "$CMD/chat/chat.sh")" "$BUNDLE/chat/chat.sh"
ok "zshrc sources the bundle"      "$(grep -c "$BUNDLE/cc-fleet.zsh" "$HOME/.zshrc")" "1"
ok "codex bb skill dir linked"     "$(readlink -f "$HOME/.agents/skills/bb")" "$BUNDLE/codex-skills/bb"

echo "=== the point of it all: edit the clone, the live command changes ==="
# THE BUNDLE UNDER TEST IS A COPY, NEVER THE CLONE. This wrote a sentinel line into the real
# blueprint file and removed it with a GNU-form `sed -i` — which BSD sed rejects (it reads the
# expression as the backup suffix and then waits on stdin), so on macOS the removal silently
# failed and the fixture left a sentinel behind in a tracked source file. A test that proves
# "editing the bundle changes the live command" must not prove it on the only copy that matters.
SANDBOX="$T/bundle"; cp -R "$BUNDLE" "$SANDBOX"
"$SANDBOX/install.sh" --apply >/dev/null 2>&1
echo "SENTINEL-$$" >> "$SANDBOX/chat/ls.command.md"
ok "live command sees the edit"    "$(grep -c "SENTINEL-$$" "$CMD/chat/ls.md")" "1"
# put it back exactly as it was — a temp file, not sed -i, so both seds behave the same
grep -v "SENTINEL-$$" "$SANDBOX/chat/ls.command.md" > "$SANDBOX/.ls.tmp" && mv "$SANDBOX/.ls.tmp" "$SANDBOX/chat/ls.command.md"
ok "sentinel removed again"        "$(grep -c "SENTINEL-$$" "$SANDBOX/chat/ls.command.md")" "0"
ok "the real clone was untouched"  "$(grep -c "SENTINEL-" "$BUNDLE/chat/ls.command.md")" "0"
# The sandbox install re-pointed every link at $SANDBOX. Put them back on $BUNDLE before the
# idempotence check, or that check measures the relink THIS TEST just caused instead of the
# no-op it means to prove.
run --apply >/dev/null 2>&1

echo "=== re-running is free (idempotent) ==="
out="$(run --apply)"
ok "second apply relinks nothing"  "$(printf '%s' "$out" | grep -cE '^  (link|relink|backup) ')" "0"
ok "second apply reports ok rows"  "$([ "$(printf '%s' "$out" | grep -c '^  ok ')" -gt 20 ] && echo many || echo few)" "many"
ok "zshrc still has ONE source line" "$(grep -c 'cc-fleet\.zsh' "$HOME/.zshrc")" "1"

echo "=== a REAL file at the destination is preserved, never destroyed ==="
rm -f "$CMD/chat/ls.md"; printf 'my own version\n' > "$CMD/chat/ls.md"
out="$(run --apply)"
bk="$(ls -1 "$CMD/chat/"ls.md.pre-professor-* 2>/dev/null | tail -1)"
ok "the real file was backed up"   "$([ -n "$bk" ] && echo yes || echo no)" "yes"
ok "backup keeps the content"      "$(cat "$bk" 2>/dev/null)" "my own version"
ok "destination is now the link"   "$(readlink -f "$CMD/chat/ls.md")" "$BUNDLE/chat/ls.command.md"

echo "=== a link from an OLDER clone is repointed ==="
mkdir -p "$T/oldclone"; printf 'old\n' > "$T/oldclone/cc-db.sh"
ln -sfn "$T/oldclone/cc-db.sh" "$BIN/cc-db.sh"
out="$(run --apply)"
ok "stale link was relinked"       "$(printf '%s' "$out" | grep -c 'relink')" "1"
ok "now points at this bundle"     "$(readlink -f "$BIN/cc-db.sh")" "$BUNDLE/cc-db.sh"

echo "=== a ~/.zshrc pointing at another copy is REWRITTEN, not appended to ==="
printf '# my shell\n[[ -r "/somewhere/else/cc-fleet.zsh" ]] && source "/somewhere/else/cc-fleet.zsh"\nalias x=y\n' > "$HOME/.zshrc"
out="$(run --apply)"
ok "still exactly one source line" "$(grep -c 'cc-fleet\.zsh' "$HOME/.zshrc")" "1"
ok "it points at this bundle"      "$(grep -c "$BUNDLE/cc-fleet.zsh" "$HOME/.zshrc")" "1"
ok "unrelated lines survive"       "$(grep -c 'alias x=y' "$HOME/.zshrc")" "1"
# EXISTENCE, not a count: the property is "the file it rewrote was preserved first". Earlier
# sections in this fixture also apply, and each rewrite banks its own timestamped backup, so a
# count is really an assertion about how many times this test file happens to call the installer
# — it broke the moment a new case was added above, pointing at install.sh, which was innocent.
ok "the old zshrc was backed up"   "$([ -n "$(ls -1 "$HOME/".zshrc.pre-professor-* 2>/dev/null)" ] && echo yes || echo no)" "yes"

echo "=== bare --uninstall dry-runs by default: banner, and NOTHING changes ==="
out="$(run --uninstall)"
ok "banner names the action (uninstall)"     "$(printf '%s' "$out" | grep -c '^MODE: uninstall$')" "1"
ok "banner names it a dry run"               "$(printf '%s' "$out" | grep -c 'dry run')" "1"
ok "bare --uninstall: cc-hide.sh still linked" "$([ -L "$BIN/cc-hide.sh" ] && echo yes || echo no)" "yes"
ok "bare --uninstall: chat/ls.md still linked" "$([ -L "$CMD/chat/ls.md" ] && echo yes || echo no)" "yes"

echo "=== --uninstall --apply: a link with NO backup is DROPPED, a link WITH one is RESTORED ==="
# cc-hide.sh has never had a competing real file placed at its destination (only chat/ls.md did,
# in the "REAL file at the destination" case above) -- so it carries no .pre-professor-* backup
# and must be dropped outright, while chat/ls.md's backup must come back byte for byte. ONE
# uninstall run covers the whole fleet at once, so both "before" snapshots are taken first and
# both outcomes are checked against that SAME run.
hide_target_before="$([ -L "$BIN/cc-hide.sh" ] && readlink -f "$BIN/cc-hide.sh" || echo none)"
ls_bk_content_before="$(cat "$(ls -1 "$CMD/chat/"ls.md.pre-professor-* 2>/dev/null | tail -1)" 2>/dev/null)"
run --uninstall --apply >/dev/null
hide_after="$([ -e "$BIN/cc-hide.sh" ] && echo "still-present:$(readlink -f "$BIN/cc-hide.sh" 2>/dev/null)" || echo gone)"
ok "cc-hide.sh had a real symlink before"    "$hide_target_before" "$BUNDLE/cc-hide.sh"
ok "cc-hide.sh (no backup) removed, not silently reinstalled" "$hide_after" "gone"
ok "chat/ls.md is no longer a symlink (restored in place)" "$([ -L "$CMD/chat/ls.md" ] && echo still-a-link || echo restored)" "restored"
ok "restored content matches the pre-install backup"       "$(cat "$CMD/chat/ls.md" 2>/dev/null)" "$ls_bk_content_before"

echo "=== --apply --uninstall (OPPOSITE flag order) behaves IDENTICALLY ==="
run --apply >/dev/null   # re-link everything (including cc-hide.sh) before the order check
ok "re-apply relinked cc-hide.sh" "$([ -L "$BIN/cc-hide.sh" ] && echo yes || echo no)" "yes"
run --apply --uninstall >/dev/null
ok "--apply --uninstall also removes cc-hide.sh (no backup)" "$([ -e "$BIN/cc-hide.sh" ] && echo still-present || echo gone)" "gone"

# leave a clean, linked install behind — this is the last section in the file, but a fixture
# should never end mid-uninstall in case a case is appended below later.
run --apply >/dev/null

echo
printf 'PASS %d   FAIL %d\n' "$pass" "$fail"
[ "$fail" = 0 ]
