# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- pfm: `pfm chat status <target> --ask` — a new flag, and a matching `ask` parameter on the `chat_status` MCP tool, answering what a chat is doing RIGHT NOW rather than recapping what it last did. A collector-tier model (the existing `Ask` config: `claude-haiku-4-5` / `gpt-5.6-luna`, effort low) reads the live tmux pane capture with the last human exchange as background, and returns a bounded ≤40-word verdict: working, waiting on input, blocked, finished, or errored. `--engine` / `--model` now apply to `--ask` as well as `--summary`.

  Unlike `--summary` it never caches, and that is deliberate rather than an omission: `Summarize` keys its cache on transcript offset, which is sound for a finished exchange and wrong for a pane whose contents differ a second later — a cached ask would answer confidently about a chat that has moved on.

  It also refuses to let a failed probe read as an empty one. A chat that is not live, a capture that errored, and a chat with no exchange yet each produce their own distinct wording (`TRANSCRIPT-ONLY (chat is not live: …)`, `TRANSCRIPT-ONLY (pane capture failed: …)`, `PANE-ONLY (no human exchange recorded yet)`), and a missing engine binary says so by name. The answer is never empty, because an empty answer is indistinguishable from a quiet chat.

- pfm: `pfm mcp serve` now honours `mcp.servers.<name>.enabled`. The combined HTTP daemon mounted both the chat and harvester backends unconditionally, so disabling a server in config disabled nothing — `pfm mcp chat disable` left the route answering tool calls exactly as before. Each backend is now constructed and mounted only when it is enabled, gated once at startup rather than re-checked per request.

  A disabled route answers `503` with `pfm mcp: <name> is disabled by config; enable it with: pfm mcp <name> enable`, and deliberately not a `404`: an operator hitting a dark route must be able to tell "you turned this off" from "this daemon is broken or gone". When every registered server is disabled the daemon refuses to start before binding the port rather than serving an empty surface. The startup line now reports `chat=<state> harvester=<state>`, and `/status`'s `Servers` list is built from the handlers actually mounted — it previously named both regardless of what was running, so the status document itself was the thing misreporting the daemon.

  #### → For: anyone who disabled an MCP server in config and saw it keep serving — the setting now takes effect, and that route will answer 503 instead of tool results.

- pfm: `pfm install` stopped reporting backups it never wrote. Four writers logged `rewrite <path> (backup preserved)` unconditionally while the backup itself was guarded by `if existed`, so creating a file that had never been there announced a preserved backup of nothing — and an operator who went looking for that `.bak` after a bad install found no such file. A shared `changeDescription(path, existed)` now derives the wording from the SAME condition that gates the write, and says `create <path>` when nothing was backed up. This closes an observed report: a `pfm install --yes` run logged a backed-up rewrite of `~/.mcp.json` with no `.mcp.json.bak*` anywhere on disk.

- pfm: the fleet TUI footer says `kill` where it still said `hide`. The `⌃X` hint at both widths and the carousel action carried the pre-rename word long after the command itself became `kill`, so the interface taught one verb and answered to another. The persisted `_HIDE` prefix is a wire format and is untouched.
