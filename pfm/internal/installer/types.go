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

	"hostops/pfm/internal/deps"
	"hostops/pfm/internal/harvestpy"
)

// ErrNameSyncRunning refuses a mutating install while the Linux name-sync
// service is executing. A reachable but idle user manager is safe, and a dry
// run never needs this gate because it writes nothing.
var ErrNameSyncRunning = errors.New("the pfm name-sync service is running")

// ErrLaunchAgentRunning is the macOS half of the same narrow refusal: a
// mutating install must not rewrite the agent and its binary mid-execution.
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

// HarvestProvisioner is the narrow seam between host wiring and the pinned
// Python runtime. The real implementation delegates to harvestpy; installer
// tests inject a fake so they never download the conversion lock.
type HarvestProvisioner interface {
	Plan(harvestpy.Platform) (harvestpy.InstallPlan, error)
	Provision(context.Context, harvestpy.ProvisionOptions) (harvestpy.ProvisionResult, error)
	Check(context.Context, string, harvestpy.Platform) (harvestpy.CheckReport, error)
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
	// ConfigDirs is the config-driven settings fanout. A nil value retains
	// the historical discovery of existing .cc account settings for callers
	// that construct Options directly.
	ConfigDirs []string
	// CodexHomes is the config-driven hooks.json fanout. A nil value retains
	// the historical single ~/.codex target for direct legacy callers; an
	// explicitly empty roster installs no Codex hook.
	CodexHomes []string
	// SourceRepo is the clone whose blueprint and binary are being installed.
	// Empty preserves an existing marker when install is invoked elsewhere.
	SourceRepo string
	Force      bool
	Now        func() time.Time
	Stdout     io.Writer
	Runner     CommandRunner

	MCPEnabled     map[string]bool
	MCPPort        int
	MCPAuthToken   string
	MCPConfigPath  string
	ClaudeBinary   string
	ClaudePrompted map[int]bool
	CodexYolo      map[int]bool

	// ProvisionHarvest makes install/uninstall own the pinned conversion
	// environment. The command sets this for real user actions; existing
	// installer unit tests leave it false and inject no network-capable worker.
	ProvisionHarvest   bool
	HarvestProvisioner HarvestProvisioner
	HarvestPlatform    harvestpy.Platform
	HarvestOffline     bool

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
	command := exec.CommandContext(ctx, deps.Executable(name), args...)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return command.Run()
}

func (execCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, deps.Executable(name), args...).Output()
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
	if options.MCPPort == 0 {
		options.MCPPort = 8377
	}
	if options.ClaudePrompted == nil {
		options.ClaudePrompted = map[int]bool{1: false, 2: false, 3: false}
	}
	if options.CodexYolo == nil {
		options.CodexYolo = map[int]bool{1: true, 2: true, 3: true}
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
	if options.ProvisionHarvest && options.HarvestProvisioner == nil {
		options.HarvestProvisioner = NewHarvestProvisioner()
	}
	return options, nil
}
