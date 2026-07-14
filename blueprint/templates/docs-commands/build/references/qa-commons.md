# QA Commons — shared rules for the pipeline QA gates

Shared by the per-roster QA protocols (`{project}/.claude/agents/qa.md`), spawned as `qa-{project}` by `/wave:builder`, `/wave:orchestrator`, and `/wave:live`. Each child `qa.md` keeps only its project-specific delta (paths, commands, compliance checks) and cites this card for the rules below.

## 360° sweep

Before writing any tests, spawn a separate agent for the 360° sweep — it must run with a clean context to avoid bias. Use `Agent(subagent_type: "general-purpose")` with a prompt containing ONLY: the subject (one sentence describing the change under test), the domain (`test`), and an instruction to read `.claude/commands/p/360.md` and execute the protocol. Do NOT include any of your own analysis or findings in the prompt. Use the returned angle list to guide which adversarial tests to write.

## Affected-first

Root CLAUDE.md § Zero-Tolerance Tests governs: run the tests/scripts you wrote or changed, plus the directly affected ones, first; only once green, proceed to the scope's run — TARGETED re-runs failing+affected only; FULL/POST-MERGE runs the full suite once as the gate, never looped to chase a fix.

## Inline-fix escape hatch

If a bug is trivial (<5 lines, single file, zero logic change — e.g. typo, missing import, off-by-one), fix it in-place and note it in the bug report as `INLINE-FIXED`. Don't create a fix-loop cycle for trivia.
