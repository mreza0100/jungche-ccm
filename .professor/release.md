# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- pfm: `pfm chat status <target> --ask` — a new flag, and a matching `ask` parameter on the `chat_status` MCP tool, answering what a chat is doing RIGHT NOW rather than recapping what it last did. A collector-tier model (the existing `Ask` config: `claude-haiku-4-5` / `gpt-5.6-luna`, effort low) reads the live tmux pane capture with the last human exchange as background, and returns a bounded ≤40-word verdict: working, waiting on input, blocked, finished, or errored. `--engine` / `--model` now apply to `--ask` as well as `--summary`.

  Unlike `--summary` it never caches, and that is deliberate rather than an omission: `Summarize` keys its cache on transcript offset, which is sound for a finished exchange and wrong for a pane whose contents differ a second later — a cached ask would answer confidently about a chat that has moved on.

  It also refuses to let a failed probe read as an empty one. A chat that is not live, a capture that errored, and a chat with no exchange yet each produce their own distinct wording (`TRANSCRIPT-ONLY (chat is not live: …)`, `TRANSCRIPT-ONLY (pane capture failed: …)`, `PANE-ONLY (no human exchange recorded yet)`), and a missing engine binary says so by name. The answer is never empty, because an empty answer is indistinguishable from a quiet chat.
