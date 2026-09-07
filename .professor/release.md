# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

_(empty — v0.70.0 shipped every pending bullet)_
- Patch: tokens — the `opus-4-0` PRICING row is replaced by `opus-4-20`, and a new `scripts/check-token-pricing.mjs` gate resolves every published model id against the table. PRICING matches by first substring hit on the lowercased id, and Opus 4.0's id carries no minor digit (`claude-opus-4-<date>`), so the literal `opus-4-0` matched nothing and every Opus 4.0 transcript fell through to the 5/25 catch-all — a 3x undercount that reading the table could not reveal, only resolving it could. The gate is wired into the templates suite and reports `PRICING-UNREADABLE` (exit 2) when it cannot find or parse the table, so "every rate as intended" and "the table could not be read" never print the same.
- Patch: scripts/check-codex-markers.mjs — a tracked path that is not a regular file (a symlink to a directory) is NAMED as not-scannable and excluded, instead of failing as an unreadable regular file. It carries no marker header and never could, so it is outside the gate's domain rather than something the gate failed to read; failing on it made the verdict depend on whether the link happened to materialize — deleted in one checkout it warned and passed, present in another it failed, on identical content.
