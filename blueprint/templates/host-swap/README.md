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
