# RELEASE — How updates flow from Freudche to adopters

This blueprint is regenerated and published from the live Freudche repo. This doc explains the release process so adopters can reliably consume updates via `/pcm update`.

---

## Versioning

The blueprint follows [Semantic Versioning](https://semver.org/):

| Bump                            | When                                                                                   | Adopter impact                                                                           |
| ------------------------------- | -------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| **PATCH** (e.g., 1.0.0 → 1.0.1) | Bug fixes, doc clarifications, mechanic tweaks that don't change interfaces            | `/pcm update` auto-applies with diff preview                                             |
| **MINOR** (e.g., 1.0.0 → 1.1.0) | Added Tier B archetype, new mechanics command, new pipeline step, character refinement | `/pcm update` walks through additions; auto-applies mechanics; asks before adding Tier B |
| **MAJOR** (e.g., 1.x.x → 2.0.0) | Breaking rename, removed command, changed core convention                              | `/pcm update` requires explicit consent for each migration step                          |

The version is stored in three places (all must agree):

- `VERSION` at the repo root — single-line semver (source of truth)
- `CHANGELOG.md` — versioned entries with categorized changes
- **Git tag** (`v1.0.0`) on every release — this is what adopters pin to at install and what `/pcm update` fetches

### Git tag convention

Every release creates a tag `v{MAJOR}.{MINOR}.{PATCH}` on the release commit. Tags are **immutable** — never delete or move a tag after push. The `/blueprint release` subcommand handles tag creation automatically.

Adopters install by cloning at a specific tag:

```bash
git clone --branch v0.5.0 https://github.com/mreza0100/professor.git
```

The update flow discovers available versions via:

```bash
git ls-remote --tags https://github.com/mreza0100/professor.git 'refs/tags/v*'
```

Then fetches the target tag into a temp clone for comparison. This means adopters never need to track `main` — they hop between tagged releases.

---

## Adopter version tracking

When a user installs Professor via `SETUP.md`, the install records:

- `.professor/VERSION` — single-line semver matching the blueprint version at install time
- `.professor/manifest.json` — contains three things:
  1. **Version + tag** — which release was installed (`version`, `installed_from_tag`)
  2. **Interview answers** — the full replay seed (project name, character, disciplines, tech stack, Tier B opts, ports). Enables re-parameterizing new upstream templates without re-interviewing.
  3. **File hashes** — SHA-256 of every Professor-owned file post-substitution. The baseline for three-way comparison (installed vs. current-on-disk vs. re-parameterized-upstream). See `templates/commands/pcm.md` § "Update Protocol — Step 5" for the truth table.

The manifest is regenerated after every successful `/pcm update` so the new on-disk state becomes the next baseline.

### The update flow

When the user runs `/pcm update`:

1. Reads `.professor/VERSION` and `.professor/manifest.json`
2. Fetches available git tags via `git ls-remote --tags`
3. Determines target version (latest tag by default, or `--to vX.Y.Z`)
4. Clones the target tag: `git clone --branch v{TARGET} --depth 1`
5. Reads `CHANGELOG.md` entries between installed and target versions
6. **Replays interview answers** from manifest against new templates → computes re-parameterized hashes
7. **Three-way hash comparison** per file → classifies into auto-apply / review / manual buckets
8. Presents buckets to user, applies accepted changes
9. Regenerates manifest (new version, updated interview answers if new questions, fresh file hashes)

---

## Change categories `/pcm update` understands

Bullets in `CHANGELOG.md` follow this shape:

```
- {Tier A|Tier B|Mechanics|Docs|Scripts}: {file path or scope} — {what changed semantically}
```

The update flow uses the prefix to decide how to apply:

| Prefix                       | Default apply mode                                        | User opt-out                             |
| ---------------------------- | --------------------------------------------------------- | ---------------------------------------- |
| `Tier A:` (character)        | Show diff, ask confirmation                               | User can keep their version              |
| `Tier B:` (opt-in archetype) | Ask if user wants to opt in; if yes, run interview subset | User can skip                            |
| `Mechanics:`                 | Auto-apply with diff preview                              | User can review before commit            |
| `Docs:`                      | Auto-apply                                                | Always reviewable                        |
| `Scripts:`                   | Auto-apply unless user customized                         | If user changed the script, manual merge |

Optional trailing tags refine behavior:

- `(opt-in)` — Tier B additions; requires explicit yes
- `(breaking)` — overrides the default; always interactive
- `(safe-auto)` — overrides; auto-apply unconditionally

---

## How to release a new version (maintainer)

Done from inside the Freudche repo via the `/blueprint release` subcommand:

```
/blueprint release {patch|minor|major} "{summary}"
```

What it does:

1. **Refresh** — runs `/blueprint refresh` to mirror current source-project state to the local professor clone's `blueprint/` directory
2. **Bump VERSION** — increments according to the bump type
3. **Update CHANGELOG.md** — moves `[Unreleased]` content into a new dated `[x.y.z]` section, prepends a fresh `[Unreleased]` skeleton
4. **Prompt for changelog content** — if `[Unreleased]` is empty, asks the maintainer to fill in the categories (Added/Changed/Fixed/Removed/Breaking)
5. **Commit** — single commit at the blueprint repo with message `release: vx.y.z — {summary}`
6. **Tag** — `git tag vx.y.z`
7. **Push** — `git push origin main --follow-tags`
8. **Report** — public URL of the release tag

If a maintainer wants to add changelog entries between releases (without bumping), they edit the `[Unreleased]` section directly. Those entries get bundled into the next release.

---

## Pre-release checklist

Before running `/blueprint release`:

- [ ] All Freudche changes that should be in the release are merged to main
- [ ] `/blueprint status` shows no unexpected drift
- [ ] CHANGELOG `[Unreleased]` accurately reflects what's shipping (or you'll be prompted to fill it in during release)
- [ ] No secrets staged in the blueprint clone
- [ ] No Freudche-specific identifiers leaked into templates (smell-test in `BLUEPRINT.md`: would a neuropsych team see themselves?)

---

## What `/pcm update` does NOT do

- It does NOT touch `.claude/settings.json` — that's hand-curated per project
- It does NOT touch `CLAUDE.md` Professor persona section without explicit confirmation — your character may have drifted
- It does NOT touch any file under `docs/commands/{cmd}/` — those are command-owned content, not blueprint templates
- It does NOT auto-apply MAJOR version migrations — those always require explicit consent per step
- It does NOT downgrade — if your local `.professor/VERSION` is somehow ahead, it reports and asks
