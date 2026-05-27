# Claude Memory Auto-Backup

Reference document for the optional **memory backup** capability — a `SessionEnd` hook that auto-syncs Claude Code's persistent project memory to a private git repo. Opt-in at install (see `SETUP.md` § "Memory backup"). The genericized script template lives at `blueprint/templates/scripts/memory-sync.sh`.

---

## What it is

Claude Code keeps a persistent **auto-memory** for each project — the facts, feedback, and context Claude accumulates across conversations. It lives on disk at:

```
~/.claude/projects/<PROJECT-KEY>/memory/
```

This directory is **machine-local and NOT in your code repo.** A machine wipe loses all of it. A new machine starts from zero. The memory that makes Claude feel like it "knows your project" doesn't travel.

Memory backup fixes that: it points the live memory directory at a **private git repo** and pushes any change automatically when a session ends. Plain git — no LLM, no tokens, ~1 second.

---

## Why it matters

The auto-memory is institutional knowledge — corrections you gave Claude, project conventions it learned, decisions it remembers. Re-teaching it on a new machine is slow and lossy. Backing it up means:

- **Survives a machine wipe** — the vault lives off-machine in a private repo.
- **Travels to a new machine** — clone the vault, symlink, done.
- **Versioned** — every sync is a commit; you can see how the memory evolved.
- **Zero friction** — it runs on session end with no prompt, no token cost.

---

## Architecture

Three pieces:

1. **A private git repo** is the backup vault (e.g. `<gh-user>/<project>-memory`). Private because memory can contain project-specific context you don't want public.
2. **A symlink** replaces the live memory directory. The real folder `~/.claude/projects/<PROJECT-KEY>/memory` becomes a symlink to a local clone of the vault (e.g. `~/work/<project>-memory`). Claude reads and writes memory straight through the symlink — it never knows the difference.
3. **A `SessionEnd` hook** in `~/.claude/settings.json` runs a small shell script (`memory-sync.sh`) every time a session ends. It commits and pushes any memory change.

```
Claude writes memory
        │
        ▼
~/.claude/projects/<PROJECT-KEY>/memory  ──(symlink)──▶  ~/work/<project>-memory  ──(git push)──▶  <gh-user>/<project>-memory (private)
        ▲                                                          ▲
        │                                                          │
   reads/writes transparently                          SessionEnd hook runs memory-sync.sh
```

---

## The hook config

Added to the `"hooks"` object in `~/.claude/settings.json` (same shape as the Stop / PreToolUse hooks the notify and formatter scripts use):

```json
"SessionEnd": [
  { "matcher": "", "hooks": [ { "type": "command", "command": "sh $HOME/work/<project>-memory/.sync.sh" } ] }
]
```

The hook lives in **global** `~/.claude/settings.json`, so it fires on EVERY session end for ANY project — but it no-ops harmlessly when there's nothing in the vault to push (see Tip 1).

---

## The sync script

The genericized template is `blueprint/templates/scripts/memory-sync.sh` — install it into the vault as `.sync.sh`. It uses the machine's ambient git `user.name` / `user.email`; no hardcoded identity.

```sh
#!/bin/sh
# Auto-backup Claude Code's project auto-memory to a private git repo.
# Installed as a SessionEnd hook (~/.claude/settings.json). Plain git — no LLM, no tokens.
# Self-healing: pushes whenever local is ahead of remote, so a push cut off by a hard
# window-close is recovered on the next session end.
export GIT_TERMINAL_PROMPT=0          # never hang on a credential prompt — fail fast + log
REPO="$HOME/work/{PROJECT_NAME}-memory"
LOG="$HOME/.claude/{PROJECT_NAME}-memory-sync.log"
cd "$REPO" || exit 0
# commit any pending memory changes (no-op if clean)
if [ -n "$(git status --porcelain)" ]; then
    git add -A
    git commit -q -m "memory: auto-sync $(date '+%Y-%m-%d %H:%M:%S')" >>"$LOG" 2>&1
fi
# push if local is ahead of remote (also recovers a previously cut-off push)
if [ -n "$(git rev-list origin/main..HEAD 2>/dev/null)" ]; then
    if git push -q origin main >>"$LOG" 2>&1; then
        echo "$(date '+%F %T') pushed" >>"$LOG"
    else
        echo "$(date '+%F %T') PUSH FAILED — retries next session" >>"$LOG"
    fi
fi
```

Replace `{PROJECT_NAME}` with your project's name at install (matches the blueprint placeholder convention).

---

## Setup procedure (one-time)

1. **Create a PRIVATE repo** `<gh-user>/<project>-memory` on GitHub.
2. **Seed the vault off-machine first.** Copy the current memory dir contents into `~/work/<project>-memory`, then `git init`, commit, and push. The vault now exists off-machine BEFORE you touch the original — data lives in two places before anything is replaced.
3. **Configure headless auth:** `gh auth setup-git`. This registers `gh` as the git credential helper so `push` works with no terminal prompt; the token stays in the OS keychain (not plaintext on disk). Use HTTPS, not SSH (no passphrase to cache).
4. **Swap the live dir for a symlink.** Rename the original to `memory.bak` (a `mv`, never `rm`), then symlink:
   ```sh
   mv ~/.claude/projects/<PROJECT-KEY>/memory ~/.claude/projects/<PROJECT-KEY>/memory.bak
   ln -s ~/work/<project>-memory ~/.claude/projects/<PROJECT-KEY>/memory
   ```
   Verify Claude can read through it, THEN optionally remove `memory.bak`.
5. **Install the `SessionEnd` hook** in `~/.claude/settings.json` (see § "The hook config" and the user-run one-liner in Tip 11).
6. **Test** with the test-payload trick (Tip 7).

---

## Tips & Pitfalls

1. **It is a shell hook, NOT an LLM prompt.** It runs as plain `git`, ~1 second, zero tokens — no model wakes up. The opening `git status --porcelain` guard makes it an instant no-op when nothing changed. Because the hook lives in _global_ `~/.claude/settings.json`, it fires on EVERY session end for ANY project — but it no-ops harmlessly when there's nothing in the vault to push.
2. **`SessionEnd` hooks ARE awaited.** Claude Code blocks on them (up to a 600s timeout) on a clean exit, so a synchronous `git push` has plenty of time to finish.
3. **Window-close race (the key pitfall).** Hard-closing the terminal window can fire the hook AND tear down the process before the network push finishes — the local commit lands, but the push gets cut off. A clean exit (`/quit`, `/clear`, Ctrl-D) guarantees the push completes synchronously.
4. **Self-healing push (the key invention).** Push whenever `git rev-list origin/main..HEAD` is non-empty — i.e. whenever local is ahead of remote — NOT only when there's a fresh commit. Without this, a commit whose push got cut off would be stranded forever: the next run sees a clean working tree (`git status` empty) and would skip. With it, the orphaned commit pushes on the very next session end. This is what makes the window-close race harmless.
5. **`GIT_TERMINAL_PROMPT=0`.** Set it so git never blocks waiting on a credential prompt in a headless context; it fails fast and logs `PUSH FAILED` instead of hanging until the hook timeout kills it.
6. **Headless auth via `gh auth setup-git`.** This registers `gh` as the credential helper; the OS keychain holds the token (not plaintext on disk). Verify headless auth works with `GIT_TERMINAL_PROMPT=0 git ls-remote origin HEAD` — it should return instantly without a prompt. Use HTTPS, not SSH (no passphrase to cache).
7. **Test-payload trick.** To verify the hook end-to-end you MUST stage a deliberate pending change first (a file the hook is forced to push). A clean repo makes the hook a silent no-op, which is indistinguishable from "the hook never fired" — so you can't tell success from failure without bait. After the close, confirm a new `pushed` line in the log AND that the file reached the remote.
8. **Symlink + `.bak` safety.** Always get the data into the vault AND pushed BEFORE replacing the live dir. Rename the original to `memory.bak` (`mv`), never delete it until the symlink is verified working. Data should live in two places before you touch the original.
9. **Logging is the receipt.** `~/.claude/<project>-memory-sync.log` records `pushed` / `PUSH FAILED` with timestamps — it's how you confirm the hook ran. An empty log after a close that should have pushed = the push was cut off (window-close race) → the self-heal catches it next time.
10. **Project-key portability.** The memory folder name under `~/.claude/projects/` is derived from the absolute path of the code repo on that machine (e.g. a repo at `~/work/Foo` becomes a key like `-Users-you-work-Foo`). On a new machine or a different clone path, that segment changes — so on restore, symlink whatever folder Claude actually uses, not a hardcoded name.
11. **Permission-mode pitfall (install-time reality).** Installing the hook edits global `~/.claude/settings.json`. If you run Claude with auto-permission mode and `skipAutoPermissionPrompt`, the classifier will SILENTLY DENY the edit (it's a persistent, code-running config change) without even prompting. The robust workaround: have the USER run the edit themselves so it isn't an agent action. This idempotent one-liner won't duplicate or clobber existing hooks — paste it with your own values:
    ```
    python3 -c "import json,pathlib; p=pathlib.Path.home()/'.claude/settings.json'; d=json.loads(p.read_text()); d.setdefault('hooks',{}).setdefault('SessionEnd',[{'matcher':'','hooks':[{'type':'command','command':'sh \$HOME/work/<project>-memory/.sync.sh'}]}]); p.write_text(json.dumps(d,indent=2)); print('SessionEnd hook added')"
    ```
12. **Clean-exit habit.** Exit with `/quit` or `/clear` for a guaranteed synchronous flush. A hard window-close still works but leans on the self-heal (Tip 4) to catch up on the next close.

---

## Restore on a new machine

The vault is the source of truth. To restore memory on a fresh machine or a different clone path:

1. **Clone the vault:** `git clone https://github.com/<gh-user>/<project>-memory.git ~/work/<project>-memory`
2. **Find the project key.** It's derived from the absolute path of your code repo on THIS machine — list `~/.claude/projects/` to see the folder Claude actually created for the project (Tip 10). The segment changes per machine / clone path, so never hardcode it.
3. **Symlink** that folder to the clone:
   ```sh
   # back up anything Claude already wrote, then symlink
   mv ~/.claude/projects/<PROJECT-KEY>/memory ~/.claude/projects/<PROJECT-KEY>/memory.bak  # if it exists
   ln -s ~/work/<project>-memory ~/.claude/projects/<PROJECT-KEY>/memory
   ```
4. **Re-arm the hook:** `gh auth setup-git`, then install the `SessionEnd` hook (Tip 11). New machine, same vault — memory is back, and future sessions keep syncing.
