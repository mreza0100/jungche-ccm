# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Added: guided Professor update discovery — every `pfm ls` starts a silent detached latest-release lookup, consumes only a previous successful result, and presents a full-width animated gold banner on the next interactive picker; Claude, Codex, and OpenCode choices open the recorded Professor clone with an overview-first, approval-gated `pfm update` brief. (cost)

- Fixed: Google Drive Harvester — public file-share URLs resolve to complete download bytes before conversion, stale preview-only caches cannot substitute Drive's truncated viewer HTML, and a failed full-file download is explicit instead of falling through to a partial summary.

- Fixed: picker deactivation — Deactive terminates only the selected live chat, keeps the PFM dashboard open, removes the row immediately, and suppresses stale refresh frames until the socket is confirmed gone.

- Migration: `v0.61.1` → `v0.61.2` — update the machine binary, re-run the exact installer plan, and relaunch every already-running PFM dashboard so complete Drive harvesting, in-frame deactivation, and next-run update discovery take effect.

  #### → For: use `pfm update --to v0.61.2` from v0.60.1 or later; older installations bootstrap the checksum-verified asset, then require `pfm install`, `pfm install --yes`, `pfm doctor`, a full public Drive-file proof, and a disposable deactivation proof to succeed.
