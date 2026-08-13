#!/usr/bin/env bash
set -euo pipefail

export LC_ALL=C
umask 077

ENGINE_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
RUNNER="$ENGINE_DIR/dreamer-night.sh"
FIXTURES="$ENGINE_DIR/tests/fixtures"
REPO=/home/user/work/proja
HOOK="$ENGINE_DIR/dreamer-agent-inject.sh"
NUDGE="$ENGINE_DIR/dreamer-nudge.sh"
MORNING="$ENGINE_DIR/dreamer-morning.sh"
CODEX_HOOK="$ENGINE_DIR/dreamer-codex-subagent-inject.sh"
INTUITA=/home/user/work/projc
test_root=$(mktemp -d /tmp/dreamer-night-tests.XXXXXX)
chmod 0700 "$test_root"

fail() {
  printf 'FAIL: %s (artifacts: %s)\n' "$*" "$test_root" >&2
  exit 1
}

cp "$FIXTURES/coverage-paths.txt" "$test_root/coverage-paths.txt"
cp "$FIXTURES/coverage-typo.md" "$test_root/coverage-typo.md"
if "$RUNNER" gate-coverage "$test_root/coverage-paths.txt" "$test_root/coverage-typo.md" > "$test_root/coverage.out" 2>&1; then
  fail 'coverage typo fixture passed'
fi
grep -q '^DUPLICATE INDEXES:' "$test_root/coverage.out" || fail 'coverage failure did not name the duplicated index'
grep -q '^UNRULED INDEXES:' "$test_root/coverage.out" || fail 'coverage failure did not name the unruled index'
if grep -q '^COVERAGE PASS' "$test_root/coverage.out"; then fail 'a duplicated index passed coverage'; fi
printf 'PASS coverage mismatch fails closed\n'
sed '2s/^1\t/2\t/' "$test_root/coverage-typo.md" > "$test_root/coverage-valid.md"
pin=$(sha256sum "$test_root/coverage-paths.txt" | cut -d ' ' -f 1)
printf '%s\n' "$pin" > "$test_root/coverage-paths.sha256"
"$RUNNER" gate-pin "$test_root/coverage-paths.txt" "$test_root/coverage-paths.sha256" > "$test_root/pin.out"
"$RUNNER" gate-coverage "$test_root/coverage-paths.txt" "$test_root/coverage-valid.md" "$test_root/coverage-paths.sha256" > "$test_root/coverage-valid.out"
grep -q '^COVERAGE PASS 2 paths' "$test_root/coverage-valid.out" || fail 'valid coverage did not pass'
grep -qx $'/corpus/agent-one.jsonl\tREAD\tcondensed and checked' "$test_root/coverage-valid.md.expanded" || \
  fail 'the engine did not expand index 1 back to its path'
grep -qx $'/corpus/agent-two.jsonl\tSKIP\tno durable investigation' "$test_root/coverage-valid.md.expanded" || \
  fail 'the engine did not expand index 2 back to its path'
if grep -q '/corpus/' "$test_root/coverage-valid.md"; then fail 'the seat contract still lets a path be retyped in coverage'; fi
printf 'PASS coverage is keyed by index and the engine expands it back to paths\n'

mkdir -m 0700 "$test_root/maps"
path_one=.claude/scripts/build-codex.mjs
path_two=.claude/scripts/check-professor-process-contract.sh
read -r commit_one_full date_one < <(git -C "$REPO" log -1 --format='%H %as' -- "$path_one")
read -r commit_two_full date_two < <(git -C "$REPO" log -1 --format='%H %as' -- "$path_two")
hash_one_full=$(git -C "$REPO" rev-parse "HEAD:$path_one")
hash_two_full=$(git -C "$REPO" rev-parse "HEAD:$path_two")
type_one=$(git -C "$REPO" cat-file -t "$hash_one_full")
type_two=$(git -C "$REPO" cat-file -t "$hash_two_full")
commit_one=${commit_one_full:0:12}
commit_two=${commit_two_full:0:12}
hash_one=${hash_one_full:0:12}
hash_two=${hash_two_full:0:12}
case "${hash_one:0:1}" in
  0) bad_hash_one="1${hash_one:1}" ;;
  *) bad_hash_one="0${hash_one:1}" ;;
esac
sed \
  -e "s/{{TODAY}}/$(date +%F)/g" \
  -e "s#{{PATH_ONE}}#$path_one#g" \
  -e "s/{{COMMIT_ONE}}/$commit_one/g" \
  -e "s/{{DATE_ONE}}/$date_one/g" \
  -e "s/{{TYPE_ONE}}/$type_one/g" \
  -e "s/{{BAD_HASH_ONE}}/$bad_hash_one/g" \
  -e "s#{{PATH_TWO}}#$path_two#g" \
  -e "s/{{COMMIT_TWO}}/$commit_two/g" \
  -e "s/{{DATE_TWO}}/$date_two/g" \
  -e "s/{{TYPE_TWO}}/$type_two/g" \
  -e "s/{{HASH_TWO}}/$hash_two/g" \
  "$FIXTURES/anchor-flipped-hash.md.in" > "$test_root/maps/flipped-hash.md"
"$RUNNER" gate-anchors "$REPO" "$test_root/maps" "$test_root/anchor-results.tsv" "$test_root/anchor-survivors.txt" > "$test_root/anchor.out"
grep -q $'^REJECT\tmaps/flipped-hash.md\tanchor hash mismatch:' "$test_root/anchor-results.tsv" || fail 'flipped hash lacked named rejection'
[ ! -s "$test_root/anchor-survivors.txt" ] || fail 'flipped-hash map survived'
printf 'PASS flipped anchor hash is rejected\n'
mkdir -m 0700 "$test_root/maps-valid"
sed "s/$bad_hash_one/$hash_one/" "$test_root/maps/flipped-hash.md" > "$test_root/maps-valid/valid-anchor.md"
"$RUNNER" gate-anchors "$REPO" "$test_root/maps-valid" "$test_root/anchor-valid-results.tsv" "$test_root/anchor-valid-survivors.txt" > "$test_root/anchor-valid.out"
grep -q $'^ACCEPT\tmaps/valid-anchor.md\t' "$test_root/anchor-valid-results.tsv" || fail 'valid anchor fixture did not pass'
printf 'PASS canonical live anchors pass\n'

mkdir -m 0700 "$test_root/maps-legacy"
sed \
  -e "s/{{TODAY}}/$(date +%F)/g" \
  -e "s#{{PATH_ONE}}#$path_one#g" \
  -e "s/{{COMMIT_ONE}}/$commit_one_full/g" \
  -e "s/{{DATE_ONE}}/$date_one/g" \
  -e "s/{{TYPE_ONE}}/$type_one/g" \
  -e "s/{{HASH_ONE}}/$hash_one_full/g" \
  -e "s#{{PATH_TWO}}#$path_two#g" \
  -e "s/{{COMMIT_TWO}}/$commit_two_full/g" \
  -e "s/{{DATE_TWO}}/$date_two/g" \
  -e "s/{{TYPE_TWO}}/$type_two/g" \
  -e "s/{{HASH_TWO}}/$hash_two_full/g" \
  "$FIXTURES/anchor-legacy-40.md.in" > "$test_root/maps-legacy/legacy-40.md"
"$RUNNER" gate-anchors "$REPO" "$test_root/maps-legacy" "$test_root/anchor-legacy-results.tsv" "$test_root/anchor-legacy-survivors.txt" > "$test_root/anchor-legacy.out"
grep -q $'^REJECT\tmaps/legacy-40.md\tanchor row grammar mismatch$' "$test_root/anchor-legacy-results.tsv" || fail 'legacy 40-character anchors did not reject'
printf 'PASS legacy 40-character anchors reject\n'

mkdir -m 0700 "$test_root/maps-legacy-commit"
sed \
  -e "s/{{TODAY}}/$(date +%F)/g" \
  -e "s#{{PATH_ONE}}#$path_one#g" \
  -e "s/{{COMMIT_ONE}}/$commit_one/g" \
  -e "s/{{DATE_ONE}}/$date_one/g" \
  -e "s/{{TYPE_ONE}}/$type_one/g" \
  -e "s/{{HASH_ONE}}/$hash_one/g" \
  -e "s#{{PATH_TWO}}#$path_two#g" \
  -e "s/{{COMMIT_TWO}}/$commit_two/g" \
  -e "s/{{DATE_TWO}}/$date_two/g" \
  -e "s/{{TYPE_TWO}}/$type_two/g" \
  -e "s/{{HASH_TWO}}/$hash_two/g" \
  "$FIXTURES/anchor-legacy-commit-row.md.in" > "$test_root/maps-legacy-commit/legacy-commit.md"
"$RUNNER" gate-anchors "$REPO" "$test_root/maps-legacy-commit" "$test_root/anchor-legacy-commit-results.tsv" "$test_root/anchor-legacy-commit-survivors.txt" > "$test_root/anchor-legacy-commit.out"
grep -q $'^REJECT\tmaps/legacy-commit.md\tanchor row grammar mismatch$' "$test_root/anchor-legacy-commit-results.tsv" || \
  fail 'the retired git log -1 anchor row did not reject'
if grep -rq 'git log -1' /home/user/work/proja/.professor/stm/maps /home/user/work/projc/.professor/stm/maps; then
  fail 'a live organ still carries the retired anchor row shape'
fi
grep -Fq 'never emit a commit sha' "$ENGINE_DIR/dreamer-distill.prompt.md" || fail 'distill prompt still allows a commit sha'
grep -Fq 'REVIEW TRIGGERS, not citations' "$ENGINE_DIR/dreamer-distill.prompt.md" || fail 'distill prompt omits the review-trigger law'
grep -Fq 'REVIEW TRIGGER' "$ENGINE_DIR/dreamer-refiner.prompt.md" || fail 'verify prompt omits the review-trigger law'
grep -Fq '2–8 anchor rows and no more' "$ENGINE_DIR/dreamer-refiner.prompt.md" || fail 'verify prompt omits the anchor-count law an AMEND must respect'
printf 'PASS the retired commit-row grammar rejects, no organ retains it, and both seats carry the trigger law\n'

mkdir -m 0700 "$test_root/maps-comma-range"
sed \
  -e "s/{{TODAY}}/$(date +%F)/g" \
  -e "s#{{PATH_ONE}}#$path_one#g" \
  -e "s/{{COMMIT_ONE}}/$commit_one/g" \
  -e "s/{{DATE_ONE}}/$date_one/g" \
  -e "s/{{TYPE_ONE}}/$type_one/g" \
  -e "s/{{HASH_ONE}}/$hash_one/g" \
  -e "s#{{PATH_TWO}}#$path_two#g" \
  -e "s/{{COMMIT_TWO}}/$commit_two/g" \
  -e "s/{{DATE_TWO}}/$date_two/g" \
  -e "s/{{TYPE_TWO}}/$type_two/g" \
  -e "s/{{HASH_TWO}}/$hash_two/g" \
  "$FIXTURES/anchor-comma-range.md.in" > "$test_root/maps-comma-range/comma-range.md"
"$RUNNER" gate-anchors "$REPO" "$test_root/maps-comma-range" "$test_root/anchor-comma-range-results.tsv" "$test_root/anchor-comma-range-survivors.txt" > "$test_root/anchor-comma-range.out"
grep -q $'^REJECT\tmaps/comma-range.md\tanchor path absent at HEAD: .*:1-2,4-5$' "$test_root/anchor-comma-range-results.tsv" || fail 'comma-separated anchor ranges did not reject'
[ ! -s "$test_root/anchor-comma-range-survivors.txt" ] || fail 'comma-range map survived'
grep -Fq 'multiple regions use separate anchor rows or the bare file path' "$ENGINE_DIR/dreamer-distill.prompt.md" || fail 'distill prompt omits the single-range contract'
printf 'PASS comma-separated anchor ranges reject and the distill prompt teaches the canonical form\n'

printf 'maps/valid-anchor.md\n' > "$test_root/verdict-survivors.txt"
printf 'CONFIRM\tmaps/valid-anchor.md\tclaims falsified at pinned HEAD\n' > "$test_root/verdicts.md"
"$RUNNER" gate-verdicts "$test_root/verdict-survivors.txt" "$test_root/verdicts.md" "$test_root/verdicts-normalized.tsv" > "$test_root/verdicts.out"
cmp "$test_root/verdicts.md" "$test_root/verdicts-normalized.tsv" || fail 'valid verdict normalization changed the verdict'
: > "$test_root/verdicts-empty.md"
"$RUNNER" gate-verdicts "$test_root/verdict-survivors.txt" "$test_root/verdicts-empty.md" "$test_root/verdicts-unruled.tsv" > "$test_root/verdicts-unruled.out"
grep -q $'^UNRULED\tmaps/valid-anchor.md\t' "$test_root/verdicts-unruled.tsv" || fail 'missing verdict did not become UNRULED'
printf 'PASS verdict gate rules valid lines and marks omissions UNRULED\n'

mkdir -m 0700 "$test_root/organ"
cp -a "$REPO/.professor/stm/maps" "$test_root/organ/maps"
cp "$REPO/.professor/stm/stm.md" "$test_root/organ/stm.md"
[ ! -f "$REPO/.professor/stm/lanes.tsv" ] || cp "$REPO/.professor/stm/lanes.tsv" "$test_root/organ/lanes.tsv"
"$RUNNER" test-surfaces "$test_root/organ" > "$test_root/surfaces.out"
grep -q '^SURFACES PASS byte-stable' "$test_root/surfaces.out" || fail 'surface byte-stability test failed'
printf 'PASS surfaces regenerate byte-stable\n'

empty_engine="$test_root/empty-engine"
fixture_repo="$test_root/fixture-repo"
fixture_organ="$fixture_repo/.professor/stm"
registry_base="$test_root/registries"
registry_key=${fixture_repo//\//-}
fixture_registry="$registry_base/$registry_key"
fake_bin="$test_root/fake-bin"
seat_marker="$test_root/seat-invoked"
mkdir -m 0700 "$empty_engine" "$fixture_repo" "$fixture_repo/.professor" "$fixture_organ" "$fixture_organ/maps" "$fixture_organ/dreamer" "$fixture_organ/archive" "$fixture_organ/lanes" "$registry_base" "$fixture_registry" "$fake_bin"
cp -al "$REPO/.git" "$fixture_repo/.git"
printf '# fixture STM\n' > "$fixture_organ/stm.md"
cp "$RUNNER" "$empty_engine/dreamer-night.sh"
cp "$ENGINE_DIR/dreamer-distill.prompt.md" "$ENGINE_DIR/dreamer-refiner.prompt.md" "$empty_engine/"
cp -a "$ENGINE_DIR/lanes" "$empty_engine/lanes"
printf 'GLOBAL QA PROFILE\n' > "$empty_engine/lanes/qa-projb-cortex.md"
printf 'ORGAN QA PROFILE\n' > "$fixture_organ/lanes/qa-projb-cortex.md"
sed -i "s#^REGISTRY_BASE=.*#REGISTRY_BASE=$registry_base#" "$empty_engine/dreamer-night.sh"
printf '#!/usr/bin/env bash\nprintf invoked > %q\nexit 97\n' "$seat_marker" > "$fake_bin/codex"
chmod 0700 "$fake_bin/codex" "$empty_engine/dreamer-night.sh"
cp "$FIXTURES/window-applied.md" "$fixture_organ/dreamer/2026-08-10.md"

proja_context=$("$RUNNER" inspect-repo)
projc_context=$("$RUNNER" --repo "$INTUITA" inspect-repo)
grep -q $'^REGISTRY\t/home/user/.claude/projects/-home-user-work-proja$' <<< "$proja_context" || fail 'Proja registry encoding is wrong'
grep -q $'^REGISTRY\t/home/user/.claude/projects/-home-user-work-projc$' <<< "$projc_context" || fail 'Projc registry encoding is wrong'
if "$RUNNER" --repo relative inspect-repo > "$test_root/relative-repo.out" 2>&1; then
  fail 'relative repository root passed'
fi
missing_organ_repo="$test_root/missing-organ-repo"
mkdir -m 0700 "$missing_organ_repo"
cp -al "$REPO/.git" "$missing_organ_repo/.git"
if "$empty_engine/dreamer-night.sh" --repo "$missing_organ_repo" inspect-repo > "$test_root/missing-organ.out" 2>&1; then
  fail 'missing organ skeleton passed'
fi
grep -q 'missing directory: .*/\.professor/stm' "$test_root/missing-organ.out" || fail 'missing organ failure was not named'
printf 'PASS repo parameterization resolves both registries and rejects wrong or missing organs\n'

run_empty_window() {
  local expected_cutoff=$1 output stage
  output=$(PATH="$fake_bin:$PATH" "$empty_engine/dreamer-night.sh" --repo "$fixture_repo") || fail 'empty window did not exit zero'
  [ "$(wc -l <<< "$output")" -eq 1 ] || fail 'empty window printed more than one line'
  grep -qF "(no Explore transcripts since $expected_cutoff)" <<< "$output" || fail "empty window used wrong cutoff: $output"
  stage=$(sed -n 's/^dreamer-night: EMPTY-WINDOW stage=\([^ ]*\).*/\1/p' <<< "$output")
  if [ -z "$stage" ] || [ -e "$stage" ]; then
    fail 'empty-window staging directory survived'
  fi
  [ ! -e "$seat_marker" ] || fail 'empty window invoked the distill seat'
}

run_empty_window '2026-08-10T12:34:56+02:00'
sed -i '/^enumerated-at\t/d' "$fixture_organ/dreamer/2026-08-10.md"
run_empty_window '2026-08-10T13:00:00+02:00'
sed -i '/^Applied: /d' "$fixture_organ/dreamer/2026-08-10.md"
run_empty_window '2026-08-10 00:00:00'
printf 'PASS cutoff precedence and empty window skip the seat, log, ledger, and staging retention\n'

make_meta() {
  local name=$1 agent_type=$2 mtime=$3 paired=$4
  local dir="$fixture_registry/$name"
  mkdir -m 0700 "$dir"
  if [ "$agent_type" = INVALID ]; then
    printf '{}\n' > "$dir/agent-$name.meta.json"
  else
    printf '{"agentType":"%s"}\n' "$agent_type" > "$dir/agent-$name.meta.json"
  fi
  if [ "$paired" = paired ]; then
    printf '{}\n' > "$dir/agent-$name.jsonl"
  fi
  touch -d "$mtime" "$dir/agent-$name.meta.json"
}

make_meta newest Explore '2026-08-12 08:00:00.900000000 +0200' paired
make_meta tie-a Explore '2026-08-12 07:00:00.500000000 +0200' paired
make_meta tie-b Explore '2026-08-12 07:00:00.500000000 +0200' paired
make_meta older Explore '2026-08-11 23:00:00.000000000 +0200' paired
make_meta missing Explore '2026-08-12 09:00:00.000000000 +0200' missing
make_meta worker general-purpose '2026-08-12 10:00:00.000000000 +0200' paired
make_meta invalid INVALID '2026-08-12 11:00:00.000000000 +0200' paired

# shellcheck disable=SC2016
printf '%s\n' '#!/usr/bin/env bash' 'set -euo pipefail' \
  'stage=""' \
  'while [ "$#" -gt 0 ]; do if [ "$1" = --cd ]; then stage=$2; shift 2; else shift; fi; done' \
  '[ -n "$stage" ]' \
  'index=0' \
  'while IFS= read -r path; do index=$((index + 1)); printf "%s\tSKIP\tfixture coverage\n" "$index"; done < "$stage/paths.txt" > "$stage/coverage.md"' \
  'printf "CONDUCT\ttechnique\tNONE\tfixture\nCONDUCT\tprior\tNONE\tfixture\nCONDUCT\tbaseline\tNONE\tfixture\nEND-OF-RUN\n" >> "$stage/coverage.md"' > "$fake_bin/codex"
chmod 0700 "$fake_bin/codex"
bootstrap_output=$(PATH="$fake_bin:$PATH" "$empty_engine/dreamer-night.sh" --repo "$fixture_repo" --bootstrap-count 3 supervise) || fail 'bootstrap-count fixture failed'
bootstrap_stage=$(sed -n 's/^dreamer-night: HOLD-BEFORE-APPLY ZERO-SURVIVORS stage=\([^ ]*\)$/\1/p' <<< "$bootstrap_output")
if [ -z "$bootstrap_stage" ] || [ ! -d "$bootstrap_stage" ]; then
  fail 'bootstrap stage was not preserved'
fi
[ ! -e "$fixture_organ/lanes/explorer.md" ] || fail 'the fixture unexpectedly carries an organ-local explorer profile'
grep -Fq 'Lane `explorer` — the `Explore` subagent type.' "$bootstrap_stage/distill-brief.md" || \
  fail 'the global-only explorer profile did not reach the runner brief'
printf 'PASS a lane profile that exists only globally still runs\n'
grep -q $'^ZERO-SURVIVORS\t' "$bootstrap_stage/READY-FOR-APPLY" || fail 'zero-survivor READY marker hid the state'
grep -q '^dreamer-night: no signed apply command: zero anchor-valid staged maps$' <<< "$bootstrap_output" || fail 'zero-survivor HOLD offered an apply command'
if grep -q '^dreamer-night: signed apply command:' <<< "$bootstrap_output"; then fail 'zero-survivor HOLD printed a signed apply command'; fi
grep -qx $'window-meta-count\t7' "$bootstrap_stage/census.tsv" || fail 'bootstrap census lost total metas'
grep -qx $'agent-meta-count\t5' "$bootstrap_stage/census.tsv" || fail 'bootstrap census lost Explore metas'
grep -qx $'paired-transcript-count\t4' "$bootstrap_stage/census.tsv" || fail 'bootstrap census lost paired truth'
grep -qx $'selected-paired-transcript-count\t3' "$bootstrap_stage/census.tsv" || fail 'bootstrap selection count is wrong'
grep -qx $'omitted-paired-transcript-count\t1' "$bootstrap_stage/census.tsv" || fail 'bootstrap omitted count is wrong'
grep -qx $'coverage-gap-count\t1' "$bootstrap_stage/census.tsv" || fail 'bootstrap census lost unpaired Explore meta'
grep -qx $'excluded-other-agent-or-invalid-count\t2' "$bootstrap_stage/census.tsv" || fail 'bootstrap excluded count is wrong'
grep -qx $'invalid-meta-count\t1' "$bootstrap_stage/census.tsv" || fail 'bootstrap invalid count is wrong'
grep -q '/newest/agent-newest.jsonl$' "$bootstrap_stage/paths.txt" || fail 'bootstrap omitted newest paired meta'
grep -q '/tie-a/agent-tie-a.jsonl$' "$bootstrap_stage/paths.txt" || fail 'bootstrap tie-break omitted tie-a'
grep -q '/tie-b/agent-tie-b.jsonl$' "$bootstrap_stage/paths.txt" || fail 'bootstrap tie-break omitted tie-b'
if grep -q '/older/agent-older.jsonl$' "$bootstrap_stage/paths.txt"; then fail 'bootstrap selected older fourth pair'; fi
grep -qx $'window-mode\tbootstrap-count' "$bootstrap_stage/meta/window.tsv" || fail 'bootstrap window mode not recorded'
grep -qx $'bootstrap-count\t3' "$bootstrap_stage/meta/window.tsv" || fail 'bootstrap request not recorded'
printf 'PASS bootstrap-count selects newest paired metas with deterministic ties, honest full census, and loud zero-survivor HOLD\n'

nudge_repo="$test_root/nudge-repo"
nudge_tmp="$test_root/nudge-tmp"
nudge_under_test="$test_root/dreamer-nudge.sh"
mkdir -m 0700 "$nudge_repo" "$nudge_repo/.professor" "$nudge_repo/.professor/stm" "$nudge_repo/.professor/stm/dreamer" "$nudge_tmp"
cp "$NUDGE" "$nudge_under_test"
sed -i "s|^  failure_root=/tmp|  failure_root=$nudge_tmp|" "$nudge_under_test"
chmod 0700 "$nudge_under_test"

outside_output=$(CLAUDE_PROJECT_DIR="$test_root/no-organ" bash "$nudge_under_test" <<< '{}')
[ -z "$outside_output" ] || fail 'nudge spoke outside an organ-bearing repository'
sed "s/{{APPLIED_AT}}/$(date -Is)/" "$FIXTURES/nudge-sweep.md.in" > "$nudge_repo/.professor/stm/dreamer/$(date +%F).md"
healthy_output=$(CLAUDE_PROJECT_DIR="$nudge_repo" bash "$nudge_under_test" <<< '{}')
[ -z "$healthy_output" ] || fail 'nudge spoke for a recent healthy sweep'
sed 's/{{APPLIED_AT}}/2000-01-01T00:00:00+00:00/' "$FIXTURES/nudge-sweep.md.in" > "$nudge_repo/.professor/stm/dreamer/$(date +%F).md"
stale_output=$(CLAUDE_PROJECT_DIR="$nudge_repo" bash "$nudge_under_test" <<< '{}')
if [ "$(wc -l <<< "$stale_output")" -ne 1 ] || ! grep -q '^🌙 dreamer-night stale' <<< "$stale_output"; then
  fail 'stale nudge was not exactly one line'
fi
mkdir -m 0700 "$nudge_tmp/dreamer-night-fixture"
printf 'fixture\n' > "$nudge_tmp/dreamer-night-fixture/FAILED"
mkdir -m 0700 "$nudge_tmp/dreamer-night-fixture/meta"
printf '%s\n' "$nudge_repo" > "$nudge_tmp/dreamer-night-fixture/meta/repo-root.txt"
failed_output=$(CLAUDE_PROJECT_DIR="$nudge_repo" bash "$nudge_under_test" <<< '{}')
if [ "$(wc -l <<< "$failed_output")" -ne 1 ] || ! grep -q '^🌙 dreamer-night failed' <<< "$failed_output"; then
  fail 'failure nudge was not exactly one line'
fi
printf '%s\n' "$test_root/another-repo" > "$nudge_tmp/dreamer-night-fixture/meta/repo-root.txt"
cross_repo_output=$(CLAUDE_PROJECT_DIR="$nudge_repo" bash "$nudge_under_test" <<< '{}')
grep -q '^🌙 dreamer-night stale' <<< "$cross_repo_output" || fail 'another repository failure masked this repository stale state'
printf 'PASS nudge is silent when healthy and emits at most one failure-or-stale line\n'

# shellcheck disable=SC2016
grep -q 'index="$organ/agents/$lane.md"' "$HOOK" || fail 'hook does not resolve the lane surface'
hook_input='{"tool_input":{"subagent_type":"Explore","prompt":"fixture prompt"}}'
hook_output=$(CLAUDE_PROJECT_DIR="$REPO" bash "$HOOK" <<< "$hook_input")
hook_prompt=$(jq -r '.hookSpecificOutput.updatedInput.prompt' <<< "$hook_output")
grep -q -- ' -> maps/' <<< "$hook_prompt" || fail 'hook injected no map pointers'

# Drift is asserted POSITIVELY on a fixture, never as "the live repo is currently
# clean". A live map goes DRIFTED the moment anyone edits a file it anchors, which
# is the mechanism working — asserting its absence made this test fail on healthy
# repository change and proved nothing about the marker itself.
drift_organ="$test_root/drift-repo/.professor/stm"
mkdir -p "$drift_organ/maps" "$drift_organ/agents" "$drift_organ/dreamer" "$drift_organ/archive"
cp -al "$REPO/.git" "$test_root/drift-repo/.git"
printf '# fixture STM\n' > "$drift_organ/stm.md"
printf '%s\n' '# Drifted fixture map' '' '## Question' '' 'q' '' '## Answer' '' 'a' '' \
  '## Anchors' '' '- `README.md` — blob `000000000000`' > "$drift_organ/maps/drifted.md"
printf -- '- Drifted fixture map -> maps/drifted.md\n' > "$drift_organ/agents/explorer.md"
drift_out=$(CLAUDE_PROJECT_DIR="$test_root/drift-repo" bash "$HOOK" <<< "$hook_input" \
  | jq -r '.hookSpecificOutput.updatedInput.prompt')
grep -q '⚠ DRIFTED' <<< "$drift_out" || fail 'a moved anchor did not render the DRIFTED marker'
printf 'PASS hook prefers agents/explorer.md and a moved anchor renders DRIFTED\n'

# Repo-independence is asserted against a FIXTURE organ, never another project's
# live one: that organ answers to its own owner, who may disable a surface for a
# benchmark, and a battery that reads it fails for reasons the engine did not cause.
other_repo="$test_root/other-repo"
other_organ="$other_repo/.professor/stm"
mkdir -m 0700 -p "$other_organ/agents" "$other_organ/maps"
printf -- '- Other repository subject -> maps/other-subject.md\n' > "$other_organ/agents/explorer.md"
other_hook_prompt=$(CLAUDE_PROJECT_DIR="$other_repo" bash "$HOOK" <<< "$hook_input" | jq -r '.hookSpecificOutput.updatedInput.prompt // empty')
grep -q 'Other repository subject -> maps/other-subject.md' <<< "$other_hook_prompt" || \
  fail 'Claude hook did not resolve a second repository organ'
other_codex_context=$(bash "$CODEX_HOOK" <<< "{\"agent_type\":\"explorer\",\"cwd\":\"$other_repo\"}" | jq -r '.hookSpecificOutput.additionalContext // empty')
grep -q 'Other repository subject -> maps/other-subject.md' <<< "$other_codex_context" || \
  fail 'Codex hook did not resolve a second repository organ'
[ -d "$INTUITA/.professor/stm" ] || fail 'the Projc organ vanished'
printf 'PASS both hooks resolve any repository organ, hermetically\n'

morning_engine="$test_root/morning-engine"
morning_calls="$test_root/morning-calls.tsv"
mkdir -m 0700 "$morning_engine"
cp "$MORNING" "$morning_engine/dreamer-morning.sh"
printf '/fixture/fail\n# retained comment\n/fixture/pass\n' > "$morning_engine/repos.list"
# shellcheck disable=SC2016
printf '%s\n' '#!/usr/bin/env bash' 'set -u' \
  'repo=$2' \
  'printf "%s" "$1" >> '"$(printf '%q' "$morning_calls")" \
  'shift' \
  'printf "\t%s" "$@" >> '"$(printf '%q' "$morning_calls")" \
  'printf "\n" >> '"$(printf '%q' "$morning_calls")" \
  'if [ "$repo" = /fixture/fail ]; then exit 23; fi' > "$morning_engine/dreamer-night.sh"
chmod 0700 "$morning_engine/dreamer-morning.sh" "$morning_engine/dreamer-night.sh"
if bash "$morning_engine/dreamer-morning.sh" > "$test_root/morning.out" 2>&1; then
  fail 'morning wrapper hid a repository failure'
fi
grep -qx -- $'--repo\t/fixture/fail' "$morning_calls" || fail 'morning wrapper did not call failing repository'
grep -qx -- $'--repo\t/fixture/pass' "$morning_calls" || fail 'morning wrapper stopped before the later repository'
grep -q '^dreamer-morning: FAIL repo=/fixture/fail agent=Explore rc=23$' "$test_root/morning.out" || fail 'morning wrapper omitted failure section'
grep -q '^dreamer-morning: PASS repo=/fixture/pass agent=Explore$' "$test_root/morning.out" || fail 'morning wrapper omitted later pass section'
: > "$morning_calls"
printf '/fixture/pass qa-projb-cortex\n' > "$morning_engine/repos.list"
bash "$morning_engine/dreamer-morning.sh" > "$test_root/morning-lane.out" 2>&1 || fail 'lane row failed the morning wrapper'
grep -qx -- $'--repo\t/fixture/pass\t--agent\tqa-projb-cortex' "$morning_calls" || fail 'lane row lost its manual override'
grep -q '^dreamer-morning: PASS repo=/fixture/pass agent=qa-projb-cortex$' "$test_root/morning-lane.out" || fail 'lane row did not name its agent'
printf 'PASS morning wrapper runs sequentially, names each lane, and continues after failure\n'

morning_repo="$test_root/morning-repo"
mkdir -m 0700 -p "$morning_repo/.professor/stm/lanes"
printf 'fixture QA lane\n' > "$morning_repo/.professor/stm/lanes/fixture-qa.md"
printf 'fixture Explore lane\n' > "$morning_repo/.professor/stm/lanes/explorer.md"
: > "$morning_calls"
printf '%s\n' "$morning_repo" > "$morning_engine/repos.list"
bash "$morning_engine/dreamer-morning.sh" > "$test_root/morning-discovery.out" 2>&1 || fail 'organ lane discovery failed the morning wrapper'
[ "$(grep -Fxc -- $'--repo\t'"$morning_repo" "$morning_calls")" -eq 1 ] || fail 'morning wrapper did not run Explore exactly once'
[ "$(grep -Fxc -- $'--repo\t'"$morning_repo"$'\t--agent\tfixture-qa' "$morning_calls")" -eq 1 ] || fail 'morning wrapper did not run the discovered organ lane exactly once'
[ "$(wc -l < "$morning_calls")" -eq 2 ] || fail 'morning wrapper duplicated explorer.md or ran an undiscovered lane'
grep -q "^dreamer-morning: PASS repo=$morning_repo agent=Explore$" "$test_root/morning-discovery.out" || fail 'discovered-lane run omitted the Explore pass section'
grep -q "^dreamer-morning: PASS repo=$morning_repo agent=fixture-qa$" "$test_root/morning-discovery.out" || fail 'discovered-lane run omitted the organ lane pass section'
printf 'PASS morning wrapper discovers organ lanes once and never duplicates explorer.md\n'

surface_artifacts=$(sed -n 's/^SURFACES PASS byte-stable .*artifacts=\(.*\)$/\1/p' "$test_root/surfaces.out")
[ -n "$surface_artifacts" ] || fail 'surface test did not name its artifacts'
cmp "$surface_artifacts/one/agents/explorer.md" "$REPO/.professor/stm/agents/explorer.md" || \
  fail 'the lane refactor changed the live explorer surface'
printf 'PASS explorer surface stays byte-identical to the live organ under lanes\n'

lanes_maps="$test_root/lane-maps"
cp -a "$REPO/.professor/stm/maps" "$lanes_maps"
printf '# New QA map\n' > "$lanes_maps/qa-fixture-map.md"
pool_count=$(find "$REPO/.professor/stm/maps" -maxdepth 1 -type f -name '*.md' | wc -l)
"$RUNNER" --agent qa-projb-cortex lane-membership "$lanes_maps" "$test_root/absent-lanes.tsv" "$test_root/lanes-backfill.tsv" > "$test_root/lanes-backfill.out"
grep -qx $'qa-fixture-map.md\tqa-projb-cortex' "$test_root/lanes-backfill.tsv" || fail 'new map did not take the running lane'
[ "$(awk -F '\t' '$2 == "explorer"' "$test_root/lanes-backfill.tsv" | wc -l)" -eq "$pool_count" ] || \
  fail 'legacy pool was not backfilled to explorer'
first_map=$(find "$REPO/.professor/stm/maps" -maxdepth 1 -type f -name '*.md' -printf '%f\n' | sort | head -1)
awk -F '\t' -v m="$first_map" '$1 != m' "$test_root/lanes-backfill.tsv" > "$test_root/lanes-holed.tsv"
if "$RUNNER" --agent qa-projb-cortex lane-membership "$lanes_maps" "$test_root/lanes-holed.tsv" "$test_root/lanes-out.tsv" > "$test_root/lanes-holed.out" 2>&1; then
  fail 'a lane ledger hiding a pre-existing map passed'
fi
grep -q 'pre-existing map carries no lane row' "$test_root/lanes-holed.out" || fail 'hidden lane row was not named'
printf 'PASS lane membership backfills a pre-lane pool and fails closed on a hidden map\n'

lane_repo="$test_root/lane-repo"
lane_organ="$lane_repo/.professor/stm"
mkdir -m 0700 -p "$lane_organ/agents" "$lane_organ/maps"
printf -- '- Explorer subject -> maps/explorer-subject.md\n' > "$lane_organ/agents/explorer.md"
printf -- '- Cortex QA subject -> maps/cortex-qa-subject.md\n' > "$lane_organ/agents/qa-projb-cortex.md"
lane_prompt() {
  CLAUDE_PROJECT_DIR="$lane_repo" bash "$HOOK" <<< "{\"tool_input\":{\"subagent_type\":\"$1\",\"prompt\":\"fixture\"}}" |
    jq -r '.hookSpecificOutput.updatedInput.prompt // empty'
}
explore_lane_prompt=$(lane_prompt Explore)
grep -q 'Explorer subject' <<< "$explore_lane_prompt" || fail 'Explore did not receive its own lane surface'
if grep -q 'Cortex QA subject' <<< "$explore_lane_prompt"; then fail 'Explore received the QA lane surface'; fi
qa_lane_prompt=$(lane_prompt qa-projb-cortex)
grep -q 'Cortex QA subject' <<< "$qa_lane_prompt" || fail 'the QA agent did not receive its own lane surface'
if grep -q 'Explorer subject' <<< "$qa_lane_prompt"; then fail 'the QA agent received the explorer lane surface'; fi
[ -z "$(lane_prompt gitter)" ] || fail 'an agent type with no lane received an injection'
[ -z "$(lane_prompt general-purpose)" ] || fail 'general-purpose received an injection'
[ -z "$(lane_prompt ../../etc/passwd)" ] || fail 'a traversal agent type produced output'
codex_lane_context=$(bash "$CODEX_HOOK" <<< "{\"agent_type\":\"qa-projb-cortex\",\"cwd\":\"$lane_repo\"}" | jq -r '.hookSpecificOutput.additionalContext // empty')
grep -q 'Cortex QA subject' <<< "$codex_lane_context" || fail 'codex hook lost the QA lane surface'
if grep -q 'Explorer subject' <<< "$codex_lane_context"; then fail 'codex hook leaked the explorer lane surface'; fi
printf 'PASS each lane is injected only its own surface, and a lane-less type gets nothing\n'

corpus_list="$test_root/corpus.txt"
printf '%s\n%s\n' "$fixture_registry/newest/agent-newest.jsonl" "$fixture_registry/older/agent-older.jsonl" > "$corpus_list"
corpus_output=$(PATH="$fake_bin:$PATH" "$empty_engine/dreamer-night.sh" --repo "$fixture_repo" --agent qa-projb-cortex --corpus-file "$corpus_list" supervise) || fail 'corpus-file run failed'
corpus_stage=$(sed -n 's/^dreamer-night: HOLD-BEFORE-APPLY ZERO-SURVIVORS stage=\([^ ]*\)$/\1/p' <<< "$corpus_output")
if [ -z "$corpus_stage" ] || [ ! -d "$corpus_stage" ]; then
  fail 'corpus-file stage was not preserved'
fi
grep -Fq 'ORGAN QA PROFILE' "$corpus_stage/distill-brief.md" || fail 'the organ-local lane profile did not reach the runner brief'
if grep -Fq 'GLOBAL QA PROFILE' "$corpus_stage/distill-brief.md"; then fail 'the global lane profile overrode the organ copy'; fi
printf 'PASS an organ-local lane profile takes precedence over the same global lane\n'
grep -qx $'window-mode\tcorpus-file' "$corpus_stage/meta/window.tsv" || fail 'corpus-file mode was not recorded'
grep -qx $'lane\tqa-projb-cortex' "$corpus_stage/meta/window.tsv" || fail 'the lane was not recorded in the window'
grep -qx "corpus-file-sha256"$'\t'"$(sha256sum "$corpus_list" | cut -d ' ' -f 1)" "$corpus_stage/meta/window.tsv" || \
  fail 'the corpus digest was not recorded'
[ "$(wc -l < "$corpus_stage/paths.txt")" -eq 2 ] || fail 'corpus-file selected the wrong path count'
"$RUNNER" gate-pin "$corpus_stage/paths.txt" "$corpus_stage/paths.sha256" > /dev/null || fail 'the explicit corpus failed the pin gate'
printf '/nonexistent/agent-ghost.jsonl\n' > "$test_root/corpus-ghost.txt"
if PATH="$fake_bin:$PATH" "$empty_engine/dreamer-night.sh" --repo "$fixture_repo" --agent qa-projb-cortex --corpus-file "$test_root/corpus-ghost.txt" supervise > "$test_root/corpus-ghost.out" 2>&1; then
  fail 'corpus-file accepted a nonexistent transcript'
fi
grep -q 'corpus path is not a readable file' "$test_root/corpus-ghost.out" || fail 'the ghost corpus path was not named'
printf 'PASS corpus-file pins an explicit corpus, records its digest and lane, and fails closed on a ghost path\n'

lane_window_output=$(PATH="$fake_bin:$PATH" "$empty_engine/dreamer-night.sh" --repo "$fixture_repo" --agent qa-projb-cortex) || fail 'QA lane window run failed'
grep -qF '(no qa-projb-cortex transcripts since 7 days ago)' <<< "$lane_window_output" || \
  fail "the QA lane inherited another lane's cutoff: $lane_window_output"
if "$empty_engine/dreamer-night.sh" --repo "$fixture_repo" --agent no-such-agent > "$test_root/lane-profile.out" 2>&1; then
  fail 'a lane without a profile ran'
fi
grep -Fq "lane has no profile: expected $fixture_organ/lanes/no-such-agent.md or $empty_engine/lanes/no-such-agent.md" "$test_root/lane-profile.out" || \
  fail 'the missing lane failure did not name both expected profile paths'
printf 'PASS lane windows are independent and a lane without a profile cannot run\n'

grep -qx 'DISTILL_MODEL=gpt-5.6-luna' "$RUNNER" || fail 'the dreamer no longer distills on luna'
grep -q 'refusing DISTILL_MODEL=' "$RUNNER" || fail 'the engine lost its luna-only guard'
luna_engine="$test_root/luna-law-engine"
mkdir -m 0700 "$luna_engine"
cp "$RUNNER" "$ENGINE_DIR/dreamer-distill.prompt.md" "$ENGINE_DIR/dreamer-refiner.prompt.md" "$luna_engine/"
cp -a "$ENGINE_DIR/lanes" "$luna_engine/lanes"
sed -i 's/^DISTILL_MODEL=gpt-5.6-luna$/DISTILL_MODEL=gpt-5.6-sol/' "$luna_engine/dreamer-night.sh"
chmod 0700 "$luna_engine/dreamer-night.sh"
if PATH="$fake_bin:$PATH" "$luna_engine/dreamer-night.sh" --repo "$fixture_repo" supervise > "$test_root/luna-law.out" 2>&1; then
  fail 'a night distilling on a model other than luna was allowed to run'
fi
grep -q 'the dreamer distills on luna only; refusing DISTILL_MODEL=gpt-5.6-sol' "$test_root/luna-law.out" || \
  fail 'the luna-only refusal was not named'
printf 'PASS the dreamer distills on luna and refuses any other model\n'

printf 'ALL PASS artifacts=%s\n' "$test_root"
