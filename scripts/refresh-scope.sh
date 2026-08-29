#!/usr/bin/env bash
set -euo pipefail

# refresh-scope.sh — incremental refresh: reads templates/refresh-map.json (template →
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

# `ledgers` is the other half of a release's scope, and it is mechanical for the
# same reason `scan` is. A pending `.professor/release.md` bullet in a LINKED
# project is a framework change waiting to ship, and before this subcommand
# existed nothing ever opened one: /ptm:release read this repo's own ledger and
# no other, so a release that swept every linked project and a release that
# swept none printed the identical output. This enumerates the ledgers, counts
# what is pending in each, and — the part that matters — refuses to be silent
# about a ledger it could not read. Bullet TEXT is emitted for a human or an
# LLM to judge; which bullet maps to which template is judgment, not shell.
MISSING_SOURCE_EXIT=3
LEDGER_UNREADABLE_EXIT=4

usage() {
  echo "usage: $(basename "$0") scan|regen <project_root> [map_path]" >&2
  echo "       $(basename "$0") ledgers <root> [root...]" >&2
  echo "  exit ${MISSING_SOURCE_EXIT}: unruled MISSING-SOURCE entries (see above)" >&2
  echo "  exit ${LEDGER_UNREADABLE_EXIT}: a .professor/release.md could not be read" >&2
  exit 1
}

[[ $# -ge 2 ]] || usage
CMD="$1"

# ledgers enumerates every `.professor/release.md` reachable from the named
# roots — each root itself, plus the sub-projects its own manifest names by
# role, which refresh-map.json already treats as first-class live sources.
#
# Every ledger gets a verdict, and the four verdicts are deliberately distinct:
# ABSENT means the directory is not a Professor install, EMPTY means the ledger
# was opened and holds nothing pending, UNREADABLE means the look FAILED, and
# PENDING carries a count plus the bullet text. Collapsing UNREADABLE into
# EMPTY is the exact defect this whole subcommand exists to prevent: a release
# is allowed to ship with nothing pending, and is never allowed to ship because
# it could not find out.
ledgers() {
  [[ $# -ge 1 ]] || usage
  local swept=0 pending=0 bullets=0 empty=0 absent=0 unreadable=0
  local -a queue=() seen=()
  local root abs manifest sub dir ledger count already prior

  for root in "$@"; do
    if [[ ! -d "$root" ]]; then
      echo "refresh-scope: ledger root not found: $root" >&2
      return 1
    fi
    abs="$(cd "$root" && pwd)"
    queue+=("$abs")
    manifest="$abs/.professor/manifest.json"
    if [[ -f "$manifest" ]]; then
      while IFS= read -r sub; do
        [[ -z "$sub" ]] && continue
        case "$sub" in
          /*) ;;
          "~/"*) sub="${HOME}/${sub#\~/}" ;;
          *) sub="$abs/$sub" ;;
        esac
        if [[ -d "$sub" ]]; then
          queue+=("$(cd "$sub" && pwd)")
        else
          echo "LEDGER-ROOT-MISSING ${sub} (named by ${manifest})"
        fi
      done < <(jq -r '.interview.projects // {} | to_entries[]? | .value // empty' "$manifest" 2>/dev/null || true)
    fi
  done

  for dir in "${queue[@]}"; do
    already=0
    for prior in ${seen[@]+"${seen[@]}"}; do
      if [[ "$prior" == "$dir" ]]; then
        already=1
      fi
    done
    (( already )) && continue
    seen+=("$dir")
    swept=$((swept + 1))
    ledger="$dir/.professor/release.md"

    if [[ ! -e "$ledger" ]]; then
      echo "LEDGER-ABSENT ${dir}"
      absent=$((absent + 1))
      continue
    fi
    if [[ ! -r "$ledger" ]] || ! head -c 1 "$ledger" >/dev/null 2>&1; then
      echo "LEDGER-UNREADABLE ${ledger}"
      unreadable=$((unreadable + 1))
      continue
    fi

    count="$(awk '/^## Pending/{inside=1;next} /^## /{inside=0} inside && /^- /{n++} END{print n+0}' "$ledger")"
    if (( count == 0 )); then
      echo "LEDGER-EMPTY ${ledger}"
      empty=$((empty + 1))
      continue
    fi
    pending=$((pending + 1))
    bullets=$((bullets + count))
    echo "LEDGER-PENDING ${ledger} bullets=${count}"
    awk '/^## Pending/{inside=1;next} /^## /{inside=0} inside' "$ledger" | sed 's/^/  | /'
  done

  echo "refresh-scope: ledgers swept=${swept} pending=${pending} bullets=${bullets} empty=${empty} absent=${absent} unreadable=${unreadable}"

  if (( unreadable > 0 )); then
    echo "refresh-scope: BLOCKED — ${unreadable} ledger(s) could not be READ; a release cannot claim it swept a ledger it never opened. Fix the permission or name the root correctly, then re-run" >&2
    return "$LEDGER_UNREADABLE_EXIT"
  fi
  return 0
}

# ledgers takes N roots and no map, so it is dispatched before the single-root
# argument handling below claims $2.
if [[ "$CMD" == "ledgers" ]]; then
  shift
  ledgers "$@"
  exit $?
fi

PROJECT_ROOT_ARG="$2"
case "$CMD" in
  scan|regen) ;;
  *) usage ;;
esac

[[ -d "$PROJECT_ROOT_ARG" ]] || { echo "refresh-scope: project_root not found: $PROJECT_ROOT_ARG" >&2; exit 1; }
PROJECT_ROOT="$(cd "$PROJECT_ROOT_ARG" && pwd)"

DEFAULT_MAP="$(dirname "$(readlink -f "$0")")/../templates/refresh-map.json"
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
