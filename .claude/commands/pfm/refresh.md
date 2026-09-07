---
name: pfm:refresh
description: Re-derive templates/** from a live source project, hunk by hunk — classifies every diff hunk SYNC (framework change, belongs upstream) vs LOCAL (project-specific, never ships) vs TOKEN (value the template parameterizes), then applies the SYNC set in reviewed sonnet batches. Modes: `rulings` settles MISSING-SOURCE / UNMAPPED-LIVE map entries and unblocks the scan gate; `--only {glob}` scopes to matching templates; `--batch N` sets templates per wave (default 4); `--dry-run` classifies and reports without writing. Invoked by /pfm:refresh or "refresh the blueprint from {project}"; the refresh LAW is $CDOCS/pfm/$REFS/refresh.md, this command is its execution mechanic. Runs inside /pfm:release step 3.
argument-hint: {live-project-root} [rulings] [--only {glob}] [--batch N] [--dry-run]
disable-model-invocation: true
---

# PFM Refresh — Batched Blueprint Re-derivation

Read `$CDOCS/pfm/$REFS/refresh.md` — the tier table, the preservation list, and the placeholder law it carries are the rules every worker applies. This file owns only HOW the pass runs: cheaply, in reviewed batches.

## The cost law

The pass is diff-driven, never file-driven. Whole-file reads are what make a refresh unaffordable, and a 5,000-line sweep dies of context long before it dies of difficulty.

- The orchestrator reads NO template and NO live source. It reads `refresh-scope.sh` output, `diff -u` hunks, and worker reports.
- A worker reads at most **2 files**: its one template and its one live source. Nothing else — not a sibling template, not the map, not a reference doc beyond what its brief quotes.
- One worker owns exactly one template pair.

## Step 1 — Scope

```bash
bash scripts/refresh-scope.sh scan {live-project-root}
```

`CHANGED` is the work list. `UNCHANGED` is a mechanical untouched-proof — skipped, never re-read. A scan that fails to RUN is a failed look, not an empty one: stop and report it.

Add any template named by a bullet the `refresh-scope.sh ledgers` sweep collected, from any ledger — a linked project's bullet earns its template a re-derivation exactly as this repo's does.

`MISSING-SOURCE` exits 3 and blocks the pass → Step 2. `curated` templates have no live source and are never derived here.

## Step 2 — `rulings`

Runs alone as `/pfm:refresh {root} rulings`, and runs first whenever a scan exits 3. Every `MISSING-SOURCE` gets one of three rulings, and each is a judgment the user's blueprint has to live with, so state the evidence for each:

- **REMAP** — the live source moved or was renamed. Point the entry at the successor. Prove the successor is the same file (same role, continuous content), never a same-named coincidence.
- **DELETE** — the live source is gone with no successor and the template ships a dead pattern. Remove the template file AND its map entry, end to end, including any pointer that cited it.
- **CURATED** — the template has legitimately outgrown its live source and is now hand-maintained here (every machine-global template is this by law: `templates/global/**` IS the truth, so a global entry still carrying a live source is a mapping bug). Set `curated: true` and drop the dead `sources` map.

`UNMAPPED-LIVE` gets a mapping or an `ignore_sources` entry, same evidence bar.

Re-baselining around a missing source keeps a zombie template alive forever — never regen to clear one.

Two integrity checks the scan structurally cannot make, because it reads the map's keys rather than the tree:

```bash
# ZOMBIE — map entry whose template file does not ship
jq -r '.templates | keys[]' templates/refresh-map.json | while read -r k; do [ -e "templates/$k" ] || echo "ZOMBIE $k"; done
# ORPHAN — shipped template with no map entry
comm -13 <(jq -r '.templates | keys[]' templates/refresh-map.json | sort) \
         <(cd templates && find . -type f -not -name refresh-map.json | sed 's|^\./||' | sort)
```

## Step 3 — Cascades first, then batch

A per-template worker structurally cannot carry a change that spans templates: it sees one file, so it applies the cascade's local fragment and leaves the blueprint half-migrated — some files on the new contract, one on the old, every check still green. Detect these BEFORE batching and route them out of the per-template lane.

A hunk is a CASCADE when it renames or retires a token, path var, command, agent, artifact, or term that other templates name. Measure the blast radius, closed-world, before ruling it:

```bash
grep -rl '{the symbol}' templates/ | sort        # every template that must move together
```

Each cascade becomes ONE task owning every file in its radius, dispatched alone, verified by the symbol's count reaching zero (or its full replacement) across `templates/`, `docs/`, and every citing pointer. A cascade half-applied is a worse defect than one not started, so a batch worker that meets a cascade hunk reports it and applies nothing.

The reverse holds too: a term this blueprint's other files depend on is not renamed because one live file renamed it. Grep the term before accepting a rename — a live-side rename with citers here is LOCAL until its own cascade task lands.

Order the remaining `CHANGED` list smallest diff first (`diff -u {template} {live} | grep -c '^[+-]'`) so the cheap batches retire early and the expensive ones arrive with the classification pattern already established. Batch at `--batch N` (default 4), one worker per template.

Dispatch each batch as ONE message — every sibling in a wave goes together, and a missing report is a named coverage hole, never a silent one.

Tier and effort per the fleet prompt § Model Selection: **spec-execution (sonnet), effort High**. The work arrives with a spec; the judgment that stays here is which hunks were classified wrong.

### The worker brief

Every dispatch carries all five briefing fields (root `CLAUDE.md` § Subagent dispatch), plus one input the 2-file cap makes it impossible for a worker to fetch:

**Quote every ledger bullet naming this template's mechanism into the brief.** The source project's `.professor/release.md` is where that project ALREADY ruled the change framework-bound — a bullet carrying a `#### → For:` adopter migration line is a declaration of SYNC intent, and a worker that cannot see it will read a generic mechanism as install-specific topology and rule it LOCAL. Grep the Step 2b sweep output for the template's subject before dispatching; a template with no matching bullet is briefed as such, so "no bullet quoted" means the orchestrator looked, not that it skipped.

> Re-derive ONE blueprint template from its live source. Read at most these 2 files: the template `templates/{key}` and the live source `{live-root}/{source}`. Read nothing else.
>
> 1. Stage and run the deterministic pass first: copy the LIVE source to a `tmp/` scratch path, `bash scripts/genericize.sh -i {scratch}`. It applies `scripts/placeholder-map.tsv` longest-search-first. Never hand-substitute a value that map already covers.
> 2. `diff -u templates/{key} {scratch}` — the hunks are the whole job.
> 3. Classify EVERY hunk as exactly one of:
>    - **SYNC** — a framework change: a mechanism, rule, gate, threshold, structure, or correction any adopter of this blueprint would want. Apply it to the template.
>    - **LOCAL** — project-specific: the source project's brand, roster, ports, stack, domain nouns, its own business rules, a fix meaningful only in that repo. Never applied. A LOCAL hunk that carries a value the template already parameterizes is TOKEN instead. A mechanism is not LOCAL merely because the source project is its only instance today — judge the mechanism, and treat the concrete repo, remote, or path it names as the TOKEN half. A brief quoting a ledger bullet for the hunk has already settled it as SYNC.
>    - **TOKEN** — a project value sitting where the template holds a registered placeholder. Apply with the token, never the literal. One canonical token per concept; a concept with no registered token is UNRULED, not an invented one.
>    - **UNRULED** — you cannot tell. Report it verbatim with your reasoning. An unruled hunk is a result; a silently dropped one is a defect.
> 4. Apply only SYNC and TOKEN, surgically, with `Edit`. A template IS the live source file verbatim — same structure, mechanics, character, logic; only project-specific values swap for tokens. Never abstract, skeletonize, or thin prose, and never trim a persona's voice sections.
> 5. Verify: `bash scripts/leak-check.sh --files templates/{key}` and quote its exit status. No machine-absolute path (`/home/…`, `/Users/…`), no brand current or former, no PII.
>
> Return: one line per hunk (`SYNC` / `LOCAL` / `TOKEN` / `UNRULED` + a phrase naming it), the leak-check exit status quoted, and a draft `release.md` bullet for the SYNC set in the ledger's final-bullet shape. Report a tool that would not run as a failure naming the tool — never as a clean result.

## Step 4 — Review, then continue

The orchestrator reviews each batch before the next dispatches. Agent reports are evidence, not truth: read the actual diff, never the worker's claim about it.

```bash
git diff --stat templates/            # scope: only briefed templates moved
git diff templates/{key}              # the hunks that landed
bash scripts/leak-check.sh --files $(git diff --name-only templates/)
```

Reject and re-dispatch on any of: a template touched that no brief named, a LOCAL hunk applied, a literal where a registered token belongs, prose thinned rather than derived, a leak-check the worker did not quote. A worker's UNRULED hunks are ruled HERE, by the orchestrator or by the user — never left in the report.

Reconcile telemetry per batch: workers dispatched vs reports received, and the count appears in the report.

## Step 5 — Close

1. `bash scripts/refresh-scope.sh regen {live-project-root}` — fresh hashes are the next release's baseline. Only after every ruling from Step 2 has landed; regen over an unruled MISSING-SOURCE re-baselines a zombie.
2. Every SYNC set logs its bullet to `.professor/release.md`; a change this repo alone wants logs to `drift.md` (§ Logging in `/pfm`).
3. Report: templates re-derived / skipped-unchanged / ruled, the per-verdict hunk totals, every UNRULED hunk and how it was ruled, workers dispatched vs reports received, leak-check status, and what a reader must verify by hand.

## Rules

- `--dry-run` classifies and reports; it writes no template, no map entry, and no ledger line.
- Never regen hashes for a template a worker did not actually re-derive — the baseline would claim a sync that never happened.
- A worker that reports zero hunks names the command it ran; "found nothing" and "failed to look" are different results and are reported differently.
- The public repo is the stakes: a leaked identifier cannot be unpublished. Leak-check is the backstop, never the plan.
