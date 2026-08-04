---
name: pcm:release
description: Regenerate the portable Professor blueprint from the live .claude/ via the refresh pass, then version, tag, and push it upstream, consuming .professor/release.md. Invoked by /pcm:release, "blueprint release", or "publish the blueprint".
argument-hint: {patch|minor|major} "{summary}"
---

# PCM Release — Publish the Blueprint Upstream

**Persona:** Read `.claude/output-styles/dr-house.md` now and adopt it for all responses while this command's work is active.

## Constants

- Public repo: `{BLUEPRINT_REPO}` (public git host)
- Local clone: `{BLUEPRINT_CLONE_PATH}` — the ONLY working copy
- Blueprint tree: `{BLUEPRINT_CLONE_PATH}blueprint/`
- Public README: `{BLUEPRINT_CLONE_PATH}README.md` — hand-curated, repo root
- GH user: `{GH_USER}`

## Pre-flight

1. `gh auth status` — must be `{GH_USER}` (the host git-host bridge — `/h:gh` for GitHub, `/h:glab` for GitLab — marks which CLI bridges this host; fork+release through it)
2. `git status` in the project repo — note uncommitted state (don't fail)
3. Inside `{BLUEPRINT_CLONE_PATH}`, confirm clean or only in-progress refresh edits (bail on unrelated dirty state)

## Steps

0. Update-gate — release only from current. Run `.claude/scripts/baseline-sync.sh`: a version gap that is purely our own self-publish round-trip syncs mechanically; a PEER-CONTENT report (exit 10) → run `/pcm:update` first, then continue. Either way the version Step 4 computes must be greater than every published tag, or the tag push collides.

1. Validate args — bump type + summary required, bail if missing. patch = bug fixes/doc tweaks | minor = new archetype/command/step | major = breaking/migration.

2. Ensure the clone exists and is current: no `{BLUEPRINT_CLONE_PATH}.git` → create the repo on the host if needed → clone; else `git fetch origin && git pull --ff-only origin main` (STOP if it fails).

3. Run the refresh pass, scoped by the refresh map:

   a. `{BLUEPRINT_CLONE_PATH}scripts/refresh-scope.sh scan {project-root}` — hashes every live source in `blueprint/refresh-map.json`; UNCHANGED sources are a mechanical untouched-proof, their templates are SKIPPED. Re-derive only CHANGED templates plus any file a `.professor/release.md` bullet names; UNMAPPED-LIVE files get a mapping ruling (map or `ignore_sources`) before continuing; `curated` templates are hand-maintained, never auto-derived.

   b. Read `docs/commands/pcm/references/refresh.md` and execute it over that scope: run `scripts/genericize.sh` first on each re-derived template (deterministic placeholder pass from `scripts/placeholder-map.tsv`), hand-judge structure only. Update the public README.

   c. After template edits land: `refresh-scope.sh regen {project-root}` — fresh hashes are the next release's baseline. STOP if any step fails.

4. Read VERSION, compute the new version.

5. Build CHANGELOG bullets from `.professor/release.md` — entries are already final bullets (`- {Tier}: {scope} — {semantic change}` + optional `#### → For:` migration line): copy verbatim, never re-author. Bullets carrying env-var/hook/permission/model-config changes are tagged `(cost)` — update Step 6 routes them to Bucket 2 review. Empty `release.md` → prompt the maintainer for bullets. Informational-only bullets marked: **`update`: skip — informational only.**

5b. Source-fetched skill release — for each pending bullet naming a `sources.json` skill, ship the substance to the skill's OWN public repo first (the blueprint never vendors it): clone/pull the canonical repo → rebase-first against its current state (both-changed is the A→B→C conflict — keep the richer, never blast-overwrite) → genericize project identifiers in the public copy (brand current AND former, internal role/example names), then sync the live `.claude/skills/{name}/` to byte-identical (zero standing drift) → bump the skill's `version:` frontmatter (semver by change nature) + repo README version refs → leak-grep the staged diff (brand names, founder PII, machine home paths) → commit + annotated tag v{X.Y.Z} + push to the skill repo. Then rewrite the professor bullet as a version pointer marked **`update`: skip — informational only** with a `#### → For:` re-pull note — update Step 8b and fresh installs (`sources.json`) consume it.

6. Write release notes as a NEW per-release file `{BLUEPRINT_CLONE_PATH}releases/v{NEW_VERSION}.md` (title `# v{NEW_VERSION} — {YYYY-MM-DD}` + bullets grouped under `## Added/Changed/Fixed/Removed/Breaking/Migration`). Then prepend one line to the `## Releases` index in `CHANGELOG.md`: `- [v{NEW_VERSION}](releases/v{NEW_VERSION}.md) — {summary}`. CHANGELOG.md stays a slim index — full notes live in `releases/`, one file per version.

6b. Reconcile hand-curated docs against the shipped templates: `README.md` + `blueprint/BLUEPRINT.md` cast/command/skill lists must match `templates/`, and version references stay current (prefer version-neutral phrasing). The README's universal "any repo / any stack" promise is the CONTRACT — keep it; fix drifted templates up to it, never downgrade the README to match drift.

7. `echo "{NEW_VERSION}" > {BLUEPRINT_CLONE_PATH}VERSION`

8. Gate, then commit + tag + push:

   - `scripts/leak-check.sh` clean on the staged diff (brand current+former, founder PII, machine home paths) and zero secrets — the committed `.githooks/pre-push` hook enforces the same gate at push time; a single leftover is a refresh bug, not an exception
   - commit: `release: v{NEW_VERSION} — {summary}\nSource: {sha}\nCo-Authored-By: Professor <noreply@anthropic.com>`
   - `git tag -a "v{NEW_VERSION}" -m "v{NEW_VERSION}"` — annotated; `--follow-tags` skips lightweight tags
   - `git push origin main --follow-tags` (STOP if it fails, NEVER force-push)

9. Clear `.professor/release.md` — its entries shipped in this release; empty the pending list, keep the header.

10. Report tag URL, commit, source SHA, and changelog bullets, ending with: `Blueprint released: v{NEW_VERSION}. URL: https://github.com/{BLUEPRINT_REPO}/releases/tag/v{NEW_VERSION}`

## Hard rules

**NEVER:** push secrets, commit project-specific identifiers (the project's own brand name — current AND former — founder PII, internal URLs, machine-absolute home paths such as `/home/…` and `/Users/…`), force-push, ship Tier A characters with empty placeholders, strip archetype identity to abstraction, auto-bump README version without re-checking template, stage in `tmp/` or anywhere outside `{BLUEPRINT_CLONE_PATH}`. **Repo is PUBLIC — every push is world-visible.**
