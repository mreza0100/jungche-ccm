# prompts — the fleet's system-prompt layer

Claude replacement, Codex appendix, and the Claude drift baseline.

- `professor.md` — the Professor system prompt. `"systemPrompt": "professor"` makes every managed Claude
  launch inject it via `--system-prompt-file`, replacing the harness's built-in prose (tool schemas and
  CLAUDE.md are separate request lanes and are unaffected). Byte-identical to the embedded installer asset
  `prompts/professor-prompt.md`; a Go test enforces the pairing.
- `codex-appendix.md` — appended to effective `developer_instructions` for every model in
  Professor-managed Codex launches: `cx`, chat new/resume/fork, reload, Dream seats, and harvest ask.
  `pfm internal codex-launch` resolves existing instructions through Codex's `config/read` API,
  then passes the merged text as a final `-c developer_instructions=...` override. The model's
  base instructions stay intact. Raw `codex` invocations remain native.
  Runtime-only profile and ignore-user-config selectors use a temporary sibling config-reader
  home; the real launch keeps its original account and arguments. No model turn is used.
  These two selectors require file credential storage; keyring/auto storage fails explicitly
  because a temporary home cannot preserve the native keyring lookup identity.
  Native role/custom child instructions can replace the parent's appendix. Default/full-history
  children inherit it; the appendix asks parents to brief coordination rules into children whose
  overrides replace them. This is prompt guidance, not a tool-access restriction.
  `pfm install` stages the byte-identical embedded asset and removes the old marked Professor
  block from global config, preserving personal instructions and existing numeric wait settings.
  Edit the template and matching installer asset, then rebuild and install to deploy.
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
