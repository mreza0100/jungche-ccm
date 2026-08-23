# Codex `/clear`: the fleet must follow the new thread

**Status:** queued · **Project:** `pfm` · **Lands after:** `train/engine-contract` (it rewrites `clear_kill_command.go`, `gather`, `kill`).

## 0. Preconditions

- Work in a git worktree (`.worktrees/codex-clear`); every build/test through `./.claude/scripts/dev.sh iso {verify|test|all} pfm`. Never `go test` on the host, never write git, never edit `.claude/**` or a `CLAUDE.md`.
- `SchemaVersion` stays 7. No migration: the two new/retired keys live in `meta`, which is derived cache, not operator state.

## 1. The verified failure

Codex 0.149 does not run the pane's session inside the pane. Evidence, host-observed 2026-08-23:

1. The rollout file of a live fleet chat is held open by pid `2569702` = `codex … app-server --listen unix://`, **not** by the pane's `codex` process.
2. `/proc/2569702/environ` carries `CODEX_HOME` and `PWD` and **no `TMUX`, no `TMUX_PANE`**.
3. Therefore `pfm internal clear-kill`, spawned by that daemon, always takes the pane-less branch: `hasPane=false`, `os.Getppid()` = the daemon = the SAME value for every Codex chat on the host. `fleet.db` `meta` holds exactly one such key (`codex_clear_process_2569702`); it is a single global slot.
4. The hook is DEFERRED: `SessionStart(source=clear)` fires on the new session's FIRST TURN (Codex `session.rs` queues `queue_pending_session_start_source`), not when `/clear` is typed. Reproduced with a probe chat: `/clear` alone changed nothing; the next prompt fired the hook.
5. Reproduced end to end: with only one chat starting sessions, the clear-kill landed correctly (`hidden` row with `at_payload=1`). With ANY other Codex chat starting a session in between, the slot holds that chat's thread — pfm then kills the WRONG chat and leaves the cleared one visible.
6. Identity does not follow the clear at all: after `/clear`, `pfm chat resolve <name>` still returns the pre-clear thread, and Codex's own state store shows the new thread with `name` empty while the chat's name stays on the dead one.
7. Ledger at diagnosis time: `~/.cc/fleet.db` `hidden` held 18 Claude clear-kills (non-null `at_payload`) and **0** Codex ones — the single Codex row that exists now is the probe run described in (5).

The hook cannot be repaired in place: nothing in its payload (`session_id`, `cwd`, `transcript_path`, `model`, `permission_mode`, `source`) names a pane, and `cwd` is shared by many chats.

## 2. The anchor: the pane says what it is running

Codex's TUI bottom status line, captured with `tmux capture-pane`:

```
  ENGINE_BUILDER · ~/.professor · Full Access · gpt-5.6-sol xhigh · Context 66% used · …
  01a02e86-f64d-7253-bb68-1b8956cf9fd7 · /tmp · Full Access · gpt-5.6-luna medium · Context 0% used · …
```

Fields are separated by `·` (U+00B7). The first field is the thread NAME, or the raw thread id when the thread is unnamed — and a thread born from `/clear` is always unnamed. pfm already captures panes every gather pass for the Claude 🔖 label (`gather.CaptureClaudeLabels`, fan-out capped at 8): same mechanism, new parser.

## 3. Tasks

### #1 `gather`: read Codex pane identity

New `CaptureCodexIdentity(ctx, capturer, panes)` beside `CaptureClaudeLabels`, returning per pane `{Socket, PaneID, Name, ThreadID, Failed}`:

- Parse the LAST non-empty captured line; split on `·`; trim field 0.
- Field 0 that parses as a thread id (`^[0-9a-f]{8}-[0-9a-f]{4}-…$`) → `ThreadID`; otherwise → `Name`.
- A capture that errored sets `Failed: true`. **`Failed` is not "no identity"** — a failed capture must never be read as "this pane runs nothing".

### #2 `kill`: the pane binding advances instead of being seeded once

- Replace `SeedCodexPane` (writes only when absent) with `AdvanceCodexPane(ctx, socket, pane, threadID) (previous string, changed bool, err error)`.
- Delete `CodexProcessBinding` / `BindCodexProcess` and the `codex_clear_process_` key shape entirely — a host-global slot keyed on a shared daemon pid can only ever be wrong.

### #3 `pipeline`: reconcile drift, and that IS the clear

In `rememberCodexPaneBindings` (rename to `reconcileCodexPanes`), for each live Codex pane:

- Resolve the pane's observed thread: `ThreadID` from #1, else the thread whose `cx_names` row matches `Name` **uniquely**; ambiguous or unresolved → do nothing and say so.
- `previous, changed := AdvanceCodexPane(...)`. When `changed && previous != ""` → `KillClearedCodex(previous)` (prompt-baseline kill, the same auto-unkill ratchet Claude gets).
- Never act on a `Failed` capture. Report the two states in different words: `codex pane %s %s: capture failed (%v)` vs `codex pane %s %s: status line named no thread`.

### #4 `clear_kill_command`: stop guessing

- Delete `runCodexClearKill` and the `SessionStart` branch. The hook keeps ONLY Claude's `SessionEnd(reason=clear)` path.
- `pfm install` drops the Codex `SessionStart` hook from `~/.codex/hooks.json` (`codex_hooks.go`), and the installer's ownership ledger stops claiming it. A hook that cannot identify its own pane must not stay wired.

### #5 `resolve`: the binding outranks the birth guess

`store.NewCodexThreadResolverRoots` matches a pane's thread by process birth, so it returns the PRE-clear thread forever (the TUI process is not restarted). The pane binding, now advanced by #3, must be consulted first for a pane that has one; the birth match stays as the fallback for a pane with no binding.

### #6 the name follows the clear

After a successful #3 kill, re-apply the chat's name to the NEW thread with `spawn.RenameCodex` (the same `/rename` TUI drive `pfm chat new` already uses — Codex has no launch flag for a thread name, so this is the only way). Fire it exactly once per detected clear, and treat `blocked`/`warning` from `nameCodexThread` as a reported non-event, never a retry loop. Do NOT fake the name by writing `cx_names` alone: the fleet and Codex would then disagree about who the chat is.

## 4. Tests (RED first — each must be watched failing)

1. `gather`: status line with a name → `Name`; with a bare id → `ThreadID`; a capture error → `Failed` with neither field set.
2. `kill`: `AdvanceCodexPane` returns the previous binding and `changed=false` for an unchanged write.
3. `pipeline`: bound thread T1, pane now shows T2 → T1 is killed with a prompt baseline and the binding becomes T2. Capture failed → nothing is killed, and the failure is named on stderr.
4. `pipeline`: two panes in the same cwd, one clears → only the clearing pane's thread is killed (this is the regression that the process-slot design could not pass).
5. `resolve`: a pane with a binding resolves to the bound thread even when a newer thread exists in the same cwd.
6. `installer`: after `pfm install`, `~/.codex/hooks.json` carries no `SessionStart` entry, and `pfm doctor` says so rather than reporting a hook that is not there.

## 5. Acceptance

- `dev.sh iso verify pfm` and `dev.sh iso test pfm` green, fence line quoted.
- Manual, on the host after the merge: spawn two Codex chats in the same cwd, `/clear` one, prompt both, and show `pfm ls` — the cleared chat's old row is gone, the other chat is untouched, and `pfm chat resolve` on the cleared chat returns the NEW thread.
- `grep -rn "codex_clear_process_" pfm/` returns nothing.

## 6. Out of scope

Claude's `/clear` path (`SessionEnd`) — it works and is not touched. The OpenCode engine. Any change to `SchemaVersion` or the shared store.
