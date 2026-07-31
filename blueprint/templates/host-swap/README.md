# Professor — Multi-Account Fleet Tooling

Run two or three Claude subscriptions and pick which one a chat launches under, or reboot an
already-running chat onto a different one — **without disturbing any other running session**.
Universal Tier C mechanic, no domain placeholders. **Linux and macOS** — each account is a plain
config dir; nothing here depends on a Keychain or other OS credential store.

## What it does

```
cc          ->  launch Claude in its own tmux socket, under the current primary account
cc2         ->  launch Claude in its own tmux socket, under account 2
cc-swap     ->  change which account a FUTURE bare `cc` opens (fzf picker; cc-swap 2 jumps directly)
/swap 2     ->  reboot THIS running chat in place onto account 2 (same pane, same socket)
cc-ls       ->  fzf picker over every live + resumable chat (Enter attaches or resumes)
/bb         ->  bye-bye: hide this chat from cc-ls and close it, reaping its teammates
```

`cc-swap` only changes which account a **future** `cc` launch uses — it never touches a chat
that's already running. Moving an already-running chat to another account is `/swap`'s job:
`CLAUDE_CONFIG_DIR` (and the cache-TTL env) binds once at process birth, so retargeting a live
chat means ending that process and starting a fresh one in the same pane, not rewriting a
credential in place. `swap.command.md` documents that reboot contract and `cc-swap-chat.sh` is the
engine that performs it — both ship here, so `/swap` works after `install.sh --apply` with nothing
left for you to author.

## How it works

Each **account** IS its own Claude Code config dir directly — no per-launch session dir, no
credential store to seed or rotate:

```
~/.claude          account 1 (CLAUDE_CONFIG_DIR unset — Claude's own default)
~/.claude2         account 2
~/.claude3         account 3 (optional — everything below reads correctly with just 1 and 2)
```

One `/login` per account, ever — no credential copying (a copied OAuth token forks and dies).
Every launch gets its **own tmux server** (`-L cc-<epoch>-<pid>-<rand>`) so a single crashed tmux
can't take every chat down at once, and so a chat picker or `/swap`'s engine can address one
chat's pane precisely instead of guessing which pane in a shared server is which chat. Which
account a bare `cc` opens (default `1`) lives in the state store, `~/.cc/fleet.db`; `cc-swap`
writes it. The statusline badge (🥇/🥈/🥉) reads each chat's own `CLAUDE_CONFIG_DIR` on every
refresh — no separate marker to keep in sync.

## Files in this template

`install.sh` puts every one of these in place as a **symlink back into this clone** — see § Install.

| File | Installs as | Purpose |
|------|-------------|---------|
| `install.sh` | — | wires the whole bundle by symlink; dry-run by default, `--apply` to commit, `--uninstall` to undo |
| `cc-fleet.zsh` | sourced from `~/.zshrc` | the whole fleet: `cc`/`cc1`…`cc4` launchers (each its own tmux server), `cc-swap`, and the `cc-ls` picker |
| `cc-db.sh` | `~/.claude/bin/cc-db.sh` | the fleet's state store — one SQLite database at `~/.cc/fleet.db` holding the hide list, the primary account, spawned children, the chat index and the swap log |
| `cc-hide.sh` | `~/.claude/bin/cc-hide.sh` | `/bb`'s engine — hide this chat from `cc-ls` then close it; **pane-aware**: kills only its own pane (never the shared server) and reaps the teammates it spawned |
| `cx-hide.sh` | `~/.claude/bin/cx-hide.sh` | the **Codex** twin of `cc-hide.sh` — `/bb` for a `cx-*` chat, identified by rollout cwd since Codex passes no tmux context to its tool shell |
| `cc-agent-open.sh` | `~/.claude/bin/cc-agent-open.sh` | `cc-ls`'s Enter-target for a chat locked by a live background agent: the takeover/attach chooser |
| `cc-swap-chat.sh` | `~/.claude/bin/cc-swap-chat.sh` | `/swap`'s engine — reboot a running chat in place onto another account and/or flip its cache mode |
| `cc-reap.sh` | `~/.claude/bin/cc-reap.sh` | reclaim RAM from the `cc-*` socket graveyard (dry-run by default; `--kill` reaps unattached orphans + stale socket files) |
| `cc-archive.sh` | `~/.claude/bin/cc-archive.sh` | move hidden chats and old subagent transcripts out of the live pool, reversibly (dry-run by default; `--restore` puts one back) |
| `bb.command.md` | `~/.claude/commands/bb.md` | `/bb` slash command — bye-bye: hide + close this chat (and any detached teammates it spawned) |
| `swap.command.md` | `~/.claude/commands/swap.md` | `/swap` slash command — reboot this chat onto another account, in place |
| `chat/` | `~/.claude/commands/chat/` | the `chat:*` command family + its engine `chat.sh` |
| `statusline-badge.snippet.sh` | merge into `~/.claude/statusline-command.sh` | 🥇/🥈/🥉 account badge (the one piece that is still a manual merge — it edits a file you own) |
| `tests/` | — | fixtures for the state store, the self-location resolver, and the installer |

## Install

**1. Clone the blueprint to a stable home.** The clone IS the install — nothing is copied out of
it, so keep it somewhere permanent:

```bash
git clone https://github.com/{GH_USER}/professor.git ~/.professor
```

**2. Log in to each account's config dir** you plan to use. Each account is its own config dir;
one `/login` per account, ever:

```bash
# Account 1 (the default ~/.claude — already set if you use Claude Code normally)
CLAUDE_CONFIG_DIR="$HOME/.claude2" claude   # account 2, then /login inside CC
CLAUDE_CONFIG_DIR="$HOME/.claude3" claude   # account 3 (optional)
```

**3. Look at what the installer will do, then let it:**

```bash
cd ~/.professor/blueprint/templates/host-swap
./install.sh            # dry run — prints every link it would make, changes nothing
./install.sh --apply
```

It symlinks the scripts into `~/.claude/bin`, the commands into `~/.claude/commands`, and adds one
`source` line to `~/.zshrc`. Anything real already sitting at a destination is moved aside to
`<name>.pre-professor-<timestamp>` first — it never destroys a file you wrote. Re-running after a
`git pull` is free: links already pointing at the clone are reported `ok` and left alone.

**4. Edit the account table** in `cc-fleet.zsh` — one clearly marked block near the top lists the
account numbers and their config dirs. A two-account setup just drops the account-3 entries.

**5. Add the account badge to your statusline** (if you use the Professor statusline):

Merge the block from `statusline-badge.snippet.sh` into `~/.claude/statusline-command.sh` just before the `# ── LINE 1` section (where `badge` is referenced). The badge variable is already consumed by the `l1` line — you just need the computation block.

**6. Open a new shell and test:**

```bash
cc1        # should open CC under account 1, its own tmux socket
cc2        # in another terminal — independent session, unaffected
cc-swap 2  # change which account a FUTURE bare `cc` opens
cc-ls      # the picker over every live + resumable chat (needs fzf)
```

## Updating

```bash
git -C ~/.professor pull
```

That is the whole update. Every command and script is a link into the clone, so a pull moves all
of them at once — there is no second copy to re-install and none to drift. Re-run
`install.sh --apply` only when a release ADDS a file (it reports the new links and leaves the rest
alone); `install.sh --uninstall` reverses everything, restoring any file it backed up.

Because the live files ARE the clone's files, editing `~/.claude/commands/chat/ls.md` edits the
repo. That is deliberate — it is what makes local fixes visible to `git diff` instead of silently
diverging — but it does mean a `git pull` can conflict with a local edit. Commit or stash yours.

## Where the fleet keeps its state

One SQLite database, `~/.cc/fleet.db`, written only through `cc-db.sh`:

| Table | Holds |
|-------|-------|
| `hidden` | chats hidden from `cc-ls`, each with the transcript-size baseline that drives auto-unhide |
| `meta` | the primary account a bare `cc` opens |
| `children` | teammates a chat spawned, so `/bb` can reap them |
| `chat` | the chat index — mtime, size, cwd, label, prompt count (what makes the picker fast) |
| `swap_event` | the `/swap` log |

Every write is one transaction, so concurrent chats cannot lose each other's updates — the failure
mode the previous sidecar files had. If `sqlite3` is missing, every read falls back to the legacy
flat files and every write appends to them: the picker is never down because a database is unhappy.

Run the fixtures against a scratch database any time — they never touch `~/.cc/fleet.db`:

```bash
cd ~/.professor/blueprint/templates/host-swap
bash tests/db-fixtures.sh          # the state store
bash tests/selflocate-fixtures.sh  # the bundle finds itself through symlinks
bash tests/install-fixtures.sh     # the installer, against a scratch HOME
```

## Maintenance

Nothing to prune — each account is its own config dir directly, not a per-launch session dir, so
there's no session-dir graveyard to clean up. If Anthropic ever rotates a refresh token, just
re-`/login` under that account's config dir (`CLAUDE_CONFIG_DIR=~/.claude2 claude`, then
`/login`) — no other chat is affected.

## Fleet management — `cc-ls`, `/bb`, `cc-reap`

The pieces above launch and bill chats; these manage the resulting fleet. They are **launcher-agnostic** — they work with any setup that runs each chat in its own `tmux -L cc-*` socket (`cc-fleet.zsh` is one such launcher) and a statusline that writes a `/tmp/cc-sid/<socket>` → transcript breadcrumb (`chmod 700` — the breadcrumbs are transcript paths and the name cache carries prompt text, not for other uids). The Professor statusline template (`blueprint/templates/statusline/statusline-command.sh`) already writes this breadcrumb, so installing it alongside these pieces is enough — no separate wiring needed.

- **`cc-ls`** — one fzf list of every chat: `●` live tmux sessions (Enter attaches), `↻` resumable transcripts with no live tmux (Enter resumes in a fresh tmux), and `⚙` live background/forked agents — a `claude --bg` session, an RR brainer, a `/chat:new --detach` teammate — which have no tmux socket, so `--resume` refuses them; Enter instead opens the **takeover/attach chooser** (`cc-agent-open.sh`) — take the agent over fresh under the current primary account, or attach to the running process (see § Agent mode — takeover vs attach). `⌃T` re-sorts recent⇄prompts, `⌃R` rotates the project on top, `⌃X` hides⇄shows a chat. `cc-ls -a` shows all; `cc-ls --hidden` shows only hidden. **Auto-unhide:** a hidden chat that gets new activity (a size baseline recorded lazily, and finalized by `/bb`) drops off the hide list and reappears on its own — hiding is for finished chats, not a way to silence a still-live one.
- **`/bb`** (bye-bye) — hide THIS chat from `cc-ls` and close it. Pane-aware: it kills only its **own** pane (so a chat spawned beside others via `/chat:branch` or `/chat:new` never drops its neighbours) and reaps the teammates it spawned (pane teammates by `kill-pane`, detached `--detach` teammates by `kill-server`). Identifies the chat by `$CLAUDE_CODE_SESSION_ID`, so it never hides the wrong transcript on a shared socket. Closing types a real `/exit` into the pane (flushes the transcript, runs Stop hooks) and **polls** for the pane to close itself — up to 20s, since compaction can outlive a fixed grace — before a `kill-pane` backstop, then records the post-exit transcript size as the auto-unhide baseline so the `/exit` flush itself is never mistaken for new activity.
- **`cc-reap`** — reclaim RAM from orphaned `cc-*` servers (a closed terminal tab detaches the client but leaves the server + its `claude` node alive). Dry-run report by default; `cc-reap --kill` reaps unattached orphans and removes stale socket files. KEEP guards protect more than just attached chats: a `cc-new-*` detached teammate is kept as `mate` (headless by design — its parent's `/bb` reaps it), and any session `claude agents --json` reports **busy** is kept as `busy` (deliberately detached, still grinding — socket maps to session via the `/tmp/cc-sid` breadcrumb, scanned across every configured account). Never touches an attached chat or your own socket.

- **`cc-archive`** — the fleet's disk valve. Hidden chats and old subagent transcripts get *moved*
  out of the live pool, not deleted, so the picker and both CLIs stop seeing them while every byte
  stays recoverable (`--restore <uuid>` puts one back). Dry-run by default; `--apply` acts. It
  re-checks liveness at run time and refuses to archive a transcript whose chat is still running.

`install.sh --apply` puts all of these in place. `cc-ls` needs `fzf`; nothing else here needs a
dependency beyond `sqlite3` (and the fleet degrades to flat files without it). Pairs naturally with
`/chat:branch` and `/chat:new` for spawn-and-orchestrate teammate workflows.

## Agent mode — takeover vs attach

Claude Code runs a per-account **daemon** that hosts sessions headlessly (`claude daemon`, listed by `claude agents`). A session living there is a **background agent** — "agent mode." Plenty of things breed them: a `claude --bg` run, an RR brainer, a `/chat:new --detach` teammate, a forked `/chat:inject` reply. They have a transcript but **no tmux socket**, so no `/tmp/cc-sid` breadcrumb exists — yet they hold the session lock, so a plain `claude --resume <uuid>` **refuses** (`Session … is currently running as a background agent`) and a naive resume window instantly `[exited]`s.

**A live agent keeps the account, model, effort, and permission-mode it was BORN with.** `cc-swap` (or `/swap`) only affects *new* processes — so merely *attaching* to an agent after an account swap keeps the OLD account, model, and permissions. Reopening such a chat under a freshly-chosen account needs a fresh process, not an attach.

That is why `cc-ls` routes every agent-locked chat (the `⚙` rows, and any resume that refuses because the session is live somewhere the `ps` scan can't see) through **`cc-agent-open.sh`**, a chooser it opens in the new tmux window:

- **`t` takeover** *(default when the agent is idle / blocked / done)* — SIGTERM the agent gracefully (15 s grace, SIGKILL backstop), then `claude --resume` the **same transcript fresh under the current primary account** — the one `cc-swap`/`/swap` last selected. A stale registry entry with no live pid just resumes directly.
- **`a` attach** *(default when the agent is actively working — never kill in-flight work)* — join the running process via `claude agents --cwd` under the agent's **owning** account, keeping its original everything.
- **`q`** cancel.

The default is the safe move for the agent's current state; the founder can override it at the prompt. `cc-agent-open.sh` is **N-account generic** — the current primary comes from the state store `~/.cc/fleet.db` (number N → `~/.claudeN`; 1 → the unset default `~/.claude`), and it probes the agent registry across the default account plus every `~/.claude[0-9]*` config dir on disk, so it finds the agent whichever account owns it. To reclaim a *done* background agent's RAM, take it over and `/bb` it (or kill its pid) — `cc-reap` only sweeps `cc-*` tmux sockets, not bg-agent processes.
