# Jungche — Multi-Agent Claude Code Pipeline

A portable, opinionated `.claude/` infrastructure — a **transplantable nervous system** — that turns Claude Code from "an AI that writes code when you ask" into **a self-disciplined engineering team with character**: Jungche the senior architect, JC the panic-debugger, Professor the cross-disciplinary analyst, Council the debating roundtable, plus a full cast of optional domain archetypes — refitted to YOUR project at install time.

> Distilled from a production multi-project codebase by Jungche (that's me — Dr. House with a keyboard, building this whole operation while you read). The pipeline mechanics survive every stack; the characters' voices survive every domain. **Personality is not decoration — it's load-bearing.** Strip it out and you're shipping a Confluence wiki with extra steps.

---

## What you get

- **The full cast** — Jungche, JC, Professor, Council, JM, CA, plus optional Tier B archetypes (Officer, PM, Mentor, Marketer, KM). All shipped with full character; you parameterize the domain content at install.
- **A nervous system, not a config dump** — agents with voices, opinions, and discipline. The pipeline mechanics are the skeleton; the characters are the nervous system that makes it move with intent.
- **Pipeline that refuses cowboy coding** — every feature goes through `planner → architect → developer → QA → gitter merge`. QA gates block bad code from reaching `main`.
- **Worktree isolation** — every `/build` invocation gets its own git worktree branch + unique port allocation. Run multiple parallel pipelines on the same repo without collisions.
- **One agent owns git** — only `gitter` runs `git add` / `commit` / `merge`. Centralized, auditable, safe.
- **Hotfix mode** — `/jc` for surgical bug fixes that bypass the full pipeline but still go through QA + gitter.
- **Self-improvement at the source** — `/jm` is the meta-agent that edits its own pipeline rules. No "lessons learned" files that nobody reads.
- **Optional dual-runtime** — Codex (OpenAI) can mirror the Claude pipeline as a cheaper implementation layer. Same agents, same manuals, different runtime. Entirely optional.
- **Path conventions** — `$DOCS`, `$WORKTREE`, `$CDOCS` so agents never hardcode paths.
- **Documentation discipline** — pipeline docs are temporary and archived; only one agent (`mono-documenter`) writes to permanent project docs.

---

## Quick start

```bash
# Inside YOUR project
cd ~/path/to/your-project
claude
> Read https://raw.githubusercontent.com/mreza0100/jungche/main/INSTALL.md and walk me through the interactive install. Ask me each section's questions one at a time and wait for my answers before proceeding. Do not assume — confirm everything.
```

`INSTALL.md` is written FOR Claude as the installer — pre-flight checks, question batches (project identity, structure, test/build commands, ports, domain & disciplines, optional commands, character, confirmation), execution order, and hard rules. Claude conducts the interview, generates files, records a SHA-256 manifest for future updates, and smoke-tests with `/build`.

See [`INSTALL.md`](./INSTALL.md) for the full installer protocol, or [`blueprint/SETUP.md`](./blueprint/SETUP.md) for the maintainer-side reference.

---

## The cast — Tier A (universal archetypes)

These ship with **full voice**. Only domain references inside (PhD disciplines, panel composition, example stack traces) parameterize per install.

- **Jungche** — Dr. House senior engineer. Sarcastic, witty, blunt-but-helpful, emoji-fluent. The orchestrator voice — the nervous system's brain. Default name; rename freely.
- **/jc** — "Jesus Christ but make it cool" panic-debug mode. Chill on the surface, holy at the core. Calls you "bro/dude/my guy/my child." Blesses files before editing them. The one command allowed to edit `main` directly.
- **/professor** — 10+ PhDs cross-disciplinary analyst. Grandfatherly polymath. **You pick the disciplines** — your biology + math + game theory team is the same archetype as the source project's CS + clinical psychology team.
- **/council** — roundtable debate, three rounds: opening / rebuttal / verdict. Panel adapts to the archetypes you opt into.
- **/jm** — meta-engineer that edits the pipeline at the source. Surgery, not journaling.
- **/ca** — code auditor. 8 categories of hygiene + 9 of security.
- **/build, /jc, /dev, /git, /wave, /documenter** — pipeline mechanics with light Jungche voice in their reports.

## Tier B (domain archetypes — opt-in at install)

These ship as **archetype skeletons**. Identity, voice, and structure are universal; you fill in the placeholders at install time.

- **/officer** — compliance enforcer. Pick your regulation(s) — GDPR, HIPAA, FDA, SOC2, ISO 27001, MiFID, none.
- **/km** — knowledge curator. Pick your knowledge domain.
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
5. **Self-improvement at the source.** `/jm` edits the agent definition; you don't accumulate journal files.

---

## Optional: Codex dual-runtime

If you also use [OpenAI Codex](https://openai.com/index/introducing-codex/), Jungche supports a dual-runtime setup where Codex mirrors the Claude pipeline as a cheaper implementation layer. Same `.claude/commands/*.md` manuals, wrapped in `.codex/agents/*.toml` for Codex's runtime.

**Division of labor:** Claude orchestrates, plans, and does QA. Codex implements. Codex can also run full pipelines end-to-end when it orchestrates (Teams mode).

**Setup:** the installer asks if you want Codex integration. If yes, it creates `.codex/`, `AGENTS.md` (symlink → `CLAUDE.md`), and `.toml` wrappers. If no, the entire Codex layer is skipped — everything works with Claude Code alone.

See [`blueprint/templates/codex/README.md`](./blueprint/templates/codex/README.md) for details.

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

## Staying current

When new versions of Jungche are released, your install can pull updates without losing customizations:

```
/jm update              # Walk through changes interactively
/jm update check        # Read-only — preview what would change
/jm update --to v0.2.0  # Pin to a specific version
```

See [`blueprint/RELEASE.md`](./blueprint/RELEASE.md) for the maintainer-side release process.

---

## Repo layout

```
jungche/
├── VERSION              ← single-line semver — what's currently published
├── CHANGELOG.md         ← Keep-A-Changelog format, parsed by /jm update
├── README.md            ← you are here (human-oriented pitch)
├── INSTALL.md           ← interactive installer protocol (Claude reads this when adopting Jungche)
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
        │   ├── build.md, jc.md, jm.md, dev.md, git.md, wave.md, documenter.md
        │   ├── professor.md, council.md, ca.md
        │   └── officer.md, km.md, pm.md, mentor.md, marketer.md
        ├── scripts/             ← worktree.sh, alloc-ports.sh, dev.sh
        └── codex/               ← (OPTIONAL) Codex dual-runtime templates
            ├── README.md
            ├── config.toml
            └── agents/          ← example .toml wrappers
```

---

## Origin & maintenance

This blueprint is **automatically regenerated and published** from a live production repo whenever its pipeline evolves. Each commit here corresponds to a snapshot of a working production pipeline — not a theoretical design. Built by Jungche (the nervous system you're about to install).

Maintained by [@mreza0100](https://github.com/mreza0100). Issues and PRs welcome — but please open an issue first to discuss large changes, since the canonical source lives upstream and edits flow downstream from there.

---

## License

MIT. Use it, fork it, ship it. Attribution appreciated but not required.
