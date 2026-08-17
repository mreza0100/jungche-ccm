# Drift — this install's local customizations

Customizations of **this repo's own self-install** that must stay local and must NOT be
generalized into `blueprint/**`. `/pcm` appends here; `/pcm:release` never consumes it.

The test: would this make sense in a stranger's repo? If yes it belongs in `release.md` and its
template twin. If it only makes sense because this repo IS the blueprint, it belongs here.

## Update history

| Date | Version | Mode | Notes |
| --- | --- | --- | --- |
| 2026-08-13 | 0.53.0 | install (self-hosted, `minimal` profile) | Source is this working tree at `f07e6c0`, not a downloaded tag. 27 framework files written. Roster: `blueprint/`, `pfm/`, `dreamer/`, `ENGINES/wave-walker/engine/`. |

## Post-install customizations

- **KEEP-LOCAL: no worktree pipeline.** `/wave:*`, `worktree.sh`, `alloc-ports.sh`, and the
  per-project planner/architect/developer/qa agents are NOT installed here. They ship as
  `blueprint/**` source. Work in this repo lands on `main` under `/dev` verification and
  a gitter commit. Adopters get the full pipeline; the framework repo does not run itself through it.

- **KEEP-LOCAL: no `/pcm:update`.** This repo is upstream. There is no newer tag to replay a
  manifest against, so shipping the command would be a route to nowhere.

- **KEEP-LOCAL: `gitter` is trimmed to COMMIT / PUSH / PULL / TAG.** The SETUP / MERGE /
  WORKTREE-CHECKPOINT / SYNC phases and their `docs/commands/git/references/` cards are absent
  because the pipeline that dispatches them is absent. The Remote Publication Boundary, the banned
  commands, and the scoped-commit discipline are kept verbatim.

- **KEEP-LOCAL: the execpolicy git lock is ENABLED.** `.codex/rules/repo-law.rules` promotes the
  git monopoly from prose to a pin (`git commit` / `git push` / `git tag` / `gh release` forbidden).
  The blueprint ships those rules commented out, and that default is correct for a private repo
  where a mistake is recoverable. This repo is public; a push cannot be unpublished. The rules file
  states the trade it buys.

- **KEEP-LOCAL: `.claude/skills/` is gitignored.** Source-fetched skills (`rr`) are cloned at
  install from their own public repos and never vendored — the `sources.json` law. Each also
  carries its upstream LICENSE naming its author, which this repo's leak gate correctly refuses.

- **KEEP-LOCAL: no markdown formatter hook.** `prettier` is absent on this host, and an
  `npx`-fetching PostToolUse hook is a silent network call in the middle of a turn.

- **KEEP-LOCAL: `blueprint` is a roster entry with mechanical gates instead of a build.**
  `dev.sh verify blueprint` runs the leak gate and the placeholder-registry gate. No other install
  has a project whose "tests" are a publication check.
