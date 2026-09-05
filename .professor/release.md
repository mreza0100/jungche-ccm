# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Engine: Limits monitoring — refreshes account quotas independently with five-second cache freshness; keeps reset countdowns and confirmation ages live while idle, labels percentages as used, cancels requests on tab exit, and prevents stale cache timestamps and delayed animation messages from freezing or rewinding monitoring.

- Engine: Codex coordination defaults — `pfm install` merges `templates/global/codex/config.toml` into each configured Codex home, preserves existing settings and unrelated TOML, and updates a marked developer-instruction block for mailbox waits and native child-message routing (cost: adds developer instructions to Codex context).

- Global: harness prompts — keep Claude's Professor replacement and a model-independent Codex appendix in paired templates and installer assets; append after effective personal developer instructions across managed launch paths, remove the retired marked global prompt, and keep numeric wait defaults. (cost: one local config/read exchange per managed Codex launch; no model tokens)
- Project: runtime guidance — move the autonomy and status-summary triggers into both harness prompts and remove duplicated completion guidance from project instructions.
