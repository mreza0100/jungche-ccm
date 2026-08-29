# Professor — VSCode tmux Launcher

Make every new VSCode integrated terminal open straight into **tmux + Claude Code**, with a clean fall-back: when you `/exit` Claude, the tmux session ends and you land back at a normal shell — the terminal never closes on you. Universal Tier C mechanic, no domain placeholders.

## What it does

```
New VSCode terminal  ->  tmux session running Claude Code (cc)
/exit Claude         ->  tmux session ends -> back to your shell prompt
Open another         ->  another fresh tmux + Claude
```

## How it works

Two cooperating pieces:

1. **`terminal-profile.json`** — a `tmux + claude` terminal profile (set as default) that launches a login shell carrying a `VSCODE_AUTO_CC=1` env flag.
2. **`zshrc-cc.snippet.sh`** — a `cc` launcher (`claude` inside `tmux new-session`) plus a guard that, on an interactive shell with the flag set, runs `cc` once and **unsets the flag before launching** so the tmux/claude children never re-trigger it.

The `cc` function is `typeset -f`-guarded: if you already define `cc` in your rc, the snippet leaves yours untouched.

## Install (opt-in — edits your *global* editor + shell config)

**1. VSCode `settings.json`** — `Cmd+Shift+P -> Preferences: Open User Settings (JSON)`, then merge the two `terminal.integrated.*` keys from `terminal-profile.json`. On Linux/Windows, replace `osx` with `linux`/`windows`.

**2. Shell rc** — append the snippet to `~/.zshrc`:

```bash
cat zshrc-cc.snippet.sh >> ~/.zshrc
```

**3. tmux config** — copy `tmux.conf` to `~/.tmux.conf` for mouse scroll + drag/double/triple-click copy to the system clipboard (the comfort these terminals assume):

```bash
cp tmux.conf ~/.tmux.conf   # or merge into an existing ~/.tmux.conf
```

Open a new terminal to test. Requires `tmux` and the `claude` CLI on `PATH`. On Linux/Windows, swap the `pbcopy` in `tmux.conf` for your platform's clipboard command.
