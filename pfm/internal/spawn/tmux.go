package spawn

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	pfmconfig "hostops/pfm/internal/config"
	"hostops/pfm/internal/deps"
	"hostops/pfm/internal/paths"
)

// CommandTmux invokes tmux only through the configured socket directory, the
// same jailed shape action.CommandTmux uses.
type CommandTmux struct {
	Binary  string
	TmuxDir string
	// Titles is the resolved tmux.titles policy. NIL is the default (pfm owns
	// the terminal title), never "off": a client built without a machine
	// config keeps today's behaviour instead of silently handing the title to
	// the host.
	Titles *pfmconfig.TmuxTitles
}

// preflightBinary proves the executable word a pane is about to run resolves
// from THIS process's environment — the same environment the pane's shell
// inherits. A pane whose command cannot resolve dies at launch and takes the
// fresh server with it, so the visible failure becomes "no server running":
// tmux named, the engine never mentioned. That silence is exactly how an MCP
// daemon under systemd's default PATH failed to spawn `claude` on a live box.
func preflightBinary(binary string) error {
	if binary == "" {
		return nil
	}
	if strings.Contains(binary, "/") {
		info, err := os.Stat(binary)
		if err != nil {
			return fmt.Errorf(
				"engine binary %s is not on disk: %w — fix the configured <engine>.binary path",
				binary, err,
			)
		}
		if info.IsDir() || info.Mode()&0o111 == 0 {
			return fmt.Errorf("engine binary %s is not an executable file", binary)
		}
		return nil
	}
	if _, err := deps.Resolve(binary); err != nil {
		return fmt.Errorf(
			"engine binary %q is not reachable from this process's PATH: %w — a systemd user service starts with systemd's default PATH; pin an absolute <engine>.binary in the machine config or extend the unit's Environment=PATH",
			binary, err,
		)
	}
	return nil
}

// NewSession creates the detached session and gives it the options a fleet
// chat is expected to carry.
//
// The title options are applied only when tmux.titles is enabled, so a headless
// pane's terminal title reads like an attached one WITHOUT seizing the title
// from a host that set its own before tmux started; automatic-rename is always
// off, because the window name is the fleet's DNS record and pfm is its only
// writer.
func (tmux CommandTmux) NewSession(
	ctx context.Context,
	spec SessionSpec,
) error {
	if err := preflightBinary(spec.Binary); err != nil {
		return err
	}
	if err := paths.EnsureTmuxDir(tmux.TmuxDir); err != nil {
		return err
	}
	arguments := append(paths.TmuxConfigArguments(),
		"new-session", "-d",
		"-s", spec.Session,
		"-n", spec.Window,
		"-c", spec.CWD,
		"-x", strconv.Itoa(spec.Width),
		"-y", strconv.Itoa(spec.Height),
		spec.Run,
	)
	command, err := tmux.newSessionCommand(
		ctx,
		spec.Socket,
		arguments...,
	)
	if err != nil {
		return fmt.Errorf("create chat server: %w", err)
	}
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("create chat server: %w: %s", err, output)
	}
	serverOptions := append(
		pfmconfig.TmuxTitlesOrDefault(tmux.Titles).Options(),
		[]string{"set-window-option", "-g", "automatic-rename", "off"},
	)
	for _, options := range serverOptions {
		if output, err := tmux.command(
			ctx,
			spec.Socket,
			options...,
		).CombinedOutput(); err != nil {
			// A server that vanished between creation and configuration died
			// with its only pane — name the pane's command, because that is
			// where the death almost always started.
			return fmt.Errorf(
				"configure chat server: %w: %s — the server died before it could be configured; its pane command likely exited at launch (%s)",
				err, output, spec.Run,
			)
		}
	}
	return nil
}

func (tmux CommandTmux) newSessionCommand(
	ctx context.Context,
	socket string,
	arguments ...string,
) (*exec.Cmd, error) {
	binary := tmux.Binary
	if binary == "" {
		binary = deps.Executable("tmux")
	}
	commandArguments := []string{"-S", filepath.Join(tmux.TmuxDir, socket)}
	commandArguments = append(commandArguments, arguments...)
	environment := append(os.Environ(), "TMUX=")
	return serviceScopeCommand(ctx, binary, commandArguments, environment)
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
		binary = deps.Executable("tmux")
	}
	commandArguments := []string{"-S", filepath.Join(tmux.TmuxDir, socket)}
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(ctx, binary, commandArguments...)
	// TMUX= keeps a spawn made from inside a chat from nesting the new server
	// into the caller's own.
	command.Env = append(os.Environ(), "TMUX=")
	return command
}
