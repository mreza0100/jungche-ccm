# Claude Memory Auto-Backup

Reference for the optional **memory backup** capability — two `SessionStart` + `SessionEnd` hooks that auto-wire and auto-sync Claude Code's persistent project memory to one private git vault. Opt-in at install (see `SETUP.md` § "Memory backup"). Genericized script templates live at `blueprint/templates/scripts/{cc-memory-wire,cc-memory-consolidate,memory-sync}.sh`.

---

## What it is

Claude Code keeps a persistent **auto-memory** for each project — the facts, feedback, and context Claude accumulates across conversations. It lives on disk at:

```
~/.claude/projects/<PROJECT-KEY>/memory/
```

`<PROJECT-KEY>` is the absolute path of the code repo on that machine, slash-encoded (a repo at `~/work/Foo` becomes `-Users-you-work-Foo`). This directory is machine-local and NOT in your code repo — a machine wipe loses it, a new machine starts from zero, and the memory that makes Claude feel like it "knows your project" doesn't travel.

Memory backup fixes that with **one vault for every project**: a single private git repo holds every project's memory, each project in its own subdirectory. A `SessionStart` hook auto-wires whatever project you open; a `SessionEnd` hook syncs the whole vault. Plain git — no LLM, no tokens, ~1 second.

---

## Why it matters

The auto-memory is institutional knowledge — corrections you gave Claude, project conventions it learned, decisions it remembers. Backing it up means it survives a machine wipe, travels to a new machine, stays versioned (every sync is a commit), and runs with zero friction (session-start wiring + session-end sync, no prompt, no token cost). One vault covers all your projects, so a new machine restores everything with a single clone.

---

## Architecture

The model is **ONE vault, MANY subdirs**:

1. **One private git vault** holds every project's memory. Each project gets a subdirectory named after the project folder's basename, so the SAME project on two machines maps to the SAME subdir — memory is shared per project across machines, with no intermingling. Private because memory can hold project-specific context.
2. **A symlink per project** replaces each live memory dir. `~/.claude/projects/<PROJECT-KEY>/memory` becomes a symlink to `<vault>/<project-basename>/`. Claude reads and writes straight through it.
3. **A `SessionStart` hook** (`cc-memory-wire.sh`) auto-wires the project you open: it pulls the vault current, then ensures the current project's memory dir is symlinked into its subdir — migrating a pre-existing real memory dir in first (non-destructive), and leaving an already-correct link alone (idempotent). Zero manual setup per project.
4. **A `SessionEnd` hook** (`memory-sync.sh`) syncs the whole vault: pull-rebase first (multi-writer-safe), then commit + push any change.

```
Claude writes memory
        │
        ▼
~/.claude/projects/<PROJECT-KEY>/memory  ──(symlink)──▶  <vault>/<project-basename>/  ──(git push)──▶  private vault repo
        ▲                                                          ▲                              ▲
        │                                                          │                              │
   reads/writes transparently              SessionStart wires the link            SessionEnd syncs the whole vault
```

---

## The vault path (single config point)

Every script reads ONE config point — the env var `CLAUDE_MEMORY_REPO` if set, otherwise the install-baked default `$HOME/work/{MEMORY_VAULT_DIR}` (the `{MEMORY_VAULT_DIR}` placeholder is filled at install with your vault's directory name). Set `CLAUDE_MEMORY_REPO` in your shell profile to point all three scripts at a different vault location without re-installing. The vault is never hardcoded.

---

## The hooks

Both hooks live in **global** `~/.claude/settings.json`, so they fire once per session for whatever project is open — they are cross-project and belong at the user level, not duplicated into each project's `.claude/settings.json`. Each no-ops harmlessly when there's nothing to do (no vault, already-wired link, clean tree).

```json
"SessionStart": [
  { "matcher": "", "hooks": [ { "type": "command", "command": "sh $HOME/.claude/scripts/cc-memory-wire.sh" } ] }
],
"SessionEnd": [
  { "matcher": "", "hooks": [ { "type": "command", "command": "sh $HOME/.claude/scripts/memory-sync.sh" } ] }
]
```

Install copies the scripts to `~/.claude/scripts/` (user-level — they target `~/.claude/...` across every project, so they are not part of any one project's `.claude/`).

---

## The scripts

Three genericized templates under `blueprint/templates/scripts/`:

- **`cc-memory-wire.sh`** — `SessionStart`. Pulls the vault, reads the project dir from the hook's stdin JSON (`.cwd`, falling back to `$PWD`), derives the subdir from the project basename, and ensures the memory dir is symlinked into it. **Root-guard:** a memory dir already symlinked to the vault ROOT (a legacy single-project "main brain") is left untouched, never re-homed into a subdir. Migrates a pre-existing real memory dir into the subdir before linking (non-destructive — never clobbers a file already in the vault). Idempotent.
- **`memory-sync.sh`** — `SessionEnd`. Pull-rebase --autostash, then commit + push the whole vault. Self-healing push (below).
- **`cc-memory-consolidate.sh`** — one-time, run once per machine. Walks every `~/work/<project>` memory dir and migrates each into its vault subdir: copy in, VERIFY file-for-file, only THEN swap the local dir for a symlink. Skips any dir already linked into the vault (including the root brain). After it runs once, the `SessionStart` hook handles every new project automatically.

All three use the ambient git `user.name` / `user.email` — no hardcoded identity.

---

## Setup procedure (one-time)

1. **Create ONE private repo** as your memory vault (e.g. `<gh-user>/<you>-memory`) on GitHub, and `git init` a local clone at the vault path (`$HOME/work/{MEMORY_VAULT_DIR}` or your `CLAUDE_MEMORY_REPO`). One vault for all projects.
2. **Configure headless auth:** `gh auth setup-git`. This registers `gh` as the git credential helper so `push` works with no terminal prompt; the token stays in the OS keychain (not plaintext on disk). Use HTTPS, not SSH (no passphrase to cache).
3. **Install the scripts + hooks.** Copy the three scripts to `~/.claude/scripts/` and add the `SessionStart` + `SessionEnd` hooks to `~/.claude/settings.json` (see § "The hooks" and the user-run one-liner in Tip 9).
4. **Run the consolidator once:** `sh ~/.claude/scripts/cc-memory-consolidate.sh`. It migrates every existing `~/work/<project>` memory dir into the vault, verifying each before swapping. New projects need no manual step — the `SessionStart` hook wires them on first open.
5. **Test** with the test-payload trick (Tip 5).

---

## Tips & Pitfalls

1. **It is a shell hook, NOT an LLM prompt.** Plain `git`, ~1 second, zero tokens — no model wakes up. The `git status --porcelain` guard makes the sync an instant no-op when nothing changed. Both hooks fire on every session for any project, and each no-ops when there's nothing to do.
2. **`SessionEnd` hooks ARE awaited.** Claude Code blocks on them (up to a 600s timeout) on a clean exit, so a synchronous `git push` has time to finish.
3. **Window-close race.** Hard-closing the terminal can fire the hook AND tear down the process before the network push finishes — the local commit lands, the push gets cut off. Exit cleanly (`/quit`, `/clear`, Ctrl-D) for a guaranteed synchronous flush; a hard close leans on the self-heal (Tip 4).
4. **Self-healing push.** Push whenever `git rev-list origin/main..HEAD` is non-empty — i.e. whenever local is ahead of remote — not only on a fresh commit. A commit whose push got cut off pushes on the very next session end, so the window-close race is harmless.
5. **Test-payload trick.** A clean vault makes the sync a silent no-op, indistinguishable from "the hook never fired." Stage a deliberate pending change first (a file the hook is forced to push). After a clean exit, confirm a new `pushed` line in `~/.claude/memory-sync.log` AND that the file reached the remote.
6. **`GIT_TERMINAL_PROMPT=0`.** Set so git never blocks on a credential prompt in a headless context — it fails fast and logs `PUSH FAILED` instead of hanging until the hook timeout.
7. **Multi-writer safety.** `memory-sync.sh` and `cc-memory-wire.sh` both `git pull --rebase --autostash` before any local commit, so two machines writing the same vault rebase cleanly instead of colliding. The vault is safe to share across machines concurrently.
8. **Root-guard for a legacy single-project vault.** If you previously ran the single-project model (a memory dir symlinked directly to the vault root), `cc-memory-wire.sh` and the consolidator both detect that and leave it untouched — your accumulated "main brain" is never re-homed into a subdir.
9. **Permission-mode pitfall.** Installing the hooks edits global `~/.claude/settings.json` — a persistent, code-running config change that the classifier SILENTLY DENIES under auto-permission mode with `skipAutoPermissionPrompt`, without prompting. Have the USER run this idempotent one-liner (it won't duplicate or clobber existing hooks):
   ```
   python3 -c "import json,pathlib; p=pathlib.Path.home()/'.claude/settings.json'; d=json.loads(p.read_text()); h=d.setdefault('hooks',{}); h.setdefault('SessionStart',[]).append({'matcher':'','hooks':[{'type':'command','command':'sh \$HOME/.claude/scripts/cc-memory-wire.sh'}]}); h.setdefault('SessionEnd',[]).append({'matcher':'','hooks':[{'type':'command','command':'sh \$HOME/.claude/scripts/memory-sync.sh'}]}); p.write_text(json.dumps(d,indent=2)); print('memory hooks added')"
   ```
10. **Logging is the receipt.** `~/.claude/memory-sync.log` records `pushed` / `PUSH FAILED` with timestamps — it's how you confirm the hook ran. An empty log after a close that should have pushed = the push was cut off (window-close race) → the self-heal catches it next time.
11. **Project-key portability.** The `<PROJECT-KEY>` segment under `~/.claude/projects/` is derived from the repo's absolute path on that machine, so it changes per machine / clone path. The vault subdir is keyed off the project BASENAME instead, so the same project shares one subdir everywhere regardless of clone path. No hardcoded key.

---

## Restore on a new machine

The vault is the source of truth. To restore every project's memory on a fresh machine:

1. **Clone the vault** to the vault path: `git clone https://github.com/<gh-user>/<vault>.git $HOME/work/{MEMORY_VAULT_DIR}` (or your `CLAUDE_MEMORY_REPO`).
2. **Re-arm auth + hooks:** `gh auth setup-git`, then install the scripts + `SessionStart`/`SessionEnd` hooks (Tip 9).
3. **Run the consolidator once** (Tip — only needed if the machine already has pre-existing real memory dirs to fold in): `sh ~/.claude/scripts/cc-memory-consolidate.sh`. Otherwise just open each project — the `SessionStart` hook pulls the vault and symlinks each project's memory dir into its subdir on first open. Same vault, same subdirs — memory is back, and future sessions keep syncing.
