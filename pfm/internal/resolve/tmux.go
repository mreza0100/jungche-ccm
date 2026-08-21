package resolve

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"hostops/pfm/internal/deps"
	"hostops/pfm/internal/tmuxfmt"
)

// CommandTmux invokes tmux with an explicit socket pathname.
type CommandTmux struct {
	Binary string
}

func (tmux CommandTmux) ListPanes(
	ctx context.Context,
	socketPath string,
) ([]Pane, error) {
	format := strings.Join([]string{
		"#{session_name}",
		"#{pane_id}",
		"#{pane_current_command}",
		"#{window_name}",
	}, "\x1f")
	output, err := tmux.command(
		ctx,
		socketPath,
		"list-panes",
		"-a",
		"-F",
		format,
	).Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	panes := make([]Pane, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Either spelling of the control separator: see internal/tmuxfmt.
		fields := tmuxfmt.SplitN(line, 4)
		if len(fields) != 4 {
			return nil, fmt.Errorf(
				"tmux socket %q returned %d fields in %q",
				socketPath,
				len(fields),
				line,
			)
		}
		panes = append(panes, Pane{
			SocketPath:     socketPath,
			SessionName:    fields[0],
			PaneID:         fields[1],
			CurrentCommand: fields[2],
			WindowName:     fields[3],
		})
	}
	return panes, nil
}

func (tmux CommandTmux) CapturePane(
	ctx context.Context,
	socketPath, paneID string,
) (string, error) {
	output, err := tmux.command(
		ctx,
		socketPath,
		"capture-pane",
		"-t",
		paneID,
		"-p",
		"-J",
	).Output()
	return string(output), err
}

func (tmux CommandTmux) command(
	ctx context.Context,
	socketPath string,
	arguments ...string,
) *exec.Cmd {
	binary := tmux.Binary
	if binary == "" {
		binary = deps.Executable("tmux")
	}
	commandArguments := make([]string, 0, len(arguments)+2)
	commandArguments = append(commandArguments, "-S", socketPath)
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(ctx, binary, commandArguments...)
	command.Env = append(os.Environ(), "TMUX=")
	return command
}
