# pfm (Professor-Fleet-Manager) TESTPLAN — the complete flow matrix

Every pathway through the fleet tooling: the Go engine (`pfm/`), the embedded zsh launcher shim,
the `pfm chat` surface and its two-line compatibility delegate, the MCP server, the history
helper, and the self-installer. One row per flow.

Go paths are relative to `~/.professor/pfm/` (the engine lives at the repo root — it is a
program, not a template); embedded helper paths live under `internal/installer/assets/`.
Absolute paths are absolute.

## Legend — the SAFETY column

| Token              | Meaning                                                                                                                                                                                                                                                                                                                                                         |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `JAIL`             | Fully exercisable in a path jail: `paths.go:11-21` env overrides (`PFM_HOME`, `PFM_DB`, `PFM_SHARED_DB`, `PFM_SID_DIR`, `PFM_CLAUDE_ROOTS`, `PFM_CODEX_ROOT`, `PFM_TMUX_DIR`, `PFM_PROC_ROOT`) plus `PFM_TEST_NOW_NS` / `PFM_TEST_FRESH_SOCKET` / `PFM_CODEX_AVAILABLE` / `PFM_DB_SCRIPT` (`pipeline.go:25-30`). Reference harness: `pfm/testdata/e2e.sh:1-56`. |
| `JAIL+tmux`        | Needs a real tmux **server on a scratch socket inside the jail's `TMUX_TMPDIR`** — never a live fleet socket. Reference harness: `internal/*/tmux_jail_test.go` (e.g. `internal/hide/tmux_jail_test.go:58-66`).                                                                                                                                                 |
| `JAIL+sh`          | zsh/bash fixture with a fake `$HOME`, scratch db and an EMPTY socket dir. References: `shim/shim_test.go` and `internal/installer/installer_test.go`. Window-name convergence is covered by the Go `probe-*` tmux jails.                                                                                                                                          |
| `LIVE-READ`        | Read-only observation of live state (`pfm ls --tsv`, `--plain`, `--hidden`, `doctor`, `sqlite3 -readonly`). Never mutates.                                                                                                                                                                                                                                      |
| **`REAL-SESSION`** | ⚠ **CANNOT be jailed.** Needs a genuine `claude` / `codex` process, a real transcript/rollout writer, or a real Claude-account login. The supervisor must schedule these deliberately on a scratch project directory.                                                                                                                                           |

## Legend — the REGRESSION column

The four identity/state regressions that established this plan. A tagged row must retain a fixture.

- **B1** — resumed store-only codex thread renders the wrong name.
- **B2** — workflow twin threads: a hide resurrects as a doppelgänger row.
- **B3** — an agent-row hide is a no-op.
- **B4** — a rename that lands only in `~/.codex/session_index.jsonl` never reaches the picker.

---

## Table of contents

1. [A — `pfm` CLI subcommands and flags](#a--pfm-cli-subcommands-and-flags)
2. [B — Picker TUI: key bindings and model state](#b--picker-tui-key-bindings-and-model-state)
3. [C — Row-kind × operation cross-matrix](#c--row-kind--operation-cross-matrix)
4. [D — Index, naming and identity resolution](#d--index-naming-and-identity-resolution)
5. [E — Hide / unhide and shared state](#e--hide--unhide-and-shared-state)
6. [F — Action synthesis and launch](#f--action-synthesis-and-launch)
7. [G — MCP server (7 tools)](#g--mcp-server-7-tools)
8. [H — `pfm chat`: subcommands, guards, `--then`, exit codes](#h--pfm-chat-subcommands-guards---then-exit-codes)
9. [I — zsh shell surface: launchers and revivers](#i--zsh-shell-surface-launchers-and-revivers)
10. [J — Internal wiring and store](#j--internal-wiring-and-store)
11. [K — Installer and systemd units](#k--installer-and-systemd-units)
12. [Flows that CANNOT be jailed](#flows-that-cannot-be-jailed)
13. [Residual known divergences](#residual-known-divergences)
14. [Highest-risk joins](#highest-risk-joins)

---

## A — `pfm` CLI subcommands and flags

| flow                                                                                                                  | safety    | expected behavior (source)                                                                                | regression |
| --------------------------------------------------------------------------------------------------------------------- | --------- | --------------------------------------------------------------------------------------------------------- | ---------- |
| no args → the same interactive picker as `pfm ls`                                                                     | JAIL+tmux | `main.go`, `attach_e2e_test.go`                                                                           |            |
| unknown subcommand → usage, rc 2                                                                                      | JAIL      | `main.go:62-66`                                                                                           |            |
| `help` / `-h` / `--help` → usage on stdout, rc 0                                                                      | JAIL      | `main.go:59-61`                                                                                           |            |
| `version` → `pfm <version>`; extra arg → rc 2                                                                         | JAIL      | `main.go:95-106`                                                                                          |            |
| local `pfm.dev` is absent or carries the current `hostops/pfm/cmd/pfm` build path                                     | JAIL      | `cmd/pfm/dev_binary_test.go`                                                                              |            |
| `ls` (interactive) → BubblePicker on `/dev/tty`, cached first frame then streamed refresh                             | JAIL+tmux | `commands.go:101-121`, `ui/picker.go:12-68`                                                               |            |
| `ls --plain` → PlainPicker, one pass, rc 0                                                                            | JAIL      | `commands.go:88-100,126-128`                                                                              |            |
| `ls --tsv` → TSVPicker, stable rows                                                                                   | JAIL      | `commands.go:96-99`; golden `testdata/golden/ui.tsv`                                                      |            |
| `ls -a/--all` → AllView                                                                                               | JAIL      | `commands.go:33-34,60-65`                                                                                 | B2         |
| `ls -H/--hidden` → the hide ledger (`id`, engine, timestamp), noninteractive                                          | JAIL      | `commands.go`, `main_test.go`                                                                             | B2, B3     |
| `ls --all --hidden` → rc 2 (mutually exclusive)                                                                       | JAIL      | `commands.go:43-48`                                                                                       |            |
| `ls --plain --tsv` (or any two renderers) → rc 2                                                                      | JAIL      | `commands.go:44` (`boolCount`)                                                                            |            |
| `ls <id>` → same as `chat open <id>`                                                                                  | JAIL      | `commands.go`                                                                                             |            |
| `ls <id>` combined with any flag → rc 2                                                                               | JAIL      | `commands.go:53-56`                                                                                       |            |
| `chat open <target>` on an unindexed id → `is not indexed`, rc 1                                                      | JAIL      | `chat_command.go`, `commands.go`                                                                          | B3         |
| `chat open <target>` whose CWD vanished → falls back to `$PWD` for non-live kinds                                     | JAIL      | `commands.go`                                                                                             |            |
| `chat new --name X [prompt]` (or positional X) → chat on a fresh immutable fleet socket, rc 0                         | JAIL+tmux | `run_command.go`, `action/headless.go`, `spawn/spawn.go`; `run_jail_test.go`                              |            |
| `chat new` without a name → rc 2; unknown `--engine` or `--effort` → rc 2                                             | JAIL      | `headless_matrix_test.go`                                                                                 |            |
| `chat new --engine cx` → thread named through Codex's own `/rename` UI, THEN prompted                                 | JAIL+tmux | `spawn/spawn.go` (`nameCodexThread`)                                                                      |            |
| `chat new --engine cx` on a Codex without `/rename` → live chat, `UNNAMED`, warning, rc 1, composer cleared           | JAIL+tmux | `TestRunReportsACodexBuildThatCannotBeRenamed`                                                            |            |
| Codex boots through startup overlays (hooks review, trust) — Escape, then a composer that HOLDS, before a keystroke   | JAIL+tmux | `TestCodexBootsThroughStartupModals`, `TestCodexComposerFlashBeforeAModalIsNotReadiness`                  |            |
| A startup screen that never clears → nothing typed, chat live, `UNNAMED`, rc 1                                        | JAIL      | `TestCodexStuckAtAStartupScreenTypesNothing`                                                              |            |
| The rename field is CLEARED before typing (Codex pre-fills it with the current name)                                  | JAIL      | `TestCodexThreadIsRenamedThenPrompted` (clear token)                                                      |            |
| The rename Enter is re-sent until the status line carries the name; zero typing gap DROPS it                          | JAIL      | `spawn/types.go` (`orDefaults`), `spawn/spawn.go` (`confirmPresses`)                                      |            |
| `chat new --prompt-file PATH` → prompt read from the file; with an inline prompt → rc 2                               | JAIL      | `TestRunPromptSourcesAreExclusive`                                                                        |            |
| `chat new --model M --effort E` → engine-specific model and effort arguments                                          | JAIL      | `TestModelAndEffortReachBothEngines`                                                                      |            |
| `chat new` on a chat that dies at birth → `died at birth`, rc 1                                                       | JAIL      | `spawn/spawn.go` (`waitForBoot`)                                                                          |            |
| `chat status <target> [--json]` → state/idle_seconds/engine/model/cwd/session_id/context_pct                          | JAIL      | `headless/headless.go`; `headless_test.go`                                                                |            |
| working vs idle comes from the TRANSCRIPT (assistant spoke last = idle), never from a timer                           | JAIL      | `TestStateComesFromTheTranscriptNotAClock`                                                                |            |
| `chat read <target> [--tail N] [--condensed] [--json]`                                                                | JAIL      | `transcript/transcript.go`; `transcript_test.go`                                                          |            |
| `chat last <target>` → the last assistant message, bare                                                               | JAIL      | `TestLastFindsTheNewestAssistantTurn`                                                                     |            |
| `chat stream <target> [--filter RE] [--margin N]` → follow prompts and replies                                        | JAIL      | `TestStreamFilterWithMargin`                                                                              |            |
| `chat stream` ends when the chat dies instead of hanging on a quiet file                                              | JAIL      | `TestStreamFollowEndsWhenTheChatDies`                                                                     |            |
| `chat inject <target> <message>` → guarded delivery with `--file`, `--force-now`, and repeated `--then`               | JAIL+tmux | `headless_command.go`, `internal/inject`                                                                  |            |
| `chat ask <target> <message>` → deliver, wait, print the answer on stdout alone                                       | JAIL+tmux | `ask_command.go`, `headless/converse.go`; `TestAskHoldsATwoWayConversation`                               |            |
| `ask` answers the question just asked — the frontier is taken BEFORE delivery, so an older answer cannot be returned  | UNIT      | `TestAwaitReturnsTheAnswerToTheQuestionJustAsked`                                                         |            |
| `ask` waits through tool work: an assistant line followed by a tool call is a preamble, not an answer                 | UNIT      | `TestAwaitWaitsThroughToolWork`                                                                           |            |
| `ask --timeout N` → rc 5 with the message still DELIVERED (a different fact from unheard)                             | JAIL+tmux | `TestAskReportsATimeoutWithoutLosingDelivery`                                                             |            |
| `ask --json` → name/delivered/answer/state/tools/waited_seconds; `--progress` → condensed turns on stderr             | JAIL+UNIT | `headless/converse.go`; `TestAwaitStreamsProgress`                                                        |            |
| `ask` on a WORKING chat re-offers the message until it lands (inject rc 7) instead of aborting; `--now` interrupts    | LIVE      | `ask_command.go` (`busyRetry`, `inject.CodeBusy`)                                                         |            |
| `ask` reports `superseded` when a second human turn lands mid-wait — the answer may be theirs                         | UNIT      | `TestAwaitFlagsAnAnswerSomebodyElseMayOwn`                                                                |            |
| `--timeout` bounds the WHOLE exchange: time spent waiting for a busy chat is spent from the same budget               | JAIL      | `ask_command.go` (`remaining`)                                                                            |            |
| `ask` on a chat that dies mid-answer keeps what it said, then reports it gone                                         | UNIT      | `TestAwaitKeepsWhatADyingChatManagedToSay`                                                                |            |
| `chat new` with a prompt PROVES delivery from the engine transcript; unproven → rc 6 + attach hint, chat left running | JAIL+tmux | `awaitLaunch`; `TestRunRefusesToCallAnUnheardPromptDelivered`                                             |            |
| `chat new --await` → the first answer on stdout, launch summary moved to stderr                                       | JAIL+tmux | `TestRunAwaitsTheAnswerItAskedFor`                                                                        |            |
| The prompt's Enter is re-sent until the composer releases it; a composer that never does → `Prompted=false` + warning | UNIT      | `TestCodexPromptIsResentUntilItLeavesTheComposer`, `TestCodexPromptThatNeverSubmitsIsReportedUndelivered` |            |
| A wait re-reads the transcript every poll but re-scans the fleet only every `ResolveEvery`                            | UNIT      | `headless/converse.go` (`ResolveEvery`)                                                                   |            |
| `transcript.From` never consumes a half-written record, and restarts when the file shrinks                            | UNIT      | `TestFromHoldsAPartialLine`, `TestFromRestartsWhenTheFileShrinks`                                         |            |
| `chat watch <target>` → `IDLE`/`EXIT`/`DEAD` lines + `--on-idle`/`--on-exit` hooks                                    | JAIL      | `TestWatchAnnouncesIdleThenExit`, `TestWatchReportsAChatThatVanishesAsDead`                               |            |
| `chat ls [--all]` → live-seat listing from the Go gather/compose pipeline                                             | JAIL      | `chat_satellite_command.go`, `chat_satellite_command_test.go`                                             |            |
| An unknown name → rc 4 with `not-found` on stdout, on EVERY verb — never empty at rc 0                                | JAIL      | `TestUnknownChatIsRc4WithAMachineShape`                                                                   |            |
| A dead (non-live) chat → rc 3, explicitly, never silence                                                              | JAIL      | `headless_command.go` (`codeDeadChat`)                                                                    |            |
| Flags parse before or after the target (`status seat --json`)                                                         | JAIL      | `main.go` (`parseFlagsAnywhere`); `TestChatArgumentMatrix`                                                |            |
| `chat inject` takes its message verbatim — a leading dash is not a flag                                               | JAIL      | `runHeadlessInject`                                                                                       |            |
| hidden `headless` compatibility alias prints a deprecation to stderr                                                  | JAIL      | `TestHeadlessCompatibilityAliasIsHiddenAndDeprecated`                                                     |            |
| Name resolution: name, id-prefix, socket; live row beats its resume twin; ambiguity refused                           | JAIL      | `TestChatMatchingPrefersTheLiveSeat`                                                                      |            |
| `index` → counters line on stdout                                                                                     | JAIL      | `commands.go:351-398`, `formatCounters` `commands.go:400-412`                                             | B4         |
| `index --full` → reparse all + `last_full_index_at` meta                                                              | JAIL      | `commands.go:386-392`                                                                                     | B4         |
| `index --progress` → start/elapsed on stderr                                                                          | JAIL      | `commands.go:378-379,394-396`                                                                             |            |
| `revive` → resume-only rows through PlainPicker; empty → "no resumable chats"                                         | JAIL      | `commands.go:414-458`                                                                                     | B2         |
| `hide <id>` → shared row + carrier line                                                                               | JAIL      | `main.go:108-146`, `hide/manager.go:69-111`                                                               | B3         |
| `hide --self` → identity from `$TMUX`/`$TMUX_PANE`/`$CLAUDE_CODE_SESSION_ID`                                          | JAIL+tmux | `main.go:114`, `hide/self.go:16-38`                                                                       | B3         |
| `hide --self --exit` → detached finisher; `--exit` without a live pane → error                                        | JAIL+tmux | `main.go:115`, `hide/manager.go:86-88,98-109`                                                             |            |
| `hide` arg-shape violations (`--self` with an id, no id without `--self`) → rc 2                                      | JAIL      | `main.go:119-123`                                                                                         |            |
| `unhide <id>` → canonicalizes a Codex id to its lineage root first                                                    | JAIL      | `main.go:148-168`, `hide/manager.go:113-131`                                                              | B2         |
| `hidden` → `id\tengine\thidden_at` per row                                                                            | JAIL      | `main.go:207-215`                                                                                         | B3         |
| `hidden --prune-orphans` → dry-run report; `--yes` deletes; `--yes` alone → rc 2                                      | JAIL      | `main.go:176-192`, `commands.go:463-511`                                                                  |            |
| `chat resolve <target>` → socket/session/id tuple; missing target rc 4                                                | JAIL+tmux | `chat_command.go`, `resolve/resolve.go`                                                                   |            |
| `resolve` with a bad kind → rc 2                                                                                      | JAIL      | `resolve/resolve.go:92-93`, `main.go:241-244`                                                             |            |
| `whoami` → this process's own tmux session name                                                                       | JAIL+tmux | `main.go:53`, `resolve/whoami.go:165-213`                                                                 |            |
| `doctor` → db + jail health                                                                                           | LIVE-READ | `main.go:39-40`; `internal/store/health.go`                                                               |            |
| `mcp` → stdio server; any arg → rc 2                                                                                  | JAIL      | `main.go:69-93`                                                                                           |            |
| `internal hide-exit --engine…` → detached finisher; missing flags → rc 2                                              | JAIL+tmux | `main.go:250-299`, `hide/finisher.go:94-144`                                                              |            |
| `internal then …` → steer waiter                                                                                      | JAIL+tmux | `main.go:251-253`, `inject/then.go`                                                                       |            |

| `reap` dry run classifies every socket, changes nothing | JAIL+tmux | `cmd/pfm/reap_jail_test.go:134-189`, `internal/reap/reap.go:139-160` | |
| `reap` KEEP rules: attached, self, `cc-new-*`, busy, transcript written < 60s | JAIL | `internal/reap/reap_test.go:14-200` | |
| `reap` never kills a socket hosting non-chat processes (dev servers, `uv`) | JAIL+tmux | `internal/reap/proc.go:78-110`, `cmd/pfm/reap_jail_test.go:134-189` | |
| `reap` exempts a chat's OWN subtree (its MCP servers, its tool shells) | JAIL | `internal/reap/proc_test.go:86-97` | |
| `reap` fails closed when the busy query fails or a `cc-*` crumb is missing | JAIL | `internal/reap/reap_test.go:118-146` | |
| `reap` empty socket younger than 1h left alone (a server may be starting) | JAIL+tmux | `cmd/pfm/reap_jail_test.go:220-260` | |
| `reap` never reaps from AGE alone; a wedged socket times out into SKIP | JAIL | `internal/reap/reap_test.go:203-224`, `internal/reap/runner.go:33-38` | |
| `reap --apply` re-verifies attachment at kill time and clears crumbs | JAIL+tmux | `internal/reap/runner.go:320-360` | |
| `reap` sweeps idle `vsct` bunkers by SESSION, never by killing the server | JAIL | `internal/reap/reap_test.go:258-296` | |
| `archive` dry run default; `--apply`, `--subagents`, `--older-than`, `--restore` | JAIL | `internal/archive/archive_test.go:85-204` | |
| `archive` re-checks liveness at run time (argv, codex fds, sid crumbs) | JAIL | `internal/archive/live.go:20-95`, `archive_test.go:296-340` | |
| `archive` retires every decided hide, live ones included | JAIL | `internal/archive/archive.go:196-240` | |
| `archive` prunes history.jsonl and the codex index for MOVED chats only | JAIL | `internal/archive/archive_test.go:180-200` | |
| `archive --restore` puts a chat back and refuses an unknown id | JAIL | `internal/archive/archive_test.go:228-262` | |
| `heal` report: CAUGHT_UP / CONSISTENT / WEDGED / MIDLINE / NO_ROLLOUT totals | JAIL | `internal/heal/heal_test.go:170-240`, `cmd/pfm/heal_jail_test.go:108-150` | |
| `heal` report is read-only; `--apply` heals only broken threads, backup first | JAIL | `internal/heal/heal_test.go:243-320` | |
| `heal` skips a thread whose writer lock is held, and heals it once released | JAIL | `internal/heal/heal_test.go:322-380` | |
| `heal --thread` is a silent exit-0 no-op on a healthy thread | JAIL | `cmd/pfm/heal_jail_test.go:152-176` | |
| ResumeCodex runs the native projection repair before the seat is created | JAIL | `internal/action/executor.go:113-127`, `testdata/golden/cmdlines.txt:47-54` | |
| `name-sync` converges both engines' window names; `--dry-run` changes nothing | JAIL+tmux | `internal/gather/labels_jail_test.go:14-90` | |
| `bb` blocks a bare `/bb` (rc 2) and passes every other prompt through | JAIL | `cmd/pfm/bb_jail_test.go:12-100` | |
| `bb` fails OPEN on unreadable stdin, non-JSON, or a missing prompt field | JAIL | `cmd/pfm/bb_jail_test.go:12-52` | |

## B — Picker TUI: key bindings and model state

| flow                                                                              | safety    | expected behavior (source)                               | regression |
| --------------------------------------------------------------------------------- | --------- | -------------------------------------------------------- | ---------- |
| `⌃T` reload → `OutcomeReload`, loop re-scans with `ForceFull`                     | JAIL      | `ui/model.go:144-147`, `commands.go:143-145`             |            |
| `⌃R` rotate project groups, keeps cursor on the followed row                      | JAIL      | `ui/model.go:148-153`, `ui/model.go:334-352`             |            |
| `⌃R` with zero groups → rotation pinned 0, no panic                               | JAIL      | `ui/model.go:336-339`                                    |            |
| `⌃X` toggle hide, in-memory only until quit                                       | JAIL      | `ui/model.go:154-156,273-297`                            | B3         |
| `⌃X` on a split row or an ID-less row → no-op                                     | JAIL      | `ui/model.go:279-281`                                    | B3         |
| `⌃X` twice back to the initial state → change dropped, not re-written             | JAIL      | `ui/model.go:290-295`                                    |            |
| `⌃X` then quit → `applyPickerChanges` writes each change                          | JAIL      | `commands.go:296-315`                                    | **B3**     |
| `⌃X` on an agent row whose transcript is not indexed → **must not silently fail** | JAIL      | `compose/compose.go:564-567` + `hide/manager.go:142-164` | **B3**     |
| `⌃E` flip 1h cache for the next launch                                            | JAIL      | `ui/model.go:157-159`                                    |            |
| `⌃S` cycle primary account `n%MaxAccount+1`                                       | JAIL      | `ui/model.go:160-162`, `action/synth.go:19`              |            |
| `⌃S` change persisted through the shared store on exit                            | JAIL      | `commands.go:133-138`, `pipeline.go:578-631`             |            |
| `⌃O` reboot a live row → kill-server, drop crumbs, demote to resume kind          | JAIL+tmux | `ui/model.go:166-172`, `commands.go:317-349`             |            |
| `⌃O` on a non-live row → no-op                                                    | JAIL      | `ui/model.go:167-171`                                    |            |
| `⌃O` on a `LiveSplit` (no id) → error, never a half-reboot                        | JAIL      | `commands.go:323-325`                                    |            |
| `Enter` → `OutcomeSelected` with the row; empty filter → no-op                    | JAIL      | `ui/model.go:173-179`                                    |            |
| `Esc` / `⌃C` → cancelled, rc 0, no writes                                         | JAIL      | `ui/model.go:180-182`, `commands.go:162-163`             |            |
| `↑/⌃P`, `↓/⌃N`, `PgUp/PgDn`, `Home/End` bounds                                    | JAIL      | `ui/model.go:183-207`, `pageRows` `ui/model.go:429-431`  |            |
| Typing filters: substring hit, then fuzzy subsequence                             | JAIL      | `ui/model.go:354-399`, `runeSubsequence:401-416`         |            |
| `Backspace`/`⌃H`, `⌃W` (word), `⌃U` (clear)                                       | JAIL      | `ui/model.go:210-224`                                    |            |
| Paste message appends to the query                                                | JAIL      | `ui/model.go:131-133`                                    |            |
| Query longer than `CharLimit` 200 → clipped, never panics                         | JAIL      | `ui/model.go:80,228-236`                                 |            |
| Live refresh mid-picker preserves cursor, query and pending hides                 | JAIL      | `ui/model.go:247-271`                                    | B2         |
| Refresh that DROPS the selected row → cursor falls back safely                    | JAIL      | `ui/model.go:390-399`                                    | B2         |
| Window resize re-widths the query field                                           | JAIL      | `ui/model.go:123-127`                                    |            |
| Footer legend matches the real bindings                                           | JAIL      | `ui/render.go:122-125`                                   |            |

## C — Row-kind × operation cross-matrix

Each row is one **session kind** crossed with the operations that touch it. This is the table
tonight's four bugs all live in.

| flow (kind × op)                                                                                                   | safety           | expected behavior (source)                                                       | regression |
| ------------------------------------------------------------------------------------------------------------------ | ---------------- | -------------------------------------------------------------------------------- | ---------- |
| **live-claude** — one pane crumb → one row, name from indexed title else pane title                                | JAIL             | `compose/compose.go:279-354,356-399`, `naming/naming.go:23-44`                   |            |
| **live-claude, socket crumb only** (no pane crumb) → row only if a claude process holds the socket                 | JAIL             | `compose/compose.go:337-346`                                                     |            |
| **live-claude split** (≥2 pane crumbs) → one `LiveSplit` row, names joined `a+b`                                   | JAIL             | `compose/compose.go:327-329,401-480`                                             |            |
| **live-claude, two servers one transcript** → collapsed, `ServerCount` = n, newest socket wins                     | JAIL             | `compose/compose.go:759-803`                                                     |            |
| **live-codex rollout-backed** → fd-walk finds the rollout, pane must exist in the SAME snapshot                    | JAIL             | `gather/codexproc.go:61-75`, `compose/compose.go:492-501`                        |            |
| **live-codex store-only, fresh** → identity via `CODEX_THREAD_ID`, else cwd+birth ≤120s                            | JAIL             | `gather/codexproc.go:76-87`, `resolve/codex.go:35-87`                            | B1         |
| **live-codex store-only, resumed** → birth window cannot match; row must not vanish or mislabel                    | **REAL-SESSION** | `resolve/codex.go:49-85` (returns an error → `codexproc.go:82-84` drops the row) | **B1**     |
| **live-codex, exported thread id unknown to the store** → `RolloutPath` is `""` → row ID becomes `"."`             | JAIL             | `resolve/codex.go:42-48` → `compose/compose.go:502-513,834-847`                  | **B1, B2** |
| **live-codex whose thread is archived in Codex** → filtered out of candidates, same `""` path                      | JAIL             | `store/codexstate.go:161-163`                                                    | B1, B2     |
| **resume-claude** → transcript row, capped 30 in DefaultView                                                       | JAIL             | `compose/compose.go:70-108`, `store/queries.go:35-59`                            |            |
| **resume-codex** → one row per lineage, keyed by `RootID`, newest member supplies path/mtime                       | JAIL             | `compose/compose.go:110-148`, `store/lineage.go:22-81`                           | B2         |
| **store-only resumable** (thread with no rollout file) → row created with placeholder path                         | JAIL             | `index/codexstate.go:98-130`                                                     | B1, B2     |
| **store-only resumable, resumed** → `LineageRoot` is set to its OWN id, so it does NOT join its ancestor's lineage | JAIL             | `index/codexstate.go:120-129` (no `SessionID`/`ParentThread` set)                | **B1, B2** |
| **twin threads from one conversation** → must collapse to ONE row; a hide on either must cover both                | JAIL             | `store/lineage.go:22-81`, `compose/compose.go:660-673`                           | **B2**     |
| **agent row** → live claude under a NON-primary `CLAUDE_CONFIG_DIR` carrying `--session-id`/`--resume`             | JAIL             | `gather/agents.go:12-69`                                                         | B3         |
| **agent row, transcript not indexed** → row synthesized from the session id alone                                  | JAIL             | `compose/compose.go:564-567`                                                     | **B3**     |
| **agent row already live as a chat** → suppressed, never doubled                                                   | JAIL             | `compose/compose.go:559-562,568`                                                 |            |
| **workflow / SDK background (claude)** → `IsBG`, hidden from DefaultView, visible under `-a`                       | JAIL             | `index/claude.go:41-44,83-95`, `compose/compose.go:725-736`                      | B2         |
| **workflow / SDK background (codex sub-thread)** → `thread_source != user` → not `Listed()`                        | JAIL             | `store/codexstate.go:44-46,252`                                                  | B2         |
| **squatter socket** (a session whose name ≠ its socket) → never a chat row                                         | JAIL+tmux        | `check_command.go:181-203`; naming `gather/labels.go:36-56`                      |            |
| **vsct bunker chat** → `vsct`/`revive` sockets excluded from the probe; open uses `exec`                           | JAIL+tmux        | `gather/tmuxprobe.go:356-361`, `pipeline.go:666-668`, `action/synth.go:290-310`  |            |
| **hidden row** → excluded from DefaultView, counted in `HiddenCount`, shown under `-H`                             | JAIL             | `compose/compose.go:660-673,708-716,738-747`                                     | B2, B3     |
| **empty row** (size 0 / 0 prompts) → suppressed from DefaultView, counted in `SuppressedCount`                     | JAIL             | `compose/compose.go:725-736`                                                     |            |
| **both accounts** — a row's account comes from the longest matching config root                                    | JAIL             | `compose/compose.go:865-887`, `pipeline.go:527-542`                              |            |
| **primary switch** — db meta first, `~/.claude-primary` mirror second, off-roster → 1                              | JAIL             | `shared/shared.go:441-482`, `pipeline.go:553-559`                                |            |
| **cache badge on/off** — `C1H` from `/proc` env of the live process, per socket                                    | JAIL             | `gather/cache1h.go`, `compose/compose.go:378,413,586`                            |            |

## D — Index, naming and identity resolution

| flow                                                                                              | safety | expected behavior (source)                                                  | regression |
| ------------------------------------------------------------------------------------------------- | ------ | --------------------------------------------------------------------------- | ---------- |
| Incremental skip: size+mtime unchanged → skipped                                                  | JAIL   | `index/index.go:337-348`                                                    |            |
| Delta parse: file grew and a prior parse exists                                                   | JAIL   | `index/index.go:350-366`                                                    |            |
| Row with `parsed_offset` 0 (store-created) → FULL parse, never a delta                            | JAIL   | `index/index.go:354-366`                                                    | B1         |
| Parser-version bump forces a full reparse for each engine                                         | JAIL   | `index/index.go:96-116`                                                     |            |
| `PriorityCWD` sorts this project's transcripts first                                              | JAIL   | `index/index.go:288-335`                                                    |            |
| `PriorityOnly` pass skips codex, deletes and cx-names entirely                                    | JAIL   | `index/index.go:88-93,193,210,239`                                          |            |
| Deleted transcript/rollout is pruned only in a full pass                                          | JAIL   | `index/index.go:208-225`                                                    |            |
| Codex state store: newest `state_<N>.sqlite` wins per thread id                                   | JAIL   | `store/codexstate.go:48-107,114-139`                                        |            |
| Codex state store opened `mode=ro`, never `immutable` (WAL visibility)                            | JAIL   | `store/codexstate.go:182-193`                                               | B4         |
| Older state schema missing columns → `COALESCE` fallback, never a failed pass                     | JAIL   | `store/codexstate.go:266-309`                                               |            |
| Unreadable state generation is SKIPPED, never blanks the Codex half                               | JAIL   | `store/codexstate.go:119-123`                                               |            |
| Store-vouched thread survives the prune that removes file-less rows                               | JAIL   | `index/codexstate.go:45-52`                                                 | B2         |
| `applyCodexThread` only takes store content when the row has no parsed bytes                      | JAIL   | `index/codexstate.go:79-96`                                                 | B1         |
| `reloadCxNames` truncates and rebuilds `cx_names` from `session_index.jsonl` on size/mtime change | JAIL   | `index/cxindex.go:22-93`                                                    | **B4**     |
| `reconcileCodexNames` applies store names AFTER the file rebuild — store outranks file            | JAIL   | `index/index.go:256-265`, `index/codexstate.go:132-179`                     | **B4**     |
| Rename made only in `session_index.jsonl` while the store holds an older `name`                   | JAIL   | conflict between `index/cxindex.go:51-63` and `index/codexstate.go:149-159` | **B4**     |
| `CxName` lineage walk: own id → session id → parent thread → first prompt                         | JAIL   | `naming/naming.go:88-106`                                                   | **B1**     |
| `CxName` on a ≥3-deep lineage whose NAME sits on the root                                         | JAIL   | `naming/naming.go:97-105` vs root from `compose/compose.go:644-658`         | **B1**     |
| `DisplayName` precedence: custom title → AI title → first prompt                                  | JAIL   | `naming/naming.go:10-18`                                                    |            |
| `LiveFallback` for cc sockets uses pane title, never the generated session name                   | JAIL   | `naming/naming.go:23-44`                                                    |            |
| Junk-prompt filter (`<x…`, `Caveat:`, `[Request`) and compact-summary skip                        | JAIL   | `naming/naming.go:46-56`, `index/claude.go:52-56`                           |            |
| Lineage cycle in the parent chain → lowest id in the cycle becomes root                           | JAIL   | `store/lineage.go:83-128`                                                   | B2         |
| `ReconcileCodexLineageRoots` denormalizes roots after each full pass                              | JAIL   | `store/lineage.go:201-236`                                                  | B2         |

## E — Hide / unhide and shared state

| flow                                                                                | safety    | expected behavior (source)                                                                         | regression |
| ----------------------------------------------------------------------------------- | --------- | -------------------------------------------------------------------------------------------------- | ---------- |
| A hide writes one SQLite row and never recreates the retired carrier                | JAIL      | `shared/shared.go`, `shared/shared_test.go`                                                         | B2         |
| `HiddenAt` returns SQLite rows or reports the lookup failure                        | JAIL      | `shared/shared.go`, `shared/shared_test.go`                                                         | B2         |
| Persistent `SQLITE_BUSY` on hide → retry, warn, count, return the write error       | JAIL      | `store/hidden.go:43-79`                                                                            |            |
| Persistent `SQLITE_BUSY` on unhide follows the same nonzero failure contract       | JAIL      | `store/hidden.go:43-79`                                                                            |            |
| Engine derived from the index, not stored; transcript wins a collision              | JAIL      | `store/hidden.go:171-241`                                                                          | B3         |
| Hidden id no index knows → empty engine = "hidden whatever the engine"              | JAIL      | `store/hidden.go:171-177`, `compose/compose.go:664-668`                                            | B3         |
| `applyHide` skips split rows and ID-less rows                                       | JAIL      | `compose/compose.go:660-663`                                                                       | B2         |
| Hide is PERMANENT — a growing prompt count never un-hides                           | JAIL      | `compose/compose.go:669-671`; `compose/compose_test.go`                                            |            |
| `hide <id>` for an id in NEITHER table → `chat %q is not indexed`, rc 1             | JAIL      | `hide/manager.go:138-165`                                                                          | **B3**     |
| Picker hide failure surfaces as `pfm ls` rc 1, not a silent drop                    | JAIL      | `commands.go:129-132,296-315`                                                                      | **B3**     |
| `unhide` maps a Codex member id to its lineage root before deleting                 | JAIL      | `hide/manager.go:113-131`                                                                          | B2         |
| Hide by lineage ROOT vs a RAW rollout id written by an older writer                 | JAIL      | `compose/compose.go:116,645`, `compose/compose_test.go:904-950`                                    | **B2**     |
| `--exit` finisher: `/exit` (cc) or `/quit` (cx), poll, kill-pane, sweep crumbs      | JAIL+tmux | `hide/finisher.go:104-131`                                                                         |            |
| Post-exit re-hide keeps the ORIGINAL `hidden_at`                                    | JAIL      | `hide/finisher.go:149-167`                                                                         |            |
| Teammate reap: `new` → kill-server, `pane` → kill-pane, never kill-server           | JAIL+tmux | `hide/finisher.go:181-245`                                                                         |            |
| Teammate reap falls back to the flat file when the table has no row                 | JAIL      | `hide/finisher.go:256-290`                                                                         |            |
| Two concurrent pickers hiding different chats → both survive (WAL + transaction)    | JAIL      | `shared/shared.go:135-157`, `shared/shared_test.go`                                                |            |
| A `_HIDE…` LABEL hides a chat with no store row, live or resumable, either case     | JAIL      | `naming.LabelHidden`, `compose/compose.go` (`applyHide`); `compose/label_hide_test.go`             |            |
| A label-hidden chat still shows under `-a` and `-H`, and counts as hidden           | JAIL      | `compose/label_hide_test.go`                                                                       |            |
| A split row is never label-hidden — its name is a join, not a label                 | JAIL      | `compose/label_hide_test.go`                                                                       |            |
| Label-hidden rows never spend a cached-frame candidate slot (30/15)                 | JAIL      | `store/queries.go` (`labelHiddenSQL`, `codexLineageLabelHidden`); `store/label_candidates_test.go` |            |
| Picker hide key refuses a label-hidden row — renaming is the unhide                 | JAIL      | `ui/model.go` (`toggleHidden`); `ui/label_hide_test.go`                                            |            |

## F — Action synthesis and launch

| flow                                                                                  | safety           | expected behavior (source)                              | regression |
| ------------------------------------------------------------------------------------- | ---------------- | ------------------------------------------------------- | ---------- |
| `NewClaude` → `(cd -- <cwd> && CC_ARM_1H=… cc<N>)`                                    | JAIL             | `action/synth.go:83-93`                                 |            |
| `NewCodex` → `(cd -- <cwd> && cx)`                                                    | JAIL             | `action/synth.go:94-98`                                 |            |
| `Live` → `TMUX= tmux -L <sock> attach -t <target>`                                    | JAIL             | `action/synth.go:99-107,300-310`                        |            |
| Live codex target is `session:window`, falling back to session then socket            | JAIL             | `action/synth.go:312-322`                               | B1         |
| Live codex window name verified against the live server before use                    | JAIL+tmux        | `action/executor.go:92-98,139-156`                      | B1         |
| `ResumeClaude` → resume, with the agent router as the `                               |                  | ` fallback                                              | JAIL       | `action/synth.go:144-173` |     |
| `Agent` → agent router first, fresh resume as the `                                   |                  | ` fallback                                              | JAIL       | `action/synth.go:108-143` | B3  |
| `ResumeCodex` → detached server created BEFORE the attach line is emitted             | JAIL+tmux        | `action/synth.go:174-191`, `action/executor.go:128-135` |            |
| Bunker (`$TMUX` socket basename `vsct`) → `exec` prefix on every launch line          | JAIL             | `action/synth.go:290-310`, `pipeline.go:666-668`        |            |
| Primary account outside `1..MaxAccount` → refuses to synthesize                       | JAIL             | `action/synth.go:59-65`, `action/synth.go:19`           |            |
| NUL in any row value → refuses                                                        | JAIL             | `action/synth.go:66-79`                                 |            |
| Launch hygiene strips inherited identity, cache, and endpoint state                   | JAIL             | `action/synth.go:21-37`                                 |            |
| Autonomy flags appended AFTER the resume argument, never duplicated on fresh launches | JAIL             | `action/synth.go:39-50,215-242`                         |            |
| Dead live socket at open time → demoted to a resume in a fresh server                 | JAIL+tmux        | `action/executor.go:64-87`                              |            |
| Dead live socket on a SPLIT row (no id) → hard error                                  | JAIL             | `action/executor.go:68-73`                              |            |
| Self-switch: already on the target server → select the engine window, emit NO line    | JAIL+tmux        | `action/executor.go:223-278`                            |            |
| Engine-window choice: exact `claude`/`codex` → `node`/version → lowest index          | JAIL             | `action/executor.go:264-278`                            |            |
| Open gate: birth account/cache ≠ picker → offer reboot; failure attaches as-is        | **REAL-SESSION** | `action/executor.go:158-207`, `action/gate.go`          |            |
| Reboot-to-match invokes `pfm chat swap --sock … <acct> --1h <0\|1>`                   | JAIL+Go          | `action/executor.go:190-199`                            |            |
| `_cc_solo` stray-claude sweep before an attach/resume                                 | JAIL             | `action/executor.go:208-215`, `action/solo.go`          |            |

## G — MCP server (7 tools)

> The brief says 8 tools. There are **7**: `register()` at `mcpserv/server.go:65-103`.

| flow                                                                              | safety    | expected behavior (source)                                         | regression |
| --------------------------------------------------------------------------------- | --------- | ------------------------------------------------------------------ | ---------- |
| Tool roster is exactly 7 with correct read-only / mutating annotations            | JAIL      | `mcpserv/server.go:65-103`                                         |            |
| `chat_ls` default view, `all`, `hidden`, `project` filter                         | JAIL      | `mcpserv/backend.go:82-253`                                        | B2         |
| `chat_ls` with both `all` and `hidden` → error                                    | JAIL      | `mcpserv/backend.go:83-85`                                         |            |
| `chat_ls` state: `busy` / `idle` / `dead` / `resumable` per row                   | JAIL+tmux | `mcpserv/backend.go:181-225`, `inject.IsBusy`                      |            |
| `chat_ls` treats an **agent** row as live and captures its pane                   | JAIL+tmux | `mcpserv/backend.go:303-308` (diverges from `ui/model.go:467-471`) | B3         |
| `chat_ls` split row folds every pane's state into one verdict                     | JAIL+tmux | `mcpserv/backend.go:184-208`                                       |            |
| `chat_ls` engine label for a `NewCodex`/agent kind                                | JAIL      | `mcpserv/backend.go:317-322`                                       |            |
| `chat_ls` primary account agrees with the picker                                  | JAIL+sh   | `mcpserv/backend.go:272-279` vs `pipeline.go:553-559`              |            |
| `chat_resolve` kind validation (`label`/`session`/`cxwin`)                        | JAIL      | `mcpserv/server.go:114-124`                                        |            |
| `chat_resolve` status mapping 0/1/2 → `ok`/`not_found`/`ambiguous`                | JAIL+tmux | `mcpserv/server.go:126-142`                                        |            |
| `chat_inject` full guard chain, mirrors chat.sh                                   | JAIL+tmux | `mcpserv/server.go:145-172`, `inject/engine.go:296-663`            |            |
| `chat_inject` `/compact` without `then` → refused code 6                          | JAIL      | `inject/engine.go:683-694`                                         |            |
| `chat_inject` a `then` steer that is itself `/compact` → refused code 1           | JAIL      | `inject/engine.go:668-682`                                         |            |
| `chat_inject` long `/compact` focus → refused code 6                              | JAIL      | `inject/engine.go:696-708`                                         |            |
| `chat_inject` codex inline cap by RUNE count                                      | JAIL+tmux | `inject/engine.go:327-336`                                         |            |
| `chat_inject` absolute byte cap before anything resolves                          | JAIL      | `inject/engine.go:301-310`                                         |            |
| `chat_capture` `tail_lines` 1..1000, `max_bytes` 1..4Mi, rune-safe tail cut       | JAIL+tmux | `mcpserv/server.go:209-271`                                        |            |
| `chat_whoami` takes NO arguments; identity from this process only                 | JAIL+tmux | `mcpserv/server.go:177-207`, `mcpserv/types.go:95-97`              |            |
| `chat_whoami` failure returns `not_found` + message, never an error               | JAIL      | `mcpserv/server.go:186-195`                                        |            |
| `chat_find` needle extraction identical to chat.sh's awk pass                     | JAIL      | `mcpserv/search.go:33-68` vs `chat.sh:455-456`                     |            |
| `chat_find` excludes the asking session unless `include_self`                     | JAIL      | `mcpserv/search.go:70-98` vs `chat.sh:462`                         |            |
| `chat_read` bounds: `last_n` ≤200, `max_bytes` ≤1Mi, Claude and Codex turn shapes | JAIL      | `mcpserv/read.go:18-21,49-70,199-273`                              |            |

## H — `pfm chat`: subcommands, guards, `--then`, exit codes

| flow                                                                                                                 | safety    | expected behavior (source)                                                  | regression             |
| -------------------------------------------------------------------------------------------------------------------- | --------- | --------------------------------------------------------------------------- | ---------------------- |
| root `whoami [--label]` → this chat's immutable socket identity or display label                                     | JAIL+tmux | `whoami_command.go`, `whoami_test.go`                                       |                        |
| `chat new NAME` → detached chat on a fresh immutable `cc-*`/`cx-*` socket                                            | JAIL+tmux | `run_command.go`, `run_jail_test.go`                                        |                        |
| `chat new NAME --attach` → launch, then attach this terminal; `--await --attach` → rc 2                              | JAIL+tmux | `run_command.go`, `attach_e2e_test.go`                                      |                        |
| `chat open <name                                                                                                     | socket    | id                                                                          | self>` → attach action | JAIL+tmux | `chat_command.go`, `attach_e2e_test.go` |     |
| `chat inject` ladder: self → any-socket live session → label → Codex thread → id/path/excerpt                        | JAIL+tmux | `headless_command.go`, `inject_resume.go`, `internal/inject/resolve`        | B1                     |
| busy Codex and Claude inject through the safe composer queue without Esc; receipts distinguish queued from delivered | JAIL+tmux | `internal/inject`, `inject_cli_jail_test.go`, `engine_test.go`              |                        |
| `chat inject --file` uses bracketed paste, preserving long/multi-line/shell-syntax bodies past the Codex inline cap  | JAIL+tmux | `internal/inject`, `inject_cli_jail_test.go`                                |                        |
| repeated `chat inject --then` waits busy→stable-idle and survives caller exit                                        | JAIL+tmux | `internal/inject/then.go`, waiter jail tests                                |                        |
| `/compact` without `--then`, or a `/compact` steer, is refused before delivery                                       | JAIL      | `internal/inject`                                                           |                        |
| `--force-now` interrupts only a busy live target and marks the forced delivery                                       | JAIL+tmux | `internal/inject`                                                           |                        |
| signature: `/`-prefixed commands travel bare; plain text carries the sender identity                                 | JAIL+tmux | `internal/inject`                                                           |                        |
| `chat ask` delivers, waits and prints only the answer; timeout remains rc 5                                          | JAIL+tmux | `ask_command.go`, `internal/headless/converse.go`                           |                        |
| `chat read`, `last`, `stream`, `status`, and `watch` preserve transcript/state semantics                             | JAIL      | `headless_command.go`, `internal/headless`, `internal/transcript`           |                        |
| `chat capture` resolves the target, requires a live pane, and prints full scrollback                                 | JAIL+tmux | `chat_command.go`                                                           |                        |
| `chat name` sends `/rename`, then converges that exact pane's window in the same process                             | JAIL+tmux | `chat_command.go`, `chat_name_jail_test.go`                                 | B4                     |
| `chat hide` / `unhide` resolve names through the store; `self` shares the picker/BB path                             | JAIL+tmux | `chat_command.go`, `hide_cli_engine_jail_test.go`                           | B2, B3                 |
| `chat bb` and `chat group hook` drain stdin and fail open at rc 0                                                    | JAIL      | `bb_jail_test.go`, `chat_command.go`                                        |                        |
| `chat end` kills only the resolved chat server                                                                       | JAIL+tmux | `chat_command.go`                                                           |                        |
| `chat find`, `save`, `load`, `branch`, and `ls` are native Go; `history` retains its helper contract                 | JAIL+sh   | `chat_satellite_command.go`, `chat_satellite_command_test.go`, `history.sh` |                        |
| `chat group {create,ls,send,read,subscribe,invite,hook}` is the complete group bus                                   | JAIL      | `chat_command.go`, `chat_group_command.go`                                  |                        |
| `chat resolve <target>` prints immutable socket, tmux session and chat id                                            | JAIL      | `chat_command.go`                                                           | B1                     |
| exit contract: 0 delivered/queued, 2 usage, 3 dead, 4 unknown, 5 answer timeout, 6 undelivered                       | JAIL      | `headless_command.go`, `headless_matrix_test.go`, `inject_cli_jail_test.go` |                        |
| hidden root compatibility alias emits a deprecation; `run`, `dump`, and the old stream verb are gone                 | JAIL      | `main.go`, `headless_matrix_test.go`                                        |                        |
| embedded `chat.sh` is an executable two-line compatibility delegate to `pfm chat`                                    | JAIL+sh   | `internal/installer/assets/chat/chat.sh`, installer tests                   |                        |

## I — zsh shell surface: launchers and revivers

The embedded `internal/installer/assets/shim/pfm.zsh` owns fresh interactive launchers and
delegates fleet operations to the Go binary. `pfm install` wires this single active shim.

| flow                                                                              | safety    | expected behavior (source)                       | regression |
| --------------------------------------------------------------------------------- | --------- | ------------------------------------------------ | ---------- |
| shim aborts when `~/.local/bin/pfm` is missing/not executable                     | JAIL+sh   | `shim/pfm.zsh:5-9`                               |            |
| `cc-ls` routes `--check/--plain/--tsv` direct, everything else through `eval`     | JAIL+sh   | `shim/pfm.zsh:25-33`                             |            |
| `cc-open` / `cc-revive` pass-through                                              | JAIL+sh   | `shim/pfm.zsh:35-36`                             |            |
| `_pfm_eval` evals ONLY on rc 0 and non-empty output                               | JAIL+sh   | `shim/pfm.zsh:15-23`                             |            |
| `cc` uses the primary account from hidden `pfm internal primary-get`              | JAIL+sh   | `shim/pfm.zsh`, `shim/shim_test.go`              |            |
| `cc1` / `cc2` force an account; account 3 has no launcher                         | JAIL+sh   | `shim/pfm.zsh:107-108`, `action/synth.go:19`     |            |
| `_cc_run` per-element quoting of `CC_AUTONOMY_FLAGS` and user args                | JAIL+sh   | `shim/pfm.zsh:89-93`                             |            |
| `_cc_run` env hygiene + `FORCE_PROMPT_CACHING_5M` when un-armed                   | JAIL+sh   | `shim/pfm.zsh:83-101`                            |            |
| `_cc_arm1h`: `CC_ARM_1H=1`, or `ENABLE_PROMPT_CACHING_1H=1` from a non-chat shell | JAIL+sh   | `shim/pfm.zsh:62-66`                             |            |
| `cx` creates its server DETACHED with title plumbing, then attaches               | JAIL+sh   | `shim/pfm.zsh:116-139`                           | B1         |
| `_cc_selfswitch` refuses to nest a session inside itself                          | JAIL+tmux | `shim/pfm.zsh:201-222`                           |            |
| `_cc_in_bunker` → `exec` into the client so no husk survives                      | JAIL+sh   | `shim/pfm.zsh:95,125,197`                        |            |
| `cc-swap <1\|2>` / fzf picker → hidden `pfm internal primary-set` is the writer   | JAIL+sh   | `shim/pfm.zsh`, `shim/shim_test.go`              |            |
| `_cc_label` reads account 1's identity from `~/.claude.json`, not the config dir  | JAIL+sh   | `shim/pfm.zsh:142-154`                           |            |
| `cc-revive` lists resumable chats by project through the engine                   | JAIL+tmux | `cmd/pfm/commands.go:493-538`, `shim/pfm.zsh:41` | B1         |
| `vsct-revive` restores orphaned bunkers, skipping viewport husks                  | JAIL+tmux | `shim/pfm.zsh:226-252`                           |            |

## J — Internal wiring and store

| flow                                                                                                  | safety           | expected behavior (source)                                               | regression |
| ----------------------------------------------------------------------------------------------------- | ---------------- | ------------------------------------------------------------------------ | ---------- |
| shared-store open is idempotent, initializes schema, enables WAL and sets a busy timeout              | JAIL             | `internal/shared/shared.go`, `internal/shared/shared_test.go`            |            |
| hide add/delete/lookup use SQLite exclusively and never recreate the retired carrier                  | JAIL             | `internal/shared/shared.go`, `internal/shared/shared_test.go`            | B2         |
| hide sync and orphan pruning replace only the intended set, in one transaction                        | JAIL             | `internal/shared/shared.go`, `internal/store/hidden_test.go`             |            |
| chat cache load/save/prune preserves tab-bearing prompt text and exact ids                            | JAIL             | `internal/store`, `internal/store/queries_test.go`                       |            |
| hidden `internal primary-get/set` validates the 1/2 roster and keeps its mirror coherent              | JAIL             | `cmd/pfm/main.go`, `internal/shared/shared_test.go`, `shim/shim_test.go` |            |
| child add/list/clear distinguishes `new` servers from `pane` children                                 | JAIL             | `internal/shared/shared.go`, `internal/shared/shared_test.go`            |            |
| `/bb` and the Codex `$bb` skill both run `pfm chat hide self --exit`                                  | JAIL+tmux        | `bb.command.md`, `codex-skills/bb/SKILL.md`, hide jail tests             |            |
| hidden `internal agent-open` holds a per-id mutex; exactly one concurrent takeover wins               | JAIL             | `internal/agentopen/agentopen.go`, `agentopen_test.go`                   |            |
| agent registry accepts `sessionId` or `id` prefixes across configured account roots                   | **REAL-SESSION** | `internal/agentopen/agentopen.go`, `agentopen_test.go`                   | B3         |
| agent routing attaches busy/tmux-resident work and takes over idle work                               | JAIL+tmux        | `internal/agentopen`, `agentopen_test.go`                                |            |
| a failed registry scan refuses a fresh resume instead of opening the same transcript twice            | JAIL             | `TestOpenRefusesFreshResumeWhenEveryRegistryQueryFails`                  |            |
| `chat swap` parses account/cache-only forms and rejects a multi-pane server                           | JAIL+tmux        | `swap_command.go`, `swap_jail_test.go`                                   |            |
| `chat swap` preserves current account for cache-only requests and fresh-boots without a transcript    | JAIL+tmux        | `internal/swap`, `swap_test.go`                                          |            |
| `chat swap` serializes by pane, refuses an open selector, and waits for a real prompt before `--then` | JAIL+tmux        | `internal/swap`, `swap_jail_test.go`, `swap_test.go`                     |            |
| `chat recover` resolves id or rollout, rebuilds normal/compacted output, and is idempotent            | JAIL             | `internal/recovery`, `recover_jail_test.go`                              |            |

## K — Installer and systemd units

| flow                                                                                                              | safety           | expected behavior (source)                | regression |
| ----------------------------------------------------------------------------------------------------------------- | ---------------- | ----------------------------------------- | ---------- |
| `pfm install` dry run is the default and writes nothing                                                          | JAIL             | `internal/installer`, installer tests     |            |
| a reachable user bus refuses before any write with rc 97                                                         | JAIL             | `install_command_test.go`, installer tests |            |
| apply stages embedded assets, removes retired links, and leaves `~/.claude/bin` empty                            | JAIL             | `internal/installer`, installer tests     |            |
| command cards, helpers, Codex skill, shim and units link to the managed asset tree                               | JAIL+sh          | `internal/installer`, installer tests     |            |
| real destinations are backed up; uninstall restores the newest backup                                             | JAIL             | `internal/installer`, installer tests     |            |
| zshrc and canonical settings converge without duplicate commands                                                  | JAIL+sh          | `internal/installer`, installer tests     |            |
| immediate second apply reports `changed=0`                                                                        | JAIL             | `TestApplyIsSelfContainedIdempotentAndReversible` |            |
| systemd units and `.wants/` links converge without a manager; live transitions use only the injected manager     | JAIL             | installer idempotence and unit-transition tests |            |
| `pfm-name-sync.path` fires on `~/.codex/session_index.jsonl` modification                                         | **REAL-SESSION** | `systemd/pfm-name-sync.path`              | **B4**     |
| `pfm-name-sync.timer` 15-min drift fallback                                                                       | JAIL+sh          | `systemd/pfm-name-sync.timer`             |            |
| `pfm-name-sync.service` `ExecStart` runs the BINARY, never a `.sh`                                                | JAIL+sh          | `systemd/pfm-name-sync.service`           |            |
| installer retires the carrier, old units, script links, statusline shell, segments and Python refreshers         | JAIL             | `internal/installer`, installer tests     |            |
| installer rewires BB, group, statusline, usage and dream hooks across canonical accounts                         | JAIL             | `internal/installer`, installer tests     |            |

---

## Flows that CANNOT be jailed

Schedule these deliberately on a scratch project directory. Rows tagged `REAL-SESSION` are here
verbatim; the rest are `JAIL+tmux` rows whose jail proves only the
mechanics (synthetic screen text, fake `/proc`) and which need one authentic engine run before
they count as covered.

**Needs a real `codex` process:**

1. live-codex store-only **resumed** thread — identity, name, window (`resolve/codex.go:49-85`).
2. live-codex store-only **fresh** thread that exports `CODEX_THREAD_ID`.
3. Codex writing a rollout and immediately closing it (the fd-scan blind spot).
4. Codex rename inside the TUI → `threads.name` update.
5. Codex rename landing only in `session_index.jsonl` (**B4**).
6. `codex resume <name>` creating a paginated thread that writes no rollout.
7. Two codex chats born in the SAME directory within the birth window.
8. Codex approval / plan-overlay modals during an inject.
9. Codex composer tall-message truncation (the false-green class).
10. `pfm chat hide self --exit` `/quit` flush on a REAL codex seat.
11. `pfm name-sync` converging a live cx window name onto the VS Code tab.
12. Codex-origin `pfm chat inject` signature via ancestry recovery.
13. `chat_ls` state=`busy` for a genuinely generating codex pane.
14. `cx` launcher self-switch when already inside its own server.
15. ✅ DONE — `pfm chat new --engine cx --name X` against the REAL
    codex TUI (0.147): confirmed live end to end. It found three things no stub
    had: the TUI boots into a hooks/trust modal that swallows keystrokes, its
    modal selection cursor is the SAME glyph as the composer (so readiness
    needs the status line too), and the rename field arrives pre-filled. All
    three are fixtured now. Re-run this experiment on any codex upgrade.
16. ✅ DONE — the two-way surface against BOTH real engines: `chat new --await`
    (inline and `--prompt-file`, multi-line), three-turn `ask` conversations on
    a codex and a claude seat, `--json`, `--progress`, a `--timeout` that
    expires with the question still in the record, and the read verbs over the
    same live chats. 22 checks, all green. Re-run on any engine upgrade —
    `ask` is only as true as the transcript shapes both engines write.

**Needs a real `claude` process:** 16. Agent row: a real non-primary-config-dir claude with `--session-id`. 17. Agent takeover through hidden `pfm internal agent-open` (`claude agents --json`). 18. The daemon-agent guard on the resume-inject path. 19. `/bb` graceful `/exit` flush + Stop hooks + post-exit re-index. 20. Open gate: a live chat whose birth account ≠ primary. 21. `pfm chat swap` full reboot-in-place (`respawn-pane -k`) + `--then` delivery. 22. Trust-prompt handling on a fresh config-dir/cwd pair. 23. `/chat:branch` fork (`--fork-session`) and its pane-child registration. 24. `pfm chat new NAME` teammate spawn and immutable socket identity. 25. `⚡1h` badge read from a live process's `/proc` environ. 26. `[Pasted text #N]` collapse in a real Claude composer. 27. Dim-SGR placeholder vs a real draft (mash guard). 28. Selector/permission modal handling.
28b. `pfm reap`'s busy probe against REAL `claude agents --json`; the jail proves fail-closed
plumbing but not the live daemon's JSON shape, so re-run it on any Claude Code upgrade.

**Needs real multi-account state:** 29. `_cc_label` reading `oauthAccount.emailAddress` per account. 30. `cc-swap` fzf picker → primary flips for `cc` but not `cc1`/`cc2`. 31. Transcripts under a SEPARATE account root (not a symlink back to account 1). 32. Statusline badge computation across accounts.

---

## Residual known divergences

1. **Store-only rows carry no parent link** (`index/codexstate.go`) — a live-state audit found
   no real instance because Codex ≥0.146 resume continues the same thread id. Re-open if Codex
   changes that behavior; detect it by grouping same-cwd rows with the same first message.
2. **Account roster disagrees three ways.** `paths.go:64-68` builds THREE Claude roots,
   `compose/compose.go:873` accepts accounts 1-3, `pipeline.go:536` labels them 1-3 — but
   `action/synth.go:19` caps at 2 and the shim has no `cc3`. `readPrimaryAccount` clamps
   off-roster values upstream, so launches stay correct; the divergence is dormant, not dead.
3. **`bb.command.md` documents `kill-server`**, but `internal/hide/finisher.go` deliberately
   does kill-PANE (and the doc's claim would take split siblings down). Stale contract.
4. **`mcpserv/backend.go:303-308` counts `Agent` as live** while `ui/model.go:467-471` does not.
   An agent row is capture-probed for busy state in MCP and treated as non-live in the TUI.

---

## Highest-risk joins

Ranked by identity resolution across resume/store/fork edges, hide-store integrity, and
live-vs-indexed disagreement.

| #   | flow                                                                                                         | why it hides bugs                                                                                                                                                                                       |
| --- | ------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | **Live codex → indexed row join** (`compose/compose.go:492-513`)                                             | The join walks path → resolver ThreadID (`liveCodexID`); a process neither resolves is still a guess. Every Codex row bug passes through here.                                                          |
| 2   | **Store-only thread lineage** (`index/codexstate.go:98-130`)                                                 | A resumed store-only thread is given itself as its lineage root, so it can never merge with the conversation it continues. Hides, names and prompt counts all split.                                    |
| 3   | **Codex name precedence across two writers** (`index/index.go`, `naming/naming.go`)                          | Name provenance (`cx_names.source`/`renamed_at`, schema v4) settles store-vs-file, but three walkers still differ on lineage breadth, and `reloadCxNames` still wipes and re-folds in two transactions. |
| 4   | **Agent-row identity** (`gather/agents.go:12-69` → `compose/compose.go:541-590` → `hide/manager.go:138-165`) | An agent row is the only kind whose ID can exist with no index row behind it. Every id-keyed operation must vouch an engine for it; a new call site that forgets recreates the silent-refusal bug.      |
| 5   | **Codex thread ↔ pane pairing** (`resolve/codex.go:49-85`, `gather/codexproc.go`)                            | The argv `resume <uuid>` rung leads and the ±120s window only backstops fresh threads — but a resumed thread with a scrubbed argv still falls to the window.                                            |
| 6   | **Live-vs-indexed refresh race in the picker** (`pipeline.go:315-424`, `ui/model.go:247-271`)                | Three snapshots stream into an open TUI while the user is toggling hides. A row that changes identity between frames takes the pending hide with it.                                                    |
| 7   | **`collapseLiveServers` + `ServerCount`** (`compose/compose.go:759-803`)                                     | Silently merges rows by `engine+ID`. Two genuinely different chats that resolve to the same id (see #1) disappear into one row; the loser is unreachable.                                               |
| 8   | **`--exit` finisher choreography** (`hide/finisher.go:94-245`)                                               | Detached, delayed, and it re-asserts a hide AFTER an index refresh. It races the engine's own transcript flush and reaps teammates by an id that may already have been canonicalized.                   |
| 9   | **Primary-account round trip** (`pipeline.go:553-631`, `shared/shared.go:441-482`, `shim/pfm.zsh`)           | The store and launcher must enforce the same account roster.                                                                              |
