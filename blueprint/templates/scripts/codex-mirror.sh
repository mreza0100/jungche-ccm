#!/usr/bin/env bash
# codex-mirror.sh — mirror every Claude entry point (.claude/commands/**.md +
# .claude/skills/*/) into .codex/skills/ so Codex routes to the SAME protocols
# Claude does. `.claude/` stays the single source of truth; `.codex/skills/` is a
# generated pointer layer with zero authored protocol inside it.
#
# ─── WHY POINTER FILES, NOT SYMLINKS (a measured harness constraint) ─────────
# codex 0.144.1 follows a symlinked skill DIRECTORY, but silently DROPS a real
# directory whose SKILL.md is a symlinked FILE. No warning, no error, no log line
# — the skill simply does not exist as far as Codex is concerned. Measured on a
# 56-entry mirror: 9 skills discovered before the fix, 56 after, with the same 56
# directories on disk the whole time. So:
#
#   Claude SKILL — already a DIRECTORY -> true symlink to that directory (0 bytes)
#   Claude COMMAND — a single .md FILE -> generated SKILL.md pointer inside a real
#                                         directory (it has no dir to link, and
#                                         linking the FILE loses it silently)
#
# Re-probe this on every codex upgrade BEFORE "simplifying" it back to symlinks:
# the failure mode is invisible, so a regression looks exactly like success.
#
# ─── MODES ───────────────────────────────────────────────────────────────────
#   generate  (default) — (re)write every pointer; leaves hand-written cards alone
#   check               — exit 1 if any pointer is stale or missing; prints what
#                         and why. Wire this into CI / the pre-merge gate: a
#                         command whose frontmatter changed leaves Codex routing
#                         on a stale description until the mirror runs again.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
MODE="${1:-generate}"

# ─── Hand-written Codex cards — never overwritten ────────────────────────────
# A card here is a REAL SKILL.md holding Codex-specific mechanics that have no
# Claude-side equivalent (harness deltas, ping channel, launch command). Add the
# .codex/skills/ directory name of every card you author, space-padded on both
# sides. Everything NOT listed is generated and freely clobbered.
HANDWRITTEN=" chat wave-builder "

# Frontmatter block of a markdown file (between the first two --- fences), verbatim.
frontmatter() { awk '/^---[[:space:]]*$/{n++; next} n==1{print} n>=2{exit}' "$1"; }

# Pointer paths are REPO-RELATIVE, never absolute: worktrees materialize these
# committed files, and a lane must read its OWN branch's protocol — an absolute
# path would silently route every worktree to the main checkout's version.
pointer_body() {
  printf -- '---\n%s\n---\n\nRead `%s` (relative to the repo root you were launched in) in full — that file is your complete protocol; execute it exactly.\n' \
    "$(frontmatter "$1")" "$1"
}

stale=0; wrote=0; kept=0; linked=0

# ─── Enumerate the three entry-point shapes ──────────────────────────────────
# 1. directory-shaped COMMANDS — a .claude/commands/**/SKILL.md means its PARENT
#    directory is the entry point (a command that grew into a directory). Those
#    are already directories, so they get a true symlink like a skill, and their
#    SKILL.md is excluded from the pointer pass.
dir_cmds=()
while IFS= read -r rel; do
  [ -n "$rel" ] || continue
  dir_cmds+=("${rel%/SKILL.md}")
done < <(find .claude/commands -name 'SKILL.md' 2>/dev/null | sed 's|^\.claude/commands/||' | sort)

# 2. single-file COMMANDS -> generated pointer.
cmd_files=()
while IFS= read -r rel; do
  [ -n "$rel" ] || continue
  case "$rel" in */SKILL.md | SKILL.md) continue ;; esac   # handled as dir_cmds
  cmd_files+=("$rel")
done < <(find .claude/commands -name '*.md' ! -name 'README.md' 2>/dev/null | sed 's|^\.claude/commands/||' | sort)

# 3. SKILLS -> true directory symlinks.
skills=()
for d in .claude/skills/*/; do
  [ -d "$d" ] || continue                                   # no skills yet: fine
  skills+=("$(basename "$d")")
done

# ─── Single-file commands -> generated pointer ───────────────────────────────
# A nested command (`wave/refine.md`) flattens to one skill name (`wave-refine`):
# Codex skill names are a flat namespace.
for rel in ${cmd_files[@]+"${cmd_files[@]}"}; do
  name="$(printf '%s' "${rel%.md}" | tr '/' '-')"
  case "$HANDWRITTEN" in *" $name "*) kept=$((kept + 1)); continue ;; esac
  src=".claude/commands/$rel"
  dst=".codex/skills/$name/SKILL.md"
  want="$(pointer_body "$src")"
  if [ -L "$dst" ] || [ ! -f "$dst" ] || [ "$(cat "$dst" 2>/dev/null)" != "$want" ]; then
    if [ "$MODE" = check ]; then
      echo "STALE  $name  (source: $src)"
      stale=$((stale + 1))
      continue
    fi
    mkdir -p "$(dirname "$dst")"
    rm -f "$dst"
    printf '%s' "$want" >"$dst"
    wrote=$((wrote + 1))
  fi
done

# ─── Directory-shaped entry points -> true symlinks ──────────────────────────
link_dir() {
  local name="$1" target="$2" dst=".codex/skills/$1"
  case "$HANDWRITTEN" in *" $name "*) kept=$((kept + 1)); return ;; esac
  if [ -L "$dst" ] && [ "$(readlink "$dst")" = "$target" ]; then
    linked=$((linked + 1))
    return
  fi
  if [ "$MODE" = check ]; then
    echo "STALE  $name  (want dir symlink -> $target)"
    stale=$((stale + 1))
    return
  fi
  rm -rf "$dst"
  ln -s "$target" "$dst"
  linked=$((linked + 1))
}

for d in ${dir_cmds[@]+"${dir_cmds[@]}"}; do
  link_dir "$(printf '%s' "$d" | tr '/' '-')" "../../.claude/commands/$d"
done
for s in ${skills[@]+"${skills[@]}"}; do
  link_dir "$s" "../../.claude/skills/$s"
done

total=$(( ${#cmd_files[@]} + ${#dir_cmds[@]} + ${#skills[@]} ))

# COVERAGE LINE, not a bare verdict — both modes name what was inspected and what
# was deliberately skipped, so "nothing stale" can never be read as "nothing here".
if [ "$MODE" = check ]; then
  if [ "$stale" -gt 0 ]; then
    echo "CHECK FAIL — $stale of $total entry points stale or missing. Run: $0 generate"
    exit 1
  fi
  echo "CHECK PASS — $total entry points inspected (${#cmd_files[@]} file commands, ${#dir_cmds[@]} dir commands, ${#skills[@]} skills); hand-written cards skipped:${HANDWRITTEN}"
  exit 0
fi

echo "mirrored $total entry points: $wrote pointers written, $linked dir symlinks, $kept hand-written cards untouched"
echo "codex skill dirs now: $(find .codex/skills -mindepth 1 -maxdepth 1 | wc -l)"
