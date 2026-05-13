# Professor Statusline

A two-line Claude Code statusline synthesized from 13+ community projects. Ships as part of the Professor pipeline — universal, no domain placeholders needed.

## What it shows

```
Line 1: ◆ Opus │ myproject │ 🌳 worktree │ 🌿 main +2 ~1 │ ⚡agent-name
Line 2: 🟢 ▓▓▓░░░░░░░ 28% │ 💰$3.42 │ +156 -23 │ ⏱ 5m32s │ ▓▓░░░ 5h:42% ↻2h0m
```

**Line 1 — Identity:** model (with tier symbol), directory, worktree, git branch + staged/modified, agent name, vim mode. All conditional — only shows what's active.

**Line 2 — Metrics:** context bar with urgency emoji, cost, lines changed, duration, rate limits with reset countdown. Cost hidden at $0, rate limits hidden for non-Pro/Max.

## Features

- **Single `jq` call** with unit-separator IFS split — one subprocess for all 15 fields
- **ANSI 16-color + bold** — max Claude Code compatibility (truecolor broken since v2.1.78)
- **Emoji, not Nerd Fonts** — Nerd Font PUA glyphs broken in CC's TUI
- **`▓/░` progress bars** — community standard, 10-char context + 5-char rate limit
- **Emoji urgency escalation** — 🟢 safe → ⚡ moderate → 🔥 high → 🚨 critical
- **Model tier symbols** — ◆ Opus ◇ Sonnet ○ Haiku ● other
- **Git cache (5s TTL)** — md5-keyed `/tmp` files prevent subprocess spam
- **Rate limits from stdin** — reads CC's v2.1.80+ JSON directly, no API call
- **Cost thresholds** — dim < $2, yellow $2-10, red $10+
- **Reset prepend** — `\033[0m` prefix fights CC's dimColor wrapper bug

## Install

```bash
cp statusline-command.sh ~/.claude/statusline-command.sh
```

Add to `~/.claude/settings.json`:

```json
{
  "statusLine": {
    "type": "command",
    "command": "bash ~/.claude/statusline-command.sh",
    "padding": 0,
    "refreshInterval": 10,
    "hideVimModeIndicator": true
  }
}
```

Requires `jq` and `git`.

## Research

Built from analysis of: daniel3303/ClaudeCodeStatusLine, fredrikaverpil/claudeline, sirmalloc/ccstatusline, Owloops/claude-powerline, vtmocanu/cc-statusline, Mohamed3on gist, jtbr gist, wmoto-ai gist, Oh My Posh Claude segment, plus Starship/Powerlevel10k/Oh My Posh design patterns and 13 Claude Code GitHub issues on rendering constraints.
