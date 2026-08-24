# Wave 2 — blueprint-framework

**Train:** pfm-wave-2
**Status:** SCHEDULED (#16 held — RE-REFINE flag F3)
**Source:** `docs/dev/trains/queue/2026-08-20-config-ui-mcp-install.md` — Refined 2026-08-20 · main @ `7b01caa`
**Touches:** blueprint, .claude, docs
**Seat:** builder-2 (the main loop; every task here is `[CMD: /pcm]`-guarded and never goes to a code builder)
**Write paths (exclusive):** `blueprint/**`, `.claude/commands/wave/**`, `docs/PLACEHOLDERS.md`, `docs/SETUP.md`, `scripts/placeholder-map.tsv`
**Tasks:** #13, #14, #16
**In-wave order:** #13 → #14 → #16. #14's repo-wide grep gate runs LAST of the two blueprint sweeps so it also covers anything #13 adds.
**Numbering:** source numbers preserved — see `../../train.md` § Numbering.

Task bodies below are byte-identical to the source spec. Scheduler flags follow the bodies and never edit them.

## All-task rules

- **Public repo.** No founder name, email, or machine-absolute `/home/…` path in any code, fixture, comment, or doc. Test fixtures invent neutral values.
- **Only gitter commits; publication (push/tag/release) is founder-owned** and never instructed by this spec.
- **Live-box law:** never reboot devbox; never touch live `cc-*`/`cx-*`/`vsct*` sockets or real `~/.claude`/`~/.cc`/`~/.codex` in tests — every fixture runs in a temp HOME/jail. Never press Enter/⌃O in a jailed picker (launch path escapes the jail); drive picker tests through the pure `ui` layer or `--plain`.
- **Ship = installed:** every pfm task ends with `go build -o ~/.local/bin/pfm ./cmd/pfm` and hash-verifies which binary PATH resolves.
- **Build agents:** `dev` per task, `qa` per modified project, except tasks tagged `[CMD: /pcm]` (blueprint/CLAUDE.md/.claude are hook-guarded).
- **Pre-authorized founder touchpoints:** writing `~/.config/pfm/config.json` on devbox (Task #1 rollout); one Haiku API call per stale-token limits refresh (Task #6); installing the MCP daemon systemd user unit on devbox (Task #8); registering MCP servers in client configs when enabled (Task #10). No other outward-facing action is authorized.

---

### Task #13 — Blueprint: persona/Verdict in CLAUDE.md (a), live-source content diff (b), tokyo-night theme file (c) `[CMD: /pcm]`

**Routing:** /pcm (guarded paths). **Build agents:** /pcm flow, qa.

**Key behaviors:**
a. `blueprint/CLAUDE.md` gains a `## Persona` section: respond as the install's persona (pointer to the active output style) and the mandatory one-line Verdict close. **Why it must live here:** Codex reads only CLAUDE.md/AGENTS.md — output-styles never reach it, so a Codex chat today has no Verdict rule. Wording mirrors the host-ops pattern (2 lines), passes `/quality:prompt` (no voice content in CLAUDE.md — a pointer plus the one structural mandate).
b. Section-level diff of a live source project's `CLAUDE.md` against `blueprint/CLAUDE.md`: any live section the template lacks gets added genericized (refresh.md Tier-A law: parameterize, never trim). Deltas found by the diff are listed in the task's PR-style report to the user before landing.
c. NEW `blueprint/themes/tokyo-night.json` (content salvaged from the `add-tokyo-night-theme` branch, path corrected from dead `blueprint/templates/`); `blueprint/themes/` gains a 3-line README naming the palette JSON shape and that pfm embeds palettes at build time (Task #2).

**Publication surface:** all three touch `blueprint/**` — no founder name, no machine paths, invented example values only.

**File plan:** EDIT `blueprint/CLAUDE.md`; NEW `blueprint/themes/tokyo-night.json`, `blueprint/themes/README.md`.

---

### Task #14 — Retire `{FOUNDER_NAME}`: templates address "the user" `[CMD: /pcm]`

**Why:** founder's rule — templates never address by name; no name placeholder at all. The walk found `{FOUNDER_NAME}` in ~22 blueprint files (zero literal names — the token IS the violation now).

**Routing:** /pcm. **Build agents:** /pcm flow, qa.

**Key behaviors:**
a. Every `{FOUNDER_NAME}` occurrence → "the user" (grammar-adjusted per site; officer.md's "Name the founder in full as {FOUNDER_NAME}" instruction is REWRITTEN to address "the user" role without naming — its semantics change with founder pre-approval via this spec).
b. `{USER_NOUN}` (end-users of the product) is a DIFFERENT concept and stays untouched — the sweep must not conflate them (tracer-flagged trap).
c. `docs/PLACEHOLDERS.md` + the SETUP interview drop the name question; `blueprint/scripts/build-codex.mjs` and mentor.md's two bare-word address sites (ll.116,122) follow.
d. Grep gate: zero `{FOUNDER_NAME}` matches repo-wide when done; `{USER_NOUN}` count unchanged.

**Publication surface:** blueprint-wide; the leak-check + PII rules apply as everywhere.

**File plan:** EDIT the 22-file `{FOUNDER_NAME}` set (T5's table), `blueprint/commands/mentor.md`, `docs/PLACEHOLDERS.md`, `docs/SETUP.md`, `blueprint/scripts/build-codex.mjs`.

---

### Task #16 — Register the wave's invariant `[CMD: /pcm]`

`.claude/commands/wave/walker-invariants.md` gains: **"Account identity, emoji, theme, and permission posture come ONLY from `internal/config` — a hardcoded account count, `.cc/N` literal, medal emoji, or bypass flag outside the config package is a defect."** One entry, registered in the same wave that introduces it (refine law).

---

## Scheduler flags (not part of the source bodies)

### F2 — Task #14 file plan: three anchors do not hold

Verified on `main @ 7b01caa` with `blueprint/**` untouched by the working tree, so these were wrong at
refine time rather than drifted:

| Spec claim | Tree |
| --- | --- |
| "the 22-file `{FOUNDER_NAME}` set (T5's table)" | **18 files, 84 occurrences** — 15 under `blueprint/`, plus `docs/PLACEHOLDERS.md`, `scripts/placeholder-map.tsv`, `.professor/drift.md` |
| EDIT `blueprint/scripts/build-codex.mjs` | **zero** `{FOUNDER_NAME}` occurrences (it carries 10 other tokens: `{AI_PROJECT}`, `{PROJECT_NAME}`, `{SELF}`, …) |
| EDIT `docs/SETUP.md` — "the SETUP interview drop the name question" | **zero** matches for `{FOUNDER_NAME}`, "your name", or "full name" — no name question exists to drop |
| *(absent from the file plan)* | `scripts/placeholder-map.tsv` — rows 9–14 map real names/email **to** `{FOUNDER_NAME}`; it is the leak gate's substitution table and `scripts/leak-check.sh` excludes it at lines 48, 100, 108 |

The 15 blueprint carriers: `CLAUDE.md`, `agents/scheduler.md`, `commands/{documenter,officer,pm}.md`,
`commands/wave/{live,orchestrator,refine,sentinel,walker}.md`,
`output-styles/{dr-house.compact,dr-house.full,professor.compact,professor.full}.md`,
`skills/legal/references/skill-optimizer.md`.

**Ruling needed (R2):** what happens to `scripts/placeholder-map.tsv`. Deleting the `{FOUNDER_NAME}`
target rows disarms the leak gate's name substitution; keeping them leaves a token the task's own
`d.` grep gate ("zero `{FOUNDER_NAME}` matches repo-wide") fails on. A third option is retargeting the
rows to `the user`. This is a security-gate change, not a wording change — the scheduler does not pick.

### F3 — Task #16 cannot produce a conformant registry entry as written [#16 HELD]

`.claude/commands/wave/walker-invariants.md` § Registry Format (ll. 42–48) mandates **five** fields per
entry — Law, Territory, Triggers, Exemplars, Hunt Brief. Task #16 specifies **one** (the Law sentence).
Both live entries (`HONEST-ABSENCE`, `LEAK-LINE`) carry all five.

Two of the four missing fields are derivable from this train's own source spec and two are not:

- **Exemplars** — derivable: Task #1 § Why names four with `file:line` (`config.go:140` `<=3`,
  `swap.go:260` `[]int{1,2,3}`, `installer.go:767` `<=4`, `statusline/render.go:406-428`), all now
  STATUS: FIXED by #1/#2/#3.
- **Territory, Triggers, Hunt Brief** — undecided. Territory globs in particular are the arming
  surface; § Curation warns a bad entry "either arms nothing (dead territory globs) or arms everything
  (a territory of `**`), both silently."
- **Law** — the format requires it "quoted VERBATIM from its CLAUDE.md source, with the source
  pointer." No CLAUDE.md bullet codifies config ownership. The format's escape hatch ("the closest
  codified law is quoted and the gap is flagged") applies, or a root/`pfm/CLAUDE.md` bullet must be
  authored first — an unstated dependency either way.

**Ruling needed (R3):** refine #16 to full registry shape, or authorize the closest-codified-law escape
hatch plus scheduler-proposed territory globs. Until ruled, #16 is written into the train and NOT
scheduled to a seat.
