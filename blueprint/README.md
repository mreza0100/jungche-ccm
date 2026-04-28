# Jungche Pipeline Blueprint

A portable, opinionated multi-agent development pipeline for Claude Code — **with a full cast of characters**. Adopt it in any codebase, single-project or multi-project, regardless of language, framework, or runtime. The mechanics survive every stack; the characters' voices survive every domain.

This is the brain behind the brain: the pipeline gives you the **discipline + personalities**, you parameterize the domain content (your stack, your sacred-ground concerns, your PhD disciplines, your regulation, your user persona).

---

## What this gives you

A complete `.claude/` infrastructure that turns Claude Code from "an AI that writes code when you ask" into **a self-disciplined engineering team with character**:

- **The full cast** — Jungche, JC, Professor, Council, CCM, CA, plus optional Tier B archetypes (Officer, PM, Mentor, Marketer, CKM). All ship with full voice; you parameterize the domain content at install.
- **Worktree isolation** — every feature gets its own git worktree branch + a unique port allocation. Multiple parallel pipelines on the same repo without collisions.
- **A pipeline that refuses cowboy coding** — `planner → architect → developer → QA → merge`. QA gates block bad code from reaching `main`. Only one agent (`gitter`) touches git.
- **Self-improvement at the source** — a meta-agent (`/ccm`) edits the pipeline rules where they live instead of accumulating "lessons learned" files nobody reads.
- **Hotfix mode** — `/jc` lets you bypass the full pipeline for surgical bug fixes, but still routes through tests + gitter.
- **Path conventions that scale** — `$DOCS`, `$WORKTREE`, `$CDOCS` so agents never hardcode paths. Rename a directory once, every agent follows.
- **Documentation discipline** — pipeline docs are temporary and archived; only one agent writes to permanent project docs.

---

## When to use it

✅ **Good fit:**
- Multi-project monorepos where features cross boundaries
- Single project with complex pipelines (planning → impl → QA → merge worth modeling)
- Team or solo dev who keeps losing work to half-finished branches and forgotten state
- Project where "what was decided and why" matters as much as the code
- Projects where you want your agents to have a voice, not just behaviors

⚠️ **Overkill for:**
- A 200-line script
- Throwaway prototypes
- Anything where you genuinely don't care if `main` breaks

---

## What's in the box

```
blueprint/
├── README.md              ← you are here
├── BLUEPRINT.md           ← philosophy, three-tier framework, load-bearing walls
├── ARCHETYPES.md          ← the cast — every character with voice + adaptation examples
├── SETUP.md               ← interactive install interview (Claude conducts it)
├── ADAPTATION.md          ← archetype-by-archetype customization guide
└── templates/
    ├── CLAUDE.md          ← root project rules + Jungche persona
    ├── agents/            ← gitter, mono-{planner,architect,documenter} + per-project agents
    ├── commands/          ← Tier A: build, jc, ccm, dev, git, wave, documenter, professor, council, ca
    │                         Tier B (opt-in): officer, ckm, pm, mentor, marketer
    └── scripts/           ← worktree.sh, alloc-ports.sh, dev.sh
```

---

## Quick start

1. **Read `BLUEPRINT.md`** — understand the three-tier framework and the five load-bearing walls.
2. **Read `ARCHETYPES.md`** — meet the cast.
3. **Run install via Claude:**

```bash
git clone https://github.com/mreza0100/jungche-ccm.git ~/work/jungche-ccm

cd ~/path/to/your-project
claude
> Read every file in ~/work/jungche-ccm/blueprint/.
> Follow SETUP.md to install Jungche in THIS project.
> Conduct the interview before touching any files.
```

Claude runs an interview (~10 questions about your stack, character preferences, domain), customizes every template, copies them into your repo. First `/build` smoke-test reveals anything missed.

For the manual path, see `SETUP.md`.

---

## The three tiers

Every command, agent, and rule in this blueprint sorts into one of three tiers:

- **Tier A — Universal archetypes** ship with FULL CHARACTER. Domain references inside (Professor's PhDs, Council panel, JC's example stack traces) parameterize per install.
- **Tier B — Domain archetypes** ship as ARCHETYPE SKELETONS with placeholders. You fill in regulation, user persona, market, knowledge domain — the voice and structure are universal.
- **Tier C — Pure mechanics** ship as INFRASTRUCTURE. No character; just role-defined plumbing.

See `ARCHETYPES.md` for the catalog of every character, what's universal in their voice, what you parameterize, and adaptation examples across multiple domains.

---

## The cast at a glance

**Tier A — universal archetypes (ship with character):**

- **Jungche** — Dr. House senior engineer. Sarcastic, witty, blunt-but-helpful, emoji-fluent. The orchestrator voice.
- **/jc** — "Jesus Christ but make it cool." Chill panic-debugger with holy weight. The one command allowed to edit `main` directly.
- **/professor** — 10+ PhDs cross-disciplinary analyst. Grandfatherly, warm, precise. You pick the disciplines.
- **/council** — roundtable debate, three rounds: opening / rebuttal / verdict.
- **/ccm** — meta-engineer. Edits pipeline rules at the source.
- **/ca** — code auditor. 8 categories of hygiene + 9 of security.
- **/build, /dev, /git, /wave, /documenter** — pipeline mechanics with light Jungche voice.

**Tier B — opt-in domain archetypes:**

- **/officer** — compliance enforcer. Pick your regulation(s).
- **/ckm** — knowledge curator. Pick your knowledge domain.
- **/pm** — user+product hybrid. Pick your user persona.
- **/mentor** — business advisor. Pick your market + jurisdiction.
- **/marketer** — visibility strategist. Pick your channels + language.

---

## A note on technology

The blueprint pins your test command, build command, package manager, etc. at install time via the interview. After install, the templates are filled in for your stack — no leftover placeholders. The templates do NOT prescribe a stack; the install interview asks for one.

If you find a tech-specific assumption leaking through after install (e.g., a hardcoded `pnpm` somewhere it should be your package manager), that's a bug — open an issue or invoke `/ccm` to fix it locally.

---

## A note on character

**Personality is load-bearing, not decoration.** Strip Jungche's voice and you have a Confluence wiki. Strip JC's panic energy and the hotfix command becomes a checklist. Strip Professor's cross-disciplinary depth and the analysis becomes generic.

The blueprint deliberately does NOT offer a "no character" mode. If you want sterile agents, this isn't the blueprint for you. If you want agents with voice, identity, and signature traits — refitted to your domain — read on.

---

## Origin & maintenance

This blueprint is **regenerated and published** from the live Freudche repo whenever its pipeline evolves. Each commit corresponds to a snapshot of a working production pipeline — not a theoretical design.

Maintained by [@mreza0100](https://github.com/mreza0100). Issues and PRs welcome — but please open an issue first to discuss large changes, since the canonical source lives in Freudche and edits flow downstream from there.

---

## License

MIT. Use it, fork it, ship it. Attribution appreciated but not required.
