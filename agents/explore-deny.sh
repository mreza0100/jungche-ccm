#!/usr/bin/env bash
# explore-deny.sh — PreToolUse gate on Agent/Task spawns.
#
# Explore is retired in favour of the `tracer` agent: same read-only fan-out,
# but quote-pinned edges, per-file dispositions, and a coverage close that
# separates "looked and found nothing" from "failed to look".
#
# ONE sanctioned exception: the tracer dispatches its OWN children as
# Explore + model:haiku (lean context, no CLAUDE.md chain). Denying those
# would break the very agent this gate steers callers toward, so a spawn
# carrying model:haiku passes. That is a steering allowance, not a security
# boundary — this gate shapes which tool an agent reaches for, nothing more.
#
# Fails OPEN: a parse error or missing jq must never wedge every Agent spawn
# on the box. Silence = allow.
set -uo pipefail

command -v jq >/dev/null 2>&1 || exit 0

INPUT="$(cat 2>/dev/null)" || exit 0
[[ -n "$INPUT" ]] || exit 0

SUBAGENT="$(printf '%s' "$INPUT" | jq -r '.tool_input.subagent_type // empty' 2>/dev/null)" || exit 0
[[ "$SUBAGENT" == "Explore" ]] || exit 0

MODEL="$(printf '%s' "$INPUT" | jq -r '.tool_input.model // empty' 2>/dev/null)"
[[ "$MODEL" == "haiku" ]] && exit 0   # the tracer's own children — sanctioned

REASON='Explore is disabled — use the `tracer` agent instead.

Spawn `subagent_type: "tracer"` with the mission in the prompt. It returns the
same read-only map with evidence Explore cannot give you: every edge quoted at
file:line, every bucket file dispositioned (EDGE / RED-HERRING / FRONTIER /
FAILED-TO-LOOK), caller greps behind every named consumer, and a coverage close
that distinguishes "looked and found nothing" from "failed to look".

The tracer dispatches its own Explore children internally; that path is allowed
and needs nothing from you.'

jq -n --arg r "$REASON" '{
  hookSpecificOutput: {
    hookEventName: "PreToolUse",
    permissionDecision: "deny",
    permissionDecisionReason: $r
  }
}'
exit 0
