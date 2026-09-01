# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pfm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- B: pfm + project scaffolding — `pfm init` now scaffolds project templates once with per-file baseline pins; local files own truth, `pfm update check` reports upstream deltas for reviewed hand application, and the dormant Codex override layer is removed.

#### → For: adopters — run `pfm update check`, review and apply wanted template diffs, then pin each accepted local file; do not regenerate the project or replay its install interview.

- A: `templates/CLAUDE.md` — docs-map example gains the truth hierarchy (code truth: grep; schema truth: introspect the live DB) + `docs/runbooks/{project}/`, `docs/features/`, `docs/references/`, `docs/business/` trees; /documenter rule routes officer/mentor/marketer output to `docs/business/{compliance,marketing}/`
- B: officer/mentor/marketer/pm/jc commands — owned-document paths migrate `$CDOCS/{cmd}/$REFS/` → `docs/business/{compliance,marketing}/` and `docs/agents/features/` → `docs/features/`
- A: wave commands — builder reads the spec spine (`spec.md` index + `tasks/T{n}.md` per task, monolithic fallback); refine fans out `tracer` agents and splits architect findings into auto-folded gaps vs user-ruled judgment; live appends lane events to `tmp/wave-sensor/events.log` as a guaranteed-wake fallback; walker follows threads past the diff's edge
- C: scheduler agent — writes the spec spine (task index + `tasks/T{n}.md` opening with `Binds:`); architect agent — new Zero-gap walk section; per-project qa — parallelism is a test-health invariant (`{PARALLEL_FLAG}`; never lower workers to pass)
- C: scripts — `dev.sh` health probe rejects a stale listener answering for a dead service (false-GREEN fix) and `up` exits 1 on any RED; `codex-sync.sh` resolves `pfm` off-PATH; `km-guard.sh` stops false-flagging prose arrows; `alloc-ports.sh` metrics-column semantics; `format-md.sh` doc-tree allow-list + `AGENTS.md`
- C: codex layer — `config.toml` comment documents the generated `mcp_servers` fence (`pfm codex build` owns it); `repo-law.rules` read-only-git list matches canon
- A: `p/rnd.md` — sandbox layout (`{family}/{N}-{goal}`) + self-contained harness sections; Step 0 baseline rule
- A: templates — two-scope split: `templates/global/` (machine-global originals: agents, commands, skills) + `templates/project/` (per-install); `pfm install` symlinks global originals into engine registries (`~/.claude/agents|commands`), Codex `.toml` twins generate at release and link beside them
- A: global agent bench — architect, reviewer (unified: seat-review body + merge-gating `REVIEW.md` contract), rr, scheduler, tracer; project-twin reviewer/tracer removed; gitter ruled PROJECT-scope (each install customizes its own — template at `project/agents/gitter.md`)
- A: wave family — reviewer-gated merges: orchestrator entry-point census → `reviewer` agent → wave dir `REVIEW.md` (`F{n}` open/resolved @sha/waived) → gitter reads it from disk as the merge precondition (`gitter-phase-merge.md` § 1); walker demoted to optional reachability supplement; `/test` references neutralized (install ships no `test.md`); builder briefs carry the spine rules blocks a task's `Binds:` line names; refine cites the migrated officer path (`docs/business/compliance/`)
- B: commands — dead `slow-burn`, `sleep`, `goal-manager`, `animate` removed end to end; `p/tokens`→`/tokens`, `p/rnd`→`/rnd`, `context-meter` flat; `quality:*`, `wave:*`, `h:gh` globalized
- B: `/ptm` → `/pfm` (Professor Framework Management) — command, guard (`pfm-guard.sh`, marker `professor_pfm_active`), reference docs, templates twins (cost: hook path + marker rename)
- C: pfm — installer registers global symlinks via a conflict-preserving classifier (foreign files report `CONFLICT`, never overwritten); limits TUI hides engines without a limits concept, renders the rule above the section header, Codex shows first window only
- C: `scripts/refresh-scope.sh` — an unresolvable `{project:role}` source now classifies MISSING-SOURCE (still blocking, exit 3) instead of killing the whole scan at first hit

#### → For: adopters with existing officer/mentor/marketer documents under `docs/commands/{cmd}/references/` — move them to `docs/business/{compliance,marketing}/`; the cards now read/write there.

#### → For: adopters — rename installs' `/ptm` routes to `/pfm` and re-run `pfm install` to link the global roster; local copies of globalized agents/commands shadow the symlinked originals and should be deleted.
- B: pfm + templates — the fleet prompt layer: `claude.systemPrompt` config (`production` default | `lean` | `professor`) injects `--system-prompt-file` / `CLAUDE_CODE_SIMPLE_SYSTEM_PROMPT=1` on Claude spawn paths (launcher + headless synth; hygiene now strips the lean var); `templates/prompts/` ships `professor.md` plus the captured v2.1.251 harness baseline with its sha256; `pfm doctor` gains a harness-prompt drift check (sink capture, three distinct outcomes: match / DRIFT / FAILED-TO-CHECK); asset↔template byte-equality gated by test
- B: framework — output styles retired end to end: voice lives in the fleet prompt (`templates/prompts/professor.md`); § Model Selection consolidated into that prompt and out of every CLAUDE.md (template keeps a stub pointer; live citers retargeted); Analysis Protocol migrated into root CLAUDE.md § Cross-Disciplinary System Analysis

#### → For: adopters — output-style files are gone: set `claude.systemPrompt = "professor"` in pfm config for the voice + tier law, or stay on `production` and restore your own Model Selection section; delete any local `.claude/output-styles/` copies. (cost: new pfm config key; doctor spawns one local no-token claude capture per run)
- B: pfm — one Claude spawn door: `action.ClaudeSpawn` (purpose: interactive / resume / probe / query) renders every spawn — launcher, headless, chat reload, agent-open, credential probe, busy/resume queries — through the single hygiene strip, autonomy posture, and prompt policy; a reloaded chat now carries the configured system prompt; the `chat swap` alias is retired (dispatch refuses it) and the dead `swap_event` table dropped
- C: pfm — `pfm doctor` spawn-audit: every live Claude seat classified INJECTED / PREDATES-LAYER / VIOLATION from its own /proc argv+environ via the fleet's one engine matcher (version-named binaries recognized); four distinct surfaces — production policy (nothing to audit), empty fleet, CHECK FAILED, unauditable seats counted as warnings
- C: templates — `templates/project/vscode/` launcher snippet retired end to end (README, SETUP step renumber, refresh-map, BLUEPRINT, INSTALL); the installer's own VS Code `PFM` terminal profile is its live successor
- A: `/rnd:hammer` + `/rnd:referee` — the two-layer RND loop engine as a standalone global command pair (`templates/global/commands/rnd/`): the driver scaffolds a deterministic run-dir (`GOAL.md`/`STATE.md`/`ledger.jsonl`/frozen hash-pinned `gate/`) and hammers one change per round; a timed fresh-context full referee ratifies champions and gate amendments, detects ruts and instrument defects, and signs the exit; `/rnd` itself is untouched

#### → For: adopters — re-run `pfm install --yes` to link the new `rnd/` command family and regenerate its Codex twins.

- B: global roster — registry-description diet across the always-injected surface: `wave:walker` (1244→550 chars; the § Fast mode mechanism paragraph cut to its trigger routing, which the body already carries in full), `wave:orchestrator` (771→467), `wave:refine` (773→479), `wave:builder` (662→398), `wave:live` (404→386), `deep-rr` (811→401), `/tokens` (1222→610; the flag catalogue replaced by the mode set plus `--help`). Every trigger phrase and named entry point survives verbatim — triggers are the loading contract — and every cut clause was grep-verified present in its own file's body before the cut.
- C: `/tokens` — dead script pointers repaired: the four `node .claude/commands/tokens/token-ledger.mjs` invocations and the README pointer still resolved against the pre-globalization repo path, which exists in no project since the roster moved to `~/.claude/commands/tokens/`.
- C: `/context-meter` — three defects. The `{PROJECT_NAME}` template placeholder shipped verbatim into every session's registry (limits are now named as the framework's). The byte sweep delegated to the retired `Explore` agent (now a haiku child). The nested-command-dir list still named the retired `p/` (now `rnd/`). Its disk enumeration also swept only the repo `.claude/`, blind to the machine-global `~/.claude/` roster it now shares — a context audit reading half the roster reports a falsely small floor, so the sweep and its instruction now name both.
