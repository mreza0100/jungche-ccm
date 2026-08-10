package hide

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// CommandSpawner starts the binary's hidden finisher under a new session.
type CommandSpawner struct {
	Executable string
	Setsid     string
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
			return fmt.Errorf("resolve cc-fleet executable: %w", err)
		}
	}
	setsid := spawner.Setsid
	if setsid == "" {
		setsid = "setsid"
	}
	command := exec.CommandContext(
		ctx,
		setsid,
		"-f",
		executable,
		"internal",
		"hide-exit",
		"--engine",
		args.Engine,
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
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open null device for hide finisher: %w", err)
	}
	defer null.Close()
	command.Stdin = null
	command.Stdout = null
	command.Stderr = null
	if err := command.Run(); err != nil {
		return fmt.Errorf("start detached hide finisher: %w", err)
	}
	return nil
}
