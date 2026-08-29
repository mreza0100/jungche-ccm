# prompts — the fleet's system-prompt layer

Two artifacts, consumed by pfm's `claude.systemPrompt` config key and the `pfm doctor` drift check.

- `professor.md` — the Professor system prompt. `"systemPrompt": "professor"` makes every managed Claude
  launch inject it via `--system-prompt-file`, replacing the harness's built-in prose (tool schemas and
  CLAUDE.md are separate request lanes and are unaffected). Byte-identical to the embedded installer asset
  `prompts/professor-prompt.md`; a Go test enforces the pairing.
- `harness-original-v2.1.251.md` — the captured Claude Code v2.1.251 built-in system prompt (print-mode,
  dynamic sections excluded), the drift baseline. Captured via a localhost sink: point `ANTHROPIC_BASE_URL`
  at a listener that records the request body and answers a non-retryable 400 — the exact assembled prompt,
  zero tokens spent.
- `harness-original.sha256` — the baseline's pinned hash. `pfm doctor` re-captures the live CLI's prompt the
  same way and compares hashes: a mismatch means a Claude Code update changed the harness prompt — recapture,
  review the diff, and re-pin.

`claude.systemPrompt` values: `production` (default — the CLI's own prompt, untouched), `lean` (the CLI's
built-in minimal prompt via `CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT=1`), `professor` (inject `professor.md`).
