#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
JAIL="$(mktemp -d /tmp/ccf-e2e.XXXXXX)"
cleanup() {
  if [[ "${CC_FLEET_KEEP_JAIL:-0}" == 1 ]]; then
    printf 'e2e: kept jail %s\n' "$JAIL"
  else
    rm -rf -- "$JAIL"
  fi
}
trap cleanup EXIT

BIN="$JAIL/cc-fleet"
HOME_DIR="$JAIL/home"
CLAUDE_ROOT="$JAIL/claude"
CODEX_ROOT="$JAIL/codex"
SID_DIR="$JAIL/sid"
PROC_ROOT="$JAIL/proc"
TMUX_BASE="$JAIL/t"
TMUX_DIR="$TMUX_BASE/tmux-$(id -u)"
DB="$JAIL/state/fleet.db"

mkdir -p \
  "$HOME_DIR/.claude" \
  "$CLAUDE_ROOT" \
  "$CODEX_ROOT" \
  "$SID_DIR" \
  "$PROC_ROOT" \
  "$TMUX_DIR" \
  "$(dirname -- "$DB")"
cp -R "$ROOT/testdata/claude-store/." "$CLAUDE_ROOT/"
cp -R "$ROOT/testdata/codex-store/." "$CODEX_ROOT/"
find "$CLAUDE_ROOT" "$CODEX_ROOT" -type f -exec touch -d '@1700000000' {} +

(
  cd "$ROOT"
  CGO_ENABLED=0 GOTOOLCHAIN=local GOTELEMETRY=off \
    mise exec -- go build -o "$BIN" ./cmd/cc-fleet
)

export HOME="$HOME_DIR"
export TMUX=
export TMUX_PANE=
export TMUX_TMPDIR="$TMUX_BASE"
export CC_FLEET_DB="$DB"
export CC_FLEET_SID_DIR="$SID_DIR"
export CC_FLEET_CLAUDE_ROOTS="$CLAUDE_ROOT"
export CC_FLEET_CODEX_ROOT="$CODEX_ROOT"
export CC_FLEET_TMUX_DIR="$TMUX_DIR"
export CC_FLEET_HOME="$HOME_DIR"
export CC_FLEET_PROC_ROOT="$PROC_ROOT"
export CC_FLEET_CODEX_AVAILABLE=1
export CC_FLEET_TEST_FRESH_SOCKET="cc-1700000000-4242-7"
export CC_FLEET_TEST_NOW_NS=1800000000000000000

printf '2\n' > "$HOME_DIR/.claude-primary"
ALPHA="$CLAUDE_ROOT/project-alpha/alpha.jsonl"
COMPACT="$CLAUDE_ROOT/project-beta/compact.jsonl"
alpha_legacy_size="$(wc -c < "$ALPHA" | tr -d ' ')"
compact_legacy_size="$(wc -c < "$COMPACT" | tr -d ' ')"
printf 'alpha\ncompact\n' > "$HOME_DIR/.claude/.cc-ls-hidden"
printf 'alpha\t%s\ncompact\t%s\n' \
  "$alpha_legacy_size" \
  "$compact_legacy_size" \
  > "$HOME_DIR/.claude/.cc-ls-hidden.at"

assert_file() {
  local actual="$1" expected="$2" label="$3"
  if ! cmp -s -- "$actual" "$expected"; then
    printf 'e2e: %s mismatch\n' "$label" >&2
    diff -u -- "$expected" "$actual" >&2 || true
    exit 1
  fi
}

assert_tsv_has() {
  local path="$1" id="$2"
  awk -F $'\t' -v id="$id" 'NR > 1 && $2 == id { found=1 } END { exit !found }' "$path"
}

assert_tsv_lacks() {
  local path="$1" id="$2"
  if assert_tsv_has "$path" "$id"; then
    printf 'e2e: TSV unexpectedly contains id %s\n' "$id" >&2
    exit 1
  fi
}

index_full="$("$BIN" index --full)"
[[ "$index_full" == \
  "files=8 skipped=0 delta=0 full=8 deleted=0 touched=13 bytes=2511 cx_names=true" ]]
printf 'e2e: initial %s\n' "$index_full"

"$BIN" ls --tsv > "$JAIL/e2e.tsv"
assert_file "$JAIL/e2e.tsv" "$ROOT/testdata/golden/e2e.tsv" "ls --tsv"

index_warm="$("$BIN" index)"
[[ "$index_warm" == \
  "files=8 skipped=8 delta=0 full=0 deleted=0 touched=0 bytes=0 cx_names=false" ]]
printf 'e2e: warm %s\n' "$index_warm"

"$BIN" open alpha > "$JAIL/open.line"
sed \
  -e "s|@HOME@|$HOME_DIR|g" \
  -e "s|@CWD@|$ROOT|g" \
  "$ROOT/testdata/golden/e2e-open.txt" > "$JAIL/open.expected"
assert_file "$JAIL/open.line" "$JAIL/open.expected" "open eval line"

if [[ "${CC_FLEET_STRESS:-0}" == 1 ]]; then
  timings="$JAIL/warm-ms"
  : > "$timings"
  for _ in $(seq 1 100); do
    started="$(date +%s%N)"
    "$BIN" ls --tsv > "$JAIL/warm.tsv"
    finished="$(date +%s%N)"
    assert_file "$JAIL/warm.tsv" "$ROOT/testdata/golden/e2e.tsv" "warm ls"
    printf '%d\n' "$(((finished-started)/1000000))" >> "$timings"
  done
  sort -n "$timings" > "$timings.sorted"
  median="$(sed -n '50p' "$timings.sorted")"
  p95="$(sed -n '95p' "$timings.sorted")"
  limit=2000
  if [[ "${CC_FLEET_STRESS_STRICT:-0}" == 1 ]]; then
    limit=250
  fi
  (( p95 < limit ))
  printf 'STRESS warm_ls iterations=100 median_ms=%s p95_ms=%s limit_ms=%s strict=%s\n' \
    "$median" "$p95" "$limit" "${CC_FLEET_STRESS_STRICT:-0}"

  baseline_counts="$("$BIN" doctor | awk '/doctor: rows /')"
  for turn in $(seq 1 20); do
    if (( turn % 2 == 0 )); then
      "$BIN" index --full > "$JAIL/hammer.index"
    else
      "$BIN" index > "$JAIL/hammer.index"
    fi
    [[ "$("$BIN" doctor | awk '/doctor: rows /')" == "$baseline_counts" ]]
    "$BIN" ls --tsv > "$JAIL/hammer.tsv"
    assert_file "$JAIL/hammer.tsv" "$ROOT/testdata/golden/e2e.tsv" "idempotence parity"
  done
  printf 'STRESS index_hammer passes=20 row_counts_stable=true golden_parity=true\n'

  pids=()
  for worker in $(seq 1 8); do
    "$BIN" ls --tsv > "$JAIL/concurrent-ls-$worker.tsv" \
      2> "$JAIL/concurrent-ls-$worker.err" &
    pids+=("$!")
  done
  concurrent_ids=(agent alpha compact junk)
  for id in "${concurrent_ids[@]}"; do
    (
      "$BIN" hide "$id"
      "$BIN" unhide "$id"
    ) > "$JAIL/concurrent-hide-$id.out" \
      2> "$JAIL/concurrent-hide-$id.err" &
    pids+=("$!")
  done
  for pid in "${pids[@]}"; do
    wait "$pid"
  done
  if grep -R -n 'WARNING:' "$JAIL"/concurrent-*.err; then
    printf 'e2e: concurrent CLI emitted a busy warning\n' >&2
    exit 1
  fi
  [[ -z "$("$BIN" hidden)" ]]
  "$BIN" ls --tsv > "$JAIL/concurrent-final.tsv"
  assert_file \
    "$JAIL/concurrent-final.tsv" \
    "$ROOT/testdata/golden/e2e.tsv" \
    "concurrent final parity"
  "$BIN" doctor > "$JAIL/concurrent.doctor"
  printf 'STRESS concurrent_cli ls=8 hide_unhide=4 busy_warnings=0 corruption=0\n'

  cp -- "$DB" "$JAIL/damaged.db"
  truncate -s 16 "$JAIL/damaged.db"
  if CC_FLEET_DB="$JAIL/damaged.db" "$BIN" doctor \
    > "$JAIL/damaged.doctor" 2> "$JAIL/damaged.err"; then
    printf 'e2e: damaged doctor unexpectedly exited zero\n' >&2
    exit 1
  fi
  grep -q '^doctor: unhealthy database:' "$JAIL/damaged.doctor"
  printf 'STRESS damaged_doctor exit=1 panic=false unhealthy=true\n'
fi

"$BIN" hide alpha > "$JAIL/hide.out"
"$BIN" ls --tsv > "$JAIL/hidden.tsv"
assert_tsv_lacks "$JAIL/hidden.tsv" alpha
"$BIN" unhide alpha > "$JAIL/unhide.out"
"$BIN" ls --tsv > "$JAIL/unhidden.tsv"
assert_tsv_has "$JAIL/unhidden.tsv" alpha

"$BIN" hide alpha > "$JAIL/hide-again.out"
printf '%s\n' \
  '{"type":"user","cwd":"/work/alpha","message":{"content":"Alpha wakes again"}}' \
  >> "$ALPHA"
delta="$("$BIN" index)"
[[ "$delta" == files=8\ skipped=7\ delta=1\ full=0\ deleted=0\ touched=1\ bytes=*" cx_names=false" ]]
"$BIN" ls --tsv > "$JAIL/still-hidden.tsv"
assert_tsv_lacks "$JAIL/still-hidden.tsv" alpha
if ! "$BIN" hidden | awk -F $'\t' '$1 == "alpha" { found=1 } END { exit !found }'; then
  printf 'e2e: alpha stopped being hidden after a new prompt\n' >&2
  exit 1
fi
printf 'e2e: hide/unhide/permanent-hide round-trip passed\n'

# Captured HERE, not at the top of the script: the hide/unhide/hide-again
# round trip just above legitimately reorders the carrier file (unhide
# rewrites it minus the id, the re-hide appends it back at the end), so a
# baseline from before that dance would never match no matter what `legacy
# import` itself does. The carrier's SOURCE — the id set `legacy import`
# must not touch — is its state right before the command runs.
legacy_before="$(sha256sum \
  "$HOME_DIR/.claude/.cc-ls-hidden" \
  "$HOME_DIR/.claude/.cc-ls-hidden.at")"

legacy_out="$("$BIN" legacy import)"
[[ "$legacy_out" == legacy\ import:\ imported=2\ unknown=0\;\ * ]]
legacy_after="$(sha256sum \
  "$HOME_DIR/.claude/.cc-ls-hidden" \
  "$HOME_DIR/.claude/.cc-ls-hidden.at")"
[[ "$legacy_after" == "$legacy_before" ]]
"$BIN" hidden > "$JAIL/imported.hidden"
awk -F $'\t' '
  $1 == "compact" && $2 == "cc" && $4 == "" { compact=1 }
  $1 == "alpha" && $2 == "cc" && $4 == "" { alpha=1 }
  END { exit !(compact && alpha) }
' "$JAIL/imported.hidden"
printf 'e2e: legacy import hid_every_known_id=true source_untouched=true\n'

"$BIN" doctor | tee "$JAIL/doctor.out"
printf 'e2e: PASS\n'
