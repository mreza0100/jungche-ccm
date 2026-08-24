# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Fixed: `pfm harvest ask` — the documented command and `pfm harvest --ask` compatibility spelling
  now harvest up to 50 URL, identifier, or local sources into full cache artifacts, keep failed
  sources visible as temporary error receipts, and run the configured Claude or Codex model/effort
  adapter end to end.
- Fixed: host Codex reconciliation — `pfm install` and `pfm uninstall` now reconcile global
  Claude-command mirrors without compiling project files, deleting only marker-owned orphans and
  preserving unmarked conflicts and foreign prompt/skill files.
- Fixed: repository publication gate — `pfm doctor` distinguishes an armed pre-push hook from an
  existing-but-unwired or broken one, and the hook prints its effective `core.hooksPath` before the
  leak gate runs.
- Changed: retired integrations — the Vertex/gcloud statusline dependency and implicit spend rows
  are removed, while the isolated test fence now selects its real build architecture.
- Fixed: Harvester/update edge isolation — a first failed harvest creates its stats directory
  without a noisy append error, and self-update validates host health outside the invoking
  checkout so a repository-only pre-push warning cannot roll back a healthy binary.
- Fixed: cross-runtime agent registry — Codex and OpenCode now compile every Claude source agent,
  including the sole Git writer gitter, while all other roles remain Git-read-only and a roster
  parity gate fails on any future omission.
- Fixed: in-place reload — bare `/reload` keeps the current engine account, Codex app-server turns
  recover their tmux seat through the fleet-bound thread identity when `$TMUX` is absent, and the
  reboot confirms `/exit` rendered and was submitted before replacing the pane; `--then` recognizes
  both Claude and Codex composers before delivering its continuation, and state probes inspect only
  the visible pane so historical scrollback cannot trigger a reload action.
- Fixed: doctor SID metadata — completed reload logs are recognized as owned satellite metadata
  instead of inflating invalid-crumb warnings after every successful reboot.

#### → For: in a maintainer checkout, run `git config core.hooksPath .githooks`, then require

`pfm doctor` to report `pre-push gate=armed core.hooksPath=.githooks` before publication.

After updating, rebuild both runtime mirrors and start a new or reloaded session so the harness
discovers the refreshed agent registry.
