# `pfm chat status --summary`: the last prompt + response, summarized by `ask`

Status: QUEUED · Refined: 2026-08-21 by CCC (user idea: "use the ask feature — isolate the last prompt + response, it reads it and returns a very short summary, same design as the post-compact read") · Project: pfm · Fenced wave · Depends on: the `internal/ask` runners (this spec ships them — see #1).

## Why (verified 2026-08-21)

- `pfm chat status` (`internal/headless/headless.go`) renders `Last` = `transcript.Condensed(lastEntry)` (`:131`) — the raw tail of one record, truncated. A watcher reading many seats gets a clipped sentence, not the state of the conversation.
- `internal/ask` (`ask.go:1-3, 139-141`) is a content-agnostic contract with **stub engines** — "the registry is deliberately closed until a next-wave runner supplies process semantics". Config already carries `ask.engine` (`codex|claude`), `ask.codex.{model,effort}`, `ask.claude.{model,effort}` (`docs/dev/pfm-surface.md:116-121`). No runner exists yet, so nothing can ask anything.
- `pfm chat read` + the post-compact read protocol already define the shape: isolate spans, hand a fixed harness prompt + prepared files to a cheap model, get back a bounded summary.

## Tasks (inside-out)

### #1 — `internal/ask` runners (process semantics, both engines)

- Implement the two `Engine`s behind `ResolveEngine`: `codex` → `codex exec` headless with `--model`/effort from config, stdin = `BuildPrompt(input)`, stdout captured; `claude` → `claude -p` with `--model`, `--output-format text`, the same prompt; both with `CLAUDE_CONFIG_DIR`/`CODEX_HOME` taken from the engine roster's default account (CONFIG-OWNERSHIP), bounded by a context timeout (default 60s), binaries resolved through the deps registry when spec 5 has landed (else `exec.LookPath` with a named MISSING error). `HarnessPrompt` stays byte-stable (`ask.go:63-66`).
- `TokenUsage` parsed from each engine's usage line when present; absent usage is `nil`, never zeros.
- Tests (JAIL): fake `codex`/`claude` stubs on PATH echo a canned answer + usage; engine selection, model/effort plumbing, timeout → named error, non-zero exit → error with the stderr tail quoted, missing binary → MISSING (absence) vs crash (error) distinguished.

### #2 — span isolation: the last human turn + the assistant response that followed

- New `transcript.LastExchange(entries) (prompt, response []Entry, ok bool)`: walks back from the tail to the most recent human turn; the exchange = that turn plus every record after it (tool calls included, condensed the way `chat read` condenses them) up to the tail. If the seat is `working` (tail not assistant) the response is PARTIAL and the summary says so.
- Engine-agnostic: Codex rollouts and Claude JSONL both already normalise into `transcript.Entry` — one function, both engines (NO duplication).

### #3 — `pfm chat status --summary [name]` and the MCP `chat_status` field

- `--summary` (off by default — status must stay instant and token-free): build an `AskInput` from the exchange written to a private temp file under `tmp/` (same prepared-file path the ask contract expects), prompt suffix: "Summarize this exchange in ≤ 40 words: what was asked, what was delivered or is still in flight." Render as a second line `summary: …` (TSV: a new trailing column; JSON: `summary`).
- Cache by `(transcript path, byte offset of the last record)` in the store so a watcher polling every 30s pays once per exchange; a cache hit is labelled `summary(cached)`. Cache misses while the seat is `working` are NOT stored (the exchange is still changing).
- Failure surface (HONEST-ABSENCE): engine missing → `summary: unavailable (codex binary MISSING)`; timeout/crash → `summary: failed (…)`; never an empty field that reads like "nothing happened".
- `--summary` honours `ask.engine`; `--engine`/`--model` per-call overrides as the `ask` tools define.

## Acceptance

- `dev.sh iso test pfm` + `iso verify pfm` green with fence proof; no real engine spawned in tests.
- Host proof (user-run): `pfm chat status --summary P:BUILDER` prints a ≤40-word summary within ~5s the first time, `summary(cached)` instantly the second; `pfm chat status P:BUILDER` without the flag is byte-identical to today.
- Walker with HONEST-ABSENCE + CONFIG-OWNERSHIP armed.

## Out of scope

- Summaries in the picker rows (a follow-up once cost is measured).
- `harvest ask` (the other consumer of the same runners) — it starts working for free once #1 lands, but its own acceptance is its own spec.
