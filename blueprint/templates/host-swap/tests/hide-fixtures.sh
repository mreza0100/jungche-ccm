#!/usr/bin/env zsh
# Fixtures for the cc-ls hide path — the REAL cc-ls, not a re-implementation of its logic.
# Everything is redirected to scratch: a fake HOME (so ~/.claude and the hide-list file are
# throwaway), a scratch fleet db, an EMPTY tmux socket dir (so the live-session loop finds
# nothing and only the resume scan runs), and an fzf shim that captures the row list instead of
# drawing a picker. Never the live db, never the live sockets, never the real ~/.claude.
#
# What it pins:
#   · a hidden chat stays hidden — even while it is growing with new prompts (hide is permanent)
#   · a ⌃X hide reaches the DATABASE, so it survives into the next run
#   · a ⌃X unhide likewise removes it
#   · a uuid stranded in the legacy hide-list file is imported on the next run
emulate -L zsh
setopt no_nomatch

BUNDLE="${0:A:h:h}"
REAL_HOME="$HOME"
T="$(mktemp -d)"
pass=0 fail=0
ok(){ if [[ "$2" == "$3" ]]; then (( pass++ )); print -r -- "  ✓ $1"
      else (( fail++ )); print -r -- "  ✗ $1"; print -r -- "     want=[$3]"; print -r -- "     got =[$2]"; fi }

# ── scratch world ─────────────────────────────────────────────────────────────────────────────
export HOME="$T/home"
export TMUX_TMPDIR="$T/tmux"
export CC_FLEET_HOME="$BUNDLE"
export CC_DB="$BUNDLE/cc-db.sh"
export CC_FLEET_DB="$T/fleet.db"
PROJ="$HOME/.claude/projects/-fixture"
# .cc/<n> has to exist: cc-ls globs the account dirs unguarded, so an empty scratch HOME dies on
# a nomatch before it ever reaches the hide logic. Give the scratch world the shape of a real one.
mkdir -p "$PROJ" "$HOME/.cc/2/projects" "$HOME/.claude/bin" "$HOME/.codex/sessions" \
         "$TMUX_TMPDIR/tmux-$(id -u)" "$T/bin"
HF="$HOME/.claude/.cc-ls-hidden"

# An fzf that captures the rows it was fed, optionally presses ⌃X, and refuses to pick, so cc-ls
# returns without attaching to anything. Exit 130 is the code a real ⌃C gives it.
# Pressing ⌃X from HERE and not between runs is the whole point: the toggle has to land after
# cc-ls materialised the hide-list file and before it syncs that file back, which is exactly
# where the real keybinding fires. A between-runs edit is overwritten by the next materialise.
cat > "$T/bin/fzf" <<'SHIM'
#!/bin/sh
cat > "$FZF_ROWS"
if [ -n "${FZF_TOGGLE:-}" ]; then      # the ⌃X binding, same file, same lock, same toggle
  ( flock 9 2>/dev/null || true
    if grep -qxF -- "$FZF_TOGGLE" "$FZF_HF" 2>/dev/null; then
      grep -vxF -- "$FZF_TOGGLE" "$FZF_HF" > "$FZF_HF.t.$$" 2>/dev/null; mv "$FZF_HF.t.$$" "$FZF_HF"
    else
      printf '%s\n' "$FZF_TOGGLE" >> "$FZF_HF"
    fi
  ) 9>"$FZF_HF.lock"
fi
exit 130
SHIM
chmod +x "$T/bin/fzf"
export PATH="$T/bin:$PATH" FZF_ROWS="$T/rows"

# transcript <uuid> <title> <prompts…> — a minimal but REAL transcript: cc-ls needs a cwd, a
# non-junk user turn (the prompt count) and a title, or strict mode drops the row as a husk.
transcript(){
  local u="$1" title="$2" f="$PROJ/$1.jsonl"; shift 2
  : > "$f"
  local p
  for p in "$@"; do
    print -r -- "{\"type\":\"user\",\"cwd\":\"/fixture\",\"message\":{\"role\":\"user\",\"content\":\"$p\"}}" >> "$f"
  done
  print -r -- "{\"type\":\"custom-title\",\"customTitle\":\"$title\",\"sessionId\":\"$u\"}" >> "$f"
}
# grow <uuid> <prompt> — append a genuine new prompt, exactly what talking to a live chat does.
grow(){ print -r -- "{\"type\":\"user\",\"cwd\":\"/fixture\",\"message\":{\"role\":\"user\",\"content\":\"$2\"}}" >> "$PROJ/$1.jsonl" }

# run [uuid] — one full cc-ls in a fresh zsh; with a uuid, ⌃X is pressed on it mid-picker.
# A fresh shell per run is the point: it proves the state survived in the db, not in memory.
run(){
  : > "$FZF_ROWS"
  zsh -f -c "
    export HOME='$HOME' TMUX_TMPDIR='$TMUX_TMPDIR' PATH='$PATH' FZF_ROWS='$FZF_ROWS'
    export FZF_HF='$HF' FZF_TOGGLE='${1:-}'
    export CC_FLEET_HOME='$CC_FLEET_HOME' CC_DB='$CC_DB' CC_FLEET_DB='$CC_FLEET_DB'
    source '$BUNDLE/cc-fleet.zsh' >/dev/null 2>&1
    cc-ls >/dev/null 2>&1
  " >/dev/null 2>&1
}
shown(){ grep -c "$1" "$FZF_ROWS" 2>/dev/null | tr -d ' ' }
indb(){ bash "$CC_DB" hidden-has "$1" >/dev/null 2>&1 && print yes || print no }

A=aaaaaaaa-1111-1111-1111-111111111111   # stays visible throughout — the control
B=bbbbbbbb-2222-2222-2222-222222222222   # hidden with ⌃X
C=cccccccc-3333-3333-3333-333333333333   # hidden, then keeps growing
transcript $A ALPHA   "first prompt for alpha"
transcript $B BRAVO   "first prompt for bravo"
transcript $C CHARLIE "first prompt for charlie"
bash "$CC_DB" init >/dev/null 2>&1

echo "=== 1. baseline: all three chats are offered ==="
run
ok "ALPHA listed"   "$(shown ALPHA)"   "1"
ok "BRAVO listed"   "$(shown BRAVO)"   "1"
ok "CHARLIE listed" "$(shown CHARLIE)" "1"

echo "=== 2. ⌃X hide reaches the db and survives the next run ==="
run $B                                 # ⌃X on BRAVO — the picker is the last thing cc-ls does,
run                                    # so the NEXT run is what commits it. This is where the
ok "BRAVO gone"                  "$(shown BRAVO)"  "0"   # old build brought the row back.
ok "⌃X hide reached the db"      "$(indb $B)"      "yes"
ok "ALPHA still listed"          "$(shown ALPHA)"  "1"
run
ok "and it stays gone"           "$(shown BRAVO)"  "0"

echo "=== 3. hide is PERMANENT — a growing, actively-used chat stays hidden ==="
bash "$CC_DB" hidden-add "$C" >/dev/null 2>&1
run
ok "CHARLIE hidden"              "$(shown CHARLIE)" "0"
grow $C "a brand new prompt, someone is talking to it"
grow $C "and another one"
run
ok "still hidden after 2 new prompts" "$(shown CHARLIE)" "0"
ok "and still hidden in the db"       "$(indb $C)"       "yes"

echo "=== 4. ⌃X unhide removes it from the db too ==="
run $B                                 # same key, other direction — cc-ls -a / --hidden then ⌃X
run
ok "BRAVO back in the list"      "$(shown BRAVO)"   "1"
ok "unhide left the db"          "$(indb $B)"       "no"

echo "=== 5. a uuid stranded in the legacy file is adopted — ONCE ==="
# Every ⌃X hide made before the sync was fixed sits in this file and in no db. The first run
# must adopt them in one pass. It must also be the LAST run that reads the file as an input:
# re-adopting on every run would undo the very next ⌃X, which is how §4 caught this.
sqlite3 "$CC_FLEET_DB" "DELETE FROM hidden;" >/dev/null 2>&1
rm -f "$HF.adopted"                       # a box that has never adopted
printf '%s\n' "$A" > "$HF"
run
ok "orphan adopted into the db"  "$(indb $A)"       "yes"
ok "ALPHA hidden on that run"    "$(shown ALPHA)"   "0"
run $A                                              # now ⌃X the adopted one back
run
ok "ALPHA visible again"         "$(shown ALPHA)"   "1"
ok "adoption does not resurrect it" "$(indb $A)"    "no"

echo "=== 6. a missing hide-list file never wipes the db ==="
# the exit-sync DELETEs whatever the file omits, so a clobbered file must abort the sync,
# not empty the hidden set. An EMPTY file is different: that legitimately means nothing is hidden.
bash "$CC_DB" hidden-add "$C" >/dev/null 2>&1
rm -f "$HF"
run
ok "hides survived a missing file" "$(indb $C)"     "yes"

echo
printf 'PASS %d   FAIL %d\n' "$pass" "$fail"
[[ "$HOME" == "$T/home" ]] && rm -rf "$T"
(( fail == 0 ))
