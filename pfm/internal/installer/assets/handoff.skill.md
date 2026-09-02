---
name: handoff
description: 'Hand this chat''s whole working context to a FRESH chat in the same pane and hide this one: write a handoff file, then run `pfm chat reload --fresh --hide --then "Read <file> in FULL, then continue: …"`. Use when context is nearly full, before a long task''s next phase, or when the user says /handoff [message].'
---

# `/handoff` — hand this chat's context to a fresh chat in the same pane, then hide this one

Run these THREE steps, in order, as your LAST action — the chat is about to exit and, once the
fresh one owns the pane, this conversation is hidden from the picker (`pfm chat unkill <id>`
brings it back):

1. **Write the handoff file.** `mkdir -p ~/.local/share/pfm/handoff` first, then write
   `~/.local/share/pfm/handoff/<YYYYMMDD-HHMMSS>-<cwd basename>.md` with these sections, in this
   order:

   - **Task** — the user's original ask VERBATIM plus every later addition, quoted.
   - **State** — what's done and what's running: agent ids, worktrees, commits, files touched.
   - **Decisions** — each one with its why.
   - **Open questions**
   - **Next steps** — ordered, concrete; the first one runnable as-is.
   - **Anchors** — `path:line` for every file that matters.
   - **Commands** — the exact gates/scripts to run.
   - **Rules learned** — constraints discovered this session.
   - **Owed to the user** — every item the final report must contain.

   The fresh chat has NO access to this conversation. Write what it needs to continue without
   asking: complete sentences, the user's own words wherever wording matters, never a summary of
   a summary.

2. **Reboot fresh and hide this chat, once, via Bash:**
   ```
   ~/.local/bin/pfm chat reload --fresh --hide --then "Read <path> in FULL before anything else, then continue from its § Next steps. <the user's /handoff message, if any>"
   ```
   `--hide` is what hides the conversation being handed off — the command records it only after
   the reboot completes, so a failed reload leaves this chat listed and live. Keep any
   `--account N` / `--1h` the user asked for. Never pass `--sock`.

3. Reply ONE short line — the handoff path — and END THE TURN. In-flight sub-agents die with the
   reboot, so land or checkpoint them in the file FIRST.
