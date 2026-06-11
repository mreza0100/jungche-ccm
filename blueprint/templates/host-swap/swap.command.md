---
description: Swap the account THIS chat bills to (per-session Keychain credential) — no arg opens an account picker, or pass 1|2|3 to jump; other sessions untouched, takes effect on the next message
allowed-tools: Bash(~/.claude/bin/cc-account-swap.sh:*)
---

## Current account (marker: `<num> <label> <email>`)

!`~/.claude/bin/cc-account-swap.sh status`

## Arguments

$ARGUMENTS

## Accounts

<!-- EDIT: update this table with your real labels, emails, master dirs, and Keychain items -->
| n   | badge | label    | email                  | master     | keychain item                    |
| --- | ----- | -------- | ---------------------- | ---------- | -------------------------------- |
| 1   | 🥇    | work     | you@work.example       | ~/.claude  | Claude Code-credentials          |
| 2   | 🥈    | personal | you@personal.example   | ~/.claude2 | Claude Code-credentials-XXXXXXXX |
| 3   | 🥉    | third    | you3@example           | ~/.claude3 | Claude Code-credentials-YYYYYYYY |
<!-- END EDIT -->

## Task

1. Legacy session (no per-chat marker above) → run `~/.claude/bin/cc-account-swap.sh` with no args and relay its ABORT output verbatim, including the relaunch instructions. Stop.
2. Arguments contain a standalone 1, 2, or 3 → skip the picker, go to step 4 with that number.
3. Otherwise present the picker with AskUserQuestion — one question, header "Account", question "Which account should THIS chat bill to?":
   - One option per account, ordered in cycle order (1→2→3→1) starting with the account AFTER the current one; append " (Recommended)" to that first label — it is what the no-arg cycle would have picked.
   - Label: badge + label, e.g. "🥈 personal". Description: the email; for the current account append " — current".
   - Each option gets a `preview` card in exactly this shape, filled from the table:

     ```
     🥈  ACCOUNT 2 · personal
     ────────────────────────────────
     email     you@personal.example
     master    ~/.claude2
     keychain  Claude Code-credentials-XXXXXXXX
     ```

   - If the pick equals the current account: reply "unchanged — still on <label>" and stop without running the script.

4. Run `~/.claude/bin/cc-account-swap.sh <n>`.
5. Relay the result in at most two lines: which account THIS chat now bills to, that other sessions are untouched, and that it takes effect on the next message (~30s Keychain cache). If the output contains ABORT, report it verbatim — including the relaunch instructions it mentions.
