#!/usr/bin/env bash
# Fixtures for cc-db.sh. Uses a SCRATCH database and scratch legacy files — never ~/.cc/fleet.db,
# never ~/.claude. Every legacy path is redirected by running with a fake HOME.
set -uo pipefail
CC="$(cd "$(dirname "$0")/.." && pwd)/cc-db.sh"
T="$(mktemp -d)"; export HOME="$T"
mkdir -p "$T/.claude" "$T/.cc"
export CC_FLEET_DB="$T/.cc/fleet.db"
pass=0; fail=0
# n() — a count that means the same thing on both platforms. BSD `wc -l` right-pads its output
# ("       2"), GNU does not, so every `ok "…" "$(… | wc -l | n)" "2"` in this file compared a padded
# string to a bare one and failed on macOS while the code under test was perfectly correct. A
# fixture that cries wolf on one platform is worse than no fixture: it trains you to skim the
# failures. Pipe every count through this.
n(){ tr -d ' '; }
ok(){ if [ "$2" = "$3" ]; then pass=$((pass+1)); printf '  ✓ %s\n' "$1"; else fail=$((fail+1)); printf '  ✗ %s\n     want=[%s]\n     got =[%s]\n' "$1" "$3" "$2"; fi; }

echo "=== 1. init + schema ==="
bash "$CC" init >/dev/null 2>&1
ok "db file created" "$([ -f "$CC_FLEET_DB" ] && echo yes)" "yes"
ok "tables present" "$(sqlite3 "$CC_FLEET_DB" "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('meta','hidden','children','swap_event','chat');")" "5"
ok "WAL mode" "$(sqlite3 "$CC_FLEET_DB" 'PRAGMA journal_mode;')" "wal"
bash "$CC" init >/dev/null 2>&1
ok "init is idempotent" "$?" "0"

echo "=== 2. hidden round-trip ==="
bash "$CC" hidden-add aaaaaaaa-0000-0000-0000-000000000001 >/dev/null
bash "$CC" hidden-add aaaaaaaa-0000-0000-0000-000000000002 12345 >/dev/null
ok "two hidden" "$(bash "$CC" hidden-list | wc -l | n)" "2"
bash "$CC" hidden-has aaaaaaaa-0000-0000-0000-000000000001; ok "hidden-has finds it" "$?" "0"
bash "$CC" hidden-has ffffffff-0000-0000-0000-00000000ffff; ok "hidden-has rejects unknown" "$?" "1"
ok "payload stored" "$(sqlite3 "$CC_FLEET_DB" "SELECT at_payload FROM hidden WHERE uuid='aaaaaaaa-0000-0000-0000-000000000002';")" "12345"
bash "$CC" hidden-del aaaaaaaa-0000-0000-0000-000000000001 >/dev/null
ok "after delete" "$(bash "$CC" hidden-list | wc -l | n)" "1"
bash "$CC" hidden-add aaaaaaaa-0000-0000-0000-000000000002 >/dev/null
ok "re-add is not a duplicate" "$(bash "$CC" hidden-list | wc -l | n)" "1"

echo "=== 3. THE RACE — 20 concurrent writers, the bug that started this ==="
sqlite3 "$CC_FLEET_DB" "DELETE FROM hidden;"
for i in $(seq 1 20); do
  bash "$CC" hidden-add "bbbbbbbb-0000-0000-0000-$(printf '%012d' "$i")" &
done
wait
ok "all 20 concurrent hides survived" "$(bash "$CC" hidden-list | wc -l | n)" "20"

echo "=== 3b. legacy file behaviour under the same load (proves the bug was real) ==="
LEG="$T/.claude/legacy-hidden"; : > "$LEG"
for i in $(seq 1 20); do
  ( cur="$(cat "$LEG")"; sleep 0.01; { [ -n "$cur" ] && printf '%s\n' "$cur"; printf 'x%02d\n' "$i"; } > "$LEG" ) &
done
wait
legacy_kept="$(grep -c . "$LEG" 2>/dev/null || echo 0)"
printf '  legacy read-modify-write kept %s of 20 (lost %s)\n' "$legacy_kept" "$((20-legacy_kept))"
ok "db beats legacy under concurrency" "$([ "$(bash "$CC" hidden-list | wc -l | n)" -gt "$legacy_kept" ] && echo yes || echo no)" "yes"

echo "=== 4. hidden-reap is one transaction ==="
printf 'bbbbbbbb-0000-0000-0000-000000000001\nbbbbbbbb-0000-0000-0000-000000000002\n' | bash "$CC" hidden-reap
ok "reap kept only the live set" "$(bash "$CC" hidden-list | wc -l | n)" "2"

echo "=== 5. primary account ==="
ok "default primary" "$(bash "$CC" primary-get)" "1"
bash "$CC" primary-set 3 >/dev/null
ok "primary set to 3" "$(bash "$CC" primary-get)" "3"
ok "legacy file kept in lockstep" "$(cat "$T/.claude-primary" 2>/dev/null)" "3"
bash "$CC" primary-set 9 >/dev/null 2>&1; ok "rejects out-of-range" "$?" "2"

echo "=== 6. children ==="
bash "$CC" child-add pane sess-1 cc-sock-a >/dev/null
bash "$CC" child-add pane sess-1 cc-sock-b >/dev/null
bash "$CC" child-add new  sess-1 cc-sock-c >/dev/null
ok "pane children for sess-1" "$(bash "$CC" child-list pane sess-1 | wc -l | n)" "2"
ok "new children are a separate kind" "$(bash "$CC" child-list new sess-1 | wc -l | n)" "1"
bash "$CC" child-add pane sess-1 cc-sock-a >/dev/null
ok "duplicate child ignored" "$(bash "$CC" child-list pane sess-1 | wc -l | n)" "2"

echo "=== 7. FALLBACK — db gone, legacy path still works ==="
mv "$CC_FLEET_DB" "$CC_FLEET_DB.away"
printf 'cccccccc-0000-0000-0000-000000000001\n' > "$T/.claude/.cc-ls-hidden"
ok "reads legacy file when db absent" "$(bash "$CC" hidden-list | wc -l | n)" "1"
bash "$CC" hidden-add cccccccc-0000-0000-0000-000000000002 >/dev/null
ok "writes legacy file when db absent" "$(grep -c . "$T/.claude/.cc-ls-hidden")" "2"
bash "$CC" hidden-has cccccccc-0000-0000-0000-000000000002; ok "hidden-has via legacy" "$?" "0"
mv "$CC_FLEET_DB.away" "$CC_FLEET_DB"

echo "=== 8. migrate is idempotent and non-destructive ==="
rm -f "$CC_FLEET_DB"; rm -rf "$T/.claude" "$T/.cc"; mkdir -p "$T/.claude" "$T/.cc"
printf 'dddddddd-0000-0000-0000-000000000001\ndddddddd-0000-0000-0000-000000000002\n' > "$T/.claude/.cc-ls-hidden"
printf 'dddddddd-0000-0000-0000-000000000001\t999\n' > "$T/.claude/.cc-ls-hidden.at"
printf '2\n' > "$T/.claude-primary"
mkdir -p "$T/.claude/.cc-pane-children"; printf 'sock-x\n' > "$T/.claude/.cc-pane-children/s1"
printf 'swap line one\n' > "$T/.claude/.cc-swap.log"
bash "$CC" migrate >/dev/null 2>&1
ok "hidden migrated" "$(bash "$CC" hidden-list | wc -l | n)" "2"
ok "payload migrated" "$(sqlite3 "$CC_FLEET_DB" "SELECT at_payload FROM hidden WHERE uuid='dddddddd-0000-0000-0000-000000000001';")" "999"
ok "primary migrated" "$(bash "$CC" primary-get)" "2"
ok "children migrated" "$(bash "$CC" child-list pane s1 | wc -l | n)" "1"
ok "swap migrated" "$(sqlite3 "$CC_FLEET_DB" 'SELECT COUNT(*) FROM swap_event;')" "1"
ok "original renamed aside, NOT deleted" "$(ls -A "$T/.claude/" | grep -c 'cc-ls-hidden.pre-db-')" "1"
ok "lock file removed" "$([ -e "$T/.claude/.cc-ls-hidden.lock" ] && echo present || echo gone)" "gone"
bash "$CC" migrate >/dev/null 2>&1
ok "second migrate is a no-op" "$(bash "$CC" hidden-list | wc -l | n)" "2"

echo "=== 9. chat index — EMPTY FIELDS must survive (tab is IFS-whitespace in bash) ==="
bash "$CC" init >/dev/null 2>&1
printf 'e1111111-0000-0000-0000-000000000001\t100\t200\t/some/cwd\tMY_LABEL\t7\n' > "$T/in.tsv"
printf 'e2222222-0000-0000-0000-000000000002\t101\t201\t0\t\t0\n' >> "$T/in.tsv"
bash "$CC" chat-save < "$T/in.tsv"
ok "two chat rows" "$(bash "$CC" chat-load | wc -l | n)" "2"
ok "label preserved" "$(sqlite3 "$CC_FLEET_DB" "SELECT label FROM chat WHERE uuid='e1111111-0000-0000-0000-000000000001';")" "MY_LABEL"
ok "prompts preserved" "$(sqlite3 "$CC_FLEET_DB" "SELECT prompts FROM chat WHERE uuid='e1111111-0000-0000-0000-000000000001';")" "7"
ok "EMPTY label stays empty, not shifted" "$(sqlite3 "$CC_FLEET_DB" "SELECT label FROM chat WHERE uuid='e2222222-0000-0000-0000-000000000002';")" ""
ok "count after empty label is 0, not shifted" "$(sqlite3 "$CC_FLEET_DB" "SELECT prompts FROM chat WHERE uuid='e2222222-0000-0000-0000-000000000002';")" "0"
bash "$CC" chat-load | sort > "$T/r1"; bash "$CC" chat-save < "$T/r1"; bash "$CC" chat-load | sort > "$T/r2"
ok "load->save->load is stable" "$(diff -q "$T/r1" "$T/r2" >/dev/null && echo same || echo differ)" "same"
bash "$CC" chat-prune
ok "prune drops rows with no transcript" "$(bash "$CC" chat-load | wc -l | n)" "0"

echo "=== 9b. a record with MORE than six fields is refused, not stored shifted ==="
# How the poison got in: a caller packed a tab-bearing value into one field, every field to its
# right shifted, and the row landed with cwd=<path prefix>, label=<byte count> and prompts=0.
# A 0-prompt row is one the picker HIDES, so the chat vanished and stayed vanished — the cache
# entry still matched the file's mtime and size, so no later run re-parsed it. Refusing the row
# costs one re-parse. Storing it costs the chat.
printf 'f1111111-0000-0000-0000-000000000001\t100\t200\t/good/cwd\tGOOD\t5\n' > "$T/in2.tsv"
printf 'f2222222-0000-0000-0000-000000000002\t100\t200\t/x\t200\t/bad/cwd\tBAD\t0\n' >> "$T/in2.tsv"
bash "$CC" chat-save < "$T/in2.tsv"
ok "the well-formed row is stored" "$(sqlite3 "$CC_FLEET_DB" "SELECT label FROM chat WHERE uuid='f1111111-0000-0000-0000-000000000001';")" "GOOD"
ok "the over-long row is refused"  "$(sqlite3 "$CC_FLEET_DB" "SELECT COUNT(*) FROM chat WHERE uuid='f2222222-0000-0000-0000-000000000002';")" "0"
sqlite3 "$CC_FLEET_DB" "DELETE FROM chat;"

echo "=== 9c. chat_gen — a cache written by a shipped bug clears itself once ==="
# The chat table is 100% derived, so the honest repair for rows a bug wrote is to drop them and
# let the next scan rebuild. The marker makes that happen exactly once per host.
printf 'a1111111-0000-0000-0000-000000000001\t100\t200\t/c\tL\t5\n' > "$T/in3.tsv"
bash "$CC" chat-save < "$T/in3.tsv"
ok "stamped db keeps its rows across a load" "$(bash "$CC" chat-load | wc -l | n)" "1"
sqlite3 "$CC_FLEET_DB" "DELETE FROM meta WHERE key='chat_gen';"   # a db from before the marker
ok "unstamped db is purged on load"          "$(bash "$CC" chat-load | wc -l | n)" "0"
ok "and is stamped, so it purges only once"  "$(sqlite3 "$CC_FLEET_DB" "SELECT val FROM meta WHERE key='chat_gen';")" "2"
bash "$CC" chat-save < "$T/in3.tsv"
ok "rows written after the purge survive"    "$(bash "$CC" chat-load | wc -l | n)" "1"
# init must NOT stamp a db that already holds rows — migrate calls init, and an adopter running
# migrate again would otherwise certify the poison as current and suppress the purge forever.
sqlite3 "$CC_FLEET_DB" "DELETE FROM meta WHERE key='chat_gen';"
bash "$CC" init >/dev/null 2>&1
ok "init leaves a populated db unstamped"    "$(sqlite3 "$CC_FLEET_DB" "SELECT COUNT(*) FROM meta WHERE key='chat_gen';")" "0"
sqlite3 "$CC_FLEET_DB" "DELETE FROM chat;"
bash "$CC" init >/dev/null 2>&1
ok "init stamps an empty db, so a fresh install never purges" "$(sqlite3 "$CC_FLEET_DB" "SELECT val FROM meta WHERE key='chat_gen';")" "2"

echo "=== 10. /bb two-step hide must not lose the auto-unhide baseline ==="
bash "$CC" init >/dev/null 2>&1
sqlite3 "$CC_FLEET_DB" "DELETE FROM hidden;"
bash "$CC" hidden-add bb-0000-0000-0000-000000000001            # step 1: cc-hide.sh, no payload
ok "no baseline yet" "$(sqlite3 "$CC_FLEET_DB" "SELECT COALESCE(at_payload,'NULL') FROM hidden WHERE uuid='bb-0000-0000-0000-000000000001';")" "NULL"
bash "$CC" hidden-add bb-0000-0000-0000-000000000001 29239      # step 2: detached post-exit writer
ok "baseline survives the upsert" "$(sqlite3 "$CC_FLEET_DB" "SELECT at_payload FROM hidden WHERE uuid='bb-0000-0000-0000-000000000001';")" "29239"
bash "$CC" hidden-add bb-0000-0000-0000-000000000001            # a later payload-less re-hide
ok "payload-less re-add does NOT wipe it" "$(sqlite3 "$CC_FLEET_DB" "SELECT at_payload FROM hidden WHERE uuid='bb-0000-0000-0000-000000000001';")" "29239"
bash "$CC" hidden-add bb-0000-0000-0000-000000000001 55555      # a newer baseline wins
ok "a newer baseline replaces the old" "$(sqlite3 "$CC_FLEET_DB" "SELECT at_payload FROM hidden WHERE uuid='bb-0000-0000-0000-000000000001';")" "55555"

echo
printf 'PASS %d   FAIL %d\n' "$pass" "$fail"
rm -rf "$T"
[ "$fail" = 0 ]
