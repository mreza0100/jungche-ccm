# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Minor: skills — `architecture-design` becomes a machine-global skill shipped in-tree at `templates/global/skills/architecture-design/` instead of a per-project copy: it designs a codebase's layout for agent maintainers (one directory per unit of change, fixed file anatomy, grep-true names, façades, the reader's path — brief anchors, consumer index, work-tree anatomy, ceiling ratchet) and emits a design document, greenfield or brownfield. Project nouns are parameterized — the contracts hub reads as "the roster's wire-contract/schema hub (when it has one)" and its consumer index as the command the root `CLAUDE.md` § Architecture names, so the skill carries no repo-specific script.
- Mechanics: `pfm install` — `wireGlobalSkills` now links every top-level DIRECTORY under `templates/global/skills/` into `~/.claude/skills/{name}` as one whole-directory symlink (the idiom the global-commands registry already uses), beside the in-tree `engines/deep-rr` link. The `sources.json` registry sitting among them is a file, never linked as a skill; a skill source without a `SKILL.md` is reported `SKILL-SOURCE-MISSING {name}` and left unlinked; an absent or empty source directory is reported, never an error. (safe-auto)
  #### → For: adopters — `pfm update` rebuilds the binary and refreshes the registry symlinks; nothing to hand-apply. A project holding its own copy of a now-machine-global skill under `.claude/skills/` can delete it — the host-global link supersedes it.
- Docs: BLUEPRINT.md · docs/README.md · SETUP.md · INSTALL.md — the machine-global skill surface is documented as `templates/global/skills/` (shipped skill directories plus the `sources.json` fetch registry), with `architecture-design` listed in the skill roster, the SETUP source table, and the `~/.claude/skills/` symlink list.
