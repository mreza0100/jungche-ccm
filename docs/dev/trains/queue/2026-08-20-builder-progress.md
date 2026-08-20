# LUNA_BUILDER progress

- task #1 · files: `pfm/internal/config/config.go`, `pfm/internal/config/config_test.go`, `pfm/cmd/pfm/config_command.go`, `pfm/cmd/pfm/runtime_config.go`, `pfm/cmd/pfm/main.go`, `pfm/cmd/pfm/doctor.go`, `pfm/internal/action/synth.go`, `pfm/internal/action/headless.go`, `pfm/cmd/pfm/agent_open_command.go`, `pfm/cmd/pfm/reload_command.go`, `pfm/cmd/pfm/install_command.go`, `pfm/internal/installer/{types.go,assets.go,installer.go,settings_wiring_test.go,installer_test.go}`, `pfm/internal/installer/assets/shim/pfm.zsh` · tests: config tests, posture-rendering tests, explicit configured-directory fanout tests, targeted installer tests, and the full pfm gate passed · deviations: none; the shim emits per-account posture maps and wireSettings consumes the command's Config.Accounts-derived ConfigDirs.
- task #2 · files: `pfm/internal/theme/theme.go`, `pfm/internal/theme/theme_test.go`, `pfm/internal/naming/{label.go,label_test.go}`, `pfm/internal/ui/types.go`, `pfm/internal/ui/model.go`, `pfm/internal/ui/render.go`, `pfm/cmd/pfm/pipeline.go`, `pfm/cmd/pfm/statusline_command.go`, `pfm/internal/statusline/{runtime.go,render.go}`, `pfm/internal/resolve/{resolve.go,resolve_test.go}`, `pfm/cmd/pfm/whoami_command.go` · tests: configured-label naming and resolver tests were added; targeted naming/resolver tests and the full pfm gate passed · deviations: unknown themes use the specified default-with-visible-warning fallback; legacy medals remain accepted alongside configured emojis.
- task #3 · files: `pfm/internal/compose/compose.go`, `pfm/internal/compose/compose_test.go`, `pfm/internal/statusline/render.go`, `pfm/internal/statusline/runtime.go`, `pfm/internal/statusline/statusline_test.go`, `pfm/cmd/pfm/pipeline.go` · tests: symlinked compose attribution and unknown statusline config-dir red tests were written first; the full `pfm` suite reached the later UI compile gate · deviations: none recorded.
- task #4 · files: `pfm/internal/ui/types.go`, `pfm/internal/ui/model.go`, `pfm/internal/ui/render.go`, `pfm/internal/ui/model_test.go`, `pfm/internal/ui/stats_test.go`, `pfm/internal/ui/golden_test.go` fixtures, `pfm/cmd/pfm/commands.go` · tests: red pure-UI carousel tests first; `./.claude/scripts/dev.sh verify pfm` passed; `./.claude/scripts/dev.sh test pfm` passed all packages; `./.claude/scripts/dev.sh build pfm` and installed command build passed · deviations: merged new-chat behavior uses the spec-permitted UI-level merge flag, leaving plain/TSV compose rows unchanged.
- task #5 · files: `pfm/internal/resolve/whoami.go`, `pfm/internal/resolve/procfs_linux.go`, `pfm/internal/resolve/procfs_darwin.go`, `pfm/internal/resolve/procfs_other.go`, `pfm/internal/resolve/whoami_test.go`, `.github/workflows/verify.yml` · tests: fake-ProcFS parent test was written and run red first; Darwin cross-vet passed; `./.claude/scripts/dev.sh all pfm` passed build, vet, and the full test suite; installed command build passed · deviations: installer launchd call sites were already fully gated by `schedulerIsLaunchd`, so no installer source change was necessary.
- task #6 · files: `pfm/internal/usagehook/hook.go`, `pfm/internal/usagehook/hook_test.go`, `pfm/internal/usagehook/testdata/usage-fable.json`, `pfm/internal/stats/{stats.go,limits.go,limits_test.go}`, `pfm/internal/ui/types.go`, `pfm/internal/ui/render.go`, `pfm/internal/ui/stats_test.go` and stats golden fixtures, `pfm/internal/statusline/{render.go,statusline_test.go}`, `pfm/cmd/pfm/commands.go` · tests: red usage/stats/UI tests first; the ACK fallback regression now proves one attempt per account across cache expiry and repeated failure; targeted stats tests and the full pfm gate passed · deviations: no authenticated live payload was available for the probe; unknown windows are retained and rendered generically as specified.
- task #7 · files: `pfm/internal/reload/reload.go`, `pfm/internal/reload/reload_test.go`, `pfm/internal/reload/config_policy_test.go`, `pfm/cmd/pfm/reload_command.go`, `pfm/cmd/pfm/reload_command_test.go`, `pfm/cmd/pfm/swap_jail_test.go`, `pfm/cmd/pfm/headless_command.go`, `pfm/cmd/pfm/main.go`, `pfm/internal/action/executor.go`, `pfm/internal/action/executor_test.go`, `pfm/internal/installer/installer.go`, `pfm/internal/installer/reload_test.go`, `pfm/internal/installer/assets/reload.command.md`, public docs and `pfm/TESTPLAN.md` · tests: canonical reload and legacy-command migration tests were written first and failed; `./.claude/scripts/dev.sh all pfm` passed build, vet, and the full test suite; installed command build passed · deviations: `swap` remains only as the specified hidden dispatch alias and legacy command-file cleanup target.
- task #8 · files: `pfm/internal/chatkeys/keys.go`, `pfm/cmd/pfm/chat_keys_command.go`, `pfm/internal/mcpserv/{server.go,backend.go,actions.go,read.go,search.go,types.go}`, `pfm/internal/mcpserv/{shared_operations_test.go,shared_fixture_test.go,server_test.go,whoami_find_test.go,httpserv_test.go}`, `pfm/cmd/pfm/{main.go,mcp_shared.go,headless_command.go,chat_satellite_command.go,mcp_serve_command.go,mcp_serve_test.go}`, `pfm/internal/harvestmcp/{service.go,httpserv_test.go}`, `pfm/internal/installer/assets/{systemd/pfm-mcp.service,launchd/com.professor.pfm.mcp.plist}` · tests: shared-operation and in-process-dispatch tests were written first and failed before the seam existed; targeted MCP/CLI tests passed; jailed stdio surface and chat family passed; `./.claude/scripts/dev.sh all pfm` passed build, vet, and the full suite (all packages green) · deviations: none; CLI and MCP now share in-process list/find/read callbacks and the stateful chat dispatcher, with no executable delegation.
- task #9 · files: `pfm/internal/harvest/cache_roundtrip_test.go`, `pfm/cmd/pfm/doctor.go` · tests: local-httptest round-trip/mtime test was written first and initially failed on the intended private-host guard, then passed through a public-host transport shim; younger/older/zero-TTL pins pass; `./.claude/scripts/dev.sh all pfm` passed build, vet, and the full suite (all packages green); installed command build passed · deviations: no production cache fix was needed because the current source already reads the cache correctly; the regression comment records the stale-binary diagnosis, and doctor counts cache files without changing cache semantics.
- task #10 · files: `pfm/cmd/pfm/explore_deny_command.go`, `pfm/internal/installer/{settings.go,installer.go,types.go,assets.go,mcp.go,settings_wiring_test.go,mcp_wiring_test.go,installer_test.go}`, `pfm/internal/config/config.go`, `pfm/internal/codexgen/globalagents.go`, MCP unit/client assets and related tests · tests: hook, cleanup, Codex-agent, MCP credential/registration, and ownership tests plus `./.claude/scripts/dev.sh all pfm` passed build, vet, and the full suite · deviations: none recorded.
- task #15 · files: `pfm/cmd/pfm/{epic_inject_command.go,epic_inject_command_test.go,main.go}`, `pfm/internal/store/{epic.go,epic_test.go,migration_v5.sql,store.go}` · tests: dedupe fixture was written first and failed before the command/store contract existed; matching, same-epic, rename, rename-back, and structured-store tests pass · deviations: git-root discovery uses filesystem `.git` markers to honor the no-git-command law.
- task #17 · files: `pfm/internal/ask/{ask.go,ask_test.go}` · tests: contract tests were written first and failed to compile before the package existed; config precedence, both stub engines, sentinel error, and fake transcript/harvester adapter tests now pass through `ResolveInput`, `BuildPrompt`, and the shared `Evidence` types · deviations: runners remain the specified `ErrNotImplemented` stubs.
- task #12 · files: `pfm/cmd/pfm/{update_command.go,update_command_test.go,init_command.go,install_command.go,main.go,main_test.go,doctor.go}`, `pfm/internal/installer/{update_metadata.go,types.go,installer.go}`, `pfm/internal/store/store_test.go`, `INSTALL.md` · tests: temp-repository dirty refusal, parsed v0.10.0-over-v0.9.0 selection, owned/unowned replacement, doctor-after-success, injected rollback, and init refusal/force tests passed; `go -C pfm vet ./...`, `go -C pfm test ./...`, and installed build passed · deviations: runtime update shells only to git fetch/tag/status/checkout in the selected clone; fixture git commands use only t.TempDir repositories as authorized, and install/uninstall now records/removes the source marker and canonical ownership ledger.

## Final

1. All files changed — full ledger:

   - #1: `pfm/internal/config/{config.go,config_test.go}`, `pfm/cmd/pfm/{config_command.go,runtime_config.go,main.go,doctor.go,agent_open_command.go,reload_command.go,install_command.go}`, `pfm/internal/action/{synth.go,headless.go}`, `pfm/internal/installer/{types.go,assets.go,installer.go,settings_wiring_test.go,installer_test.go}`, `pfm/internal/installer/assets/shim/pfm.zsh`.
   - #2: `pfm/internal/theme/{theme.go,theme_test.go}`, `pfm/internal/naming/{label.go,label_test.go}`, `pfm/internal/ui/{types.go,model.go,render.go}`, `pfm/cmd/pfm/{pipeline.go,statusline_command.go,whoami_command.go}`, `pfm/internal/statusline/{runtime.go,render.go}`, `pfm/internal/resolve/{resolve.go,resolve_test.go}`.
   - #3–#4: `pfm/internal/compose/{compose.go,compose_test.go}`, `pfm/internal/statusline/{render.go,runtime.go,statusline_test.go}`, `pfm/internal/ui/{types.go,model.go,render.go,model_test.go,stats_test.go,golden_test.go}`, `pfm/cmd/pfm/{pipeline.go,commands.go}`.
   - #5: `pfm/internal/resolve/{whoami.go,procfs_linux.go,procfs_darwin.go,procfs_other.go,whoami_test.go}`, `.github/workflows/verify.yml`.
   - #6: `pfm/internal/usagehook/{hook.go,hook_test.go,testdata/usage-fable.json}`, `pfm/internal/stats/{stats.go,limits.go,limits_test.go}`, `pfm/internal/ui/{types.go,render.go,stats_test.go}`, `pfm/internal/statusline/{render.go,statusline_test.go}`, `pfm/cmd/pfm/commands.go` and the changed stats/statusline golden fixtures.
   - #7: `pfm/internal/reload/{reload.go,reload_test.go,config_policy_test.go}`, `pfm/cmd/pfm/{reload_command.go,reload_command_test.go,swap_jail_test.go,headless_command.go,main.go}`, `pfm/internal/action/{executor.go,executor_test.go}`, `pfm/internal/installer/{installer.go,reload_test.go}`, `pfm/internal/installer/assets/reload.command.md`, `pfm/TESTPLAN.md` and the public reload documentation.
   - #8: `pfm/internal/chatkeys/keys.go`, owner-supplied `pfm/cmd/pfm/{chat_keys_command.go,chat_keys_jail_test.go,headless_command.go,run_command.go,run_jail_test.go,two_way_jail_test.go}`, `pfm/internal/mcpserv/{server.go,backend.go,actions.go,read.go,search.go,types.go,shared_operations_test.go,shared_fixture_test.go,server_test.go,whoami_find_test.go,httpserv_test.go}`, `pfm/cmd/pfm/{main.go,mcp_shared.go,chat_satellite_command.go,mcp_serve_command.go,mcp_serve_test.go}`, `pfm/internal/harvestmcp/{service.go,httpserv_test.go}`, and the MCP installer assets/tests.
   - #9–#10: `pfm/internal/harvest/cache_roundtrip_test.go`, `pfm/cmd/pfm/{doctor.go,explore_deny_command.go}`, `pfm/internal/installer/{settings.go,installer.go,types.go,assets.go,mcp.go,settings_wiring_test.go,mcp_wiring_test.go,installer_test.go}`, `pfm/internal/config/config.go`, `pfm/internal/codexgen/globalagents.go`, and the MCP unit/client assets and related tests.
   - #12: `pfm/cmd/pfm/{update_command.go,update_command_test.go,init_command.go,install_command.go,main.go,main_test.go,doctor.go}`, `pfm/internal/installer/{update_metadata.go,types.go,installer.go}`, `pfm/internal/store/store_test.go`, `INSTALL.md`.
   - #15: `pfm/cmd/pfm/{epic_inject_command.go,epic_inject_command_test.go,main.go}`, `pfm/internal/store/{epic.go,epic_test.go,migration_v5.sql,store.go}`.
   - #17: `pfm/internal/ask/{ask.go,ask_test.go}`; the progress file itself is `docs/dev/trains/queue/2026-08-20-builder-progress.md`.

2. Behavioral changes: config-driven account posture and custom emoji attribution; the pure UI picker carousel and stats/statusline surfaces; reload rescue; complete installer hooks, MCP wiring, Codex agents, cleanup, and cache diagnostics; in-process shared CLI/MCP chat operations; transactional tagged `pfm update` with source/ownership markers; marker-backed `pfm init`; structured epic injection dedupe; and the content-agnostic ask contract with fake transcript/harvester preparation tests and dual stub engines.

3. Full test result:

```text
$ go -C pfm test ./...
ok   hostops/pfm/cmd/pfm (cached)
ok   hostops/pfm/internal/action (cached)
ok   hostops/pfm/internal/agentopen (cached)
ok   hostops/pfm/internal/archive (cached)
ok   hostops/pfm/internal/ask 0.046s
?    hostops/pfm/internal/chatkeys [no test files]
ok   hostops/pfm/internal/codexgen (cached)
ok   hostops/pfm/internal/compose (cached)
ok   hostops/pfm/internal/config (cached)
ok   hostops/pfm/internal/dream 13.095s
ok   hostops/pfm/internal/dream/apply (cached)
ok   hostops/pfm/internal/dream/artifact (cached)
ok   hostops/pfm/internal/dream/corpus (cached)
ok   hostops/pfm/internal/dream/gate (cached)
ok   hostops/pfm/internal/dream/lane (cached)
ok   hostops/pfm/internal/dream/organ (cached)
ok   hostops/pfm/internal/dream/resources (cached)
ok   hostops/pfm/internal/dream/seat (cached)
ok   hostops/pfm/internal/gather (cached)
ok   hostops/pfm/internal/harvest (cached)
ok   hostops/pfm/internal/harvestmcp (cached)
ok   hostops/pfm/internal/harvestpy (cached)
ok   hostops/pfm/internal/headless (cached)
ok   hostops/pfm/internal/heal (cached)
ok   hostops/pfm/internal/hide (cached)
ok   hostops/pfm/internal/index (cached)
ok   hostops/pfm/internal/inject (cached)
ok   hostops/pfm/internal/installer (cached)
ok   hostops/pfm/internal/mcpserv (cached)
ok   hostops/pfm/internal/naming (cached)
ok   hostops/pfm/internal/paths (cached)
ok   hostops/pfm/internal/reap (cached)
ok   hostops/pfm/internal/recovery (cached)
ok   hostops/pfm/internal/reload (cached)
ok   hostops/pfm/internal/resolve (cached)
ok   hostops/pfm/internal/shared (cached)
ok   hostops/pfm/internal/sky (cached)
ok   hostops/pfm/internal/spawn (cached)
ok   hostops/pfm/internal/stats (cached)
ok   hostops/pfm/internal/statusline (cached)
ok   hostops/pfm/internal/store (cached)
?    hostops/pfm/internal/testjail [no test files]
ok   hostops/pfm/internal/theme (cached)
ok   hostops/pfm/internal/tmuxfmt (cached)
ok   hostops/pfm/internal/transcript (cached)
ok   hostops/pfm/internal/ui (cached)
ok   hostops/pfm/internal/usagehook (cached)
ok   hostops/pfm/prompts (cached)
ok   hostops/pfm/shim (cached)
```

`go -C pfm vet ./...` and `go -C pfm build -o ~/.local/bin/pfm ./cmd/pfm` also passed.

4. Remaining spec↔implementation mismatches: none within the requested tasks. #11, #13/#14/#16, and the explicitly next-wave ask-runner/engine bodies remain skipped by instruction; the `ErrNotImplemented` engine bodies are intentional.
