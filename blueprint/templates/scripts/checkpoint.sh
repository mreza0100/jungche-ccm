#!/usr/bin/env bash
# checkpoint.sh — per-worktree audit trail at <worktree>/.checkpoint.json:
# a traceable, append-only log of which agent did what, when, at which git HEAD,
# touching which files. File-backed JSON; macOS-safe mkdir lock for concurrent
# appends. gitter inits it at SETUP and archives it to the pipeline docs at MERGE
# (the worktree is torn down, the audit trail survives in $DOCS/audit-trail.json).
set -euo pipefail

cmd="${1:-}"
wt="${2:-}"
[ -z "$cmd" ] || [ -z "$wt" ] && {
  echo "usage: checkpoint.sh {init|log|archive} <worktree> [args]" >&2
  exit 2
}

CP="$wt/.checkpoint.json"
LOCK="$wt/.checkpoint.lock"

acquire() {
  local waited=0
  while ! mkdir "$LOCK" 2>/dev/null; do
    waited=$((waited + 1))
    [ "$waited" -ge 30 ] && {
      echo "checkpoint: lock busy on $wt" >&2
      exit 1
    }
    sleep 1
  done
  trap 'rmdir "$LOCK" 2>/dev/null || true' EXIT
}

case "$cmd" in
init)
  pipeline="${3:-unknown}"
  session="${4:-unknown}"
  acquire
  python3 - "$CP" "$pipeline" "$session" <<'PY'
import json, sys, datetime
cp, pipeline, session = sys.argv[1:4]
json.dump({"pipeline": pipeline, "session": session,
           "created": datetime.datetime.now().isoformat(timespec="seconds"),
           "events": []},
          open(cp, "w"), indent=2)
PY
  echo "checkpoint: initialized $CP"
  ;;
log)
  phase="${3:-unknown}"
  actor="${4:-orchestrator}"
  note="${5:-}"
  [ -f "$CP" ] || {
    echo "checkpoint: no $CP — run init first" >&2
    exit 1
  }
  head=$(git -C "$wt" rev-parse --short HEAD 2>/dev/null || echo none)
  files=$(git -C "$wt" status --porcelain 2>/dev/null | awk '{print $NF}' | head -50 | paste -sd, - || true)
  acquire
  python3 - "$CP" "$phase" "$actor" "$note" "$head" "$files" <<'PY'
import json, sys, datetime
cp, phase, actor, note, head, files = sys.argv[1:7]
data = json.load(open(cp))
data["events"].append({
    "ts": datetime.datetime.now().isoformat(timespec="seconds"),
    "phase": phase, "actor": actor, "note": note, "head": head,
    "files": [f for f in files.split(",") if f]})
json.dump(data, open(cp, "w"), indent=2)
PY
  echo "checkpoint: logged $phase / $actor"
  ;;
archive)
  dest="${3:-}"
  [ -z "$dest" ] && {
    echo "usage: checkpoint.sh archive <worktree> <dest-file>" >&2
    exit 2
  }
  [ -f "$CP" ] || {
    echo "checkpoint: no $CP to archive"
    exit 0
  }
  cp "$CP" "$dest"
  echo "checkpoint: archived $CP -> $dest"
  ;;
*)
  echo "usage: checkpoint.sh {init|log|archive} <worktree> [args]" >&2
  exit 2
  ;;
esac
