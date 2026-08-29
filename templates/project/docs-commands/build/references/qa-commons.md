# QA Commons — shared rules for the pipeline QA gates

Shared by the per-roster QA protocols (`{project}/.claude/agents/qa.md`), spawned as `qa-{project}` by `/wave:builder`, `/wave:orchestrator`, and `/wave:live`. Each child `qa.md` keeps only its project-specific delta (paths, commands, compliance checks) and cites this card for the rules below.

## 360° sweep

Before writing any tests, spawn a separate agent for the 360° sweep — it must run with a clean context to avoid bias. Use `Agent(subagent_type: "general-purpose")` with a prompt containing ONLY: the subject (one sentence describing the change under test), the domain (`test`), and an instruction to read `.claude/commands/p/360.md` and execute the protocol. Do NOT include any of your own analysis or findings in the prompt. Use the returned angle list to guide which adversarial tests to write.

## Test economy

Acceptance gates for every test written.

- Reuse first: grep for a test already covering the contract, then extend or parametrize it instead of adding a near-copy (root CLAUDE.md § Reuse before you write, applied to tests).
- Fail-without-fix: accept a regression test only after observing it fail against the unfixed code — or against a deliberate safe re-break when the fix already landed. One that passes either way pins nothing.
- Parametrize siblings: three or more cases in one class/describe differing only in a literal become one parametrized test, each case id carrying its original test name verbatim (`test_scan[large_payload]`) so a failure names the case.
- Extend, don't accumulate: write into the existing module owning the contract area; add a module only when none covers it.
- Retire by the same gate: break the contract an existing test claims to guard; a test that still passes pins nothing — remove it and name the test that catches that break instead. A test whose contract cannot be broken (prompt text, schema, {DOMAIN_ADJ} value-set, absence guard) is judged by reading, never by this gate.
- Report the net: state tests added, tests removed, and the project's test:production LOC ratio beside the coverage figure.

## Test validity

§ Test economy asks whether a test is worth keeping. These ask whether it tests anything at all.

- Reaches production: imports and calls the symbol it names. A test asserting over a value it declared itself, or over its own inline re-implementation of the logic, is green against broken code by construction.
- Asserts the assembled artifact: the resolver map the server serves, the registry the agent binds, the graph the runtime builds — never a copy the test rebuilds from parts.
- Name honoured: the body asserts what the name claims. A name claiming an absence the body never checks is a false coverage report.
- Substring assertions name their subject: `"x" in PROMPT` matches anywhere in the text, including a different field. Assert the section or structure that must carry it.
- Counts derived, never literal: `sorted(actual) == sorted(declared)`, not `len(x) == 21`. A literal count cannot name what changed, and drifts against its twin in another file.
- A guard whose scan can return empty asserts non-empty first — a check that passes on nothing is not a check.

## Affected-first

Root CLAUDE.md § Zero-Tolerance Tests governs: run the tests/scripts you wrote or changed, plus the directly affected ones, first; only once green, proceed to the scope's run — TARGETED re-runs failing+affected only; FULL/POST-MERGE runs the full suite once as the gate, never looped to chase a fix.

## Isolation on suspicion

A gate run shares the box with whatever else is running, so a failure whose shape is infrastructural — timeout, hook, teardown, spawn, connection — fits contention as well as it fits a defect. One run cannot tell them apart, and a gate that reports the same verdict for a broken build and a busy machine has decided nothing.

Re-run each such failure alone on an otherwise idle box, through the project's own runner — invoking the underlying test tool bare (bypassing the project's wrapper script or config) skips the loader and per-worker env and dies with a misleading module-resolution error. One isolation round decides it:

- passes alone: contention. Report both readings — "failed in the full run, passed 3/3 isolated" — never the isolated one by itself.
- fails alone: a real defect. Route it through the fix loop.
- isolated runs disagree with each other: the test is flaky, and that is the finding — `BUG-FLAKY-TEST`.

An assertion failure is a bug on first showing and never qualifies for this. Read a failure's own text before trusting its label: runners mislabel which hook fired, and a profile that printed a passing scenario tally already ran its scenarios.

## Inline-fix escape hatch

If a bug is trivial (<5 lines, single file, zero logic change — e.g. typo, missing import, off-by-one), fix it in-place and note it in the bug report as `INLINE-FIXED`. Don't create a fix-loop cycle for trivia.
