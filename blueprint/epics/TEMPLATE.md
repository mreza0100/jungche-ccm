# Epic templates

An epic is an initiative-level persistent context at `docs/epics/{name}/`. It survives across conversations: a fresh chat says "Load epic {name}" and picks up where the last one left off. Epic files are **current-state consolidations**, never append-only logs — each update rewrites or merges into the relevant section, and git history keeps the superseded versions.

## Structure

```
docs/epics/{name}/
├── manifest.md          ← anchor: frontmatter + narrative + decisions + load index
├── update.md            ← work doc: current state + per-area delivered
├── {topic}.md           ← optional topic files (RND results, RR reports, POC notes), each registered in manifest ## Files
└── archive/             ← superseded material (cold — never auto-loaded)
```

**Load protocol:** read `manifest.md` + `update.md`, then open topic files from `## Files` (fall back to `ls`) only as the task requires. Never read `archive/`.

**Ownership:** the Professor owns the lifecycle and narrative (`## Vision & Scope`, `status:`, topic files, epic creation/deletion). `/documenter` (standalone builds + `/documenter epic`) and `/wave` (waves) consolidate shipped/session work into `update.md` + the manifest's working sections per the Epic consolidation contract in `documenter.md`.

---

## `manifest.md`

```markdown
---
epic: { kebab-case-name }
status: PLANNING | IN_PROGRESS | SHIPPED
created: { YYYY-MM-DD }
updated: { YYYY-MM-DD }
pipelines: []
waves: []
---

# {Epic Name}

## Vision & Scope

{What this initiative is and what "done" means. Professor-owned.}

## Key Decisions

{Each decision with its why, deduped. Sharpen an existing entry over adding a near-duplicate.}

## Progress Log

{Exactly ONE line per milestone — substance lives in update.md and Key Decisions, never here:}

- {YYYY-MM-DD} — {pipeline|wave|session}: {one sentence} ({SHA})

## Discoveries

{Gotchas, failed attempts, surprises learned the hard way. Deduped.}

## Open Questions

{Items awaiting a founder ruling.}

## Files

{The load index — one-line hook per topic file in the epic dir:}

- `{topic}.md` — {what it holds}
```

---

## `update.md`

The epic's work doc — current-state, rewritten/merged each consolidation.

```markdown
# {Epic Name} — Work

## State of work

{REWRITTEN every consolidation: what is live, the exact in-flight position, and ordered
next steps precise enough to execute — paths, commands, expected outcomes.}

## Delivered

### {Feature / area A}

{What exists NOW: behavior, key files/symbols, merge SHAs woven in as facts. When a later
ship supersedes earlier work, rewrite this subsection — replaced designs vanish.}

### {Feature / area B}

{…}
```
