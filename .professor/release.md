# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

### Added

- pfm: the Cosmos tab — the fleet rendered as a living universe over a durable chat-to-chat comms ledger. Every chat is exactly one node; every project is a star its chats orbit as revolving planets; a spawned chat rises as a moon around its parent and is born at the parent's angle, so lineage is visible in the sky itself. Orbits draw their tracks, the galaxy breathes between messages, labels cap at 14 runes and dim under their glyphs so the sky stays legible on a crowded box, and a VS Code terminal gets a reduced sky that cannot wedge its renderer. Pressing `o` collapses the whole hierarchy back to the first cosmos — every chat on one shared ring, no suns, no revolution — and `o` again restores the solar systems; with the sky animation off, the toggle seats instantly instead of easing. Behind the pixels, the ledger records who spoke to whom so an edge is a fact, not a guess, and a graph refresh may retarget a seat but never teleport it.

- pfm: `issue_servicedesk` — a complaint box agents can file through over MCP, read back with `pfm issues`. A subagent that hits a broken tool now has somewhere to say so besides its own doomed transcript.

- pfm: a chat DNS — `pfm chat resolve` answers what a label IS (name, session, socket, thread) through one resolver with the exact-match return-code contract, so an identity can never silently render as a display name again.

- pfm: `pfm chat new --role ROLE` — a seat is its agent role from birth: the role's constitution arrives with the first prompt, and a `--role` seat re-arms that constitution after every reset (`/clear`), so a long-lived role seat cannot drift into an unbriefed generic chat.

- pfm: `pfm reap` gains the idle horizon — chats idle past a configurable horizon are classified for retirement in the same dry-run-first sweep the socket graveyard uses; the preview and the `--apply` share one classifier, so what reap says it will take is what reap takes.

- pfm: a Makefile for the engine — build, the repo gates, host install, and the stale-process sweep in one place. Replacing `~/.local/bin/pfm` never touches a process already running the old image, so `make install` restarts the MCP daemon every time and names every other stale process it can see, turning the silent old-binary trap into a listed one.

### Changed

- pfm: `chat_new` over MCP is born in its caller's project directory — a chat spawned by another chat starts where its creator works, not where the daemon happens to run.

- blueprint: `limits-hook.sh` becomes opt-in — the shipped `settings.json` no longer wires the rate-limit whisper into every `UserPromptSubmit`; the hook's own header now carries the exact stanza to re-enable it. (cost)

  #### → For: adopters who relied on the automatic rate-limit gauge in orchestrator turns — re-add the `UserPromptSubmit` stanza from the top of `scripts/limits-hook.sh` to your `settings.json`.

- pfm: the chat and Harvester MCP surfaces route natural phrasings first-try. The server instructions now carry the mapping an operator reaches for — "send / tell / message chat X" is `chat_inject`, "give yourself a compact" is `chat_self_compact`, "fetch this DOI" is `fetch`, "find papers about X" is `findWorks` — so an agent no longer has to be told twice which verb a plain-language ask means.

### Fixed

- pfm: a chat the MCP daemon spawns can actually find its engine. A systemd user service starts with systemd's bare default PATH, which cannot see `~/.local/bin` — so the pane's `claude` died at birth on "command not found", took the fresh tmux server with it, and the operator read "no server running": tmux named, the engine never mentioned. Three lids close over that hole: the spawn now preflights the engine binary against its own environment and refuses loudly — naming the binary, the PATH, and both remedies — before any server exists; a server that dies between creation and configuration now names its pane command as the likely cause instead of reporting tmux trivia; and the installed `pfm-mcp.service` extends PATH with `%h/.local/bin` so the standard layout just works. (cost)

  #### → For: rerun `pfm install` so the updated unit is staged and the daemon restarted; a machine whose engines live elsewhere pins absolute `<engine>.binary` paths in the machine config instead.

- pfm: identity and lifetime hardening for Codex seats — a live-process rollout identity outranks stale store rows, a pane binding advances when Codex rotates rollout files instead of following the retired one, and same-name resolution goes roster-first for CLI and MCP inject and `chat_resolve`: a unique named row beats a stale duplicate window name, and a genuinely ambiguous name returns code 2 carrying the candidates' thread ids and sockets rather than guessing.

- pfm: chat servers spawned from inside the MCP daemon's systemd service escape into their own transient scope (`systemd-run --user --collect --scope`), so restarting the daemon reaps daemon work without killing every terminal it ever created; a service host without `systemd-run` is refused loudly instead of spawned mortally.

- pfm: the harvestpy worker's stderr sink was a bare `bytes.Buffer` that os/exec's pipe-copy goroutine writes for the life of the subprocess while error paths read it mid-flight to decorate their messages — a data race the `-race` sweep catches in both the conversion and the browser worker. The sink is now mutex-guarded; the whole harvestpy suite runs clean under the race detector.

- pfm: `pfm install` no longer refuses the whole machine because one engine CLI is unauthenticated, and the themes the config declares are actually source-fetched at install time (issue #13); the install prerequisites and runbook say what the installer now does.

- pfm: the `--then` waiter identifies the turn it rides out instead of guessing from timing, so a steer queued behind a long turn lands after that turn, not inside it.

- pfm: a tmux probe warning no longer swallows the real failure it was attached to — the warning prints AND the error still propagates.

- pfm: `chat_self_compact`'s ambient-identity refusal told a Claude chat to "run the equivalent `pfm chat ...` command" — a CLI twin that does not exist for this one verb, making the remedy a dangling pointer at the exact moment the caller needs a working next step. The self-compact refusal now names the real fallback: `pfm chat inject $(pfm whoami) --force-now '/compact <focus>' --then '<steer>'` from the chat's own shell.

- infra: the dev fence container runs a real init (`init: true`), so a double-forked descendant is reaped as on a real machine instead of lingering as a zombie that makes the dream seat's proc-jailer stress read "still exists" for a process that is already dead.
