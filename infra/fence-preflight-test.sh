#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PREPARE="$SCRIPT_DIR/prepare-fence-mounts.sh"
ROOT="$(mktemp -d)"
trap 'rm -rf -- "$ROOT"' EXIT

bash "$PREPARE" "$ROOT"
for relative in \
  engines/wave-walker/engine/node_modules \
  engines/wave-walker/engine/dist/cross-workflow
do
  if [[ ! -d "$ROOT/$relative" ]]; then
    echo "fence-preflight-test: missing prepared directory $relative" >&2
    exit 1
  fi
done

BROKEN="$ROOT/broken"
mkdir -p "$BROKEN/engines/wave-walker/engine"
touch "$BROKEN/engines/wave-walker/engine/node_modules"
if bash "$PREPARE" "$BROKEN" >"$ROOT/broken.out" 2>"$ROOT/broken.err"; then
  echo "fence-preflight-test: non-directory mount target was accepted" >&2
  exit 1
fi
if ! grep -q "not a directory" "$ROOT/broken.err"; then
  echo "fence-preflight-test: broken-state error did not name the invalid target" >&2
  cat "$ROOT/broken.err" >&2
  exit 1
fi
