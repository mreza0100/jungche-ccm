#!/usr/bin/env bash
# leak-check.sh — mechanical leak gate: brand (current+former), founder PII,
# machine paths must never reach the public blueprint repo. Wired as pre-push
# via .githooks/ (git config core.hooksPath .githooks). Stock service ports
# (5432/5433/4566/4567) are industry defaults, not identifying — deliberately
# ungated.
set -euo pipefail

PATTERN='([Ii]ntuita|[Ff]reudche|Khosravivala|Mohammadreza|\bReza\b|reza@|/home/reza|/Users/[A-Za-z0-9])'

usage() {
  echo "Usage: leak-check.sh [--range OLD NEW | --files f1 [f2 ...]]" >&2
  exit 2
}

mode="staged"
range_old=""
range_new=""
files=()

if [[ $# -eq 0 ]]; then
  mode="staged"
elif [[ "$1" == "--range" ]]; then
  [[ $# -eq 3 ]] || usage
  mode="range"
  range_old="$2"
  range_new="$3"
elif [[ "$1" == "--files" ]]; then
  [[ $# -ge 2 ]] || usage
  mode="files"
  shift
  files=("$@")
else
  usage
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

is_excluded_path() {
  local p="$1"
  case "$p" in
    scripts/placeholder-map.tsv|scripts/leak-check.sh|LICENSE) return 0 ;;
    .githooks/*|.githooks) return 0 ;;
  esac
  return 1
}

# Reads a unified diff (-U0) on stdin, prints "LEAK {file}: {content}" for
# every match found in an ADDED line (starts with "+", not "+++"), tracking
# the current file from "+++ b/..." headers.
scan_diff_stream() {
  local file=""
  local line content
  while IFS= read -r line; do
    if [[ "$line" == "+++ /dev/null" ]]; then
      file=""
    elif [[ "$line" == "+++ b/"* ]]; then
      file="${line#+++ b/}"
    elif [[ "$line" == "+++"* ]]; then
      file="${line#+++ }"
    elif [[ "$line" == "+"* ]]; then
      content="${line#+}"
      if grep -qE "$PATTERN" <<<"$content"; then
        printf 'LEAK %s: %s\n' "$file" "$content"
      fi
    fi
  done
}

hits_file="$(mktemp)"
trap 'rm -f "$hits_file"' EXIT

case "$mode" in
  staged)
    git diff --cached -U0 --no-color -- . \
      ':(exclude)scripts/placeholder-map.tsv' \
      ':(exclude)scripts/leak-check.sh' \
      ':(exclude).githooks' \
      ':(exclude)LICENSE' \
      | scan_diff_stream > "$hits_file"
    ;;
  range)
    git diff "$range_old" "$range_new" -U0 --no-color -- . \
      ':(exclude)scripts/placeholder-map.tsv' \
      ':(exclude)scripts/leak-check.sh' \
      ':(exclude).githooks' \
      ':(exclude)LICENSE' \
      | scan_diff_stream > "$hits_file"
    ;;
  files)
    : > "$hits_file"
    for f in "${files[@]}"; do
      if is_excluded_path "$f"; then
        continue
      fi
      if [[ -f "$f" ]]; then
        matches="$(grep -nE "$PATTERN" "$f" || true)"
        if [[ -n "$matches" ]]; then
          while IFS=: read -r lnum content; do
            printf 'LEAK %s: %s\n' "$f" "$content"
          done <<< "$matches" >> "$hits_file"
        fi
      fi
    done
    ;;
esac

n="$(wc -l < "$hits_file" | tr -d ' ')"

if [[ "$n" -gt 0 ]]; then
  cat "$hits_file"
  echo "leak-check: FAILED — ${n} leak line(s)" >&2
  exit 1
fi

echo "leak-check: clean"
exit 0
