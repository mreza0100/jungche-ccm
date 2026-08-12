package hide

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// CommandTmux invokes tmux only through an explicit socket pathname.
type CommandTmux struct {
	Binary string
}

func (tmux CommandTmux) PanePID(
	ctx context.Context,
	socketPath, paneID string,
) (int, error) {
	output, err := tmux.command(
		ctx,
		socketPath,
		"list-panes",
		"-t",
		paneID,
		"-F",
		"#{pane_pid}",
	).Output()
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(string(output))
	pid, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse pane pid %q: %w", value, err)
	}
	return pid, nil
}

func (tmux CommandTmux) PaneExists(
	ctx context.Context,
	socketPath, paneID string,
) bool {
	output, err := tmux.command(
		ctx,
		socketPath,
		"list-panes",
		"-a",
		"-F",
		"#{pane_id}",
	).Output()
	if err != nil {
		return false
	}
	for _, candidate := range strings.Split(string(output), "\n") {
		if candidate == paneID {
			return true
		}
	}
	return false
}

func (tmux CommandTmux) SendLine(
	ctx context.Context,
	socketPath, paneID, line string,
) error {
	if output, err := tmux.command(
		ctx,
		socketPath,
		"send-keys",
		"-t",
		paneID,
		"-l",
		"--",
		line,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("send literal line: %w: %s", err, output)
	}
	if output, err := tmux.command(
		ctx,
		socketPath,
		"send-keys",
		"-t",
		paneID,
		"Enter",
	).CombinedOutput(); err != nil {
		return fmt.Errorf("send Enter: %w: %s", err, output)
	}
	return nil
}

func (tmux CommandTmux) KillPane(
	ctx context.Context,
	socketPath, paneID string,
) error {
	return tmux.command(ctx, socketPath, "kill-pane", "-t", paneID).Run()
}

func (tmux CommandTmux) KillServer(
	ctx context.Context,
	socketPath string,
) error {
	return tmux.command(ctx, socketPath, "kill-server").Run()
}

func (tmux CommandTmux) command(
	ctx context.Context,
	socketPath string,
	arguments ...string,
) *exec.Cmd {
	binary := tmux.Binary
	if binary == "" {
		binary = "tmux"
	}
	commandArguments := make([]string, 0, len(arguments)+2)
	commandArguments = append(commandArguments, "-S", socketPath)
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(ctx, binary, commandArguments...)
	command.Env = append(os.Environ(), "TMUX=")
	return command
}
