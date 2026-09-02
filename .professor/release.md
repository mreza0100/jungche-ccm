# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- C: pfm — the live cosmos never shows a dead or hidden chat: `BuildCosmos(..., live=true)` now drops every node whose chat is Killed or whose unmatched identity has aged past its grace window (and every edge naming one) before the graph is ever returned, so a killed or hidden chat gets no blink, no fade, no debris in the sky — a node the picker would not list is not in the graph. The chronoscope replay is unchanged: a chat dead now still renders dimmed as the ghost it was, and `enter` on it reads "enter needs a live chat — {label} is gone; only the ledger remembers it". The two death-animation constants and the model's death-window bookkeeping (`cosmosDiedAtNS`, `cosmosDeathVisual`) are removed along with the behaviour they animated; `CosmosRowGrace` replaces `CosmosDeathBlink`+`CosmosDeathFade` as the one grace window an unmatched identity gets before absence means gone.
