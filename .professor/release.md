# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Change: codex mirror — `pfm codex build` is now the single writer; the Stop hook (`codex-sync.sh`)
  calls it instead of the legacy `node build-codex.mjs`, which stays in the tree unwired. Two writers
  stamped different generated markers into the same files and rewrote each other's output on every
  run. A new `.claude/codex-build.json` carries the repo's compiler config (model map, root adapter,
  agent preamble, `blueprint` project exclusion, `gitter` never-register) so the native compiler
  reproduces the legacy output byte-for-byte apart from the compiler's own name.
  #### → For: adopters get the same wiring from the blueprint twin; `pfm` must be on PATH for the
  Stop hook to compile (it exits quietly when absent). (cost: hook now shells to `pfm`, not `node`)

- Fix: codex compiler — `build-codex.mjs` now claims `pfm codex build`'s generated marker, so a
  repo mid-migration between the two compilers can still reclaim its own outputs instead of
  reporting them unmanaged forever. A `dev.sh verify` gate (`scripts/check-codex-markers.mjs`)
  holds every copy to claiming all three marker generations while still rejecting unmarked files.
  #### → For: nothing to do — the transition window is now safe in both directions.

- Mechanics: the statusline's cache window reports an unreadable transcript as `💾{ttl}!` instead of vanishing. A segment that renders nothing when it cannot measure the window is indistinguishable from a statusline that has no cache timer, so the one state worth shouting about — a chat running with transcript saving off, which can neither be resumed nor measured — arrived as silence. `!` is deliberately not `?`: `?` means the transcript was read and carries no user turn to anchor on, a fact about the chat; `!` is a fact about us.

  #### → For: a red `💾{ttl}!` on a chat that used to show a timer means that chat's transcript is not on disk — check for an inherited `CLAUDE_CODE_CHILD_SESSION`, and expect `/resume` to have nothing to resume.

- Mechanics: the captured-input goldens carry a jail-supplied `transcript_path`, so the cache window is exercised on the path that renders a WINDOW. Both goldens previously omitted the field — a real one is a machine home path and cannot enter a tracked file — which left every state of the segment, working and broken alike, with no golden coverage at all.
