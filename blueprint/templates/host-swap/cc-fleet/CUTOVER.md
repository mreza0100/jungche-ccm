# cc-fleet WP11 cutover and rollback

This is the execution runbook for WP11. WP10 only prepares and tests it.
Nothing in this document is performed until the supervisor explicitly orders
WP11. The gitter agent owns every commit.

## Preconditions

1. `cc-fleet ls --check` has reported an empty or allowlisted-only diff at
   three different times of day.
2. The repository is at the accepted WP10 commit and the working tree has no
   unrelated edits in the three cutover targets.
3. Keep one existing shell open with the legacy functions loaded until the
   verification and rollback drill have completed.

## 1. Build and install the binary

From `~/work/host-ops/cc-fleet`:

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local GOTELEMETRY=off \
  mise exec -- go build -trimpath -o /tmp/cc-fleet.wp11 ./cmd/cc-fleet
file /tmp/cc-fleet.wp11
install -d -m 0755 "$HOME/.local/bin"
install -m 0755 /tmp/cc-fleet.wp11 "$HOME/.local/bin/cc-fleet"
"$HOME/.local/bin/cc-fleet" version
```

`file` must identify a statically linked executable.

## 2. Import legacy hide state

Run once before changing the sourced shell implementation:

```sh
"$HOME/.local/bin/cc-fleet" legacy import
"$HOME/.local/bin/cc-fleet" doctor
```

The import performs a cold full index, imports `.cc-ls-hidden` and `.at`, and
leaves both legacy files untouched.

## 3. Flip the single zsh source line

In `~/.zshrc`, replace only the existing fleet source line:

```zsh
[[ -r "$HOME/work/host-ops/oldbox/scripts/cc-fleet.zsh" ]] && source "$HOME/work/host-ops/oldbox/scripts/cc-fleet.zsh"
```

with:

```zsh
[[ -r "$HOME/work/host-ops/cc-fleet/shim/cc-fleet.zsh" ]] && source "$HOME/work/host-ops/cc-fleet/shim/cc-fleet.zsh"
```

Validate before reloading:

```sh
zsh -n "$HOME/work/host-ops/cc-fleet/shim/cc-fleet.zsh"
zsh -n "$HOME/.zshrc"
```

Then run `reload` in one disposable plain terminal. Do not reload the retained
legacy rollback shell yet.

## 4. Delegate the two hide scripts

Replace `oldbox/scripts/cc-hide.sh` with exactly:

```sh
#!/usr/bin/env bash
exec "$HOME/.local/bin/cc-fleet" hide --self "$@"
```

Replace `oldbox/scripts/cx-hide.sh` with exactly:

```sh
#!/usr/bin/env bash
exec "$HOME/.local/bin/cc-fleet" hide --self "$@"
```

Keep both executable. Their existing `--exit` callers pass through unchanged;
the detached exit choreography now lives in `internal/hide`.

## 5. MCP registration after WP12 acceptance

WP12 implements and tests the stdio-only `cc-fleet mcp` server. Do not
register it while building or testing WP12. At cutover+, after the installed
binary has passed the jailed MCP client suite, register it with Claude Code:

```sh
claude mcp add cc-fleet -- cc-fleet mcp
```

Add the equivalent server to `~/.codex/config.toml`:

```toml
[mcp_servers.cc-fleet]
command = "cc-fleet"
args = ["mcp"]
```

Restart the respective client and verify that `chat_ls`, `chat_resolve`,
`chat_inject`, `chat_capture`, `chat_find`, and `chat_read` are listed.
`chat.sh` remains installed and working as the shell fallback.

## 6. Documentation patch to apply in WP11

Update `oldbox/docs/cc-fleet.md` with these changes:

- State that `~/.zshrc` sources `cc-fleet/shim/cc-fleet.zsh` and that the
  installed binary is `~/.local/bin/cc-fleet`.
- Replace the zsh picker internals with the Go architecture:
  SQLite WAL store, incremental Claude/Codex indexing, read-only live gather,
  pure compose, and Bubble Tea picker.
- Document `ls`, `open`, `index`, `hide`, `unhide`, `hidden`, `resolve`,
  `revive`, `legacy import|export`, `doctor`, and `version`.
- Document `ls --plain`, `--tsv`, and the read-only `--check` shadow command,
  including the reviewed allowlist format.
- Keep the shell surface (`cc`, `cc1`, `cc2`, `cc3`, `cx`, `cc-swap`,
  `cc-ls`, `cc-open`, `cc-revive`, and `vsct-revive`) and explain that picker
  actions are one-line eval output.
- Replace the legacy hide-file behavior with exact SQLite hidden IDs,
  prompt-count baselines, auto-unhide, and `/bb` detached exit handling.
- Retain `/tmp/cc-sid`, `~/.claude-primary`, `cc-swap-chat.sh`, and
  `cc-agent-open.sh` as deliberate satellite contracts.
- Add the rollback steps below and document the optional post-WP12 MCP
  registration commands above.

Do not change this documentation before the WP11 order.

### Shadow comparison semantics

- Claude rows compare by transcript UUID. Crumb and indexed paths may use
  different symlink spellings, so enrichment also joins by UUID.
- Live Codex rows compare in the `cx-*` socket identity space because the
  legacy plain fallback has no stable rollout-ID column.
- A legacy multi-session Codex server is one comparison row keyed by socket
  with a sorted project set. Legacy repeats the first detected rollout's
  prompt metadata for every session, so prompt count is not compared for this
  multi-session shape.
- Projects are compared at the legacy display width of 14 runes.
- Semantic allowlist classes are code-verified: newest-120 membership,
  paired prompt-only differences, composed account ownership, or paired live
  drift. A class name alone cannot suppress a tuple.

## 7. Verification checklist

Run the repository gates:

```sh
cd "$HOME/work/host-ops/cc-fleet"
GOTOOLCHAIN=local GOTELEMETRY=off mise exec -- go vet ./...
GOTOOLCHAIN=local GOTELEMETRY=off mise exec -- go test ./...
bash testdata/e2e.sh
```

Then perform the final live acceptance:

- Warm `cc-fleet ls --plain` completes in under one second wall time.
- `cc-fleet ls --check` is empty or allowlisted-only.
- TUI Ctrl-T, Ctrl-R, and Ctrl-X round trips work.
- `/bb` in a scratch chat hides and exits it; a later real prompt
  auto-unhides it.
- `cc-fleet resolve session <name>` output is identical to `chat.sh`
  `_resolve_session`.
- Enter from a `vsct` bunker replaces the viewport and leaves no husk.
- A blank line appended to the legacy hide file changes nothing.
- The rollback drill below succeeds.

## Rollback

If SQLite-era hides must survive the rollback, export them before flipping
back:

```sh
"$HOME/.local/bin/cc-fleet" legacy export
```

Restore the old `~/.zshrc` source line exactly:

```zsh
[[ -r "$HOME/work/host-ops/oldbox/scripts/cc-fleet.zsh" ]] && source "$HOME/work/host-ops/oldbox/scripts/cc-fleet.zsh"
```

Restore the two version-controlled hide scripts:

```sh
git -C "$HOME/work/host-ops" restore \
  oldbox/scripts/cc-hide.sh \
  oldbox/scripts/cx-hide.sh
```

Run `zsh -n ~/.zshrc`, reload a disposable shell, and verify `whence -f cc-ls`
shows the legacy function. The installed binary and SQLite database may remain
in place; legacy code does not read them.
