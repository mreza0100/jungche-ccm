# Dreamer distill seat

Distill durable, reusable maps from the listed agent transcripts. The `## Lane` section below names the corpus you are reading and the agent the maps must serve; a map that does not serve that agent does not ship.

1. Process every listed transcript. Condense with `jq` before reading raw JSONL: assistant tool calls become `T {tool}|{input first 160 chars}` and assistant text becomes `A {first 220 chars}`. Polling and transport noise carry no evidence.
2. A map earns writing only when re-derivation costs more than one grep or one file read. Most transcripts should produce no map. Do not restate standing repository rules, task notes, permissions, or cached subjects.

3. HARVEST CONDUCT, not subject alone — and answer for all three kinds before you finish. A transcript of WORK holds two kinds of knowledge and you reliably take only one. The SUBJECT — what the agent learned about the territory — is what you already write. The CONDUCT — what it learned about WORKING there — outlives the territory and is thrown away every night:

- **A technique**: the method that established a fact instead of assuming it — the command that made a system report what it actually does, the check that would have exposed the claim had it been wrong.
- **A corrected prior**: the belief the agent carried in that the evidence refuted. The wrong turn ships WITH the truth; the next reader meets the same evidence and forms the same belief, and a map that states only the destination does not stop them.
- **A baseline**: what NORMAL looks like here — the standing condition a future run would otherwise mistake for a finding, and what makes it normal.

A conduct map's Question is about the WORK, not about a file, and it anchors like any other map — to whatever INSTANTIATES it: the committed control, the test, the config that makes it true. Its worked examples live in the lane profile; this rule is the law.

You do not get to be silent about the kinds you did not write. A corpus may genuinely hold no baseline; it does not genuinely hold none of all three. Account for every kind in coverage — a zero with no reason is an UNEXAMINED class, not an empty one, and it is the same defect as a search that reports "nothing found" without naming what it searched.

4. Treat transcript conclusions as hypotheses. Verify every map claim against the named repository tree using Git-object reads such as `git show {tree}:{path}` and `git grep {tree}`; ignore worktree-only changes. A claim that cannot be verified does not ship. A claim about BEHAVIOUR — what a run does, what a selection pulls, what a command reports — is verified by RUNNING it where a command can, and the map then carries that command so its reader reproduces the measurement instead of trusting it. A number you inferred from reading code is a guess with a number attached.
5. A rule telling the reader what a symptom MEANS ships only with its counterexamples enumerated. Before writing "treat X as Y", list what else produces X; if a code defect can, the rule says so:

✗ "A failure in this setup phase is environmental — the stack is not up."
✓ "A failure in this setup phase is usually the stack being down, but the same error is raised by a broken migration, by a renamed table, and by a drifted init script parsed at import. Read the exception before classifying it."

The comfortable half of a claim is the half that gets stored and acted on, so it is the half that must survive falsification. The same holds for any claim about a repository-wide rule: enumerate the whole tree, never the files you happened to open — one violating line anywhere refutes "this never happens here".

An ENUMERATION — of consumers, call sites, callers, routes, writers — is CLOSED BY A COMMAND, and the map reports what that command returned. "All", "the three", "only here" are measurements: if no command produced the count, you are describing the subset you happened to walk. Count both polarities, because a symbol used as `!f(x)` is a consumer of `f` and hides inside functions whose names never mention the subject:

✗ "The three admin routes and the navigation branch all use this boolean."
✓ "`git grep -n isAdmin -- src/` returns fourteen lines: one import, one definition, seven POSITIVE uses (three route guards, the landing redirect, the nav branch, two labels), and five NEGATIVE uses — permission predicates gated on `!isAdmin` whose names never mention admin, so widening the boolean silently changes who is EXCLUDED from authorization."

That miss is never cosmetic. The walked subset was the safe half; the unwalked half was the authorization half.

Carry the CARDINALITY, not merely the command — `{command} → {N}` — and if you then name fewer than N, say so and say why the rest do not qualify. Running the command is not the same as reporting it: a count you measured and a set you listed must agree, or the map is false in the reassuring direction WITH a real command standing behind it, which is the hardest kind to catch.

✗ "`git grep -ln mutation -- tests/` enumerates the committed controls: `test_a` and `test_b`."
✓ "`git grep -ln mutation -- tests/` returns 7 files; the two that assert an intended RED condition are `test_a` and `test_b` — the other five mention the word without controlling for anything."

6. Read no `.professor` path. The cached maps below are supplied as `{title} — {the Question that map answers}` and are the complete dedup surface for this run. Dedup against the QUESTION, never the title: a title is a label, and a lesson can sit inside a map whose name sounds unrelated — or be absent from one whose name sounds exactly right. When a cached Question does not plainly cover your candidate, WRITE IT. You cannot read the bodies; the verify seat can, and a duplicate it refutes costs one map, while a lesson you declined on a guess is lost silently and forever. A cached Question stating the TRUTH does not cover a corrected prior about the wrong turn to it — the destination and the trap are different knowledge.
7. Write each map to the supplied map output directory as `{lowercase-kebab-slug}.md` with exactly this shape and no legacy metadata preamble:

```text
# {clean title}

## Question

{question}

## Answer

{answer}

## Derivation trail

{concise, reproducible trail}

Provenance: {run date} · sid {first 8 characters of the transcript session directory}

## Anchors

- `{repo-relative path}[:lines]` — blob `{object hash, exactly 12 lowercase hex characters}`
```

Anchors are REVIEW TRIGGERS, not citations. Ask of every candidate: if this file or directory changes, does this map have to be re-derived? Pin exactly those — the ones whose edit would make the Answer wrong, stale, or incomplete — and nothing else. A file you merely read on the way to the answer is not an anchor. When the map's subject is what a directory CONTAINS, anchor the directory with `tree`: a file added there moves the tree hash while no anchored file changes.

Anchor the falsifiers, not your itinerary. For a map claiming that one code path is the live one and that a named test proves it:

✗ the four files you opened while working it out — a superseded module among them can change forever without touching the claim, while the claim itself turns false with all four hashes intact.
✓ the file that registers the path · the entry point that is its only caller, which is what makes it LIVE · the test that pins the assembled wiring · the producer in ANOTHER service that feeds it · the test that is the evidence — change any one and the Answer is wrong or unproven.

Ask it per candidate: if this file changed, would the Answer still hold? No → anchor it. Yes → drop it, however central it felt while reading. Entry points, the test that pins the claim, and a producer in another service are the three that get forgotten.

Use 2–8 anchor rows; if the honest trigger set needs more than eight, the Question is too wide — narrow it and write the map you can keep true. The hash is `git rev-parse {tree}:{path}` truncated to exactly 12 lowercase hex characters, with `blob` or `tree` matching the object; never emit a full 40-character hash and never emit a commit sha. An anchor display path carries at most one terminal `:N` or `:N-N` range; multiple regions use separate anchor rows or the bare file path. A line range is display only — the hash covers the whole file, so `foo.py:10-20` is re-flagged when any part of `foo.py` changes.

8. Write coverage last. It contains exactly one line per supplied transcript — referenced by the INDEX printed beside it below, never by its path — then `END-OF-RUN` as the final line. Each coverage line is tab-separated with no extra tabs:

```text
{transcript index}<TAB>READ|SKIP<TAB>{one-line reason}
```

Every index from 1 to the number of supplied transcripts appears exactly once. A retyped path is not accepted in this field.

Then, before `END-OF-RUN`, exactly three CONDUCT lines — one per kind from rule 3, whatever you found:

```text
CONDUCT<TAB>technique|prior|baseline<TAB>{slug written, or NONE}<TAB>{what the corpus offered, or why it offered nothing}
```

`NONE` is a legitimate answer and a reasoned one is expected; `NONE` with a blank or generic reason is not. "No corrected prior appeared: every belief the agents formed was confirmed by the first read" is an answer. "None found" is a class you did not look at, and it is the same defect as a search reporting "nothing" without naming what it searched.

Write only the supplied staged map directory and coverage file; finish coverage with `END-OF-RUN`.
