package kill

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"hostops/pfm/internal/deps"
)

// CommandSpawner starts the binary's killed finisher under a new session.
type CommandSpawner struct {
	Executable string
	Setsid     string
	ConfigPath string
}

func (spawner CommandSpawner) Spawn(
	ctx context.Context,
	args ExitArgs,
) error {
	executable := spawner.Executable
	if executable == "" {
		var err error
		executable, err = os.Executable()
		if err != nil {
			return fmt.Errorf("resolve pfm executable: %w", err)
		}
	}
	setsid := spawner.Setsid
	if setsid == "" {
		setsid = deps.Executable("setsid")
	} else {
		setsid = deps.Executable(setsid)
	}
	arguments := []string{"-f", executable}
	if spawner.ConfigPath != "" {
		arguments = append(arguments, "--config", spawner.ConfigPath)
	}
	arguments = append(arguments,
		"internal",
		"kill-exit",
		"--engine",
		string(args.Engine),
		"--id",
		args.ID,
		"--path",
		args.DataPath,
		"--socket",
		args.SocketPath,
		"--socket-name",
		args.SocketName,
		"--pane",
		args.PaneID,
	)
	command := exec.CommandContext(ctx, setsid, arguments...)
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open null device for kill finisher: %w", err)
	}
	defer null.Close()
	command.Stdin = null
	command.Stdout = null
	command.Stderr = null
	if err := command.Run(); err != nil {
		return fmt.Errorf("start detached kill finisher: %w", err)
	}
	return nil
}
