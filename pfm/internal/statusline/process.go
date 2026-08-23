package statusline

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	pfmengine "hostops/pfm/internal/engine"
	"syscall"
	"time"

	"hostops/pfm/internal/deps"
)

// SpawnDetached starts one refresher as a new session and releases the child.
// The render path never waits for credentials, networks, or App Server startup.
func SpawnDetached(kind RefreshKind) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve pfm executable: %w", err)
	}
	argument := ""
	switch kind {
	case RefreshKindVertex:
		argument = "--refresh-vertex"
	case RefreshKindGPT:
		argument = "--refresh-gpt"
	default:
		return fmt.Errorf("unknown statusline refresher %q", kind)
	}
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open null device: %w", err)
	}
	defer null.Close()
	command := exec.Command(executable, "statusline", argument)
	command.Stdin = null
	command.Stdout = null
	command.Stderr = null
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start detached %s refresher: %w", kind, err)
	}
	if err := command.Process.Release(); err != nil {
		return fmt.Errorf("release detached %s refresher: %w", kind, err)
	}
	return nil
}

// GCloudAccessToken reads an OAuth token without ever placing it in argv.
func GCloudAccessToken(ctx context.Context) (string, error) {
	child, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(child, deps.Executable("gcloud"), "auth", "print-access-token").Output()
	if err != nil {
		return "", fmt.Errorf("gcloud auth print-access-token: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// ResolveVertexProject uses explicit environment first, then gcloud's active
// project. No machine- or account-specific project name is compiled into pfm.
func ResolveVertexProject(ctx context.Context) (string, error) {
	for _, name := range []string{"PFM_VERTEX_PROJECT", "GOOGLE_CLOUD_PROJECT"} {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value, nil
		}
	}
	child, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(
		child,
		deps.Executable("gcloud"), "config", "get-value", "project",
	).Output()
	if err != nil {
		return "", fmt.Errorf("gcloud config get-value project: %w", err)
	}
	project := strings.TrimSpace(string(output))
	if project == "" || project == "(unset)" {
		return "", fmt.Errorf("gcloud has no active project")
	}
	return project, nil
}

// ReadGPTRateLimits performs the complete Codex App Server initialize exchange
// and returns its id=1 response. It runs only in a detached refresher child.
func ReadGPTRateLimits(ctx context.Context) ([]byte, error) {
	return ReadGPTRateLimitsWithBinary(ctx, pfmengine.MustLookup(pfmengine.Codex).Binary)
}

// ReadGPTRateLimitsWithBinary uses the machine-configured Codex command.
func ReadGPTRateLimitsWithBinary(ctx context.Context, binary string) ([]byte, error) {
	if binary == "" {
		binary = pfmengine.MustLookup(pfmengine.Codex).Binary
	}
	child, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	return readGPTRateLimitsCommand(exec.CommandContext(child, deps.Executable(binary), "app-server"))
}

func readGPTRateLimitsCommand(command *exec.Cmd) ([]byte, error) {
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open App Server stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open App Server stdout: %w", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start Codex App Server: %w", err)
	}
	payload := strings.Join([]string{
		`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"clientInfo":{"name":"statusline","version":"1.0.0"}}}`,
		`{"jsonrpc":"2.0","method":"initialized","params":null}`,
		`{"jsonrpc":"2.0","id":1,"method":"account/rateLimits/read","params":null}`,
	}, "\n") + "\n"
	if _, err := io.WriteString(stdin, payload); err != nil {
		_ = stdin.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, fmt.Errorf("write App Server handshake: %w", err)
	}
	_ = stdin.Close()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		var envelope struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal(line, &envelope) == nil && string(envelope.ID) == "1" {
			_ = command.Process.Kill()
			_ = command.Wait()
			return append(line, '\n'), nil
		}
	}
	if err := scanner.Err(); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, fmt.Errorf("read App Server response: %w", err)
	}
	waitErr := command.Wait()
	return nil, fmt.Errorf(
		"App Server returned no id=1 response: wait=%v stderr=%s",
		waitErr,
		strings.TrimSpace(stderr.String()),
	)
}
