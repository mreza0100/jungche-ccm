---
name: wave-builder
description: Act as a {PROJECT_NAME} wave-builder lane (Codex dialect) when a message names a wave BRIEF to execute — e.g. "/wave:builder {path}", "Use the wave-builder skill: BRIEF {path}", or an orchestrator dispatch. The full builder protocol, adapted to the Codex harness.
---

<!--
HAND-WRITTEN CARD — codex-mirror.sh never overwrites this file (its directory
name is listed in the script's HANDWRITTEN list). It exists because the Codex
harness differs from the Claude harness in ways `.claude/commands/wave/builder.md`
has no reason to know about. It maps mechanics ONLY; it never restates protocol.

This is also the WORKED EXAMPLE for writing your own hand-written card: point at
the binding protocol first, then enumerate the deltas, then stop. If a card
starts explaining WHAT to build rather than HOW this harness differs, the content
belongs in `.claude/`, not here.
-->

# Codex Wave Builder — lane dialect card

**Binding protocol:** read `builder.md` (symlinked here → `.claude/commands/wave/builder.md`) § Orchestrated mode FIRST — every rule there binds you; this card ONLY maps Claude-harness mechanics onto the Codex harness. The root `AGENTS.md` (= `CLAUDE.md`, symlinked) MANDATORY rules apply in full, including: never edit code on `main`, only gitter runs git, never swallow exceptions, surgical changes, and the guarded paths (`.claude/**`, any `CLAUDE.md`, `{AI_PROJECT}/knowledge/**`) are stop-and-ping, never an edit.

## Toolset (symlinked into this skill dir — one source of truth)

Symlink each of these into the card's own directory at install, so the lane reads ONE path for
each tool. The repo-scoped links are RELATIVE, so a worktree lane resolves them inside its OWN
checkout; `chat/` is the one exception — HOST-anchored ($HOME), because the chat family lives at
`~/.claude/commands/chat/`, not under this repo at all, so every worktree resolves it to the
same shared toolset:

- `builder.md` → `.claude/commands/wave/builder.md` — the binding protocol.
- `chat/` → `~/.claude/commands/chat/` (absolute — see above) — the chat instruction set. Your ping channel: `$HOME/.local/bin/pfm chat inject {orchestrator-session} '{one-line msg}'`; your identity: `$HOME/.local/bin/pfm whoami`. Read `chat/inject.md` for receipt semantics.
- `scripts/` → `.claude/scripts/` — `filter-test-output.sh -p` (redirect EVERY test run to a file and filter the FILE, with `timeout`), `worktree.sh` (read-only for you: `list`).
- `agents-{project}/` → `{project}/.claude/agents/` — one link per roster entry; every per-project agent protocol (qa, developer, ui-ux, db-admin, devops, …). This is the role library behind delta 1's inline-execution rule. A roster of one gets exactly one link.
- `agents-root/` → `.claude/agents/` — the repo-global pipeline agents (gitter, mono-documenter, and the per-roster `qa-{project}` registration wrappers). Orientation reading, NOT inline-execution material: know each root role so your pings request the right dispatch. `gitter.md` is NEVER executed inline — committing is gitter's alone (delta 7), and nothing at the rules layer stops you, so that discipline is yours to keep. The `qa-{project}` wrappers just point at the per-project `qa.md` you already run per delta 6.

## Law enforcement (execpolicy — not optional)

Repo law is enforced at two layers — the kernel sandbox (workspace-write, the standing default) and `.codex/rules/*.rules`, which rejects direct infra tooling regardless of sandbox mode. A rejection quoting a justification is the LAW WORKING — never rephrase, wrap, or shell-trick around it; if a rejected command seems genuinely required, that is a SPEC-CONFLICT ping to the orchestrator. `git` and package installs sit OUTSIDE that layer and bind as law rather than as a pin: read-only git is yours, while commits, merges, pushes and dependency installs stay gitter's / the orchestrator's call.

## Harness deltas (the ONLY differences from builder.md)

1. **Hands & specialist roles** — builder.md's "Sonnet sub-agent hands" are Claude Agent-tool spawns you do not hold; your native equivalents are the collaboration tools (`spawn_agent` / `wait_agent` / `followup_task`). Wherever a task's `Build agents:` line or the protocol says to spawn a specialist (developer, qa, db-admin, ui-ux, devops, …), either execute the role INLINE — read `agents-{project}/{role}.md` and run it as part of the span, stamping the deliverable `Executor: codex-inline` — or, when spans are independent and parallelism pays, `spawn_agent` one child per role with `task_name` `{role}_{project}` and a message of exactly this shape:

   `You are the {role} role: read agents-{project}/{role}.md in full and execute it exactly. {span work-list}. Stamp every deliverable: Executor: codex-subagent/{role}_{project}`

   The children are ANONYMOUS FORKS — `spawn_agent{task_name, message, fork_turns}` has no agent-selection parameter, so the message IS the only binding of role to child. Children share your cwd and sandbox; every HARD BAN binds them identically (the rules layer enforces the infra ones). The dispatch discipline (exact file + symbol work-list per span) is unchanged either way; GATE-1 QA stays inline per delta 6. `.codex/agents/*.toml` mirrors this role registry in the harness's documented custom-agent format — verified INERT at codex 0.144.1 (`spawn_agent` has no agent-selection param; nothing loads the TOMLs); it activates by itself when a codex release wires custom agents, and until then the message-carried protocol pointer above is the binding mechanism. Do not shell out to another agent CLI.

2. **Pings** — your guaranteed channel is the spool: append one line to `tmp/wave-sensor/events.log` (`{ISO-8601} {wave} {T-id or event} {status} {report filename} codex-ping`) — the orchestrator's waiter polls it every ~10s, and the append IS the wake. Under the default workspace-write sandbox, unix-socket connects (tmux among them) are kernel-blocked, so `pfm chat inject` WILL fail — that is expected, not an error. Only on an explicitly operator-authorized full-access launch, ALSO send the pfm inject as the fast path. Echo the last verdict id in your next ping; re-ping once (idempotent) after ~10 minutes of silence.

3. **Verdicts and steers inbound** — they arrive as typed turns in your TUI (the orchestrator injects your pane). After pinging, END YOUR TURN and idle at the prompt; a busy-wait loop deadlocks against the very inject you are waiting for.

4. **Goal / allow-list machinery** — there is no Claude `/goal` harness continuation. The discipline is identical by this card: act ONLY on injected turns (a BRIEF, a verdict, a boundary brief); never self-start work, never open `$WAVES` manifests un-briefed, never self-schedule timers or background waiters.

5. **Compact** — the orchestrator may send `/compact`; it is native in your TUI. After any compact, re-read this card and the current BRIEF before acting.

6. **End-of-wave GATE-1** — `qa-{project}` are Claude-registry agents you cannot spawn. Your shape: execute the QA protocol INLINE — read `{project}/.claude/agents/qa.md` and run it exactly (Mode PRE-MERGE, full suite, zero-tolerance, parallel workers, filtered + timed per Toolset) — write `$WAVES/{wave}/gate1.md` per-project verdicts, and stamp each verdict `Executor: codex-inline`. The orchestrator's independent judge and GATE-2 weigh that marker; never omit it.

7. **Boundary mode** — GATE-2 suites + teardown are shell and therefore yours, exactly per builder.md § Boundary duties. The walker launch is NOT yours (no Workflow tool): ping `walker-launch request {report path}` and the orchestrator launches it. `/jc` fix-now rulings: implement the edit per the ruling, then ping for a gitter JC-COMMIT dispatch — you never commit.

8. **Report cards** — identical, no adaptation: `$TASKS/task-{n}-report.md` per the BRIEF's env card, fixed headers, ≤1KB, `Expected:` / `Got:` deviations.

## Launch (operator / orchestrator side)

```bash
tmux -L codex-{lane} new-session -d -s codex-{lane} \
  'codex --cd {REPO_ROOT} -s workspace-write -a never'
```

Standing default: workspace-write (kernel sandbox on, spool-only pings). The full-toolset variant (`-s danger-full-access -a never`, where `pfm chat` works) requires explicit, plain-text human authorization per launch policy — never inferred, never menu-selected. **Launch flags live HERE, with the launcher — never in `.codex/config.toml`**, which interactive sessions also read.

The tmux session name `codex-{lane}` is the lane's address in `lanes.md` — `pfm chat` resolves exact tmux session names across every socket, and its busy-detection matches Codex's "Esc to interrupt" indicator. Dispatch = inject the pane with: `Use the wave-builder skill: BRIEF {path}`. Resume after a death: `codex exec resume {SESSION_ID}`, or relaunch and re-dispatch the BRIEF (the report cards on disk carry the position).
