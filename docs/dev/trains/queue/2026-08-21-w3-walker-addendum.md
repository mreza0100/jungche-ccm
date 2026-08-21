# W3 addendum — pre-existing pfm defects confirmed by the wave-1 walker (2026-08-21)

Status: QUEUED · Source: docs/dev/trains/pfm-wave-2/waves/1-pfm-e2e-verification/walk.md (walker ROUGH SEAS, R9-INV confirmed items) · Project: pfm · Fold each into the W3 spec named; RED-first where a behaviour changes.

## → picker-kill-rename.md (CONFIG-OWNERSHIP family, join task #1)
- `reload`: `rosterContains` falls back to a hardcoded 1–3 roster on an empty accounts slice — fail closed (empty → false).
- `agentopen`: synthesizes a 1/2/3 roster with re-literaled `.cc/N` paths on empty Accounts — return an error, or build defaults via `pfmconfig.DefaultAccountDir`.
- Medal emojis: `accountBadgeForID` (`pfm/internal/statusline/render.go:445-459`) and `accountMedal` duplicates — export ONE config-owned helper (`config.DefaultEmoji(id)`) and delete the copies; replace statusline's `account == 4` Codex gate with the `Engine:"codex"` discriminator `commands.go` already uses.
- `pfm/internal/ui/render.go:23,65`: header background literal `"#5f3dc4"` — add `HeaderBg` to `theme.Palette`, populate per theme, read it in `configureStyles`.
- `mcpserv/backend.go newBackend()`: `resolved.ClaudeRoots` never reassigned from `machine.ProjectRoots()` — match `loadCommandRuntime`, or delete `newBackend()`/`mcpserv.New()` if dead.
- `hide.NewFinisher` fallback trusts bare `paths.Resolve()` — route through `pfmconfig.Defaults(home, roots).ProjectRoots()` or fail loudly when both `Dependencies.Paths` and `Dependencies.ClaudeRoots` are empty. (Lands with the hide→kill rename, task #3.)

## → limits-live-fetch.md (HONEST-ABSENCE family)
- `config.go:336-346 hasValidAccountCredentials`: absent / unreadable / malformed all collapse to `"no valid credentials"` — branch the read and unmarshal errors into distinct skip reasons (`credentials unreadable: {err}`, `credentials malformed`).
- `statusline.rateLimits.UnmarshalJSON`: a per-key decode failure of a KNOWN window persists as a false zero — add a `Log` seam to `statusline.Runtime` and log the raw error per dropped key, or route the failure through a visible gap label. (Joins task #2: one parser.)
- `ui/render.go` Limits row: `source = cleanField(account.Label)` so the sanitizer covers every rendered field. (Joins task #3.)

## → wave-1 (handled in `.worktrees/pfm-wave-2` — listed here for the ledger only)
- HIGH: `pfm/cmd/pfm/doctor.go:415,436-439` out-of-home PATH skip; HIGH: `pfm/e2e/install_e2e_test.go:187-200,222-228` `refuseRealHome` tautology + `sourceRepositoryReady` fast path + README/TESTPLAN:529 wording; `doctor_harvest_test` unreadable-root + permission-denied cases; `scripts/e2e-linux.sh` `:ro` mount + image digest pin; `PFM_E2E_HARVESTPY_GATE` comment.

## Process (CCC-owned, done)
- `scripts/leak-check.sh`: widen PATTERN with a home-relative `~/` alternative — a framework change, routed via /pcm separately.
