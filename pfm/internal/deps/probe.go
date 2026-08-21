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
		result.SelfDoctor, err = probeSelfDoctor(ctx, path, entry, options.VerboseDir, options.Timeout)
		if err != nil {
			result.VerboseErr = err.Error()
		}
		if result.SelfDoctor == "broken" {
			result.State = StateBroken
			result.Error = "self-doctor failed"
		}
	}
	return result
}

func probeSelfDoctor(ctx context.Context, path string, entry Entry, verboseDir string, timeout time.Duration) (string, error) {
	helpArgs := []string{entry.SelfDoctorArgs[0], "--help"}
	help, helpErr := boundedOutput(ctx, timeout, path, helpArgs...)
	if err := writeVerbose(verboseDir, entry.Name+"-self-doctor-help", help); err != nil {
		return "", err
	}
	if helpErr != nil {
		if errors.Is(helpErr, context.DeadlineExceeded) {
			return "broken", nil
		}
		var exitErr *exec.ExitError
		if errors.As(helpErr, &exitErr) {
			return "unavailable", nil
		}
		return "broken", nil
	}
	output, err := boundedOutput(ctx, timeout, path, entry.SelfDoctorArgs...)
	if writeErr := writeVerbose(verboseDir, entry.Name+"-self-doctor", output); writeErr != nil {
		return "", writeErr
	}
	if err != nil {
		if strings.Contains(strings.ToLower(string(output)), "tty") || strings.Contains(strings.ToLower(string(output)), "interactive") {
			return "unavailable (interactive-only)", nil
		}
		return "broken", nil
	}
	return "ok", nil
}

func boundedOutput(parent context.Context, timeout time.Duration, path string, args ...string) ([]byte, error) {
	if timeout <= 0 {
		timeout = ProbeTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	command := exec.CommandContext(ctx, path, args...)
	command.Stdin = nil
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		return output, ctx.Err()
	}
	return output, err
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
