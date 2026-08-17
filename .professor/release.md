# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Tier B: dreamer — legacy lane surfaces migrate atomically to map-backed memory on first hook consultation, with tracked-source and ownership checks.
- Tier B: pfm — `/clear` hides the completed Claude or Codex chat until its transcript grows; Codex completes the hide on the fresh chat's first prompt, `/exit` stays unchanged, and `/bb` retires across commands, skills, and hook wiring. (hook)
- Tier C: pfm — the picker re-scans every four seconds, agent rows use an orange identity distinct from Codex, and labeled Stats properties carry metric-specific colors.
- Tier C: pfm — `/chat:branch` creates a detached fork without changing the current pane, named chats resolve before their first prompt, and a bare fleet terminal closes with its harness.
- Tier C: pfm — Stats treats a transcript that disappears during refresh as unknown token usage instead of painting the expected race as a warning.
