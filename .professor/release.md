# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Changed: GitHub Actions runtime — Verify, Installer, and Release now use the current Node-24/ESM generations of checkout, Go setup, artifact upload, and artifact download, removing GitHub's forced deprecated-runtime compatibility path. (cost)
