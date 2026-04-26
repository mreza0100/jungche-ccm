# Dev Environment

Manage the local development environment.

Argument: $ARGUMENTS — subcommand (optional, defaults to `start`)

| Subcommand | Action |
|------------|--------|
| *(none)* / `start` | Start all dev servers |
| `kill` / `stop` | Kill all dev server processes |
| `restart` | Stop, then start |
| `status` | Show running processes + ports |
| `log [project]` | Tail logs for a project (or all if omitted) |

---

## Start

Run `.claude/scripts/dev.sh start`. The script:

1. Boots local infrastructure (whatever your `start_infrastructure` block in `dev.sh` does — containers, backing services, mock servers, etc.)
2. Starts each project's dev server with the right env file (`.env.local`)
3. Logs each project's stdout to `tmp/dev-logs/{project}.log`
4. Reports the URL or PID for each service

Report to user:

```
Dev environment running.
- {project-a}: http://localhost:{PORT}
- {project-b}: http://localhost:{PORT}
- {project-c}: pid {N}, log: tmp/dev-logs/{project-c}.log
```

---

## Stop

Run `.claude/scripts/dev.sh kill`. Kills all dev server PIDs tracked in `tmp/dev-pids/`. Stops infrastructure containers (the script knows which).

Report:

```
Dev environment stopped. {N} processes killed.
```

---

## Status

Run `.claude/scripts/dev.sh status`. Shows:

```
- {project-a}: running (pid {N}, port {PORT})
- {project-b}: running (pid {N}, port {PORT})
- {project-c}: stopped
- {infra-a} (local): up
- {infra-a} (test):  down
```

---

## Log

Tail the relevant log file. If `$ARGUMENTS` includes a project name, tail that project's log. Otherwise, multiplex all logs.

```bash
tail -f tmp/dev-logs/{project}.log
# or
tail -f tmp/dev-logs/*.log
```

---

## Hard rules

- **Never start dev servers manually** — always go through `dev.sh`. The script knows env files, ports, log paths, and PID tracking.
- **Never kill processes by name** — use the PID files in `tmp/dev-pids/`.
- **Never use ports outside the documented dev range.** Worktree pipelines own everything else.
