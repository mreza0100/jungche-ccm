#!/usr/bin/env bash
set -euo pipefail

# PostToolUse(Bash) — failure-biased filter for test-runner output. Strips passing
# noise so only failures, errors, and the summary block reach the agent's context.
# Wired GLOBALLY in settings.json PostToolUse(Bash) so EVERY agent that runs a suite
# (developers + fix-loop devs included — the heaviest token burners) gets it, not just
# the qa-{project} frontmatter hooks. No-ops on any non-test Bash command.
# Verify live field name (tool_output vs tool_response) during pilot.

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
  *"{PACKAGE_MANAGER} test"*|*"{PACKAGE_MANAGER} run test"*|\
  *"{PACKAGE_MANAGER} run integration"*|*"{PACKAGE_MANAGER} run e2e"*|\
  *"{TEST_RUNNER}"*) ;;
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

# Failure-biased keep: failures, errors, tracebacks, warning/coverage summaries,
# and the final summary block. Cap to the last 200 kept lines.
FILTERED=$(printf '%s\n' "$OUTPUT" \
  | grep -iE 'fail|error|exception|assert|✗|FAILED|ERROR|Traceback|warning|coverage|passed|failed|[0-9]+ (passed|failed|error)' \
  | tail -200 || true)

if [[ -z "$FILTERED" ]]; then
  FILTERED="(all tests passed — verbose output suppressed by filter-test-output.sh)"
fi

jq -nc --arg out "$FILTERED" '
  {hookSpecificOutput: {hookEventName: "PostToolUse", updatedToolOutput: $out}}
'
