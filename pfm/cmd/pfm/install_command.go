package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/deps"
	"hostops/pfm/internal/installer"
)

// installHarvestProvisioner is nil in production and resolves to the real
// pinned adapter. The command-package TestMain replaces it with a no-network
// fake so existing CLI wiring tests never download the conversion lock.
var installHarvestProvisionerOverride installer.HarvestProvisioner

var runInstaller = installer.Run

func installHarvestProvisioner() installer.HarvestProvisioner {
	if installHarvestProvisionerOverride != nil {
		return installHarvestProvisionerOverride
	}
	return installer.NewHarvestProvisioner()
}

func runInstall(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet(
		"install",
		"usage: pfm install [--yes] [--vscode] [--skip-harvest] [--force] [--config-dir DIR]",
		stderr,
	)
	yes := flags.Bool("yes", false, "apply the installation")
	vscode := flags.Bool("vscode", false, "make PFM the default VS Code terminal profile")
	skipHarvest := flags.Bool("skip-harvest", false, "skip harvestpy provisioning")
	force := flags.Bool("force", false, "reconcile installer-owned wiring")
	configDir := flags.String("config-dir", "", "target config directory instead of ~/.claude")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	mode := installer.ModeDryRun
	if *yes {
		mode = installer.ModeApply
	}
	runtime, runtimeErr := optionalCommandRuntime(runtimes)
	if runtimeErr != nil {
		fmt.Fprintf(stderr, "pfm install: resolve dependency config: %v\n", runtimeErr)
		return 1
	}
	entries := deps.Registry(deps.Options{
		Home: runtime.Paths.Home, ClaudeBinary: runtime.Config.Claude.Binary, CodexBinary: runtime.Config.Codex.Binary,
		ClaudeAccounts: len(runtime.Config.Accounts), CodexAccounts: len(runtime.Config.CodexAccounts),
	})
	preflight := printDependencyDoctor(context.Background(), stdout, entries, deps.ProbeOptions{
		SkipHarvest: *skipHarvest, Provisioning: true,
	})
	if preflight != 0 && mode == installer.ModeApply {
		fmt.Fprintln(stderr, "pfm install: required dependency preflight failed")
		return 1
	}
	options := newInstallerOptions(mode, *configDir, *force, *skipHarvest, stdout, runtime)
	options.VSCode = *vscode
	code := runInstallerCommand("install", options, stderr)
	if code == 0 && mode == installer.ModeDryRun {
		if preflight != 0 {
			fmt.Fprintln(stderr, "pfm install: required dependency preflight failed — the preview above is read-only; fix the dependencies it names before applying")
			return 1
		}
		confirmation := "if you agree, run again: pfm install --yes"
		if *vscode {
			confirmation += " --vscode"
		}
		if *skipHarvest {
			confirmation += " --skip-harvest"
		}
		fmt.Fprintln(stdout, confirmation)
	}
	return code
}

func newInstallerOptions(
	mode installer.Mode,
	configDir string,
	force bool,
	skipHarvest bool,
	stdout io.Writer,
	runtimes ...commandRuntime,
) installer.Options {
	options := installer.Options{
		Mode:               mode,
		ConfigDir:          configDir,
		SourceRepo:         discoverSourceRepo(),
		Force:              force,
		Stdout:             stdout,
		ProvisionHarvest:   !skipHarvest,
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
		options.MCPConfigPath = runtime.Config.Path
		options.ClaudeBinary = runtime.Config.Claude.Binary
		options.ClaudePrompted = make(map[int]bool, len(runtime.Config.Accounts))
		options.CodexYolo = make(map[int]bool, len(runtime.Config.CodexAccounts))
		for _, account := range runtime.Config.Accounts {
			options.ClaudePrompted[account.ID] = runtime.Config.EffectiveClaude(account.ID).PermissionMode == pfmconfig.PermissionPrompt
		}
		options.CodexHomes = make([]string, 0, len(runtime.Config.CodexAccounts))
		for _, account := range runtime.Config.CodexAccounts {
			options.CodexHomes = append(options.CodexHomes, account.Home)
			options.CodexYolo[account.ID] = runtime.Config.EffectiveCodex(account.ID).Yolo
		}
		if configDir == "" {
			options.ConfigDirs = make([]string, 0, len(runtime.Config.Accounts))
			for _, account := range runtime.Config.Accounts {
				options.ConfigDirs = append(options.ConfigDirs, account.ConfigDir)
			}
		}
	}
	return options
}

func runInstallerCommand(command string, options installer.Options, stderr io.Writer) int {
	_, err := runInstaller(context.Background(), options)
	if errors.Is(err, installer.ErrNameSyncRunning) {
		fmt.Fprintf(stderr, "pfm %s: the pfm name-sync service is running; wait for it to finish or run `systemctl --user stop pfm-name-sync.service`, then retry\n", command)
		return 97
	}
	if errors.Is(err, installer.ErrLaunchAgentRunning) {
		fmt.Fprintf(stderr, "pfm %s: the pfm name-sync launch agent is running; wait for it to finish or `launchctl bootout gui/$(id -u)/com.professor.pfm.name-sync` first\n", command)
		return 97
	}
	if err != nil {
		fmt.Fprintf(stderr, "pfm %s: %v\n", command, err)
		return 1
	}
	return 0
}
