// Package deps owns the external-command contract for pfm.
package deps

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Entry describes one external command pfm may execute. Command is the
// configured name or absolute path; Name is the stable doctor-row key.
type Entry struct {
	Name           string
	Command        string
	Purpose        string
	Required       bool
	Platforms      []string
	VersionArgs    []string
	MinVersion     string
	Parse          func(string) (string, error)
	InstallHint    string
	Harvest        bool
	SelfDoctorArgs []string
}

// Options materializes the config- and platform-owned registry entries.
type Options struct {
	Home         string
	ClaudeBinary string
	CodexBinary  string
	GOOS         string
	GOARCH       string
}

var fixedCommands = []Entry{
	// tmux 1.8 introduced wait-for, the newest primitive used by the managed
	// Claude launcher. Reference: upstream CHANGES, "CHANGES FROM 1.7 TO 1.8":
	// https://github.com/tmux/tmux/blob/master/CHANGES
	{Name: "tmux", Purpose: "fleet panes and chat transport", Required: true, VersionArgs: []string{"-V"}, MinVersion: "1.8", Parse: prefixedVersion("tmux"), InstallHint: "install tmux 1.8 or newer"},
	{Name: "git", Purpose: "repository inspection and updates", Required: true, VersionArgs: []string{"--version"}, Parse: prefixedVersion("git version"), InstallHint: "install git"},
	{Name: "sh", Purpose: "portable shell command execution", Required: true, InstallHint: "install a POSIX shell"},
	{Name: "bash", Purpose: "installed compatibility scripts", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion, InstallHint: "install bash"},
	{Name: "zsh", Purpose: "interactive generated action execution", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion, InstallHint: "install zsh"},
	{Name: "ps", Purpose: "Darwin process-table inspection", Required: true, Platforms: []string{"darwin"}, InstallHint: "restore the system ps command"},
	{Name: "lsof", Purpose: "Darwin open-file inspection", Required: true, Platforms: []string{"darwin"}, VersionArgs: []string{"-v"}, Parse: firstVersion, InstallHint: "install lsof"},
	{Name: "script", Purpose: "terminal-backed command execution", InstallHint: "install util-linux or the BSD script command"},
	{Name: "setsid", Purpose: "detached Linux helper processes", Required: true, Platforms: []string{"linux"}, VersionArgs: []string{"--version"}, Parse: firstVersion, InstallHint: "install util-linux (setsid)"},
	{Name: "nohup", Purpose: "detached Linux helper fallback", Platforms: []string{"linux"}, VersionArgs: []string{"--version"}, Parse: firstVersion, InstallHint: "install coreutils (nohup)"},
	{Name: "sleep", Purpose: "bounded shell-side polling", Required: true, InstallHint: "restore the system sleep command"},
	{Name: "go", Purpose: "building a staged pfm update", VersionArgs: []string{"version"}, Parse: firstVersion, InstallHint: "install Go 1.24 or newer to use pfm update"},
	{Name: "gcloud", Purpose: "optional Vertex credentials and project discovery", VersionArgs: []string{"version"}, Parse: firstVersion, InstallHint: "install the Google Cloud CLI to use Vertex status"},
	{Name: "systemctl", Purpose: "Linux user-service wiring", Platforms: []string{"linux"}, VersionArgs: []string{"--version"}, Parse: firstVersion, InstallHint: "install systemd to enable user units"},
	{Name: "launchctl", Purpose: "Darwin launch-agent wiring", Required: true, Platforms: []string{"darwin"}, InstallHint: "restore the system launchctl command"},
}

// Registry is the one complete dependency table. Configured engine names and
// provisioned harvestpy paths are materialized here rather than copied into
// doctor or install.
func Registry(options ...Options) []Entry {
	var resolved Options
	if len(options) != 0 {
		resolved = options[0]
	}
	if resolved.Home == "" {
		resolved.Home = "."
	}
	if resolved.GOOS == "" {
		resolved.GOOS, resolved.GOARCH = runtime.GOOS, runtime.GOARCH
	}
	entries := append([]Entry(nil), fixedCommands...)
	claude := strings.TrimSpace(resolved.ClaudeBinary)
	if claude == "" {
		claude = "claude"
	}
	codex := strings.TrimSpace(resolved.CodexBinary)
	if codex == "" {
		codex = "codex"
	}
	entries = append(entries,
		Entry{Name: "claude", Command: claude, Purpose: "configured Claude engine", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion, InstallHint: "install the configured Claude CLI", SelfDoctorArgs: []string{"doctor"}},
		// Official Codex CLI reference documents doctor as stable and these
		// flags as its bounded, non-interactive summary surface:
		// https://developers.openai.com/codex/cli/reference#codex-doctor
		Entry{Name: "codex", Command: codex, Purpose: "configured Codex engine", Required: true, VersionArgs: []string{"--version"}, Parse: firstVersion, InstallHint: "install the configured Codex CLI", SelfDoctorArgs: []string{"doctor", "--summary", "--ascii", "--no-color"}},
	)
	harvestRoot := filepath.Join(resolved.Home, ".local", "state", "pfm", "harvest-python")
	current := filepath.Join(harvestRoot, "env", resolved.GOOS+"-"+resolved.GOARCH, "current")
	entries = append(entries,
		Entry{Name: "uv", Command: filepath.Join(current, "uv"), Purpose: "provisioned harvestpy package verifier", Required: true, Platforms: []string{"linux", "darwin"}, VersionArgs: []string{"--version"}, Parse: firstVersion, InstallHint: "run pfm install to provision harvestpy", Harvest: true},
		Entry{Name: "harvestpy", Command: filepath.Join(current, "project", ".venv", "bin", "python"), Purpose: "provisioned harvestpy interpreter", Required: true, Platforms: []string{"linux", "darwin"}, VersionArgs: []string{"--version"}, Parse: firstVersion, InstallHint: "run pfm install to provision harvestpy", Harvest: true},
	)
	for index := range entries {
		if entries[index].Command == "" {
			entries[index].Command = entries[index].Name
		}
	}
	return entries
}

// Registered reports whether name is a fixed literal or stable registry key.
func Registered(name string) bool {
	for _, entry := range Registry(Options{Home: ".", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}) {
		if entry.Name == name || entry.Command == name {
			return true
		}
	}
	return false
}

// Resolve is the only production seam that obtains an executable path.
// Config-owned names are permitted even though source-literal names are held
// to the registry by the source guard.
func Resolve(name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("dependency command is empty")
	}
	for _, entry := range Registry(Options{Home: ".", GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}) {
		if entry.Name != name && entry.Command != name {
			continue
		}
		if !entry.AppliesTo(runtime.GOOS) {
			return "", fmt.Errorf("dependency %s is not supported on %s", entry.Name, runtime.GOOS)
		}
		break
	}
	return exec.LookPath(name)
}

// Executable preserves exec.Cmd's normal not-found error while ensuring that
// every successful PATH lookup crosses Resolve.
func Executable(name string) string {
	path, err := Resolve(name)
	if err != nil {
		return name
	}
	return path
}

// AppliesTo reports whether the entry belongs to goos.
func (entry Entry) AppliesTo(goos string) bool {
	if len(entry.Platforms) == 0 {
		return true
	}
	for _, platform := range entry.Platforms {
		if platform == goos {
			return true
		}
	}
	return false
}

func prefixedVersion(prefix string) func(string) (string, error) {
	return func(output string) (string, error) {
		line := FirstLine(output)
		if !strings.HasPrefix(line, prefix) {
			return "", fmt.Errorf("expected %q prefix", prefix)
		}
		return firstVersion(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
	}
}

func firstVersion(output string) (string, error) {
	line := FirstLine(output)
	for _, field := range strings.Fields(line) {
		trimmed := strings.TrimLeft(field, "vV")
		if trimmed != "" && trimmed[0] >= '0' && trimmed[0] <= '9' {
			return strings.TrimRight(trimmed, ",;"), nil
		}
	}
	return "", fmt.Errorf("no version in %q", line)
}

// FirstLine bounds arbitrary command output for a visible doctor row.
func FirstLine(output string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(output), "\n")
	return strings.TrimSpace(line)
}
