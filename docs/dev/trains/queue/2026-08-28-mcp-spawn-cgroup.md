# Chats spawned via the MCP service die with the service

## The incident, observed live (2026-08-28 ~02:16)

`pfm-mcp.service` (systemd user unit, Main PID 120571) dispatched five
`chat new` spawns (luna-chat-1..5). Their tmux servers — sockets
`cx-17878748xx-120571-xxxxx`, the creator PID visible in the names — were
daemonized tmux processes but REMAINED IN THE SERVICE'S CGROUP. A
`systemctl --user restart pfm-mcp.service` (routine: picking up a rebuilt
binary) cgroup-killed all five chats along with the server.

Aftermath was honest but noisy: `pfm ls` emitted one probe warning per
orphaned pane ("capture failed — the pane was not read; this is not an idle
pane") until the index caught up and the dead sockets cleared; no data loss
beyond the killed chats themselves.

## The law this violates

A chat IS a terminal the user lives in. Its lifetime must never be coupled to
the lifetime of whatever process happened to dispatch its creation. tmux
daemonization does not escape a systemd cgroup; KillMode=control-group (the
default) reaps daemonized grandchildren on every restart.

## Fix shape (to refine before building)

- Spawn path (`internal/spawn/`, and whatever the MCP backend's CLI dispatch
  reaches it through): when running inside a systemd user service, launch the
  chat's tmux server outside the service cgroup — `systemd-run --user
--collect --scope` around the tmux invocation is the clean mechanism; a
  setsid alone does NOT leave the cgroup.
- Detection must be honest: only take the systemd-run path when actually
  under a service (INVOCATION_ID / cgroup inspection), and fail LOUDLY if
  systemd-run is unavailable there — a chat silently born mortal is this bug
  again.
- Consider the unit too (`internal/installer/assets/` pfm-mcp.service):
  KillMode=process would stop the bleeding but leaks the rest of the cgroup
  semantics — prefer the spawn-side scope; note the choice in the unit file
  either way.
- Jail test: spawn under a fake-cgroup/systemd-run stub and assert the
  invocation wraps; REAL-SESSION verification (restart the service, chats
  survive) is named in TESTPLAN.md as unjailable, per law.

## Status

Complete on branch `engine-fixes`. A Linux chat spawn carrying
`INVOCATION_ID` wraps only tmux server creation in
`systemd-run --user --collect --scope`, scrubs the inherited invocation marker,
and refuses loudly when `systemd-run` is unavailable. The wrapper and refusal
regressions were watched failing before the fix; the full fenced pfm suite
passes. A real `pfm-mcp.service` restart-survival probe remains unjailable and
is named in `pfm/TESTPLAN.md`.
