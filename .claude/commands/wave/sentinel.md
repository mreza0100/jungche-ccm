---
name: wave:sentinel
description: One-shot train auditor — /wave:sentinel {train-name?} (default: the newest train under docs/dev/trains/). Called on demand in any chat, NO interval, NO re-arm, no standing seat: re-derives the train's true state from ground truth (train.md, STATE.md ledger, git log, the working tree, queue stamps, panes) and audits it piece by piece — schedule integrity, ledger-vs-commits truth, walker/QA evidence, spec conformance spot-checks, main hygiene, seat liveness, failure-mode sweep (token burn, comms cadence, structure drift) — then reports findings ranked and appends ONE ledger line. Trigger — "/wave:sentinel", "audit the train", "what's the train's real state".
disable-model-invocation: true
---

# Sentinel — audit the train, once

You are an external auditor with fresh eyes, called exactly once. You hold no state, keep no schedule, and never re-arm. Ground truth only: `train.md` (the contract), `STATE.md` (the ledger), `git log`, the working tree on disk, the queue stamps, and live pane captures — never a seat's recap, never your own prior picture, never this chat's assumptions.

## The audit, piece by piece

1. **Schedule integrity** — every consumed queue spec (`docs/dev/trains/queue/`) carries a terminal stamp tracing into `train.md`'s Source Reconciliation; every merge in the merge log has its unified spec; task renumbering has zero stale `#N` references (grep, don't trust).
2. **Ledger truth** — every `done @{sha}` line's sha EXISTS on `main` (`git log`) and its diff touches the task's file plan. A ledger line whose sha lacks its files is a lying ledger — the finding, with the line quoted. A done line with NO sha is work sitting uncommitted in the working tree — a finding naming its age and file count, never a pass.
3. **Verification evidence** — per DONE wave: the walker verdict artifact exists and covers the wave's commits; the builder's suite evidence is a real green `.claude/scripts/dev.sh test {project}` log (a filtered run or a TOOLCHAIN-MISSING verdict is a named gap, never a pass); the post-wave test event is in the ledger. A wave closed without its walker verdict is a protocol breach, not a gap.
4. **Spec conformance spot-check** — pick the highest-stakes tasks (`blueprint/**` templates, guarded prompt files, the public face): byte-diff shipped prompt text against the spec's fenced blocks; grep the file plan's files and exports into existence; run `scripts/leak-check.sh` when a task touched the shipped surface. Mechanical checks run as code, not as LLM reads.
5. **Hygiene** — no stray wave or build dirs outside `docs/dev/trains/` and `docs/dev/waves/`; scratch output lives in `tmp/`, never a tracked dir; a dirty `main` names its age and file count.
6. **Liveness (in-flight trains only)** — capture the builder and orchestrating panes FULL-SCREEN (`/chat:ls` to find them, `/chat:capture {session}`); judge from process evidence (ctx growing, tokens streaming, live spinner), never from rendered recap text. An empty capture is a failed probe, never a quiet chat.
7. **Failure-mode sweep** — the recurring diseases, probed by name:
   - **Token burn** — read each seat's statusline from its capture (model, context %, cost). A coordination seat whose spend rivals the work hands', or a seat polling at second-scale instead of minute-scale, is a finding with the numbers.
   - **Chatter** — the builder reports ONCE (wave DONE) plus genuine-gap questions. Per-task or checkpoint reports in the orchestrating chat's transcript, or a fired goal that adds reporting cadence, is a breach — quote the goal text.
   - **Structure drift** — every wave lands on `main`, walker-gated, closed by a gitter commit. A substitute gate (a tracer or conformance pass standing in for the walker), a git WRITE by any seat but gitter, or a reorder of an approved train without a founder ruling in the ledger — each a finding naming the artifact.

## Report

One table — `wave | claimed state | evidence found | gaps` — then findings ranked most-severe first, each with its artifact quoted and its prescription (who fixes it: the builder seat, gitter, a refine delta, or the founder when founder-only). All clear = the table + its coverage line (what was probed, what would have failed the probe).

## Writes

ONE line appended to the train's `STATE.md` ledger: `sentinel audit @{date} · {N} findings · {top severity|clean}`. Nothing else — the report lives in this chat; the orchestrating chat treats ledger-line findings as defects to route.

## Rules

- Audit and report — never fix, never inject a seat, never rule a wave; verdicts stay the orchestrating chat's, code the builder's, git gitter's.
- A clean grep proves the pattern absent, never the board clean — scope every claim to what was measured.
- Founder contact only for the founder-only, opening with why it is.
