package main

import (
	"io"

	"hostops/pfm/internal/installer"
)

func runUninstall(args []string, stdout, stderr io.Writer, runtimes ...commandRuntime) int {
	flags := newFlagSet(
		"uninstall",
		"usage: pfm uninstall [--config-dir DIR]",
		stderr,
	)
	configDir := flags.String("config-dir", "", "target config directory instead of ~/.claude")
	if code, ok := parseFlags(flags, args); !ok {
		return code
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	return runInstallerCommand(
		"uninstall",
		newInstallerOptions(installer.ModeUninstall, *configDir, false, false, stdout, runtimes...),
		stderr,
	)
}
