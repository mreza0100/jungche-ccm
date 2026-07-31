#!/usr/bin/env bash
# cc-archive.sh — move hidden chats out of sight of BOTH engines, reversibly.
#
# WHY: `~/.claude/.cc-ls-hidden` is a text file that every cc-ls rewrites, so two pickers racing
# each other lose each other's hides — that is the "my hidden chats came back" bug. Archiving
# retires the list instead of locking it: a chat that is not on disk cannot come back.
#
# WHAT IT MOVES: only a uuid that is (a) on the hide list, (b) resolvable to a real file, and
# (c) NOT live right now. Liveness is re-checked at run time, never read from a cached list —
# `mv`ing a transcript out from under a running claude corrupts it or spawns a phantom.
#
# REVERSIBLE: MANIFEST.tsv records the original path of every file, so `--restore <uuid>` puts it
# back exactly where it was. Nothing is ever deleted.
#
# IDEMPOTENT: a second run finds nothing left to do and exits 0.
#
# SUBAGENT MODE (--subagents): archives sidechain transcripts instead of hidden chats. These are
# the artifacts every subagent/workflow run leaves behind — 96.7% of the files under projects/ and
# ~83% of the bytes, none of which cc-ls can ever show (they have no user prompt and are not
# resumable). cc-ls still has to stat and head every one just to learn that, so archiving them is
# what actually makes the picker fast. Age-gated: a sidechain file younger than --older-than DAYS
# is left alone, because a running subagent is still appending to it.
#
#   cc-archive.sh                          # dry run — prints the plan, touches nothing (DEFAULT)
#   cc-archive.sh --apply                  # archive hidden chats
#   cc-archive.sh --subagents              # dry run over sidechain transcripts
#   cc-archive.sh --subagents --apply
#   cc-archive.sh --subagents --older-than 7 --apply
#   cc-archive.sh --restore UUID
set -uo pipefail

ARCHIVE="${CC_ARCHIVE_DIR:-$HOME/.claude-archive}"
CL="$HOME/.claude/projects"
CX="$HOME/.codex/sessions"
HID="$HOME/.claude/.cc-ls-hidden"
HIDAT="$HOME/.claude/.cc-ls-hidden.at"
HIST="$HOME/.claude/history.jsonl"
CXIDX="$HOME/.codex/session_index.jsonl"
MANIFEST="$ARCHIVE/MANIFEST.tsv"
STAMP="$(date +%Y%m%d-%H%M%S)"

APPLY=0; RESTORE=""; SUBAGENTS=0; OLDER_THAN=2
while [ $# -gt 0 ]; do
  case "$1" in
    --apply)      APPLY=1 ;;
    --subagents)  SUBAGENTS=1 ;;
    --older-than) OLDER_THAN="${2:?--older-than needs a number of days}"; shift ;;
    --restore)    RESTORE="${2:-}"; shift ;;
    -h|--help)    sed -n '2,28p' "$0"; exit 0 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
  shift
done
case "$OLDER_THAN" in ''|*[!0-9]*) echo "--older-than must be a whole number of days" >&2; exit 2 ;; esac
AGE_MIN=$(( OLDER_THAN * 1440 ))

say() { printf '%s\n' "$*"; }
run() { if [ "$APPLY" = 1 ]; then "$@"; else say "    DRY: $*"; fi; }

# ---------------------------------------------------------------- restore
if [ -n "$RESTORE" ]; then
  [ -f "$MANIFEST" ] || { echo "no manifest at $MANIFEST" >&2; exit 1; }
  line="$(grep -F "$RESTORE" "$MANIFEST" | head -1)" || true
  [ -n "$line" ] || { echo "uuid not in manifest: $RESTORE" >&2; exit 1; }
  orig="$(printf '%s' "$line" | cut -f3)"
  cur="$(printf '%s' "$line" | cut -f5)"
  [ -f "$cur" ] || { echo "archived file missing: $cur" >&2; exit 1; }
  mkdir -p "$(dirname "$orig")"
  mv -n "$cur" "$orig" && say "restored -> $orig"
  exit 0
fi

# ---------------------------------------------------------------- liveness (re-checked NOW)
LIVE="$(mktemp)"; trap 'rm -f "$LIVE"' EXIT
ps -eo args 2>/dev/null \
  | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' \
  | sort -u > "$LIVE"
# a codex chat's uuid never appears in argv — map each live cx-* socket to its pane cwd, then to
# the rollout whose meta line records that cwd (codex v0.146 closes its rollout fd, so /proc is out)
for s in "${TMUX_TMPDIR:-/tmp}/tmux-$(id -u)"/cx-*; do
  [ -S "$s" ] || continue
  tmux -S "$s" list-panes -a -F '#{pane_current_path}' 2>/dev/null | sort -u | while read -r d; do
    [ -n "$d" ] || continue
    grep -ilF "\"cwd\":\"$d\"" "$CX"/rollout-*.jsonl 2>/dev/null | head -1
  done
done | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' >> "$LIVE" || true
sort -u -o "$LIVE" "$LIVE"
say "live uuids (excluded): $(wc -l < "$LIVE")"

[ "$SUBAGENTS" = 1 ] || [ -f "$HID" ] || { echo "no hide list at $HID — nothing to do"; exit 0; }

# ---------------------------------------------------------------- plan
n_arch=0; n_live=0; n_orphan=0; n_young=0; bytes=0
PLAN="$(mktemp)"; DECIDED="$(mktemp)"
trap 'rm -f "$LIVE" "$PLAN" "$DECIDED"' EXIT

if [ "$SUBAGENTS" = 1 ]; then
  # ---- sidechain sweep. The age gate is the liveness guard that matters here: a subagent writes
  # its transcript for as long as it runs, and `find -mmin` is the cheap, exact way to leave those
  # alone. The argv check still runs, but a subagent's uuid rarely appears in any command line.
  say "mode: SUBAGENTS  (sidechain transcripts older than $OLDER_THAN day(s))"
  while IFS= read -r src; do
    # classify from the head only — a sidechain marker appears in the first record
    head -c 4096 "$src" 2>/dev/null | grep -q '"isSidechain":true' || continue
    u="$(basename "$src" .jsonl)"
    if grep -qxF "$u" "$LIVE"; then n_live=$((n_live+1)); continue; fi
    sz="$(stat -c '%s' "$src" 2>/dev/null || echo 0)"
    dst="$ARCHIVE/subagents/$(basename "$(dirname "$src")")/$(basename "$src")"
    printf '%s\t%s\t%s\t%s\t%s\n' "$u" "subagent" "$src" "$sz" "$dst" >> "$PLAN"
    n_arch=$((n_arch+1)); bytes=$((bytes+sz))
  done < <(find -P "$CL" -name '*.jsonl' -mmin "+$AGE_MIN" 2>/dev/null)
  n_young=$(find -P "$CL" -name '*.jsonl' -mmin "-$AGE_MIN" 2>/dev/null | wc -l)
  say ""
  say "PLAN:"
  say "  archive      : $n_arch files  ($(awk -v b=$bytes 'BEGIN{printf "%.1f", b/1048576}') MB)"
  say "  skip (live)  : $n_live"
  say "  skip (younger than $OLDER_THAN day(s)): $n_young"
  say ""
  if [ "$n_arch" = 0 ]; then say "nothing to do"; exit 0; fi
else
# DECIDED collects EVERY uuid this run resolves — archived, orphaned, or skipped-because-live.
# All three leave the hide list: archived chats cannot come back, orphans point at nothing, and a
# live chat is by definition one the operator is still using, so it belongs in the picker, not hidden.
while read -r u; do
  [ -n "$u" ] || continue
  printf '%s\n' "$u" >> "$DECIDED"
  if grep -qxF "$u" "$LIVE"; then n_live=$((n_live+1)); continue; fi
  src="$(find -P "$CL" -name "${u}.jsonl" 2>/dev/null | head -1)"
  eng=claude
  if [ -z "$src" ]; then
    src="$(find -P "$CX" -name "*${u}*.jsonl" 2>/dev/null | head -1)"; eng=codex
  fi
  if [ -z "$src" ]; then n_orphan=$((n_orphan+1)); continue; fi
  sz="$(stat -c '%s' "$src" 2>/dev/null || echo 0)"
  # mirror the source layout under the archive so restore is an exact reverse
  case "$eng" in
    claude) dst="$ARCHIVE/claude/$(basename "$(dirname "$src")")/$(basename "$src")" ;;
    codex)  dst="$ARCHIVE/codex/$(basename "$src")" ;;
  esac
  printf '%s\t%s\t%s\t%s\t%s\n' "$u" "$eng" "$src" "$sz" "$dst" >> "$PLAN"
  n_arch=$((n_arch+1)); bytes=$((bytes+sz))
done < "$HID"

say ""
say "PLAN:"
say "  archive      : $n_arch files  ($(awk -v b=$bytes 'BEGIN{printf "%.1f", b/1048576}') MB)"
say "  skip (live)  : $n_live"
say "  orphan (prune only, no file): $n_orphan"
say "  hide-list entries remaining after: $(( $(grep -c . "$HID") - n_arch - n_live - n_orphan ))"
say ""

if [ "$n_arch" = 0 ] && [ "$n_orphan" = 0 ]; then say "nothing to do"; exit 0; fi
fi   # end hidden-chat plan branch

# ---------------------------------------------------------------- backups (always, before writes)
# subagent mode touches no sidecar, so it needs no sidecar backup
if [ "$APPLY" = 1 ] && [ "$SUBAGENTS" = 0 ]; then
  mkdir -p "$ARCHIVE/_sidecar-backups"
  for f in "$HID" "$HIDAT" "$HIST" "$CXIDX"; do
    [ -f "$f" ] && cp -a "$f" "$ARCHIVE/_sidecar-backups/$(basename "$f").$STAMP"
  done
  say "sidecars backed up -> $ARCHIVE/_sidecar-backups/*.$STAMP"
fi

# ---------------------------------------------------------------- move
[ "$APPLY" = 1 ] && { mkdir -p "$ARCHIVE/claude" "$ARCHIVE/codex" "$ARCHIVE/subagents"; \
  [ -f "$MANIFEST" ] || printf 'uuid\tengine\torig_path\tbytes\tarchived_path\tarchived_at\n' > "$MANIFEST"; }
while IFS=$'\t' read -r u eng src sz dst; do
  if [ "$APPLY" = 1 ]; then
    mkdir -p "$(dirname "$dst")"
    if mv -n "$src" "$dst" 2>/dev/null; then
      printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$u" "$eng" "$src" "$sz" "$dst" "$STAMP" >> "$MANIFEST"
    else
      say "  WARN could not move $src"
    fi
  else
    say "    DRY mv $src -> $dst"
  fi
done < "$PLAN"

# ---------------------------------------------------------------- prune sidecars
# Hidden-chat mode only. A sidechain transcript is not a chat: it has no hide-list entry, no
# history.jsonl prompts (the operator never typed into it) and no codex index row, so subagent
# mode has nothing to prune and must not rewrite those files.
if [ "$APPLY" = 1 ] && [ "$SUBAGENTS" = 1 ]; then
  say ""
  say "done. manifest: $MANIFEST"
  say "restore any one with: $0 --restore <uuid>"
elif [ "$APPLY" = 1 ]; then
  # .pruned = ARCHIVED only — history/codex-index rows for a live chat must survive, it still exists
  cut -f1 "$PLAN" | sort -u > "$ARCHIVE/.pruned.$STAMP"
  # the hide list loses every decided uuid, live ones included (they go back to being visible)
  sort -u "$DECIDED" > "$ARCHIVE/.unhidden.$STAMP"
  awk 'NR==FNR{d[$0];next} !($0 in d)' "$ARCHIVE/.unhidden.$STAMP" "$HID" > "$HID.new" && mv "$HID.new" "$HID"
  [ -f "$HIDAT" ] && { awk 'NR==FNR{d[$0];next} !($1 in d)' "$ARCHIVE/.unhidden.$STAMP" "$HIDAT" > "$HIDAT.new" && mv "$HIDAT.new" "$HIDAT"; }
  say "hide list: $(grep -c . "$HID") entries remain; $n_live live chat(s) returned to the picker"
  # history.jsonl: drop prompt entries belonging to archived sessions
  if [ -f "$HIST" ] && [ -s "$ARCHIVE/.pruned.$STAMP" ]; then
    before=$(grep -c . "$HIST")
    grep -vF -f "$ARCHIVE/.pruned.$STAMP" "$HIST" > "$HIST.new" && mv "$HIST.new" "$HIST"
    say "history.jsonl: $before -> $(grep -c . "$HIST") lines"
  fi
  # codex index: drop rows pointing at rollouts we moved
  if [ -f "$CXIDX" ] && [ -s "$ARCHIVE/.pruned.$STAMP" ]; then
    before=$(grep -c . "$CXIDX")
    grep -vF -f "$ARCHIVE/.pruned.$STAMP" "$CXIDX" > "$CXIDX.new" && mv "$CXIDX.new" "$CXIDX"
    say "codex session_index.jsonl: $before -> $(grep -c . "$CXIDX") rows"
  fi
  say ""
  say "done. manifest: $MANIFEST"
  say "restore any one with: $0 --restore <uuid>"
else
  say ""
  say "DRY RUN — nothing moved. re-run with --apply"
fi
