#!/usr/bin/env bash
# Sequential multi-repository Dreamer launcher.
set -uo pipefail

ENGINE_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
RUNNER="$ENGINE_DIR/dreamer-night.sh"
REPOS="$ENGINE_DIR/repos.list"

[ -x "$RUNNER" ] || { printf 'dreamer-morning: FAIL missing runner: %s\n' "$RUNNER" >&2; exit 1; }
[ -f "$REPOS" ] || { printf 'dreamer-morning: FAIL missing repository list: %s\n' "$REPOS" >&2; exit 1; }

overall=0
count=0
# Each row is `{repo root}` or `{repo root} {agent type}`; a row without an agent
# runs the default Explore lane, so old single-field lists keep working.
run_lane() {
  local repo=$1 agent=$2 rc
  printf 'dreamer-morning: BEGIN repo=%s agent=%s\n' "$repo" "$agent"
  if [ "$agent" = Explore ]; then
    bash "$RUNNER" --repo "$repo"
  else
    bash "$RUNNER" --repo "$repo" --agent "$agent"
  fi
  rc=$?
  if [ "$rc" -eq 0 ]; then
    printf 'dreamer-morning: PASS repo=%s agent=%s\n' "$repo" "$agent"
  else
    overall=1
    printf 'dreamer-morning: FAIL repo=%s agent=%s rc=%s\n' "$repo" "$agent" "$rc" >&2
  fi
  printf 'dreamer-morning: END repo=%s agent=%s\n' "$repo" "$agent"
}

while IFS= read -r raw || [ -n "$raw" ]; do
  line=$(sed -e 's/[[:space:]]*#.*$//' -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' <<< "$raw")
  [ -n "$line" ] || continue
  repo=${line%%[[:space:]]*}
  agent=Explore
  if [ "$line" != "$repo" ]; then
    agent=$(sed -e 's/^[^[:space:]]*[[:space:]]*//' -e 's/[[:space:]].*$//' <<< "$line")
  fi
  count=$((count + 1))
  run_lane "$repo" "$agent"

  shopt -s nullglob
  profiles=("$repo"/.professor/stm/lanes/*.md)
  shopt -u nullglob
  for profile in "${profiles[@]}"; do
    lane=$(basename "$profile" .md)
    [ "$lane" != explorer ] || continue
    [ "$lane" != "$agent" ] || continue
    run_lane "$repo" "$lane"
  done
done < "$REPOS"

[ "$count" -gt 0 ] || { printf 'dreamer-morning: FAIL repository list is empty\n' >&2; exit 1; }
exit "$overall"
