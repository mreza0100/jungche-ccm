# Wave: config-ui-mcp-install

**Status:** SCHEDULED → pfm-wave-2 (2026-08-20)
**Refined:** 2026-08-20 · main @ 7b01caa
**Scheduled:** #11 → wave 1; #13, #14 → wave 2; #16 → wave 2 HELD (RE-REFINE F3). #1–#10, #12, #15, #17 already DONE — see `docs/dev/trains/pfm-wave-2/train.md` § Source Reconciliation.
**Epic:** none
**Touches:** pfm, blueprint
**Scope:** the founder's 2026-08-20 backlog in one wave — global JSON config (accounts/emoji/theme/permission flags), account-attribution fix, LS picker action-carousel redesign, stats Limits sub-tab + Fable statusline line, /swap→/reload, chat+harvester MCP behind one HTTP daemon, harvester cache proof, installer completeness (hooks, cleanupPeriodDays, codex agents), `pfm update`, `pfm init`, e2e install/update/uninstall harness (Linux container + macOS runner), blueprint persona/naming fixes, epic-manifest inject hook, `internal/ask` contract with dual-engine (codex-luna / claude-haiku) abstraction.
**Deferred:** harvester prompt→GPT production feature (RND-gated — frr findings land in § RND, POC next wave); harvester internal byte-parity with the node harvester (builder's paused queue); harvestpy Torch-tier opt-in sizing (separate wave); pfm mcp auth beyond loopback binding; limits history charts (limit-dashboard's SQLite history — Limits sub-tab ships live-only).

## All-task rules

- **Public repo.** No founder name, email, or machine-absolute `/home/…` path in any code, fixture, comment, or doc. Test fixtures invent neutral values.
- **Only gitter commits; publication (push/tag/release) is founder-owned** and never instructed by this spec.
- **Live-box law:** never reboot devbox; never touch live `cc-*`/`cx-*`/`vsct*` sockets or real `~/.claude`/`~/.cc`/`~/.codex` in tests — every fixture runs in a temp HOME/jail. Never press Enter/⌃O in a jailed picker (launch path escapes the jail); drive picker tests through the pure `ui` layer or `--plain`.
- **Ship = installed:** every pfm task ends with `go build -o ~/.local/bin/pfm ./cmd/pfm` and hash-verifies which binary PATH resolves.
- **Build agents:** `dev` per task, `qa` per modified project, except tasks tagged `[CMD: /pcm]` (blueprint/CLAUDE.md/.claude are hook-guarded).
- **Pre-authorized founder touchpoints:** writing `~/.config/pfm/config.json` on devbox (Task #1 rollout); one Haiku API call per stale-token limits refresh (Task #6); installing the MCP daemon systemd user unit on devbox (Task #8); registering MCP servers in client configs when enabled (Task #10). No other outward-facing action is authorized.

## Task Reconciliation

| Original (founder, 2026-08-20) | Disposition | New # | Notes |
| --- | --- | --- | --- |
| check out feat/autonomy-opt-in | REFINED (ruling: don't merge) | #1 | bypass becomes per-account/per-engine config flags; branch stays parked, superseded |
| global JSON config (accounts, emoji, theme, "whatever") | REFINED | #1, #2 | schema v2 + `pfm config` CLI; theme engine separate task |
| chats auto-delete 30d → maximize on install | REFINED | #10 | live boxes already at 36500; installer must own it |
| stats sub-tab with account limits (limit-dashboard tricks + Haiku ACK fallback) | REFINED | #6 | |
| all chats shown at account 1 | REFINED | #3 | reproduced live; `~/.cc/1` symlink breaks prefix match |
| install misses hooks / Darwin bugs / container + Darwin machine verify | REFINED | #10, #5, #11 | hooks+cleanup → #10, darwin code fixes → #5, harness → #11 |
| ls hotkeys: action carousel, engine halves, claude color | REFINED (approved w/ Tab=tabs) | #4 | |
| /chat:* → MCP, harvester separate server, same process | REFINED (ruling: HTTP daemon) | #8 | |
| harvester byte-identical internals | DEFERRED | — | builder's queue |
| harvester cache expiry (1 day) | REFINED | #9 | TTL already exists (24h default) — prove or fix with the current binary |
| harvester prompt→GPT via Codex | DEFERRED after RND | § RND | frr launched during refine; POC next wave |
| Verdict footer in blueprint CLAUDE.md + live-source diff | REFINED | #13 | |
| dedicated serial integration test, isolated env, no markdown greps | MERGED INTO #11 | #11 | |
| /swap → /reload | REFINED | #7 | |
| templates say "user", no name placeholder | REFINED | #14 | retire `{FOUNDER_NAME}` |
| Fable-only limit in statusline | REFINED | #6b | rides the same usage-endpoint extension |
| E_{EPIC}_* name → inject manifest.md once | REFINED | #15 | /rename needs no hook — per-prompt re-check |
| e2e test also uninstalls | MERGED INTO #11 | #11 | uninstall phase |
| `pfm update` updates whole professor + pipeline test | REFINED | #12, #11 | |
| per-project local install | REFINED | #12b (`pfm init`) | |
| rename transcript_read → transcript_ask | REFINED | § RND, #8 | one merged tool, targeting params optional |
| Claude engine (haiku low) for ask, engine in config | REFINED | #1, #17 | `ask.engine` + dual engine registry |
| founder spec-review (config/update/MCP/ask/epic amendments) | REFINED | #1, #8, #10, #12, #15, #17 | folded verbatim into those tasks |

---

### Task #1 — Config schema v2 + `pfm config` CLI [MILESTONE]

Everything downstream reads accounts/emoji/theme/permission posture from one file. **Why:** four divergent hardcodes exist today (`config.go:140` `<=3`, `swap.go:260` `[]int{1,2,3}`, `installer.go:767` `<=4`, `statusline/render.go:406-428` literal `.cc/N` paths) and the bypass flags are unconditional at every launch site.

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. `~/.config/pfm/config.json` (path via existing `ResolvePath`, `$XDG_CONFIG_HOME` honored) parses as v2; a v1 file (no new fields) still loads — new fields default.
b. Per-account overrides fall back to top-level: effective permission posture for (account, engine) = `accounts[i].claude.permissionMode` else `claude.permissionMode`; same for `codex.yolo`.
c. `pfm config init` writes the default file as **strict, valid JSON — no comments** (0600, atomic via existing `writeAtomic`), refuses to overwrite without `--force`, and prints per-field documentation to stdout on completion (docs, not the file, carry the explanations). `pfm config show` prints the resolved config with per-field `(default)`/`(file)` provenance (same style as `pfm doctor`'s config lines) and **redacts secrets by default** (the MCP auth credential and any future secret field render as `<redacted>`). `pfm config validate` loads and reports OK or the exact decode error with position.
c2. **Failure semantics:** missing file → defaults. Malformed/invalid file → **hard failure** for every command (config carries accounts, permission posture, and MCP settings — silent defaults are a security hazard). Exactly three diagnostic commands stay usable on a broken config — `pfm doctor`, `pfm config show`, `pfm config validate` — each running on defaults while printing the config error prominently enough to repair it.
d. `DisallowUnknownFields` stays — a typo'd key is a hard validate error.
e. Launch sites consume the posture: the four sites that append the bypass pair (`internal/action/synth.go` claude/codex command builders, `agentopen/real.go`, the zsh shim via an emitted env/flag) append it only when the effective posture says so. Devbox rollout step (pre-authorized): `pfm config init` + set every account's posture to today's live behavior (`bypass`/`yolo:true`) **before** the new binary is installed — fleet behavior unchanged.

**Data model:** none (JSON file, no DB).

**Contracts:** schema v2 —

```json
{
  "version": 2,
  "theme": "default",
  "accounts": [
    { "id": 1, "configDir": "~/.cc/1", "emoji": "🥇",
      "claude": { "permissionMode": "bypass" }, "codex": { "yolo": true } }
  ],
  "claude": { "permissionMode": "bypass", "binary": "claude" },
  "codex":  { "yolo": true, "binary": "codex" },
  "mcp": { "servers": { "chat": {"enabled": false}, "harvester": {"enabled": false} },
           "http": { "port": 8377 } },
  "ask": { "engine": "codex",
           "codex":  { "model": "gpt-5.6-luna", "effort": "low" },
           "claude": { "model": "claude-haiku-4-5", "effort": "low" } }
}
```

`permissionMode` enum: `bypass` | `prompted`. Defaults (no file): today's values — 3 accounts `~/.cc/1..3`, emoji `🥇🥈🥉` (id 4 → `🍀` when configured), `bypass`, `yolo:true`, theme `default`, port `8377`. `Account` struct gains `Emoji string`, `Claude *ClaudePrefs`, `Codex *CodexPrefs`; exported helpers `Config.AccountByID(id) (Account, bool)`, `Config.EffectiveClaude(id) ClaudePrefs`, `Config.EffectiveCodex(id) CodexPrefs`, `Config.EmojiFor(id) string` (unknown id → `"·"`).

**File plan:** EDIT `pfm/internal/config/config.go` (schema, defaults, helpers), `pfm/internal/config/config_test.go` (v1-compat, override fallback, unknown-field rejection, emoji default, missing-file→defaults, malformed-file→hard failure on a normal command, doctor/show/validate usable on malformed config, `config init` output round-trips through the strict loader, secrets redacted in `show`). NEW `pfm/cmd/pfm/config_command.go` — `runConfig(args)` dispatching `init|show|validate`; register in `cmd/pfm/main.go` dispatch + usage. EDIT `pfm/internal/swap/swap.go:260` (accounts from config), `pfm/internal/installer/installer.go:767` (fanout over `Config.Accounts`, not `1..4`), `pfm/internal/action/synth.go` + `pfm/internal/agentopen/real.go` + `pfm/internal/installer/assets/shim/pfm.zsh` (posture-gated bypass flags).

**Boundaries & anchors:** the parked `origin/feat/autonomy-opt-in` branch is NOT merged and its `autonomy` key is NOT adopted — `permissionMode`/`yolo` are the canonical names (they already exist; one name per concept). Its test ideas (assert bypass ABSENT when posture=prompted) are re-implemented against this schema. Statusline's account map is Task #3's; medal unification is Task #2's.

---

### Task #2 — Theme engine + per-account emoji unification

One palette struct feeds every lipgloss literal; one config-driven emoji map replaces three divergent hardcodes. **Why:** `ui/render.go:19-60` styles are literals; medal maps at `ui/render.go:637-648`, `statusline/render.go:435-446`, `naming/label.go:45-50` disagree on defaults (`"·"` vs `🥇`).

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. `config.theme` selects an embedded palette; unknown name → `default` + stderr warning.
b. Palettes: `default` (exact current literals: codex `#e879f9`, agent `#fb923c`, statsClaude `#5eead4`, statsCPU `#4ade80`, plus the rest of render.go:19-60) and `tokyo-night` (mapped from `origin/add-tokyo-night-theme`'s `tokyo-night.json`; the branch itself is then closeable by the founder).
c. All three emoji sites read `config.EmojiFor(id)`; unknown → `"·"` everywhere (the statusline's silent 🥇 default dies in Task #3).

**Data model:** none.

**Contracts:** NEW `pfm/internal/theme/theme.go` — `type Palette struct { CodexRow, ClaudeRow, AgentRow, StatsClaude, StatsCPU, StatsRAM, Accent, Muted, Warn string }`, `func Load(name string) Palette`, palettes as Go literals (no runtime JSON). `naming.ContainsMedal` derives its detection set from the configured emoji list + legacy medals (so old names still parse).

**File plan:** NEW `pfm/internal/theme/theme.go`, `theme_test.go`. EDIT `pfm/internal/ui/render.go` (styles built from `theme.Load` at model construction), `pfm/internal/statusline/render.go:435-446` (emoji via config), `pfm/internal/naming/label.go`. Salvage colors from `git show origin/add-tokyo-night-theme:blueprint/templates/themes/tokyo-night.json`; also NEW `blueprint/themes/tokyo-night.json` lands via Task #13c.

**Boundaries & anchors:** no user-authored palette files in v1 (embedded only — `blueprint/themes/` stays documentation/source-of-truth for humans); statusline account RESOLUTION is Task #3.

---

### Task #3 — Account attribution fix (the "everything is 🥇" bug)

**Why:** reproduced live — `pfm ls --plain` shows 🥇 on every row including account-2/3 chats. `~/.cc/1` is a **symlink** to `~/.claude`; `compose.accountForPath` (compose.go:1043-1065) longest-prefix-matches unresolved paths, and the statusline defaults `account := 1` (`statusline/render.go:395-396`) whenever `CLAUDE_CONFIG_DIR` is unset/unmatched.

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. Both sides of every prefix match are canonicalized once (`filepath.EvalSymlinks`) — the root list at build time (`mcpserv/backend.go:286-298` `configuredAccountRoots`, `cmd/pfm/pipeline.go:820-832` `accountRoots` — the tracer-named FRONTIER pair), not per-row.
b. Unattributable stays honest: compose keeps `Account==0` → blank badge; statusline unknown renders NO badge (never 🥇). The literal `.cc/2/.cc/3/.cc/4` switch at `statusline/render.go:406-413` dies — resolution goes through `config.Accounts` (canonicalized ConfigDirs).
c. RED test first: fixture HOME where root 1 is a symlink into a sibling dir (mirrors devbox) + transcripts under roots 1, 2, 3 — assert per-row account 1/2/3 through `compose`; second fixture for statusline `CLAUDE_CONFIG_DIR` set/unset.
d. Acceptance on devbox: `pfm ls --plain` shows 🥈 on this session's chat and mixed medals across the fleet.

**Data model:** none (`store.Transcript` correctly carries no account — attribution stays in compose).

**Contracts:** none crossing projects.

**File plan:** EDIT `pfm/internal/compose/compose.go` (accountForPath + root canonicalization), `pfm/internal/mcpserv/backend.go`, `pfm/cmd/pfm/pipeline.go`, `pfm/internal/statusline/render.go` + `runtime.go`; NEW tests in `compose` and `statusline` packages.

**Boundaries & anchors:** `internal/index/stream.go` (tracer coverage gap) gets opened and confirmed account-free during this task; no store schema change.

---

### Task #4 — LS picker: action carousel + merged new-chat row + claude color (founder-approved model)

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. **Tab/Shift-Tab** switches Chats↔Stats (replaces ←/→ at `model.go:248-250`).
b. Chat rows: ←/→ cycles the selected row's action — order `▶ open → ⟳ reload → ⚡ reboot → 🕐 1h → ✖ hide` — rendered as an emoji box at the row's right edge (selected row only); **Enter runs the highlighted action**. Cursor movement resets the carousel to `open`. Existing ctrl bindings stay as aliases (ctrl+t reload, ctrl+o reboot, ctrl+e 1h, ctrl+x hide, ctrl+r rotate, ctrl+s primary).
c. New-chat row: ONE row, `[ Claude ] Codex` halves, ←/→ toggles engine (default Claude), Enter opens the highlighted engine. Replaces the two `Kind: NewClaude`/`NewCodex` rows from `compose`.
d. Claude rows render in the stats teal (`Palette.StatsClaude`, `#5eead4` in default) — the unstyled `default:` branch at `render.go:470-477` dies. Codex/agent styling unchanged.
e. `--plain`/`--tsv` output is byte-stable except the new-chat merge (they emit both engines as today — plain mode has no carousel).

**Data model:** none.

**Contracts:** `compose` emits one `Kind: NewChat` row carrying both engines (or UI-level merge of the two existing rows — dev picks the smaller diff; if compose changes, `rotate.go:88` moves with it). UI model gains `actionIndex int` + `newChatEngine Engine` fields; `updateKey` switch is the single binding site (no key registry exists — keep it that way).

**File plan:** EDIT `pfm/internal/ui/model.go` (updateKey), `pfm/internal/ui/render.go` (row + carousel + halves), `pfm/internal/ui/types.go`, `pfm/internal/compose/compose.go` + `rotate.go` (row merge), `pfm/internal/ui/plain.go` (guard stability). Tests: pure `ui` package key-sequence tests (bubbletea update loop) — **no pty, no tmux, no Enter on a live picker**; assert outcomes as `Outcome*` values, never spawned processes.

**Boundaries & anchors:** rename/hide flows themselves unchanged (`toggleHidden` model.go:565 reused by the ✖ action); reboot uses existing `OutcomeReboot`; no new outcomes invented.

---

### Task #5 — Darwin correctness fixes

**Why:** two pinned suspects from the walk: `internal/resolve/whoami.go:417` reads `/proc/<pid>/stat` directly (no darwin branch — bypasses the `gather.ProcFS` abstraction that HAS one), and `internal/installer/launchd.go` carries no build tag while calling `launchctl` (T1/T3 disagreed whether `schedulerIsLaunchd` gates it).

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. `ProcTree.Parent` routes through the ProcFS abstraction (sysctl on darwin) — RED test with a fake ProcFS.
b. Verify every `launchd.go` exported call site in `installer.go` is behind `schedulerIsLaunchd`; if any isn't, gate it and add the regression test to the fakeRunner suite (`TestUnitTransitionsUseOnlyTheInjectedManager` pattern, installer_test.go:419).
c. `GOOS=darwin go vet ./...` joins CI's verify job (cheap cross-vet — catches darwin-only breaks the linux test run can't).

**Data model / Contracts:** none.

**File plan:** EDIT `pfm/internal/resolve/whoami.go`, possibly `pfm/internal/installer/installer.go`/`launchd.go`; EDIT `.github/workflows/verify.yml` (one vet line). Runtime darwin verification itself is Task #11's macOS job.

---

### Task #6 — Stats Limits sub-tab (a) + Fable statusline line (b)

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. `StatsSubtab` gains `StatsLimits` (types.go:24-26 enum + `renderStatsPanel` branch at render.go:243+; the modular cycle at model.go:345 auto-includes it). One row per configured Claude account: 5h %, 7d %, 7d-opus %, 7d-fable % (when the endpoint reports it), each with a live `1D 12H 05M`-style reset countdown (limit-dashboard's format); plus the Codex row from the existing `refresh_gpt` JSON-RPC path. Data via the existing `usagehook` fetch (`GET https://api.anthropic.com/api/oauth/usage`, per-account OAuth token from each `configDir`), cached ≤60s in-process — the sampler never blocks the UI thread (async refresh like `startStatsSample`).
b. **Haiku ACK fallback (pre-authorized):** when an account's token is expired/unreadable, run the account's claude binary headlessly once — `CLAUDE_CONFIG_DIR=<configDir> claude -p "ACK" --model claude-haiku-4-5 --max-turns 1` via `exec.CommandContext` (precedent: `agentopen/real.go:60`) — which refreshes the OAuth token on disk; then re-read. At most once per account per pfm process; failure renders `limits unavailable` for that account, never blocks the tab.
c. Statusline: `usagehook` struct (hook.go:36-45) gains the model-scoped weekly fields; render appends `· 7d-fable N%` beside the existing `7d-opus` segment (render.go:304-326) only when present; `modelSymbol` (render.go:115-120) gains `Fable → ✦`.
d. **In-task probe (read-only):** capture one live `/api/oauth/usage` response to learn the Fable window's exact field name; commit a SANITIZED fixture (invented values, no ids/emails) driving the parse test. If the endpoint exposes no Fable window yet, parse generically (retain unknown windows in a `map[string]Window`) and render any window whose key contains `fable` — the feature degrades to absent, never wrong.

**Data model:** none (no SQLite history — deferred).

**Contracts:** `usagehook.Usage` adds typed `SevenFable` + `Extra map[string]Window`; `stats.Snapshot` gains `Limits []AccountLimits{Account int, Emoji string, Windows []Window{Name string, UsedPct int, ResetAt time.Time}}`.

**File plan:** EDIT `pfm/internal/usagehook/hook.go`, `pfm/internal/stats/stats.go`, `pfm/internal/ui/types.go`, `model.go`, `render.go`, `pfm/internal/statusline/render.go`; NEW `pfm/internal/stats/limits.go` (fetch+cache+ACK fallback) + tests with the sanitized fixture.

**Boundaries & anchors:** porting source is `mreza0100/limit-dashboard` (windows + countdown semantics) — port the logic, never its Swift; Vertex/GPT-proxy account 4 stays out (retired); no new EGRESS beyond the two existing endpoints + the Haiku poke.

---

### Task #7 — /swap → /reload

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. Slash command installs as `/reload`: asset `swap.command.md` → `reload.command.md`, mapping at `installer.go:554-555` follows; installer migration removes a previously-installed `swap.md` on apply (idempotent).
b. CLI verb: `pfm chat reload` is canonical; `swap` stays a hidden dispatch alias at `headless_command.go:105` (one release of muscle-memory/back-compat, not in usage strings). Usage line (swap_command.go:27) renames.
c. Package `internal/swap` renames to `internal/reload` (module-local rename, one name per concept); statusline/other callers follow.
d. Docs sweep: README/INSTALL/CHANGELOG `/swap` references → `/reload` (grep-verified; the tracer flagged these as unquoted — dev re-greps).

**Data model / Contracts:** none new.

**File plan:** RENAME `pfm/internal/swap/` → `pfm/internal/reload/`; EDIT `pfm/cmd/pfm/swap_command.go` (→ `reload_command.go`), `headless_command.go`, `pfm/internal/installer/installer.go`, `files.go`, `installer_test.go`, assets; docs.

**Boundaries & anchors:** the legacy `cc-swap` zsh function in `assets/shim/pfm.zsh:182-209` is UNRELATED (tracer-confirmed trap) — do not touch. `main.go:316` (unquoted swap ref) gets opened and dispositioned during the task.

---

### Task #8 — Chat MCP: full /chat:* family behind one HTTP daemon (founder-ruled)

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. `pfm mcp serve` (no name) starts ONE process binding `127.0.0.1:<config mcp.http.port>` (loopback only — security non-negotiable; refuse any other bind) exposing BOTH servers via streamable-HTTP: `/mcp/chat` and `/mcp/harvester`. Existing per-name stdio mode (`pfm mcp chat|harvester serve`) stays for clients without HTTP MCP support (Codex, if verified so during dev).
b. Chat MCP grows from 7 tools to the family, each calling the EXACT function the CLI dispatch calls (tracer map is the wiring table): `chat_new`→runRun path, `chat_open`→openID/action.Open, `chat_read`→`runChatRead`'s functions (converge — today's `backend.read()` diverges), `chat_last`→transcript.Tail/Last, `chat_status`→headless.Inspect, `chat_inject`→inject engine (already shared), `chat_capture` (already shared), `chat_keys`→`runChatKeys`'s validation + `inject.CommandTmux` SendKey/SendLiteral (already implemented as the CLI verb `pfm chat keys` — adapt, never reimplement; the tool takes `{target, keys: [string], literal?, delay_ms?, capture?}` and MUST reject an unknown key name rather than typing it as text), `chat_name`→deliverChatName, `chat_hide`/`chat_unhide`→hide.Manager, `chat_reload`→reload.Run, `chat_find`→converge onto `findTranscript`, `chat_save`→writeRepositorySnapshot path, `chat_ls`→scanFleet (converge), `chat_whoami` (already shared), `chat_resolve`→resolveChat.
c. Convergence rule: where MCP and CLI implementations diverge today (`find`, `read`, `ls`), the CLI's function becomes the single implementation and the MCP handler thins to a parameter adapter — behavior byte-identical to the CLI (same defaults, same errors).
d. Daemon lifecycle: systemd user unit `pfm-mcp.service` (linux) / launchd plist (darwin), installed by Task #10 only when a server is enabled in config; `pfm doctor` reports daemon state + port reachability.
e. Excluded from MCP (boundary): `end`, `modal`, `group`, `watch`, `stream`, `recover`, `history`, `branch`, `load` — interactive/plumbing verbs; listed in the server's doc string so the absence is stated, not silent.
f. **Auth:** loopback binding is NOT treated as authentication. Every HTTP request requires a bearer token; unauthenticated → 401. The credential is generated at install (Task #10), stored 0600, sent by registered clients via header, and redacted by `pfm config show`. A Unix-domain socket as primary local transport was evaluated and DEFERRED: Claude Code's HTTP MCP client speaks TCP URLs — forcing a socket bridge this wave is churn; named as a hardening follow-up.
g. **Single instance:** the systemd/launchd unit is the primary guard; `pfm mcp serve` additionally refuses to start when the configured port is already served by a healthy pfm daemon (probe `/status` before bind; a friendly "already running (pid N, since T)" error, never a second instance).
h. **Capabilities/status:** an authed `GET /status` (and the same data via an internal function for stdio/doctor) returns `{pfmVersion, protocolVersion, servers: {chat: [tools], harvester: [tools]}, pid, startTime, endpoint}`. `pfm doctor` consumes it for its daemon line and flags version skew between the daemon binary and the invoking binary.

**Data model:** none.

**Contracts:** `internal/mcpserv` and `internal/harvestmcp` both gain a `NewHTTPHandler()` (Go MCP SDK streamable-HTTP handler); NEW `pfm/cmd/pfm/mcp_serve_command.go` mounts both under one `http.Server`. Tool input schemas mirror the CLI flags 1:1 (same names).

**File plan:** EDIT `pfm/internal/mcpserv/server.go`, `backend.go`, NEW `httpserv.go`; EDIT `pfm/internal/harvestmcp/service.go`; NEW `pfm/cmd/pfm/mcp_serve_command.go`; EDIT `cmd/pfm/main.go` dispatch, `cmd/pfm/doctor.go`; NEW systemd/launchd assets in `installer/assets/`. Tests: httptest against the mounted handler — unauthenticated request → 401; authenticated list-tools + `chat_whoami` + harvester `searchCache` succeed; second `serve` against a healthy instance refuses with the friendly error; `/status` reports version + tool surface; stdio mode still serves both servers — no live fleet access (jail HOME).

**Boundaries & anchors:** supersedes the earlier "MCP stays defected/OFF" ruling — founder re-ordered it ON this wave. `chat_inject` through the daemon keeps the existing inject signing rules (CHAT_SENDER_* rungs, inject-detached-cannot-sign law).

---

### Task #9 — Harvester cache: prove-or-fix + TTL regression pin

**Why:** the release-day "write-only cache" finding was measured through the stale mise-shim binary; the code walk shows a working read path (`harvest.go:160-164`) and an existing 24h TTL (`cacheTTLFromEnv`, `HARVESTER_CACHE_TTL`, volatile-kind `stale()`).

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. RED-first fixture: fetch URL X twice through a local httptest server — 2nd call makes NO network request (server hit-counter) and cache file mtime is unchanged. Run against the CURRENT build. If it passes, the bug was the stale binary — record that in the test's comment and keep the test as the regression pin. If it fails, root-cause (named suspects: `identifiers.go:1201` DOI path, `fetchLocal` harvest.go:370, meta-stamp parse in `stale()`) and fix.
b. TTL pin: with an injected clock (or a backdated stamp in the cache meta), a volatile-kind entry older than 24h re-fetches; younger serves from cache. Env override `HARVESTER_CACHE_TTL` respected; `0` documented as "never stale".
c. `pfm doctor` gains one line: cache dir path, entry count, TTL in effect.

**Data model / Contracts:** none new.

**File plan:** NEW `pfm/internal/harvest/cache_roundtrip_test.go`; EDIT `pfm/internal/harvest/cache.go` only if (a) fails; EDIT `cmd/pfm/doctor.go`.

**Boundaries & anchors:** byte-parity with the node harvester and `images.go`/`net_chrome_transport.go` internals stay with the builder; this task touches only the cache seam.

---

### Task #10 — Installer completeness (hooks, cleanup, codex agents, MCP units)

**Why:** `pfm install` wires 3 hook entries; the live box runs the pfm-owned set plus more. Missing: `pfm dream hook agent-inject` (PreToolUse, matcher `Agent|Task`), `pfm dream hook nudge` (UserPromptSubmit), explore-deny, epic-inject (Task #15), cleanupPeriodDays, `pfm codex agents`.

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. Hook set installed into every managed settings.json (ConfigDir + each `config.Accounts[].configDir` — fanout config-driven per Task #1): existing 3 + `pfm dream hook agent-inject` (PreToolUse `Agent|Task`), `pfm dream hook nudge` (UserPromptSubmit), `pfm internal explore-deny` (PreToolUse `Agent|Task`; the script's logic moves INTO pfm as a subcommand — no external script path in a hook the installer owns), `pfm internal epic-inject` (UserPromptSubmit, Task #15).
b. `"cleanupPeriodDays": 36500` written into every managed settings.json when absent; an existing value is left untouched (founder may have customized).
c. `pfm codex agents` runs as an install step (global agents md→toml mirror, `internal/codexgen/globalagents.go` already ships it).
d. MCP daemon unit + client registration (pre-authorized): when `mcp.servers.*.enabled`, install the systemd/launchd unit from Task #8, **generate the MCP bearer credential** (random 32 bytes hex, written 0600 under managedRoot, regenerated only on `--force`), and register the HTTP URLs + auth header in `~/.claude/settings.json`-adjacent `.mcp.json` / codex config (stdio entries where HTTP unsupported). Disabled → units removed on apply (idempotent); the credential file is installer-owned (ledger) and removed on uninstall.
e. Third-party/manual hooks are preserved on apply AND uninstall (existing `TestDreamHookMigrationIsMigrateOnlyAndUninstallPreservesManualHooks` extends to the new entries).
f. Uninstall removes exactly the installer-owned entries (ownership ledger `settings-hook-ownership.json` grows the new names).

**Data model / Contracts:** none new beyond Task #8's assets.

**File plan:** EDIT `pfm/internal/installer/settings.go`, `installer.go`, `settings_ownership.go`, `codex_hooks.go` (codex epic/nudge parity where applicable), `settings_wiring_test.go` (every new hook asserted, idempotency, uninstall). NEW `pfm/cmd/pfm/explore_deny_command.go` porting `.claude/scripts/explore-deny.sh` logic (stdin JSON → allow/deny verdict; fixture-tested against recorded hook payloads).

**Boundaries & anchors:** Agent-Monitor and cc-memory hooks are host-local — never installed, always preserved. The devbox `~/.claude/settings.json` line currently pointing at `explore-deny.sh` migrates to the pfm subcommand on first apply (migration mirrors the dream-hook migrate-only pattern).

---

### Task #11 — E2E install → init → update → uninstall harness (Linux container + macOS runner) [MILESTONE]

**Why:** every installer test today calls `Run()` in-process with a fakeRunner; nothing executes the real binary end-to-end, and Darwin never runs at all. Founder ordered: fresh isolated Linux + Darwin machines, everything mechanical checked, uninstall too, update too, no markdown-content greps.

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors (serial phases, one Go test file, build tag `e2e`):**
a. **Install:** exec the freshly-built REAL `pfm` binary, `HOME=$(t.TempDir())`, `pfm install --apply`. Assert mechanically: every target path from the installer-surface table exists (symlinks resolve, zsh shim sourced line present in `.zshrc`, hook entries present via JSON parse — never text grep), `zsh -n` passes on the shim, `pfm doctor` exits 0 in that HOME.
b. **Init:** `pfm init $TMP/project` (Task #12b) then assert the scaffold file set exists with placeholders intact (file EXISTENCE and JSON validity only — no markdown content assertions, founder's rule).
c. **Update:** build the PREVIOUS release tag's binary from a local clone, install it into a second fresh HOME, then run the HEAD binary's `pfm install --apply` over it; assert the resulting file set + hook entries are byte/set-identical to phase (a)'s fresh install (convergence = the update path).
d. **Uninstall:** `pfm uninstall` (ModeUninstall); assert every installer-owned path is gone AND a planted manual hook + foreign file survived.
e. Runs serially (`-p 1`, ordered subtests); each phase's failure prints the differing path list, not a diff blob.
f. CI: NEW workflow `install-verify.yml` — job `linux` runs the e2e test inside `ubuntu:24.04` container (installs zsh/tmux/git only — harvestpy provisioning explicitly skipped via the installer's existing offline/skip path, asserted as "blocked, not attempted"); job `darwin` on `macos-14` (the "brand-new isolated Darwin machine" — every GH runner is fresh) runs the same test natively. Both jobs required.
g. Local runbook: `pfm/e2e/README` one-liner `go test -tags e2e -p 1 ./e2e/...` + the docker wrapper `scripts/e2e-linux.sh` for devbox runs (never against the real HOME — the harness refuses to run when `HOME` equals the invoking user's real home).

**Data model / Contracts:** none.

**File plan:** NEW `pfm/e2e/install_e2e_test.go` (phases as ordered subtests, `//go:build e2e`), `pfm/e2e/doc.go`; NEW `scripts/e2e-linux.sh`; NEW `.github/workflows/install-verify.yml`.

**Boundaries & anchors:** devbox itself is NEVER the test host for install/uninstall (live fleet); reboots forbidden everywhere; tmux config validation runs `tmux -f <conf> -L e2e-probe-$$ start-server \; kill-server` inside the container/temp socket only. No markdown-content grep tests (founder's explicit rule — existence + parseability only).

---

### Task #12 — `pfm update` (a) + `pfm init` (b)

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. `pfm update [--to vX.Y.Z] [--repo <path>]` — **ownership-aware and transactional.** Flow: fetch tags → resolve version (default: highest **semantic-version** tag, parsed comparison, never lexical) → refuse dirty worktree → checkout → build twice → **hash-equality gate** (empty GOFLAGS — candidate-hash law) → stage replacement binaries → **atomically replace ONLY installer-owned pfm copies** (per the ownership ledger; the canonical binary is recorded as owned at install) → `pfm install --apply` → `pfm doctor` → finalize. Unowned PATH copies (e.g. a mise shim) are NEVER overwritten — update surfaces doctor's `pfm_path_resolves`/`pfm_hash_mismatch` warnings loudly with a one-line remediation hint instead. Failure after staging: roll back update-owned changes from the preserved previous binary (kept under managedRoot until finalize); where rollback is impossible, report exactly what changed and what needs manual repair — never a silent half-update. Never pushes. Signed-tag/release verification is a named hardening follow-up, not invented here.
b. `pfm init [dir]` (default cwd): copies the blueprint template set from the recorded clone — `CLAUDE.md`, `AGENTS.md`, `.claude/{settings.json, output-styles/, commands/, agents/, skills/}` — placeholders INTACT; refuses a dir with an existing `.claude/` unless `--force`; prints the handoff: "open Claude here and follow docs/SETUP.md". No git operations, no global writes.
c. Both surface in `pfm --help` and `INSTALL.md`.

**Data model:** managedRoot gains `source-repo` marker file (one line, the clone path) written at install.

**Contracts:** update never pushes/tags (publication fence — it only consumes published tags).

**File plan:** NEW `pfm/cmd/pfm/update_command.go`, `init_command.go`; EDIT `cmd/pfm/main.go`, `pfm/internal/installer/installer.go` (marker), `INSTALL.md`; tests: update against a local fixture repo (no network) — dirty worktree refuses; semver selection picks `v0.10.0` over `v0.9.0` (parsed, not lexical); an unowned PATH copy is untouched while the ledger-owned copy updates; an injected failure after staging rolls back (or reports the exact residue); doctor runs after success; init into temp dirs (refuses non-empty `.claude/` without `--force`).

**Boundaries & anchors:** `/pcm:update` (the Claude-driven interview replay) remains the semantic update for blueprint CONTENT; `pfm update` owns the machine layer (binary + wiring). INSTALL.md states the split in one line.

---

### Task #13 — Blueprint: persona/Verdict in CLAUDE.md (a), live-source content diff (b), tokyo-night theme file (c) `[CMD: /pcm]`

**Routing:** /pcm (guarded paths). **Build agents:** /pcm flow, qa.

**Key behaviors:**
a. `blueprint/CLAUDE.md` gains a `## Persona` section: respond as the install's persona (pointer to the active output style) and the mandatory one-line Verdict close. **Why it must live here:** Codex reads only CLAUDE.md/AGENTS.md — output-styles never reach it, so a Codex chat today has no Verdict rule. Wording mirrors the host-ops pattern (2 lines), passes `/quality:prompt` (no voice content in CLAUDE.md — a pointer plus the one structural mandate).
b. Section-level diff of a live source project's `CLAUDE.md` against `blueprint/CLAUDE.md`: any live section the template lacks gets added genericized (refresh.md Tier-A law: parameterize, never trim). Deltas found by the diff are listed in the task's PR-style report to the user before landing.
c. NEW `blueprint/themes/tokyo-night.json` (content salvaged from the `add-tokyo-night-theme` branch, path corrected from dead `blueprint/templates/`); `blueprint/themes/` gains a 3-line README naming the palette JSON shape and that pfm embeds palettes at build time (Task #2).

**Publication surface:** all three touch `blueprint/**` — no founder name, no machine paths, invented example values only.

**File plan:** EDIT `blueprint/CLAUDE.md`; NEW `blueprint/themes/tokyo-night.json`, `blueprint/themes/README.md`.

---

### Task #14 — Retire `{FOUNDER_NAME}`: templates address "the user" `[CMD: /pcm]`

**Why:** founder's rule — templates never address by name; no name placeholder at all. The walk found `{FOUNDER_NAME}` in ~22 blueprint files (zero literal names — the token IS the violation now).

**Routing:** /pcm. **Build agents:** /pcm flow, qa.

**Key behaviors:**
a. Every `{FOUNDER_NAME}` occurrence → "the user" (grammar-adjusted per site; officer.md's "Name the founder in full as {FOUNDER_NAME}" instruction is REWRITTEN to address "the user" role without naming — its semantics change with founder pre-approval via this spec).
b. `{USER_NOUN}` (end-users of the product) is a DIFFERENT concept and stays untouched — the sweep must not conflate them (tracer-flagged trap).
c. `docs/PLACEHOLDERS.md` + the SETUP interview drop the name question; `blueprint/scripts/build-codex.mjs` and mentor.md's two bare-word address sites (ll.116,122) follow.
d. Grep gate: zero `{FOUNDER_NAME}` matches repo-wide when done; `{USER_NOUN}` count unchanged.

**Publication surface:** blueprint-wide; the leak-check + PII rules apply as everywhere.

**File plan:** EDIT the 22-file `{FOUNDER_NAME}` set (T5's table), `blueprint/commands/mentor.md`, `docs/PLACEHOLDERS.md`, `docs/SETUP.md`, `blueprint/scripts/build-codex.mjs`.

---

### Task #15 — Epic manifest inject hook (`E_{EPIC}_*` chats)

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. NEW `pfm internal epic-inject`, wired as a UserPromptSubmit hook (Task #10 installs it). Reads the hook's stdin JSON (`transcript_path`, `cwd`).
b. Chat name resolved per-turn: `resolve.NewWhoami().Identify()` → tmux `WindowName` (tmux.go:169-185). Name matching `^E_([A-Za-z0-9][A-Za-z0-9-]*)_` captures the epic slug.
c. Manifest lookup: from hook `cwd` upward to the git root, first `docs/epics/{slug}/manifest.md` (case-sensitive, then lowercased slug as fallback). Missing → exit 0 silent.
d. **Dedupe — structured state, not transcript text:** the shared store (fleet.db) records injections keyed `(session_id, slug)`; before injecting, check the row; after injecting, insert it. NO manifest/content hashes — a manifest edit never re-injects. Rename semantics fall out of the key: rename into a different epic → that slug has no row → injects once; rename back → row exists → silent. The marker line `INJECTED EPIC {slug}/manifest.md` is still the first line of the emitted `additionalContext` (human-visible provenance) but is never consulted as state — transcript text can collide naturally and is not the source of truth.
e. **`/rename` answer (embedded for the founder):** no hook fires on a rename and none is needed — this hook re-checks the name on EVERY prompt, so a chat renamed to `E_*` mid-life gets the manifest on its next message, exactly once, via the transcript marker.
f. Fixtures (temp store + fake name, no live anything): matching epic injects once; same epic never repeats; rename to another epic injects that epic once; rename back does not duplicate; missing manifest silent; grep-verifiable absence of any hash behavior.

**Data model:** additive `migration_vN.sql` (next free N): `CREATE TABLE epic_injections (session_id TEXT NOT NULL, slug TEXT NOT NULL, injected_at TEXT NOT NULL, PRIMARY KEY (session_id, slug));` — never an edit to `schema.sql`.

**Contracts:** store gains `EpicInjected(sessionID, slug string) (bool, error)` and `RecordEpicInjection(sessionID, slug string) error`.

**File plan:** NEW `pfm/cmd/pfm/epic_inject_command.go` + tests, the migration file, store methods in `pfm/internal/store/`; EDIT `pfm/internal/installer/settings.go` (hook entry — lands with Task #10).

---

### Task #16 — Register the wave's invariant `[CMD: /pcm]`

`.claude/commands/wave/walker-invariants.md` gains: **"Account identity, emoji, theme, and permission posture come ONLY from `internal/config` — a hardcoded account count, `.cc/N` literal, medal emoji, or bypass flag outside the config package is a defect."** One entry, registered in the same wave that introduces it (refine law).

---

### Task #17 — `internal/ask` contract: types, Evidence, engine abstraction (NO runner)

The runner is next-wave; the CONTRACT lands now so provenance and the engine choice never need a retrofit. **Why:** two consumers are already ordered (harvester ask, transcript ask) plus a second engine (Claude/haiku) — a domain-specific or evidence-free v1 would force a breaking rewrite.

**Routing:** pfm. **Build agents:** dev, qa.

**Key behaviors:**
a. NEW package `pfm/internal/ask` with ONLY: types, the engine interface, engine registry (`"codex"`, `"claude"`), config resolution, and stub engines whose `Run` returns a sentinel `ErrNotImplemented`. No codex/claude invocation code this wave.
b. Engine + model + effort resolve from config `ask.*` (Task #1 schema); explicit `AskInput` fields override config.
c. The engine harness prompt is FIXED here, verbatim (next wave copies it byte-identical, never paraphrases):

```
Read the prepared content files listed below. Work ONLY from them; no network access, no other files.
{numbered file list: "N. <file path> — source: <source label>"}
TASK: {user prompt}
Rules: if a file is truncated or unusable, say so explicitly for that file instead of guessing.
After your answer, append a section titled exactly "EVIDENCE" listing one line per load-bearing claim:
  [file N] <location: line range, turn number, or chunk id> — "<short verbatim quote>"
```

**Data model:** none.

**Contracts (compile-enforced this wave):**

```go
type AskInput struct {
    ContentFiles []string   // prepared by the CALLER (harvester / transcript layer) — ask never fetches, pages, or discovers sources
    SourceLabels []string   // parallel to ContentFiles; e.g. a URL or "session 3f4e…#turns 1-14"
    Prompt       string
    Engine       string     // "" → config ask.engine
    Model        string     // "" → config ask.<engine>.model
    Effort       string     // "" → config ask.<engine>.effort
}
type SourceSpan struct { Kind string /* "lines" | "turns" | "chunk" */; Start, End int }
type Evidence struct { File string; Label string; Span SourceSpan; Quote string }
type FileStatus struct { File string; Status string /* "complete" | "clipped" | "unusable" */; Note string }
type TokenUsage struct { Input, CachedInput, Output int }
type AskResult struct {
    Answer   string
    Evidence []Evidence
    PerFile  []FileStatus
    Usage    TokenUsage
    Duration time.Duration
}
type Engine interface { Run(ctx context.Context, in AskInput) (AskResult, error) }
```

**File plan:** NEW `pfm/internal/ask/ask.go` (types, registry, config resolution, prompt constant), `pfm/internal/ask/ask_test.go` — tests: config resolution precedence; both stub engines resolvable by name; a fake transcript adapter and a fake harvester adapter each construct `Evidence` (turn-span and line-span) through the shared types with zero domain fields needed — proving the contract stays content-agnostic.

**Boundaries & anchors:** the model NEVER drives paging, harvesting, cache selection, or source discovery inside `internal/ask` — preparation layers own that mechanically and pass files + labels. Preparation layers (next wave) must preserve source metadata (URL, session/turn range, cache path) into `SourceLabels`/`Evidence` mapping.

---

## RND — harvester prompt→GPT (frr findings, 2026-08-20)

Fast research pass (frr) on codex one-shot invocation, run during refine per the founder's order. The POC (tmp/RND/POC/harvest-prompt/) runs next wave with these as input.

- **Invocation route (de-facto standard):** `codex exec --json -o <file> "<prompt>"` — single non-interactive turn, prompt in, result out, process exits. Useful flags: `--output-schema <json-schema-file>` (constrain the reply), `-m/--model`, `-c key=value`, `--skip-git-repo-check`, `-C <dir>`, `resume <id>`/`resume --last`. Community wrappers (codex-python-sdk, subprocess/PTY runners) all shell out to exec — no official REST surface exists.
- **Alternative route:** `codex app-server` — bidirectional JSON-RPC 2.0 over stdio (`thread/start`, `turn/start`, streaming deltas). Heavier integration; no corroborated prompt-size/cost limit was found (a claimed 4,000-char cap did NOT survive verification — treat limits as undocumented, measure in the POC).
- **Model (locally verified):** codex 0.148.0 on devbox defaults to **`gpt-5.6-sol`** (read from a live probe's session_meta — ground truth, not web). OpenAI's mid-2026 GPT-5.6 tiers: Sol (deep), Terra (balanced), Luna (fast/cheap); selectable per invocation via `-m`. POC should benchmark Terra/Luna for harvester summarize-style prompts — Sol is likely overkill.
- **Cost datum:** the one-word probe cost 18,117 input tokens (6,912 cached) / 5 output — codex exec front-loads a large system prompt; per-call overhead is material and the POC must measure it per model tier.
- **Open questions for the POC:** exact JSONL event schema (version drift across CLI releases), app-server request limits, whether `--ephemeral` (claimed flag) suppresses session persistence — if it does, it may complement the isolated-`CODEX_HOME` invariant below.
- **Luna effort benchmark (founder-ordered, 2026-08-20, live run on a Nature article summarize+gaps prompt):** low = 45s, 109k-in/1.5k-out; medium = 52s, 154k/1.9k; high = 84s, 219k/3.2k. All three produced the SAME five gap categories — higher effort bought precision and extra citations, not different substance. Input cost is dominated by codex's own page/source fetching and ~doubles low→high. **Ruling for the production feature: default `gpt-5.6-luna` + `model_reasoning_effort=low`; expose effort as a parameter. Division of labor (founder-confirmed): the HARVESTER fetches the content (cache + TTL apply), codex only reasons over it — content passed into the run, sandbox read-only with NO network (the benchmark's `danger-full-access` was for the experiment only; codex fetched natively there, which is why input scaled with effort).** Harvester-fed control run (luna@low, read-only sandbox, 50KB harvested md): 51s, 146k-in (104k cached)/2.1k-out vs native's 45s/109k(76k)/1.5k — NO token savings (per-round context resends dominate); the shape wins on determinism, cache (5067ms miss → 35ms hit, measured live — the task #9 symptom does NOT reproduce on the current binary), zero codex network egress, and citations grounded in the page's own references. Cache-status header line differs between miss/hit runs while the body is byte-identical — the POC's determinism check must compare past the header.
- **Multi-source stress run (founder-ordered, 10 URLs in ONE variadic `pfm harvest` call, 46s, incl. a JS-heavy clinicaltrials.gov page — all 10 returned):** luna@low over the 382KB combined stream = 66s, 194k-in (144k cached)/2.7k-out, ZERO codex auto-compaction, all 10 sources covered and every clipped source correctly flagged as truncated. **Discovery:** harvest's stdout clips large sources (~40-50KB each) while the CACHE holds the full documents (887KB for this corpus, ~480k tokens) — the printed `path:` per source points at the full file. The component must read cache files, not the stream, and be size-aware: single-pass up to a context budget, map-reduce (per-source digest → synthesis) beyond it.
- **Reusable-component requirement (founder, 2026-08-20):** the prompt-over-content runner is a content-agnostic pfm internal — inputs: N content files + prompt + model + effort; output: answer + usage metrics; isolated CODEX_HOME; no network. First consumer is harvester ask; the SECOND consumer is already ordered: running luna-min over chat transcripts that became unreachable after a compact — so nothing harvester-specific may leak into the component's interface.
- **Transcript-pager test (user-ordered, validated 2026-08-20):** luna@low, given the user's `transcript-pager.py` from a temporary live-source fixture as its ONLY tool over a 106MB 40-compact transcript, answered a question about the last-eaten segment PERFECTLY (both open items + both binary hashes correctly attributed, evidence pinned to turn+timestamp) in 6 pager calls, ~16k marginal input tokens. Wall time 407s, dominated by the pager re-parsing 106MB per invocation — the production shape rides Task #8's daemon, which parses/indexes a transcript once and serves extraction internally in ms. The pager's span/turn/negative-offset semantics are validated as-is and port verbatim to Go. **User correction on the consumption shape:** the model never drives the pager as a tool — pfm runs the MECHANICAL part itself (denoise the transcript with the pager logic: span slice, sidechain/tool-noise filtering, turn pairing), writes the prepared text, and luna reads THAT and answers in one pass — identical contract to harvester ask (mechanical fetch → model reads). ONE MCP tool ships, named `transcript_ask` (user rename) = mechanical extract (span/offset/limit params optional) + one-shot model answer; no standalone read tool. The isolated-`CODEX_HOME` invariant held empirically: three runs, zero `pfm ls` pollution. Working invocation validated verbatim: `CODEX_HOME=<iso> codex exec --json --skip-git-repo-check --sandbox danger-full-access -m gpt-5.6-luna -c model_reasoning_effort="low" -o <answer-file> "<prompt>"`. Already settled from the code walk: a headless codex run stays out of `pfm ls` only via an isolated `CODEX_HOME` (its `state_N.sqlite` never enters the scanned set) — `Listed()` has no liveness filter, so sharing the real home WOULD list it. **Empirically confirmed during refine** (accidental live probe, codex 0.148.0): `codex exec` against a shared home writes a rollout file AND a store row with `source=exec, thread_source=user, archived=0` — i.e. `Listed()==true`. The isolated-`CODEX_HOME` requirement is therefore a hard invariant of the POC, not a precaution.
