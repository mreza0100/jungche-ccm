# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- C: interactive picker — enable terminal colors by default even when a parent exports `NO_COLOR=1` or `CLICOLOR=0`; preserve uncolored plain and TSV output.
- C: shell launch surface — remove Professor `cc*` functions and aliases, launch fresh Claude chats through the native Go action policy, retire legacy launch/account scripts, and use `PFM_AUTO_OPEN=pfm` for the managed terminal profile.
- C: optional memory helpers — rename `cc-memory-wire.sh` and `cc-memory-consolidate.sh` to `memory-wire.sh` and `memory-consolidate.sh`; migrate recognized installed copies and exact hook paths without running the helpers or changing memory data. Refuse customized copies and ambiguous hook commands instead of overwriting them.

#### → For: shell users

Use `pfm` and `pfm chat open <target>`; choose accounts in the picker. Run `pfm install --yes`, then open a new shell or source the updated shim. Legacy auto-open profile values open the picker without invoking retired commands. Removed regular launch/account scripts are recoverable from `~/.local/state/pfm/retired-commands/`. (cost: terminal-profile environment changes from `CC_AUTO_OPEN` to `PFM_AUTO_OPEN`.)
