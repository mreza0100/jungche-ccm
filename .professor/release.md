# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- C: pfm — a delivered message's reply hint names a chat the fleet resolves, never its tmux socket. The inject footer now reads the sender's LABEL afresh for every delivery — from the fleet roster first (the exact-name rung a peer's `chat_inject` matches, via the new `resolve.ResolveRosterSeat` reverse lookup on both the MCP and CLI doors), then the sender's own 🔖 statusline, then the codex window name — instead of a screen scrape cached once per process, which left one chat's MCP server signing every message for a day as `to reply: chat_inject cc-…` after its first capture landed seconds after a reload with no statusline on screen. Without a label the hint falls back to the sid (the same eight characters the footer already shows, which the roster resolves as an id prefix); a sender with neither is UNSIGNED and says so. A rename now reaches the footer on the next delivery, and the comms ledger records the label the cosmos draws edges by.
