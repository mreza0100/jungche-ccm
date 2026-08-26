# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Fixed: `pfm` Codex `/clear` — a chat's display NAME can no longer move its pane binding. Codex's
  index lags a post-clear rename by one refresh, so for that window the name resolved to exactly
  one thread: the dead pre-clear one. The pass took that as the pane's identity, walked the binding
  BACKWARD onto the corpse, and clear-killed the live thread that had replaced it — after which
  `pfm chat resolve` answered with the dead thread and every inject landed where nobody was. A name
  may now SEED an unbound pane and nothing more; only a bare thread id observed on the pane's own
  status line — which is exactly what a post-clear thread shows, because it is unnamed — may move a
  binding.
- Fixed: `pfm` Codex `/clear` — one thread can no longer be bound to two panes. Two panes sharing a
  display name both resolved to the single thread that carried it, so the fleet followed one of them
  into a chat it was not running. The incumbent keeps the thread; a second claimant is refused and
  says so.
- Fixed: `pfm` Codex `/clear` — a resume or fork, which lands a CHILD rollout in the same pane and
  looks exactly like a clear from the status line, no longer retires the chat's own lineage root and
  makes the chat vanish from the fleet. The lineage is consulted, and a lineage that could not be
  READ refuses the kill rather than guessing.
- Fixed: `pfm` Codex `/clear` — a pane binding that points at a thread a clear already retired is now
  DROPPED. That binding is impossible, and it was also a trap: from then on the pane shows a name,
  and a name may not move a binding, so nothing could ever move it off the dead thread. Dropping it
  returns the pane to unbound — the honest answer — and its own status line re-seats it. An
  unreadable kill table drops nothing, and an explicit `pfm chat kill` (which hides a chat that is
  still running) is not a retirement.
- Added: `pfm doctor` reports the Codex pane bindings against each other — two panes on one thread,
  a pane bound to a thread a clear already retired, and every live pane pfm currently cannot follow
  through a clear. All three were invisible on every surface until now; the first two were found by
  reading the meta table by hand.
- Fixed: `pfm doctor` counts a Codex pane binding whose PANE is gone as stale rather than contested,
  and reports the split (`total / live / stale / contested / retired`) so the numbers reconcile. The
  binding table outlives its panes — a real host held 76 bindings for 19 live panes — so the first
  cut of this check reported eight fleet-wide collisions that were entirely dead-pane litter. A
  check that cries wolf fails the same way as one that stays silent.
- Fixed: `pfm doctor` no longer says a binding "was dropped" when it did not drop one. The reason
  string is shared with the reconcile pass, which is the only writer; the read-only report now
  states the condition and the pass states the write.
- Changed: `pfm chat inject` REFUSES to send when no sender identity can be derived, instead of
  delivering the message stamped `UNSIGNED — sender identity underivable`. An unsigned message asks
  its recipient to act on an instruction from nobody, and the recipient's only defensible answer is
  to distrust it — so the send accomplished nothing except to look like it worked. The refusal fires
  at the sender, the one party who can still repair the identity.
  #### → For: a detached or environment-scrubbed caller (`setsid`, `nohup`, a disowned worker) must
  now state its identity — `CHAT_SENDER_SESSION=$(pfm whoami) CHAT_SENDER_LABEL=<label> <command>` —
  or pass the new `pfm chat inject --allow-unsigned` to send anyway. Sends from inside a chat are
  unaffected.
- Changed: `pfm chat reload` takes `--account N`; every setting is now a flag and there are no
  positional arguments. A rejected word is answered with the flag it should have been
  (`cache off` → `--1h off`) rather than a bare usage line, and the command card states that the
  calling chat's own pane is detected automatically so `--sock` is never needed to reload yourself.
  The bare positional account number still works.
- Changed: `pfm` picker — a lone `{GROUP}:{NAME}` chat gets its group panel and indent immediately
  instead of sitting flat until a second member arrives and re-lays out the list. The prefix rule
  tightened to match: a group is a whitespace-free prefix, a colon, and a non-empty name, so
  `fix: the bug` stays the sentence it is.
