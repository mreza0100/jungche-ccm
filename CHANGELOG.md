# Changelog

All notable changes to the Jungche blueprint will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

**For adopters:** run `/jm update` in your installed project to apply changes between your local version and the latest release. The update command parses this file to walk you through changes interactively.

---

## How `/jm update` reads this file

Each release section uses categorized headings the update flow understands:

| Heading | Apply how |
|---------|-----------|
| `### Added` | Auto-apply mechanics changes; ask before adding Tier B archetypes |
| `### Changed` | Auto-apply mechanics; show diff + ask for character changes |
| `### Fixed` | Auto-apply (bug fixes don't touch customization) |
| `### Removed` | Walk through interactively — never auto-delete |
| `### Breaking` | **Interactive walkthrough required.** Each change has explicit migration steps. |
| `### Migration` | Step-by-step transformation instructions for adopters |

Bullets MUST follow this shape:

```
- {Tier A|Tier B|Mechanics|Docs|Scripts}: {file path or scope} — {what changed semantically}
```

Optional trailing tags: `(opt-in)` for Tier B additions, `(breaking)` if it requires migration even outside a Breaking section, `(safe-auto)` to mark unconditional auto-apply.

---

## [Unreleased]

*Pending changes for the next release will accumulate here.*
