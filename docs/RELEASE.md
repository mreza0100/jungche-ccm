# RELEASE — How the blueprint ships and how adopters pull it

Two mechanisms ship on every tag: the portable blueprint tree (this repo, at `templates/`) and the
compiled `pfm` CLI binaries (built from `pfm/`, attached to the GitHub Release). Both are versioned
by the same git tag.

---

## Versioning

[Semantic Versioning](https://semver.org/), one `VERSION` file at the repo root as the source of
truth, one annotated git tag `v{MAJOR}.{MINOR}.{PATCH}` per release. Tags are immutable — never
deleted or moved after push.

| Bump | When | Adopter impact |
| --- | --- | --- |
| **PATCH** | Bug fixes, doc tweaks, non-interface mechanic changes | adopters re-derive from the updated clone |
| **MINOR** | New Tier B archetype, new mechanics command, new pipeline step | Mix of auto + interactive; may add optional files |
| **MAJOR** | Breaking rename, removed command, changed core convention | Full interactive walkthrough, no silent applies |

Magnitude for a multi-version update is the **largest single-release bump in the chain**, never the
endpoint semver diff alone — one major release anywhere in the walked range makes the whole update
major.

## Release notes layout

Per-version notes live in `releases/v{X.Y.Z}.md`, one file per version, each titled
`# v{X.Y.Z} — {YYYY-MM-DD}` with bullets grouped under `## Added/Changed/Fixed/Removed/Breaking/Migration`.
`CHANGELOG.md` is a **slim index only** — one line per release (`- [v{X.Y.Z}](releases/v{X.Y.Z}.md) —
{summary}`), prepended on every release. Never write full notes into `CHANGELOG.md` itself.

Bullets carry a category prefix and optional trailing tags, both read at update time:

- Prefix → `Tier A:` / `Tier B:` / `Mechanics:` / `Docs:` / `Scripts:`
- Trailing tag → `(safe-auto)`, `(breaking)`, `(opt-in)`, `(cost)` (env var/hook/permission/model-config
  changes — always routed to manual review regardless of prefix)

## Cutting a release (maintainer) — `/ptm:release {patch|minor|major} "{summary}"`

Run from inside the live source project against the local clone at `{BLUEPRINT_CLONE_PATH}` —
the only working copy — targeting the public repo (`{BLUEPRINT_REPO}`, GH user `{GH_USER}`).

1. **Update-gate.** `baseline-sync.sh` checks the clone isn't behind published tags for reasons other
   than this repo's own prior publish round-trip; a genuine peer-content gap forces updating the install from the clone first.
2. **Sync the clone** — `git fetch && git pull --ff-only origin main` (stop on failure).
3. **Refresh pass**, scoped by `templates/refresh-map.json`: `refresh-scope.sh scan` hashes every live
   source — unchanged sources skip their templates, changed sources (plus anything named in
   `.professor/release.md`) get re-derived, unmapped live files get a mapping ruling, `curated`
   templates are never auto-derived. Then execute `docs/commands/ptm/references/refresh.md` over that
   scope: `scripts/genericize.sh` runs the deterministic placeholder pass first
   (`scripts/placeholder-map.tsv`), hand-judgment covers structure only (roster collapsing, domain
   nouns, persona metaphors). `refresh-scope.sh regen` re-baselines the hashes afterward.
4. **Compute the new version** from `VERSION` + the bump type.
5. **Build changelog bullets** from `.professor/release.md` (already-final, copied verbatim — never
   re-authored). Bullets touching a `sources.json` skill ship to that skill's own public repo first,
   then get rewritten as a version-pointer bullet.
6. **Write `releases/v{X.Y.Z}.md`**, prepend the index line to `CHANGELOG.md`.
7. **Reconcile hand-curated docs** — `README.md` + `BLUEPRINT.md`'s cast/command/skill lists must
   match `templates/`; the README's "any repo, any stack" promise is the contract to keep, never
   downgrade.
8. `echo "{X.Y.Z}" > VERSION`.
9. **Gate, then ship:** `leak-check.sh` clean on the staged diff (brand names current+former, PII,
   machine-absolute home paths, secrets) → commit (`release: v{X.Y.Z} — {summary}`) → annotated tag
   `v{X.Y.Z}` → `git push origin main --follow-tags` (never force-push).
10. Clear `.professor/release.md`'s pending entries (keep the header), then report the tag URL,
    commit, source SHA, and changelog bullets.

**Never:** push secrets or project identifiers (current/former brand, PII, internal URLs,
machine-absolute home paths), force-push, ship a Tier A character with empty placeholders, or
auto-bump the README version without re-checking the templates it describes.

## What the tag push triggers — `.github/workflows/release.yml`

A `v*` tag push (or manual `workflow_dispatch`) runs on GitHub Actions:

1. **Build** — `pfm` for `linux/amd64`, `linux/arm64`, `darwin/arm64`, `darwin/amd64` (Go, `-trimpath`,
   `CGO_ENABLED=0`); each platform binary uploads as its own workflow artifact.
2. **Assemble** — downloads every platform binary, writes `SHA256SUMS`.
3. **Require authored notes** — fails the run if `releases/{tag}.md` doesn't exist; a tag with no
   hand-written release file cannot publish.
4. **Publish the GitHub Release** — attaches the `pfm_*` binaries + `SHA256SUMS`, with
   `releases/{tag}.md` as the release body verbatim (`generate_release_notes: false` — no
   auto-summary, the authored file is the only source of truth for the release body).

The authored-notes gate is why step 6 above (write `releases/v{X.Y.Z}.md` *before* tagging) is not
optional — the tag push fails release assembly without it.

---

## Pulling an update (adopter)

State lives in `.professor/` inside the adopter's project: `VERSION` (installed version),
`manifest.json` (file hashes + interview-answer replay seed), `drift.md` (forced KEEP-LOCAL
customizations), `release.md` (local framework changes queued to publish upstream — swept by the
framework repo's release flow).

Updating is deliberate, by-hand work today: update the clone (`git pull --tags`, or `pfm update`,
which also rebuilds the pfm binary), read `CHANGELOG.md` between the installed version and the new
tag, port what applies, and honor `drift.md`'s KEEP-LOCAL entries — a customized file is never
blindly overwritten. Two standing rules survive from the old protocol: a `GENERATED FILE — DO NOT
EDIT` banner means whole-file rebuild by its stated generator, never a line-merge; a symlink into
the blueprint clone updates through the clone's own `git pull`. Source-fetched skills
(`templates/skills/sources.json`) update from their own repos — compare the installed `version:`
frontmatter against the skill repo's latest tag; never downgrade. A mechanical, reviewed update
transaction (per-file report, nothing silently applied) is queued as the blueprint-compiler train.
