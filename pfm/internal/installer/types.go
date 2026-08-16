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

type Mode uint8

const (
	ModeDryRun Mode = iota
	ModeApply
	ModeUninstall
)

type CommandRunner interface {
	Run(context.Context, string, ...string) error
}

type Options struct {
	Mode      Mode
	Home      string
	ConfigDir string
	Now       func() time.Time
	Stdout    io.Writer
	Runner    CommandRunner
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
