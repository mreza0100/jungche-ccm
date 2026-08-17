---
name: sleep
description: Defer a task — sleep a wall-clock duration, then run a prompt as if the user just sent it. A background timer re-invokes on wake; the deferred prompt runs then, not now. Trigger — /sleep <30m|2h|1h30m|90s> <prompt>.
argument-hint: <30m|2h|1h30m|90s> <prompt to run on wake>
disable-model-invocation: true
---

# /sleep — deferred execution

Wait a wall-clock duration, then run a prompt. The pause is a background `sleep`; the harness re-invokes you when it exits, and the deferred prompt runs at that point — never now.

## On invocation

In `$ARGUMENTS`, the **first whitespace-delimited token** is the duration; **everything after it** is the prompt to run on wake. A missing duration, a non-positive duration, or an empty prompt → print the usage line `/sleep <30m|2h|1h30m|90s> <prompt>` and stop.

1. **Resolve the duration and a task dir** in one Bash call. Units are `h`/`m`/`s` and their combinations (`2h`, `30m`, `90s`, `1h30m`); a bare number means minutes. Substitute the first token for `<DUR>`:

   ```bash
   DUR="<DUR>"
   case "$DUR" in ''|*[!0-9hms]*) echo BAD_DURATION; exit 0 ;; esac
   if echo "$DUR" | grep -qE '^[0-9]+$'; then SECS=$((DUR*60))
   else SECS=$(echo "$DUR" | sed -E 's/([0-9]+)h/\1*3600+/g; s/([0-9]+)m/\1*60+/g; s/([0-9]+)s/\1+/g; s/\+$//' | bc); fi
   [ "$SECS" -gt 0 ] 2>/dev/null || { echo BAD_DURATION; exit 0; }
   DIR="tmp/sleep/$(date +%Y%m%d-%H%M%S)-$$"; mkdir -p "$DIR"
   echo "SECS=$SECS DIR=$DIR"
   ```

2. **Persist the prompt verbatim** to `$DIR/task.md` with the Write tool — never via `echo`, since the prompt may hold quotes, newlines, or `$`.

3. **Arm the timer** as background Bash (`run_in_background: true`), then end the turn immediately. Substitute the resolved `<SECS>` and `<DIR>`:

   ```bash
   sleep <SECS>; echo "/sleep elapsed — execute the deferred task now: Read <DIR>/task.md and carry out its instruction verbatim, exactly as if the user just sent it."
   ```

4. Confirm in one line — `💤 sleeping <DUR> — on wake: <≤8-word prompt summary>` — and stop. Do not poll, busy-wait, or run the task now.

## On wake

The timer exits and re-invokes you with the `/sleep elapsed` line naming the task file. Read that file and carry out its instruction verbatim — as if the user had just sent it, including when the prompt is itself a slash command.
