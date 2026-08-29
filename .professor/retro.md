# Retro — the steering-conscience inbox

An **inbox**, not a change log. Sessions append when a steering correction reveals something the
framework files should have said; `/ptm retro` consumes it.

Append one entry per correction:

```
## {date} — {one-line subject}
Observed: what actually happened, concretely.
Amend: {file}#{section} — what the file should say instead. Or `judgment` if no text fix applies.
Resolved:            ← /ptm retro stamps this in place: `Resolved: {date} — {where it landed}`
```

Entries without a `Resolved:` line are the open queue. `/ptm retro` folds each `Amend:` into the
named file through the normal change flow, stamps the entry, and logs the fold to `drift.md` or
`release.md` like any other change.

## Entries
