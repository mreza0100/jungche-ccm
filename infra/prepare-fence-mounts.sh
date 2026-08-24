#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-}"
if [[ -z "$ROOT" || ! -d "$ROOT" ]]; then
  echo "prepare-fence-mounts: checkout root is not a directory: ${ROOT:-<missing>}" >&2
  exit 1
fi

for relative in \
  engines/wave-walker/engine/node_modules \
  engines/wave-walker/engine/dist/cross-workflow
do
  target="$ROOT/$relative"
  if [[ -e "$target" && ! -d "$target" ]]; then
    echo "prepare-fence-mounts: mount target is not a directory: $relative" >&2
    exit 1
  fi
  if ! mkdir -p -- "$target"; then
    echo "prepare-fence-mounts: create mount target failed: $relative" >&2
    exit 1
  fi
done
