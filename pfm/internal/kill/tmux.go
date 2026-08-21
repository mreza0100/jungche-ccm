package kill

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"hostops/pfm/internal/deps"
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

// ClientTTYs lists the terminals attached to this server. A chat's clients ARE
// its viewports — the panes a person is watching it through.
func (tmux CommandTmux) ClientTTYs(
	ctx context.Context,
	socketPath string,
) ([]string, error) {
	output, err := tmux.command(
		ctx, socketPath, "list-clients", "-F", "#{client_tty}",
	).Output()
	if err != nil {
		return nil, err
	}
	var ttys []string
	for _, line := range strings.Split(string(output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			ttys = append(ttys, line)
		}
	}
	return ttys, nil
}

// PanesByTTY maps each pane on this server to the terminal it runs in — the
// join that traces a viewport client back to the pane hosting it.
//
// Space-delimited, never a tab: tmux hands a control character in a format
// string back as "_" unless the caller's environment carries a UTF-8 locale or
// merely defines $TMUX, and a row that arrives "_"-joined is silently
// unsplittable. A tty is /dev/pts/N and a pane is %N, so a space cannot be
// ambiguous.
func (tmux CommandTmux) PanesByTTY(
	ctx context.Context,
	socketPath string,
) (map[string]string, error) {
	output, err := tmux.command(
		ctx, socketPath, "list-panes", "-a", "-F", "#{pane_tty} #{pane_id}",
	).Output()
	if err != nil {
		return nil, err
	}
	panes := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		tty, paneID, found := strings.Cut(strings.TrimSpace(line), " ")
		if found && tty != "" && paneID != "" {
			panes[tty] = paneID
		}
	}
	return panes, nil
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
		binary = deps.Executable("tmux")
	}
	commandArguments := make([]string, 0, len(arguments)+2)
	commandArguments = append(commandArguments, "-S", socketPath)
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(ctx, binary, commandArguments...)
	command.Env = append(os.Environ(), "TMUX=")
	return command
}
