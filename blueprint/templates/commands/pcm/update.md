---
name: pcm:update
description: Pull a newer Professor blueprint release tag from upstream and three-way-merge it with local customizations via manifest replay, honoring drift.md KEEP-LOCAL. Modes: default (full interactive update to latest), `check` (read-only preview, no writes), `--to vX.Y.Z` (pin a specific tag), `--force` (repair/re-apply current version), `--re-interview N` (redo interview question N). Invoked by /pcm:update, "blueprint update", or "pull the latest professor blueprint".
argument-hint: [check | --to vX.Y.Z | --force | --re-interview N]
---

# PCM Update — Consume the Upstream Blueprint

**Persona:** Read `.claude/output-styles/dr-house.md` now and adopt it for all responses while this command's work is active.

The manifest (`.professor/manifest.json`) stores file hashes and interview answers, enabling replay against new templates. If `.professor/` is absent (this repo seeded the blueprint instead of installing from it), Step 1 bootstraps it.

## Constants

- Public repo: `{BLUEPRINT_REPO}` (public git host)
- Local clone: `{BLUEPRINT_CLONE_PATH}` — the ONLY working copy
- Blueprint tree: `{BLUEPRINT_CLONE_PATH}blueprint/`
- GH user: `{GH_USER}`

## Pre-flight

1. `gh auth status` — must be `{GH_USER}` (the host git-host bridge — `/h:gh` for GitHub, `/h:glab` for GitLab — marks which CLI bridges this host)
2. `git status` in the project repo — note uncommitted state (don't fail)

---

### Step 1 — Read local state

1. Read `.professor/VERSION` → installed version
2. Read `.professor/manifest.json` → file hashes + interview answers
3. If either missing → warn, offer bootstrap: compute the manifest from current files, ask the user for version and interview answers

### Step 2 — Fetch upstream via git tags

```bash
git ls-remote --tags https://github.com/{BLUEPRINT_REPO}.git 'refs/tags/v*'
```

Determine target:

- Default → latest tag (highest semver)
- `--to vX.Y.Z` → that tag
- `--force` → the installed version, re-applied even though it matches (repair mode)
- Otherwise target ≤ installed → report "up to date" and exit (never downgrade)

Fetch target version into temp:

```bash
git clone --branch v{TARGET} --depth 1 https://github.com/{BLUEPRINT_REPO}.git /tmp/professor-update-{TARGET}
```

### Step 3 — Parse CHANGELOG between versions

Read every per-release file `releases/v*.md` for versions `> {INSTALLED}` and `<= {TARGET}` — each holds one release's full notes, `CHANGELOG.md` is only the index. Group its bullets by heading (Added/Changed/Fixed/Removed/Breaking/Migration).

Parse each bullet:

- Prefix → category (`Tier A:`, `Tier B:`, `Mechanics:`, `Docs:`, `Scripts:`)
- Trailing tags → override (`(safe-auto)`, `(breaking)`, `(opt-in)`, `(cost)`)

### Step 4 — Classify bump magnitude

- Patch → all auto-apply with preview
- Minor → mix of auto + interactive; may add optional files
- Major → full interactive walkthrough, no silent applies

### Step 5 — Three-way hash comparison

**Rebase-first — never overwrite blindly:** always re-hash the on-disk files fresh (never trust the manifest's cached hash — a local edit since the last update must register as "Current"), and re-read `.professor/drift.md`. Every divergence the ledger records is a **forced KEEP LOCAL** that overrides any auto-apply the comparison would otherwise suggest — a ledger-marked customization is never silently overwritten.

Re-apply interview answers from the manifest to the upstream templates → compute "parameterized upstream" hashes. Then compare three hashes per file — installed (manifest) → current (on-disk) → upstream (re-parameterized), `none` where the file is absent on that side:

- `A→A→A` — **Skip**: unchanged everywhere
- `A→A→B` — **Auto-apply**: upstream changed, user hasn't touched
- `A→B→A` — **Keep**: user customized, upstream didn't change
- `A→B→C` — **Conflict**: both changed → show diff, ask user
- `none→none→B` — **New file**: add (auto for mechanics, ask for Tier A/B)
- `A→A→none` — **Removed upstream**: interactive walkthrough
- `A→B→none` — **User customized + removed upstream**: warn, keep

If new templates introduce placeholders not in the manifest → flag as `[manual]`, present the new interview question, update the manifest before proceeding. `--re-interview N` re-runs question N here, updates the manifest, and re-derives every file that answer feeds.

### Step 6 — Present three buckets

**Bucket 1 — Auto-apply** (summary, apply unless user objects): `A→A→B` files, new Tier C / `(safe-auto)` files, `Scripts:`/`Mechanics:` the user hasn't customized.

**Bucket 2 — Review** (show diff, ask per-file): `A→B→C` conflicts, `Tier A:` content changes, new `(opt-in)` Tier B archetypes, entries marked `(breaking)`. Cost-bearing deltas — env vars, hooks, permissions, model/config changes (`settings.json` or any file) — ALWAYS land here with an explicit cost/behavior note, regardless of the comparison or `(safe-auto)` tags.

**Bucket 3 — Manual** (interactive walkthrough): new interview questions (new placeholders), structural migrations (renames, moves, deletes), `### Breaking` and `### Migration` entries.

In `check` mode: show all three buckets, write nothing.

### Step 7 — Apply accepted changes

1. Write accepted files (overwrite or merge per approval)
2. Create new files in correct locations
3. Handle removals (confirm before delete)
4. Update `.professor/VERSION` → target version
5. Regenerate `.professor/manifest.json`: `version` → target; `updated_at` → ISO 8601 UTC now; `interview` → updated with any new answers from Step 5; `files` → fresh SHA-256 of every Professor-owned file as it now exists on disk
6. Append to `.professor/drift.md` — under "## Update history" a version-change row `| {date} | v{OLD} | v{TARGET} | {summary of choices} |`; under "## Post-install customizations" every file where the user kept their version, new Tier B opt-ins/opt-outs, and changed re-interview answers

### Step 8 — Cleanup and report

```bash
rm -rf /tmp/professor-update-{TARGET}
```

Report: `v{OLD} → v{TARGET}`; applied `{N}` auto · `{M}` reviewed (`{K}` kept local) · `{P}` manual migrations; manifest regenerated at the target version; the key changelog bullets between versions.

### Step 8b — Refresh source-fetched skills

The blueprint update covers only blueprint-owned files. For each entry in the blueprint tree's `templates/skills/sources.json`, compare the installed `.claude/skills/{name}/SKILL.md` `version:` frontmatter against the latest tag in its `repo`; when the repo is newer, offer a re-fetch of the skill's files. Never downgrade a skill whose installed version is ahead of its repo — that marks an unreleased local fix pending `/pcm:release` step 5b.

### Step 9 — Offer to sync upstream

If `.professor/release.md` is non-empty (framework changes are queued), or the update surfaced local improvements worth sharing, ask the founder: **publish via `/pcm:release`?** Never auto-publish — the founder confirms, since it pushes to a public repo.

**Genericity gate — release.md is a claim, not a verdict.** Before offering publish, measure every queued entry: Professor-generic (any adopter could apply it) or customized to this project (names its projects, domain, infra, paths, or local-only process)? A customized entry is mislabeled drift — move it to `drift.md` and report the move; only generic entries count toward the publish offer.

> **Source-repo caution:** when this repo also _publishes_ the blueprint (the release refresh pass mines its live `.claude/`), an update that round-trips this repo's own release is a no-op — real deltas come from what other peers published. Never overwrite a richer live original with a reconstituted placeholder; that is the `A→B→C` conflict — resolve it by keeping local.
