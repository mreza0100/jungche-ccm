# Drift — this install's local customizations

Customizations of **this repo's own self-install** that must stay local and must NOT be
generalized into `blueprint/**`. `/pcm` appends here; `/pcm:release` never consumes it.

The test: would this make sense in a stranger's repo? If yes it belongs in `release.md` and its
template twin. If it only makes sense because this repo IS the blueprint, it belongs here.

## Update history

| Date | Version | Mode | Notes |
| --- | --- | --- | --- |
| 2026-08-13 | 0.53.0 | install (self-hosted, `minimal` profile) | Source is this working tree at `f07e6c0`, not a downloaded tag. 27 framework files written. Roster: `blueprint/`, `pfm/`, `dreamer/`, `ENGINES/wave-walker/engine/`. |

## Post-install customizations

- **KEEP-LOCAL: the fence replaces "no worktree pipeline" (2026-08-20, user-ordered).** Code waves
  build in `.worktrees/{train}/` through `dev.sh iso` — the `infra/` pfm-dev container (fresh
  machine, worktree mounted; design `docs/dev/isolated-dev-foundation.md`). Markdown-only waves
  stay on `main`. The host mirror (build to `~/.local/bin/pfm` + `pfm install --yes`) runs only
  when a fenced wave fully closes: QA pass → orchestrator review, issues fixed → gitter merge.
  The adopter worktree pipeline (`worktree.sh`, `alloc-ports.sh`, per-project agents) remains
  uninstalled; this fence is the repo's own, container-backed variant.

- **KEEP-LOCAL: no `/pcm:update`.** This repo is upstream. There is no newer tag to replay a
  manifest against, so shipping the command would be a route to nowhere.

- **KEEP-LOCAL: `gitter` is trimmed to COMMIT / PUSH / PULL / TAG.** The SETUP / MERGE /
  WORKTREE-CHECKPOINT / SYNC phases and their `docs/commands/git/references/` cards are absent
  because the pipeline that dispatches them is absent. The Remote Publication Boundary, the banned
  commands, and the scoped-commit discipline are kept verbatim.

- **KEEP-LOCAL: the execpolicy git lock is ENABLED.** `.codex/rules/repo-law.rules` promotes the
  git monopoly from prose to a pin (`git commit` / `git push` / `git tag` / `gh release` forbidden).
  The blueprint ships those rules commented out, and that default is correct for a private repo
  where a mistake is recoverable. This repo is public; a push cannot be unpublished. The rules file
  states the trade it buys.

- **KEEP-LOCAL: `.claude/skills/` is gitignored.** Source-fetched skills (`rr`) are cloned at
  install from their own public repos and never vendored — the `sources.json` law. Each also
  carries its upstream LICENSE naming its author, which this repo's leak gate correctly refuses.

- **KEEP-LOCAL: no markdown formatter hook.** `prettier` is absent on this host, and an
  `npx`-fetching PostToolUse hook is a silent network call in the middle of a turn.

- **KEEP-LOCAL: `blueprint` is a roster entry with mechanical gates instead of a build.**
  `dev.sh verify blueprint` runs the leak gate and the placeholder-registry gate. No other install
  has a project whose "tests" are a publication check.

- **KEEP-LOCAL: the `/wave:*` commands are installed here, rewired to this repo's cast.**
  `blueprint/commands/wave/{refine,live,walker,walker-invariants}.md` are the shipped source and
  keep the worktree pipeline; the installed copies under `.claude/commands/wave/` drop it, because
  this install lands every wave on `main`. The rewiring replaces an absent cast: `/jc` and its
  jc-core card become `dev` → `.claude/scripts/dev.sh` → `qa` → `gitter`; `/documenter` becomes an
  in-pass docs update routed through `/pcm` for guarded files; `Explore` readers become `tracer`;
  `/wave:orchestrator`, `/officer`, `/pm`, `/km`, `architect`, `db-admin-*` and `ui-ux-*` are cut
  entirely rather than left as dangling pointers, which removes refine's merge mode and live's lane
  mode (both orchestrator-invoked, so both had no caller here).

- **KEEP-LOCAL: `walker-invariants.md` § Engine Config carries a REPO-RELATIVE script path.**
  The blueprint template writes a machine-absolute path because an adopter's engine lives in a
  separate clone. This repo IS that clone and carries `engines/wave-walker/engine/dist/` in-tree, so
  the path is `engines/wave-walker/engine/dist/active-workflow.js`. An absolute `/home/...` path here
  would both pin one machine and fail `scripts/leak-check.sh`. For the same reason `{FOUNDER_NAME}`
  resolves to "the founder", never a name — the gate matches the name case-insensitively.

- **KEEP-LOCAL: the `args.project` profile carries no gate keys.** This repo exposes no
  request-authenticated surface, no roles, and no resolvers, so `authDoc`/`roles`/`gateResolverPattern`
  are absent and the engine reports `gates: SKIPPED — no project profile supplied`. That SKIPPED is
  the correct reading here, not a hidden pass; the thread walk carries every wave.

- **KEEP-LOCAL: `dev` and `qa` are global agents at the repo root.** One of each serves all four
  projects, parameterised by project name, rather than the blueprint's per-project `qa-{project}`
  roster — four projects do not justify eight agent files, and the per-project delta already lives
  in each project's own `CLAUDE.md`.

- **KEEP-LOCAL: `/wave:live` drops the `tmp/wave-boundary.lock` test mutex.** It guards a
  single-tenant shared test stack. This repo has none: `pfm`'s jail law gives every test its own
  `TMUX_TMPDIR`, and the npm suites are independent, so the lock would serialise runs that cannot
  collide.

- **KEEP-LOCAL: `/wave:sentinel` installed, rewired to the on-main pipeline.** The blueprint twin
  audits a worktree train (`/p:tokens` token probe, worktree-hygiene law, `{FOUNDER_NAME}`
  prescriptions) — none of which exist here. This install's rewire: ledger truth checks shas on
  `main` and names uncommitted done-work as a finding; hygiene checks stray dirs and a dirty `main`;
  suite evidence reads `.claude/scripts/dev.sh test` logs; token burn reads seat statuslines from
  `/chat:capture`; structure drift adds the gitter-only git-write law; `{FOUNDER_NAME}` resolves to
  "the founder". Blueprint twin unchanged — an install rewire, not an upstream improvement.

- **KEEP-LOCAL: root `CLAUDE.md` rewritten lean — 137 → 118 lines, agent roster removed, `## Repo
  structure` added.** The cast table duplicated the harness registry (`.claude/agents/` descriptions
  are injected every session; `model:` frontmatter already pins each tier), so it died; § Subagent
  dispatch keeps only the briefing contract and dispatch laws, and `pcm.md`'s new-agent step
  retargeted off the dead table. The structure section now tells the truth the old file omitted:
  `agents/` (host-global `tracer`/`frr`, live copies at `~/.claude/agents/`, `.toml` Codex twins)
  and `harvester/` (Python/uv, own `CLAUDE.md`, outside `dev.sh`'s roster) exist. Meta's three-lenses
  bullet folded into § Cross-Disciplinary System Analysis (its canonical, cited home); Meta's
  remaining rules moved into Process. Every behavioral rule, threshold, and sacred-ground law
  survived verbatim in substance. Local file only — `blueprint/CLAUDE.md` is a different document
  and unchanged.
