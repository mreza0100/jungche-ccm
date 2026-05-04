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

---

## Mode: UP (default)

### Step 1 — Run Script Maintenance (Step 0 above)

### Step 2 — Run the script

```bash
./.claude/scripts/dev.sh up 2>&1
```

Timeout: 120 seconds (infra + deps + migrations can be slow).

### Step 3 — Parse the output

The script outputs structured markers between `---REPORT---` and `---END---`:
- `{PROJECT}_STATUS=GREEN|RED|YELLOW` per service
- `CREDENTIALS_FILE=<path>|MISSING`
- `ERRORS=none|<error details>`
- `ALREADY_RUNNING=true` — if servers are already running, ask the user to kill first

### Step 4 — Read credentials

If `CREDENTIALS_FILE` is not `MISSING`, read the JSON file and parse the credentials.

### Step 5 — Report

**ALWAYS show login credentials in the report — every time, no exceptions.**

```
Dev environment is up!

| Service  | Status | URL |
|----------|--------|-----|
| {Service A} | {[GREEN] running / [RED] down} | http://localhost:{PORT} |
| {Service B} | {[GREEN] running / [RED] down / [YELLOW] bundling} | http://localhost:{PORT} |
| ... | ... | ... |

Login credentials:

| Role | Email | Password |
|------|-------|----------|
| {ROLE} | {email} | {password} |

Commands:
  /dev           Start dev environment
  /dev kill      Stop all servers
  /dev restart   Kill + start fresh
  /dev drop      Nuke containers + rebuild from scratch
  /dev fresh     Kill + drop + start — full clean slate
  /dev status    Show what's running
  /dev log       Show recent logs
  /dev cl        Clear all logs
  /dev snapshot  Snapshot DB → seed-data
```

### Step 6 — Auto-heal with JC (if errors detected)

**Skip if `DEV_NO_AUTOHEAL=1` is set** — prevents infinite loops.

After showing the report, check if any service has RED status or `ERRORS` is not `none`. If so:

1. Tell the user: "One or more services came up unhealthy — calling JC to diagnose and fix."
2. Collect error context: which services are RED, the ERRORS value, last 30 lines of log files for RED services
3. Invoke `/jc` with the error context

**When NOT to escalate:**
- Service is YELLOW — still bundling/compiling, not broken
- `ALREADY_RUNNING=true` — servers were already up, not an error
- All GREEN but `CREDENTIALS_FILE` is MISSING — a warning, not a startup failure

**Loop prevention:** When JC restarts a service, use `DEV_NO_AUTOHEAL=1` to prevent re-trigger.

---

## Mode: KILL

```bash
./.claude/scripts/dev.sh kill 2>&1
```

Report which processes were killed, ports freed.

---

## Mode: RESTART

1. Run Script Maintenance (Step 0)
2. Run `./.claude/scripts/dev.sh restart 2>&1`
3. Parse and report (same as UP)
4. Auto-heal if needed

---

## Mode: DROP

```bash
./.claude/scripts/dev.sh drop 2>&1
```

Timeout: 180 seconds (nuke + rebuild + deps + migrations + servers).

Nukes Docker containers, rebuilds from scratch. If servers were running before drop, restarts them. If servers were NOT running, only rebuilds infrastructure.

---

## Mode: FRESH

```bash
./.claude/scripts/dev.sh fresh 2>&1
```

Timeout: 180 seconds. Kill + drop + start — full clean slate. Always starts servers.

---

## Mode: STATUS

```bash
./.claude/scripts/dev.sh status 2>&1
```

Report each service's PID, port, and health status. Include credentials if available.

---

## Mode: LOG

Parse arguments for service filter and line count:
- `/dev log` — all services, 50 lines
- `/dev log {project}` — specific project only
- `/dev log {project} N` — specific project, N lines

```bash
./.claude/scripts/dev.sh log [service] [N] 2>&1
```

---

## Mode: CLEAR-LOGS

```bash
./.claude/scripts/dev.sh clear-logs 2>&1
```

---

## Mode: SNAPSHOT

```bash
./.claude/scripts/dev.sh snapshot 2>&1
```

Dumps live DB state to seed-data files. Reports table counts, row counts, output directory.

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
