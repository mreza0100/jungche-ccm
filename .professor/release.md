# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Engine: harness drift checks — compare Sonnet and Opus against separately reviewed baselines, keep model names and versions informational, and warn on instruction changes. Metadata normalization is limited to complete identity lines in their owning sections; failed captures and invalid baselines remain distinct coverage warnings.
