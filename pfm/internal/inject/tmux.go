package inject

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"

	"hostops/pfm/internal/deps"
)

var pasteSequence atomic.Uint64

// CommandTmux invokes tmux against an explicit socket pathname.
type CommandTmux struct {
	Binary string
}

func (tmux CommandTmux) Capture(
	ctx context.Context,
	socketPath, target string,
	styled bool,
	scrollback int,
) (string, error) {
	arguments := []string{"capture-pane", "-t", target, "-p", "-J"}
	if styled {
		arguments = append(arguments, "-e")
	}
	switch {
	case scrollback == FullScrollback:
		// chat.sh:1219 — `-S -` starts at the top of the retained buffer, so
		// the whole scrollback is captured, not just the visible fold.
		arguments = append(arguments, "-S", "-")
	case scrollback > 0:
		arguments = append(arguments, "-S", fmt.Sprintf("-%d", scrollback))
	}
	output, err := tmux.command(ctx, socketPath, arguments...).Output()
	return string(output), err
}

func (tmux CommandTmux) SendLiteral(
	ctx context.Context,
	socketPath, target, text string,
) error {
	return tmux.command(
		ctx,
		socketPath,
		"send-keys",
		"-t",
		target,
		"-l",
		"--",
		text,
	).Run()
}

// SendPaste carries a file-backed message through tmux's paste buffer. The
// -p flag asks tmux to use bracketed-paste mode when the target application
// advertises it; Codex then keeps the full body behind one composer block
// instead of rendering a several-screen inline draft whose submit state
// cannot be proven. A private, one-shot buffer prevents concurrent injects
// into different panes from sharing bytes.
func (tmux CommandTmux) SendPaste(
	ctx context.Context,
	socketPath, target, text string,
) error {
	buffer := fmt.Sprintf("pfm-inject-%d-%d", os.Getpid(), pasteSequence.Add(1))
	load := tmux.command(ctx, socketPath, "load-buffer", "-b", buffer, "-")
	load.Stdin = strings.NewReader(text)
	if err := load.Run(); err != nil {
		return err
	}
	paste := tmux.command(
		ctx,
		socketPath,
		"paste-buffer",
		"-d",
		"-p",
		"-b",
		buffer,
		"-t",
		target,
	)
	if err := paste.Run(); err != nil {
		_ = tmux.command(ctx, socketPath, "delete-buffer", "-b", buffer).Run()
		return err
	}
	return nil
}

func (tmux CommandTmux) SendKey(
	ctx context.Context,
	socketPath, target, key string,
) error {
	return tmux.command(
		ctx,
		socketPath,
		"send-keys",
		"-t",
		target,
		key,
	).Run()
}

func (tmux CommandTmux) CancelCopyMode(
	ctx context.Context,
	socketPath, target string,
) error {
	return tmux.command(
		ctx,
		socketPath,
		"send-keys",
		"-t",
		target,
		"-X",
		"cancel",
	).Run()
}

func (tmux CommandTmux) PaneInMode(
	ctx context.Context,
	socketPath, target string,
) (bool, error) {
	output, err := tmux.command(
		ctx,
		socketPath,
		"display-message",
		"-t",
		target,
		"-p",
		"#{pane_in_mode}",
	).Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(output)) == "1", nil
}

func (tmux CommandTmux) PaneCommand(
	ctx context.Context,
	socketPath, target string,
) (string, error) {
	output, err := tmux.command(
		ctx,
		socketPath,
		"display-message",
		"-t",
		target,
		"-p",
		"#{pane_current_command}",
	).Output()
	return strings.TrimSpace(string(output)), err
}

func (tmux CommandTmux) CurrentSession(
	ctx context.Context,
	socketPath string,
) (string, error) {
	output, err := tmux.command(
		ctx,
		socketPath,
		"display-message",
		"-p",
		"#{session_name}",
	).Output()
	return strings.TrimSpace(string(output)), err
}

// WindowName reads the tmux window name backing a target, the human thread
// name pfm sets on codex panes.
func (tmux CommandTmux) WindowName(
	ctx context.Context,
	socketPath, target string,
) (string, error) {
	output, err := tmux.command(
		ctx,
		socketPath,
		"display-message",
		"-t",
		target,
		"-p",
		"#{window_name}",
	).Output()
	return strings.TrimSpace(string(output)), err
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
