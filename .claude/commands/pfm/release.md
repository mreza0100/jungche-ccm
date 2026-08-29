---
name: pfm:release
description: Version, tag, and publish this repo — regenerating the portable blueprint from a live source project via the refresh pass when one is named, sweeping every linked project's .professor/release.md, then consuming the collected bullets into the CHANGELOG and a releases/ note. Invoked by /pfm:release, "blueprint release", or "publish the blueprint". Publication is explicit-request-only.
argument-hint: {patch|minor|major} "{summary}" [--from {live-project-root}] [--ledger {root}]…
---

# PTM Release — Publish the Blueprint

**Persona:** Read `.claude/output-styles/dr-house.md` now and adopt it for all responses while this command's work is active.

## Constants

- Public repo: `mreza0100/professor` — **this repo IS the upstream.** There is no separate clone to sync into; the working copy you are in is the one that publishes.
- Blueprint tree: `templates/` · Public README: `README.md` · Release notes: `releases/vX.Y.Z.md` · Version file: `VERSION`
- The **live source project** — the private repo whose `.claude/` the templates are derived from — is NOT this repo. It is named with `--from {path}` and is optional; without it, the refresh pass is skipped (§ Step 3).

## Pre-flight

1. **Publication authority.** This command pushes. It runs only on an explicit in-turn request to release/publish. No authority → stop here and say so.
2. `gh auth status` — must be the repo owner.
3. `git status` — the working tree must be clean or hold only this release's edits. Bail on unrelated dirty state; never sweep it in.
4. `git fetch --tags origin` — the version Step 4 computes must be greater than every published tag, or the tag push collides. Report the current newest tag.

## Steps

1. **Validate args** — bump type + summary required, bail if missing. `patch` = bug fixes / doc tweaks · `minor` = new archetype, command, or step · `major` = breaking change or migration.

2. `git pull --ff-only origin main` — STOP if it fails.

2b. **Ledger sweep — always runs.** `scripts/refresh-scope.sh ledgers . {--from root, if given} {each --ledger root}` enumerates every reachable `.professor/release.md`: each named root plus the sub-projects its own manifest names by role. Report the `swept=… pending=… bullets=… empty=… absent=… unreadable=…` line verbatim — it is the proof of how many ledgers were OPENED, and a sweep of zero must never read like a sweep that found nothing. `LEDGER-UNREADABLE` exits 4 and STOPS the release: a ledger that could not be read is a failed look, not an empty one. Every `LEDGER-PENDING` bullet joins this release; the union is what Step 5 consumes and Step 9 clears.

3. **Refresh pass — only when `--from {live-project-root}` is given.** Without it, say `refresh skipped — no live source named` and go to Step 4; a release of hand-authored blueprint edits is legitimate, a SILENT skip is not.

   a. `scripts/refresh-scope.sh scan {live-project-root}` — hashes every live source in `templates/refresh-map.json`. UNCHANGED hashes are a mechanical untouched-proof; those templates are SKIPPED. Re-derive only CHANGED templates plus any file named by a bullet the Step 2b sweep collected, from ANY ledger — a linked project's bullet earns its template a re-derivation exactly as this repo's does. UNMAPPED-LIVE files get a mapping ruling (map it, or add to `ignore_sources`) before continuing; `curated` templates are hand-maintained, never auto-derived. **If the scan itself fails to run, that is a FAILED SCAN, not an empty one — stop.**

   b. Read `docs/commands/pfm/references/refresh.md` and execute it over that scope: run `scripts/genericize.sh` first on each re-derived template (deterministic placeholder pass from `scripts/placeholder-map.tsv`), then hand-judge structure only. Update the public README.

   c. After template edits land: `scripts/refresh-scope.sh regen {live-project-root}` — fresh hashes are the next release's baseline. STOP if any step fails.

4. **Read `VERSION`, compute the new version.** It must exceed every tag from Pre-flight 4.

5. **Build CHANGELOG bullets from the Step 2b union** — every `LEDGER-PENDING` bullet from every swept ledger, this repo's included. Entries are already final bullets (`- {Tier}: {scope} — {semantic change}` + optional `#### → For:` migration line): copy verbatim, never re-author. Bullets carrying env-var / hook / permission / model-config changes are tagged `(cost)`. Two ledgers describing one change merge into a single bullet. `bullets=0` across the whole sweep → prompt the user rather than inventing any.

5b. **Source-fetched skill release** — for each pending bullet naming a `sources.json` skill, ship the substance to the skill's OWN public repo first (the blueprint never vendors it): clone/pull the canonical repo → rebase-first against its current state (both-changed is the A→B→C conflict — keep the richer, never blast-overwrite) → genericize project identifiers in the public copy → sync the live `.claude/skills/{name}/` to byte-identical (zero standing drift) → bump the skill's `version:` frontmatter + README version refs → leak-grep the staged diff → commit + annotated tag + push to the skill repo. Then rewrite the professor bullet as a version pointer marked **`update`: skip — informational only** with a `#### → For:` re-pull note.

6. **Write release notes** as a NEW file `releases/v{NEW_VERSION}.md` (title `# v{NEW_VERSION} — {YYYY-MM-DD}` + bullets grouped under `## Added/Changed/Fixed/Removed/Breaking/Migration`). Then prepend one line to the `## Releases` index in `CHANGELOG.md`: `- [v{NEW_VERSION}](releases/v{NEW_VERSION}.md) — {summary}`. `CHANGELOG.md` stays a slim index; full notes live in `releases/`, one file per version.

6b. **Reconcile hand-curated docs against the shipped templates:** `README.md` + `docs/BLUEPRINT.md` cast / command / skill lists must match `templates/`, and version references stay current (prefer version-neutral phrasing). The README's universal "any repo / any stack" promise is the CONTRACT — keep it; fix drifted templates up to it, never downgrade the README to match drift.

7. `echo "{NEW_VERSION}" > VERSION`

8. **Gate, then use the authorized Git writer** — route each phase through `/git`; it uses gitter when available and the active main Codex fallback only after explicit current-turn authorization:

   a. `scripts/leak-check.sh --files <every changed file>` — brand current+former, user PII, machine home paths, zero secrets. Report its exit status. A single leftover is a refresh bug, not an exception. The committed `.githooks/pre-push` hook enforces the same gate at push time; do not treat that as a reason to skip this one.

   b. `/git commit` — name the exact paths and message `release: v{NEW_VERSION} — {summary}` with a `Source: {sha}` trailer (the live source SHA when Step 3 ran; omit the trailer when it did not) and `Co-Authored-By: Professor <noreply@anthropic.com>`.

   c. `/git tag v{NEW_VERSION}` — create an annotated tag after re-checking that `VERSION`, `CHANGELOG.md`, and `releases/v{NEW_VERSION}.md` agree.

   d. `/git push origin main --follow-tags`, carrying the user's explicit publish request as its authority. Relay the pre-push hook's output verbatim. STOP if it fails; NEVER force-push.

9. **Clear EVERY ledger Step 2b reported as PENDING** — each one's entries shipped in this release; empty each pending list, keep each header. A ledger left unclear re-ships its bullets in the next release; clearing one this release never opened would delete work that never shipped, so clear exactly the paths the sweep printed.

10. **Report** tag URL, commit SHA, source SHA (or "no refresh"), and the changelog bullets, ending with:
    `Blueprint released: v{NEW_VERSION}. URL: https://github.com/mreza0100/professor/releases/tag/v{NEW_VERSION}`

## Hard rules

**NEVER:** push secrets; commit project-specific identifiers (a private project's brand name — current AND former — user PII, internal URLs, machine-absolute home paths such as `/home/…` and `/Users/…`); force-push; bypass the pre-push hook; ship Tier A characters with empty placeholders; strip an archetype's identity down to abstraction; auto-bump the README version without re-checking the templates; stage anything from `tmp/`. **The repo is PUBLIC — every push is world-visible, and a leak cannot be unpublished.**
