package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/deps"
	pfmengine "hostops/pfm/internal/engine"
	"hostops/pfm/internal/installer"
)

// installHarvestProvisioner is nil in production and resolves to the real
// pinned adapter. The command-package TestMain replaces it with a no-network
// fake so existing CLI wiring tests never download the conversion lock.
var installHarvestProvisionerOverride installer.HarvestProvisioner
var installThemeHTTPClientOverride *http.Client

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
		"usage: pfm install [--yes] [--vscode] [--skip-harvest] [--skip-engine codex] [--skip-themes] [--config-dir DIR]",
		stderr,
	)
	yes := flags.Bool("yes", false, "apply the installation")
	vscode := flags.Bool("vscode", false, "make PFM the default VS Code terminal profile")
	skipHarvest := flags.Bool("skip-harvest", false, "skip harvestpy provisioning")
	skipEngine := flags.String("skip-engine", "", "skip one optional engine (supported: codex)")
	skipThemes := flags.Bool("skip-themes", false, "skip source-fetched Claude Code themes")
	configDir := flags.String("config-dir", "", "target config directory instead of ~/.claude")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	skipCodex := false
	if value := strings.TrimSpace(*skipEngine); value != "" {
		id, err := pfmengine.Parse(value)
		if err != nil || id != pfmengine.Codex {
			fmt.Fprintf(stderr, "pfm install: --skip-engine supports only codex, got %q\n", value)
			return 2
		}
		skipCodex = true
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
	})
	preflight := printDependencyDoctor(context.Background(), stdout, entries, deps.ProbeOptions{
		SkipHarvest: *skipHarvest, SkipEngines: map[pfmengine.ID]bool{pfmengine.Codex: skipCodex}, Provisioning: true,
	})
	if preflight != 0 && mode == installer.ModeApply {
		fmt.Fprintln(stderr, "pfm install: required dependency preflight failed")
		return 1
	}
	options := newInstallerOptions(mode, *configDir, *skipHarvest, stdout, runtime)
	options.VSCode = *vscode
	options.InstallThemes = !*skipThemes
	options.ThemeManifestURL = professorThemeManifestURL(version)
	options.ThemeHTTPClient = installThemeHTTPClientOverride
	if skipCodex {
		options.CodexHomes = []string{}
		options.CodexYolo = map[int]bool{}
	}
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
		if skipCodex {
			confirmation += " --skip-engine codex"
		}
		if *skipThemes {
			confirmation += " --skip-themes"
		}
		fmt.Fprintln(stdout, confirmation)
	}
	return code
}

func professorThemeManifestURL(currentVersion string) string {
	reference := strings.TrimSpace(currentVersion)
	if reference == "" || reference == "dev" {
		reference = "main"
	}
	return "https://raw.githubusercontent.com/mreza0100/professor/" + reference + "/templates/themes/sources.json"
}

func newInstallerOptions(
	mode installer.Mode,
	configDir string,
	skipHarvest bool,
	stdout io.Writer,
	runtimes ...commandRuntime,
) installer.Options {
	options := installer.Options{
		Mode:               mode,
		ConfigDir:          configDir,
		SourceRepo:         discoverSourceRepo(),
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
		options.CodexBinary = runtime.Config.Codex.Binary
		if options.CodexBinary == "" {
			options.CodexBinary = pfmengine.MustLookup(pfmengine.Codex).Binary
		}
		options.NameSyncInterval = runtime.Config.NameSync.Interval
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
