# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Fixed: Darwin installer verification — the guided-update picker E2E now keeps tmux's Unix socket root under a short isolated path, eliminating macOS's path-length failure while preserving the test jail and every v0.61.2 production behavior.

- Migration: `v0.61.2` → `v0.61.3` — replace the machine binary with the corrective tagged build and re-run the exact installer plan; no adopter blueprint or runtime configuration migration is required.

  #### → For: use `pfm update --to v0.61.3`, then require `pfm install`, `pfm install --yes`, `pfm doctor`, and the tagged Release, Installer, and Verify workflows to succeed before treating the release as accepted.
