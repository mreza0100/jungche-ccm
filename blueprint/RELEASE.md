# RELEASE — How updates flow from Freudche to adopters

This blueprint is regenerated and published from the live Freudche repo. This doc explains the release process so adopters can reliably consume updates via `/ccm update`.

---

## Versioning

The blueprint follows [Semantic Versioning](https://semver.org/):

| Bump | When | Adopter impact |
|------|------|----------------|
| **PATCH** (e.g., 1.0.0 → 1.0.1) | Bug fixes, doc clarifications, mechanic tweaks that don't change interfaces | `/ccm update` auto-applies with diff preview |
| **MINOR** (e.g., 1.0.0 → 1.1.0) | Added Tier B archetype, new mechanics command, new pipeline step, character refinement | `/ccm update` walks through additions; auto-applies mechanics; asks before adding Tier B |
| **MAJOR** (e.g., 1.x.x → 2.0.0) | Breaking rename, removed command, changed core convention | `/ccm update` requires explicit consent for each migration step |

The version is stored in:
- `VERSION` at the repo root — single-line semver
- `CHANGELOG.md` — versioned entries with categorized changes
- Git tag (`v1.0.0`) on every release

---

## Adopter version tracking

When a user installs Jungche via `SETUP.md`, the install records:

- `.claude/JUNGCHE_VERSION` — single-line semver matching the blueprint version at install time
- `.claude/JUNGCHE_MANIFEST.json` — SHA-256 hash of every Jungche-owned file as installed (post-placeholder-substitution). This is the baseline `/ccm update` uses to detect which files the user has customized vs. left pristine, via a three-way hash compare (installed vs. current-on-disk vs. new-upstream). See `templates/commands/ccm.md` § "Step 5 — Detect what changed" for the truth table. The manifest is regenerated after every successful `/ccm update` so the new on-disk state becomes the next baseline.

When the user runs `/ccm update`, the update flow:

1. Reads `.claude/JUNGCHE_VERSION` (the user's currently-installed version)
2. Fetches the latest from `mreza0100/jungche-ccm`
3. Reads the new blueprint's `VERSION`
4. Reads `CHANGELOG.md` entries between the two versions (using the `## [x.y.z]` headings as boundaries)
5. Walks the user through each change interactively
6. Applies accepted changes
7. Updates `.claude/JUNGCHE_VERSION` to the new version

---

## Change categories `/ccm update` understands

Bullets in `CHANGELOG.md` follow this shape:

```
- {Tier A|Tier B|Mechanics|Docs|Scripts}: {file path or scope} — {what changed semantically}
```

The update flow uses the prefix to decide how to apply:

| Prefix | Default apply mode | User opt-out |
|--------|-------------------|--------------|
| `Tier A:` (character) | Show diff, ask confirmation | User can keep their version |
| `Tier B:` (opt-in archetype) | Ask if user wants to opt in; if yes, run interview subset | User can skip |
| `Mechanics:` | Auto-apply with diff preview | User can review before commit |
| `Docs:` | Auto-apply | Always reviewable |
| `Scripts:` | Auto-apply unless user customized | If user changed the script, manual merge |

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

1. **Refresh** — runs `/blueprint refresh` to mirror current Freudche state to `~/work/jungche-ccm/blueprint/`
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

## What `/ccm update` does NOT do

- It does NOT touch `.claude/settings.json` — that's hand-curated per project
- It does NOT touch `CLAUDE.md` Jungche persona section without explicit confirmation — your character may have drifted
- It does NOT touch any file under `docs/commands/{cmd}/` — those are command-owned content, not blueprint templates
- It does NOT auto-apply MAJOR version migrations — those always require explicit consent per step
- It does NOT downgrade — if your local `JUNGCHE_VERSION` is somehow ahead, it reports and asks
