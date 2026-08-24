# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Fixed: OpenCode fleet indexing — reads the native `message`/`part` prompt schema instead of an invented `session_input` table, and `pfm doctor` now reports an unreadable configured store as unhealthy.
- Fixed: project scaffolding — `pfm init` copies adopter sources from `blueprint/`, derives both runtime instruction files from the blueprint contract, and `--force` replaces stale Professor-owned scaffold paths.
- Fixed: installer migration and preview — recorded-clone `/bb` fossils retire safely; dry-run enumerates exact Codex command, global-agent, MCP, and update-metadata paths, and apply revalidates the same full plan before any host mutation.
- Fixed: release update grammar — semantic `##` headings are authoritative, tier labels are extensible, and breaking/migration sections are replayed at their actual heading depth.
- Docs: v0.61.0 disclosure — records the shipped `pfm init` command, idle picker backoff, and OpenCode host-global command reconciliation omitted from the published notes.
- Fixed: release verification — carries the post-tag test-isolation repairs absent from the v0.61.0 artifact, whose mandatory Verify run failed after publication.
