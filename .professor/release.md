# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Tier B: dreamer — legacy lane surfaces migrate atomically to map-backed memory on first hook consultation, with tracked-source and ownership checks.
- Tier B: pfm — `/clear` hides the completed Claude chat until its transcript grows; `/exit` stays unchanged and `/bb` retires across commands, skills, and hook wiring. (hook)
