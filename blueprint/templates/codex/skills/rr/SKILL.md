---
name: rr
description: Research-and-Report protocol for Codex. Invoked by `RR`, `RRP`, `research and report`, broad competitor/market/persona/regulatory research, or explicit research fan-out requests.
---

Read `.claude/skills/rr/SKILL.md` in full — it is the source-of-truth protocol. Follow it verbatim.

## Codex-only execution mapping

- Claude `Agent` calls map to Codex `spawn_agent` calls.
- Use a scout agent unless the subquestions are already obvious from the request.
- For fan-out, spawn 2-6 Codex research agents in parallel, one self-contained RRP per subquestion.
- Fan-out agents return findings in chat. They do not write files.
- The orchestrator writes exactly one aggregate report under `docs/dev/research/` using the RR filename convention.
- Do not execute explicit RR in the main conversation with inline WebSearch/WebFetch/grep. Inline search is allowed only for narrow fact checks outside RR.
- If the live-agent ceiling prevents all workers from launching at once, fill available slots, wait/close completed agents, and continue the queue until every subquestion is covered.
- If Codex child-agent spawning is unavailable, do not pretend inline research was RR. Produce an RRP prompt for the user to run elsewhere, or report the RR execution as blocked by missing agent support.
