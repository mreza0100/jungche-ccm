#!/usr/bin/env bash
set -euo pipefail

# refresh-scope.sh — incremental refresh: reads blueprint/refresh-map.json (template →
# live source paths + SHA-256 as of the last sync), hashes the live sources, and
# reports which templates need LLM re-derivation. UNCHANGED hashes are a mechanical
# untouched-proof — skipped. UNMAPPED live files need a mapping decision. `regen`
# rewrites the map's hashes after a release's template edits land.
#
# A MISSING-SOURCE entry (the live source a template was derived from is gone) is a
# BLOCKING ruling, not a warning: both `scan` and `regen` exit MISSING_SOURCE_EXIT so a
# release cannot proceed and cannot re-baseline until each one is ruled — delete the
# template file AND its refresh-map.json entry, or remap it to the source's new path.
# Re-baselining around a missing source is what keeps a zombie template alive forever.

MISSING_SOURCE_EXIT=3

usage() {
  echo "usage: $(basename "$0") scan|regen <project_root> [map_path]" >&2
  echo "  exit ${MISSING_SOURCE_EXIT}: unruled MISSING-SOURCE entries (see above)" >&2
  exit 1
}

[[ $# -ge 2 ]] || usage
CMD="$1"
PROJECT_ROOT_ARG="$2"
case "$CMD" in
  scan|regen) ;;
  *) usage ;;
esac

[[ -d "$PROJECT_ROOT_ARG" ]] || { echo "refresh-scope: project_root not found: $PROJECT_ROOT_ARG" >&2; exit 1; }
PROJECT_ROOT="$(cd "$PROJECT_ROOT_ARG" && pwd)"

DEFAULT_MAP="$(dirname "$(readlink -f "$0")")/../blueprint/refresh-map.json"
MAP_PATH="${3:-$DEFAULT_MAP}"

[[ -f "$MAP_PATH" ]] || { echo "refresh-scope: map not found at $MAP_PATH" >&2; exit 1; }

MANIFEST_FILE="$PROJECT_ROOT/.professor/manifest.json"

# Resolves {project:ROLE} (via .professor/manifest.json .interview.projects.ROLE)
# and a leading ~/ (to $HOME) in a map path/glob string.
resolve_path() {
  local resolved="$1"
  while [[ "$resolved" =~ \{project:([a-zA-Z0-9_-]+)\} ]]; do
    local role="${BASH_REMATCH[1]}"
    [[ -f "$MANIFEST_FILE" ]] || {
      echo "refresh-scope: manifest not found at $MANIFEST_FILE (needed to resolve {project:$role})" >&2
      exit 1
    }
    local val
    val="$(jq -r --arg r "$role" '.interview.projects[$r] // empty' "$MANIFEST_FILE")"
    [[ -n "$val" ]] || {
      echo "refresh-scope: manifest .interview.projects.$role is missing/null" >&2
      exit 1
    }
    resolved="${resolved//\{project:$role\}/$val}"
  done
  case "$resolved" in
    "~/"*) resolved="${HOME}/${resolved#\~/}" ;;
  esac
  printf '%s\n' "$resolved"
}

abspath_under_project() {
  case "$1" in
    /*) printf '%s\n' "$1" ;;
    *) printf '%s\n' "$PROJECT_ROOT/$1" ;;
  esac
}

is_ignored() {
  local path="$1" entry resolved_entry
  while IFS= read -r entry; do
    [[ -z "$entry" ]] && continue
    resolved_entry="$(resolve_path "$entry")"
    if [[ "$resolved_entry" == */ ]]; then
      [[ "$path" == "$resolved_entry"* ]] && return 0
    else
      [[ "$path" == "$resolved_entry" ]] && return 0
    fi
  done < <(jq -r '.ignore_sources[]? // empty' "$MAP_PATH")
  return 1
}

list_glob_files() {
  local pattern resolved
  pattern="$1"
  resolved="$(resolve_path "$pattern")"
  (
    cd "$PROJECT_ROOT"
    shopt -s globstar nullglob
    for f in $resolved; do
      [[ -f "$f" ]] && printf '%s\n' "$f"
    done
  )
}

scan() {
  local c=0 u=0 k=0 m=0 x=0

  k="$(jq -r '.templates | to_entries[] | select(.value.curated == true) | .key' "$MAP_PATH" | wc -l | tr -d ' ')"

  local mapped_sources_file
  mapped_sources_file="$(mktemp)"

  declare -A template_ok=()
  declare -A template_seen=()

  while IFS=$'\t' read -r tmpl src expected; do
    [[ -z "$tmpl" ]] && continue
    template_seen["$tmpl"]=1
    [[ -z "${template_ok[$tmpl]+x}" ]] && template_ok["$tmpl"]=1

    local resolved_rel abs
    resolved_rel="$(resolve_path "$src")"
    abs="$(abspath_under_project "$resolved_rel")"
    printf '%s\n' "$resolved_rel" >> "$mapped_sources_file"

    if [[ ! -f "$abs" ]]; then
      echo "MISSING-SOURCE ${tmpl} <= ${src}"
      x=$((x + 1))
      template_ok["$tmpl"]=0
      continue
    fi

    local actual
    actual="$(sha256sum "$abs" | awk '{print $1}')"
    if [[ "$actual" != "$expected" ]]; then
      echo "CHANGED ${tmpl} <= ${src}"
      c=$((c + 1))
      template_ok["$tmpl"]=0
    fi
  done < <(jq -r '.templates | to_entries[] | select(.value.sources) | .key as $t | .value.sources | to_entries[] | [$t, .key, .value] | @tsv' "$MAP_PATH")

  for tmpl in "${!template_seen[@]}"; do
    [[ "${template_ok[$tmpl]}" == "1" ]] && u=$((u + 1))
  done

  mapfile -t MAPPED_SOURCES < <(sort -u "$mapped_sources_file")
  rm -f "$mapped_sources_file"

  mapfile -t ALL_GLOB_FILES < <(
    while IFS= read -r glob; do
      [[ -z "$glob" ]] && continue
      list_glob_files "$glob"
    done < <(jq -r '.source_globs[]? // empty' "$MAP_PATH") | sort -u
  )

  for f in "${ALL_GLOB_FILES[@]}"; do
    local is_mapped=0 ms
    for ms in "${MAPPED_SOURCES[@]}"; do
      if [[ "$f" == "$ms" ]]; then
        is_mapped=1
        break
      fi
    done
    (( is_mapped )) && continue
    is_ignored "$f" && continue
    echo "UNMAPPED-LIVE ${f}"
    m=$((m + 1))
  done

  echo "refresh-scope: ${c} changed, ${u} unchanged (skip re-derivation), ${k} curated, ${m} unmapped-live, ${x} missing-source"

  if (( x > 0 )); then
    echo "refresh-scope: BLOCKED — ${x} missing-source entr$( (( x == 1 )) && echo y || echo ies) must be ruled before releasing: delete the template file AND its refresh-map.json entry, or remap it" >&2
    return "$MISSING_SOURCE_EXIT"
  fi
}

regen() {
  local frag_file n=0 x=0

  # Refuse to re-baseline while any mapped source is missing — writing fresh hashes
  # around a gone source silently keeps its template alive as a zombie. Nothing is
  # written until every mapped source resolves.
  while IFS=$'\t' read -r tmpl src expected; do
    [[ -z "$tmpl" ]] && continue
    local resolved_rel abs
    resolved_rel="$(resolve_path "$src")"
    abs="$(abspath_under_project "$resolved_rel")"
    if [[ ! -f "$abs" ]]; then
      echo "MISSING-SOURCE ${tmpl} <= ${src}" >&2
      x=$((x + 1))
    fi
  done < <(jq -r '.templates | to_entries[] | select(.value.sources) | .key as $t | .value.sources | to_entries[] | [$t, .key, .value] | @tsv' "$MAP_PATH")

  if (( x > 0 )); then
    echo "refresh-scope: REFUSING to regen — ${x} missing-source entr$( (( x == 1 )) && echo y || echo ies); ${MAP_PATH} left unchanged. Rule each first: delete the template file AND its refresh-map.json entry, or remap it" >&2
    return "$MISSING_SOURCE_EXIT"
  fi

  frag_file="$(mktemp)"

  while IFS=$'\t' read -r tmpl src expected; do
    [[ -z "$tmpl" ]] && continue
    local resolved_rel abs
    resolved_rel="$(resolve_path "$src")"
    abs="$(abspath_under_project "$resolved_rel")"
    local actual
    actual="$(sha256sum "$abs" | awk '{print $1}')"
    jq -nc --arg t "$tmpl" --arg s "$src" --arg h "$actual" '{t:$t,s:$s,h:$h}' >> "$frag_file"
    n=$((n + 1))
  done < <(jq -r '.templates | to_entries[] | select(.value.sources) | .key as $t | .value.sources | to_entries[] | [$t, .key, .value] | @tsv' "$MAP_PATH")

  local tmp
  tmp="$(mktemp "${MAP_PATH}.XXXXXX")"
  jq --slurpfile updates "$frag_file" '
    reduce $updates[] as $u (.; .templates[$u.t].sources[$u.s] = $u.h)
  ' "$MAP_PATH" > "$tmp"
  mv "$tmp" "$MAP_PATH"
  rm -f "$frag_file"

  echo "refresh-scope: regenerated ${n} hashes into ${MAP_PATH}"
}

case "$CMD" in
  scan) scan ;;
  regen) regen ;;
esac
