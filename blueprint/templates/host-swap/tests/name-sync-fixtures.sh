#!/usr/bin/env bash
# Fixtures for cc-name-sync.sh — the single writer of chat window names. Everything runs on
# SCRATCH tmux sockets under a throwaway TMUX_TMPDIR and a fake CODEX_HOME; the live fleet is
# never touched. The cases guard the two behaviors that earned this tool its existence:
# a codex rename reaching the window (and so the VS Code tab), and two same-cwd codex chats
# keeping their OWN names (the newest-in-cwd resolver labeled both with the newer thread).
set -uo pipefail
BUNDLE="$(cd "$(dirname "$0")/.." && pwd)"
SYNC="$BUNDLE/cc-name-sync.sh"
# cd -P, because tmux reports pane_current_path PHYSICALLY: on macOS /var is a symlink to
# /private/var, so a logical $T makes every fixture cwd differ from the one the code under test
# reads back, and not one codex pane matches its rollout. The scratch dir has to be spelled the
# way the kernel spells it.
T="$(cd -P "$(mktemp -d)" && pwd)"
export TMUX_TMPDIR="$T/tmux"
export CODEX_HOME="$T/codex"
export CC_NAME_SYNC_LOCK="$T/sync.lock"
mkdir -p "$TMUX_TMPDIR" "$CODEX_HOME/sessions/2026/08/02" "$T/projA" "$T/projB"
SESS="$CODEX_HOME/sessions/2026/08/02"

cleanup() {
  local s
  for s in "$TMUX_TMPDIR"/*; do [ -S "$s" ] && tmux -S "$s" kill-server 2>/dev/null; done
  rm -rf "$T"
}
trap cleanup EXIT

pass=0; fail=0
ok(){ if [ "$2" = "$3" ]; then pass=$((pass+1)); printf '  ✓ %s\n' "$1"; else fail=$((fail+1)); printf '  ✗ %s\n     want=[%s]\n     got =[%s]\n' "$1" "$3" "$2"; fi; }
win(){ tmux -S "$TMUX_TMPDIR/$1" list-windows -t "=$1" -F '#{window_name}' 2>/dev/null | head -1; }
# utc / settime — the two GNU date/touch spellings this fixture used, taught to fall back to BSD.
# `date -d @EPOCH` and `touch -d @EPOCH` are both GNU-only; macOS wants `date -r EPOCH` and
# `touch -t CCYYMMDDhhmm.SS`. Neither failure was loud: touch printed a usage block to stderr and
# carried on, so every rollout file kept the WRONG mtime, no codex thread ever matched its pane,
# and thirteen assertions failed pointing at cc-name-sync.sh instead of at this file.
utc(){ date -u -d "@$1" +%Y-%m-%dT%H:%M:%S.000Z 2>/dev/null || date -u -r "$1" +%Y-%m-%dT%H:%M:%S.000Z; }
settime(){ touch -d "@$1" "$2" 2>/dev/null || touch -t "$(date -r "$1" +%Y%m%d%H%M.%S)" "$2"; }

# rollout <file-stamp> <uuid> <birth-epoch> <cwd> [first-prompt] [envelope-epoch]
# The envelope timestamp defaults to the birth; pass it to model what codex actually writes —
# an outer stamp carrying the record's WRITE time, minutes or hours past the thread's birth.
rollout(){
  local f="$SESS/rollout-$1-$2.jsonl"
  printf '{"timestamp":"%s","type":"session_meta","payload":{"id":"%s","timestamp":"%s","cwd":"%s","thread_source":"user"}}\n' \
    "$(utc "${6:-$3}")" "$2" "$(utc "$3")" "$4" > "$f"
  [ -n "${5:-}" ] && printf '{"timestamp":"%s","type":"event_msg","payload":{"type":"user_message","message":"%s"}}\n' "$(utc "$3")" "$5" >> "$f"
  settime "$3" "$f"
}

NOW="$(date +%s)"

echo "=== codex: two chats in ONE directory keep their OWN names (birth-time disambiguation) ==="
# pane A starts now; pane B starts 6s later; each thread's rollout is born 1s after its pane.
tmux -S "$TMUX_TMPDIR/cx-100" new-session -d -s cx-100 -n Codex -c "$T/projA" 'sleep 300'
TA="$(date +%s)"
sleep 6
tmux -S "$TMUX_TMPDIR/cx-200" new-session -d -s cx-200 -n Codex -c "$T/projA" 'sleep 300'
TB="$(date +%s)"
rollout "2026-08-02T00-00-01" "aaaaaaaa-0000-0000-0000-000000000001" "$((TA+1))" "$T/projA"
rollout "2026-08-02T00-00-07" "bbbbbbbb-0000-0000-0000-000000000002" "$((TB+1))" "$T/projA"
{ printf '{"id":"aaaaaaaa-0000-0000-0000-000000000001","thread_name":"OLD_NAME_A"}\n'
  printf '{"id":"aaaaaaaa-0000-0000-0000-000000000001","thread_name":"CONTRACTS"}\n'   # rename appends; last wins
  printf '{"id":"bbbbbbbb-0000-0000-0000-000000000002","thread_name":"FACTS"}\n'
} > "$CODEX_HOME/session_index.jsonl"

echo "--- dry run first: plans both, changes nothing"
plan="$("$SYNC" --dry-run | grep -c 'would rename codex')"
ok "dry run plans 2 codex renames"        "$plan" "2"
ok "dry run left window A untouched"      "$(win cx-100)" "Codex"
ok "dry run left window B untouched"      "$(win cx-200)" "Codex"

"$SYNC" >/dev/null
ok "pane A got ITS thread's last name"    "$(win cx-100)" "CONTRACTS"
ok "pane B got ITS thread's name"         "$(win cx-200)" "FACTS"
ok "cx server titles to the tab"          "$(tmux -S "$TMUX_TMPDIR/cx-100" show-options -g -v set-titles-string)" '⬢ #{window_name} · #{pane_title}'

echo "=== codex: a rename landing in the index converges on the next run ==="
printf '{"id":"aaaaaaaa-0000-0000-0000-000000000001","thread_name":"CONTRACTS_BUILDER"}\n' >> "$CODEX_HOME/session_index.jsonl"
"$SYNC" >/dev/null
ok "appended rename wins"                 "$(win cx-100)" "CONTRACTS_BUILDER"

echo "=== codex: no index entry falls back to the first prompt, truncated to 24 ==="
tmux -S "$TMUX_TMPDIR/cx-300" new-session -d -s cx-300 -n Codex -c "$T/projB" 'sleep 300'
TC="$(date +%s)"
rollout "2026-08-02T00-00-09" "cccccccc-0000-0000-0000-000000000003" "$((TC+1))" "$T/projB" \
  "Execute tmp/prompt-save.md and report everything back"
"$SYNC" >/dev/null
ok "first-prompt name, 24 chars"          "$(win cx-300)" "Execute tmp/prompt-save."

echo "=== codex: lineage — a resumed thread's fresh id finds the ancestor's name ==="
# the fresh rollout's OWN id has no index entry; only its parent_thread_id does
mkdir -p "$T/projD"
tmux -S "$TMUX_TMPDIR/cx-350" new-session -d -s cx-350 -n Codex -c "$T/projD" 'sleep 300'
TD="$(date +%s)"
printf '{"timestamp":"%s","type":"session_meta","payload":{"id":"dddddddd-0000-0000-0000-000000000004","timestamp":"%s","cwd":"%s","thread_source":"user","parent_thread_id":"cccccccc-0000-0000-0000-000000000003"}}\n' \
  "$(utc "$((TD+1))")" "$(utc "$((TD+1))")" "$T/projD" > "$SESS/rollout-2026-08-02T00-00-11-dddddddd-0000-0000-0000-000000000004.jsonl"
settime "$((TD+1))" "$SESS/rollout-2026-08-02T00-00-11-dddddddd-0000-0000-0000-000000000004.jsonl"
printf '{"id":"cccccccc-0000-0000-0000-000000000003","thread_name":"RESUMED_ANCESTOR"}\n' >> "$CODEX_HOME/session_index.jsonl"
"$SYNC" >/dev/null
ok "ancestor's name via parent_thread_id" "$(win cx-350)" "RESUMED_ANCESTOR"

echo "=== codex: an elder-thread pane (nothing born near its start) is left alone ==="
# the rollout is an hour old, the pane just started — outside the ±120s birth bound, and
# there is no newest-in-cwd guess anymore: no match means no rename
mkdir -p "$T/projC"
tmux -S "$TMUX_TMPDIR/cx-400" new-session -d -s cx-400 -n Codex -c "$T/projC" 'sleep 300'
rollout "2026-08-02T00-00-13" "eeeeeeee-0000-0000-0000-000000000005" "$((NOW-3600))" "$T/projC"
printf '{"id":"eeeeeeee-0000-0000-0000-000000000005","thread_name":"ELDER"}\n' >> "$CODEX_HOME/session_index.jsonl"
"$SYNC" >/dev/null
ok "elder pane is left alone"             "$(win cx-400)" "Codex"

echo "=== codex: a name whose 24-char cut lands on a space is trimmed ==="
mkdir -p "$T/projE"
tmux -S "$TMUX_TMPDIR/cx-450" new-session -d -s cx-450 -n Codex -c "$T/projE" 'sleep 300'
TE="$(date +%s)"
rollout "2026-08-02T00-00-15" "ffffffff-0000-0000-0000-000000000006" "$((TE+1))" "$T/projE"
printf '{"id":"ffffffff-0000-0000-0000-000000000006","thread_name":"AAAAAAAAAAAAAAAAAAAAAAA BBBB"}\n' >> "$CODEX_HOME/session_index.jsonl"
"$SYNC" >/dev/null
ok "no trailing space in the window"      "$(win cx-450)" "AAAAAAAAAAAAAAAAAAAAAAA"

echo "=== codex: the birth is payload.timestamp, not the envelope's write time ==="
# Codex rewrites the meta record's OUTER stamp as the thread runs, so dating a thread by the first
# "timestamp" in the line dated it by its last write — here an hour late, which hands the pane to
# the sibling chat launched a minute after it.
mkdir -p "$T/projG"
tmux -S "$TMUX_TMPDIR/cx-460" new-session -d -s cx-460 -n Codex -c "$T/projG" 'sleep 300'
TG="$(date +%s)"
rollout "2026-08-02T00-00-17" "11111111-0000-0000-0000-000000000007" "$((TG+1))"  "$T/projG" "" "$((TG+3600))"
rollout "2026-08-02T00-00-18" "22222222-0000-0000-0000-000000000008" "$((TG+60))" "$T/projG"
{ printf '{"id":"11111111-0000-0000-0000-000000000007","thread_name":"MINE"}\n'
  printf '{"id":"22222222-0000-0000-0000-000000000008","thread_name":"SIBLING"}\n'
} >> "$CODEX_HOME/session_index.jsonl"
"$SYNC" >/dev/null
ok "dated by its birth, not its writes"   "$(win cx-460)" "MINE"

echo "=== codex: the state store names the threads that write no rollout file ==="
# Codex keeps a paginated thread's history in ~/.codex/state_<N>.sqlite and may write no rollout
# at all. Every case below has NO rollout file — before the store was read, such a pane silently
# inherited the newest OTHER rollout in its directory, and its renames never reached the window.
if command -v sqlite3 >/dev/null 2>&1; then
  CXDB="$CODEX_HOME/state_9.sqlite"
  sqlite3 "$CXDB" "create table threads (id text primary key, cwd text not null, created_at integer not null,
    name text, title text not null default '', first_user_message text not null default '',
    thread_source text not null default 'user', archived integer not null default 0);" 2>/dev/null
  # dbrow <id> <cwd> <created-epoch> <name> [first-prompt]
  dbrow(){ sqlite3 "$CXDB" "insert or replace into threads (id,cwd,created_at,name,first_user_message)
             values ('$1','$2',$3,'$4','${5:-}');" 2>/dev/null; }

  mkdir -p "$T/projF"
  # the stale sibling that used to win: an hour-old rollout, the only one in this directory
  rollout "2026-08-02T00-00-16" "99999999-0000-0000-0000-000000000009" "$((NOW-3600))" "$T/projF"
  printf '{"id":"99999999-0000-0000-0000-000000000009","thread_name":"STALE_SIBLING"}\n' >> "$CODEX_HOME/session_index.jsonl"
  tmux -S "$TMUX_TMPDIR/cx-500" new-session -d -s cx-500 -n Codex -c "$T/projF" 'sleep 300'
  TF="$(date +%s)"
  dbrow "aaaa0001-0000-0000-0000-00000000000a" "$T/projF" "$TF" "STORE_NAME"
  "$SYNC" >/dev/null
  ok "rollout-less thread names its window" "$(win cx-500)" "STORE_NAME"

  dbrow "aaaa0001-0000-0000-0000-00000000000a" "$T/projF" "$TF" "STORE_RENAMED"
  "$SYNC" >/dev/null
  ok "a rename in the store converges"      "$(win cx-500)" "STORE_RENAMED"

  # a thread renamed before codex kept the name in the store: the index still holds it
  mkdir -p "$T/projH"
  tmux -S "$TMUX_TMPDIR/cx-600" new-session -d -s cx-600 -n Codex -c "$T/projH" 'sleep 300'
  TH="$(date +%s)"
  dbrow "aaaa0002-0000-0000-0000-00000000000b" "$T/projH" "$TH" "" "Wire the queue up and report back"
  printf '{"id":"aaaa0002-0000-0000-0000-00000000000b","thread_name":"INDEX_NAME"}\n' >> "$CODEX_HOME/session_index.jsonl"
  "$SYNC" >/dev/null
  ok "no store name → the index's name"     "$(win cx-600)" "INDEX_NAME"

  # neither → the thread's own first prompt, cut to 24
  mkdir -p "$T/projI"
  tmux -S "$TMUX_TMPDIR/cx-700" new-session -d -s cx-700 -n Codex -c "$T/projI" 'sleep 300'
  TI="$(date +%s)"
  dbrow "aaaa0003-0000-0000-0000-00000000000c" "$T/projI" "$TI" "" "Wire the queue up and report back"
  "$SYNC" >/dev/null
  ok "no name anywhere → first prompt"      "$(win cx-700)" "Wire the queue up and re"

  # an elder pane the store cannot place (nothing born near its start) is left alone —
  # the rollout scan is bounded by the same ±120s birth rule, no fallback guess
  mkdir -p "$T/projJ"
  tmux -S "$TMUX_TMPDIR/cx-800" new-session -d -s cx-800 -n Codex -c "$T/projJ" 'sleep 300'
  dbrow "aaaa0004-0000-0000-0000-00000000000d" "$T/projJ" "$((NOW-7200))" "TOO_OLD"
  rollout "2026-08-02T00-00-19" "aaaa0005-0000-0000-0000-00000000000e" "$((NOW-3600))" "$T/projJ"
  printf '{"id":"aaaa0005-0000-0000-0000-00000000000e","thread_name":"ELDER_ROLLOUT"}\n' >> "$CODEX_HOME/session_index.jsonl"
  "$SYNC" >/dev/null
  ok "elder pane is left alone (store miss)" "$(win cx-800)" "Codex"
else
  echo "  — sqlite3 absent, store cases skipped"
fi

echo "=== squatter: a second session on a chat's socket is not a chat ==="
tmux -S "$TMUX_TMPDIR/cx-100" new-session -d -s squat -n keepme -c "$T/projA" 'sleep 300'
"$SYNC" >/dev/null
ok "squatter window untouched"            "$(tmux -S "$TMUX_TMPDIR/cx-100" list-windows -t "=squat" -F '#{window_name}')" "keepme"

echo "=== claude: window follows the 🔖 statusline label ==="
tmux -S "$TMUX_TMPDIR/cc-500" new-session -d -s cc-500 -n claude -c "$T/projA" \
  'printf "some output\n🔖 LEGAL_DOCS │ 🥇 acct1 │ main\n"; sleep 300'
sleep 1   # let the fake statusline render before the scrape
"$SYNC" >/dev/null
ok "claude window renamed to the label"   "$(win cc-500)" "LEGAL_DOCS"

echo "=== claude: unlabeled chat keeps its window name ==="
tmux -S "$TMUX_TMPDIR/cc-600" new-session -d -s cc-600 -n claude -c "$T/projA" \
  'printf "no label here, just a badge 🥇 line\n"; sleep 300'
sleep 1
"$SYNC" >/dev/null
ok "no 🔖 → window left alone"            "$(win cc-600)" "claude"

echo "=== claude: two differently-labeled panes in one window → conflicted, left alone ==="
tmux -S "$TMUX_TMPDIR/cc-700" new-session -d -s cc-700 -n claude -c "$T/projA" \
  'printf "🔖 FIRST │ 🥇\n"; sleep 300'
tmux -S "$TMUX_TMPDIR/cc-700" split-window -t "cc-700:claude" -c "$T/projA" \
  'printf "🔖 SECOND │ 🥈\n"; sleep 300'
sleep 1
"$SYNC" >/dev/null
ok "conflicted window not renamed"        "$(win cc-700)" "claude"

echo "=== viewport: a pane running a tmux client is a mirror, never a chat ==="
tmux -S "$TMUX_TMPDIR/cc-800" new-session -d -s cc-800 -n viewport -c "$T/projA" \
  "TMUX= tmux -S '$TMUX_TMPDIR/cc-500' attach -t '=cc-500'"
sleep 1
"$SYNC" >/dev/null
ok "viewport window untouched"            "$(win cc-800)" "viewport"

echo "=== lock: a held lock means a silent, clean no-op ==="
printf '{"id":"aaaaaaaa-0000-0000-0000-000000000001","thread_name":"MUST_NOT_LAND"}\n' >> "$CODEX_HOME/session_index.jsonl"
# Hold the lock exactly the way the script takes it: a LIVE owner process holding cc_trylock.
# The old form used flock(1), which stock macOS does not ship — the holder failed to start, the
# sync ran unlocked, and the assertion measured nothing. A background subshell is also the more
# faithful fixture: what makes a lock held is an owner that is still alive.
( . "$BUNDLE/cc-portable.sh"; cc_trylock "$CC_NAME_SYNC_LOCK" && sleep 10 ) &
holder=$!
sleep 0.5
out="$("$SYNC"; echo "rc=$?")"
ok "locked run exits 0 with no renames"   "$out" "rc=0"
ok "locked run changed nothing"           "$(win cx-100)" "CONTRACTS_BUILDER"
# Killing the holder leaves an owner file naming a dead pid — the next run must STEAL it, which
# is the crash-recovery path flock got for free from the kernel and mkdir has to earn.
kill "$holder" 2>/dev/null; wait "$holder" 2>/dev/null
"$SYNC" >/dev/null   # and the next unlocked run converges it
ok "next run picks the rename up"         "$(win cx-100)" "MUST_NOT_LAND"

echo ""
echo "name-sync fixtures: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
