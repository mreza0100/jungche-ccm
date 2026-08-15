# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

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
