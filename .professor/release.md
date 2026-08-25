# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Changed: GitHub Actions runtime — Verify, Installer, and Release now use the current Node-24/ESM generations of checkout, Go setup, artifact upload, and artifact download, removing GitHub's forced deprecated-runtime compatibility path. (cost)
- Fixed: OpenCode indexing — reads agent, model, token, and cost data from native assistant-message JSON, so both the v1.14 store without session summary columns and newer denormalized stores index successfully.
- Fixed: transactional updates — builds release tags without detaching the live clone, advances its attached branch before installation so staged assets come from the selected release, refuses source downgrades, and restores the previous source revision before reapplying and diagnosing the old installer during rollback.
- Fixed: cross-platform update verification — preserves the installer-recorded spelling of aliased source paths across candidate and rollback installs, and waits for launched-pane process evidence to converge before declaring the live-session assertion failed.
- Fixed: OpenCode concurrency verification — gives the live-writer fixture a bounded SQLite busy timeout, so ordinary WAL recovery contention retries while a lock persisting beyond five seconds still fails the mandatory suite.

- Migration: `v0.61.3` → `v0.61.5` — replace the machine binary, re-run the exact installer plan after the old updater returns, and relaunch running dashboards; no adopter blueprint or configuration-schema migration is required.

  #### → For: use `pfm update --to v0.61.5`, then run `pfm install`, `pfm install --yes`, `pfm doctor`, and `pfm ls --tsv`; require the tagged Release, Installer, and Verify workflows to succeed before accepting the release.
