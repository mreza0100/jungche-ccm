#!/usr/bin/env bash
# Fixtures for cc-name-sync.sh — the single writer of chat window names. Everything runs on
# SCRATCH tmux sockets under a throwaway TMUX_TMPDIR and a fake CODEX_HOME; the live fleet is
# never touched. The cases guard the two behaviors that earned this tool its existence:
# a codex rename reaching the window (and so the VS Code tab), and two same-cwd codex chats
# keeping their OWN names (the newest-in-cwd resolver labeled both with the newer thread).
set -uo pipefail
BUNDLE="$(cd "$(dirname "$0")/.." && pwd)"
SYNC="$BUNDLE/cc-name-sync.sh"
T="$(mktemp -d)"
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
utc(){ date -u -d "@$1" +%Y-%m-%dT%H:%M:%S.000Z; }

# rollout <file-stamp> <uuid> <birth-epoch> <cwd> [first-prompt]
rollout(){
  local f="$SESS/rollout-$1-$2.jsonl"
  printf '{"timestamp":"%s","type":"session_meta","payload":{"id":"%s","timestamp":"%s","cwd":"%s","thread_source":"user"}}\n' \
    "$(utc "$3")" "$2" "$(utc "$3")" "$4" > "$f"
  [ -n "${5:-}" ] && printf '{"timestamp":"%s","type":"event_msg","payload":{"type":"user_message","message":"%s"}}\n' "$(utc "$3")" "$5" >> "$f"
  touch -d "@$3" "$f"
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
touch -d "@$((TD+1))" "$SESS/rollout-2026-08-02T00-00-11-dddddddd-0000-0000-0000-000000000004.jsonl"
printf '{"id":"cccccccc-0000-0000-0000-000000000003","thread_name":"RESUMED_ANCESTOR"}\n' >> "$CODEX_HOME/session_index.jsonl"
"$SYNC" >/dev/null
ok "ancestor's name via parent_thread_id" "$(win cx-350)" "RESUMED_ANCESTOR"

echo "=== codex: an elder-thread pane (nothing born near its start) falls back to newest-in-cwd ==="
# the rollout is an hour old, the pane just started — closest-birth finds nothing acceptable
mkdir -p "$T/projC"
tmux -S "$TMUX_TMPDIR/cx-400" new-session -d -s cx-400 -n Codex -c "$T/projC" 'sleep 300'
rollout "2026-08-02T00-00-13" "eeeeeeee-0000-0000-0000-000000000005" "$((NOW-3600))" "$T/projC"
printf '{"id":"eeeeeeee-0000-0000-0000-000000000005","thread_name":"ELDER"}\n' >> "$CODEX_HOME/session_index.jsonl"
"$SYNC" >/dev/null
ok "elder pane still gets a name"         "$(win cx-400)" "ELDER"

echo "=== codex: a name whose 24-char cut lands on a space is trimmed ==="
mkdir -p "$T/projE"
tmux -S "$TMUX_TMPDIR/cx-450" new-session -d -s cx-450 -n Codex -c "$T/projE" 'sleep 300'
TE="$(date +%s)"
rollout "2026-08-02T00-00-15" "ffffffff-0000-0000-0000-000000000006" "$((TE+1))" "$T/projE"
printf '{"id":"ffffffff-0000-0000-0000-000000000006","thread_name":"AAAAAAAAAAAAAAAAAAAAAAA BBBB"}\n' >> "$CODEX_HOME/session_index.jsonl"
"$SYNC" >/dev/null
ok "no trailing space in the window"      "$(win cx-450)" "AAAAAAAAAAAAAAAAAAAAAAA"

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
out="$(flock "$CC_NAME_SYNC_LOCK" -c "\"$SYNC\"; echo rc=\$?")"
ok "locked run exits 0 with no renames"   "$out" "rc=0"
ok "locked run changed nothing"           "$(win cx-100)" "CONTRACTS_BUILDER"
"$SYNC" >/dev/null   # and the next unlocked run converges it
ok "next run picks the rename up"         "$(win cx-100)" "MUST_NOT_LAND"

echo ""
echo "name-sync fixtures: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
