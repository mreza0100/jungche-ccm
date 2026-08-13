#!/usr/bin/env bash
# shellcheck disable=SC2016
# One-night agent-transcript distillation: two Codex seats, four mechanical gates.
set -euo pipefail

export LC_ALL=C
umask 077

ENGINE_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
DEFAULT_REPO_ROOT=/home/user/work/proja
REGISTRY_BASE=/home/user/.claude/projects
REPO_ROOT=$DEFAULT_REPO_ROOT
ORGAN=''
REGISTRY=''
BOOTSTRAP_COUNT=''
CORPUS_FILE=''
AGENT_TYPE=Explore
LANE=explorer
LANE_DIR="$ENGINE_DIR/lanes"
DISTILL_TEMPLATE="$ENGINE_DIR/dreamer-distill.prompt.md"
REFINER_TEMPLATE="$ENGINE_DIR/dreamer-refiner.prompt.md"
DISTILL_MODEL=gpt-5.6-luna
DISTILL_EFFORT=xhigh
REFINER_MODEL=gpt-5.6-luna
REFINER_EFFORT=xhigh
SEAT_TIMEOUT_SECONDS=2700
ANCHOR_RE='^- `([^`]+)` — (blob|tree) `([0-9a-f]{12})`$'
LEGACY_ANCHOR_RE='^- `([^`]+)` — `git log -1`: `([0-9a-f]{12})` \(([0-9]{4}-[0-9]{2}-[0-9]{2})\); (blob|tree) `([0-9a-f]{12})`$'

die() {
  printf 'dreamer-night: FAIL: %s\n' "$*" >&2
  exit 1
}

require_file() {
  [ -f "$1" ] || die "missing file: $1"
}

require_dir() {
  [ -d "$1" ] || die "missing directory: $1"
}

require_commands() {
  local command_name
  for command_name in awk basename chmod cmp codex comm cp cut date diff dirname find flock git grep head id jq mkdir mktemp mv realpath rm sed sha256sum sort stat tail tee timeout tr uniq wc; do
    command -v "$command_name" >/dev/null 2>&1 || die "required command unavailable: $command_name"
  done
}

configure_repo() {
  local requested=$1 resolved encoded
  [[ "$requested" == /* ]] || die "repository root must be absolute: $requested"
  resolved=$(realpath -e "$requested") || die "repository root does not resolve: $requested"
  [ "$resolved" = "$requested" ] || die "repository root must be canonical: $requested"
  [ ! -L "$requested" ] || die "repository root is a symlink: $requested"
  REPO_ROOT=$requested
  ORGAN="$REPO_ROOT/.professor/stm"
  encoded=${REPO_ROOT//\//-}
  REGISTRY="$REGISTRY_BASE/$encoded"
}

configure_lane() {
  local requested=$1 slug
  [ -n "$requested" ] || die 'agent type must not be empty'
  [[ "$requested" != *[[:space:]]* ]] || die "agent type must not contain whitespace: $requested"
  AGENT_TYPE=$requested
  if [ "$requested" = Explore ]; then
    slug=explorer
  else
    slug=$(tr '[:upper:]' '[:lower:]' <<< "$requested" | tr -c 'a-z0-9-' '-')
    slug=${slug%-}
  fi
  [[ "$slug" =~ ^[a-z0-9][a-z0-9-]*$ ]] || die "agent type does not yield a lane slug: $requested"
  LANE=$slug
}

# A lane profile names the corpus and the audience the maps must serve; a lane
# without one cannot run, so no agent type is ever harvested by accident.
# A profile for a PROJECT-SPECIFIC agent type lives in that project's own organ;
# the engine directory carries only lanes every repository shares.
LANE_PROFILE=''
# LAW (the operator, 2026-08-12): the dreamer distills on luna. Always. The verify seat
# is a different job — an adversarial falsifier — and runs sol. Swapping the
# distilling model changes what the organ remembers, so it fails closed here
# rather than depending on whoever edits the constant above.
require_seat_law() {
  [ "$DISTILL_MODEL" = gpt-5.6-luna ] || \
    die "the dreamer distills on luna only; refusing DISTILL_MODEL=$DISTILL_MODEL"
}

require_lane_profile() {
  if [ -f "$ORGAN/lanes/$LANE.md" ]; then
    LANE_PROFILE="$ORGAN/lanes/$LANE.md"
  elif [ -f "$LANE_DIR/$LANE.md" ]; then
    LANE_PROFILE="$LANE_DIR/$LANE.md"
  else
    die "lane has no profile: expected $ORGAN/lanes/$LANE.md or $LANE_DIR/$LANE.md"
  fi
}

lane_of_map() {
  local lanes_file=$1 map=$2 lane
  if [ ! -f "$lanes_file" ]; then
    printf 'explorer\n'
    return 0
  fi
  lane=$(awk -F '\t' -v m="$map" '$1 == m { print $2; exit }' "$lanes_file")
  [ -n "$lane" ] || die "map carries no lane row: $map"
  printf '%s\n' "$lane"
}

# Membership is written over the whole pool every apply. A pool that predates
# lanes carries no ledger file at all, and every map in it came from an Explore
# night; a pool that HAS the ledger and still hides a map was hand-edited, and
# that fails closed rather than guessing an audience for it.
write_lane_membership() {
  local maps_dir=$1 existing=$2 lane=$3 out=$4
  local map_file slug row_lane
  : > "$out"
  shopt -s nullglob
  for map_file in "$maps_dir"/*.md; do
    slug=$(basename "$map_file")
    row_lane=''
    [ ! -f "$existing" ] || row_lane=$(awk -F '\t' -v m="$slug" '$1 == m { print $2; exit }' "$existing")
    if [ -z "$row_lane" ]; then
      if [ -e "$ORGAN/maps/$slug" ]; then
        [ ! -f "$existing" ] || die "pre-existing map carries no lane row: $slug"
        row_lane=explorer
      else
        row_lane=$lane
      fi
    fi
    [[ "$row_lane" =~ ^[a-z0-9][a-z0-9-]*$ ]] || die "invalid lane for map: $slug -> $row_lane"
    printf '%s\t%s\n' "$slug" "$row_lane" >> "$out"
  done
  shopt -u nullglob
  sort -o "$out" "$out"
  [ -z "$(cut -f1 "$out" | uniq -d)" ] || die "duplicate lane rows: $out"
  chmod 0600 "$out"
}

require_repo_context() {
  local repo_top organ_top
  require_dir "$REPO_ROOT"
  git -C "$REPO_ROOT" rev-parse --verify HEAD >/dev/null 2>&1 || die "repository has no HEAD: $REPO_ROOT"
  repo_top=$(git -C "$REPO_ROOT" rev-parse --show-toplevel 2>/dev/null) || die "repository has no top level: $REPO_ROOT"
  [ "$repo_top" = "$REPO_ROOT" ] || die "repository root is not the Git top level: $REPO_ROOT"
  require_dir "$ORGAN"
  require_dir "$ORGAN/maps"
  require_dir "$ORGAN/dreamer"
  require_dir "$ORGAN/archive"
  require_file "$ORGAN/stm.md"
  # The organ lives INSIDE its repository and is tracked with it — the dreams are part
  # of the project, not a hidden side-ledger. This engine therefore writes FILES only:
  # committing is the repository's own flow (gitter), never this script's. A `git -C`
  # here would walk up and commit straight to the parent branch.
  organ_top=$(git -C "$ORGAN" rev-parse --show-toplevel 2>/dev/null) || die "organ is not inside a Git repository: $ORGAN"
  # Two shapes are legal: tracked inside its repository (the organ is part of the
  # project), or its own ledger (repos not yet migrated). Anything else is an organ
  # sitting outside version control entirely.
  case "$ORGAN" in
    "$organ_top") : ;;
    "$organ_top"/*) : ;;
    *) die "organ escapes its repository root: $ORGAN" ;;
  esac
  require_dir "$REGISTRY"
}

take_runner_lock() {
  exec 9<"$0"
  flock -n 9 || die 'another dreamer-night process holds the runner lock'
}

# The stage lives INSIDE the organ so a night is watchable and diffable while it
# runs, and so a failed night leaves committed evidence instead of a /tmp corpse
# nobody can find. The seat cannot exploit that proximity: `codex exec --sandbox
# workspace-write --cd {stage}` scopes writes to the stage even when the stage
# sits inside a Git repository — probed 2026-08-12, writes to `../`,
# `../../maps/`, and the repository root are all DENIED.
stage_root() {
  printf '%s\n' "$ORGAN/dreamer/staging"
}

stage_path() {
  local stage=$1
  local resolved
  resolved=$(realpath -e "$stage") || die "staging directory does not resolve: $stage"
  [ "$resolved" = "$stage" ] || die "staging path must be canonical: $stage"
  [[ "$stage" == "$(stage_root)"/* || "$stage" == /tmp/dreamer-night-* ]] || \
    die "staging path is outside $(stage_root): $stage"
  [ ! -L "$stage" ] || die "staging directory is a symlink: $stage"
  [ "$(stat -c %u "$stage")" = "$(id -u)" ] || die "staging directory has the wrong owner: $stage"
  [ "$(stat -c %a "$stage")" = 700 ] || die "staging directory mode is not 0700: $stage"
}

new_stage() {
  local stamp stage
  stamp=$(date +%Y%m%dT%H%M%S)
  mkdir -m 0700 -p "$(stage_root)"
  stage=$(mktemp -d "$(stage_root)/${LANE}-${stamp}.XXXXXX")
  chmod 0700 "$stage"
  mkdir -m 0700 "$stage/maps" "$stage/meta"
  printf '%s\n' "$stage"
}

map_fingerprint() {
  local maps_dir=$1
  local map_file file_hash
  {
    shopt -s nullglob
    for map_file in "$maps_dir"/*.md; do
      file_hash=$(sha256sum "$map_file" | cut -d ' ' -f 1)
      printf '%s  %s\n' "$file_hash" "$(basename "$map_file")"
    done
    shopt -u nullglob
  } | sha256sum | cut -d ' ' -f 1
}

latest_applied_sweep() {
  local sweep base candidate_date candidate_sequence sweep_lane
  local latest='' latest_date='' latest_sequence=0
  shopt -s nullglob
  for sweep in "$ORGAN"/dreamer/*.md; do
    base=$(basename "$sweep")
    [[ "$base" =~ ^([0-9]{4}-[0-9]{2}-[0-9]{2})(-([0-9]+))?\.md$ ]] || continue
    candidate_date=${BASH_REMATCH[1]}
    candidate_sequence=${BASH_REMATCH[3]:-1}
    grep -qx 'END-OF-SWEEP' "$sweep" || continue
    # The window belongs to the lane: an Explore sweep must never cut off the
    # QA lane's corpus, and a sweep written before lanes existed is Explore's.
    sweep_lane=$(awk -F '\t' '$1 == "lane" && NF == 2 { print $2; exit }' "$sweep")
    [ -n "$sweep_lane" ] || sweep_lane=explorer
    [ "$sweep_lane" = "$LANE" ] || continue
    if [ -z "$latest" ] || [[ "$candidate_date" > "$latest_date" ]] || \
      { [ "$candidate_date" = "$latest_date" ] && [ "$candidate_sequence" -gt "$latest_sequence" ]; }; then
      latest=$sweep
      latest_date=$candidate_date
      latest_sequence=$candidate_sequence
    fi
  done
  shopt -u nullglob
  printf '%s\n' "$latest"
}

valid_cutoff() {
  [ -n "$1" ] && date -d "$1" +%s >/dev/null 2>&1
}

write_cached_titles() {
  local output=$1
  local map_file title question
  : > "$output"
  shopt -s nullglob
  for map_file in "$ORGAN"/maps/*.md; do
    IFS= read -r title < "$map_file" || true
    [[ "$title" == '# '* ]] || die "existing map lacks an H1: $map_file"
    title=${title#\# }
    [[ "$title" != *$'\t'* ]] || die "existing map title contains a tab: $map_file"
    # The dedup surface carries what each map ANSWERS, not only what it is called.
    # The seat may not read map bodies, so a bare title makes it guess at containment
    # and decline a real lesson because some existing title sounded close enough.
    question=$(awk '/^## Question/ { flag = 1; next } flag && NF { print; exit }' "$map_file" | tr '\t\n' '  ')
    if [ -n "$question" ]; then
      printf '%s — %s\n' "$title" "$question" >> "$output"
    else
      printf '%s\n' "$title" >> "$output"
    fi
  done
  shopt -u nullglob
  sort -u -o "$output" "$output"
}

enumerate_corpus() {
  local stage=$1
  local latest cutoff cutoff_source enumerated_at applied_at filename_date meta agent_type transcript
  local candidates="$stage/meta/paired-candidates.tsv"
  local candidates_sorted="$stage/meta/paired-candidates.sorted.tsv"
  local metas="$stage/meta/metas.nul"
  local total=0 explore=0 paired=0 selected=0 gaps=0 excluded=0 invalid=0
  local listed=0 corpus_line

  enumerated_at=$(date -Is)
  if [ -n "$CORPUS_FILE" ]; then
    : > "$stage/paths.txt"
    : > "$stage/gaps.tsv"
    while IFS= read -r corpus_line || [ -n "$corpus_line" ]; do
      [ -n "$corpus_line" ] || continue
      # A corpus file documents its own selection rule in leading # comments;
      # the seat reads them, and the stage keeps the file as the audit trail.
      [[ "$corpus_line" != '#'* ]] || continue
      listed=$((listed + 1))
      [[ "$corpus_line" == /* ]] || die "corpus path is not absolute: $corpus_line"
      [ -f "$corpus_line" ] || die "corpus path is not a readable file: $corpus_line"
      printf '%s\n' "$corpus_line" >> "$stage/paths.txt"
    done < "$CORPUS_FILE"
    sort -u -o "$stage/paths.txt" "$stage/paths.txt"
    selected=$(wc -l < "$stage/paths.txt")
    cp "$CORPUS_FILE" "$stage/meta/corpus-file.txt"
    {
      printf 'window-mode\tcorpus-file\n'
      printf 'corpus-file\t%s\n' "$CORPUS_FILE"
      printf 'corpus-file-sha256\t%s\n' "$(sha256sum "$CORPUS_FILE" | cut -d ' ' -f 1)"
      printf 'agent-type\t%s\nlane\t%s\n' "$AGENT_TYPE" "$LANE"
      printf 'cutoff-exclusive\tNONE\nenumerated-at\t%s\n' "$enumerated_at"
    } > "$stage/meta/window.tsv"
    {
      printf 'window-meta-count\t%s\n' "$listed"
      printf 'agent-meta-count\t%s\n' "$listed"
      printf 'paired-transcript-count\t%s\n' "$listed"
      printf 'selected-paired-transcript-count\t%s\n' "$selected"
      printf 'omitted-paired-transcript-count\t%s\n' "$((listed - selected))"
      printf 'coverage-gap-count\t0\n'
      printf 'excluded-other-agent-or-invalid-count\t0\n'
      printf 'invalid-meta-count\t0\n'
    } > "$stage/census.tsv"
    sha256sum "$stage/paths.txt" | cut -d ' ' -f 1 > "$stage/paths.sha256"
    return 0
  fi
  if [ -n "$BOOTSTRAP_COUNT" ]; then
    cutoff='NONE'
    printf 'window-mode\tbootstrap-count\nbootstrap-count\t%s\nagent-type\t%s\nlane\t%s\ncutoff-exclusive\tNONE\nenumerated-at\t%s\n' \
      "$BOOTSTRAP_COUNT" "$AGENT_TYPE" "$LANE" "$enumerated_at" > "$stage/meta/window.tsv"
  else
    latest=$(latest_applied_sweep)
    if [ -n "$latest" ]; then
      cutoff=$(awk -F '\t' '$1 == "enumerated-at" && NF == 2 { value=$2 } END { print value }' "$latest")
      cutoff_source=enumerated-at
      if ! valid_cutoff "$cutoff"; then
        applied_at=$(sed -n 's/^Applied: //p' "$latest" | tail -n 1)
        cutoff=$applied_at
        cutoff_source=Applied
      fi
      if ! valid_cutoff "$cutoff"; then
        filename_date=$(basename "$latest")
        filename_date=${filename_date:0:10}
        cutoff="$filename_date 00:00:00"
        cutoff_source=filename-date
      fi
      printf 'window-mode\tsweep-cutoff\nnewest-applied-sweep\t%s\ncutoff-source\t%s\nagent-type\t%s\nlane\t%s\ncutoff-exclusive\t%s\nenumerated-at\t%s\n' \
        "$(basename "$latest")" "$cutoff_source" "$AGENT_TYPE" "$LANE" "$cutoff" "$enumerated_at" > "$stage/meta/window.tsv"
    else
      cutoff='7 days ago'
      printf 'window-mode\tsweep-cutoff\nnewest-applied-sweep\tNONE\ncutoff-source\tbootstrap\nagent-type\t%s\nlane\t%s\ncutoff-exclusive\t%s\nenumerated-at\t%s\n' \
        "$AGENT_TYPE" "$LANE" "$cutoff" "$enumerated_at" > "$stage/meta/window.tsv"
    fi
  fi

  : > "$stage/paths.txt"
  : > "$stage/gaps.tsv"
  : > "$candidates"
  if [ -n "$BOOTSTRAP_COUNT" ]; then
    find "$REGISTRY" -type f -name 'agent-*.meta.json' -print0 > "$metas"
  else
    find "$REGISTRY" -type f -name 'agent-*.meta.json' -newermt "$cutoff" -print0 > "$metas"
  fi
  sort -z -o "$metas" "$metas"
  while IFS= read -r -d '' meta; do
    total=$((total + 1))
    if ! agent_type=$(jq -er '.agentType | strings' "$meta" 2>/dev/null); then
      invalid=$((invalid + 1))
      excluded=$((excluded + 1))
      continue
    fi
    if [ "$agent_type" != "$AGENT_TYPE" ]; then
      excluded=$((excluded + 1))
      continue
    fi
    explore=$((explore + 1))
    transcript=${meta%.meta.json}.jsonl
    if [ -f "$transcript" ]; then
      paired=$((paired + 1))
      if [ -n "$BOOTSTRAP_COUNT" ]; then
        [[ "$meta" != *$'\t'* && "$meta" != *$'\n'* && "$transcript" != *$'\t'* && "$transcript" != *$'\n'* ]] || \
          die "registry path cannot be represented safely: $meta"
        printf '%s\t%s\t%s\n' "$(date -r "$meta" +%s.%N)" "$meta" "$transcript" >> "$candidates"
      else
        printf '%s\n' "$transcript" >> "$stage/paths.txt"
      fi
    else
      printf 'META-PRESENT-TRANSCRIPT-MISSING\t%s\t%s\n' "$meta" "$transcript" >> "$stage/gaps.tsv"
      gaps=$((gaps + 1))
    fi
  done < "$metas"

  if [ -n "$BOOTSTRAP_COUNT" ]; then
    sort -t $'\t' -k1,1r -k2,2 "$candidates" > "$candidates_sorted"
    awk -v count="$BOOTSTRAP_COUNT" 'NR <= count { print }' "$candidates_sorted" > "$stage/meta/bootstrap-selection.tsv"
    cut -f3 "$stage/meta/bootstrap-selection.tsv" | sort -u > "$stage/paths.txt"
  fi

  sort -u -o "$stage/paths.txt" "$stage/paths.txt"
  sort -u -o "$stage/gaps.tsv" "$stage/gaps.tsv"
  selected=$(wc -l < "$stage/paths.txt")
  gaps=$(wc -l < "$stage/gaps.tsv")
  {
    printf 'window-meta-count\t%s\n' "$total"
    printf 'agent-meta-count\t%s\n' "$explore"
    printf 'paired-transcript-count\t%s\n' "$paired"
    printf 'selected-paired-transcript-count\t%s\n' "$selected"
    printf 'omitted-paired-transcript-count\t%s\n' "$((paired - selected))"
    printf 'coverage-gap-count\t%s\n' "$gaps"
    printf 'excluded-other-agent-or-invalid-count\t%s\n' "$excluded"
    printf 'invalid-meta-count\t%s\n' "$invalid"
  } > "$stage/census.tsv"
  sha256sum "$stage/paths.txt" | cut -d ' ' -f 1 > "$stage/paths.sha256"
}

gate_pin() {
  local paths=$1 pin_file=$2
  local expected actual
  require_file "$paths"
  require_file "$pin_file"
  expected=$(sed -n '1p' "$pin_file")
  [[ "$expected" =~ ^[0-9a-f]{64}$ ]] || die "path pin is not one SHA-256: $pin_file"
  [ "$(wc -l < "$pin_file")" -eq 1 ] || die "path pin has extra lines: $pin_file"
  if grep -qEv '^/[^[:cntrl:]]+$' "$paths"; then
    die "paths file contains a blank, relative, or control-character path: $paths"
  fi
  if ! cmp -s "$paths" <(sort -u "$paths"); then
    die "paths file is not sorted and unique: $paths"
  fi
  actual=$(sha256sum "$paths" | cut -d ' ' -f 1)
  [ "$actual" = "$expected" ] || die "path pin mismatch: expected $expected, got $actual"
  printf 'PIN PASS %s %s\n' "$actual" "$paths"
}

# Coverage is keyed by the INDEX the runner handed the seat, never by a retyped
# path: a seat that must reproduce thirteen absolute paths by hand will
# eventually mistype one, and did. The engine expands indices back to paths for
# the ledger, so the audit trail still names every transcript.
gate_coverage() {
  local paths=$1 coverage=$2 pin_file=${3:-}
  local total actual_indexes duplicate_indexes missing_indexes format_errors expanded want have conduct_lines
  require_file "$paths"
  require_file "$coverage"
  [ -z "$pin_file" ] || gate_pin "$paths" "$pin_file" >/dev/null

  total=$(wc -l < "$paths")
  actual_indexes="${coverage}.indexes"
  duplicate_indexes="${coverage}.duplicates"
  missing_indexes="${coverage}.missing"
  format_errors="${coverage}.format-errors"
  expanded="${coverage}.expanded"
  want="${coverage}.want"
  have="${coverage}.have"
  conduct_lines="${coverage}.conduct"
  : > "$actual_indexes"
  : > "$format_errors"
  : > "$conduct_lines"

  [ "$(tail -n 1 "$coverage")" = END-OF-RUN ] || printf 'missing final END-OF-RUN\n' >> "$format_errors"
  [ "$(grep -c '^END-OF-RUN$' "$coverage" || true)" -eq 1 ] || printf 'END-OF-RUN must occur exactly once\n' >> "$format_errors"
  awk -F '\t' -v total="$total" -v indexes_out="$actual_indexes" -v errors_out="$format_errors" -v conduct_out="$conduct_lines" '
    $0 == "END-OF-RUN" { next }
    # The conduct accounting rides in the same file: one line per kind, so a night
    # that harvested no technique/prior/baseline has to say WHY rather than go quiet.
    $1 == "CONDUCT" {
      if (NF != 4) { print "line " NR ": expected CONDUCT<TAB>kind<TAB>slug|NONE<TAB>reason" >> errors_out; next }
      if ($2 != "technique" && $2 != "prior" && $2 != "baseline") { print "line " NR ": conduct kind is not technique, prior, or baseline" >> errors_out; next }
      if ($3 == "" || $4 == "") { print "line " NR ": conduct slug and reason are both required" >> errors_out; next }
      print $2 >> conduct_out; next
    }
    NF != 3 { print "line " NR ": expected index<TAB>READ|SKIP<TAB>reason" >> errors_out; next }
    $1 !~ /^[1-9][0-9]*$/ { print "line " NR ": first field is not a transcript index" >> errors_out; next }
    $1 + 0 > total { print "line " NR ": index " $1 " exceeds the " total " supplied transcripts" >> errors_out; next }
    $2 != "READ" && $2 != "SKIP" { print "line " NR ": status is not READ or SKIP" >> errors_out; next }
    $3 == "" { print "line " NR ": reason is empty" >> errors_out; next }
    { print $1 >> indexes_out }
  ' "$coverage"

  sort "$actual_indexes" | uniq -d > "$duplicate_indexes"
  seq 1 "$total" | sort > "$want"
  sort -u "$actual_indexes" > "$have"
  comm -23 "$want" "$have" > "$missing_indexes"
  # All three conduct kinds are accounted for, or the seat went quiet on one —
  # which is the silence rule 3 exists to forbid.
  for kind in technique prior baseline; do
    grep -qx "$kind" "$conduct_lines" || printf 'missing CONDUCT accounting for: %s\n' "$kind" >> "$format_errors"
  done
  if [ -s "$format_errors" ] || [ -s "$duplicate_indexes" ] || [ -s "$missing_indexes" ]; then
    printf 'COVERAGE FAIL %s\n' "$coverage" >&2
    [ ! -s "$format_errors" ] || { printf '%s\n' 'FORMAT:' >&2; sed -n '1,200p' "$format_errors" >&2; }
    [ ! -s "$duplicate_indexes" ] || { printf '%s\n' 'DUPLICATE INDEXES:' >&2; sed -n '1,200p' "$duplicate_indexes" >&2; }
    [ ! -s "$missing_indexes" ] || { printf '%s\n' 'UNRULED INDEXES:' >&2; sed -n '1,200p' "$missing_indexes" >&2; }
    return 1
  fi
  awk -F '\t' -v paths="$paths" '
    BEGIN { while ((getline line < paths) > 0) { count++; path[count] = line } }
    $0 == "END-OF-RUN" { next }
    NF == 3 { print path[$1 + 0] "\t" $2 "\t" $3 }
  ' "$coverage" | sort -u > "$expanded"
  chmod 0600 "$expanded"
  printf 'COVERAGE PASS %s paths\n' "$total"
}

anchor_lookup_path() {
  local display_path=$1
  if [[ "$display_path" =~ ^(.+):([0-9]+(-[0-9]+)?)$ ]]; then
    printf '%s\n' "${BASH_REMATCH[1]}"
  else
    printf '%s\n' "$display_path"
  fi
}

MAP_REASON=''
validate_map() {
  local repo=$1 map_file=$2
  local first title question_line answer_line trail_line anchors_line provenance_line
  local anchor_count=0 line display_path stored_type stored_hash
  local lookup_path current_hash current_type

  MAP_REASON=''
  IFS= read -r first < "$map_file" || { MAP_REASON='empty file'; return 1; }
  [[ "$first" =~ ^#[[:space:]]+[^#[:space:]].+$ ]] || { MAP_REASON='missing clean H1'; return 1; }
  title=${first#\# }
  [[ "$title" =~ ^(C:|L:|M:|MAP[-_:[:space:]]) ]] && { MAP_REASON='legacy title prefix'; return 1; }

  [ "$(grep -c '^## Question$' "$map_file" || true)" -eq 1 ] || { MAP_REASON='Question heading count is not one'; return 1; }
  [ "$(grep -c '^## Answer$' "$map_file" || true)" -eq 1 ] || { MAP_REASON='Answer heading count is not one'; return 1; }
  [ "$(grep -c '^## Derivation trail$' "$map_file" || true)" -eq 1 ] || { MAP_REASON='Derivation trail heading count is not one'; return 1; }
  [ "$(grep -c '^## Anchors$' "$map_file" || true)" -eq 1 ] || { MAP_REASON='Anchors heading count is not one'; return 1; }
  if grep '^## ' "$map_file" | grep -qvE '^## (Question|Answer|Derivation trail|Anchors)$'; then
    MAP_REASON='unexpected section heading'
    return 1
  fi
  [ "$(grep -c '^Provenance:' "$map_file" || true)" -eq 1 ] || { MAP_REASON='Provenance line count is not one'; return 1; }
  grep -Eq '^Provenance: [0-9]{4}-[0-9]{2}-[0-9]{2} · sid [0-9a-f]{8}$' "$map_file" || { MAP_REASON='Provenance grammar mismatch'; return 1; }

  question_line=$(grep -n '^## Question$' "$map_file" | cut -d: -f1)
  answer_line=$(grep -n '^## Answer$' "$map_file" | cut -d: -f1)
  trail_line=$(grep -n '^## Derivation trail$' "$map_file" | cut -d: -f1)
  anchors_line=$(grep -n '^## Anchors$' "$map_file" | cut -d: -f1)
  provenance_line=$(grep -n '^Provenance:' "$map_file" | cut -d: -f1)
  (( question_line < answer_line && answer_line < trail_line && trail_line < provenance_line && provenance_line < anchors_line )) || {
    MAP_REASON='section order mismatch'
    return 1
  }
  if sed -n "2,$((question_line - 1))p" "$map_file" | grep -q '[^[:space:]]'; then
    MAP_REASON='legacy preamble before Question'
    return 1
  fi
  awk -v q="$question_line" -v a="$answer_line" -v d="$trail_line" -v p="$provenance_line" '
    NR > q && NR < a && /[^[:space:]]/ { qbody=1 }
    NR > a && NR < d && /[^[:space:]]/ { abody=1 }
    NR > d && NR < p && /[^[:space:]]/ { dbody=1 }
    END { exit !(qbody && abody && dbody) }
  ' "$map_file" || { MAP_REASON='Question, Answer, or Derivation trail is empty'; return 1; }

  while IFS= read -r line; do
    [ -z "$line" ] && continue
    [[ "$line" =~ $ANCHOR_RE ]] || { MAP_REASON='anchor row grammar mismatch'; return 1; }
    display_path=${BASH_REMATCH[1]}
    stored_type=${BASH_REMATCH[2]}
    stored_hash=${BASH_REMATCH[3]}
    [[ "$display_path" != /* && "$display_path" != ../* && "$display_path" != *'/../'* && "$display_path" != .git/* ]] || {
      MAP_REASON="unsafe anchor path: $display_path"
      return 1
    }
    lookup_path=$(anchor_lookup_path "$display_path")
    current_hash=$(git -C "$repo" rev-parse --verify -q "HEAD:$lookup_path" 2>/dev/null) || {
      MAP_REASON="anchor path absent at HEAD: $lookup_path"
      return 1
    }
    [[ "$current_hash" == "$stored_hash"* ]] || {
      MAP_REASON="anchor hash mismatch: $lookup_path expected=$stored_hash actual=$current_hash"
      return 1
    }
    current_type=$(git -C "$repo" cat-file -t "$current_hash" 2>/dev/null) || {
      MAP_REASON="anchor object unreadable: $lookup_path"
      return 1
    }
    [ "$current_type" = "$stored_type" ] || {
      MAP_REASON="anchor object type mismatch: $lookup_path expected=$stored_type actual=$current_type"
      return 1
    }
    anchor_count=$((anchor_count + 1))
  done < <(tail -n "+$((anchors_line + 1))" "$map_file")
  (( anchor_count >= 2 && anchor_count <= 8 )) || { MAP_REASON="anchor count outside 2-8: $anchor_count"; return 1; }
  return 0
}

gate_anchors() {
  local repo=$1 maps_dir=$2 results=$3 survivors=$4
  local map_file relative
  require_dir "$repo"
  require_dir "$maps_dir"
  git -C "$repo" rev-parse --verify HEAD >/dev/null 2>&1 || die "repository has no HEAD: $repo"
  : > "$results"
  : > "$survivors"
  shopt -s nullglob
  for map_file in "$maps_dir"/*.md; do
    relative="maps/$(basename "$map_file")"
    if [[ ! "$(basename "$map_file")" =~ ^[a-z0-9][a-z0-9-]*\.md$ || "$(basename "$map_file")" == *--*.md ]]; then
      printf 'REJECT\t%s\tinvalid map filename\n' "$relative" >> "$results"
      continue
    fi
    if validate_map "$repo" "$map_file"; then
      printf 'ACCEPT\t%s\tcanonical map and live anchors\n' "$relative" >> "$results"
      printf '%s\n' "$relative" >> "$survivors"
    else
      printf 'REJECT\t%s\t%s\n' "$relative" "$MAP_REASON" >> "$results"
    fi
  done
  shopt -u nullglob
  sort -u -o "$survivors" "$survivors"
  printf 'ANCHORS PASS accepted=%s rejected=%s\n' \
    "$(grep -c '^ACCEPT' "$results" || true)" "$(grep -c '^REJECT' "$results" || true)"
}

gate_verdicts() {
  local survivors=$1 verdicts=$2 normalized=$3
  local scratch expected parsed duplicates unknown malformed map_path verdict evidence line extra
  require_file "$survivors"
  require_file "$verdicts"
  scratch="${normalized}.work"
  expected="${scratch}.expected"
  parsed="${scratch}.parsed"
  duplicates="${scratch}.duplicates"
  unknown="${scratch}.unknown"
  malformed="${scratch}.malformed"
  sort -u "$survivors" > "$expected"
  : > "$parsed"
  : > "$malformed"

  while IFS= read -r line || [ -n "$line" ]; do
    [ -z "$line" ] && continue
    IFS=$'\t' read -r verdict map_path evidence extra <<< "$line"
    if [ -n "${extra:-}" ] || [[ "$verdict" != CONFIRM && "$verdict" != AMEND && "$verdict" != REFUTE ]] || \
      [[ ! "$map_path" =~ ^maps/[a-z0-9][a-z0-9-]*\.md$ ]] || [ -z "${evidence:-}" ]; then
      printf '%s\n' "$line" >> "$malformed"
      continue
    fi
    printf '%s\t%s\t%s\n' "$verdict" "$map_path" "$evidence" >> "$parsed"
  done < "$verdicts"

  cut -f2 "$parsed" | sort | uniq -d > "$duplicates"
  cut -f2 "$parsed" | sort -u | comm -23 - "$expected" > "$unknown"
  if [ -s "$malformed" ] || [ -s "$duplicates" ] || [ -s "$unknown" ]; then
    printf 'VERDICTS FAIL %s\n' "$verdicts" >&2
    [ ! -s "$malformed" ] || { printf '%s\n' 'MALFORMED:' >&2; sed -n '1,160p' "$malformed" >&2; }
    [ ! -s "$duplicates" ] || { printf '%s\n' 'DUPLICATE MAPS:' >&2; sed -n '1,160p' "$duplicates" >&2; }
    [ ! -s "$unknown" ] || { printf '%s\n' 'UNKNOWN MAPS:' >&2; sed -n '1,160p' "$unknown" >&2; }
    return 1
  fi

  : > "$normalized"
  while IFS= read -r map_path; do
    line=$(awk -F '\t' -v target="$map_path" '$2 == target { print; exit }' "$parsed")
    if [ -n "$line" ]; then
      printf '%s\n' "$line" >> "$normalized"
    else
      printf 'UNRULED\t%s\tno verifier verdict; not applied\n' "$map_path" >> "$normalized"
    fi
  done < "$expected"
  printf 'VERDICTS PASS ruled=%s unruled=%s\n' \
    "$(grep -cEv '^UNRULED' "$normalized" || true)" "$(grep -c '^UNRULED' "$normalized" || true)"
}

build_distill_brief() {
  local stage=$1 today=$2 repo_head=$3
  {
    cat "$DISTILL_TEMPLATE"
    printf '\n## Lane\n\n'
    cat "$LANE_PROFILE"
    printf '\n## Run context\n\n'
    printf 'Agent type: `%s` (lane `%s`)\n' "$AGENT_TYPE" "$LANE"
    printf 'Repository root: `%s`\n' "$REPO_ROOT"
    printf 'Repository tree: `%s`\n' "$repo_head"
    printf 'Staging root: `%s`\n' "$stage"
    printf 'Map output directory: `%s/maps`\n' "$stage"
    printf 'Coverage output: `%s/coverage.md`\n' "$stage"
    printf 'Run date: `%s`\n' "$today"
    printf '\n### Cached map titles\n\n'
    if [ -s "$stage/cached-titles.txt" ]; then
      sed 's/^/- /' "$stage/cached-titles.txt"
    else
      printf '(none)\n'
    fi
    if [ -n "$CORPUS_FILE" ] && grep -q '^#' "$CORPUS_FILE"; then
      printf '\n### Corpus provenance\n\n'
      sed -n 's/^#[[:space:]]\?//p' "$CORPUS_FILE"
    fi
    printf '\n### Transcript paths (coverage indices)\n\n'
    if [ -s "$stage/paths.txt" ]; then
      awk '{ printf "%s. %s\n", NR, $0 }' "$stage/paths.txt"
    else
      printf '(none)\n'
    fi
    printf '\nWrite only `%s/maps/*.md` and `%s/coverage.md`; finish coverage with `END-OF-RUN`.\n' "$stage" "$stage"
  } > "$stage/distill-brief.md"
}

build_verify_brief() {
  local stage=$1 repo_head=$2
  {
    cat "$REFINER_TEMPLATE"
    printf '\n## Lane\n\n'
    cat "$LANE_PROFILE"
    printf '\n## Run context\n\n'
    printf 'Agent type: `%s` (lane `%s`)\n' "$AGENT_TYPE" "$LANE"
    printf 'Repository root: `%s`\n' "$REPO_ROOT"
    printf 'Repository tree: `%s`\n' "$repo_head"
    printf 'Staging root: `%s`\n' "$stage"
    printf 'Verdict output: `%s/verdicts.md`\n' "$stage"
    printf '\n### Existing map titles\n\n'
    if [ -s "$stage/cached-titles.txt" ]; then
      sed 's/^/- /' "$stage/cached-titles.txt"
    else
      printf '(none)\n'
    fi
    printf '\n### Staged maps to rule\n\n'
    if [ -s "$stage/anchor-survivors.txt" ]; then
      sed "s#^#- $stage/#" "$stage/anchor-survivors.txt"
    else
      printf '(none)\n'
    fi
    printf '\nWrite only `%s/verdicts.md` and AMEND edits to the listed staged maps. Rule every listed map or leave it mechanically UNRULED.\n' "$stage"
  } > "$stage/refiner-brief.md"
}

run_seat() {
  local stage=$1 model=$2 effort=$3 brief=$4 log=$5 last_message=$6
  timeout --signal=TERM --kill-after=30s "$SEAT_TIMEOUT_SECONDS" \
    codex exec \
      --ignore-user-config \
      --ephemeral \
      --skip-git-repo-check \
      --sandbox workspace-write \
      --cd "$stage" \
      --model "$model" \
      --config "model_reasoning_effort=\"$effort\"" \
      --output-last-message "$last_message" \
      - < "$brief" > "$log" 2>&1
}

# stm.md indexes the WHOLE pool for the operator; each agents/{lane}.md carries only the
# lane's own maps, so an agent type is fed its own memory and no one else's.
render_surfaces() {
  local maps_dir=$1 old_stm=$2 lanes_file=$3 out_stm=$4 out_agents=$5
  local rows="$out_stm.rows" map_file first title slug lane
  local lanes_present=()
  mkdir -p "$out_agents"
  chmod 0700 "$out_agents"
  : > "$rows"
  shopt -s nullglob
  for map_file in "$maps_dir"/*.md; do
    IFS= read -r first < "$map_file" || die "cannot read map title: $map_file"
    [[ "$first" == '# '* ]] || die "map lacks H1 during surface render: $map_file"
    title=${first#\# }
    slug=$(basename "$map_file")
    [[ "$title" != *$'\t'* && "$title" != *' -> maps/'* ]] || die "unsafe map title during surface render: $map_file"
    lane=$(lane_of_map "$lanes_file" "$slug")
    printf '%s\t%s\t- %s -> maps/%s\n' "$lane" "$title" "$title" "$slug" >> "$rows"
  done
  shopt -u nullglob
  if [ -n "$(cut -f2 "$rows" | sort | uniq -d)" ]; then
    die 'duplicate map titles prevent deterministic surface generation'
  fi
  mapfile -t lanes_present < <(cut -f1 "$rows" | sort -u)
  for lane in "${lanes_present[@]}"; do
    awk -F '\t' -v lane="$lane" '$1 == lane { print }' "$rows" | sort -t $'\t' -k2,2 | cut -f3- > "$out_agents/$lane.md"
    chmod 0600 "$out_agents/$lane.md"
  done
  {
    printf '%s\n' '# Index of maps/ — stale content: edit the map file directly.'
    sort -t $'\t' -k2,2 "$rows" | cut -f3-
    if [ -f "$old_stm" ]; then
      awk '/^- / && $0 !~ / -> maps\/[a-z0-9][a-z0-9-]*\.md$/ { print }' "$old_stm"
    fi
  } > "$out_stm"
  rm -f "$rows"
  chmod 0600 "$out_stm"
}

surface_byte_stability_test() {
  local organ=$1 test_stage first second lanes_file=''
  require_dir "$organ/maps"
  require_file "$organ/stm.md"
  [ ! -f "$organ/lanes.tsv" ] || lanes_file="$organ/lanes.tsv"
  test_stage=$(mktemp -d /tmp/dreamer-night-surface-test.XXXXXX)
  chmod 0700 "$test_stage"
  mkdir -m 0700 "$test_stage/one" "$test_stage/two"
  first="$test_stage/one"
  second="$test_stage/two"
  render_surfaces "$organ/maps" "$organ/stm.md" "$lanes_file" "$first/stm.md" "$first/agents"
  render_surfaces "$organ/maps" "$organ/stm.md" "$lanes_file" "$second/stm.md" "$second/agents"
  cmp "$first/stm.md" "$second/stm.md"
  diff -r "$first/agents" "$second/agents" >/dev/null || die 'lane surfaces are not byte-stable'
  printf 'SURFACES PASS byte-stable maps=%s lanes=%s artifacts=%s\n' \
    "$(find "$organ/maps" -maxdepth 1 -type f -name '*.md' | wc -l)" \
    "$(find "$first/agents" -maxdepth 1 -type f -name '*.md' | wc -l)" "$test_stage"
}

restamp_map() {
  local source=$1 destination=$2 today=$3
  local line display_path lookup_path current_hash current_type
  local provenance_seen=0
  : > "$destination"
  while IFS= read -r line || [ -n "$line" ]; do
    if [[ "$line" =~ $ANCHOR_RE ]] || [[ "$line" =~ $LEGACY_ANCHOR_RE ]]; then
      display_path=${BASH_REMATCH[1]}
      lookup_path=$(anchor_lookup_path "$display_path")
      current_hash=$(git -C "$REPO_ROOT" rev-parse --verify "HEAD:$lookup_path")
      current_type=$(git -C "$REPO_ROOT" cat-file -t "$current_hash")
      printf -- '- `%s` — %s `%s`\n' "$display_path" "$current_type" "${current_hash:0:12}" >> "$destination"
    elif [[ "$line" =~ ^Provenance:[[:space:]][0-9]{4}-[0-9]{2}-[0-9]{2}[[:space:]]·[[:space:]]sid[[:space:]]([0-9a-f]{8})$ ]]; then
      printf 'Provenance: %s · sid %s\n' "$today" "${BASH_REMATCH[1]}" >> "$destination"
      provenance_seen=$((provenance_seen + 1))
    else
      printf '%s\n' "$line" >> "$destination"
    fi
  done < "$source"
  [ "$provenance_seen" -eq 1 ] || die "restamp found $provenance_seen Provenance lines: $source"
  chmod 0600 "$destination"
}

# Legacy rows carried a commit sha and date beside the content hash. The
# translation is TEXTUAL on purpose: it keeps each stored hash byte-for-byte, so
# a map that was already drifted stays drifted and no pending review is erased.
migrate_anchors() {
  local organ=$1
  local map_file tmp line rows total=0 touched=0 translated=0
  require_dir "$organ/maps"
  shopt -s nullglob
  for map_file in "$organ"/maps/*.md; do
    total=$((total + 1))
    rows=0
    tmp="$map_file.dreamer-migrate"
    : > "$tmp"
    while IFS= read -r line || [ -n "$line" ]; do
      if [[ "$line" =~ $LEGACY_ANCHOR_RE ]]; then
        printf -- '- `%s` — %s `%s`\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[4]}" "${BASH_REMATCH[5]}" >> "$tmp"
        rows=$((rows + 1))
      else
        printf '%s\n' "$line" >> "$tmp"
      fi
    done < "$map_file"
    if [ "$rows" -eq 0 ]; then
      rm -f "$tmp"
      continue
    fi
    chmod 0600 "$tmp"
    mv "$tmp" "$map_file"
    touched=$((touched + 1))
    translated=$((translated + rows))
  done
  shopt -u nullglob
  if grep -rl 'git log -1' "$organ/maps" >/dev/null 2>&1; then
    die "legacy anchor rows survive migration in $organ/maps"
  fi
  printf 'MIGRATE PASS organ=%s maps=%s rewritten=%s rows=%s\n' "$organ" "$total" "$touched" "$translated"
}

archive_name() {
  local base=$1 today=$2 candidate counter=2
  candidate="$today-$base"
  while [ -e "$ORGAN/archive/$candidate" ]; do
    candidate="$today-$counter-$base"
    counter=$((counter + 1))
  done
  printf '%s\n' "$candidate"
}

sweep_name() {
  local today=$1
  local candidate="$today.md" counter=2
  while [ -e "$ORGAN/dreamer/$candidate" ]; do
    candidate="$today-$counter.md"
    counter=$((counter + 1))
  done
  printf '%s\n' "$candidate"
}

prepare_apply() {
  local stage=$1
  local today repo_head recorded_head recorded_maps current_maps prep candidate map_path verdict evidence target
  local archive_target sweep_target applied_at surface explorer_archive='NONE'

  stage_path "$stage"
  require_file "$stage/READY-FOR-APPLY"
  require_file "$stage/meta/repo-head.txt"
  require_file "$stage/meta/maps.sha256"
  [ "$(sed -n '1p' "$stage/meta/repo-root.txt")" = "$REPO_ROOT" ] || die 'staged repository root mismatch'
  [ "$(sed -n '1p' "$stage/meta/organ.txt")" = "$ORGAN" ] || die 'staged organ mismatch'
  require_file "$stage/meta/lane.txt"
  [ "$(sed -n '1p' "$stage/meta/lane.txt")" = "$LANE" ] || \
    die "staged lane mismatch: stage=$(sed -n '1p' "$stage/meta/lane.txt") invocation=$LANE"
  today=$(sed -n '1p' "$stage/meta/run-date.txt")
  [[ "$today" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || die 'staged run date is invalid'
  gate_pin "$stage/paths.txt" "$stage/paths.sha256" >/dev/null
  gate_coverage "$stage/paths.txt" "$stage/coverage.md" "$stage/paths.sha256" >/dev/null
  gate_anchors "$REPO_ROOT" "$stage/maps" "$stage/anchor-postrefine.tsv" "$stage/anchor-postrefine-survivors.txt" >/dev/null
  gate_verdicts "$stage/anchor-survivors.txt" "$stage/verdicts.md" "$stage/verdicts-normalized.tsv" >/dev/null

  repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD)
  recorded_head=$(sed -n '1p' "$stage/meta/repo-head.txt")
  [ "$repo_head" = "$recorded_head" ] || die "repository HEAD moved since verification: $recorded_head -> $repo_head"
  recorded_maps=$(sed -n '1p' "$stage/meta/maps.sha256")
  current_maps=$(map_fingerprint "$ORGAN/maps")
  [ "$current_maps" = "$recorded_maps" ] || die 'organ maps changed since preflight; rerun the night'

  prep="$stage/apply"
  [ ! -e "$prep" ] || die "apply preparation already exists: $prep"
  mkdir -m 0700 "$prep" "$prep/maps" "$prep/refuted" "$prep/surfaces" "$prep/surfaces-second"
  cp -a "$ORGAN/maps/." "$prep/maps/"
  : > "$prep/apply-candidates.tsv"
  : > "$prep/archive-plan.tsv"
  : > "$prep/ops.tsv"

  while IFS=$'\t' read -r verdict map_path evidence; do
    case "$verdict" in
      CONFIRM|AMEND)
        if ! grep -qxF "$map_path" "$stage/anchor-postrefine-survivors.txt"; then
          printf 'NOT-APPLIED\t%s\tpost-refine anchor rejection\n' "$map_path" >> "$prep/ops.tsv"
          continue
        fi
        target="$ORGAN/$map_path"
        [ ! -e "$target" ] || die "map target collision: $target"
        candidate="$prep/maps/$(basename "$map_path")"
        restamp_map "$stage/$map_path" "$candidate" "$today"
        printf '%s\t%s\t%s\n' "$verdict" "$map_path" "$evidence" >> "$prep/apply-candidates.tsv"
        printf 'APPLY-%s\t%s\t%s\n' "$verdict" "$map_path" "$evidence" >> "$prep/ops.tsv"
        ;;
      REFUTE)
        archive_target=$(archive_name "$(basename "$map_path")" "$today")
        cp "$stage/$map_path" "$prep/refuted/$archive_target"
        printf '\nVerdict: REFUTE — %s\n' "$evidence" >> "$prep/refuted/$archive_target"
        chmod 0600 "$prep/refuted/$archive_target"
        printf '%s\t%s\n' "$archive_target" "$map_path" >> "$prep/archive-plan.tsv"
        printf 'ARCHIVE-REFUTE\t%s\tarchive/%s\n' "$map_path" "$archive_target" >> "$prep/ops.tsv"
        ;;
      UNRULED)
        printf 'NOT-APPLIED\t%s\tunruled\n' "$map_path" >> "$prep/ops.tsv"
        ;;
    esac
  done < "$stage/verdicts-normalized.tsv"

  write_lane_membership "$prep/maps" "$ORGAN/lanes.tsv" "$LANE" "$prep/lanes.tsv"
  render_surfaces "$prep/maps" "$ORGAN/stm.md" "$prep/lanes.tsv" "$prep/surfaces/stm.md" "$prep/surfaces/agents"
  render_surfaces "$prep/maps" "$ORGAN/stm.md" "$prep/lanes.tsv" "$prep/surfaces-second/stm.md" "$prep/surfaces-second/agents"
  cmp "$prep/surfaces/stm.md" "$prep/surfaces-second/stm.md"
  diff -r "$prep/surfaces/agents" "$prep/surfaces-second/agents" >/dev/null || die 'lane surfaces are not byte-stable'
  shopt -s nullglob
  for surface in "$prep"/surfaces/agents/*.md; do
    printf 'SURFACE\tagents/%s\t%s map rows\n' "$(basename "$surface")" "$(wc -l < "$surface")" >> "$prep/ops.tsv"
  done
  shopt -u nullglob

  if [ -f "$ORGAN/explorer-index.md" ]; then
    explorer_archive=$(archive_name explorer-index.md "$today")
    printf 'MIGRATE-SURFACE\texplorer-index.md\tarchive/%s\n' "$explorer_archive" >> "$prep/ops.tsv"
  fi
  sweep_target=$(sweep_name "$today")
  applied_at=$(date -Is)
  {
    printf '# Dreamer sweep — %s\n\n' "$today"
    printf '## Coverage\n\n'
    cat "$stage/meta/window.tsv"
    printf 'paths-sha256\t%s\n' "$(sed -n '1p' "$stage/paths.sha256")"
    cat "$stage/census.tsv"
    printf '\n### Paths\n\n```text\n'
    cat "$stage/paths.txt"
    printf '```\n\n### Typed gaps\n\n```text\n'
    cat "$stage/gaps.tsv"
    printf '```\n\n### Coverage\n\n```text\n'
    cat "$stage/coverage.md.expanded"
    printf '```\n\n## Gate results\n\n### Distill anchor gate\n\n```text\n'
    cat "$stage/anchor-results.tsv"
    printf '```\n\n### Post-verify anchor gate\n\n```text\n'
    cat "$stage/anchor-postrefine.tsv"
    printf '```\n\n### Lane membership\n\n```text\n'
    cat "$prep/lanes.tsv"
    printf '```\n\n## Verdicts\n\n```text\n'
    cat "$stage/verdicts-normalized.tsv"
    printf '```\n\n## Ops\n\n```text\n'
    cat "$prep/ops.tsv"
    printf '```\n\nEND-OF-SWEEP\nApplied: %s\n' "$applied_at"
  } > "$prep/sweep.md"
  chmod 0600 "$prep/sweep.md"
  printf '%s\n' "$explorer_archive" > "$prep/explorer-archive.txt"
  printf '%s\n' "$sweep_target" > "$prep/sweep-target.txt"
}

apply_stage() {
  local stage=$1 prep today explorer_archive sweep_target map_file refuted_file tmp_surface surface
  prepare_apply "$stage"
  prep="$stage/apply"
  today=$(sed -n '1p' "$stage/meta/run-date.txt")
  explorer_archive=$(sed -n '1p' "$prep/explorer-archive.txt")
  sweep_target=$(sed -n '1p' "$prep/sweep-target.txt")

  mkdir -p "$ORGAN/agents" "$ORGAN/archive" "$ORGAN/dreamer" "$ORGAN/maps"
  shopt -s nullglob
  for map_file in "$prep"/maps/*.md; do
    if [ ! -e "$ORGAN/maps/$(basename "$map_file")" ]; then
      mv "$map_file" "$ORGAN/maps/"
    fi
  done
  for refuted_file in "$prep"/refuted/*.md; do
    mv "$refuted_file" "$ORGAN/archive/"
  done
  shopt -u nullglob
  if [ "$explorer_archive" != NONE ] && [ -f "$ORGAN/explorer-index.md" ]; then
    mv "$ORGAN/explorer-index.md" "$ORGAN/archive/$explorer_archive"
  fi

  shopt -s nullglob
  for surface in "$prep"/surfaces/agents/*.md; do
    tmp_surface="$ORGAN/agents/.$(basename "$surface").dreamer-night"
    cp "$surface" "$tmp_surface"
    chmod 0600 "$tmp_surface"
    mv "$tmp_surface" "$ORGAN/agents/$(basename "$surface")"
  done
  shopt -u nullglob
  tmp_surface="$ORGAN/.lanes.tsv.dreamer-night"
  cp "$prep/lanes.tsv" "$tmp_surface"
  chmod 0600 "$tmp_surface"
  mv "$tmp_surface" "$ORGAN/lanes.tsv"
  tmp_surface="$ORGAN/.stm.md.dreamer-night"
  cp "$prep/surfaces/stm.md" "$tmp_surface"
  chmod 0600 "$tmp_surface"
  mv "$tmp_surface" "$ORGAN/stm.md"
  mv "$prep/sweep.md" "$ORGAN/dreamer/$sweep_target"

  # No commit here. The organ is tracked by its repository, so the sweep leaves the
  # working tree dirty on purpose — visible in `git status`, reviewable as a diff, and
  # committed through the repository's normal flow.
  printf 'APPLIED\t%s\n' "$(date -Is)" > "$stage/APPLIED"
  printf 'dreamer-night: APPLIED stage=%s sweep=%s — organ files written, uncommitted by design\n' \
    "$stage" "$sweep_target"
}

run_night() {
  local mode=$1 stage today repo_head paths_count cutoff distill_rc refiner_rc survivor_count ready_state
  require_seat_law
  require_commands
  require_repo_context
  require_file "$DISTILL_TEMPLATE"
  require_file "$REFINER_TEMPLATE"
  require_lane_profile
  take_runner_lock

  stage=$(new_stage)
  today=$(date +%F)
  repo_head=$(git -C "$REPO_ROOT" rev-parse HEAD)
  printf '%s\n' "$REPO_ROOT" > "$stage/meta/repo-root.txt"
  printf '%s\n' "$ORGAN" > "$stage/meta/organ.txt"
  printf '%s\n' "$AGENT_TYPE" > "$stage/meta/agent-type.txt"
  printf '%s\n' "$LANE" > "$stage/meta/lane.txt"
  printf '%s\n' "$repo_head" > "$stage/meta/repo-head.txt"
  printf '%s\n' "$today" > "$stage/meta/run-date.txt"
  map_fingerprint "$ORGAN/maps" > "$stage/meta/maps.sha256"
  write_cached_titles "$stage/cached-titles.txt"
  enumerate_corpus "$stage"
  paths_count=$(wc -l < "$stage/paths.txt")
  if [ "$paths_count" -eq 0 ]; then
    if [ -n "$CORPUS_FILE" ]; then
      cutoff="corpus-file $CORPUS_FILE"
    elif [ -n "$BOOTSTRAP_COUNT" ]; then
      cutoff="bootstrap-count $BOOTSTRAP_COUNT"
    else
      cutoff=$(awk -F '\t' '$1 == "cutoff-exclusive" { print $2; exit }' "$stage/meta/window.tsv")
    fi
    stage_path "$stage"
    rm -rf -- "$stage"
    printf 'dreamer-night: EMPTY-WINDOW stage=%s (no %s transcripts since %s)\n' "$stage" "$AGENT_TYPE" "$cutoff"
    return 0
  fi
  gate_pin "$stage/paths.txt" "$stage/paths.sha256" > "$stage/gate-pin.log"
  build_distill_brief "$stage" "$today" "$repo_head"
  printf 'dreamer-night: PREFLIGHT stage=%s paths=%s gaps=%s\n' \
    "$stage" "$paths_count" "$(wc -l < "$stage/gaps.tsv")"

  set +e
  run_seat "$stage" "$DISTILL_MODEL" "$DISTILL_EFFORT" "$stage/distill-brief.md" "$stage/distill-seat.log" "$stage/distill-last-message.txt"
  distill_rc=$?
  set -e
  if [ "$distill_rc" -ne 0 ]; then
    printf 'DISTILL-FAILED\t%s\n' "$distill_rc" > "$stage/FAILED"
    die "distill seat failed once; artifacts preserved at $stage"
  fi
  find "$stage" -type f -exec chmod 0600 {} +
  gate_pin "$stage/paths.txt" "$stage/paths.sha256" | tee "$stage/gate-pin-post-distill.log"
  gate_coverage "$stage/paths.txt" "$stage/coverage.md" "$stage/paths.sha256" | tee "$stage/gate-coverage.log"
  gate_anchors "$REPO_ROOT" "$stage/maps" "$stage/anchor-results.tsv" "$stage/anchor-survivors.txt" | tee "$stage/gate-anchors.log"
  build_verify_brief "$stage" "$repo_head"

  if [ -s "$stage/anchor-survivors.txt" ]; then
    set +e
    run_seat "$stage" "$REFINER_MODEL" "$REFINER_EFFORT" "$stage/refiner-brief.md" "$stage/refiner-seat.log" "$stage/verify-last-message.txt"
    refiner_rc=$?
    set -e
    if [ "$refiner_rc" -ne 0 ]; then
      printf 'VERIFY-FAILED\t%s\n' "$refiner_rc" > "$stage/FAILED"
      die "verify seat failed once; artifacts preserved at $stage"
    fi
  else
    : > "$stage/verdicts.md"
    printf 'VERIFY SKIP zero anchor-valid staged maps\n' > "$stage/refiner-seat.log"
  fi
  find "$stage" -type f -exec chmod 0600 {} +
  gate_pin "$stage/paths.txt" "$stage/paths.sha256" | tee "$stage/gate-pin-post-refine.log"
  gate_verdicts "$stage/anchor-survivors.txt" "$stage/verdicts.md" "$stage/verdicts-normalized.tsv" | tee "$stage/gate-verdicts.log"
  gate_anchors "$REPO_ROOT" "$stage/maps" "$stage/anchor-postrefine.tsv" "$stage/anchor-postrefine-survivors.txt" | tee "$stage/gate-anchors-postrefine.log"
  survivor_count=$(wc -l < "$stage/anchor-postrefine-survivors.txt")
  # Survivors are not yield: a night whose every survivor is REFUTEd or UNRULED
  # applies no map at all, and that must never read as a green night.
  yield_count=$(awk -F '\t' '
    NR == FNR { survivor[$0] = 1; next }
    ($1 == "CONFIRM" || $1 == "AMEND") && survivor[$2] { count++ }
    END { print count + 0 }
  ' "$stage/anchor-postrefine-survivors.txt" "$stage/verdicts-normalized.tsv")
  if [ "$survivor_count" -eq 0 ]; then
    ready_state=ZERO-SURVIVORS
  elif [ "$yield_count" -eq 0 ]; then
    ready_state=ZERO-YIELD
  else
    ready_state=READY
  fi
  printf '%s\n' "$yield_count" > "$stage/meta/apply-yield.txt"
  printf '%s\t%s\n' "$ready_state" "$(date -Is)" > "$stage/READY-FOR-APPLY"
  chmod 0600 "$stage/READY-FOR-APPLY"

  if [ "$mode" = supervise ]; then
    if [ "$survivor_count" -eq 0 ]; then
      printf 'dreamer-night: HOLD-BEFORE-APPLY ZERO-SURVIVORS stage=%s\n' "$stage"
      printf 'dreamer-night: no signed apply command: zero anchor-valid staged maps\n'
    else
      printf 'dreamer-night: HOLD-BEFORE-APPLY stage=%s\n' "$stage"
      printf 'dreamer-night: signed apply command: %s --repo %s --agent %s apply %s\n' \
        "$0" "$REPO_ROOT" "$AGENT_TYPE" "$stage"
    fi
  else
    apply_stage "$stage"
  fi
}

usage() {
  cat <<'EOF'
Usage:
  dreamer-night.sh [--repo ROOT] [--agent TYPE] [--bootstrap-count N | --corpus-file FILE] [supervise]
  dreamer-night.sh [--repo ROOT] [--agent TYPE] apply STAGE
  dreamer-night.sh [--repo ROOT] [--agent TYPE] inspect-repo

  --agent TYPE selects the subagent type to harvest (default Explore) and its
  lane; the lane needs a profile at lanes/{lane}.md and owns
  agents/{lane}.md plus its own sweep window.
  dreamer-night.sh gate-pin PATHS SHA256
  dreamer-night.sh gate-coverage PATHS COVERAGE [SHA256]
  dreamer-night.sh [--repo ROOT] gate-anchors MAPS RESULTS SURVIVORS
  dreamer-night.sh gate-anchors REPO MAPS RESULTS SURVIVORS
  dreamer-night.sh gate-verdicts SURVIVORS VERDICTS NORMALIZED
  dreamer-night.sh test-surfaces ORGAN
  dreamer-night.sh [--repo ROOT] [--agent TYPE] lane-membership MAPS EXISTING OUT
EOF
}

requested_repo=$DEFAULT_REPO_ROOT
requested_agent=Explore
while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo)
      [ "$#" -ge 2 ] || die '--repo requires one absolute root'
      requested_repo=$2
      shift 2
      ;;
    --agent)
      [ "$#" -ge 2 ] || die '--agent requires one subagent type'
      requested_agent=$2
      shift 2
      ;;
    --bootstrap-count)
      [ "$#" -ge 2 ] || die '--bootstrap-count requires one positive integer'
      [[ "$2" =~ ^[1-9][0-9]*$ ]] || die '--bootstrap-count requires one positive integer'
      BOOTSTRAP_COUNT=$2
      shift 2
      ;;
    --corpus-file)
      [ "$#" -ge 2 ] || die '--corpus-file requires one path'
      [[ "$2" == /* ]] || die '--corpus-file requires an absolute path'
      require_file "$2"
      CORPUS_FILE=$2
      shift 2
      ;;
    *)
      break
      ;;
  esac
done
configure_repo "$requested_repo"
configure_lane "$requested_agent"
command_name=${1:-run}
[ -z "$BOOTSTRAP_COUNT" ] || [ -z "$CORPUS_FILE" ] || die '--bootstrap-count and --corpus-file are mutually exclusive'
if [ -n "$BOOTSTRAP_COUNT$CORPUS_FILE" ] && [ "$command_name" != run ] && [ "$command_name" != supervise ]; then
  die '--bootstrap-count and --corpus-file are valid only for run or supervise'
fi
case "$command_name" in
  run)
    [ "$#" -le 1 ] || die 'run accepts only global flags'
    run_night autonomous
    ;;
  supervise)
    [ "$#" -eq 1 ] || die 'supervise accepts only global flags'
    run_night supervise
    ;;
  apply)
    [ "$#" -eq 2 ] || die 'apply requires one staging path'
    require_commands
    require_repo_context
    take_runner_lock
    apply_stage "$2"
    ;;
  inspect-repo)
    [ "$#" -eq 1 ] || die 'inspect-repo accepts only global flags'
    require_commands
    require_repo_context
    printf 'REPO\t%s\nORGAN\t%s\nREGISTRY\t%s\n' "$REPO_ROOT" "$ORGAN" "$REGISTRY"
    ;;
  gate-pin)
    [ "$#" -eq 3 ] || die 'gate-pin requires PATHS SHA256'
    gate_pin "$2" "$3"
    ;;
  gate-coverage)
    [ "$#" -eq 3 ] || [ "$#" -eq 4 ] || die 'gate-coverage requires PATHS COVERAGE [SHA256]'
    gate_coverage "$2" "$3" "${4:-}"
    ;;
  gate-anchors)
    if [ "$#" -eq 4 ]; then
      gate_anchors "$REPO_ROOT" "$2" "$3" "$4"
    elif [ "$#" -eq 5 ]; then
      gate_anchors "$2" "$3" "$4" "$5"
    else
      die 'gate-anchors requires MAPS RESULTS SURVIVORS (or legacy REPO MAPS RESULTS SURVIVORS)'
    fi
    ;;
  gate-verdicts)
    [ "$#" -eq 4 ] || die 'gate-verdicts requires SURVIVORS VERDICTS NORMALIZED'
    gate_verdicts "$2" "$3" "$4"
    ;;
  test-surfaces)
    [ "$#" -eq 2 ] || die 'test-surfaces requires ORGAN'
    surface_byte_stability_test "$2"
    ;;
  migrate-anchors)
    [ "$#" -eq 2 ] || die 'migrate-anchors requires ORGAN'
    migrate_anchors "$2"
    ;;
  lane-membership)
    [ "$#" -eq 4 ] || die 'lane-membership requires MAPS EXISTING OUT'
    require_repo_context
    write_lane_membership "$2" "$3" "$LANE" "$4"
    printf 'LANES PASS %s rows=%s\n' "$4" "$(wc -l < "$4")"
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage >&2
    die "unknown command: $command_name"
    ;;
esac
