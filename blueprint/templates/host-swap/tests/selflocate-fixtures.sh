#!/usr/bin/env bash
# Fixtures for PFM_HOME — the bundle's self-location. The install layer symlinks these
# scripts into ~/.claude/bin, so a resolver that stops at the first dirname silently points the
# whole fleet at the wrong tree: cc-db.sh "not found", hidden chats resurrecting, no error.
# Every case below runs the EXACT lines the shipped scripts carry (extracted, never retyped) so
# the test cannot drift away from the code it guards.
set -uo pipefail
BUNDLE="$(cd "$(dirname "$0")/.." && pwd)"
# macOS resolves /var through a symlink to /private/var, and every resolver under test resolves
# symlinks BY DESIGN — so the expected value has to be the resolved spelling too, or the test
# reports a failure whose "want" and "got" name the same directory. cd -P is how the shipped
# code spells it; spell it the same way here.
T="$(cd -P "$(mktemp -d)" && pwd)"
trap 'rm -rf "$T"' EXIT
pass=0; fail=0
ok(){ if [ "$2" = "$3" ]; then pass=$((pass+1)); printf '  ✓ %s\n' "$1"; else fail=$((fail+1)); printf '  ✗ %s\n     want=[%s]\n     got =[%s]\n' "$1" "$3" "$2"; fi; }

# The shipped resolver, lifted verbatim out of cc-db.sh (the walk line + the assignment). Any
# bash satellite carries the same two lines; cc-db.sh is the one every other one depends on.
BASH_RESOLVER="$(grep -E '^_pfms=|^PFM_HOME=' "$BUNDLE/cc-db.sh")"
[ -n "$BASH_RESOLVER" ] || { echo "FATAL: could not extract the bash resolver from cc-db.sh"; exit 1; }
mkdir -p "$T/real" "$T/links" "$T/links2" "$T/rel"
printf '#!/usr/bin/env bash\n%s\necho "$PFM_HOME"\n' "$BASH_RESOLVER" > "$T/real/probe.sh"
chmod +x "$T/real/probe.sh"

echo "=== bash: direct invocation ==="
ok "runs from its own dir" "$(bash "$T/real/probe.sh")" "$T/real"

echo "=== bash: through a symlink (the install shape) ==="
ln -sf "$T/real/probe.sh" "$T/links/probe.sh"
ok "absolute symlink resolves to the real dir" "$(bash "$T/links/probe.sh")" "$T/real"

echo "=== bash: chained symlink ==="
ln -sf "$T/links/probe.sh" "$T/links2/probe.sh"
ok "link -> link -> file walks the whole chain" "$(bash "$T/links2/probe.sh")" "$T/real"

echo "=== bash: RELATIVE symlink target ==="
# git checkouts and hand-made links often store a relative target; a resolver that forgets to
# re-anchor it against the link's own directory lands in $PWD instead.
ln -sf "../real/probe.sh" "$T/rel/probe.sh"
ok "relative target re-anchors on the link's dir" "$(cd / && bash "$T/rel/probe.sh")" "$T/real"

echo "=== bash: env override wins ==="
ok "PFM_HOME from the env is respected" "$(PFM_HOME=/opt/fleet bash "$T/links/probe.sh")" "/opt/fleet"

echo "=== bash: a path with spaces ==="
mkdir -p "$T/dir with spaces" "$T/links3"
cp "$T/real/probe.sh" "$T/dir with spaces/probe.sh"
ln -sf "$T/dir with spaces/probe.sh" "$T/links3/probe.sh"
ok "spaces in the resolved path survive" "$(bash "$T/links3/probe.sh")" "$T/dir with spaces"

echo "=== the shipped bundle locates its own siblings ==="
# The point of the whole exercise: from the resolved home, every sibling the scripts call must exist.
for sib in cc-db.sh cc-swap-chat.sh cc-agent-open.sh; do
  ok "sibling present: $sib" "$([ -f "$BUNDLE/$sib" ] && echo yes || echo no)" "yes"
done
# The invariant that matters, stated without naming any one install path: no script may reach a
# sibling through a hand-written absolute path — that is exactly what breaks when the bundle moves.
ok "siblings are reached only via PFM_HOME" "$(grep -lE '\$HOME/[^"]*/(cc-db|cc-swap-chat|cc-agent-open)\.sh' "$BUNDLE"/*.sh "$BUNDLE"/*.zsh 2>/dev/null | wc -l | tr -d ' ')" "0"

echo
printf 'PASS %d   FAIL %d\n' "$pass" "$fail"
[ "$fail" = 0 ]
