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

- **KEEP-LOCAL: the Codex execpolicy git lock is removed.** Gitter remains preferred; when it is
  unavailable, the active main Codex chat may perform scoped Git writes after explicit in-turn user
  authorization. Subagents stay read-only, and the publication boundary, leak gate, scoped-commit
  discipline, banned commands, and pre-push hook remain unchanged. The blueprint keeps its
  gitter-only prose law and commented-out lock because this fallback is specific to this repo.

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
  would both pin one machine and fail `scripts/leak-check.sh`. For the same reason the
  templates address "the user", never a real name — the gate matches the name case-insensitively.

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

- **KEEP-LOCAL: `/wave:ccc` installed, rewired to the on-main + fence pipeline.** The blueprint twin
  commands a worktree train (`/p:tokens` token probe, worktree-hygiene law, ONE WAVE = ONE WORKTREE
  = ONE MERGE) — none of which exist here. This install's rewire: ledger truth checks shas on
  `main` and names uncommitted done-work as a finding; hygiene checks stray dirs, a dirty `main`,
  and the fence worktree under `.worktrees/{train}/`; suite evidence reads `.claude/scripts/dev.sh
  test` logs, fenced code waves through `dev.sh iso` logs opening with their fence proof line;
  token burn reads seat statuslines from `/chat:capture`; structure drift adds the gitter-only
  git-write law and the fence close order. The CCC identity itself (standing command seat replacing
  the one-shot sentinel) ships upstream via `release.md` — this entry records only the rewire.

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

- **KEEP-LOCAL: `/p:tokens` installed from `blueprint/commands/p/tokens/` into `.claude/commands/p/tokens/`**
  (SKILL.md + README.md + token-ledger.mjs, 2026-08-21). Verbatim except the three Codex
  `PRICING` rows: `{CODEX_MODEL_FRONTIER}` → `gpt-5.6-sol`, `{CODEX_MODEL_SPEC}` →
  `gpt-5.6-luna`; the collector row is dropped because this host maps collector to the spec
  model (`build-codex.mjs` MODEL_MAP) and a duplicate substring row can never match. The
  `refresh-map.json` entries for these templates keep pointing at the source project's copies.

- **KEEP-LOCAL: `/p:rnd` + `/p:360` installed from `blueprint/commands/p/` into `.claude/commands/p/`**
  (2026-08-21). Value swaps only (sandbox path kept at `.professor/RND/{goal}` by the user's ruling); `/jc` · `/wave:builder` → `/wave:live`; `/km` + `knowledge/` → `/pcm` +
  `.claude/`; `{AI_PROJECT}` chain clause → the wave-walker engine's prompt modules + dist build;
  360's `{USER_NOUN} vs {SUBJECT_NOUN}` → `adopter vs maintainer`. 360 ships because rnd.md's
  blind-spot sweep points at `.claude/commands/p/360.md`.

- **KEEP-LOCAL: `/pcm` § Special Operations names `pfm codex build .` as the agent-TOML compiler.** This
  repo's Codex mirror has one writer, the Go `pfm codex build`; the repo-local `build-codex.mjs` is
  gone, so the "New agent" step and the scripts roster (a duplicated `codex-sync.sh`) now name the
  live compiler. `blueprint/commands/pcm.md` keeps `build-codex.mjs` — adopters ship the JS compiler.
