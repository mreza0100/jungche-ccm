# Jungche CCM — Multi-Agent Claude Code Pipeline

**Current version:** [v0.1.0](https://github.com/mreza0100/jungche-ccm/releases/tag/v0.1.0) · _Pre-stable — public API still settling._ · See [`CHANGELOG.md`](./CHANGELOG.md) for what's new

```
> /ccm update          # in your installed project, when a new version drops
```

A portable, opinionated `.claude/` infrastructure that turns Claude Code from "an AI that writes code when you ask" into **a self-disciplined engineering team with character** — Jungche the senior architect, JC the panic-debugger, Professor the cross-disciplinary analyst, Council the debating roundtable, plus a full cast of optional domain archetypes — refitted to YOUR project at install time.

> Distilled from a production multi-project codebase. The pipeline mechanics survive every stack; the characters' voices survive every domain. **Personality is not decoration — it's load-bearing.** Strip it out and you're shipping a Confluence wiki with extra steps.

---

## What you get

- **The full cast** — Jungche, JC, Professor, Council, CCM, CA, plus optional Tier B archetypes (Officer, PM, Mentor, Marketer, CKM). All shipped with full character; you parameterize the domain content at install.
- **Pipeline that refuses cowboy coding** — every feature goes through `planner → architect → developer → QA → gitter merge`. QA gates block bad code from reaching `main`.
- **Worktree isolation** — every `/build` invocation gets its own git worktree branch + unique port allocation. Run multiple parallel pipelines on the same repo without collisions.
- **One agent owns git** — only `gitter` runs `git add` / `commit` / `merge`. Centralized, auditable, safe.
- **Hotfix mode** — `/jc` for surgical bug fixes that bypass the full pipeline but still go through QA + gitter.
- **Self-improvement at the source** — `/ccm` is the meta-agent that edits its own pipeline rules. No "lessons learned" files that nobody reads.
- **Versioned updates that don't clobber your customizations** — `/ccm update` reads `CHANGELOG.md` between your version and the latest, walks you through changes interactively. Auto-applies mechanics, asks before character refinements, opt-in for new Tier B archetypes, explicit consent per step for breaking migrations. See [§ Staying current](#staying-current).
- **Path conventions** — `$DOCS`, `$WORKTREE`, `$CDOCS` so agents never hardcode paths.
- **Documentation discipline** — pipeline docs are temporary and archived; only one agent (`mono-documenter`) writes to permanent project docs.

---

## Quick start

**One-liner (recommended — machine-to-machine):**

```
# Inside YOUR project
cd ~/path/to/your-project
claude
> Read https://raw.githubusercontent.com/mreza0100/jungche-ccm/main/LLM_INSTALL.md and install Jungche in this project.
```

That single fetch hands Claude a token-dense install briefing — identity, full cast roster, install protocol, lazy-load URL map, verification checklist. Claude lazy-loads templates only as it writes them, runs the 10-question interview from `SETUP.md`, generates files, records a SHA-256 manifest for future updates, and smoke-tests with `/build`. Designed to minimize round-trips and avoid pulling marketing-flavored docs during install.

See [`LLM_INSTALL.md`](./LLM_INSTALL.md) for the full M2M protocol — including efficiency rules, verification gate, and what NOT to fetch.

**Manual path** (clone-and-read, for humans who want to inspect first):

```bash
git clone https://github.com/mreza0100/jungche-ccm.git ~/work/jungche-ccm
cd ~/path/to/your-project
claude
> Read every file in ~/work/jungche-ccm/blueprint/.
> Follow SETUP.md to install Jungche in THIS project.
```

For the full manual interview steps, see [`blueprint/SETUP.md`](./blueprint/SETUP.md).

---

## The cast — Tier A (universal archetypes)

These ship with **full voice**. Only domain references inside (PhD disciplines, panel composition, example stack traces) parameterize per install.

- **Jungche** — Dr. House senior engineer. Sarcastic, witty, blunt-but-helpful, emoji-fluent. The orchestrator voice. Default name; rename freely.
- **/jc** — "Jesus Christ but make it cool" panic-debug mode. Chill on the surface, holy at the core. Calls you "bro/dude/my guy/my child." Blesses files before editing them. The one command allowed to edit `main` directly.
- **/professor** — 10+ PhDs cross-disciplinary analyst. Grandfatherly polymath. **You pick the disciplines** — your biology + math + game theory team is the same archetype as Freudche's CS + clinical psychology team.
- **/council** — roundtable debate, three rounds: opening / rebuttal / verdict. Panel adapts to the archetypes you opt into.
- **/ccm** — meta-engineer that edits the pipeline at the source. Surgery, not journaling.
- **/ca** — code auditor. 8 categories of hygiene + 9 of security.
- **/build, /jc, /dev, /git, /wave, /documenter** — pipeline mechanics with light Jungche voice in their reports.

## Tier B (domain archetypes — opt-in at install)

These ship as **archetype skeletons**. Identity, voice, and structure are universal; you fill in the placeholders at install time.

- **/officer** — compliance enforcer. Pick your regulation(s) — GDPR, HIPAA, FDA, SOC2, ISO 27001, MiFID, none.
- **/ckm** — knowledge curator. Pick your knowledge domain.
- **/pm** — user+product hybrid. Pick your user persona — therapist, neuropsychologist, gamer, surgeon, lawyer, developer.
- **/mentor** — business advisor. Pick your market + jurisdiction.
- **/marketer** — visibility strategist. Pick your channels + language.

---

## The five load-bearing walls

These are the rules that make the system work. Touch anything else, but leave these alone:

1. **Only `gitter` touches git.** Loosening this is how three agents race for the merge and corrupt the index.
2. **QA gates the merge.** Pre-merge AND post-merge. No "I'll fix it later."
3. **Path variables, not hardcoded paths.** Rename once, follow everywhere.
4. **Worktree isolation per pipeline.** Running pipelines on `main` is how you lose work.
5. **Self-improvement at the source.** `/ccm` edits the agent definition; you don't accumulate journal files.

---

## When to use it

✅ **Good fit:**
- Multi-project monorepos where features cross boundaries
- Single project with complex pipelines (planning → impl → QA → merge worth modeling)
- Teams or solo devs who lose work to half-finished branches and forgotten state
- Projects where "what was decided and why" matters as much as the code
- Projects where you want your agents to have a voice, not just behaviors

⚠️ **Overkill for:**
- A 200-line script
- Throwaway prototypes
- Anything where you don't care if `main` breaks

---

## The smell test

Could a neuropsychology lab, a tabletop RPG studio, and a SCADA controls team all read this blueprint and see *their version of Jungche, Professor, and Council* — same archetypes, different content? **If yes, the blueprint is right. If anyone has to delete personality before using it, the blueprint failed.**

The mechanics survive every stack. The voices survive every domain. Personality is not decoration — it's load-bearing.

---

## <a name="staying-current"></a>Staying current

When new versions of Jungche are released, your install can pull updates without losing customizations:

```
/ccm update              # Walk through changes interactively
/ccm update check        # Read-only — preview what would change
/ccm update --to v1.2.0  # Pin to a specific version
/ccm update --tier-b     # Only consider new Tier B archetypes
```

**How it works:**
1. Reads `.claude/JUNGCHE_VERSION` (recorded at install)
2. Fetches the latest blueprint from this repo
3. Reads `CHANGELOG.md` entries between your version and the latest
4. Walks you through changes per category:

| Category | Apply mode |
|----------|-----------|
| **Mechanics** (build step, gitter phase, script fix) | Auto-applies with diff preview |
| **Tier A** (Jungche / JC / Professor / Council voice refinement) | Shows diff, asks confirmation — preserves your customization by default |
| **Tier B** (new domain archetype published) | Asks "want to opt in?" — if yes, runs the SETUP interview subset to fill placeholders |
| **Breaking** (renames, removed commands, convention changes) | Interactive walkthrough, explicit consent per migration step |

5. Updates `.claude/JUNGCHE_VERSION` on success.

**Safety rails:**
- Never overwrites user customizations without explicit consent
- Never auto-applies MAJOR migrations
- Never touches `.claude/settings.json` (hand-curated per project)
- Never touches `docs/commands/{cmd}/` (command-owned content, not blueprint templates)
- Never downgrades

See [`blueprint/RELEASE.md`](./blueprint/RELEASE.md) for the maintainer-side release process and the precise semantics of each change category.

---

## Repo layout

```
jungche-ccm/
├── VERSION              ← single-line semver — what's currently published
├── CHANGELOG.md         ← Keep-A-Changelog format, parsed by /ccm update
├── README.md            ← you are here (human-oriented)
├── LLM_INSTALL.md       ← machine-to-machine install briefing (LLM-oriented)
├── INSTALL.md           ← legacy manual install path (still works)
├── LICENSE              ← MIT
└── blueprint/
    ├── README.md        ← entry point + when to use
    ├── BLUEPRINT.md     ← philosophy, three-tier framework, load-bearing walls
    ├── ARCHETYPES.md    ← the cast — every character with voice + adaptation examples
    ├── SETUP.md         ← interactive install interview Claude conducts
    ├── ADAPTATION.md    ← archetype-by-archetype customization guide
    ├── RELEASE.md       ← versioning + release process for maintainers
    └── templates/
        ├── CLAUDE.md            ← root rules + Jungche persona (non-deletable)
        ├── agents/
        │   ├── gitter.md
        │   ├── mono-{planner,architect,documenter}.md
        │   └── per-project/     ← child agents (planner, architect, developer, qa)
        ├── commands/            ← Tier A always + Tier B opt-in
        │   ├── build.md, jc.md, ccm.md, dev.md, git.md, wave.md, documenter.md
        │   ├── professor.md, council.md, ca.md
        │   └── officer.md, ckm.md, pm.md, mentor.md, marketer.md
        └── scripts/             ← worktree.sh, alloc-ports.sh, dev.sh
```

---

## Origin & maintenance

This blueprint is **automatically regenerated and published** from the live Freudche repo whenever its pipeline evolves. Each commit here corresponds to a snapshot of a working production pipeline — not a theoretical design.

Maintained by [@mreza0100](https://github.com/mreza0100). Issues and PRs welcome — but please open an issue first to discuss large changes, since the canonical source lives in Freudche and edits flow downstream from there.

---

## License

MIT. Use it, fork it, ship it. Attribution appreciated but not required.
