# Professor

Professor is a discipline layer for Claude Code, Codex, and OpenCode — agent prompts, commands,
hooks, and scripts you clone into a repo — plus `pfm`, a Go CLI that manages every AI coding chat
running on your machine.

The two halves are independent. The blueprint is a portable prompt/template tree: it changes how
agents behave in one repository. `pfm` is a single Go binary: it treats chats as infrastructure
rather than scrollback, across Claude Code, Codex, and OpenCode. You can adopt either without the
other.

```bash
REPO=mreza0100/professor
SOURCE_DIR="$HOME/.professor"
TAG=$(git ls-remote --tags --sort=-v:refname "https://github.com/${REPO}.git" 'v*' \
  | grep -v '\^{}' | head -1 | sed 's#.*/##')
git clone "https://github.com/${REPO}.git" "$SOURCE_DIR"
git -C "$HOME/.professor" checkout "$TAG"
cd "$SOURCE_DIR"
git config core.hooksPath .githooks
cat docs/SETUP.md      # the install interview — start here
```

The checkout is pinned to the latest semantic version tag, not an unversioned branch. For the
binary path, source-build stamp, filtered-network guidance, and the full preview/apply flag
family, see [INSTALL.md](INSTALL.md).

For a maintainer checkout, `pfm doctor` must report
`pre-push gate=armed core.hooksPath=.githooks`. A hook file that merely exists is not armed; an
unwired or non-executable hook is a warning and a non-zero doctor result.

`pfm` installs separately and is opt-in: see [INSTALL.md](INSTALL.md).
Upgrading an existing installation? Follow the complete
[v0.64.0 LLM upgrade runbook](releases/v0.64.0.md#llm-upgrade-runbook).

---

## The fleet, at a glance

```text
 pfm  🥇 account 1 · ⚡1h · 12 rows · 38 killed · 64 empty
 tabs   Chats   Stats   Limits    tab/shift+tab
 Chats · fuzzy search and all existing chat controls
find › type project or name                                                        12/12 visible
╭─ fleet 12 ───────────────────────────────────────────────────────────────────────────────────────╮
│╭─ api                                                                                            │
│› ✦ [ Claude ] Codex OpenCode     🥇                                             0p     0B      0s│
││ ● PAYMENTS_REFACTOR             ⬢ 🥇 ⇄                                       118p    14M      2m│
││ ⚙ SCHEMA_MIGRATION              ⚙ agent 🥈                                    20p   6.1M      7h│
│╭─ webapp                                                                                         │
││ ● ORCHESTRATOR                  🥇 ⇄ ←here                                    37p   2.7M      1m│
││ ⚙ DESIGN_PASS                   ⚙ agent 🥇                                    18p   1.9M     58m│
││ ↻ DEPLOY_PROD                   🥇                                            59p   8.6M      2d│
│╭─ ops                                                                                            │
││ ● FLEET_BUILDER                 ⬢ ⇄                                           77p    94M      0s│
││ ↻ CCC                           🥈                                           452p    32M     54m│
╰──────────────────────────────────────────────────────────────────────────────────────────────────╯
 ↑↓ move  enter open  esc cancel  type to fuzzy-find
 ⌃X hide  ⌃E 1h  ⌃S account  ⌃O reboot
```

> Every AI chat on the machine — Claude Code, Codex (⬢), and OpenCode — across accounts
> (🥇🥈), grouped by repo, live (●), resumable (↻), or agent-run (⚙). `⇄` marks chats that
> talk to other chats; `←here` is the one you are sitting in; `✦` opens a new one on any
> harness. Pick one, attach, or fire it a goal without ever attaching.

Every `pfm ls` invocation also starts a detached, silent Professor release check. A successful
lookup is consumed only by the next invocation: when a newer release exists, the interactive
picker leads with a full-width animated gold **PROFESSOR UPDATE** banner. Choose Claude, Codex, or
OpenCode on that banner and press Enter; the selected engine opens in the recorded Professor clone,
summarizes the release and migration impact, asks for approval, and only then runs `pfm update`.
The lookup never blocks or writes into the active picker frame.

`tab` cycles to **Limits** — every registered provider window on the box, one panel:

```text
 pfm  🥇 account 1 · ⚡1h · 12 rows · 38 killed · 64 empty
 tabs   Chats   Stats   Limits    tab/shift+tab
 Limits · live usage windows across every account
 limits  live usage windows, no controls
╭─ limits ─────────────────────────────────────────────────────────────────────────────────────────╮
│  🥇 account 1 · Claude · provider confirmed 22s ago                                              │
│  ────────────────────────────────────────────────────────────────────────────────────────────────│
│  5h          ▕█████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░▏   12%   ↻ 3h 45m                         │
│  7d          ▕████████████████░░░░░░░░░░░░░░░░░░░░░░░░▏   41%   ↻ 4d 10h                         │
│  7d-fable    ▕█████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░▏   22%   ↻ 4d 10h                         │
│  🥈 account 2 · Claude · provider confirmed 22s ago                                              │
│  ────────────────────────────────────────────────────────────────────────────────────────────────│
│  5h          ▕██░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░▏    5%   ↻ 4h 15m                         │
│  7d          ▕███████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░▏   18%   ↻ 5d 7h                          │
│  🥇 Codex 1 · Pro · provider confirmed 22s ago                                                   │
│  ────────────────────────────────────────────────────────────────────────────────────────────────│
│  7d          ▕████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░▏   31%   ↻ 3d 13h                         │
│  · account 1 · OpenCode · provider confirmation unavailable                                      │
│  ────────────────────────────────────────────────────────────────────────────────────────────────│
│  ⚠ engine ox: no usage source registered                                                         │
╰──────────────────────────────────────────────────────────────────────────────────────────────────╯
```

> The last two rows are the whole point. OpenCode has no usage source registered, so the
> panel _says so_. A provider it cannot reach never renders as a 0% bar: absence claims
> "nothing there", an error claims "we failed to look", and a panel that confuses the two
> is a coincidence detector wearing a progress bar.

`tab` once more reaches **cosmos** — the same fleet, drawn as a sky:

![The pfm cosmos tab: the agent fleet drawn as a star map, each project a star and each chat a body orbiting it](docs/img/pfm-cosmos.png)

> Every chat is exactly one node; every project is a star its chats orbit, and a spawned chat
> rises as a moon around its parent, born at the parent's angle — so lineage is visible in the
> sky itself. Stars take their colour from the hour's traffic and cool over the hours after
> their last message, so a busy project burns blue-white. When chats talk to each other the sky
> draws an edge between them, read from a durable comms ledger — so an edge is a fact rather
> than a guess. The capture above is a busy sky: six projects with an orchestrator seated in
> each, every orchestrator having spawned two workers of its own — 22 chats and 81 edges in the
> window, the moons risen at their parents' angles, and the ticker at the bottom scrolling the
> ledger message by message. `↑↓` rings a chat with a reticle and `enter` opens it; `s` focuses
> one project's system; `o` collapses the hierarchy back to one shared ring. The chronoscope
> replays the last 24h (`[ ]` ±5m, `{ }` ±1h,
> `space` plays at 60×, `n` returns to now) — and a chat that is dead now still renders as the
> ghost it was back then.

---

## The discipline layer (`templates/`)

Clone it into a repo and you get the complete agent, command, hook, script, and `CLAUDE.md`
template set under `templates/`. `docs/SETUP.md` walks an interview that substitutes your
project's names into every placeholder; `docs/PLACEHOLDERS.md` is the substitution law.

The single idea underneath it is the **honest-looking absence** — an instrument that answers
"nothing found" both when nothing is there and when the instrument itself is broken. Every check
in the framework is required to name what its own broken state reports. The wave walker says it
out loud:

> SCOUT FAILURE… An empty enumeration is never a verdict.

What that discipline looks like in practice:

- **One agent writes git.** `gitter` runs six named phases (SETUP, COMMIT, MERGE, PUSH, PULL,
  TAG). No other agent commits; the active main Codex chat may use the explicit-authority fallback
  only when the registered role is unavailable.
- **Guarded files.** A PreToolUse hook gates `.claude/**` and every `CLAUDE.md` behind `/pfm` plus
  a session that has read the quality-prompt contract. The deny message carries the unlock steps.
- **Fix loops are capped.** Three attempts, then BLOCKED-DEFERRED — a bounded failure instead of
  an agent grinding until context runs out.
- **Read-only mappers, separate judges.** `tracer` returns a consumer tree and never a verdict;
  `reviewer` and `architect` judge what the map shows.
- **Three runtimes, one contract.** `CLAUDE.md` and `AGENTS.md` are the same law; the `.codex/`
  and `.opencode/` layers are compiled pointers, never restatements.

Optional roles ship for teams that want them — `/officer`, `/km`, `/pm`, `/mentor`, `/marketer` —
along with a legal skill shelf.

**Read the philosophy in [docs/BLUEPRINT.md](docs/BLUEPRINT.md)** rather than here.

---

## The fleet CLI (`pfm/`)

One Go binary with embedded installer assets. Its installer writes machine state; `pfm init` is the
explicit project-scaffold exception. It exists because a chat that scrolled off a closed terminal
tab is not gone — it is a resumable transcript nobody can find.

- **The picker, above.** Every live chat, resumable transcript, and running background agent, on
  every account and all three harnesses, grouped by repo. Plus a Stats tab (host, process-tree
  and cgroup pressure) and the Limits tab shown above.
- **Chats talk to each other.** `pfm chat inject` types a real turn into another chat's pane —
  under a per-target lock, signed so the recipient knows who is speaking, safe against a busy
  target (`--force-now`) and shell-hostile payloads (`--file`). `pfm chat ask` waits for the
  answer, with named exit codes: `0 done · 2 usage · 3 chat dead · 4 no such chat · 5 answer
timed out · 6 message not delivered`.
- **Reload without losing the conversation.** `pfm chat reload` reboots a running chat in place
  onto another account — same pane, same history, new billing identity. With `--then`, a chat the
  user sends to another account hands itself the baton unattended. Typed by a human,
  `/reload …` runs through a hook without spending a model turn; `--fresh` starts a new
  conversation in the same pane, `--fresh --hide` also hides the one left behind, and the
  `/handoff` skill uses that pair to carry a full context across and retire the chat it came from.
- **Repository memory is manually available, automatically paused.** `pfm dream` retains its
  development commands (`night`, `apply`, `inspect`, `morning`, `migrate-anchors`, `restamp`, and
  `hook`), but install removes automatic Dream/STM injection hooks and never adds them back. Chats
  do not read STM at startup.
- **A research harvester.** `pfm harvest` turns a URL, DOI, ISBN, PMID, PMCID, or local path into
  Markdown, over a pinned Python sidecar and an open-access resolver chain (Unpaywall, OpenAlex,
  Semantic Scholar, Europe PMC, OpenAIRE, Crossref, CORE and more). It exposes the same surface
  through MCP. `pfm harvest ask -p "…" <sources...>` feeds the full cached artifacts—not clipped
  terminal previews—to the configured Claude or Codex ask engine; failed sources remain visible as
  explicit receipts instead of disappearing from the answer corpus. `pfm harvest --ask -p "…"`
  is the equivalent compatibility spelling.
- **Crash-safety by construction.** `reap`, `archive`, `heal`, and `install` all default to a dry
  run, and the dry run **is** the apply's preview — identical classification either way, only the
  actions differ. `heal` backs up the store before it deletes a row. A probe that could not run is
  never reported as "nothing found".
- **Housekeeping.** `doctor` checks the dependency registry and fleet DB; `index` refreshes the
  transcript index; `config` validates the machine config; `codex build|check` is the single
  writer of the Codex mirror; `statusline` renders identity, session and spend. Host install also
  reconciles global Claude commands into marker-owned Codex prompt/skill mirrors, preserving every
  unmarked conflict and foreign file. `pfm install --vscode` additionally installs a reversible
  `PFM` VS Code terminal profile so each new integrated terminal opens at the fleet picker.

**Requirements** (from `pfm doctor`'s own registry, not prose): Linux or macOS, `amd64` or
`arm64`, plus `tmux` ≥ 1.8, `git`, `sh`, `bash`, `zsh`, and `sleep`; `setsid` on Linux, and
`ps`/`lsof`/`launchctl` on macOS. Go **1.24.13 or newer** is needed for source builds and
`pfm update`. The `claude` and `codex` CLIs are optional diagnostics even when accounts are
configured: self-doctor failures stay visible without blocking unrelated installation.
`--skip-engine codex` suppresses the Codex probe and Codex mirror/hooks.
The harvester provisions its own pinned `uv` and CPython, skippable with `--skip-harvest`.
Themes are source-fetched from `templates/themes/sources.json`, skippable with `--skip-themes`.
Both MCP servers ship disabled.

Before applying an install, run its dry preview with the same options and inspect the harvest
plan. For the current Linux `amd64` lock, that optional runtime is about 3.1 GB to download and
5.8 GB on disk; platform, cache, and lock revisions change the footprint. The complete command
pair is documented in [INSTALL.md](INSTALL.md#preview-optional-components-and-harvest-footprint).

**Read before opting in:** `pfm` defaults Claude to bypass mode and Codex to approval bypass;
machine and per-account configuration can select the prompted posture. The default trade-off is
deliberate and documented, not hidden.

---

## Engines (`engines/`)

- **RR** — research-and-report, TypeScript compiled to a single bundled workflow.
- **Wave Walker** — post-merge wiring verification: a scout, parallel walkers, a rule engine and a
  final judge. One TypeScript source compiled for both the Claude Workflow runtime and the Codex
  SDK. Node ≥ 22.13.

---

## Repo map

| Path                                  | What it is                                                                                                                                          |
| ------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `templates/`                          | The shipped framework an adopter clones — agents, commands, scripts, codex and opencode templates. Every file here is production prompt code.       |
| `pfm/`                                | The Go fleet engine: `cmd/pfm` plus its `internal/` packages. Owns its staged host assets.                                                          |
| `engines/`                            | `rr/` (research) and `wave-walker/` (wiring verification).                                                                                          |
| `templates/global/agents/`            | Host-global agents — `tracer`, `rr`, `reviewer` — with their Codex `.toml` twins.                                                                  |
| `docs/`                               | `BLUEPRINT.md` (philosophy), `SETUP.md` (install interview), `PLACEHOLDERS.md` (substitution law), `ARCHITECTURE.md`, plus command reference cards. |
| `scripts/`                            | Repo gates — `leak-check.sh` runs `pre-push`.                                                                                                       |
| `infra/`                              | The isolated-dev container every code wave builds inside.                                                                                           |
| `.claude/` · `.codex/` · `.opencode/` | This repo's own install. `.claude/` is the source of truth; the other two are compiled.                                                             |
| `.professor/`                         | Ledgers — `drift.md`, `release.md`, `retro.md`.                                                                                                     |
| `releases/`                           | Authored release notes; current version in `VERSION`.                                                                                               |

---

## Origin

Extracted from a live production monorepo, not designed in the abstract. Every rule here exists
because something went wrong without it — the gate that reads disk instead of chat exists because
an agent once claimed green; the scoped-commit rule exists because two concurrent commits once
swallowed each other's files; the prevention step exists because the same bug class shipped
twice. The characters exist because a generic agent wasn't good enough to argue with.

Built by [@mreza0100](https://github.com/mreza0100). Issues and PRs welcome.

**License:** MIT
