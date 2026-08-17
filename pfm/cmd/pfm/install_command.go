package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"hostops/pfm/internal/installer"
)

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
		Mode:      mode,
		ConfigDir: *configDir,
		Stdout:    stdout,
	})
	if errors.Is(err, installer.ErrReachableUserBus) {
		fmt.Fprintln(stderr, "pfm install: live user systemd bus is reachable; run in a proven dead-bus jail")
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
