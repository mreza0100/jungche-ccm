# cc-fleet TESTPLAN — the complete flow matrix

Every pathway through the fleet tooling: the Go engine (`cc-fleet/`), the zsh surface
(`cc-fleet.zsh`, `cc-fleet/shim/cc-fleet.zsh`), `chat/chat.sh`, `cc-db.sh`, the satellites,
the MCP server, and the installer. One row per flow. The experiment wave drives this map.

Go paths are relative to `~/.professor/cc-fleet/` (the engine lives at the repo root — it is a
program, not a template); the zsh/satellite paths are relative to the bundle,
`~/.professor/blueprint/templates/host-swap/`. Absolute paths are absolute.

## Legend — the SAFETY column

| Token | Meaning |
| --- | --- |
| `JAIL` | Fully exercisable in a path jail: `paths.go:11-21` env overrides (`CC_FLEET_HOME`, `CC_FLEET_DB`, `CC_FLEET_SHARED_DB`, `CC_FLEET_SID_DIR`, `CC_FLEET_CLAUDE_ROOTS`, `CC_FLEET_CODEX_ROOT`, `CC_FLEET_TMUX_DIR`, `CC_FLEET_PROC_ROOT`) plus `CC_FLEET_TEST_NOW_NS` / `CC_FLEET_TEST_FRESH_SOCKET` / `CC_FLEET_CODEX_AVAILABLE` / `CC_FLEET_DB_SCRIPT` (`pipeline.go:25-30`). Reference harness: `cc-fleet/testdata/e2e.sh:1-56`. |
| `JAIL+tmux` | Needs a real tmux **server on a scratch socket inside the jail's `TMUX_TMPDIR`** — never a live fleet socket. Reference harness: `internal/*/tmux_jail_test.go` (e.g. `internal/hide/tmux_jail_test.go:58-66`). |
| `JAIL+sh` | zsh/bash fixture with a fake `$HOME`, scratch db and an EMPTY socket dir. Reference: `tests/hide-fixtures.sh:1-20`, `tests/db-fixtures.sh`, `tests/name-sync-fixtures.sh`, `tests/install-fixtures.sh`, `tests/selflocate-fixtures.sh`. |
| `LIVE-READ` | Read-only observation of live state (`cc-fleet ls --tsv`, `--plain`, `--check`, `hidden`, `doctor`, `sqlite3 -readonly`). Never mutates. |
| **`REAL-SESSION`** | ⚠ **CANNOT be jailed.** Needs a genuine `claude` / `codex` process, a real transcript/rollout writer, or a real Claude-account login. The supervisor must schedule these deliberately on a scratch project directory. |

## Legend — the REGRESSION column

Tonight's four escapes. A row tagged with one of these MUST get a fixture before the wave closes.

- **B1** — resumed store-only codex thread renders the wrong name.
- **B2** — workflow twin threads: a hide resurrects as a doppelgänger row.
- **B3** — an agent-row hide is a no-op.
- **B4** — a rename that lands only in `~/.codex/session_index.jsonl` never reaches the picker.

---

## Table of contents

1. [A — `cc-fleet` CLI subcommands and flags](#a--cc-fleet-cli-subcommands-and-flags) — 63 flows
2. [B — Picker TUI: key bindings and model state](#b--picker-tui-key-bindings-and-model-state) — 25 flows
3. [C — Row-kind × operation cross-matrix](#c--row-kind--operation-cross-matrix) — 26 flows
4. [D — Index, naming and identity resolution](#d--index-naming-and-identity-resolution) — 23 flows
5. [E — Hide / unhide and the two-writer shared state](#e--hide--unhide-and-the-two-writer-shared-state) — 25 flows
6. [F — Action synthesis and launch](#f--action-synthesis-and-launch) — 20 flows
7. [G — MCP server (7 tools)](#g--mcp-server-7-tools) — 22 flows
8. [H — `chat.sh`: subcommands, guards, `--then`, exit codes](#h--chatsh-subcommands-guards---then-exit-codes) — 46 flows
9. [I — zsh shell surface: launchers and revivers](#i--zsh-shell-surface-launchers-and-revivers) — 16 flows
10. [J — Satellite scripts](#j--satellite-scripts) — 42 flows
11. [K — Installer and systemd units](#k--installer-and-systemd-units) — 12 flows
12. [Flows that CANNOT be jailed](#flows-that-cannot-be-jailed)
13. [Residual known divergences](#residual-known-divergences)
14. [Top 10 — where the next bugs are hiding](#top-10--where-the-next-bugs-are-hiding)

**Total: 334 flows.** 10 rows are marked `REAL-SESSION` (no jail can reach them at all); the
scheduling list below expands those into **32 distinct real-engine experiments**, because several
`JAIL+tmux` rows only prove the *mechanics* against synthetic pane content and still need one
authentic engine run to be conclusive.

---

## A — `cc-fleet` CLI subcommands and flags

| flow | safety | expected behavior (source) | regression |
| --- | --- | --- | --- |
| no args → usage, rc 2 | JAIL | `main.go:25-28`, `main.go:337-354` | |
| unknown subcommand → usage, rc 2 | JAIL | `main.go:62-66` | |
| `help` / `-h` / `--help` → usage on stdout, rc 0 | JAIL | `main.go:59-61` | |
| `version` → `cc-fleet <version>`; extra arg → rc 2 | JAIL | `main.go:95-106` | |
| `ls` (interactive) → BubblePicker on `/dev/tty`, cached first frame then streamed refresh | JAIL+tmux | `commands.go:101-121`, `ui/picker.go:12-68` | |
| `ls --plain` → PlainPicker, one pass, rc 0 | JAIL | `commands.go:88-100,126-128` | |
| `ls --tsv` → TSVPicker, stable rows | JAIL | `commands.go:96-99`; golden `testdata/golden/ui.tsv` | |
| `ls -a/--all` → AllView | JAIL | `commands.go:33-34,60-65` | B2 |
| `ls -H/--hidden` → HiddenView | JAIL | `commands.go:35-36,64` | B2, B3 |
| `ls --all --hidden` → rc 2 (mutually exclusive) | JAIL | `commands.go:43-48` | |
| `ls --plain --tsv` (or any two renderers) → rc 2 | JAIL | `commands.go:44` (`boolCount`) | |
| `ls <id>` → same as `open <id>` | JAIL | `commands.go:52-58` | |
| `ls <id>` combined with any flag → rc 2 | JAIL | `commands.go:53-56` | |
| `ls --check` with any other flag/arg → rc 2 | JAIL | `commands.go:45` | |
| `ls --check` → shadow-run legacy zsh picker, diff verified tuples, rc 1 on unallowed | LIVE-READ | `check_command.go:23-179`, `internal/check/legacy.go:49-80` | |
| `open <id>` on an unindexed id → `is not indexed`, rc 1 | JAIL | `commands.go:217-218` | B3 |
| `open <id>` whose CWD vanished → falls back to `$PWD` for non-live kinds | JAIL | `commands.go:233-241` | |
| `headless run --name X [prompt]` → detached chat on a fresh fleet socket with the full launch ceremony, rc 0 | JAIL+tmux | `run_command.go`, `action/headless.go`, `spawn/spawn.go`; `run_jail_test.go` | |
| `headless run` without `--name` → rc 2; unknown `--engine` → rc 2; unknown `--effort` → rc 2 | JAIL | `headless_matrix_test.go` | |
| `headless run --engine cx` → thread named through Codex's own `/rename` UI, THEN prompted | JAIL+tmux | `spawn/spawn.go` (`nameCodexThread`) | |
| `headless run --engine cx` on a Codex without `/rename` → live chat, `UNNAMED`, warning, rc 1, composer cleared | JAIL+tmux | `TestRunReportsACodexBuildThatCannotBeRenamed` | |
| Codex boots through startup overlays (hooks review, trust) — Escape, then a composer that HOLDS, before a keystroke | JAIL+tmux | `TestCodexBootsThroughStartupModals`, `TestCodexComposerFlashBeforeAModalIsNotReadiness` | |
| A startup screen that never clears → nothing typed, chat live, `UNNAMED`, rc 1 | JAIL | `TestCodexStuckAtAStartupScreenTypesNothing` | |
| The rename field is CLEARED before typing (Codex pre-fills it with the current name) | JAIL | `TestCodexThreadIsRenamedThenPrompted` (clear token) | |
| The rename Enter is re-sent until the status line carries the name; zero typing gap DROPS it | JAIL | `spawn/types.go` (`orDefaults`), `spawn/spawn.go` (`confirmPresses`) | |
| `headless run --prompt-file PATH` → prompt read from the file; with an inline prompt → rc 2 | JAIL | `TestRunPromptSourcesAreExclusive` | |
| `headless run --model M --effort E` → `--model`/`--effort` (cc) and `--model`/`-c model_reasoning_effort` (cx) | JAIL | `TestModelAndEffortReachBothEngines` | |
| `headless run` on a chat that dies at birth → `died at birth`, rc 1 | JAIL | `spawn/spawn.go` (`waitForBoot`) | |
| `headless status <name> [--json]` → state/idle_seconds/engine/model/cwd/session_id/context_pct | JAIL | `headless/headless.go`; `headless_test.go` | |
| working vs idle comes from the TRANSCRIPT (assistant spoke last = idle), never from a timer | JAIL | `TestStateComesFromTheTranscriptNotAClock` | |
| `headless transcript <name> [--tail N] [--condensed] [--json]` | JAIL | `transcript/transcript.go`; `transcript_test.go` | |
| `headless last <name>` → the last assistant message, bare | JAIL | `TestLastFindsTheNewestAssistantTurn` | |
| `headless stream <name> [--filter RE] [--margin N]` → grep -C over the live transcript | JAIL | `TestStreamFilterWithMargin` | |
| `headless stream` ends when the chat dies instead of hanging on a quiet file | JAIL | `TestStreamFollowEndsWhenTheChatDies` | |
| `headless inject <name> <message>` → the existing guarded delivery, addressed by socket | JAIL+tmux | `headless_command.go`, `internal/inject` | |
| `headless ask <name> <message>` → deliver, wait, print the ANSWER on stdout alone | JAIL+tmux | `ask_command.go`, `headless/converse.go`; `TestAskHoldsATwoWayConversation` | |
| `ask` answers the question just asked — the frontier is taken BEFORE delivery, so an older answer cannot be returned | UNIT | `TestAwaitReturnsTheAnswerToTheQuestionJustAsked` | |
| `ask` waits through tool work: an assistant line followed by a tool call is a preamble, not an answer | UNIT | `TestAwaitWaitsThroughToolWork` | |
| `ask --timeout N` → rc 5 with the message still DELIVERED (a different fact from unheard) | JAIL+tmux | `TestAskReportsATimeoutWithoutLosingDelivery` | |
| `ask --json` → name/delivered/answer/state/tools/waited_seconds; `--progress` → condensed turns on stderr | JAIL+UNIT | `headless/converse.go`; `TestAwaitStreamsProgress` | |
| `ask` on a WORKING chat re-offers the message until it lands (inject rc 7) instead of aborting; `--now` interrupts | LIVE | `ask_command.go` (`busyRetry`, `inject.CodeBusy`) | |
| `ask` reports `superseded` when a second human turn lands mid-wait — the answer may be theirs | UNIT | `TestAwaitFlagsAnAnswerSomebodyElseMayOwn` | |
| `--timeout` bounds the WHOLE exchange: time spent waiting for a busy chat is spent from the same budget | JAIL | `ask_command.go` (`remaining`) | |
| `ask` on a chat that dies mid-answer keeps what it said, then reports it gone | UNIT | `TestAwaitKeepsWhatADyingChatManagedToSay` | |
| `run` with a prompt PROVES delivery from the engine's transcript; unproven → rc 6 + attach hint, chat left running | JAIL+tmux | `awaitLaunch`; `TestRunRefusesToCallAnUnheardPromptDelivered` | |
| `run --await` → the first answer on stdout, launch summary moved to stderr | JAIL+tmux | `TestRunAwaitsTheAnswerItAskedFor` | |
| The prompt's Enter is re-sent until the composer releases it; a composer that never does → `Prompted=false` + warning | UNIT | `TestCodexPromptIsResentUntilItLeavesTheComposer`, `TestCodexPromptThatNeverSubmitsIsReportedUndelivered` | |
| A wait re-reads the transcript every poll but re-scans the fleet only every `ResolveEvery` | UNIT | `headless/converse.go` (`ResolveEvery`) | |
| `transcript.From` never consumes a half-written record, and restarts when the file shrinks | UNIT | `TestFromHoldsAPartialLine`, `TestFromRestartsWhenTheFileShrinks` | |
| `headless watch <name>` → `IDLE`/`EXIT`/`DEAD` lines + `--on-idle`/`--on-exit` hooks | JAIL | `TestWatchAnnouncesIdleThenExit`, `TestWatchReportsAChatThatVanishesAsDead` | |
| `headless ls [--json]` → every live seat with its state and last line | JAIL | `headless_command.go` | |
| An unknown name → rc 4 with `not-found` on stdout, on EVERY verb — never empty at rc 0 | JAIL | `TestUnknownChatIsRc4WithAMachineShape` | |
| A dead (non-live) chat → rc 3, explicitly, never silence | JAIL | `headless_command.go` (`codeDeadChat`) | |
| Flags parse BEFORE or AFTER the name (`status seat --json`) | JAIL | `main.go` (`parseFlagsAnywhere`); `TestHeadlessArgumentMatrix` | |
| `headless inject` takes its message verbatim — a leading dash is not a flag | JAIL | `runHeadlessInject` | |
| Name resolution: name, id-prefix, socket; live row beats its resume twin; ambiguity refused | JAIL | `TestChatMatchingPrefersTheLiveSeat` | |
| `index` → counters line on stdout | JAIL | `commands.go:351-398`, `formatCounters` `commands.go:400-412` | B4 |
| `index --full` → reparse all + `last_full_index_at` meta | JAIL | `commands.go:386-392` | B4 |
| `index --progress` → start/elapsed on stderr | JAIL | `commands.go:378-379,394-396` | |
| `revive` → resume-only rows through PlainPicker; empty → "no resumable chats" | JAIL | `commands.go:414-458` | B2 |
| `hide <id>` → shared row + carrier line | JAIL | `main.go:108-146`, `hide/manager.go:69-111` | B3 |
| `hide --self` → identity from `$TMUX`/`$TMUX_PANE`/`$CLAUDE_CODE_SESSION_ID` | JAIL+tmux | `main.go:114`, `hide/self.go:16-38` | B3 |
| `hide --self --exit` → detached finisher; `--exit` without a live pane → error | JAIL+tmux | `main.go:115`, `hide/manager.go:86-88,98-109` | |
| `hide` arg-shape violations (`--self` with an id, no id without `--self`) → rc 2 | JAIL | `main.go:119-123` | |
| `unhide <id>` → canonicalizes a Codex id to its lineage root first | JAIL | `main.go:148-168`, `hide/manager.go:113-131` | B2 |
| `hidden` → `id\tengine\thidden_at` per row | JAIL | `main.go:207-215` | B3 |
| `hidden --prune-orphans` → dry-run report; `--yes` deletes; `--yes` alone → rc 2 | JAIL | `main.go:176-192`, `commands.go:463-511` | |
| `resolve label\|session\|cxwin <name>` → chat.sh return codes 0/1/2 verbatim | JAIL+tmux | `main.go:218-248`, `resolve/resolve.go:76-95` | |
| `resolve` with a bad kind → rc 2 | JAIL | `resolve/resolve.go:92-93`, `main.go:241-244` | |
| `whoami` → this process's own tmux session name | JAIL+tmux | `main.go:53`, `resolve/whoami.go:165-213` | |
| `legacy import` → cold full index, then carrier→table union | JAIL | `commands.go:542-567`, `legacy/legacy.go:47-77` | B2 |
| `legacy export` → import first, then whole-file carrier rewrite | JAIL | `commands.go:569-574`, `legacy/legacy.go:90-107` | |
| `legacy` with no/unknown verb → rc 2 | JAIL | `commands.go:519-533` | |
| `doctor` → db + jail health | LIVE-READ | `main.go:39-40`; `internal/store/health.go` | |
| `mcp` → stdio server; any arg → rc 2 | JAIL | `main.go:69-93` | |
| `internal hide-exit --engine…` → detached finisher; missing flags → rc 2 | JAIL+tmux | `main.go:250-299`, `hide/finisher.go:94-144` | |
| `internal then …` → steer waiter | JAIL+tmux | `main.go:251-253`, `inject/then.go` | |

## B — Picker TUI: key bindings and model state

| flow | safety | expected behavior (source) | regression |
| --- | --- | --- | --- |
| `⌃T` reload → `OutcomeReload`, loop re-scans with `ForceFull` | JAIL | `ui/model.go:144-147`, `commands.go:143-145` | |
| `⌃R` rotate project groups, keeps cursor on the followed row | JAIL | `ui/model.go:148-153`, `ui/model.go:334-352` | |
| `⌃R` with zero groups → rotation pinned 0, no panic | JAIL | `ui/model.go:336-339` | |
| `⌃X` toggle hide, in-memory only until quit | JAIL | `ui/model.go:154-156,273-297` | B3 |
| `⌃X` on a split row or an ID-less row → no-op | JAIL | `ui/model.go:279-281` | B3 |
| `⌃X` twice back to the initial state → change dropped, not re-written | JAIL | `ui/model.go:290-295` | |
| `⌃X` then quit → `applyPickerChanges` writes each change | JAIL | `commands.go:296-315` | **B3** |
| `⌃X` on an agent row whose transcript is not indexed → **must not silently fail** | JAIL | `compose/compose.go:564-567` + `hide/manager.go:142-164` | **B3** |
| `⌃E` flip 1h cache for the next launch | JAIL | `ui/model.go:157-159` | |
| `⌃S` cycle primary account `n%MaxAccount+1` | JAIL | `ui/model.go:160-162`, `action/synth.go:19` | |
| `⌃S` change persisted via `cc-db.sh primary-set` on exit | JAIL+sh | `commands.go:133-138`, `pipeline.go:578-631` | |
| `⌃O` reboot a live row → kill-server, drop crumbs, demote to resume kind | JAIL+tmux | `ui/model.go:166-172`, `commands.go:317-349` | |
| `⌃O` on a non-live row → no-op | JAIL | `ui/model.go:167-171` | |
| `⌃O` on a `LiveSplit` (no id) → error, never a half-reboot | JAIL | `commands.go:323-325` | |
| `Enter` → `OutcomeSelected` with the row; empty filter → no-op | JAIL | `ui/model.go:173-179` | |
| `Esc` / `⌃C` → cancelled, rc 0, no writes | JAIL | `ui/model.go:180-182`, `commands.go:162-163` | |
| `↑/⌃P`, `↓/⌃N`, `PgUp/PgDn`, `Home/End` bounds | JAIL | `ui/model.go:183-207`, `pageRows` `ui/model.go:429-431` | |
| Typing filters: substring hit, then fuzzy subsequence | JAIL | `ui/model.go:354-399`, `runeSubsequence:401-416` | |
| `Backspace`/`⌃H`, `⌃W` (word), `⌃U` (clear) | JAIL | `ui/model.go:210-224` | |
| Paste message appends to the query | JAIL | `ui/model.go:131-133` | |
| Query longer than `CharLimit` 200 → clipped, never panics | JAIL | `ui/model.go:80,228-236` | |
| Live refresh mid-picker preserves cursor, query and pending hides | JAIL | `ui/model.go:247-271` | B2 |
| Refresh that DROPS the selected row → cursor falls back safely | JAIL | `ui/model.go:390-399` | B2 |
| Window resize re-widths the query field | JAIL | `ui/model.go:123-127` | |
| Footer legend matches the real bindings | JAIL | `ui/render.go:122-125` | |

## C — Row-kind × operation cross-matrix

Each row is one **session kind** crossed with the operations that touch it. This is the table
tonight's four bugs all live in.

| flow (kind × op) | safety | expected behavior (source) | regression |
| --- | --- | --- | --- |
| **live-claude** — one pane crumb → one row, name from indexed title else pane title | JAIL | `compose/compose.go:279-354,356-399`, `naming/naming.go:23-44` | |
| **live-claude, socket crumb only** (no pane crumb) → row only if a claude process holds the socket | JAIL | `compose/compose.go:337-346` | |
| **live-claude split** (≥2 pane crumbs) → one `LiveSplit` row, names joined `a+b` | JAIL | `compose/compose.go:327-329,401-480` | |
| **live-claude, two servers one transcript** → collapsed, `ServerCount` = n, newest socket wins | JAIL | `compose/compose.go:759-803` | |
| **live-codex rollout-backed** → fd-walk finds the rollout, pane must exist in the SAME snapshot | JAIL | `gather/codexproc.go:61-75`, `compose/compose.go:492-501` | |
| **live-codex store-only, fresh** → identity via `CODEX_THREAD_ID`, else cwd+birth ≤120s | JAIL | `gather/codexproc.go:76-87`, `resolve/codex.go:35-87` | B1 |
| **live-codex store-only, resumed** → birth window cannot match; row must not vanish or mislabel | **REAL-SESSION** | `resolve/codex.go:49-85` (returns an error → `codexproc.go:82-84` drops the row) | **B1** |
| **live-codex, exported thread id unknown to the store** → `RolloutPath` is `""` → row ID becomes `"."` | JAIL | `resolve/codex.go:42-48` → `compose/compose.go:502-513,834-847` | **B1, B2** |
| **live-codex whose thread is archived in Codex** → filtered out of candidates, same `""` path | JAIL | `store/codexstate.go:161-163` | B1, B2 |
| **resume-claude** → transcript row, capped 30 in DefaultView | JAIL | `compose/compose.go:70-108`, `store/queries.go:35-59` | |
| **resume-codex** → one row per lineage, keyed by `RootID`, newest member supplies path/mtime | JAIL | `compose/compose.go:110-148`, `store/lineage.go:22-81` | B2 |
| **store-only resumable** (thread with no rollout file) → row created with placeholder path | JAIL | `index/codexstate.go:98-130` | B1, B2 |
| **store-only resumable, resumed** → `LineageRoot` is set to its OWN id, so it does NOT join its ancestor's lineage | JAIL | `index/codexstate.go:120-129` (no `SessionID`/`ParentThread` set) | **B1, B2** |
| **twin threads from one conversation** → must collapse to ONE row; a hide on either must cover both | JAIL | `store/lineage.go:22-81`, `compose/compose.go:660-673` | **B2** |
| **agent row** → live claude under a NON-primary `CLAUDE_CONFIG_DIR` carrying `--session-id`/`--resume` | JAIL | `gather/agents.go:12-69` | B3 |
| **agent row, transcript not indexed** → row synthesized from the session id alone | JAIL | `compose/compose.go:564-567` | **B3** |
| **agent row already live as a chat** → suppressed, never doubled | JAIL | `compose/compose.go:559-562,568` | |
| **workflow / SDK background (claude)** → `IsBG`, hidden from DefaultView, visible under `-a` | JAIL | `index/claude.go:41-44,83-95`, `compose/compose.go:725-736` | B2 |
| **workflow / SDK background (codex sub-thread)** → `thread_source != user` → not `Listed()` | JAIL | `store/codexstate.go:44-46,252` | B2 |
| **squatter socket** (a session whose name ≠ its socket) → never a chat row | JAIL+tmux | `check_command.go:181-203`; name-sync `cc-name-sync.sh:172` | |
| **vsct bunker chat** → `vsct`/`revive` sockets excluded from the probe; open uses `exec` | JAIL+tmux | `gather/tmuxprobe.go:356-361`, `pipeline.go:666-668`, `action/synth.go:290-310` | |
| **hidden row** → excluded from DefaultView, counted in `HiddenCount`, shown under `-H` | JAIL | `compose/compose.go:660-673,708-716,738-747` | B2, B3 |
| **empty row** (size 0 / 0 prompts) → suppressed from DefaultView, counted in `SuppressedCount` | JAIL | `compose/compose.go:725-736` | |
| **both accounts** — a row's account comes from the longest matching config root | JAIL | `compose/compose.go:865-887`, `pipeline.go:527-542` | |
| **primary switch** — db meta first, `~/.claude-primary` mirror second, off-roster → 1 | JAIL+sh | `shared/shared.go:441-482`, `pipeline.go:553-559`, `cc-db.sh:262-275` | |
| **cache badge on/off** — `C1H` from `/proc` env of the live process, per socket | JAIL | `gather/cache1h.go`, `compose/compose.go:378,413,586` | |

## D — Index, naming and identity resolution

| flow | safety | expected behavior (source) | regression |
| --- | --- | --- | --- |
| Incremental skip: size+mtime unchanged → skipped | JAIL | `index/index.go:337-348` | |
| Delta parse: file grew and a prior parse exists | JAIL | `index/index.go:350-366` | |
| Row with `parsed_offset` 0 (store-created) → FULL parse, never a delta | JAIL | `index/index.go:354-366` | B1 |
| Parser-version bump forces a full reparse for each engine | JAIL | `index/index.go:96-116` | |
| `PriorityCWD` sorts this project's transcripts first | JAIL | `index/index.go:288-335` | |
| `PriorityOnly` pass skips codex, deletes and cx-names entirely | JAIL | `index/index.go:88-93,193,210,239` | |
| Deleted transcript/rollout is pruned only in a full pass | JAIL | `index/index.go:208-225` | |
| Codex state store: newest `state_<N>.sqlite` wins per thread id | JAIL | `store/codexstate.go:48-107,114-139` | |
| Codex state store opened `mode=ro`, never `immutable` (WAL visibility) | JAIL | `store/codexstate.go:182-193` | B4 |
| Older state schema missing columns → `COALESCE` fallback, never a failed pass | JAIL | `store/codexstate.go:266-309` | |
| Unreadable state generation is SKIPPED, never blanks the Codex half | JAIL | `store/codexstate.go:119-123` | |
| Store-vouched thread survives the prune that removes file-less rows | JAIL | `index/codexstate.go:45-52` | B2 |
| `applyCodexThread` only takes store content when the row has no parsed bytes | JAIL | `index/codexstate.go:79-96` | B1 |
| `reloadCxNames` truncates and rebuilds `cx_names` from `session_index.jsonl` on size/mtime change | JAIL | `index/cxindex.go:22-93` | **B4** |
| `reconcileCodexNames` applies store names AFTER the file rebuild — store outranks file | JAIL | `index/index.go:256-265`, `index/codexstate.go:132-179` | **B4** |
| Rename made only in `session_index.jsonl` while the store holds an older `name` | JAIL | conflict between `index/cxindex.go:51-63` and `index/codexstate.go:149-159` | **B4** |
| `CxName` lineage walk: own id → session id → parent thread → first prompt | JAIL | `naming/naming.go:88-106` | **B1** |
| `CxName` on a ≥3-deep lineage whose NAME sits on the root | JAIL | `naming/naming.go:97-105` vs root from `compose/compose.go:644-658` | **B1** |
| `DisplayName` precedence: custom title → AI title → first prompt | JAIL | `naming/naming.go:10-18` | |
| `LiveFallback` for cc sockets uses pane title, never the generated session name | JAIL | `naming/naming.go:23-44` | |
| Junk-prompt filter (`<x…`, `Caveat:`, `[Request`) and compact-summary skip | JAIL | `naming/naming.go:46-56`, `index/claude.go:52-56` | |
| Lineage cycle in the parent chain → lowest id in the cycle becomes root | JAIL | `store/lineage.go:83-128` | B2 |
| `ReconcileCodexLineageRoots` denormalizes roots after each full pass | JAIL | `store/lineage.go:201-236` | B2 |

## E — Hide / unhide and the two-writer shared state

| flow | safety | expected behavior (source) | regression |
| --- | --- | --- | --- |
| A hide writes the SQLite row then the carrier line, unconditionally | JAIL | `shared/shared.go:193-220`, mirrors `cc-db.sh:129-138` | |
| `HiddenAt` returns the UNION of db rows and carrier ids | JAIL | `shared/shared.go:241-277` | B2 |
| Carrier-only id reports `hidden_at` 0, never an invented time | JAIL | `shared/shared.go:243,271-275` | |
| Persistent `SQLITE_BUSY` on hide → warn, count, carry on (carrier already holds it) | JAIL | `store/hidden.go:53-94` | |
| Persistent `SQLITE_BUSY` on **unhide** → the row survives, the chat stays hidden | JAIL | `store/hidden.go:76-89` (documented asymmetry — pin it) | |
| Engine derived from the index, not stored; transcript wins a collision | JAIL | `store/hidden.go:171-241` | B3 |
| Hidden id no index knows → empty engine = "hidden whatever the engine" | JAIL | `store/hidden.go:171-177`, `compose/compose.go:664-668` | B3 |
| `applyHide` skips split rows and ID-less rows | JAIL | `compose/compose.go:660-663` | B2 |
| Hide is PERMANENT — a growing prompt count never un-hides | JAIL+sh | `compose/compose.go:669-671`; `tests/hide-fixtures.sh` | |
| `hide <id>` for an id in NEITHER table → `chat %q is not indexed`, rc 1 | JAIL | `hide/manager.go:138-165` | **B3** |
| Picker hide failure surfaces as `cc-fleet ls` rc 1, not a silent drop | JAIL | `commands.go:129-132,296-315` | **B3** |
| `unhide` maps a Codex member id to its lineage root before deleting | JAIL | `hide/manager.go:113-131` | B2 |
| Hide by lineage ROOT vs `cx-hide.sh` hiding by RAW rollout id | JAIL+sh | `compose/compose.go:116,645` vs `cx-hide.sh:147` | **B2** |
| `--exit` finisher: `/exit` (cc) or `/quit` (cx), poll, kill-pane, sweep crumbs | JAIL+tmux | `hide/finisher.go:104-131` | |
| Post-exit re-hide keeps the ORIGINAL `hidden_at` | JAIL | `hide/finisher.go:149-167` | |
| Teammate reap: `new` → kill-server, `pane` → kill-pane, never kill-server | JAIL+tmux | `hide/finisher.go:181-245` | |
| Teammate reap falls back to the flat file when the table has no row | JAIL | `hide/finisher.go:256-290` | |
| `legacy import` counts unknown ids but imports them anyway | JAIL | `legacy/legacy.go:47-77` | |
| `legacy export` unions before rewriting so no hide is destroyed | JAIL | `legacy/legacy.go:90-107` | |
| Two concurrent pickers hiding different chats → both survive (WAL + transaction) | JAIL | `shared/shared.go:135-157`, `cc-db.sh:53-57` | |
| A `_HIDE…` LABEL hides a chat with no store row, live or resumable, either case | JAIL | `naming.LabelHidden`, `compose/compose.go` (`applyHide`); `compose/label_hide_test.go` | |
| A label-hidden chat still shows under `-a` and `-H`, and counts as hidden | JAIL | `compose/label_hide_test.go` | |
| A split row is never label-hidden — its name is a join, not a label | JAIL | `compose/label_hide_test.go` | |
| Label-hidden rows never spend a cached-frame candidate slot (30/15) | JAIL | `store/queries.go` (`labelHiddenSQL`, `codexLineageLabelHidden`); `store/label_candidates_test.go` | |
| Picker hide key refuses a label-hidden row — renaming is the unhide | JAIL | `ui/model.go` (`toggleHidden`); `ui/label_hide_test.go` | |

## F — Action synthesis and launch

| flow | safety | expected behavior (source) | regression |
| --- | --- | --- | --- |
| `NewClaude` → `(cd -- <cwd> && CC_ARM_1H=… cc<N>)` | JAIL | `action/synth.go:83-93` | |
| `NewCodex` → `(cd -- <cwd> && cx)` | JAIL | `action/synth.go:94-98` | |
| `Live` → `TMUX= tmux -L <sock> attach -t <target>` | JAIL | `action/synth.go:99-107,300-310` | |
| Live codex target is `session:window`, falling back to session then socket | JAIL | `action/synth.go:312-322` | B1 |
| Live codex window name verified against the live server before use | JAIL+tmux | `action/executor.go:92-98,139-156` | B1 |
| `ResumeClaude` → resume, with the agent router as the `||` fallback | JAIL | `action/synth.go:144-173` | |
| `Agent` → agent router first, fresh resume as the `||` fallback | JAIL | `action/synth.go:108-143` | B3 |
| `ResumeCodex` → detached server created BEFORE the attach line is emitted | JAIL+tmux | `action/synth.go:174-191`, `action/executor.go:128-135` | |
| Bunker (`$TMUX` socket basename `vsct`) → `exec` prefix on every launch line | JAIL | `action/synth.go:290-310`, `pipeline.go:666-668` | |
| Primary account outside `1..MaxAccount` → refuses to synthesize | JAIL | `action/synth.go:59-65`, `action/synth.go:19` | |
| NUL in any row value → refuses | JAIL | `action/synth.go:66-79` | |
| Launch hygiene strip is identical to the zsh launcher's | JAIL | `action/synth.go:21-37` vs `cc-fleet.zsh:105,119,150` | |
| Autonomy flags appended AFTER the resume argument, never duplicated on fresh launches | JAIL | `action/synth.go:39-50,215-242` | |
| Dead live socket at open time → demoted to a resume in a fresh server | JAIL+tmux | `action/executor.go:64-87` | |
| Dead live socket on a SPLIT row (no id) → hard error | JAIL | `action/executor.go:68-73` | |
| Self-switch: already on the target server → select the engine window, emit NO line | JAIL+tmux | `action/executor.go:223-278` | |
| Engine-window choice: exact `claude`/`codex` → `node`/version → lowest index | JAIL | `action/executor.go:264-278` | |
| Open gate: birth account/cache ≠ picker → offer reboot; failure attaches as-is | **REAL-SESSION** | `action/executor.go:158-207`, `action/gate.go` | |
| Reboot-to-match invokes `cc-swap-chat.sh --sock … <acct> --1h <0\|1>` | JAIL+sh | `action/executor.go:190-199` | |
| `_cc_solo` stray-claude sweep before an attach/resume | JAIL | `action/executor.go:208-215`, `action/solo.go` | |

## G — MCP server (7 tools)

> The brief says 8 tools. There are **7**: `register()` at `mcpserv/server.go:65-103`.

| flow | safety | expected behavior (source) | regression |
| --- | --- | --- | --- |
| Tool roster is exactly 7 with correct read-only / mutating annotations | JAIL | `mcpserv/server.go:65-103` | |
| `chat_ls` default view, `all`, `hidden`, `project` filter | JAIL | `mcpserv/backend.go:82-253` | B2 |
| `chat_ls` with both `all` and `hidden` → error | JAIL | `mcpserv/backend.go:83-85` | |
| `chat_ls` state: `busy` / `idle` / `dead` / `resumable` per row | JAIL+tmux | `mcpserv/backend.go:181-225`, `inject.IsBusy` | |
| `chat_ls` treats an **agent** row as live and captures its pane | JAIL+tmux | `mcpserv/backend.go:303-308` (diverges from `ui/model.go:467-471`) | B3 |
| `chat_ls` split row folds every pane's state into one verdict | JAIL+tmux | `mcpserv/backend.go:184-208` | |
| `chat_ls` engine label for a `NewCodex`/agent kind | JAIL | `mcpserv/backend.go:317-322` | |
| `chat_ls` primary account agrees with the picker | JAIL+sh | `mcpserv/backend.go:272-279` vs `pipeline.go:553-559` | |
| `chat_resolve` kind validation (`label`/`session`/`cxwin`) | JAIL | `mcpserv/server.go:114-124` | |
| `chat_resolve` status mapping 0/1/2 → `ok`/`not_found`/`ambiguous` | JAIL+tmux | `mcpserv/server.go:126-142` | |
| `chat_inject` full guard chain, mirrors chat.sh | JAIL+tmux | `mcpserv/server.go:145-172`, `inject/engine.go:296-663` | |
| `chat_inject` `/compact` without `then` → refused code 6 | JAIL | `inject/engine.go:683-694` | |
| `chat_inject` a `then` steer that is itself `/compact` → refused code 1 | JAIL | `inject/engine.go:668-682` | |
| `chat_inject` long `/compact` focus → refused code 6 | JAIL | `inject/engine.go:696-708` | |
| `chat_inject` codex inline cap by RUNE count | JAIL+tmux | `inject/engine.go:327-336` | |
| `chat_inject` absolute byte cap before anything resolves | JAIL | `inject/engine.go:301-310` | |
| `chat_capture` `tail_lines` 1..1000, `max_bytes` 1..4Mi, rune-safe tail cut | JAIL+tmux | `mcpserv/server.go:209-271` | |
| `chat_whoami` takes NO arguments; identity from this process only | JAIL+tmux | `mcpserv/server.go:177-207`, `mcpserv/types.go:95-97` | |
| `chat_whoami` failure returns `not_found` + message, never an error | JAIL | `mcpserv/server.go:186-195` | |
| `chat_find` needle extraction identical to chat.sh's awk pass | JAIL | `mcpserv/search.go:33-68` vs `chat.sh:455-456` | |
| `chat_find` excludes the asking session unless `include_self` | JAIL | `mcpserv/search.go:70-98` vs `chat.sh:462` | |
| `chat_read` bounds: `last_n` ≤200, `max_bytes` ≤1Mi, Claude and Codex turn shapes | JAIL | `mcpserv/read.go:18-21,49-70,199-273` | |

## H — `chat.sh`: subcommands, guards, `--then`, exit codes

| flow | safety | expected behavior (source) | regression |
| --- | --- | --- | --- |
| `whoami` → own tmux session name | JAIL+tmux | `chat.sh:482-484,91-96` | |
| `whoami` with `$TMUX` unset → ancestry recovery via `cc_pane_of` | JAIL+tmux | `chat.sh:78-84`, `cc-portable.sh:148` | |
| `whoami` recovered value is NEVER exported globally | JAIL+tmux | `chat.sh:86-96` | |
| `self_label` scrape anchored on 🔖 + a medal; codex falls back to the window name | JAIL+tmux | `chat.sh:111-123` | B1 |
| `find <excerpt-file>` → 5 longest ≥20-char needles, ranked by hits, own session excluded | JAIL | `chat.sh:440-478` | |
| `find` with no ≥20-char line → rc 1; no match → rc 2 | JAIL | `chat.sh:457,463` | |
| `read <excerpt> [N]` → `tmp/chat-loads/<sid>.md` | JAIL | `chat.sh:490-508` | |
| `inject` target `self`/`me` | JAIL+tmux | `chat.sh:540-542` | |
| `inject` target raw `%pane` with `CHAT_INJECT_SOCKET` | JAIL+tmux | `chat.sh:545-549` | |
| `inject` ladder: session → 🔖 label → codex window → transcript resume | JAIL+tmux | `chat.sh:550-588` | B1 |
| `inject` ambiguous at any rung → rc 1 with the rung's message | JAIL+tmux | `chat.sh:558,565,574` | |
| `inject` no match at all → rc 1 | JAIL+tmux | `chat.sh:583-585` | |
| `inject --file` for shell-metacharacter bodies | JAIL+tmux | `chat.sh:521-530` | |
| `inject` empty message → rc 1 | JAIL | `chat.sh:535` | |
| `inject --then` on a non-live target → warning, steers dropped | JAIL | `chat.sh:993-994` | |
| `/compact` without `--then` → rc 1 | JAIL | `chat.sh:607-611` | |
| `--then` steer that starts with `/compact` → rc 1 | JAIL | `chat.sh:600-606` | |
| `/compact` focus over `COMPACT_FOCUS_MAX` (600) → rc 6 | JAIL | `chat.sh:612-624` | |
| codex pane message over `CHAT_INJECT_CX_INLINE_MAX` (1500) → rc 6 | JAIL+tmux | `chat.sh:709-715` | |
| inject lock: mkdir-atomic, double-steal guard, ownership-checked release, heartbeat | JAIL | `chat.sh:144-199` | |
| inject lock timeout → rc 4 | JAIL | `chat.sh:681-684` | |
| Signature: `/`-prefixed messages travel bare, everything else is signed | JAIL+tmux | `chat.sh:627-668,724-726` | |
| Signature underivable → explicit `UNSIGNED` footer + warning | JAIL+tmux | `chat.sh:648-657` | |
| copy-mode / Rewind / "Create a plan?" overlays dismissed before typing | JAIL+tmux | `chat.sh:728-758` | |
| Selector-menu guard (❯ and › dialects) → rc 4, nothing typed | JAIL+tmux | `chat.sh:760-790` | |
| Draft stash `C-s`, mash guard, dim-SGR placeholder exemption → rc 5 | JAIL+tmux | `chat.sh:792-848` | |
| Settle poll then Enter confirm; sub-40-char needle fallback | JAIL+tmux | `chat.sh:850-921` | |
| `[Pasted text #N]` collapse never counts as submitted | JAIL+tmux | `chat.sh:915-919` | |
| Blind composer line (tall message) → scrollback re-capture, else rc 3 | JAIL+tmux | `chat.sh:898-914,989-990` | |
| Proof contradiction after a "submitted" verdict → rc 4 | JAIL+tmux | `chat.sh:952-982` | |
| `--force-now` Esc loop only while busy; marker only when it fired | JAIL+tmux | `chat.sh:687-699,722-723` | |
| `__then` chain: min wait → busy → stable idle → exec next hop | JAIL+tmux | `chat.sh:1048-1085` | |
| `__then` log truncates on a fresh chain, appends on a hop | JAIL | `chat.sh:940-949` | |
| Resume inject: live-process guard refuses a pane-less LIVE session | JAIL | `chat.sh:996-1016` | |
| Resume inject: daemon-agent registry guard | **REAL-SESSION** | `chat.sh:1017-1026` | |
| Resume inject: backup + parented user event appended | JAIL | `chat.sh:1027-1045` | |
| `ls` / `ls --all` with the hidden-set filter (file wins over db) | JAIL+tmux | `chat.sh:1087-1185`, esp. `1100-1106` | B2 |
| `ls` codex row id via `cx_thread_id` birth window; unresolvable → LISTED | JAIL+tmux | `chat.sh:1136-1149`, `cc-portable.sh` `cx_thread_id` | **B1, B2** |
| `capture` resolves session → label → cxwin, then whole scrollback `-S -` | JAIL+tmux | `chat.sh:1187-1220` | |
| `save` / `tail` derive the transcript from `$PWD` and `$CLAUDE_CODE_SESSION_ID` | JAIL | `chat.sh:1223-1257` | |
| `load <dir>` enumerates text files only | JAIL | `chat.sh:1259-1276` | |
| `branch` forks in a split pane, inherits model + account + cache, registers a pane child | **REAL-SESSION** | `chat.sh:1278-1310`, `_spawn_envpfx:415-425`, `_register_pane_child:431-436` | |
| `new` / `new --detach` numbering from 🔖 prefixes; detach registers a `new` child | **REAL-SESSION** | `chat.sh:1312-1373` | |
| `extract <jsonl>` renders visible turns only | JAIL | `chat.sh:1375-1378`, `JQ_EXTRACT:32-57` | |
| `modal <session> deny <n>` takes the inject lock, sends N Downs + Enter | JAIL+tmux | `chat.sh:1380-1404` | |
| unknown subcommand → usage, rc 1 | JAIL | `chat.sh:1406-1409` | |

## I — zsh shell surface: launchers and revivers

Two surfaces exist and disagree: the **legacy** `cc-fleet.zsh` (full zsh picker) and the
**post-cutover** `cc-fleet/shim/cc-fleet.zsh` (thin wrapper over the Go binary). `install.sh:181`
still wires the legacy one.

| flow | safety | expected behavior (source) | regression |
| --- | --- | --- | --- |
| shim aborts when `~/.local/bin/cc-fleet` is missing/not executable | JAIL+sh | `shim/cc-fleet.zsh:5-9` | |
| `cc-ls` routes `--check/--plain/--tsv` direct, everything else through `eval` | JAIL+sh | `shim/cc-fleet.zsh:25-33` | |
| `cc-open` / `cc-revive` pass-through | JAIL+sh | `shim/cc-fleet.zsh:35-36` | |
| `_cc_fleet_eval` evals ONLY on rc 0 and non-empty output | JAIL+sh | `shim/cc-fleet.zsh:15-23` | |
| `cc` uses the primary account from `cc-db.sh primary-get` | JAIL+sh | `shim/cc-fleet.zsh:105-106` | |
| `cc1` / `cc2` force an account; account 3 has no launcher | JAIL+sh | `shim/cc-fleet.zsh:107-108`, `action/synth.go:19` | |
| `_cc_run` per-element quoting of `CC_AUTONOMY_FLAGS` and user args | JAIL+sh | `shim/cc-fleet.zsh:89-93` | |
| `_cc_run` env hygiene + `FORCE_PROMPT_CACHING_5M` when un-armed | JAIL+sh | `shim/cc-fleet.zsh:83-101` | |
| `_cc_arm1h`: `CC_ARM_1H=1`, or `ENABLE_PROMPT_CACHING_1H=1` from a non-chat shell | JAIL+sh | `shim/cc-fleet.zsh:62-66` | |
| `cx` creates its server DETACHED with title plumbing, then attaches | JAIL+sh | `shim/cc-fleet.zsh:116-139` | B1 |
| `_cc_selfswitch` refuses to nest a session inside itself | JAIL+tmux | `shim/cc-fleet.zsh:201-222` | |
| `_cc_in_bunker` → `exec` into the client so no husk survives | JAIL+sh | `shim/cc-fleet.zsh:95,125,197` | |
| `cc-swap <1\|2>` / fzf picker → `cc-db.sh primary-set` is the only writer | JAIL+sh | `shim/cc-fleet.zsh:156-190` | |
| `_cc_label` reads account 1's identity from `~/.claude.json`, not the config dir | JAIL+sh | `shim/cc-fleet.zsh:142-154` | |
| `cc-revive` rebuilds a dashboard of unattached `cc-*`/`cx-*` servers | JAIL+tmux | `cc-fleet.zsh:226-259` | B1 |
| `vsct-revive` restores orphaned bunkers, skipping viewport husks | JAIL+tmux | `shim/cc-fleet.zsh:226-252` | |

## J — Satellite scripts

| flow | safety | expected behavior (source) | regression |
| --- | --- | --- | --- |
| `cc-db.sh init` idempotent schema + WAL | JAIL+sh | `cc-db.sh:68-103` | |
| `cc-db.sh` `.timeout` dot-command, never `PRAGMA busy_timeout` | JAIL+sh | `cc-db.sh:53-57` | |
| `hidden-add` / `-del` / `-has` write BOTH db and carrier | JAIL+sh | `cc-db.sh:106-148` | B2 |
| `hidden-sync` replaces the whole set in one transaction (`WHERE true`) | JAIL+sh | `cc-db.sh:171-200` | |
| `hidden-reap` prunes to the live set from stdin | JAIL+sh | `cc-db.sh:149-161` | |
| `hidden-at-list` (retired payload column) | JAIL+sh | `cc-db.sh:163-169` | |
| `chat-load` / `chat-save` / `chat-prune` name cache, tab-exact field split | JAIL+sh | `cc-db.sh:202-259` | |
| `primary-get` / `primary-set`: roster `1\|2`, mirror kept in lockstep | JAIL+sh | `cc-db.sh:262-275` | |
| `child-add/list/clear` for `new` and `pane` kinds | JAIL+sh | `cc-db.sh:277-303` | |
| `swap-log` single line and stdin-drain forms | JAIL+sh | `cc-db.sh:305-321` | |
| `migrate` idempotent, renames legacy files aside | JAIL+sh | `cc-db.sh:323-366` | |
| Degraded mode: no `sqlite3` → every read/write falls back to the flat files | JAIL+sh | `cc-db.sh:51,17-18` | |
| `cc-hide.sh` id chain: env session id (only if a transcript exists) → pane crumb → socket crumb | JAIL+sh | `cc-hide.sh:31-56` | |
| `cc-hide.sh` refuses when the id is not uuid-shaped (no newest-transcript fallback) | JAIL+sh | `cc-hide.sh:47-56` | |
| `cc-hide.sh --exit` reaps `new` then `pane` children, then kill-PANE (not kill-server) | JAIL+tmux | `cc-hide.sh:64-115` | |
| `cx-hide.sh` `$TMUX` ancestry recovery, trusted only for a `cx-*` socket | JAIL+sh | `cx-hide.sh:38-56` | |
| `cx-hide.sh` cwd fallback when codex strips tmux entirely | JAIL+sh | `cx-hide.sh:66-103` | B1 |
| `cx-hide.sh` hides by the RAW rollout id, not the lineage root | JAIL+sh | `cx-hide.sh:141-151` | **B2** |
| `cx-hide.sh --exit` kill-PANE only | JAIL+tmux | `cx-hide.sh:161-183` | |
| `cc-name-sync.sh --dry-run` prints, changes nothing | JAIL+sh | `cc-name-sync.sh:35-40,148-165` | B4 |
| `cc-name-sync.sh` single-writer lock, no double run | JAIL+sh | `cc-name-sync.sh:44-52` | |
| `cx_db_name` precedence: store `name` → `session_index` → first prompt | JAIL+sh | `cc-name-sync.sh:77-99` | **B4** |
| `cx_db_name` birth window ±120s — a RESUMED thread cannot match | JAIL+sh | `cc-name-sync.sh:87` | **B1** |
| `cx_rollout_for` has NO fallback: nothing within ±120s of pane birth → window left alone | JAIL+sh | `cc-name-sync.sh:105-124` | **B1** |
| Claude window name from the 🔖 + medal scrape; conflicted split window left alone | JAIL+tmux | `cc-name-sync.sh:193-210` | |
| Sessions are never renamed; squatters skipped | JAIL+tmux | `cc-name-sync.sh:172` | |
| `cc-agent-open.sh` per-uuid takeover flock; loser exits 1 | JAIL+sh | `cc-agent-open.sh:18-31` | |
| `cc-agent-open.sh` registry lookup by `sessionId` OR `id` prefix, both accounts | **REAL-SESSION** | `cc-agent-open.sh:52-68` | B3 |
| `cc-agent-open.sh` routing: busy → attach, everything else → takeover | **REAL-SESSION** | `cc-agent-open.sh:107-125` | |
| `cc-agent-open.sh` tmux-resident lock-holder → attach its window instead | JAIL+tmux | `cc-agent-open.sh:90-105` | |
| `cc-swap-chat.sh` account+`--1h` arg parsing and the usage bail | JAIL+sh | `cc-swap-chat.sh:21-33` | |
| `cc-swap-chat.sh --sock` refuses a multi-pane server | JAIL+sh | `cc-swap-chat.sh:46-53` | |
| `cc-swap-chat.sh` `--1h`-only path (no account) keeps the chat's account from its live process env | JAIL+sh | `cc-swap-chat.sh:115-125` | |
| `cc-swap-chat.sh` no-transcript chat → fresh reboot, not a dead `--resume` | JAIL+sh | `cc-swap-chat.sh:79-90` | |
| `cc-swap-chat.sh` per-pane swap flock, open-menu abort, `/exit` timeout refusal | JAIL+tmux | `cc-swap-chat.sh:134-186` | |
| `cc-swap-chat.sh --then` trust-prompt handling, typed-render proof, submit confirm | **REAL-SESSION** | `cc-swap-chat.sh:204-260` | |
| `cc-reap.sh` dry run classifies without touching anything | JAIL+tmux | `cc-reap.sh:34-40,192-194` | |
| `cc-reap.sh` KEEP rules: attached, self, `cc-new-*`, busy, recent transcript write | JAIL+tmux | `cc-reap.sh:126-163` | |
| `cc-reap.sh` fail-closed when the agents query fails or a crumb is missing | JAIL+tmux | `cc-reap.sh:53-69,167-182` | |
| `cc-reap.sh` empty socket younger than 1h left alone (mid-startup) | JAIL+tmux | `cc-reap.sh:105-120` | |
| `cc-archive.sh` dry run default; `--apply`, `--subagents`, `--older-than`, `--restore` | JAIL+sh | `cc-archive.sh:24-62` | |
| `cc-archive.sh` re-checks liveness at run time, never from a cached list | JAIL+sh | `cc-archive.sh:8-11,80+` | |

## K — Installer and systemd units

| flow | safety | expected behavior (source) | regression |
| --- | --- | --- | --- |
| `install.sh` dry run is the default | JAIL+sh | `install.sh:25,99,210-212` | |
| `install.sh --apply` links the 9 fleet scripts into `~/.claude/bin` | JAIL+sh | `install.sh:107-112` | |
| `install.sh` links `chat/*.command.md` → `chat/*.md`, plus `chat/self/` | JAIL+sh | `install.sh:124-138` | |
| `install.sh` links codex skills into `~/.agents/skills` | JAIL+sh | `install.sh:141-150` | |
| `install.sh` backs up a real file as `.pre-professor-<ts>` | JAIL+sh | `install.sh:69-79` | |
| `install.sh --uninstall` restores the newest backup | JAIL+sh | `install.sh:82-95` | |
| `install.sh` rewrites (never appends a second) `~/.zshrc` source line | JAIL+sh | `install.sh:180-206` | |
| `install.sh` sources the LEGACY `cc-fleet.zsh`, not the shim, and installs no binary | JAIL+sh | `install.sh:181` vs `CUTOVER.md:44-55` | |
| systemd units skipped cleanly where `systemctl --user` is absent | JAIL+sh | `install.sh:160-177` | |
| `cc-name-sync.path` fires on `~/.codex/session_index.jsonl` modification | **REAL-SESSION** | `systemd/cc-name-sync.path` | **B4** |
| `cc-name-sync.timer` 2-min drift converge | JAIL+sh | `systemd/cc-name-sync.timer` | |
| `cc-name-sync.service` `ExecStart` points at `~/.claude/bin/cc-name-sync.sh` | JAIL+sh | `systemd/cc-name-sync.service` + `install.sh:107` | |

---

## Flows that CANNOT be jailed

Schedule these deliberately on a scratch project directory. **32 experiments.** The 10 table rows
tagged `REAL-SESSION` are here verbatim; the rest are `JAIL+tmux` rows whose jail proves only the
mechanics (synthetic screen text, fake `/proc`) and which need one authentic engine run before
they count as covered.

**Needs a real `codex` process (15):**
1. live-codex store-only **resumed** thread — identity, name, window (`resolve/codex.go:49-85`).
2. live-codex store-only **fresh** thread that exports `CODEX_THREAD_ID`.
3. Codex writing a rollout and immediately closing it (the fd-scan blind spot).
4. Codex rename inside the TUI → `threads.name` update.
5. Codex rename landing only in `session_index.jsonl` (**B4**).
6. `codex resume <name>` creating a paginated thread that writes no rollout.
7. Two codex chats born in the SAME directory within the birth window.
8. Codex approval / plan-overlay modals during an inject.
9. Codex composer tall-message truncation (the false-green class).
10. `cx-hide.sh --exit` `/quit` flush.
11. `cc-name-sync` converging a live cx window name onto the VS Code tab.
12. Codex-origin `chat.sh` signature via ancestry recovery.
13. `chat_ls` state=`busy` for a genuinely generating codex pane.
14. `cx` launcher self-switch when already inside its own server.
15. ✅ DONE — `cc-fleet headless run --engine cx --name X` against the REAL
    codex TUI (0.147): confirmed live end to end. It found three things no stub
    had: the TUI boots into a hooks/trust modal that swallows keystrokes, its
    modal selection cursor is the SAME glyph as the composer (so readiness
    needs the status line too), and the rename field arrives pre-filled. All
    three are fixtured now. Re-run this experiment on any codex upgrade.
16. ✅ DONE — the two-way surface against BOTH real engines: `run --await`
    (inline and `--prompt-file`, multi-line), three-turn `ask` conversations on
    a codex and a claude seat, `--json`, `--progress`, a `--timeout` that
    expires with the question still in the record, and the read verbs over the
    same live chats. 22 checks, all green. Re-run on any engine upgrade —
    `ask` is only as true as the transcript shapes both engines write.

**Needs a real `claude` process (13):**
16. Agent row: a real non-primary-config-dir claude with `--session-id`.
17. Agent takeover through `cc-agent-open.sh` (`claude agents --json`).
18. The daemon-agent guard on the resume-inject path (`chat.sh:1017-1026`).
19. `/bb` graceful `/exit` flush + Stop hooks + post-exit re-index.
20. Open gate: a live chat whose birth account ≠ primary.
21. `cc-swap-chat.sh` full reboot-in-place (`respawn-pane -k`) + `--then` delivery.
22. Trust-prompt handling on a fresh config-dir/cwd pair.
23. `/chat:branch` fork (`--fork-session`) and its pane-child registration.
24. `/chat:new` and `/chat:new --detach` teammate spawn + numbering.
25. `⚡1h` badge read from a live process's `/proc` environ.
26. `[Pasted text #N]` collapse in a real Claude composer.
27. Dim-SGR placeholder vs a real draft (mash guard).
28. Selector/permission modal + `chat.sh modal deny`.

**Needs real multi-account state (4):**
29. `_cc_label` reading `oauthAccount.emailAddress` per account.
30. `cc-swap` fzf picker → primary flips for `cc` but not `cc1`/`cc2`.
31. Transcripts under a SEPARATE account root (not a symlink back to account 1).
32. Statusline badge computation across accounts.

---

## Residual known divergences

0. **The legacy oracle is blind to sqlite-resident Codex threads** — `ls --check` reports
   go-only `live-codex` rows for them. VERIFIED not a Go defect: the three flagged on
   2026-08-12 (`BUILD_DREAMER`, `CC_FLEET_BUILDER`, `CROSS_WORKFLOW_BUILDER`) were real,
   named, live chats with running codex panes on their sockets. The zsh walk is
   rollout-file-based and codex ≥0.146.1 keeps a paginated thread in
   `~/.codex/state_<N>.sqlite`, so the ORACLE is wrong, not the engine. Every unallowed
   diff gets this treatment — verified against live state, then written down — because a
   red check nobody explains is a red check everybody learns to ignore.

1. **Store-only rows carry no parent link** (`index/codexstate.go`) — evidence-audited and
   found to have no real-world instance: a full sweep of the live state store (213 user
   threads) shows zero cases of a resume minting a second row, and the direct positive case
   (a picker-resumed, twice-renamed thread) kept a single row throughout. Codex ≥0.146 resume
   CONTINUES the same thread id, so there is never an ancestor lineage to merge; the one
   fixture that "reproduced" the split inserted two synthetic rows by hand. Re-opens only if
   codex changes resume behavior — the audit query (same cwd + same non-empty first message,
   grouped) is the detector.
2. **Account roster disagrees three ways.** `paths.go:64-68` builds THREE Claude roots,
   `compose/compose.go:873` accepts accounts 1-3, `pipeline.go:536` labels them 1-3 — but
   `action/synth.go:19` caps at 2 and the shim has no `cc3`. `readPrimaryAccount` clamps
   off-roster values upstream, so launches stay correct; the divergence is dormant, not dead.
3. **`bb.command.md` documents `kill-server`**, but `cc-hide.sh:88-109` deliberately does
   kill-PANE (and the doc's claim would take split siblings down). Stale contract.
4. **`install.sh:181` wires the legacy `cc-fleet.zsh`** and installs neither the Go binary nor
   `cc-fleet/shim/cc-fleet.zsh`; `install.sh:107` links the pre-cutover `cc-hide.sh`/`cx-hide.sh`
   rather than the `exec cc-fleet hide --self` delegates `CUTOVER.md:69-84` specifies. This is
   the cutover decision itself — flips only on the owner's word.
5. **`mcpserv/backend.go:303-308` counts `Agent` as live** while `ui/model.go:467-471` does not.
   An agent row is capture-probed for busy state in MCP and treated as non-live in the TUI.
6. **CWD encoding divergence:** `index/index.go:307` replaces only path separators, while
   `chat.sh:394`, `chat.sh:1232` and `chat.sh:1253` do `tr '/.' '--'`. A project path containing
   a dot resolves differently in the two halves.
7. **`chat.sh:1104-1106` prefers the carrier FILE over the db** ("the file wins when it
   exists"), while `shared/shared.go:241-277` takes the UNION. A db-only hide is invisible to
   `chat.sh ls`.
8. **`cc-agent-open.sh:62` registry lookup degrades to "not found" on timeout** (`cc_timeout
   20` vs real `claude agents --json` latency of 10-20s under load) — and the fallback then
   runs `claude --resume` on the LIVE session, which claude 2.1.224 does NOT refuse: a second
   claude boots on the same transcript (observed live; killed before it wrote). The
   tmux-resident attach branch is unreachable behind the same miss. Fix wants a longer/retried
   lookup and a live-pid guard before any fresh resume.
9. **A booting chat is invisible to the Go picker until its first statusline render** writes
   the SID crumb — a spawn wedged at a startup prompt (folder trust, MCP approval) is listed
   by legacy (pane scan) and missed by Go (crumb scan) indefinitely. Observed live on a
   `cc-new-*` teammate at the MCP prompt; the row appeared the moment the prompt was answered.
10. **`chat.sh branch` reports success and registers the child before verifying the fork
    survived.** A branch of a never-prompted chat always dies ("No conversation found" —
    transcripts are written on the first message) yet prints the success line and leaves a
    corpse child registration.

---

## Top 10 — where the next bugs are hiding

Ranked by tonight's pattern: identity resolution across resume/store/fork edges, cross-writer
state, and live-vs-indexed disagreement.

| # | flow | why it hides bugs |
| --- | --- | --- |
| 1 | **Live codex → indexed row join** (`compose/compose.go:492-513`) | The join walks path → resolver ThreadID (`liveCodexID`); a process neither resolves is still a guess. Every Codex row bug passes through here. |
| 2 | **Store-only thread lineage** (`index/codexstate.go:98-130`) | A resumed store-only thread is given itself as its lineage root, so it can never merge with the conversation it continues. Hides, names and prompt counts all split. |
| 3 | **Codex name precedence across three writers** (`index/index.go`, `cc-name-sync.sh:77-99`, `naming/naming.go`) | Name provenance (`cx_names.source`/`renamed_at`, schema v4) settles store-vs-file, but three walkers still differ on lineage breadth, and `reloadCxNames` still wipes and re-folds in two transactions. |
| 4 | **Agent-row identity** (`gather/agents.go:12-69` → `compose/compose.go:541-590` → `hide/manager.go:138-165`) | An agent row is the only kind whose ID can exist with no index row behind it. Every id-keyed operation must vouch an engine for it; a new call site that forgets recreates the silent-refusal bug. |
| 5 | **Hide across the two writers** (`shared/shared.go:193-277`, `cc-db.sh:106-148`, `chat.sh:1100-1106`) | Four readers with three different precedence rules (union / file-wins / db-only). A hide that half the fleet honours is exactly how a chat "comes back". |
| 6 | **Codex thread ↔ pane pairing** (`resolve/codex.go:49-85`, `cc-portable.sh` `cx_thread_id`, `cc-name-sync.sh:87`) | The argv `resume <uuid>` rung now leads and the ±120s window only backstops fresh threads — but a resumed thread with a scrubbed argv still falls to the window, and the failure mode differs per caller. |
| 7 | **Live-vs-indexed refresh race in the picker** (`pipeline.go:315-424`, `ui/model.go:247-271`) | Three snapshots stream into an open TUI while the user is toggling hides. A row that changes identity between frames takes the pending hide with it. |
| 8 | **`collapseLiveServers` + `ServerCount`** (`compose/compose.go:759-803`) | Silently merges rows by `engine+ID`. Two genuinely different chats that resolve to the same id (see #1) disappear into one row; the loser is unreachable. |
| 9 | **`--exit` finisher choreography** (`hide/finisher.go:94-245`) | Detached, delayed, and it re-asserts a hide AFTER an index refresh. It races the engine's own transcript flush and reaps teammates by an id that may already have been canonicalized. |
| 10 | **Primary-account round trip** (`pipeline.go:553-631`, `shared/shared.go:441-482`, `cc-db.sh:262-275`, `shim/cc-fleet.zsh:105`) | Two files, two writers, four readers, and a roster constant that disagrees with the path layout (see finding 8). The picker and the launcher can silently differ on which account a chat is born under. |
