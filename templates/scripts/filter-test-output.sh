#!/usr/bin/env bash
set -euo pipefail

# Failure-biased filter for test-runner output — keeps failures, errors, tracebacks,
# the summary block, and coverage totals; drops passing noise. TWO entry modes:
#
#   PIPE (subagents + anyone):  <test cmd> 2>&1 | filter-test-output.sh -p
#     Reads RAW test output on stdin, prints the filtered subset to stdout. This is the
#     path QA/dev agents use, because Claude Code does NOT propagate settings.json hooks
#     to subagents — the hook below never fires for an agent spawned via the Agent tool.
#
#   HOOK (default, no args):  wired in settings.json PostToolUse(Bash).
#     Reads the hook JSON envelope on stdin, returns updatedToolOutput. Fires ONLY for
#     the main agent loop (and for a QA agent spawned as a registered qa-{project} type,
#     whose frontmatter re-declares this hook). No-ops on any non-test command.

# Shared filter: raw test output on stdin -> failure-biased subset on stdout. Keeps
# failures, errors, tracebacks, warning/coverage summaries, coverage totals (TOTAL /
# All files — so a piped coverage run still shows the % number), and the final summary
# block. Caps to the last 200 kept lines.
_filter() {
  local out kept
  out=$(cat)
  [[ -z "$out" ]] && return 0
  kept=$(printf '%s\n' "$out" \
    | grep -iE 'fail|error|exception|assert|✗|FAILED|ERROR|Traceback|warning|coverage|TOTAL|All files|passed|failed|[0-9]+ (passed|failed|error)' \
    | tail -200 || true)
  if [[ -z "$kept" ]]; then
    # A green run ALWAYS emits an "N passed" summary the keep-grep catches, so an empty
    # result means the runner crashed, was killed (timeout/OOM), or printed an
    # unrecognized format — surface the raw tail so a real failure is never reported green.
    kept="(no pass/fail summary recognized — runner may have crashed or been killed; raw tail follows)
$(printf '%s\n' "$out" | tail -40)"
  fi
  printf '%s\n' "$kept"
}

# --- PIPE mode ---
# Exit status is DEFENSE-IN-DEPTH, never the run verdict — the runner's OWN exit
# code, captured by the caller, is the verdict. This mode never asserts success:
#   3 — no pass/fail summary recognized (runner crashed, was killed, or printed an
#       unrecognized format)
#   1 — a failure indicator was recognized in the kept output
#   0 — a green summary was seen
# A wrapper keying on this exit can therefore no longer read a failing or absent
# run as success.
if [[ "${1:-}" == "-p" || "${1:-}" == "--pipe" ]]; then
  out=$(_filter)
  printf '%s\n' "$out"
  if printf '%s' "$out" | grep -q 'no pass/fail summary recognized'; then
    exit 3
  elif printf '%s' "$out" | grep -qiE '(^|[^0-9])[1-9][0-9]* (failed|failures?)|FAILED|Traceback|✗'; then
    exit 1
  fi
  exit 0
fi

# --- HOOK mode ---
INPUT=$(cat)

# Act only on test-runner Bash commands; otherwise leave tool output untouched.
TOOL=$(echo "$INPUT" | jq -r '.tool_name // empty')
[[ "$TOOL" == "Bash" ]] || exit 0

# Match the test commands this project actually runs — bare invocations and the
# `run`-script shapes (test/integration/e2e). INSTALL: extend this case list with
# your project's actual test commands (your package manager, your test runner, and
# any extra suites such as <your test commands>) so nothing slips through unfiltered.
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // empty')
case "$CMD" in
  *"{PROJECT_PKG_MGR} test"*|*"{PROJECT_PKG_MGR} run test"*|\
  *"{PROJECT_PKG_MGR} run integration"*|*"{PROJECT_PKG_MGR} run e2e"*|\
  *"{PROJECT_TEST_RUNNER}"*) ;;
  *) exit 0 ;;
esac

# Output field name varies by Claude Code version (tool_output vs tool_response),
# and the value may be a plain string or an object with stdout/stderr fields.
OUTPUT=$(echo "$INPUT" | jq -r '
  (.tool_output // .tool_response // empty) as $o
  | if ($o | type) == "object"
    then ([$o.stdout, $o.stderr] | map(select(. != null)) | join("\n"))
    else ($o // "") end
')
[[ -z "$OUTPUT" ]] && exit 0

FILTERED=$(printf '%s\n' "$OUTPUT" | _filter)

jq -nc --arg out "$FILTERED" '
  {hookSpecificOutput: {hookEventName: "PostToolUse", updatedToolOutput: $out}}
'
