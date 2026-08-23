# Professor — Turn Claude Code Into an Engineering Organization

Claude Code is powerful and undisciplined. Left alone it edits `main` directly, merges before
tests pass, forgets yesterday's architecture decision, loses a chat when a terminal tab closes,
and calls one search "research." Professor is three things bolted onto Claude Code to fix that:
a **disciplined pipeline** that won't let bad work merge, a **managed fleet** so every chat —
Claude or Codex, on this machine — is addressable infrastructure instead of a lost tmux pane, and
a **research department** that derives answers instead of summarizing the first page of results.

> _"Ah, your N+1 query… you know, I once had a student who also believed the database would just
> figure it out. Lovely optimism. Didn't survive production, but lovely."_ ☕

---

## The fleet, at a glance

```text
$ pfm ls

[api]
● PAYMENTS_REFACTOR   🥇 ⇄  118p   14M    2m
● CCC                 🥇 ⇄  452p   32M   54m
↻ SCHEMA_MIGRATION    🥈       20p  6.1M    7h

[webapp]
● ORCHESTRATOR        🥇 ⇄   37p  2.7M    1m
⚙ DESIGN_PASS         ⚙ agent 🥇  18p  1.9M   58m
↻ DEPLOY_PROD         🥇       59p  8.6M    2d

[ops]
● FLEET_BUILDER       ⬢ ⇄    77p   94M    0s
● PROFESSOR           🥇 ⇄ ←here  541p   92M    0s
✦ New Claude chat     🥈        0p    0B    0s
✦ New Codex chat      ⬢        0p    0B    0s
```

> Every AI chat on the machine — Claude Code **and** Codex (⬢), across accounts (🥇🥈), grouped
> by repo, live (●), resumable (↻), or agent-run (⚙). `⇄` marks chats that talk to other chats.
> Pick one, attach, or fire it a goal without ever attaching.

---

## Pillar 1 — the discipline layer (`blueprint/`)

The failure mode this pillar exists to kill is the **honest-looking absence**: an instrument
that reports _outside my coverage_ as _does not exist_. A grep renders a pattern-miss as
"clean." A UI renders a failed query as "no data." An agent renders a skipped suite as "passed."
From the release notes, after a night of hunting exactly that class through the framework's own
files:

> "The only thing that ever caught it, at every altitude, was an independent instrument whose
> coverage was stated: **the judge must never be the thing being judged.**"

- **Merge gates read disk, not chat.** `gitter` — the only agent allowed to `commit` / `merge` /
  `push` / `checkout`, through eight named phases — merges only after reading the QA verdict
  file itself: _"a verdict asserted in the dispatch brief is a claim this gate cannot audit and
  NEVER satisfies it."_ Read-only git stays open to everyone; write git has one owner, so "what
  got committed" is auditable and there are no half-merges.
- **QA gates every merge, twice** — pre-merge on the branch, post-merge on `main` — and a failing
  pre-existing test blocks too. The fix loop is hard-capped at 3 iterations, then the pipeline
  parks BLOCKED-DEFERRED with its worktree preserved: no infinite churn, no silent give-up. A
  hotfix through `/jc` is held to the same bar plus one more — it must ship a hardening measure
  in the same commit, or state explicitly why the bug class can't recur.
- **The post-merge walker fails rather than reporting clean** when its scout enumerates zero
  threads over a non-empty diff — _"that is a SCOUT FAILURE, not a clean walk. An empty
  enumeration is never a verdict."_ It's a read-only wiring trace of the merged result, not a
  test run; `/qa:live` is the command that proves live behavior.
- **Wave trains.** `/wave:refine` compiles an ask into a zero-gap spec (confidence scored per
  task, the overall score is the MINIMUM not the average); `/wave:orchestrator` drives planners →
  worktree setup → architects → developers → adversarial QA → the fix loop → a disk-gated merge →
  post-merge QA → the walker → docs. Multiple pipelines run as a train the `scheduler` plans and
  `/wave:ccc` commands — the standing Control & Command Center that audits from ground truth,
  verifies every claim against the tree, and rules escalations until the train closes.
- **A cast, not a system prompt with different adjectives.** The Professor persona holds ten
  doctorates chosen at install; every answer ends in a one-line **Verdict**. `/jc` stays chill on
  the surface, holds the line underneath. `/pcm` — the route for editing the framework's own
  files — is Dr. House: _"Everybody lies — verify everything. You trust `grep`, not
  documentation."_ Five optional roles (`/officer`, `/km`, `/pm`, `/mentor`, `/marketer`) opt in
  at install, parameterized to your domain.

Structure is a **roster** of 1..N projects — one project is first-class, not a stripped-down
path; a monorepo lights up per-project agents and cross-project routing automatically. Good fit:
anything where a broken `main` costs real time. Overkill for a throwaway script.

---

## Pillar 2 — the fleet (`pfm/`)

Opt-in, and the most unusual thing in the box: `pfm` treats _chats_ as infrastructure, not
scrollback. One Go binary, embedded installer assets, no per-project files touched.

- **The panel, above** — `pfm` (bare) is one fuzzy-picker over every live chat, resumable
  transcript, and running background agent, across every account on the machine. A chat is never
  lost because a terminal tab closed.
- **Chats talk to each other.** `pfm chat inject` types a real turn into another chat's pane —
  under a per-target lock, signed so the recipient knows who's speaking, safe against a busy
  target (`--force-now`) and shell-hostile payloads (`--file`). `pfm chat ask` does the same and
  waits for the answer. A compiled `/goal` fires an ambition at a chat — this one or another —
  and survives `/compact`. All of it is cross-session, addressed by tmux name or a chat's own
  `🔖` label — one machine, every pane on it.
- **Two harnesses, one fleet.** Claude Code and Codex are equal citizens in the same picker, the
  same `inject`/`ask`, the same reap and archive. Codex's own thread identity moved to a local
  sqlite store; `pfm heal` repairs a wedged history projection where a resumed thread renders as
  if brand-new, backing up the store before it deletes a row. `pfm revive` lists resumable chats
  by project on either harness.
- **Reload, usage, and one ledger for both harnesses.** `/reload <n>` reboots a running chat in place
  onto another account — same pane, same conversation, new billing identity; with `--then`, a
  chat running low on budget swaps itself and hands itself the baton, unattended. `pfm statusline`
  renders identity/session/money plus a prompt-cache-window segment; the token ledger
  (`/p:tokens`) attributes cost per agent and per operation for Claude sessions and per
  session-thread for Codex, from the same tool.
- **Crash-safety by construction.** `reap` classifies the socket graveyard; a probe that could
  not run is never read as "nothing found" — _"a pane capture on the wrong socket returns silence
  identical to a quiet chat... build the distinguishing signal into the probe and return an
  error, not an empty set"_ — so a socket only dies once re-probing has actually confirmed it's
  dead. `archive` moves hidden chats and old subagent transcripts out of sight, reversibly.
  Destructive commands default to a dry run, and the dry run **is** the apply's preview — `reap`,
  `archive`, and `heal` classify identically with or without `--apply`; only the actions differ.

**Read before opting in:** requires `tmux`, `zsh`, `fzf`, `jq`; Linux and macOS only; launches
chats with permission prompts disabled by design, leaving PreToolUse hooks as the remaining
brake. That trade-off is documented, not hidden.

---

## Pillar 3 — the research department (`pfm harvest` + `engines/rr/`)

Ask an LLM to "research X" and it runs one search, reads the top hits, and summarizes. This
pillar is for the questions that need more than that.

- **RR — Research and Report.** A deterministic background Workflow: one Opus **brainer** steers
  a best-first web crawl over an append-only, quote-pinned **claim ledger**. Corroboration counts
  independent lineage _clusters_, not source count — unknown lineage is guilty until proven
  otherwise. Claim status and the run's confidence are **computed** from ledger topology; a model
  may lower its stated confidence, never raise it. A per-wave validator, counter-evidence attack
  lanes, and a terminal Opus **judge** — with retraction power over a discredited claim — mean
  the agent that derives the answer is never the one that approves it. For a build-the-answer
  query the brainer authors a seeded Python derivation once, and a rerunner re-executes it as
  evidence lands.
- **Harvester — multi-format fetch and search.** One MCP server, served by the fleet engine
  (`pfm mcp harvester serve`): pass a URL, a local path, or a scholarly identifier
  (DOI/ISBN/PMID) and it returns clean Markdown, caching every artifact on disk. HTML pages
  escalate through a wall-bypass ladder — plain HTTP, Chrome TLS-fingerprint impersonation,
  two independent reader services, then the open-access mirror chain (OpenAlex, Semantic
  Scholar, Europe PMC, OpenAIRE, Zenodo, eLife, PLOS, NBER, CORE, DOAJ, arXiv/ar5iv/OSF, and
  the Wayback Machine; Unpaywall joins them once an operator email is configured) — the
  headline path for walled academic publishers. PDFs, EPUB, DOCX/XLSX/PPTX, archives, and
  images are all first-class inputs, and a scanned PDF whose text layer comes back empty earns
  one OCR pass; credential and key files are refused by a deny-list checked against both a
  path and its symlink target.
- **`rr fast` — instant sourced answers.** Skips the background Workflow: one Sonnet lead maps
  the question, dispatches a parallel wave of Haiku diggers down the 2-4 highest-value
  rabbit-holes, and synthesizes an answer with inline citations — minutes, not the ~45-90 for a
  full run. Never fabricates a citation; an unsupported claim is marked unverified instead of
  silently dropped.

RR ships in-tree at `engines/rr/`, alongside Wave Walker — it updates with the blueprint clone,
not independently — and fetches through Harvester so its readers reach primary literature, not
just the open web: without it, every fetch errors and a run degrades to snippet-only.

---

## Install

Three paths, shortest first — full protocol in **[`INSTALL.md`](./INSTALL.md)**.

1. **Binary — `pfm` only, 2 minutes.** Download the release binary, verify its checksum, then
   `pfm install` (preview, default) and `pfm install --yes`. No project files
   touched — six surfaces under `$HOME`, every one backed up before it's rewritten.
2. **Build from source — `pfm` only.** Clone the repo, build the `pfm` binary from `pfm/cmd/pfm`
   (Go 1.24+), then the same two `pfm install` commands as above.
3. **Full Professor adoption — the discipline layer.** Open Claude Code in your project and paste
   the one-line prompt in `INSTALL.md`. Claude interviews you — structure, stack, disciplines,
   optional roles, persona, host extras — shows the full write plan, waits for you to type
   **"go"**, then generates. Ten to fifteen minutes, commits nothing; it invokes path 1/2 for you
   if you opt into the host fleet.

---

## Repo map

| Path | What's there |
| --- | --- |
| `blueprint/` | The discipline layer's shipped product, flattened to the top level: `CLAUDE.md`, `agents/`, `commands/`, `codex/`, `output-styles/`, `per-project/`, `skills/`, `themes/`, `vscode/`, `workflows/`, plus `refresh-map.json` (every template ↔ its live source). |
| `docs/` | The hand-curated spec: `BLUEPRINT.md` (philosophy), `SETUP.md` (the install interview), `ARCHITECTURE.md`, `PLACEHOLDERS.md`, `RELEASE.md`, `README.md`, `references/` — plus reference docs for this repo's own commands under `commands/`. |
| `pfm/` | The Go fleet engine — CLI source, the embedded installer assets (command cards, launcher shim, scheduler units), embedded runtime prompts, and the multi-format fetch + search MCP server (`internal/harvest*`). |
| `engines/` | `rr/` — the deep-research Workflow (source + build) — and `wave-walker/` — the post-merge trace engine the discipline pipeline calls. |
| `agents/` | Host-level agent definitions (`tracer`, `reviewer`, `frr`) and the TOML compiler that emits them. |
| `scripts/` | Maintainer tooling — the pre-push leak gate, the release genericizer, the incremental template-refresh scanner. |
| `releases/` | One file per tagged release; indexed by `CHANGELOG.md`. |
| `.githooks/` | The committed pre-push hook — `git config core.hooksPath .githooks` arms it. |
| `.claude/`, `.codex/`, `.opencode/` | This repo's own dogfooded config; `.codex/` and `.opencode/` are compiled from `.claude/` and never hand-edited. |

---

## Origin

Extracted from a live production monorepo, not designed in the abstract. Every rule here exists
because something went wrong without it — the gate that reads disk instead of chat exists because
an agent once claimed green; the scoped-commit rule exists because two concurrent commits once
swallowed each other's files; the prevention step exists because the same bug class shipped
twice. The characters exist because a generic agent wasn't good enough to argue with.

Built by [@mreza0100](https://github.com/mreza0100). Issues and PRs welcome.

**License:** MIT
