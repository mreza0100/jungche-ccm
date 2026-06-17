---
name: dev
description: Manage the {PROJECT_NAME} local dev environment via dev.sh — app and infra lifecycle, health, ports, seeds, logs. Route dev-environment, port, and local/test-mode requests here; fresh wipes the DB volume (destructive).
argument-hint: [status|start|stop|restart|fresh|infra|snapshot|logs]
---

# Dev Environment Setup

Manage the {PROJECT_NAME} development environment: $ARGUMENTS

---

## How this works

The `/dev` command has two jobs:

1. **Maintain** `.claude/scripts/dev.sh` — keep it in sync with the actual project state
2. **Run** the script and present a pretty report

The script does the heavy lifting (infra, deps, migrations, servers) in native bash with parallelism.
You are the script's therapist — you check on it, make sure it's doing okay, and adjust it when the project evolves.

---

## Subcommands

Parse `$ARGUMENTS` to determine the mode:

| Input                  | Mode                                                                                          |
| ---------------------- | --------------------------------------------------------------------------------------------- |
| (empty), `up`, `start` | **UP** — start the full dev environment                                                       |
| `kill`, `stop`, `down` | **KILL** — stop all running dev servers                                                       |
| `restart [{project}]`  | **RESTART** — kill+start all servers, or bounce just the named roster server                  |
| `status`               | **STATUS** — show what's running                                                              |
| `log`, `logs`          | **LOG** — show recent log output (all servers or one)                                         |
| `drop`                 | **DROP** — nuke Docker containers, rebuild from scratch, restart servers if they were running |
| `fresh`                | **FRESH** — kill + drop + start — full clean slate, always starts servers                     |
| `clear-logs`, `cl`     | **CLEAR-LOGS** — delete all current and archived logs                                         |
| `export`               | **EXPORT** — dump live DB state, diff against seed-data, update missing entries               |
| `credentials`, `creds` | **CREDENTIALS** — print seeded login credentials for the active profile                       |
| `iso`                  | **ISO** — isolated environment management (init, start, kill, destroy, list)                  |

---

## Step 0 — Script Maintenance (runs BEFORE every UP or RESTART)

**Skip this step for `kill`, `drop`, `fresh`, `status`, `log`, and `clear-logs` modes** — those don't depend on project state.

### 0a. Read current project state (parallel)

Read these files to understand the current project reality — for every roster entry (`dev.sh` holds the `PROJECTS=(…)` array; iterate it):

1. Each runnable project's manifest (`{project}/package.json` scripts, `{project}/pyproject.toml` entry points, etc.) — dev/start command
2. An infra/config project's `Makefile`, if the roster has one — target names (up-local, down-local, etc.)
3. Each project's `docs/runbook.md` (or `docs/runbook-local.md` for infra) — ports, env vars, startup commands, health check endpoints

### 0b. Read the current script

Read `.claude/scripts/dev.sh`.

### 0c. Compare and detect drift

Check for discrepancies between what the script does and what the project state says:

| Check                     | Script location                                                                                            | Source of truth                                                                                                               |
| ------------------------- | ---------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| **Roster + ports**        | `PROJECTS=(…)` array + per-entry `*_PORT` variables (a non-HTTP project like an {ai} consumer has no port) | Runbooks + `.env.local` files                                                                                                 |
| **Dep install commands**  | `cmd_up()` dependencies section                                                                            | Each roster entry's `{PROJECT_PKG_MGR}`                                                                                       |
| **Migration commands**    | `cmd_up()` database section                                                                                | An infra project's `Makefile` targets (db-migrate-local), if the roster has one — seeding may be handled by a project on boot |
| **Server start commands** | `cmd_up()` server section                                                                                  | Each entry's manifest scripts / entry point ({PROJECT_PKG_MGR} run dev, `run python -m {ai_module}`, etc.)                    |
| **Health check URLs**     | `cmd_up()` health section                                                                                  | Runbooks (health endpoints, ports)                                                                                            |
| **Prereq tools**          | `check_prereqs()`                                                                                          | Package managers across the roster ({PROJECT_PKG_MGR} per entry)                                                              |
| **Infra command**         | `make -C ... up-local`                                                                                     | The infra project's `Makefile` target names, if present                                                                       |
| **Infra health checks**   | `make -C ... pg-ready-local`, `ls-ready-local`                                                             | The infra project's `Makefile` readiness targets                                                                              |
| **DB creation**           | `make -C ... db-create-local`                                                                              | The infra project's `Makefile` DB targets                                                                                     |
| **Kill patterns**         | `clean_ports()` pkill patterns                                                                             | Server start commands (must match what UP launches)                                                                           |
| **Service count**         | Number of servers started/killed                                                                           | Must equal the roster size; if a new project appears, add it                                                                  |
| **Env file defaults**     | Per-project `.env.local` template                                                                          | Runbook required env vars                                                                                                     |

### 0d. Update the script if drift detected

If ANY discrepancy is found:

1. Use the Edit tool to update **only the drifted sections** of `dev.sh`
2. Preserve the overall script structure (functions, helpers, output format)
3. Preserve the `---REPORT---` / `---END---` structured output markers
4. Keep `set -euo pipefail` and the helper functions
5. Log what changed: tell the user "Updated dev.sh: {what changed}"

**Do NOT rewrite the entire script** — surgical edits only. The script's structure is stable;
only the project-specific values drift.

If NO discrepancy is found, skip silently — don't waste time telling the user the script is fine.

### 0e. What to do when a new service appears

If a runbook or project directory exists that the script doesn't handle:

1. Add it to the `PROJECTS=(…)` array and the `cmd_up()` server start section
2. Add a `*_PORT` variable for it (skip for non-HTTP services like an {ai} consumer)
3. Add it to `cmd_kill()` kill patterns
4. Add its health check to the health checks section
5. Add its log file to the log archival and log reading functions
6. Add its port to `clean_ports()`

---

## Canonical Report Template

All modes assemble from these blocks. `{COMMAND FOOTER}` is always shown last.

**HEADER** — mode-specific one-liner (defined per mode below).

**INFRA BLOCK** (DROP/FRESH only, when the roster has an infra project): `Infrastructure: Docker containers: nuked + recreated | {DATABASE} ({DB_PORT}): {ready/failed} | {QUEUE} ({QUEUE_PORT}): {ready/failed} | Database: migrated`

**SERVICE TABLE** (UP/RESTART/DROP-with-restart/FRESH):
`| Service | Status | URL |` — one row per runnable roster entry at its port/URL ({PROJECT_ROLE} at `:{PROJECT_PORT}`; a non-HTTP {ai} consumer shows "{QUEUE} consumer"; add a Health row for whichever project exposes `/health`). Status: GREEN=running, RED=down, YELLOW=bundling/compiling.

**STATUS TABLE** (STATUS mode only): Same rows but columns are `| Service | PID | Port | Status |` + a row for any API endpoint a project exposes. Seed progress line: `Seed progress: {INSERTED}/{EXPECTED} ({STATUS}) — {DETAIL}` (omit if unknown or no seeding).

**CREDENTIALS** — show if the project seeds login credentials. The file is a flat `{ "email": "password" }` map. Derive the role from the email local-part suffix (the install's seeding convention defines the suffix→role map, e.g. `+god`→Admin, `+manager`→Manager, and per-role suffixes for {USER_NOUN}/{SUBJECT_NOUN}). Render `| Role | Email | Password |`. If MISSING: warn "Credentials file missing — seeded on boot in LOCAL env. Check the seeding project's logs." Omit this block entirely for projects with no auth/seeding.

**COMMAND FOOTER:**

```
Commands:
  /dev           Start dev environment
  /dev kill      Stop all servers
  /dev restart [{project}]  Kill + restart all, or one roster server

  /dev drop      Nuke containers + rebuild from scratch
  /dev fresh     Kill + drop + start — full clean slate
  /dev status    Show what's running
  /dev log       Show recent logs (all or /dev log {project})
  /dev cl        Clear all logs
  /dev export    Export DB → seed-data
  /dev creds     Print seeded login credentials
```

If `ERRORS` is not `none`, add "Errors detected" section with details.

---

## Auto-Heal Escalation (applies to: UP, RESTART, DROP-with-restart, FRESH)

**Skip if `DEV_NO_AUTOHEAL=1` is set** — prevents infinite loops when JC restarts via `/dev`.

After showing the report, check if any service has RED status or `ERRORS` is not `none`. If so:

1. Tell the user: "One or more services came up unhealthy — calling JC to diagnose and fix. ☕"
2. Collect error context:
   - Which services are RED
   - The `ERRORS` value from the report
   - Read the last 30 lines of `tmp/dev/{project}.log` for each RED roster entry
3. Invoke `/jc` with the error context:
   ```
   /jc Dev environment startup failed. {RED services list}. Errors from logs: {error details}. Fix the issue and restart the failing service(s). When restarting, set DEV_NO_AUTOHEAL=1 to prevent re-escalation.
   ```

**When NOT to escalate:**

- A bundling/compiling project is YELLOW — building, not broken
- `ALREADY_RUNNING=true` — servers were already up, not an error
- All services are GREEN but `CREDENTIALS_FILE` is MISSING — warning, not failure

**Loop prevention:** When JC fixes a service and needs to restart it, JC should either:

- Restart the individual service with `/dev restart {project}` (bounces just that roster server)
- Or run the dev script with `DEV_NO_AUTOHEAL=1 ./.claude/scripts/dev.sh up` so auto-heal doesn't re-trigger

---

## Mode: UP (default)

1. Run Script Maintenance (Step 0)
2. Run: `./.claude/scripts/dev.sh up 2>&1` (timeout: 120s)
3. Parse structured output between `---REPORT---` / `---END---`:
   - One `{PROJECT}_STATUS=GREEN|RED|YELLOW` marker per roster entry (the script emits one per project it launches)
   - `CREDENTIALS_FILE=<path>|MISSING`, `ERRORS=none|<details>`
   - `ALREADY_RUNNING=true` → ask user: "Dev servers appear to be running. Kill them first with `/dev kill`?"
4. Read credentials JSON if not MISSING (see CREDENTIALS block for the flat-map format)
5. Report using Canonical Template with header: "Dev environment is up!"
6. Auto-heal if needed

---

## Mode: KILL

1. Run: `./.claude/scripts/dev.sh kill 2>&1` (no maintenance needed)
2. Report:

```
Dev servers stopped.
  {PROJECT_ROLE} (port {PROJECT_PORT}): killed   <!-- one line per roster entry; a non-HTTP {ai} consumer shows "({QUEUE} consumer)" -->
  Orphan processes: cleaned
  Registry: cleared

{COMMAND FOOTER}
```

---

## Mode: RESTART

**Bare `/dev restart` (all servers):**

1. Run Script Maintenance (Step 0)
2. Run: `./.claude/scripts/dev.sh restart 2>&1`
3. Parse and report using Canonical Template with header: "Dev environment restarted (killed old servers, started fresh)."
4. Auto-heal if needed

**`/dev restart {project}` (single roster server):** a targeted bounce — kill that one server, start it, brief health. `{project}` is any roster entry's key. Skip Script Maintenance (Step 0).

1. Run: `./.claude/scripts/dev.sh restart {project} 2>&1`
2. Parse `RESTART_RESULT=success|fail` and `RESTARTED={project}`.
3. Report which server was bounced and its result.
4. Auto-heal if `RESTART_RESULT=fail`.

---

## Mode: DROP / FRESH (nuke + rebuild)

Both modes nuke Docker containers and rebuild infrastructure from scratch.

**Difference:** DROP only restarts servers if they were already running. FRESH always starts servers (kill + drop + start).

### Execution

1. Run (no maintenance needed):
   - DROP: `./.claude/scripts/dev.sh drop 2>&1` (timeout: 180s)
   - FRESH: `./.claude/scripts/dev.sh fresh 2>&1` (timeout: 180s)

2. Parse structured output between `---REPORT---` / `---END---`:
   - `NUKE_RESULT=success|fail`, `INFRA_RESULT=success|fail`
   - DROP only: `WERE_RUNNING=true|false`, `SERVERS_SKIPPED=true` (if servers were not restarted)
   - Server markers (same as UP): one `{PROJECT}_STATUS` per roster entry, `CREDENTIALS_FILE`, `ERRORS`

3. Read credentials JSON if servers were (re)started and `CREDENTIALS_FILE` is not MISSING.

4. Report using Canonical Template:
   - DROP header: "Dev environment dropped and rebuilt from scratch."
   - FRESH header: "Dev environment rebuilt from scratch."
   - Include the infrastructure section when the roster has an infra project
   - DROP without restart: add "Servers were not running before drop — infrastructure is ready, use `/dev` to start servers."

5. Auto-heal if servers were (re)started and any is RED or has errors.

---

## Mode: STATUS

1. Run: `./.claude/scripts/dev.sh status 2>&1` (no maintenance needed)
2. Parse output:
   - If `NO_SERVERS=true`: report "No dev servers running."
   - Otherwise parse `SVC=...|PID=...|PORT=...|ALIVE=...|RESPONDING=...` lines, health check results, and seed progress line
   - Seed progress format: `SEED_STATUS=...|SEED_INSERTED=N|SEED_EXPECTED=N|SEED_REMAINING=N|SEED_DETAIL={SUBJECT_NOUN}1=3/6, {SUBJECT_NOUN}2=✓`
3. Read credentials JSON if not MISSING
4. Report using Canonical Template (STATUS variant with PID/Port table + seed progress), header: "Dev server status:"
   - If `SEED_STATUS=unknown` or missing, omit the seed progress section

---

## Mode: LOG

1. Parse args: `/dev log [{project}] [N]` — `{project}` is any roster entry's key; defaults: all services, 50 lines.
2. Run: `./.claude/scripts/dev.sh log [service] [N] 2>&1` (no maintenance)
3. Display output directly (script formats with headers + error summary).

---

## Mode: CLEAR-LOGS

1. Run: `./.claude/scripts/dev.sh clear-logs 2>&1` (no maintenance)
2. Report: "Logs cleared. Current logs: removed. Archive: removed." + `{COMMAND FOOTER}`. If 0 cleared: "No logs to clear."

---

## Mode: EXPORT

1. Run: `./.claude/scripts/dev.sh export 2>&1` (no maintenance)
2. Parse: `EXPORT_FILES=N`, `EXPORT_ROWS=N`, `EXPORT_DIR=<path>`
3. Report: table with files/rows/dir, list tables dumped + empty. If `EXPORT_FILES=0`: "Nothing to export — all tables empty. Run a session first."

---

## Mode: CREDENTIALS

No script run — read the seed file directly.

1. Pick the profile: `demo` if `/dev creds demo`, else `local`.
2. Read the seeding project's `{project}/seeding/{profile}/passwords.json`. If absent: warn "Credentials file missing — seeded on boot. Run `/dev` first."
3. Report the CREDENTIALS block (header: "Seeded credentials ({profile}):") + `{COMMAND FOOTER}`.

---

## Mode: ISO (Isolated Environment)

Manages fully isolated development environments — each with its own worktree, Docker containers ({DATABASE} + {QUEUE} and any other infra the roster runs), and allocated ports. No interference with the main dev environment or other isolated envs.

### Subcommand routing

Parse the arguments after `iso`:

**Convention:** The profile (`demo` or `local`) IS the environment name. Only one isolated environment per profile at a time — if `.worktrees/{profile}/` already exists, refuse and tell the user to destroy it first.

| Input                                                | Action                                                               |
| ---------------------------------------------------- | -------------------------------------------------------------------- |
| `init {demo\|local}`                                 | **INIT** — create a new isolated environment named after the profile |
| `{start\|kill\|restart\|status\|log\|...} {profile}` | **FORWARD** — run any /dev command in that environment               |
| `pull {profile}`                                     | **PULL** — pull latest committed code from main into the worktree    |
| `merge {profile}`                                    | **MERGE** — merge the worktree branch into main                      |
| `destroy {profile}`                                  | **DESTROY** — tear down an isolated environment completely           |
| `list`                                               | **LIST** — show all active isolated environments                     |

---

### ISO INIT — `/dev iso init {demo|local}`

Creates a fully isolated environment with its own worktree, Docker infra, and port allocation.

**Guard:** If `.worktrees/{profile}/` exists, refuse: "Already exists. Destroy first with `/dev iso destroy {profile}`."

**Steps:**

1. Create worktree + allocate ports via gitter SETUP (use `{profile}` as pipeline name)
2. Create `.dev-ports` in worktree root (sourced by `dev.sh`): `PROFILE`, one `*_PORT` per runnable roster entry, the infra ports the roster needs ({DATABASE}, {QUEUE}, plus any analytics/object-store containers), `DB_NAME={db_prefix}_{profile}`, a `DOCKER_*_CONTAINER` per infra container, `DB_USER`
3. Create `docker-compose.{profile}.yml` mirroring the infra project's local compose (same services + images), with per-profile ports
4. Start Docker + wait for health checks
5. Apply DB schema (extensions + migrations) + create any auxiliary databases the infra needs
6. Symlink `schema/` at worktree root → the schema-owning project's `schema/` dir, if the project uses one
7. Patch env files — see [Env-Patch Procedure](#env-patch-procedure)
8. Install deps for every runnable roster entry ({PROJECT_PKG_MGR} per entry)
9. Verify code committed — warn if `git status` shows uncommitted changes (worktrees checkout HEAD only)
10. Report with all URLs and ports

---

### Env-Patch Procedure

Used by ISO INIT (Step 7), ISO PULL (Step 3), and ISO MERGE (sanitization).

**Profile-aware env loading:** each runnable roster entry loads its env per its framework's convention — one PATTERN line per entry (SETUP fills the actual mechanism, e.g. `NODE_ENV`-selected dotenv file, an `ENV_FILE` var, a copied `.env`, or a framework-default `.env.local`). For a `demo` profile the per-project loader resolves to the `.env.demo` variant; for `local`, to `.env.local`.

**For `demo` profile:** Patch `.env.demo` in each project, perform each project's framework-specific copy/var step, dev.sh passes the per-project selectors (e.g. `NODE_ENV=demo`, `ENV_FILE=.env.demo`).

**For `local` profile:** Patch `.env.local` in each project, no profile selectors needed.

**Keys to patch/append** — one PATTERN block per roster entry. SETUP fills each block from the project's runbook required-env-vars list; the values below are the _kinds_ of keys to rewrite, point them at the isolated ports:

**PATTERN — `{project}` env keys:** its own `PORT={its_port}`; DB connection (`DB_HOST`/`DB_PORT={pg_port}`/`DB_NAME={db_prefix}_{profile}`/`DB_USER`, or a single `DATABASE_URL`); the {QUEUE} endpoint + queue URLs (all on `{queue_port}`) and credentials, if the project talks to the queue; the cross-project URLs of its peers (each peer's `{peer_port}`); external-service API keys ({LLM_API_KEY}, {TRANSCRIPTION_API_KEY}, {EMAIL_API_KEY}, etc.) copied from the main `.env.{profile}`; any analytics/object-store host vars. Append `{project}`-specific extras its runbook lists.

**CRITICAL — respect each project's strict-env mode:** if a project's settings layer forbids unknown keys (e.g. a strict-env validator), patch ONLY the exact key names that project expects — never add a key by a name from a different project. Confirm names against the project's settings/runbook before writing.

Seed data for `demo` profile: a worktree checkout from HEAD includes the seeding project's `{project}/seeding/demo/` — no copy needed unless uncommitted on main.

---

### ISO FORWARD — `/dev iso {command} {profile}`

1. Verify `.worktrees/{profile}/.dev-ports` exists.
2. Run the worktree's dev.sh:
   - Demo: `NODE_ENV=demo .claude/scripts/dev.sh {command}`
   - Local: `.claude/scripts/dev.sh {command}` (no NODE_ENV)
   - dev.sh reads `.dev-ports` for port/infra config automatically (ISO_MODE).
3. Parse and report with isolated ports.

---

### ISO PULL — `/dev iso pull {profile}`

Pulls latest committed code from main into the worktree.

1. Verify `.worktrees/{profile}/` exists.
2. Merge: `cd .worktrees/{profile} && git merge main --no-edit`
3. Re-apply env patches via [Env-Patch Procedure](#env-patch-procedure) — merge may have overwritten patched files. Read `.dev-ports` for values.
4. Reinstall deps for any roster entry whose manifest (`package.json`/`pyproject.toml`/etc.) changed:
   ```bash
   # PATTERN — per changed roster entry
   cd .worktrees/{profile}/{project} && {PROJECT_PKG_MGR} install   # add the project's install flags (e.g. --legacy-peer-deps) if its manifest needs them
   ```
5. Report:

```
Iso "{profile}" updated from main.
  Merged: {commit count} new commits
  Env files: re-patched with iso ports
  Deps: reinstalled

Restart servers: /dev iso restart {profile}
```

---

### ISO MERGE — `/dev iso merge {profile}`

Merges the worktree branch into main. **ISO-specific files must NEVER reach main.**

1. Verify `.worktrees/{profile}/` exists.

2. **Sanitize** (in worktree dir):
   - 2a. Restore every project's profile env file from main: `git checkout main -- {project}/.env.{profile} 2>/dev/null || true` per roster entry (plus any infra deploy env, e.g. `{infra}/.../.env.{profile}`)
   - 2b. `rm -f .dev-ports docker-compose.{profile}.yml schema`
   - 2c. Restore the port files from main: `git checkout main -- .env.ports {project}/.env.ports 2>/dev/null || true` per entry that carries one

3. **Verify** — `git diff --name-only | grep -E '\.env\.(demo|local)$|\.dev-ports|docker-compose\.(demo|local)\.yml|\.env\.ports$'` must be empty. If not, restore those files.

4. **Commit** — `git add -A && git diff --cached --quiet || git commit -m "chore: iso {profile} changes"`

5. **Merge** — `cd {repo_root} && git merge pipeline/{profile} --no-edit`

6. **Post-merge check** — `grep -l "{db_prefix}_{profile}\|{pg_port}\|{queue_port}" {project}/.env.{profile} 2>/dev/null` across all entries — if matches, revert those files from `HEAD~1` and commit fix.

7. **Report:**

```
Iso "{profile}" merged into main.
  Branch: pipeline/{profile} → main
  Commits: {count} merged
  Env sanitization: ✅ iso-specific files excluded
  Post-merge check: ✅ no iso artifacts on main

The iso env is still running — destroy it when done: /dev iso destroy {profile}
```

Does NOT destroy the iso env — keeps running so you can verify. Destroy separately.

---

### ISO LIST — `/dev iso list`

Scan `.worktrees/*/` for dirs with `.dev-ports`. Report profile, ports, infra/server status.

---

### ISO DESTROY — `/dev iso destroy {profile}`

1. Kill servers via dev.sh kill
2. Docker compose down -v
3. Gitter: remove worktree, delete branch, free ports, remove docs
