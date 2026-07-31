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
export CLAUDE_CONFIG_DIR="$HOME/.claude"
CMD="$CLAUDE_CONFIG_DIR/commands"; BIN="$CLAUDE_CONFIG_DIR/bin"
run() { bash "$BUNDLE/install.sh" "$@" 2>&1; }

echo "=== dry run changes nothing ==="
out="$(run)"
ok "dry run announces itself"      "$(printf '%s' "$out" | grep -c 'dry run')" "1"
ok "no bin dir created"            "$([ -d "$BIN" ] && echo yes || echo no)" "no"
ok "no commands dir created"       "$([ -d "$CMD" ] && echo yes || echo no)" "no"
ok "no zshrc created"              "$([ -f "$HOME/.zshrc" ] && echo yes || echo no)" "no"

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

echo "=== the point of it all: edit the clone, the live command changes ==="
echo "SENTINEL-$$" >> "$BUNDLE/chat/ls.command.md"
ok "live command sees the edit"    "$(grep -c "SENTINEL-$$" "$CMD/chat/ls.md")" "1"
# put the bundle back exactly as it was
sed -i "/SENTINEL-$$/d" "$BUNDLE/chat/ls.command.md"
ok "sentinel removed again"        "$(grep -c "SENTINEL-$$" "$BUNDLE/chat/ls.command.md")" "0"

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
ok "the old zshrc was backed up"   "$(ls -1 "$HOME/".zshrc.pre-professor-* 2>/dev/null | wc -l)" "1"

echo "=== --uninstall puts the operator's files back ==="
out="$(run --uninstall --apply 2>/dev/null || run --uninstall)"
# --uninstall is itself a dry run unless applied; drive it explicitly
MODE_OUT="$(bash "$BUNDLE/install.sh" --uninstall 2>&1)"
ok "uninstall dry-runs by default" "$([ -L "$BIN/cc-hide.sh" ] && echo yes || echo no)" "yes"

echo
printf 'PASS %d   FAIL %d\n' "$pass" "$fail"
[ "$fail" = 0 ]
