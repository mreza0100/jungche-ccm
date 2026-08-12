package spawn

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// CommandTmux invokes tmux only through the configured socket directory, the
// same jailed shape action.CommandTmux uses.
type CommandTmux struct {
	Binary  string
	TmuxDir string
}

// NewSession creates the detached session and gives it the title options a
// fleet chat is expected to carry, so a headless pane's terminal title reads
// like an attached one.
func (tmux CommandTmux) NewSession(
	ctx context.Context,
	spec SessionSpec,
) error {
	arguments := []string{
		"-f", "/dev/null",
		"new-session", "-d",
		"-s", spec.Session,
		"-n", spec.Window,
		"-c", spec.CWD,
		"-x", strconv.Itoa(spec.Width),
		"-y", strconv.Itoa(spec.Height),
		spec.Run,
	}
	if output, err := tmux.command(
		ctx,
		spec.Socket,
		arguments...,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("create chat server: %w: %s", err, output)
	}
	for _, options := range [][]string{
		{"set-option", "-g", "set-titles", "on"},
		{"set-option", "-g", "set-titles-string", "⬢ #{window_name} · #{pane_title}"},
		{"set-window-option", "-g", "automatic-rename", "off"},
	} {
		if output, err := tmux.command(
			ctx,
			spec.Socket,
			options...,
		).CombinedOutput(); err != nil {
			return fmt.Errorf("configure chat server: %w: %s", err, output)
		}
	}
	return nil
}

func (tmux CommandTmux) Capture(
	ctx context.Context,
	socket, target string,
) (string, error) {
	output, err := tmux.command(
		ctx,
		socket,
		"capture-pane", "-t", target, "-p", "-J",
	).Output()
	return string(output), err
}

func (tmux CommandTmux) SendLiteral(
	ctx context.Context,
	socket, target, text string,
) error {
	return tmux.command(
		ctx,
		socket,
		"send-keys", "-t", target, "-l", "--", text,
	).Run()
}

func (tmux CommandTmux) SendKey(
	ctx context.Context,
	socket, target, key string,
) error {
	return tmux.command(ctx, socket, "send-keys", "-t", target, key).Run()
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
	commandArguments := []string{"-S", filepath.Join(tmux.TmuxDir, socket)}
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(ctx, binary, commandArguments...)
	// TMUX= keeps a spawn made from inside a chat from nesting the new server
	// into the caller's own.
	command.Env = append(os.Environ(), "TMUX=")
	return command
}
