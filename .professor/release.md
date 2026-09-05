# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Engine: Limits monitoring — refreshes account quotas independently with five-second cache freshness; keeps reset countdowns and confirmation ages live while idle, labels percentages as used, cancels requests on tab exit, and prevents stale cache timestamps and delayed animation messages from freezing or rewinding monitoring.
