# prompts — the fleet's system-prompt layer

Claude replacement, Codex appendix, and the Claude drift baseline.

- `professor.md` — the Professor system prompt. `"systemPrompt": "professor"` makes every managed Claude
  launch inject it via `--system-prompt-file`, replacing the harness's built-in prose (tool schemas and
  CLAUDE.md are separate request lanes and are unaffected). Byte-identical to the embedded installer asset
  `prompts/professor-prompt.md`; a Go test enforces the pairing.
- `codex-appendix.md` — a model-independent developer message from Codex's native SessionStart
  `additionalContext` hook. `pfm install` registers and individually trusts the owned handler in
  each configured account; it applies to native launches as well as Professor launches. Personal
  hooks and native model/project/managed instructions retain their usual behavior. No preparatory
  config reader, copied credential home, or `developer_instructions` CLI override is used.
  The installer stages the byte-identical embedded asset and removes the retired marked Professor
  block from global config while preserving personal instructions and numeric wait settings.
  Install fails visibly if native hook discovery is unavailable or managed policy excludes the hook.
  A later explicit hook disable or `--ignore-user-config` can bypass this account-level installation.
  The hook runs on startup, resume, clear and compact. A bounded reverse scan of local history
  suppresses an exact retained developer copy, respecting replacement-history checkpoints.
  Null/remote transcripts, ancestor-only fork history, rollback, legacy compaction and history
  beyond the 16 MiB/32,768-record budget fall back to injection with a visible warning that
  duplication could not be ruled out. An updated appendix can coexist with an older version in
  an existing conversation; a new session starts clean. Native persistence errors can expose stale
  transcript data, so this is best-effort deduplication, not a guarantee across every lifecycle.
  Native subagent starts do not rerun this SessionStart hook. Full-history children inherit context;
  fresh/custom children need the coordination briefing specified in the appendix. These instructions
  guide tool selection; they do not remove chat MCP access. Edit the template and matching installer
  asset, then rebuild and install to deploy. Claude uses replacement; Codex uses this separate appendix.
- `harness-original-v2.1.257.md` and `harness-opus-v2.1.261.md` are reviewed Sonnet and Opus
  built-in prompt baselines, captured in print mode with dynamic sections excluded. Each has a
  `.sha256` pin and `.model` provenance file under its `harness-original` or `harness-opus` stem;
  the template and embedded installer assets stay byte-identical.
- `pfm doctor` checks both stable aliases, `sonnet` and `opus`, against their respective baselines.
  It records the requested alias, resolved model ID, CLI version, baseline filename, and original
  model ID. Model names and versions are informational: changing those alone never reports drift.
  A changed prompt behind an alias still requires review, even if the resolved model name also changed.
- Normalization masks `cc_version` only in the leading billing system block and removes only complete known
  model-identity and month/year knowledge-cutoff lines inside `# Environment`. Instructions appended
  to those lines, similar text elsewhere, fenced examples, and model-specific behavioral sections remain checked.
  Recognizing a text pattern alone is insufficient reason to discard it.
- `DRIFT` means normalized instruction text changed: review the upstream additions, deletions, or
  rewording before re-pinning. Failed capture and missing or inconsistent baseline files report
  separate coverage warnings; they never count as drift or a match. Failure of one model's check
  does not suppress the other. These checks do not validate the active chat, Fable, Codex, or the
  Professor replacement.

Captures use a localhost sink with dummy credentials that rejects the API request with HTTP 400;
no model inference occurs. Render system blocks with
`jq -r '.system | map(.text) | join("\n\n=== SYSTEM BLOCK ===\n\n")'` before normalization.
Re-pinning requires human review of instruction differences, followed by updating the prompt,
SHA256, and model provenance together. Never automatically accept a newly captured prompt.

`claude.systemPrompt` values: `production` (default — the CLI's own prompt, untouched), `lean` (the CLI's
built-in minimal prompt via `CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT=1`), `professor` (inject `professor.md`).
