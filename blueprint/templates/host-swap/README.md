# Professor — Multi-Account Billing Swap

Run two or three Claude subscriptions and switch which one a given chat bills to — **without disturbing any other running session**. Universal Tier C mechanic, no domain placeholders.

> **macOS only.** The mechanism relies on the macOS Keychain (`security` CLI). Linux/Windows users would need to substitute a different credential store.

## What it does

```
cc          ->  launch Claude in tmux, billing to your primary account
cc2         ->  launch Claude in tmux, billing to account 2
/swap       ->  switch THIS chat's billing account (others untouched)
/swap 2     ->  jump straight to account 2
cc-swap     ->  change the default "primary" account for future cc launches
```

Each `cc` launch gets its own **private Keychain credential** seeded from your chosen account's master token. `/swap` rewrites only that session's private item — other sessions keep their own. The change takes effect on the next message (~30 s Keychain cache).

## How it works

Claude Code keys its macOS Keychain credential by the `CLAUDE_CONFIG_DIR` path:

```
item name = "Claude Code-credentials-" + first_8_hex(sha256(config_dir_path))
           (unsuffixed when CLAUDE_CONFIG_DIR is unset — the default ~/.claude session)
```

Each **account** has a **master config dir** (`~/.claude`, `~/.claude2`, `~/.claude3`) with its own master Keychain item seeded once via `/login`. Each **launch** creates a fresh **per-session dir** under `~/.claude-sessions/` — a symlink shell over the chosen master (so settings, commands, and projects stay shared) — and seeds a private Keychain item from the master token. That private item is what `/swap` rewrites.

```
~/.claude          master dir, account 1
~/.claude2         master dir, account 2
~/.claude3         master dir, account 3 (optional)
~/.claude-sessions/s20250101-120000-12345/
  ├── (symlinks → master dir files)
  ├── .claude.json -> master .claude.json
  └── account      "<num> <label> <email>"   ← read by /swap + statusline
```

The `account` marker file (`<num> <label> <email>`) is written by the launcher and updated by `/swap`. The statusline badge (🥇/🥈/🥉) reads it on every refresh.

## Files in this template

| File | Destination | Purpose |
|------|-------------|---------|
| `cc-launch.sh` | `~/.claude/bin/cc-launch.sh` | Create per-session dir + seed Keychain + write marker |
| `cc-account-swap.sh` | `~/.claude/bin/cc-account-swap.sh` | Per-chat swap: cycle / jump / status |
| `zshrc-swap.snippet.sh` | append to `~/.zshrc` | `cc`/`cc1`/`cc2`/`cc3` launchers, `cc-swap` picker, `cc-clean` |
| `swap.command.md` | `~/.claude/commands/swap.md` | `/swap` slash command |
| `statusline-badge.snippet.sh` | merge into `~/.claude/statusline-command.sh` | 🥇/🥈/🥉 account badge |
| `cc-ls.snippet.sh` | append to `~/.zshrc` | `cc-ls` — fzf picker over every live + resumable chat (Enter attaches a live tmux, or resumes a transcript in a fresh tmux) |
| `cc-hide.sh` | `~/.claude/bin/cc-hide.sh` | `/bb`'s engine — hide this chat from `cc-ls` then close it; **pane-aware**: kills only its own pane (never the shared server) and reaps the teammates it spawned |
| `cc-reap.sh` | `~/.claude/bin/cc-reap.sh` | reclaim RAM from the `cc-*` socket graveyard (dry-run by default; `--kill` reaps unattached orphans + stale socket files) |
| `cc-agent-open.sh` | `~/.claude/bin/cc-agent-open.sh` | `cc-ls`'s Enter-target for a chat locked by a live background agent: the takeover/attach chooser (take it over fresh under the current primary account, or attach to the running agent) |
| `bb.command.md` | `~/.claude/commands/bb.md` | `/bb` slash command — bye-bye: hide + close this chat (and any detached `/chat:new --detach` teammates it spawned) |

## Install (opt-in)

**1. Edit the account table first** — see the section below. Every file has a clearly marked `# EDIT: your accounts` block. Update those blocks before running any script.

**2. Create master config dirs** for accounts 2 and 3 if you haven't already, then seed their master Keychain items:

```bash
# Seed account 1 (default ~/.claude — already set if you use CC normally)
# Seed account 2:
CLAUDE_CONFIG_DIR="$HOME/.claude2" claude   # then /login inside CC
# Seed account 3 (optional):
CLAUDE_CONFIG_DIR="$HOME/.claude3" claude   # then /login inside CC
```

**3. Install scripts:**

```bash
mkdir -p ~/.claude/bin
cp cc-launch.sh ~/.claude/bin/cc-launch.sh
cp cc-account-swap.sh ~/.claude/bin/cc-account-swap.sh
chmod +x ~/.claude/bin/cc-launch.sh ~/.claude/bin/cc-account-swap.sh
```

**4. Append the shell snippet:**

```bash
cat zshrc-swap.snippet.sh >> ~/.zshrc
source ~/.zshrc
```

**5. Install the `/swap` command:**

```bash
cp swap.command.md ~/.claude/commands/swap.md
```

**6. Add the account badge to your statusline** (if you use the Professor statusline):

Merge the block from `statusline-badge.snippet.sh` into `~/.claude/statusline-command.sh` just before the `# ── LINE 1` section (where `badge` is referenced). The badge variable is already consumed by the `l1` line — you just need the computation block.

**7. Test:**

```bash
cc1   # should open CC billing to account 1
# inside CC:  /swap 2    → confirm "now bills to account 2"
# in another terminal:  cc2   → independent session, unaffected
```

## Editing the account table

Every file contains a block like this (keyed by a `# EDIT: your accounts` comment):

```
# num  label       master dir      master Keychain item suffix   email
#  1   work        ~/.claude       (none — unsuffixed default)   you@work.example
#  2   personal    ~/.claude2      <8-hex suffix>                you@personal.example
#  3   third       ~/.claude3      <8-hex suffix>                you3@example
```

**How to find your master Keychain item suffixes** — run this for each master dir:

```bash
printf '%s' "$HOME/.claude2" | shasum -a 256 | cut -c1-8   # → suffix for account 2
printf '%s' "$HOME/.claude3" | shasum -a 256 | cut -c1-8   # → suffix for account 3
```

(Account 1 uses the unsuffixed item `Claude Code-credentials` because it runs without `CLAUDE_CONFIG_DIR` set.)

Then update the `case` blocks in `cc-launch.sh`, `cc-account-swap.sh`, `zshrc-swap.snippet.sh`, and `swap.command.md` with your real labels, emails, master dirs, and suffix values.

**Two-account setup:** remove all `3)` cases and references to account 3. Works fine with just 1 and 2.

## Maintenance

- **Re-seed a master token** (if Anthropic rotates it): launch the plain master (`CLAUDE_CONFIG_DIR=~/.claude2 claude`) and `/login` again. Existing per-session items will fail on next swap — relaunch with `cc2 -c` to continue the chat under a fresh session.
- **Prune old session dirs:** `cc-clean` (default: prune dirs older than 7 days) removes both the dir and its private Keychain item.

## Fleet management — `cc-ls`, `/bb`, `cc-reap`

The pieces above launch and bill chats; these manage the resulting fleet. They are **launcher-agnostic** — they work with any setup that runs each chat in its own `tmux -L cc-*` socket (the swap snippet above is one such launcher) and a statusline that writes a `/tmp/cc-sid/<socket>` → transcript breadcrumb (`chmod 700` — the breadcrumbs are transcript paths and the name cache carries prompt text, not for other uids). The Professor statusline template (`blueprint/templates/statusline/statusline-command.sh`) already writes this breadcrumb, so installing it alongside these pieces is enough — no separate wiring needed.

- **`cc-ls`** — one fzf list of every chat: `●` live tmux sessions (Enter attaches), `↻` resumable transcripts with no live tmux (Enter resumes in a fresh tmux), and `⚙` live background/forked agents — a `claude --bg` session, an RR brainer, a `/chat:new --detach` teammate — which have no tmux socket, so `--resume` refuses them; Enter instead opens the **takeover/attach chooser** (`cc-agent-open.sh`) — take the agent over fresh under the current primary account, or attach to the running process (see § Agent mode — takeover vs attach). `⌃T` re-sorts recent⇄prompts, `⌃R` rotates the project on top, `⌃X` hides⇄shows a chat. `cc-ls -a` shows all; `cc-ls --hidden` shows only hidden. **Auto-unhide:** a hidden chat that gets new activity (a size baseline recorded lazily, and finalized by `/bb`) drops off the hide list and reappears on its own — hiding is for finished chats, not a way to silence a still-live one.
- **`/bb`** (bye-bye) — hide THIS chat from `cc-ls` and close it. Pane-aware: it kills only its **own** pane (so a chat spawned beside others via `/chat:branch` or `/chat:new` never drops its neighbours) and reaps the teammates it spawned (pane teammates by `kill-pane`, detached `--detach` teammates by `kill-server`). Identifies the chat by `$CLAUDE_CODE_SESSION_ID`, so it never hides the wrong transcript on a shared socket. Closing types a real `/exit` into the pane (flushes the transcript, runs Stop hooks) and **polls** for the pane to close itself — up to 20s, since compaction can outlive a fixed grace — before a `kill-pane` backstop, then records the post-exit transcript size as the auto-unhide baseline so the `/exit` flush itself is never mistaken for new activity.
- **`cc-reap`** — reclaim RAM from orphaned `cc-*` servers (a closed terminal tab detaches the client but leaves the server + its `claude` node alive). Dry-run report by default; `cc-reap --kill` reaps unattached orphans and removes stale socket files. KEEP guards protect more than just attached chats: a `cc-new-*` detached teammate is kept as `mate` (headless by design — its parent's `/bb` reaps it), and any session `claude agents --json` reports **busy** is kept as `busy` (deliberately detached, still grinding — socket maps to session via the `/tmp/cc-sid` breadcrumb, scanned across every configured account). Never touches an attached chat or your own socket.

**Install (opt-in):**

```bash
mkdir -p ~/.claude/bin
cp cc-hide.sh cc-reap.sh cc-agent-open.sh ~/.claude/bin/ && chmod +x ~/.claude/bin/cc-hide.sh ~/.claude/bin/cc-reap.sh ~/.claude/bin/cc-agent-open.sh
cat cc-ls.snippet.sh >> ~/.zshrc && source ~/.zshrc      # needs fzf
cp bb.command.md ~/.claude/commands/bb.md                 # /bb
```

`cc-ls` needs `fzf`. `/bb`, `cc-reap`, and `cc-agent-open` need no extra dependency. `cc-ls` invokes `cc-agent-open.sh` at `~/.claude/bin/cc-agent-open.sh`, so keep it there (the path is baked into `cc-ls.snippet.sh`). Pairs naturally with `/chat:branch` and `/chat:new` (the chat: command family) for spawn-and-orchestrate teammate workflows.

## Agent mode — takeover vs attach

Claude Code runs a per-account **daemon** that hosts sessions headlessly (`claude daemon`, listed by `claude agents`). A session living there is a **background agent** — "agent mode." Plenty of things breed them: a `claude --bg` run, an RR brainer, a `/chat:new --detach` teammate, a forked `/chat:inject` reply. They have a transcript but **no tmux socket**, so no `/tmp/cc-sid` breadcrumb exists — yet they hold the session lock, so a plain `claude --resume <uuid>` **refuses** (`Session … is currently running as a background agent`) and a naive resume window instantly `[exited]`s.

**A live agent keeps the account, model, effort, and permission-mode it was BORN with.** `cc-swap` (or `/swap`) only affects *new* processes — so merely *attaching* to an agent after an account swap keeps the OLD account, model, and permissions. Reopening such a chat under a freshly-chosen account needs a fresh process, not an attach.

That is why `cc-ls` routes every agent-locked chat (the `⚙` rows, and any resume that refuses because the session is live somewhere the `ps` scan can't see) through **`cc-agent-open.sh`**, a chooser it opens in the new tmux window:

- **`t` takeover** *(default when the agent is idle / blocked / done)* — SIGTERM the agent gracefully (15 s grace, SIGKILL backstop), then `claude --resume` the **same transcript fresh under the current primary account** — the one `cc-swap`/`/swap` last selected. A stale registry entry with no live pid just resumes directly.
- **`a` attach** *(default when the agent is actively working — never kill in-flight work)* — join the running process via `claude agents --cwd` under the agent's **owning** account, keeping its original everything.
- **`q`** cancel.

The default is the safe move for the agent's current state; the founder can override it at the prompt. `cc-agent-open.sh` is **N-account generic** — the current primary comes from `~/.claude-primary` (number N → `~/.claudeN`; 1 → the unset default `~/.claude`), and it probes the agent registry across the default account plus every `~/.claude[0-9]*` config dir on disk, so it finds the agent whichever account owns it. To reclaim a *done* background agent's RAM, take it over and `/bb` it (or kill its pid) — `cc-reap` only sweeps `cc-*` tmux sockets, not bg-agent processes.
