# harvest: restore the real-browser rung (Patchright + system Chrome)

Status: QUEUED · Refined: 2026-08-22 · Project: pfm (Go + pinned Python sidecar) · **Fenced wave** (code — worktree + `dev.sh iso`)

Self-contained: every decision below is SETTLED. Build it as written. If the code contradicts a fact here, the code wins — say so in the report rather than improvising a different design.

---

## Why

The harvest wall-bypass ladder lost its last rung when the Python harvester was retired (merge `114b4dd`). Today the ladder ends at `wayback`, so a Cloudflare **managed challenge** is a terminal dead end: Go's `tls-client` Chrome impersonation matches the TLS/JA3 fingerprint at the wire level but runs no JS and holds no real browser surface, which is exactly what the managed tier checks.

The retired implementation cleared that tier without solving anything interactive. It is 80 lines of Python and it is recoverable verbatim. Restore it.

---

## Rulings (already decided — do not re-litigate)

**R1 — Port the Python; do not write a Go browser driver.** `go-rod`+`stealth` and `chromedp` are rejected. The retired code's own research note (2026-08 probe) records that vanilla `playwright-stealth`/`rebrowser` bench ≈ vanilla; Patchright's edge is hiding the CDP automation handshake, which those Go paths do not do. A Go port would ship a rung that fails at the tier it exists for.

**R2 — A SECOND, separately-digested Python environment. Never add `patchright` to the conversion lock.** `pfm/internal/harvestpy/digest.go:66-84` derives the environment digest from `LockMetadata()` (`assets/uv.lock`) + `ConverterSource()` (`assets/converter.py`). Touching either invalidates the digest and forces **every host to re-provision the 5.79 GB Docling/Torch closure** (`assets/size-report.json` → `managed_environment.bytes`) to gain an opt-in rung most hosts never enable. So:
- `assets/converter.py` stays **byte-identical**. The browser worker is a new, separate script.
- New assets live under `pfm/internal/harvestpy/assets/browser/`.
- The new environment reuses the already-pinned uv + CPython from `assets/targets.json` — no new toolchain downloads, only the patchright wheel set.
- `patchright` drives **system Chrome** (`channel="chrome"`), so no ~150 MB Chromium download. Provisioning must NOT run `patchright install chromium`.

**R3 — Go owns the SSRF decision; the worker asks.** `pfm/internal/harvest/net.go` `assertFetchable` (exported in the same file as `AssertFetchable`, whose doc comment already reads "the public SSRF/scheme chokepoint for adapters that need to validate a URL before handing it to another worker") stays the single authority. Do **not** reimplement the predicate in Python and do **not** pass it as a policy blob — both rot out of sync. The worker asks Go per URL over the existing stdio channel (§ Protocol).

**R4 — Port from `fac3319`, never earlier.** `git show fac3319:harvester/src/harvester/net.py` lines 593-672. That revision contains `browser_route_guard()` + `context.route("**/*", ...)`. Earlier revisions call `assert_fetchable` **once** on the initial URL and then let Chrome follow its redirect chain unguarded — the repo's own RND walk logged this as a hand-confirmed CRITICAL SSRF: a public URL 302ing to `169.254.169.254` sails past the one-shot check, because no `route()` interceptor exists anywhere in that revision's `net.py`. (Full finding: `.professor/RND/walker-consumer-tree/results-harvest/REVIEW.md` — **untracked**, so it exists only in the main checkout, not in your worktree. The sentence above is the whole of it you need; verify it yourself against `git show 4edf9c5^:harvester/src/harvester/net.py`.) Porting the pre-fix shape reintroduces a critical vulnerability.

**R5 — Opt-in, off by default.** Gate: `HARVESTER_BROWSER=1`. Absent, unset, or any other value = rung disabled and not attempted. This matches the retired gate and the reference's `_BROWSER_ENABLED is False` default assertion.

**R6 — Rung placement: after `defuddle`, before the legal mirror pivot.** The reference order is httpx → curl_cffi → Jina → defuddle → **browser** → mirror (`dispatch.py:444-521` at `fac3319`). In `pfm/internal/harvest/harvest.go` that is: after the `defuddle.md` block, before the `// A publisher wall often leaves citation_pdf_url/citation_doi metadata` comment. Rung label: `"browser"`.

---

## Anchors

| What | Where |
|---|---|
| Reference implementation | `git show fac3319:harvester/src/harvester/net.py` L593-672 (`browser_route_guard`, `fetch_browser`, `_render`) |
| Reference call site | `git show fac3319:harvester/src/harvester/dispatch.py` L500-521 |
| Rung ladder + insertion point | `pfm/internal/harvest/harvest.go`, the `defuddle.md` block |
| Optional-interface pattern to copy | `pfm/internal/harvest/types.go` `OCRConverter` + `harvest.go`'s OCR escalation rung + `pfm/internal/harvestmcp/service.go` `pythonConverter.ConvertOCR` |
| SSRF chokepoint | `pfm/internal/harvest/net.go` — `assertFetchable` / exported `AssertFetchable` |
| Worker process + stdio protocol | `pfm/internal/harvestpy/converter.go:79-165` (`Convert`, `run`, `request`, `ensureWorkerLocked`) |
| Worker script shape + `op` dispatch | `pfm/internal/harvestpy/assets/converter.py` `main()` / `smoke()` |
| Embedded assets | `pfm/internal/harvestpy/embed.go` `//go:embed` line + accessor funcs |
| Provisioning | `pfm/internal/harvestpy/provision.go` `ProvisionOptions{Root, Cache, Platform, Offline, Download, Run, Smoke}` |
| Digest | `pfm/internal/harvestpy/digest.go` |
| Terminal challenge message | `pfm/internal/harvest/net.go:80` |
| Wiring | `pfm/internal/harvestmcp/service.go:104-162` `newHarvester` |
| Tool description text | `pfm/internal/harvestmcp/service.go` fetch-tool description, "**Wall-bypass**" line |

System Chrome is present on the dev host at `/usr/bin/google-chrome`.

---

## Protocol — the browser worker

New script `assets/browser/browser.py`, own long-lived process, JSON-lines over stdin/stdout, same discipline as `converter.py` (one request serialized at a time; a crash discards the process).

Go → worker, one line:

```json
{"op":"fetch","url":"https://…","proxy":"http://…|null","timeout_ms":45000}
```

Worker → Go, **zero or more** guard asks before the final line:

```json
{"ask":"fetchable","url":"https://…"}
```

Go → worker, one line per ask:

```json
{"allow":true}
{"allow":false,"reason":"refusing private/internal host 169.254.169.254"}
```

Worker → Go, exactly one final line:

```json
{"ok":true,"html":"…","status":403,"headless":false}
{"ok":false,"error":"patchright not installed"}
```

Go side: add `requestInteractive(ctx, body []byte, onAsk func(url string) error) ([]byte, string, error)` beside `converter.go`'s existing `request`. It loops reading lines: a line carrying `"ask"` is answered via `onAsk` (which calls `harvest.AssertFetchable`) and the loop continues; any other line is the final response. **Leave `request` and `Convert` untouched** — the conversion path must not change shape.

Worker side, in the Playwright route handler: emit the ask, block on one stdin line, `route.continue_()` on allow, `route.abort("blocked")` on deny, and log the refusal to stderr. Every request Chrome makes — the initial navigation, every redirect, every subresource, every XHR — goes through it (`context.route("**/*", …)`).

---

## Tasks (inside-out; each lands with its tests)

**T1 — the pinned browser environment.**
`assets/browser/pyproject.toml`: `requires-python = "==3.11.*"`, single dependency `patchright` at an exact pin, `[tool.uv] package = false`. Generate `assets/browser/uv.lock` with the uv version already pinned in `assets/targets.json` (`0.11.32`). Extend `embed.go`'s `//go:embed` line with the three new `assets/browser/*` files and add accessors mirroring `ProjectMetadata()` / `LockMetadata()` / `ConverterSource()`. Measure the new closure and record it in `assets/size-report.json` under a new `browser_environment` key — measured, never estimated; the file's own contract is that unknown fields stay unknown rather than inferred.

**T2 — `assets/browser/browser.py`.** Port L593-672 from `fac3319` near-verbatim: headed launch first, one headless retry on failure, `channel="chrome"`, `wait_until="domcontentloaded"` then a best-effort `networkidle` wait, `page.content()` for the HTML, `resp.status`. Replace the Python `assert_fetchable` call inside the route handler with the ask protocol (§ Protocol). Missing `patchright` import returns `{"ok":false,"error":"patchright not installed"}` — never raises. Add an `op: "smoke"` that reports patchright importability and whether a Chrome binary resolves, without launching a page.

**T3 — provisioning + doctor.** Provision the browser environment lazily — only when `HARVESTER_BROWSER=1` — reusing `provision.go`'s machinery under a distinct root suffix so the conversion environment's digest and directory are untouched. Never invoke `patchright install chromium`. Extend `pfm doctor`'s harvest section with one row: environment digest, patchright presence, Chrome path — and per root `CLAUDE.md`, that row must report its own broken state distinguishably ("browser env NOT provisioned" ≠ "provisioned but Chrome missing" ≠ "probe failed").

**T4 — the Go seam.** In `types.go`, beside `OCRConverter`:

```go
// BrowserFetcher is implemented by adapters that can render one URL in a real
// browser (the ladder's last wall-bypass rung). Optional: a plain Converter
// never escalates to it.
type BrowserFetcher interface {
	FetchBrowser(ctx context.Context, source string) (html string, status int, err error)
}
```

Implement it on `pythonConverter` in `service.go` (mirror `ConvertOCR`'s shape), driving the T2 worker through `requestInteractive` with `harvest.AssertFetchable` as the ask handler.

**T5 — the rung.** In `harvest.go` at the R6 insertion point, gated on R5 and `!isPrivateURL(source)`:
append `"browser"` to `rungs`; type-assert `h.options.Converter.(BrowserFetcher)`; on HTML back, run it through the same `usableContent` + `contentChars(converted) > lastContentChars` acceptance the `defuddle` rung uses, and return `h.storeResult(source, "html", "browser-chrome", …)`. Re-evaluate `isChallenge` on the browser body: a challenge page that is merely *longer* must not be accepted as content.

**T6 — honest failure surfaces.** Two distinct states, never collapsed (this repeats the OCR fix in `75106d6`):
- the rung **ran** and returned nothing usable;
- the rung **could not run** — disabled, environment unprovisioned, patchright absent, Chrome absent, or the launch errored.

Carry that distinction into the terminal message at `net.go:80`, which today blames datacenter IP reputation and never mentions that a real-browser rung is what is missing. When the ladder dead-ends on a challenge and the rung was unavailable, the message must say so and name the enable path. Update the `**Wall-bypass**` line in `service.go`'s fetch-tool description to include the browser rung and its opt-in gate.

---

## Tests — RED first

Every regression test below is accepted only after being **watched failing** against the unfixed tree. Quote the failure in the report.

1. **Route guard without a browser.** The reference extracted `browser_route_guard()` from `fetch_browser` precisely so it is testable with no Chrome in CI — keep that seam. Feed a fake route object whose `request.url` is `http://169.254.169.254/latest/meta-data/`; assert `abort("blocked")`, not `continue_()`. This is the R4 CRITICAL; it must be pinned on the Go side too — a redirect ask that Go denies must abort the request.
2. **Ask protocol round-trip (Go).** Fake worker emits two asks — one public, one `169.254.169.254` — then a final response. Assert `AssertFetchable` was consulted for both, the private one was answered `allow:false`, and the final response still parsed. No real process, no browser.
3. **Rung order.** With the browser rung enabled and every earlier rung failing, `result.Rungs` == `["direct","chrome-impersonation","jina","defuddle","browser","wayback"]`. Note: `legacy_dispatch_parity_test.go:97` and `legacy_rescue_graph_test.go:31,90,186` already pin rung lists — update them in the same commit or the suite fails. Merge `114b4dd` broke exactly this way; do not repeat it.
4. **Off by default.** `HARVESTER_BROWSER` unset ⇒ `"browser"` never appears in `Rungs` and the worker process is never started.
5. **Outage ≠ absence.** patchright missing ⇒ terminal error names the rung as unavailable and does **not** claim the browser tried and found nothing.
6. **Challenge not laundered.** Browser returns a longer Cloudflare challenge page ⇒ rejected, not stored, cache stays free of content artifacts (exclude `stats.jsonl`, as the three sibling cache-purity tests do).

A live-Chrome test is **optional and must be skippable**: `t.Skip` with a named reason when `HARVESTER_BROWSER` is unset or no Chrome resolves. A skipped suite is a named gap in the report, never a pass.

---

## Acceptance

- `dev.sh iso test pfm` and `dev.sh iso verify pfm` green, with fence proof in the report.
- `gofmt -l pfm/internal` clean; `go vet ./...` clean.
- `go test ./internal/harvest/... ./internal/harvestmcp/... ./internal/harvestpy/...` green.
- **The conversion environment digest is unchanged.** Prove it: `assets/converter.py` and `assets/uv.lock` are byte-identical to `main`, and `TestEmbeddedLockKeepsEveryResolvedConversionPackageAtOracleVersions` still passes. This is R2's mechanical gate.
- `./scripts/leak-check.sh --files $(git ls-files)` shows **no new** leak lines (8 pre-existing lines under `docs/dev/trains/**` are known and are not this wave's).
- Every claim in the report backed by quoted command output.

---

## Out of scope

- Turnstile / interactive CAPTCHA solving. The reference is explicit: this rung never solves anything interactive. Do not add a solver, a paid service, or a wait-and-click loop.
- Touching the conversion path — `converter.py`, `assets/uv.lock`, `Convert`, `run`, `request`.
- Any change to the OA resolver chain, the cache layout, or `stats.jsonl`.
- Proxy/residential-exit work. `ProxyURL` is threaded through and passed to the browser; procuring or configuring an exit is a separate decision.
- Publication. No push, no tag, no release.

---

## Known traps

1. **`Convert` rejects empty markdown** — `converter.go`'s `run` returns `"harvestpy worker returned empty markdown"`. The browser worker returns HTML, not markdown, and legitimately returns `ok:true` with a short body — do not route it through `Convert`.
2. **The worker mutex.** `request` holds `converter.mu` for the whole call. A browser fetch can take 45 s. Use a **separate** worker process and mutex, or a slow page blocks every document conversion.
3. **Rung-list assertions are load-bearing** — see test 3.
4. **Headed launch on a display-less host fails.** Keep the headed-first / headless-retry order and log the fallback; do not silently start headless (the research bench is why headed goes first).
5. **The host mirror build is currently UNSAFE.** `~/.local/bin/pfm` was built from uncommitted work in `.worktrees/opencode-runtime` where `SchemaVersion = 7`; `main` is at 6 and the live DB is already migrated. Rebuilding `pfm` from `main` hands the user a binary that refuses its own database. Close this wave at the gitter merge and **stop** — do not run the mirror-build step until the schema-7 work lands on `main`.
