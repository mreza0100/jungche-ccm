package action

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"hostops/cc-fleet/internal/paths"
)

// CommandTmux invokes tmux only through the configured jailed socket directory.
type CommandTmux struct {
	Binary  string
	TmuxDir string
}

func (tmux CommandTmux) ListPanes(
	ctx context.Context,
	socket string,
) ([]Pane, error) {
	format := strings.Join([]string{
		"#{pane_id}",
		"#{pane_tty}",
		"#{session_name}",
		"#{window_name}",
		"#{window_index}",
		"#{pane_current_command}",
	}, "\x1f")
	output, err := tmux.command(
		ctx,
		socket,
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
		fields := strings.SplitN(line, `\037`, 6)
		if len(fields) != 6 {
			return nil, fmt.Errorf(
				"tmux socket %q returned %d action fields",
				socket,
				len(fields),
			)
		}
		windowIndex, err := strconv.Atoi(fields[4])
		if err != nil {
			return nil, fmt.Errorf("parse tmux window index %q: %w", fields[2], err)
		}
		panes = append(panes, Pane{
			PaneID:         fields[0],
			TTY:            strings.TrimPrefix(fields[1], "/dev/"),
			SessionName:    fields[2],
			WindowName:     fields[3],
			WindowIndex:    windowIndex,
			CurrentCommand: fields[5],
		})
	}
	return panes, nil
}

func (tmux CommandTmux) SocketAlive(
	ctx context.Context,
	socket string,
) bool {
	return tmux.command(ctx, socket, "list-panes", "-a").Run() == nil
}

func (tmux CommandTmux) KillPane(
	ctx context.Context,
	socket, paneID string,
) error {
	return tmux.command(ctx, socket, "kill-pane", "-t", paneID).Run()
}

func (tmux CommandTmux) KillServer(
	ctx context.Context,
	socket string,
) error {
	return tmux.command(ctx, socket, "kill-server").Run()
}

func (tmux CommandTmux) SetWindowSizeLatest(
	ctx context.Context,
	socket string,
) error {
	return tmux.command(
		ctx,
		socket,
		"set-option",
		"-g",
		"window-size",
		"latest",
	).Run()
}

func (tmux CommandTmux) SelectWindow(
	ctx context.Context,
	socket string,
	windowIndex int,
) error {
	return tmux.command(
		ctx,
		socket,
		"select-window",
		"-t",
		":"+strconv.Itoa(windowIndex),
	).Run()
}

func (tmux CommandTmux) CreateCodexServer(
	ctx context.Context,
	server CodexServer,
) error {
	arguments := append(paths.TmuxConfigArguments(),
		"new-session",
		"-d",
		"-s",
		server.Socket,
		"-c",
		server.CWD,
		"-n",
		"Codex",
		server.Run,
	)
	if output, err := tmux.command(
		ctx,
		server.Socket,
		arguments...,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("create Codex server: %w: %s", err, output)
	}
	for _, arguments := range [][]string{
		{"set-option", "-g", "set-titles", "on"},
		{"set-option", "-g", "set-titles-string", "⬢ #{window_name} · #{pane_title}"},
		{"set-window-option", "-g", "automatic-rename", "off"},
	} {
		if output, err := tmux.command(
			ctx,
			server.Socket,
			arguments...,
		).CombinedOutput(); err != nil {
			return fmt.Errorf("configure Codex server: %w: %s", err, output)
		}
	}
	return nil
}

func (tmux CommandTmux) command(
	ctx context.Context,
	socket string,
	arguments ...string,
) *exec.Cmd {
	binary := tmux.Binary
	if binary == "" {
		binary = "tmux"
	}
	commandArguments := []string{
		"-S",
		filepath.Join(tmux.TmuxDir, socket),
	}
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(ctx, binary, commandArguments...)
	command.Env = append(os.Environ(), "TMUX=")
	return command
}

type ExecRunner struct {
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}

func (runner ExecRunner) Run(
	ctx context.Context,
	name string,
	args ...string,
) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = runner.Stdin
	command.Stdout = runner.Stdout
	command.Stderr = runner.Stderr
	return command.Run()
}
