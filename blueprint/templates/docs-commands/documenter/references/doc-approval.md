# Doc Consolidation Contract — worker card

Worker-facing extract of `.claude/commands/quality/doc.md` (canonical) — edit both together. Every fan-out documenter worker reads this card BEFORE touching any doc; main-loop sessions load the full skill instead.

## Write rules

- Cluster model: a reference doc is a directory — `_index.md` (pointer table `| Topic | File | Covers |`, ≤150 lines, no prose) + self-contained topic files. Route a merge into the topic file whose `_index.md` entry matches; otherwise create one and register it.
- Size: topic file ≤500 lines target, ~80 KB hard cap — on breach, split the largest self-contained section into a sibling and register it. ToC at the top of any file >100 lines.
- Record format: any long free-text field → one `###` section per record (heading = the code symbol verbatim, one-line bold metadata strip, prose below); only short uniform cells → table. Never long prose in a table cell.
- Current-state only: delete removed records — no tombstones, `~~strikethrough~~`, "removed/added in wave-N" notes, or per-pipeline changelog framing. History lives in git.
- Grep-true: every identifier is the exact code/DB/SDL symbol verbatim; when code renames, the doc renames in the same edit. Verify claims against actual code, not the dev report — a renamed or removed symbol the report missed is the #1 drift source.
- One hop: inline the essential fact, never doc → doc → doc. No `> Author:` / `> Last updated:` bylines.

## Boundaries (sacred)

- Write ONLY your scope card's write set — another worker owns every other path; outside writes are a write race.
- NEVER write: `$CDOCS/officer/`, `$CDOCS/mentor/`, `.claude/agents/gitter.md` Living Reference, CLAUDE.md or `.claude/` files, source code, pipeline/temp files (`docs/dev/builds/`, `docs/dev/waves/`), research dirs (`docs/commands/*/research/`, `docs/dev/research/`).
- Leave `$DOCS/` in place — gitter commits and archives it. Run no git.
- Never lose decisions — pipeline architecture/UI/API decisions MUST land in permanent docs. Permanent docs are unnumbered (no number prefixes).
- **Registry duty** — if your merge added, removed, or renamed a doc/cluster, or changed ownership, update the matching row in `documenter.md` § Document Registry — this is separate from your cluster's own `_index.md`.

## Approval gate — run over every doc you touched

A doc is APPROVED only when ALL hold; otherwise REJECTED with the failing checks named — fix and re-check. A REJECTED doc never ships.

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

Emit `APPROVED: {path}` or `REJECTED: {path} — checks {n,…}` per doc.

## Finish

`npx prettier --write --prose-wrap preserve` every file you wrote. Write no other files.
