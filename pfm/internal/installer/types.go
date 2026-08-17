// Package installer owns the host-level pfm command, hook, launcher, and unit
// wiring. Its assets are embedded so one pfm binary is a complete installer.
package installer

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"
)

var ErrReachableUserBus = errors.New("live user systemd bus is reachable")

// ErrLaunchAgentRunning is the macOS half of the same refusal.
//
// It is deliberately NOT a transliteration of the dead-bus gate. That gate
// demands a manager that is not live; launchd is ALWAYS live for a logged-in
// user and there is no jail where it is not, so a literal port would refuse
// every install forever. What survives of the intent is the narrow window the
// gate actually protects: an apply must not rewrite the agent, and the binary
// it points at, while that agent is MID-EXECUTION — that is the half-configured
// host the Linux gate exists to prevent.
var ErrLaunchAgentRunning = errors.New("the pfm name-sync launch agent is running")

type Mode uint8

const (
	ModeDryRun Mode = iota
	ModeApply
	ModeUninstall
)

type CommandRunner interface {
	Run(context.Context, string, ...string) error
}

// OutputRunner is the optional half of CommandRunner for probes that need to
// READ a manager's answer rather than just its exit status. launchctl reports a
// job's state in its output and exits zero either way, so the launch-agent gate
// cannot be built on exit codes alone. A runner that does not implement it
// cannot be probed, and the installer says so rather than assuming safety.
type OutputRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
}

type Options struct {
	Mode      Mode
	Home      string
	ConfigDir string
	Now       func() time.Time
	Stdout    io.Writer
	Runner    CommandRunner

	// launchGateUnprobed records that the launch-agent gate could not ask its
	// question. It is set by Run, never by a caller.
	launchGateUnprobed bool
}

type Report struct {
	Changed int
	OK      int
	Skipped int
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return command.Run()
}

func (execCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func normalize(options Options) (Options, error) {
	if options.Home == "" {
		var err error
		options.Home, err = os.UserHomeDir()
		if err != nil {
			return options, err
		}
	}
	if options.ConfigDir == "" {
		options.ConfigDir = options.Home + "/.claude"
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	if options.Runner == nil {
		options.Runner = execCommandRunner{}
	}
	return options, nil
}
