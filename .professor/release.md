# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Important: wave pipeline (`commands/wave/refine.md` + `agents/scheduler.md`) — precondition anchors: a production behavior a task relies on (an "existing" code path, a CLI verb or flag, an exit code) is code-verified and anchor-cited at refine time, with a missing surface becoming a scheduled dependency task; the scheduler gains an **Anchors** step that grep/read-verifies every prose-relied surface before scheduling — diff-scoped staleness catches a spec broken by later commits, the new step catches one born wrong (RE-REFINE instead of a silent pass).
