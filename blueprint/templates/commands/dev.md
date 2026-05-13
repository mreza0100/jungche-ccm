# Dev Environment Setup

> **Tier A — Universal archetype.** Script-driven dev environment management. The subcommand structure and script-maintenance loop are universal. Project-specific server names, ports, health checks, and package managers parameterize per install.

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

| Input | Mode |
|-------|------|
| (empty), `up`, `start` | **UP** — start the full dev environment |
| `kill`, `stop`, `down` | **KILL** — stop all running dev servers |
| `restart` | **RESTART** — kill all servers then start fresh |
| `status` | **STATUS** — show what's running |
| `log`, `logs` | **LOG** — show recent log output (all servers or one) |
| `drop` | **DROP** — nuke Docker containers, rebuild from scratch, restart servers if they were running |
| `fresh` | **FRESH** — kill + drop + start — full clean slate, always starts servers |
| `clear-logs`, `cl` | **CLEAR-LOGS** — delete all current and archived logs |
| `snapshot` | **SNAPSHOT** — dump live DB state, diff against seed-data, update missing entries |
| `iso` | **ISO** — isolated environment management (init, start, kill, destroy, list) |

---

## Step 0 — Script Maintenance (runs BEFORE every UP or RESTART)

**Skip this step for `kill`, `drop`, `fresh`, `status`, `log`, and `clear-logs` modes** — those don't depend on project state.

### 0a. Read current project state (parallel)

Read these files to understand the current project reality:
1. Each project's `package.json` / `pyproject.toml` — scripts, dev commands
2. Infrastructure Makefile — target names (up-local, down-local, etc.)
3. Each project's runbook — ports, env vars, startup commands, health check endpoints

### 0b. Read the current script

Read `.claude/scripts/dev.sh`.

### 0c. Compare and detect drift

Check for discrepancies between what the script does and what the project state says:

| Check | Script location | Source of truth |
|-------|----------------|-----------------|
| **Ports** | Port variables | Runbooks + `.env.local` files |
| **Dep install commands** | `cmd_up()` dependencies section | Project package managers |
| **Migration commands** | `cmd_up()` database section | Infrastructure Makefile targets |
| **Server start commands** | `cmd_up()` server section | Package.json scripts / pyproject.toml |
| **Health check URLs** | `cmd_up()` health section | Runbooks (health endpoints, ports) |
| **Prereq tools** | `check_prereqs()` | Package managers from each project |
| **Infra commands** | `make -C ... up-local` | Infrastructure Makefile target names |
| **Kill patterns** | `clean_ports()` pkill patterns | Server start commands (must match what UP launches) |
| **Service count** | Number of servers started/killed | If a new project/service appears, add it |

### 0d. Update the script if drift detected

If ANY discrepancy found:
1. Use Edit tool to update **only the drifted sections** of `dev.sh`
2. Preserve the overall script structure (functions, helpers, output format)
3. Preserve the `---REPORT---` / `---END---` structured output markers
4. Keep `set -euo pipefail` and the helper functions
5. Log what changed: tell the user "Updated dev.sh: {what changed}"

**Do NOT rewrite the entire script** — surgical edits only.

### 0e. What to do when a new service appears

If a runbook or project directory exists that the script doesn't handle:
1. Add it to the `cmd_up()` server start section
2. Add its port variable (skip for non-HTTP services like SQS consumers)
3. Add it to `cmd_kill()` kill patterns
4. Add its health check to the health checks section
5. Add its log file to the log archival and log reading functions
6. Add its port to `clean_ports()`

---

## Canonical Report Template

All modes assemble from these blocks. `{COMMAND FOOTER}` is always shown last.

**HEADER** — mode-specific one-liner (defined per mode below).

**INFRA BLOCK** (DROP/FRESH only): Infrastructure status — containers, DB readiness, migrations.

**SERVICE TABLE** (UP/RESTART/DROP-with-restart/FRESH):
`| Service | Status | URL |` — one row per service. Status: GREEN=running, RED=down, YELLOW=bundling/compiling.

**STATUS TABLE** (STATUS mode only): Same services but columns are `| Service | PID | Port | Status |`.

**CREDENTIALS** — always show if available. Table: `| Role | Email | Password |`. If MISSING: warn appropriately.

**COMMAND FOOTER:**
```
Commands:
  /dev           Start dev environment
  /dev kill      Stop all servers
  /dev restart   Kill + start fresh
  /dev drop      Nuke containers + rebuild from scratch
  /dev fresh     Kill + drop + start — full clean slate
  /dev status    Show what's running
  /dev log       Show recent logs (all or /dev log {service})
  /dev cl        Clear all logs
  /dev snapshot  Snapshot DB → seed-data
```

If `ERRORS` is not `none`, add "Errors detected" section with details.

---

## Auto-Heal Escalation (applies to: UP, RESTART, DROP-with-restart, FRESH)

**Skip if `DEV_NO_AUTOHEAL=1` is set** — prevents infinite loops when JC restarts via `/dev`.

After showing the report, check if any service has RED status or `ERRORS` is not `none`. If so:

1. Tell the user: "One or more services came up unhealthy — calling JC to diagnose and fix."
2. Collect error context:
   - Which services are RED
   - The `ERRORS` value from the report
   - Read the last 30 lines of log files for RED services
3. Invoke `/jc` with the error context

**When NOT to escalate:**
- Service is YELLOW — just bundling/compiling, not broken
- `ALREADY_RUNNING=true` — servers were already up, not an error
- All services are GREEN but `CREDENTIALS_FILE` is MISSING — warning, not failure

**Loop prevention:** When JC fixes a service and needs to restart it, JC should either:
- Restart the individual service directly
- Or run the dev script with `DEV_NO_AUTOHEAL=1 ./.claude/scripts/dev.sh up` so auto-heal doesn't re-trigger

---

## Mode: UP (default)

1. Run Script Maintenance (Step 0)
2. Run: `./.claude/scripts/dev.sh up 2>&1` (timeout: 120s)
3. Parse structured output between `---REPORT---` / `---END---`:
   - `{SERVICE}_STATUS=GREEN|RED|YELLOW` per service
   - `CREDENTIALS_FILE=<path>|MISSING`, `ERRORS=none|<details>`
   - `ALREADY_RUNNING=true` → ask user: "Dev servers appear to be running. Kill them first with `/dev kill`?"
4. Read credentials if not MISSING (parse JSON: `role`, `email`, `password`)
5. Report using Canonical Template with header: "Dev environment is up!"
6. Auto-heal if needed

---

## Mode: KILL

1. Run: `./.claude/scripts/dev.sh kill 2>&1` (no maintenance needed)
2. Report which processes were killed, ports freed, registry cleared.

---

## Mode: RESTART

1. Run Script Maintenance (Step 0)
2. Run: `./.claude/scripts/dev.sh restart 2>&1`
3. Parse and report using Canonical Template with header: "Dev environment restarted."
4. Auto-heal if needed

---

## Mode: DROP / FRESH (nuke + rebuild)

Both modes nuke Docker containers and rebuild infrastructure from scratch.

**Difference:** DROP only restarts servers if they were already running. FRESH always starts servers (kill + drop + start).

1. Run (no maintenance needed):
   - DROP: `./.claude/scripts/dev.sh drop 2>&1` (timeout: 180s)
   - FRESH: `./.claude/scripts/dev.sh fresh 2>&1` (timeout: 180s)
2. Parse structured output — `NUKE_RESULT`, `INFRA_RESULT`, server markers (same as UP)
3. Read credentials if servers were (re)started and `CREDENTIALS_FILE` is not MISSING
4. Report using Canonical Template (always include infrastructure section)
5. Auto-heal if servers were (re)started and any is RED or has errors

---

## Mode: STATUS

1. Run: `./.claude/scripts/dev.sh status 2>&1` (no maintenance needed)
2. Parse output:
   - If `NO_SERVERS=true`: report "No dev servers running."
   - Otherwise parse `SVC=...|PID=...|PORT=...|ALIVE=...|RESPONDING=...` lines and health check results
3. Read credentials if not MISSING
4. Report using Canonical Template (STATUS variant with PID/Port table)

---

## Mode: LOG

1. Parse args: `/dev log [service] [N]` — defaults: all services, 50 lines.
2. Run: `./.claude/scripts/dev.sh log [service] [N] 2>&1` (no maintenance)
3. Display output directly (script formats with headers + error summary).

---

## Mode: CLEAR-LOGS

1. Run: `./.claude/scripts/dev.sh clear-logs 2>&1` (no maintenance)
2. Report: "Logs cleared." + `{COMMAND FOOTER}`. If 0 cleared: "No logs to clear."

---

## Mode: SNAPSHOT

1. Run: `./.claude/scripts/dev.sh snapshot 2>&1` (no maintenance)
2. Parse: `SNAPSHOT_FILES=N`, `SNAPSHOT_ROWS=N`, `SNAPSHOT_DIR=<path>`
3. Report: table with files/rows/dir. If `SNAPSHOT_FILES=0`: "Nothing to snapshot — all tables empty."

---

## Mode: ISO (Isolated Environment)

Manages fully isolated development environments — each with its own worktree, Docker containers, and allocated ports. No interference with the main dev environment or other isolated envs.

### Subcommand routing

| Input | Action |
|-------|--------|
| `init {profile}` | **INIT** — create a new isolated environment |
| `{command} {profile}` | **FORWARD** — run any /dev command in that environment |
| `pull {profile}` | **PULL** — pull latest committed code from main into the worktree |
| `merge {profile}` | **MERGE** — merge the worktree branch into main |
| `destroy {profile}` | **DESTROY** — tear down an isolated environment completely |
| `list` | **LIST** — show all active isolated environments |

### ISO INIT

1. Guard: one per profile — check `.worktrees/{profile}/` exists
2. Create worktree + allocate ports via gitter SETUP
3. Create `.dev-ports` in worktree root (sourced by dev.sh for port/infra config)
4. Create `docker-compose.{profile}.yml` in worktree root (containers on allocated ports)
5. Start Docker + wait for health checks
6. Apply DB schema (migrations, extensions)
7. Patch `.env.{profile}` files with allocated ports per project
8. Install dependencies (each project's package manager)
9. Verify code is committed — worktrees checkout from HEAD
10. Report with all URLs and ports

### ISO FORWARD

1. Verify `.worktrees/{profile}/.dev-ports` exists
2. Run the worktree's dev.sh with the profile's env
3. Parse and report with isolated ports

### ISO PULL

1. Merge main into the worktree branch
2. Re-apply env patches (merge may have overwritten iso-specific keys)
3. Reinstall deps if manifests changed
4. Report

### ISO MERGE

**Critical: iso-specific files must NEVER reach main.** Sanitize before merging:

1. Restore tracked env files to main's version
2. Remove untracked iso-specific files (`.dev-ports`, `docker-compose.{profile}.yml`)
3. Verify sanitization — grep for iso-specific values in diff
4. Commit actual code changes
5. Switch to main and merge
6. Post-merge verification — confirm no iso artifacts leaked
7. Report

### ISO LIST

Scan `.worktrees/*/` for dirs with `.dev-ports`. Report profile, ports, status.

### ISO DESTROY

1. Kill servers via dev.sh kill
2. Docker compose down -v
3. Gitter: remove worktree, delete branch, free ports, remove docs
