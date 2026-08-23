package deps

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const ProbeTimeout = 5 * time.Second

// DefaultSelfDoctorTimeout bounds the self-doctor summary call on its own —
// separately from the ordinary version-probe timeout. A codex/claude
// self-doctor can legitimately run past a few seconds while scanning a large
// transcript corpus, and that must read as a named timeout, never as a
// broken engine that blocks install preflight.
const DefaultSelfDoctorTimeout = 30 * time.Second

type State string

const (
	StateOK      State = "ok"
	StateMissing State = "missing"
	StateBroken  State = "broken"
	StateSkipped State = "skipped"
)

// Result is one dependency's three-state probe result.
type Result struct {
	Entry      Entry
	State      State
	Path       string
	Version    string
	SelfDoctor string
	Error      string
	Raw        string
	VerboseErr string
}

// ProbeOptions supplies the only variability required by tests and callers.
type ProbeOptions struct {
	GOOS         string
	SkipHarvest  bool
	Provisioning bool
	VerboseDir   string
	LookPath     func(string) (string, error)
	Timeout      time.Duration
	// SelfDoctorTimeout bounds only the self-doctor calls. Unset falls back
	// to Timeout (so existing callers that only ever set Timeout keep
	// driving the self-doctor bound), and Timeout unset too falls back to
	// DefaultSelfDoctorTimeout.
	SelfDoctorTimeout time.Duration
}

// selfDoctorTimeout resolves the effective self-doctor bound: an explicit
// SelfDoctorTimeout wins, then the general Timeout (existing test setups
// already drive the self-doctor probe through it), then the 30s default.
func selfDoctorTimeout(options ProbeOptions) time.Duration {
	if options.SelfDoctorTimeout > 0 {
		return options.SelfDoctorTimeout
	}
	if options.Timeout > 0 {
		return options.Timeout
	}
	return DefaultSelfDoctorTimeout
}

// Probe resolves and runs every applicable registry entry with a per-command
// timeout. It never maps execution or parse failures to absence.
func Probe(ctx context.Context, entries []Entry, options ProbeOptions) []Result {
	if options.GOOS == "" {
		options.GOOS = runtime.GOOS
	}
	if options.LookPath == nil {
		options.LookPath = exec.LookPath
	}
	results := make([]Result, 0, len(entries))
	for _, entry := range entries {
		result := Result{Entry: entry}
		switch {
		case !entry.AppliesTo(options.GOOS):
			result.State = StateSkipped
			result.Error = "not this platform"
		case entry.Harvest && options.SkipHarvest:
			result.State = StateSkipped
			result.Error = "--skip-harvest"
		case entry.Harvest && options.Provisioning:
			result.State = StateSkipped
			result.Error = "provisioned by install"
		default:
			result = probeOne(ctx, entry, options)
		}
		results = append(results, result)
	}
	return results
}

func probeOne(ctx context.Context, entry Entry, options ProbeOptions) Result {
	result := Result{Entry: entry}
	path, err := options.LookPath(entry.Command)
	if err != nil {
		result.State = StateMissing
		if info, statErr := os.Stat(entry.Command); (statErr == nil && !info.IsDir()) || (statErr != nil && !errors.Is(statErr, os.ErrNotExist)) {
			result.State = StateBroken
			result.Error = err.Error()
		} else if !errors.Is(err, exec.ErrNotFound) {
			result.State = StateBroken
			result.Error = err.Error()
		}
		return result
	}
	result.Path = path
	if len(entry.VersionArgs) != 0 {
		output, runErr := boundedOutput(ctx, options.Timeout, path, entry.VersionArgs...)
		result.Raw = string(output)
		if verboseErr := writeVerbose(options.VerboseDir, entry.Name+"-version", output); verboseErr != nil {
			result.VerboseErr = verboseErr.Error()
		}
		if runErr != nil {
			result.State = StateBroken
			result.Error = commandError(runErr, output)
			return result
		}
		version, parseErr := entry.Parse(string(output))
		if parseErr != nil {
			result.State = StateBroken
			result.Error = parseErr.Error()
			return result
		}
		result.Version = version
		if entry.MinVersion != "" && !atLeast(version, entry.MinVersion) {
			result.State = StateBroken
			result.Error = fmt.Sprintf("version %s is below minimum %s", version, entry.MinVersion)
			return result
		}
	}
	result.State = StateOK
	if len(entry.SelfDoctorArgs) != 0 {
		var selfDoctorRaw string
		result.SelfDoctor, selfDoctorRaw, err = probeSelfDoctor(ctx, path, entry, options.VerboseDir, selfDoctorTimeout(options))
		if err != nil {
			result.VerboseErr = err.Error()
		}
		switch {
		case result.SelfDoctor == "broken":
			result.State = StateBroken
			if selfDoctorRaw != "" {
				result.Error = fmt.Sprintf("self-doctor failed raw=%q", selfDoctorRaw)
			} else {
				result.Error = "self-doctor failed"
			}
		case strings.HasPrefix(result.SelfDoctor, "timeout"):
			// The binary already answered --version; a self-doctor summary
			// that merely outran its own bound is not a broken engine and
			// must never block install preflight.
			result.Error = fmt.Sprintf("self-doctor %s", result.SelfDoctor)
		}
	}
	return result
}

// probeSelfDoctor returns the self-doctor status label, the raw first output
// line for a genuine ("broken") failure so the caller can quote it, and any
// verbose-write error. The --help probe's own timeout still reads as
// "broken" — a self-doctor that cannot even answer --help within the bound
// is unsupported or hung, not a legitimate slow summary. Only the summary
// call itself (entry.SelfDoctorArgs) distinguishes a timeout from a real
// failure, because that is the call the regression this guards against
// actually outruns.
func probeSelfDoctor(ctx context.Context, path string, entry Entry, verboseDir string, timeout time.Duration) (string, string, error) {
	helpArgs := []string{entry.SelfDoctorArgs[0], "--help"}
	help, helpErr := boundedOutputWithEnvironment(ctx, timeout, terminalEnvironment(), path, helpArgs...)
	if err := writeVerbose(verboseDir, entry.Name+"-self-doctor-help", help); err != nil {
		return "", "", err
	}
	if helpErr != nil {
		if errors.Is(helpErr, context.DeadlineExceeded) {
			return "broken", "", nil
		}
		var exitErr *exec.ExitError
		if errors.As(helpErr, &exitErr) {
			return "unavailable", "", nil
		}
		return "broken", "", nil
	}
	output, err := boundedOutputWithEnvironment(ctx, timeout, terminalEnvironment(), path, entry.SelfDoctorArgs...)
	if writeErr := writeVerbose(verboseDir, entry.Name+"-self-doctor", output); writeErr != nil {
		return "", "", writeErr
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Sprintf("timeout (%s)", timeout), "", nil
		}
		if strings.Contains(strings.ToLower(string(output)), "tty") || strings.Contains(strings.ToLower(string(output)), "interactive") {
			return "unavailable (interactive-only)", "", nil
		}
		return "broken", FirstLine(string(output)), nil
	}
	return "ok", "", nil
}

func boundedOutput(parent context.Context, timeout time.Duration, path string, args ...string) ([]byte, error) {
	return boundedOutputWithEnvironment(parent, timeout, nil, path, args...)
}

func boundedOutputWithEnvironment(parent context.Context, timeout time.Duration, environment []string, path string, args ...string) ([]byte, error) {
	if timeout <= 0 {
		timeout = ProbeTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	command := exec.CommandContext(ctx, path, args...)
	command.Stdin = nil
	if environment != nil {
		command.Env = environment
	}
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return output, ctx.Err()
	}
	return output, err
}

// terminalEnvironment gives interactive engine doctors a real terminal
// capability description even though PFM itself runs them without a TTY.
// Inheriting TERM=dumb makes Codex report a host defect that is not present.
func terminalEnvironment() []string {
	environment := os.Environ()
	for index, value := range environment {
		if strings.HasPrefix(value, "TERM=") {
			environment[index] = "TERM=xterm-256color"
			return environment
		}
	}
	return append(environment, "TERM=xterm-256color")
}

func commandError(err error, output []byte) string {
	line := FirstLine(string(output))
	if line == "" {
		return err.Error()
	}
	return fmt.Sprintf("%v raw=%q", err, line)
}

func writeVerbose(directory, name string, output []byte) error {
	if directory == "" || len(output) == 0 {
		return nil
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create verbose probe directory: %w", err)
	}
	safe := regexp.MustCompile(`[^a-zA-Z0-9._-]+`).ReplaceAllString(name, "-")
	if err := os.WriteFile(filepath.Join(directory, safe+".log"), output, 0o600); err != nil {
		return fmt.Errorf("write verbose probe output: %w", err)
	}
	return nil
}

func atLeast(version, minimum string) bool {
	left := numericVersion(version)
	right := numericVersion(minimum)
	for index := 0; index < len(left) || index < len(right); index++ {
		var l, r int
		if index < len(left) {
			l = left[index]
		}
		if index < len(right) {
			r = right[index]
		}
		if l != r {
			return l > r
		}
	}
	return true
}

func numericVersion(value string) []int {
	fields := regexp.MustCompile(`[0-9]+`).FindAllString(value, -1)
	result := make([]int, 0, len(fields))
	for _, field := range fields {
		number, _ := strconv.Atoi(field)
		result = append(result, number)
	}
	return result
}
