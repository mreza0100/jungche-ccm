package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/installer"
)

// installHarvestProvisioner is nil in production and resolves to the real
// pinned adapter. The command-package TestMain replaces it with a no-network
// fake so existing CLI wiring tests never download the conversion lock.
var installHarvestProvisionerOverride installer.HarvestProvisioner

func installHarvestProvisioner() installer.HarvestProvisioner {
	if installHarvestProvisionerOverride != nil {
		return installHarvestProvisionerOverride
	}
	return installer.NewHarvestProvisioner()
}

func runInstall(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet(
		"install",
		"usage: pfm install [--apply | --uninstall | --dry-run] [--force] [--config-dir DIR]",
		stderr,
	)
	apply := flags.Bool("apply", false, "apply the installation")
	uninstall := flags.Bool("uninstall", false, "remove installed links and restore backups")
	dryRun := flags.Bool("dry-run", false, "preview the installation without changing files")
	force := flags.Bool("force", false, "regenerate installer-owned credentials")
	configDir := flags.String("config-dir", "", "target config directory instead of ~/.claude")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 || boolCount(*apply, *uninstall, *dryRun) > 1 {
		flags.Usage()
		return 2
	}
	mode := installer.ModeDryRun
	if *apply {
		mode = installer.ModeApply
	} else if *uninstall {
		mode = installer.ModeUninstall
	}
	options := installer.Options{
		Mode:               mode,
		ConfigDir:          *configDir,
		SourceRepo:         discoverSourceRepo(),
		Force:              *force,
		Stdout:             stdout,
		ProvisionHarvest:   true,
		HarvestProvisioner: installHarvestProvisioner(),
	}
	if len(runtimes) != 0 {
		runtime := runtimes[0]
		options.Home = runtime.Paths.Home
		options.MCPEnabled = make(map[string]bool, len(runtime.Config.MCPServers))
		for name, server := range runtime.Config.MCPServers {
			options.MCPEnabled[name] = server.Enabled
		}
		options.MCPPort = runtime.Config.MCP.HTTP.Port
		options.MCPAuthToken = runtime.Config.MCP.AuthToken
		options.MCPConfigPath = runtime.Config.Path
		options.ClaudePrompted = make(map[int]bool, len(runtime.Config.Accounts))
		options.CodexYolo = make(map[int]bool, len(runtime.Config.Accounts))
		for _, account := range runtime.Config.Accounts {
			options.ClaudePrompted[account.ID] = runtime.Config.EffectiveClaude(account.ID).PermissionMode == pfmconfig.PermissionPrompt
			options.CodexYolo[account.ID] = runtime.Config.EffectiveCodex(account.ID).Yolo
		}
		if *configDir == "" {
			options.ConfigDirs = append(options.ConfigDirs, runtime.Paths.Home+"/.claude")
			for _, account := range runtime.Config.Accounts {
				options.ConfigDirs = append(options.ConfigDirs, account.ConfigDir)
			}
		}
	}
	_, err := installer.Run(context.Background(), options)
	if errors.Is(err, installer.ErrNameSyncRunning) {
		fmt.Fprintln(stderr, "pfm install: the pfm name-sync service is running; wait for it to finish or run `systemctl --user stop pfm-name-sync.service`, then retry")
		return 97
	}
	if errors.Is(err, installer.ErrLaunchAgentRunning) {
		fmt.Fprintln(stderr, "pfm install: the pfm name-sync launch agent is running; wait for it to finish or `launchctl bootout gui/$(id -u)/com.professor.pfm.name-sync` first")
		return 97
	}
	if err != nil {
		fmt.Fprintf(stderr, "pfm install: %v\n", err)
		return 1
	}
	return 0
}
