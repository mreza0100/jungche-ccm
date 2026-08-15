#!/usr/bin/env bash
# cc-db.sh — the fleet's single state store. One SQLite database at ~/.cc/fleet.db replaces the
# seven sidecar files cc-ls and friends used to scatter across ~/.claude.
#
# WHY A DB AND NOT A BETTER FILE: the hide list was read-modify-write. Two pickers running at once
# each read 222 uuids, each wrote back their own edit, and the second write erased the first — that
# is the "my hidden chats came back" bug. A transaction makes that arithmetic impossible. WAL mode
# also retires the hand-rolled .cc-ls-hidden.lock, which never actually prevented the race.
#
# WHAT LIVES HERE: only state the FLEET owns. Transcripts, codex rollouts, history.jsonl,
# session_index.jsonl and .claude.json stay exactly where they are — Claude and Codex read those
# files themselves, and a database they cannot see would blind them.
#
# CALLED FROM BOTH SHELLS: this is a bash executable, not a zsh library, so bash satellites and
# the zsh surface can both reach it (a .zsh library could only be sourced by one of them).
#
# FALLBACK: no sqlite3, or an unopenable db, means every read falls back to the legacy file and
# every write appends to it. The picker must never be down because a database is unhappy.
#
#   cc-db.sh init                      # create schema (idempotent)
#   cc-db.sh migrate                   # import the legacy sidecars, renaming them aside
#   cc-db.sh stats                     # row counts + disk counts, the proof
#   cc-db.sh hidden-list               # one uuid per line
#   cc-db.sh hidden-add UUID [PAYLOAD]
#   cc-db.sh hidden-del UUID
#   cc-db.sh hidden-has UUID           # exit 0 if hidden
#   cc-db.sh primary-get | primary-set N
#   cc-db.sh child-add new|pane KEY VAL
#   cc-db.sh child-list new|pane KEY
#   cc-db.sh swap-log LINE
set -uo pipefail

# CC_FLEET_HOME — this bundle's own directory, resolved THROUGH symlinks, because install.sh
# links this script into ~/.claude/bin and $BASH_SOURCE is then the link. Plain `readlink`
# (never -f) because macOS ships BSD readlink.
_ccfs="${BASH_SOURCE[0]}"; while [ -L "$_ccfs" ]; do _ccfd="$(cd -P "$(dirname "$_ccfs")" && pwd)"; _ccfs="$(readlink "$_ccfs")"; case "$_ccfs" in /*) ;; *) _ccfs="$_ccfd/$_ccfs" ;; esac; done
CC_FLEET_HOME="${CC_FLEET_HOME:-$(cd -P "$(dirname "$_ccfs")" && pwd)}"
. "$CC_FLEET_HOME/cc-portable.sh"   # GNU/BSD seam


DB="${CC_FLEET_DB:-$HOME/.cc/fleet.db}"
LEG_HID="$HOME/.claude/.cc-ls-hidden"
LEG_AT="$HOME/.claude/.cc-ls-hidden.at"
LEG_PRIM="$HOME/.claude-primary"
LEG_NEWC="$HOME/.claude/.cc-new-children"
LEG_PANEC="$HOME/.claude/.cc-pane-children"
LEG_SWAP="$HOME/.claude/.cc-swap.log"
STAMP="$(date +%Y%m%d-%H%M%S)"
NOW="$(date +%s)"

have_db() { command -v sqlite3 >/dev/null 2>&1 && [ -f "$DB" ]; }

# Every statement goes through here: WAL + a busy timeout so concurrent chats queue rather than fail.
# Use the `.timeout` DOT-COMMAND, never `PRAGMA busy_timeout=5000` — the pragma RETURNS A ROW, and
# via -cmd that value is printed ahead of the real result, silently corrupting every count and
# every scalar read. `.timeout` sets the same thing and emits nothing.
_q() { sqlite3 -batch -noheader -cmd '.timeout 5000' "$DB" "$@"; }

# Single-quote escaping for SQL literals. Two rules, both learned by measurement:
#   1. never `| sed` — that cost 493 process spawns per chat-save, 67% of everything cc-ls forked.
#   2. never call it as `$(_esc "$x")` — command substitution forks a SUBSHELL even when the body
#      is pure builtins. `strace -e execve` cannot see those forks, so the exec count looked fixed
#      while chat-save still burned 1.9s on ~650 hidden forks. Assign to a global and read it.
# _esc sets $_E. Keep it that way; the printf form below exists only for one-off callers.
_esc_v() { _E="${1//\'/\'\'}"; }
_esc()   { local s="${1//\'/\'\'}"; printf '%s' "$s"; }

cmd_init() {
  command -v sqlite3 >/dev/null 2>&1 || { echo "sqlite3 not installed" >&2; return 1; }
  mkdir -p "$(dirname "$DB")"
  # journal_mode returns the new mode as a row — discard it, callers parse this command's stdout
  sqlite3 -batch "$DB" >/dev/null <<'SQL'
PRAGMA journal_mode=WAL;
CREATE TABLE IF NOT EXISTS meta(key TEXT PRIMARY KEY, val TEXT NOT NULL, updated_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS hidden(
  uuid TEXT PRIMARY KEY,
  hidden_at INTEGER NOT NULL,
  at_payload INTEGER
);
CREATE TABLE IF NOT EXISTS children(
  kind TEXT NOT NULL CHECK(kind IN ('new','pane')),
  key  TEXT NOT NULL,
  val  TEXT,
  created_at INTEGER NOT NULL,
  PRIMARY KEY(kind, key, val)
);
CREATE TABLE IF NOT EXISTS swap_event(id INTEGER PRIMARY KEY, at INTEGER NOT NULL, line TEXT NOT NULL);
-- chat — the persistent name cache. Mirrors cc-ls's in-memory NC record exactly
-- (mtime, bytes, cwd, label, prompts) because it REPLACES /tmp/cc-sid/.namecache.v5.
-- That file lived in /tmp, so every reboot wiped it and cc-ls cold-parsed every transcript
-- on the next run; here it survives. 100% derived — delete the row, the next scan rebuilds it.
CREATE TABLE IF NOT EXISTS chat(
  uuid    TEXT PRIMARY KEY,
  mtime   INTEGER NOT NULL DEFAULT 0,
  bytes   INTEGER NOT NULL DEFAULT 0,
  cwd     TEXT,
  label   TEXT,
  prompts INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS chat_mtime ON chat(mtime);
INSERT OR IGNORE INTO meta(key,val,updated_at) VALUES('schema_version','1',strftime('%s','now'));
-- chat_gen (see CHAT_GEN below): stamp it ONLY on a db with no chat rows yet. A fresh install
-- has nothing to repair and should not pay a purge; a db that already holds rows predates this
-- marker, may hold rows a shipped bug wrote, and must be left unstamped so the next chat-load
-- drops them. Which is why this is a conditional SELECT and not a plain VALUES: `migrate` calls
-- init, and an adopter re-running migrate would otherwise stamp the poison as current.
INSERT OR IGNORE INTO meta(key,val,updated_at)
  SELECT 'chat_gen','2',strftime('%s','now') WHERE NOT EXISTS(SELECT 1 FROM chat);
SQL
}

# ---------------------------------------------------------------- hidden
cmd_hidden_list() {
  if have_db; then _q "SELECT uuid FROM hidden ORDER BY hidden_at DESC;"
  else [ -f "$LEG_HID" ] && grep . "$LEG_HID" || true; fi
}
# $LEG_HID is not a legacy leftover: it is the hidden set as it stands between cc-ls runs. The
# picker is the last thing cc-ls does, so a ⌃X toggle can only be carried in that file and
# committed by the next run — which means the next run READS it, and a hide that reached only the
# db would be deleted by that run's sync. So every writer of the db writes the file too, here, in
# one place. Locked against the picker's own read-modify-writes; flock is absent on macOS, where
# a plain append is still atomic for a line this short.
_leg_add() {
  mkdir -p "$(dirname "$LEG_HID")" 2>/dev/null
  ( flock 9 2>/dev/null || true
    grep -qxF -- "$1" "$LEG_HID" 2>/dev/null || printf '%s\n' "$1" >> "$LEG_HID"
  ) 9>"$LEG_HID.lock"
}
_leg_del() {
  [ -f "$LEG_HID" ] || return 0
  ( flock 9 2>/dev/null || true
    grep -vxF -- "$1" "$LEG_HID" > "$LEG_HID.n.$$" 2>/dev/null
    [ "$?" -le 1 ] && mv "$LEG_HID.n.$$" "$LEG_HID" || rm -f "$LEG_HID.n.$$"
  ) 9>"$LEG_HID.lock"
}
cmd_hidden_add() {
  local u="${1:?uuid}" p="${2:-}"
  # COALESCE, not a bare assignment: a second add for the same chat must not blank a payload an
  # earlier one supplied, whichever order the two land in.
  have_db && _q "INSERT INTO hidden(uuid,hidden_at,at_payload) VALUES('$(_esc "$u")',$NOW,$( [ -n "$p" ] && printf '%s' "$p" || printf 'NULL' ))
        ON CONFLICT(uuid) DO UPDATE SET
          hidden_at=$NOW,
          at_payload=COALESCE(excluded.at_payload, hidden.at_payload);"
  _leg_add "$u"
}
cmd_hidden_del() {
  local u="${1:?uuid}"
  have_db && _q "DELETE FROM hidden WHERE uuid='$(_esc "$u")';"
  _leg_del "$u"
}
cmd_hidden_has() {
  local u="${1:?uuid}"
  if have_db; then [ -n "$(_q "SELECT 1 FROM hidden WHERE uuid='$(_esc "$u")';")" ]
  else grep -qxF "$u" "$LEG_HID" 2>/dev/null; fi
}
# Prune the hidden set to the uuids still on disk, as ONE transaction — pass the live set on stdin.
# Housekeeping for hides whose transcript is gone; hidden-sync is what cc-ls uses.
cmd_hidden_reap() {
  have_db || { echo "reap needs the db" >&2; return 1; }
  local tmp; tmp="$(mktemp)"; cat > "$tmp"
  { echo "BEGIN IMMEDIATE;"
    echo "CREATE TEMP TABLE keep(uuid TEXT PRIMARY KEY);"
    while read -r u; do [ -n "$u" ] || continue; _esc_v "$u"; echo "INSERT OR IGNORE INTO keep VALUES('$_E');"; done < "$tmp"
    echo "DELETE FROM hidden WHERE uuid NOT IN (SELECT uuid FROM keep);"
    echo "COMMIT;"
  } | sqlite3 -batch -cmd '.timeout 5000' "$DB"
  rm -f "$tmp"
}

# uuid<TAB>payload for every hidden chat carrying a size baseline
cmd_hidden_at_list() {
  # options must precede the db filename — sqlite3 reads anything after it as SQL
  if have_db; then sqlite3 -batch -noheader -separator "$(printf '\t')" -cmd '.timeout 5000' "$DB" \
      "SELECT uuid, at_payload FROM hidden WHERE at_payload IS NOT NULL;"
  else [ -f "$LEG_AT" ] && grep . "$LEG_AT" || true; fi
}

# hidden-sync — replaces cc-ls's entire read-modify-write block in ONE transaction. Reads
# uuid<TAB>payload lines on stdin: that set becomes the hidden set exactly, payloads included.
# The old version rewrote .cc-ls-hidden and .cc-ls-hidden.at under a flock whose own comment
# conceded "last-writer-wins" — which is precisely how a concurrent picker's hides vanished.
cmd_hidden_sync() {
  have_db || { # fallback: legacy two-file rewrite, unchanged semantics
    local t; t="$(mktemp)"; cat > "$t"
    cut -f1 "$t" | grep . > "$LEG_HID" 2>/dev/null || : > "$LEG_HID"
    awk -F'\t' 'NF>1 && $2!=""' "$t" > "$LEG_AT" 2>/dev/null || : > "$LEG_AT"
    rm -f "$t"; return 0; }
  local t; t="$(mktemp)"; cat > "$t"
  { echo "BEGIN IMMEDIATE;"
    echo "CREATE TEMP TABLE want(uuid TEXT PRIMARY KEY, payload INTEGER);"
    while IFS="$(printf '\t')" read -r u p; do
      [ -n "$u" ] || continue
      case "$p" in ''|*[!0-9]*) p=NULL ;; esac
      _esc_v "$u"; echo "INSERT OR REPLACE INTO want VALUES('$_E',$p);"
    done < "$t"
    echo "DELETE FROM hidden WHERE uuid NOT IN (SELECT uuid FROM want);"
    # WHERE true is load-bearing, not decoration. On an INSERT..SELECT the parser cannot tell an
    # upsert's ON CONFLICT from a join's ON, so SQLite requires a WHERE clause to end the SELECT
    # first; without it the statement is a syntax error, the -batch run aborts mid-transaction,
    # and the whole sync rolls back. cc-ls pipes here with 2>/dev/null, so it failed in silence.
    echo "INSERT INTO hidden(uuid,hidden_at,at_payload)
            SELECT uuid,$NOW,payload FROM want WHERE true
          ON CONFLICT(uuid) DO UPDATE SET at_payload=excluded.at_payload;"
    echo "COMMIT;"
  } | sqlite3 -batch -cmd '.timeout 5000' "$DB"
  rm -f "$t"
}

# ---------------------------------------------------------------- chat name cache
# Wire format is cc-ls's own NC record: uuid<TAB>mtime<TAB>bytes<TAB>cwd<TAB>label<TAB>prompts.
# Keeping the shape identical means the picker needs a one-line change at each end, not a rewrite.
CHATCACHE="${CC_CHATCACHE:-/tmp/cc-sid/.namecache.v5}"

# CHAT_GEN — the derived-cache generation. BUMP IT whenever a shipped bug could have written
# WRONG rows here, and the poison clears itself on every host at the next picker run.
#
# This exists because a cache that outlives the bug that filled it is worse than no cache. A
# macOS mtime-parse fault (see cc_find_meta) shifted every field of the rows it wrote: cwd became
# a path prefix, label became the byte count, and prompts became 0 — and a 0-prompt chat is one
# the picker HIDES. Fixing the parse did not bring those chats back, because the next run found a
# cache entry whose mtime and size still matched the file and trusted it. The table is 100%
# derived, so the honest repair is to drop it once and let the scan rebuild.
#
# 1 → the pre-marker state (any db written before this check existed)
# 2 → after the macOS mtime-parse fix
CHAT_GEN=2
_chat_gen_ok() {
  [ "$(_q "SELECT COALESCE((SELECT val FROM meta WHERE key='chat_gen'),'1');" 2>/dev/null)" = "$CHAT_GEN" ] && return 0
  _q "DELETE FROM chat;
      INSERT INTO meta(key,val,updated_at) VALUES('chat_gen','$CHAT_GEN',strftime('%s','now'))
        ON CONFLICT(key) DO UPDATE SET val=excluded.val, updated_at=excluded.updated_at;" >/dev/null 2>&1
}
cmd_chat_load() {
  if have_db; then
    _chat_gen_ok
    sqlite3 -batch -noheader -separator "$(printf '\t')" -cmd '.timeout 5000' "$DB" \
      "SELECT uuid,mtime,bytes,COALESCE(cwd,''),COALESCE(label,''),prompts FROM chat;"
  else [ -f "$CHATCACHE" ] && cat "$CHATCACHE" || true; fi
}
cmd_chat_save() {
  have_db || { mkdir -p "$(dirname "$CHATCACHE")" 2>/dev/null; cat > "$CHATCACHE"; return 0; }
  local t; t="$(mktemp)"; cat > "$t"
  { echo "BEGIN IMMEDIATE;"
    # Split by hand, never `IFS=<tab> read`. Tab is an IFS *whitespace* character in bash, so
    # consecutive tabs collapse into one delimiter and EMPTY fields disappear — a row with no
    # label ("…\t<cwd>\t\t<count>") came back with the count shifted into the label. Manual
    # %%/# splitting preserves every empty field exactly.
    local line rest u mt sz cwd lbl ct
    while IFS= read -r line; do
      u="${line%%$'\t'*}";   rest="${line#*$'\t'}"
      mt="${rest%%$'\t'*}";  rest="${rest#*$'\t'}"
      sz="${rest%%$'\t'*}";  rest="${rest#*$'\t'}"
      cwd="${rest%%$'\t'*}"; rest="${rest#*$'\t'}"
      lbl="${rest%%$'\t'*}"; ct="${rest#*$'\t'}"
      [ -n "$u" ] || continue
      # A tab still in the LAST field means the record carried more than six, which only happens
      # when a caller packed a tab-bearing value into one of them. Refuse it rather than store the
      # shifted row: that is exactly how a bad mtime turned into cwd="$HOME/", label=<bytes>
      # and prompts=0, and a 0-prompt row makes the picker hide a chat that is perfectly fine.
      # Dropping it costs one re-parse; keeping it costs the chat. `$'\t'` and not a command
      # substitution — see the note on _esc_v: a fork per row is the cost this function exists
      # to avoid.
      case "$ct" in *$'\t'*) continue ;; esac
      case "$mt" in ''|*[!0-9]*) mt=0 ;; esac
      case "$sz" in ''|*[!0-9]*) sz=0 ;; esac
      case "$ct" in ''|*[!0-9]*) ct=0 ;; esac
      # $_E, not $(_esc …) — see the note on _esc_v: a command substitution per field forked a
      # subshell per field, ~650 per save, and was half of cc-ls's warm time.
      _esc_v "$u";   local eu="$_E"
      _esc_v "$cwd"; local ec="$_E"
      _esc_v "$lbl"; local el="$_E"
      echo "INSERT INTO chat(uuid,mtime,bytes,cwd,label,prompts)
              VALUES('$eu',$mt,$sz,'$ec','$el',$ct)
            ON CONFLICT(uuid) DO UPDATE SET
              mtime=excluded.mtime, bytes=excluded.bytes, cwd=excluded.cwd,
              label=excluded.label, prompts=excluded.prompts;"
    done < "$t"
    echo "COMMIT;"
  } | sqlite3 -batch -cmd '.timeout 5000' "$DB"
  rm -f "$t"
}
# drop rows whose transcript no longer exists (archived or deleted)
cmd_chat_prune() {
  have_db || return 0
  local t; t="$(mktemp)"
  # basename-only, without GNU find's -printf '%f': strip the directory with sed, which every
  # find can feed. The old form emitted nothing at all on BSD, silently pruning the whole index.
  find -P "$HOME/.claude/projects" -name '*.jsonl' 2>/dev/null | sed 's|.*/||; s/\.jsonl$//' > "$t"
  { echo "BEGIN IMMEDIATE;"
    echo "CREATE TEMP TABLE live(uuid TEXT PRIMARY KEY);"
    while read -r u; do [ -n "$u" ] || continue; _esc_v "$u"; echo "INSERT OR IGNORE INTO live VALUES('$_E');"; done < "$t"
    echo "DELETE FROM chat WHERE uuid NOT IN (SELECT uuid FROM live);"
    echo "COMMIT;"
  } | sqlite3 -batch -cmd '.timeout 5000' "$DB"
  rm -f "$t"
}

# ---------------------------------------------------------------- primary account
cmd_primary_get() {
  local n=""
  have_db && n="$(_q "SELECT val FROM meta WHERE key='primary_account';")"
  [ -n "$n" ] || n="$(cat "$LEG_PRIM" 2>/dev/null)"
  case "$n" in 1|2) printf '%s\n' "$n" ;; *) printf '1\n' ;; esac
}
cmd_primary_set() {
  local n="${1:?account}"
  case "$n" in 1|2) ;; *) echo "account must be 1-2" >&2; return 2 ;; esac
  have_db && _q "INSERT INTO meta(key,val,updated_at) VALUES('primary_account','$n',$NOW)
                 ON CONFLICT(key) DO UPDATE SET val='$n',updated_at=$NOW;"
  # keep the legacy file in lockstep: the statusline and other readers still consult it
  printf '%s\n' "$n" > "$LEG_PRIM"
}

# ---------------------------------------------------------------- children
cmd_child_add() {
  local k="${1:?kind}" key="${2:?key}" val="${3:-}"
  if have_db; then
    _q "INSERT OR IGNORE INTO children(kind,key,val,created_at) VALUES('$(_esc "$k")','$(_esc "$key")','$(_esc "$val")',$NOW);"
  else
    local d; case "$k" in new) d="$LEG_NEWC" ;; pane) d="$LEG_PANEC" ;; *) return 2 ;; esac
    mkdir -p "$d" && printf '%s\n' "$val" >> "$d/$key"
  fi
}
cmd_child_list() {
  local k="${1:?kind}" key="${2:?key}"
  if have_db; then _q "SELECT val FROM children WHERE kind='$(_esc "$k")' AND key='$(_esc "$key")';"
  else
    local d; case "$k" in new) d="$LEG_NEWC" ;; pane) d="$LEG_PANEC" ;; *) return 2 ;; esac
    [ -f "$d/$key" ] && cat "$d/$key" || true
  fi
}

cmd_child_clear() {
  local k="${1:?kind}" key="${2:?key}"
  if have_db; then _q "DELETE FROM children WHERE kind='$(_esc "$k")' AND key='$(_esc "$key")';"
  else
    local d; case "$k" in new) d="$LEG_NEWC" ;; pane) d="$LEG_PANEC" ;; *) return 2 ;; esac
    rm -f "$d/$key"
  fi
}

# ---------------------------------------------------------------- swap log
# With args, logs one line. With none, drains stdin — one transaction for a whole swap report,
# which is how cc-swap-chat.sh emits it.
cmd_swap_log() {
  if [ $# -gt 0 ]; then
    local line="$*"
    if have_db; then _q "INSERT INTO swap_event(at,line) VALUES($NOW,'$(_esc "$line")');"
    else printf '%s\n' "$line" >> "$LEG_SWAP"; fi
    return 0
  fi
  if have_db; then
    { echo "BEGIN IMMEDIATE;"
      while IFS= read -r l; do [ -n "$l" ] || continue; _esc_v "$l"; echo "INSERT INTO swap_event(at,line) VALUES($NOW,'$_E');"; done
      echo "COMMIT;"
    } | sqlite3 -batch -cmd '.timeout 5000' "$DB"
  else cat >> "$LEG_SWAP"; fi
}

# ---------------------------------------------------------------- migrate
# Idempotent. Imports each legacy store, then renames the original aside — never deletes, so
# rollback is renaming it back. A second run finds the originals gone and does nothing.
cmd_migrate() {
  cmd_init || return 1
  local moved=0
  if [ -f "$LEG_HID" ]; then
    local n=0
    while read -r u; do
      [ -n "$u" ] || continue
      local pay; pay="$(awk -v k="$u" '$1==k{print $2; exit}' "$LEG_AT" 2>/dev/null)"
      cmd_hidden_add "$u" "$pay"; n=$((n+1))
    done < "$LEG_HID"
    echo "  hidden      : $n imported"
    mv "$LEG_HID" "$LEG_HID.pre-db-$STAMP"; moved=$((moved+1))
    [ -f "$LEG_AT" ] && { mv "$LEG_AT" "$LEG_AT.pre-db-$STAMP"; moved=$((moved+1)); }
    [ -f "$HOME/.claude/.cc-ls-hidden.lock" ] && rm -f "$HOME/.claude/.cc-ls-hidden.lock"
  else echo "  hidden      : already migrated"; fi

  if [ -f "$LEG_PRIM" ]; then
    cmd_primary_set "$(cat "$LEG_PRIM" 2>/dev/null || echo 1)" >/dev/null
    echo "  primary     : $(cmd_primary_get)"   # legacy file intentionally kept in lockstep
  fi

  local c=0
  for pair in "new:$LEG_NEWC" "pane:$LEG_PANEC"; do
    local kind="${pair%%:*}" dir="${pair#*:}"
    [ -d "$dir" ] || continue
    for f in "$dir"/*; do
      [ -f "$f" ] || continue
      while read -r v; do [ -n "$v" ] && { cmd_child_add "$kind" "$(basename "$f")" "$v"; c=$((c+1)); }; done < "$f"
    done
    mv "$dir" "$dir.pre-db-$STAMP" 2>/dev/null && moved=$((moved+1))
  done
  echo "  children    : $c imported"

  if [ -f "$LEG_SWAP" ]; then
    local s=0
    while IFS= read -r l; do [ -n "$l" ] && { cmd_swap_log "$l"; s=$((s+1)); }; done < "$LEG_SWAP"
    echo "  swap events : $s imported"
    mv "$LEG_SWAP" "$LEG_SWAP.pre-db-$STAMP"; moved=$((moved+1))
  fi
  echo "  legacy files renamed aside: $moved  (rollback = rename back)"
}

cmd_stats() {
  have_db || { echo "no db at $DB"; return 1; }
  echo "db: $DB  ($(du -h "$DB" | cut -f1))"
  echo "  hidden      : $(_q 'SELECT COUNT(*) FROM hidden;')"
  echo "  children    : $(_q 'SELECT COUNT(*) FROM children;')"
  echo "  swap_event  : $(_q 'SELECT COUNT(*) FROM swap_event;')"
  echo "  chat (index): $(_q 'SELECT COUNT(*) FROM chat;')"
  echo "  primary     : $(cmd_primary_get)"
  echo "  transcripts on disk: $(find -P "$HOME/.claude/projects" -name '*.jsonl' 2>/dev/null | wc -l)"
}

case "${1:-}" in
  init)         cmd_init ;;
  migrate)      shift; cmd_migrate ;;
  stats)        cmd_stats ;;
  hidden-list)  cmd_hidden_list ;;
  hidden-add)   shift; cmd_hidden_add "$@" ;;
  hidden-del)   shift; cmd_hidden_del "$@" ;;
  hidden-has)   shift; cmd_hidden_has "$@" ;;
  hidden-reap)  cmd_hidden_reap ;;
  hidden-at-list) cmd_hidden_at_list ;;
  hidden-sync)  cmd_hidden_sync ;;
  primary-get)  cmd_primary_get ;;
  primary-set)  shift; cmd_primary_set "$@" ;;
  child-add)    shift; cmd_child_add "$@" ;;
  child-list)   shift; cmd_child_list "$@" ;;
  child-clear)  shift; cmd_child_clear "$@" ;;
  chat-load)    cmd_chat_load ;;
  chat-save)    cmd_chat_save ;;
  chat-prune)   cmd_chat_prune ;;
  swap-log)     shift; cmd_swap_log "$@" ;;
  -h|--help|"") sed -n '2,30p' "$0" ;;
  *) echo "unknown: $1" >&2; exit 2 ;;
esac
