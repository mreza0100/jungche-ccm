# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/pcm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- Important: wave pipeline (`commands/wave/refine.md` + `agents/scheduler.md`) — precondition anchors: a production behavior a task relies on (an "existing" code path, a CLI verb or flag, an exit code) is code-verified and anchor-cited at refine time, with a missing surface becoming a scheduled dependency task; the scheduler gains an **Anchors** step that grep/read-verifies every prose-relied surface before scheduling — diff-scoped staleness catches a spec broken by later commits, the new step catches one born wrong (RE-REFINE instead of a silent pass).
- Important: `blueprint/CLAUDE.md` — new `## Persona` section (respond as the install's active output style; every reply closes with a one-line **Verdict**) so Codex runtimes, which read only CLAUDE.md/AGENTS.md, carry the Verdict rule; Docs map gains a facts-registry example (`docs/facts/_index.md` — user-ruled invariants, read before touching data lifecycle or {SENSITIVE_DATA}, contradiction = escalate) and the /documenter bullet gains `docs/facts/` (main loop only, on the user's explicit ruling).
- Minor: `blueprint/themes/` — new `tokyo-night.json` palette (47 UI overrides) + a README naming the palette JSON shape; pfm embeds palettes at build time.
- Important: `commands/wave/sentinel.md` → `commands/wave/ccc.md` — the one-shot sentinel auditor becomes the Control & Command Center (CCC): the standing command seat over a running train. Full audit from ground truth on arrival (the sentinel checklist survives as CCC's instrument), then it holds command until the train closes — verifies every DONE/green claim against the tree, rules scope-allocation escalations as ledger lines, routes user-only decisions up with why, dispatches through the orchestrator only; `/wave:orchestrator` now names CCC's ledger-logged rulings authoritative.

#### → For: adopters re-running refresh — `/wave:sentinel` is gone; call `/wave:ccc` (same triggers, plus "take command of the train").

- Important: the founder-name placeholder is retired across the framework — templates address "the user", never a name and never a name placeholder; the `NEEDS-FOUNDER-SPEC` status becomes `NEEDS-USER-SPEC`; officer.md identifies the user by ROLE in legal documents; the placeholder registry drops the token (startup-domain "founder" vocabulary in mentor/marketer is content, not address, and stays).

#### → For: adopters re-running SETUP see no name question; installs carrying the old founder-name token re-resolve it to "the user" on next refresh.

- Major: `pfm` engine contract — Claude, Codex, and OpenCode are one descriptor-driven roster across discovery, launch, picker, limits, doctor, MCP, and headless operations; OpenCode sessions are indexed from `opencode.db`, rendered in the picker, and written only by their owning engine.

#### → For: add only real Codex homes to `codex.homes`; OpenCode is discovered from its native store and needs no fabricated account entry. (cost: new config fields and optional third-engine process)

- Major: `pfm` configuration — schema v2 adds per-account Claude/Codex posture, multi-account Codex homes, OpenCode, theme, prompt-cache choice, MCP loopback port, and one-shot ask model/effort preferences; v1 remains readable for this release and resolves through v2 defaults.

#### → For: run `pfm config validate` and `pfm config show`; preserve customized account paths, and change the file's `version` to `2` when adopting new fields.

- Major: `pfm` installer/update lifecycle — `pfm install` is a read-only preview, `pfm install --yes` applies exactly that plan, `pfm uninstall` is the inverse verb, and the new transactional `pfm update --to vX.Y.Z` builds twice, replaces only owned binaries, runs install + doctor, and rolls back on failure.

#### → For: `v0.58.0` cannot self-update because it predates `pfm update`; install the `v0.60.0` binary first, then use `pfm update` for later releases.

- Major: `pfm harvest` — the Go fleet engine is the only Harvester server and cache owner; the standalone Python service tree is removed, while the pinned Python conversion sidecar remains internal. The resolver ladder gains broader open-access sources, browser-backed HTML rescue behind an SSRF chokepoint, OCR escalation, parity/stress coverage, and explicit absence/error receipts.

#### → For: retire standalone Harvester services and invoke `pfm harvest` or `pfm mcp harvester serve`; keep the pinned sidecar managed by `pfm install`.

- Important: `pfm` MCP wiring — Chat and Harvester share one loopback-only HTTP daemon, stay disabled by default, register only installer-owned client entries, persist across service restarts, and require no bearer credential; legacy auth tokens and owned Authorization headers are removed during install.

#### → For: enable servers with `pfm mcp chat enable` / `pfm mcp harvester enable`, run `pfm install --yes`, and remove hand-written auth headers from loopback PFM entries. Never expose the daemon beyond loopback. (cost: optional local daemon)

- Important: `pfm` fleet UI — Chats, Stats, and Limits become separate live tabs; Limits reads per-provider windows with last-good cache/backoff, Stats measures prompt activity rather than transcript age alone, sparklines use shared on-disk samples, and missing providers render as named unavailable states instead of fabricated zeroes.
- Important: `pfm` chat lifecycle — picker `hide` becomes reversible `kill`, `deactive` ends the pane while keeping the chat resumable, Codex `/clear` reconciles the actual pane/thread identity, Claude launches always enter owned tmux sessions, and chat status can summarize the last exchange through an isolated one-shot engine.

#### → For: replace automation that calls `hide` with `kill`; legacy `_HIDE` records remain readable during migration.

- Important: `pfm doctor` + installer — external dependencies, engine self-doctors, hook ownership, launcher placement, MCP state, source marker, and database schema are probed as named states; the installer preserves third-party hooks and refuses unreadable or ambiguous ownership instead of treating it as absence.
- Important: development/release gates — code waves run through the isolated `pfm-dev` fence, the full PFM install/update/uninstall transaction runs on Linux and Darwin, CI cross-compiles four targets twice for reproducibility, and the public leak/codex-marker gates fail the build when their own probes cannot run.
- Important: Professor agent cast — `reviewer` is the seat-level judgment lead while `tracer` stays a read-only mapper; `frr` replaces `rr-fast`; `pfm codex agents` replaces the global-agent Python compiler; the live Claude, Codex, and OpenCode adapters carry one shared contract.
- Fixed: `pfm` stabilization — account matching, pane-name resolution, duplicate tmux warning suppression, Limits refresh/backoff, stale transcript classification, shim re-sourcing, MCP ownership reconciliation, Harvester fallbacks, doctor timeout/version parsing, hook adoption, and headless tool-work synchronization are hardened by regression and stress tests.
- Fixed: repository dev gate — the retired standalone Dreamer is removed from the project roster because the memory organ now belongs to PFM; missing project directories increment the failure count, blueprint status scans the actual shipped tree, and the OpenCode mirror gate runs once.
- Fixed: `pfm` isolated e2e fixture — only tracked and unignored source files are staged, safe repository-internal generated skill symlinks are preserved, and absolute or source-escaping links fail by name, so ignored worktrees cannot poison the install/update/uninstall fence.
- Fixed: isolated development fence — the multi-architecture image pins Node/npm alongside Go, walker dependencies and reproducible candidates live in container-only volumes, its legacy bundle and active pointer remain tracked, walker `all` builds before verifying, e2e disables Go's result cache, dependency detection checks npm's lock marker, and every `dev.sh iso` invocation builds the current Dockerfile before grading the tree.
- Migration: `v0.58.0` → `v0.60.0` — follow the runbook below in order; machine PFM and project blueprint are independent lanes, and an updater applies only the lanes the installation actually uses.
