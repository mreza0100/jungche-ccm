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

- **KEEP-LOCAL: the self-hosted manifest and isolated fence are mechanically audited.** The manifest
  follows the live three-project roster, records the complete tracked Professor surface, and omits
  an impossible self-referential source SHA; `dev.sh test blueprint` verifies its version, roster,
  coverage, and hashes. Before Docker mounts the checkout read-only, the fence creates Walker's
  ignored nested volume targets and mounts the active Git common directory read-only, so Docker
  Desktop and linked worktrees run the same gates without weakening source immutability.

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
  token burn reads seat statuslines from `chat_capture`; structure drift adds the gitter-only
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

- **KEEP-LOCAL: `scripts/placeholder-map.tsv` is untracked (leak-stop).** The scrub table necessarily
  holds this repo's real private values (name, email, domain, machine paths) mapped to placeholder
  tokens, and `scripts/leak-check.sh` excludes it from its own scan by design — so a tracked copy is
  a leak the gate can never see. It is now `.gitignore`d and read from disk locally, where the refresh
  pass still finds it. FLAGGED FOR THE MAINTAINER: this stops FUTURE exposure only; the value has been
  public in git history since `9b9663d` — purging the past needs a history rewrite + force-push, a
  destructive decision left to the user.

- **KEEP-LOCAL: the three memory-backup templates are `curated: true` in the public refresh map (leak-stop).**
  `blueprint/refresh-map.json` shipped their source paths verbatim: two under a private `~/work/<repo>/`
  checkout and one under the memory vault whose directory name IS this repo's `{MEMORY_VAULT_DIR}` value —
  the very thing that token exists to hide — all three naming private repos in a public file. The literal
  strings are deliberately NOT repeated here: this ledger is tracked and published too, and a note that
  quotes the paths it removed leaks them a second time. Recover them from the map's history if a hand
  refresh is ever needed (`git log -p -- blueprint/refresh-map.json`). No adopter can resolve those paths,
  so the honest public classification is `curated`. Refresh `blueprint/scripts/cc-memory-consolidate.sh`,
  `cc-memory-wire.sh`, and `memory-sync.sh` by hand from their host originals. TRADEOFF: `refresh-scope.sh`
  no longer raises a drift signal when the host copies change — this note is the only reminder.
  `settings-global.json → ~/.claude/settings.json` stays mapped: that path is generic to every install.

- **KEEP-LOCAL: `leak-check.sh`'s home-path rule is `~/work/[A-Za-z0-9]`, not bare `~/work`.**
  `$HOME/work/{MEMORY_VAULT_DIR}` and `~/work/<project>` are the blueprint's OWN documented defaults
  (`docs/references/memory-backup.md`, `blueprint/scripts/cc-memory-*.sh`) and must pass the gate; a
  concrete directory under `~/work/` names a private repo and must not. Same discriminator as the
  pre-existing `/Users/[A-Za-z0-9]` alternative — a home path leaks only when it names a real directory.
  This ledger is TRACKED: a drift note must describe a leak it fixed, never quote the string verbatim.

- **KEEP-LOCAL: `leak-check.sh --files` fails when it examined NOTHING (2026-08-26).** The files-mode
  loop guarded every path with `[[ -f "$f" ]]` and had no else branch, so a path that was not a regular
  file was skipped in silence and the run still printed `leak-check: clean`, exit 0. "We scanned forty
  files and found no leak" and "we scanned zero files" printed the same word — the coincidence detector
  this repo exists to hunt, sitting inside the publication gate itself. Found the honest way: the gate
  passed a file set that DID contain a leak, because the caller ran it from zsh where an unquoted
  `$changed` does not word-split, so all 69 paths arrived as ONE argument that matched no file. Now a
  non-regular path prints `NOT-SCANNED` by name on stderr, and a run that examined zero of the paths it
  was given fails outright. Absence alone does not fail — a deleted file in a changed-file list is
  ordinary and must not block a commit — but it is never counted as clean either. Red/green proven:
  `--files /nonexistent/xyz.md` was exit 0 "clean" before and is exit 1 after; a real file plus a
  deleted one still exits 0 while naming the unscanned path.

- **KEEP-LOCAL: `.claude/scripts/dev.sh` gate-honesty pass (2026-08-26).** This repo's build/test entry
  point, no blueprint twin (`blueprint/scripts/dev.sh` is the environment manager). Four fixes, each
  red/green-proven against the pre-change script: (1) bare `status` exited 0 while printing `WARN go —
  MISSING`, contradicting its own header contract — a missing `go`/`node`/`npm`/`git` now increments
  FAILURES; (2) `status {project}` ignored its TARGET arg and always reported the full fleet, so
  `status bogus` printed "all steps passed" — it now scopes to that project (new `proj_tools`: a tool
  the scope does not need warns, a tool it does need fails) and an unknown project exits 2 with usage;
  (3) `iso` checked only the `docker` binary, so a dead daemon surfaced as an opaque compose connect
  error AFTER `prepare-fence-mounts.sh` had already run its `mkdir -p` — `docker info` is now probed
  first and named TOOLCHAIN-MISSING; (4) the blueprint token gate demoted unregistered template tokens
  to `warn`, which nobody read and which let 27 tokens sit unruled — it now `fail_step`s, and the
  registry was completed upstream first so the flip lands green rather than permanently red.

- **KEEP-LOCAL: `scripts/leak-check.sh` reports its own broken state (2026-08-26).** The gate that
  guards this repo's publication; no blueprint twin. Files-mode swallowed read errors with `|| true`,
  so a permission-denied or unreadable file printed "leak-check: clean" and exited 0 — identical to a
  genuinely clean scan. A `grep` exit ≥ 2 is now a loud `SCAN-ERROR … treated as FAILURE, never as
  clean` line, red/green-proven against an unreadable file.

- **KEEP-LOCAL: roster and pointer corrections across this repo's own install (2026-08-26).**
  `CLAUDE.md` § Repo structure listed `agents/` as `(tracer, frr)` after `reviewer.md` joined it;
  `.claude/commands/pcm.md` still named the registered agents as "gitter and tracer" when
  `.claude/agents/` holds five, and now points at `ls` instead of a roster that re-rots;
  `.claude/agents/gitter.md` frontmatter claimed to be the ONLY agent allowed to run git writes,
  contradicting the § Process fallback that lets the active main Codex chat write when gitter is
  unavailable — it now reads "the registered Git writer — no other subagent runs git WRITES here";
  `.claude/commands/p/tokens/SKILL.md` pointed at `.claude/workflows/`, a directory this repo deleted,
  and now names the `wave-walker` engine; `.claude/commands/wave/live.md` claimed "this install has no
  worktree pipeline" while `CLAUDE.md` fences code waves — it now states what is actually true, that
  `/wave:live` lands on `main` under `/dev` verification and the isolated fence is for code-wave trains.

- **KEEP-LOCAL: published history rewritten to purge the scrub table (2026-08-26).** `scripts/placeholder-map.tsv`
  was tracked and published from `9b9663d` onward, carrying a real name, email, machine home paths, and
  private brand strings; `leak-check.sh` excludes that exact path from its own scan by design, so every
  pre-push gate reported clean while it sat in the tree. Two steps, both done: it was untracked
  (`git rm --cached`, commit `f96c660`) to stop future publication, then `git filter-repo` stripped it from
  all 357 published commits and the result was force-pushed over 5 branches and 77 tags.
  `origin/main` moved `f96c660` → `08b78b3`; 25 tags (`v0.44.0`–`v0.62.0`) were re-pointed. Verified from a
  FRESH clone, not from the mirror: zero commits and zero of 77 tags carry the file.
  Backup of the pre-rewrite history: `~/professor-backup-f96c660.bundle` (20M, `git bundle verify` clean).
  **The rewrite was done against this repo's own `NEVER force-push` hard rule** (`.claude/commands/pcm/release.md`
  § Hard rules), on the user's explicit, thrice-reaffirmed in-turn override after being shown the full cost.
  The registered `gitter` agent refused the operation categorically and correctly — it also refused to treat a
  relayed quote of user consent as consent, which is the behavior a Git writer should have; the push was run
  from the main loop instead.
  **WHAT IT DID NOT ACHIEVE — do not record this as fully purged:** GitHub's `refs/pull/*/head` refs are
  read-only and rejected the push (`deny updating a hidden ref`, the reason the push exited 1). PR refs **#3,
  #5, and #7 still serve the file** and only GitHub Support can remove them — draft request at
  `tmp/github-support-purge-request.md`. One fork and any existing clone also retain the original history,
  permanently, by the user's accepted ruling ("that's not our responsibility"). The values are a name, an
  email, and paths — unlike a credential they cannot be rotated, so treat them as permanently disclosed and
  design accordingly.
  **Local fallout:** the 21 pre-existing worktree branches still sit on pre-rewrite commits and share no
  ancestry with the new `main`; each needs rebasing onto `08b78b3` before its next merge.
