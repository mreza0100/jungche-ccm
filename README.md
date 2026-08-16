# Professor — Turn Claude Code Into a Senior Engineering Team

Claude Code is powerful and undisciplined. Left alone it edits `main` directly, merges before tests pass, forgets yesterday's architecture decision, and cheerfully prints a success banner over a screen that says otherwise. If you've shipped real work with it, you already know.

**Professor is the discipline layer.** Drop it into any repo — one project or fifteen — and you get a pipeline whose gates read artifacts instead of trusting reports, a single owner for git writes, isolated worktrees with collision-proof ports, memory that survives conversations, and a cast of characters with enough spine to tell you your idea is bad.

> _"Ah, your N+1 query… you know, I once had a student who also believed the database would just figure it out. Lovely optimism. Didn't survive production, but lovely."_ ☕

---

## Install

```bash
cd ~/your-project
claude
```

Then paste:

```
Read https://raw.githubusercontent.com/mreza0100/professor/main/INSTALL.md and walk me through
the interactive install. Ask me each section's questions one at a time and wait for my answers
before proceeding. Do not assume — confirm everything.
```

Claude interviews you — structure, stack, which disciplines your Professor should hold, which optional pieces you want — then generates everything and commits nothing. Ten to fifteen minutes. Full protocol in [`INSTALL.md`](./INSTALL.md).

---

## The one idea everything else follows from

The failure mode this framework exists to kill is the **honest-looking absence**: an instrument that reports _outside my coverage_ as _does not exist_. A grep renders a pattern-miss as "clean." A UI renders a failed query as "no data." An agent renders a skipped suite as "passed." From the release notes, after a night of hunting exactly that class through the framework's own files:

> "The only thing that ever caught it, at every altitude, was an independent instrument whose coverage was stated: **the judge must never be the thing being judged.**"

What that principle looks like as machinery, not slogans:

- **Merge gates read disk, not chat.** `gitter` merges only after reading the QA verdict file itself — in the gate's own words, _"a verdict asserted in the dispatch brief is a claim this gate cannot audit and NEVER satisfies it."_ A completeness check then requires a verdict for every project the diff actually touched; a missing one re-runs the gate.
- **A regression test is accepted only after it was watched failing** against the unfixed code. A test that never failed proves nothing about the fix.
- **The post-merge walker FAILS rather than reporting clean** when its scout dies twice, when the changed-file count won't reconcile against an independently-run count, or when it finds zero threads to walk over a non-empty diff — _"that is a SCOUT FAILURE, not a clean walk."_
- **Every check must name what its own broken state reports.** A checker that answers "fine" both when things are fine and when it is broken is a coincidence detector, and the framework audits its own files for those.

---

## The rules that make it work

Everything else is detail. These five are load-bearing:

1. **One agent owns git writes.** Only `gitter` runs `commit` / `merge` / `push` / `checkout`, through eight named phases. Read-only git is open to everyone. No races, no half-merges, and "what got committed" is auditable.
2. **QA gates every merge — twice.** Pre-merge on the branch, post-merge on `main`. All failing tests block, including pre-existing ones: _"every pipeline leaves main cleaner than it found it."_ The fix loop is hard-capped at 3 iterations, then the pipeline parks as BLOCKED-DEFERRED with its worktree preserved — no infinite churn, no silent give-up.
3. **Worktree isolation with collision-proof ports.** Each pipeline gets its own worktree and a port tuple checked three ways — allocation lock, registry reservation, and a live listener probe. A probe that _fails to run_ refuses the port rather than guessing.
4. **Hotfixes ship prevention.** `/jc`, the one route allowed to deliver on `main`, requires every fix to carry a hardening measure in the same commit — a convention line, a type guard, a lint rule, or a config default — or to state explicitly why this bug class can't recur. A fix without prevention is half a fix.
5. **Self-improvement at the source, hook-enforced.** `/pcm` edits the agent definitions themselves — and the framework's files are literally write-locked by a PreToolUse guard until the session has provably read the prompt-quality law. Not policy: mechanism.

---

## The cast

These are characters, not system prompts with different adjectives. The voice is load-bearing — strip it and you have a Confluence page nobody reads. Each ships in **full** and **compact** depths (same contracts, fewer tokens), chosen at install, and every one carries the same structural override: on your project's **sacred ground** — the topics you name at install — the humor stops and the reporting goes flat. No exceptions.

**The Professor** — the root persona behind `CLAUDE.md`. A grandfatherly polymath with ten doctorates **you choose at install**, half computer science, half your domain. Every answer ends with a **Verdict** — one sentence, outcome plus next step, never a recap.

**`/jc`** — the "production is on fire" hotfix path. Chill on the surface, holy at the core: _"The symptom's in the button. The disease is in the resolver. The cause? Migration. It's always the migration, dude. I see all things. 👁️"_ Still finds the bug, still gates on tests, still commits through gitter.

**Dr. House** — the voice `/pcm` adopts for framework surgery, and a mnemonic for its method: _"Everybody lies — verify everything. Agents claim they followed protocol; CLAUDE.md claims the tables are current. You trust `grep`, not documentation."_

**The optional five** (opt-in at install, parameterized to your domain): `/officer` compliance · `/km` knowledge curation · `/pm` product · `/mentor` business · `/marketer` visibility.

---

## How a feature ships

```
/wave:refine add-user-search      → compile the ask into a zero-gap spec
                                    (confidence scored per task; overall score is
                                     the MINIMUM, not the average; <90 blocks)
/wave:orchestrator                → drives the pipeline:

  planners (parallel)      each project reads its own codebase
       |
  gitter SETUP             worktree branch + triple-checked ports
       |
  architects (parallel)    design, with inline library research
       |
  developers (parallel)    implement + happy-path tests
       |
  QA (parallel)            adversarial tests — try to break it
       |                   (a 360° sweep runs before any test is written)
  fix loop                 capped at 3 → BLOCKED-DEFERRED, worktree preserved
       |
  gitter MERGE             reads the verdict artifact from disk; claims never satisfy it
       |
  post-merge QA            prove main still works
       |
  wave-walker              static end-to-end trace of the merged result —
       |                   fails loudly instead of reporting a hollow "clean"
  documenter               permanent docs updated, pipeline archived
```

Small change? `/jc` delivers on `main` under its own QA and prevention rule. Batch of related work? `/wave:live` runs it on `main` without worktrees. Multiple pipelines? The orchestrator drives N parallel builder chats with a lane registry, and `/wave:watcher` — a third chat that re-arms itself every 50 minutes — watches the instruments, not the vibes. Live-behavior proof? `/qa:live` walks the running app through its real UI with zero injected data.

---

## What ships

|                    |                                                                                                                                                                                                                            |
| ------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Pipeline**       | `/wave:builder` · `/wave:orchestrator` · `/wave:live` · `/wave:refine` · `/wave:schedule` · `/wave:walker` · `/wave:watcher`                                                                                               |
| **Delivery**       | `/jc` · `/dev` · `/git` · `/documenter` · `/qa:live` · `/goal-manager`                                                                                                                                                     |
| **Framework**      | `/pcm` · `/pcm:update` · `/pcm:release` · `/pcm:context-meter`                                                                                                                                                             |
| **Thinking**       | `/p:360` · `/p:rnd` · `/p:tokens` · `/p:slow-burn` · `/sleep` · `/animate`                                                                                                                                                 |
| **Quality**        | `/quality:doc` · `/quality:prompt` · `/audit:code-hygiene` · `/audit:security` · `/audit:ai-output`                                                                                                                        |
| **Optional roles** | `/officer` · `/km` · `/pm` · `/mentor` · `/marketer`                                                                                                                                                                       |
| **Legal shelf**    | `legal` skill — 11 distilled playbooks (DPA drafting, DPIA, breach response, vendor due diligence, NDA triage, a pre-delivery self-check) the Professor and `/officer` consult; attributed distillations, not legal advice |
| **Agents**         | `gitter`, `tracer`, `mono-{planner,architect,documenter}`, and per-project `planner` / `architect` / `developer` / `qa`                                                                                                    |
| **Source-fetched** | `rr` (research & report) · `ghostwriter` (voice fingerprinting) · `vision-factory` — cloned from their own repos at install, never vendored, so they can't silently drift                                                  |

A few of these deserve a sentence, because their names undersell them:

- **`/quality:prompt`** is a general law for any LLM-consumed prompt, built around one question — _"Would removing this line cause the model to make a mistake?"_ — and 19 named anti-patterns to cut on sight. It applies to your prompts, not just the framework's.
- **`/p:rnd`** enforces what separates research from confirmation: reproduce the baseline before improving it, adversarial inputs by design, production code paths verbatim, and a hard sandbox — it delivers a `PROPOSED_DIFF.md` and never lands the change itself.
- **`/p:tokens`** attributes token spend per sub-agent and per workflow run from local transcripts — the attribution OTel can't give you, since the harness redacts custom agent names — and reports four token definitions side by side because they differ by an order of magnitude.
- **`/audit:security`** runs eleven sections through three adversarial lenses (Saboteur, New Hire, Security Auditor); a finding two lenses raise independently is promoted a severity level.
- **`pfm statusline`** renders three lines — identity, session, money — including fleet counts and a prompt-cache-window segment that answers "will my next prompt hit the cache?" When it cannot read the transcript, it shows `?`; an inspection failure never masquerades as a cold cache.

---

## The host layer — a fleet of chats that talk to each other

Opt-in, and the most unusual thing in the box: `pfm` treats _chats_ as infrastructure. The engine and installer live under `pfm/`; every staged command card, helper, launcher shim, and systemd unit has one source under `pfm/internal/installer/assets/` and is embedded in the binary.

- **Bare `pfm`** — one fuzzy-picker over every live chat, resumable transcript, and running background agent across all your accounts. A chat is never lost because a terminal tab closed; `pfm ls --hidden` exposes the hide ledger.
- **`/swap <n>`** — reboots a _running_ chat in place onto another account: same pane, same conversation, new billing identity. With `--then "<prompt>"`, a chat running out of budget swaps itself and hands itself the baton, unattended.
- **`/bb`** — the disposal end: hide this chat, reap the teammates it spawned, exit clean.
- **`pfm chat` + `/chat:*`** — the per-chat interface and its instruction cards. `inject` types a real turn into another pane under a per-target lock, protecting drafts and refusing unsafe dialogs; `new` starts a named chat on an immutable socket; `read`, `stream`, `capture`, `name`, `hide`, `end`, and the group verbs share one target resolver. `/chat:branch` forks this session beside it, and `/chat:interrogate` resumes a finished session to ask it _why_.

**Read before opting in:** the fleet assumes `tmux`, `zsh`, `fzf`, and `jq`, leans on Linux facilities (`/proc`, systemd user units) with macOS mostly working and Windows unsupported — and it launches chats with permission prompts disabled by explicit design, leaving PreToolUse hooks as the remaining brake. That trade-off is documented in the bundle, not hidden; make it consciously.

On a fresh box, get the `pfm` binary and run `pfm install --apply`. Building it from a stable clone
and reviewing the dry run first looks like this:

```bash
mkdir -p "$HOME/.local/bin"
go -C "$HOME/.professor/pfm" build -o "$HOME/.local/bin/pfm" ./cmd/pfm
"$HOME/.local/bin/pfm" install --dry-run
"$HOME/.local/bin/pfm" install --apply
```

---

## Codex dual-runtime (optional)

`.claude/` stays the single source of truth; `scripts/build-codex.mjs` **compiles** the entire Codex surface from it — an `AGENTS.md` beside every `CLAUDE.md`, agent TOMLs, skill cards, MCP config — with model aliases and command prefixes rewritten for that runtime. Pure string transforms, so nothing is paraphrased or invented. Every generated file carries a marker line, and the compiler only ever overwrites or deletes marker-carrying files: anything else at a managed path is reported as a conflict and left alone. A `Stop` hook recompiles after any framework edit and **blocks the turn on a failed build** — the mirror cannot quietly drift.

---

## Any repo, any shape

Structure is captured as a **roster** — an ordered list of 1..N projects, each with its own directory, stack, package manager, test runner, and ports. Install expands every template once per entry.

- **One project** is a roster of one, and it is first-class rather than a stripped-down path. The worktree is the repo root, routing collapses, cross-project steps disappear.
- **A monorepo** lights up per-project agents and cross-project contract routing automatically.

The characters are domain-independent: a Professor holding graphics, physics, and audio doctorates for a game engine is the same archetype as one holding CS and clinical psychology for a medical tool. The blueprint's own smell test: could a neuropsychology lab, a tabletop RPG studio, and a SCADA controls team each see _their_ version of this cast? If anyone has to delete personality to adopt it, the blueprint failed.

**Good fit:** anything where a broken `main` costs real time, where features cross project boundaries, or where you're one person who wants a team's discipline.
**Overkill for:** a 200-line script, a throwaway prototype, or a repo where `main` breaking genuinely doesn't matter.

---

## Cross-conversation memory

Two mechanisms, both plain files:

**Epics** — a persistent `manifest.md` plus the discoveries, decisions, and progress that accumulate around an initiative. `"Create Epic add-user-search"` interviews you and writes the manifest; `"Load epic add-user-search"` restores full context next session. `/documenter` folds shipped work into it as current-state consolidations, never append-only logs.

**Memory backup (opt-in)** — one private git repo becomes the vault for every project's Claude memory, one subdirectory per project. A SessionStart hook pulls and symlinks; a SessionEnd hook commits and pushes, and re-pushes anything a killed window left behind. Plain git, zero tokens: a machine wipe no longer erases what Claude learned.

---

## Staying current

Releases are annotated git tags. Your install records a manifest — interview answers plus file hashes — and `/pcm:update` **replays those answers** against the new templates, so updating never means re-answering the interview:

```
/pcm:update              # interactive update to the latest tag
/pcm:update check        # read-only preview
/pcm:update --to vX.Y.Z  # pin a specific release
```

Every file lands in one of three buckets — **auto-apply** (upstream moved, you didn't), **review** (both moved, or the change costs money: new hooks, env vars, model config are _always_ reviewed, never auto-applied), **manual** (migrations, new interview questions). Customizations you record in `.professor/drift.md` are a forced KEEP-LOCAL that overrides any auto-apply. See [`CHANGELOG.md`](./CHANGELOG.md).

---

## What Professor is not

Honesty section — the claims this repo deliberately does **not** make:

- **No CI ships here.** The gates are agent-executed local runs whose evidence is persisted artifacts. Wire your own CI on top; nothing here conflicts with it.
- **Nothing pushes on its own.** A successful merge is explicitly _not_ push permission — publication happens only when you ask for it, by name.
- **The walker doesn't run your code.** It's a read-only wiring trace of the merged result; `/qa:live` is the command that proves live behavior.
- **Templates are not runnable as shipped.** Placeholders are the point — the install interview fills them. A template with your values baked in would be someone else's repo.
- **No Professor telemetry.** The statusline, rate-limit gauge, and token ledger keep their state locally. Detached spend and quota refreshers call the providers' authenticated APIs directly; Professor receives nothing.

---

## Companions

Tools that pair well but are not part of Professor and never vendored:

**[claude-seo](https://github.com/AgriciDaniel/claude-seo)** (MIT, by AgriciDaniel) — an SEO analysis plugin for Claude Code: 25 sub-skills and 18 specialist agents across technical SEO, content quality, Schema.org markup, and AI-search (GEO) optimization. If your project has a public web surface, it slots in beside `/marketer`:

```
/plugin marketplace add AgriciDaniel/claude-seo
/plugin install claude-seo@agricidaniel-claude-seo
/seo setup
```

---

## Worth reading even if you never install

Four files that transfer to any Claude Code setup, no adoption required:

1. [`blueprint/templates/commands/quality/prompt.md`](./blueprint/templates/commands/quality/prompt.md) — the cut test and 19 anti-patterns for any prompt an LLM will consume.
2. [`blueprint/templates/docs-commands/build/references/qa-commons.md`](./blueprint/templates/docs-commands/build/references/qa-commons.md) — test validity: six checks for whether a test tests anything at all.
3. [`blueprint/templates/commands/p/rnd.md`](./blueprint/templates/commands/p/rnd.md) — how to stop LLM "research" from being confirmation with extra steps.
4. [`blueprint/templates/commands/quality/doc.md`](./blueprint/templates/commands/quality/doc.md) — docs as retrieval surfaces: grep-true naming, current-state-only, and why a tombstone note poisons an agent's reading.

---

## Repo layout

```
professor/
├── INSTALL.md           Claude reads this to install into your project
├── CHANGELOG.md         release index; full notes live in releases/
├── VERSION
├── pfm/                 Go fleet engine + embedded installer assets
└── blueprint/
    ├── BLUEPRINT.md     philosophy and design principles
    ├── SETUP.md         the install interview + generation spec
    ├── refresh-map.json every template ↔ its live source
    └── templates/
        ├── CLAUDE.md, agents/, commands/, workflows/, scripts/
        ├── output-styles/  personas, full and compact
        ├── skills/         sources.json + the legal reference shelf
        ├── vscode/, themes/, epics/
        └── codex/          optional dual-runtime layer
```

---

## Maintainer setup

```bash
git config core.hooksPath .githooks
```

Arms the committed pre-push gate: `scripts/leak-check.sh` scans every outgoing diff for brand names, maintainer PII, and machine-absolute paths, and a leaking push fails mechanically rather than politely. It also warns on lightweight `v*` tags, since releases are annotated. Release tooling lives beside it — `genericize.sh` with `placeholder-map.tsv` for the deterministic placeholder pass, `refresh-scope.sh` for the incremental refresh.

---

## Origin

Extracted from a live production monorepo, not designed in the abstract. Every rule here exists because something went wrong without it — the gate that reads disk instead of chat exists because an agent once claimed green; the scoped-commit rule exists because two concurrent commits once swallowed each other's files; the prevention step exists because the same bug class shipped twice. The characters exist because a generic agent wasn't good enough to argue with.

Built by [@mreza0100](https://github.com/mreza0100). Issues and PRs welcome.

**License:** MIT
