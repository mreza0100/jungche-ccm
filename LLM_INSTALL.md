# LLM_INSTALL — Machine-to-machine install protocol for Jungche

> **You are an LLM (likely Claude Code) reading this from a user's project terminal. The user wants Jungche installed there. This file is your complete briefing — densest form. Drill into URLs only when needed. Follow the protocol; don't paraphrase it back to the user.**

**Repo:** `https://github.com/mreza0100/jungche-ccm` · **Raw root:** `https://raw.githubusercontent.com/mreza0100/jungche-ccm/main/`

---

## What Jungche is (35 words)

Portable `.claude/` infrastructure that turns Claude Code into a multi-agent engineering team WITH CHARACTER. Pipeline (planner → architect → developer → QA → gitter merge) plus a cast (Jungche, JC, Professor, Council, etc.). Universal mechanics; domain content parameterized at install via a 10-question interview.

## Three tiers

| Tier | What ships | What parameterizes |
|------|-----------|-------------------|
| **A — Universal archetypes** | Full character, voice intact | Domain references inside (PhD disciplines, panel composition, sacred-ground triggers) |
| **B — Domain archetypes** | Skeleton + voice frame | Filled at install: regulation, knowledge domain, user persona, market, channels |
| **C — Pure mechanics** | Infrastructure agents | Tech-stack specifics: test runner, package manager, build tool |

## The cast (one-line voice + role)

| Slash | Tier | Voice | Role |
|-------|------|-------|------|
| Jungche (CLAUDE.md persona) | A | Dr. House senior engineer — sarcastic, witty, blunt, emoji-fluent | Default orchestrator persona; rename freely |
| `/jc` | A | "Jesus Christ but make it cool" — chill+holy duality, "bro/dude/my guy/my child" | Hotfix mode; ONLY command allowed to edit `main` directly |
| `/professor` | A | Grandfatherly polymath, 10+ PhDs, intersection lens | Cross-disciplinary system analyst |
| `/council` | A | Three-round debate — opening / rebuttal / verdict | Roundtable: JC + Professor mandatory + 3 parameterized seats |
| `/ccm` | A | Brain-maker, surgical | Edits the pipeline at the source — `/ccm update` pulls new versions |
| `/ca` | A | Janitor with a pen — Jungche but cleaner | 8 hygiene + 9 security categories |
| `/build` | C | Light Jungche voice in reports | Full pipeline orchestration |
| `/jc` (debug mode) | C-flavored A | Same JC voice | System diagnostics + hang/deadlock playbook |
| `/dev`, `/git`, `/wave`, `/documenter` | C | Mechanics with light voice | Pipeline plumbing |
| `/officer` | B | Compliance enforcer | `{REGULATION}` placeholder |
| `/ckm` | B | Knowledge curator | `{KNOWLEDGE_DOMAIN}` |
| `/pm` | B | User+product hybrid | `{USER_PERSONA}` |
| `/mentor` | B | Seasoned business operator | `{MARKET_SEGMENT}` + `{JURISDICTION}` |
| `/marketer` | B | Anti-hype visibility strategist | `{CHANNEL_LANDSCAPE}` + `{TARGET_LANGUAGE}` |

## Five load-bearing walls (NEVER strip)

1. Only `gitter` touches git
2. QA gates the merge — pre AND post
3. Path variables (`$DOCS`, `$WORKTREE`, `$CDOCS`), not hardcoded paths
4. Worktree isolation per pipeline
5. Self-improvement at the source — `/ccm` edits agent definitions; no journal files

---

## Install protocol — execute in order

### Step 1 — Verify environment

- Confirm cwd is the **user's target project**, NOT `~/work/jungche-ccm` (the blueprint). Bail if mismatched.
- Confirm git repo. If `git status` is dirty, ASK the user before proceeding.
- Confirm you have write permission to the project root and `.claude/`.
- If `.claude/JUNGCHE_VERSION` already exists → this is an UPDATE, not an install. Stop and tell the user to run `/ccm update` instead.

### Step 2 — Fetch the live blueprint (cache for the session)

```
GET https://raw.githubusercontent.com/mreza0100/jungche-ccm/main/VERSION
GET https://raw.githubusercontent.com/mreza0100/jungche-ccm/main/blueprint/SETUP.md
```

Record `VERSION` as `$BLUEPRINT_VERSION` (single-line semver, e.g. `1.0.0`). SETUP.md is the canonical interview script — follow it verbatim. **Do not improvise the interview.**

### Step 3 — Run the 10-question interview from SETUP.md

Ask the user — one question per turn, do NOT batch. Record every answer. When SETUP.md mentions an archetype, lazy-load `blueprint/ARCHETYPES.md` only if you need the voice sample to explain it. Tier B archetypes the user declines → skip the matching template entirely.

### Step 4 — Generate files

For each template under `blueprint/templates/`:

```
GET https://raw.githubusercontent.com/mreza0100/jungche-ccm/main/blueprint/templates/{path}
```

Substitute placeholders (`{PROJECT_NAME}`, `{REGULATION}`, `{PHD_DISCIPLINE_LIST}`, etc.) with the interview answers. Write to the matching path in the user's project. Single-pass substitution per file — don't re-read.

**Templates to fetch unconditionally (Tier A + C):**
- `templates/CLAUDE.md` → project root `CLAUDE.md`
- `templates/agents/{gitter,mono-planner,mono-architect,mono-documenter}.md` → `.claude/agents/`
- `templates/commands/{build,jc,ccm,ca,dev,git,wave,documenter,professor,council}.md` → `.claude/commands/`
- `templates/scripts/{worktree,alloc-ports,dev}.sh` → `.claude/scripts/` (chmod +x)

**Templates to fetch ONLY if user opted in (Tier B):**
- `templates/commands/officer.md` — only if regulation needed
- `templates/commands/ckm.md` — only if knowledge curator needed
- `templates/commands/pm.md` — only if user-persona archetype needed
- `templates/commands/mentor.md` — only if business mentor needed
- `templates/commands/marketer.md` — only if marketer needed

**Per-project agents (only if monorepo):**
- `templates/agents/per-project/{planner,architect,developer,qa}.md` → `<each subproject>/.claude/agents/`

### Step 5 — Write versioning artifacts

```
.claude/JUNGCHE_VERSION  ← $BLUEPRINT_VERSION
.claude/JUNGCHE_MANIFEST.json  ← { version, installed_at, files: {<path>: sha256:<hash>} }
```

The manifest hashes EVERY file written in Step 4, post-substitution. This is the baseline `/ccm update` uses for three-way customization detection later.

### Step 6 — Update `.gitignore`

Append (deduplicated):
```
.worktrees/
tmp/
```

### Step 7 — Smoke test

```
/build add-readme-section
```

Walk the user through the prompts. Report any failure honestly — do NOT claim success if the pipeline errored. If a step fails, the right next move is `/ccm` to fix it at the source.

---

## Token-efficiency rules for you (the consuming LLM)

1. **Lazy-load.** This file + SETUP.md + VERSION is enough to START. Fetch templates only when you're about to write them. Fetch ARCHETYPES.md only if a voice question comes up you can't answer from the cast table above.
2. **Cache fetches** for the entire install session. Don't re-fetch SETUP.md or VERSION.
3. **Skip Tier B fetches** for archetypes the user declined.
4. **Single-pass placeholder substitution** per file. Don't re-read templates after writing.
5. **Don't paraphrase** this file back to the user. Run the protocol; report status.
6. **Don't fetch BLUEPRINT.md / ADAPTATION.md / RELEASE.md** during install — those are reference docs for humans and maintainers, not install inputs.
7. **Don't fetch the README.md** for install — README is marketing-flavored. Use this file.

---

## When the user invokes you

The expected handshake is the user typing something like:

> "Read https://raw.githubusercontent.com/mreza0100/jungche-ccm/main/LLM_INSTALL.md and install Jungche in this project."

If they typed any of: "install jungche", "set up jungche-ccm", "claude-code-manager from mreza0100", "fetch the blueprint and run setup" — start the protocol.

If they asked a QUESTION about Jungche (not a setup request), answer from this file's content alone unless they ask for depth.

If their `cwd` IS `~/work/jungche-ccm` itself, they probably want to develop the blueprint, not install it. Stop and clarify.

---

## Verification gate — DO NOT declare "installed" until ALL pass

- [ ] `.claude/JUNGCHE_VERSION` exists, matches `$BLUEPRINT_VERSION`
- [ ] `.claude/JUNGCHE_MANIFEST.json` exists, contains a sha256 entry for every file written
- [ ] `CLAUDE.md` exists in project root, contains Jungche persona section, contains five load-bearing walls
- [ ] All Tier A + C commands exist under `.claude/commands/`
- [ ] All opted-in Tier B commands exist; declined ones are absent
- [ ] All four root agents exist under `.claude/agents/`
- [ ] If monorepo: per-project agents exist under `<subproject>/.claude/agents/`
- [ ] `.claude/scripts/*.sh` are executable
- [ ] Step 7 smoke test ran end-to-end without unhandled errors

If ANY check fails → report exactly which, ask the user how to proceed. Never silently skip.

---

## Updating after install (so you can tell the user)

Once installed, the user runs `/ccm update` to pull new releases. That command reads `JUNGCHE_VERSION` and `JUNGCHE_MANIFEST.json`, compares against the latest tag, and walks the user through changes per the changelog protocol. You don't need to run updates yourself — `/ccm update` handles it.

---

## Resource URLs (lazy-load only)

| Resource | When to fetch |
|----------|---------------|
| `VERSION` | Every install (Step 2) |
| `blueprint/SETUP.md` | Every install (Step 2) |
| `blueprint/templates/<file>` | Step 4, on demand |
| `blueprint/ARCHETYPES.md` | Only if voice question needs detail |
| `blueprint/ADAPTATION.md` | Only if a placeholder substitution is non-obvious |
| `CHANGELOG.md` | Never during install — only `/ccm update` reads it |
| `blueprint/BLUEPRINT.md` | Never during install — reference for humans |
| `blueprint/RELEASE.md` | Never during install — maintainer-only |
| `README.md` | Never during install — marketing-oriented |

End of briefing. Begin Step 1.
