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
| `restart`              | **RESTART** — kill all servers then start fresh                                               |
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

Read these files to understand the current project reality:

1. `{BACKEND_PROJECT}/package.json` — scripts section (dev command)
2. `{FRONTEND_PROJECT}/package.json` — scripts section (web/start command)
3. `{AI_PROJECT}/pyproject.toml` — scripts/entry points if any
4. `{WEB_PROJECT}/package.json` — scripts section (dev command, port {WEB_PORT})
5. `{INFRA_PROJECT}/Makefile` — target names (up-local, down-local, etc.)
6. Each project's `docs/runbook.md` (or `docs/runbook-local.md` for infra) — ports, env vars, startup commands, health check endpoints

### 0b. Read the current script

Read `.claude/scripts/dev.sh`.

### 0c. Compare and detect drift

Check for discrepancies between what the script does and what the project state says:

| Check                     | Script location                                                                                    | Source of truth                                                                                                                                  |
| ------------------------- | -------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Ports**                 | `BE_PORT`, `FE_PORT`, `WEB_PORT` variables ({AI_SERVICE_NAME} has no port — pure {QUEUE} consumer) | Runbooks + `.env.local` files                                                                                                                    |
| **Dep install commands**  | `cmd_up()` dependencies section                                                                    | `{BE_PKG_MGR}` for BE, `{FE_PKG_MGR}` for FE, `{AI_PKG_MGR}` for {AI_SERVICE_NAME}, `{FE_PKG_MGR}` for Web                                       |
| **Migration commands**    | `cmd_up()` database section                                                                        | `{INFRA_PROJECT}/Makefile` targets (db-migrate-local) — seeding is handled by BE on boot                                                         |
| **Server start commands** | `cmd_up()` server section                                                                          | `package.json` scripts (dev for BE, web for FE, dev for Web on port {WEB_PORT}) + `{AI_PKG_MGR} run python -m {ai_module}` for {AI_SERVICE_NAME} |
| **Health check URLs**     | `cmd_up()` health section                                                                          | Runbooks (health endpoints, ports)                                                                                                               |
| **Prereq tools**          | `check_prereqs()`                                                                                  | Package managers from each project ({BE_PKG_MGR}, {FE_PKG_MGR}, {AI_PKG_MGR})                                                                    |
| **Infra command**         | `make -C ... up-local`                                                                             | `{INFRA_PROJECT}/Makefile` target names                                                                                                          |
| **Infra health checks**   | `make -C ... pg-ready-local`, `ls-ready-local`                                                     | `{INFRA_PROJECT}/Makefile` readiness targets                                                                                                     |
| **DB creation**           | `make -C ... db-create-local`                                                                      | `{INFRA_PROJECT}/Makefile` DB targets                                                                                                            |
| **Kill patterns**         | `clean_ports()` pkill patterns                                                                     | Server start commands (must match what UP launches)                                                                                              |
| **Service count**         | Number of servers started/killed                                                                   | If a new project/service appears, it should be added                                                                                             |
| **Env file defaults**     | Backend `.env.local` template                                                                      | Runbook required env vars                                                                                                                        |

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

1. Add it to the `cmd_up()` server start section
2. Add its port to `BE_PORT`/`FE_PORT` or add a new `*_PORT` variable (skip for non-HTTP services like {AI_SERVICE_NAME})
3. Add it to `cmd_kill()` kill patterns
4. Add its health check to the health checks section
5. Add its log file to the log archival and log reading functions
6. Add its port to `clean_ports()`

---

## Canonical Report Template

All modes assemble from these blocks. `{COMMAND FOOTER}` is always shown last.

**HEADER** — mode-specific one-liner (defined per mode below).

**INFRA BLOCK** (DROP/FRESH only): `Infrastructure: Docker containers: nuked + recreated | {DATABASE} ({DB_PORT}): {ready/failed} | LocalStack ({QUEUE_PORT}): {ready/failed} | Database: migrated`

**SERVICE TABLE** (UP/RESTART/DROP-with-restart/FRESH):
`| Service | Status | URL |` — Backend(:{BACKEND_PORT}/graphql), Frontend(:8081), {AI_SERVICE_NAME}({QUEUE} consumer), Web(:{WEB_PORT}), Health(:{BACKEND_PORT}/health). Status: GREEN=running, RED=down, YELLOW=bundling/compiling.

**STATUS TABLE** (STATUS mode only): Same services but columns are `| Service | PID | Port | Status |` + GraphQL row. Seed progress line: `Seed progress: {INSERTED}/{EXPECTED} ({STATUS}) — {DETAIL}` (omit if unknown).

**CREDENTIALS** — always show. The file is a flat `{ "email": "password" }` map. Derive the role from the email local-part suffix (`+god`→Admin, `+manager`→Manager, `+t-`→{USER_NOUN}, `+p-`→{SUBJECT_NOUN}). Render `| Role | Email | Password |`. If MISSING: warn "Credentials file missing — BE seeds on boot in LOCAL env. Check BE logs."

**COMMAND FOOTER:**

```
Commands:
  /dev           Start dev environment
  /dev kill      Stop all servers
  /dev restart   Kill + start fresh
  /dev drop      Nuke containers + rebuild from scratch
  /dev fresh     Kill + drop + start — full clean slate
  /dev status    Show what's running
  /dev log       Show recent logs (all or /dev log be|{ai}|fe|web)
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
   - Read the last 30 lines of log files for RED services (`tmp/dev/be.log`, `tmp/dev/{ai}.log`, `tmp/dev/fe.log`, `tmp/dev/web.log`)
3. Invoke `/jc` with the error context:
   ```
   /jc Dev environment startup failed. {RED services list}. Errors from logs: {error details}. Fix the issue and restart the failing service(s). When restarting, set DEV_NO_AUTOHEAL=1 to prevent re-escalation.
   ```

**When NOT to escalate:**

- Frontend is YELLOW — just bundling, not broken
- Web is YELLOW — just compiling, not broken
- `ALREADY_RUNNING=true` — servers were already up, not an error
- All services are GREEN but `CREDENTIALS_FILE` is MISSING — warning, not failure

**Loop prevention:** When JC fixes a service and needs to restart it, JC should either:

- Restart the individual service directly (using the manual restart commands in JC Step 3)
- Or run the dev script with `DEV_NO_AUTOHEAL=1 ./.claude/scripts/dev.sh up` so auto-heal doesn't re-trigger

---

## Mode: UP (default)

1. Run Script Maintenance (Step 0)
2. Run: `./.claude/scripts/dev.sh up 2>&1` (timeout: 120s)
3. Parse structured output between `---REPORT---` / `---END---`:
   - `BE_STATUS=GREEN|RED`, `CORTEX_STATUS=GREEN|RED|YELLOW`, `FE_STATUS=GREEN|RED|YELLOW`, `WEB_STATUS=GREEN|RED|YELLOW`
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
  Backend  (port {BACKEND_PORT}): killed
  {AI_SERVICE_NAME}   ({QUEUE} consumer): killed
  Frontend (port 8081): killed
  Web      (port {WEB_PORT}): killed
  Orphan processes: cleaned
  Registry: cleared

{COMMAND FOOTER}
```

---

## Mode: RESTART

1. Run Script Maintenance (Step 0)
2. Run: `./.claude/scripts/dev.sh restart 2>&1`
3. Parse and report using Canonical Template with header: "Dev environment restarted (killed old servers, started fresh)."
4. Auto-heal if needed

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
   - Server markers (same as UP): `BE_STATUS`, `CORTEX_STATUS`, `FE_STATUS`, `WEB_STATUS`, `CREDENTIALS_FILE`, `ERRORS`

3. Read credentials JSON if servers were (re)started and `CREDENTIALS_FILE` is not MISSING.

4. Report using Canonical Template:
   - DROP header: "Dev environment dropped and rebuilt from scratch."
   - FRESH header: "Dev environment rebuilt from scratch."
   - Always include infrastructure section
   - DROP without restart: add "Servers were not running before drop — infrastructure is ready, use `/dev` to start servers."

5. Auto-heal if servers were (re)started and any is RED or has errors.

---

## Mode: STATUS

1. Run: `./.claude/scripts/dev.sh status 2>&1` (no maintenance needed)
2. Parse output:
   - If `NO_SERVERS=true`: report "No dev servers running."
   - Otherwise parse `SVC=...|PID=...|PORT=...|ALIVE=...|RESPONDING=...` lines, health check results, and seed progress line
   - Seed progress format: `SEED_STATUS=...|SEED_INSERTED=N|SEED_EXPECTED=N|SEED_REMAINING=N|SEED_DETAIL=Subject1=3/6, Subject2=✓`
3. Read credentials JSON if not MISSING
4. Report using Canonical Template (STATUS variant with PID/Port table + seed progress), header: "Dev server status:"
   - If `SEED_STATUS=unknown` or missing, omit the seed progress section

---

## Mode: LOG

1. Parse args: `/dev log [be|{ai}|fe|web] [N]` — defaults: all services, 50 lines.
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
2. Read `{BACKEND_PROJECT}/seeding/{profile}/passwords.json`. If absent: warn "Credentials file missing — BE seeds on boot. Run `/dev` first."
3. Report the CREDENTIALS block (header: "Seeded credentials ({profile}):") + `{COMMAND FOOTER}`.

---

## Mode: ISO (Isolated Environment)

Manages fully isolated development environments — each with its own worktree, Docker containers ({DATABASE} + LocalStack), and allocated ports. No interference with the main dev environment or other isolated envs.

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
2. Create `.dev-ports` in worktree root (sourced by `dev.sh`): `PROFILE`, `BE_PORT`, `FE_PORT`, `WEB_PORT`, `PG_PORT`, `LS_PORT`, `PULSE_PORT`, `DB_NAME={db_prefix}_{profile}`, `DOCKER_PG_CONTAINER={project}-{profile}-postgres`, `DOCKER_LS_CONTAINER=...-localstack`, `DOCKER_PULSE_CONTAINER=...-pulse`, `DB_USER=postgres`
3. Create `docker-compose.{profile}.yml` — PG + LocalStack + Umami/Pulse (`ghcr.io/umami-software/umami:postgresql-v2.20.2`, `{pulse_port}:3000`, same config as `{INFRA_PROJECT}/local/docker-compose.yml` pulse service)
4. Start Docker + wait for health checks
5. Apply DB schema (pgvector + SQL migrations) + create `{db_prefix}_pulse` database
6. Symlink `schema/` at worktree root → `{BACKEND_PROJECT}/schema/`
7. Patch env files — see [Env-Patch Procedure](#env-patch-procedure)
8. Install deps (BE {BE_PKG_MGR}, FE {FE_PKG_MGR}, Web {FE_PKG_MGR})
9. Verify code committed — warn if `git status` shows uncommitted changes (worktrees checkout HEAD only)
10. Report with all URLs and ports

---

### Env-Patch Procedure

Used by ISO INIT (Step 7), ISO PULL (Step 3), and ISO MERGE (sanitization).

**Profile-aware env loading:**

- **BE:** `dotenv.config({ path: '.env.${NODE_ENV}' })` — `NODE_ENV=demo` loads `.env.demo`
- **{AI_SERVICE_NAME}:** pydantic-settings reads `ENV_FILE` env var — set `ENV_FILE=.env.demo` for demo
- **FE:** Expo reads `.env` at build/start — copy `.env.{profile}` → `.env`
- **Web:** Next.js reads `.env.local` — write `NEXT_PUBLIC_DEMO_URL=http://localhost:{fe_port}`

**For `demo` profile:** Patch `.env.demo` in each project, copy FE `.env.demo` → `.env`, dev.sh passes `NODE_ENV=demo` for BE, `ENV_FILE=.env.demo` for {AI_SERVICE_NAME}.

**For `local` profile:** Patch `.env.local` in each project, copy FE `.env.local` → `.env`, no NODE_ENV needed.

**Keys to patch/append:**

**BE** — `PORT={be_port}`, `DB_HOST=localhost`, `DB_PORT={pg_port}`, `DB_NAME={db_prefix}_{profile}`, `DB_USER=postgres`, `JWT_SECRET={profile}-secret`, `SQS_ENDPOINT_URL=http://localhost:{ls_port}`, `SQS_QUEUE_URL` / `SQS_RESULT_QUEUE_URL` / `SQS_LIVE_CHUNKS_QUEUE_URL` / `SQS_LIVE_NOTES_QUEUE_URL` (all with `{ls_port}`), `AWS_REGION=eu-west-1`, `AWS_ACCESS_KEY_ID=test`, `AWS_SECRET_ACCESS_KEY=test`, `AWS_DEFAULT_REGION=eu-west-1`, `{TRANSCRIPTION_API_KEY}` (copy from main `.env.local`), `{EMAIL_API_KEY}` (copy, optional), `PULSE_HOST=http://localhost:{pulse_port}`, `PULSE_ADMIN_USER=admin`, `PULSE_ADMIN_PASSWORD=umami`, `ALLOWED_ORIGINS=http://localhost:{fe_port},http://localhost:{be_port}`

**FE** — `EXPO_PUBLIC_BACKEND_URL=http://localhost:{be_port}`

**{AI_SERVICE_NAME}** — `DATABASE_URL=postgresql+asyncpg://postgres@localhost:{pg_port}/{db_prefix}_{profile}`, `AWS_ENDPOINT_URL=http://localhost:{ls_port}`, `AWS_ACCESS_KEY_ID=test`, `AWS_SECRET_ACCESS_KEY=test`, `AWS_DEFAULT_REGION=eu-west-1`, `SQS_QUEUE_URL` / `SQS_RESULT_QUEUE_URL` / `SQS_LIVE_CHUNKS_QUEUE_URL` / `SQS_LIVE_NOTES_QUEUE_URL` (all with `{ls_port}`), `{LLM_API_KEY}` + `GOOGLE_CLOUD_PROJECT` + `GOOGLE_CLOUD_LOCATION` (copy from main `.env.local`)

**Web** — `NEXT_PUBLIC_DEMO_URL=http://localhost:{fe_port}` (in `.env.local`)

**CRITICAL: {AI_SERVICE_NAME} pydantic-settings uses `extra="forbid"`** — do NOT add `SQS_ENDPOINT_URL` or `AWS_REGION`. Correct names: `AWS_ENDPOINT_URL`, `AWS_DEFAULT_REGION`.

Seed data for `demo` profile: worktree checkout from HEAD includes `{BACKEND_PROJECT}/seeding/demo/` — no copy needed unless uncommitted on main.

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
4. Reinstall deps if `package.json`/`pyproject.toml` changed:
   ```bash
   cd .worktrees/{profile}/{BACKEND_PROJECT} && {BE_PKG_MGR} install
   cd .worktrees/{profile}/{FRONTEND_PROJECT} && {FE_PKG_MGR} install --legacy-peer-deps
   cd .worktrees/{profile}/{WEB_PROJECT} && {FE_PKG_MGR} install
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
   - 2a. `git checkout main -- {BACKEND_PROJECT}/.env.{profile} {FRONTEND_PROJECT}/.env.{profile} {AI_PROJECT}/.env.{profile} {WEB_PROJECT}/.env.{profile} {INFRA_PROJECT}/hetzner/.env.{profile} 2>/dev/null || true`
   - 2b. `rm -f .dev-ports docker-compose.{profile}.yml schema`
   - 2c. `git checkout main -- .env.ports {BACKEND_PROJECT}/.env.ports {FRONTEND_PROJECT}/.env.ports {INFRA_PROJECT}/.env.ports {WEB_PROJECT}/.env.local 2>/dev/null || true`

3. **Verify** — `git diff --name-only | grep -E '\.env\.(demo|local)$|\.dev-ports|docker-compose\.(demo|local)\.yml|\.env\.ports$'` must be empty. If not, restore those files.

4. **Commit** — `git add -A && git diff --cached --quiet || git commit -m "chore: iso {profile} changes"`

5. **Merge** — `cd {repo_root} && git merge pipeline/{profile} --no-edit`

6. **Post-merge check** — `grep -l "{db_prefix}_{profile}\|{pg_port}\|{ls_port}" {BACKEND_PROJECT}/.env.{profile} {FRONTEND_PROJECT}/.env.{profile} {AI_PROJECT}/.env.{profile} 2>/dev/null` — if matches, revert those files from `HEAD~1` and commit fix.

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
