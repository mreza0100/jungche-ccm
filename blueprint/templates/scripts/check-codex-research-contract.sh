#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

status=0

if [[ -d ".codex/agents" && -d ".codex/skills" ]]; then
  CODEX_AGENTS_DIR=".codex/agents"
  CODEX_SKILLS_DIR=".codex/skills"
elif [[ -d "blueprint/templates/codex/agents" && -d "blueprint/templates/codex/skills" ]]; then
  CODEX_AGENTS_DIR="blueprint/templates/codex/agents"
  CODEX_SKILLS_DIR="blueprint/templates/codex/skills"
elif [[ -d "templates/codex/agents" && -d "templates/codex/skills" ]]; then
  CODEX_AGENTS_DIR="templates/codex/agents"
  CODEX_SKILLS_DIR="templates/codex/skills"
else
  echo "FAIL: no Codex agent/skill directory found"
  exit 1
fi

required_skills=(360 rr rnd ghostwriter)
for skill in "${required_skills[@]}"; do
  if [[ ! -e "$CODEX_SKILLS_DIR/$skill/SKILL.md" ]]; then
    echo "FAIL: missing Codex shared skill wrapper: $CODEX_SKILLS_DIR/$skill/SKILL.md"
    status=1
  fi
done

if rg -n 'rr skill is Claude-side|The `rr` skill is Claude-side' "$CODEX_AGENTS_DIR" "$CODEX_SKILLS_DIR"; then
  echo "FAIL: stale Codex RR wording found; wrappers must preserve RR protocol instead of declaring it unavailable"
  status=1
fi

if rg -n 'use WebSearch/WebFetch directly' "$CODEX_AGENTS_DIR" "$CODEX_SKILLS_DIR"; then
  echo "FAIL: direct WebSearch/WebFetch wording found; narrow lookup wording must say narrow fact checks, not replace RR"
  status=1
fi

if ! rg -n 'RR-compatible pipeline|shared rr protocol|shared `rr` protocol' "$CODEX_AGENTS_DIR" "$CODEX_SKILLS_DIR" >/dev/null; then
  echo "FAIL: no Codex wrapper advertises the RR-compatible pipeline contract"
  status=1
fi

if [[ "$status" -eq 0 ]]; then
  echo "PASS: Codex research contract preserves shared RR skill semantics"
fi

exit "$status"
