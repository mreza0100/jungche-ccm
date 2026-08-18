package main

import (
	"context"
	"errors"
	"fmt"
	"io"

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

func runInstall(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet(
		"install",
		"usage: pfm install [--apply | --uninstall | --dry-run] [--config-dir DIR]",
		stderr,
	)
	apply := flags.Bool("apply", false, "apply the installation")
	uninstall := flags.Bool("uninstall", false, "remove installed links and restore backups")
	dryRun := flags.Bool("dry-run", false, "preview the installation without changing files")
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
	_, err := installer.Run(context.Background(), installer.Options{
		Mode:               mode,
		ConfigDir:          *configDir,
		Stdout:             stdout,
		ProvisionHarvest:   true,
		HarvestProvisioner: installHarvestProvisioner(),
	})
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
