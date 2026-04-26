# Claude Pipeline Blueprint

A portable, opinionated multi-agent development pipeline for Claude Code. **Technology-agnostic by design** — adopt it in any codebase, single-project or multi-project, regardless of language, framework, or runtime.

This is the brain behind the brain: the pipeline gives you the *discipline*, you bring the stack.

---

## What this gives you

A complete `.claude/` infrastructure that turns Claude Code from "an AI that writes code when you ask" into **a self-disciplined engineering team** with:

- **Worktree isolation** — every feature gets its own git worktree branch + a unique port allocation. Multiple parallel pipelines on the same repo without collisions or "did I commit on the wrong branch?" moments.
- **A pipeline that refuses cowboy coding** — `planner → architect → developer → QA → merge`. QA gates block bad code from reaching `main`. Only one agent (`gitter`) touches git.
- **Self-improvement at the source** — a meta-agent (`/ccm`) edits the pipeline rules where they live instead of accumulating "lessons learned" files nobody reads.
- **Hotfix mode** — `/jc` lets you bypass the full pipeline for surgical bug fixes, but still routes through tests + gitter.
- **Path conventions that scale** — `$DOCS`, `$WORKTREE`, `$CDOCS` variables so agents never hardcode paths. Rename a directory once, every agent follows.
- **Documentation discipline** — pipeline docs are temporary and archived; only one agent writes to permanent project docs.

---

## When to use it

✅ **Good fit:**
- Multi-project monorepos where features cross boundaries
- Single project with complex pipelines (planning → impl → QA → merge worth modeling)
- Team or solo dev who keeps losing work to half-finished branches and forgotten state
- Project where "what was decided and why" matters as much as the code

⚠️ **Overkill for:**
- A 200-line script
- Throwaway prototypes
- Anything where you genuinely don't care if `main` breaks

---

## What's in the box

```
blueprint/
├── README.md              ← you are here
├── BLUEPRINT.md           ← philosophy, structure, design decisions
├── SETUP.md               ← step-by-step install for your project
├── ADAPTATION.md          ← how to fit the discipline to *your* stack
└── templates/
    ├── CLAUDE.md          ← root project rules (the contract)
    ├── agents/            ← agent definitions (planner, architect, developer, qa, gitter, documenter)
    ├── commands/          ← /build, /jc, /ccm, /dev
    └── scripts/           ← worktree.sh, alloc-ports.sh, dev.sh
```

---

## Quick start

1. Read `BLUEPRINT.md` to understand what you're agreeing to.
2. Follow `SETUP.md` to copy the scaffolding into your repo.
3. Read `ADAPTATION.md` to fit the templates to your stack.
4. Run `/build my-first-feature` and watch the system work. The first run reveals whatever you missed.

---

## A note on technology

You will not find any specific language, framework, package manager, database, or cloud provider mentioned in these templates — and that is the point. The pipeline is the *brain behind the brain*: the discipline survives every stack. You fill in your test command, your build command, your project layout, your conventions. The pipeline doesn't care whether you're shipping a compiler, a mobile app, a config repo, a game, or an embedded firmware project.

If you find a tech-specific assumption leaking through, that's a bug — open an issue.

---

## Optional: character

The pipeline works equally well with or without a defined personality for the assistant. Templates leave a clearly-marked optional section in the root `CLAUDE.md` where you can either define a character (tone, sarcasm level, what NOT to do) or delete the block entirely. Mechanics first; personality is taste.
