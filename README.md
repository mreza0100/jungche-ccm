# Jungche CCM — Multi-Agent Claude Code Pipeline

A portable, opinionated `.claude/` infrastructure that turns Claude Code from "an AI that writes code when you ask" into **a self-disciplined engineering team** with worktree isolation, QA gates, and self-improvement at the source.

> Distilled from [Freudche](https://github.com/mreza0100), a production AI clinical-documentation assistant. Battle-tested on a 4-language monorepo (TypeScript backend, React Native frontend, Python AI engine, Docker infra). The mechanics survive every stack — the personality is optional.

---

## What you get

- **A pipeline that refuses cowboy coding** — every feature flows through `planner → architect → developer → QA → gitter merge`. QA gates block bad code from reaching `main`.
- **Worktree isolation** — every `/build` invocation gets its own git worktree branch + unique port allocation. Run multiple parallel pipelines on the same repo without collisions.
- **One agent owns git** — only `gitter` runs `git add` / `commit` / `merge`. Centralized, auditable, safe.
- **Hotfix mode** — `/jc` for surgical bug fixes that bypass the full pipeline but still go through QA + gitter.
- **Self-improvement** — `/ccm` is the meta-agent that edits its own pipeline rules at the source. No "lessons learned" files that nobody reads.
- **Path conventions** — `$DOCS`, `$WORKTREE`, `$CDOCS` so agents never hardcode paths. Rename a directory once, every agent follows.
- **Documentation discipline** — pipeline docs are temporary and archived; only one agent (`mono-documenter`) writes to permanent project docs.

---

## Quick start

The blueprint is a set of templates + docs. The fastest install is to let Claude Code do it for you.

```bash
claude
> Clone https://github.com/mreza0100/jungche-ccm/ in /tmp and install here
```

Claude reads [`INSTALL.md`](./INSTALL.md), runs a pre-flight on your repo, then asks you 8 batched question groups (project identity, structure, commands, ports, domain & disciplines, optional commands, character, confirmation). Customizes a `/professor` agent specifically for your domain. Confirms before writing anything. First `/build` smoke-test reveals whatever the installer missed.

For a manual / non-interactive install, see [`blueprint/SETUP.md`](./blueprint/SETUP.md).

---

## What's inside

```
blueprint/
├── README.md            # entry point + when to use
├── BLUEPRINT.md         # philosophy, 7 core principles, architecture diagram
├── SETUP.md             # step-by-step install
├── ADAPTATION.md        # stack-by-stack customization (Node/Python/Rust/Go/mobile/etc.)
└── templates/
    ├── CLAUDE.md        # root project rules with {PLACEHOLDERS}
    ├── agents/          # gitter, mono-{planner,architect,documenter}, planner, architect, developer, qa
    ├── commands/        # /build, /jc, /ccm, /dev
    └── scripts/         # worktree.sh, alloc-ports.sh, dev.sh
```

20 files. ~125 KB. No dependencies, no install script — copy what you need.

---

## The pipeline at a glance

```
/build {feature} →
  child planners (parallel codebase analysis) →
    mono-planner (consolidates → routing decision) →
      gitter SETUP (worktree, branch, ports) →
        mono-architect (cross-project contracts) →
          child architects (parallel — per project, with library research) →
            child developers (parallel — implements code, writes happy-path tests) →
              child QAs (parallel — adversarial tests, runs full suite) →
                fix loop (developer fixes QA bugs until green) →
                  gitter MERGE (squash to main) →
                    POST-MERGE QA (catches merge-introduced bugs) →
                      mono-documenter (updates permanent docs, archives pipeline) →
                        gitter DOCS-COMMIT
```

Hotfix path: `/jc {bug}` → locate → diagnose → fix → test → gitter JC-COMMIT. Same safety, less ceremony.

Meta path: `/ccm {pipeline change}` → edits the agent definitions at the source. Surgery, not journaling.

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

⚠️ **Overkill for:**
- A 200-line script
- Throwaway prototypes
- Anything where you don't care if `main` breaks

---

## Adaptation

Out of the box, the blueprint covers Node (npm/pnpm), Python (uv/poetry), Rust (cargo), Go, Next.js, and Expo/React Native. See [`blueprint/ADAPTATION.md`](./blueprint/ADAPTATION.md) for stack-specific test/lint/build commands and worktree setup snippets.

The templates use `{PLACEHOLDER}` markers wherever stack-specific content goes. Editing for your stack is mostly find-and-replace plus filling in your test commands.

---

## Origin & maintenance

This blueprint is **regenerated and published from the live Freudche repo** whenever its pipeline evolves. Each commit corresponds to a snapshot of a working production pipeline — not a theoretical design.

Maintained by [@mreza0100](https://github.com/mreza0100). Issues and PRs welcome — but please open an issue first to discuss large changes, since the canonical source lives in Freudche and edits flow downstream from there.

---

## License

MIT. Use it, fork it, ship it. Attribution appreciated but not required.
