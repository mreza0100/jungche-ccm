# Every `claude` launch is a pfm chat: a real launcher in front of the binary

Status: QUEUED · Refined: 2026-08-21 by CCC (user ruling: "a headless chat MUST be in a tmux, just like how I open it — this is a defect") · Project: pfm · Fenced wave

## Why (verified 2026-08-21)

`ORCHESTRATOR_RECOVERY` (Claude session `91f2668f…`) ran as pid 3468972, a direct child of a Codex app-server sandbox: `claude --resume 91f2668f… --dangerously-skip-permissions`, no tmux, stdin a pipe. `pfm ls` listed it as an `agent` row; `pfm internal agent-open` probed every `cc-*` socket for that pid (all `exit status 1`), fell back to the `claude agents` takeover view, and failed — nothing can attach to a process that has no pty. The user could see the chat and not open it.

Root cause: pfm's interception of `claude` is the zsh FUNCTION in `pfm/internal/installer/assets/shim/pfm.zsh` — it exists only inside an interactive zsh. `~/.local/bin/claude` is a bare symlink to the real binary (`~/.local/share/claude/versions/{v}`), so a Codex sandbox, a hook, an `exec` from any program, or a non-zsh shell reaches the binary with pfm never in the path. pfm owns the pane only when a human typed the command.

## Contract

**Every interactive `claude` launch on this host runs inside a pfm-owned tmux session** (`cc-{ts}-{pid}-{rand}` socket, same naming/env-scrub/registration as `pfm open` — ONE spawn path, `internal/spawn` + `internal/action`, never a second copy in a shell script), whether the launcher was a terminal, a Codex sandbox, a hook, or `exec`. A caller with a TTY is attached; a caller without one gets the socket + session printed on stdout, and the launcher WAITS for the pane to end and exits with claude's exit status, so fire-and-forget parents (Codex's `exec`) keep their semantics.

## Tasks (inside-out)

### #1 — `pfm internal launch` — the Go side of the launcher

- Anchors: `internal/spawn/types.go:14 SessionSpec`, `internal/spawn/tmux.go:24 NewSession`, `internal/action/solo.go:63` (what counts as a pfm socket), `internal/action/synth.go:411` (the `new-session` command pfm synthesises for a real open — reuse its env-unset list: `CC_ENDPOINT_UNSET` + `CC_SESSION_UNSET` in the shim), `cmd/pfm/main.go:335` (the `internal` verb table).
- New verb `pfm internal launch --real {path-to-real-claude} [--cwd DIR] -- {claude argv…}`:
  1. **Pass-through predicate** (pure function, table-tested): argv is NON-interactive when it contains `-p`/`--print`, `--output-format`, `-h`/`--help`, `--version`/`-v`, or a subcommand in {`agents`, `mcp`, `update`, `install`, `doctor`, `setup-token`, `plugin`, `config`} as its first non-flag token; OR the process is already inside a pfm socket (`$TMUX` path base matches `cc-*`/`cx-*`); OR `PFM_LAUNCH_PASSTHROUGH=1`. Non-interactive → `syscall.Exec` the real binary with argv unchanged (zero added latency, zero behaviour change for SDK/hook/headless callers).
  2. Otherwise build a `SessionSpec` exactly as `pfm open` would for a new Claude chat in `--cwd` (socket naming, `CLAUDE_CONFIG_DIR` from the caller's env or the config primary, env scrub, `FORCE_PROMPT_CACHING_5M=1`, the registry/crumb write so `pfm ls` shows it live immediately), with `Run` = the real binary + argv verbatim (`--resume`, `--dangerously-skip-permissions`, everything).
  3. Caller has a TTY (`term.IsTerminal(stdin)`) → `exec tmux -L {socket} attach`. No TTY → print ONE line `pfm launch: {socket} {session}` to stdout, then `tmux -L {socket} set remain-on-exit on` is NOT used; instead wait via `tmux -L {socket} wait-for {channel}` armed by a `pane-died` hook, read `#{pane_dead_status}`, exit with it. A tmux that cannot start is a loud error on stderr + exit 1 — never a silent fallback to a bare binary.
- Tests (JAIL, RED-first): predicate table; a fake `claude` script under `t.TempDir()` launched with no TTY ends up in a `cc-*` session listed by `pfm ls --plain`; exit status propagates (fake exits 3 → launcher exits 3); `-p` argv execs the fake directly with no tmux server created.

### #2 — install the launcher in front of the binary, survive claude updates

- Anchors: `internal/installer/installer.go:487,1055` (shim asset + rc source line), `installer.go` PATH/bin handling, `cmd/pfm/doctor.go`.
- `pfm install` writes `{managedRoot}/bin/claude` — a tiny POSIX `sh` script: resolves the real binary (config `claude.binary` when it is an absolute non-launcher path; else the newest `~/.local/share/claude/versions/*`; else `command -v claude` skipping itself) and `exec`s `pfm internal launch --real {real} -- "$@"`. Then ensures `~/.local/bin/claude` IS this launcher (replace the symlink; keep a record of the displaced target in `{managedRoot}/launcher.state`) — because PATH order cannot be trusted for processes pfm did not start (the Codex sandbox inherited a PATH with `~/.local/bin` first; that is the hook point).
- The Claude native updater rewrites `~/.local/bin/claude` on every update. Defence: (a) `pfm doctor` row `launcher: ok | DISPLACED by {target} — run pfm install` (HONEST-ABSENCE: a missing `~/.local/bin/claude` is its own named state); (b) the SessionStart hook pfm already installs re-asserts the launcher (idempotent, <5ms when correct) so the window of exposure is one session, not until the next install.
- The zsh function shim keeps its interactive niceties but stops being the ONLY interception: the `claude` function calls the launcher path, never `command claude` directly.
- Tests: install into a jail HOME with a fake `~/.local/bin/claude` symlink → launcher in place, state file records the displaced target; simulate an update (re-point the symlink) → doctor reports DISPLACED; `pfm install` again → restored. e2e harness (`pfm/e2e/install_e2e_test.go`) gains one phase: after install, a no-TTY `claude --version` execs straight through and a no-TTY fake interactive launch lands in a `cc-*` socket.

### #3 — agent rows: say why they cannot be opened

- Anchor: `cmd/pfm/agent_open_command.go` (`probe tmux socket … exit status 1` ×N, then the takeover view).
- When no pfm socket owns the pid, the failure renders ONE visible line in the picker/open path: `⚙ {name}: running outside pfm (pid {pid}, parent {comm}) — no pane to attach; kill {pid} and open the row to resume it in a pane` — never N probe lines and a TTY error. With #2 installed this state should no longer arise for new launches; the message is the HONEST-ABSENCE surface for the ones that predate it.

## Acceptance

- `dev.sh iso test pfm` + `iso verify pfm` green with fence proof; e2e phase green in the container.
- Host proof after the mirror build + `pfm install --yes` (user-run): `env -i HOME=$HOME PATH=$PATH sh -c 'claude --resume {any-sid} </dev/null' &` appears in `pfm ls` as a live `cc-*` chat within 2s and opens from the picker; `claude -p "hi"` still prints without creating a tmux server.
- `pfm doctor` shows `launcher: ok`.
- Walker over the merge candidate with HONEST-ABSENCE + CONFIG-OWNERSHIP armed; LEAK-LINE on the installer assets.

## Out of scope

- Codex (`codex`) launches — separate spec if wanted; the `cx-*` path is unchanged.
- Adopting already-running pane-less processes (impossible without a pty).
