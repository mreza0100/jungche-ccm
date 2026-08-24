# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Fixed: `pfm` Darwin and config diagnostics — Apple's multiline `lsof -v` revision is parsed as
  the installed version, while `config show` and `doctor` distinguish a v1 input file from the
  effective v2 schema.

- Fixed: `pfm update` — the selected release tag is stamped into both reproducible candidates,
  `--skip-harvest` reaches install and doctor, linked-worktree sources remain enumerable inside the
  isolated fence, and the target candidate—not the old running updater—performs post-build checks.

  #### → For: do not use the v0.60.0 binary's `pfm update` for this one-time repair; bootstrap the
  verified v0.60.1 binary using `INSTALL.md`, run `pfm install --yes` (add `--skip-harvest` if that
  runtime is intentionally unmanaged), then require `pfm version` and `pfm doctor` to pass. Later
  releases can use `pfm update` normally.

- Fixed: `pfm doctor` Harvester cutover — legacy standalone, foreign, malformed, and retired-auth
  Claude/Codex registrations are warnings; only exact header-free PFM loopback routes count as
  complete, and registration paths or credentials are never printed.

- Fixed: repository verification — Docker Desktop creates nested read-only fence mountpoints before
  container start, linked worktrees use a read-only mounted Git common directory, and the
  self-hosted manifest's version, three-project roster, tracked surface, and hashes are mechanically
  gated in CI.
