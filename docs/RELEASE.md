# RELEASE — How the blueprint ships and how adopters pull it

Two mechanisms ship on every tag: the portable blueprint tree (this repo, at `blueprint/`) and the
compiled `pfm` CLI binaries (built from `pfm/`, attached to the GitHub Release). Both are versioned
by the same git tag.

---

## Versioning

[Semantic Versioning](https://semver.org/), one `VERSION` file at the repo root as the source of
truth, one annotated git tag `v{MAJOR}.{MINOR}.{PATCH}` per release. Tags are immutable — never
deleted or moved after push.

| Bump | When | Adopter impact |
| --- | --- | --- |
| **PATCH** | Bug fixes, doc tweaks, non-interface mechanic changes | `/pcm:update` auto-applies with a diff preview |
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

Bullets carry a category prefix and optional trailing tags, both read by `/pcm:update`:

- Prefix → `Tier A:` / `Tier B:` / `Mechanics:` / `Docs:` / `Scripts:`
- Trailing tag → `(safe-auto)`, `(breaking)`, `(opt-in)`, `(cost)` (env var/hook/permission/model-config
  changes — always routed to manual review regardless of prefix)

## Cutting a release (maintainer) — `/pcm:release {patch|minor|major} "{summary}"`

Run from inside the live source project against the local clone at `{BLUEPRINT_CLONE_PATH}` —
the only working copy — targeting the public repo (`{BLUEPRINT_REPO}`, GH user `{GH_USER}`).

1. **Update-gate.** `baseline-sync.sh` checks the clone isn't behind published tags for reasons other
   than this repo's own prior publish round-trip; a genuine peer-content gap forces `/pcm:update` first.
2. **Sync the clone** — `git fetch && git pull --ff-only origin main` (stop on failure).
3. **Refresh pass**, scoped by `blueprint/refresh-map.json`: `refresh-scope.sh scan` hashes every live
   source — unchanged sources skip their templates, changed sources (plus anything named in
   `.professor/release.md`) get re-derived, unmapped live files get a mapping ruling, `curated`
   templates are never auto-derived. Then execute `docs/commands/pcm/references/refresh.md` over that
   scope: `scripts/genericize.sh` runs the deterministic placeholder pass first
   (`scripts/placeholder-map.tsv`), hand-judgment covers structure only (roster collapsing, domain
   nouns, persona metaphors). `refresh-scope.sh regen` re-baselines the hashes afterward.
4. **Compute the new version** from `VERSION` + the bump type.
5. **Build changelog bullets** from `.professor/release.md` (already-final, copied verbatim — never
   re-authored). Bullets touching a `sources.json` skill ship to that skill's own public repo first,
   then get rewritten as a version-pointer bullet.
6. **Write `releases/v{X.Y.Z}.md`**, prepend the index line to `CHANGELOG.md`.
7. **Reconcile hand-curated docs** — `README.md` + `BLUEPRINT.md`'s cast/command/skill lists must
   match `blueprint/`; the README's "any repo, any stack" promise is the contract to keep, never
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

## Pulling an update (adopter) — `/pcm:update [check | --to vX.Y.Z | --force | --re-interview N]`

State lives in `.professor/` inside the adopter's project: `VERSION` (installed version),
`manifest.json` (file hashes + interview-answer replay seed), `drift.md` (forced KEEP-LOCAL
customizations), `release.md` (local framework changes queued to publish upstream).

1. **Fetch tags** via `git ls-remote --tags`; target = latest by default, a pinned `--to` tag, or the
   installed version again under `--force` (repair mode). Never downgrades.
2. **Clone the target tag** into a temp dir.
3. **Walk the release chain version by version** — for every version `> installed` and `<= target`,
   read that version's `releases/v{X}.md` in full and record a per-version ledger entry (bullets by
   heading, `### Breaking`, `### Migration`, new interview placeholders). The range is never
   flattened into one pool — order matters for the migration-chain replay below.
4. **Migration-chain replay** (runs before the hash comparison) — apply each version's structural
   `### Migration`/`### Breaking` steps (renames, moves, deletes, splits/merges) in order against the
   manifest's file paths and `drift.md`'s KEEP-LOCAL paths, so a customized file follows its rename
   lineage to the correct final path before content is ever compared.
5. **Three-way hash comparison**, per file — installed (manifest) → current (on-disk, always
   re-hashed fresh, never trusted from cache) → upstream (re-parameterized with the manifest's
   interview answers):

   | Pattern | Verdict |
   | --- | --- |
   | `A→A→A` | Skip — unchanged |
   | `A→A→B` | Auto-apply |
   | `A→B→A` | Keep — user customized, upstream didn't move |
   | `A→B→C` | Conflict — show diff, ask |
   | `none→none→B` | New file |
   | `A→A→none` | Removed upstream — interactive |
   | `A→B→none` | User customized + removed upstream — warn, keep |

   A `GENERATED FILE — DO NOT EDIT` banner or a built bundle under `.claude/workflows/` is always a
   whole-file copy or rebuild — never a line-merge. A symlink into the blueprint clone is skipped
   entirely; its update channel is the clone's own `git pull`.
6. **Present three buckets:** Auto-apply (`A→A→B`, new safe-auto Tier C) · Review (conflicts, Tier A
   content changes, new opt-in Tier B, `(breaking)`, and every cost-bearing delta regardless of tag)
   · Manual (new interview questions, structural migrations, walked in version order). `check` mode
   shows all three and writes nothing.
7. **Apply, then regenerate `manifest.json`** (target version, fresh hashes, updated interview
   answers) and append an update-history row + any new customizations to `drift.md`.
8. Clean up the temp clone, report `v{OLD} → v{TARGET}` with counts (auto / reviewed / kept-local /
   manual).
9. **Refresh source-fetched skills** — for each `blueprint/skills/sources.json` entry, compare the
   installed skill's `version:` frontmatter against its own repo's latest tag; offer a re-fetch when
   behind, never downgrade an installed skill ahead of its repo.
10. **Offer to publish** — if `.professor/release.md` has entries, or the update surfaced local
    improvements worth sharing, ask before running `/pcm:release` (never auto-publishes). Every queued
    entry is checked for genericity first — a customized entry moves to `drift.md` instead of counting
    toward the publish offer.

---

## What `/pcm:update` does NOT do

- Touch `.claude/settings.json` — hand-curated per project
- Touch the Tier A persona sections without explicit per-file confirmation
- Auto-apply MAJOR migrations — always explicit consent, walked in version order
- Downgrade — an installed version ahead of the target reports and asks, never rolls back
- Overwrite a richer local original on a self-round-trip update — that's the `A→B→C` conflict path,
  resolved by keeping local
