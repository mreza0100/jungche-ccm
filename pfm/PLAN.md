# pfm (Professor-Fleet-Manager) — Go rewrite of the cc-ls chat-fleet tooling

**Builder:** the Codex chat `CC_FLEET_BUILDER` (this is your brief — work one WP at a time).
**Supervisor:** the planning Claude chat — reviews diffs read-only, runs fixture tests, never edits code.
**Committer:** gitter only. You never run git; report when a WP is ready and the supervisor routes the commit.
**Ground rule:** until WP11 you create/modify files ONLY under `~/work/host-ops/pfm/`.
The living zsh (`oldbox/scripts/pfm.zsh` and satellites) stays untouched and in service.

## Why

`cc-ls` today is `oldbox/scripts/pfm.zsh` — 1,293 lines of zsh + fzf listing every live and
resumable Claude/Codex chat on the box. It is slow and brittle:

- **Fork storm.** Two forked procs per transcript (`head|grep`) ≈ 56–65 ms/file vs ~0.4 ms
  single-process. Corpus: **12,716 transcripts / 3.75 GB** (one physical store), 211 codex
  rollouts / 385 MB, 16 live tmux sockets. `_cc_isbg` alone forks ~300× per refresh; 4–6 forks
  per rendered row; `date` per row; `tac` over multi-MB files; cold `_cc_meta` = jq + 2 greps
  over files up to 195 MB.
- **Scattered state.** `/tmp/cc-sid/*` crumbs, `.namecache.v5` (TSV, rewritten wholesale,
  unlocked → concurrent runs lose updates), `~/.claude/.cc-ls-hidden{,.at,.lock}`, codex
  session_index — plus naming logic duplicated 3×, hide-ratchet 4×, already drifting.
- One month of history: ~1 fix per working day; the last five commits are bug-fixes to
  previously shipped behavior.

Decisions (locked): **Go**, full ecosystem in one binary, **on-demand incremental SQLite** (no
daemon), greenfield in `pfm/`, fancy Bubble Tea TUI.

## Tech stack (research-confirmed 2026)

- `charm.land/bubbletea/v2` + `charmbracelet/bubbles` + `charmbracelet/lipgloss` — verify the
  current v2 API against the bubbletea repo before coding UI.
- `sahilm/fuzzy` for find-as-you-type matching.
- `modernc.org/sqlite` — pure Go, `CGO_ENABLED=0`, static binary. (Escalation if profiling ever
  demands: `zombiezen.com/go/sqlite`.)
- Go via mise: `mise use -g go@1.24` (pinned; NOT latest). Nothing else. No cobra, no config
  system, no telemetry — boring Go for one box.

## Design keystones

- **K1 — eval protocol.** The binary never execs the final tmux attach. TUI renders on
  `/dev/tty`; preparatory side effects (solo-reap, swap-reboot, `window-size latest`, crumb
  sweeps) run inside the binary; then ONE shell line goes to stdout and the zsh wrapper evals it:
  `cc-ls() { local out; out="$(pfm ls "$@")" || return $?; [[ -n "$out" ]] && eval "$out"; }`
  Why: bunker semantics need `exec tmux attach` to replace the interactive shell; ✦-new rows
  must call the `cc`/`cx` shell functions; emitted lines are golden-testable. Info → stderr,
  TUI → /dev/tty, command → stdout. Open-gate reads its one key from /dev/tty inside the
  binary; the selfswitch case runs `select-window` itself and emits nothing.
- **K2 — ratchet in prompts, not bytes.** The indexer keeps an exact incremental `prompt_count`
  per file; hide baselines are `baseline_prompts`; auto-unhide = count > baseline. Same
  observable behavior as the zsh byte-ratchet, ONE implementation. Legacy `.at` byte baselines
  converted exactly at import (scan past legacy baseline once: real prompt found → not hidden;
  else baseline = current count).
- **K3 — one implementation each**: naming precedence, hide-ratchet, row classification,
  run-string synthesis — each in ONE package with table-driven tests.
- **K4 — test jail.** All tests: `TMUX_TMPDIR = t.TempDir()`; real paths overridable via env
  `PFM_DB / _SID_DIR / _CLAUDE_ROOTS / _CODEX_ROOT / _TMUX_DIR / _HOME` (test-only, not a
  config system); `/proc` behind a `ProcFS` interface with fixture fakes. **No test may touch
  live `cc-*`/`cx-*` sockets or `/tmp/cc-sid`.** Keep scratch socket paths SHORT (long
  TMUX_TMPDIR paths hit "File name too long").

## Repo layout

```
pfm/
  go.mod                      # module hostops/pfm
  cmd/pfm/main.go        # stdlib flag dispatch
  internal/store/             # sqlite open/migrate/pragmas + queries (schema.sql via go:embed)
  internal/index/             # walk.go claude.go codex.go cxindex.go — delta parse engine
  internal/naming/            # naming-precedence pure functions + junk filter
  internal/gather/            # tmuxprobe.go crumbs.go codexproc.go agents.go cache1h.go procfs.go
  internal/compose/           # row model, classification, ⊞ merge, ⚠Nsrv, hide/caps/sort
  internal/action/            # run-string + attach-line synthesis, solo, open-gate, quoting
  internal/hide/              # hide/unhide/self-identify + --exit detached choreography
  internal/resolve/           # label|session|cxwin resolvers (chat.sh contract)
  internal/ui/                # Bubble Tea v2 picker behind a Picker interface; plain/tsv renderer
  shim/pfm.zsh           # post-cutover thin wrapper (WP10; wired only at WP11)
  testdata/                   # claude-store/ codex-store/ crumbs/ proc/ golden/
```

## SQLite schema + DB policy

DB: `~/.local/state/pfm/fleet.db` (spans 3 accounts + codex → not under ~/.claude).
`PRAGMA user_version` migrations. Every open: `journal_mode=WAL`, `synchronous=NORMAL`,
`busy_timeout=10000`, `foreign_keys=ON`, `SetMaxOpenConns(1)`. Writes are tiny
`BEGIN IMMEDIATE` txns; cold full-index commits in ~500-file batches (no txn holds the write
lock >~1 s). `hide`/`unhide`: retry once on BUSY, then WARN LOUDLY and **exit 0** — the /bb
choreography must never abort on a lock (matches today's advisory flock semantics). `doctor`
surfaces dropped writes.

```sql
CREATE TABLE transcripts (
  uuid TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE,
  size INTEGER NOT NULL DEFAULT 0, mtime_ns INTEGER NOT NULL DEFAULT 0,
  parsed_offset INTEGER NOT NULL DEFAULT 0,   -- always ends on a newline boundary
  cwd TEXT NOT NULL DEFAULT '',
  custom_title TEXT NOT NULL DEFAULT '',      -- last custom-title/agent-name record
  ai_title TEXT NOT NULL DEFAULT '',          -- last ai-title (precedence computed, never baked)
  first_prompt TEXT NOT NULL DEFAULT '',      -- first real junk-filtered prompt
  last_prompt TEXT NOT NULL DEFAULT '',       -- kills the tac-over-195MB path
  prompt_count INTEGER NOT NULL DEFAULT 0,
  is_bg INTEGER NOT NULL DEFAULT 0);          -- sessionKind:"bg" seen in head
CREATE INDEX transcripts_mtime ON transcripts(mtime_ns DESC);

CREATE TABLE rollouts (
  id TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE,
  size INTEGER NOT NULL DEFAULT 0, mtime_ns INTEGER NOT NULL DEFAULT 0,
  parsed_offset INTEGER NOT NULL DEFAULT 0, cwd TEXT NOT NULL DEFAULT '',
  user_thread INTEGER NOT NULL DEFAULT 0,     -- thread_source=="user"; 0 == zsh's §sub§
  session_id TEXT NOT NULL DEFAULT '', parent_thread TEXT NOT NULL DEFAULT '',
  first_prompt TEXT NOT NULL DEFAULT '', prompt_count INTEGER NOT NULL DEFAULT 0);
CREATE INDEX rollouts_mtime ON rollouts(mtime_ns DESC);

CREATE TABLE cx_names (id TEXT PRIMARY KEY, thread_name TEXT NOT NULL); -- session_index mirror
CREATE TABLE hidden (
  id TEXT PRIMARY KEY, engine TEXT NOT NULL CHECK (engine IN ('cc','cx')),
  hidden_at INTEGER NOT NULL,
  baseline_prompts INTEGER);                  -- NULL = lazy-baseline on next sight
CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
-- meta keys: cx_index_size, cx_index_mtime_ns, legacy_import_done, last_full_index_at
```

Delta rules (mirror of zsh `_cc_metac`/`_cx_metac`): (size,mtime) unchanged → skip file; grown →
stream-parse `[parsed_offset, EOF)` — delta custom-title always wins, delta ai-title updates
`ai_title` only, `first_prompt` fills once, every real prompt updates `last_prompt`,
`prompt_count += new`, new offset = after the last COMPLETE line (partial tail re-read next
run); shrunk → full reparse from 0 (fields reset); deleted → row deleted (anti-join vs the
walk's present-set). Parser: `bufio.Reader.ReadBytes('\n')` — records routinely exceed 64K so
NEVER default `bufio.Scanner` — with a contains-prefilter (`custom-title`, `agent-name`,
`ai-title`, `"type":"user"`, `sessionKind:"bg"`, `isCompactSummary`) before any
`json.Unmarshal`; the 195 MB file must stream without RSS blowup. Walk:
`realpath`-deduped roots of `~/.cc/{1,2,3}/projects` (on this box all → `~/.claude/projects`;
the dedup fixes the dormant account-2/3 bug), `filepath.WalkDir`, NEVER follow symlinks (skips
per-project `memory` links), maxdepth 2, `*.jsonl` only.

## Scan pipeline (every ls/open/resolve run; `index` runs B alone)

**A — gather** (parallel errgroup, target ~50–100 ms):

1. tmux socket enumeration under `${TMUX_TMPDIR:-/tmp}/tmux-$UID`, skip `vsct`/`revive*`; one
   concurrent `list-panes -a` per socket (same field format as zsh:797); corpse-socket sweep
   with the >1 h age guard.
2. Crumb read of the sid dir with a STRICT filename grammar — a crumb is exactly `<sock>` or
   `<sock>.%<pane>`; everything else (`.then-failed`, `.open.*`, `.takeover-*`, dot-temps) is
   ignored (fixes the sentinel-collision bug). Stale pane-crumb sweep as today.
3. Codex `/proc` fd-walk (port of `_cx_scan`: numeric fd order, first user-thread rollout, map
   onto pid + ≤4 ancestors) via the ProcFS interface.
4. Agent scan (port of `_cc_agents`) reading `/proc/*/cmdline` + `environ` directly — no ps
   fork; strict uuid extraction of `--session-id`/`--resume`; `CLAUDE_CONFIG_DIR` from environ.
5. c1h probe (`FORCE_PROMPT_CACHING_5M=1` in environ of claude pids on the server's ttys).
   Side effect preserved: **codex window-name convergence** to thread_name **clipped to 24 chars**
   (chat.sh `_resolve_cxwin` depends on exactly this).

**B — index**: stat-walk all roots in one process (12.7k files well under 1 s warm), delta-parse
per the rules above, reload `cx_names` only when session_index.jsonl (size,mtime) moved, batched
txns, delete vanished rows.

**C — compose** (pure functions; gather snapshot + DB in, `[]Row` out). Kind ∈ {LiveClaude,
LiveCodex, LiveSplit, Agent, ResumeClaude, ResumeCodex, NewClaude, NewCodex} + flags (attached,
here, accounts, c1h, srvCount, hiddenTag) + hidden counter. Rules ported 1:1:

- pane-crumb beats socket-crumb; socket-crumb trusted only while a claude actually runs on that
  server's ttys (else crumb swept); `live` dedupe set vs resumable rows.
- ⊞ split merge (≥2 pane crumbs on one socket): `+`-joined names, summed bytes/counts, newest
  pane mtime, unioned medals; never hidden, never collapsed.
- ⚠Nsrv collapse: same transcript/rollout on N live servers → newest socket birth-epoch wins,
  survivor gets `⚠Nsrv`.
- bg-twin skip (`is_bg`): strict mode only, never for live agents; visible under `-a`.
- ONE hide/auto-unhide function used by all four row sources; `--hidden` inverts; `-a` tags
  `·hidden`.
- size-0 + promptless hiding (agents exempt); caps: 30 Claude / 15 Codex shown (strict). The
  zsh's 120-candidate codex bound is dropped (perf hack, moot with the DB) — allowlisted
  deviation in `--check`.
- Live naming fallback: indexed name → pane title (ignoring "Claude Code"/empty) → session
  name → `last_prompt` from DB (no tac, ever).
- Sort project ▸ recency; ⌃R rotation is a view transform; a PROJDIR map (project → newest real
  cwd, $PWD-seeded) feeds ✦-new rows so **new chats launch in the rotated project's dir**.

## Naming — one pure package (`internal/naming`)

- `DisplayName(customTitle, aiTitle, firstPrompt)`: custom/agent-name (last) → ai-title (last,
  STRICTLY second — a renamed chat keeps emitting ai-title afterwards; folding them would
  relabel every renamed chat) → first real prompt.
- `IsJunkPrompt`: `^(<[a-z]|Caveat:|\[Request` prefixes; isCompactSummary records are never
  prompts.
- `FlattenPromptText(json.RawMessage)`: string content as-is; array → join top-level text
  blocks (never tool_result); squash tabs/newlines.
- `LiveFallback(indexed, paneTitle, sessionName, lastPrompt, isCCSock)`.
- `CxName(ownID, sessionID, parentThread, names, firstUserMsg)`: lineage order, 60-char clip.
- 24-char cx window clip at the gather call site; 30-char display clip in ui.
  Table-driven tests transcribed from the zsh comment blocks + fuzz on FlattenPromptText.

## Subcommands

```
pfm ls [-a|--all] [-H|--hidden] [<uuid>] [--plain|--tsv|--check]
pfm open <uuid>                     # cc-open port; stdout = eval line
pfm index [--full] [--progress]
pfm hide [--self|<id>] [--exit]  ·  unhide <id>  ·  hidden
pfm resolve label|session|cxwin <name>   # chat.sh contract: "sockpath\tpane" rc 0/1/2, candidates → stderr
pfm revive  ·  legacy import|export  ·  doctor  ·  version
pfm mcp                             # stdio MCP server — the /chat:* family as tools (WP12)
pfm internal hide-exit …            # hidden: detached --exit finisher (setsid re-exec)
```

Satellite policy: the binary SHELLS OUT to `cc-swap-chat.sh` (`--sock <sock> <acct> --1h <0|1>`)
and `cc-agent-open.sh` (`<uuid> <cwd> <ocfg>`) — argv contracts untouched. At cutover,
`cc-hide.sh`/`cx-hide.sh` become 2-line delegators to `pfm hide --self [--exit]`; their
`--exit` choreography (sleep 1.5 → type `/exit`|`/quit` → poll ≤20 s → kill-pane → crumb sweep →
post-flush baseline → teammate reaping of `~/.claude/.cc-new-children/<uuid>` +
`.cc-pane-children/<uuid>`) moves into `internal/hide`, launched detached via Setsid. Stays zsh
in `shim/pfm.zsh`: `cc`/`cc1-3`/`cx`, `_cc_run`, `_cx_server`, `_cc_arm1h`, `_cc_primary`,
`_cc_in_bunker`, `cc-swap`, `vsct-revive`, + the one-line eval wrappers. `~/.claude-primary`
and `/tmp/cc-sid` crumbs stay plain files (statusline-command.sh, `pfm reap`, cc-swap-chat,
chat.sh all keep reading/writing them).

## TUI — the fancy spec

Bubble Tea v2 + bubbles + lipgloss behind a `Picker` interface (`plain.go`/`tsv.go` renderers
keep it honest). **Fuzzy find-as-you-type** (`sahilm/fuzzy` over project+name). **Live preview
pane** for the highlighted row: name, cwd, account medal, size, age, prompt count, last prompt —
all from the DB/gather snapshot, zero forks. Styled header (account medal + cache state),
project-grouped blocks, badges (⬢ ⚙ ⚠Nsrv 🥇🥈🥉 ·hidden), right-aligned relative ages. Cached
rows paint first (**<100 ms to first paint**), live gather refreshes asynchronously in-frame.
Keys: **⌃T real reload** (gather+index+compose), ⌃R rotate project block (cursor follows row
id), ⌃X hide-toggle with cursor memory, ⌃E ⚡1h arm (in-memory), ⌃S account cycle (writes
`~/.claude-primary`), **⌃B live-chat reboot** (kill server, sweep socket+crumbs, degrade to
resume path), Enter dispatch by kind, Esc. Mouse disabled (deliberate — focus-clicks stole the
cursor in fzf). Fancy never buys sluggish.

## Migration / cutover

1. **Shadow**: `pfm ls --check` runs `zsh -ic 'cc-ls -a'` with fzf masked from PATH
   (forces the zsh plain-print fallback — zero modification of the script), strips ANSI, diffs
   `(id, kind, project, prompt-count)` tuples vs its own compose output, allowlisting the
   intentional deviations (codex 120-bound, prompt-vs-byte baseline edges, partial-tail
   counts). Run repeatedly across days; empty diff = parity.
2. **Import**: `pfm legacy import` — cold `index --full`, then hidden + `.at` conversion
   per K2. Legacy files stay in place untouched (simply no longer read).
3. **Cutover (WP11 — the ONLY step touching existing files; gitter commits):** flip
   `~/.zshrc:56` to source `pfm/shim/pfm.zsh`; replace the two hide scripts with
   delegators; update `docs/pfm.md` (drafted in `CUTOVER.md` beforehand). VS Code tasks
   keep working — command names are unchanged.
4. **Rollback**: flip the source line back + `git restore` the hide scripts; `legacy export`
   first if SQLite-era hides must survive (writes `.cc-ls-hidden`/`.at` under the legacy flock).

## Bug ledger (fixed by construction — each has a test)

| Bug in the zsh                                                          | Fix                              | WP  |
| ----------------------------------------------------------------------- | -------------------------------- | --- |
| ⌃T reload bind targets a never-created script; real handler unreachable | native ui keybindings            | 8   |
| Blank line in hide file blanks the whole list (substring grep -vFf)     | exact-id set from `hidden` table | 5   |
| ✦-new rows launch in $PWD, ignoring ⌃R rotation                         | action uses row's PROJDIR        | 7   |
| Only account-1 store walked (siblings walk all)                         | realpath-deduped roots           | 3   |
| Unlocked wholesale namecache rewrite races                              | SQLite WAL txns                  | 1/3 |
| `<sock>.then-failed` collides with crumb glob                           | strict crumb grammar             | 4   |
| `_cc_isbg` fork-per-file storm                                          | `is_bg` column at index time     | 3   |

## Work packages — one per session, in order; every WP ends `go vet ./... && go test ./...` green

- **WP0 — toolchain + skeleton.** `mise use -g go@1.24`; `go mod init hostops/pfm`;
  `cmd/pfm` dispatch + `version`; package stubs; `internal/paths` env-override helper (K4);
  `testdata/` scaffold. Accept: `CGO_ENABLED=0 go build ./...` produces a static binary;
  `pfm version` prints.
- **WP1 — store.** Schema, migrate, pragmas, plain query functions, batch-txn helper,
  busy-retry policy. Accept: fresh-DB migrate honors user_version; a 2-process concurrent
  hidden-write test loses nothing.
- **WP2 — naming.** All functions + table tests from the zsh comment rules; fuzz
  FlattenPromptText (must never panic).
- **WP3 — index.** Walk + both parsers + delta engine + cx_names loader. Fixtures: grown-delta,
  shrunk-rewrite, partial trailing line, custom-title-then-ai-title ordering, bg twin, compact
  summary, subagent rollout, memory-symlink in a project dir, a >70 KB single line. Accept:
  golden `index.tsv` parity; no-change rerun touches 0 rows (change counters); delta run reads
  only appended bytes (bytes-read counter).
- **WP4 — gather.** All probes behind ProcFS + jailed-tmux tests: 3 scratch servers (cc-, cx-,
  one corpse socket file) inside `TMUX_TMPDIR=$(mktemp -d)` running `sleep`; probe returns
  exact rows; corpse swept only when >1 h; crumb grammar rejects `then-failed`. Cleanup kills
  its own servers.
- **WP5 — compose.** Full §C incl. K2 hide/auto-unhide + hidden counter. Accept: golden-row
  tests from fixture DB + fake gather structs for default / -a / --hidden / split / ⚠2srv /
  bg-twin / agent / caps-overflow.
- **WP6 — hide + resolve.** /bb both engines + `internal hide-exit`; resolve trio with the
  exact chat.sh output/rc contract incl. the same-chat ⚠Nsrv tie-break and 24-clip. Jailed
  tests: a pane printing a fake 🔖/🥇 statusline for resolve; hide --exit against a pane whose
  sh exits on any input (assert pane death + baseline write).
- **WP7 — action.** Run-string synthesis N/C/L/A/R/X × bunker × account 1/2/3 × 1h on/off;
  quoting helper; solo port (pane-crumb kill-pane vs provably-alone kill-server vs stray-TERM
  with keep-tty exemption); open-gate rendering + key read (injected io.Reader). Accept: golden
  command lines byte-compared against strings hand-extracted from the zsh (`golden/cmdlines.txt`)
  — the load-bearing parity test for launch env hygiene (`env -u CLAUDE_CODE_SESSION_ID -u
CLAUDECODE -u CLAUDE_CONFIG_DIR …` + `ENABLE_PROMPT_CACHING_1H`/`FORCE_PROMPT_CACHING_5M`
  selection + the cc-agent-open failure net). Solo in jailed tmux.
- **WP8 — ui.** The fancy picker per spec. Accept: model-level KeyMsg tests (rotation,
  hide-toggle cursor memory, 1h arm, account cycle, reload request, Enter outcome per kind);
  fixed-width ANSI render goldens incl. the preview pane.
- **WP9 — cli wiring.** End-to-end in a fully jailed env (fixture stores + jailed tmux + fake
  sid dir + temp DB). Accept: jailed `ls --tsv` == `golden/e2e.tsv`; `open` emits the golden
  eval line; `index` twice is idempotent; `doctor` clean; `testdata/e2e.sh` runnable by the
  supervisor.
- **WP10 — shadow + shim.** `ls --check`; write `shim/pfm.zsh`; draft the docs update as
  `CUTOVER.md` (not applied). Accept: `--check` on the live box, read-only, reports empty or
  allowlisted-only diff at three different times of day.
- **WP11 — cutover.** `legacy import`; flip the zshrc source line; delegate the hide scripts;
  apply the docs update; run the final checklist; keep the rollback recipe in `CUTOVER.md`.
- _*WP12 — MCP server (the /chat:* family as tools)._* `pfm mcp`: a stdio MCP server via
  the official `github.com/modelcontextprotocol/go-sdk`, so Claude Code and Codex chats call
  the fleet as tools instead of shelling to chat.sh. New `internal/inject` (port of chat.sh's
  inject core: resolve target → capture guard → `send-keys -l` + Enter; sender signature for
  plain text; `--force-now` Esc interrupt) + `internal/mcpserv`. Tools:
  `chat_ls` (rows as JSON: session, engine, idle/busy, dir, name, account),
  `chat_resolve` (label|session|cxwin → sockpath+pane, candidates on ambiguity),
  `chat_inject` (target, message, force_now — MUST carry chat.sh's safety rails verbatim:
  the open-selector-menu guard that aborts rather than pressing Enter into a menu, for both
  engines' marker glyphs ❯ and ›),
  `chat_capture` (target → screen text),
  `chat_find` (excerpt → sid/path/date candidates; DB-accelerated, grep-confirmed),
  `chat_read` (sid|path, last N → transcript extract).
  Accept: stdio handshake test; each tool golden-tested against jailed tmux fixtures; the
  inject-guard fixture suite (open menu → abort rc, idle composer → typed, `› 1.` numbered
  draft → allowed) green. Registration at cutover+: `claude mcp add pfm -- pfm mcp`
  - codex `config.toml` `mcp_servers`; chat.sh itself stays untouched and working.

**Final acceptance (supervisor, live box):** warm `pfm ls --plain` < 1 s wall; `--check`
empty; ⌃T/⌃R/⌃X round-trips work; /bb in a scratch chat hides + exits it and a later real
prompt auto-unhides; `resolve session <name>` output identical to chat.sh `_resolve_session`;
Enter from a vsct bunker replaces the viewport (no husk); rollback drill passes; a blank line
appended to the legacy hide file changes nothing.

## Reference sources (read, never modify)

- `oldbox/scripts/pfm.zsh` — the behavioral source of truth (row composition, keybinds,
  Enter dispatch, launch env hygiene).
- `oldbox/scripts/cc-hide.sh` + `cx-hide.sh` — the /bb choreography being absorbed.
- `oldbox/scripts/cc-swap-chat.sh`, `cc-agent-open.sh` — satellites the binary shells out to.
- `~/.claude/commands/chat/chat.sh` lines ~200–344 — the `_resolve_*` contracts.
- `~/.claude/statusline-command.sh` — the crumb writer (format: single line, no trailing
  newline, atomic dot-tmp + mv).
