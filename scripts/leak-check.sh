#!/usr/bin/env bash
# leak-check.sh — mechanical leak gate: brand (current+former), founder PII,
# machine paths must never reach the public blueprint repo. Wired as pre-push
# via .githooks/ (git config core.hooksPath .githooks). Stock service ports
# (5432/5433/4566/4567) are industry defaults, not identifying — deliberately
# ungated.
#
# MATCHED CASE-INSENSITIVELY (grep -i), and that is load-bearing: the pattern once spelled the
# brand `[Ii]ntuita` and matched neither `INTUITA` nor `iNTUITA`, so an all-caps chat name carried
# the client's name straight through a "clean" gate. A denylist that only knows two capitalisations
# of a word does not know the word. Keep the alternatives lowercase and let -i do the work.
set -euo pipefail

PATTERN='(intuita|freudche|nervah|soulcheck|khosravivala|mohammadreza|\breza\b|reza@|/home/reza|/Users/[A-Za-z0-9])'

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
      if grep -qiE "$PATTERN" <<<"$content"; then
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
        matches="$(grep -niE "$PATTERN" "$f" || true)"
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
