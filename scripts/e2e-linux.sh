#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v docker >/dev/null 2>&1; then
  echo "e2e-linux: docker is required" >&2
  exit 1
fi

docker run --rm \
  -e HOME=/tmp/pfm-e2e-home \
  -e PFM_DEV_FENCE=1 \
  -e PFM_E2E_HARVESTPY_GATE='blocked, not attempted' \
  -e PFM_HARVESTPY_OFFLINE=1 \
  -e PFM_E2E_SOURCE_REPO=/workspace \
  -v "${repo_root}:/workspace" \
  -w /workspace \
  golang:1.24-bookworm \
  bash -euo pipefail -c '
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends git tmux zsh
    mkdir -p "$HOME"
    go -C pfm test -tags e2e -p 1 ./e2e/...
  '
