#!/usr/bin/env bash
# Lane-scoped PreToolUse injection with per-map anchor-drift annotations.
# A subagent type is fed only the surface its own lane generated; a type with no
# generated surface receives nothing, so isolation needs no allow-list.
set -euo pipefail

in=$(cat)
repo_root="${CLAUDE_PROJECT_DIR:-}"
[ -n "$repo_root" ] || exit 0
organ_root="${repo_root%%/.worktrees/*}"
organ="$organ_root/.professor/stm"
[ -d "$organ" ] || exit 0

type=$(jq -r '.tool_input.subagent_type // empty' <<<"$in" 2>/dev/null || true)
prompt=$(jq -r '.tool_input.prompt // empty' <<<"$in" 2>/dev/null || true)
[ -n "$type" ] && [ -n "$prompt" ] || exit 0
if [ "$type" = "Explore" ]; then
  lane=explorer
else
  lane=$(tr '[:upper:]' '[:lower:]' <<<"$type" | tr -c 'a-z0-9-' '-')
  lane=${lane%-}
fi
[[ "$lane" =~ ^[a-z0-9][a-z0-9-]*$ ]] || exit 0

index="$organ/agents/$lane.md"
# The fallback exists only until the signed first-night APPLY retires the old surface.
[ "$lane" != explorer ] || [ -s "$index" ] || index="$organ/explorer-index.md"
[ -s "$index" ] || exit 0
surface=$(grep -E '^- [^[:space:]].* -> maps/[a-z0-9][a-z0-9-]*\.md$' "$index" || true)
[ -n "$surface" ] || exit 0
[ "$surface" = "$(cat "$index")" ] || exit 1

# The index always remains injectable. Annotation is best-effort: any repository,
# parser, or unexpected git failure falls back to the byte-exact plain surface.
annotated_surface="$surface"
if git -C "$repo_root" rev-parse --verify -q HEAD >/dev/null 2>&1; then
  declare -A moved_by_slug=()
  declare -A total_by_slug=()
  scan_ok=1
  for map_file in "$organ"/maps/*.md; do
    [ -f "$map_file" ] || continue
    slug=$(basename "$map_file" .md)
    anchor_rows=$(awk '
      /^## Anchors[[:space:]]*$/ { in_anchors=1; found=1; next }
      in_anchors && /^##[[:space:]]/ { in_anchors=0 }
      !in_anchors { next }
      /^[[:space:]]*$/ { next }
      {
        if ($0 !~ /^- `[^`]+` — (blob|tree) `[0-9a-f]+`$/) {
          bad=1
          next
        }
        fields=split($0, ticks, "`")
        if (fields != 5) { bad=1; next }
        path=ticks[2]
        hash=ticks[4]
        if (length(hash) != 12 || hash !~ /^[0-9a-f]+$/) {
          bad=1
          next
        }
        print path "\t" hash
        count++
      }
      END { if (!found || !count || bad) exit 2 }
    ' "$map_file" 2>/dev/null) || { scan_ok=0; break; }
    moved=0
    total=0
    while IFS=$'\t' read -r anchor_path expected_hash; do
      if [ -z "$anchor_path" ] || [[ ! "$expected_hash" =~ ^[0-9a-f]{12}$ ]]; then
        scan_ok=0
        break
      fi
      lookup_path="${anchor_path%/}"
      lookup_path="${lookup_path%:*}"
      total=$((total + 1))
      current_hash=''
      if current_hash=$(git -C "$repo_root" rev-parse --verify -q "HEAD:$lookup_path" 2>/dev/null); then
        [[ "$current_hash" == "$expected_hash"* ]] || moved=$((moved + 1))
      else
        lookup_status=$?
        if [ "$lookup_status" -eq 1 ]; then
          moved=$((moved + 1))
        else
          scan_ok=0
          break
        fi
      fi
    done <<<"$anchor_rows"
    [ "$scan_ok" -eq 1 ] || break
    moved_by_slug["$slug"]="$moved"
    total_by_slug["$slug"]="$total"
  done
  if [ "$scan_ok" -eq 1 ]; then
    annotated_surface=''
    while IFS= read -r index_line; do
      pointer="${index_line##* -> }"
      slug="${pointer#maps/}"
      slug="${slug%.md}"
      moved="${moved_by_slug[$slug]:-0}"
      total="${total_by_slug[$slug]:-0}"
      if [ "$moved" -gt 0 ]; then
        index_line="$index_line ⚠ DRIFTED ($moved/$total anchors moved)"
      fi
      annotated_surface="${annotated_surface}${index_line}"$'\n'
    done <<<"$surface"
    annotated_surface="${annotated_surface%$'\n'}"
  fi
fi

jq -c --arg surface "$annotated_surface" --arg organ "$organ" \
  '{hookSpecificOutput:{hookEventName:"PreToolUse",updatedInput:(.tool_input | .prompt = (
     .prompt + "\n\nCached maps for this repository (bodies under " + $organ +
     "/maps/). Consult a covering map before re-deriving its subject, cite it when used, and re-verify any row marked DRIFTED:\n" + $surface))}}' <<<"$in"
