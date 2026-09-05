# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Engine: Limits monitoring — refreshes account quotas independently with five-second cache freshness; keeps reset countdowns and confirmation ages live while idle, labels percentages as used, cancels requests on tab exit, and prevents stale cache timestamps and delayed animation messages from freezing or rewinding monitoring.

- Engine: Codex coordination defaults — `pfm install` merges `templates/global/codex/config.toml` into each configured Codex home, preserves existing settings and unrelated TOML, and migrates the retired marked developer-instruction block into the separately installed harness appendix.

- Global: harness prompts — keep Claude's Professor replacement and a model-independent Codex appendix in paired templates and installer assets. Codex receives a separate native SessionStart developer message with bounded, best-effort history deduplication and visible warnings for unsupported history. The individually trusted account hook preserves final-project and managed instructions; install-time native RPC has bounded cancellation. No preparatory launch/config-home process remains.

- Project: runtime guidance — move the autonomy and status-summary triggers into both harness prompts and remove duplicated completion guidance from project instructions.
