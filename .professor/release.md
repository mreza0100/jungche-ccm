# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Changed: GitHub Actions runtime — Verify, Installer, and Release now use the current Node-24/ESM generations of checkout, Go setup, artifact upload, and artifact download, removing GitHub's forced deprecated-runtime compatibility path. (cost)
- Fixed: OpenCode indexing — reads agent, model, token, and cost data from native assistant-message JSON, so both the v1.14 store without session summary columns and newer denormalized stores index successfully.
- Fixed: transactional updates — builds release tags in a temporary worktree without detaching the live source clone, preserves its branch with a final fast-forward, and proves rollback by reapplying and diagnosing the previous binary's installer state before claiming recovery.
