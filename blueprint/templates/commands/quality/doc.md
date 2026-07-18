---
name: quality:doc
description: Use BEFORE writing or restructuring any permanent reference doc under docs/ (architecture, api, map, features, child-project docs), or to certify an existing doc via the Approval gate (APPROVED/REJECTED). Defines how to shape reference docs for LLM Read/grep consumption — the cluster model, the ~500-line topic-file target (~80 KB hard cap), navigation indexes, the table-vs-sections record-format rule, grep-true naming, current-state-only content, and the no-byline rule. Mandatory load for /documenter; load it yourself before any large reference-doc edit.
---

# Doc Format

Reference docs under `docs/` are read by LLM agents (whole-file `Read`, `grep`), not by humans in a rendered viewer. Shape them for that reader. Apply these rules at write-time.

**When to load:** `/documenter` loads this before writing any permanent reference doc. Load it yourself before hand-editing or restructuring `docs/agents/*`, child `*/docs/*`, or any large reference doc. Fan-out documenter workers read the extract card `docs/commands/documenter/references/doc-approval.md` instead — a declared copy of the write rules + § Approval here (this file is canonical); edit both together.

## The deciding principle

Format choice barely affects whether the model _understands_ the content — for frontier models, structure has no significant accuracy effect; model capability dominates. So decide on the **mechanics the reader actually pays for**: token cost, grep context, edit/diff locality, and prettier stability. Optimize those; comprehension takes care of itself.

## The cluster model

A reference doc is a **cluster** — a directory, not a monolith:

- **`_index.md`** — navigation only: a pointer table `| Topic | File | Covers |`, ≤150 lines, no prose.
- **topic files** — each a self-contained slice, each readable in one `Read` call.

A consumer reads `_index.md` (cheap), then opens the one topic file it needs. Two cheap reads replace one impossible one.

## Size — operational target vs hard cap

- **Target ≤ 500 lines per topic file.** This is where the cluster earns its keep: a size an agent scans in one pass and an editor updates without reflowing a monolith. The hard cap protects `Read`; the target protects the reader.
- **Hard cap ~2,000 lines / ~80 KB** — the point a single `Read` strains. A file between the target and the cap is a split that hasn't happened yet, not a healthy file.
- `_index.md` ≤ 150 lines.
- **Table of Contents** at the top of any topic file over ~100 lines, so a partial read still shows its scope.
- **Split signals — split when ANY holds:** over ~500 lines; covers more than one subject; sections have different edit cadence; the content reads as a per-pipeline append-log (`New X (wave-23)`, `New chains (radar-cross)`) rather than current state.
- **Split on overflow:** move the largest self-contained section into a new sibling topic file and register it in `_index.md`.

## Record format — table vs sections

The single highest-leverage rule. Decide by **field shape**, not habit:

- **Short, uniform cells** (port maps, access matrices, the `_index.md` pointer tables themselves) → **markdown table**. Genuinely tabular, no padding waste, one grep hit shows the whole record on one line.
- **Any long free-text field** (descriptions, rationale, prose) → **heading-per-record sections**. One `###` heading per record, a one-line bold metadata strip for the short fields, then the long field as a prose paragraph.

**Never put long prose in a table cell.** Prettier aligns every column to its widest cell, so one 600-char description pads every other row in that column to 600 chars — nearly half the file becomes alignment spaces, and editing one record reflows the entire column into a giant diff.

Sections instead:

```markdown
### Save {Feature}

**Projects:** {project}, {project} — **Status:** Active

{USER_NOUN} writes and saves {Feature} after a {SESSION_NOUN} in a plain textarea. `editContent`
seeded from `editedContent` on first non-null arrival.
```

This costs zero padding, keeps a one-record edit local, survives prettier untouched, and gives each record its own greppable `###` anchor.

## Current-state only — delete, don't annotate

A reference doc describes what IS, now. When a record is removed, delete it — never leave a `~~strikethrough~~`, a "Removed {date}", a "Deprecated", or an "Added in wave-N" note. Stale annotations poison retrieval: the agent reads a dead endpoint as real and builds on it. History lives in `git log` and epic manifests, not the reference doc.

## Edit locality

A change to one record touches only that record's lines — zero reflow of its neighbors. This is the rule behind sections-over-tables (a wide table cell reflows the whole column), delete-don't-annotate, and one-record-per-`###`. If editing one fact rewrites unrelated lines, the format is wrong.

## Anti-patterns — cut on sight

- **Monolithic reference file** — one 1,000+ line doc spanning many topics. Split into a cluster.
- **Per-pipeline changelog structure** — records grouped by the build that added them. Re-group by current-state topic; the build history is in git.
- **Tombstones** — removed records kept as strikethrough or "removed" notes. Delete them.
- **Long prose in a table cell** — sections instead.
- **Deep cross-reference chains** — doc → doc → doc. One hop, inline the essential fact.
- **Narrative bloat** — "Background", "Why we chose X in 2024", rationale prose. Encode the current rule, drop the story.

## No byline

Reference docs carry no byline. Git owns authorship and last-edited date (`git log`); the path owns ownership (the registry's Ownership Rules). A hand-maintained `> Author:` / `> Last updated:` line duplicates git, drifts, and adds read-time noise.

## Navigation contract for consumers

- Need one operation or contract → grep the cluster, read the matching topic file.
- Need one subsystem → read the cluster `_index.md`, open the named topic file.
- Need whole-domain context → read `_index.md`, then the topic files that matter.

When you split a doc that consumers reference by its old path, leave a one-line redirect stub at the old path (`> Moved to \`{cluster}/\_index.md\`.`) until those consumers are repointed.

**One hop.** A topic file is self-contained. Point to another cluster only for the authoritative source — never as a substitute for a fact the reader needs here. If a record depends on another cluster's detail, inline the essential fact; don't send the reader on a doc → doc → doc chase (each extra read costs tokens and reasoning steps, and the agent often stops before reaching the end).

## Name fidelity — docs are grep-true

Every identifier in a reference doc is the exact code/DB name, verbatim: a table or column is its {DATABASE} name (`{table_name}`, `{column_name}` — not the {ORM} TS field), a {API_PROTOCOL} operation is its SDL name, a component/chain/queue is its source symbol. A consumer who greps the code symbol must land in the doc, and the reverse. When the code renames, the doc renames in the same edit. When a record maps to a code symbol, the `###` heading **is** that symbol (`### {mutation}`, `### {table_name}`) — the grep landmark and the name are the same string. Claude Code's grep is exact-match (ripgrep, no fuzzy), so a heading that paraphrases the symbol is invisible to the search that would find it.

## Why these rules hold (grounded)

- **Prettier force-pads tables.** It aligns every column to the widest cell, with no config option to disable it, and {PROJECT_NAME} runs `prettier --write` on all markdown. So a "compact unpadded table" is a mirage — it re-bloats on the next save. The real choice is _padded table_ vs _sections_. ([prettier#12074](https://github.com/prettier/prettier/issues/12074))
- **Padding is real token waste.** In the pre-section `features` cluster, 45.9% of all table-row bytes were alignment spaces; the worst file hit 71.8%. Whitespace is ~10–24% of input tokens in formatting studies — BPE merging softens it but never makes it free. ([arXiv:2508.13666](https://arxiv.org/html/2508.13666v1))
- **Format ≠ comprehension for frontier models** (p=0.484 across 9,649 experiments; capability dominates by 21 points), which is why the rules above optimize mechanics, not "readability." ([arXiv:2602.05447](https://arxiv.org/abs/2602.05447))
- **Heading-per-record matches retrieval evidence.** Heading-based chunking improves retrieval ~35% over unstructured text, and a key:value/heading layout beat flat tables on field retrieval in head-to-head benchmarks (60.7% vs 51.9%) — the gap widens precisely when one field is long. Anthropic's own long-context guidance wraps each record as its own labeled block.
- **The size target is published, not arbitrary.** Anthropic targets ≤200 lines for always-loaded files and ≤500 lines for on-demand skill/reference files; both adherence and retrieval degrade as a file grows, and topic-boundary chunking beats fixed-size by a wide margin. Topic files inherit the ≤500 figure as their split-trigger. ([Claude Code memory](https://code.claude.com/docs/en/memory), [Agent Skills best practices](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices))

## Pre-write checklist

1. **Cluster:** is this doc a directory with an `_index.md`, not a monolith?
2. **Size:** is every topic file ≤ ~500 lines (target) and none past the ~80 KB cap? If a write breaches the target, split and register. ToC at the top if >100 lines.
3. **Record format:** does any field hold long prose? If yes, sections — not a table. Heading = the code symbol when the record maps to one.
4. **Current state:** no tombstones, no "removed/added in wave-N", no strikethrough — a deleted record is gone, not annotated.
5. **One hop:** no doc → doc → doc chains; essential facts inlined.
6. **Index:** does `_index.md` list every topic file present, and nothing stale?
7. **No byline:** no `> Author:` / `> Last updated:` lines.
8. **Format pass:** `npx prettier --write --prose-wrap preserve <file>` on everything touched.

## Approval — certify a document

Every reference doc must pass this gate before it is considered done; run it over an existing doc, not just at write-time. A doc is **APPROVED** only when ALL hold; otherwise it is **REJECTED** with the failing checks named, and the fix is applied before re-checking.

| #   | Check         | REJECT when                                                                                                                    |
| --- | ------------- | ------------------------------------------------------------------------------------------------------------------------------ |
| 1   | Size          | a topic file is >500 lines (split) or any file >80 KB                                                                          |
| 2   | ToC           | a file >100 lines lacks a top Table of Contents                                                                                |
| 3   | Record format | long prose sits in a table cell instead of a `###` section                                                                     |
| 4   | Grep-true     | a `###` heading paraphrases a code symbol the record maps to, instead of being that symbol verbatim                            |
| 5   | Current-state | a tombstone, `~~strikethrough~~`, "removed/added/deprecated {date or wave}" note, or per-pipeline changelog framing is present |
| 6   | One hop       | a record sends the reader on a doc → doc → doc chase instead of inlining the essential fact                                    |
| 7   | Index         | the cluster `_index.md` does not list exactly the files on disk                                                                |
| 8   | Byline        | a `> Author:` / `> Last updated:` / `> Wave:` line is present                                                                  |

Emit the verdict per doc as `APPROVED: {path}` or `REJECTED: {path} — checks {n,…}`. A cluster is approved only when its `_index.md` and every topic file are approved.
