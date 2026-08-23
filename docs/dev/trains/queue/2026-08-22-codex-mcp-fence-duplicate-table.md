# `pfm mcp <server> enable` must refuse to duplicate a hand-defined Codex `[mcp_servers.X]`

Status: QUEUED · Refined: 2026-08-22 by CCC (reproduced on the host: `pfm mcp harvester enable` appended the installer-owned `[mcp_servers.harvester]` fence to `~/.codex/config.toml` while a hand-written `[mcp_servers.harvester]` already existed → `Error loading config.toml: duplicate key` → every Codex launch died at birth) · Project: pfm · Fenced wave

## Why (verified)

- The installer's Codex `mcp_servers` fence writer (`# BEGIN pfm mcp_servers — installer-owned`) appends tables by name without scanning the rest of the file. `internal/codexgen/mcp.go:56-68` already has exactly this conflict scan for the repo-level fence (`CONFLICT … hand-defined outside the generated fence`) — a second writer without it.
- `pfm chat new --engine cx` then reports only `the chat died at birth on socket … exit status 1` (`spawn/spawn.go:192-210`): the pane's stderr (`duplicate key` at `config.toml:342`) is lost because the pane exits before the first capture — the death has no stated reason (HONEST-ABSENCE).

## Tasks

### #1 — one conflict rule for both fence writers
- Before writing the installer fence, scan `config.toml` outside the fence for `^\[mcp_servers\.<name>(\.|\])`; on a hit, refuse with `pfm mcp <name> enable: [mcp_servers.<name>] is hand-defined at config.toml:<line> — remove it or run with --replace-hand-defined` (exit 1, file untouched). `--replace-hand-defined` removes the hand block (backup `config.toml.bak-<ts>`) and writes the fence. Reuse the `codexgen` scan (export it), never a second regex.
- `pfm doctor` row: `doctor: codex config.toml parse=ok` / `broken error=<toml error>` (delegate to the engine: `codex --version` does not parse config; use a TOML decode in Go — the same one `codexgen` uses).
- RED-first (JAIL): plant the hand block, run enable → refusal line + unchanged file; `--replace-hand-defined` → one table, parses; doctor on a duplicate-key file → `broken` row.

### #2 — died-at-birth carries the pane's last words
- `spawn.go` `waitForBoot`: start the pane with `remain-on-exit on` for the boot window (tmux ≥1.8 option), so a dead pane still captures; on death, include the last 5 non-empty captured lines in the error (`the chat died at birth on socket …: <last lines>`), then kill the dead pane. Off after boot settles (today's semantics).
- RED-first (JAIL): a fake engine that prints `boom: bad config` and exits 1 → the error names `boom: bad config`; watched failing on HEAD where it says only `exit status 1`.

## Acceptance
- `dev.sh iso test pfm` + `iso verify pfm` green with fence proof; host proof: with a planted duplicate, `pfm mcp harvester enable` refuses by name; `pfm chat new --engine cx` on a broken `config.toml` says why.
