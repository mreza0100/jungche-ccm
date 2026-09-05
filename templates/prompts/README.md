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
- `harness-original-v2.1.257.md` — the captured Claude Code v2.1.257 built-in system prompt (print-mode,
  dynamic sections excluded), the drift baseline. Captured via a localhost sink: point `ANTHROPIC_BASE_URL`
  at a listener that records the request body and answers a non-retryable 400 — the exact assembled prompt,
  zero tokens spent. Rendered with `jq -r '.system | map(.text) | join("\n\n=== SYSTEM BLOCK ===\n\n")'`,
  then the billing header's build stamp masked (`sed 's/cc_version=[^; ]*;/cc_version=*;/'`) — every release
  changes that stamp, so it is masked on both sides and DRIFT means the prose changed.
- `harness-original.sha256` — the baseline's pinned hash (`sha256sum` of the masked file reproduces it).
  `pfm doctor` re-captures the live CLI's prompt the same way, masks the stamp, and compares hashes: a mismatch
  means a Claude Code update changed the harness prompt — recapture, review the diff, and re-pin.

`claude.systemPrompt` values: `production` (default — the CLI's own prompt, untouched), `lean` (the CLI's
built-in minimal prompt via `CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT=1`), `professor` (inject `professor.md`).
