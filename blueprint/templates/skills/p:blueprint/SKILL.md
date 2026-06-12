---
name: p:blueprint
description: 'Professor blueprint bus — consume and publish framework changes between peer installs. Two subcommands: `update` (pull a newer blueprint release tag from upstream and three-way-merge it with local customizations via manifest replay, honoring drift.md KEEP-LOCAL) and `release` (regenerate the portable blueprint from the live .claude/ via the refresh pass, then version, tag, and push upstream, consuming .professor/release.md). Triggered by `/pcm update`, `/pcm release`, "blueprint update", "blueprint release", "publish the blueprint", or "pull the latest professor blueprint".'
---

# Blueprint — The Professor Framework Bus

Every project carrying the blueprint is a peer on a shared bus: it **consumes** others' improvements (`update`) and **publishes** its own (`release`). There is no privileged "mothership" — this repo included. The blueprint exports a **transplantable nervous system**: same characters, ranks, and swagger, refitted to each adopter's domain at install time. **The character IS the pipeline** — strip it and you ship a Confluence wiki.

Entry points: `/pcm update` and `/pcm release` route here; the real work happens in this skill.

| Subcommand                                  | Action                                                                       |
| ------------------------------------------- | ---------------------------------------------------------------------------- |
| `update`                                    | Full interactive update to latest release tag                                |
| `update check`                              | Read-only — show what would change, no writes                                |
| `update --to vX.Y.Z`                        | Pin to a specific git tag (not necessarily latest)                           |
| `update --force`                            | Re-apply manifest even if version matches (repair mode)                      |
| `update --re-interview N`                   | Re-run interview question N, update manifest, re-derive affected files       |
| `release {patch\|minor\|major} "{summary}"` | Refresh pass, bump VERSION, finalize CHANGELOG, commit + tag + push upstream |

## Constants

- **Public repo:** `{BLUEPRINT_REPO}` (public git host)
- **Local clone:** `{BLUEPRINT_CLONE_PATH}` — the ONLY working copy
- **Blueprint tree:** `{BLUEPRINT_CLONE_PATH}blueprint/`
- **Public README:** `{BLUEPRINT_CLONE_PATH}README.md` — hand-curated, repo root
- **GH user:** `{GH_USER}`

If `{BLUEPRINT_CLONE_PATH}` is missing and the subcommand is `release`, clone it (or create the repo on the host first if it doesn't exist).

## Pre-flight (before either subcommand)

1. `gh auth status` — must be `{GH_USER}` (the `host-gh`/`host-glab` skill marks which CLI bridges this host)
2. `git status` in the project repo — note uncommitted state (don't fail)
3. For `release`: inside `{BLUEPRINT_CLONE_PATH}`, confirm clean or only in-progress refresh edits (bail on unrelated dirty state)

---

## Subcommand: `update`

Pulls changes from the upstream blueprint and merges them with local customizations. The manifest (`.professor/manifest.json`) stores file hashes AND interview answers, enabling replay against new templates. If `.professor/` is absent (this repo seeded the blueprint instead of installing from it), Step 1 bootstraps it.

### Step 1 — Read local state

1. Read `.professor/VERSION` → installed version (e.g., `0.15.0`)
2. Read `.professor/manifest.json` → file hashes + interview answers
3. If either missing → warn, offer bootstrap: compute manifest from current files, ask user for version and interview answers

### Step 2 — Fetch upstream via git tags

```bash
# List all release tags
git ls-remote --tags https://github.com/{BLUEPRINT_REPO}.git 'refs/tags/v*'
```

Determine target:

- Default → latest tag (highest semver)
- `--to vX.Y.Z` → specified tag
- If target ≤ installed → report "up to date" and exit (never downgrade)

Fetch target version into temp:

```bash
git clone --branch v{TARGET} --depth 1 https://github.com/{BLUEPRINT_REPO}.git /tmp/professor-update-{TARGET}
```

### Step 3 — Parse CHANGELOG between versions

Read the per-release files `releases/v*.md` for every version `> {INSTALLED}` and `<= {TARGET}` (`CHANGELOG.md` is just the index — full notes live one-file-per-version in `releases/`). Each file is one release's full notes; group its bullets by heading (Added/Changed/Fixed/Removed/Breaking/Migration).

Parse each bullet:

- Prefix → category (`Tier A:`, `Tier B:`, `Mechanics:`, `Docs:`, `Scripts:`)
- Trailing tags → override (`(safe-auto)`, `(breaking)`, `(opt-in)`)

### Step 4 — Classify bump magnitude

| Bump      | Behavior                                          |
| --------- | ------------------------------------------------- |
| **Patch** | All auto-apply with preview                       |
| **Minor** | Mix of auto + interactive; may add optional files |
| **Major** | Full interactive walkthrough, no silent applies   |

### Step 5 — Three-way hash comparison

**Rebase-first — never overwrite blindly:** always re-hash the on-disk files fresh (never trust the manifest's cached hash — a local edit since the last update must register as "Current"), and re-read `.professor/drift.md`. Every divergence the ledger records is a **forced KEEP LOCAL** that overrides any auto-apply the hash table would otherwise suggest — a ledger-marked customization is never silently overwritten.

Re-apply interview answers from manifest to upstream templates → compute "parameterized upstream" hashes. Then compare three hashes per file:

| Installed (manifest) | Current (on-disk) | Upstream (re-parameterized) | Action                                                    |
| -------------------- | ----------------- | --------------------------- | --------------------------------------------------------- |
| A                    | A                 | A                           | **Skip** — unchanged everywhere                           |
| A                    | A                 | B                           | **Auto-apply** — upstream changed, user hasn't touched    |
| A                    | B                 | A                           | **Keep** — user customized, upstream didn't change        |
| A                    | B                 | C                           | **Conflict** — both changed → show diff, ask user         |
| —                    | —                 | B                           | **New file** — add (auto for mechanics, ask for Tier A/B) |
| A                    | A                 | —                           | **Removed** — interactive walkthrough                     |
| A                    | B                 | —                           | **User customized + removed upstream** — warn, keep       |

If new templates introduce placeholders not in the manifest → flag as `[manual]`, present the new interview question, update manifest before proceeding.

### Step 6 — Present three buckets

**Bucket 1 — Auto-apply** (summary, apply unless user objects): `A→A→B` files, new Tier C / `(safe-auto)` files, `Scripts:`/`Mechanics:` the user hasn't customized.

**Bucket 2 — Review** (show diff, ask per-file): `A→B→C` conflicts, `Tier A:` content changes, new `(opt-in)` Tier B archetypes, entries marked `(breaking)`.

**Bucket 3 — Manual** (interactive walkthrough): new interview questions (new placeholders), structural migrations (renames, moves, deletes), `### Breaking` and `### Migration` entries.

For `update check`: show all three buckets, write nothing.

### Step 7 — Apply accepted changes

1. Write accepted files (overwrite or merge per approval)
2. Create new files in correct locations
3. Handle removals (confirm before delete)
4. Update `.professor/VERSION` → target version
5. Regenerate `.professor/manifest.json`: `version` → target; `updated_at` → ISO 8601 UTC now; `interview` → updated with any new answers from Step 5; `files` → fresh SHA-256 of every Professor-owned file as it now exists on disk
6. Append to `.professor/drift.md` under "## Update history" — version-change row `| {date} | v{OLD} | v{TARGET} | {summary of choices} |`; under "## Post-install customizations" record any files where the user kept their version, new Tier B opt-ins/opt-outs, and changed re-interview answers

### Step 8 — Cleanup and report

```bash
rm -rf /tmp/professor-update-{TARGET}
```

```
Professor updated: v{OLD} → v{TARGET}
Applied: {N} auto · {M} reviewed ({K} kept local) · {P} manual migrations
Manifest regenerated. Version: {TARGET}
Changelog highlights: {key bullets between versions}
```

### Step 8b — Refresh source-fetched skills

The blueprint update covers only blueprint-owned files. For each `sources.json` entry, compare the installed `.claude/skills/{name}/SKILL.md` `version:` frontmatter against the latest tag in its `repo`; when the repo is newer, offer a re-fetch of the skill's files. Never downgrade a skill whose installed version is ahead of its repo — that marks an unreleased local fix pending `release` step 5b.

### Step 9 — Offer to sync upstream

If `.professor/release.md` is non-empty (framework changes are queued), or the update surfaced local improvements worth sharing, ask the founder: **publish via `release`?** A peer both consumes and publishes. Never auto-publish — the founder confirms, since it pushes to a public repo.

> **Source-repo caution:** when this repo also _publishes_ the blueprint (the release refresh pass mines its live `.claude/`), an update that round-trips this repo's own release is a no-op by the regeneration principle — real deltas come from what other peers published. Never overwrite a richer live original with a reconstituted placeholder; that is the `A→B→C` conflict — resolve it by keeping local.

---

## Subcommand: `release {patch|minor|major} "{summary}"`

```pseudo
1. Validate args (bump type + summary required, bail if missing)
   patch = bug fixes/doc tweaks | minor = new archetype/command/step | major = breaking/migration

2. Ensure clone exists + up-to-date:
   if !exists {BLUEPRINT_CLONE_PATH}.git → create repo on host if needed → clone
   else → git fetch origin && git pull --ff-only origin main (STOP if fails)

3. Run the refresh pass — read `references/refresh.md` (in this skill dir) and execute it
   end-to-end: re-derive the blueprint from the live `.claude/` + `CLAUDE.md`, update the
   public README. STOP if it fails.

4. Read VERSION, compute new version

5. Build CHANGELOG bullets from `.professor/release.md` — the pending-sync queue is the source
   of what ships (format: "- {Tier}: {scope} — {semantic change}").
   if release.md empty → prompt maintainer for bullets
   Per-bullet migration sub-headings (#### → For:) required for adopter-side action
   Informational-only bullets marked: **`update`: skip — informational only.**

5b. Source-fetched skill release — for each pending bullet naming a `sources.json` skill, ship
    the substance to the skill's OWN public repo first (the blueprint never vendors it):
    clone/pull the canonical repo → rebase-first against its current state (both-changed is the
    A→B→C conflict — keep the richer, never blast-overwrite) → genericize project identifiers in
    the public copy (brand current AND former, internal role/example names), then sync the live
    `.claude/skills/{name}/` to byte-identical (zero standing drift) → bump the skill's `version:`
    frontmatter (semver by change nature) + repo README version refs → leak-grep the staged diff
    (brand names, founder PII, `/Users/` paths) → commit + annotated tag v{X.Y.Z} + push to the
    skill repo. Then rewrite the professor bullet as a version pointer marked
    **`update`: skip — informational only** with a `#### → For:` re-pull note — update Step 8b
    and fresh installs (sources.json) consume it.

6. Write release notes as a NEW per-release file `{BLUEPRINT_CLONE_PATH}releases/v{NEW_VERSION}.md`
   (title `# v{NEW_VERSION} — {YYYY-MM-DD}` + bullets grouped under
   `## Added/Changed/Fixed/Removed/Breaking/Migration`). Then prepend one line to the
   `## Releases` index in `CHANGELOG.md`: `- [v{NEW_VERSION}](releases/v{NEW_VERSION}.md) — {summary}`.
   CHANGELOG.md stays a slim index — full notes live in `releases/`, one file per version.

6b. Reconcile hand-curated docs against the shipped templates: `README.md` + `blueprint/BLUEPRINT.md`
    cast/command/skill lists must match `templates/`, and version references stay current (prefer
    version-neutral phrasing). The README's universal "any repo / any stack" promise is the CONTRACT —
    keep it; fix drifted templates up to it, never downgrade the README to match drift.

7. echo "{NEW_VERSION}" > {BLUEPRINT_CLONE_PATH}VERSION

8. Commit + tag + push:
   commit: "release: v{NEW_VERSION} — {summary}\nSource: {sha}\nCo-Authored-By: Professor <noreply@anthropic.com>"
   git tag -a "v{NEW_VERSION}" -m "v{NEW_VERSION}"   # annotated — --follow-tags skips lightweight tags
   git push origin main --follow-tags (STOP if fails, NEVER force-push)

9. Clear `.professor/release.md` — its entries shipped in this release; empty the pending list, keep the header.

10. Report: tag URL, commit, source SHA, changelog bullets
```

### Pre-release checklist

- `gh auth status` authenticated as `{GH_USER}`
- Refresh pass succeeded
- `.professor/release.md` non-empty (or maintainer provided bullets)
- No secrets in staged diff
- Staged templates grep clean (0 hits) for the project brand (current AND former name), founder name, and `/Users/` machine paths — the refresh pass swaps the brand for `{PROJECT_NAME}`, so a single leftover is a refresh bug, not an exception
- Every pending `sources.json`-skill bullet shipped via step 5b (skill repo tagged + pushed); the professor diff vendors none of their files
- New version > local version

---

## Hard rules

**NEVER:** push secrets, commit project-specific identifiers (the project's own brand name — current AND former — founder PII, internal URLs, machine-absolute `/Users/` paths), force-push, ship Tier A characters with empty placeholders, strip archetype identity to abstraction, auto-bump README version without re-checking template, stage in `tmp/` or anywhere outside `{BLUEPRINT_CLONE_PATH}`. **Repo is PUBLIC — every push is world-visible.**

## Reporting

Always end with one of:

- `Professor updated: v{OLD} → v{TARGET}. {N} auto · {M} reviewed · {P} manual.`
- `Blueprint released: v{NEW_VERSION}. URL: https://github.com/{BLUEPRINT_REPO}/releases/tag/v{NEW_VERSION}`
