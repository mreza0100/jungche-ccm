#!/usr/bin/env bash
# cx-recover.sh — rebuild a Codex thread's conversation from its rollout.
#
# Codex resumes a `history_mode = paginated` thread from its history store, not
# from the rollout on disk. When that store cannot supply the thread, `codex
# resume <id>` opens a seat with no memory while the rollout beside it is whole:
# every instrument reports a healthy session and the pane is empty. The rollout
# is the durable record, so recovery is a parse, not an investigation — this
# turns it into one command.
#
#   cx-recover.sh <thread-id|rollout-path>
#
# Writes, under ~/.codex/recovered-<id>/ :
#   transcript.md         every user/assistant message, in order
#   compaction-memory.md  the messages carried through the thread's compactions
#   brief.md              a re-brief for a replacement seat: what happened,
#                         where its memory is, and the last exchanges verbatim
#
# Idempotent: re-running overwrites the three files from the rollout, which is
# append-only, so a later run is a superset of an earlier one.
set -euo pipefail

CODEX_HOME="${CODEX_HOME:-$HOME/.codex}"

die() { printf '%s\n' "cx-recover: $*" >&2; exit 1; }

[ $# -eq 1 ] || die "usage: cx-recover.sh <thread-id|rollout-path>"
target="$1"

if [ -f "$target" ]; then
  rollout="$target"
else
  # Newest rollout naming the thread. A resumed thread appends in place, so the
  # newest file carrying the id is the complete one.
  rollout="$(find "$CODEX_HOME/sessions" -name "*${target}*.jsonl" -type f \
    -printf '%T@ %p\n' 2>/dev/null | sort -rn | head -1 | cut -d' ' -f2-)"
  [ -n "$rollout" ] || die "no rollout found for thread '$target' under $CODEX_HOME/sessions"
fi
[ -r "$rollout" ] || die "cannot read rollout: $rollout"

thread="$(basename "$rollout" .jsonl)"
thread="${thread##*-rollout-}"
thread="$(printf '%s\n' "$thread" | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -1)"
[ -n "$thread" ] || thread="$target"

out="$CODEX_HOME/recovered-$thread"
mkdir -p "$out"

python3 - "$rollout" "$out" "$thread" <<'EXTRACT'
import json, os, sys

rollout, out, thread = sys.argv[1], sys.argv[2], sys.argv[3]

def text_of(payload):
    parts = []
    for chunk in payload.get('content') or []:
        value = chunk.get('text')
        if isinstance(value, str) and value.strip():
            parts.append(value)
    return '\n'.join(parts).strip()

# Codex re-sends these every turn; they are configuration, not conversation.
BOILERPLATE = ('# AGENTS.md instructions', '<permissions instructions>')

turns, carried, malformed = [], [], 0
with open(rollout, encoding='utf-8', errors='replace') as handle:
    for raw in handle:
        try:
            record = json.loads(raw)
        except ValueError:
            malformed += 1
            continue
        kind = record.get('type')
        payload = record.get('payload') or {}
        stamp = record.get('timestamp', '')
        if kind == 'compacted':
            for item in payload.get('replacement_history') or []:
                body = text_of(item)
                if body and not body.startswith(BOILERPLATE):
                    carried.append((stamp, item.get('role', '?'), body))
            continue
        if kind != 'response_item' or payload.get('type') != 'message':
            continue
        if payload.get('role') not in ('user', 'assistant'):
            continue
        body = text_of(payload)
        if body and not body.startswith(BOILERPLATE):
            turns.append((stamp, payload['role'], body))

def dump(name, header, rows):
    with open(os.path.join(out, name), 'w', encoding='utf-8') as handle:
        handle.write(header)
        for stamp, role, body in rows:
            handle.write(f'## {role} · {stamp}\n\n{body}\n\n')

dump(
    'transcript.md',
    f'# Recovered conversation — thread {thread}\n\n'
    f'{len(turns)} user/assistant messages, parsed from {rollout}\n\n',
    turns,
)
dump(
    'compaction-memory.md',
    f'# What survived compaction — thread {thread}\n\n'
    f'{len(carried)} messages carried through this thread\'s compactions.\n\n',
    carried,
)

tail = turns[-12:]
with open(os.path.join(out, 'brief.md'), 'w', encoding='utf-8') as handle:
    handle.write(f"""# Recovery brief — thread {thread}

You are the seat for thread `{thread}`. Codex brought you up with no memory: this thread's
history store could not supply it, so `codex resume` opened an empty session. Nothing was lost —
your rollout is complete and your conversation is reconstructed below.

| File | Holds |
|------|-------|
| `{out}/compaction-memory.md` | {len(carried)} messages carried through your compactions — your condensed long-term state |
| `{out}/transcript.md` | all {len(turns)} user/assistant messages, in order |
| `{rollout}` | the raw rollout this was parsed from |

Read `compaction-memory.md` first, then the tail of `transcript.md` for the immediate position.

A pane's emptiness is not evidence: the status line's context/token counts are what decide
whether a seat is warm.

---

## The last {len(tail)} exchanges before the thread went dark

""")
    for stamp, role, body in tail:
        handle.write(f'### {role} · {stamp}\n\n{body}\n\n')

print(f'messages={len(turns)} carried={len(carried)} malformed={malformed}')
EXTRACT

printf '%s\n' \
  "recovered thread $thread" \
  "  $out/brief.md" \
  "  $out/compaction-memory.md" \
  "  $out/transcript.md" \
  "" \
  "Brief a replacement seat with:" \
  "  chat.sh inject <socket> 'RECOVERY: read $out/brief.md, then compaction-memory.md, then the tail of transcript.md.'"
