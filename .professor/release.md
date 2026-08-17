# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Fixed: `gather/tmuxprobe.go` — a socket is swept ONLY on `ErrServerGone`. Every other probe error now warns and leaves the socket alone. Previously any error reached `os.Remove`, so an unreadable-but-healthy server lost the path every client and every `pfm` lookup reaches it by; the surviving chat rendered as merely resumable. The `>1h` age guard did not protect against this — it selected for it, since "untouched for an hour" describes a live idle chat, not a dead one.
#### → For: adopters on tmux 3.6 who saw chats become resumable — the servers are usually still running. `ps -Ao pid=,command= | awk '/tmux -L (cc|cx)-/ {for(i=1;i<=NF;i++) if($i=="-L"){print $1, $(i+1); break}}'` recovers the socket name from the server's own argv, and `kill -USR1 <pid>` makes tmux recreate the socket file. Sweep every chat pid before attaching anything; a `pfm ls` in between will not re-list them.
- Fixed: `internal/tmuxfmt` — one parser for tmux `-F` output, accepting the separator as a raw `0x1F` byte or the printable escape `\037`. All five parse sites route through it. tmux ≤3.5 escapes the separator and 3.6 emits the byte, so a parser knowing one spelling read a whole record as a single field and every live chat silently lost its row.
#### → For: this and the sweep fix are each insufficient alone — parser-only leaves delete-on-any-error primed for the next unrelated tmux error; sweep-only leaves the fleet blind on tmux 3.6.
- Added: `pfm` runs on macOS as well as Linux, from one binary and one source tree. The process table comes from `sysctl` where there is no `/proc` (`gather.NewProcFS`, `action`, `dream/seat`), the open-gate ioctl is build-tagged per kernel, and `doctor` now reports `process_table readable pids=N` by READING the table rather than stat-ing `/proc` — a better probe on Linux too, since a `/proc` that exists and denies the read is the failure that matters.
- Added: `pfm install` wires a launchd agent on macOS instead of systemd units, carrying both triggers in one job (`WatchPaths` = the `.path` unit, `StartInterval` 900 = the `.timer`). Only the current platform's scheduler assets are staged.
#### → For: on macOS the rc 97 refusal narrows from "a user bus is reachable" to "the name-sync agent is mid-execution" — launchd is always live, so a literal port would refuse every install forever. A runner that cannot probe the gate makes the installer SAY the gate did not run rather than imply it passed. (cost)
- Fixed: the shim strips `CLAUDE_CODE_CHILD_SESSION` on every launch, alongside `CLAUDE_CODE_SESSION_ID` and `CLAUDECODE`, from one `CC_SESSION_UNSET` array used by all three launch paths. A chat born inside another chat inherited the marker and silently ran with transcript saving OFF. (cost)
- Fixed: the auto-open hook (`CC_AUTO_OPEN`) returns to the shim, deferred to the first prompt and disarmed before it launches. A terminal profile calling `cc` from `~/.zshrc` reached `/usr/bin/cc` — the C compiler — because the installer appends its source line to the BOTTOM. `pfm install` now reports any fleet command called above that line.
- Fixed: two flakes that are not platform-specific — the branch jail read a detached seat's argv with a single `ReadFile` before the pane had exec'd, and a spawned nohup stub was given a one-second ceiling that only holds outside a parallel suite. Both now wait.
