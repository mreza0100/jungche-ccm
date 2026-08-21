# Limits tab: live Codex fetch, Fable from `limits[]`, readable bars

Status: QUEUED · Refined: 2026-08-21 by CCC (user-ordered; anchors verified against pfm and the reference) · Project: pfm · Fenced wave · Depends on: `2026-08-21-picker-kill-rename.md` landing first only if both touch `internal/ui/render.go` in the same lines — otherwise independent.

**Reference implementation (the user's ruling — follow its way of fetching):** `mreza0100/limit-dashboard`, cloned at `~/work/limit-dashboard` (Swift). Anchors below are in that clone; the builder reads them, never guesses.

## Why (verified 2026-08-21)

- **Codex limit is wrong.** `internal/stats/limits.go:273 readCodexLimits` reads a cache file `statusline.GPTCachePath` → `/tmp/cc-gpt-usage-{uid}.json`; on the host that file is dated two weeks back, so the pane shows `codex-7d 78% (now)` — a stale number with a past reset. The reference fetches live.
- **Fable window never appears.** `internal/usagehook/hook.go:47-51` decodes `five_hour`, `seven_day`, `seven_day_opus`, `seven_day_fable` + an `Extra` map. The live payload carries Fable in a `limits` array, not a `seven_day_fable` key (reference: `Sources/LimitDashboard/CredentialStore.swift:1275-1315 fableUsageWindow`). pfm cannot see it by construction.
- **Ghost window `unknown[nimbus_quill]`.** Top-level keys pfm does not know are rendered as windows. The reference reads only `five_hour`, `seven_day`, and `limits[]` (`ProviderAPI.swift:176-200`).
- **Unreadable.** One line per account, windows joined by ` · `, no bars (`internal/ui/render.go:363-400`).

## Tasks (inside-out)

### #1 — Codex: live usage fetch, reference-identical

- Reference: `ProviderAPI.swift:70-78 fetchCodex` — `GET https://chatgpt.com/backend-api/wham/usage`, `Authorization: Bearer {tokens.access_token}`, header `ChatGPT-Account-ID: {tokens.account_id}`, 20s request timeout, no cache. Credential: `~/.codex/auth.json` → `tokens.access_token`, `tokens.account_id` (`CredentialStore.swift:762-790 loadCodex`); empty token or account id = "Codex session incomplete" (a named error, not absence); missing file = "no local Codex sign-in".
- Decode: `ProviderAPI.swift:4-30` — `rate_limit.primary_window` / `secondary_window`, each `{used_percent, reset_at (epoch seconds), limit_window_seconds}`; `plan_type`. Window NAME comes from `limit_window_seconds` (`ProviderAPI.swift:282-296 codexWindowTitle`: 18000 → `5h`, 604800 → `7d`, else whole days/hours/minutes) — never from its position. Missing secondary is normal, not an error.
- pfm: new `internal/stats/codex.go` (or inside `limits.go`) replaces `readCodexLimits` + `codexUsageCache`; the `CodexCachePath` field and `GPTCachePath` wiring in `cmd/pfm/commands.go:239` go away from the Limits sampler (grep for other consumers of `GPTCachePath` — the statusline's own use stays unless it is dead; remove dead code end to end). HTTP 401/403 → `Status: "Codex credential rejected (HTTP 401)"`; network error → `Status: "Codex fetch failed: {err}"`; decode error → `Status: "Codex payload unreadable"`. An error row is never an empty row (HONEST-ABSENCE).
- Tests: fixture JSON from the reference's own test fixtures (`Tests/` in the clone — reuse the Codex body there); table test over window naming by seconds; RED-first test that a stale cache file is no longer consulted (watch it fail on the current code).

### #2 — Claude: Fable from `limits[]`, unknown keys dropped

- Reference: `CredentialStore.swift:1275-1315` — from `limits[]` pick entries with `kind == "weekly_scoped"` and `scope.model.display_name` equal to `Fable` (case-insensitive, trimmed); prefer `is_active == true`; `percent` is the value; `resets_at` ISO-8601; a reset in the past drops the window.
- pfm: extend `usagehook.Usage` with `Limits []ScopedLimit \`json:"limits"\`` (fields: `kind`, `scope.model.display_name`, `percent`, `resets_at`, `is_active`); `NamedWindows()` yields `5h`, `7d`, `7d-fable` (from `limits[]`), in that order. Delete `SevenOpus`/`SevenFable` top-level fields if the live payload no longer carries them (verify with one real fetch, record the key set in the test fixture), and drop the `Extra`/`unknown[...]` rendering entirely — an unrecognised key is logged once at debug level, never rendered. The statusline (`internal/statusline/render.go:333-334`) consumes the same `NamedWindows()` — ONE mapping, no second parser.
- Tests: RED-first — a payload with a `limits[]` Fable entry and a `seven_day_nimbus_quill` key renders exactly `5h, 7d, 7d-fable`; the statusline test flips with it.

### #3 — Limits pane: bars, one row per window

- `internal/ui/render.go:363-400` — the user's word is **fancy**; this is the showpiece tab. Per account a card: a bold header row (`🥇 account 1 · Max 20x · provider confirmed 12s ago`) with a thin rule under it, then one row per window:
  `  7d-fable   ▕████████▓▒░░░░░░░░░░▏  52%   ↻ 6d 20h`
  - Bar: sub-cell precision using the eighth-block glyphs (`▏▎▍▌▋▊▉█`) for the partial cell, so 52.4% and 55% look different; width scales to `innerWidth` (min 12, max 40 cells).
  - Colour is a gradient by usage, not a step: theme green → amber → red mapped across 0–100% (lipgloss blend over the existing theme tokens; the theme package owns the three anchor colours — no literal hex in `ui`). ≥95% pulses the percentage in the error style; 100% renders `FULL` instead of the bar's tail.
  - Reset: `↻ 6d 20h` countdown, dim; under 1h it switches to `↻ 14m` in the warning style; a past reset renders `↻ refreshing…` never `now`.
  - Window name column fixed width, right-padded; percentage right-aligned 4 cells.
  - Totals row per provider when it has >1 account (`Σ claude` with the max of each window across accounts) — one glance tells which account to switch to.
  - Error/status rows render as their own dim row under the account header, verbatim (`⚠ Codex credential rejected (HTTP 401)`). Skip notes (`skipped .cc/N: …`) move to ONE dim footer line after all cards — honest, not mistaken for an account.
  - Narrow terminals (innerWidth < 60): drop the reset column first, then shrink the bar to its minimum — never wrap a row.
- Golden files: add `ui_limits_80.ansi` / `ui_limits_120.ansi` captured from a fixture sampler (not from live HTTP); existing goldens untouched unless the header changed.

## Acceptance

- `.claude/scripts/dev.sh iso test pfm` green with fence proof; `iso verify pfm` green. HTTP never runs inside tests (fixtures + a `httptest.Server` for the transport).
- A host run of the Limits pane (user-verified after the host mirror build) shows Codex `5h`/`7d` from the live endpoint, Fable on every Claude account, no `unknown[...]`, bars present.
- `grep -rn 'cc-gpt-usage' pfm` — zero hits in the Limits path.
- Walker over the merge candidate with HONEST-ABSENCE + CONFIG-OWNERSHIP in scope.

## Out of scope

- Fetch cadence/caching policy beyond the reference's (fresh per sample; the existing sampler TTL in `limits.go:86-97` stays).
- The statusline's own Codex display — unless it reads the same stale cache, in which case it is a NAMED follow-up in the report, not fixed here.
