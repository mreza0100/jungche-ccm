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
# `~/work/[A-Za-z0-9]` and `/Users/[A-Za-z0-9]` share one discriminator: a home path is a
# leak only when it names a CONCRETE directory. `~/work/<project>` and
# `$HOME/work/{MEMORY_VAULT_DIR}` are the blueprint's own documented defaults and must pass;
# `~/work/host-ops/devbox` names a private repo and must not.
set -euo pipefail

PATTERN='(intuita|freudche|nervah|soulcheck|khosravivala|mohammadreza|\breza\b|reza@|/home/reza|~/work/[A-Za-z0-9]|/Users/[A-Za-z0-9])'

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

if [[ -n "${PFM_DEV_REPO_GIT_DIR:-}" || -n "${PFM_DEV_REPO_WORK_TREE:-}" ]]; then
  if [[ -z "${PFM_DEV_REPO_GIT_DIR:-}" || -z "${PFM_DEV_REPO_WORK_TREE:-}" ]]; then
    echo "leak-check: PFM_DEV_REPO_GIT_DIR and PFM_DEV_REPO_WORK_TREE must be set together" >&2
    exit 1
  fi
  repo_root="$PFM_DEV_REPO_WORK_TREE"
  repo_git() {
    git --git-dir="$PFM_DEV_REPO_GIT_DIR" --work-tree="$PFM_DEV_REPO_WORK_TREE" \
      -c safe.directory="$PFM_DEV_REPO_WORK_TREE" "$@"
  }
else
  repo_root="$(git rev-parse --show-toplevel)"
  repo_git() { git "$@"; }
fi
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
#
# Single-pass: the loop below only accumulates added lines + their file into
# arrays (no subprocess). Matching runs as ONE `grep -inE` over the whole
# accumulated stream, instead of forking grep once per added line — a diff
# with tens of thousands of added lines used to fork tens of thousands of
# grep processes and never return.
scan_diff_stream() {
  local file=""
  local line
  local -a files=()
  local -a contents=()

  while IFS= read -r line; do
    if [[ "$line" == "+++ /dev/null" ]]; then
      file=""
    elif [[ "$line" == "+++ b/"* ]]; then
      file="${line#+++ b/}"
    elif [[ "$line" == "+++"* ]]; then
      file="${line#+++ }"
    elif [[ "$line" == "+"* ]]; then
      contents+=("${line#+}")
      files+=("$file")
    fi
  done

  [[ ${#contents[@]} -eq 0 ]] && return 0

  local matches
  matches="$(printf '%s\n' "${contents[@]}" | grep -niE "$PATTERN" || true)"
  [[ -z "$matches" ]] && return 0

  local idx content
  while IFS=: read -r idx content; do
    printf 'LEAK %s: %s\n' "${files[idx-1]}" "$content"
  done <<< "$matches"
}

hits_file="$(mktemp)"
trap 'rm -f "$hits_file"' EXIT

case "$mode" in
  staged)
    repo_git diff --cached -U0 --no-color -- . \
      ':(exclude)scripts/placeholder-map.tsv' \
      ':(exclude)scripts/leak-check.sh' \
      ':(exclude).githooks' \
      ':(exclude)LICENSE' \
      | scan_diff_stream > "$hits_file"
    ;;
  range)
    repo_git diff "$range_old" "$range_new" -U0 --no-color -- . \
      ':(exclude)scripts/placeholder-map.tsv' \
      ':(exclude)scripts/leak-check.sh' \
      ':(exclude).githooks' \
      ':(exclude)LICENSE' \
      | scan_diff_stream > "$hits_file"
    ;;
  files)
    : > "$hits_file"
    # A named path that is not a regular file was NOT examined. That is normal for a
    # deleted file in a changed-file list, so absence alone does not fail — but it is
    # never silently counted as clean, and a run that examined NOTHING always fails:
    # "we scanned 40 files and found no leak" and "we scanned zero files" must not
    # print the same word. A caller whose shell folded its file list into one argument
    # (zsh does not word-split an unquoted variable) lands exactly here.
    scanned=0
    skipped=0
    for f in "${files[@]}"; do
      if is_excluded_path "$f"; then
        continue
      fi
      if [[ -f "$f" ]]; then
        scanned=$((scanned + 1))
        matches="$(grep -niE "$PATTERN" "$f")" && rc=0 || rc=$?
        if (( rc >= 2 )); then
          printf 'SCAN-ERROR %s: leak-check could NOT read this file (grep rc=%d) — treated as FAILURE, never as clean\n' "$f" "$rc" >> "$hits_file"
        elif [[ -n "$matches" ]]; then
          while IFS=: read -r lnum content; do
            printf 'LEAK %s: %s\n' "$f" "$content"
          done <<< "$matches" >> "$hits_file"
        fi
      else
        skipped=$((skipped + 1))
        printf 'NOT-SCANNED %s: not a regular file (deleted or misnamed) — examined by nothing, counted as clean by nothing\n' "$f" >&2
      fi
    done
    if (( scanned == 0 && ${#files[@]} > 0 )); then
      printf 'SCAN-ERROR: --files named %d path(s) and NONE were examined (%d not a regular file) — refusing to report clean\n' "${#files[@]}" "$skipped" >> "$hits_file"
    fi
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
