# Engine contract: one identity type parsed at the edges, descriptors as data, behaviour behind per-consumer registries — no engine name spelled outside `internal/engine/**`

Status: QUEUED · Refined: 2026-08-23 by Professor (user ruling: "pfm needs to be engine agnostic, so we can work with other engines without refactoring all of it every time — keep the translation in one module") · Project: pfm · Fenced wave · Pairs with `2026-08-21-engine-roster-symmetry.md` (build THIS first; roster symmetry lands on top and consumes `engine.All()`).

This document is written to be executed without judgment calls. Where it says "exactly", copy it. Where a decision was possible, it has been made. If you meet a situation this document does not cover, **stop and report it** (§ 9) — do not improvise.

---

## 0. Before you touch anything

### 0.1 Where you work
- Your checkout is the worktree `.worktrees/engine-contract/` on branch `train/engine-contract`. **The user creates it before you start.** If `.worktrees/engine-contract/` does not exist, stop and report `PREREQ MISSING: worktree .worktrees/engine-contract`. Do not create it yourself.
- The branch must contain BOTH of these, or the wave cannot be built safely. Check with the two commands; if either prints nothing, stop and report `PREREQ MISSING: <name>`:
  - the OpenCode engine: `ls pfm/internal/index/opencode.go`
  - the host-escape jail guard: `grep -n 'PFM_TEST_REAL_HOME' pfm/internal/paths/paths.go`
- Every path in this document is relative to that worktree root.

### 0.2 What you may never do
- **Never run `go test`, `go build`, or `go vet` on the host.** Every build and test runs inside the fence: `./.claude/scripts/dev.sh iso test pfm` from the worktree root (see § 0.3). The jail guard on this branch makes an unfenced `go test` fail with `refusing to resolve the operator's real home directory inside a test` — if you see that line, you ran outside the fence; that is your error, not a flaky test.
- **Never run `git commit`, `git push`, `git tag`, `git merge`, `git rebase`, `git stash`, `git checkout`, `git reset`.** The harness denies them; the repo law says only the gitter agent writes git. You build; a gitter session commits at the checkpoints in § 8.
- Never edit `.claude/**`, any `CLAUDE.md`, any `AGENTS.md`, `.opencode/**`, `.codex/**`.
- Never change `pfm/internal/store/store.go`'s `SchemaVersion` or add a `migration_v*.sql`. Storage is out of scope (§ 7).
- Never write an absolute path of this machine (`/home/…`) into any tracked file.

### 0.3 How you build and test
```
cd .worktrees/engine-contract            # the worktree root
./.claude/scripts/dev.sh iso test pfm    # full suite inside the container; ~10 min
./.claude/scripts/dev.sh iso verify pfm  # vet + gofmt + build inside the container
./.claude/scripts/dev.sh iso shell       # an interactive shell INSIDE the fence, for fast iteration:
                                         #   cd pfm && go test ./internal/engine/ -run TestParse -v
```
A green run ends with `all steps passed.` and prints one line `fence: container=<id> HOME=/root work=/worktree`. **Quote that line in every report.** A report without it is a report of an unfenced run.

---

## 1. Why (verified 2026-08-23 on `train/opencode-runtime`; line numbers drift — grep the quoted text, never trust the number alone)

1. **No seam exists.** Engine words appear in 29 of 37 non-test files in `cmd/pfm` and in 33 of 39 `internal/` packages. `grep -rnoE '"(cc|cx|ox|claude|codex|opencode)"' --include='*.go' --exclude='*_test.go' pfm/` lists **188 literals in 47 files**, plus the socket-prefix literals `"cc-"`, `"cx-"`, `"ox-"` in 15 more places.
2. **Identity is declared four times** (`store/models.go` `ClaudeEngine = "cc"`, `resolve/whoami.go` same, `dream/seat/runner.go` `codexEngine = "cx"`, `kill/types.go` aliases) and **normalized twice in opposite directions**: `action.NormalizeEngine` (`action/headless.go`) canonicalizes to the short code `cc/cx/ox`; `config.go`'s ask-engine resolver (`case "opencode", "ox": return "opencode"`) canonicalizes to the long name. So callers compare both: `request.Engine == "cx" || request.Engine == "codex"` (`reload/reload.go`, `ui/model.go` twice, `cmd/pfm/reload_command.go`).
3. **The `else` branch is Claude.** 23 `== "<engine>"` compares vs 4 `switch` statements; an engine that is not Codex silently gets Claude behaviour. Live today: `inject/engine.go` `engineName` → `default: return "Claude"`; `sky/palette.go` `bodyCls` → anything not Claude draws the **Codex** star (OpenCode chats render wrong now); `statusline/runtime.go` `EngineFromEnvironment` → `return "claude"`; `stats/limits.go` `if engine == "" { engine = "claude" }`; `gather/tmuxprobe.go` `isChatSocket` knows only `cc-`/`cx-`, so an `ox-` socket is "not a chat". This is the coincidence-detector shape root `CLAUDE.md` forbids: the broken state (unknown engine) renders as the healthy state (it's Claude).
4. **The abstraction has been invented four times, uncoordinated:** `ask.Engine` (interface), `inject.Engine` (struct with `claudeBinary/codexBinary/opencodeBinary` fields), `sky.engine` (two-value enum), `config.EngineCounts{Claude,Codex,Opencode int}` + `config.AskConfig{Codex, Claude EnginePrefs}` — one struct field per engine, so every new engine edits `config` and every consumer of it.
5. **Engine-specific strings live in engine-neutral code:** `spawn/spawn.go` carries Codex's `/rename` TUI texts (`"›"`, `"% used"`, `"Session renamed to"`, `"Type a name and press Enter"`, `"Thread name cannot be empty."`); `gather/agents.go` `IsClaudeCommand`/`IsCodexCommand` are re-derived by hand in `dream/seat/process_tree.go` (`filepath.Base(argv[1]) == "codex"`).
6. **What is already right — the model for everything below:** `index/opencode.go`. The engine answers "give me sessions" over its own store; a missing store is `nil, nil` (named absence), a present-but-unreadable store is an error. Keep that file's shape; generalize it.

---

## 2. The leaf package — `pfm/internal/engine` (write exactly this)

**Import rule:** `internal/engine` imports only the Go standard library. Every other package may import it. It imports none of them. (`testjail → deps` was the cycle that bit on 2026-08-22; this leaf must never grow an import.)

`pfm/internal/engine/engine.go`:

```go
// Package engine is the one place pfm knows which chat engines exist. Every
// other package asks this one; none of them spells an engine name.
package engine

import (
	"fmt"
	"sort"
	"strings"
)

// ID is an engine's short code. It is the ONE spelling: stored in fleet.db,
// printed in logs, used as the tmux socket prefix, emitted in JSON.
type ID string

const (
	Claude   ID = "cc"
	Codex    ID = "cx"
	Opencode ID = "ox"
)

// Descriptor is everything pfm knows about an engine that is DATA. Behaviour
// (how to index it, how to spawn it, how to recognise its process) lives in
// the consumer packages' registries — see the spec, § 3.
type Descriptor struct {
	ID    ID
	Name  string // "Claude Code" — the product name, for doctor and help text
	Short string // "Claude" — the one-word label pickers and the statusline show
	// Binary is the executable's base name; BinaryPathHints are substrings of
	// an executable path that also identify the engine (Claude's versioned
	// installs live under ".../claude/versions/<v>/claude").
	Binary          string
	BinaryPathHints []string
	// SocketPrefix names the tmux servers this engine's chats run on ("cc-").
	SocketPrefix string
	// SessionEnv is the variable the engine exports into a chat's shell that
	// carries the session id; "" when the engine exports none (then whoami
	// falls back to SocketPrefix). HomeEnv is the variable that relocates the
	// engine's config dir; "" when the engine has none.
	SessionEnv string
	HomeEnv    string
	// RootEnv is the PFM_* variable a test jail sets to relocate this engine's
	// session store; DefaultRoots computes the production roots from $HOME.
	RootEnv      string
	DefaultRoots func(home string) []string
	// LongName is the spelling a human types on the CLI ("claude"); Parse
	// accepts it alongside ID.
	LongName string
}

var registry = map[ID]Descriptor{}

// Register adds a descriptor. It is called from this package's init for the
// three built-in engines and from tests that prove a fourth engine needs
// nothing else. Registering an ID twice is a programming error and panics:
// it can only happen at init time, never from user input.
func Register(d Descriptor) {
	if _, dup := registry[d.ID]; dup {
		panic(fmt.Sprintf("engine %q registered twice", d.ID))
	}
	registry[d.ID] = d
}

// All returns every registered ID in stable order (sorted by ID).
func All() []ID {
	ids := make([]ID, 0, len(registry))
	for id := range registry {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// Lookup returns the descriptor for a KNOWN id. An unknown id is an error
// naming the accepted set — never a default engine.
func Lookup(id ID) (Descriptor, error) {
	d, ok := registry[id]
	if !ok {
		return Descriptor{}, fmt.Errorf("unknown engine %q (want %s)", id, accepted())
	}
	return d, nil
}

// Parse turns what a human typed, a config file holds, or a database row
// stores into an ID. It accepts the short code and the long name, case- and
// space-insensitively. Empty input is an ERROR: the caller must say which
// engine it means; defaulting to Claude is how a third engine inherited
// Claude's behaviour for a year.
func Parse(value string) (ID, error) {
	want := strings.ToLower(strings.TrimSpace(value))
	if want == "" {
		return "", fmt.Errorf("no engine given (want %s)", accepted())
	}
	for _, id := range All() {
		d := registry[id]
		if want == string(d.ID) || want == strings.ToLower(d.LongName) {
			return d.ID, nil
		}
	}
	return "", fmt.Errorf("unknown engine %q (want %s)", value, accepted())
}

// accepted renders "cc/claude, cx/codex, ox/opencode" from the registry, so
// the error text is never a hand-written list that drifts.
func accepted() string {
	parts := make([]string, 0, len(registry))
	for _, id := range All() {
		parts = append(parts, string(id)+"/"+registry[id].LongName)
	}
	return strings.Join(parts, ", ")
}

// MustLookup is for call sites holding an ID that came from Parse or a
// constant. It panics on an unknown ID because that is a programming error,
// not an input error. Never call it on a string from outside the process.
func MustLookup(id ID) Descriptor {
	d, err := Lookup(id)
	if err != nil {
		panic(err)
	}
	return d
}
```

`pfm/internal/engine/builtin.go` — the three descriptors, exactly these values:

```go
package engine

import "path/filepath"

func init() {
	Register(Descriptor{
		ID: Claude, Name: "Claude Code", Short: "Claude", LongName: "claude",
		Binary: "claude", BinaryPathHints: []string{"/claude/versions/"},
		SocketPrefix: "cc-",
		SessionEnv:   "CLAUDE_CODE_SESSION_ID",
		HomeEnv:      "CLAUDE_CONFIG_DIR",
		RootEnv:      "PFM_CLAUDE_ROOTS",
		DefaultRoots: claudeDefaultRoots, // MOVE the body of today's paths.Resolve ClaudeRoots computation here, verbatim
	})
	Register(Descriptor{
		ID: Codex, Name: "Codex", Short: "Codex", LongName: "codex",
		Binary: "codex", BinaryPathHints: nil,
		SocketPrefix: "cx-",
		SessionEnv:   "CODEX_THREAD_ID",
		HomeEnv:      "CODEX_HOME",
		RootEnv:      "PFM_CODEX_ROOT",
		DefaultRoots: func(home string) []string { return []string{filepath.Join(home, ".codex")} },
	})
	Register(Descriptor{
		ID: Opencode, Name: "OpenCode", Short: "OpenCode", LongName: "opencode",
		Binary: "opencode", BinaryPathHints: nil,
		SocketPrefix: "ox-",
		SessionEnv:   "", // OpenCode exports no session variable; whoami uses SocketPrefix
		HomeEnv:      "", // none today
		RootEnv:      "PFM_OPENCODE_ROOT",
		DefaultRoots: func(home string) []string { return []string{filepath.Join(home, ".local", "share", "opencode")} },
	})
}
```

`RootEnv` semantics for Claude: `PFM_CLAUDE_ROOTS` is a list (today's `paths.go` splits it on the list separator). Keep that: `paths` reads `RootEnv`, and if the descriptor is Claude's, splits — **no**: make it uniform. Every `RootEnv` is a `filepath.ListSeparator`-separated list; a single path is a list of one. `PFM_CODEX_ROOT=/x` keeps working unchanged.

**Tests in `pfm/internal/engine/engine_test.go` (write all four; watch each fail first where a failure is possible):**
- `TestParseAcceptsShortAndLongSpellings` — table: `"cc"`, `"Claude"`, `" codex "`, `"OX"` → the IDs.
- `TestParseRefusesEmptyAndUnknown` — `""` and `"bogus"` both error; both error strings contain `cc/claude, cx/codex, ox/opencode`.
- `TestEveryDescriptorIsComplete` — for every `All()`: `ID`, `Name`, `Short`, `LongName`, `Binary`, `SocketPrefix`, `RootEnv` non-empty; `DefaultRoots` non-nil and returns ≥1 path for `"/h"`; `SocketPrefix == string(ID)+"-"`; `LongName` is lowercase. Iterates `All()` so a fourth engine is covered without editing this test.
- `TestRegisterRefusesADuplicate` — `Register` of an existing ID panics.

---

## 3. Behaviour: interfaces declared by the CONSUMER, implementations in engine sub-packages

Go idiom: accept interfaces, return structs. `internal/engine` never knows about `store`, `tmux`, or HTTP. Each package with engine-varying behaviour declares ONE interface and ONE registry. The implementations live in `pfm/internal/engine/claude/`, `pfm/internal/engine/codex/`, `pfm/internal/engine/opencode/` — those sub-packages may import anything. **One wiring file registers everything: `pfm/cmd/pfm/engines.go`.** It is the only file outside `internal/engine/**` allowed to import an engine sub-package or mention an engine by name.

### 3.1 The registry shape (copy this pattern in every consumer; do not invent a second shape)

```go
// in package index (same for spawn, gather, stats, action, ask)
var sources = map[engine.ID]Source{}

// RegisterSource wires one engine's Source. Called only from cmd/pfm/engines.go
// and from tests. A duplicate is a programming error.
func RegisterSource(id engine.ID, s Source) {
	if _, dup := sources[id]; dup {
		panic(fmt.Sprintf("index: source for engine %q registered twice", id))
	}
	sources[id] = s
}

// SourceFor returns the engine's Source. A missing one is a NAMED error —
// engine and capability — never a fallback to another engine's Source.
func SourceFor(id engine.ID) (Source, error) {
	s, ok := sources[id]
	if !ok {
		return nil, fmt.Errorf("engine %s: no index source registered", id)
	}
	return s, nil
}

// RegisteredSources lists the engines that have a Source, for doctor.
func RegisteredSources() []engine.ID { /* sorted keys */ }
```

Error text is exactly `engine <id>: no <capability> registered` with capability ∈ `index source | launcher | process matcher | usage source | headless planner | ask runner`.

### 3.2 The six interfaces (exact signatures; each wraps code that exists today — MOVE that code, do not rewrite it)

| Package | Interface | Today's code it wraps (move verbatim into the engine sub-package) |
|---|---|---|
| `index` | `type Source interface { Sync(ctx context.Context, db *store.Store, roots []string, counters *Counters) error }` | `walkClaudeRoots` + the Claude half of `Indexer.Run`; `walkCodexRollouts` + `cxindex.go` + `codexstate.go`; `syncOpencodeMirror` + `ReadOpencodeSessions` (`index/opencode.go`) |
| `spawn` | `type Launcher interface { ComposerReady(capture string) bool; Rename(ctx context.Context, tmux Tmux, socket, target, name string, timings Timings, trace tracer) (warning string, err error) }` — `Rename` returns `("", nil)` for an engine with no rename protocol | Codex: `composerReady`, `waitForCodexComposer`, `nameCodexThread`, `renameCodexThread`, `renameLanded` and the five `codexRename*`/`codexComposer`/`codexStatus` constants; Claude: whatever `Run` does today on the `request.Engine != store.CodexEngine` path (`Named: request.Engine == store.ClaudeEngine`) |
| `gather` | `type Matcher interface { IsCommand(argv []string, binaries ...string) bool }` | `IsClaudeCommand`, `IsCodexCommand`; the OpenCode rule from `inject/engine.go` (`name == "opencode" \|\| …`) |
| `stats` | `type UsageSource interface { Fetch(ctx context.Context, account LimitAccount) (AccountLimits, error) }` | the `engine == "codex"` branch of `LimitsSampler.refresh` → `fetchCodex`; the Claude branch → `fetchClaudeCached`. OpenCode: **do not register** (no usage API exists in the tree today) |
| `action` | `type HeadlessPlanner interface { Plan(request HeadlessRequest) (HeadlessPlan, error) }` | the `case store.ClaudeEngine:` and `case store.CodexEngine:` bodies of `HeadlessRun`. OpenCode: **do not register** (`HeadlessRun` refuses it today; the registry error replaces that refusal) |
| `ask` | `ask.Engine` (exists: `Run(context.Context, AskInput) (AskResult, error)`) | the `resolved.Engine == "claude"` branch and the `processEngine{name: "codex"}` construction in `ask.go`. OpenCode: register only if `ask.go` already has a working OpenCode process path; if it does not, do not register, and note it in the report |

`gather.Matcher` is consumed by `gather`, `reap`, `kill`, `dream/seat`. Every hand-rolled re-derivation (`dream/seat/process_tree.go`: `filepath.Base(argv[1]) == "codex"`, `descendant.Command == "codex"`) is deleted and replaced by `gather.MatcherFor(id).IsCommand(argv, binaries...)`.

### 3.3 Data consumers (no interface — a map keyed by `engine.ID` + a completeness test)

For a consumer that only needs a per-engine VALUE (a colour, a star class, a label), declare `var x = map[engine.ID]T{…}` in that package and a test `TestEveryEngineHasA<X>` that iterates `engine.All()` and fails on a missing key. Consumers in this class: `sky` (star class per engine — add `clsOpencode0..3`; delete the `engine uint8` enum), `theme` (colours), `ui` (labels come from `Descriptor.Short`; colours from `theme`), `naming`, `statusline/render.go` (socket-prefix switch → `Descriptor.SocketPrefix` loop over `engine.All()`).

---

## 4. The oracle — the sweep test that IS the work list

`pfm/internal/engine/no_engine_literal_test.go`. Write it in task #1, watch it fail, and drive its output to zero through tasks #2–#5. It stays in the suite forever.

- Walks every `*.go` under the module root that is NOT `_test.go`, NOT under `internal/engine/`, and NOT `cmd/pfm/engines.go`.
- Fails on any line containing one of these string literals, built with `strconv.Quote` so the test cannot match itself: `"cc"`, `"cx"`, `"ox"`, `"claude"`, `"codex"`, `"opencode"`, `"cc-"`, `"cx-"`, `"ox-"`, `"Claude"`, `"Codex"`, `"OpenCode"`.
- Reports every hit as `path:line: <the line>` and the total count, then `t.Fatalf`.
- An unreadable directory is `t.Fatalf("sweep of %s failed — a failure to look, not a clean result: %v", root, err)`.
- Allow-list: a `map[string]string{path: reason}` of whole files, initially EMPTY. A file may be added only when every literal in it is one of: (a) a CLI flag's help text, (b) a JSON/TOML struct tag that is a shipped on-disk format (`ask.claude.model` config keys, the `engine` field of an exported JSON document), (c) an environment-variable NAME (`"CLAUDE_CONFIG_DIR"` is not matched anyway — only the bare words are). Each entry's reason must say which of (a)/(b)/(c). Expected final allow-list size: ≤ 4 files. If you are about to add a fifth, stop and report (§ 9).

**Disposition rules for every hit** (apply in order; the first that fits wins):

1. A compare (`== "codex"`, `!= "cx"`, `case "claude":`, `HasPrefix(x, "cc-")`) → the value becomes an `engine.ID` upstream (§ 5 task #2) and the compare becomes `== engine.Codex` / `strings.HasPrefix(x, d.SocketPrefix)` over `engine.All()`.
2. A label for a human (`"Claude"` in a picker row, `"Codex"` in a doctor line) → `engine.MustLookup(id).Short` or `.Name`.
3. A binary name (`"opencode"` as a default executable) → `Descriptor.Binary`.
4. A behaviour fork (`if codex { …40 lines… } else { … }`) → one of the six interfaces (§ 3.2); the branch bodies move into the engine sub-package.
5. A per-engine constant (a TUI prompt string, a JSON field name of that engine's own store) → moves into the engine sub-package with the code that uses it.
6. Otherwise → allow-list with a reason, subject to the ≤ 4 files limit.

The 47 files, by count on the base commit (your numbers will differ slightly; the test's output is authoritative): `config/config.go` 30 · `ui/model.go` 12 · `reload/reload.go` 11 · `inject/engine.go` 11 · `compose/compose.go` 11 · `ask/ask.go` 8 · `archive/archive.go` 8 · `cmd/pfm/commands.go` 8 · `store/models.go` 6 · `statusline/runtime.go` 6 · `action/headless.go` 6 · `ui/render.go` 5 · `action/synth.go` 5 · `stats/stats.go` 4 · `stats/limits.go` 4 · `deps/registry.go` 4 · then 31 files with 1–3 each.

---

## 5. Tasks — in this order, one gitter commit per task (§ 8). The suite is green at every checkpoint.

### #1 — the leaf + the oracle (≈ ½ day)
1. Create `internal/engine/{engine.go,builtin.go,engine_test.go,no_engine_literal_test.go}` per § 2 and § 4.
2. Run the sweep test inside the fence. **It must fail.** Quote its first 20 lines and its total in the report — that number is the wave's starting debt.
3. Delete the four constant sites: `store/models.go` (`ClaudeEngine/CodexEngine/OpencodeEngine`), `resolve/whoami.go` (the same three + `OpencodeSocketPrefix`), `dream/seat/runner.go` (`codexEngine`), `kill/types.go` (the aliases). Replace every use with `engine.Claude` etc. Do not leave aliases behind — `gofmt`/`go vet` in the fence will show you every site.
4. Delete `action.NormalizeEngine`; its two callers (`action/headless.go`, `cmd/pfm/run_command.go`) call `engine.Parse` and return its error verbatim. Delete the `config.go` ask-engine resolver's `case "opencode", "ox"` ladder and its `default:` error; it calls `engine.Parse` too.
5. Checkpoint: full fence green except the sweep test (expected red); `READY FOR COMMIT — #1` (§ 8).

### #2 — parse at every edge (≈ ½ day)
Every place a string enters the process becomes an `engine.ID` there, and a bad one is an error naming the accepted set. Edges, exactly:
1. **CLI flags:** `--engine` in `cmd/pfm/run_command.go`, `chat new`, `headless`, `reload` (`cmd/pfm/reload_command.go`), `chat satellite`. Unknown → the `engine.Parse` error, exit 1.
2. **Config:** `ask.engine` (`config.go`), and the per-engine prefs keys. `AskConfig{Codex, Claude EnginePrefs}` becomes `Prefs map[engine.ID]EnginePrefs`; the on-disk keys stay `ask.claude.*` / `ask.codex.*` and gain `ask.opencode.*` for free — decode by `engine.Parse` of the key segment, unknown key → config error naming the accepted set. `EngineCounts{Claude,Codex,Opencode int}` becomes `map[engine.ID]int`.
3. **Environment:** `statusline.EngineFromEnvironment(getenv) (engine.ID, error)` — loop `engine.All()`: first descriptor whose `SessionEnv` (non-empty) is set wins; then the same over `HomeEnv`; none → `(“”, ErrNoEngineInEnvironment)`. The `return "claude"` is deleted. Its one production caller, `cmd/pfm/statusline_command.go`, handles the error with exactly this, and nothing else defaults: `const statuslineHostEngine = engine.Claude // pfm statusline is launched only by Claude Code's statusline hook; an environment that names no engine is that hook's` — the default lives in one named constant with its reason, not in the parser.
4. **tmux socket names:** every `strings.HasPrefix(name, "cc-")` / `"cx-"` / `"ox-"` (`agentopen`, `gather/crumbless.go`, `gather/labels.go`, `gather/tmuxprobe.go` `isChatSocket`, `kill/self.go`, `compose/compose.go`, `resolve/resolve.go`, `resolve/whoami.go`, `statusline/render.go`) becomes `engine.FromSocket(name) (engine.ID, bool)` — add that function to the leaf: loops `All()`, matches `SocketPrefix`. `isChatSocket` becomes `_, ok := engine.FromSocket(name); return ok` — this is the fix for the `ox-` hole in § 1.3.
5. **Database rows:** `store.Killed.Engine string`, `ui/types.go` `Engine string`, `spawn.Request.Engine string`, `compose`'s row engine, `reload`'s request engine — the field type becomes `engine.ID`. At the SQL scan boundary (`store/killed.go` and wherever a row's engine column is read) the string is `engine.Parse`d; an unparseable stored value is returned as a row-level error `fleet.db row <id>: unknown engine %q` — never silently Claude. At the SQL write boundary, `string(id)`.
6. **RED-first per edge:** a test per edge that feeds `"bogus"` and asserts the error text contains `cc/claude, cx/codex, ox/opencode`; for the environment edge, a test that an environment naming no engine returns `ErrNoEngineInEnvironment` — watch it fail on the base where it returns `"claude"`.
7. Checkpoint: sweep count quoted (must be lower than #1); `READY FOR COMMIT — #2`.

### #3 — descriptors replace the data branches (≈ 1–2 days)
1. `paths.Values`: delete `ClaudeRoots []string`, `CodexRoot string`, `OpenCodeRoot string`; add `Roots map[engine.ID][]string`, built in `Resolve` by looping `engine.All()`: `os.Getenv(d.RootEnv)` split on `filepath.ListSeparator` if set, else `d.DefaultRoots(home)`. Every consumer (`index`, `config`, `installer`, `doctor`, `mcpserv`) reads `resolved.Roots[id]`. `index.NewWithCodexRoots` becomes `index.NewWithRoots(db, resolved, map[engine.ID][]string)`.
2. `inject.Engine`: delete `claudeBinary/codexBinary/opencodeBinary`; add `binaries map[engine.ID]string`; `resolve.Binaries{Claude, Codex}` becomes `map[engine.ID]string` likewise; `engineName` is deleted — callers use `engine.MustLookup(id).Short`.
3. `sky`: delete `type engine uint8` and `engClaude/engCodex`; add `clsOpencode0..3` style classes with a colour distinct from the other two (pick the theme's third accent; if the theme has none, use the Codex hue rotated — note the choice in the report); `bodyCls(id engine.ID, lvl int)` reads `var starBase = map[engine.ID]styleClass{engine.Claude: clsClaude0, engine.Codex: clsCodex0, engine.Opencode: clsOpencode0}`; `TestEveryEngineHasAStar` iterates `engine.All()`.
4. `theme`, `naming`, `ui/render.go`, `ui/model.go` (`newChatEngine` becomes `engine.ID`; the `== "claude"`/`== "codex"` toggles become a cycle over the engines present), `statusline/render.go`, `compose`, `archive`, `transcript`, `deps/registry.go` (binary names from `Descriptor.Binary`), `installer/launcher.go`, `mcpserv/backend.go` (`runtime.OpencodeBinary = "opencode"` → from the descriptor): apply the disposition rules (§ 4).
5. **Goldens:** the existing `*.ansi` goldens for Claude and Codex rows must NOT change — if one does, you changed behaviour, not wiring; find out why before regenerating. Add one golden for an OpenCode row (it now has its own star and label).
6. Checkpoint: sweep count quoted; `READY FOR COMMIT — #3`.

### #4 — behaviour registries + engine sub-packages (≈ 3 days; one sub-checkpoint per consumer, in this order)
1. `gather.Matcher` → `internal/engine/{claude,codex,opencode}/match.go`. Then delete the hand-rolled matching in `reap`, `kill`, `dream/seat/process_tree.go`. Checkpoint `#4a`.
2. `index.Source` → `internal/engine/claude/index.go` (moved `walk.go` Claude half + `claude.go`), `internal/engine/codex/index.go` (moved `cxindex.go`, `codexstate.go`, Codex half of `walk.go`), `internal/engine/opencode/index.go` (moved `opencode.go`). `Indexer.Run` becomes: for each `id` in `engine.All()`, `SourceFor(id)` — a missing source is **skipped with a counted, visible reason** (`counters.Skipped[id] = "no index source registered"`), never silently. Move the tests with the code. Checkpoint `#4b`.
3. `spawn.Launcher` → the Codex rename choreography and constants move to `internal/engine/codex/launch.go`; Claude's path to `internal/engine/claude/launch.go`; `spawn.Run` calls `LauncherFor(request.Engine)` once at the top and returns its error. Checkpoint `#4c`.
4. `stats.UsageSource` → `internal/engine/codex/usage.go` (`fetchCodex`), `internal/engine/claude/usage.go` (`fetchClaudeCached`). `LimitsSampler.refresh`: `UsageSourceFor(account.Engine)`; a missing one sets `entry.limits.Status = err.Error()` (the named error) and a warning — the same path a fetch failure takes today. The `if engine == "" { engine = "claude" }` is deleted; `LimitAccount.Engine` is `engine.ID` and is set by whoever builds the account list. Checkpoint `#4d`.
5. `action.HeadlessPlanner` and `ask.Engine` → `internal/engine/{claude,codex}/headless.go`, `…/ask.go`. `HeadlessRun`: `PlannerFor(id)`; the old `unsupported engine %q (want %q or %q)` text is deleted — the registry error replaces it. Checkpoint `#4e`.
6. `cmd/pfm/engines.go` is the single registration point: one `init()` (or one `registerEngines()` called first thing in `main`) that calls every `Register*` for every engine that has the capability. Nothing else calls a `Register*` outside tests.
7. **RED-first per registry:** `TestUnknownEngineIsANamedError` in each consumer package — asks for `engine.ID("zz")`, asserts the error text is exactly `engine zz: no <capability> registered`.

### #5 — the instrument reports itself + the fourth-engine proof (≈ ½ day)
1. `pfm doctor` gains one row, built from the registries, never hand-written:
   `doctor: engines cc=index,launcher,matcher,usage,headless,ask cx=index,launcher,matcher,usage,headless,ask ox=index,matcher`
   (capability words in that fixed order, omitted when unregistered). **Broken state:** an engine that has a descriptor but zero capabilities renders `ox=NONE (descriptor only)` and the row's status is WARN, not PASS. Expected on this branch: the line above, PASS. If `ox` has more or fewer, the report says which and why.
2. **`TestFourthEngineNeedsOnlyItsOwnPackage`** in `cmd/pfm`: registers `engine.Descriptor{ID: "zz", Name: "Zed", Short: "Zed", LongName: "zed", Binary: "zed", SocketPrefix: "zz-", RootEnv: "PFM_ZZ_ROOT", DefaultRoots: …}` plus a stub for every one of the six interfaces, through the same `Register*` calls `engines.go` uses; then asserts, with **no edit outside the test**: `engine.Parse("zed") == "zz"`; `engine.FromSocket("zz-1")` hits; `paths.Resolve()` under `PFM_ZZ_ROOT=<tmp>` yields `Roots["zz"]`; a config with `ask.engine = "zed"` and `ask.zed.model = "m"` decodes; the doctor engines row contains `zz=index,launcher,matcher,usage,headless,ask`; the picker's new-chat engine cycle includes `Zed`; `sky` does **not** panic on `zz` (it renders the fallback star class — define `starFallback` for exactly this case and make `TestEveryEngineHasAStar` still require an explicit entry for every *built-in* engine). **If this test needs any file outside the test edited to pass, the wave is not done** — go back to the consumer that needed it.
3. Wave report section "A fourth engine touches": the list of files a real fourth engine would edit. Target: `internal/engine/<name>/**` (new), `internal/engine/builtin.go` (one `Register`), `cmd/pfm/engines.go` (one block), `sky`'s star map and `theme`'s colour map (one entry each). Anything else is listed under "remaining debt" with the reason.

---

## 6. Acceptance (all of these, verbatim in the wave report)

1. `./.claude/scripts/dev.sh iso test pfm` and `iso verify pfm` both end `all steps passed.`; the `fence: container=…` line quoted.
2. The § 4 sweep test: its first failing output on the base commit (first 20 lines + total) quoted; final run green with the allow-list (≤ 4 files, each with its (a)/(b)/(c) reason) quoted.
3. `git diff main -- pfm/internal/store/store.go | grep -i schemaversion` prints nothing; `git diff main --stat -- pfm/internal/store/*.sql` prints nothing.
4. `TestFourthEngineNeedsOnlyItsOwnPackage` green; the "A fourth engine touches" list in the report.
5. Every RED-first test named in § 5 is listed with the line of its failure on the unfixed code.
6. Host proof (the USER runs these after the mirror build — write them into the report as the user's checklist, do not run them yourself): `pfm ls` goldens unchanged for Claude/Codex rows; `pfm chat new --engine bogus` prints `unknown engine "bogus" (want cc/claude, cx/codex, ox/opencode)`; `pfm doctor` prints the engines row; an OpenCode chat in `pfm sky` draws its own star.

---

## 7. Out of scope — do not do these even if they look easy

- Unifying `transcripts` / `rollouts` / `oc_sessions` or bumping `SchemaVersion` (a separate wave with a downgrade note; see the v7 lockout in `tmp/review-opencode-runtime-2026-08-22.md` C3 if that file still exists — otherwise: a binary that bumps the shared `fleet.db` schema locks every older installed `pfm` out of it).
- `internal/codexgen` and `.claude/scripts/build-opencode.mjs` — mirror compilers for one target each.
- Roster counting (0..N accounts per engine) — `2026-08-21-engine-roster-symmetry.md`.
- Adding a real fourth engine. The test proves it is cheap; it does not do it.
- UI layout changes beyond deriving labels/colours from descriptors.
- Anything in `pfm/internal/harvest*` — it has no engine concept.

---

## 8. Checkpoints and commits

You cannot commit. At each checkpoint (`#1`, `#2`, `#3`, `#4a`–`#4e`, `#5`) stop and print exactly:

```
READY FOR COMMIT — <checkpoint>
fence: <the fence proof line from the green run>
sweep: <N> literals remaining in <M> files
files:
  <one path per line, from `git status --short` — only files you changed>
```

then wait. A gitter session commits with the message `refactor(pfm): engine contract <checkpoint> — <one line>` and tells you to continue. Do not start the next task on an uncommitted checkpoint: the wave must stay rebase-able, and a 60-file diff that is all-or-nothing is not.

---

## 9. When to stop and report instead of guessing

Stop, print `BLOCKED — <reason>` with the exact command and its output, and wait, when:
- a prerequisite in § 0.1 is missing;
- a golden for a Claude or Codex row changes and you cannot name the behavioural cause in one sentence;
- the allow-list would need a fifth file;
- `TestFourthEngineNeedsOnlyItsOwnPackage` needs an edit outside the test;
- a registry consumer has no sensible "missing capability" path (you would be tempted to default to Claude);
- the fence does not start (`docker`/`compose` error) — never fall back to a host run;
- a test that this document says must fail on the base passes on the base (the document is wrong somewhere; say where).

A dead end reported is a result. A silent workaround is not.
