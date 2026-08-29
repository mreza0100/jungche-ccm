#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-}"
shift || true
MANIFEST="$ROOT/.professor/manifest.json"
STATE_VERSION="$ROOT/.professor/VERSION"
SOURCE_VERSION="$ROOT/VERSION"

failures=0
fail() {
  echo "self-hosted-manifest: $*" >&2
  failures=$((failures + 1))
}

repo_git() {
  if [[ -n "${PFM_DEV_REPO_GIT_DIR:-}" && -n "${PFM_DEV_REPO_WORK_TREE:-}" ]]; then
    git --git-dir="$PFM_DEV_REPO_GIT_DIR" --work-tree="$PFM_DEV_REPO_WORK_TREE" \
      -c safe.directory="$PFM_DEV_REPO_WORK_TREE" "$@"
  else
    git -C "$ROOT" "$@"
  fi
}

for tool in git jq sort; do
  command -v "$tool" >/dev/null 2>&1 || { fail "TOOLCHAIN-MISSING $tool"; }
done
if (( failures > 0 )); then exit 1; fi
for path in "$ROOT" "$MANIFEST" "$STATE_VERSION" "$SOURCE_VERSION"; do
  [[ -e "$path" ]] || fail "missing $path"
done
if (( failures > 0 )); then exit 1; fi
if ! jq -e . "$MANIFEST" >/dev/null; then
  fail "unreadable JSON: $MANIFEST"
  exit 1
fi

mode="$(jq -r '.installed_from.mode // empty' "$MANIFEST")"
[[ "$mode" == "self-hosted" ]] || fail "installed_from.mode=$mode, want self-hosted"
if jq -e '.installed_from | has("source_sha")' "$MANIFEST" >/dev/null; then
  fail "installed_from.source_sha must be absent for live self-hosted source"
fi
source_version="$(<"$SOURCE_VERSION")"
state_version="$(<"$STATE_VERSION")"
manifest_version="$(jq -r '.installed_from.version // empty' "$MANIFEST")"
[[ "$state_version" == "$source_version" ]] || fail ".professor/VERSION=$state_version, root VERSION=$source_version"
[[ "$manifest_version" == "$source_version" ]] || fail "manifest version=$manifest_version, root VERSION=$source_version"

TMP="$(mktemp -d)"
trap 'rm -rf -- "$TMP"' EXIT
printf '%s\n' "$@" | LC_ALL=C sort -u >"$TMP/want-roster"
if ! jq -r '.answers.roster[]?.dir' "$MANIFEST" | LC_ALL=C sort -u >"$TMP/got-roster"; then
  fail "cannot enumerate answers.roster"
elif ! diff -u "$TMP/want-roster" "$TMP/got-roster"; then
  fail "answers.roster does not match the development roster"
fi

repo_git ls-files | while IFS= read -r path; do
  case "$path" in
    .claude/*|.codex/*|.opencode/*|.gitignore|AGENTS.md|CLAUDE.md|pfm/AGENTS.md|pfm/CLAUDE.md|docs/commands/pfm/references/*)
      printf '%s\n' "$path"
      ;;
  esac
done | LC_ALL=C sort -u >"$TMP/want-files"
if ! jq -r '.file_hashes | keys[]' "$MANIFEST" | LC_ALL=C sort -u >"$TMP/got-files"; then
  fail "cannot enumerate file_hashes"
elif ! diff -u "$TMP/want-files" "$TMP/got-files"; then
  fail "file_hashes coverage does not match the installed tracked surface"
fi

digest() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    return 127
  fi
}

while IFS=$'\t' read -r path expected; do
  case "$path" in
    /*|../*|*/../*) fail "unsafe file_hashes path=$path"; continue ;;
  esac
  if [[ ! -f "$ROOT/$path" ]]; then
    fail "hashed file missing: $path"
    continue
  fi
  if ! actual="$(digest "$ROOT/$path")"; then
    fail "TOOLCHAIN-MISSING sha256sum or shasum"
    break
  fi
  [[ "$actual" == "$expected" ]] || fail "hash mismatch: $path"
done < <(jq -r '.file_hashes | to_entries[] | [.key,.value] | @tsv' "$MANIFEST")

if (( failures > 0 )); then
  echo "self-hosted-manifest: $failures failure(s)" >&2
  exit 1
fi
echo "self-hosted-manifest: clean"
