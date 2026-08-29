# Release — framework changes pending publication

Bullets here are FINAL changelog entries. `/ptm:release` copies them verbatim into
`releases/vX.Y.Z.md`, then clears this file, keeping this header.

Shape: `- {Tier}: {scope} — {semantic change}`, plus a `#### → For:` line when adopters must act,
and `(cost)` on any env / hook / permission / model-config delta.

## Pending

- A: `templates/CLAUDE.md` — docs-map example gains the truth hierarchy (code truth: grep; schema truth: introspect the live DB) + `docs/runbooks/{project}/`, `docs/features/`, `docs/references/`, `docs/business/` trees; /documenter rule routes officer/mentor/marketer output to `docs/business/{compliance,marketing}/`
- B: officer/mentor/marketer/pm/jc commands — owned-document paths migrate `$CDOCS/{cmd}/$REFS/` → `docs/business/{compliance,marketing}/` and `docs/agents/features/` → `docs/features/`
- A: wave commands — builder reads the spec spine (`spec.md` index + `tasks/T{n}.md` per task, monolithic fallback); refine fans out `tracer` agents and splits architect findings into auto-folded gaps vs user-ruled judgment; live appends lane events to `tmp/wave-sensor/events.log` as a guaranteed-wake fallback; walker follows threads past the diff's edge
- C: scheduler agent — writes the spec spine (task index + `tasks/T{n}.md` opening with `Binds:`); architect agent — new Zero-gap walk section; per-project qa — parallelism is a test-health invariant (`{PARALLEL_FLAG}`; never lower workers to pass)
- C: scripts — `dev.sh` health probe rejects a stale listener answering for a dead service (false-GREEN fix) and `up` exits 1 on any RED; `codex-sync.sh` resolves `pfm` off-PATH; `km-guard.sh` stops false-flagging prose arrows; `alloc-ports.sh` metrics-column semantics; `format-md.sh` doc-tree allow-list + `AGENTS.md`
- C: codex layer — `config.toml` comment documents the generated `mcp_servers` fence (`pfm codex build` owns it); `repo-law.rules` read-only-git list matches canon
- A: `p/rnd.md` — sandbox layout (`{family}/{N}-{goal}`) + self-contained harness sections; Step 0 baseline rule

#### → For: adopters with existing officer/mentor/marketer documents under `docs/commands/{cmd}/references/` — move them to `docs/business/{compliance,marketing}/`; the cards now read/write there.
