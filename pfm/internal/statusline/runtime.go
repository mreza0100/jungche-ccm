package statusline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"hostops/pfm/internal/paths"
)

// RefreshKind names one detached cache refresher the render path may arm.
type RefreshKind string

const (
	RefreshKindVertex RefreshKind = "vertex"
	RefreshKindGPT    RefreshKind = "gpt"
)

// CommandRunner is the small read-only command seam used by the git segment.
type CommandRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
}

type commandRunner struct{}

func (commandRunner) Output(
	ctx context.Context,
	name string,
	args ...string,
) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	return command.Output()
}

// Runtime holds every environmental input to a render. Tests replace all of
// them, so no fixture reads a live socket, account, cache, process tree, or
// repository.
type Runtime struct {
	Now func() time.Time

	Home          string
	ConfigDir     string
	CacheDir      string
	RateLimitDir  string
	SIDDir        string
	TmuxDir       string
	ProcRoot      string
	Columns       int
	UID           int
	AccountDirs   map[string]int
	AccountEmojis map[int]string

	// A non-nil Env is a closed test environment. Nil reads os.Getenv.
	Env map[string]string

	Command CommandRunner
	Spawn   func(RefreshKind) error
}

func DefaultRuntime() (Runtime, error) {
	resolved, err := paths.Resolve()
	if err != nil {
		return Runtime{}, err
	}
	columns, _ := strconv.Atoi(os.Getenv("COLUMNS"))
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		configDir = filepath.Join(resolved.Home, ".claude")
	}
	cacheDir := os.TempDir()
	if jailHome := os.Getenv(paths.EnvHome); jailHome != "" {
		cacheDir = filepath.Join(jailHome, "tmp")
	}
	rateDir := filepath.Join(cacheDir, "cc-rate-limits")
	return Runtime{
		Now:          time.Now,
		Home:         resolved.Home,
		ConfigDir:    configDir,
		CacheDir:     cacheDir,
		RateLimitDir: rateDir,
		SIDDir:       resolved.SIDDir,
		TmuxDir:      resolved.TmuxDir,
		ProcRoot:     resolved.ProcRoot,
		Columns:      columns,
		UID:          os.Getuid(),
		Command:      commandRunner{},
	}, nil
}

func (runtime Runtime) getenv(name string) string {
	if runtime.Env != nil {
		return runtime.Env[name]
	}
	return os.Getenv(name)
}

func (runtime Runtime) now() time.Time {
	if runtime.Now != nil {
		return runtime.Now()
	}
	return time.Now()
}

func (runtime Runtime) normalized() Runtime {
	if runtime.Home == "" {
		runtime.Home, _ = os.UserHomeDir()
	}
	if runtime.ConfigDir == "" {
		runtime.ConfigDir = filepath.Join(runtime.Home, ".claude")
	}
	if runtime.CacheDir == "" {
		runtime.CacheDir = os.TempDir()
	}
	if runtime.RateLimitDir == "" {
		runtime.RateLimitDir = filepath.Join(runtime.CacheDir, "cc-rate-limits")
	}
	if runtime.SIDDir == "" {
		runtime.SIDDir = filepath.Join(runtime.CacheDir, "cc-sid")
	}
	if runtime.TmuxDir == "" {
		runtime.TmuxDir = filepath.Join(runtime.CacheDir, "tmux-"+strconv.Itoa(runtime.UID))
	}
	if runtime.ProcRoot == "" {
		runtime.ProcRoot = "/proc"
	}
	if runtime.Command == nil {
		runtime.Command = commandRunner{}
	}
	return runtime
}
