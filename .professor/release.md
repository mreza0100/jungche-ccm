# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Tier A: `pfm install` and its embedded assets — the self-contained binary now owns the
  host command cards, helpers, Codex BB skill, launcher shim, settings rewrites, systemd links,
  retired-file sweep, backups, and uninstall. Dry-run is the default; a reachable user bus
  refuses before any write with rc 97; offline unit enablement links converge without touching the
  user bus; an immediate second apply is a measured zero-change run.
  SQLite is the sole hide store, so `pfm legacy` and its carrier are removed. The source bundle
  closes at one shim plus three units, with WAL sidecars ignored. (cost: the host installer path
  changed and the old hide carrier is retired)

  #### → For: build the new `pfm` binary, review `pfm install --dry-run`, then run

  `pfm install --apply` once.

- Tier A: `pfm statusline`, `pfm usage-hook`, and embedded installer assets — native Go rendering
  replaces the high-frequency statusline shell, both local segments, and the Vertex/Codex
  Python refreshers while preserving badges, gauges, cache TTLs, detached refresh, and the
  dormant fourth-account view; the one-line sky snapshot adds live fleet counts. The native
  fail-open usage hook carries the existing warn/recover/quiet contract. The installer rewires
  every existing canonical account settings file and sweeps the six retired statusline files.
  (cost: statusline and UserPromptSubmit command paths changed)

  #### → For: build and install `pfm`, then run

  `pfm install --apply` once to rewire settings and retire the old shell/Python copies.

- Tier A: `pfm` and its embedded installer assets — complete the Professor-Fleet-Manager cutover: the
  parity oracle/checker is retired; bare `pfm` opens the picker; per-chat work lives under
  `pfm chat` with named creation, immutable socket identities, target resolution, group/BB
  wiring, and inline name convergence; `ls --hidden` exposes the hide ledger; the old
  per-chat command names are removed except for one hidden compatibility alias. Chat command
  cards call the binary directly, while executable `chat.sh` is now a two-line compatibility
  delegate. Hidden Go wiring owns primary-account state and agent opening; `pfm chat swap` and
  `pfm chat recover` replace their shell engines, and the installer removes all retired links.
  Inject restores the full target ladder, signed sender ancestry, safe busy-composer queueing,
  detached `--then` delivery, long `--file` transport and explicit refusal exit codes across both
  engines. Codex rename events drive the path unit,
  Claude name writes converge in-process, and the timer is a 15-minute drift fallback. The
  installer rewires both UserPromptSubmit hooks to fail-open `pfm chat` verbs. (cost: CLI,
  binary, unit, hook, and chat-command paths changed)

  #### → For: build and install the new `pfm` binary first, then run

  `pfm install --apply` once to rewire hooks and units.

- Tier C: `templates/scripts/build-codex.mjs` — the compiler now carries a **never-register set**
  and skips `gitter`. Before this it emitted `.codex/agents/gitter.toml` from `.claude/agents/gitter.md`
  for every adopter, registering the git monopolist as a spawnable Codex role — the exact thing
  `templates/codex/README.md` and `BLUEPRINT.md` both say must not exist. The build step was
  breaking the architecture's own rule. The skip is announced in `notes`, never silent: a quietly
  absent output is indistinguishable from a compiler that failed to find the file.

  #### → For: re-run `node .claude/scripts/build-codex.mjs generate` and delete any existing

  `.codex/agents/gitter.toml` — `generate` will not remove it for you, because it no longer knows
  the file is its own.

- Tier A: `templates/commands/pcm.md` — § Step 4's gate paragraph claimed "the Stop hook clears
  this session's markers at turn end". `pcm-guard.sh` and `guard-stamp.sh` both say the opposite in
  their own headers: markers SURVIVE turn ends and die by TTL alone (1500s sliding freshness plus a
  1h reap), so a session stamps once, not once per turn. The command was teaching a stamping cadence
  the hooks do not implement — an agent following it would re-stamp every turn and read a denial as
  a hook bug. Corrected to match the scripts, and the deny message's sandboxed-stamp caveat is now
  named where the operator reads it.

- Tier C: `templates/commands/pcm/references/audit-scopes.md` — the `structure` scope's
  Codex-mirror row and the `agents`/`commands` file globs assumed a multi-project roster with child
  `.claude/agents/` dirs. Reworded so a roster of one reads correctly (`PLACEHOLDERS.md`
  § Single-project collapse says a template must).

- Tier A: `pfm` and installer QA — reject stale pre-rename `pfm.dev` binaries; cover bare-picker
  attach, all public exit codes, BB/group fail-open hooks, inline Claude naming, and Codex
  `session_index.jsonl` window convergence on isolated `probe-*` tmux servers; installer fixtures
  sever and prove the user systemd bus unreachable before their first installer call.

- Tier A: fleet installation source — the Go installer and `internal/installer/assets/` are the
  single source of every staged host file. The duplicate host template bundle is removed; a fresh
  box gets the `pfm` binary and runs `pfm install --apply`.
