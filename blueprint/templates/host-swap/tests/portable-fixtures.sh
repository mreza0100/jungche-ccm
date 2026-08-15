#!/usr/bin/env bash
# Fixtures for cc-portable.sh — the GNU/BSD seam.
#
# The seam's whole promise is that a caller never has to know which platform it is on. When that
# promise is broken the failure is silent: a scan returns nothing, or worse, returns something
# SHAPED wrong that the caller happily parses into garbage and persists. The bug these fixtures
# were written for did the second thing — cc_find_meta's mtime field carried a fraction on GNU
# and not on BSD, both callers stripped it with `${line%%.*}`, and on a Mac that pattern cut at
# the first dot in the PATH instead. mt came back as "<mtime><TAB><size><TAB>$HOME/", went
# into the chat cache with every field shifted one to the right, and every affected chat then
# read as prompts=0 — which the picker HIDES. cc-ls listed nothing and reported no error.
#
# The assertions here are mostly about the portable seam's output shape.
set -uo pipefail
BUNDLE="$(cd "$(dirname "$0")/.." && pwd)"
# macOS resolves /var through /private/var; resolve the scratch dir the same way the shipped
# code does, or a path assertion fails while naming the same directory twice.
T="$(cd -P "$(mktemp -d)" && pwd)"
trap 'rm -rf "$T"' EXIT
pass=0; fail=0
ok(){ if [ "$2" = "$3" ]; then pass=$((pass+1)); printf '  ✓ %s\n' "$1"; else fail=$((fail+1)); printf '  ✗ %s\n     want=[%s]\n     got =[%s]\n' "$1" "$3" "$2"; fi; }

. "$BUNDLE/cc-portable.sh"

# A store shaped like the real one: dots in the directory names AND in the file names, which is
# what made the old parse cut in the wrong place. Distinct mtimes, oldest first on disk.
mkdir -p "$T/.claude/projects/-Users-x--claude" "$T/.claude/projects/-Users-x-work"
OLD="$T/.claude/projects/-Users-x--claude/aaaaaaaa-0000-0000-0000-000000000001.jsonl"
MID="$T/.claude/projects/-Users-x-work/bbbbbbbb-0000-0000-0000-000000000002.jsonl"
NEW="$T/.claude/projects/-Users-x-work/cccccccc-0000-0000-0000-000000000003.jsonl"
printf 'x\n'      > "$OLD"
printf 'xxxxx\n'  > "$MID"
printf 'xxxxxxx\n'> "$NEW"
# stagger the mtimes without `touch -d @EPOCH` (a GNU form BSD touch rejects) — cc_mtime reads
# whatever the filesystem records, and three writes seconds apart are enough to order them.
touch "$OLD"; sleep 1; touch "$MID"; sleep 1; touch "$NEW"

echo "=== cc_find_meta — the shape every caller depends on ==="
SCAN="$(cc_find_meta "$T/.claude/projects" -maxdepth 2 -name '*.jsonl')"
ok "three rows"                 "$(printf '%s\n' "$SCAN" | grep -c .)" "3"
ok "newest first"               "$(printf '%s\n' "$SCAN" | head -1 | cut -f3)" "$NEW"
ok "oldest last"                "$(printf '%s\n' "$SCAN" | tail -1 | cut -f3)" "$OLD"
ok "field 2 is the byte count"  "$(printf '%s\n' "$SCAN" | head -1 | cut -f2)" "$(cc_size "$NEW")"
ok "field 3 survives dots"      "$(printf '%s\n' "$SCAN" | tail -1 | cut -f3)" "$OLD"
ok "exactly three fields"       "$(printf '%s\n' "$SCAN" | awk -F'\t' '{print NF}' | sort -u | tr -d '\n')" "3"

# THE regression. Field 1 must be a bare integer on EVERY platform — that is the normalization
# the seam exists to provide. GNU's `%T@` carries a fraction and BSD's `%m` does not; if that
# difference ever reaches a caller again, this fails.
ok "field 1 is an integer epoch" \
   "$(printf '%s\n' "$SCAN" | cut -f1 | grep -cE '^[0-9]+$')" "3"
ok "field 1 carries no fraction" \
   "$(printf '%s\n' "$SCAN" | cut -f1 | grep -c '\.')" "0"

echo "=== cc_find_meta — the empty answers ==="
ok "missing dir is empty, not an error" "$(cc_find_meta "$T/nope" -name '*.jsonl' | grep -c .)" "0"
ok "missing dir exits 0"                "$(cc_find_meta "$T/nope" -name '*.jsonl' >/dev/null; echo $?)" "0"
ok "no matches is empty"                "$(cc_find_meta "$T/.claude/projects" -name '*.nomatch' | grep -c .)" "0"

echo "=== cc_find_newest — paths only ==="
ok "paths only, newest first" "$(cc_find_newest "$T/.claude/projects" -maxdepth 2 -name '*.jsonl' | head -1)" "$NEW"
ok "no tabs in the output"    "$(cc_find_newest "$T/.claude/projects" -maxdepth 2 -name '*.jsonl' | grep -c "$(printf '\t')")" "0"

echo "=== cc_mtime / cc_size — dots in the name must not matter ==="
ok "cc_mtime is an integer"  "$(cc_mtime "$NEW" | grep -cE '^[0-9]+$')" "1"
ok "cc_size matches wc -c"   "$(cc_size "$NEW")" "$(wc -c < "$NEW" | tr -d ' ')"
ok "cc_mtime on a missing file is empty" "$(cc_mtime "$T/nope" | grep -c .)" "0"
ok "cc_size on a missing file is empty"  "$(cc_size  "$T/nope" | grep -c .)" "0"

echo "=== grep -f with an EMPTY pattern file is not portable ==="
# cc-ls filters its reload through a hide-list file that is empty whenever nothing is hidden —
# the common case. BSD and GNU grep read an empty pattern file as "no patterns" (so -v passes
# everything); ugrep, which a Homebrew `grep` on PATH can be, does the opposite and passes
# NOTHING. The picker must therefore never hand an empty file to grep -f; it tests -s first.
# This asserts the guarded form, which is the only form that is the same on every grep.
: > "$T/empty.pat"
printf 'row-a\nrow-b\n' > "$T/rows"
guarded_v(){ if [ -s "$1" ]; then grep -vFf "$1"; else cat; fi; }
ok "empty hide list drops nothing"  "$(guarded_v "$T/empty.pat" < "$T/rows" | grep -c .)" "2"
printf 'row-a\n' > "$T/one.pat"
ok "non-empty hide list drops that row" "$(guarded_v "$T/one.pat" < "$T/rows" | grep -c .)" "1"
echo "=== zsh: re-declaring a local without a value PRINTS it ==="
# Not a GNU/BSD split — a zsh one, and it lands in the same place: on the picker's stdout.
# `local name` with no assignment, for a name ALREADY local in this scope, is a LISTING request,
# so it emits "name=value". Declared inside cc-ls's per-pane loop that is one junk line per pane
# printed straight into the picker's output, which is exactly how it was found.
ok "the trap is real" \
   "$(zsh -c 'f(){ local a; a=1; local a; }; f' 2>&1)" "a=1"
ok "an assignment suppresses it" \
   "$(zsh -c 'f(){ local a; a=1; local a=2; }; f' 2>&1)" ""
echo "=== cc_sed_i / cc_timeout / cc_detach ==="
printf 'alpha\nbeta\n' > "$T/edit.txt"
cc_sed_i 's/alpha/ALPHA/' "$T/edit.txt"
ok "cc_sed_i edited in place"    "$(head -1 "$T/edit.txt")" "ALPHA"
ok "cc_sed_i left no backup"     "$(ls "$T" | grep -c 'edit.txt.')" "0"
ok "cc_timeout runs the command" "$(cc_timeout 5 printf 'ran')" "ran"
ok "cc_detach names a real command" "$(command -v "$(cc_detach)" >/dev/null && echo yes || echo no)" "yes"

echo
if [ "$fail" -eq 0 ]; then printf 'PASS %-4s FAIL %s\n' "$pass" "$fail"; else printf 'PASS %-4s FAIL %s\n' "$pass" "$fail"; exit 1; fi
