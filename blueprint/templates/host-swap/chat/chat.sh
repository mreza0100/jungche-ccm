#!/usr/bin/env bash
set -euo pipefail

# The source-backed install can expose this shim before the matching pfm binary
# is installed. Keep the established transport available through that cutover.
script_path="${BASH_SOURCE[0]}"
while [[ -L "$script_path" ]]; do
  script_dir="$(cd -P "$(dirname "$script_path")" && pwd)"
  script_path="$(readlink "$script_path")"
  [[ "$script_path" == /* ]] || script_path="$script_dir/$script_path"
done
script_dir="$(cd -P "$(dirname "$script_path")" && pwd)"

if command -v pfm >/dev/null 2>&1 && pfm chat --help >/dev/null 2>&1; then
  exec pfm chat "$@"
fi

exec bash "$script_dir/chat-ops.sh" "$@"
